"""gRPC server for AI detection service.

Supports both single-frame Detect and batch BatchDetect RPCs.
BatchDetect mirrors 乌骨鸡 project's batch inference pattern for
significantly higher GPU throughput.
"""

import os
import sys
import argparse
import logging
import time
import json
import threading
import subprocess
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from concurrent import futures

import grpc
import cv2
import numpy as np

sys.path.insert(0, os.path.dirname(__file__))

from proto import detection_pb2, detection_pb2_grpc
from model_manager import ModelManager
from detector import Detector, DetectorParams
from visualizer import draw_detections, encode_jpeg  # noqa: F401 — kept for standalone/debug use

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)

MODEL_DIR = os.environ.get("MODEL_DIR", "models")
DEFAULT_MODEL = "best.pt"
GRPC_PORT = os.environ.get("GRPC_PORT", "50051")
EVAL_HTTP_PORT = int(os.environ.get("EVAL_HTTP_PORT", "50052"))
MAX_WORKERS = 4
VIS_JPEG_QUALITY = int(os.environ.get("VIS_JPEG_QUALITY", "82"))  # kept for debug/standalone use
ORIGINAL_JPEG_QUALITY = int(os.environ.get("ORIGINAL_JPEG_QUALITY", "90"))  # kept for debug/standalone use
MODEL_INPUT_WIDTH = int(os.environ.get("MODEL_INPUT_WIDTH", "640"))
MODEL_INPUT_HEIGHT = int(os.environ.get("MODEL_INPUT_HEIGHT", "640"))
MODEL_INPUT_CHANNELS = 3
RAW_IMAGE_BYTES = MODEL_INPUT_WIDTH * MODEL_INPUT_HEIGHT * MODEL_INPUT_CHANNELS
PERF_LOG_ENABLED = False
EVAL_LOCK = threading.Lock()


def _decode_image(img_bytes: bytes) -> np.ndarray | None:
    # Fast path: Go side sends fixed-shape raw RGB tensor bytes.
    if len(img_bytes) == RAW_IMAGE_BYTES:
        rgb = np.frombuffer(img_bytes, dtype=np.uint8).reshape(
            MODEL_INPUT_HEIGHT, MODEL_INPUT_WIDTH, MODEL_INPUT_CHANNELS
        )
        return cv2.cvtColor(rgb, cv2.COLOR_RGB2BGR)

    # Backward-compatible fallback: JPEG encoded bytes.
    arr = np.frombuffer(img_bytes, dtype=np.uint8)
    return cv2.imdecode(arr, cv2.IMREAD_COLOR)


def _perf_log(msg: str, *args):
    if PERF_LOG_ENABLED:
        logger.info(msg, *args)


def _resolve_eval_script() -> str:
    return os.path.join(os.path.dirname(__file__), "evaluate_map.py")


def _resolve_existing_dataset_dir(raw_dataset_dir: str) -> tuple[str, list[str]]:
    candidates: list[str] = []

    def add_candidate(path: str):
        p = str(path).strip()
        if not p:
            return
        if p not in candidates:
            candidates.append(p)

    add_candidate(raw_dataset_dir)

    normalized = raw_dataset_dir.replace("\\", "/")
    add_candidate(normalized)

    if normalized.endswith("/data/indoor"):
        add_candidate(normalized.replace("/data/indoor", "/indoor"))
    if normalized.endswith("\\data\\indoor"):
        add_candidate(normalized.replace("\\data\\indoor", "\\indoor"))

    add_candidate("../indoor")
    add_candidate(os.path.join(os.path.dirname(__file__), "..", "indoor"))

    for c in candidates:
        if os.path.isdir(c):
            return c, candidates

    return raw_dataset_dir, candidates


