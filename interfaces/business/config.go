package business

import (
	"context"
	"raise-child/model/dtos/request"
)

// UpdateChildEditMealNeedDates(req request.UpdateChildEditNeedDatesRequest, ctx context.Context) (response.BuildTransactionResponse, error)
// UpdateChildEditBooksNeedDates(req request.UpdateChildEditNeedDatesRequest, ctx context.Context) (response.BuildTransactionResponse, error)
// UpdateChildEditHealthInsuranceNeedDates(req request.UpdateChildEditNeedDatesRequest, ctx context.Context) (response.BuildTransactionResponse, error)

type IConfigService interface {
	UpdateChildEditMealNeedDates(req request.UpdateChildEditNeedDatesRequest, ctx context.Context) error
	UpdateChildEditBooksNeedDates(req request.UpdateChildEditNeedDatesRequest, ctx context.Context) error
	UpdateChildEditHealthInsuranceNeedDates(req request.UpdateChildEditNeedDatesRequest, ctx context.Context) error
	EditSpecialNeedDao(req request.EditDaoRequest, ctx context.Context) error
	EditChildMaxAgeInSupport(ctx context.Context) error
}
