// 视频配置
export interface VideoConfig {
  source_type: 'rtsp' | 'local'
  rtsp_url: string
  local_path: string
  fps: 30 | 60
}

// AI 配置
export interface AIConfig {
  confidence_threshold: number
  entropy_threshold: number
  nms_iou_threshold: number
  input_size: number
}

// 过滤器配置
export interface FilterConfig {
  spatial_iou_threshold: number
  time_window_seconds: number
  enable_alert_push: boolean
  auto_save_sample: boolean
}

// 训练配置
export interface TrainingConfig {
  trigger_threshold: number
}

// 帧数据
export interface FrameData {
  frameId: number
  imageData: string  // Base64 编码
  width: number
  height: number
  inferenceTime: number
}

// 检测结果
export interface Detection {
  id: number
  x1: number
  y1: number
  x2: number
  y2: number
  class_name: string
  class_id: number
  confidence: number
  entropy: number
  is_uncertain: boolean
}

// 边界框
export interface BoundingBox {
  x1: number
  y1: number
  x2: number
  y2: number
}

// 报警
export interface Alert {
  id: number
  frameId: number
  timestamp: number
  imageData: string
  bbox: BoundingBox
  className: string
  confidence: number
  entropy: number
  countdown: number
}

// 训练状态
export interface TrainingStatus {
  labeled_samples_count: number
  trigger_threshold: number
  can_train: boolean
  is_training: boolean
  latest_training?: {
    id: number
    status: 'running' | 'completed' | 'failed'
    start_time: string
    end_time?: string
    error_message?: string
  }
}

// 系统配置
export interface SystemConfig {
  video: VideoConfig
  ai: AIConfig
  filter: FilterConfig
  training: TrainingConfig
}
