package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"raise-child/model/dtos/response"
	"raise-child/util/ai/prompts"
	"strings"
	"time"
)

type ValidateTaskProof struct {
	TaskDescription string
	ProofImageURL   string
	CreatedAt       time.Time
}

type ValidateTaskProofResponse struct {
	AIEvaluation string `json:"ai_evaluation"`
	AIReason     string `json:"ai_reason"`
}

type aiClient struct {
	groqProvider *GroqClient
	errLogger    *log.Logger
}

var _aiClient *aiClient

type IAiClientProvider interface {
	ExtractChildInfo(birthCertURL string, firstGuardianIDCardURL string, secondGuardianIDCardURL *string, ctx context.Context) (*response.ExtractChildUploadInfoResponse, error)
	ValidateTaskProof(proof ValidateTaskProof, ctx context.Context) (*ValidateTaskProofResponse, error)
}

func InitializeAiProvider(errLogger *log.Logger) IAiClientProvider {
	if _aiClient == nil {
		_aiClient = &aiClient{
			groqProvider: &GroqClient{},
			errLogger:    errLogger,
		}
	}
	return _aiClient
}

func GetAiProvider() IAiClientProvider {
	return _aiClient
}

// ValidateTaskProof implements IAiClientProvider.
func (a *aiClient) ValidateTaskProof(proof ValidateTaskProof, ctx context.Context) (*ValidateTaskProofResponse, error) {
	// Abort early if context already canceled / timed out
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	AIUnavailableResponse := &ValidateTaskProofResponse{
		AIEvaluation: "uncertain",
		AIReason:     "AI validation temporarily unavailable, please wait for human review",
	}

	raw, err := a.groqProvider.AskWithImages(
		ctx,
		os.Getenv("GROQ_API_KEY"),
		fmt.Sprintf(prompts.TaskValidatePrompt, proof.TaskDescription),
		[]string{proof.ProofImageURL},
	)
	if err != nil {
		a.errLogger.Printf("validate task proof: %s", err)
		return AIUnavailableResponse, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var result ValidateTaskProofResponse

	// Try direct unmarshal first
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		// Attempt to extract a JSON object from noisy model output
		jsonCandidate := extractJSONObject(raw)
		if jsonCandidate == "" {
			a.errLogger.Printf("parse AI response direct failed and no JSON found: %v", err)
			return AIUnavailableResponse, nil
		}

		if err2 := json.Unmarshal([]byte(jsonCandidate), &result); err2 != nil {
			a.errLogger.Printf("parse AI response from candidate failed: %v; candidate: %s", err2, jsonCandidate)
			return AIUnavailableResponse, nil
		}
	}

	// Normalize and validate evaluation value
	eval := strings.ToLower(strings.TrimSpace(result.AIEvaluation))
	switch eval {
	case "valid", "invalid", "uncertain":
		result.AIEvaluation = eval
	default:
		// Unknown evaluation -> mark uncertain and preserve reason
		result.AIEvaluation = "uncertain"
		if result.AIReason == "" {
			result.AIReason = "AI returned unexpected evaluation"
		}
	}

	// Ensure a reason exists
	if strings.TrimSpace(result.AIReason) == "" {
		result.AIReason = "AI did not provide a reason"
	}

	return &result, nil
}

// extractJSONObject tries to find the first balanced JSON object in s.
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

// ExtractChildInfo implements IAiClientProvider.
func (a *aiClient) ExtractChildInfo(birthCertURL string, firstGuardianIDCardURL string, secondGuardianIDCardURL *string, ctx context.Context) (*response.ExtractChildUploadInfoResponse, error) {
	// Validate required URLs
	if birthCertURL == "" {
		return nil, fmt.Errorf("extract child info: missing birth certificate URL")
	}
	if firstGuardianIDCardURL == "" {
		return nil, fmt.Errorf("extract child info: missing first guardian ID card URL")
	}

	// Build image URLs list
	imageURLs := []string{birthCertURL, firstGuardianIDCardURL}
	
	if secondGuardianIDCardURL != nil && *secondGuardianIDCardURL != "" {
		imageURLs = append(imageURLs, *secondGuardianIDCardURL)
	}

	raw, err := a.groqProvider.AskWithImages(
		ctx,
		os.Getenv("GROQ_API_KEY"),
		prompts.ChildUploadInfoExtractionPrompt,
		imageURLs,
	)
	if err != nil {
		a.errLogger.Printf("extract child info: %s", err)
		return nil, fmt.Errorf("extract child info: %w", err)
	}

	var result response.ExtractChildUploadInfoResponse
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		a.errLogger.Printf("parse AI response: %s", err)
		return nil, fmt.Errorf("parse AI response: %w", err)
	}

	return &result, nil
}
