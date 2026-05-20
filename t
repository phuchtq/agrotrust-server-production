package main

import (
	"mime/multipart"
)

// WalrusUploadRequest đại diện cho dữ liệu từ Frontend gửi lên hệ thống
type WalrusUploadRequest struct {
	// Bắt trường dữ liệu file nhị phân (phía frontend đặt key là "file")
	File *multipart.FileHeader `form:"file" binding:"required"`
	
	// Bắt tham số vòng đời lưu trữ, nếu frontend không truyền thì nhận giá trị default
	Epochs int `form:"epochs,default=1"`
}


=====================================================================================

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"://github.com"
)

func main() {
	r := gin.Default()

	r.POST("/upload", func(c *gin.Context) {
		// 1. Khởi tạo và bắt dữ liệu Request vào Struct một cách tường minh
		var uploadReq WalrusUploadRequest
		if err := c.ShouldBind(&uploadReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Dữ liệu request không hợp lệ: " + err.Error(),
			})
			return
		}

		// 2. Mở file từ struct đã bind thành công
		openedFile, err := uploadReq.File.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể mở file"})
			return
		}
		defer openedFile.Close()

		// Đọc nội dung file vào memory buffer
		var fileBuffer bytes.Buffer
		if _, err := io.Copy(&fileBuffer, openedFile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi đọc dữ liệu file"})
			return
		}

		// 3. Tiến hành chuyển tiếp payload sang Walrus Publisher
		// Sử dụng biến uploadReq.Epochs lấy trực tiếp từ Struct
		walrusURL := fmt.Sprintf("https://walrus.space", uploadReq.Epochs)
		
		req, err := http.NewRequest(http.MethodPut, walrusURL, &fileBuffer)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi khởi tạo request lên Walrus"})
			return
		}
		req.Header.Set("Content-Type", "application/octet-stream")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể kết nối Walrus: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Đọc kết quả JSON trả về từ Walrus
		body, _ := io.ReadAll(resp.Body)
		
		// Giải mã cấu trúc Response để lấy blobId (sử dụng bộ struct định nghĩa trước đó)
		var walrusResult WalrusUploadResponse
		if err := json.Unmarshal(body, &walrusResult); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi giải mã kết quả từ mạng lưới"})
			return
		}

		// Kiểm tra và lấy Blob ID dựa trên trạng thái file cũ/mới
		var blobId string
		if walrusResult.NewlyCreated != nil {
			blobId = walrusResult.NewlyCreated.BlobObject.BlobId
		} else if walrusResult.AlreadyCertified != nil {
			blobId = walrusResult.AlreadyCertified.BlobId
		}

		c.JSON(http.StatusOK, gin.H{
			"blobId": blobId,
		})
	})

	r.Run(":8080")
}
