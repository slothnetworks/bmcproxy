# bmcproxy foundation contracts

A reference to the exported API of `pkg/obs`, `pkg/bmc` and `pkg/transport`
(module `github.com/slothnetworks/bmcproxy`). Read this before building on these
packages. `go doc ./pkg/<name>` has the full details.

Import graph: `obs` imports only the stdlib. `bmc` imports only the stdlib.
`transport` imports `obs`, `bmc` (for error sentinels only) and `x/sync/semaphore`.

All third-party modules are already in `go.mod`: prometheus/client_golang,
bougou/go-ipmi (use `github.com/bougou/go-ipmi/pkg/client`) and
golang.org/x/sync (`singleflight`, `semaphore`). `pkg/transport/tools.go`
(`//go:build tools`) pins them, so you don't need to edit go.mod.

---

## pkg/obs: logging, ids, redaction, HAR, debug handlers

### Setup (cmd/*)
```go
var o obs.Options
o.RegisterFlags(flag.CommandLine) // --log-level --debug --log-secrets --trace-http --har-out --log-format
flag.Parse()
o.ApplyEnv()                      // merges BMCPROXY_DEBUG into o.Debug
ob, err := obs.New(o)             // logs a WARN if LogSecrets is set
obs.SetDefault(ob)                // fallback for contexts with no Obs
defer ob.Close()
ctx = obs.WithObs(ctx, ob)        // the server does this on every request context
```
- `obs.LevelTrace` (= slog level -8) sits below DEBUG and is used for full wire dumps.
- `--debug` syntax: `name`, `name=trace`, `name:trace` or `all`. A dotted name inherits from its parent: `driver=trace` covers `driver.asrock`. `--debug` only ever makes a subsystem more verbose.
- Subsystem constants: `SubTransport "transport"`, `SubAMI "amimegarac"`, `SubRedfish "redfish"`, `SubIPMI "ipmi"`, `SubSessions "sessions"`, `SubNorth "north"`, `SubCache "cache"`, `SubHealth "health"`, `SubMetrics "metrics"`, `SubCore "core"`, `SubTestsim "testsim"`, `SubDriverASRock "driver.asrock"`, `SubDriverRedfish "driver.redfish"`. A new package may use a new name; it is created on demand.
- `ob.Levels()` returns a `*Levels`: `Set(sub, lvl)`, `SetDefault`, `Reset`, `Level(sub)`, `ApplyDebug(csv)` and `Snapshot()`. Changes take effect at runtime.

### Logging in a package
```go
log := obs.FromContext(ctx, obs.SubAMI)       // a subsystem logger with req_id/corr_id baked in
log.DebugContext(ctx, "session reuse", "bmc", host)
if log.Enabled(ctx, obs.LevelTrace) { log.Log(ctx, obs.LevelTrace, "dump", "body", string(b)) }
```
- Never use `slog.Default()`. Use the `*Context` methods so ids added later (a correlation id, `obs.WithAttrs`) are picked up.
- `obs.LoggerFrom(ctx, fallback *Obs, sub)` works like FromContext but takes an explicit fallback Obs.
- `obs.WithLogger(ctx, *slog.Logger)` is for tests: it bypasses per-subsystem gating.
- Each log line carries these attrs: `subsys`, `req_id`, `corr_id`, plus anything set with `obs.WithAttrs(ctx, k, v, ...)`.

### Ids
- `obs.NewID()`
- Request id: `obs.WithRequestID(ctx, id)` / `obs.RequestID(ctx)`. The server echoes it as `X-Request-Id`.
- Correlation id: `obs.WithCorrelationID(ctx, id)`, `obs.CorrelationID(ctx)` and `obs.EnsureCorrelationID(ctx) (ctx, id)`. `transport.Endpoint.Do` creates one per southbound call if none exists. To give one id to several HTTP calls (login, then GET), set it yourself first.

