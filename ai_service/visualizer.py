"""OpenCV-based frame visualization for detection results."""

import cv2
import numpy as np

NORMAL_COLOR = (0, 255, 0)
UNCERTAIN_COLOR = (0, 0, 255)
FONT = cv2.FONT_HERSHEY_SIMPLEX


def draw_detections(image: np.ndarray, detections: list[dict]) -> np.ndarray:
    """Draw bounding boxes and labels on the image.

    Normal detections get green boxes, uncertain ones get red.
    """
    vis = image.copy()

    for det in detections:
        x1, y1 = int(det["x1"]), int(det["y1"])
        x2, y2 = int(det["x2"]), int(det["y2"])
        conf = det["confidence"]
        is_uncertain = det.get("is_uncertain", False)

        color = UNCERTAIN_COLOR if is_uncertain else NORMAL_COLOR
        thickness = 2

        cv2.rectangle(vis, (x1, y1), (x2, y2), color, thickness)

        label = f'{det.get("class_name", "obj")} {conf:.2f}'
        if is_uncertain:
            label += " [?]"

        (tw, th), _ = cv2.getTextSize(label, FONT, 0.5, 1)
        cv2.rectangle(vis, (x1, y1 - th - 6), (x1 + tw, y1), color, -1)
        cv2.putText(vis, label, (x1, y1 - 4), FONT, 0.5, (255, 255, 255), 1)

    return vis


def encode_jpeg(image: np.ndarray, quality: int = 85) -> bytes:
    """Encode an image as JPEG bytes."""
    params = [cv2.IMWRITE_JPEG_QUALITY, quality]
    _, buf = cv2.imencode(".jpg", image, params)
    return buf.tobytes()
