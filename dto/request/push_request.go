package request

// PushKeyPayload carries the browser push subscription keys.
type PushKeyPayload struct {
	P256dh string `json:"p256dh" binding:"required" example:"BCVxsG6..."`
	Auth   string `json:"auth" binding:"required" example:"5Kpqz..."`
}

// SubscribeRequest carries the Web Push subscription parameters.
type SubscribeRequest struct {
	Endpoint string         `json:"endpoint" binding:"required" example:"https://fcm.googleapis.com/fcm/send/..."`
	Keys     PushKeyPayload `json:"keys" binding:"required"`
}

// UnsubscribeRequest carries the endpoint to remove from push notifications.
type UnsubscribeRequest struct {
	Endpoint string `json:"endpoint" example:"https://fcm.googleapis.com/fcm/send/..."`
}

// AdminTestPushRequest carries parameters for an admin testing a push notification to a user.
type AdminTestPushRequest struct {
	UserID uint   `json:"user_id" example:"1"`
	Title  string `json:"title" example:"Timesheet Reminder"`
	Body   string `json:"body" example:"Waktunya isi timesheet hari ini!"`
	URL    string `json:"url" example:"/activity"`
}