### Redaction (on by default; `--log-secrets` turns it off)
- Redaction is built into the base handler, so it applies to every log record. These are masked:
  - attr keys `IsSensitiveKey(k)`: Authorization, Cookie, Set-Cookie, X-CSRFTOKEN, X-Auth-Token, QSESSIONID, *password*, *token*, *secret*, *csrf*, *sessionid*, pass/pwd
  - `http.Header` values
  - strings, using `RedactString`, which catches header lines, JSON `"password":"..."`, form `password=...`, URL userinfo and `Basic xxx`/`Bearer xxx`
  - `bmc.Creds`, which also implements `LogValue` so the password is never printed
- Helpers: `RedactHeader(h)`, `RedactString(s)`, `RedactBytes(b)`, `RedactAttr(a)` and `NewRedactingHandler(next, enabled)`. The masked placeholder is `obs.Redacted == "[REDACTED]"`.
- `ob.Redacting()` returns `!LogSecrets`.

### HAR
- `ob.HAR()` returns `*HARWriter`, or nil if `--har-out` is unset. `NewHARWriter(dir, redact)` creates one directly.
- `w.Record(obs.HARRecord{Host, Request, ReqBody, Response, RespBody, Started, Duration, Timings, ServerIP, Err, Comment})`
- Output is one file per host, `<host>_<port>.har`. Each Record rewrites the file atomically (rotating after 5000 entries), so it is always valid HAR 1.2 JSON.
- The exported `obs.HAR`/`HAREntry`/... types can be used to parse HAR files, for example in internal/testsim.
- `pkg/transport` records southbound traffic automatically. Protocol clients don't call this.

### Debug endpoints
`obs.RegisterDebugHandlers(mux, ob)` mounts:
- `/debug/pprof/*`
- `/debug/vars` (expvar)
- `/debug/levels`: GET returns the levels. `POST ?set=transport=trace,redfish`, `?default=debug` and `?reset=x` change them.

Register these only when `--debug-endpoints` is set, on a localhost listener.

---

## pkg/bmc: the plug contract

### Target / Creds / Match
```go
t, err := bmc.ParseTarget(r.Header.Get("X-BMC-Host")) // "host", "host:port", "[v6]:port"; errors wrap ErrInvalidArgument
t.VendorHint = r.Header.Get("X-BMC-Vendor")
t.Addr()      // "host:443": the key for transport and caches
t.BaseURL()   // "https://host[:port]"
t.IPMIAddr()  // "host:623"
c := bmc.Creds{User, Pass} // String/GoString/LogValue redact Pass
```
`Match{Score, Vendor, Manufacturer, Board, Firmware, Backends []string, Details map[string]string}`.
Score constants: `ScoreNone 0`, `ScoreFallback 10`, `ScoreLikely 50`, `ScoreCertain 100`.

### Driver and registration
```go
type Driver interface {
    Name() string                                             // "asrock": registry key, X-BMC-Vendor value, /api/v1/vendor/{name}/
    Detect(ctx, Target) (Match, error)                        // unauthenticated and cheap; return a zero Match (not an error) if it's not yours
    Open(ctx, Target, Creds) (Session, error)                 // errors wrap ErrAuth/ErrTimeout/ErrUpstream
}
// optional:
type CredDetector interface { DetectWithCreds(ctx, Target, Creds) (Match, error) }
type Prioritized  interface { Priority() int }               // higher is tried first; generic Redfish should be negative
type VendorRouter interface { VendorRoutes() []Route }

func init() { bmc.Register(&Driver{}) }  // panics on nil, empty or duplicate name
```
- Registry functions: `bmc.Register`, `bmc.Lookup(name)` and `bmc.Drivers()` (in detection order). `bmc.NewRegistry()` gives an isolated registry for tests.
- `bmc.Resolve(ctx, t, c *Creds) (Driver, Match, error)` resolves in this order:
  1. If `t.VendorHint` is set, that driver is used. Its Detect is still called to fill in board and firmware, but a Detect failure is ignored. An unknown vendor name returns 400.
  2. Otherwise each driver's Detect runs in priority order. The first match at `ScoreCertain` wins; otherwise the highest score wins.
  3. If nothing matches, Resolve returns `ErrUnsupported` (501). If every driver errored, the first error's class is kept instead, so a timeout returns 504.

