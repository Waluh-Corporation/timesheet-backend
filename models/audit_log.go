package models

import (
	"time"

	"gorm.io/datatypes"
)

// AuditLog records sensitive administrative and data access operations for compliance.
type AuditLog struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	ActorID       *uint          `gorm:"index" json:"actor_id,omitempty"`
	ActorUsername string         `gorm:"size:64;not null;default:''" json:"actor_username"`
	Action        string         `gorm:"size:64;not null;index" json:"action"`
	ResourceType  string         `gorm:"size:64;not null;index:idx_audit_resource,priority:1" json:"resource_type"`
	ResourceID    string         `gorm:"size:64;not null;default:'';index:idx_audit_resource,priority:2" json:"resource_id"`
	IPAddress     string         `gorm:"size:45;not null;default:''" json:"ip_address"`
	UserAgent     string         `gorm:"type:text;not null;default:''" json:"user_agent"`
	Details       datatypes.JSON `json:"details,omitempty"`

	Actor *User `gorm:"foreignKey:ActorID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"actor,omitempty"`
}

// TableName returns the table name for the AuditLog model.
func (AuditLog) TableName() string {
	return "audit_logs"
}
