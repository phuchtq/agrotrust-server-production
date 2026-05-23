package response

type ExtractChildUploadInfoResponse struct {
	IdentityCode           string `json:"identity_code" `
	FirstName              string `json:"first_name"`
	LastName               string `json:"last_name"`
	Gender                 string `json:"gender"`
	DateOfBirth            string `json:"date_of_birth"`
	FirstGuardianFullName  string `json:"first_guardian_full_name"`
	SecondGuardianFullName string `json:"second_guardian_full_name"`
}