### Session and capabilities
```go
type Session interface {
    Close(ctx) error      // logout of every backend; idempotent
    Info() SessionInfo    // Driver, Target, User, Board, Firmware, OpenedAt, ClockOffset, Backends []BackendStatus,
                          // Capabilities map[Capability]string (capability -> backend), Degraded
}
```
- Sessions are shared by concurrent requests, so they must be concurrency-safe.
- Backend names are `bmc.BackendRedfish "redfish"`, `BackendAMI "ami"` and `BackendIPMI "ipmi"`.

Capability interfaces. The API layer type-asserts the session against these. A missing interface, or an `ErrUnsupported` return, becomes a 501.

| Interface | Methods |
|---|---|
| `Identity` | `SystemInfo(ctx) (SystemInfo, error)` |
| `PowerController` | `PowerState(ctx) (PowerState, error)`; `Power(ctx, PowerAction) error` |
| `PowerRestorePolicy` | `GetPowerRestorePolicy(ctx) (RestorePolicy, error)`; `SetPowerRestorePolicy(ctx, RestorePolicy) error` |
| `IndicatorLED` | `GetIndicatorLED(ctx) (LEDState, error)`; `SetIndicatorLED(ctx, LEDState) error` |
| `BootOverride` | `GetBootOverride(ctx) (BootOverrideConfig, error)`; `SetBootOverride(ctx, BootOverrideConfig) error` |
| `SensorReader` | `Sensors(ctx) ([]Sensor, error)` |
| `EventLog` | `Events(ctx, EventQuery{SinceID, Limit}) (EventPage{Entries, LastID, More}, error)` |
| `EventLogClearer` | `ClearEvents(ctx) error` |
| `AuditLog` | `AuditLog(ctx, AuditQuery) ([]AuditEntry, error)` |
| `FRUReader` | `FRU(ctx) ([]FRU, error)` |
| `Inventory` | `CPUs`, `Memory`, `PCIeDevices`, `Storage`, `Baseboard` |
| `NetworkManager` | `NetworkInterfaces(ctx)`; `SetNetworkInterface(ctx, NetworkInterfaceUpdate)` |
| `BondManager` | `Bond(ctx) (BondConfig, error)`; `SetBond(ctx, BondConfig) error` |
| `UserReader` | `Users(ctx) ([]User, error)` |
| `ServiceReader` | `Services(ctx) ([]Service, error)` |
| `TimeManager` | `Time(ctx) (TimeConfig, error)`; `SetTime(ctx, TimeUpdate) error`; `SyncTime(ctx) (TimeConfig, error)` |
| `FirmwareInventory` | `Firmware(ctx) ([]FirmwareComponent, error)` |
| `FirmwareUpdater` | `UpdateFirmware(ctx, FirmwareUpdateRequest) (Task, error)`; `FirmwareTask(ctx, id) (Task, error)` |
| `VirtualMedia` | `VirtualMedia(ctx)`; `InsertMedia(ctx, VirtualMediaInsert)`; `EjectMedia(ctx, slotID)` |
| `KVMLauncher` | `KVMLaunch(ctx) (KVMLaunchInfo, error)` |
| `BMCReset` | `ResetBMC(ctx, BMCResetKind) error` |

- `bmc.Capability` constants: `CapPower`, `CapSensors`, and so on; `bmc.AllCapabilities()` lists them all.
- `bmc.Implements(s, cap)`.
- `bmc.Describe(s) []CapabilityReport{Capability, Supported, Backend}` combines interface checks with `Info().Capabilities` and backs `/capabilities`. If `Info().Capabilities` is nil, every implemented interface counts as supported.

### Normalized types (types.go; JSON lowerCamelCase)
- **Enums**:
  - `PowerState` (on/off/poweringOn/poweringOff/unknown)
  - `PowerAction` (on/off/shutdown/cycle/reset/nmi, validated by `ParsePowerAction`)
  - `RestorePolicy` (`ParseRestorePolicy`)
  - `LEDState`
  - `BootTarget` and `BootOverrideMode`
  - `Health` (ok/warning/critical/unknown)
  - `SensorType`
  - `Privilege`
  - `InventoryKind`
  - `TaskState`
  - `BMCResetKind`
