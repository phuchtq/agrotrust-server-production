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

// PresignedUrl implements business.IImageService.
func (i *imageService) PresignedUrl(ctx context.Context) (response.PresignedUrlResponse, error) {
	var timestamp int64 = time.Now().Add(time.Hour).Unix()
	var folder string = os.Getenv(env.CLOUDINARY_STORAGE_FOLDER)

	return response.PresignedUrlResponse{
		UploadUrl: os.Getenv(env.CLOUDINARY_UPLOAD_URL),
		APIKey:    os.Getenv(env.CLOUDINARY_API_KEY),
		Timestamp: timestamp,
		Signature: cloudinary_provider.GenerateSignature(timestamp, folder),
		Folder:    folder,
	}, nil
}
