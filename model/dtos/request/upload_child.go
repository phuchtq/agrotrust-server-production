package request

type ExtractChildUploadInfoRequest struct {
	ChildBirthCertificateURL string  `json:"child_birth_certificate_url" validate:"url,required"`
	FirstGuardianIDCardURL   string  `json:"first_guardian_id_card_url" validate:"url,required"`
	SecondGuardianIDCardURL  *string `json:"second_guardian_id_card_url" validate:"url"`
}