def _run_eval_request(payload: dict) -> tuple[int, dict]:
    if not EVAL_LOCK.acquire(blocking=False):
        return 409, {"ok": False, "error": "evaluation already running", "logs": []}

    try:
        dataset_dir_raw = str(payload.get("dataset_dir") or os.environ.get("MAP_EVAL_DATASET_DIR") or "../indoor")
        dataset_dir, dataset_candidates = _resolve_existing_dataset_dir(dataset_dir_raw)
        model_path = str(payload.get("model") or os.environ.get("MAP_EVAL_MODEL_PATH") or os.path.join(MODEL_DIR, DEFAULT_MODEL))
        metrics_out = str(payload.get("metrics_out") or os.path.join(MODEL_DIR, "map_eval_metrics.json"))
        imgsz = int(payload.get("imgsz") or os.environ.get("TRAINING_IMGSZ") or 640)
        device = str(payload.get("device") or os.environ.get("MAP_EVAL_DEVICE") or os.environ.get("TRAINING_DEVICE") or "")

        cmd = [
            sys.executable,
            _resolve_eval_script(),
            "--dataset-dir", dataset_dir,
            "--model", model_path,
            "--metrics-out", metrics_out,
            "--imgsz", str(max(32, imgsz)),
        ]
        if device.strip():
            cmd.extend(["--device", device.strip()])

        proc = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
        )

        merged_output = "\n".join([x for x in [proc.stdout, proc.stderr] if x])
        logs = [line for line in merged_output.splitlines() if line.strip()]

        payload_out = {
            "ok": proc.returncode == 0,
            "dataset": dataset_dir,
            "dataset_candidates": dataset_candidates,
            "model": model_path,
            "device": device or "auto",
            "logs": logs,
        }

        for line in reversed(logs):
            s = line.strip()
            if not s.startswith("{") or not s.endswith("}"):
                continue
            try:
                maybe = json.loads(s)
                if isinstance(maybe, dict):
                    payload_out.update(maybe)
                    break
            except Exception:
                continue

        if proc.returncode != 0:
            if not payload_out.get("error"):
                payload_out["error"] = f"evaluate_map.py exited with {proc.returncode}"
            return 500, payload_out
        return 200, payload_out
    except Exception as exc:
        return 500, {"ok": False, "error": str(exc), "logs": []}
    finally:
        EVAL_LOCK.release()


class _EvalHTTPHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        logger.info("EvalHTTP %s", format % args)

    def do_POST(self):
        if self.path != "/eval-map":
            self._write_json(404, {"ok": False, "error": "not found"})
            return

        try:
            content_len = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            content_len = 0
        raw = self.rfile.read(content_len) if content_len > 0 else b"{}"
        try:
            payload = json.loads(raw.decode("utf-8")) if raw else {}
            if not isinstance(payload, dict):
                payload = {}
        except Exception:
            payload = {}

        status, data = _run_eval_request(payload)
        self._write_json(status, data)

    def _write_json(self, status: int, data: dict):
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def _build_detect_response(
    image: np.ndarray,
    det_dicts: list[dict],
    frame_id: int,
) -> tuple["detection_pb2.DetectResponse", float]:
    """Build a DetectResponse with detection coordinates only.

    Visualization and image encoding are no longer performed here.
    The Go backend handles high-res streaming and the frontend draws
    bounding boxes on a canvas overlay.
    """
    has_uncertain = any(d.get("is_uncertain", False) for d in det_dicts)

    serialize_start = time.perf_counter()
    meta_list = []
    for d in det_dicts:
        meta_list.append(detection_pb2.DetectionMeta(
            x1=d["x1"], y1=d["y1"], x2=d["x2"], y2=d["y2"],
            confidence=d["confidence"],
            class_id=d["class_id"],
            class_name=d["class_name"],
            is_uncertain=d.get("is_uncertain", False),
            entropy=d.get("entropy", 0.0),
            anomaly_score=d.get("anomaly_score", 0.0),
        ))
    coords_serialize_ms = (time.perf_counter() - serialize_start) * 1000.0

    response = detection_pb2.DetectResponse(
        detections=meta_list,
        has_uncertain=has_uncertain,
        frame_id=frame_id,
    )
    return response, coords_serialize_ms


