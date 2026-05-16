package business

import (
	"context"
	"raise-child/model/dtos/response"
)

type IImageService interface {
	PresignUrl(ctx context.Context) (response.PresignUrlResponse, error)
}
