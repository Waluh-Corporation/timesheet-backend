package push

import (
	"encoding/json"
	"log"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
	"gorm.io/gorm"

	"timesheet-backend/config"
	"timesheet-backend/models"
)

// Service wraps VAPID keys and dispatches Web Push notifications.
type Service struct {
	cfg *config.Config
	db  *gorm.DB
}

// New constructs a push Service, generating a VAPID key pair on the fly when
// none is configured. Generated keys are logged so an operator can pin them.
func New(cfg *config.Config, db *gorm.DB) *Service {
	if cfg.VAPIDPublicKey == "" || cfg.VAPIDPrivateKey == "" {
		priv, pub, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			log.Printf("[push] failed to generate VAPID keys: %v", err)
		} else {
			cfg.VAPIDPrivateKey = priv
			cfg.VAPIDPublicKey = pub
			log.Printf("[push] generated ephemeral VAPID keys; pin these to keep subscriptions valid:")
			log.Printf("[push]   VAPID_PUBLIC_KEY=%s", pub)
			log.Printf("[push]   VAPID_PRIVATE_KEY=%s", priv)
		}
	}
	return &Service{cfg: cfg, db: db}
}

// PublicKey exposes the VAPID application server key for the frontend.
func (s *Service) PublicKey() string { return s.cfg.VAPIDPublicKey }

// Payload is the JSON body delivered to the service worker's push handler.
type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

// SendToSubscription pushes a single payload to one subscription. A 404/410
// response means the subscription has expired and is pruned from the database.
func (s *Service) SendToSubscription(sub *models.PushSubscription, payload Payload) error {
	body, _ := json.Marshal(payload)
	resp, err := webpush.SendNotification(body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}, &webpush.Options{
		Subscriber:      s.cfg.VAPIDSubject,
		VAPIDPublicKey:  s.cfg.VAPIDPublicKey,
		VAPIDPrivateKey: s.cfg.VAPIDPrivateKey,
		TTL:             60,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		s.db.Delete(&models.PushSubscription{}, sub.ID)
	}
	return nil
}

// SendToUser fans a payload out to every subscription a user owns.
func (s *Service) SendToUser(userID uint, payload Payload) {
	var subs []models.PushSubscription
	if err := s.db.Where("user_id = ?", userID).Find(&subs).Error; err != nil {
		log.Printf("[push] failed to load subscriptions for user %d: %v", userID, err)
		return
	}
	for i := range subs {
		if err := s.SendToSubscription(&subs[i], payload); err != nil {
			log.Printf("[push] delivery failed for sub %d: %v", subs[i].ID, err)
		}
	}
}
