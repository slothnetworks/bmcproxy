// Command bmcproxy is the stateless BMC abstraction proxy daemon.
//
//	@title			bmcproxy API
//	@version		1.0
//	@description	bmcproxy exposes one vendor-neutral HTTP API over heterogeneous BMCs (Redfish, AMI MegaRAC /api, IPMI) and picks the right southbound protocol per board and firmware.
//	@description
//	@description	## Request model (no stored secrets)
//	@description	Every BMC request names its target and carries the BMC's own credentials:
//	@description	- `X-BMC-Host: <ip|host[:port]>` (required) selects the BMC.
//	@description	- `Authorization: Basic base64(user:pass)` is the BMC's own account, passed through for the life of the request (never logged, never stored; sessions are cached in memory only, keyed by host+user+hash(password)).
//	@description	- `X-BMC-Vendor: <driver>` (optional) forces a driver instead of auto-detection.
//	@description	- `X-Request-Id` (optional) is echoed back; one is generated otherwise.
//	@description
//	@description	## Tiers
//	@description	- `read`: never changes BMC state.
//	@description	- `write`: reversible (LED, boot override, power, time).
//	@description	- `disruptive`: may cut connectivity or lose data (network, BMC reset, firmware, log clear). Tiers are informational: the proxy does not gate on them; the calling management layer is responsible for authorising disruptive operations.
//	@description
//	@description	## Errors
//	@description	Errors use the envelope `{"error":{"status","class","message","request_id","debug"?}}` (debug block only in debug mode).
//	@description	| Status | Class | Meaning |
//	@description	|---|---|---|
//	@description	| 400 | invalid_argument | malformed request |
//	@description	| 401 | auth | BMC rejected (or request lacks) credentials |
//	@description	| 403 | forbidden | proxy policy (e.g. denylisted raw path) |
//	@description	| 404 | not_found | object or route not found |
//	@description	| 501 | unsupported | capability not supported by this BMC/driver |
//	@description	| 502 | upstream | BMC error, with vendor detail |
//	@description	| 503 | busy | BMC busy, connection-capped or circuit open |
//	@description	| 504 | timeout | BMC did not answer in time |
//	@BasePath					/
//	@securityDefinitions.basic	BMCBasicAuth
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/api/v1"
	"github.com/slothnetworks/bmcproxy/internal/api/vendorroutes"
	"github.com/slothnetworks/bmcproxy/internal/apidocs"
	"github.com/slothnetworks/bmcproxy/internal/cache"
	"github.com/slothnetworks/bmcproxy/internal/health"
	"github.com/slothnetworks/bmcproxy/internal/metrics"
	"github.com/slothnetworks/bmcproxy/internal/server"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// version is set with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bmcproxy:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		o                obs.Options
		listen           = flag.String("listen", server.DefaultAddr, "listen address (keep it on localhost: there is no proxy API auth yet)")
		debugEndpoints   = flag.Bool("debug-endpoints", false, "serve /debug/* (pprof, expvar, levels, targets, sessions, capabilities, stats) to loopback clients and add debug blocks to error envelopes")
		swagger          = flag.Bool("swagger", true, "serve the API docs: Swagger UI at /swagger/, OpenAPI 3 at /openapi.json, Swagger 2 at /swagger/doc.json")
		sessionTTL       = flag.Duration("session-ttl", 5*time.Minute, "idle TTL of cached BMC sessions (keep below the BMC's own session timeout)")
		sessionMaxAge    = flag.Duration("session-max-age", 0, "maximum session age regardless of use (0 = unlimited)")
		targetTTL        = flag.Duration("target-ttl", 10*time.Minute, "how long a detected driver per host is trusted before re-detection")
		cacheTTL         = flag.Duration("cache-ttl", 2*time.Second, "TTL of cached read results (0 = coalesce only)")
		globalPool       = flag.Int("global-concurrency", 64, "fleet-wide cap on in-flight southbound requests")
		retries          = flag.Int("retry-attempts", 3, "max southbound attempts for retryable failures")
		attemptTimeout   = flag.Duration("attempt-timeout", 60*time.Second, "per-attempt southbound timeout")
		breakerThreshold = flag.Int("breaker-threshold", 5, "consecutive failures that open a host's circuit breaker")
		breakerCooldown  = flag.Duration("breaker-cooldown", 30*time.Second, "how long an open breaker fails fast before half-opening")
		powerPoll        = flag.Duration("power-poll-interval", 2*time.Second, "interval between power-state polls after a power action")
		powerTimeout     = flag.Duration("power-poll-timeout", 60*time.Second, "default time to wait for a power action to converge")
		skewWarn         = flag.Duration("skew-warn", 2*time.Minute, "clock skew above which status reports a warning")
		shutdownTimeout  = flag.Duration("shutdown-timeout", 30*time.Second, "time allowed to drain requests on SIGTERM before sessions are logged out")
		showVersion      = flag.Bool("version", false, "print the version and exit")
	)
	o.RegisterFlags(flag.CommandLine)
	flag.Parse()
	if *showVersion {
		fmt.Println("bmcproxy", version)
		return nil
	}
	o.ApplyEnv()
	ob, err := obs.New(o)
	if err != nil {
		return err
	}
	obs.SetDefault(ob)
	defer ob.Close()
	log := ob.Logger(obs.SubCore)

	met := metrics.New(metrics.Sources{})
	tm := transport.NewManager(transport.Config{
		Obs:               ob,
		GlobalConcurrency: *globalPool,
		AttemptTimeout:    *attemptTimeout,
		Retry:             transport.RetryPolicy{MaxAttempts: *retries},
		Breaker:           transport.BreakerConfig{FailureThreshold: *breakerThreshold, OpenTimeout: *breakerCooldown},
		Hooks:             met.Hooks(),
	})
	defer tm.Close()
	transport.SetDefault(tm) // drivers registered via init() pick this up
	met.SetTransport(tm)

	sm := sessions.New(sessions.Config{
		Registry:  bmc.DefaultRegistry,
		IdleTTL:   *sessionTTL,
		MaxAge:    *sessionMaxAge,
		TargetTTL: *targetTTL,
		Stats:     tm,
		Obs:       ob,
	})
	rc := cache.New(cache.Config{TTL: *cacheTTL, Obs: ob})
	met.SetSources(metrics.Sources{Sessions: sm, Cache: rc})

	srv := server.New(server.Config{
		Addr:           *listen,
		DebugEndpoints: *debugEndpoints,
		Obs:            ob,
		Sessions:       sm,
		Cache:          rc,
		Stats:          tm,
		Observer:       met,
		Version:        version,
	})
	v1.Register(srv, v1.Config{PowerPollInterval: *powerPoll, PowerPollTimeout: *powerTimeout, SkewWarn: *skewWarn})
	n := vendorroutes.Mount(srv, nil)
	health.Register(srv, health.Config{SkewWarn: *skewWarn})
	met.Mount(srv.Mux())
	if *swagger {
		server.MountAPIDocs(srv, apidocs.OpenAPI, apidocs.Swagger)
	}

	var names []string
	for _, d := range bmc.Drivers() {
		names = append(names, d.Name())
	}
	if len(names) == 0 {
		log.Warn("no BMC drivers compiled in (see cmd/bmcproxy/drivers.go); every BMC request will return 501")
	}
	log.Info("starting bmcproxy", "version", version, "drivers", names, "vendor_routes", n,
		"session_ttl", *sessionTTL, "cache_ttl", *cacheTTL, "global_concurrency", *globalPool)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()

	select {
	case err := <-errc:
		if err != nil {
			_ = sm.Shutdown(context.Background())
			return err
		}
		return nil
	case <-ctx.Done():
	}
	stop()
	log.Info("signal received; shutting down", "timeout", *shutdownTimeout)
	sctx, cancel := context.WithTimeout(context.Background(), *shutdownTimeout)
	defer cancel()
	err = srv.Shutdown(sctx)
	if serr := <-errc; serr != nil {
		err = errors.Join(err, serr)
	}
	if err != nil {
		log.Warn("shutdown finished with errors", "err", err)
	} else {
		log.Info("shutdown complete")
	}
	return err
}
