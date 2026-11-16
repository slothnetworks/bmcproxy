# bmcproxy

`bmcproxy` is a stateless HTTP proxy in front of a fleet of server BMCs. Clients make one uniform JSON API call (`/api/v1/...`) and name the BMC in a header. The proxy then:

- picks a vendor driver;
- logs in and keeps the BMC session;
- serialises and retries traffic the way each BMC needs;
- answers in a vendor-neutral shape.

The first full driver is **ASRock Rack** (AMI MegaRAC). It covers:

- **X470D4U** (AST2500, firmware 2.32), through the legacy AMI `/api` REST interface plus IPMI;
- **B650D4U** (AST2600, firmware 4.x), Redfish-first, using `/api` for the OEM gaps.

A generic DMTF Redfish driver is the fallback for everything else.

`bmcprobe` is a companion one-shot CLI. It runs the same drivers against one BMC, for diagnostics.

The research behind the drivers is in [docs/](docs/): firmware analysis, HAR captures, live probes and the endpoint catalogs. Start at [docs/README.md](docs/README.md). The plan and the API contracts are in [docs/PLAN.md](docs/PLAN.md) and [docs/CONTRACTS.md](docs/CONTRACTS.md).

> There is no authentication on the proxy API yet. Callers supply the BMC's own credentials on every request. Keep `--listen` on localhost or behind something that authenticates.

## Build

```sh
make build          # go generate (API docs), then build cmd/... into bin/
make test           # go test ./...
make vet
make check-openapi  # CI: fail if the committed specs are stale
```

Every `make build` regenerates `internal/apidocs/swagger.json` (swag annotations) and `internal/apidocs/openapi.json` (`cmd/openapigen`). The proxy embeds the specs and serves:

| Path | What |
| --- | --- |
| `/openapi.json` | OpenAPI 3 spec (generic routes plus every driver's vendor routes) |
| `/swagger/` | Swagger UI |
| `/swagger/doc.json` | Swagger 2 spec |

After changing a route or its annotations, run `make generate` and commit the specs. `make check-openapi` fails otherwise.

Go: see `go.mod`. Everything builds with `CGO_ENABLED=0`.

## Run

```sh
bin/bmcproxy --listen 127.0.0.1:8080
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--listen` | `127.0.0.1:8080` | listen address |
| `--allow-disruptive` | false | permit disruptive-tier operations (each request also needs `X-BMC-Confirm: true`) |
| `--debug-endpoints` | false | serve `/debug/*` to loopback clients and add debug blocks to error envelopes |
| `--swagger` | true | serve `/swagger/`, `/openapi.json` |
| `--session-ttl` | 5m | idle TTL of cached BMC sessions (keep below the BMC's own timeout) |
| `--session-max-age` | 0 | maximum session age (0 = unlimited) |
| `--target-ttl` | 10m | how long a detected driver per host is trusted |
| `--cache-ttl` | 2s | TTL of cached read results (0 = coalesce identical in-flight reads only) |
| `--global-concurrency` | 64 | fleet-wide cap on in-flight southbound requests |
| `--retry-attempts` | 3 | max southbound attempts for retryable failures |
| `--attempt-timeout` | 60s | per-attempt southbound timeout |
| `--breaker-threshold` / `--breaker-cooldown` | 5 / 30s | per-host circuit breaker |
| `--power-poll-interval` / `--power-poll-timeout` | 2s / 60s | power-action convergence polling |
| `--skew-warn` | 2m | BMC clock skew that produces a warning |
| `--shutdown-timeout` | 30s | drain time on SIGTERM before all BMC sessions are logged out |
| `--log-level`, `--log-format` | info, text | logging |
| `--debug`, `--trace-http`, `--har-out`, `--log-secrets` | | see [Debugging](#debugging) |
| `--version` | | print the version |

On SIGINT or SIGTERM the proxy stops accepting requests and drains in-flight ones. It then **logs out of every cached BMC session**: AMI BMCs have few session slots, and leaked sessions lock out the web UI.

## Request model

Every BMC route takes:

- `X-BMC-Host`: required. The BMC as `ip`, `host` or `host:port`.
- `Authorization: Basic ...`: required. The **BMC's** credentials. The proxy uses them to log in southbound and keys its session cache on them; they are never logged.
- `X-BMC-Vendor`: optional. Forces a driver (`asrock`, `redfish`) instead of auto-detection.
- `X-BMC-Confirm: true`: required for disruptive operations (see below).

```sh
B="-u admin:secret -H X-BMC-Host:10.0.0.42"

curl -s $B localhost:8080/api/v1/capabilities | jq    # driver, board, firmware, per-capability backend
curl -s $B localhost:8080/api/v1/system
curl -s $B localhost:8080/api/v1/power
curl -s $B localhost:8080/api/v1/sensors
curl -s $B 'localhost:8080/api/v1/events?since=1234'   # incremental SEL read
curl -s $B localhost:8080/api/v1/fru
curl -s $B localhost:8080/api/v1/inventory/memory      # cpus|memory|pcie|storage|baseboard
curl -s $B localhost:8080/api/v1/firmware

# writes
curl -s $B -X POST localhost:8080/api/v1/led   -d '{"state":"on"}'
curl -s $B -X POST localhost:8080/api/v1/power -d '{"action":"shutdown","timeoutSeconds":120}'
curl -s $B -X PUT  localhost:8080/api/v1/boot  -d '{"target":"pxe","mode":"once"}'
curl -s $B -X POST localhost:8080/api/v1/time/sync

# vendor routes: /api/v1/vendor/{driver}/{op}
curl -s $B localhost:8080/api/v1/vendor/asrock/bonding
curl -s $B 'localhost:8080/api/v1/vendor/asrock/fanTables?type=open&table=3'
curl -s $B localhost:8080/api/v1/vendor/asrock/raw/api/asrr/fw-info   # raw passthrough
```

Behaviour worth knowing:

- **Sessions.** One login per (host, user, driver) is reused across requests until it idles out. When a session expires or the BMC backend restarts, the proxy invalidates it and logs in again once, transparently.
- **Serialisation.** The AMI `/api` lane is one request at a time per host (the X470 web server falls over otherwise). Redfish allows 4 per host. The X470 fan-table read is a two-step select-then-read, held exclusively.
- **Power actions** return `200` with the converged state after polling. If the state never converges, they return `202` with a `warning` (for example, a soft-off that the host ignored mid-POST) instead of claiming success.
- **Clock skew.** Event timestamps carry both `raw` (BMC clock) and `corrected` (real UTC) values, and a skew warning shows up in `capabilities.warnings`.
- **Errors** use one envelope: `{"error":{"status","class","message","request_id"}}`. The `class` is stable (`auth`, `unsupported`, `upstream`, `busy`, `timeout`, `disruptive_denied`, ...).
- Southbound requests never carry `X-HTTP-Method-Override`, `X-HTTP-Method` or `X-Method-Override`.

The full contract is in `/openapi.json` and [docs/CONTRACTS.md](docs/CONTRACTS.md).

## Tiers and disruptive gating

Every operation has a tier:

| Tier | Examples | Gate |
| --- | --- | --- |
| read | all GETs | none |
| write | LED, boot override, power, time, fan mode/tables, keep-NIC-link-up | none |
| disruptive | enable/disable bonding, NC-SI mode, clear MCA log, BMC cold reset, firmware update, clearing the SEL, raw writes to sensitive paths | `--allow-disruptive` **and** `X-BMC-Confirm: true` |

Without both, a disruptive request gets `403 disruptive_denied` and nothing reaches the BMC.

Raw passthrough (`raw/api/...`, `raw/redfish/...`) is classified per method and path. A **denylist** answers `403` for paths that crash or leak on these BMCs, such as `logs/help-data`, `asrr/mca-file` and `GlobalLuaTables`, and those requests never reach the BMC. Paths are canonicalised (case, `./`, encoding) before the check.

## Debugging

| Knob | Effect |
| --- | --- |
| `BMCPROXY_DEBUG=transport,ami` (or `--debug`) | raise subsystems to DEBUG (`name=trace` for TRACE, `all`); `--help` lists the subsystems |
| `--trace-http` | dump every southbound HTTP request/response (redacted) |
| `--har-out DIR` | write one HAR file of southbound traffic per BMC host (redacted), viewable in browser devtools |
| `--log-secrets` | **dangerous**: disable redaction of passwords, cookies, CSRF tokens and X-Auth-Tokens in logs and HAR |
| `--debug-endpoints` | loopback-only `/debug/pprof`, `/debug/vars`, `/debug/levels` (GET/POST to change levels live), `/debug/targets`, `/debug/sessions`, `/debug/capabilities`, `/debug/stats`; error envelopes gain a `debug` block |

## Metrics and health

- `GET /api/v1/health`: liveness, version, uptime and registered drivers. Needs no auth.
- `GET /metrics`: Prometheus text format:
  - `bmcproxy_http_requests_total`, `bmcproxy_http_request_duration_seconds`
  - `bmcproxy_southbound_requests_total`, `bmcproxy_southbound_latency_seconds`
  - `bmcproxy_breaker_state`
  - `bmcproxy_sessions_live`, `bmcproxy_session_relogins_total`
  - `bmcproxy_cache_hits_total`, `bmcproxy_cache_hit_ratio`

## bmcprobe

```sh
bin/bmcprobe --host 10.0.0.42 detect                         # no credentials: unauthenticated detection only
BMC_PASS=... bin/bmcprobe --host 10.0.0.42 --user admin --password-env BMC_PASS detect
bin/bmcprobe --host 10.0.0.42 --user admin --password-stdin capabilities < passfile
bin/bmcprobe ... get sensors | power | events --since 1234 --limit 50 | system | fru |
                    inventory memory | network | users | services | time | firmware |
                    led | boot | restore-policy | audit
bin/bmcprobe ... --vendor redfish detect                     # force a driver
bin/bmcprobe ... --har-out /tmp/har --trace-http get power   # same debug flags as the daemon
```

Passwords are never accepted as flag values. Use `--password-env` or `--password-stdin`. `get` only reads. Exit codes: 0 ok, 1 operation failed, 2 usage error.

## Adding a vendor driver

1. Create `vendors/<name>/` with a type implementing `bmc.Driver` (`Name`, `Detect`, `Open`). Optionally also implement:
   - `bmc.CredDetector` (detection that needs credentials);
   - `bmc.Prioritized` (detection order; generic fallbacks are negative).

   `Detect` must be cheap and read-only, and return a zero `Match` for BMCs that aren't yours.
2. `Open` returns a `bmc.Session`. Implement the generic capability interfaces you support (`PowerController`, `SensorReader`, `EventLog`, `TimeManager`, ...) and report them from `Capabilities()`. Return `bmc.Unsupported(...)` for the rest.
3. Vendor-specific operations: implement `bmc.VendorRouter`. Routes mount under `/api/v1/vendor/<name>/`, each with a tier, and get swag-style annotations so `make generate` puts them in the OpenAPI spec.
4. Do all southbound HTTP through `pkg/transport` (per-host lanes, retries, breaker, redaction, HAR). Reuse `pkg/protocol/{redfish,amimegarac,ipmi}`. Wrap errors with `bmc.ErrAuth`, `ErrUpstream`, `ErrTimeout`, or `ErrSessionLost`; the last one makes the session manager re-login once.
5. Register from `init()` with `bmc.Register(...)`, then blank-import the package in `cmd/bmcproxy/drivers.go` and `cmd/bmcprobe/drivers.go`.
6. Test against `internal/testsim` (below). Use `vendors/asrock` as the worked example; its board quirks are data in `vendors/asrock/boards`.

## Testing

```sh
go test ./...                 # unit + simulator integration, no hardware
go test -race ./...
```

- **Unit tests** live next to the code. Protocol clients are tested against in-process fakes such as `pkg/protocol/redfish/redfishtest`.
- **Simulator integration.** `internal/testsim` replays the captured HARs as a stateful BMC simulator, with one profile for each of `StartX470` and `StartB650`. It models:
  - AMI login, CSRF and session slots;
  - mod_evasive 403s;
  - the date-time full-object PUT;
  - fan-table interleaving;
  - backend restarts;
  - power delays and ignored soft-offs;
  - clock offset;
  - a Redfish tree for the B650.

  It counts everything (`Stats()`), including in-flight `/api`, logins, logouts, method overrides and interleaves. `testsim.StartProxy` runs the real server, sessions, cache, handlers and transport in-process.

  `internal/testsim/integration` runs the production ASRock and generic Redfish drivers against both boards, with a fake IPMI dialer for the X470's IPMI-backed capabilities.
- **Live tests** (`//go:build live`, `internal/testsim/live`) hit one real BMC. They are read-only apart from an identify-LED round trip that restores the original state, and they skip unless all of `BMCPROXY_LIVE_HOST`, `BMCPROXY_LIVE_USER` and `BMCPROXY_LIVE_PASS` are set:

  ```sh
  BMCPROXY_LIVE_HOST=... BMCPROXY_LIVE_USER=... BMCPROXY_LIVE_PASS=... \
    go test -tags live -count=1 -v ./internal/testsim/live/
  ```

  Run them only against a BMC you are allowed to touch. They are never part of `go test ./...`.

### `x470_pam` build tag

X470D4U firmware keeps a separate Redfish account store, so the `/api` user gets 401 from Redfish. With `-tags x470_pam`, the ASRock driver opens an X470 session and then:

1. logs in to Redfish as a bootstrap admin;
2. sets the OEM `PAMEnabled: true` on `/redfish/v1/AccountService/Configurations` (with `If-Match`) and verifies it, so the caller's IPMI/web credentials work on Redfish as well.

This changes BMC configuration and is opt-in twice: the build tag, plus the environment variables below.

| Variable | Meaning |
| --- | --- |
| `BMCPROXY_X470_PAM_ADMIN_USER` | bootstrap Redfish admin (default `Administrator`) |
| `BMCPROXY_X470_PAM_ADMIN_PASS` | its password (required; unset = feature off) |

```sh
go build -tags x470_pam -o bin/ ./cmd/...
```

The manual equivalent is `scripts/x470d4u-redfish-pam-auth.sh`.

## Verified vs unverified behaviour

The ASRock driver is built from four kinds of evidence:

- live probes of one X470D4U 2.32 and one B650D4U 4.10;
- browser HAR captures;
- the web UI JavaScript in the firmware images;
- the vendor Redfish guide.

Anything not seen working on hardware carries an `// UNVERIFIED:` comment in the code. The simulator tests prove the proxy does what the code intends. They cannot prove the BMC agrees.

**Confirmed on hardware** (live probe or HAR):

- Detection via `asrr/model-name` and `asrr/fw-info`. The X470 Redfish rejects `/api` users (without `x470_pam`).
- AMI login, CSRF and session handling; logout. The B650 4.10 logs in without `certlogin`.
- One-at-a-time `/api` on the X470; mod_evasive limits on the B650.
- X470 power `power_command` 0 (off), 1 (on) and 5 (soft off). Codes 2 and 3 come from the UI JS.
- X470 identify LED `uid_command` 0 (off) and 2 (on).
- X470 `getfancontrolmode` shape (array of `{mode1..mode16}`, all 0). A same-value `setfancontrolmode` write returned 200. The open/closed-loop table-number selects returned 200.
- B650 4.10 `settings/fan/mode` (GET/PUT); B650 Redfish paging (`$skip=50` nextLink without `$expand`; the proxy re-applies `$expand`).
- B650 `settings/network-bond` PUT body (from the HAR).
- SEL, sensors, FRU, inventory, network, users, services and firmware reads on both boards.

**Corrected from evidence during integration:**

- X470 `PUT settings/date-time` now omits the response-only `id` and `localized_timestamp` fields. The live board answered 500 to a verbatim echo of the GET object (`docs/x470/scripts/rt.py`, `docs/x470/endpoint-reference.json`).
- B650 fan-table writes go to the item, `PUT settings/fan/{openloop,closedloop}/{table}`, not the GET-only collection (`docs/api/legacy-api-endpoints.json` item_path).
- Fan-table numbers are range-checked against the UI's allowed values: open loop 1..8, closed loop 1..10.

**Still UNVERIFIED** (implemented from the UI JS or by analogy; never exercised on hardware):

- Identify-LED `uid_command` 1 (timed blink).
- X470 NMI via `power_command` 4 (IPMI "pulse diagnostic interrupt").
- The power code encoding on the B650 `/api`, assumed the same as the X470's. The B650 normally uses Redfish for power.
- `PUT settings/date-time` succeeding with the trimmed body. Also, what `ntp_auto_date` value 2 means on the B650 4.10.
- Writing X470 fan tables (`setfanopenloopcontroltable` / `setfancloseloopcontroltable`), and fan-mode codes other than 0.
- B650 fan-table item PUT (`{id}` being the table number).
- `asrr/{Get,Set}PowerRestore` `powerRestore` values (assumed to follow IPMI encoding).
- Keep-NIC-link-up:
  - X470 `PUT asrr/setvetobit {vetobit}`;
  - B650 4.10 `settings/ncsilinkup` (seen only in the 7.02/9.05 builds).
- NC-SI mode codes (0 auto failover / 1 manual) and bonding mode values.
- `asrr/maintenance/BIOS/UpdateOption` value encoding.

When you verify one of these on hardware, record the evidence under `docs/` and remove the `UNVERIFIED` comment. If a run with `--har-out` captures it, keep the HAR.
