package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"raise-child/model/dtos/response"
)

type aiClient struct {
	geminiProvider *geminiProvider
	errLogger      *log.Logger
}

var _aiClient *aiClient

type ImageValidateCase string

type IAiClientProvider interface {
	ExtractChildInfo(birthCertB64 string, guardianIdB64 string) (*response.ExtractChildInfoResponse, error)
	ValidateTaskProof(req ValidateTaskProof, ctx context.Context) string
	ValidateWithdrawProposal(req ValidateWithdrawProposal, ctx context.Context) string
	ValidateSpecialNeedProposal(req ValidateChildSpecialNeedProposal, ctx context.Context) string

	// ValidateUploadChildRequest(req ValidateUploadChildRequest, ctx context.Context) string
	// ValidateCreateCenterRequest(req ValidateCreateCenterRequest, ctx context.Context) string
	// ValidateRegistrationRequest(req ValidateRegistrationRequest, ctx context.Context) string
	// ValidateProvideNeedForChildTaskProof(req ValidateProvideNeedForChildTaskProof, ctx context.Context) string
	// ValidatePoolCampaign(req ValidatePoolCampaign, ctx context.Context) string
}

func InitializeAiProvider(ctx context.Context, errLogger *log.Logger) IAiClientProvider {
	if _aiClient == nil {
		if ctx == nil {
			ctx = context.Background()
		}

		_aiClient = &aiClient{
			geminiProvider: initializeGeminiClient(ctx, errLogger),
			errLogger:      errLogger,
		}
	}

	return _aiClient
}

func (a *aiClient) ExtractChildInfo(birthCertB64 string, guardianIdB64 string) (*response.ExtractChildInfoResponse, error) {
	prompt := "You are a specialized OCR data extractor. Extract information from the provided images. " +
		"Image 1 is a child's birth certificate. Image 2 is the guardian's identity card. " +
		"Return ONLY a valid JSON object with the following keys: " +
		"region, first_name, last_name, gender, date_of_birth, home_address, guardian_full_name. " +
		"Do not use markdown blocks or extra text."

	var contentParts = []ContentPart{
		{Type: "text", Text: prompt},
		{Type: "image_url", ImageURL: &ImageURL{URL: birthCertB64}},
		{Type: "image_url", ImageURL: &ImageURL{URL: guardianIdB64}},
	}

	var reqBody ChatRequest = ChatRequest{
		Model: model, // Your configured text/vision model
		Messages: []Message{
			{Role: "user", Content: contentParts},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", groqAPIURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("GROQ_API_KEY"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, err
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("groq error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	result := chatResp.Choices[0].Message.Content

	// Parse result into struct
	var extractResp response.ExtractChildInfoResponse
	if err := json.Unmarshal([]byte(result), &extractResp); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return &extractResp, nil
}

// ValidateTaskProof implements IAiClientProvider.
func (a *aiClient) ValidateTaskProof(req ValidateTaskProof, ctx context.Context) string {
	return ""
}

// ValidateChildSpecialNeedProposal implements IAiClientProvider.
func (a *aiClient) ValidateChildSpecialNeedProposal(req ValidateChildSpecialNeedProposal, ctx context.Context) string {
	return ""
}

// ValidateWithdrawProposal implements IAiClientProvider.
func (a *aiClient) ValidateWithdrawProposal(req ValidateWithdrawProposal, ctx context.Context) string {
	return ""
}

func (a *aiClient) ValidateSpecialNeedProposal(req ValidateChildSpecialNeedProposal, ctx context.Context) string {
	return ""
}
