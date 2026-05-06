package repository

import (
	"context"
	"raise-child/model/entities"
)

type IBackgroundChildrenWithdrawProposalRequestRepository interface {
	CreateRequest(req entities.BackgroundChildrenWithdrawProposalsRequest, ctx context.Context) error
	GetCurrentPendingRequests(ctx context.Context) ([]entities.BackgroundChildrenWithdrawProposalsRequest, error)
}
