package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"raise-child/constants/env"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
)

type aiClient struct {
	geminiProvider *geminiProvider
	errLogger      *log.Logger
}

var _aiClient *aiClient

type ImageValidateCase string

const ()

type IAiClientProvider interface {
	ValidateUploadChildRequest(req ValidateUploadChildRequest, ctx context.Context) string
	ValidateCreateCenterRequest(req ValidateCreateCenterRequest, ctx context.Context) string
	ValidateRegistrationRequest(req ValidateRegistrationRequest, ctx context.Context) string
	ValidateTaskProof(req ValidateTaskProof, ctx context.Context) string
	ValidateProvideNeedForChildTaskProof(req ValidateProvideNeedForChildTaskProof, ctx context.Context) string
	ValidateWithdrawProposal(req ValidateWithdrawProposal, ctx context.Context) string
	ValidateChildSpecialNeedProposal(req ValidateChildSpecialNeedProposal, ctx context.Context) string
	ValidatePoolCampaign(req ValidatePoolCampaign, ctx context.Context) string
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

// // ValidateUploadChildRequest implements IAiClientProvider.
// func (a *aiClient) ValidateUploadChildRequest(req ValidateUploadChildRequest, ctx context.Context) string {
// 	if !a.geminiProvider.limiter.Allow() {
// 		return ""
// 	}

// 	jsonData, _ := json.MarshalIndent(req, "", "  ")
// 	var prompt = []genai.Part{
// 		genai.Text(fmt.Sprintf("Case: %s", upload_child_validate_case)),
// 		genai.Text(string(jsonData)),
// 	}

// 	if req.AvatarBytesImage != nil {
// 		prompt = append(prompt, genai.Text("Label: Child Avatar"), genai.ImageData("jpeg", req.AvatarBytesImage))
// 	}

// 	if req.ChildBirthCertificateBytesImage != nil {
// 		prompt = append(prompt, genai.Text("Label: Child Birth Certificate"), genai.ImageData("jpeg", req.ChildBirthCertificateBytesImage))
// 	}

// 	if req.FirstGuardian.IdentityCardBytesImage != nil {
// 		prompt = append(prompt, genai.Text("Label: Child First Guardian Identity Card"), genai.ImageData("jpeg", req.FirstGuardian.IdentityCardBytesImage))
// 	}

// 	if req.SecondGuardian.IdentityCardBytesImage != nil {
// 		prompt = append(prompt, genai.Text("Label: Child Second Guardian Identity Card"), genai.ImageData("jpeg", req.SecondGuardian.IdentityCardBytesImage))
// 	}

// 	return a.processPrompt(prompt, ctx)
// }

// ValidateUploadChildRequest implements IAiClientProvider.
func (a *aiClient) ValidateUploadChildRequest(req ValidateUploadChildRequest, ctx context.Context) string {
	type TextContext struct {
		IdentityCode   string                `json:"identity_code"`
		Region         string                `json:"region"`
		FirstName      string                `json:"first_name"`
		LastName       string                `json:"last_name"`
		Gender         string                `json:"gender"`
		DateOfBirth    string                `json:"date_of_birth"`
		HomeAddress    string                `json:"home_address"`
		FirstGuardian  ChildGuardianProfile  `json:"first_guardian"`
		SecondGuardian *ChildGuardianProfile `json:"second_guardian"`
	}

	var textCtx = TextContext{
		IdentityCode:   req.IdentityCode,
		Region:         req.Region,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Gender:         req.Gender,
		DateOfBirth:    req.DateOfBirth,
		HomeAddress:    req.HomeAddress,
		FirstGuardian:  req.FirstGuardian,
		SecondGuardian: req.SecondGuardian,
	}

	dataBytes, _ := json.Marshal(textCtx)
	if dataBytes == nil {
		return ""
	}

	var prompt string = fmt.Sprintf("Validate case: %s\n", upload_child_validate_case)
	prompt += fmt.Sprintf("Data context: %s\n", string(dataBytes))
	prompt += _prompt_instruction
	prompt += "Answer: "

	var contentParts = []ContentPart{
		{Type: "text", Text: prompt},
	}

	contentParts = append(contentParts, ContentPart{
		Type:     "image_url",
		ImageURL: &ImageURL{URL: req.HomeBase64},
	})

	contentParts = append(contentParts, ContentPart{
		Type:     "image_url",
		ImageURL: &ImageURL{URL: req.ChildBirthCertificateBase64},
	})

	contentParts = append(contentParts, ContentPart{
		Type:     "image_url",
		ImageURL: &ImageURL{URL: req.AvatarBase64},
	})

	contentParts = append(contentParts, ContentPart{
		Type:     "image_url",
		ImageURL: &ImageURL{URL: req.FirstGuardian.IdentityCardBase64},
	})

	if req.SecondGuardian != nil && req.SecondGuardian.IdentityCardBase64 != "" {
		contentParts = append(contentParts, ContentPart{
			Type:     "image_url",
			ImageURL: &ImageURL{URL: req.SecondGuardian.IdentityCardBase64},
		})
	}

	var reqBody ChatRequest = ChatRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "user",
				Content: contentParts,
			},
		},
	}

	return a.processPromptV2(reqBody)
}

