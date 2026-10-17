package models

import (
	"time"

	"gorm.io/gorm"
)

// SignatureStatus enumerates the lifecycle states of a user's DocuSeal signature specimen.
type SignatureStatus string

const (
	SignatureStatusPending   SignatureStatus = "pending"
	SignatureStatusCompleted SignatureStatus = "completed"
	SignatureStatusDeclined  SignatureStatus = "declined"
	SignatureStatusRevoked   SignatureStatus = "revoked"
)

// SignatureType enumerates the method used to create the signature specimen.
type SignatureType string

const (
	SignatureTypeDrawn    SignatureType = "drawn"
	SignatureTypeTyped    SignatureType = "typed"
	SignatureTypeUploaded SignatureType = "uploaded"
)

// UserSignature persists a user's DocuSeal signature onboarding session and normalized PNG specimen.
type UserSignature struct {
	ID                   uint            `gorm:"primaryKey" json:"id"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	DeletedAt            gorm.DeletedAt  `gorm:"index" json:"-"`
	UserID               uint            `gorm:"not null;index:idx_user_signatures_user_id" json:"user_id"`
	User                 *User           `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user,omitempty"`
	DocusealSubmissionID int64           `gorm:"column:docuseal_submission_id;not null;default:0" json:"docuseal_submission_id"`
	DocusealSubmitterID  int64           `gorm:"column:docuseal_submitter_id;not null;uniqueIndex" json:"docuseal_submitter_id"`
	DocusealSlug         string          `gorm:"column:docuseal_slug;size:128;not null;index:idx_user_signatures_slug" json:"docuseal_slug"`
	SignatureType        SignatureType   `gorm:"column:signature_type;size:16;not null;default:'drawn'" json:"signature_type"`
	SignatureText        string          `gorm:"column:signature_text;size:255" json:"signature_text,omitempty"`
	Status               SignatureStatus `gorm:"column:status;size:32;not null;default:'pending'" json:"status"`
	S3SignatureKey       string          `gorm:"column:s3_signature_key;size:512" json:"s3_signature_key,omitempty"`
	S3AuditPDFKey        string          `gorm:"column:s3_audit_pdf_key;size:512" json:"s3_audit_pdf_key,omitempty"`
	SignerIP             string          `gorm:"column:signer_ip;size:45" json:"signer_ip,omitempty"`
	SignerUserAgent      string          `gorm:"column:signer_user_agent;type:text" json:"signer_user_agent,omitempty"`
	SignedAt             *time.Time      `gorm:"column:signed_at" json:"signed_at,omitempty"`
	IsActive             bool            `gorm:"column:is_active;type:boolean;default:false;not null" json:"is_active"`
}

// TableName explicitly binds UserSignature to the user_signatures database table.
func (UserSignature) TableName() string {
	return "user_signatures"
}
