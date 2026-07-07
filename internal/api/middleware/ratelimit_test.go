package middleware

import (
	"testing"
	"time"
)

func TestRateLimiterBurst(t *testing.T) {
	// Low refill so fast successive calls don't meaningfully refill mid-test.
	rl := NewRateLimiter(1, 3)

	allowed := 0
	for i := 0; i < 5; i++ {
		if rl.Allow("1.2.3.4") {
			allowed++
		}
	}
	if allowed != 3 {
		t.Errorf("allowed %d of 5, want 3 (burst)", allowed)
	}

	// A different key has its own bucket.
	if !rl.Allow("5.6.7.8") {
		t.Error("distinct key should be allowed")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(1000, 1) // refills quickly
	if !rl.Allow("k") {
		t.Fatal("first call should be allowed")
	}
	if rl.Allow("k") {
		t.Fatal("second immediate call should be denied")
	}
	time.Sleep(5 * time.Millisecond) // ~5 tokens refilled at 1000/s
	if !rl.Allow("k") {
		t.Error("call after refill should be allowed")
	}
}

func TestDeduper(t *testing.T) {
	d := NewDeduper(time.Minute)
	if d.Seen("delivery-1") {
		t.Error("first sighting should not be a duplicate")
	}
	if !d.Seen("delivery-1") {
		t.Error("second sighting should be a duplicate")
	}
	if d.Seen("delivery-2") {
		t.Error("different key should not be a duplicate")
	}
}

func TestDeduperExpiry(t *testing.T) {
	d := NewDeduper(2 * time.Millisecond)
	d.Seen("x")
	time.Sleep(5 * time.Millisecond)
	if d.Seen("x") {
		t.Error("entry should have expired and not count as duplicate")
	}
}
