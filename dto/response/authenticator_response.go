package response

import "time"

// AuthenticatorSyncResponse describes the result of synchronizing authenticators from the community registry.
type AuthenticatorSyncResponse struct {
	TotalSynced int       `json:"total_synced" example:"56"`
	SyncedAt    time.Time `json:"synced_at" example:"2026-09-20T10:30:00Z"`
}

// AuthenticatorItemResponse represents a registered authenticator catalog entry.
type AuthenticatorItemResponse struct {
	AAGUID    string    `json:"aaguid" example:"42a048a9-4b68-45a8-aa5a-cfb3d4a462ec"`
	Name      string    `json:"name" example:"Bitwarden"`
	Icon      string    `json:"icon,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AuthenticatorListResponse represents a paginated list of authenticators.
type AuthenticatorListResponse struct {
	Authenticators []AuthenticatorItemResponse `json:"authenticators"`
	Total          int64                       `json:"total" example:"56"`
	Page           int                         `json:"page" example:"1"`
	Limit          int                         `json:"limit" example:"50"`
}
