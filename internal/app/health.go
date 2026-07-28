package app

import (
	"context"
	"sync"
	"time"
)

const healthCacheTTL = 5 * time.Second

type healthResult struct {
	ready           bool
	mysqlReady      bool
	clickhouseReady bool
	expiresAt       time.Time
}

type healthCache struct {
	mu       sync.Mutex
	current  *healthResult
	inFlight bool
	cond     *sync.Cond
}

func newHealthCache() *healthCache {
	hc := &healthCache{}
	hc.cond = sync.NewCond(&hc.mu)
	return hc
}

func (h *healthCache) get(ctx context.Context, probe func(ctx context.Context) *healthResult) *healthResult {
	h.mu.Lock()
	if h.current != nil && time.Now().Before(h.current.expiresAt) {
		res := h.current
		h.mu.Unlock()
		return res
	}
	for h.inFlight {
		h.cond.Wait()
		if h.current != nil && time.Now().Before(h.current.expiresAt) {
			res := h.current
			h.mu.Unlock()
			return res
		}
	}
	h.inFlight = true
	h.mu.Unlock()

	res := probe(ctx)
	res.expiresAt = time.Now().Add(healthCacheTTL)

	h.mu.Lock()
	h.current = res
	h.inFlight = false
	h.cond.Broadcast()
	h.mu.Unlock()
	return res
}
