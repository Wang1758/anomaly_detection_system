import argparse
import json
import shutil
import sqlite3
from pathlib import Path

import cv2


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--db-path", required=True)
    parser.add_argument("--data-dir", required=True)
    parser.add_argument("--output-model", required=True)
    parser.add_argument("--metrics-out", required=True)
    parser.add_argument("--base-dataset", default="")
    parser.add_argument("--base-model", default="")
    parser.add_argument("--epochs", type=int, default=10)
    parser.add_argument("--imgsz", type=int, default=640)
    parser.add_argument("--batch", type=int, default=8)
    parser.add_argument("--device", default="")
    return parser.parse_args()


def resolve_image_path(data_dir: Path, image_path: str) -> Path:
    p = Path(image_path)
    if p.is_absolute():
        return p
    candidate = data_dir / "images" / p
    if candidate.exists():
        return candidate
    return data_dir / p


def load_human_labeled_samples(db_path: Path) -> list[tuple[int, str, bool, str]]:
    conn = sqlite3.connect(str(db_path))
    try:
        cur = conn.cursor()
        cur.execute(
            """
            SELECT id, image_path, label, detections_json
            FROM samples
            WHERE status = 'labeled' AND source = 'human' AND label IS NOT NULL
            ORDER BY id ASC
            """
        )
        rows = cur.fetchall()
        out = []
        for row in rows:
            sample_id = int(row[0])
            image_path = str(row[1])
            label = bool(row[2])
            detections_json = str(row[3] or "[]")
            out.append((sample_id, image_path, label, detections_json))
        return out
    finally:
        conn.close()


def parse_detections_json(raw: str) -> list[dict]:
    try:
        payload = json.loads(raw)
    except Exception:
        return []
    if not isinstance(payload, list):
        return []
    out = []
    for item in payload:
        if not isinstance(item, dict):
            continue
        if not all(key in item for key in ("x1", "y1", "x2", "y2")):
            continue
        out.append(item)
    return out


def to_yolo_bbox(det: dict, width: int, height: int) -> str | None:
    x1 = float(det["x1"])
    y1 = float(det["y1"])
    x2 = float(det["x2"])
    y2 = float(det["y2"])
    if x2 <= x1 or y2 <= y1:
        return None

    x1 = max(0.0, min(x1, width - 1))
    y1 = max(0.0, min(y1, height - 1))
    x2 = max(0.0, min(x2, width - 1))
    y2 = max(0.0, min(y2, height - 1))

    bw = x2 - x1
    bh = y2 - y1
    if bw <= 1 or bh <= 1:
        return None

    cx = x1 + bw / 2.0
    cy = y1 + bh / 2.0

    return f"0 {cx / width:.6f} {cy / height:.6f} {bw / width:.6f} {bh / height:.6f}"


def copy_base_dataset(base_dataset: Path, image_dir: Path, label_dir: Path) -> int:
    if not base_dataset.exists():
        return 0

    src_image_dir = base_dataset / "images" / "train"
    src_label_dir = base_dataset / "labels" / "train"
    if not src_image_dir.exists() or not src_label_dir.exists():
        return 0

    copied = 0
    for img in src_image_dir.iterdir():
        if not img.is_file():
            continue
        stem = img.stem
        lbl = src_label_dir / f"{stem}.txt"
        if not lbl.exists():
            continue

        target_img = image_dir / f"base_{img.name}"
        target_lbl = label_dir / f"base_{stem}.txt"
        shutil.copy2(img, target_img)
        shutil.copy2(lbl, target_lbl)
        copied += 1
    return copied


