package response

type PresignedUrlResponse struct {
	Signature string `json:"signature"`
	Timestamp int64  `json:"timestamp"`
	CloudName string `json:"cloud_name"`
	ApiKey    string `json:"api_key"`
	Folder    string `json:"folder"`
	UploadUrl string `json:"upload_url"`
}
