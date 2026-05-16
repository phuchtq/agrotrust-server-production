package cloudinary

import (
	"crypto/sha1"
	"fmt"
	"os"
	"raise-child/constants/env"
	"strings"
	"time"
)

func GenerateSignature(timestamp int64, folder string) string {
	var keys []string = []string{"folder", "timestamp"}
	var params = map[string]string{
		keys[0]: folder,
		keys[1]: fmt.Sprintf("%d", timestamp),
	}

	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}

	var payload string = strings.Join(parts, "&") + os.Getenv(env.CLOUDINARY_API_SECRET)
	var hash = sha1.New()
	hash.Write([]byte(payload))

	return fmt.Sprintf("%x", hash.Sum(nil))
}

func GeneratePresignedUrl(id string) string {
	var timestamp int64 = time.Now().Add(time.Hour).Unix()
	var folder string = os.Getenv(env.CLOUDINARY_STORAGE_FOLDER)
	var signature string = GenerateSignature(timestamp, folder)

	return fmt.Sprintf(
		"https://cloudinary.com%s/image/authenticated?api_key=%s&public_id=%s&timestamp=%d&signature=%s",
		os.Getenv(env.CLOUDINARY_CLOUD_NAME), os.Getenv(env.CLOUDINARY_API_KEY), id, timestamp, signature,
	)
}
