package auth

import (
	"strings"
	"sync"
	"time"
)

// loginAttempts locks out repeated failed logins per (email, IP) to slow
// credential stuffing. Distinct from Traefik's per-IP rate limit, which
// throttles request volume regardless of outcome at the network edge.
type loginAttempts struct {
	mu      sync.Mutex
	entries map[string]attempt
}

type attempt struct {
	failures    int
	lockedUntil time.Time
	lastSeen    time.Time
}

const (
	maxLoginAttemptEntries = 10_000
	loginAttemptTTL        = 15 * time.Minute
)

func (l *loginAttempts) key(email, ip string) string { return strings.ToLower(email) + "|" + ip }

func (l *loginAttempts) allow(email, ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := l.key(email, ip)
	a, ok := l.entries[k]
	now := time.Now()
	if ok && now.Sub(a.lastSeen) > loginAttemptTTL && !now.Before(a.lockedUntil) {
		delete(l.entries, k)
		return true
	}
	return !now.Before(a.lockedUntil)
}

func (l *loginAttempts) fail(email, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	k := l.key(email, ip)
	if _, exists := l.entries[k]; !exists && len(l.entries) >= maxLoginAttemptEntries {
		l.evictIdleOrOldest(now)
	}
	a := l.entries[k]
	a.failures++
	a.lastSeen = now
	if a.failures >= 5 {
		a.lockedUntil = now.Add(time.Duration(1<<min(a.failures-5, 6)) * time.Minute)
	}
	l.entries[k] = a
}

func (l *loginAttempts) reset(email, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, l.key(email, ip))
}

func (l *loginAttempts) evictIdleOrOldest(now time.Time) {
	var oldestKey string
	var oldest time.Time
	for key, entry := range l.entries {
		if now.Sub(entry.lastSeen) > loginAttemptTTL && !now.Before(entry.lockedUntil) {
			delete(l.entries, key)
			continue
		}
		if oldestKey == "" || entry.lastSeen.Before(oldest) {
			oldestKey = key
			oldest = entry.lastSeen
		}
	}
	if len(l.entries) >= maxLoginAttemptEntries && oldestKey != "" {
		delete(l.entries, oldestKey)
	}
}