- **Sensors**: `Sensor.Reading *float64`. `Thresholds` has six `*float64` fields, and nil means "NA". Use `bmc.F64(v)` to build a pointer.
- **Clock skew**: `Timestamp{Raw, Corrected}`, built with `bmc.NewTimestamp(raw, offset)` where offset = bmc − proxy. `SELEntry.Time` and `AuditEntry.Time` use it.
- **Other structs**:
  - `SystemInfo`, `FRU` (with `FRUChassis`, `FRUBoard`, `FRUProduct`)
  - Inventory: `CPU`, `MemoryModule`, `PCIeDevice`, `StorageDevice`, `Baseboard`
  - Network: `NetworkInterface` (`IPv4Config`/`IPv6Config`/`VLANConfig`), `NetworkInterfaceUpdate` (nil means unchanged), `BondConfig`
  - Accounts and services: `User` (never a password), `Service`
  - Time: `TimeConfig` (`OffsetSeconds`), `TimeUpdate`
  - Firmware: `FirmwareComponent`, `FirmwareUpdateRequest`, `Task`
  - Remote access: `VirtualMediaSlot`, `VirtualMediaInsert`, `KVMLaunchInfo`
- Vendor leftovers go in an `Oem map[string]any` field on each struct.

### Errors (errors.go)
| Sentinel | HTTP |
|---|---|
| `ErrInvalidArgument` | 400 |
| `ErrAuth` | 401 |
| `ErrDisruptiveDenied` | 403 |
| `ErrForbidden` (denylist/policy) | 403 |
| `ErrNotFound` | 404 |
| `ErrUnsupported` | 501 |
| `ErrUpstream` | 502 |
| `ErrBusy` | 503 |
| `ErrTimeout` (also `context.DeadlineExceeded`) | 504 |
| `context.Canceled` | 499 |
| anything else | 500 |

```go
return &bmc.OpError{Pkg: "amimegarac", Op: "GET /api/chassis-status", Backend: bmc.BackendAMI,
    UpstreamStatus: 401, Detail: `{"cc":7}`, Err: bmc.ErrAuth}
bmc.Unsupported("driver.asrock", "BootOverride")       // *OpError wrapping ErrUnsupported
bmc.Errorf(bmc.ErrUpstream, pkg, op, "decode: %v", err)
bmc.Wrap(pkg, op, err)                                  // nil-safe; classifies plain errors (timeout -> ErrTimeout, otherwise ErrUpstream)
bmc.HTTPStatus(err) int; bmc.Class(err) string          // the class is a stable metrics/envelope label
oe, ok := bmc.AsOpError(err)                            // for the envelope's debug block
```
To keep a cause while wrapping a sentinel, use `fmt.Errorf("%w: %w", bmc.ErrUpstream, cause)` as `Err`.

### Vendor routes (vendorroutes.go)
```go
type HandlerFunc func(s bmc.Session, r *http.Request) (any, error)
type Route struct {
    Method  string                              // "" = any method
    Path    string                              // relative ServeMux pattern: "bonding", "componentStatus/{component}", "raw/api/{path...}"
    Tier    bmc.Tier                            // TierRead | TierWrite | TierDisruptive
    TierFor func(*http.Request) (bmc.Tier, error) // optional per-request tier; an error rejects (e.g. ErrForbidden)
    Handler HandlerFunc
    Summary string
}
```
- The server mounts each route at `route.Pattern("/api/v1/vendor/"+driver.Name())` and checks the tier from `route.EffectiveTier(r)`. A disruptive route needs both `--allow-disruptive` and the `X-BMC-Confirm: true` header; otherwise the server returns `ErrDisruptiveDenied`. It then resolves and opens the session and calls the handler.
- The server encodes the handler's result as follows:

  | Result | Response |
  |---|---|
  | `err != nil` | Error envelope with `HTTPStatus(err)` |
  | `nil` result | 204 |
  | Implements `bmc.Responder` (e.g. `*bmc.RawResponse{Status, Header, Body}`) | Written as-is |
  | Anything else | 200 JSON |

