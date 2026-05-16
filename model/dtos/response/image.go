package response

type PresignUrlResponse struct {
	UploadUrl string `json:"upload_url"`
	APIKey    string `json:"api_key"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
	Folder    string `json:"folder"`
}
