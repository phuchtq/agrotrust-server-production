package business

import (
	"context"
	"raise-child/model/dtos/request"
	"raise-child/model/dtos/response"
)

type IPaymentService interface {
	GetPayments(req request.GetPaymentsRequest, ctx context.Context) (response.PaginationDataResponse, error)
	ApprovePayment(id string, ctx context.Context) (response.BuildTransactionResponse, error)
	RefusePayment(id string, ctx context.Context) error
	Donate(req request.DonateRequest, ctx context.Context) (response.UrlAPIResponse, error)
	Callback(id string, ctx context.Context) (string, error)
	CallbackWithAuth(id, capturedImgBlobId string, ctx context.Context) (string, error)
}
