package server

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/automatika-robotics/emos-cli/internal/config"
)

// Connectivity probes whether the Automatika API is reachable. The result is
// cached for ~30s so concurrent /recipes/remote requests don't all fire DNS
// + TCP probes
type Connectivity struct {
	target string

	mu          sync.Mutex
	online      bool
	lastChecked time.Time
	cacheFor    time.Duration
}

func NewConnectivity() *Connectivity {
	host := "support-api.automatikarobotics.com"
	if u, err := url.Parse(config.APIBaseURL); err == nil && u.Host != "" {
		host = u.Host
	}
	return &Connectivity{target: host, cacheFor: 30 * time.Second}
}

// Online returns the cached status, refreshing if stale.
func (c *Connectivity) Online(ctx context.Context) bool {
	c.mu.Lock()
	if time.Since(c.lastChecked) < c.cacheFor {
		online := c.online
		c.mu.Unlock()
		return online
	}
	c.mu.Unlock()

	online := c.probe(ctx)

	c.mu.Lock()
	c.online = online
	c.lastChecked = time.Now()
	c.mu.Unlock()
	return online
}

// Snapshot returns the cached state (online, last_checked, target) without
// triggering a probe
func (c *Connectivity) Snapshot() (online bool, lastChecked time.Time, target string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.online, c.lastChecked, c.target
}

// Invalidate clears the cache so the next Online() call re-probes
func (c *Connectivity) Invalidate() {
	c.mu.Lock()
	c.lastChecked = time.Time{}
	c.mu.Unlock()
}

func (c *Connectivity) probe(ctx context.Context) bool {
	dialCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	d := net.Dialer{}
	conn, err := d.DialContext(dialCtx, "tcp", c.target+":443")
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// HTTPClient returns an http.Client with sensible timeouts for upstream calls.
func (c *Connectivity) HTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}
