package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// These in-memory limiters/dedupers are process-local. They are structured behind
// small types so they can later be swapped for a Redis-backed implementation (for
// multi-instance deployments) without touching the middleware wiring.

const (
	rlMaxBuckets = 10000           // cap distinct client entries before pruning
	rlIdleTTL    = 10 * time.Minute // evict client buckets idle longer than this
)

// RateLimiter is a per-key token-bucket limiter.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens refilled per second
	capacity float64 // max burst
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewRateLimiter allows up to burst requests instantaneously, refilling at rps/second.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{buckets: make(map[string]*bucket), rate: rps, capacity: float64(burst)}
}

// Allow reports whether a request for key may proceed, consuming a token if so.
func (rl *RateLimiter) Allow(key string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[key]
	if !ok {
		if len(rl.buckets) >= rlMaxBuckets {
			rl.pruneLocked(now)
		}
		b = &bucket{tokens: rl.capacity, last: now}
		rl.buckets[key] = b
	}

	b.tokens += now.Sub(b.last).Seconds() * rl.rate
	if b.tokens > rl.capacity {
		b.tokens = rl.capacity
	}
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func (rl *RateLimiter) pruneLocked(now time.Time) {
	for k, b := range rl.buckets {
		if now.Sub(b.last) > rlIdleTTL {
			delete(rl.buckets, k)
		}
	}
}

// RateLimitMiddleware limits requests per client IP.
func RateLimitMiddleware(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.Allow(c.ClientIP()) {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// Deduper tracks recently-seen keys with a TTL, for idempotency.
type Deduper struct {
	mu   sync.Mutex
	seen map[string]time.Time
	ttl  time.Duration
}

// NewDeduper creates a deduper that treats a key as duplicate within ttl.
func NewDeduper(ttl time.Duration) *Deduper {
	return &Deduper{seen: make(map[string]time.Time), ttl: ttl}
}

// Seen records key and reports whether it was already seen (and unexpired).
func (d *Deduper) Seen(key string) bool {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()

	if ts, ok := d.seen[key]; ok && now.Sub(ts) < d.ttl {
		return true
	}
	// Opportunistically prune expired entries.
	if len(d.seen) > rlMaxBuckets {
		for k, ts := range d.seen {
			if now.Sub(ts) >= d.ttl {
				delete(d.seen, k)
			}
		}
	}
	d.seen[key] = now
	return false
}

// WebhookDedupMiddleware short-circuits duplicate GitHub deliveries (retries) using
// the X-GitHub-Delivery header, so the same PR event is not enqueued twice.
func WebhookDedupMiddleware(d *Deduper) gin.HandlerFunc {
	return func(c *gin.Context) {
		if id := c.GetHeader("X-GitHub-Delivery"); id != "" && d.Seen(id) {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": "duplicate delivery ignored"})
			return
		}
		c.Next()
	}
}
