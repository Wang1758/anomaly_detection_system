"""Double-buffering model hot-swap manager.

References 乌骨鸡 project patterns:
- torch.cuda.set_device() for explicit GPU binding
- Model warmup after load to pre-compile CUDA kernels
- FP16 half-precision support for ~1.5-2x inference speedup
"""

import threading
import logging
from pathlib import Path
import os

import numpy as np
import torch

logger = logging.getLogger(__name__)


class ModelManager:
    """Manages YOLO model lifecycle with atomic pointer swap for zero-downtime reloads."""

    def __init__(self, model_dir: str = "/models"):
        self._model_dir = Path(model_dir)
        self._model = None
        self._lock = threading.Lock()
        self._loading = False

        if torch.cuda.is_available():
            self._device = "cuda:0"
            torch.cuda.set_device(self._device)
            self._use_half = True
            logger.info("CUDA available, device=%s, FP16 enabled", self._device)
        else:
            self._device = "cpu"
            self._use_half = False
            logger.info("No CUDA, running on CPU")

    @property
    def model(self):
        return self._model

    @property
    def device(self) -> str:
        return self._device

    @property
    def use_half(self) -> bool:
        return self._use_half

    def load_initial(self, model_path: str | None = None):
        """Load the initial model on startup. Falls back to mock mode if unavailable."""
        model_path = os.path.join(self._model_dir, model_path)
        if model_path and Path(model_path).exists():
            try:
                from ultralytics import YOLO
                model = YOLO(model_path)
                self._warmup(model)
                self._model = model
                logger.info("Loaded YOLO model from %s", model_path)
                return True
            except Exception as e:
                logger.warning("Failed to load model: %s. Running in mock mode.", e)
        else:
            logger.info("No model file found at %s. Running in mock mode.", model_path)
        return False

    def _warmup(self, model):
        """Run dummy inference to pre-compile CUDA kernels (eliminates cold-start).

        First CUDA inference triggers JIT compilation of GPU kernels which
        adds 2-5 seconds of latency. Running a dummy frame at startup
        amortizes this cost before real frames arrive.
        """
        logger.info("Warming up model on %s (half=%s)...", self._device, self._use_half)
        dummy = np.zeros((640, 640, 3), dtype=np.uint8)
        model.predict(
            source=dummy,
            device=self._device,
            half=self._use_half,
            imgsz=640,
            verbose=False,
        )
        logger.info("Model warmup complete")

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
            self._warmup(new_model)

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
