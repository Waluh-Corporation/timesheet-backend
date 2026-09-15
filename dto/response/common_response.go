package response

// APIResponse represents the standard enterprise response envelope.
type APIResponse struct {
	Code    int         `json:"code" example:"200"`
	Status  string      `json:"status" example:"success"`
	Message string      `json:"message,omitempty" example:"operation successful"`
	Data    interface{} `json:"data,omitempty"`
}

// MessageResponse represents a generic success response message with status code.
type MessageResponse struct {
	Code    int    `json:"code" example:"200"`
	Status  string `json:"status" example:"success"`
	Message string `json:"message" example:"operation successful"`
}

// ErrorResponse represents an API error response with status code.
type ErrorResponse struct {
	Code    int    `json:"code" example:"400"`
	Status  string `json:"status" example:"error"`
	Error   string `json:"error" example:"invalid request or unauthorized"`
	Message string `json:"message,omitempty" example:"invalid request or unauthorized"`
}

// DeleteResponse represents a deletion status response with status code.
type DeleteResponse struct {
	Code    int    `json:"code" example:"200"`
	Status  string `json:"status" example:"success"`
	Deleted bool   `json:"deleted" example:"true"`
}

// PaginationMeta carries pagination navigation metadata.
type PaginationMeta struct {
	Page       int   `json:"page" example:"1"`
	Limit      int   `json:"limit" example:"10"`
	TotalRows  int64 `json:"total_rows" example:"35"`
	TotalPages int   `json:"total_pages" example:"4"`
}

// PaginatedResponse wraps a paginated data payload along with navigation metadata.
type PaginatedResponse struct {
	Code       int            `json:"code" example:"200"`
	Status     string         `json:"status" example:"success"`
	Data       interface{}    `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}
