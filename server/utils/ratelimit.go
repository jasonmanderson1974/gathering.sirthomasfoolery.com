package utils

import (
	"sync"
	"time"
)

// RateLimiter is a simple in-memory sliding-window rate limiter, safe for
// concurrent use. It is intended for a single-instance server: state is not
// shared across processes and resets on restart (acceptable for throttling
// abuse like OTP spam / enumeration).
type RateLimiter struct {
	mu       sync.Mutex
	hits     map[string][]time.Time
	stop     chan struct{}
	stopOnce sync.Once
}

const (
	// janitorInterval is how often stale keys are swept.
	janitorInterval = 10 * time.Minute
	// staleAfter is how long a key must sit untouched before it is evicted.
	staleAfter = time.Hour
)

// NewRateLimiter returns a ready limiter with a background janitor that evicts
// stale keys so the map doesn't grow unbounded under many distinct keys. Call
// Stop when the limiter is not a process-lifetime singleton.
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{hits: make(map[string][]time.Time), stop: make(chan struct{})}
	go rl.janitor()
	return rl
}

// Stop halts the background janitor and releases its ticker. The singleton in
// routes/auth.go lives as long as the process and never needs this, but any
// limiter built per-test or per-request would otherwise leak a goroutine and a
// ticker for the rest of the run. Safe to call more than once.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stop) })
}

// Allow records a hit for key and reports whether it is within `limit` events
// per `window`. When the limit is already reached, the hit is NOT recorded and
// false is returned.
func (rl *RateLimiter) Allow(key string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-window)
	// Compact in place, dropping timestamps older than the window.
	kept := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, time.Now())
	return true
}

func (rl *RateLimiter) janitor() {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			rl.evictStale(staleAfter)
		}
	}
}

// evictStale drops every key whose most recent hit is older than age. Split out
// of janitor so it can be exercised without waiting on the ticker.
func (rl *RateLimiter) evictStale(age time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-age)
	for key, times := range rl.hits {
		stale := true
		for _, t := range times {
			if t.After(cutoff) {
				stale = false
				break
			}
		}
		if stale {
			delete(rl.hits, key)
		}
	}
}