class DetectionServicer(detection_pb2_grpc.DetectionServiceServicer):

    def __init__(self):
        self._params = DetectorParams()
        self._mm = ModelManager(MODEL_DIR)
        self._mm.load_initial(DEFAULT_MODEL)
        self._detector = Detector(self._mm, self._params)

    def Detect(self, request, context):
        req_start = time.perf_counter()
        payload_mode = "raw_rgb" if len(request.image) == RAW_IMAGE_BYTES else "jpeg"
        decode_start = time.perf_counter()
        image = _decode_image(request.image)
        decode_ms = (time.perf_counter() - decode_start) * 1000.0
        if image is None:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("Failed to decode image")
            return detection_pb2.DetectResponse()

        detections, infer_ms = self._detector.detect(image)
        response, coords_pack_ms = _build_detect_response(image, detections, request.frame_id)
        total_ms = (time.perf_counter() - req_start) * 1000.0
        _perf_log(
            "Detect perf frame_id=%d mode=%s decode_ms=%.2f infer_ms=%.2f coords_pack_ms=%.2f total_ms=%.2f det_count=%d",
            request.frame_id,
            payload_mode,
            decode_ms,
            infer_ms,
            coords_pack_ms,
            total_ms,
            len(detections),
        )
        return response

    def BatchDetect(self, request, context):
        """Batch inference — processes N frames in one GPU call.

        Mirrors 乌骨鸡 project's batch processing pattern:
          results = model.predict(source=image_list, ...)
        GPU batching yields 3-5x throughput vs sequential single-frame calls.
        """
        req_start = time.perf_counter()
        images = []
        frame_ids = list(request.frame_ids)
        decode_ms_total = 0.0
        raw_count = 0
        jpeg_count = 0

        for i, img_bytes in enumerate(request.images):
            if len(img_bytes) == RAW_IMAGE_BYTES:
                raw_count += 1
            else:
                jpeg_count += 1
            decode_start = time.perf_counter()
            image = _decode_image(img_bytes)
            decode_ms_total += (time.perf_counter() - decode_start) * 1000.0
            if image is None:
                logger.warning("BatchDetect: failed to decode image at index %d", i)
                images.append(np.zeros((640, 640, 3), dtype=np.uint8))
            else:
                images.append(image)

        if not images:
            return detection_pb2.BatchDetectResponse()

        all_detections, infer_ms = self._detector.detect_batch(images)

        results = []
        coords_pack_ms_total = 0.0
        for i, (image, dets) in enumerate(zip(images, all_detections)):
            fid = frame_ids[i] if i < len(frame_ids) else 0
            resp, coords_pack_ms = _build_detect_response(image, dets, fid)
            coords_pack_ms_total += coords_pack_ms
            results.append(resp)

        total_ms = (time.perf_counter() - req_start) * 1000.0
        batch_size = len(images)
        avg_decode_ms = decode_ms_total / batch_size if batch_size > 0 else 0.0
        avg_pack_ms = coords_pack_ms_total / batch_size if batch_size > 0 else 0.0
        _perf_log(
            "BatchDetect perf batch=%d mode_raw=%d mode_jpeg=%d decode_ms_total=%.2f decode_ms_avg=%.2f infer_ms=%.2f coords_pack_ms_total=%.2f coords_pack_ms_avg=%.2f total_ms=%.2f",
            batch_size,
            raw_count,
            jpeg_count,
            decode_ms_total,
            avg_decode_ms,
            infer_ms,
            coords_pack_ms_total,
            avg_pack_ms,
            total_ms,
        )

        return detection_pb2.BatchDetectResponse(results=results)

    def ReloadModel(self, request, context):
        success, msg = self._mm.reload(request.model_path)
        return detection_pb2.ReloadResponse(success=success, message=msg)

    def UpdateParams(self, request, context):
        if request.nms_threshold > 0:
            self._params.nms_threshold = request.nms_threshold
        if request.confidence_threshold > 0:
            self._params.confidence_threshold = request.confidence_threshold
        if request.entropy_threshold > 0:
            self._params.entropy_threshold = request.entropy_threshold
        if request.w1 > 0:
            self._params.w1 = request.w1
        if request.w2 > 0:
            self._params.w2 = request.w2

        logger.info("Params updated: nms=%.2f conf=%.2f entropy=%.2f w1=%.2f w2=%.2f",
                     self._params.nms_threshold, self._params.confidence_threshold,
                     self._params.entropy_threshold, self._params.w1, self._params.w2)
        return detection_pb2.ParamsResponse(success=True, message="Parameters updated")


def serve(perf_log: bool = False):
    global PERF_LOG_ENABLED
    PERF_LOG_ENABLED = perf_log

    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=MAX_WORKERS),
        options=[
            ("grpc.max_send_message_length", 100 * 1024 * 1024),
            ("grpc.max_receive_message_length", 100 * 1024 * 1024),
        ],
    )
    detection_pb2_grpc.add_DetectionServiceServicer_to_server(
        DetectionServicer(), server
    )

    eval_http = ThreadingHTTPServer(("0.0.0.0", EVAL_HTTP_PORT), _EvalHTTPHandler)
    threading.Thread(target=eval_http.serve_forever, daemon=True).start()
    logger.info("AI Eval HTTP started on 0.0.0.0:%d", EVAL_HTTP_PORT)

    addr = f"[::]:{GRPC_PORT}"
    server.add_insecure_port(addr)
    server.start()
    logger.info("AI Detection Service started on %s", addr)
    server.wait_for_termination()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="AI detection gRPC server")
    parser.add_argument("--perf-log", action="store_true", help="enable performance logs")
    args = parser.parse_args()

    if args.perf_log:
        logger.info("Performance logging enabled")

    serve(perf_log=args.perf_log)
