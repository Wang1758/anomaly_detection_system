package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"anomaly_detection_system/backend/internal/config"
	"anomaly_detection_system/backend/internal/db"
	"anomaly_detection_system/backend/internal/models"

	"github.com/gin-gonic/gin"
)

type remoteEvalRequest struct {
	DatasetDir string `json:"dataset_dir,omitempty"`
	Model      string `json:"model,omitempty"`
	MetricsOut string `json:"metrics_out,omitempty"`
	Device     string `json:"device,omitempty"`
	Imgsz      int    `json:"imgsz,omitempty"`
}

type remoteEvalResponse struct {
	OK      bool     `json:"ok"`
	Map50   float64  `json:"map50"`
	Map5095 float64  `json:"map50_95"`
	Error   string   `json:"error"`
	Logs    []string `json:"logs"`
	Dataset string   `json:"dataset"`
	Model   string   `json:"model"`
	Device  string   `json:"device"`
}

func (h *APIHandler) StartMapEvalScheduler() {
	go func() {
		for {
			snap := h.cfg.Read()
			interval := snap.MapEvalIntervalHours
			if interval <= 0 {
				interval = 24
			}
			time.Sleep(time.Duration(interval) * time.Hour)
			h.runMapEvalOnce()
		}
	}()
}

func (h *APIHandler) runMapEvalOnce() {
	if !h.startEvalRun() {
		return
	}
	defer h.finishEvalRun()
	h.runMapEvalCore()
}

func (h *APIHandler) runMapEvalCore() {
	snap := h.cfg.Read()
	endpoint := resolveMapEvalEndpoint(snap)
	if endpoint == "" {
		msg := "map eval endpoint is empty"
		h.persistEvalRun("failed", 0, 0, msg)
		h.appendEvalLog("[failed] " + msg)
		return
	}

	request := remoteEvalRequest{
		DatasetDir: strings.TrimSpace(os.Getenv("MAP_EVAL_REMOTE_DATASET_DIR")),
		Model:      strings.TrimSpace(os.Getenv("MAP_EVAL_MODEL_PATH")),
		MetricsOut: strings.TrimSpace(os.Getenv("MAP_EVAL_REMOTE_METRICS_OUT")),
		Device:     resolveEvalDeviceOverride(),
	}
	if request.DatasetDir == "" {
		request.DatasetDir = strings.TrimSpace(snap.MapEvalDatasetDir)
	}
	if v := strings.TrimSpace(os.Getenv("TRAINING_IMGSZ")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			request.Imgsz = n
		}
	}

	h.appendEvalLog(fmt.Sprintf("[start] remote map eval endpoint=%s", endpoint))
	h.appendEvalLog(fmt.Sprintf("[config] dataset=%s model=%s device=%s", displayOrAuto(request.DatasetDir), displayOrAuto(request.Model), displayOrAuto(request.Device)))

	resp, err := invokeRemoteMapEval(endpoint, request)
	if err != nil {
		h.persistEvalRun("failed", 0, 0, err.Error())
		h.appendEvalLog("[failed] " + err.Error())
		return
	}

	for _, line := range resp.Logs {
		h.appendEvalLog("[remote] " + line)
	}

	if !resp.OK {
		msg := strings.TrimSpace(resp.Error)
		if msg == "" {
			msg = "remote map eval failed"
		}
		h.persistEvalRun("failed", 0, 0, msg)
		h.appendEvalLog("[failed] " + msg)
		return
	}

	h.persistEvalRun("success", resp.Map50, resp.Map5095, "")
	h.appendEvalLog(fmt.Sprintf("[done] map50=%.4f map50_95=%.4f", resp.Map50, resp.Map5095))
}

