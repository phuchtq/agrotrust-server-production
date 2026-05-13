package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GroqClient struct{}

const (
	groqAPIURL string = "https://api.groq.com/openai/v1/chat/completions"
	model      string = "meta-llama/llama-4-scout-17b-16e-instruct"
)

type AiResponse struct {
	Result string `json:"result"`
	Reason string `json:"reason"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type Message struct {
	Role    string        `json:"role"`
	Content []ContentPart `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Choice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type ChatResponse struct {
	Choices Choice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// AskWithImages gửi prompt kèm nhiều ảnh (Cloudinary URL) tới Groq vision model.
// Dùng cho các tác vụ cần phân tích nhiều tài liệu cùng lúc (vd: giấy khai sinh + CCCD).
func (g *GroqClient) AskWithImages(apiKey, prompt string, imageURLs []string) (string, error) {
	parts := []ContentPart{{Type: "text", Text: prompt}}
	for _, url := range imageURLs {
		parts = append(parts, ContentPart{
			Type:     "image_url",
			ImageURL: &ImageURL{URL: url},
		})
	}

	reqBody := ChatRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: parts},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request thất bại: %w", err)
	}

	req, err := http.NewRequest("POST", groqAPIURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("tạo HTTP request thất bại: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gọi API thất bại: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("đọc response thất bại: %w", err)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return "", fmt.Errorf("parse response thất bại: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("Groq API lỗi: %s", chatResp.Error.Message)
	}

	if chatResp.Choices.Message.Content == "" {
		return "", fmt.Errorf("không có kết quả trả về")
	}

	return chatResp.Choices.Message.Content, nil
}
