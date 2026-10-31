package response

import "time"

// VAPIDKeyResponse carries the Web Push VAPID public application server key.
type VAPIDKeyResponse struct {
	PublicKey string `json:"public_key" example:"BEl62iUYgUivxIkv..."`
}

// PushScheduleResponse carries information about the daily push reminder schedule for the frontend.
type PushScheduleResponse struct {
	Hour           int        `json:"hour"`
	Minute         int        `json:"minute"`
	Timezone       string     `json:"timezone"`
	NextRun        *time.Time `json:"next_run"`
	HumanReadable  string     `json:"human_readable"`
	CronExpression string     `json:"cron_expression,omitempty"`
	IsEnabled      bool       `json:"is_enabled"`
}
