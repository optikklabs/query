package middleware

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// keyedLimiter tracks one token bucket per key, evicting idle entries when
// the map is full. Limits are per-replica: the effective global rate is the
// configured rate multiplied by the number of running replicas.
type keyedLimiter[K comparable] struct {
	mu         sync.Mutex
	entries    map[K]*limiterEntry
	limit      rate.Limit
	burst      int
	maxEntries int
	ttl        time.Duration
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newKeyedLimiter[K comparable](requestsPerSecond float64, burst, maxEntries int, ttl time.Duration) *keyedLimiter[K] {
	return &keyedLimiter[K]{
		entries:    make(map[K]*limiterEntry),
		limit:      rate.Limit(requestsPerSecond),
		burst:      burst,
		maxEntries: maxEntries,
		ttl:        ttl,
	}
}

func (l *keyedLimiter[K]) allow(key K) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[key]
	if entry == nil {
		if len(l.entries) >= l.maxEntries {
			l.evictIdleOrOldest(now)
		}
		entry = &limiterEntry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = entry
	}
	entry.lastSeen = now
	return entry.limiter.AllowN(now, 1)
}

// evictIdleOrOldest drops idle entries; if the map is still full afterwards
// it drops the least-recently-seen entry to bound memory.
func (l *keyedLimiter[K]) evictIdleOrOldest(now time.Time) {
	var oldestKey K
	var oldest time.Time
	found := false
	for key, entry := range l.entries {
		if now.Sub(entry.lastSeen) > l.ttl {
			delete(l.entries, key)
			continue
		}
		if !found || entry.lastSeen.Before(oldest) {
			oldestKey = key
			oldest = entry.lastSeen
			found = true
		}
	}
	if len(l.entries) >= l.maxEntries && found {
		delete(l.entries, oldestKey)
	}
}
