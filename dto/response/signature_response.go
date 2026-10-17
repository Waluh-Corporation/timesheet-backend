package response

import "time"

// SignatureStatusResponse represents the current signature specimen status for the authenticated user.
type SignatureStatusResponse struct {
	DocusealEnabled      bool       `json:"docuseal_enabled"`
	HasActiveSignature   bool       `json:"has_active_signature"`
	Status               string     `json:"status"`
	SignatureType        string     `json:"signature_type,omitempty"`
	DefaultSignatureText string     `json:"default_signature_text"`
	PreviewURL           string     `json:"preview_url,omitempty"`
	SignedAt             *time.Time `json:"signed_at,omitempty"`
	UpdatedAt            *time.Time `json:"updated_at,omitempty"`
}

// SignatureSessionResponse represents an initialized DocuSeal embedded signing session.
type SignatureSessionResponse struct {
	SubmissionID         int64  `json:"submission_id"`
	SubmitterID          int64  `json:"submitter_id"`
	Slug                 string `json:"slug"`
	EmbedSrc             string `json:"embed_src"`
	DefaultSignatureText string `json:"default_signature_text"`
}
