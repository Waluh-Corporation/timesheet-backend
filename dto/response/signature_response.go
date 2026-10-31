package response

import "time"

// UserSignatureResponse details the signature status for the authenticated user.
type UserSignatureResponse struct {
	HasSignature bool   `json:"has_signature" example:"true"`
	SignatureURL string `json:"signature_url,omitempty" example:"https://s3.example.com/signatures/1.png"`
}

// TimesheetSubmissionResponse details the monthly timesheet sign-off state.
type TimesheetSubmissionResponse struct {
	ID                string     `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID            uint       `json:"user_id" example:"1"`
	Month             int        `json:"month" example:"10"`
	Year              int        `json:"year" example:"2026"`
	Status            string     `json:"status" example:"signed"`
	SignedAt          *time.Time `json:"signed_at,omitempty"`
	VerificationToken string     `json:"verification_token" example:"ts-sec-token-123"`
	DocumentHash      string     `json:"document_hash" example:"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"`
	VerificationURL   string     `json:"verification_url" example:"https://timesheet.example.com/verify/doc/ts-sec-token-123"`
}

// BeritaAcaraSubmissionResponse details the multi-party sign-off state for a Berita Acara.
type BeritaAcaraSubmissionResponse struct {
	ID                    string     `json:"id" example:"550e8400-e29b-41d4-a716-446655440001"`
	UserID                uint       `json:"user_id" example:"1"`
	EmployeeName          string     `json:"employee_name" example:"John Doe"`
	WitnessID             uint       `json:"witness_id" example:"2"`
	WitnessName           string     `json:"witness_name" example:"Jane Smith"`
	Month                 int        `json:"month" example:"10"`
	Year                  int        `json:"year" example:"2026"`
	Status                string     `json:"status" example:"pending_witness"`
	EmployeeSignedAt      *time.Time `json:"employee_signed_at,omitempty"`
	WitnessSignedAt       *time.Time `json:"witness_signed_at,omitempty"`
	WitnessRejectionNotes string     `json:"witness_rejection_notes,omitempty"`
	VerificationToken     string     `json:"verification_token" example:"ba-sec-token-456"`
	DocumentHash          string     `json:"document_hash,omitempty"`
	VerificationURL       string     `json:"verification_url" example:"https://timesheet.example.com/verify/doc/ba-sec-token-456"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// DocumentAuditTrailItem represents a single step in a document's verification audit log trail.
type DocumentAuditTrailItem struct {
	Step        int        `json:"step" example:"1"`
	Title       string     `json:"title" example:"Document Submitted & Signed"`
	Description string     `json:"description" example:"Signed and submitted by John Doe"`
	Actor       string     `json:"actor" example:"John Doe"`
	Role        string     `json:"role" example:"Employee"`
	Timestamp   *time.Time `json:"timestamp,omitempty"`
	MaskedIP    string     `json:"masked_ip,omitempty" example:"192.168.***.***"`
	Status      string     `json:"status" example:"completed"` // "completed", "pending", "rejected", "revoked"
}

// PublicDocumentVerificationResponse is returned by the public document verification endpoint.
type PublicDocumentVerificationResponse struct {
	DocumentType     string                   `json:"document_type" example:"timesheet"` // "timesheet" or "berita_acara"
	Status           string                   `json:"status" example:"valid"`            // "valid", "revoked", "invalid"
	IsValid          bool                     `json:"is_valid" example:"true"`
	Month            int                      `json:"month" example:"10"`
	Year             int                      `json:"year" example:"2026"`
	EmployeeName     string                   `json:"employee_name" example:"John Doe"`
	EmployeeBNIID    string                   `json:"employee_bni_id,omitempty" example:"12345"`
	EmployeeSignedAt *time.Time               `json:"employee_signed_at,omitempty"`
	WitnessName      string                   `json:"witness_name,omitempty" example:"Jane Smith"`
	WitnessSignedAt  *time.Time               `json:"witness_signed_at,omitempty"`
	DocumentHash     string                   `json:"document_hash" example:"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"`
	VerifiedAt       time.Time                `json:"verified_at"`
	AuditTrail       []DocumentAuditTrailItem `json:"audit_trail,omitempty"`
}
