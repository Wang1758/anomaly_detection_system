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

    h, w = vis.shape[:2]
    scale_base = max(h, w) / 1920.0
    font_scale = max(0.45, 0.5 * scale_base)
    thickness = max(2, int(round(1.8 * scale_base)))
    text_thickness = max(1, int(round(1.2 * scale_base)))

    for det in detections:
        x1, y1 = int(det["x1"]), int(det["y1"])
        x2, y2 = int(det["x2"]), int(det["y2"])
        conf = det["confidence"]
        is_uncertain = det.get("is_uncertain", False)

        color = UNCERTAIN_COLOR if is_uncertain else NORMAL_COLOR

        cv2.rectangle(vis, (x1, y1), (x2, y2), color, thickness)

        label = f'{det.get("class_name", "obj")} {conf:.2f}'
        if is_uncertain:
            label += " [?]"

        (tw, th), _ = cv2.getTextSize(label, FONT, font_scale, text_thickness)
        text_pad = max(3, int(round(4 * scale_base)))
        text_y1 = max(0, y1 - th - text_pad)
        cv2.rectangle(vis, (x1, text_y1), (x1 + tw + text_pad, y1), color, -1)
        cv2.putText(
            vis,
            label,
            (x1 + text_pad // 2, y1 - max(2, text_pad // 3)),
            FONT,
            font_scale,
            (255, 255, 255),
            text_thickness,
            cv2.LINE_AA,
        )

    return vis


def encode_jpeg(image: np.ndarray, quality: int = 95) -> bytes:
    """Encode an image as JPEG bytes."""
    params = [cv2.IMWRITE_JPEG_QUALITY, quality]
    _, buf = cv2.imencode(".jpg", image, params)
    return buf.tobytes()
