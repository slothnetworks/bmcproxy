// Package v1 implements the generic, vendor-neutral /api/v1 routes.
//
// Every handler follows one pattern: resolve the request's session (via the
// server's session cache) -> type-assert the capability interface -> call ->
// JSON. A capability the session does not implement (or that SessionInfo
// marks unsupported, or that the driver reports as ErrUnsupported) is a 501.
// Reads go through internal/cache (singleflight + short TTL); writes
// invalidate the host's cached reads (done by the server's route wrapper).
//
// Handlers carry swag annotations; `go generate ./internal/apidocs` rebuilds
// the OpenAPI document from them.
package v1

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/server"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// Config tunes the generic handlers.
type Config struct {
	// PowerPollInterval is the delay between PowerState polls after a power
	// action. Default 2s.
	PowerPollInterval time.Duration
	// PowerPollTimeout is the default time to wait for the expected power
	// state. Default 60s. A request may override with timeoutSeconds.
	PowerPollTimeout time.Duration
	// PowerPollMax caps a requested timeout. Default 10m.
	PowerPollMax time.Duration
	// SkewWarn is the |clock offset| above which a warning is reported.
	// Default 2m.
	SkewWarn time.Duration
}

func (c *Config) defaults() {
	if c.PowerPollInterval <= 0 {
		c.PowerPollInterval = 2 * time.Second
	}
	if c.PowerPollTimeout <= 0 {
		c.PowerPollTimeout = 60 * time.Second
	}
	if c.PowerPollMax <= 0 {
		c.PowerPollMax = 10 * time.Minute
	}
	if c.SkewWarn <= 0 {
		c.SkewWarn = 2 * time.Minute
	}
}

type api struct {
	s   *server.Server
	cfg Config
}

const pkg = "api.v1"

// defTTL selects the cache's default TTL.
const defTTL = -1

// need type-asserts sess to capability interface T, honouring
// SessionInfo.Capabilities when the driver populates it.
func need[T any](sess bmc.Session, c bmc.Capability, op string) (T, error) {
	var zero T
	v, ok := sess.(T)
	if !ok {
		return zero, bmc.Unsupported(pkg, op)
	}
	if caps := sess.Info().Capabilities; caps != nil && caps[c] == "" {
		return zero, bmc.Unsupported(pkg, op)
	}
	return v, nil
}

// Register mounts every generic route on s.
func Register(s *server.Server, cfg Config) {
	cfg.defaults()
	a := &api{s: s, cfg: cfg}
	for _, rt := range a.routes() {
		rt.NeedsTarget = true
		rt.Group = "generic"
		s.MustHandle(rt)
	}
}

// cachedRead runs a coalesced, cached read against capability T.
func cachedRead[T any](a *api, r *http.Request, c bmc.Capability, op, params string, ttl time.Duration,
	fn func(ctx context.Context, cap T) (any, error)) (any, error) {
	return a.s.Read(r, op, params, ttl, func(ctx context.Context, sess bmc.Session) (any, error) {
		cp, err := need[T](sess, c, op)
		if err != nil {
			return nil, err
		}
		return fn(ctx, cp)
	})
}

// direct runs an uncached operation (writes, tokens) against capability T.
func direct[T any](a *api, r *http.Request, c bmc.Capability, op string,
	fn func(ctx context.Context, cap T, h *sessions.Handle) (any, error)) (any, error) {
	return a.s.Call(r, func(ctx context.Context, h *sessions.Handle) (any, error) {
		cp, err := need[T](h.Session, c, op)
		if err != nil {
			return nil, err
		}
		return fn(ctx, cp, h)
	})
}