// ValidateCreateCenterRequest implements IAiClientProvider.
func (a *aiClient) ValidateCreateCenterRequest(req ValidateCreateCenterRequest, ctx context.Context) string {
	if !a.geminiProvider.limiter.Allow() {
		return ""
	}

	jsonData, _ := json.MarshalIndent(req, "", "  ")
	var prompt = []genai.Part{
		genai.Text(fmt.Sprintf("Case: %s", create_center_request_validate_case)),
		genai.Text(string(jsonData)),
	}

	if req.CenterBytesImage != nil {
		prompt = append(prompt, genai.Text("Label: Center Image"), genai.ImageData("jpeg", req.CenterBytesImage))
	}

	return a.processPrompt(prompt, ctx)
}

// ValidateProvideMealForChildTaskProof implements IAiClientProvider.
func (a *aiClient) ValidateProvideNeedForChildTaskProof(req ValidateProvideNeedForChildTaskProof, ctx context.Context) string {
	type TextContext struct {
		TaskDescription string    `json:"task_description"`
		CreatedAt       time.Time `json:"created_at"`
	}

	var textCtx = TextContext{
		TaskDescription: req.TaskDescription,
		CreatedAt:       req.CreatedAt,
	}

	dataBytes, _ := json.Marshal(textCtx)
	if dataBytes == nil {
		return ""
	}

	var prompt string = fmt.Sprintf("Validate case: %s\n", provide_need_for_child_task_proof_validate_case)
	prompt += fmt.Sprintf("Data context: %s\n", string(dataBytes))
	prompt += _prompt_instruction
	prompt += "Answer: "

	var contentParts = []ContentPart{
		{Type: "text", Text: prompt},
	}

	contentParts = append(contentParts, ContentPart{
		Type:     "image_url",
		ImageURL: &ImageURL{URL: req.ProofBase64},
	})

	var base64Str string = base64.StdEncoding.EncodeToString(req.ChildAvatarBytesImage)
	var dataUrl string = "data:image/jpeg;base64," + base64Str
	contentParts = append(contentParts, ContentPart{
		Type:     "image_url",
		ImageURL: &ImageURL{URL: dataUrl},
	})

	var reqBody ChatRequest = ChatRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "user",
				Content: contentParts,
			},
		},
	}

	return a.processPromptV2(reqBody)
}