- In a handler, type-assert `s` to your concrete session type. `bmc.DecodeJSON(r, &v)` reads a strict JSON body; errors wrap ErrInvalidArgument.
- `Tier` implements Text (un)marshalling as "read", "write" or "disruptive", and `bmc.ParseTier` parses those strings.

---

## pkg/transport: every southbound HTTP call goes through here

### Process setup (one Manager)
```go
tm := transport.NewManager(transport.Config{
    Obs: ob,                          // loggers + HAR writer
    GlobalConcurrency: 64,            // fleet-wide in-flight cap
    Retry:   transport.RetryPolicy{MaxAttempts: 3, BaseDelay: 250*time.Millisecond, MaxDelay: 5*time.Second},
    Breaker: transport.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30*time.Second},
    Hooks:   transport.Hooks{OnAttempt, OnRequest, OnBreakerChange, OnQueueWait}, // for internal/metrics
})
defer tm.Close()
```
The values shown are the defaults; a zero value picks the same default. The defaults also include AttemptTimeout 60s, DialTimeout 10s and TLSHandshakeTimeout 15s.

### In a protocol client
```go
ep := tm.Endpoint(target.Addr(), transport.LaneAMI)      // Lane{"ami",1}; LaneRedfish = {"redfish",4}; LaneDefault = {"default",1}
tm.SetProfile(target.Addr(), transport.ProfileLegacy)    // optional (board profile). Default ProfileAuto: modern, falling back to legacy, learned per host

req, _ := ep.NewRequest(ctx, "GET", "/api/chassis-status", nil)   // path or full URL; body: nil | []byte | string | io.Reader
req.Header.Set("X-CSRFTOKEN", tok)
resp, err := ep.Do(req)      // breaker -> lane limiter (write-priority FIFO) -> global pool -> attempt(+retry) -> TRACE/HAR
defer resp.Body.Close()      // the body is already buffered and the slot released

hc := ep.HTTPClient()        // *http.Client routed through Do (no jar, no redirects followed): for libraries
```
- **Error contract of `Do`**: errors are `*bmc.OpError` values:

  | Condition | Wraps | HTTP |
  |---|---|---|
  | Breaker open | `ErrBusy` | 503 |
  | Final 503 or B650 evasive 403 | `ErrBusy` | 503 |
  | Timeout | `ErrTimeout` | 504 |
  | Connection/TLS error | `ErrUpstream` | 502 |
  | Client gone | `context.Canceled` | 499 |

  Every other status (200, 401, 404, 500, `{cc:7}`) comes back as a normal response for the protocol client to interpret. `errors.Is` still works through `http.Client`'s `*url.Error`.
- **Retry rules**: GET/HEAD/OPTIONS are retried on 503, evasive 403, connection reset/EOF, attempt timeout and TLS handshake failure, with jittered exponential backoff and `Retry-After` honoured. Writes (POST/PUT/PATCH/DELETE) are retried only if:
  - the context is marked with `transport.WithIdempotent(ctx)`, or `RequestOptions{Idempotent: true}`, or
  - the failure happened before the request was written (dial or TLS errors).
- **Per-call options** (attach with `ctx = transport.WithOptions(ctx, transport.RequestOptions{...})`):

  | Option | Effect |
  |---|---|
  | `Idempotent` | Allows retrying a write |
  | `NoRetry` | Never retries |
  | `Priority` | `PriorityRead` / `PriorityWrite`; the default derives it from the method |
  | `Stream` | No buffering: the slot is held until `Body.Close` |
  | `Timeout` | Per-attempt timeout |
  | `RawStatus` | Returns a final 503 or evasive 403 as a response instead of `ErrBusy` |
  | `Op` | A name for logs, HAR and hooks, e.g. "amimegarac.Login"; `transport.WithOp(ctx, op)` sets only this |
