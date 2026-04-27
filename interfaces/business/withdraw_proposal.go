package business

import (
	"context"
	"raise-child/model/dtos/request"
	"raise-child/model/dtos/response"
)

// CreateWithdrawProposal(req request.CreateWithdrawProposalRequest, ctx context.Context) (response.BuildTransactionResponse, error)

type IWithdrawProposalService interface {
	GetWithdrawProposal(id string, ctx context.Context) (response.WithdrawProposalResponse, error)
	GetWithdrawProposals(req request.GetWithdrawProposalsRequest, ctx context.Context) (response.PaginationDataResponse, error)
	CreateWithdrawProposal(req request.CreateWithdrawProposalRequest, ctx context.Context) error
	VoteWithdrawProposal(id string, ctx context.Context) error
	ConfirmWithdrawProposal(id string, ctx context.Context) (map[string]interface{}, error)
	ConfirmMainPoolWithdrawProposal(id, capturedImgBlobId string, ctx context.Context) (response.BuildTransactionResponse, error)
}
