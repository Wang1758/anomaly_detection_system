"""Double-buffering model hot-swap manager."""

import threading
import logging
from pathlib import Path

logger = logging.getLogger(__name__)


class ModelManager:
    """Manages YOLO model lifecycle with atomic pointer swap for zero-downtime reloads."""

    def __init__(self, model_dir: str = "/models"):
        self._model_dir = Path(model_dir)
        self._model = None
        self._lock = threading.Lock()
        self._loading = False

    @property
    def model(self):
        return self._model

    def load_initial(self, model_path: str | None = None):
        """Load the initial model on startup. Falls back to mock mode if unavailable."""
        if model_path and Path(model_path).exists():
            try:
                from ultralytics import YOLO
                self._model = YOLO(model_path)
                logger.info("Loaded YOLO model from %s", model_path)
                return True
            except Exception as e:
                logger.warning("Failed to load model: %s. Running in mock mode.", e)
        else:
            logger.info("No model file found. Running in mock mode.")
        return False

    def reload(self, new_model_path: str) -> tuple[bool, str]:
        """Hot-swap: load new weights into buffer, then atomically swap pointer."""
        if self._loading:
            return False, "Another reload is already in progress"

        self._loading = True
        try:
            path = Path(new_model_path)
            if not path.exists():
                return False, f"Model file not found: {new_model_path}"

            from ultralytics import YOLO
            new_model = YOLO(str(path))

            with self._lock:
                old_model = self._model
                self._model = new_model

            del old_model
            logger.info("Model hot-swapped to %s", new_model_path)
            return True, "Model reloaded successfully"
        except Exception as e:
            logger.error("Model reload failed: %s", e)
            return False, str(e)
        finally:
            self._loading = False

    def is_loaded(self) -> bool:
        return self._model is not None
