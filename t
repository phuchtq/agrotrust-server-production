Để giải thích kỹ thuật pre-signed URL, mình sẽ vẽ sơ đồ luồng trực quan trước, sau đó đi vào code chi tiết.## Kỹ thuật Pre-signed URL là gì?

Ý tưởng cốt lõi rất đơn giản: *thay vì client gửi file lên server của bạn, rồi server mới đẩy lên Cloudinary* (tốn băng thông gấp đôi), pre-signed URL cho phép client *upload thẳng lên Cloudinary* — nhưng phải có "giấy phép" do server của bạn ký trước.

"Giấy phép" đó chính là *chữ ký số (signature)* — một chuỗi hash chứng minh rằng request upload này đã được server bạn cho phép, mà không cần lộ API_SECRET.

---

## Cách signature hoạt động

// Cloudinary yêu cầu bạn tạo signature như sau:
// 1. Sắp xếp các params theo thứ tự alphabet
// 2. Nối thành chuỗi: "folder=uploads&timestamp=1715000000"
// 3. Thêm API_SECRET vào cuối: "folder=uploads&timestamp=1715000000mySecret"
// 4. Hash bằng SHA-1

func GenerateSignature(params map[string]string, apiSecret string) string {
    // Bước 1: Sắp xếp key theo alphabet
    keys := make([]string, 0, len(params))
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    // Bước 2: Tạo chuỗi "key=value&key=value"
    parts := make([]string, 0, len(keys))
    for _, k := range keys {
        parts = append(parts, k+"="+params[k])
    }
    queryString := strings.Join(parts, "&")

    // Bước 3 + 4: Thêm secret rồi SHA-1
    toSign := queryString + apiSecret
    h := sha1.New()
    h.Write([]byte(toSign))
    return fmt.Sprintf("%x", h.Sum(nil))
}

Kết quả: Cloudinary nhận được signature, tự tính lại bằng cách dùng API_SECRET của chính nó, nếu khớp → cho phép upload. Nếu ai đó giả mạo timestamp hay folder → hash sẽ sai → bị từ chối.

---

## Code đầy đủ (Go)

package main

import (
    "crypto/sha1"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "sort"
    "strings"
    "time"
)

// PresignedData là dữ liệu trả về cho client
type PresignedData struct {
    UploadURL string `json:"upload_url"`
    APIKey    string `json:"api_key"`
    Timestamp int64  `json:"timestamp"`
    Signature string `json:"signature"`
    Folder    string `json:"folder"`
}

func generateSignature(params map[string]string, secret string) string {
    keys := make([]string, 0, len(params))
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    parts := make([]string, 0, len(keys))
    for _, k := range keys {
        parts = append(parts, k+"="+params[k])
    }

    payload := strings.Join(parts, "&") + secret
    h := sha1.New()
    h.Write([]byte(payload))
    return fmt.Sprintf("%x", h.Sum(nil))
}

func presignedHandler(w http.ResponseWriter, r *http.Request) {
    cloudName := os.Getenv("CLOUDINARY_CLOUD_NAME") // "myapp"
    apiKey    := os.Getenv("CLOUDINARY_API_KEY")    // public, OK để gửi ra ngoài
    apiSecret := os.Getenv("CLOUDINARY_API_SECRET") // SECRET, không bao giờ gửi ra ngoài!

    timestamp := time.Now().Unix()
    folder    := "user-avatars" // hoặc lấy từ query param

    // Chỉ đưa vào params những gì bạn muốn Cloudinary enforce
    // Nếu thêm params ở đây mà client gửi lên khác → Cloudinary từ chối
    params := map[string]string{
        "folder":    folder,
        "timestamp": fmt.Sprintf("%d", timestamp),
    }

    signature := generateSignature(params, apiSecret)

    resp := PresignedData{
        UploadURL: fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", cloudName),
        APIKey:    apiKey,       // OK để public
        Timestamp: timestamp,
        Signature: signature,
        Folder:    folder,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func main() {
    http.HandleFunc("/api/presigned-url", presignedHandler)
    fmt.Println("Server chạy tại :8080")
    http.ListenAndServe(":8080", nil)
}

## Client dùng pre-signed data để upload

// JavaScript phía client (browser)
async function uploadImage(file) {
    // Bước 1: Xin "giấy phép" từ server của bạn
    const res = await fetch('/api/presigned-url');
    const { upload_url, api_key, timestamp, signature, folder } = await res.json();

    // Bước 2: Upload thẳng lên Cloudinary, KHÔNG qua server của bạn
    const formData = new FormData();
    formData.append('file', file);
    formData.append('api_key', api_key);
    formData.append('timestamp', timestamp); 
    formData.append('signature', signature);
    formData.append('folder', folder);

    const upload = await fetch(upload_url, { method: 'POST', body: formData });
    const result = await upload.json();

    console.log('URL ảnh:', result.secure_url); // lưu cái này vào DB
}

---

## Tại sao không upload qua server của bạn?

| Cách thông thường | Pre-signed URL |
|---|---|
| File đi: Client → Server → Cloudinary | File đi: Client → Cloudinary thẳng |
| Server phải xử lý file lớn | Server chỉ tạo một chuỗi hash nhỏ |
| Tốn băng thông gấp đôi | Tiết kiệm hoàn toàn |
| Server có thể bị nghẽn | Scale tốt hơn nhiều |

Điểm mấu chốt: **API_SECRET không bao giờ rời khỏi server của bạn**. Client chỉ nhận được signature (kết quả hash) — từ đó không thể suy ngược ra secret.


Upload xong  →  lưu public_id của ảnh vào DB
                        ↓
User muốn xem ảnh  →  gọi API của bạn
                        ↓
              Server dùng public_id  của ảnh tạo signed URL (hạn 1h)
                        ↓
                  Trả signed URL về cho client
                        ↓
                  Client dùng URL đó để hiển thị ảnh