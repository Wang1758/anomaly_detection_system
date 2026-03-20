"""gRPC server for AI detection service.

Supports both single-frame Detect and batch BatchDetect RPCs.
BatchDetect mirrors 乌骨鸡 project's batch inference pattern for
significantly higher GPU throughput.
"""

import os
import sys
import logging
from concurrent import futures

import grpc
import cv2
import numpy as np

sys.path.insert(0, os.path.dirname(__file__))

from proto import detection_pb2, detection_pb2_grpc
from model_manager import ModelManager
from detector import Detector, DetectorParams
from visualizer import draw_detections, encode_jpeg

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)

MODEL_DIR = os.environ.get("MODEL_DIR", "models")
DEFAULT_MODEL = "best.pt"
GRPC_PORT = os.environ.get("GRPC_PORT", "50051")
MAX_WORKERS = 4


def _decode_image(img_bytes: bytes) -> np.ndarray | None:
    arr = np.frombuffer(img_bytes, dtype=np.uint8)
    return cv2.imdecode(arr, cv2.IMREAD_COLOR)


def _build_detect_response(
    image: np.ndarray,
    det_dicts: list[dict],
    frame_id: int,
) -> "detection_pb2.DetectResponse":
    """Shared helper to build a DetectResponse from detections."""
    vis_image = draw_detections(image, det_dicts)
    vis_bytes = encode_jpeg(vis_image)

    has_uncertain = any(d.get("is_uncertain", False) for d in det_dicts)
    original_bytes = b""
    if has_uncertain:
        original_bytes = encode_jpeg(image, quality=95)

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

    return detection_pb2.DetectResponse(
        visualized_image=vis_bytes,
        original_image=original_bytes,
        detections=meta_list,
        has_uncertain=has_uncertain,
        frame_id=frame_id,
    )


class DetectionServicer(detection_pb2_grpc.DetectionServiceServicer):

    def __init__(self):
        self._params = DetectorParams()
        self._mm = ModelManager(MODEL_DIR)
        self._mm.load_initial(DEFAULT_MODEL)
        self._detector = Detector(self._mm, self._params)

    def Detect(self, request, context):
        image = _decode_image(request.image)
        if image is None:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("Failed to decode image")
            return detection_pb2.DetectResponse()

        detections = self._detector.detect(image)
        return _build_detect_response(image, detections, request.frame_id)

    def BatchDetect(self, request, context):
        """Batch inference — processes N frames in one GPU call.

        Mirrors 乌骨鸡 project's batch processing pattern:
          results = model.predict(source=image_list, ...)
        GPU batching yields 3-5x throughput vs sequential single-frame calls.
        """
        images = []
        frame_ids = list(request.frame_ids)

        for i, img_bytes in enumerate(request.images):
            image = _decode_image(img_bytes)
            if image is None:
                logger.warning("BatchDetect: failed to decode image at index %d", i)
                images.append(np.zeros((640, 640, 3), dtype=np.uint8))
            else:
                images.append(image)

        if not images:
            return detection_pb2.BatchDetectResponse()

        all_detections = self._detector.detect_batch(images)

        results = []
        for i, (image, dets) in enumerate(zip(images, all_detections)):
            fid = frame_ids[i] if i < len(frame_ids) else 0
            results.append(_build_detect_response(image, dets, fid))

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


def serve():
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
    addr = f"[::]:{GRPC_PORT}"
    server.add_insecure_port(addr)
    server.start()
    logger.info("AI Detection Service started on %s", addr)
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
