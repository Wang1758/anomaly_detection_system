"""YOLO inference + uncertainty measurement module."""

import math
import random
import logging
import numpy as np
import cv2

logger = logging.getLogger(__name__)


class DetectorParams:
    """Mutable detection parameters (updated via gRPC UpdateParams)."""

    def __init__(self):
        self.nms_threshold: float = 0.8
        self.confidence_threshold: float = 0.25
        self.entropy_threshold: float = 0.5
        self.w1: float = 0.6
        self.w2: float = 0.4


def compute_entropy(probs: list[float]) -> float:
    """Compute Shannon entropy: H(x) = -sum(p * log(p))."""
    return -sum(p * math.log(p + 1e-10) for p in probs if p > 0)


def compute_anomaly_score(conf: float, iou_neighbors: float, w1: float, w2: float) -> float:
    """Anomaly = w1 * (1 - conf_max) + w2 * (1 - IoU_neighbors)."""
    return w1 * (1.0 - conf) + w2 * (1.0 - iou_neighbors)


def compute_iou(box_a: tuple, box_b: tuple) -> float:
    """Compute IoU between two boxes (x1, y1, x2, y2)."""
    xa = max(box_a[0], box_b[0])
    ya = max(box_a[1], box_b[1])
    xb = min(box_a[2], box_b[2])
    yb = min(box_a[3], box_b[3])

    inter = max(0, xb - xa) * max(0, yb - ya)
    area_a = (box_a[2] - box_a[0]) * (box_a[3] - box_a[1])
    area_b = (box_b[2] - box_b[0]) * (box_b[3] - box_b[1])
    union = area_a + area_b - inter
    return inter / (union + 1e-10)


class Detector:
    """Wraps YOLO model inference + uncertainty scoring. Falls back to mock mode."""

    def __init__(self, model_manager, params: DetectorParams | None = None):
        self._mm = model_manager
        self.params = params or DetectorParams()

    def detect(self, image: np.ndarray) -> list[dict]:
        """Run detection on image, return list of detection dicts."""
        model = self._mm.model
        if model is not None:
            return self._real_detect(image, model)
        return self._mock_detect(image)

    def _real_detect(self, image: np.ndarray, model) -> list[dict]:
        results = model(
            image,
            conf=self.params.confidence_threshold,
            iou=self.params.nms_threshold,
            verbose=False,
        )

        detections = []
        if not results or len(results) == 0:
            return detections

        result = results[0]
        boxes = result.boxes
        if boxes is None or len(boxes) == 0:
            return detections

        for i in range(len(boxes)):
            xyxy = boxes.xyxy[i].cpu().numpy()
            conf = float(boxes.conf[i].cpu().numpy())
            cls_id = int(boxes.cls[i].cpu().numpy())
            cls_name = result.names.get(cls_id, str(cls_id))

            probs = [conf, 1.0 - conf]
            entropy = compute_entropy(probs)

            det = {
                "x1": float(xyxy[0]), "y1": float(xyxy[1]),
                "x2": float(xyxy[2]), "y2": float(xyxy[3]),
                "confidence": conf,
                "class_id": cls_id,
                "class_name": cls_name,
                "entropy": entropy,
            }
            detections.append(det)

        self._compute_uncertainty(detections)
        return detections

    def _mock_detect(self, image: np.ndarray) -> list[dict]:
        """Generate random mock detections for testing without a model."""
        h, w = image.shape[:2]
        num = random.randint(1, 5)
        detections = []

        for _ in range(num):
            bw = random.randint(30, min(120, w // 3))
            bh = random.randint(30, min(120, h // 3))
            x1 = random.randint(0, max(0, w - bw))
            y1 = random.randint(0, max(0, h - bh))
            conf = random.uniform(0.15, 0.95)
            probs = [conf, 1.0 - conf]
            entropy = compute_entropy(probs)

            det = {
                "x1": float(x1), "y1": float(y1),
                "x2": float(x1 + bw), "y2": float(y1 + bh),
                "confidence": conf,
                "class_id": 0,
                "class_name": "chicken",
                "entropy": entropy,
            }
            detections.append(det)

        self._compute_uncertainty(detections)
        return detections

    def _compute_uncertainty(self, detections: list[dict]):
        """Compute anomaly_score and is_uncertain flag for each detection."""
        boxes = [(d["x1"], d["y1"], d["x2"], d["y2"]) for d in detections]
        p = self.params

        for i, det in enumerate(detections):
            max_iou = 0.0
            for j, other_box in enumerate(boxes):
                if i != j:
                    max_iou = max(max_iou, compute_iou(boxes[i], other_box))

            score = compute_anomaly_score(det["confidence"], max_iou, p.w1, p.w2)
            det["anomaly_score"] = score
            det["is_uncertain"] = (
                det["entropy"] > p.entropy_threshold or score > p.entropy_threshold
            )