func resolveMapEvalEndpoint(snap *config.Config) string {
	if v := strings.TrimSpace(os.Getenv("MAP_EVAL_REMOTE_URL")); v != "" {
		return v
	}
	if strings.TrimSpace(snap.MapEvalRemoteURL) != "" {
		return strings.TrimSpace(snap.MapEvalRemoteURL)
	}
	host := strings.TrimSpace(snap.AIServiceAddr)
	if host == "" {
		return ""
	}
	parts := strings.Split(host, ":")
	if len(parts) >= 2 {
		host = strings.Join(parts[:len(parts)-1], ":")
	}
	if host == "" {
		return ""
	}
	return "http://" + host + ":50052/eval-map"
}

func resolveEvalDeviceOverride() string {
	if v := strings.TrimSpace(os.Getenv("MAP_EVAL_DEVICE")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("TRAINING_DEVICE"))
}

func displayOrAuto(v string) string {
	if strings.TrimSpace(v) == "" {
		return "auto"
	}
	return v
}

func invokeRemoteMapEval(endpoint string, request remoteEvalRequest) (*remoteEvalResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{Timeout: 2 * time.Hour}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var resp remoteEvalResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("invalid remote eval response: %w", err)
	}
	if res.StatusCode >= 400 {
		msg := strings.TrimSpace(resp.Error)
		if msg == "" {
			msg = fmt.Sprintf("remote eval http %d", res.StatusCode)
		}
		return &resp, fmt.Errorf(msg)
	}
	return &resp, nil
}

func (h *APIHandler) TriggerMapEvalNow(c *gin.Context) {
	if !h.startEvalRun() {
		c.JSON(409, gin.H{"error": "mAP evaluation already running"})
		return
	}

	go func() {
		defer h.finishEvalRun()
		h.runMapEvalCore()
	}()

	c.JSON(202, gin.H{"message": "mAP evaluation triggered"})
}

func (h *APIHandler) MapEvalLogs(c *gin.Context) {
	h.evalLogMu.Lock()
	logs := make([]string, len(h.evalLogs))
	copy(logs, h.evalLogs)
	h.evalLogMu.Unlock()

	h.evalMu.Lock()
	running := h.evalRunning
	h.evalMu.Unlock()

	c.JSON(200, gin.H{"running": running, "logs": logs})
}

func (h *APIHandler) appendEvalLog(line string) {
	line = sanitizeEvalLogLine(line)
	if line == "" {
		return
	}
	entry := time.Now().Format("2006-01-02 15:04:05") + " " + line
	log.Printf("mAP eval %s", line)
	h.evalLogMu.Lock()
	h.evalLogs = append(h.evalLogs, entry)
	if len(h.evalLogs) > 400 {
		h.evalLogs = h.evalLogs[len(h.evalLogs)-400:]
	}
	h.evalLogMu.Unlock()
}

func sanitizeEvalLogLine(line string) string {
	line = strings.ReplaceAll(line, "\r", "")
	line = strings.ReplaceAll(line, "\u0000", "")
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	var filtered []rune
	for _, r := range line {
		if r == '\n' || r == '\t' || unicode.IsPrint(r) {
			filtered = append(filtered, r)
		}
	}
	return strings.TrimSpace(string(filtered))
}

func (h *APIHandler) startEvalRun() bool {
	h.evalMu.Lock()
	defer h.evalMu.Unlock()
	if h.evalRunning {
		return false
	}
	h.evalRunning = true
	return true
}

func (h *APIHandler) finishEvalRun() {
	h.evalMu.Lock()
	h.evalRunning = false
	h.evalMu.Unlock()
}

func (h *APIHandler) persistEvalRun(status string, map50, map5095 float64, message string) {
	run := models.EvalRun{
		Status:  status,
		Map50:   map50,
		Map5095: map5095,
		Message: message,
	}
	if err := db.DB.Create(&run).Error; err != nil {
		log.Printf("failed to persist eval run: %v", err)
	}
}

func (h *APIHandler) MapHistory(c *gin.Context) {
	var runs []models.EvalRun
	if err := db.DB.Order("created_at DESC").Limit(500).Find(&runs).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, runs)
}
