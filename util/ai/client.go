package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"raise-child/model/dtos/response"
	"raise-child/util/ai/prompts"
)

type aiClient struct {
	groqProvider *GroqClient
	errLogger    *log.Logger
}

var _aiClient *aiClient

type IAiClientProvider interface {
	ExtractChildInfo(birthCertURL string, guardianIdURL string) (*response.ExtractChildInfoResponse, error)
	ValidateTaskProof(req ValidateTaskProof, ctx context.Context) string
	ValidateWithdrawProposal(req ValidateWithdrawProposal, ctx context.Context) string
	ValidateSpecialNeedProposal(req ValidateChildSpecialNeedProposal, ctx context.Context) string

	// ValidateUploadChildRequest(req ValidateUploadChildRequest, ctx context.Context) string
	// ValidateCreateCenterRequest(req ValidateCreateCenterRequest, ctx context.Context) string
	// ValidateRegistrationRequest(req ValidateRegistrationRequest, ctx context.Context) string
	// ValidateProvideNeedForChildTaskProof(req ValidateProvideNeedForChildTaskProof, ctx context.Context) string
	// ValidatePoolCampaign(req ValidatePoolCampaign, ctx context.Context) string
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
func (a *aiClient) ExtractChildInfo(birthCertURL string, guardianIdURL string) (*response.ExtractChildInfoResponse, error) {
	raw, err := a.groqProvider.AskWithImages(
		os.Getenv("GROQ_API_KEY"),
		prompts.ChildExtractionPrompt,
		[]string{birthCertURL, guardianIdURL},
	)
	if err != nil {
		return nil, fmt.Errorf("extract child info: %w", err)
	}

	var result response.ExtractChildInfoResponse
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse AI response: %w", err)
	}

	return &result, nil
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
