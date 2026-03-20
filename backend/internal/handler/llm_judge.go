package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const defaultSystemPrompt = `你是一个智能视频监控系统的AI研判专家。系统已通过目标检测模型（YOLO）初步筛选出一批疑似异常的监控截图，需要你进行二次研判。

你的任务：
1. 仔细观察图片内容
2. 判断图片中是否存在真实的异常情况（如：异常行为、安全隐患、异常目标、入侵等）
3. 区分真实异常和检测模型的误报

请严格按照以下JSON格式回复，不要包含任何其他文字：
{"label": true, "reason": "简要说明你的判断依据（一句话）"}

其中：
- label 为 true 表示确认存在异常，需要人工关注
- label 为 false 表示图片正常或属于误报`

const defaultUserPrompt = "请分析这张监控截图并判断是否存在异常。"

// LLMJudger calls an OpenAI-compatible multimodal API to judge images.
type LLMJudger struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func NewLLMJudger(apiKey, baseURL, model string) *LLMJudger {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o"
	}
	return &LLMJudger{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Available reports whether an API key has been configured.
func (j *LLMJudger) Available() bool {
	return j.apiKey != ""
}

// LLMJudgeResult is the structured output from the multimodal LLM.
type LLMJudgeResult struct {
	Label  bool   `json:"label"`
	Reason string `json:"reason"`
}

// --- OpenAI-compatible request/response types ---

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []contentPart
}

type contentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *imageURLDetail `json:"image_url,omitempty"`
}

type imageURLDetail struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *chatError   `json:"error,omitempty"`
}

type chatChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type chatError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// JudgeImage sends a single image to the multimodal LLM and returns a label decision.
func (j *LLMJudger) JudgeImage(ctx context.Context, imageBytes []byte) (*LLMJudgeResult, error) {
	b64 := base64.StdEncoding.EncodeToString(imageBytes)
	dataURL := "data:image/jpeg;base64," + b64

	reqBody := chatRequest{
		Model: j.model,
		Messages: []chatMessage{
			{Role: "system", Content: defaultSystemPrompt},
			{Role: "user", Content: []contentPart{
				{Type: "text", Text: defaultUserPrompt},
				{Type: "image_url", ImageURL: &imageURLDetail{URL: dataURL, Detail: "auto"}},
			}},
		},
		MaxTokens:   300,
		Temperature: 0.1,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := j.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+j.apiKey)

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API returned %d: %s", resp.StatusCode, truncate(string(respBytes), 200))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if chatResp.Error != nil {
		return nil, fmt.Errorf("LLM error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	content := chatResp.Choices[0].Message.Content
	result, err := parseLLMJudgeResponse(content)
	if err != nil {
		log.Printf("LLM response parse failed (raw=%q): %v", truncate(content, 200), err)
		return nil, err
	}
	return result, nil
}

// parseLLMJudgeResponse extracts JSON from potentially markdown-wrapped LLM output.
func parseLLMJudgeResponse(raw string) (*LLMJudgeResult, error) {
	content := strings.TrimSpace(raw)

	// Strip markdown code fences: ```json ... ```
	if idx := strings.Index(content, "```"); idx >= 0 {
		after := content[idx+3:]
		if nl := strings.Index(after, "\n"); nl >= 0 {
			after = after[nl+1:]
		}
		if end := strings.Index(after, "```"); end >= 0 {
			content = strings.TrimSpace(after[:end])
		}
	}

	// Find the outermost JSON object
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}

	var result LLMJudgeResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse LLM JSON: %w (content=%s)", err, truncate(content, 120))
	}
	return &result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
