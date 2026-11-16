# bmcproxy: pluggable BMC abstraction proxy (Go)

## Context

We run BMCs that don't all speak the same management protocol:
- Some support Redfish and accept normal credentials (B650D4U live 4.10 → Redfish-first).
- Some run Redfish but won't accept usable credentials: the live X470D4U on 2.32.00 returns `401 AccessDenied` for the `admin` account that works on its legacy AMI `/api` (→ `/api`+IPMI-first). Confirmed non-transient on live hardware.
- Future vendors will bring their own quirks.

The proxy abstracts over this: a client asks for an operation (power, sensors, …) and the proxy picks whatever protocol that specific board+firmware actually supports.

**bmcprobe** is a stateless Go proxy:
- **Generic API:** one vendor-neutral surface (`/api/v1/...`) for common operations such as power, sensors, the event log and inventory. The right backend is picked per BMC: Redfish, AMI `/api`, or IPMI.
- **Vendor namespace:** BMC-specific features such as `POST /api/v1/vendor/asrock/disableBonding`.
- **Pluggable vendors:** each vendor is a driver package that registers itself. Adding a vendor needs no core changes.
- **No stored secrets, no proxy auth (for now):** every request carries the BMC address and the BMC's own credentials. Proxy API auth comes in a later phase.

The research behind the route mapping is in `docs/`:
- `docs/api/legacy-api-endpoints.json`: 312 handled legacy routes, with handler presence per board.
- `docs/redfish/b650-uri-inventory.json`: 339 Redfish URIs and 48 actions (from firmware).
- `docs/x470/live-probe.md`, `docs/b650/live-probe.md`: live behaviour, TLS, cookies, crash-prone endpoints, query/paging behaviour.
- `docs/x470/endpoint-reference.{md,json}`, `docs/x470/endpoint-behaviour.json`: the live `/api` surface.
- `docs/*/har-endpoints.json`: real request and response bodies.

## Live-probe findings that drive the design (two real boards)

Probed read-live: X470D4U at `10.3.10.207` (creds in `bmcprobe.env`) and B650D4U at `10.7.12.55` (creds in `b650d4u.env`). Both env files are gitignored. The two boards sit at opposite ends of almost every axis, which is the core reason the proxy must be per-board-profile, not one code path.

| Axis | X470D4U (live **2.32.00**) | B650D4U (live **4.10.00**) |
|---|---|---|
| Redfish auth | **Rejects** the admin account (401 AccessDenied, Basic and session) — confirmed non-transient | **Accepts** admin via Basic (Systems/Self 200) |
| Redfish version | `ServiceRoot.v1_3_1`, RedfishVersion 1.5.0, no Sensor/Certificate schemas | `v1_7_0`, 1.15.1, has CertificateService, 60 Sensors, Power/ThermalSubsystem |
| Primary southbound | **legacy `/api` + IPMI** | **Redfish** (`/api` for OEM/gaps) |
| TLS | SSLv3–TLS1.2, no forward secrecy, cert **expired** 2026-05-30 → Go must set legacy RSA-kx suites | TLS1.2+1.3, ECDHE/AEAD, cert valid to 2035 → Go defaults work |
| Query opts ($select/$top/$skip/$expand) | advertised; unknown param → **501** | **all honored**; unknown param → 200 (ignored) |
| Pagination | — (no auth to test) | pages at 50, **nextLink terminates** (verified; earlier "broken" claim was a probe `$`-escaping bug) |
| ETags | one service-wide ETag reused everywhere | per-resource-group (48 distinct); conditional writes plausible |
| `/api` cookie | bare `path=/`, no HttpOnly/Secure | `Secure;HttpOnly;SameSite=Lax` |
| Sensor unit / thresholds | — | `°C`, unset threshold = string `"NA"`, `settable_flag` |

**Firmware reality:** every board runs a *different* version than its dumped image (X470 live 2.32 vs dumped 3.02; B650 live 4.10 vs dumped 7.02/9.05 vs HAR 1.10). The proxy keys behaviour off the **live-reported version**, never the image, and tolerates unknown versions by falling back to capability probing.

