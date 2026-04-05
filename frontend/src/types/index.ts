export interface DetectionMeta {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  confidence: number;
  class_id: number;
  class_name: string;
  is_uncertain: boolean;
  entropy: number;
  anomaly_score: number;
}

export interface AlertEvent {
  type: 'alert';
  sample_id?: number;
  run_id?: number;
  frame_id: number;
  image_url: string;
  fallback_image_url?: string;
  detections: DetectionMeta[];
  uncertain_count?: number;
  timestamp: string;
}

export interface DetectionFrame {
  type: 'detections';
  frame_id: number;
  frame_width: number;
  frame_height: number;
  detections: DetectionMeta[];
}

export interface LiveFrameMeta {
  type: 'live_frame';
  frame_id: number;
  frame_width: number;
  frame_height: number;
  detections: DetectionMeta[];
}

export interface Sample {
  id: number;
  frame_id: number;
  image_path: string;
  visualized_image_path?: string;
  uncertain_count?: number;
  status: 'pending' | 'labeled' | 'trained';
  label: boolean | null;
  source: string;
  created_at: string;
  updated_at: string;
}

export interface TrainingRun {
  id: number;
  sample_count: number;
  accuracy: number;
  model_path: string;
  created_at: string;
}

export interface SystemConfig {
  ai_service_addr: string;
  nms_threshold: number;
  confidence_threshold: number;
  entropy_threshold: number;
  w1: number;
  w2: number;
  source_type: 'rtsp' | 'local';
  source_addr: string;
  fps: number;
  workers: number;
  batch_size: number;
  batch_timeout_ms: number;
  filter_time_window: number;
  filter_iou: number;
  server_port: string;
  data_dir: string;
  map_eval_interval_hours: number;
  map_eval_dataset_dir: string;
}

export interface MapEvalRun {
  id: number;
  status: 'success' | 'failed';
  map50: number;
  map50_95: number;
  message: string;
  created_at: string;
}

export type ViewType = 'monitor' | 'config' | 'stats' | 'library';
