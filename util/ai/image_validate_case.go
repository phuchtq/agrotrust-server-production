package ai

import "time"

// Validate cases
const (
	upload_child_validate_case                      string = "Upload Child Request Validation"
	create_center_request_validate_case             string = "Create Center Request Validation"
	registration_request_validate_case              string = "Registration Request Validation"
	task_proof_validate_case                        string = "Task Proof Validation"
	provide_need_for_child_task_proof_validate_case string = "Provide Need For Child Task Proof Validation"
	withdraw_proposal_validate_case                 string = "Withdraw Proposal Validation"
	child_special_need_proposal_validate_case       string = "Child Special Need Campaign Validation"
	pool_campaign_validate_case                     string = "Pool Campaign Validation"
)

type ValidateUploadChildRequest struct {
	HomeBase64                  string                `json:"home_base64"`
	IdentityCode                string                `json:"identity_code"`
	ChildBirthCertificateBase64 string                `json:"child_birth_certificate_base64"`
	Region                      string                `json:"region"`
	FirstName                   string                `json:"first_name"`
	LastName                    string                `json:"last_name"`
	Gender                      string                `json:"gender"`
	DateOfBirth                 string                `json:"date_of_birth"`
	HomeAddress                 string                `json:"home_address"`
	AvatarBase64                string                `json:"avatar_base64"`
	FirstGuardian               ChildGuardianProfile  `json:"first_guardian"`
	SecondGuardian              *ChildGuardianProfile `json:"second_guardian"`
}

type ValidateUploadChildRequestV2 struct {
	HomeImage                  string                `json:"home_image"`
	ChildBirthCertificateImage string                `json:"child_birth_certificate_image"`
	AvatarImage                string                `json:"avatar_image"`
	IdentityCode               string                `json:"identity_code"`
	Region                     string                `json:"region"`
	FirstName                  string                `json:"first_name"`
	LastName                   string                `json:"last_name"`
	Gender                     string                `json:"gender"`
	DateOfBirth                string                `json:"date_of_birth"`
	HomeAddress                string                `json:"home_address"`
	FirstGuardian              ChildGuardianProfile  `json:"first_guardian"`
	SecondGuardian             *ChildGuardianProfile `json:"second_guardian"`
}

type ChildGuardianProfile struct {
	FullName           string `json:"guardian_full_name"`
	PhoneNumber        string `json:"guardian_phone_number"`
	Relation           string `json:"guardian_relation_with_child"`
	IdentityCardBase64 string `json:"identity_card_base64"`
}

type ValidateCreateCenterRequest struct {
	Region           string `json:"region"`
	Address          string `json:"address"`
	PhoneNumber      string `json:"phone_number"`
	CenterBytesImage []byte `json:"center_bytes_image"`
}

type ValidateRegistrationRequest struct {
	IdentityCode       string `json:"identity_code"`
	IdentityCardBase64 string `json:"identity_card_base64"`
	AvatarBase64       string `json:"avatar_base64"`
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	Gender             string `json:"gender"`
	DateOfBirth        string `json:"date_of_birth"`
	PhoneNumber        string `json:"phone_number"`
}

type ValidateTaskProof struct {
	TaskDescription string    `json:"task_description"`
	ProofBase64     string    `json:"proof_base64"`
	CreatedAt       time.Time `json:"created_at"`
}

type ValidateProvideNeedForChildTaskProof struct {
	ChildAvatarBytesImage []byte `json:"child_avatar_bytes_image"`
	ValidateTaskProof
}

type ValidateWithdrawProposal struct {
	Purpose         string `json:"withdraw_purpose"`
	WithdrawAmount  int64  `json:"withdraw_amount"`
	Description     string `json:"description"`
	ProofBytesImage []byte `json:"proof_bytes_image"`
}

type ValidateChildSpecialNeedProposal struct {
	CampaignTarget  int64  `json:"campaign_target"`
	Description     string `json:"campaign_description"`
	ProofBytesImage []byte `json:"proof_bytes_image"`
}

type ValidatePoolCampaign struct {
	CamapaignTarget int64  `json:"campaign_target"`
	Description     string `json:"campaign_description"`
	ProofBytesImage []byte `json:"proof_bytes_image"`
}