- **Multi-step sequences that must not interleave** (e.g. the X470 fan two-step): `ep.Exclusive(ctx, func(ctx context.Context) error { ...ep.Do(req.WithContext(ctx))... })`. This holds the lane slot, and it is re-entrant.
- `X-HTTP-Method-Override`, `X-HTTP-Method` and `X-Method-Override` are always stripped from outgoing requests (`transport.MethodOverrideHeaders`).
- The evasive-403 heuristic (`DefaultIsEvasive403`) treats a 403 whose body is not JSON (HTML or empty) as evasive. To change it, set `Config.IsEvasive403`.
- **Breaker accounting**:
  - Counted as failures: connection errors, timeouts, 502/503/504 and evasive 403.
  - Counted as successes: 2xx-4xx and 500 (the BMC answered).
  - The breaker is per host and shared by all lanes on that host.

### Introspection (for /debug/stats, /metrics, /bmcs/{host}/status)
- `tm.Stats() []HostStats` and `tm.HostStats(host)` return snapshots. `HostStats` has these fields:
  - Counters: `Requests`, `Attempts`, `Retries`, and `Classes` (a map keyed by `ClassOK`, `ClassAuth`, `ClassBusy`, `ClassServerError`, `ClassTimeout`, `ClassConn`, `ClassBreakerOpen`, `ClassCanceled`, `ClassClientError`)
  - Concurrency: `Lanes` (Limit, InFlight, Queued)
  - Breaker: `Breaker` (State, ConsecutiveFailures, OpenedAt, Trips)
  - Latency: `Latency Histogram`. Its fields are Buckets, Counts (cumulative, a `map[float64]uint64` ready for `prometheus.MustNewConstHistogram`), Count and Sum, and it uses the buckets in `LatencyBuckets`.
  - Queueing: `QueueWaitSeconds`, `QueueWaits`
  - Last outcome: `LastError` (redacted), `LastErrorAt`, `LastSuccessAt`
  - TLS: `TLS *TLSInfo{Profile, Version, CipherSuite}`
- `tm.BreakerState(host)` and `tm.Breaker(host).Reset()`. `BreakerState.Gauge()` maps closed/half-open/open to 0/1/2.
- `Hooks` callbacks run synchronously; keep them fast.

### Lower-level pieces (exported for reuse/tests)
- `TLSConfig(profile)`: always sets InsecureSkipVerify. The legacy profile allows TLS 1.0–1.2 plus RSA-kx/CBC/3DES suites.
- `NewLimiter(n)`, with `Acquire(ctx, prio)`, `Release`, `TryAcquire`, `SetLimit`, `InFlight` and `Waiting`.
- `NewBreaker(cfg, now, onChange)`, with `Allow() (done func(Outcome), error)`.
- `RetryPolicy.Backoff(n)`.
- `IsTransientNetError(err)` and `IsTLSHandshakeError(err)`.
- `Tracer{Base, Obs, HAR}`: a RoundTripper that buffers, dumps at TRACE and records HAR, without limiting.
- `NewHTTPClient(profile, ob)`: a plain traced client for one-shot tools, with no limiter, breaker or retry.
- `CanonicalHost(h)` returns "host:port", defaulting the port to 443.

## Additions after wave 2 (coordinator)

- `transport.SetDefault(m)` / `transport.Default()` — the process-wide `*transport.Manager`. `cmd/bmcproxy` installs it at startup; drivers registered via `init()` MUST obtain their Manager with `transport.Default()` (or an explicit setter that defaults to it) so limiter/breaker/stats/metrics are shared. `genericredfish.Default.Transport()` already falls back to it.
- `bmc.ErrSessionLost` — wraps `ErrUpstream` (→ 502). Return it (wrapped in an OpError) when the BMC backend restarted and every session on the host is gone (e.g. amimegarac `Hooks.OnHostInvalidated`, 502 / HTML 500). `sessions.Manager.Do` then calls `InvalidateHost` and retries once.
- Swagger: handlers carry swaggo annotations; `make build` regenerates `internal/apidocs/{swagger,openapi}.json` (swag scans ./cmd/bmcproxy, ./internal, ./pkg/bmc, ./vendors). Vendor route handlers must carry their own `@Router /api/v1/vendor/<vendor>/...` annotations with the same conventions as internal/api/v1 (shared header params X-BMC-Host/X-BMC-Vendor/X-BMC-Confirm, `@Security BMCBasicAuth`, `server.ErrorBody` failures, named exported request/response structs with field comments and `example:` tags).
