// Command bmcprobe is a one-shot diagnostics CLI for a single BMC. It uses
// the same driver registry, transport (TLS profiles, retries, breaker) and
// instrumentation as the bmcproxy daemon, without running it.
//
// Usage:
//
//	bmcprobe [flags] detect
//	bmcprobe [flags] capabilities
//	bmcprobe [flags] get <op> [arg]
//
// Ops for get: power, sensors, events (--since ID, --limit N), system, fru,
// inventory <cpus|memory|pcie|storage|baseboard>, network, users, services,
// time, firmware, led, boot, restore-policy, audit. get only reads.
//
// Flags mirror the daemon: --debug, --trace-http, --har-out, --log-secrets,
// --log-level, --log-format; plus --host (required), --user, --vendor,
// --password-env NAME or --password-stdin (the password is never accepted
// as a flag value), --timeout and --json. Flags may appear before or after
// the command.
//
// detect works without credentials (it then reports only what the
// unauthenticated detection sees); with credentials it also opens a session
// and reports which backends authenticated.
//
// Exit status: 0 success, 1 operation failed, 2 usage error.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/server"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// version is set with -ldflags "-X main.version=...".
var version = "dev"

// stdin is read by --password-stdin (tests replace it).
var stdin io.Reader = os.Stdin

// registry resolves drivers (tests may replace it).
var registry = bmc.DefaultRegistry

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

const usageText = `usage: bmcprobe [flags] <command> [args]

commands:
  detect                 resolve the driver; print vendor, board, firmware, TLS
                         and (with credentials) which backends authenticate
  capabilities           open a session; print each capability's backend
                         (or 501) and the driver's vendor routes
  get <op> [arg]         call one read capability and print JSON; ops:
                         power sensors events system fru inventory <kind>
                         network users services time firmware led boot
                         restore-policy audit

flags:
`

type config struct {
	obs         obs.Options
	host        string
	user        string
	vendor      string
	passwordEnv string
	passStdin   bool
	timeout     time.Duration
	json        bool
	since       string
	limit       int
}

// refuse is a flag that rejects any value: passwords never go on the
// command line (shell history, ps).
type refuse struct{}

func (refuse) String() string { return "" }
func (refuse) Set(string) error {
	return errors.New("refusing a password on the command line: use --password-env NAME or --password-stdin")
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var cfg config
	fs := flag.NewFlagSet("bmcprobe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg.obs.RegisterFlags(fs)
	fs.StringVar(&cfg.host, "host", "", "BMC address: ip, host or host:port (required)")
	fs.StringVar(&cfg.user, "user", "", "BMC user name")
	fs.StringVar(&cfg.vendor, "vendor", "", "force a driver instead of auto-detection (e.g. asrock, redfish)")
	fs.StringVar(&cfg.passwordEnv, "password-env", "", "read the BMC password from this environment variable")
	fs.BoolVar(&cfg.passStdin, "password-stdin", false, "read the BMC password from the first line of stdin")
	fs.Var(refuse{}, "password", "not supported: use --password-env or --password-stdin")
	fs.DurationVar(&cfg.timeout, "timeout", 60*time.Second, "overall deadline")
	fs.BoolVar(&cfg.json, "json", false, "JSON output (get always prints JSON)")
	fs.StringVar(&cfg.since, "since", "", "get events: only entries after this record id")
	fs.IntVar(&cfg.limit, "limit", 0, "get events: maximum entries (0 = all)")
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprint(stderr, usageText)
		fs.PrintDefaults()
	}

	// Interleave flags and positionals: flags may follow the command.
	var pos []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	if *showVersion {
		fmt.Fprintln(stdout, "bmcprobe", version)
		return 0
	}
	if len(pos) == 0 {
		fs.Usage()
		return 2
	}

	err := execute(ctx, &cfg, pos, stdout, stderr)
	var ue usageError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ue):
		fmt.Fprintln(stderr, "bmcprobe:", ue.msg)
		return 2
	default:
		fmt.Fprintf(stderr, "bmcprobe: %s (HTTP-equivalent %d, class %s)\n", obs.RedactString(err.Error()), bmc.HTTPStatus(err), bmc.Class(err))
		return 1
	}
}