**Cross-board behaviour the proxy must encode (both boards unless noted):**
- `GET /api/logs/help-data` crashes the backend and **invalidates every web session, not just the caller's** (X470, reproduced). Hard denylist; also `asrr/mca-file`, `settings/download_image`, `hpm/upgradestatus`, `hpm/verifyimagestatus` (502/crash).
- Legacy `/api`: writes **enforce** CSRF (`401 {cc:7}` on bad/missing token); GETs do **not** (bogus token → 200). Backend honours `X-HTTP-Method-Override` — the proxy must strip it.
- `GlobalLuaTables` is served **unauthenticated** on both boards → the proxy must block it.
- TLS: **the client trusts whatever cert the BMC returns** (`InsecureSkipVerify`, no chain check, no pinning). Justified: the cert is the shared AMI firmware cert whose private key ships in firmware (public), so pinning gives no real identity assurance — network isolation is the control. Still required: **per-generation cipher/version config** (X470 offers only RSA key-exchange on TLS 1.0–1.2, which Go won't negotiate by default; B650 is modern).
- Redfish SessionTimeout is **30 seconds** (B650, schema minimum) → prefer Basic auth or refresh aggressively.
- Soft power-off (code 5 / GracefulShutdown) may be silently ignored mid-POST → the proxy must poll power state, not assume the action took.
- Both boards are reachable one-request-at-a-time; concurrent probing skewed every latency/session measurement. Serialise per host.

**Lesson baked into tooling:** multiple probe "findings" were wrong due to unescaped `$` in shell URL building (query params silently dropped). The `--har-out` debug flag (below) exists partly to catch exactly this by showing the actual bytes sent.

## Request model (no stored secrets)

```
POST /api/v1/vendor/asrock/disableBonding
X-BMC-Host: 10.3.10.207          # IP or host[:port]
Authorization: Basic <base64(bmcuser:bmcpass)>   # the BMC's own credentials, passed through
X-BMC-Vendor: asrock             # optional override; otherwise auto-detected
Content-Type: application/json
{"interface":"eth0"}
```

**Credentials**
- Used only for the life of the request.
- Never logged; redaction middleware masks `Authorization`.
- Never written to disk.

**Session cache (memory only)**
- Keyed by `host + user + sha256(pass)` with an idle TTL shorter than the BMC timeout.
- Without it, every call would cost a 1.5 s login and use up BMC session slots.
- On eviction or shutdown it logs out (`DELETE /api/session` or Redfish session DELETE).

**Target cache (memory only, non-secret)**
- Per host: detected vendor, board, firmware, and which backends authenticate. (No cert pinning — the client trusts whatever cert the BMC returns; see the TLS note above.)

**Safety without API auth**
- Default bind is `127.0.0.1:8080`.
- Operations are tiered: `read`, `write` (reversible: LED, boot override, fan mode) and `disruptive` (network/bond, BMC reset, firmware, factory reset, log clear, user changes).
- `disruptive` returns 403 unless the server runs with `--allow-disruptive`, and the request also needs `X-BMC-Confirm: true`.

**Errors (JSON envelope)**

| Status | Meaning |
|---|---|
| 501 | Capability not supported by this BMC or driver |
| 401 | The BMC rejected the credentials |
| 502 | BMC error, with the vendor detail |
| 503 | BMC busy or connection-capped (B650 evasive 403) |
| 504 | Timeout |

## Debugging and observability (cross-cutting, every layer)

A hard requirement: every part of the code is instrumented and independently switchable, because the BMCs are quirky, slow and version-dependent, and most bugs will be "what exactly went over the wire to this board".

**Logging**
- `log/slog` everywhere, structured. One `*slog.Logger` is threaded through `context.Context`; no package uses the global logger.
- Per-subsystem log levels, not one global level. `--log-level` sets the default; `--debug=<csv>` or `BMCPROXY_DEBUG=transport,amimegarac,redfish,sessions,driver.asrock,north,ipmi` raises named subsystems to DEBUG/TRACE at runtime. `--debug=all` turns on everything. Each `pkg`/`internal` package owns a named logger (e.g. `slog` group `transport`), so a flag maps 1:1 to a code area.
- Levels: INFO (lifecycle, each northbound request one line), DEBUG (backend selection, cache hit/miss, session reuse/relogin, retries, limiter waits), TRACE (full request/response incl. headers and bodies).
- **Redaction is a logging-layer concern, on by default** (`Authorization`, cookies, `X-CSRFTOKEN`, X-Auth-Token, password-ish JSON fields masked). `--log-secrets` disables it for local debugging of real BMC auth; it also prints a startup warning. Pass-through BMC credentials are redacted even at TRACE unless `--log-secrets` is set.

**Request tracing**
- A request-id per northbound call (`X-Request-Id` echoed) and a correlation id per southbound BMC call, both in every log line, so one client call can be followed through session acquire → transport queue → BMC HTTP → decode.
- `--trace-http` (or the `transport` debug subsystem at TRACE) dumps every southbound HTTP request/response, timing included, via an `httptrace`/`RoundTripper` wrapper in `pkg/transport`. Optional `--har-out <dir>` writes a real HAR of all southbound traffic per BMC — directly comparable to the `docs/*/har-endpoints.json` captures and reusable as test fixtures.
- IPMI: a debug hook logs each command/response (NetFn/Cmd, completion code).

**Introspection endpoints** (bind to localhost; gated behind `--debug-endpoints`)
- `GET /debug/targets` — the per-host target cache: detected vendor, board, firmware, negotiated TLS version/cipher, which backends authenticated, last probe time.
- `GET /debug/sessions` — live session cache (host, user, age, TTL; never the token).
- `GET /debug/capabilities?host=` — resolved driver + per-operation backend choice + supported/501 map.
- `GET /debug/stats` — per-host counters: requests, errors by class, limiter queue depth, relogins, cache hit rate, backend latencies (histogram).
- `net/http/pprof` and `expvar` under `/debug/pprof` when `--debug-endpoints` is on.

**Errors carry context**
- The JSON error envelope includes the request id and (when `--debug-endpoints`/debug level is on) a `debug` block: which backend was tried, the raw upstream status, and the decode error. Normal mode omits it.
- Every `ErrUnsupported`/`ErrAuth`/`ErrBusy` wraps the originating `pkg`+operation so a 501/401/503 is traceable to the exact capability and board profile.

**bmcprobe CLI** mirrors the flags (`--debug`, `--trace-http`, `--har-out`, `--log-secrets`) so the same instrumentation is available for one-shot diagnostics without running the daemon.

**Testing the instrumentation**: a test asserts that TRACE-level logs of a credentialed request contain no secret when redaction is on, and do when `--log-secrets` is set; and that `--har-out` produces a parseable HAR.

## Reliability, efficiency & ops (cross-cutting)

The BMCs are slow, single-threaded, flaky and finite-session, and the fleet is large, so these are first-class, not polish.

**Fleet resilience** (`pkg/transport` + `internal/core`)
- **Bounded global concurrency**: a worker pool caps total in-flight southbound work so N BMCs don't trigger N simultaneous logins/handshakes. Per-host stays serialised (1 for `/api`, ≤4 Redfish); this bounds the whole fleet on top.
- **Retry with backoff** on transient southbound failures: the B650 evasive **403**, **503**, connection resets, and TLS/handshake timeouts. Bounded retries, jittered backoff; writes retried only when safe (see verify-after-write below).
- **Per-BMC circuit breaker**: after repeated failures or timeouts a host trips open, fails fast with 503 for a cool-down, and half-opens to probe — so one wedged/rebooting BMC can't exhaust the pool or stall unrelated requests.
- **Backend-restart detection → re-login**: `help-data`/502s and backend PID changes invalidate *all* sessions at once (observed). On an unexpected `401 {cc:7}` / Redfish 401 mid-session, drop the cached session and re-login once before surfacing an error, and mark the target briefly degraded.

**Efficiency & correctness**
- **Request coalescing (singleflight)**: concurrent identical reads to one host collapse into a single southbound call whose result fans out — N clients asking for `/sensors` = 1 BMC call. Sits in `internal/cache` alongside the TTLs.
- **Clock-skew normalization**: at session open, record `offset = bmc_time − proxy_time` (BMC clocks seen off by hours, NTP often off). Normalize SEL/event/audit timestamps to real UTC in normalized models, and expose both raw and corrected. Surface large skew as a health warning — and offer to fix it via the time/NTP write below (`POST /time/sync` sets the clock to current UTC; or point the BMC at NTP). Caveat: the X470 `PUT settings/date-time` **500'd on a partial body** in our round-trip test, so the driver GETs the full object, mutates the needed fields, and PUTs the whole thing back; `timezone`/`utc_minutes` must be preserved. NTP server changes may restart BMC networking (treat as `write`, verify after).

**Ops surface**
- **Northbound health**: `GET /api/v1/health` (proxy) and `GET /api/v1/bmcs/{host}/status` (per BMC: reachable, which backends authenticated, board+firmware, circuit state, clock skew, degraded reason). Backed by the target cache.
- **Prometheus `/metrics`**: per-host request counts, error class, latency histograms, live session count, cache hit rate, circuit-breaker state. (Complements `/debug/stats`/expvar.)
- **Graceful shutdown**: on SIGTERM, stop accepting, drain in-flight, and **log out every cached session** (`DELETE /api/session` / Redfish session DELETE) so the finite BMC session slots (128 web / 36 IPMI on X470) aren't leaked.

## Directory layout

```
cmd/bmcproxy/main.go              flags, wiring; blank-imports vendor drivers (database/sql-style registration)
cmd/bmcprobe/main.go              CLI: detect + capability report for a host (dev/diagnostics)

internal/server/                  http.Server, router (stdlib ServeMux patterns), middleware:
                                    target+cred extraction, redaction, request id, tier gate, error envelope
internal/api/v1/                  generic handlers: resolve session → type-assert capability → call → JSON
internal/api/vendorroutes/        mounts each driver's VendorRoutes() under /api/v1/vendor/{vendor}/
internal/sessions/                in-memory session cache, per-host target cache, logout on evict + on shutdown
internal/health/                  northbound /api/v1/health + /bmcs/{host}/status from the target cache
internal/metrics/                 Prometheus /metrics (request/latency/error/session/cache/breaker gauges)
internal/testsim/                 HAR-replay BMC simulator: serves docs/*/har-endpoints.json fixtures with
                                    each board's quirks (cc:7, "NA", evasive-403, two-step fan) for CI tests

pkg/bmc/                          vendor-neutral contract (the "plug" interface)
  driver.go                       Driver{Name; Detect(ctx,Target) (Match,error); Open(ctx,Target,Creds) (Session,error)}
  registry.go                     Register(Driver), Resolve(ctx,target) (vendor override or Detect in priority order)
  capabilities.go                 small interfaces a Session may implement:
                                    Identity, PowerController, PowerRestorePolicy, IndicatorLED, BootOverride,
                                    SensorReader, EventLog (incremental), AuditLog, FRUReader, Inventory,
                                    NetworkManager, BondManager, UserReader, ServiceReader, TimeManager,
                                    FirmwareInventory, FirmwareUpdater, VirtualMedia, KVMLauncher, BMCReset
  vendorroutes.go                 optional VendorRouter{VendorRoutes() []Route{Method,Path,Tier,Handler}}
  types.go                        normalized models (PowerState, Sensor, SELEntry, FRU, BondConfig, ...)
  errors.go                       ErrUnsupported, ErrAuth, ErrBusy, ErrDisruptiveDenied

pkg/obs/                          shared instrumentation: named slog loggers, per-subsystem level
                                    registry (--debug csv), context logger, secret-redaction handler,
                                    request/correlation ids, HAR writer, debug http handlers
pkg/transport/                    shared HTTP client factory: TLS profiles (InsecureSkipVerify always;
                                    legacy RSA-kx suites + TLS1.0 for X470, modern for B650),
                                    keep-alive pool, per-host limiter (1 for AMI /api, ≤4 Redfish),
                                    FIFO queue with write priority, bounded global worker pool,
                                    retry/backoff (503/evasive-403/reset), per-host circuit breaker
pkg/protocol/redfish/             vendor-neutral Redfish client: sessions/Basic, lenient decode, nextLink,
                                    task monitor (Location or MessageArgs), quirk hooks
pkg/protocol/ipmi/                lanplus wrapper over github.com/bougou/go-ipmi (power, bootdev, SDR, SEL, FRU)
pkg/protocol/amimegarac/          AMI MegaRAC SPX /api client, shared by every MegaRAC-based vendor
                                    (ASRock, Gigabyte, ASUS, ...): form login, QSESSIONID + X-CSRFTOKEN,
                                    cc:7 re-login, logout, denylist (logs/help-data), lenient JSON ("NA")

vendors/
  generic/redfish/                fallback driver for any standards-compliant Redfish BMC
  asrock/
    driver.go                     Register("asrock"); Detect via /api/asrr/model-name or Redfish Manufacturer
    session.go                    Session; picks a backend per capability (Redfish → AMI /api → IPMI)
    boards/                       board+firmware profiles: x470d4u.go, b650d4u.go, b550d4u.go
                                    (dialect: endpoint names, field names, quirks, handler presence)
    x470_pam_autoenable.go        //go:build x470_pam — OPT-IN, excluded by default. When built with
                                    the `x470_pam` tag, the X470 driver, on first contact, uses an
                                    operator-supplied bootstrap cred (Administrator) to PATCH
                                    PAMEnabled=true (If-Match ETag), after which `admin` auths over
                                    Redfish and the driver prefers Redfish for common ops. Logic mirrors
                                    scripts/redfish-pam-auth.sh. A matching x470_pam_stub.go
                                    (//go:build !x470_pam) is a no-op so default builds never mutate a BMC.
    caps_power.go caps_sensors.go caps_logs.go caps_inventory.go caps_network.go ...
    oem/                          ASRock-only ops (asrr/*): fan, bonding, NC-SI, vetobit, power restore,
                                    POST code, MCA log, prochot, component firmware
    routes.go                     VendorRoutes() → /api/v1/vendor/asrock/*
    testdata/                     sanitized HAR fixtures per board/firmware
  (future) supermicro/, dell/, hpe/, gigabyte/ (can reuse pkg/protocol/amimegarac)
```

**How plug-in works**
- Go has no practical runtime plugins, so each driver is a package with `func init(){ bmc.Register(...) }`, enabled by a blank import in `cmd/bmcproxy`.
- The core only knows `pkg/bmc` interfaces. A capability a driver doesn't implement returns 501 automatically.
- `GET /api/v1/capabilities` reports, for the target, which generic operations are supported, which backend serves each, and the vendor routes.

## Generic routes `/api/v1/*` and the ASRock backend mapping

| Generic route | Tier | ASRock via AMI `/api` (X470 2.32/3.02; B650/B550 1.10, 7.02, 9.05) | ASRock via Redfish (when auth works) | IPMI fallback |
|---|---|---|---|---|
| `GET /capabilities` | read | detection (`asrr/model-name`, `asrr/fw-info`, `configuration/project`) | ServiceRoot | Get Device ID |
| `GET /system` | read | `asrr/fw-info`, `firmware-info`, `asrr/model-name` | `Systems/Self`, `Managers/Self` | mc info |
| `GET /power` | read | `chassis-status` | `Systems/Self.PowerState` | chassis status |
| `POST /power {action:on,off,shutdown,cycle,reset}` | write | `actions/power` codes 1,0,5,2,3 | `ComputerSystem.Reset` | chassis power |
| `GET/PUT /power/restore-policy` | write | `asrr/GetPowerRestore`, `SetPowerRestore` (B650) | `PowerRestorePolicy` | chassis policy (X470) |
| `POST /led {on,off}` | write | `actions/chassis-led` | `IndicatorLED` | chassis identify |
| `GET/PUT /boot` (override) | write | none on `/api` | `Boot.BootSourceOverride*` | chassis bootdev (X470 path) |
| `GET /sensors` | read | `sensors` (+`sdr`) | `Chassis/Self/Sensors`, Thermal, Power | sdr elist |
| `GET /events?since=` | read | `logs/event?LASTEVENTID=n` (incremental) | `LogServices/SEL/Entries` | sel list |
| `DELETE /events` | disruptive | `DELETE logs/event` | `LogService.ClearLog` | sel clear |
| `GET /logs/audit` | read | `logs/audit` | Manager `LogServices` | – |
| `GET /fru` | read | `fru` | Chassis/Assembly | fru |
| `GET /inventory/{cpus,memory,pcie,storage,baseboard}` | read | `host_inventory/*`, X470 `system_inventory/*` | Processors, Memory, PCIeDevices, Storage | – |
| `GET /network`, `PUT /network` | read / disruptive | `settings/network` | `EthernetInterfaces` | lan print |
| `GET /users` | read | `settings/users` | `AccountService/Accounts` | user list |
| `GET /services` | read | `settings/services` | `NetworkProtocol` | – |
| `GET /time` | read | `settings/date-time` (timestamp, timezone, utc_minutes, primary/secondary_ntp, auto_date) | Manager `DateTime`, `DateTimeLocalOffset` + `NetworkProtocol.NTP` | – |
| `PUT /time` (set clock and/or NTP) | write | `PUT settings/date-time` — full object echoed back; set `timestamp` for a manual clock set, or `auto_date`/`ntp_auto_date`=1 + `primary_ntp`/`secondary_ntp` for NTP | Manager `DateTime` PATCH (manual); `NetworkProtocol` PATCH `{NTP:{ProtocolEnabled, NTPServers[]}}` | – |
| `POST /time/sync` (correct skew) | write | compute `now` and `PUT settings/date-time` with it (uses the measured per-BMC offset) | Manager `DateTime` PATCH to current UTC | – |
| `GET /firmware` | read | `asrr/fw-info`, `maintenance/hpm/componentversions` | `FirmwareInventory` | – |
| `POST /firmware/update` | disruptive (later) | `maintenance/firmware*`, `asrr/maintenance/{BIOS,CPLD,...}` | `UpdateService/upload`, `SimpleUpdate` | – |
| `GET /virtual-media`, `POST .../insert`, `.../eject` | write | `settings/media/remote/*` | `VirtualMedia.InsertMedia`/`EjectMedia` | – |
| `GET /kvm` (launch info) | read | `kvm/token`, `remote_control/get/kvm/launch` | – | – |
| `POST /bmc/reset` | disruptive | `asrr/bmccoldreset` | `Manager.Reset` | mc reset cold |

## Vendor routes `/api/v1/vendor/asrock/*`

Board profiles resolve the dialect: X470 uses `asrr/settings/*`; B650 uses `settings/fan/*` and `ncsilinkup`.

| Route | Tier | Backend (X470 / B650) |
|---|---|---|
| `GET bonding` | read | `settings/network-bond` |
| `POST enableBonding {mode,interface}` / `POST disableBonding {interface}` | disruptive | `PUT settings/network-bond {id,bond_enable,bond_mode,auto_configuration_enable,bond_ifc}` |
| `GET ncsiMode` / `POST setNcsiMode` | read / disruptive | `settings/ncsi/mode` |
| `GET keepNicLinkUp` / `POST setKeepNicLinkUp` | read / write | `asrr/getvetobit`,`setvetobit` / `settings/ncsilinkup` |
| `GET fanMode` / `POST setFanMode` | read / write | `asrr/settings/{get,set}fancontrolmode` / `settings/fan/mode` |
| `GET fanTables?type=open,closed` / `POST setFanTable` | read / write | `asrr/settings/*loop*` (two-step select+read under a per-host lock) / `settings/fan/openloop`,`closedloop` |
| `POST setManualFanDuty` / `GET fanDuty` | write / read | `asrr/settings/setfandutymanualmode`, `GetFanDuty` / `settings/fan/manual` |
| `POST resetFanSettings` | write | `asrr/settings/setclearallfansetting` / `settings/fan/load_default` |
| `GET powerSource` | read | `asrr/PowerSource` |
| `GET postCode` | read | `asrr/getbioscode` / `logs/postcode` |
| `GET mcaLog` / `POST clearMcaLog` | read / disruptive | `asrr/getmcalog`, `clearmcalog` |
| `GET prochot` / `POST setProchot` | read / write | `asrr/getprochot`,`setprochot` (X470 only) |
| `GET webEnableTimeout` / `POST setWebEnableTimeout` | read / write | `asrr/getWebEnableTimeout`,`set...` |
| `GET biosUpdateOption` | read | `asrr/maintenance/BIOS/UpdateOption` |
| `GET componentStatus/{BIOS,CPLD,FPGA,RETIMER,PSU,BP}` | read | `asrr/maintenance/{C}/status`,`flash-progress` |
| `POST updateComponent/{C}` | disruptive (later) | `asrr/maintenance/{C}/firmware` → `upgrade` → poll |
| `POST bmcColdReset` | disruptive | `asrr/bmccoldreset` |
| `ANY raw/api/{path...}` | tier from allow/deny table | Authenticated passthrough to the AMI `/api`. Denylist: `logs/help-data`, flash/restore/reset unless disruptive is allowed |
| `ANY raw/redfish/{path...}` | same | Authenticated passthrough to the BMC Redfish |

**Out of v1 scope** but mappable later from the catalog:
- RAID (X470)
- LDAP/AD/RADIUS
- SSL certificates
- PEF/SMTP/SNMP
- video recording
- SOL
- the KVM websocket proxy
- a Redfish northbound (the earlier "both" decision)

## Milestones

1. **Skeleton.**
   - `pkg/obs` first: named loggers, the `--debug`/`--trace-http`/`--log-secrets` flags, redaction handler, request ids, HAR writer. Everything built after this is instrumented from the start, not retrofitted.
   - `pkg/bmc` contracts and registry.
   - `internal/server` with request model, redaction and tier gate.
   - `pkg/transport` (incl. the bounded pool, retry/backoff and circuit breaker from the start — they wrap every southbound call).
   - `pkg/protocol/amimegarac` with session handling + backend-restart re-login.
   - `internal/sessions`.
   - `cmd/bmcprobe detect`.
2. **ASRock driver, reads.** `capabilities`, `system`, `power`, `sensors`, `events` (incremental), `logs/audit`, `fru`, `inventory`, `network`, `users`, `services`, `firmware`, plus X470/B650 board profiles. Add here: singleflight read coalescing + TTL cache, clock-skew capture/normalization, and the `internal/testsim` HAR-replay simulator so these are tested in CI from the start. Northbound `/health`, `/metrics`, and graceful-shutdown session drain land in this milestone too (they depend only on the session/target cache).
3. **Writes.**
   - Power actions, LED, power-restore policy, and **time/NTP** (`GET /time`, `PUT /time` for manual-set or NTP-server config, `POST /time/sync` to correct skew — the `TimeManager` capability).
   - The vendor routes: bonding, NC-SI, vetobit, fan, prochot, web timeout.
   - The `raw/*` passthroughs.
4. **Protocol fallbacks.**
   - `pkg/protocol/redfish` plus the `vendors/generic/redfish` driver, used by ASRock for B650 where Redfish auth works.
   - `pkg/protocol/ipmi` for boot override and policy on the X470.
   - The opt-in `x470_pam` build-tag codepath (default X470 stays `/api`+IPMI; see `boards/x470_pam_autoenable.go`). Build it only when explicitly wanted; default builds exclude it and never mutate a BMC.
5. **Later:** firmware/component update, virtual media, KVM websocket proxy, Redfish northbound, proxy API auth, the next vendor.

## Phase 0 (small, before code)

- DONE: X470 live probe (`docs/x470/live-probe.md`), X470 endpoint reference (`docs/x470/endpoint-reference.{md,json}`, `endpoint-behaviour.json`), B650 read-only live probe (`docs/b650/live-probe.md`, `live-redfish-inventory.json`, corrected for the `$`-escaping artifacts).
- NEXT (approved): **rewrite `docs/README.md`** — the existing one (01:54) predates both live probes and is stale. The rewrite must:
  - Fix the firmware table: B650D4U field firmware is **4.10.00** (live), not 1.10; keep the dumped-vs-field distinction.
  - Add the missing docs to the index: `b650/live-probe.md` + `live-redfish-inventory.json`, `x470/endpoint-reference.{md,json}`, `x470/endpoint-behaviour.json`, `x470/redfish-auth-analysis.md`, `x470/redfish-uri-inventory.json`, and the per-area `scripts/` folders.
  - Correct the X470 Redfish story: not an "8-char password policy" guess — the real cause (separate redis account store, no IPMI→Redfish sync on X470, PAM gated off by `PAMEnabled`) per `redfish-auth-analysis.md`. State the proxy stance: **X470 = `/api`+IPMI**.
  - Refresh the capability matrix with the B650 4.10 live column (Redfish admin auth ✅, full crawl ✅, query/paging ✅, SessionTimeout 30s, per-group ETags).
  - Correct the B650 query/pagination note (works; earlier "broken" was a probe `$`-escaping bug).
  - Fix `legacy-api-endpoints.json` field names (`handler_present`, `seen_in_har`; the old `seen_in_live_capture` is gone).
  - Expand security findings: default `Administrator`/`superuser` Redfish account invisible to the web UI; shared `sysadmin` uid-0 default hash with `PermitRootLogin yes` → serial-console root; unencrypted config backup.
- Optional, needs user OK: writes-on-live confirmation (B650 IndicatorLED/Boot PATCH + If-Match behaviour, ComputerSystem.Reset, a Redfish session X-Auth-Token shape, SSE subscription) — the "needs a write to confirm" lists in both live-probe docs. These validate Milestone 3/4 assumptions before code.

## Verification

- **HAR-replay simulator (`internal/testsim`):** integration tests run the full proxy against the replayed board fixtures (no hardware), exercising each board's quirks and the resilience paths (evasive-403 → backoff → 503, backend-restart → re-login, circuit-breaker trip/half-open, singleflight coalescing, clock-skew normalization, graceful-shutdown session logout).
- **Unit tests per driver:** `httptest` fake BMCs replay `vendors/asrock/testdata` (sanitized from `docs/*/har-endpoints.json`). Cover:
  - each board dialect (X470 2.32 vs B650 1.10);
  - cc:7 re-login;
  - `"NA"` thresholds;
  - evasive-403 → 503;
  - the fan two-step lock;
  - unsupported → 501;
  - disruptive denied without the flag and confirm header.
- **Redaction test:** a request with credentials produces no password or Basic header in the logs.
- **Live tests** (build tag `live`, using `bmcprobe.env` for the X470 at 10.3.10.207):
  - `curl -H 'X-BMC-Host: …' -u admin:… localhost:8080/api/v1/{capabilities,system,power,sensors,events,fru}` matches direct `/api` results;
  - LED on/off round trip;
  - `GET /api/v1/vendor/asrock/bonding`.
  - Disruptive routes (`disableBonding`) are verified only with fixtures unless the user approves a live test.
- **Concurrency:** never more than 1 in-flight AMI `/api` request per host; session reuse across requests (1 login for N calls).
