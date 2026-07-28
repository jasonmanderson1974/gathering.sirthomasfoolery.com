package utils

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterAllowsUpToLimitThenDenies(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	for i := 1; i <= 3; i++ {
		if !rl.Allow("k", 3, time.Minute) {
			t.Fatalf("hit %d should be allowed (limit 3)", i)
		}
	}
	if rl.Allow("k", 3, time.Minute) {
		t.Fatalf("4th hit should be denied (limit 3)")
	}
}

func TestRateLimiterKeysAreIndependent(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	if !rl.Allow("a", 1, time.Minute) {
		t.Fatalf("first hit for a should be allowed")
	}
	if !rl.Allow("b", 1, time.Minute) {
		t.Fatalf("first hit for b should be allowed (independent key)")
	}
	if rl.Allow("a", 1, time.Minute) {
		t.Fatalf("second hit for a should be denied")
	}
}

func TestRateLimiterWindowExpiry(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	window := 40 * time.Millisecond
	if !rl.Allow("k", 1, window) {
		t.Fatalf("first hit should be allowed")
	}
	if rl.Allow("k", 1, window) {
		t.Fatalf("second hit within window should be denied")
	}
	time.Sleep(window + 20*time.Millisecond)
	if !rl.Allow("k", 1, window) {
		t.Fatalf("hit after the window elapsed should be allowed again")
	}
}

func TestRateLimiterDeniedHitsDoNotExtendWindow(t *testing.T) {
	// A denied hit must not be recorded, otherwise repeated attempts would keep
	// pushing the window forward and never recover.
	rl := NewRateLimiter()
	defer rl.Stop()
	window := 40 * time.Millisecond
	rl.Allow("k", 1, window) // consume the single slot
	rl.Allow("k", 1, window) // denied — must not be recorded
	rl.Allow("k", 1, window) // denied — must not be recorded
	time.Sleep(window + 20*time.Millisecond)
	if !rl.Allow("k", 1, window) {
		t.Fatalf("denied hits should not extend the window; should be allowed after expiry")
	}
}

func TestRateLimiterConcurrentSafe(t *testing.T) {
	// Race-detector smoke test: many goroutines hammering the same key must not
	// exceed the limit and must not race.
	rl := NewRateLimiter()
	defer rl.Stop()
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow("shared", 10, time.Minute) {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != 10 {
		t.Fatalf("expected exactly 10 allowed out of 100 concurrent, got %d", allowed)
	}
}

func TestRateLimiterStopHaltsTheJanitor(t *testing.T) {
	// The janitor used to loop over an unstoppable ticker, so every limiter
	// built outside the process-lifetime singleton leaked a goroutine.
	before := runtime.NumGoroutine()

	rl := NewRateLimiter()
	rl.Stop()
	rl.Stop() // must be safe to call twice

	// The janitor returns on the closed channel; give the scheduler a moment.
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > before {
		t.Fatalf("janitor still running after Stop: %d goroutines, started from %d", got, before)
	}
}

func TestRateLimiterEvictsStaleKeys(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	rl.Allow("k", 5, time.Minute)

	// Nothing is stale within the real window.
	rl.evictStale(staleAfter)
	rl.mu.Lock()
	kept := len(rl.hits)
	rl.mu.Unlock()
	if kept != 1 {
		t.Fatalf("fresh key should survive the sweep, got %d keys", kept)
	}

	// With a zero age every recorded hit is older than the cutoff.
	rl.evictStale(0)
	rl.mu.Lock()
	remaining := len(rl.hits)
	rl.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("stale key should be evicted, got %d keys", remaining)
	}
}