func execute(ctx context.Context, cfg *config, pos []string, stdout, stderr io.Writer) error {
	cmd := pos[0]
	switch cmd {
	case "detect", "capabilities", "caps", "get":
	default:
		return usageError{fmt.Sprintf("unknown command %q (want detect, capabilities or get)", cmd)}
	}
	if cfg.host == "" {
		return usageError{"--host is required"}
	}
	if cmd == "get" && len(pos) < 2 {
		return usageError{"get needs an op: " + strings.Join(opNames(), ", ")}
	}
	target, err := bmc.ParseTarget(cfg.host)
	if err != nil {
		return usageError{err.Error()}
	}
	target.VendorHint = cfg.vendor
	creds, haveCreds, err := readCreds(cfg)
	if err != nil {
		return err
	}
	if !haveCreds && cmd != "detect" {
		return usageError{cmd + " needs credentials: --user plus --password-env NAME or --password-stdin"}
	}

	cfg.obs.Writer = stderr
	cfg.obs.ApplyEnv()
	ob, err := obs.New(cfg.obs)
	if err != nil {
		return usageError{err.Error()}
	}
	defer ob.Close()
	ctx = obs.WithObs(ctx, ob)
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	tm := transport.NewManager(transport.Config{
		Obs:            ob,
		AttemptTimeout: cfg.timeout,
		Retry:          transport.RetryPolicy{MaxAttempts: 2},
		Breaker:        transport.BreakerConfig{Disabled: true}, // one-shot: never fail fast
	})
	defer tm.Close()
	prev := transport.Default()
	transport.SetDefault(tm) // drivers registered via init() use transport.Default()
	defer transport.SetDefault(prev)
	for _, d := range registry.Drivers() {
		if b, ok := d.(interface{ SetTransport(*transport.Manager) }); ok {
			b.SetTransport(tm)
			defer b.SetTransport(nil)
		}
	}
	if len(registry.Drivers()) == 0 {
		return errors.New("no BMC drivers compiled in (see cmd/bmcprobe/drivers.go)")
	}

	switch cmd {
	case "detect":
		return detect(ctx, cfg, tm, target, creds, haveCreds, stdout)
	case "capabilities", "caps":
		return capabilities(ctx, cfg, tm, target, creds, stdout)
	default:
		return get(ctx, cfg, target, creds, pos[1:], stdout)
	}
}

func readCreds(cfg *config) (bmc.Creds, bool, error) {
	var pass string
	switch {
	case cfg.passwordEnv != "" && cfg.passStdin:
		return bmc.Creds{}, false, usageError{"use only one of --password-env and --password-stdin"}
	case cfg.passwordEnv != "":
		v, ok := os.LookupEnv(cfg.passwordEnv)
		if !ok {
			return bmc.Creds{}, false, usageError{"environment variable " + cfg.passwordEnv + " is not set"}
		}
		pass = v
	case cfg.passStdin:
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && line == "" {
			return bmc.Creds{}, false, usageError{"--password-stdin: no password on stdin"}
		}
		pass = strings.TrimRight(line, "\r\n")
	}
	if cfg.user == "" && pass == "" {
		return bmc.Creds{}, false, nil
	}
	if cfg.user == "" || pass == "" {
		return bmc.Creds{}, false, usageError{"credentials need both --user and a password (--password-env or --password-stdin)"}
	}
	return bmc.Creds{User: cfg.user, Pass: pass}, true, nil
}

// ---- detect ----

// SessionReport is the authenticated part of detect.
type SessionReport struct {
	Authenticated      bool                `json:"authenticated"`
	Error              string              `json:"error,omitempty"`
	Board              string              `json:"board,omitempty"`
	Firmware           string              `json:"firmware,omitempty"`
	Backends           []bmc.BackendStatus `json:"backends,omitempty"`
	ClockOffsetSeconds float64             `json:"clockOffsetSeconds"`
	Degraded           string              `json:"degraded,omitempty"`
}

// DetectReport is the output of detect.
type DetectReport struct {
	Host         string             `json:"host"`
	Driver       string             `json:"driver"`
	Score        int                `json:"score"`
	Vendor       string             `json:"vendor,omitempty"`
	Manufacturer string             `json:"manufacturer,omitempty"`
	Board        string             `json:"board,omitempty"`
	Firmware     string             `json:"firmware,omitempty"`
	Backends     []string           `json:"detectedBackends,omitempty"`
	Details      map[string]string  `json:"details,omitempty"`
	TLS          *transport.TLSInfo `json:"tls,omitempty"`
	Session      *SessionReport     `json:"session,omitempty"`
}

