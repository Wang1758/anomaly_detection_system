package handler

import pb "anomaly_detection_system/backend/internal/grpcclient/pb"

// inferLabelFromDetectResponse maps YOLO+不确定性输出到人工标注语义：
// - label=true：确认为异常（与「待处理」队列中勾选 ✓ 一致，模型仍认为存在可疑/不确定目标）
// - label=false：判为误报（与 ✗ 一致，重判后无目标或模型对已检出目标有把握）
func inferLabelFromDetectResponse(resp *pb.DetectResponse) (label bool, reason string) {
	if resp == nil {
		return false, "empty_response"
	}
	dets := resp.GetDetections()
	if len(dets) == 0 {
		return false, "no_detection"
	}
	for _, d := range dets {
		if d.GetIsUncertain() {
			return true, "uncertain_detection"
		}
	}
	return false, "confident_detections"
}
