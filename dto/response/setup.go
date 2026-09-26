package response

// SetupStatusResponse describes the system initialization state.
type SetupStatusResponse struct {
	IsNew         string `json:"is_new" example:"Y"`
	IsInitialized bool   `json:"is_initialized"`
	RequiresSetup bool   `json:"requires_setup"`
	AdminCount    int64  `json:"admin_count"`
}
