package response

// VAPIDKeyResponse carries the Web Push VAPID public application server key.
type VAPIDKeyResponse struct {
	PublicKey string `json:"public_key" example:"BEl62iUYgUivxIkv..."`
}
