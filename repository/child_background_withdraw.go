package repository

import (
	"context"
	"database/sql"
	"log"
	"raise-child/interfaces/repository"
	"raise-child/model/entities"
)

type backgroundChildrenWithdrawRequestRepo struct {
	db        *sql.DB
	errLogger *log.Logger
}

const (
	background_children_withdraw_request_table string = "background_children_withdraw_requests"
)

func InitializeBackgroundChildrenWithdrawRequestRepository(db *sql.DB, errLogger *log.Logger) repository.IBackgroundChildrenWithdrawProposalRequestRepository {
	return &backgroundChildrenWithdrawRequestRepo{
		db:        db,
		errLogger: errLogger,
	}
}

// CreateRequest implements repository.IBackgroundChildrenWithdrawProposalRequestRepository.
func (b *backgroundChildrenWithdrawRequestRepo) CreateRequest(req entities.BackgroundChildrenWithdrawProposalsRequest, ctx context.Context) error {
	panic("unimplemented")
}

// GetCurrentPendingRequests implements repository.IBackgroundChildrenWithdrawProposalRequestRepository.
func (b *backgroundChildrenWithdrawRequestRepo) GetCurrentPendingRequests(ctx context.Context) ([]entities.BackgroundChildrenWithdrawProposalsRequest, error) {
	panic("unimplemented")
}
