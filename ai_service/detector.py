"""YOLO inference + uncertainty measurement module.

Performance patterns ported from the 乌骨鸡 project:
- Batch inference: model.predict(source=image_list) for GPU-batched processing
- FP16 half-precision: half=True for ~1.5-2x speedup on modern GPUs
- Fixed imgsz=640: matches training config for optimal accuracy
- Vectorized extraction: batch .cpu().numpy() instead of per-box GPU sync
- Strict pre-processing NMS (IoU > 0.8) for dense scenes
"""

import math
import random
import logging
import threading
import time
import numpy as np
import cv2
import torch

logger = logging.getLogger(__name__)

STRICT_NMS_IOU = 0.8
INFER_IMGSZ = 640


class DetectorParams:
    """Mutable detection parameters (updated via gRPC UpdateParams)."""

    def __init__(self):
        self.nms_threshold: float = 0.9  # 非极大值抑制的IoU阈值
        self.confidence_threshold: float = 0.25 # 置信度阈值，低于此值的会被直接过滤掉，
        self.entropy_threshold: float = 0.75 # 熵阈值，异常评分高于此值被标记成不确定
        self.w1: float = 0.6 # 异常评分，w1 * (1 - conf) + w2 * (1 - max_iou)
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


def apply_strict_nms(detections: list[dict], iou_threshold: float = STRICT_NMS_IOU) -> list[dict]:
    """Strict NMS to merge highly overlapping detections in dense scenes.

    Runs BEFORE entropy computation so uncertainty is measured on clean boxes.
    Uses IoU > 0.8 to suppress near-duplicate boxes ("double-box jitter").
    """
    if len(detections) <= 1:
        return detections

    sorted_dets = sorted(detections, key=lambda d: d["confidence"], reverse=True)
    keep: list[dict] = []

    for det in sorted_dets:
        box_a = (det["x1"], det["y1"], det["x2"], det["y2"])
        suppressed = False
        for kept in keep:
            box_b = (kept["x1"], kept["y1"], kept["x2"], kept["y2"])
            if compute_iou(box_a, box_b) > iou_threshold:
                suppressed = True
                break
        if not suppressed:
            keep.append(det)

    if len(keep) < len(detections):
        logger.debug("Strict NMS: %d -> %d detections", len(detections), len(keep))

    return keep


def _extract_detections_vectorized(result) -> list[dict]:
    """Vectorized extraction of detections from a single YOLO result.

    Uses batch .cpu().numpy() on the full tensor instead of per-box
    GPU->CPU transfers, reducing synchronization overhead significantly.
    """
    boxes = result.boxes
    if boxes is None or len(boxes) == 0:
        return []

    xyxy_all = boxes.xyxy.cpu().numpy()
    conf_all = boxes.conf.cpu().numpy()
    cls_all = boxes.cls.cpu().numpy()
    names = result.names

    detections = []
    for i in range(len(xyxy_all)):
        cls_id = int(cls_all[i])
        detections.append({
            "x1": float(xyxy_all[i][0]),
            "y1": float(xyxy_all[i][1]),
            "x2": float(xyxy_all[i][2]),
            "y2": float(xyxy_all[i][3]),
            "confidence": float(conf_all[i]),
            "class_id": cls_id,
            "class_name": names.get(cls_id, str(cls_id)),
        })

    return detections


class Detector:
    """Wraps YOLO model inference + uncertainty scoring. Falls back to mock mode."""

    def __init__(self, model_manager, params: DetectorParams | None = None):
        self._mm = model_manager
        self.params = params or DetectorParams()
        self._infer_lock = threading.Lock()

    def detect(self, image: np.ndarray) -> tuple[list[dict], float]:
        """Run detection on a single image.

        Returns:
            (detections, infer_ms)
            infer_ms is pure YOLO model.predict latency in milliseconds.
        """
        model = self._mm.model
        if model is not None:
            return self._real_detect(image, model)
        return self._mock_detect(image), 0.0  # 没有模型时，模拟运行

    def detect_batch(self, images: list[np.ndarray]) -> tuple[list[list[dict]], float]:
        """Batch inference — process multiple images in one GPU call.

        This is the #1 performance optimization ported from the 乌骨鸡 project.
        YOLO's predict(source=image_list) batches images on GPU, which:
        - Amortizes kernel launch overhead
        - Maximizes GPU memory bandwidth utilization
        - Yields 3-5x throughput improvement over sequential single-frame calls
        """
        model = self._mm.model
        if model is not None:
            return self._real_detect_batch(images, model)
        return [self._mock_detect(img) for img in images], 0.0

    def _real_detect(self, image: np.ndarray, model) -> tuple[list[dict], float]:
        """Single-frame inference (kept for backward compatibility)."""
        with self._infer_lock:
            infer_start = time.perf_counter()
            results = model.predict(
                source=image,
                conf=self.params.confidence_threshold,
                iou=self.params.nms_threshold,
                device=self._mm.device,
                half=self._mm.use_half,
                imgsz=INFER_IMGSZ,
                verbose=False,
            )
            infer_ms = (time.perf_counter() - infer_start) * 1000.0

        if not results or len(results) == 0:
            return [], infer_ms

        detections = _extract_detections_vectorized(results[0])
        detections = apply_strict_nms(detections)
        self._add_entropy_and_uncertainty(detections)
        return detections, infer_ms

    def _real_detect_batch(self, images: list[np.ndarray], model) -> tuple[list[list[dict]], float]:
        """Batch inference — mirrors 乌骨鸡 project's predict(source=image_list) pattern."""
        with self._infer_lock:
            infer_start = time.perf_counter()
            results_batch = model.predict(
                source=images,
                conf=self.params.confidence_threshold,
                iou=self.params.nms_threshold,
                device=self._mm.device,
                half=self._mm.use_half,
                imgsz=INFER_IMGSZ,
                verbose=False,
            )
            infer_ms = (time.perf_counter() - infer_start) * 1000.0

        all_detections: list[list[dict]] = []
        for result in results_batch:
            detections = _extract_detections_vectorized(result)
            detections = apply_strict_nms(detections)
            self._add_entropy_and_uncertainty(detections)
            all_detections.append(detections)

        return all_detections, infer_ms

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

    def _add_entropy_and_uncertainty(self, detections: list[dict]):
        """Compute entropy then anomaly scoring in one pass."""
        for det in detections:
            probs = [det["confidence"], 1.0 - det["confidence"]]
            det["entropy"] = compute_entropy(probs)
        self._compute_uncertainty(detections)

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
