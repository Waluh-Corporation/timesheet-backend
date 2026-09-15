package response

import "time"

// VAPIDKeyResponse carries the Web Push VAPID public application server key.
type VAPIDKeyResponse struct {
	PublicKey string `json:"public_key" example:"BEl62iUYgUivxIkv..."`
}

// PushScheduleResponse carries current Web Push reminder cron schedule details for FE.
type PushScheduleResponse struct {
	CronExpression string     `json:"cron_expression" example:"0 17 * * *"`
	Timezone       string     `json:"timezone" example:"Asia/Jakarta"`
	IsEnabled      bool       `json:"is_enabled" example:"true"`
	NextRun        *time.Time `json:"next_run" example:"2026-09-15T17:00:00+07:00"`
	HumanReadable  string     `json:"human_readable" example:"Setiap hari pukul 17:00 (Asia/Jakarta)"`
}

// AdminTestPushResponse carries feedback when an admin triggers a test push notification to a user.
type AdminTestPushResponse struct {
	UserID             uint   `json:"user_id" example:"1"`
	Username           string `json:"username" example:"johndoe"`
	SubscriptionsCount int64  `json:"subscriptions_count" example:"2"`
	Message            string `json:"message" example:"test notification dispatched"`
}
