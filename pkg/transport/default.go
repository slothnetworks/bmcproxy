package transport

import "sync"

var (
	defaultMu  sync.Mutex
	defaultMgr *Manager
)

// SetDefault installs the process-wide Manager. cmd/bmcproxy calls it once at
// startup, before serving, so drivers registered via init() (which cannot be
// handed a Manager) share the same limiter, breaker, pool, stats and metrics
// hooks as the rest of the proxy.
func SetDefault(m *Manager) {
	defaultMu.Lock()
	defaultMgr = m
	defaultMu.Unlock()
}

// Default returns the process-wide Manager. If SetDefault was never called
// (tests, ad-hoc tools) a Manager with zero Config is created lazily and
// installed, so every caller in the process still shares one Manager.
func Default() *Manager {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultMgr == nil {
		defaultMgr = NewManager(Config{})
	}
	return defaultMgr
}
