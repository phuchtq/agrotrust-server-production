package request

type ExtractChildInfoRequest struct {
	ChildBirthCertificateURL string `json:"child_birth_certificate_url" validate:"url,required"`
	GuardianIDCardURL        string `json:"guardian_id_card_url" validate:"url,required"`
}
