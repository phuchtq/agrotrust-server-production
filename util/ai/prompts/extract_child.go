package prompts

// ChildExtractionPrompt là prompt dùng để trích xuất thông tin trẻ từ giấy khai sinh
// và thông tin người thân từ CCCD/CMND. Được gửi kèm 2 ảnh Cloudinary URL.
//
// Ảnh 1: Giấy khai sinh của trẻ
// Ảnh 2: CCCD/CMND của người thân
const ChildExtractionPrompt = `You are a specialized OCR data extractor for Vietnamese civil documents.
You will receive two images:
- Image 1: A Vietnamese child's birth certificate (Giấy khai sinh)
- Image 2: The guardian's Vietnamese identity document (Căn cước công dân or CMND)

Extract the following information accurately. Apply these rules:
1. Accept Vietnamese names with or without diacritics (e.g., "Nguyen Van A" and "Nguyễn Văn A" are both valid).
2. Date format must be YYYY-MM-DD. If only year is visible, use YYYY-01-01.
3. Gender must be exactly "male" or "female" (lowercase English).
4. If a field is unreadable or not present in the image, return an empty string — never guess or fabricate.
5. "region" refers to the province/city of the child's household registration (nơi đăng ký hộ khẩu).

Return ONLY a valid JSON object with exactly these keys:
{
  "region": "",
  "first_name": "",
  "last_name": "",
  "gender": "",
  "date_of_birth": "",
  "home_address": "",
  "guardian_full_name": "",
  "guardian_identity_code": "",
  "guardian_date_of_birth": "",
  "guardian_gender": ""
}

Do not include markdown, code fences, bullet points, or any text outside the JSON object.`
