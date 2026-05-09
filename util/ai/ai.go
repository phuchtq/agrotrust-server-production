package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	groqAPIURL = "https://api.groq.com/openai/v1/chat/completions"
	model      = "meta-llama/llama-4-scout-17b-16e-instruct"
)

// ── Request structs ──────────────────────────────────────────────

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

// ── Response structs ─────────────────────────────────────────────

type Choice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ── Helper: encode ảnh local sang base64 data URL ────────────────

func encodeImageToBase64(imagePath string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("đọc file ảnh thất bại: %w", err)
	}

	// Detect MIME type đơn giản theo extension
	mime := "image/jpeg"
	if len(imagePath) > 4 {
		switch imagePath[len(imagePath)-4:] {
		case ".png":
			mime = "image/png"
		case ".gif":
			mime = "image/gif"
		case "webp":
			mime = "image/webp"
		}
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

// ── Core: gọi Groq vision API ────────────────────────────────────

func askGroqWithImage(apiKey, prompt, imageDataURL string) (string, error) {
	reqBody := ChatRequest{
		Model: model,
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentPart{
					{
						Type: "text",
						Text: prompt,
					},
					{
						Type:     "image_url",
						ImageURL: &ImageURL{URL: imageDataURL},
					},
				},
			},
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

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("không có kết quả trả về")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// ── Main ─────────────────────────────────────────────────────────

func main() {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "❌ Thiếu GROQ_API_KEY — set biến môi trường trước:\n   export GROQ_API_KEY=your_key_here")
		os.Exit(1)
	}

	// ── Ví dụ 1: Dùng ảnh local ──────────────────────────────────
	fmt.Println("=== Ví dụ 1: Ảnh local ===")
	imagePath := "photo.jpg" // đổi thành đường dẫn ảnh của bạn
	dataURL, err := encodeImageToBase64(imagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  %v\n", err)
		fmt.Println("→ Bỏ qua ví dụ 1, chạy ví dụ 2 với ảnh URL...")
	} else {
		answer, err := askGroqWithImage(apiKey, "Mô tả chi tiết những gì bạn thấy trong ảnh này bằng tiếng Việt.", dataURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Lỗi: %v\n", err)
		} else {
			fmt.Println("Kết quả:", answer)
		}
	}

	// ── Ví dụ 2: Dùng ảnh qua URL công khai ──────────────────────
	fmt.Println("\n=== Ví dụ 2: Ảnh từ URL ===")
	imageURL := "https://upload.wikimedia.org/wikipedia/commons/thumb/b/b6/Felis_catus-cat_on_snow.jpg/1024px-Felis_catus-cat_on_snow.jpg"
	answer, err := askGroqWithImage(apiKey, "Con vật trong ảnh là gì? Mô tả bằng tiếng Việt.", imageURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Lỗi: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Kết quả:", answer)
}
