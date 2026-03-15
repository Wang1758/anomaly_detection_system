"""gRPC server for AI detection service."""

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
DEFAULT_MODEL = os.path.join(MODEL_DIR, "best.pt")
GRPC_PORT = os.environ.get("GRPC_PORT", "50051")
MAX_WORKERS = int(os.environ.get("MAX_WORKERS", "4"))


class DetectionServicer(detection_pb2_grpc.DetectionServiceServicer):

    def __init__(self):
        self._params = DetectorParams()
        self._mm = ModelManager(MODEL_DIR)
        self._mm.load_initial(DEFAULT_MODEL)
        self._detector = Detector(self._mm, self._params)

    def Detect(self, request, context):
        img_bytes = request.image
        frame_id = request.frame_id

        arr = np.frombuffer(img_bytes, dtype=np.uint8)
        image = cv2.imdecode(arr, cv2.IMREAD_COLOR)
        if image is None:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("Failed to decode image")
            return detection_pb2.DetectResponse()

        detections = self._detector.detect(image)
        det_dicts = detections

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
            ("grpc.max_send_message_length", 50 * 1024 * 1024),
            ("grpc.max_receive_message_length", 50 * 1024 * 1024),
        ],
    )
    detection_pb2_grpc.add_DetectionServiceServicer_to_server(
        DetectionServicer(), server
    )
    addr = f"[::]:{GRPC_PORT}"
    server.add_insecure_port(addr)
    server.start()
    logger.info("AI Detection Service started on %s (mock=%s)", addr, not ModelManager(MODEL_DIR).is_loaded())
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