def build_dataset(data_dir: Path, samples: list[tuple[int, str, bool, str]], base_dataset: Path | None) -> tuple[Path, int, int]:
    work_dir = data_dir / "training" / "incremental"
    image_dir = work_dir / "images" / "train"
    label_dir = work_dir / "labels" / "train"
    if work_dir.exists():
        shutil.rmtree(work_dir)
    image_dir.mkdir(parents=True, exist_ok=True)
    label_dir.mkdir(parents=True, exist_ok=True)

    base_count = 0
    if base_dataset is not None:
        base_count = copy_base_dataset(base_dataset, image_dir, label_dir)

    inc_count = 0

    for sample_id, rel_image_path, is_positive, detections_json in samples:
        src = resolve_image_path(data_dir, rel_image_path)
        if not src.exists():
            continue

        img = cv2.imread(str(src))
        if img is None:
            continue
        height, width = img.shape[:2]
        if width <= 1 or height <= 1:
            continue

        ext = src.suffix.lower() if src.suffix else ".jpg"
        target_image = image_dir / f"inc_{sample_id}{ext}"
        target_label = label_dir / f"inc_{sample_id}.txt"
        shutil.copy2(src, target_image)

        if not is_positive:
            target_label.write_text("", encoding="utf-8")
            inc_count += 1
            continue

        lines = []
        for det in parse_detections_json(detections_json):
            yolo_line = to_yolo_bbox(det, width, height)
            if yolo_line is not None:
                lines.append(yolo_line)

        if not lines:
            continue

        target_label.write_text("\n".join(lines) + "\n", encoding="utf-8")
        inc_count += 1

    dataset_yaml = work_dir / "dataset.yaml"
    dataset_yaml.write_text(
        "\n".join(
            [
                f"path: {work_dir}",
                "train: images/train",
                "val: images/train",
                "nc: 1",
                "names: ['silkie']",
            ]
        ),
        encoding="utf-8",
    )
    return dataset_yaml, base_count, inc_count


def choose_base_model(args: argparse.Namespace, output_model: Path) -> str:
    if args.base_model and Path(args.base_model).exists():
        return args.base_model
    if output_model.exists():
        return str(output_model)
    local_best = Path(__file__).resolve().parent / "models" / "best.pt"
    if local_best.exists():
        return str(local_best)
    return "yolo11n.pt"


def train_and_export(args: argparse.Namespace) -> float:
    from ultralytics import YOLO

    db_path = Path(args.db_path)
    data_dir = Path(args.data_dir)
    output_model = Path(args.output_model)
    metrics_out = Path(args.metrics_out)

    if not db_path.exists():
        raise FileNotFoundError(f"database not found: {db_path}")

    samples = load_human_labeled_samples(db_path)
    if len(samples) == 0:
        raise RuntimeError("no human-labeled samples for training")

    base_dataset = Path(args.base_dataset) if args.base_dataset else None
    if base_dataset is None:
        candidate = data_dir / "training" / "base_dataset"
        if candidate.exists():
            base_dataset = candidate

    dataset_yaml, base_count, inc_count = build_dataset(data_dir, samples, base_dataset)
    if inc_count == 0:
        raise RuntimeError("no usable incremental samples after bbox filtering")

    output_model.parent.mkdir(parents=True, exist_ok=True)
    metrics_out.parent.mkdir(parents=True, exist_ok=True)

    base_model = choose_base_model(args, output_model)
    model = YOLO(base_model)

    train_kwargs = {
        "data": str(dataset_yaml),
        "epochs": max(1, args.epochs),
        "imgsz": max(32, args.imgsz),
        "batch": max(1, args.batch),
        "project": str((data_dir / "training" / "runs").resolve()),
        "name": "incremental",
        "exist_ok": True,
        "verbose": False,
    }
    if args.device:
        train_kwargs["device"] = args.device

    model.train(**train_kwargs)

    best_path = None
    if hasattr(model, "trainer") and model.trainer is not None:
        best_path = getattr(model.trainer, "best", None)
    if best_path is None:
        raise RuntimeError("training completed but best checkpoint not found")

    best_model = Path(best_path)
    if not best_model.exists():
        raise RuntimeError(f"best checkpoint not found: {best_model}")

    shutil.copy2(best_model, output_model)

    accuracy = 0.0
    if hasattr(model, "trainer") and model.trainer is not None:
        metrics = getattr(model.trainer, "metrics", None)
        if metrics is not None and hasattr(metrics, "results_dict"):
            results = metrics.results_dict or {}
            for key in ("metrics/mAP50(B)", "metrics/mAP50-95(B)", "metrics/precision(B)"):
                if key in results:
                    try:
                        accuracy = float(results[key])
                    except Exception:
                        accuracy = 0.0
                    break

    metrics_payload = {
        "accuracy": accuracy,
        "samples": len(samples),
        "incremental_samples": inc_count,
        "base_samples": base_count,
        "output_model": str(output_model),
        "base_model": base_model,
        "base_dataset": str(base_dataset) if base_dataset else "",
    }
    metrics_out.write_text(json.dumps(metrics_payload, ensure_ascii=False), encoding="utf-8")
    return accuracy


def main() -> int:
    args = parse_args()
    try:
        accuracy = train_and_export(args)
        print(json.dumps({"ok": True, "accuracy": accuracy}, ensure_ascii=False))
        return 0
    except Exception as exc:
        print(json.dumps({"ok": False, "error": str(exc)}, ensure_ascii=False))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
