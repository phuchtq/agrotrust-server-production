package business

import (
	"context"
	"log"
	"os"
	"raise-child/constants/env"
	"raise-child/constants/shared"
	"raise-child/interfaces/business"
	"raise-child/model/dtos/response"
	"raise-child/util"
	cloudinary_provider "raise-child/util/image/cloudinary"
	"time"
)

type imageService struct {
	errLogger *log.Logger
}

func initializeImageService(errLogger *log.Logger) business.IImageService {
	return &imageService{
		errLogger: errLogger,
	}
}

func GenerateImageService() business.IImageService {
	var errLogger = util.GetLogConfig(shared.ERROR_LEVEL)
	return initializeImageService(errLogger)
}

// PresignUrl implements business.IImageService.
func (i *imageService) PresignUrl(ctx context.Context) (response.PresignUrlResponse, error) {
	var timestamp int64 = time.Now().Add(time.Hour).Unix()
	var folder string = os.Getenv(env.CLOUDINARY_STORAGE_FOLDER)

	return response.PresignUrlResponse{
		UploadUrl: os.Getenv(env.CLOUDINARY_UPLOAD_URL),
		APIKey:    os.Getenv(env.CLOUDINARY_API_KEY),
		Timestamp: timestamp,
		Signature: cloudinary_provider.GenerateSignature(timestamp, folder),
		Folder:    folder,
	}, nil
}