// ValidateRegistrationRequest implements IAiClientProvider.
func (a *aiClient) ValidateRegistrationRequest(req ValidateRegistrationRequest, ctx context.Context) string {
	type TextContext struct {
		IdentityCode string `json:"identity_code"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
		Gender       string `json:"gender"`
		DateOfBirth  string `json:"date_of_birth"`
		PhoneNumber  string `json:"phone_number"`
	}

	var textCtx = TextContext{
		IdentityCode: req.IdentityCode,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Gender:       req.Gender,
		DateOfBirth:  req.DateOfBirth,
		PhoneNumber:  req.PhoneNumber,
	}

	dataBytes, _ := json.Marshal(textCtx)
	if dataBytes == nil {
		return ""
	}

	var prompt string = fmt.Sprintf("Validate case: %s\n", registration_request_validate_case)
	prompt += fmt.Sprintf("Data context: %s\n", string(dataBytes))
	prompt += _prompt_instruction
	prompt += "Answer: "

	var contentParts = []ContentPart{
		{Type: "text", Text: prompt},
	}

	contentParts = append(contentParts, ContentPart{
		Type:     "image_url",
		ImageURL: &ImageURL{URL: req.IdentityCardBase64},
	})

	contentParts = append(contentParts, ContentPart{
		Type:     "image_url",
		ImageURL: &ImageURL{URL: req.AvatarBase64},
	})

	var reqBody ChatRequest = ChatRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "user",
				Content: contentParts,
			},
		},
	}

	return a.processPromptV2(reqBody)
}

// ValidateTaskProof implements IAiClientProvider.
func (a *aiClient) ValidateTaskProof(req ValidateTaskProof, ctx context.Context) string {
	type TextContext struct {
		TaskDescription string    `json:"task_description"`
		CreatedAt       time.Time `json:"created_at"`
	}

	var textCtx = TextContext{
		TaskDescription: req.TaskDescription,
		CreatedAt:       req.CreatedAt,
	}

	dataBytes, _ := json.Marshal(textCtx)
	if dataBytes == nil {
		return ""
	}

	var prompt string = fmt.Sprintf("Validate case: %s\n", task_proof_validate_case)
	prompt += fmt.Sprintf("Data context: %s\n", string(dataBytes))
	prompt += _prompt_instruction
	prompt += "Answer: "

	var contentParts = []ContentPart{
		{Type: "text", Text: prompt},
	}

	contentParts = append(contentParts, ContentPart{
		Type:     "image_url",
		ImageURL: &ImageURL{URL: req.ProofBase64},
	})

	var reqBody ChatRequest = ChatRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "user",
				Content: contentParts,
			},
		},
	}

	return a.processPromptV2(reqBody)
}

// ValidateWithdrawProposal implements IAiClientProvider.
func (a *aiClient) ValidateWithdrawProposal(req ValidateWithdrawProposal, ctx context.Context) string {
	if !a.geminiProvider.limiter.Allow() {
		return ""
	}

	jsonData, _ := json.MarshalIndent(req, "", "  ")
	var prompt = []genai.Part{
		genai.Text(fmt.Sprintf("Case: %s - %s", withdraw_proposal_validate_case, req.Purpose)),
		genai.Text(string(jsonData)),
	}

	if req.ProofBytesImage != nil {
		prompt = append(prompt, genai.Text("Label: Withdraw Proof Image"), genai.ImageData("jpeg", req.ProofBytesImage))
	}

	return a.processPrompt(prompt, ctx)
}

// ValidateChildSpecialNeedProposal implements IAiClientProvider.
func (a *aiClient) ValidateChildSpecialNeedProposal(req ValidateChildSpecialNeedProposal, ctx context.Context) string {
	if !a.geminiProvider.limiter.Allow() {
		return ""
	}

	jsonData, _ := json.MarshalIndent(req, "", "  ")
	var prompt = []genai.Part{
		genai.Text(fmt.Sprintf("Case: %s", child_special_need_proposal_validate_case)),
		genai.Text(string(jsonData)),
	}

	if req.ProofBytesImage != nil {
		prompt = append(prompt, genai.Text("Label: Campaign Relevant Proof Image"), genai.ImageData("jpeg", req.ProofBytesImage))
	}

	return a.processPrompt(prompt, ctx)
}

// ValidatePoolCampaign implements IAiClientProvider.
func (a *aiClient) ValidatePoolCampaign(req ValidatePoolCampaign, ctx context.Context) string {
	if !a.geminiProvider.limiter.Allow() {
		return ""
	}

	jsonData, _ := json.MarshalIndent(req, "", "  ")
	var prompt = []genai.Part{
		genai.Text(fmt.Sprintf("Case: %s", pool_campaign_validate_case)),
		genai.Text(string(jsonData)),
	}

	if req.ProofBytesImage != nil {
		prompt = append(prompt, genai.Text("Label: Pool Campaign Relevant Proof Image"), genai.ImageData("jpeg", req.ProofBytesImage))
	}

	return a.processPrompt(prompt, ctx)
}

func (a *aiClient) processPrompt(prompt []genai.Part, ctx context.Context) string {
	res, err := a.geminiProvider.model.GenerateContent(ctx, prompt...)
	if err != nil {
		a.errLogger.Println(err.Error())
		return ""
	}
	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		var raw string = fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0])
		return strings.ToLower(strings.Trim(raw, " \n\r\t.\"'"))
	}

	return "uncertain"
}

func (a *aiClient) processPromptV2(body ChatRequest) string {
	bodyBytes, _ := json.Marshal(body)
	if bodyBytes == nil {
		return ""
	}

	httpReq, err := http.NewRequest("POST", groqAPIURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return ""
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+os.Getenv(env.GROQ_API_KEY))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return ""
	}

	if chatResp.Error != nil {
		return ""
	}

	if len(chatResp.Choices) == 0 {
		return ""
	}

	return strings.ToLower(strings.Trim(chatResp.Choices[0].Message.Content, " \n\r\t.\"'"))
}
