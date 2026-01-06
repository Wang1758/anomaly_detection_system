"""
智慧养殖场监控系统 - YOLO 检测器
包含 YOLOv11 推理、NMS 处理和不确定性计算
"""

import io
import time
import logging
import threading
from typing import List, Dict, Tuple, Optional
import numpy as np
from PIL import Image

logger = logging.getLogger(__name__)


class YOLODetector:
    """YOLOv11 目标检测器，支持热更新和参数动态调整"""

    def __init__(
        self,
        model_path: Optional[str] = None,
        confidence_threshold: float = 0.5,
        entropy_threshold: float = 0.5,
        nms_iou_threshold: float = 0.8,
        input_size: int = 640
    ):
        """
        初始化检测器
        
        Args:
            model_path: 模型权重文件路径，为空则使用预训练模型
            confidence_threshold: 置信度阈值
            entropy_threshold: 熵值阈值（用于判断不确定性）
            nms_iou_threshold: NMS IoU 阈值
            input_size: 输入图像尺寸
        """
        self._lock = threading.RLock()
        
        # 参数配置
        self._confidence_threshold = confidence_threshold
        self._entropy_threshold = entropy_threshold
        self._nms_iou_threshold = nms_iou_threshold
        self._input_size = input_size
        
        # 模型相关
        self._model = None
        self._model_path = model_path
        self._model_version = "v1.0.0"
        self._backup_model = None  # 用于热更新的备份模型
        
        # 检测计数器
        self._detection_id = 0
        
        # 加载模型
        self._load_model(model_path)
        
        logger.info(f"YOLODetector 初始化完成")
        logger.info(f"  - 置信度阈值: {confidence_threshold}")
        logger.info(f"  - 熵值阈值: {entropy_threshold}")
        logger.info(f"  - NMS IoU 阈值: {nms_iou_threshold}")
        logger.info(f"  - 输入尺寸: {input_size}")

    def _load_model(self, model_path: Optional[str] = None):
        """
        加载 YOLO 模型
        
        Args:
            model_path: 模型权重路径
        """
        try:
            from ultralytics import YOLO
            
            if model_path:
                logger.info(f"正在加载模型: {model_path}")
                self._model = YOLO(model_path)
            else:
                # 使用预训练模型
                logger.info("正在加载 YOLOv11n 预训练模型...")
                self._model = YOLO('yolo11n.pt')
            
            self._model_path = model_path
            logger.info("模型加载完成")
            
        except Exception as e:
            logger.error(f"模型加载失败: {e}")
            raise

    def detect(self, image_data: bytes, image_format: str = "jpeg") -> Tuple[List[Dict], float]:
        """
        执行目标检测
        
        Args:
            image_data: 图像字节数据
            image_format: 图像格式 (jpeg/png)
            
        Returns:
            Tuple[List[Dict], float]: (检测结果列表, 推理耗时)
        """
        start_time = time.time()
        
        with self._lock:
            # 解码图像
            image = self._decode_image(image_data)
            
            # 执行推理
            results = self._model.predict(
                image,
                conf=self._confidence_threshold,
                iou=self._nms_iou_threshold,
                imgsz=self._input_size,
                verbose=False
            )
            
            # 解析结果
            detections = self._parse_results(results[0])
            
            # 执行严格 NMS（合并高度重叠的同类框）
            detections = self._strict_nms(detections)
            
            # 计算不确定性
            detections = self._compute_uncertainty(detections)
            
        inference_time = time.time() - start_time
        return detections, inference_time

    def _decode_image(self, image_data: bytes) -> np.ndarray:
        """
        解码图像数据
        
        Args:
            image_data: 图像字节数据
            
        Returns:
            np.ndarray: OpenCV 格式图像
        """
        image = Image.open(io.BytesIO(image_data))
        # 转换为 RGB（YOLO 需要 RGB 格式）
        if image.mode != 'RGB':
            image = image.convert('RGB')
        return np.array(image)

    def _parse_results(self, result) -> List[Dict]:
        """
        解析 YOLO 检测结果
        
        Args:
            result: YOLO 单帧检测结果
            
        Returns:
            List[Dict]: 解析后的检测结果列表
        """
        detections = []
        
        if result.boxes is None or len(result.boxes) == 0:
            return detections
        
        boxes = result.boxes
        
        for i in range(len(boxes)):
            self._detection_id += 1
            
            # 获取边界框坐标 (xyxy 格式)
            bbox = boxes.xyxy[i].cpu().numpy().tolist()
            
            # 获取类别和置信度
            cls_id = int(boxes.cls[i].cpu().numpy())
            conf = float(boxes.conf[i].cpu().numpy())
            cls_name = result.names[cls_id]
            
            detections.append({
                'id': self._detection_id,
                'bbox': bbox,  # [x1, y1, x2, y2]
                'class_id': cls_id,
                'class_name': cls_name,
                'confidence': conf,
                'entropy': 0.0,  # 稍后计算
                'is_uncertain': False
            })
        
        return detections

    def _strict_nms(self, detections: List[Dict]) -> List[Dict]:
        """
        执行严格的非极大值抑制
        如果两个同类框 IoU > nms_iou_threshold，合并为一个框
        
        Args:
            detections: 检测结果列表
            
        Returns:
            List[Dict]: NMS 后的结果
        """
        if len(detections) <= 1:
            return detections
        
        # 按置信度排序
        detections = sorted(detections, key=lambda x: x['confidence'], reverse=True)
        
        keep = []
        used = [False] * len(detections)
        
        for i in range(len(detections)):
            if used[i]:
                continue
            
            current = detections[i]
            merge_list = [current]
            used[i] = True
            
            for j in range(i + 1, len(detections)):
                if used[j]:
                    continue
                
                other = detections[j]
                
                # 只合并同类框
                if current['class_id'] != other['class_id']:
                    continue
                
                iou = self._calculate_iou(current['bbox'], other['bbox'])
                
                if iou > self._nms_iou_threshold:
                    merge_list.append(other)
                    used[j] = True
            
            # 合并框（取最大边界）
            if len(merge_list) > 1:
                merged = self._merge_boxes(merge_list)
                keep.append(merged)
            else:
                keep.append(current)
        
        return keep

    def _merge_boxes(self, boxes: List[Dict]) -> Dict:
        """
        合并多个检测框
        
        Args:
            boxes: 待合并的检测框列表
            
        Returns:
            Dict: 合并后的检测框
        """
        # 取最大边界
        x1 = min(b['bbox'][0] for b in boxes)
        y1 = min(b['bbox'][1] for b in boxes)
        x2 = max(b['bbox'][2] for b in boxes)
        y2 = max(b['bbox'][3] for b in boxes)
        
        # 取最高置信度
        max_conf_box = max(boxes, key=lambda x: x['confidence'])
        
        return {
            'id': max_conf_box['id'],
            'bbox': [x1, y1, x2, y2],
            'class_id': max_conf_box['class_id'],
            'class_name': max_conf_box['class_name'],
            'confidence': max_conf_box['confidence'],
            'entropy': 0.0,
            'is_uncertain': False
        }

    def _calculate_iou(self, box1: List[float], box2: List[float]) -> float:
        """
        计算两个框的 IoU
        
        Args:
            box1: [x1, y1, x2, y2]
            box2: [x1, y1, x2, y2]
            
        Returns:
            float: IoU 值
        """
        x1 = max(box1[0], box2[0])
        y1 = max(box1[1], box2[1])
        x2 = min(box1[2], box2[2])
        y2 = min(box1[3], box2[3])
        
        if x2 <= x1 or y2 <= y1:
            return 0.0
        
        intersection = (x2 - x1) * (y2 - y1)
        area1 = (box1[2] - box1[0]) * (box1[3] - box1[1])
        area2 = (box2[2] - box2[0]) * (box2[3] - box2[1])
        union = area1 + area2 - intersection
        
        return intersection / union if union > 0 else 0.0

    def _compute_uncertainty(self, detections: List[Dict]) -> List[Dict]:
        """
        计算检测框的不确定性
        使用混合加权公式: Score = w1 * (1 - Conf) + w2 * IoU_With_Other_Boxes
        
        Args:
            detections: 检测结果列表
            
        Returns:
            List[Dict]: 添加不确定性信息后的结果
        """
        if not detections:
            return detections
        
        w1 = 0.7  # 置信度权重
        w2 = 0.3  # IoU 权重
        
        for i, det in enumerate(detections):
            # 计算与其他框的最大 IoU
            max_iou = 0.0
            for j, other in enumerate(detections):
                if i != j:
                    iou = self._calculate_iou(det['bbox'], other['bbox'])
                    max_iou = max(max_iou, iou)
            
            # 计算不确定性得分
            # 置信度越低，不确定性越高
            # 与其他框 IoU 越高（边界模糊），不确定性越高
            uncertainty_score = w1 * (1 - det['confidence']) + w2 * max_iou
            
            # 使用熵值表示不确定性得分
            det['entropy'] = uncertainty_score
            
            # 判断是否为不确定目标
            det['is_uncertain'] = uncertainty_score > self._entropy_threshold
        
        return detections

    def reload_model(self, model_path: Optional[str] = None) -> Tuple[bool, str, str]:
        """
        热更新模型（双缓冲机制）
        
        Args:
            model_path: 新模型路径
            
        Returns:
            Tuple[bool, str, str]: (是否成功, 消息, 当前模型版本)
        """
        try:
            from ultralytics import YOLO
            
            path = model_path or self._model_path
            if not path:
                return False, "未指定模型路径", self._model_version
            
            logger.info(f"开始热更新模型: {path}")
            
            # 在后台加载新模型
            new_model = YOLO(path)
            
            # 原子切换
            with self._lock:
                self._backup_model = self._model
                self._model = new_model
                self._model_path = path
                
                # 更新版本号
                version_parts = self._model_version.split('.')
                version_parts[-1] = str(int(version_parts[-1]) + 1)
                self._model_version = '.'.join(version_parts)
            
            # 释放旧模型
            self._backup_model = None
            
            logger.info(f"模型热更新成功，新版本: {self._model_version}")
            return True, "模型热更新成功", self._model_version
            
        except Exception as e:
            logger.error(f"模型热更新失败: {e}")
            return False, str(e), self._model_version

    def update_params(
        self,
        confidence_threshold: Optional[float] = None,
        entropy_threshold: Optional[float] = None,
        nms_iou_threshold: Optional[float] = None,
        input_size: Optional[int] = None
    ) -> Tuple[bool, str]:
        """
        更新检测参数
        
        Args:
            confidence_threshold: 置信度阈值
            entropy_threshold: 熵值阈值
            nms_iou_threshold: NMS IoU 阈值
            input_size: 输入图像尺寸
            
        Returns:
            Tuple[bool, str]: (是否成功, 消息)
        """
        try:
            with self._lock:
                if confidence_threshold is not None:
                    if not 0.0 <= confidence_threshold <= 1.0:
                        return False, "置信度阈值必须在 0.0 到 1.0 之间"
                    self._confidence_threshold = confidence_threshold
                    logger.info(f"置信度阈值更新为: {confidence_threshold}")
                
                if entropy_threshold is not None:
                    if not 0.0 <= entropy_threshold <= 1.0:
                        return False, "熵值阈值必须在 0.0 到 1.0 之间"
                    self._entropy_threshold = entropy_threshold
                    logger.info(f"熵值阈值更新为: {entropy_threshold}")
                
                if nms_iou_threshold is not None:
                    if not 0.5 <= nms_iou_threshold <= 1.0:
                        return False, "NMS IoU 阈值必须在 0.5 到 1.0 之间"
                    self._nms_iou_threshold = nms_iou_threshold
                    logger.info(f"NMS IoU 阈值更新为: {nms_iou_threshold}")
                
                if input_size is not None:
                    if input_size not in [320, 640, 1280]:
                        return False, "输入尺寸必须是 320, 640 或 1280"
                    self._input_size = input_size
                    logger.info(f"输入尺寸更新为: {input_size}")
            
            return True, "参数更新成功"
            
        except Exception as e:
            logger.error(f"参数更新失败: {e}")
            return False, str(e)

    def get_params(self) -> Dict:
        """
        获取当前参数
        
        Returns:
            Dict: 当前参数字典
        """
        with self._lock:
            return {
                'confidence_threshold': self._confidence_threshold,
                'entropy_threshold': self._entropy_threshold,
                'nms_iou_threshold': self._nms_iou_threshold,
                'input_size': self._input_size
            }

    def get_model_info(self) -> Dict:
        """
        获取模型信息
        
        Returns:
            Dict: 模型信息字典
        """
        with self._lock:
            return {
                'model_path': self._model_path,
                'model_version': self._model_version,
                'input_size': self._input_size
            }
