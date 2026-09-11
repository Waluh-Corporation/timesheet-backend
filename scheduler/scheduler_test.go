package scheduler

import (
	"testing"
	"time"
)

func TestScheduler_NewAndStop(t *testing.T) {
	// Valid timezone
	s := New(nil, nil, "Asia/Jakarta")
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if s.loc.String() != "Asia/Jakarta" {
		t.Errorf("expected timezone Asia/Jakarta, got %s", s.loc.String())
	}

	// Invalid timezone fallback to UTC
	sUTC := New(nil, nil, "Invalid/Timezone_Name")
	if sUTC.loc != time.UTC {
		t.Errorf("expected fallback to UTC, got %v", sUTC.loc)
	}

	// Start and Stop
	s.Start()
	s.Stop()
}
