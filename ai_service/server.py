"""
智慧养殖场监控系统 - AI 检测服务
gRPC 服务主入口
"""

import sys
import logging
from concurrent import futures
import grpc

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# 导入生成的 proto 代码
try:
    from pb import detection_pb2
    from pb import detection_pb2_grpc
except ImportError:
    logger.error("无法导入 proto 生成代码，请先运行 make proto-py")
    sys.exit(1)

from detector import YOLODetector


class DetectionServicer(detection_pb2_grpc.DetectionServiceServicer):
    """检测服务实现"""

    def __init__(self, detector: YOLODetector):
        """
        初始化检测服务
        
        Args:
            detector: YOLO 检测器实例
        """
        self.detector = detector
        logger.info("DetectionServicer 初始化完成")

    def Detect(self, request: detection_pb2.DetectRequest, context) -> detection_pb2.DetectResponse:
        """
        执行目标检测
        
        Args:
            request: 检测请求，包含图像数据和帧ID
            context: gRPC 上下文
            
        Returns:
            DetectResponse: 检测响应，包含检测结果列表
        """
        try:
            logger.debug(f"收到检测请求: frame_id={request.frame_id}, 图像大小={len(request.image_data)} bytes")
            
            # 执行检测
            results, inference_time = self.detector.detect(
                image_data=request.image_data,
                image_format=request.image_format or "jpeg"
            )
            
            # 构建响应
            response = detection_pb2.DetectResponse(
                frame_id=request.frame_id,
                inference_time_ms=int(inference_time * 1000)
            )
            
            # 添加检测结果
            for r in results:
                detection_result = detection_pb2.DetectionResult(
                    id=r['id'],
                    bbox=detection_pb2.BoundingBox(
                        x1=r['bbox'][0],
                        y1=r['bbox'][1],
                        x2=r['bbox'][2],
                        y2=r['bbox'][3]
                    ),
                    class_name=r['class_name'],
                    class_id=r['class_id'],
                    confidence=r['confidence'],
                    entropy=r['entropy'],
                    is_uncertain=r['is_uncertain']
                )
                response.results.append(detection_result)
            
            logger.debug(f"检测完成: frame_id={request.frame_id}, 检测到 {len(results)} 个目标")
            return response
            
        except Exception as e:
            logger.error(f"检测失败: {e}", exc_info=True)
            return detection_pb2.DetectResponse(
                frame_id=request.frame_id,
                error=str(e)
            )

    def ReloadModel(self, request: detection_pb2.ReloadModelRequest, context) -> detection_pb2.ReloadModelResponse:
        """
        重新加载模型
        
        Args:
            request: 重载请求
            context: gRPC 上下文
            
        Returns:
            ReloadModelResponse: 重载响应
        """
        try:
            model_path = request.model_path if request.model_path else None
            logger.info(f"收到模型重载请求: model_path={model_path}")
            
            # 执行热更新
            success, message, version = self.detector.reload_model(model_path)
            
            return detection_pb2.ReloadModelResponse(
                success=success,
                message=message,
                model_version=version
            )
            
        except Exception as e:
            logger.error(f"模型重载失败: {e}", exc_info=True)
            return detection_pb2.ReloadModelResponse(
                success=False,
                message=str(e),
                model_version=""
            )

    def UpdateParams(self, request: detection_pb2.UpdateParamsRequest, context) -> detection_pb2.UpdateParamsResponse:
        """
        更新检测参数
        
        Args:
            request: 参数更新请求
            context: gRPC 上下文
            
        Returns:
            UpdateParamsResponse: 更新响应
        """
        try:
            logger.info("收到参数更新请求")
            
            # 更新参数
            updates = {}
            if request.HasField('confidence_threshold'):
                updates['confidence_threshold'] = request.confidence_threshold
            if request.HasField('entropy_threshold'):
                updates['entropy_threshold'] = request.entropy_threshold
            if request.HasField('nms_iou_threshold'):
                updates['nms_iou_threshold'] = request.nms_iou_threshold
            if request.HasField('input_size'):
                updates['input_size'] = request.input_size
            
            success, message = self.detector.update_params(**updates)
            current_params = self.detector.get_params()
            
            return detection_pb2.UpdateParamsResponse(
                success=success,
                message=message,
                current=detection_pb2.CurrentParams(
                    confidence_threshold=current_params['confidence_threshold'],
                    entropy_threshold=current_params['entropy_threshold'],
                    nms_iou_threshold=current_params['nms_iou_threshold'],
                    input_size=current_params['input_size']
                )
            )
            
        except Exception as e:
            logger.error(f"参数更新失败: {e}", exc_info=True)
            return detection_pb2.UpdateParamsResponse(
                success=False,
                message=str(e)
            )


def serve(port: int = 50051, max_workers: int = 4, model_path: str = None):
    """
    启动 gRPC 服务器
    
    Args:
        port: 服务端口
        max_workers: 线程池最大工作线程数
        model_path: 模型权重文件路径
    """
    # 初始化检测器
    logger.info("正在初始化 YOLO 检测器...")
    detector = YOLODetector(model_path=model_path)
    
    # 创建 gRPC 服务器
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=max_workers),
        options=[
            ('grpc.max_send_message_length', 100 * 1024 * 1024),  # 100MB
            ('grpc.max_receive_message_length', 100 * 1024 * 1024),  # 100MB
        ]
    )
    
    # 注册服务
    detection_pb2_grpc.add_DetectionServiceServicer_to_server(
        DetectionServicer(detector), server
    )
    
    # 绑定端口
    server.add_insecure_port(f'[::]:{port}')
    
    # 启动服务
    server.start()
    logger.info(f"gRPC 服务已启动，监听端口: {port}")
    logger.info(f"线程池大小: {max_workers}")
    
    # 等待终止
    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        logger.info("收到终止信号，正在关闭服务器...")
        server.stop(grace=5)
        logger.info("服务器已关闭")


if __name__ == '__main__':
    import argparse
    
    parser = argparse.ArgumentParser(description='AI 检测服务')
    parser.add_argument('--port', type=int, default=50051, help='服务端口 (默认: 50051)')
    parser.add_argument('--workers', type=int, default=4, help='线程池大小 (默认: 4)')
    parser.add_argument('--model', type=str, default=None, help='模型权重路径')
    
    args = parser.parse_args()
    
    serve(
        port=args.port,
        max_workers=args.workers,
        model_path=args.model
    )
