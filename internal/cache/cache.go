// Package cache coalesces concurrent identical BMC reads (singleflight) and
// keeps their results for a short TTL.
//
// Keys are (host, user-key, operation, params); the user key is an opaque
// per-credential id (sessions.Key.ID), so one user's cached result is never
// served to another credential. Errors are never cached, writes are never
// cached, and any write through a host invalidates that host's entries: a
// per-host generation counter also keeps a read that started before the
// write from repopulating the cache with stale data.
package cache

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// Key identifies a cached read.
type Key struct {
	Host   string // bmc.Target.Key()
	User   string // opaque credential id (never a password)
	Op     string // e.g. "sensors"
	Params string // canonical query params, "" if none
}

func (k Key) str(gen uint64) string {
	return k.Host + "\x00" + k.User + "\x00" + k.Op + "\x00" + k.Params + "\x00" + strconv.FormatUint(gen, 10)
}

// Config configures a Cache.
type Config struct {
	// TTL is the default result lifetime. 0 disables storing (coalescing
	// still applies). Default 2s via New's caller; a zero Config caches
	// nothing but still coalesces.
	TTL time.Duration
	// MaxEntries bounds the number of stored results (oldest-expiring
	// evicted first when full). Default 10000.
	MaxEntries int
	// Obs is the fallback Obs for logging.
	Obs *obs.Obs
	// Now is the clock. Default time.Now.
	Now func() time.Time
}

type item struct {
	val     any
	expires time.Time
	host    string
}

// Stats are cumulative cache counters.
type Stats struct {
	Hits      uint64  `json:"hits"`      // served from stored results
	Misses    uint64  `json:"misses"`    // loader executions
	Coalesced uint64  `json:"coalesced"` // callers that joined an in-flight load
	Errors    uint64  `json:"errors"`    // loader errors (not cached)
	Entries   int     `json:"entries"`
	HitRate   float64 `json:"hitRate"` // (hits+coalesced)/(hits+coalesced+misses)
	Invalid   uint64  `json:"invalidations"`
}

// Cache is a read-coalescing TTL cache. The zero value is not usable; use New.
type Cache struct {
	cfg Config
	sf  singleflight.Group

	mu    sync.Mutex
	items map[string]*item // key without generation
	gens  map[string]uint64
	st    Stats
}

// New returns a Cache.
func New(cfg Config) *Cache {
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 10000
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Obs == nil {
		cfg.Obs = obs.Default()
	}
	return &Cache{cfg: cfg, items: map[string]*item{}, gens: map[string]uint64{}}
}

// TTL returns the default TTL.
func (c *Cache) TTL() time.Duration { return c.cfg.TTL }

func (c *Cache) log(ctx context.Context) *slog.Logger {
	return obs.LoggerFrom(ctx, c.cfg.Obs, obs.SubCache)
}

// Get returns the cached value for key or runs load once for all concurrent
// callers with the same key. ttl < 0 uses the default TTL; ttl == 0 coalesces
// without storing. load runs on a context detached from any single caller's
// cancellation (it keeps values such as the logger and request id); each
// caller still returns early if its own ctx is done.
func (c *Cache) Get(ctx context.Context, key Key, ttl time.Duration, load func(ctx context.Context) (any, error)) (any, error) {
	if ttl < 0 {
		ttl = c.cfg.TTL
	}
	base := key.str(0)
	now := c.cfg.Now()
	c.mu.Lock()
	if it, ok := c.items[base]; ok {
		if now.Before(it.expires) {
			c.st.Hits++
			c.mu.Unlock()
			c.log(ctx).DebugContext(ctx, "cache hit", "host", key.Host, "op", key.Op)
			return it.val, nil
		}
		delete(c.items, base)
	}
	gen := c.gens[key.Host]
	c.mu.Unlock()

	executed := false
	ch := c.sf.DoChan(key.str(gen), func() (any, error) {
		executed = true
		c.mu.Lock()
		c.st.Misses++
		c.mu.Unlock()
		c.log(ctx).DebugContext(ctx, "cache miss", "host", key.Host, "op", key.Op)
		v, err := load(context.WithoutCancel(ctx))
		c.mu.Lock()
		defer c.mu.Unlock()
		if err != nil {
			c.st.Errors++
			return nil, err
		}
		if ttl > 0 && c.gens[key.Host] == gen {
			c.storeLocked(base, &item{val: v, expires: c.cfg.Now().Add(ttl), host: key.Host})
		}
		return v, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Shared && !executed {
			c.mu.Lock()
			c.st.Coalesced++
			c.mu.Unlock()
			c.log(ctx).DebugContext(ctx, "cache coalesced", "host", key.Host, "op", key.Op)
		}
		return res.Val, res.Err
	}
}

func (c *Cache) storeLocked(k string, it *item) {
	if len(c.items) >= c.cfg.MaxEntries {
		now := c.cfg.Now()
		var oldestK string
		var oldest time.Time
		for kk, v := range c.items {
			if !now.Before(v.expires) {
				delete(c.items, kk)
				continue
			}
			if oldestK == "" || v.expires.Before(oldest) {
				oldestK, oldest = kk, v.expires
			}
		}
		if len(c.items) >= c.cfg.MaxEntries && oldestK != "" {
			delete(c.items, oldestK)
		}
	}
	c.items[k] = it
}

// InvalidateHost drops every cached result for host and bumps its generation
// so in-flight loads started earlier are neither joined nor stored.
func (c *Cache) InvalidateHost(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gens[host]++
	c.st.Invalid++
	for k, it := range c.items {
		if it.host == host {
			delete(c.items, k)
		}
	}
}

// Purge drops expired entries (optional housekeeping).
func (c *Cache) Purge() {
	now := c.cfg.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, it := range c.items {
		if !now.Before(it.expires) {
			delete(c.items, k)
		}
	}
}

// Stats returns cumulative counters.
func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.st
	s.Entries = len(c.items)
	if tot := s.Hits + s.Coalesced + s.Misses; tot > 0 {
		s.HitRate = float64(s.Hits+s.Coalesced) / float64(tot)
	}
	return s
}
