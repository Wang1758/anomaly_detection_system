import argparse
import json
import platform
import sys
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset-dir", required=True)
    parser.add_argument("--model", required=True)
    parser.add_argument("--metrics-out", required=True)
    parser.add_argument("--imgsz", type=int, default=640)
    parser.add_argument("--device", default="")
    return parser.parse_args()


def resolve_dataset_yaml(dataset_dir: Path) -> Path:
    for name in ("dataset.yaml", "data.yaml"):
        p = dataset_dir / name
        if p.exists():
            return p

    class_names = ["silkie"]
    classes_txt = dataset_dir / "classes.txt"
    if classes_txt.exists():
        lines = [line.strip() for line in classes_txt.read_text(encoding="utf-8").splitlines()]
        lines = [line for line in lines if line]
        if lines:
            class_names = lines

    images_val = dataset_dir / "images" / "val"
    labels_val = dataset_dir / "labels" / "val"
    use_flat_layout = False
    if not images_val.exists() or not labels_val.exists():
        images_flat = dataset_dir / "images"
        labels_flat = dataset_dir / "labels"
        if images_flat.exists() and labels_flat.exists():
            use_flat_layout = True
        else:
            raise FileNotFoundError(
                "dataset.yaml/data.yaml not found and neither images/val+labels/val nor images+labels layout exists"
            )

    yaml_path = dataset_dir / "dataset.auto.yaml"
    train_val_path = "images" if use_flat_layout else "images/val"
    names_repr = "[" + ", ".join([f"'{name}'" for name in class_names]) + "]"
    yaml_path.write_text(
        "\n".join(
            [
                f"path: {dataset_dir}",
                f"train: {train_val_path}",
                f"val: {train_val_path}",
                f"nc: {len(class_names)}",
                f"names: {names_repr}",
            ]
        ),
        encoding="utf-8",
    )
    return yaml_path


def main() -> int:
    args = parse_args()

    dataset_dir = Path(args.dataset_dir)
    model_path = Path(args.model)
    metrics_out = Path(args.metrics_out)

    if not dataset_dir.exists():
        print(json.dumps({"ok": False, "error": f"dataset dir not found: {dataset_dir}"}, ensure_ascii=False))
        return 1
    if not model_path.exists():
        print(json.dumps({"ok": False, "error": f"model not found: {model_path}"}, ensure_ascii=False))
        return 1

    try:
        import torch
        from ultralytics import YOLO

        data_yaml = resolve_dataset_yaml(dataset_dir)
        model = YOLO(str(model_path))

        if args.device and str(args.device).strip():
            chosen_device = str(args.device).strip()
        else:
            chosen_device = "0" if torch.cuda.is_available() else "cpu"

        print(
            json.dumps(
                {
                    "stage": "runtime",
                    "python": sys.version.split(" ")[0],
                    "python_executable": sys.executable,
                    "platform": platform.platform(),
                    "torch": getattr(torch, "__version__", "unknown"),
                    "cuda_available": bool(torch.cuda.is_available()),
                    "cuda_device_count": int(torch.cuda.device_count() if torch.cuda.is_available() else 0),
                    "selected_device": chosen_device,
                },
                ensure_ascii=False,
            )
        )

        val_kwargs = {
            "data": str(data_yaml),
            "imgsz": max(32, int(args.imgsz)),
            "verbose": False,
            "device": chosen_device,
        }

        metrics = model.val(**val_kwargs)

        map50 = 0.0
        map5095 = 0.0

        try:
            if hasattr(metrics, "box") and metrics.box is not None:
                map50 = float(getattr(metrics.box, "map50", 0.0) or 0.0)
                map5095 = float(getattr(metrics.box, "map", 0.0) or 0.0)
            elif hasattr(metrics, "results_dict"):
                rd = metrics.results_dict or {}
                map50 = float(rd.get("metrics/mAP50(B)", 0.0) or 0.0)
                map5095 = float(rd.get("metrics/mAP50-95(B)", 0.0) or 0.0)
        except Exception:
            map50 = 0.0
            map5095 = 0.0

        payload = {
            "map50": map50,
            "map50_95": map5095,
            "dataset": str(dataset_dir),
            "model": str(model_path),
            "device": chosen_device,
        }
        metrics_out.parent.mkdir(parents=True, exist_ok=True)
        metrics_out.write_text(json.dumps(payload, ensure_ascii=False), encoding="utf-8")
        print(json.dumps({"ok": True, **payload}, ensure_ascii=False))
        return 0
    except Exception as exc:
        print(json.dumps({"ok": False, "error": str(exc)}, ensure_ascii=False))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
