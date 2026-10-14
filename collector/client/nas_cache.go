package client

import (
	"github.com/go-kit/log/level"
	"sync"
	"time"
)

// Cache errors too: a failed refresh must not turn repeated scrapes into a retry
// storm or serve a previous successful response as current data.
type responseCache struct {
	mu      sync.Mutex
	expires time.Time
	data    string
	err     error
}

func (s *responseCache) get(ttl time.Duration, now func() time.Time, fetch func() (string, error)) (string, error, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now().Before(s.expires) {
		return s.data, s.err, true
	}
	s.data, s.err = fetch()
	if s.err != nil {
		s.data = ""
	}
	s.expires = now().Add(ttl)
	return s.data, s.err, false
}

func (c *Client) cachedNASStatus(fetch func() (string, error)) (string, error) {
	ttl := c.nasStatusTTL
	if ttl == 0 {
		ttl = time.Minute
	}
	data, err, hit := c.nasStatus.get(ttl, time.Now, func() (string, error) {
		level.Info(c.logger).Log("msg", "requesting PowerStore REST API", "ip", c.IP, "resource", "nas_server", "purpose", "NAS status cache refresh")
		return fetch()
	})
	if hit {
		level.Debug(c.logger).Log("msg", "reading local REST status cache", "ip", c.IP, "resource", "nas_server", "cached_error", err != nil)
	}
	return data, err
}
