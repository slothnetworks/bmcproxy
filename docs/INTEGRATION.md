# bmcproxy integration guide and API specification

This document is for teams building **external systems** that talk to `bmcproxy`: provisioning and orchestration tools, inventory/CMDB sync jobs, monitoring and alerting pipelines, and operator dashboards. It specifies the northbound HTTP API (`/api/v1`) and the behaviour a client must rely on.

- The machine-readable contract is the OpenAPI 3 document served at **`GET /openapi.json`** (committed as `internal/apidocs/openapi.json`). Generate clients from it. Where this guide and the spec disagree on a field shape, the spec is authoritative; please report the discrepancy.
- This guide adds what the spec cannot express: retry semantics, caching, cursors, safety gating, sessions, and integration patterns.
- Driver authors want [CONTRACTS.md](CONTRACTS.md) instead. Operators running the daemon want the [README](../README.md).

Contents:

1. [Model](#1-model)
2. [Quick start](#2-quick-start)
3. [Request model](#3-request-model)
4. [Conventions](#4-conventions)
5. [Safety tiers](#5-safety-tiers)
6. [Errors and retries](#6-errors-and-retries)
7. [Caching, coalescing and consistency](#7-caching-coalescing-and-consistency)
8. [Concurrency, timeouts and load](#8-concurrency-timeouts-and-load)
9. [Sessions and credentials](#9-sessions-and-credentials)
10. [Discovery: health, status and capabilities](#10-discovery-health-status-and-capabilities)
11. [Generic endpoint reference](#11-generic-endpoint-reference)
12. [Data types](#12-data-types)
13. [Vendor extensions](#13-vendor-extensions)
14. [Raw passthrough](#14-raw-passthrough)
15. [Integration recipes](#15-integration-recipes)
16. [Operating against the proxy](#16-operating-against-the-proxy)
17. [Hardware coverage and verification status](#17-hardware-coverage-and-verification-status)
18. [Integration checklist](#18-integration-checklist)

---

## 1. Model

```
 your system ──HTTP/JSON──▶ bmcproxy ──Redfish / AMI /api / IPMI──▶ BMC (ASRock Rack, Gigabyte, generic Redfish)
              /api/v1/...            per-host lanes, sessions,
              X-BMC-Host             retries, circuit breaker
              Basic (BMC creds)
```

- **Stateless northbound.** The proxy stores no inventory of BMCs and no credentials. Every request names the BMC (`X-BMC-Host`) and carries that BMC's own credentials (`Authorization: Basic`). There is no registration step.
- **One vendor-neutral shape.** `GET /api/v1/sensors` returns the same JSON whether the board answered over Redfish, the AMI `/api`, or IPMI. Vendor leftovers go in `oem` (generic routes) or `raw` (vendor routes).
- **Driver auto-detection.** The proxy probes the BMC and picks a driver: `gigabyte`, `asrock`, or the generic `redfish` fallback. You can force one with `X-BMC-Vendor`.
- **Capability-based.** Not every board supports every operation. An unsupported operation is a clean `501 unsupported`, never a guess. Ask `GET /api/v1/capabilities` first.
- **Honest writes.** Power actions are polled until the observed state matches; a write that the BMC silently ignored is reported as `202` with a `warning`, not `200`.

---

## 2. Quick start

```sh
P=http://bmcproxy.internal:8080
H=(-u admin:BMC_PASSWORD -H 'X-BMC-Host: 10.0.0.42')

curl -s $P/api/v1/health                                  # proxy alive? (no auth, no BMC contact)
curl -s "${H[@]}" $P/api/v1/capabilities | jq             # driver, board, firmware, what is supported
curl -s "${H[@]}" $P/api/v1/system | jq                   # identity
curl -s "${H[@]}" $P/api/v1/power                         # {"state":"on"}
curl -s "${H[@]}" $P/api/v1/sensors | jq '.sensors[] | select(.health!="ok")'
curl -s "${H[@]}" "$P/api/v1/events?since=0&limit=100"    # SEL, incremental

# PXE boot once, then power-cycle and wait for the host to come back on
curl -s "${H[@]}" -X PUT  $P/api/v1/boot  -d '{"target":"pxe","mode":"once"}'
curl -s "${H[@]}" -X POST $P/api/v1/power -d '{"action":"cycle","timeoutSeconds":120}'
```

Import `GET /openapi.json` into Postman, Bruno or Insomnia: every request is pre-wired with an `X-BMC-Host: {{bmcHost}}` header and Basic auth, so you define `bmcHost` and the credentials once per environment.

---

## 3. Request model

### 3.1 Base URL and transport

| Item | Value |
|---|---|
| Base path | `/api/v1` (generic and vendor routes), plus `/openapi.json`, `/swagger/`, `/metrics` |
| Protocol | Plain HTTP. The proxy does not terminate TLS itself. |
| Default listen | `127.0.0.1:8080` (operator-configurable) |
| Content type | `application/json; charset=utf-8` for requests with bodies and for every proxy-generated response |

> **Security.** The proxy has no authentication of its own. Anyone who can reach it can act on any BMC whose credentials they hold, and the BMC credentials travel in the `Authorization` header. Put it behind a TLS-terminating, authenticating reverse proxy (mTLS, OIDC, or a service mesh) or keep it on loopback. Do not expose it to untrusted networks.

### 3.2 Headers

| Header | Direction | Required | Meaning |
|---|---|---|---|
| `X-BMC-Host` | request | yes, on every BMC route | The BMC: `ip`, `host`, `host:port`, or `[ipv6]:port`. Port defaults to 443. |
| `Authorization: Basic base64(user:pass)` | request | yes, on every BMC route | The **BMC's own** account. Used to log in southbound; never logged, never persisted. A missing or malformed header is `401 auth` before any BMC contact. |
| `X-BMC-Vendor` | request | no | Force a driver: `asrock`, `gigabyte`, `redfish`. An unknown name is `400`. Omit it normally; auto-detection is the tested path. |
| `X-Request-Id` | both | no | Your correlation id. Accepted if 1–128 chars of `[A-Za-z0-9-_.:]`; otherwise the proxy generates one. Always echoed in the response and in `error.request_id`. Send one and log it: it is the key for finding the request in proxy logs and HAR captures. |
| `Content-Type: application/json` | request | for JSON bodies | Required on `POST /firmware/update` (which also accepts `application/octet-stream`); recommended everywhere. |
| `Cache-Control: no-store`, `X-Content-Type-Options: nosniff` | response | — | Set on every JSON response. Do not cache proxy responses in intermediaries. |

Routes that do **not** need `X-BMC-Host` or credentials: `GET /api/v1/health`, `GET /api/v1/bmcs/{host}/status`, `GET /metrics`, `GET /openapi.json`, `/swagger/`.

### 3.3 Request bodies

- JSON bodies are decoded **strictly**: an unknown field is `400 invalid_argument`. Send only the fields documented here or in the spec. (This is deliberate: a typo such as `"timeout"` for `"timeoutSeconds"` must not be silently ignored on a power operation.)
- Body size cap: **1 MiB** by default. Firmware push uploads are uncapped.
- Most bodies are decoded after the proxy has resolved the driver and opened the BMC session. If the BMC is unreachable you get the `5xx` for that first, before any body error. Validate request bodies on your side as well.
- Optional fields: omit them. Do not send `null` for a field you want unchanged unless the schema says `null` means unchanged.

---

## 4. Conventions

| Topic | Rule |
|---|---|
| Field names | lowerCamelCase in generic routes (`bmcFirmware`, `timeoutSeconds`). The error envelope uses snake_case (`request_id`, `upstream_status`). |
| Timestamps | RFC 3339 in UTC, e.g. `2026-11-17T03:14:15Z`. Fields documented `omitzero` are absent when unknown. |
| Clock-skew-aware times | SEL and audit entries carry `time: {raw, corrected}`. `raw` is what the BMC said (its clock may be wrong); `corrected` is `raw − offset`, i.e. real UTC. Use `corrected` for ordering and alert correlation. See [§15.3](#153-clock-skew). |
| Missing values | Numeric readings and thresholds are `null` when the BMC reports "NA" or nothing. Do not treat `null` as `0`. |
| Collections | List fields are always present as JSON arrays (`[]` when empty), never `null` or missing. Example: `{"sensors":[]}`. |
| Enums | Lowercase/lowerCamel strings (`on`, `poweringOff`, `lastState`). `unknown` is a valid value of every state enum: handle it. Treat unrecognised enum values as `unknown` (new values may be added). |
| Vendor extras | `oem` (generic types) and `raw` (vendor routes) hold the BMC's original object. Their content is board- and firmware-specific and **not** part of the stable contract. |
| Success without body | `204 No Content` for writes that return nothing. |
| Unknown route | `404` with the standard error envelope (class `not_found`), including a wrong method on a known path. |
| Forward compatibility | Ignore response fields you do not recognise; new optional fields are added without a version bump. |

---

## 5. Safety tiers

Every operation has a tier. The tier is published per route in `/openapi.json` descriptions and, for vendor routes, in `GET /api/v1/capabilities` → `vendorRoutes[].tier`.

| Tier | Meaning | Examples |
|---|---|---|
| `read` | Never changes BMC state. | every `GET` |
| `write` | Changes state, reversibly, without cutting BMC connectivity. | power, LED, boot override, restore policy, time, fan modes |
| `disruptive` | May cut BMC connectivity, reboot the BMC, or destroy data. | network/bond changes, NC-SI mode, BMC reset, SEL clear, MCA log clear, firmware update, sensitive raw writes |

The proxy does **not** gate on tier: there is no server switch and no confirmation header, and a disruptive request is sent to the BMC like any other. Your application is responsible for deciding who may run disruptive operations and when; use the tier to drive that policy (approvals, UI warnings, change windows).

Note that `write` is not harmless: `POST /power {"action":"off"}` is a `write`. Gate power actions in your own system as you see fit.

**After a disruptive network or BMC operation** the BMC may vanish for 1–3 minutes, and may come back on a different address. Expect `502`/`503`/`504` in that window (see [§15.6](#156-disruptive-network-changes)).

---

## 6. Errors and retries

### 6.1 Envelope

Every error, from any route, has this shape:

```json
{
  "error": {
    "status": 501,
    "class": "unsupported",
    "message": "api.v1 BootOverride: capability not supported",
    "request_id": "01J9Z3V8Q4K2M5N7P0R6S8T1W3",
    "debug": { "backend": "ami", "upstream_status": 401, "op": "GET /api/chassis-status", "pkg": "amimegarac", "detail": "{\"cc\":7}" }
  }
}
```

- `status` mirrors the HTTP status.
- **`class` is the stable field to branch on.** Messages are for humans and may change.
- `debug` appears only when the operator runs the proxy in debug mode. Never depend on it.
- Messages are redacted (passwords, tokens, cookies never appear).

### 6.2 Classes and what to do

| HTTP | `class` | Cause | Client action |
|---|---|---|---|
| 400 | `invalid_argument` | Malformed header, host, body, unknown field, bad enum, out-of-range value. | Fix the request. Do not retry. |
| 401 | `auth` | Missing/malformed `Authorization`, or the BMC rejected the credentials. | Do not retry with the same credentials (AMI BMCs lock accounts after repeated failures). Surface to an operator / credential store. |
| 403 | `forbidden` | Proxy policy: a raw passthrough path on the denylist. | Do not retry. The path is blocked because it crashes or leaks on these BMCs. |
| 404 | `not_found` | Unknown route, unknown inventory kind, unknown task/slot id. | Fix the request. |
| 499 | `canceled` | Your client disconnected before the answer. | (Only seen in proxy logs/metrics.) |
| 500 | `internal` | Proxy bug. | Retry once; report with the `request_id`. |
| 501 | `unsupported` | This board/firmware/driver cannot do this operation. | Do not retry. Cache it per host (until firmware changes) and hide the feature. |
| 502 | `upstream` | The BMC answered with an error or garbage, or the connection failed. | Retry reads with backoff. For writes, **re-read state first** (see below). |
| 503 | `busy` | BMC rate-limited the proxy, too many in-flight requests, or the per-host circuit breaker is open. | Retry with backoff (≥ several seconds). An open breaker stays open ~30 s by default. |
| 504 | `timeout` | The BMC did not answer within the proxy's per-attempt timeout (60 s default) after retries. | Retry later with backoff. The BMC may be rebooting or wedged. |

The proxy **already retries** idempotent reads (on `503`, connection resets, and timeouts, with jittered exponential backoff and `Retry-After` honoured) and **re-logs-in once** when a cached session expires. So by the time you see a `5xx`, the proxy has done its retries. Add your own retry at a coarser timescale (seconds to minutes), not a tight loop.

**Writes are not blindly retried** by the proxy, because a write that timed out may have taken effect. After a `502`/`504` on a write, `GET` the affected resource before deciding to resend. All generic writes here are idempotent in effect (setting the same LED/boot/policy twice is harmless) except power actions: re-check `GET /power` before resending `cycle`, `reset` or `nmi`.

---

## 7. Caching, coalescing and consistency

To protect fragile BMCs (the X470's web server falls over under concurrent load), the proxy **coalesces and briefly caches reads**:

- Identical concurrent reads (same host, same BMC user, same vendor hint, same operation and query parameters) become **one** BMC call; all callers get its result.
- The result is kept for a short TTL (**2 s** default, operator-configurable with `--cache-ttl`; `0` disables storing but keeps coalescing).
- **Any write through the proxy to a host drops that host's cached reads**, even if the write failed. A `GET` after your write never returns pre-write data from the cache.
- Not cached at all: `GET /time` (coalesced only), `GET /kvm`, `GET /firmware/tasks/{id}`, `GET /capabilities`, and raw passthrough.

Consequences for clients:

- Polling the same endpoint faster than the TTL returns the same data. Poll sensors every ≥ 10 s; there is no benefit below the TTL.
- Changes made **outside** the proxy (web UI, another tool, the host OS via IPMI in-band) may be invisible for up to one TTL.
- Many dashboards polling the same BMC cost the BMC one request per TTL, not one per dashboard. Point your consumers at the proxy rather than at BMCs directly.

---

## 8. Concurrency, timeouts and load

### 8.1 What the proxy enforces southbound

| Limit | Default | Effect on you |
|---|---|---|
| AMI `/api` lane | 1 in-flight request per BMC | Requests to the same ASRock/Gigabyte BMC that need `/api` queue behind each other. |
| Redfish lane | 4 in-flight per BMC | |
| Global cap | 64 in-flight southbound requests across all BMCs | Fleet-wide fan-out beyond this queues. |
| Per-attempt timeout | 60 s | |
| Attempts | 3 (reads; writes only when safe) | |
| Circuit breaker | opens after 5 consecutive failures, for 30 s | Requests to that host fail fast with `503 busy` while open. |

Queued requests wait; they do not fail. Writes get priority over reads in a host's queue.

### 8.2 What your client should do

- **HTTP client timeout:** at least **200 s** for ordinary calls (a read can legitimately spend 3 attempts × 60 s plus backoff on a struggling BMC). For `POST /power` with waiting, use `timeoutSeconds + 90 s`.
- **Per-BMC concurrency:** send at most a handful of concurrent requests to one BMC. More just queue inside the proxy and inflate your latency.
- **Fleet fan-out:** a bounded worker pool (e.g. 32–64 workers across hosts) matches the proxy's global cap. Spread periodic jobs with jitter rather than firing every host at the top of the minute.
- **Cancellation:** closing the connection cancels the request's queued southbound work.
- **Breaker awareness:** on `503 busy`, back off that host specifically; other hosts are unaffected.

---

## 9. Sessions and credentials

- The proxy logs in to each BMC on first use and **reuses the session** for later requests with the same host + user + password (+ vendor hint). Sessions idle out after 5 min by default (operator `--session-ttl`) and are logged out cleanly on proxy shutdown.
- An expired session or a restarted BMC backend is handled transparently: the proxy re-logs in once and retries.
- **AMI BMCs have very few session slots** (often 4–5 shared with the web UI). Each distinct username you use against a BMC holds one. Use **one service account per BMC** for automation; do not rotate between several accounts.
- **Rotating a password:** after the BMC password changes, simply send the new one. The old session is keyed on the old password, so it is not reused, and it idles out on its own.
- **Repeated `401`s lock accounts** on many AMI firmwares. Stop after the first `401` for a given credential and alert.
- `X-BMC-Vendor` is part of the session key: mixing hinted and unhinted requests to the same BMC opens two sessions. Be consistent.
- The proxy never returns passwords (`GET /users` lists accounts without them).

---

## 10. Discovery: health, status and capabilities

### 10.1 `GET /api/v1/health`: proxy liveness

No auth, no BMC contact. Use it for load-balancer/Kubernetes probes.

```json
{
  "status": "ok",
  "version": "1.0.0",
  "goVersion": "go1.27.3",
  "startedAt": "2026-11-17T00:00:00Z",
  "uptimeSeconds": 3600.5,
  "drivers": [
    {"name": "gigabyte", "priority": 10, "vendorRoutes": 10},
    {"name": "asrock",   "priority": 0,  "vendorRoutes": 28},
    {"name": "redfish",  "priority": -100, "vendorRoutes": 0}
  ],
  "sessions": 3,
  "targets": 2
}
```

### 10.2 `GET /api/v1/bmcs/{host}/status[?probe=true]`: per-BMC status

No auth. Built from what the proxy already knows (target cache, transport stats); it does **not** contact the BMC unless `probe=true`, which runs unauthenticated, read-only detection. `{host}` is `ip`, `host` or `host:port` (URL-encode IPv6 brackets).

Key fields: `known`, `reachable` (`null` = never contacted), `vendor`, `board`, `firmware`, `backends[]`, `sessions`, `circuit` (`closed|half-open|open`), `clockOffsetSeconds`, `degraded`/`degradedUntil`, `lastSeen`, `lastError`/`lastErrorClass`/`lastErrorAt`, `tls`, `warnings[]`, and a summary `health` (`ok|warning|critical|unknown`).

Use it for a cheap fleet health view that costs the BMCs nothing.

### 10.3 `GET /api/v1/capabilities`: what this BMC supports

Authenticated. Resolves the driver, opens (or reuses) a session, and reports:

```json
{
  "host": "10.0.0.42:443",
  "driver": "asrock",
  "board": "B650D4U",
  "firmware": "4.10.00",
  "backends": [
    {"name": "redfish", "available": true, "authenticated": true},
    {"name": "ami",     "available": true, "authenticated": true}
  ],
  "degraded": "",
  "capabilities": [
    {"capability": "power",        "supported": true,  "backend": "redfish"},
    {"capability": "bootOverride", "supported": true,  "backend": "redfish"},
    {"capability": "bond",         "supported": true,  "backend": "ami"},
    {"capability": "firmwareUpdate", "supported": false}
  ],
  "vendorRoutes": [
    {"method": "GET",  "path": "/api/v1/vendor/asrock/fanMode", "tier": "read",  "summary": "..."},
    {"method": "",     "path": "/api/v1/vendor/asrock/raw/api/{path...}", "tier": "read", "dynamicTier": true}
  ],
  "clockOffsetSeconds": 12.5,
  "warnings": ["BMC clock is 2h0m0s behind the proxy; correct it with POST /api/v1/time/sync or configure NTP via PUT /api/v1/time"]
}
```

Capability names and the routes they gate:

| `capability` | Routes |
|---|---|
| `identity` | `GET /system` |
| `power` | `GET`, `POST /power` |
| `powerRestorePolicy` | `GET`, `PUT /power/restore-policy` |
| `indicatorLed` | `GET`, `POST /led` |
| `bootOverride` | `GET`, `PUT /boot` |
| `sensors` | `GET /sensors` |
| `eventLog` | `GET /events` |
| `eventLogClear` | `DELETE /events` |
| `auditLog` | `GET /logs/audit` |
| `fru` | `GET /fru` |
| `inventory` | `GET /inventory/{kind}` (individual kinds may still be 501) |
| `network` | `GET`, `PUT /network` |
| `bond` | `GET`, `PUT /network/bond` |
| `users` | `GET /users` |
| `services` | `GET /services` |
| `time` | `GET`, `PUT /time`, `POST /time/sync` |
| `firmwareInventory` | `GET /firmware` |
| `firmwareUpdate` | `POST /firmware/update`, `GET /firmware/tasks/{id}` |
| `virtualMedia` | `GET /virtual-media`, `POST /virtual-media/{id}/insert|eject` |
| `kvm` | `GET /kvm` |
| `bmcReset` | `POST /bmc/reset` |

`supported: false` means the route returns `501`. `backend` tells you which protocol serves it (`redfish`, `ami`, `ipmi`): useful when diagnosing, never needed for correctness. `degraded` is non-empty when the session works in a reduced mode (e.g. Redfish auth failed and the driver fell back to `/api`).

Cache the capabilities per host and refresh when `firmware` changes or after a BMC reset/firmware update.

---

## 11. Generic endpoint reference

All paths are under `/api/v1`. Every route here requires `X-BMC-Host` and `Authorization: Basic`. Every route can additionally answer `400`, `401`, `501`, `502`, `503` and `504` as described in [§6](#6-errors-and-retries); only route-specific statuses are listed.

### Summary

| Method | Path | Tier | Success |
|---|---|---|---|
| GET | `/capabilities` | read | 200 |
| GET | `/system` | read | 200 |
| GET | `/power` | read | 200 |
| POST | `/power` | write | 200 converged / 202 not observed |
| GET | `/power/restore-policy` | read | 200 |
| PUT | `/power/restore-policy` | write | 204 |
| GET | `/led` | read | 200 |
| POST | `/led` | write | 204 |
| GET | `/boot` | read | 200 |
| PUT | `/boot` | write | 204 |
| GET | `/sensors` | read | 200 |
| GET | `/events` | read | 200 |
| DELETE | `/events` | **disruptive** | 204 |
| GET | `/logs/audit` | read | 200 |
| GET | `/fru` | read | 200 |
| GET | `/inventory/{kind}` | read | 200 |
| GET | `/network` | read | 200 |
| PUT | `/network` | **disruptive** | 204 |
| GET | `/network/bond` | read | 200 |
| PUT | `/network/bond` | **disruptive** | 204 |
| GET | `/users` | read | 200 |
| GET | `/services` | read | 200 |
| GET | `/time` | read | 200 |
| PUT | `/time` | write | 200 (re-read) / 204 |
| POST | `/time/sync` | write | 200 |
| GET | `/firmware` | read | 200 |
| POST | `/firmware/update` | **disruptive** | 202 |
| GET | `/firmware/tasks/{id}` | read | 200 |
| GET | `/virtual-media` | read | 200 |
| POST | `/virtual-media/{id}/insert` | write | 204 |
| POST | `/virtual-media/{id}/eject` | write | 204 |
| GET | `/kvm` | read | 200 |
| POST | `/bmc/reset` | **disruptive** | 202 |

### 11.1 System identity

**`GET /system`** → [`SystemInfo`](#systeminfo)

```json
{"vendor":"asrock","manufacturer":"ASRockRack","board":"B650D4U","model":"B650D4U","serial":"M80-...","uuid":"...",
 "bmcFirmware":"4.10.00","biosVersion":"10.02","bmcMac":"9c:6b:00:..","hostname":"node42","powerState":"on","health":"ok"}
```

`vendor` is the driver name. Prefer `serial` + `uuid` as the host's stable identity in a CMDB; on Gigabyte MC13-LE0 the proxy substitutes the real board serial for the FRU placeholder.

### 11.2 Power

**`GET /power`** → `{"state": "on"}`. `state` ∈ `on | off | poweringOn | poweringOff | unknown`.

**`POST /power`**

Request:

| Field | Type | Required | Notes |
|---|---|---|---|
| `action` | string | yes | `on`, `off` (hard), `shutdown` (graceful ACPI, may be ignored by the OS), `cycle` (off then on), `reset` (hard reset), `nmi` (diagnostic interrupt) |
| `wait` | bool | no | Default `true`: poll until the expected state is reached. Also `?wait=false`. |
| `timeoutSeconds` | number | no | Polling budget. Default 60 (operator `--power-poll-timeout`), capped at 600. `0` means don't wait. |

Response [`PowerActionResult`](#poweractionresult):

```json
{"action":"shutdown","expected":"off","state":"off","converged":true,"polled":true,"polls":3,"waitedSeconds":6.1,"timeoutSeconds":60}
```

| Status | Meaning |
|---|---|
| **200** | The observed power state reached `expected`. |
| **202** | The BMC accepted the action but the proxy did not observe the expected state: `converged:false` with a `warning` (e.g. the OS ignored an ACPI shutdown mid-POST), or `wait=false`, or `nmi` (nothing to poll). |

Expected states: `on`/`cycle`/`reset` → `on`; `off`/`shutdown` → `off`; `nmi` → none.

Treat `202` as **"not confirmed"**, not as failure. For a graceful-then-hard shutdown, send `shutdown` with a timeout, and on `202` send `off`. Board notes: `cycle` and `nmi` are `501` on the Gigabyte MZ32-AR0 (its Redfish lacks those reset types).

**`GET /power/restore-policy`** / **`PUT /power/restore-policy`**: body `{"policy": "alwaysOn" | "alwaysOff" | "lastState"}` (`unknown` may be returned, never accepted). PUT → 204.

### 11.3 Identify LED

**`GET /led`** → `{"state": "on" | "off" | "blinking" | "unknown"}`
**`POST /led`** with `{"state": "on" | "off" | "blinking"}` → 204. Use it for "find this server" flows; remember to turn it off.

### 11.4 Boot override

**`GET /boot`** / **`PUT /boot`** with [`BootOverrideConfig`](#bootoverrideconfig):

| Field | Required on PUT | Values |
|---|---|---|
| `target` | yes | `none`, `pxe`, `disk`, `cd`, `usb`, `bios` (enter firmware setup), `floppy` |
| `mode` | yes | `once` (next boot only), `continuous`, `disabled` |
| `bootMode` | no | `uefi`, `legacy`, or omitted (unchanged) |

PUT → 204. The override applies on the **next** boot; follow with a power action. No AMI `/api` endpoint exists for this, so it is served over Redfish or IPMI.

### 11.5 Sensors

**`GET /sensors`** → `{"sensors": [Sensor, ...]}`

```json
{"id":"cpu_temp","number":1,"name":"CPU Temp","type":"temperature","reading":46.0,"unit":"C","health":"ok",
 "thresholds":{"lowerNonRecoverable":null,"lowerCritical":null,"lowerNonCritical":null,
               "upperNonCritical":85,"upperCritical":90,"upperNonRecoverable":95},
 "backend":"ami"}
```

- `type` ∈ `temperature | voltage | fan | current | power | discrete | other`.
- `unit` normalized: `C`, `V`, `RPM`, `A`, `W`, `%`.
- `reading: null` = not available (sensor absent, host off, or no data). Fans and CPU temps commonly read `null` with the host off.
- `health` ∈ `ok | warning | critical | unknown`, computed by the BMC/driver. Alert on `health`, not on your own threshold math, unless you need custom limits.
- `id` is stable for a given board/firmware/driver; use `(host, id)` as the time-series key.

### 11.6 System event log (SEL)

**`GET /events?since=<lastId>&limit=<n>`** → [`EventsResponse`](#eventsresponse)

| Query | Meaning |
|---|---|
| `since` | Return only entries **after** this record id. Omit (or start from your stored cursor) for the full log. |
| `limit` | Maximum entries (non-negative integer; `0`/omitted = driver default, typically all). |

```json
{
  "entries": [
    {"id":"41","time":{"raw":"2026-11-17T01:00:00Z","corrected":"2026-11-17T03:00:00Z"},
     "severity":"critical","sensorType":"Temperature","sensorName":"CPU Temp","sensorNumber":1,
     "eventType":"Threshold","direction":"assertion","message":"Upper Critical going high","eventData":"0957ff","backend":"ami"}
  ],
  "lastId": "41",
  "more": false,
  "clockOffsetSeconds": -7200
}
```

- `lastId`: store it and pass it as `since` next time. It is an opaque string; do not do arithmetic on it.
- `more: true`: `limit` truncated the page; call again immediately with `since=lastId`.
- `direction` ∈ `assertion | deassertion` when known.
- `clockOffsetSeconds`: the measured BMC-minus-proxy offset used to compute `time.corrected`; absent when unknown (then `corrected` is best effort).

See [§15.2](#152-event-ingestion-sel-tailing) for the tailing loop.

**`DELETE /events`**: clears the SEL. **Disruptive** (data loss). → 204. Reset your stored cursor afterwards.

### 11.7 Audit log

**`GET /logs/audit?since=&limit=`** → `{"entries":[AuditEntry...],"clockOffsetSeconds":...}`. Entry: `id`, `time{raw,corrected}`, `user`, `source` (client IP/interface), `facility`, `severity`, `message`. Same cursor semantics as events, but there is no `lastId`/`more`: use the last entry's `id`.

### 11.8 FRU and inventory

**`GET /fru`** → `{"fru":[FRU...]}`, each with `id`, `name`, `chassis{type,partNumber,serial,extra[]}`, `board{mfgDate,manufacturer,product,serial,partNumber,fruFileId,extra[]}`, `product{manufacturer,name,partNumber,version,serial,assetTag,fruFileId,extra[]}`.

**`GET /inventory/{kind}`**, `kind` ∈ `cpus | memory | pcie | storage | baseboard`. The response always contains `kind` and exactly the matching collection, which is `[]` when empty (e.g. X470 with the host off):

| kind | Response field | Item fields |
|---|---|---|
| `cpus` | `cpus[]` | `id, socket, manufacturer, model, cores, threads, maxSpeedMHz, present, health` |
| `memory` | `memory[]` | `id, slot, manufacturer, partNumber, serial, sizeMiB, speedMTs, type, present, health` |
| `pcie` | `pcie[]` | `id, slot, name, manufacturer, vendorId, deviceId, subsystemVendorId, subsystemId, classCode, linkWidth, linkSpeed, health` (ids hex, e.g. `"0x8086"`) |
| `storage` | `storage[]` | `id, name, manufacturer, model, serial, firmware, capacityBytes, mediaType (ssd/hdd), protocol (nvme/sata/sas), health` |
| `baseboard` | `baseboard{}` | `manufacturer, product, serial, partNumber, version, biosVendor, biosVersion, biosDate` |

Unknown `kind` → 404. A kind the board cannot serve → 501. Host inventory on these boards is collected by the BIOS at POST, so it may be empty or stale while the host is off or has never booted since a BMC reset.

### 11.9 BMC network

**`GET /network`** → `{"interfaces":[NetworkInterface...]}`: `id, name (eth0/bond0), mac, enabled, linkUp, speedMbps, ipv4{dhcp,address,netmask,gateway}, ipv6{enabled,dhcp,addresses[] (CIDR),gateway}, vlan{enabled,id}, dns[], hostname`.

**`PUT /network`** (**disruptive**) with [`NetworkInterfaceUpdate`](#networkinterfaceupdate): `name` required; any of `enabled`, `ipv4`, `ipv6`, `vlan`, `dns`; omitted fields unchanged. The driver reads the full vendor object, merges and writes it back. → 204. The BMC may drop off the network or move address; plan accordingly ([§15.6](#156-disruptive-network-changes)).

**`GET /network/bond`** → [`BondResponse`](#bondresponse):

```json
{"id":1,"enabled":true,"mode":"active-backup","interfaces":["both"],"autoConfig":true,"oem":{...}}
```

**`PUT /network/bond`** (**disruptive**):

| Field | Required | Notes |
|---|---|---|
| `enabled` | yes | |
| `mode` | no | Default current. Gigabyte: not configurable (must be empty or equal to the current mode). |
| `interfaces` | no | At most **one** entry: the BMC's `bond_ifc`. ASRock B650: `eth0`/`eth1` (required there to un-bond: the interface that keeps the address). Gigabyte: `eth0` (shared), `eth1` (dedicated), `both` (failover; only while enabled). |
| `autoConfig` | no | Default current. |

→ 204. Served by the AMI `/api` on ASRock and Gigabyte; 501 on Redfish-only targets.

### 11.10 Accounts and services

**`GET /users`** → `{"users":[{"id","name","enabled","privilege","locked","email"}...]}`. `privilege` ∈ `administrator | operator | user | callback | noAccess | oem`. Never includes passwords. Read-only in v1.

**`GET /services`** → `{"services":[{"name","enabled","port","securePort","timeoutSeconds","maxSessions","activeSessions","interfaces[]"}...]}`. Useful for security audits (is IPMI-over-LAN or SSH enabled?).

### 11.11 Time

**`GET /time`** → [`TimeConfig`](#timeconfig):

```json
{"bmcTime":"2026-11-17T01:00:05Z","proxyTime":"2026-11-17T03:00:00Z","offsetSeconds":-7195,
 "timezone":"UTC","utcOffsetMinutes":0,"ntpEnabled":false,"ntpServers":["pool.ntp.org"]}
```

Reading it also refreshes the proxy's stored offset for this host (used to correct event times).

**`PUT /time`** with any of `time` (RFC 3339, manual set), `ntpEnabled`, `ntpServers` (primary first). At least one is required (else 400). Timezone is preserved. Returns `200` with the **re-read** config so you can verify, or `204` if the re-read failed. NTP changes can restart BMC networking briefly.

**`POST /time/sync`** (no body): sets the BMC clock to the proxy's current UTC → `200` with the resulting `TimeConfig`. Prefer configuring NTP; use this to fix skew on BMCs that cannot reach NTP.

### 11.12 Firmware

**`GET /firmware`** → `{"components":[{"id","name","version","updateable","releaseDate","health"}...]}`. `name` is e.g. `BMC`, `BIOS`, `CPLD`.

**`POST /firmware/update`** (**disruptive**) → `202` with a [`Task`](#task). Two modes:

- Pull: `Content-Type: application/json`, body `{"component":"BIOS","imageUri":"http://repo/bios.bin","options":{...}}`.
- Push: `Content-Type: application/octet-stream`, query `?component=BIOS&imageName=bios.bin`, image as the body (uncapped).

> **Current status:** no driver implements firmware update yet; this route returns `501` on every supported board. The contract above is reserved so clients can be written against it.

**`GET /firmware/tasks/{id}`** → `Task`: `id`, `state` (`pending | running | completed | failed`), `percentComplete`, `message`, `startTime`, `endTime`. Never cached.

### 11.13 Virtual media and KVM

**`GET /virtual-media`** → `{"slots":[{"id","mediaTypes[]","inserted","image","writeProtected","connectedVia"}...]}`

**`POST /virtual-media/{id}/insert`** with `{"image":"nfs://...|cifs://...|http(s)://...","transferProtocol":"","userName":"","password":"","writeProtected":true}`. `image` required; `slotId` optional and must equal `{id}`. → 204. The share password is redacted from logs.

**`POST /virtual-media/{id}/eject`** (no body) → 204.

**`GET /kvm`** → `{"kind":"html5|java|websocket","url":"...","token":"...","expires":"...","extra":{}}`. The token is short-lived and the response is never cached. The proxy does **not** proxy the KVM stream itself in v1; your client must be able to reach the BMC directly to use it. Not supported on Gigabyte boards.

### 11.14 BMC reset

**`POST /bmc/reset`** (**disruptive**), optional body `{"kind":"cold"|"warm"}` (default `cold`) → `202 {"accepted":true,"kind":"cold"}`.

The BMC reboots and is unreachable for roughly 1–3 minutes. The proxy drops all its sessions to that host and marks it degraded for up to 5 minutes (`/bmcs/{host}/status` → `degraded`). The host OS keeps running.

---

## 12. Data types

Full JSON Schemas are in `/openapi.json` → `components.schemas`. Field-level summary:

#### SystemInfo
`vendor` (driver), `manufacturer`, `board`, `model`, `serial`, `uuid`, `bmcFirmware`, `biosVersion`, `bmcMac`, `hostname`, `powerState`, `health`, `oem`.

#### PowerActionResult
`action`, `expected` (absent for nmi), `state`, `converged`, `polled`, `polls`, `waitedSeconds`, `timeoutSeconds`, `warning?`, `lastPollError?`.

#### BootOverrideConfig
`target`, `mode`, `bootMode?`.

#### Sensor
`id`, `number?`, `name`, `type`, `reading` (nullable), `unit?`, `health`, `state?` (discrete text), `thresholds{lowerNonRecoverable, lowerCritical, lowerNonCritical, upperNonCritical, upperCritical, upperNonRecoverable}` (each nullable), `settable?`, `backend?`, `oem?`.

#### EventsResponse
`entries[]` (SELEntry: `id`, `time{raw,corrected}`, `severity?`, `sensorType?`, `sensorName?`, `sensorNumber?`, `eventType?`, `direction?`, `message`, `eventData?` (hex), `generator?`, `backend?`, `oem?`), `lastId?`, `more?`, `clockOffsetSeconds?`.

#### NetworkInterfaceUpdate
`name` (required), `enabled?`, `ipv4?{dhcp,address,netmask,gateway}`, `ipv6?{enabled,dhcp,addresses[],gateway}`, `vlan?{enabled,id}`, `dns?[]`. Each sub-object is replaced as a whole when given.

#### BondResponse
`id`, `enabled`, `mode?`, `interfaces?[]`, `autoConfig`, `oem?`.

#### TimeConfig
`bmcTime`, `proxyTime`, `offsetSeconds` (BMC − proxy), `timezone?`, `utcOffsetMinutes`, `ntpEnabled`, `ntpServers?[]`.

#### Task
`id`, `state`, `percentComplete?`, `message?`, `startTime?`, `endTime?`.

#### Enumerations

| Type | Values |
|---|---|
| PowerState | `on`, `off`, `poweringOn`, `poweringOff`, `unknown` |
| PowerAction | `on`, `off`, `shutdown`, `cycle`, `reset`, `nmi` |
| RestorePolicy | `alwaysOn`, `alwaysOff`, `lastState`, `unknown` |
| LEDState | `on`, `off`, `blinking`, `unknown` |
| BootTarget | `none`, `pxe`, `disk`, `cd`, `usb`, `bios`, `floppy` |
| BootOverrideMode | `once`, `continuous`, `disabled` |
| Health | `ok`, `warning`, `critical`, `unknown` |
| SensorType | `temperature`, `voltage`, `fan`, `current`, `power`, `discrete`, `other` |
| InventoryKind | `cpus`, `memory`, `pcie`, `storage`, `baseboard` |
| Privilege | `administrator`, `operator`, `user`, `callback`, `noAccess`, `oem` |
| TaskState | `pending`, `running`, `completed`, `failed` |
| BMCResetKind | `cold`, `warm` |
| Tier | `read`, `write`, `disruptive` |
| Error class | `invalid_argument`, `auth`, `forbidden`, `not_found`, `unsupported`, `upstream`, `busy`, `timeout`, `canceled`, `internal` |

---

## 13. Vendor extensions

Vendor routes live under `/api/v1/vendor/{driver}/` and expose features with no generic equivalent. They follow the same request model, envelope and tiers. Two extra rules:

- A vendor route for a driver other than the one resolved for the BMC is a `404`. Check `capabilities.driver` (or the `vendorRoutes` list) before calling.
- Responses carry a `raw` field with the BMC's object; the named fields are the contract, `raw` is not.

Write-tier vendor routes marked **UNVERIFIED** below were built from the vendor web UI code and have not been exercised on real hardware. Treat them as beta: read back after writing.

### 13.1 ASRock Rack (`/api/v1/vendor/asrock/`)

| Method | Route | Tier | Request | Response | Notes |
|---|---|---|---|---|---|
| GET | `fanMode` | read | — | `{dialect, modes{"1".."16"}, raw}` | Fan control mode per fan (X470/X570) or B650 `settings/fan/mode` in `raw`. |
| POST | `setFanMode` | write | `{modes{"n":code}}` (X470/X570) or `{raw{}}` (B650) | as `fanMode` | Merged over current. Mode codes other than 0 are UNVERIFIED. |
| GET | `fanDuty` | read | — | `{dialect, source: current|manual, duties{"n":%}, raw}` | X470: live duty; X570/B650: configured manual duty. |
| POST | `setManualFanDuty` | write | `{duties{"n":0..100}}` or `{raw{}}` | 204 | Fans must be in manual mode. |
| GET | `fanTables?type=open,closed&table=1,2` | read | — | `{dialect, tables[{type, table, temps[], duties[], raw}]}` | `255` = unused point. Open loop tables 1..8, closed 1..10. |
| POST | `setFanTable` | write | `{type: open|closed, table, temps[], duties[], raw{}}` | 204 | UNVERIFIED. |
| POST | `resetFanSettings` | write | — | 204 | Restore fan defaults. |
| GET | `keepNicLinkUp` | read | — | `{dialect, enabled, sharedNic?, sharedNic2?, raw}` | Keep the shared NIC up while the host is off. |
| POST | `setKeepNicLinkUp` | write | `{enabled, sharedNic?, sharedNic2?}` | as GET | UNVERIFIED. |
| GET | `ncsiMode` | read | — | `{mode, channelNumber, interfaceName, packageId, active?, raw}` | `mode` 0 = auto failover, 1 = manual (UNVERIFIED encoding). |
| POST | `setNcsiMode` | **disruptive** | `{mode?, channelNumber?, interfaceName?, packageId?}` | as GET | Shared-NIC link can drop. |
| GET | `bonding` | read | — | `{id, enabled, mode, interface?, autoConfig, raw}` | Prefer generic `GET /network/bond`. |
| POST | `enableBonding` | **disruptive** | `{mode?, interface?}` | as `bonding` | |
| POST | `disableBonding` | **disruptive** | `{interface?}` (required on B650) | as `bonding` | |
| GET | `postCode` | read | — | `{code: "0x0caa", status?, source, raw}` | Last BIOS POST code; handy for "stuck in POST" detection. |
| GET | `powerSource` | read | — | `{powerSupplies[{index, status, serial, acInputVoltage/Current/Power, dc12vOutputVoltage/Current/Power, raw}]}` | PMBus PSUs only; values nullable. |
| GET | `prochot` / POST `setProchot` | read / write | `{enabled}` | `{enabled, value}` | X470 only; else 501. |
| GET | `mcaLog` | read | — | `{dumpPresent, mcaDump}` | CPU machine-check dump present? |
| POST | `clearMcaLog` | **disruptive** | — | 204 | X470 only. |
| GET | `webEnableTimeout` / POST `setWebEnableTimeout` | read / write | `{enabled}` | `{enabled, value}` | Web UI idle-timeout switch. |
| GET | `biosUpdateOption` | read | — | `{action}` | Encoding UNVERIFIED. |
| GET | `componentStatus/{component}` | read | — | `{component, response, action?, progress?, status, flashProgress}` | `component` ∈ BIOS, CPLD, FPGA, RETIMER, PSU, BP (board-dependent). |
| POST | `updateComponent/{component}` | **disruptive** | — | — | Not implemented: always 501. |
| POST | `bmcColdReset` | **disruptive** | — | `{accepted, message}` | Same effect as generic `POST /bmc/reset`. |
| ANY | `raw/api/{path}`, `raw/redfish/{path}` | dynamic | as BMC | as BMC | See [§14](#14-raw-passthrough). `raw/redfish` is 501 on X470. |

### 13.2 Gigabyte (`/api/v1/vendor/gigabyte/`)

| Method | Route | Tier | Request | Response | Notes |
|---|---|---|---|---|---|
| GET | `fanProfiles` | read | — | `{mode, version, source, profiles[{name, version, active, policies[...]}]}` | Redfish GBT OEM fan profiles; `active` marks the current one. |
| GET | `fanProfileDevices` | read | — | `{devices[{name, vendorId, deviceId, upperNonCritical, upperCritical}]}` | PCIe devices (mostly GPUs) a profile can key on. |
| GET | `fanMode` | read | — | `{mode, source: ami|redfish}` | `default` = factory curve. |
| POST | `setFanMode` | write | `{mode}` | `{mode, source}` | `mode` must be a `fanProfiles` name or `default`; read back, `502` if the BMC disagrees. UNVERIFIED. |
| GET | `firmwareInfo` | read | — | `{version, date, time, manufacturerId, productId, deviceId, activeImage, raw}` | `activeImage` = running image of the dual-image BMC (not in Redfish). |
| GET | `bonding` | read | — | `{id, enabled, mode, interface (eth0|eth1|both), autoConfig, raw}` | Confirmed live on both boards. |
| POST | `enableBonding` | **disruptive** | `{interface?, mode?}` | as `bonding` | UNVERIFIED. |
| POST | `disableBonding` | **disruptive** | `{interface?}` (eth0 or eth1) | as `bonding` | UNVERIFIED. |
| ANY | `raw/api/{path}`, `raw/redfish/{path}` | dynamic | as BMC | as BMC | See [§14](#14-raw-passthrough). |

The exact JSON Schemas (`AsrockFanTablesResponse`, `gigabyte.FanProfiles`, ...) are in `/openapi.json`.

---

## 14. Raw passthrough

`/api/v1/vendor/{asrock|gigabyte}/raw/api/{path}` and `.../raw/redfish/{path}` forward a request to the BMC's `/api/{path}` or `/redfish/v1/{path}` **through the proxy's own authenticated session** (cookies, CSRF tokens, re-login, serialisation and the circuit breaker are handled for you). The query string and body pass through; the BMC's status code, body and content type come back **verbatim** (not wrapped in the envelope, except for proxy-side refusals).

Use it to reach something the API does not model yet. Do not build durable integrations on it: the payloads are vendor- and firmware-specific.

Tier is decided per request:

| Method | Tier |
|---|---|
| GET, HEAD, OPTIONS | read |
| DELETE | disruptive |
| POST / PUT / PATCH to a sensitive path | disruptive |
| POST / PUT / PATCH elsewhere | write |

Sensitive paths include anything matching flash, firmware, upgrade, upload, hpm, maintenance, restore, reset, factory, network, ncsi, users, logs (plus Gigabyte one-touch-settings, GBT, GpuManager, save, license) on `/api`, and UpdateService, AccountService, SessionService, EthernetInterfaces, NetworkProtocol, certificates, `*.Reset`, ResetToDefaults, ClearLog, SimpleUpdate on Redfish.

A **denylist** is always refused with `403 forbidden` and never reaches the BMC: paths that crash the BMC web server or leak data (`logs/help-data`, `asrr/mca-file`, `GlobalLuaTables`, config/image downloads, crash and log dumps, Gigabyte diagnostics, ...) and `session` / Redfish session create/delete (which would log out the proxy's shared session). Paths are canonicalised (case, `./`, percent-encoding) before the check, so encoding tricks do not bypass it.

---

## 15. Integration recipes

### 15.1 Onboarding a BMC

1. `GET /bmcs/{host}/status?probe=true`: reachable? which vendor? (no credentials used)
2. `GET /capabilities` with the credentials: `401` → wrong credentials; otherwise record `driver`, `board`, `firmware`, and the supported set.
3. `GET /system`: store `serial`, `uuid`, `bmcMac` as identity.
4. `GET /time`: if `|offsetSeconds|` is large, configure NTP (`PUT /time`) or `POST /time/sync`.
5. Optionally `GET /services` and `GET /users` for a security baseline.

### 15.2 Event ingestion (SEL tailing)

```text
cursor = load(host) or ""                      # persist per BMC
every 60s (with jitter):
    loop:
        r = GET /events?since={cursor}&limit=500
        on 501: stop tailing this host (no SEL support)
        on 5xx: back off, try next cycle
        for e in r.entries: emit(host, e.id, e.time.corrected, e.severity, e.message, ...)
        if r.lastId: cursor = r.lastId; save(host, cursor)
        if not r.more: break
```

- De-duplicate downstream on `(host, id)`: a crash between `emit` and `save` replays a page.
- After `DELETE /events`, a BMC replacement, or a BMC firmware update, reset the cursor to `""`: record ids may restart.
- Use `time.corrected` as the event time; keep `time.raw` for forensics.

### 15.3 Clock skew

BMC clocks drift and are often never set. The proxy measures `offset = bmcTime − proxyTime` when it reads time from the BMC and reports it as `clockOffsetSeconds` (events, audit, capabilities) and `offsetSeconds` (`/time`). `capabilities.warnings` and `/bmcs/{host}/status` warn when the skew exceeds the operator threshold (2 min by default). Fix it at the source with NTP (`PUT /time {"ntpEnabled":true,"ntpServers":[...]}`) or a one-off `POST /time/sync`.

### 15.4 Monitoring and metrics collection

- Poll `GET /sensors` every 30–60 s per host (never faster than the cache TTL). Key series on `(host, sensor.id)`; export `reading` and `health`.
- Alert on `sensor.health` ∈ {`warning`, `critical`} and on SEL entries with `severity` ≥ `warning`.
- Track BMC reachability from `/bmcs/{host}/status` (free) rather than by probing BMCs yourself.
- Expect `null` readings while the host is off.

### 15.5 Provisioning: PXE reinstall

```text
PUT  /boot   {"target":"pxe","mode":"once","bootMode":"uefi"}        → 204
POST /power  {"action":"shutdown","timeoutSeconds":120}               → 200 off | 202 not observed
  if 202:   POST /power {"action":"off"}                              → 200
POST /power  {"action":"on","timeoutSeconds":120}                     → 200 on
(optional) GET /vendor/asrock/postCode while booting to detect a POST hang
```

Use `cycle` instead of shutdown+on when a graceful shutdown is not needed. Check `capabilities` for `bootOverride` first: on some boards it is served by IPMI and may be unavailable if IPMI-over-LAN is disabled.

### 15.6 Disruptive network changes

Bond, NC-SI and interface changes restart the BMC's networking. The request itself may fail with `502`/`504` even when the change was applied, because the BMC drops the connection mid-write.

1. `GET` the current config and keep it.
2. Send the change.
3. Regardless of the response, wait ~60–180 s, then `GET /bmcs/{host}/status?probe=true` and re-read the config at the expected address.
4. If the BMC does not return, recovery is out-of-band (host-side IPMI, physical access). Have a plan before automating this.

### 15.7 Handling heterogeneity

- Branch on `capabilities`, not on `board` names. New boards and firmware arrive through the generic Redfish fallback or capability probing, and your code keeps working.
- Treat `501` as a permanent, per-host fact until firmware changes.
- Use `X-BMC-Vendor` only to work around a misdetection, and report it.

---

## 16. Operating against the proxy

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/health` | Liveness/readiness probe. No auth. |
| `GET /metrics` | Prometheus metrics: `bmcproxy_http_requests_total` / `bmcproxy_http_request_duration_seconds` (northbound, by host, route, status, error class), `bmcproxy_southbound_requests_total` / `bmcproxy_southbound_latency_seconds`, `bmcproxy_breaker_state` (0 closed, 1 half-open, 2 open), `bmcproxy_sessions_live`, `bmcproxy_session_relogins_total`, `bmcproxy_cache_hits_total`, `bmcproxy_cache_hit_ratio`. |
| `GET /openapi.json` | OpenAPI 3 (generic + all compiled-in vendor routes). |
| `/swagger/`, `/swagger/doc.json` | Swagger UI and Swagger 2 spec (if the operator left `--swagger` on). |

When reporting a problem to the proxy operators, include the `X-Request-Id`, the BMC host, the time, and the error `class`. With that they can find the southbound trace (`--trace-http`, `--har-out`).

**Versioning.** The API is `v1` (`info.version` 1.0 in the spec). Additive changes (new routes, new optional fields, new enum values, new vendor routes) happen within `v1`; build clients to tolerate them ([§4](#4-conventions)). `oem`, `raw`, and raw passthrough payloads carry no compatibility promise. Regenerate clients from `/openapi.json` when you upgrade the proxy.

---

## 17. Hardware coverage and verification status

| Board | BMC firmware | Driver | Southbound |
|---|---|---|---|
| ASRock Rack X470D4U | 2.32.00 | `asrock` | AMI `/api` + IPMI |
| ASRock Rack B650D4U | 4.10.00 | `asrock` | Redfish, `/api` for OEM gaps |
| ASRock Rack X570D4U | 1.20.00 | `asrock` | Redfish for control; `/api` for sensors, SEL, firmware |
| Gigabyte MC13-LE0 | 13.04.15 | `gigabyte` | Redfish, `/api` lazily |
| Gigabyte MZ32-AR0 | 12.61.39 | `gigabyte` | Redfish, `/api` lazily |
| Any other DMTF Redfish BMC | — | `redfish` | Redfish |

Other firmware versions of these boards fall back to capability probing. All reads listed in the generic reference are confirmed on hardware for the boards above. Writes that have **not** been exercised on hardware (built from vendor UI code; covered by simulator tests only) include: LED timed blink, X470 NMI, fan-table and non-default fan-mode writes, keep-NIC-link-up writes, NC-SI mode codes, all bonding writes, Gigabyte `setFanMode`, and the B650 date-time PUT. The README's "Verified vs unverified behaviour" section has the full, current list.

Known per-board limitations visible through the API:

- **X470D4U:** no Redfish for `/api` users (raw Redfish is 501); boot override via IPMI.
- **X570D4U:** its Redfish has few sensors and a tiny SEL, so those come from `/api`.
- **MZ32-AR0:** power `cycle` and `nmi` → 501.
- **Gigabyte:** no firmware update, no KVM (501).
- **All:** firmware update is 501 until a driver implements it.

---

## 18. Integration checklist

- [ ] The proxy sits behind authentication/TLS, or on a trusted network only.
- [ ] Every BMC request sends `X-BMC-Host`, Basic auth and an `X-Request-Id`, and logs that id.
- [ ] Error handling branches on `error.class`, not on messages or bare HTTP codes.
- [ ] `401` stops retries for that credential; `501` is cached per host; `502/503/504` back off.
- [ ] Writes that fail with `502/504` are re-read before being resent; power actions especially.
- [ ] `POST /power` `202` is treated as "not confirmed", with an explicit fallback.
- [ ] HTTP client timeouts ≥ 200 s (≥ `timeoutSeconds` + 90 s for power).
- [ ] Per-BMC concurrency is small; fleet fan-out is bounded and jittered.
- [ ] Polling intervals exceed the cache TTL (sensors ≥ 10 s, typically 30–60 s).
- [ ] SEL cursors (`lastId`) are persisted per host, consumers dedupe on `(host, id)`, cursors reset after log clear/BMC swap.
- [ ] Event times use `time.corrected`; clock skew warnings are surfaced and fixed with NTP.
- [ ] Features are gated on `GET /capabilities`, not on board names.
- [ ] Disruptive operations are behind an explicit human or policy approval in your system (the proxy does not gate them) and have a recovery plan.
- [ ] One automation account per BMC; no account rotation.
- [ ] Unknown response fields and enum values are tolerated; `oem`/`raw` are not relied on.
