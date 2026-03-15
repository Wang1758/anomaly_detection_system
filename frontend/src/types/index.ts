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
  frame_id: number;
  image_url: string;
  detections: DetectionMeta[];
  timestamp: string;
}

export interface Sample {
  id: number;
  frame_id: number;
  image_path: string;
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
  filter_time_window: number;
  filter_iou: number;
  server_port: string;
  data_dir: string;
}

export type ViewType = 'monitor' | 'config' | 'stats' | 'library';
