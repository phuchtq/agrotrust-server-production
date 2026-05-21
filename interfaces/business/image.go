package business

import (
	"context"
	"raise-child/model/dtos/response"
)

type IImageService interface {
	PresignedUrl(ctx context.Context) (response.PresignedUrlResponse, error)
}