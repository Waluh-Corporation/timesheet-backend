package push

import (
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/config"
	"timesheet-backend/models"
)

func TestPush_NewAndPublicKey(t *testing.T) {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("GenerateVAPIDKeys failed: %v", err)
	}

	cfg := &config.Config{
		VAPIDPublicKey:  pub,
		VAPIDPrivateKey: priv,
		VAPIDSubject:    "mailto:test@example.com",
	}

	svc := New(cfg, nil)
	if svc == nil {
		t.Fatal("expected non-nil push service")
	}

	if svc.PublicKey() != pub {
		t.Errorf("expected PublicKey %s, got %s", pub, svc.PublicKey())
	}
}

func TestPush_NewGeneratesKeysWhenEmpty(t *testing.T) {
	cfg := &config.Config{}
	svc := New(cfg, nil)
	if svc == nil {
		t.Fatal("expected non-nil push service")
	}

	if svc.PublicKey() == "" {
		t.Error("expected generated VAPID public key, got empty")
	}
	if cfg.VAPIDPrivateKey == "" {
		t.Error("expected generated VAPID private key, got empty")
	}

	// SendToUser with nil db should safely return without panic
	svc.SendToUser(42, Payload{Title: "Title", Body: "Body", URL: "/test"})
}

func TestPush_SendToSubscription_Branches(t *testing.T) {
	cfg := &config.Config{
		VAPIDPublicKey:  "test-pub",
		VAPIDPrivateKey: "test-priv",
		VAPIDSubject:    "mailto:test@example.com",
	}
	svc := New(cfg, nil)

	sub := &models.PushSubscription{
		Endpoint: "http://127.0.0.1:99999/fake-endpoint",
		P256dh:   "invalid-p256dh",
		Auth:     "invalid-auth",
	}
	err := svc.SendToSubscription(sub, Payload{Title: "T", Body: "B", URL: "/u"})
	if err == nil {
		t.Error("expected error with invalid keys, got nil")
	}
}

func TestPush_SendToUserWithDB(t *testing.T) {
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skip("Postgres DB not available")
	}

	tx := db.Begin()
	defer tx.Rollback()

	svc := New(cfg, tx)

	sub := models.PushSubscription{
		UserID:   9999,
		Endpoint: "http://127.0.0.1:99999/fake-endpoint",
		P256dh:   "invalid-p256dh",
		Auth:     "invalid-auth",
	}
	_ = tx.Create(&sub).Error

	svc.SendToUser(9999, Payload{Title: "Title", Body: "Body", URL: "/test"})
}
