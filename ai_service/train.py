"""
智慧养殖场监控系统 - 增量训练脚本
基于已标注样本对 YOLO 模型进行 Fine-tuning
"""

import os
import sys
import json
import shutil
import logging
import argparse
import sqlite3
from datetime import datetime
from pathlib import Path
from typing import List, Dict, Tuple

import yaml

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class IncrementalTrainer:
    """增量训练器"""

    def __init__(
        self,
        data_dir: str = "../data",
        model_path: str = None,
        output_dir: str = "../data/models",
        epochs: int = 10,
        batch_size: int = 16,
        freeze_backbone: bool = True
    ):
        """
        初始化训练器
        
        Args:
            data_dir: 数据目录
            model_path: 基础模型路径（为空则使用预训练模型）
            output_dir: 输出目录
            epochs: 训练轮数
            batch_size: 批次大小
            freeze_backbone: 是否冻结骨干网络
        """
        self.data_dir = Path(data_dir)
        self.model_path = model_path
        self.output_dir = Path(output_dir)
        self.epochs = epochs
        self.batch_size = batch_size
        self.freeze_backbone = freeze_backbone

        # 数据库路径
        self.db_path = self.data_dir / "detection.db"

        # 训练数据目录
        self.train_dir = self.data_dir / "train"
        self.train_images_dir = self.train_dir / "images"
        self.train_labels_dir = self.train_dir / "labels"

        # 确保目录存在
        self.output_dir.mkdir(parents=True, exist_ok=True)
        self.train_images_dir.mkdir(parents=True, exist_ok=True)
        self.train_labels_dir.mkdir(parents=True, exist_ok=True)

    def get_untrained_samples(self) -> List[Dict]:
        """
        从数据库获取未训练的已标注样本
        
        Returns:
            样本列表
        """
        if not self.db_path.exists():
            logger.error(f"数据库文件不存在: {self.db_path}")
            return []

        conn = sqlite3.connect(str(self.db_path))
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()

        cursor.execute("""
            SELECT * FROM samples 
            WHERE label_status IN ('normal', 'abnormal') 
            AND used_for_training = 0
        """)

        samples = [dict(row) for row in cursor.fetchall()]
        conn.close()

        logger.info(f"获取到 {len(samples)} 个未训练样本")
        return samples

    def prepare_training_data(self, samples: List[Dict]) -> int:
        """
        准备训练数据（YOLO 格式）
        
        Args:
            samples: 样本列表
            
        Returns:
            有效样本数量
        """
        valid_count = 0

        for sample in samples:
            image_path = Path(sample['image_path'])
            
            if not image_path.exists():
                logger.warning(f"图片不存在: {image_path}")
                continue

            # 复制图片
            dest_image = self.train_images_dir / image_path.name
            shutil.copy(image_path, dest_image)

            # 生成标签文件（YOLO 格式）
            label_file = self.train_labels_dir / f"{image_path.stem}.txt"
            
            # 计算归一化坐标
            img_w = sample['image_width']
            img_h = sample['image_height']
            
            x1 = sample['bbox_x1']
            y1 = sample['bbox_y1']
            x2 = sample['bbox_x2']
            y2 = sample['bbox_y2']
            
            # 转换为 YOLO 格式 (class_id, x_center, y_center, width, height)
            x_center = ((x1 + x2) / 2) / img_w
            y_center = ((y1 + y2) / 2) / img_h
            width = (x2 - x1) / img_w
            height = (y2 - y1) / img_h
            
            # 根据标注结果设置类别
            # normal -> 0, abnormal -> 1
            class_id = 0 if sample['label_status'] == 'normal' else 1
            
            with open(label_file, 'w') as f:
                f.write(f"{class_id} {x_center:.6f} {y_center:.6f} {width:.6f} {height:.6f}\n")

            valid_count += 1

        logger.info(f"准备了 {valid_count} 个有效训练样本")
        return valid_count

    def create_dataset_yaml(self) -> str:
        """
        创建数据集配置文件
        
        Returns:
            配置文件路径
        """
        dataset_config = {
            'path': str(self.train_dir.absolute()),
            'train': 'images',
            'val': 'images',  # 简化版本：使用相同数据
            'names': {
                0: 'normal',
                1: 'abnormal'
            }
        }

        yaml_path = self.train_dir / "dataset.yaml"
        with open(yaml_path, 'w') as f:
            yaml.dump(dataset_config, f, default_flow_style=False)

        logger.info(f"数据集配置已保存: {yaml_path}")
        return str(yaml_path)

    def train(self) -> Tuple[bool, str]:
        """
        执行训练
        
        Returns:
            (是否成功, 新模型路径)
        """
        try:
            from ultralytics import YOLO

            # 获取未训练样本
            samples = self.get_untrained_samples()
            if not samples:
                logger.warning("没有未训练的样本")
                return False, ""

            # 准备训练数据
            valid_count = self.prepare_training_data(samples)
            if valid_count == 0:
                logger.warning("没有有效的训练样本")
                return False, ""

            # 创建数据集配置
            dataset_yaml = self.create_dataset_yaml()

            # 加载模型
            if self.model_path and Path(self.model_path).exists():
                logger.info(f"加载模型: {self.model_path}")
                model = YOLO(self.model_path)
            else:
                logger.info("加载预训练模型: yolo11n.pt")
                model = YOLO('yolo11n.pt')

            # 配置训练参数
            train_args = {
                'data': dataset_yaml,
                'epochs': self.epochs,
                'batch': self.batch_size,
                'imgsz': 640,
                'project': str(self.output_dir),
                'name': f'train_{datetime.now().strftime("%Y%m%d_%H%M%S")}',
                'exist_ok': True,
                'verbose': True,
            }

            # 冻结骨干网络
            if self.freeze_backbone:
                train_args['freeze'] = 10  # 冻结前 10 层

            logger.info("开始训练...")
            logger.info(f"训练参数: {train_args}")

            # 执行训练
            results = model.train(**train_args)

            # 获取最佳模型路径
            best_model_path = Path(results.save_dir) / "weights" / "best.pt"
            
            if best_model_path.exists():
                # 复制到标准位置
                final_model_path = self.output_dir / "latest.pt"
                shutil.copy(best_model_path, final_model_path)
                
                logger.info(f"训练完成，模型已保存: {final_model_path}")
                
                # 标记样本为已训练
                self.mark_samples_as_trained([s['id'] for s in samples])
                
                return True, str(final_model_path)
            else:
                logger.error("训练完成但未找到模型文件")
                return False, ""

        except Exception as e:
            logger.error(f"训练失败: {e}", exc_info=True)
            return False, ""

    def mark_samples_as_trained(self, sample_ids: List[int]):
        """
        标记样本为已训练
        
        Args:
            sample_ids: 样本 ID 列表
        """
        if not sample_ids:
            return

        conn = sqlite3.connect(str(self.db_path))
        cursor = conn.cursor()

        placeholders = ','.join('?' * len(sample_ids))
        cursor.execute(f"""
            UPDATE samples 
            SET used_for_training = 1 
            WHERE id IN ({placeholders})
        """, sample_ids)

        conn.commit()
        conn.close()

        logger.info(f"已标记 {len(sample_ids)} 个样本为已训练")


def main():
    parser = argparse.ArgumentParser(description='增量训练脚本')
    parser.add_argument('--data-dir', type=str, default='../data', help='数据目录')
    parser.add_argument('--model', type=str, default=None, help='基础模型路径')
    parser.add_argument('--output', type=str, default='../data/models', help='输出目录')
    parser.add_argument('--epochs', type=int, default=10, help='训练轮数')
    parser.add_argument('--batch-size', type=int, default=16, help='批次大小')
    parser.add_argument('--no-freeze', action='store_true', help='不冻结骨干网络')

    args = parser.parse_args()

    trainer = IncrementalTrainer(
        data_dir=args.data_dir,
        model_path=args.model,
        output_dir=args.output,
        epochs=args.epochs,
        batch_size=args.batch_size,
        freeze_backbone=not args.no_freeze
    )

    success, model_path = trainer.train()

    if success:
        logger.info(f"训练成功，新模型: {model_path}")
        sys.exit(0)
    else:
        logger.error("训练失败")
        sys.exit(1)


if __name__ == '__main__':
    main()