func detect(ctx context.Context, cfg *config, tm *transport.Manager, t bmc.Target, c bmc.Creds, haveCreds bool, out io.Writer) error {
	var cp *bmc.Creds
	if haveCreds {
		cp = &c
	}
	drv, m, err := registry.Resolve(ctx, t, cp)
	if err != nil {
		return err
	}
	rep := DetectReport{Host: t.Addr(), Driver: drv.Name(), Score: m.Score, Vendor: m.Vendor, Manufacturer: m.Manufacturer,
		Board: m.Board, Firmware: m.Firmware, Backends: m.Backends, Details: m.Details}
	if hs, ok := tm.HostStats(t.Addr()); ok {
		rep.TLS = hs.TLS
	}
	if haveCreds {
		sr := &SessionReport{}
		s, err := drv.Open(ctx, t, c)
		if err != nil {
			sr.Error = obs.RedactString(err.Error())
		} else {
			info := s.Info()
			sr.Authenticated = true
			sr.Board, sr.Firmware, sr.Backends, sr.Degraded = info.Board, info.Firmware, info.Backends, info.Degraded
			sr.ClockOffsetSeconds = info.ClockOffset.Seconds()
			_ = s.Close(context.WithoutCancel(ctx))
		}
		rep.Session = sr
		if rep.TLS == nil {
			if hs, ok := tm.HostStats(t.Addr()); ok {
				rep.TLS = hs.TLS
			}
		}
	}
	if cfg.json {
		return writeJSON(out, rep)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	row := func(k, v string) {
		if v != "" {
			fmt.Fprintf(tw, "%s\t%s\n", k, v)
		}
	}
	row("host", rep.Host)
	row("driver", fmt.Sprintf("%s (score %d)", rep.Driver, rep.Score))
	row("vendor", rep.Vendor)
	row("manufacturer", rep.Manufacturer)
	row("board", rep.Board)
	row("firmware", rep.Firmware)
	row("detected backends", strings.Join(rep.Backends, ", "))
	if rep.TLS != nil {
		row("tls", fmt.Sprintf("%s %s (profile %s)", rep.TLS.Version, rep.TLS.CipherSuite, rep.TLS.Profile))
	}
	for _, k := range sortedKeys(rep.Details) {
		row("detail."+k, rep.Details[k])
	}
	if sr := rep.Session; sr != nil {
		if !sr.Authenticated {
			row("session", "FAILED: "+sr.Error)
		} else {
			row("session", "authenticated")
			row("session board", sr.Board)
			row("session firmware", sr.Firmware)
			for _, b := range sr.Backends {
				row("backend "+b.Name, backendLine(b))
			}
			row("clock offset", fmt.Sprintf("%+.0fs", sr.ClockOffsetSeconds))
			row("degraded", sr.Degraded)
		}
	} else {
		row("session", "not attempted (no credentials)")
	}
	return tw.Flush()
}

func backendLine(b bmc.BackendStatus) string {
	s := "unreachable"
	switch {
	case b.Authenticated:
		s = "authenticated"
	case b.Available:
		s = "reachable, not authenticated"
	}
	if b.Detail != "" {
		s += " (" + b.Detail + ")"
	}
	return s
}

// ---- capabilities ----

// CapabilitiesReport is the output of capabilities.
type CapabilitiesReport struct {
	Host               string                   `json:"host"`
	Driver             string                   `json:"driver"`
	Board              string                   `json:"board,omitempty"`
	Firmware           string                   `json:"firmware,omitempty"`
	Backends           []bmc.BackendStatus      `json:"backends,omitempty"`
	Degraded           string                   `json:"degraded,omitempty"`
	ClockOffsetSeconds float64                  `json:"clockOffsetSeconds"`
	Capabilities       []bmc.CapabilityReport   `json:"capabilities"`
	VendorRoutes       []server.VendorRouteInfo `json:"vendorRoutes,omitempty"`
}

func open(ctx context.Context, t bmc.Target, c bmc.Creds) (bmc.Driver, bmc.Session, error) {
	drv, _, err := registry.Resolve(ctx, t, &c)
	if err != nil {
		return nil, nil, err
	}
	s, err := drv.Open(ctx, t, c)
	if err != nil {
		return nil, nil, err
	}
	return drv, s, nil
}

func capabilities(ctx context.Context, cfg *config, tm *transport.Manager, t bmc.Target, c bmc.Creds, out io.Writer) error {
	drv, s, err := open(ctx, t, c)
	if err != nil {
		return err
	}
	defer s.Close(context.WithoutCancel(ctx))
	info := s.Info()
	rep := CapabilitiesReport{Host: t.Addr(), Driver: drv.Name(), Board: info.Board, Firmware: info.Firmware,
		Backends: info.Backends, Degraded: info.Degraded, ClockOffsetSeconds: info.ClockOffset.Seconds(),
		Capabilities: bmc.Describe(s), VendorRoutes: server.VendorRoutesOf(drv)}
	if cfg.json {
		return writeJSON(out, rep)
	}
	fmt.Fprintf(out, "host %s  driver %s  board %s  firmware %s\n", rep.Host, rep.Driver, rep.Board, rep.Firmware)
	for _, b := range rep.Backends {
		fmt.Fprintf(out, "backend %s: %s\n", b.Name, backendLine(b))
	}
	if rep.Degraded != "" {
		fmt.Fprintf(out, "degraded: %s\n", rep.Degraded)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "\nCAPABILITY\tSUPPORTED\tBACKEND")
	for _, cr := range rep.Capabilities {
		sup := "501"
		if cr.Supported {
			sup = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", cr.Capability, sup, cr.Backend)
	}
	if len(rep.VendorRoutes) > 0 {
		fmt.Fprintln(tw, "\nVENDOR ROUTE\tTIER\tSUMMARY")
		for _, vr := range rep.VendorRoutes {
			m := vr.Method
			if m == "" {
				m = "ANY"
			}
			tier := vr.Tier.String()
			if vr.Dynamic {
				tier += "*"
			}
			fmt.Fprintf(tw, "%s %s\t%s\t%s\n", m, vr.Path, tier, vr.Summary)
		}
	}
	return tw.Flush()
}

// ---- get ----

type opFunc func(ctx context.Context, s bmc.Session, cfg *config, args []string) (any, error)

// need type-asserts s to T and honours Info().Capabilities, like the
// daemon's /api/v1 handlers (a missing capability is a 501).
func need[T any](s bmc.Session, c bmc.Capability, op string) (T, error) {
	var zero T
	v, ok := s.(T)
	if !ok {
		return zero, bmc.Unsupported("bmcprobe", op)
	}
	if caps := s.Info().Capabilities; caps != nil && caps[c] == "" {
		return zero, bmc.Unsupported("bmcprobe", op)
	}
	return v, nil
}

func call[T any, R any](c bmc.Capability, op string, fn func(ctx context.Context, x T) (R, error)) opFunc {
	return func(ctx context.Context, s bmc.Session, _ *config, _ []string) (any, error) {
		x, err := need[T](s, c, op)
		if err != nil {
			return nil, err
		}
		return fn(ctx, x)
	}
}

func wrap[R any](key string, fn func() (R, error)) (any, error) {
	v, err := fn()
	if err != nil {
		return nil, err
	}
	return map[string]any{key: v}, nil
}

var ops = map[string]opFunc{
	"power": call(bmc.CapPower, "PowerState", func(ctx context.Context, x bmc.PowerController) (any, error) {
		return wrap("state", func() (bmc.PowerState, error) { return x.PowerState(ctx) })
	}),
	"sensors": call(bmc.CapSensors, "Sensors", func(ctx context.Context, x bmc.SensorReader) (any, error) {
		return wrap("sensors", func() ([]bmc.Sensor, error) { return x.Sensors(ctx) })
	}),
	"system": call(bmc.CapIdentity, "SystemInfo", func(ctx context.Context, x bmc.Identity) (bmc.SystemInfo, error) {
		return x.SystemInfo(ctx)
	}),
	"fru": call(bmc.CapFRU, "FRU", func(ctx context.Context, x bmc.FRUReader) (any, error) {
		return wrap("fru", func() ([]bmc.FRU, error) { return x.FRU(ctx) })
	}),
	"network": call(bmc.CapNetwork, "NetworkInterfaces", func(ctx context.Context, x bmc.NetworkManager) (any, error) {
		return wrap("interfaces", func() ([]bmc.NetworkInterface, error) { return x.NetworkInterfaces(ctx) })
	}),
	"users": call(bmc.CapUsers, "Users", func(ctx context.Context, x bmc.UserReader) (any, error) {
		return wrap("users", func() ([]bmc.User, error) { return x.Users(ctx) })
	}),
	"services": call(bmc.CapServices, "Services", func(ctx context.Context, x bmc.ServiceReader) (any, error) {
		return wrap("services", func() ([]bmc.Service, error) { return x.Services(ctx) })
	}),
	"time": call(bmc.CapTime, "Time", func(ctx context.Context, x bmc.TimeManager) (bmc.TimeConfig, error) {
		return x.Time(ctx)
	}),
	"firmware": call(bmc.CapFirmwareInventory, "Firmware", func(ctx context.Context, x bmc.FirmwareInventory) (any, error) {
		return wrap("components", func() ([]bmc.FirmwareComponent, error) { return x.Firmware(ctx) })
	}),
	"led": call(bmc.CapIndicatorLED, "GetIndicatorLED", func(ctx context.Context, x bmc.IndicatorLED) (any, error) {
		return wrap("state", func() (bmc.LEDState, error) { return x.GetIndicatorLED(ctx) })
	}),
	"boot": call(bmc.CapBootOverride, "GetBootOverride", func(ctx context.Context, x bmc.BootOverride) (bmc.BootOverrideConfig, error) {
		return x.GetBootOverride(ctx)
	}),
	"restore-policy": call(bmc.CapPowerRestorePolicy, "GetPowerRestorePolicy", func(ctx context.Context, x bmc.PowerRestorePolicy) (any, error) {
		return wrap("policy", func() (bmc.RestorePolicy, error) { return x.GetPowerRestorePolicy(ctx) })
	}),
	"audit": func(ctx context.Context, s bmc.Session, cfg *config, _ []string) (any, error) {
		x, err := need[bmc.AuditLog](s, bmc.CapAuditLog, "AuditLog")
		if err != nil {
			return nil, err
		}
		off := s.Info().ClockOffset
		entries, err := x.AuditLog(ctx, bmc.AuditQuery{Limit: cfg.limit})
		if err != nil {
			return nil, err
		}
		for i := range entries {
			entries[i].Time = bmc.NewTimestamp(entries[i].Time.Raw, off)
		}
		return map[string]any{"entries": entries, "clockOffsetSeconds": off.Seconds()}, nil
	},
	"events": func(ctx context.Context, s bmc.Session, cfg *config, _ []string) (any, error) {
		x, err := need[bmc.EventLog](s, bmc.CapEventLog, "Events")
		if err != nil {
			return nil, err
		}
		page, err := x.Events(ctx, bmc.EventQuery{SinceID: cfg.since, Limit: cfg.limit})
		if err != nil {
			return nil, err
		}
		off := s.Info().ClockOffset
		for i := range page.Entries {
			page.Entries[i].Time = bmc.NewTimestamp(page.Entries[i].Time.Raw, off)
		}
		if page.Entries == nil {
			page.Entries = []bmc.SELEntry{}
		}
		return map[string]any{"entries": page.Entries, "lastId": page.LastID, "more": page.More, "clockOffsetSeconds": off.Seconds()}, nil
	},
	"inventory": func(ctx context.Context, s bmc.Session, _ *config, args []string) (any, error) {
		x, err := need[bmc.Inventory](s, bmc.CapInventory, "Inventory")
		if err != nil {
			return nil, err
		}
		kind := "cpus"
		if len(args) > 0 {
			kind = args[0]
		}
		switch kind {
		case "cpus":
			return wrap("cpus", func() ([]bmc.CPU, error) { return x.CPUs(ctx) })
		case "memory":
			return wrap("memory", func() ([]bmc.MemoryModule, error) { return x.Memory(ctx) })
		case "pcie":
			return wrap("pcie", func() ([]bmc.PCIeDevice, error) { return x.PCIeDevices(ctx) })
		case "storage":
			return wrap("storage", func() ([]bmc.StorageDevice, error) { return x.Storage(ctx) })
		case "baseboard":
			return wrap("baseboard", func() (bmc.Baseboard, error) { return x.Baseboard(ctx) })
		}
		return nil, usageError{"inventory kind must be cpus, memory, pcie, storage or baseboard"}
	},
}

func opNames() []string {
	names := make([]string, 0, len(ops))
	for k := range ops {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func get(ctx context.Context, cfg *config, t bmc.Target, c bmc.Creds, args []string, out io.Writer) error {
	fn, ok := ops[args[0]]
	if !ok {
		return usageError{fmt.Sprintf("unknown op %q; want one of: %s", args[0], strings.Join(opNames(), ", "))}
	}
	_, s, err := open(ctx, t, c)
	if err != nil {
		return err
	}
	defer s.Close(context.WithoutCancel(ctx))
	v, err := fn(ctx, s, cfg, args[1:])
	if err != nil {
		return err
	}
	return writeJSON(out, v)
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
