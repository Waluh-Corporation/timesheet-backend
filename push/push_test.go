package push

import (
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"timesheet-backend/config"
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