// routes is the generic route table (PLAN "Generic routes") with tiers.
func (a *api) routes() []server.Route {
	R, W, D := bmc.TierRead, bmc.TierWrite, bmc.TierDisruptive
	return []server.Route{
		{Method: "GET", Path: "/api/v1/capabilities", Tier: R, Summary: "supported operations, backend per capability, vendor routes", Handler: a.getCapabilities},
		{Method: "GET", Path: "/api/v1/system", Tier: R, Summary: "system identity", Handler: a.getSystem},
		{Method: "GET", Path: "/api/v1/power", Tier: R, Summary: "host power state", Handler: a.getPower},
		{Method: "POST", Path: "/api/v1/power", Tier: W, Summary: "power action; polls until the state converges", Handler: a.postPower},
		{Method: "GET", Path: "/api/v1/power/restore-policy", Tier: R, Summary: "AC power-loss restore policy", Handler: a.getRestorePolicy},
		{Method: "PUT", Path: "/api/v1/power/restore-policy", Tier: W, Summary: "set restore policy", Handler: a.putRestorePolicy},
		{Method: "GET", Path: "/api/v1/led", Tier: R, Summary: "identify LED state", Handler: a.getLED},
		{Method: "POST", Path: "/api/v1/led", Tier: W, Summary: "set identify LED", Handler: a.postLED},
		{Method: "GET", Path: "/api/v1/boot", Tier: R, Summary: "boot override", Handler: a.getBoot},
		{Method: "PUT", Path: "/api/v1/boot", Tier: W, Summary: "set boot override", Handler: a.putBoot},
		{Method: "GET", Path: "/api/v1/sensors", Tier: R, Summary: "sensor readings", Handler: a.getSensors},
		{Method: "GET", Path: "/api/v1/events", Tier: R, Summary: "system event log (incremental)", Handler: a.getEvents},
		{Method: "DELETE", Path: "/api/v1/events", Tier: D, Summary: "clear the system event log", Handler: a.deleteEvents},
		{Method: "GET", Path: "/api/v1/logs/audit", Tier: R, Summary: "audit log", Handler: a.getAudit},
		{Method: "GET", Path: "/api/v1/fru", Tier: R, Summary: "FRU devices", Handler: a.getFRU},
		{Method: "GET", Path: "/api/v1/inventory/{kind}", Tier: R, Summary: "inventory by kind", Handler: a.getInventory},
		{Method: "GET", Path: "/api/v1/network", Tier: R, Summary: "BMC network interfaces", Handler: a.getNetwork},
		{Method: "PUT", Path: "/api/v1/network", Tier: D, Summary: "update a BMC network interface", Handler: a.putNetwork},
		{Method: "GET", Path: "/api/v1/network/bond", Tier: R, Summary: "BMC NIC bonding", Handler: a.getBond},
		{Method: "PUT", Path: "/api/v1/network/bond", Tier: D, Summary: "set BMC NIC bonding", Handler: a.putBond},
		{Method: "GET", Path: "/api/v1/users", Tier: R, Summary: "BMC accounts", Handler: a.getUsers},
		{Method: "GET", Path: "/api/v1/services", Tier: R, Summary: "BMC network services", Handler: a.getServices},
		{Method: "GET", Path: "/api/v1/time", Tier: R, Summary: "BMC clock and NTP", Handler: a.getTime},
		{Method: "PUT", Path: "/api/v1/time", Tier: W, Summary: "set clock and/or NTP", Handler: a.putTime},
		{Method: "POST", Path: "/api/v1/time/sync", Tier: W, Summary: "set the BMC clock to the proxy's UTC", Handler: a.postTimeSync},
		{Method: "GET", Path: "/api/v1/firmware", Tier: R, Summary: "firmware inventory", Handler: a.getFirmware},
		{Method: "POST", Path: "/api/v1/firmware/update", Tier: D, MaxBody: -1, Summary: "start a firmware update", Handler: a.postFirmwareUpdate},
		{Method: "GET", Path: "/api/v1/firmware/tasks/{id}", Tier: R, Summary: "firmware update task", Handler: a.getFirmwareTask},
		{Method: "GET", Path: "/api/v1/virtual-media", Tier: R, Summary: "virtual media slots", Handler: a.getVirtualMedia},
		{Method: "POST", Path: "/api/v1/virtual-media/{id}/insert", Tier: W, Summary: "insert virtual media", Handler: a.postInsertMedia},
		{Method: "POST", Path: "/api/v1/virtual-media/{id}/eject", Tier: W, Summary: "eject virtual media", Handler: a.postEjectMedia},
		{Method: "GET", Path: "/api/v1/kvm", Tier: R, Summary: "KVM launch info", Handler: a.getKVM},
		{Method: "POST", Path: "/api/v1/bmc/reset", Tier: D, Summary: "reset the BMC", Handler: a.postBMCReset},
	}
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

// SkewWarning returns a warning when |off| exceeds limit.
func SkewWarning(off, limit time.Duration) string {
	abs := time.Duration(math.Abs(float64(off)))
	if limit <= 0 || abs <= limit {
		return ""
	}
	dir := "ahead of"
	if off < 0 {
		dir = "behind"
	}
	return fmt.Sprintf("BMC clock is %s %s the proxy; correct it with POST /api/v1/time/sync or configure NTP via PUT /api/v1/time",
		abs.Round(time.Second), dir)
}

func (a *api) hostOffset(r *http.Request) (time.Duration, bool) {
	t, ok := server.TargetFrom(r.Context())
	if !ok {
		return 0, false
	}
	return a.s.Sessions().Targets().ClockOffset(t.Key())
}

func (a *api) recordOffset(r *http.Request, tc bmc.TimeConfig) {
	t, ok := server.TargetFrom(r.Context())
	if !ok {
		return
	}
	if off, ok := sessions.OffsetFromTime(tc, time.Now()); ok {
		a.s.Sessions().Targets().SetClockOffset(t.Key(), off)
	}
}

func queryLimit(r *http.Request) (int, error) {
	ls := r.URL.Query().Get("limit")
	if ls == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(ls)
	if err != nil || n < 0 {
		return 0, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "query", "limit must be a non-negative integer")
	}
	return n, nil
}
