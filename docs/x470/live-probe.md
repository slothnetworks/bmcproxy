# X470D4U BMC: live probe

Status: preliminary assessment for the native Go proxy. This is a live, mostly read-only probe of one real ASRock Rack X470D4U BMC (`<x470-bmc>`) on 2026-11-16, done to confirm or refute the static findings in:

- `docs/firmware/x470d4u.md`, the static analysis of the **3.02.00** image (cited below as **[FW §n]**)
- `docs/x470/har-analysis.md` and `har-endpoints.json`, a web UI capture from **2.32.00** (**[HAR §n]**)
- `docs/redfish/b650-implementation.md` §9 and `b650-uri-inventory.json` (**[RF §9]**)
- `docs/api/legacy-api-catalog.md` / `legacy-api-endpoints.json` (**[CAT]**)

Companion file: `docs/x470/live-redfish-inventory.json` (the sanitized Redfish crawl).

---

## Live firmware (read this first)

> **The live unit is NOT on 3.02.00. It runs BMC firmware 2.32.00**, the same version as the HAR capture. Where the live unit differs from the static 3.02.00 analysis, the difference may be due to the firmware version and does not necessarily mean the static analysis was wrong. Each verdict below says which firmware it applies to.

| Item | Value | Source |
|---|---|---|
| Board | `X470D4U` | `GET /api/asrr/model-name` → `{"mb_name":"X470D4U"}`; FRU board `manufacturer "ASRockRack"`, `product_name "X470D4U"` |
| BMC firmware | **2.32.00**, built `May 11 2021 17:53:41 CST`, active image 0 | `/api/asrr/fw-info` `BMC_fw_version`; `/api/firmware-info` (`fw_rev_1 2`, `fw_rev_2 50` = 0x32, `fw_ver "2.32.00"`, IPMI device_id 32, mfg id 0xC1D6 = 49622 ASRock, prod_id 514) |
| BIOS | `P4.20` | `/api/asrr/fw-info` |
| ME / PSP field | `0.11.0.65` (the HAR unit reported `0.14.0.27`) | `/api/asrr/fw-info` `ME_fw_version` |
| CPU microcode | `0a50000b` | `/api/asrr/fw-info` |
| CPLD | `N/A` | `/api/asrr/fw-info` |
| Product/system name | `N/A` (FRU product area empty) | `/api/asrr/product-name` → `{"sys_name":"N\/A"}` (the HAR unit got a 500 here) |
| Redfish `Managers/Self.FirmwareVersion` | **not readable**: Redfish rejected the credentials (see §2) | n/a |
| Host power | off for the whole probe (`power_status 0`; 35 of 41 sensors `accessible=213`) | `/api/chassis`, `/api/sensors` |
| BMC uptime | last BMC start about 2026-11-14 15:06 UTC (first audit-log record). The Redfish ETag epoch `1794669113` is 15:11:53 that day | `/api/logs/audit`, Redfish `ETag` |
| IPMI Get Device ID | not run (`ipmitool` is not installed and sudo needs a password). The `/api/firmware-info` IPMI Device ID fields above are the same data | n/a |

> **Security note: the BMC uses the factory-default `admin` / `admin` credentials.** The account works on `/api` and the web UI (and, by the same IPMI user store, on IPMI-over-LAN with MD5 / RMCP+). IPMI 1.5 is enabled and cipher suite 0 is advertised (§11). Together with the expired, publicly keyed TLS cert (§8), anyone on this L2/L3 segment can take full control of the host. Change the password and disable IPMI 1.5 / cipher 0 before this unit goes anywhere near production.

> **Two probes ran at the same time.** Two agent sessions (a "first" probe that wrote §1–§9 and a "second" probe, session `4c303595`, that wrote §10–§15) probed this BMC concurrently from the same source address between about 06:38 and 07:20 UTC. Both ran the same unauthenticated Redfish sweep, the same `/api` GET walk and the same LED test, and both hit `help-data`. **Concurrent probing skewed the measurements:** per-request latencies in §5.2 and §7 include queueing behind the other probe, the `racsession_id` gaps, and the audit-log "foreign" logins in the summary are mostly the other probe, and one session the second probe lost after about 2 min idle was probably ended by the other probe's activity (a `spx_restservice` restart), not by an idle timeout. The controlled idle test in §10.1 is the authoritative idle measurement. Redfish credentialed failures across both probes: 2 (first) + 4 (second) = 6, never more than 2 inside any 30 s window.

---

## Version gap: live 2.32.00 vs the static 3.02.00 analysis

Everything in `docs/firmware/x470d4u.md` and `docs/redfish/b650-implementation.md` §9 was derived from the **3.02.00** image. The live unit (and the HAR unit) run **2.32.00**. The differences below are therefore **possibly version-related, not necessarily errors in the static docs**. Re-probe a 3.02.00 unit before treating any of them as a correction.

| Area | Static 3.02.00 prediction | Live 2.32.00 |
|---|---|---|
| TLS protocols | TLS 1.0–1.2, SSLv3 disabled | TLS 1.0–1.2 **plus SSLv3** (nmap `ssl-enum-ciphers`), TLS compression (DEFLATE) accepted |
| TLS key exchange | ECDHE-RSA on secp384r1 available | **RSA key exchange only**, no ECDHE/DHE, no forward secrecy |
| Client-cert request on :443 | CertificateRequest sent (not enforced) | none sent |
| Server cert | `CN=www.ami.com`, Norcross, 2018–2037 | `CN=AMI`, Atlanta, 2016–**2026-05-30 (expired)** |
| Redfish credentials | IPMI users mirrored into Redfish; same creds work for `/api` and `/redfish` | the `/api` admin account gets **401** on Redfish (Basic and session), still 401 after 40 min |
| Redfish `/redfish/v1` | GET route | **302** to `/redfish/v1/` |
| `/api/settings/sensor-threshold`, `/api/asrr/settings/BIOS/get/status` | in binary strings | **404** |
| `/api/settings/network-interfaces` | "probably dead UI code" | **200**, same data as `/settings/network` |
| Web session idle timeout | 1800 s (`ncml.conf`) | `settings/services` says 1800 s; a session survived 300 s idle (§10.1); 1800 s not waited out |
| Ports 5120–5127, 7578, 7582 | listening | **closed**: single-port mode is enabled, KVM/media go over 443 websockets |
| TCP 623 | not predicted | **open** (listener unidentified) |
| Product name | n/a | `/api/asrr/product-name` 200 `N/A` (the HAR unit, same version, got a 500) |

---

## Summary verdict

- **Redfish: yes, it is running and on the LAN, even on 2.32.00.** `https://<x470-bmc>/redfish/v1/` answers as `AMI MegaRAC Redfish Service`, with `RedfishVersion 1.5.0`, `ServiceRoot.v1_3_1`, `Product "AMI Redfish Server"`, `RtpVersion 1.5.b` and 16 top-level services (Systems, Chassis, Managers, SessionService, AccountService, EventService, TaskService, UpdateService, TelemetryService, CompositionService, Registries, JsonSchemas, plus the AMI `configurations` and `DynamicExtension`). `$metadata` declares 112 schemas, which match the static "older AMI generation" prediction [RF §9]. **How complete it is, is unknown:** the `admin` account that works for `/api` gets **401 `Security.1.0.AccessDenied`** from Redfish with both HTTP Basic and `POST /SessionService/Sessions`. Every resource below the service root returned 401, so no authenticated crawl was possible. Without the right credentials, Redfish on this unit cannot yet replace `/api`.
- **Auth model.**
  - `/api`: form login, then the `QSESSIONID` cookie (30 chars, `path=/`, **no `HttpOnly`, no `Secure`, no expiry**) plus the `X-CSRFTOKEN` header (8 chars). Every auth failure returns **401 `{"cc":7,"error":"Invalid Authentication"}`** and sets a fresh, unauthenticated cookie. `DELETE /api/session` really invalidates the session.
  - Redfish: advertises `WWW-Authenticate: Basic` and `X-Auth-Token`. The credentials were not accepted (cause unknown, see §2).
- **Top quirks for the proxy.**
  1. **TLS is RSA key exchange only.** There are no ECDHE or DHE suites. The only AEAD suites are `AES128/256-GCM` with RSA key exchange. TLS 1.0, 1.1 and 1.2 are accepted; TLS 1.3 is not. Go's default cipher list has excluded RSA key exchange since Go 1.22, so **the Go client must set `CipherSuites` explicitly** (`TLS_RSA_WITH_AES_128_GCM_SHA256`, `TLS_RSA_WITH_AES_256_GCM_SHA384`).
  2. **The certificate is the firmware's shared AMI `CN=AMI` cert, it expired on 2026-05-30, and its private key ships in the firmware image.** This is not the `CN=www.ami.com` 2037 cert that [FW §3.4] predicts.
  3. Redfish is single-threaded: 3 parallel GETs queue up. `/api` overlaps a little, but the TLS handshake (about 230 to 500 ms) costs more than the request itself, so **keep-alive matters most**.
  4. **The incremental SEL works:** `GET /api/logs/eventlog` dumps everything (589 KB, about 4.7 s) and ignores query strings, but `GET /api/logs/event?LASTEVENTID=n` returns only records with id > n (3.7 KB).
  5. `GET /api/logs/help-data` returned a lighttpd HTML 500, and the session was **invalid** straight afterwards (the backend PID changed). Treat it as dangerous: probably a FastCGI crash.
  6. Redfish quirks: `/redfish/v1` gives a 302 to `/redfish/v1/`. `OPTIONS` returns 405 `text/html`. One service-wide ETag is used for every resource. `GlobalLuaTables` is unauthenticated. `CORS *` is sent with credentials allowed.
  7. Another client was using this BMC's web API during the probe. The audit log shows about twice as many `http Login` records as we made, **all from our own source address** (so probably another process on the same host or NAT, such as a parallel probe), and `racsession_id` jumped 3 → 5 → 11. A KVM session from a different address had been opened at BMC boot. This supports [HAR §0] item 2 (serialise and cache) and argues for a single per-BMC session owner in the proxy.

---

## 1. Redfish service root and crawl

**Status: CONFIRMED (Redfish exists on the LAN; [FW §4.1] says it is enabled by default with no switch). The authenticated crawl was BLOCKED. Note that [FW §4.1] was written for 3.02.00 and this unit runs 2.32.00, so Redfish is on by default in 2.32.00 too. [HAR §0] item 6 inferred this from the `REDFISH` feature flag, and it is now confirmed.**

### 1.1 Unauthenticated service root (`GET /redfish/v1/`)

| Field | Value |
|---|---|
| Status | 200, `application/json; charset=UTF-8`, 1518 bytes, `Content-Length` (not chunked) |
| `@odata.type` | `#ServiceRoot.v1_3_1.ServiceRoot`. This confirms the [RF §9] version (`ServiceRoot v1_3_1`) |
| `RedfishVersion` | `1.5.0` |
| `Product` | `AMI Redfish Server` |
| `Vendor` | **absent** (the property was added in ServiceRoot v1.5; [RF §5.1] `Vendor "AMI"` applies to B650 only) |
| `UUID` | **absent** from the root (not just hidden), so no UUID to redact. [FW §4.5] lists a config UUID string, which is not exposed here |
| `Id` / `Name` | `RootService` / `Root Service` |
| `Oem.Ami` | `AMIServiceRoot.v1_0_0`, `RtpVersion "1.5.b"` (**confirms** [FW §4.5] `Redfish:Ami:RtpVersion 1.5.b`), `Configurations → /redfish/v1/configurations` |
| `Oem.Dre` | `#AMIDynamicExtension.v1_0_0`, `DynamicExtension → /redfish/v1/DynamicExtension` (confirms the [RF §9] `oem/dre` framework) |
| `ProtocolFeaturesSupported` | `ExpandQuery {ExpandAll, Levels, Links, NoLinks: true, MaxLevels: 5}`, `FilterQuery: true`, `SelectQuery: true`. No `ExcerptQuery`, `OnlyMemberQuery` or `DeepOperations` |
| Links | `Systems, Chassis, Managers, Tasks (→ /TaskService), SessionService, Links.Sessions, AccountService, EventService, JsonSchemas, Registries, CompositionService, TelemetryService, UpdateService` |

The fields visible without auth are everything in the root (nothing is filtered when unauthenticated). The root is also the only JSON resource that serves real content without auth, apart from `/redfish/v1/odata`, `$metadata` and `GlobalLuaTables`.

### 1.2 What the crawl reached

The crawler (`rf_crawl.py`, sequential, 250 ms spacing) walks every `@odata.id`, `@Redfish.ActionInfo` and `Members@odata.nextLink`, with a 600-resource cap. With no usable credentials it reached 24 URIs:

- 200: `/redfish/v1/`, `/redfish/v1/odata`, `/redfish/v1/$metadata` (XML, 46.7 KB) and `/redfish/v1/GlobalLuaTables`
- 401: everything else, **including a URI that does not exist** (`/redfish/v1/NoSuchThing`). Without auth you cannot tell 404 from 401.

So these could **not** be measured: per-resource `@odata.type` versions, `Allow` headers on sub-resources, Actions/ActionInfo/`ResetType` allowable values, `$top`/`$skip`/`nextLink` behaviour on log collections, and Members counts. See "needs approval".

### 1.3 `$metadata` schema coverage (indirect evidence of coverage)

`$metadata` lists 112 schema references (the full list is in the inventory JSON). They match the [RF §9] coverage list well:

- **Present:** AccountService, ManagerAccount, Role, Session(Service), ExternalAccountProvider (schema only), ComputerSystem, Processor, Memory, MemoryMetrics, MemoryDomain, MemoryChunks, Bios, AttributeRegistry, SecureBoot, BootOption, Storage, Drive, Volume, SimpleStorage, EthernetInterface, VLanNetworkInterface, NetworkInterface, NetworkAdapter, NetworkDeviceFunction, NetworkPort, PCIeDevice, PCIeFunction, Chassis, Power, Thermal, Assembly, Manager, ManagerNetworkProtocol, SerialInterface, VirtualMedia, HostInterface, LogService, LogEntry, EventService, EventDestination, Event, TaskService, Task, UpdateService, CompositionService, ResourceBlock, Zone, Fabric, Endpoint, Switch, Port, JsonSchemaFile, MessageRegistryFile, MessageRegistry, PrivilegeRegistry, Settings, CollectionCapabilities, Redundancy, PhysicalContext, Protocol, plus AMI OEM `AMIServiceRoot`, `AMIDynamicExtension`, `AMIUpdateService`, `AMIAccountServiceConfigurations`, `Configurations`, `DynamicExtension`.
- **Absent (consistent with [RF §9] "absent compared with B650"):** CertificateService and Certificate, Sensor, PowerSubsystem/ThermalSubsystem, PCIeSlots, SoftwareInventory, MetricDefinition, MetricReport(Definition), Triggers, TelemetryService (the schema is not declared, but the root links to it), SecureBootDatabase.
- Note: `TelemetryService` is linked from the root but has **no schema in `$metadata`**, and nothing under it could be checked without auth.

### 1.4 `GlobalLuaTables` is unauthenticated

`GET /redfish/v1/GlobalLuaTables` returns 200 without credentials. Body: `{"@odata.id":"/redfish/v1/GlobalLuaTables","Members":[{"@odata.id":"/redfish/v1/"}],"Members@odata.count":1}`, with no `Allow` header. **CONFIRMED**: [RF] line 477 / §A says it is in `PRIVILEGE_CHECKING_WHITELIST` and has GET **and DELETE**, which clears the cache. DELETE was not tested (it is forbidden). **The proxy must block `/redfish/v1/GlobalLuaTables` for every method.**

---

## 2. Redfish authentication

**Status: NEW and BLOCKING. The [FW §4.6] claim that "IPMI users are mirrored into Redfish ... the same credentials work for `/api` and `/redfish`" is REFUTED on 2.32.00.** This could be version-related; it is unverified on 3.02.00.

| Test | Result |
|---|---|
| No credentials | 401, `WWW-Authenticate: Basic realm="AMI_Redfish_Server"` **and** `WWW-Authenticate: X-Auth-Token realm="AMI_Redfish_Server"` |
| HTTP Basic, probe account (`admin`, valid for `/api`) | **401** `Security.1.0.AccessDenied` (the body is identical to the no-credentials case) |
| `POST /redfish/v1/SessionService/Sessions` `{"UserName","Password"}` with the same credentials | **401** `Security.1.0.AccessDenied`, with no `X-Auth-Token` and no `Location` |
| Bogus `X-Auth-Token` header | 401, same body |
| `SessionTimeout`, max sessions, `Location` format, token length | **not measurable** (no session) |

The Redfish attempts were limited to 2 credentialed failures, about 3 minutes apart (lockout is 5 failures in 30 s per [FW §4.6]). No further guesses were made, and `Administrator` was not tried.

Error body format (the same for 401 and the bad-password case; status 401, `application/json; charset=UTF-8`):

```json
{"error":{"@Message.ExtendedInfo":[{"@odata.type":"#Message.v1_0_5.Message",
  "Message":"While attempting to establish a connection to /redfish/v1/Systems, the service was denied access.",
  "MessageArgs":["/redfish/v1/Systems"],"MessageId":"Security.1.0.AccessDenied",
  "Resolution":"Attempt to ensure that the URI is correct and that the service has the appropriate credentials.",
  "Severity":"Critical"}],
 "code":"Security.1.0.AccessDenied","message":"While attempting ... denied access."}}
```

- The registry prefix is `Security.1.0`. There is no 404 body to show: unauthenticated 404s come back as this 401.
- A bad password cannot be told apart from "account not known to Redfish".
- A bad query parameter gives `501 Base.1.0.QueryNotSupported` (`MessageArgs ["foo"]`), which confirms the Base registry is 1.0 [RF §9].

Hypotheses for the 401 (not distinguishable without more access):

- (A) On 2.32.00 the Redfish account store (redis, seeded with `Administrator`/`HostAutoFW`/`HostAutoOS` [FW §4.6]) is not synchronised with IPMI users, so `admin` does not exist for Redfish.
- (B) Redfish applies its own password policy (`MinPasswordLength 8` in the seed) or a different hash (`SUPPORTED_HASHING_ALGORITHM MD5 SHA512`), so an IPMI password that `/api` accepts can still fail. Test with an account whose password has at least 8 characters.
- (C) The user's Redfish privilege is disabled or the account is not provisioned. The `/api/settings/users` record has no Redfish field on 2.32.00.

The `/api/logs/audit` log has one `http Login Failed ... user:admin` record from our source address, but its timestamp does not line up with either Redfish attempt, and another client shared our address (see the summary). So it is **not established** whether Redfish auth failures are audited or count toward a shared lockout.

---

## 3. Redfish conformance quirks and headers

| Topic | Observed | Status |
|---|---|---|
| `Server` header | `AMI MegaRAC Redfish Service` (lighttpd passes the backend header through). `/api` responses say `Server: lighttpd` | NEW. [FW §3.4] / [HAR §2.1] cover `/api` only |
| `OData-Version` | `4.0` on JSON and errors | as expected |
| Caching | `Cache-Control: no-cache, must-revalidate` | NEW |
| ETag | `ETag: W/"1794669113"`, the same as `@odata.etag`. **The same value on root, odata and GlobalLuaTables**: it is the service `LastModified` epoch (BMC start), not a per-resource hash. If-Match would be very coarse. Per-resource behaviour is unverified | NEW (inference) |
| `Allow` | `Allow: GET` on root and odata | partial |
| `Link` | one `Link: </redfish/v1/X>; path=/X` header per root property, plus `rel=describedby` → DMTF ServiceRoot.v1_3_1.json | NEW |
| CORS | `Access-Control-Allow-Origin: *`, `Allow-Credentials: true`, `Expose-Headers/Allow-Headers: X-Auth-Token` | NEW. The proxy should strip these northbound |
| Connection | no `Connection: close` seen on Redfish; keep-alive works | n/a |
| Compression | `Accept-Encoding: gzip` is ignored, responses are sent identity-encoded | NEW |
| Trailing slash | `/redfish/v1` → **302** `Location: /redfish/v1/` (empty `text/html`) | **REFUTES** the [RF] inventory entry "`/redfish/v1` GET" (2.32.00) |
| `/redfish` | 200 `{"v1":"/redfish/v1/"}`, unauthenticated | CONFIRMED (inventory `^/redfish$`) |
| OPTIONS | **405 `text/html` "Method Not Allowed"** (not JSON, no `Allow`) | NEW |
| HEAD | 200, `Allow: GET`, ETag, but `Content-Type: text/html` and `Content-Length: 0` | NEW |
| `$select` | works (unauthenticated on the root): `?$select=RedfishVersion` returns just that property plus the odata annotations | CONFIRMED (`SelectQuery: true`) |
| `$expand` | `?$expand=.($levels=1)` on the root without auth → whole-request 401 (it expands into protected children) | partial |
| `$filter`/`$top`/`$skip` | advertised (`FilterQuery: true`); not testable without auth | unverified |
| Unknown query param | **501** `Base.1.0.QueryNotSupported` | NEW. The proxy must not forward its own query params |
| HTTP :80 | `301` to `https://<host>/redfish/v1/` (`Server: lighttpd`) | CONFIRMED [FW §3.1] |
| Lenient decoding (`"nil"`, wrong types) | not observable on Redfish (no data). On `/api`: `ports` are encoded as negative ints (`-2147476070` = 0x80000000\|7578 in single-port mode), numbers are sent as strings (`GetFanDuty "60"`, PSU `"N\/A"`), FRU dates contain a literal `\n`, `Storage_Contoller` is misspelt and set to `null` in `host_interface_storage_info`, and `host_interface_*` strings are `"NA"` or `" "` | NEW (for `/api`) |
| Write semantics / If-Match | not testable on Redfish. The LED test was done on `/api` instead (§5.3) | blocked |

---

## 4. Coverage against the static prediction

**Status: PARTIALLY CONFIRMED.** Existence was confirmed from the root links, `odata` and `$metadata`. Contents are unverified, because everything is behind 401.

| Resource | Static prediction [RF §9] (3.02.00) | Live 2.32.00 |
|---|---|---|
| Systems/Self | yes (ComputerSystem v1_5_0) | collection linked; schema present; **401** |
| Chassis/Self | yes (Chassis v1_7_0) | linked; schema present; 401 |
| Managers/Self | yes (Manager v1_4_0) | linked; schema present; 401 |
| Thermal / Power (legacy) | yes | schemas present; 401 |
| Sensors, Power/ThermalSubsystem | absent | **absent from `$metadata`**, consistent |
| LogServices + SEL | yes | LogService/LogEntry schemas present; 401 |
| EventService (+Subscriptions) | yes, no SSE | linked; EventDestination schema; 401 |
| UpdateService | yes (SimpleUpdate HTTP/FTP, OEM BMCFwUpdate) | linked; `AMIUpdateService` schema; 401 |
| TaskService | yes (no TaskMonitors) | linked (as `Tasks`); 401 |
| VirtualMedia | yes (PATCH insert) | schema present; 401 |
| Bios / SecureBoot | yes | schemas present (Bios, AttributeRegistry, SecureBoot); 401 |
| AccountService | yes | linked; 401 |
| CertificateService | absent | **absent** (`/redfish/v1/CertificateService` → 401 like any path; no schema; not linked). Consistent |
| NetworkProtocol / EthernetInterfaces / HostInterfaces | yes | schemas present; 401 |
| TelemetryService | yes | linked from root but **no schema in `$metadata`** |
| CompositionService / Fabrics | yes | CompositionService linked; ResourceBlock/Zone/Fabric/Endpoint/Switch schemas present |
| DynamicExtension, configurations (AMI OEM) | yes | linked from root and `odata`; 401 |
| GlobalLuaTables | yes (debug, GET+DELETE) | **200 without auth** |

The B650 inventory predicts 178 URI patterns for X470 3.02 (out of 339 in total). Only 7 could be tested unauthenticated on 2.32.00; all 7 behaved as predicted, except the `/redfish/v1` 302.

---

## 5. Legacy `/api`

**Status: CONFIRMED overall [FW §5, §6.3], [HAR §4] (on 2.32.00, the HAR's own version).**

### 5.1 Login and cookie (new detail the HAR was missing)

`POST /api/session`, form `username`, `password`, `certlogin=0`, with `X-Requested-With: XMLHttpRequest`. 200 in about 1.6 to 3.0 s (expensive, as [HAR §0] item 3 says).

```
Set-Cookie: QSESSIONID=<30 chars [A-Za-z0-9]>; path=/
```

- **Attributes: only `path=/`.** There is **no `HttpOnly`, no `Secure`, no `Expires`/`Max-Age`** (a session cookie) and no `SameSite`. This answers the open question in [HAR §4.3]. The value is 30 alphanumeric characters.
- Response headers: `Cache-Control: no-store, no-cache, must-revalidate, private`, `Pragma: no-cache`, `X-Content-Type-Options`, `X-XSS-Protection`, `Content-Type: application/json`, `Transfer-Encoding: chunked`, `Server: lighttpd`. **Confirms** [HAR §2.2].
- Body fields: `ok` (int, 0), `privilege` (int, 4), `extendedpriv` (int, 259), `racsession_id` (int; values 3, 5, ..., 11 seen in successive logins), `remote_addr`/`server_name`/`server_addr` (str), `HTTPSEnabled` (int, 1), `CSRFToken` (8 characters). **Confirms** [HAR §4.2] exactly.

### 5.2 Read endpoints

The times include a fresh TLS handshake per request (about 230 to 500 ms of each figure; see §7). Bodies are in the scratchpad and are not reproduced here.

| Endpoint | Status | ms | Bytes | Notes |
|---|---|---|---|---|
| `/api/chassis`, `/api/chassis-status` | 200 | 400 | 38 | `{power_status, led_status}` |
| `/api/firmware-info` | 200 | 490 | 266 | IPMI Get Device ID |
| `/api/configuration/project` | 200 | 431 | 1805 | includes `REDFISH`, `REDFISH_HOSTINTERFACE`, `SINGLE_PORT_APP`, `RUNTIME_SINGLEPORT_SUPPORT`; `CAPTURE_BSOD_RAW` listed twice |
| `/api/configuration/runtime` | 200 | 577 | 210 | `SINGLE_PORT_APP enabled 1`; LDAP/AD/RADIUS 0 |
| `/api/fru` | 200 | 830 | 844 | board area set; chassis/product areas empty |
| `/api/sdr` | 200 | 496 | 3918 | |
| `/api/sensors` | 200 | 634 | 21975 | 41 sensors (same count as HAR) |
| `/api/detail_sensors_readings` | 200 | 518 | 20958 | same schema as `/sensors` |
| `/api/sensors/basic` | 200 | 853 | 17772 | binary-only route [FW §5.2]: works |
| `/api/asrr/fw-info`, `model-name`, `product-name`, `bios-model-name` | 200 | 650–1440 | | `product-name` is **200 `N/A`** here, not the 500 from the HAR |
| `/api/asrr/selsummary` | 200 | 502 | 42 | 566 entries, 3073 free |
| `/api/asrr/getmcalog`, `PowerSource`, `GetFanDuty`, `getWebEnableTimeout`, `getbioscode`, `getprochot`, `getvetobit`, `inventory_status`, `inventory_info` | 200 | 400–1900 | | |
| `/api/asrr/settings/get*` (7 fan-control reads) | 200 | 390–600 | | |
| `/api/asrr/settings/BIOS/get/status` | **404** | | | `{"error":"Invalid API Call","code":1010}`. Present in the 3.02.00 strings [FW §5.2], absent on 2.32.00 |
| `/api/settings/fan/fan_mode` | **404** | | | CONFIRMS [FW §5.2] (no route string) |
| `/api/settings/sensor-threshold` | **404** | | | in the [FW §5.2] binary list, missing on 2.32.00 |
| `/api/settings/network-interfaces` | **200** | 1493 | 826 | **REFUTES** "probably dead UI code" [FW §5.2] (on 2.32.00 it returns the same data as `/settings/network`) |
| `/api/settings/{users, users_read_only, network, network-bond, network-link, static-ipv6, date-time, services, service-sessions, dns, dns/*, smtp, log, log-policy, channels, pam-order, ncsi-interfaces, ncsi/mode, firewall, firewall-*-rules, web_server_instances, mouse, ssl/certificate, ssl/certificate-info, active-directory-settings, ldap-settings, radius/general, pef/*, video/*, media/*}` | 200 | 390–1710 | | all readable |
| `/api/settings/raid_management/*`, `sasit_management/*`, `licenses`, `/api/maintenance/dual_image_config` | **405** | | | `{"error":"Feature is not enabled","code":8500}`. Matches [HAR §4.6] `FEATURE_NOT_ENABLED=405` and [FW §5.1] (plugins absent) |
| `/api/system_inventory/{cpu_info,dimm_info,pci_info,device_count}` | **500** | | | `code 11272` "Error in Getting the Device Count Information" (host off / no inventory pushed) |
| `/api/tasks`, `/api/scripts` | **500** | | | `code 1301` "Error getting AE tasks/scripts" |
| `/api/hsc-info` | **500** | | | `code 1392` |
| `/api/logs/help-data` | **500 (HTML)** | | | lighttpd HTML 500 page; **the session was invalid immediately afterwards** and the `spx_restservice` PID in the audit log changed. Probably a FastCGI crash, but not proven, because another client was active at the same moment. Avoid |
| `/api/host_inventory/*` (8) | 200 | 400–525 | | mostly empty or `NA` (host off; nothing pushed over the host interface) |
| `/api/status/uptime` | 200 | | | `poh_counter_reading` (power-on hours counter), not BMC uptime |
| `/api/maintenance/fwimage_location`, `signed-image-key`, `preserve` | 200 | | | `preserve` has a `redfish` key (Redfish config can be preserved across flashes) |
| `/api/logs/audit` | 200 | 778 | 1222–3005 | `spx_restservice` login/logout records with PID and source IP |
| `/api/logs/eventlog` | 200 | **4697** | **588690** | full SEL, newest first (id 566 → 1). **Query strings are ignored** (`?start=0&count=10` returned the same 588,690 bytes) |
| `/api/logs/event?LASTEVENTID=560` | 200 | 2099 | 3696 | **only ids 566..561**: the incremental fetch works on X470 2.32 [CAT] |
| `/api/logs/dashboardevent` | 200 | 1353 | 39244 | |
| unknown path | **404** | | | `{"error":"Invalid API Call","code":1010}` |

Skipped as unsafe or not read-only: `asrr/clearmcalog`, `asrr/bmccoldreset`, `asrr/java-console`, `asrr/mca-file`, `kvm/token`, `viewerlogin`, every `maintenance/*` flash/HPM/NM/restore/backup/firmware route, `operations/ssl/certificate/generate`, `settings/dns/restart`, `settings/service/web-restart`, `ae/*`, and downloads (`logs/event-file`, `raw-event-file`, `video-data`, `settings/download_image`).

### 5.3 LED write test (the one allowed write)

Original state: `led_status 0`. Using the UI's model (`uidcontrol`: `POST /api/actions/chassis-led`, `Content-Type: application/json`, `{"uid_command":N}`):

| Step | Request | Result |
|---|---|---|
| read | `GET /api/chassis` | `{"power_status":0,"led_status":0}` |
| on | `POST /api/actions/chassis-led {"uid_command":2}` | **200** in 1.19 s, body echoes `{ "uid_command": 2 }`. No ETag, no Location, no task |
| verify | `GET /api/chassis` (immediately, and again after 2 s) | `led_status 2` |
| off | `POST ... {"uid_command":0}` | 200, echoes `{ "uid_command": 0 }` |
| verify | `GET /api/chassis` (immediately, and again after 2 s) | `led_status 0` (**restored**) |

The write is synchronous, needs no If-Match/ETag, and takes effect at once. `led_status` uses the same codes as `uid_command` (0 off, 2 on; 1 = blink per [HAR §7], not tested). **CONFIRMS** [HAR §7] / [CAT]. The Redfish `IndicatorLED` PATCH could not be tested.

### 5.4 Logout

`DELETE /api/session` (cookie + CSRF) → `200 {"ok":0}`. Replaying the old cookie and CSRF afterwards → **401**, and a second DELETE → 401. **CONFIRMED** (the logout path was missing from the HAR). All 4 `/api` sessions we created were logged out. The session killed by `help-data` was already invalid.

---

## 6. Session expiry and invalid-session behaviour

**Status: CONFIRMED [FW §6.3] ("missing or invalid token gets 401"), with a NEW body format.**

| Case | Status | Body | Set-Cookie |
|---|---|---|---|
| Valid cookie, **no** `X-CSRFTOKEN` | 401 | `{ "cc": 7, "error": "Invalid Authentication" }` | none |
| Valid cookie, wrong CSRF | 401 | same | none |
| Bogus cookie, valid CSRF | 401 | same | **new `QSESSIONID=...; path=/`** |
| No cookie | 401 | same | new `QSESSIONID` |
| Cookie after logout | 401 | same | new `QSESSIONID` |
| Session invalidated after the `help-data` 500 | 401 | same | n/a |

- The 401 body uses `cc` (IPMI-style completion code 7), **not** the `{"error","code"}` shape that other errors use. The Go client should key on HTTP 401 alone, then re-login once and retry.
- On 401 the server hands out a fresh unauthenticated cookie. A cookie jar will overwrite the old value, which is harmless.
- Missing and wrong CSRF give the same response.
- Real idle timeout: `settings/services` → `web time_out 1800` s, `maximum_sessions 148`, `active_session 129` while we held one session (128 is the baseline, so 1 active). This matches [FW §6.3]. Not waited out.

---

## 7. Latency and concurrency

Round-trip time is about 72 ms (TCP connect). Every curl opens a new TLS connection.

| Measurement | Result |
|---|---|
| TLS handshake (`appconnect - connect`) | **230–520 ms**: OpenSSL 1.0.x with RSA-2048 key exchange on the BMC CPU |
| `/api/chassis`, 5 sequential, new connection each | total 484–741 ms; server time (ttfb − tls) about 110–240 ms |
| `/api/chassis`, 3 on **one keep-alive connection** | 570 ms, then **127 ms, 117 ms** |
| Redfish root, 5 sequential | total 504–745 ms; server time about 200–380 ms |
| `/api` 3 sequential vs 3 parallel | wall 1.35 s vs **0.61 s**; server times in parallel were 118 / 236 / 137 ms (a small overlap penalty) |
| Redfish 3 sequential vs 3 parallel | wall 2.97 s vs 1.33 s; server times in parallel were 441 / **568 / 812 ms**, against about 380–470 ms sequentially: **requests queue (single-threaded Lua/Turbo)** |

- Parallelism mostly hides TLS handshakes; the backends themselves serialise.
- **CONFIRMED** [HAR §0] item 2 and [FW §0] "max-procs 1".
- Recommendation: keep 1 to 2 long-lived keep-alive connections per BMC, and serialise requests on each.

---

## 8. TLS

**Status: protocols CONFIRMED; certificate, client-cert request and ciphers REFUTED/NEW, compared with [FW §3.4] (3.02.00 defconfig). The difference is likely version-related: 2.32.00 ships a different lighttpd TLS config and cert.**

| Item | Live 2.32.00 | [FW §3.4] (3.02.00) |
|---|---|---|
| TLS 1.0 | **accepted** (`AES256-SHA`) | predicted accepted ✔ |
| TLS 1.1 | **accepted** (`AES256-SHA`) | ✔ |
| TLS 1.2 | accepted (preferred `AES256-GCM-SHA384`) | ✔ |
| TLS 1.3 | **rejected** (handshake_failure alert 40) | ✔ |
| TLS 1.2 suites accepted | `AES256-GCM-SHA384, AES128-GCM-SHA256, AES256-SHA256, AES128-SHA256, AES256-SHA, CAMELLIA256-SHA, AES128-SHA, CAMELLIA128-SHA`. **All RSA key exchange** | predicted ECDHE-RSA-AES-GCM on secp384r1. **REFUTED** |
| ECDHE (P-256/P-384/P-521/X25519) | **none negotiated** | **REFUTED** |
| Client-cert request | **none** ("No client certificate CA names sent") | predicted CertificateRequest (`verifyclient.activate=enable`). **REFUTED** |
| Secure renegotiation | supported; compression none | n/a |
| Certificate subject = issuer | `C=US, ST=Georgia, L=Atlanta, O=American Megatrends Inc, OU=Service Processors, CN=AMI, emailAddress=support@ami.com` (self-signed) | predicted `CN=www.ami.com` (Norcross), 2018–2037. **REFUTED** |
| Validity | 2016-06-01 → **2026-05-30 (EXPIRED)** | |
| Key / signature | RSA-2048, sha256WithRSAEncryption, serial `92046422C980E206` | |
| SHA-256 fingerprint | `6D:EC:FE:56:53:D9:80:0B:55:AB:8F:44:D3:AB:52:AC:8C:0A:B5:AC:92:35:95:66:BA:51:1E:D6:34:36:BC:D0` | |
| Is it the AMI default? | **Yes. It is byte-identical to the 3.02.00 image's `root/etc/defconfig/certs/server.pem`** (the stunnel cert [FW §3.1], also expired 2026-05-30). The private key sits next to it in the image (`certs/privkey.pem`). `/api/settings/ssl/certificate-info` dates it `May 11 2021`, which is the firmware build date, so it was never regenerated | |

Go client implications:

- Pin per-BMC (trust-on-first-use) or skip verification. The cert is expired, shared by every unit, and its private key is public, so it **provides no authentication**. Recommend that operators regenerate it (needs approval, below).
- Set `MinVersion: tls.VersionTLS12`.
- Set `CipherSuites: []uint16{tls.TLS_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_RSA_WITH_AES_256_GCM_SHA384}` explicitly. RSA key exchange is not in Go's defaults since 1.22; the alternative is `GODEBUG=tlsrsakex=1`.
- There is no forward secrecy: a captured session plus the public firmware key decrypts it.

---

## 9. Ports

TCP connect test with a 3 s timeout, one port at a time.

| Port | Result | Static [FW §3.1] | Note |
|---|---|---|---|
| 22 | **open**, `SSH-2.0-OpenSSH_7.3p1` | on ✔ (same OpenSSH as 3.02) | |
| 80 | **open**, 301 → https (both `/redfish` and `/api`) | on ✔ | |
| 443 | **open** | on ✔ | |
| 623/tcp | **open** | only UDP listed | NEW: something listens on TCP 623 (not identified) |
| 623/udp | **RMCP presence pong** (28 bytes, ASF type 0x40, supported entities 0x81 = IPMI) | on ✔ | |
| 5120, 5122, 5123, 5124, 5126, 5127 | closed/filtered | predicted on | explained: **single-port mode is on** (`settings/media/adviser singleport_status 1`, `runtime SINGLE_PORT_APP enabled 1`), so media and KVM go over 443 websockets. `settings/services` reports the ports with bit 31 set (`-2147478528` = 0x80000000\|5120) |
| 7578, 7582 | closed/filtered | predicted on | same single-port explanation |
| 9080 | closed/filtered | loopback-only ✔ | good |
| 9090 | closed/filtered | absent on X470 ✔ [RF §9] | |
| 4433 | closed | not active ✔ | |

---

## Feature matrix (where each proxy feature can be served from on 2.32.00)

| Feature | Redfish (2.32.00, this account) | Legacy `/api` | IPMI-over-LAN |
|---|---|---|---|
| Service discovery / version | **Y** (root, `odata`, `$metadata` unauthenticated) | Y (`/api/configuration/project`, `firmware-info`) | Y (Get Device ID; not run, no ipmitool) |
| Power state + on/off/cycle/reset/soft-off | blocked (401) | **Y** `GET /api/chassis`, `POST /api/actions/power` (§13) | expected (chassis control; not tested) |
| Identify LED | blocked | **Y** `POST /api/actions/chassis-led` (§5.3, §12) | expected (chassis identify; not tested) |
| One-time boot override | blocked | **N** (no endpoint in UI JS or catalog) | expected (chassis bootdev; not tested) |
| Sensors / thresholds | blocked | **Y** `/api/sensors`, `/api/detail_sensors_readings` | expected (SDR; not tested) |
| FRU / inventory | blocked | Y `/api/fru`; host inventory empty with host off | expected |
| SEL (full + incremental) | blocked | **Y** `/api/logs/eventlog`, `/api/logs/event?LASTEVENTID=` | expected |
| Audit log | n/a | Y `/api/logs/audit` | n/a |
| Firmware versions | blocked | Y `/api/asrr/fw-info` | partly |
| Network config (read) | blocked | Y `/api/settings/network` | expected (`lan print`) |
| Accounts / sessions (read) | blocked | Y `/api/settings/users`, `service-sessions` | n/a |
| KVM | n/a | Y: `GET /api/kvm/token` + `wss://<bmc>/kvm` (§14) | SOL (not tested) |
| Virtual media | blocked | Y (H5Viewer `cd-server` websocket, `/api/settings/media/*`); not mounted | n/a |
| Events push (SSE / subscriptions) | blocked (no SSE in schema set) | N (poll) | PET traps (not tested) |

---

## 10. Second probe: `/api` session lifetime and failure modes

### 10.1 Idle timeout (controlled test)

One fresh `/api` session; each probe (`GET /api/chassis`) resets the idle timer if it succeeds. No other request used this session.

| Gap since previous request | Result |
|---|---|
| 30 s | 200 |
| 60 s | 200 |
| 90 s | 200 |
| 120 s | 200 |
| 150 s | 200 |
| 200 s | 200 |
| 300 s | 200 |
| then `DELETE /api/session` | 200 `{ "ok": 0 }` |

- The idle timeout is **at least 300 s** and consistent with the configured 1800 s (`settings/services` `web time_out 1800`). 1800 s was not waited out.
- An earlier session of this probe was found dead after only 123 s idle. The audit log shows `spx_restservice` PID changes in that window, plus logins from the other probe, so that was a backend restart, not an idle timeout. **For the proxy:** treat any 401 as "re-login once and retry", whatever the expected lifetime.

### 10.2 Backend restarts (`spx_restservice` PID churn)

`/api/logs/audit` tags each record with the `spx_restservice` PID. During the two probes the PID changed 3035 → 4441 → 5380 → 6924 → 7349 → 8502 → 8575 → 19026 (06:42–07:08 UTC). Correlated events:

- 06:42 and 06:43–06:44: the second probe's two `GET /api/logs/help-data` calls (500 HTML), each followed by the session becoming invalid. **Reproduced twice** in a controlled sequence (`GET /api/chassis` 200 → `GET /api/logs/help-data` 500 → `GET /api/chassis` 401).
- `GET /api/asrr/mca-file` → **502** (empty body, `content-type: text/plain`) did *not* kill the session.
- One of the settings pages fetched `GET /api/settings/download_image` → **502** during the UI walk; a PID change happened around the same time. Not proven to be the cause.

The 502s are lighttpd reporting that the FastCGI backend went away. **Proxy deny-list:** `/api/logs/help-data`, `/api/asrr/mca-file`, `/api/settings/download_image` (and every flash/restore/reset path).

### 10.3 CSRF and session errors (second probe, consistent with §6)

Valid cookie + valid CSRF 200; no CSRF, wrong CSRF (`AAAAAAAA`), CSRF without cookie, garbage 30-char cookie: all **401 `{ "cc": 7, "error": "Invalid Authentication" }`**. Unknown path and wrong method (`PUT /api/chassis`): **404 `{ "error": "Invalid API Call", "code": 1010 }`**. After a real logout, the old cookie gets 401 and a second `DELETE /api/session` also gets 401.

Error codes seen in the walk (all JSON `{"error","code"}` unless noted):

| Status | Code | Example endpoint | Message |
|---|---|---|---|
| 404 | 1010 | `settings/sensor-threshold`, `settings/fan/*`, `raid_management/create_virtual_drive`, `media/remote/rmediarefreshlist` | Invalid API Call |
| 405 | 8500 | `raid_management/*`, `sasit_management/*`, `license(s)`, `maintenance/dual_image_config` | Feature is not enabled |
| 500 | 11272 | `system_inventory/*` | Error in Getting the Device Count Information |
| 500 | 9461 / (LDAP) / 1194 | `active-directory-users`, `ldap-users`, `radius/advanced` | AD/LDAP/RADIUS not enabled |
| 500 | 1232 | `video/sol-triggers`, `video/sol-remote-storage` | Error getting video triggers |
| 500 | 1377 | `maintenance/hpm/componentversions` | Error in HPM Finish Firmware Upload (a read endpoint returning an upload error) |
| 401 | `cc` 7 | any, with a bad session | Invalid Authentication |

### 10.4 Latency with and without keep-alive (second probe)

`/api/chassis` 5× on **one** connection: 561 ms (incl. TLS 380 ms), then **111, 118, 131, 112 ms**. Each new curl process pays 0.4–0.7 s. The HAR's ~84 ms median for `chassis` is the keep-alive figure. **Keep connections alive; never open one TLS connection per request.**

### 10.5 Legacy GET walk diff against the HAR

All 29 GET endpoints that appear in `har-endpoints.json` returned the **same JSON shape** (keys and types) as the HAR, except `/api/asrr/product-name` (HAR 500 `{error,code}`, live 200 `{sys_name}`). Total walked: 130 GET paths (HAR + catalog entries that are GET and do not contain flash/upgrade/reset/restore/delete/nm_/download). Results: 99 × 200, 7 × 404, 12 × 405, 10 × 500 plus the `help-data` 500, and 1 × 502.

New details:

- `GET /api/kvm/token` → `{ "client_ip", "token": <16 chars [A-Za-z0-9]>, "session": <30 chars> }`. The 30-char `session` value has the same length and charset as `QSESSIONID`, so the KVM token endpoint very likely hands the (non-HttpOnly anyway) web session id to the viewer. Both are sent inside the first KVM websocket frames (§14).
- `/api/settings/services`: web `max 148 / active 129` (128 baseline), kvm `max 132`, ssh `time_out 600`, `solssh` state 0 (port 52123, closed). Port fields with bit 31 set mean "disabled because single-port is on".
- `/api/settings/users_read_only` additionally exposes per-user SNMP settings (`snmp_access`, `snmp_authentication_protocol "sha"`, `snmp_privacy_protocol "des"` on the anonymous slot).
- `/api/maintenance/system_admin`: `username "sysadmin"`, `enable_user_access 1`: the root-equivalent shell account is enabled.
- `/api/settings/log`: audit and system logs on, remote syslog enabled with no server.

---

## 11. Second probe: IPMI over LAN (raw RMCP, no ipmitool)

`ipmitool` is not installed and `sudo` needs a password, so the `-I lanplus` command set (`mc info`, `chassis status`, `sdr elist`, `sel info`, `fru`, `lan print 1`, `channel getciphers`, cipher 3/17 logins, `-I lan`) was **not run**. Instead, three *sessionless* IPMI requests were hand-built in Node (`dgram`, no library) and sent to UDP 623. None of them authenticates or changes state.

| Request | Result |
|---|---|
| ASF Presence Ping | Pong: supported entities `0x81` (IPMI supported, ASF 1.0) |
| Get Channel Authentication Capabilities (channel E, Administrator, IPMI v2 extended bit) | cc 0, channel **1**; auth types byte `0x84` → **MD5 only** (no none/MD2/straight password); status `0x14` → non-null usernames enabled, **per-message auth disabled**, anonymous login off, **KG = default (all zero)**; extended caps `0x03` → **IPMI 1.5 and IPMI 2.0 both supported**; OEM ID `0x00C1D6` (ASRock) |
| Same, without the v2 bit | cc 0, `0x04` (MD5), `0x14` |
| Get Channel Cipher Suites (list by suite, 4 pages) | suites **0, 1, 2, 3, 6, 7, 8, 11, 12, 15, 16, 17** |

Cipher suite decode (auth / integrity / confidentiality):

| ID | Auth | Integrity | Confidentiality |
|---|---|---|---|
| 0 | none | none | none |
| 1 | HMAC-SHA1 | none | none |
| 2 | HMAC-SHA1 | HMAC-SHA1-96 | none |
| 3 | HMAC-SHA1 | HMAC-SHA1-96 | AES-CBC-128 |
| 6 | HMAC-MD5 | none | none |
| 7 | HMAC-MD5 | HMAC-MD5-128 | none |
| 8 | HMAC-MD5 | HMAC-MD5-128 | AES-CBC-128 |
| 11 | HMAC-MD5 | MD5-128 | none |
| 12 | HMAC-MD5 | MD5-128 | AES-CBC-128 |
| 15 | HMAC-SHA256 | none | none |
| 16 | HMAC-SHA256 | HMAC-SHA256-128 | none |
| 17 | HMAC-SHA256 | HMAC-SHA256-128 | AES-CBC-128 |

This matches the "usual AMI SPX" guess in [FW §3.5] exactly (that analysis could not read the compiled table). **Suite 0 is advertised**; whether it is allowed at a usable privilege level is in the LAN configuration parameter 24 (needs a session) and was not tested. The proxy should use suite 17 (or 3) and operators should restrict suite 0 and disable IPMI 1.5. TCP 623 is also open (unknown listener; a plain HTTP or TLS request to it times out).

---

## 12. Second probe: LED write (repeat of §5.3)

`GET /api/chassis` `{power_status 0, led_status 0}` → `POST /api/actions/chassis-led {"uid_command":2}` 200 in 0.89 s, echoes `{ "uid_command": 2 }` → `led_status 2` → `{"uid_command":0}` 200 in 0.89 s → `led_status 0` (**restored**). `uid_command` values from the UI template: **2 = Turn On, 1 = Temporary On (15 s blink), 0 = Turn Off**. Identical to §5.3.

---

## 13. Second probe: legacy power on → off (one cycle)

The host was OFF before the test. Polling `GET /api/chassis` at about 0.5–1 s intervals.

| Step | Request | Result |
|---|---|---|
| start | `GET /api/chassis` | `power_status 0` |
| on | `POST /api/actions/power {"power_command":1}` | 200 in 0.65 s, echoes `{ "power_command": 1 }`; `power_status 1` at the **first poll, 1.25 s** after the request |
| +20 s | `GET /api/sensors` | 24 of 41 sensors readable (VCPU 1.1 V, 12V 12.5 V, CPU 28 °C, MB 20 °C, FAN2/3/5 ~7300 RPM; PSU sensors still `213`) |
| soft off | `POST ... {"power_command":5}` (ACPI soft shutdown) | 200 in 0.96 s, echoes the body, **but the host stayed on for 60+ s** (the host was ~22 s into POST, no OS ACPI handler) |
| hard off | `POST ... {"power_command":0}` | 200 in 0.51 s; `power_status 0` after **1.14 s** |
| end | `GET /api/chassis` | `power_status 0`, `led_status 0` (**original state restored**) |

- The action endpoint is fire-and-forget: 200 + echo means "accepted", not "done". There is no task or job id. The proxy must poll `power_status` to confirm, and treat `power_command 5` as best-effort (it needs a running OS); fall back to 0 after a timeout, as Redfish `GracefulShutdown` → `ForceOff` clients expect.
- On/off transitions are fast (~1.1–1.3 s to reflect in `/api/chassis`).
- Total power cycles in this probe: **one** (on, then off). The Redfish `ComputerSystem.Reset` comparison was **not possible** (Redfish auth).

---

## 14. Second probe: web UI walk, H5Viewer KVM and logout (Playwright)

Headless Chromium (Playwright 1.63.0, chromium-1243; NSS/NSPR libraries extracted locally from the Ubuntu `.deb`s since there is no sudo). Every non-GET request except `POST/DELETE /api/session` and `POST /api/viewerlogin` was aborted by a route filter, as were GETs to help-data/flash/restore/reset paths. **Nothing was blocked**: no visited page issued a write on its own.

### 14.1 Pages visited (view only, screenshots in the scratchpad)

Dashboard, sensors, system information, FRU, power source, system inventory, IPMI event log, video log, settings (network/IP/bond/link/NCSI/DNS, date-time, users, services, service sessions, fan, media, SSL, log, PEF, SMTP, PROCHOT, BSOD, keep-NCSI-link-up, sensor thresholds, video, external users, firewall, PAM order, web server instances, mouse), remote control, power control, miscellaneous (UID, POST snoop), image redirection, maintenance (fwinfo, firmware update, BIOS update, dual image, backup config, preserve, system admin, fwimage location, cold reset). 165 API calls, all GET except the login.

New observations compared with [HAR]:

- Cookies set: `QSESSIONID` (30 chars) and the UI's own `refresh_disable` (1 char), both `path=/`, session-only, **not HttpOnly, not Secure**, SameSite defaulted to Lax by the browser.
- After login the UI stores in **localStorage**: `garc` (the CSRF token), `session_id`, `username`, `privilege`, `extended_privilege`, `kvm_access`, `vmedia_access`, `features`, `server_addr`, `remember_usrname`, ...; the same set in sessionStorage. The viewer adds `token` (sessionStorage) and `isActiveKVM` (localStorage). **The CSRF token and session id are readable by any script on the origin.**
- Polling: `/api/chassis` every 5 s and `/api/configuration/runtime` on almost every view change (56 calls in ~4 min).
- New endpoints called by pages and not in the HAR: `GET /api/settings/sensor-threshold/sensor_thresholds` (404), `GET /api/settings/download_image` (502, see §10.2), `GET /api/settings/packages`, `service-sessions`, `maintenance/backup_config`, `preserve`, `system_admin`, `dual_image_config` (405).
- The `#remote-control` page reached by hash navigation rendered only "Install SPX KVM Remote App"; reached through the sidebar menu it shows two buttons: `#download` "Launch KVM" (H5Viewer) and `#downloadjavaconsole` "Launch Java KVM".

### 14.2 H5Viewer KVM

Launching opens `https://<bmc>/viewer.html` (window name `H5Viewer`, title "Remote KVM"). Sequence:

1. Viewer GETs `/api/configuration/project`, `/api/configuration/runtime`, `/api/settings/media/adviser`, **`/api/kvm/token`**, `/libs/kvm/ast/decode_worker.js` (AST2500 video decoder in a Web Worker), `/api/settings/media/remotesession`, `/api/settings/media/instance`.
2. WebSocket **`wss://<bmc>:443/kvm`**: no query string, no token in the URL.
   - Request headers: `Sec-WebSocket-Protocol: binary, base64`, `Origin: https://<bmc>`, `Cookie: QSESSIONID=<30>; refresh_disable=1` (60 chars), `Sec-WebSocket-Extensions: permessage-deflate; client_max_window_bits` offered.
   - Response: **101**, `Sec-WebSocket-Protocol: binary`, **no extensions** (no compression), no `Server` header.
3. Frames (binary; every frame observed starts with a 1-byte message type, and the shortest frames are 8 bytes, which suggests an 8-byte header; the exact layout was not decoded):
   - client → server #1: type `0x3a`, **397 bytes**, contains the 16-char KVM `token` from `/api/kvm/token` in clear (redacted here).
   - client #2–#4: 8–9 byte control messages (types `0x22` keep-alive, `0x33`, `0x28`).
   - client #5: type `0x15`, 43 bytes, contains the **30-char session id** (the `session` field from `/api/kvm/token`).
   - server → client: 8–10 byte status messages (types `0x17`, `0x13`, `0x14`, `0x0a`, `0x35`, `0x34`, `0x09`), then type `0x25` 313 bytes (service info: `"web"`, `"both"` interface strings), type `0x27` 142 bytes (active user list, begins with the user name `admin`), type `0x26`, then type `0x03` video frames.
   - 22 frames sent and 29 received in ~30 s. Type `0x22` 8-byte frames are a periodic client keep-alive.
4. The audit log records `adviserd ... kvm Login from IP:<client> user:admin` and, on window close, `KVM logout`.

**For the proxy:** forwarding the websocket needs the upgrade, the `binary` subprotocol, and the session cookie; the KVM token travels in-band, so a proxy that rewrites auth must also mint a KVM token (`GET /api/kvm/token` on the user's BMC session) rather than substituting headers. The KVM session ends when the websocket closes.

### 14.3 Virtual media in the viewer

The H5Viewer (2.32.00) has no separate "Virtual Media" menu or wizard. The toolbar shows **"Browse File"** and **"Start Media"** next to "Stop KVM", "Video", "Mouse", "Options", "Keyboard", "Send Keys", "Hot Keys", "Video Record", "Power", "Active Users", "Language", "Help". Image redirection would open a second websocket to `wss://<bmc>/cd-server` (protocols `binary, base64`) after "Start Media"; per the instructions **no image was chosen and Start Media was not clicked**, so that websocket was not observed. `settings/media/general` shows local and remote media support disabled (`local_media_support 0`, `remote_media_support 0`), and `media/instance` allows 1 CD / 1 FD / 1 HD.

### 14.4 UI logout

Clicking "Sign out" shows a JS `confirm()` ("Would you like to Sign out of this Session?"). On OK: **`DELETE /api/session`** with `Cookie`, `X-CSRFTOKEN` and `X-Requested-With`, response 200, then the UI clears its cookies client-side and goes to `#login`. The UI JS also contains a second logout path behind a "logout warning" confirm: `DELETE /api/settings/service-sessions/<session_id>`.

Two of the second probe's UI runs failed to click Sign out (the link sits inside a hidden dropdown), so **two UI sessions were left to expire** (session ids 21 and 22, created about 07:08 and 07:13 UTC; with no traffic after ~07:15 they idle out at the 1800 s web timeout, ~07:45 UTC). Terminating them through `DELETE /api/settings/service-sessions/<id>` was refused by this session's safety policy, so it was not done.

---

## 15. Second probe: Redfish details not covered above

- **Redfish retry at 07:18:38 UTC**, 40 min after the first attempt (06:38:10): Basic auth on `/redfish/v1/Systems` → **401 `Security.1.0.AccessDenied`** again. This rules out a short (30 s) lockout left by earlier attempts. The account simply does not authenticate on Redfish on 2.32.00. No test account was created (out of scope); see "Needs approval" item 1.
- Query options on the unauthenticated root: `$select=Name,RedfishVersion` → 200 with just those properties; `$expand=.` and `$expand=*($levels=1)` → 401; **`$top=1` and `$filter=...` → 501 `Base.1.0.QueryNotSupportedOnResource`**; `$bogus=1` → 501 `Base.1.0.QueryNotSupported`. So `$top/$skip` are refused on non-collections (collections untested).
- Conditional GET: `If-None-Match: W/"1794669113"` on the root → **304**.
- `POST`, `PATCH`, `PUT`, `DELETE` on the root → 405 JSON `HttpStatus.1.0.MethodNotAllowed` with `Allow: GET`, but each took **2.3–4.2 s** (GET takes ~0.5 s): the write path is slow even when it rejects.
- Connection cap test (once): **12 parallel** new connections to the root → **12 × 200**, no 403/429/503. TLS handshakes were spread over 0.36–1.6 s and completions over 4.5–7.1 s: no per-IP cap (unlike the B650's 10), but everything queues.
- No route oracle: unauthenticated GETs return 401 for existing and non-existent paths alike, so the URI inventory beyond the root links cannot be confirmed without credentials. The machine-readable inventory of what the second probe observed is `docs/x470/redfish-uri-inventory.json` (the first probe's crawl is `live-redfish-inventory.json`).
- `SessionTimeout` value/units, max concurrent sessions, `X-Auth-Token`/`Location` format, ETag/If-Match on PATCH, boot override and `ResetType` values: **not measurable** (no session).

---

## Redfish URI inventory (what was observable)

| URI | Status (no auth) | Type | Allow / methods | Actions |
|---|---|---|---|---|
| `/redfish` | 200 | version document | n/a | n/a |
| `/redfish/v1` | 302 → `/redfish/v1/` | n/a | n/a | n/a |
| `/redfish/v1/` | 200 | `#ServiceRoot.v1_3_1.ServiceRoot` | `GET` (OPTIONS → 405, HEAD → 200) | none |
| `/redfish/v1/odata` | 200 | OData service document (16 singletons) | `GET` | n/a |
| `/redfish/v1/$metadata` | 200 | `application/xml`, 112 schema refs | `GET` | n/a |
| `/redfish/v1/GlobalLuaTables` | **200** | collection (no `@odata.type`) | none sent (static: GET, DELETE) | n/a |
| `/redfish/v1/Systems` | 401 | (ComputerSystemCollection) | n/a | n/a |
| `/redfish/v1/Systems/Self` | 401 | n/a | n/a | n/a |
| `/redfish/v1/Chassis` | 401 | n/a | n/a | n/a |
| `/redfish/v1/Chassis/Self` | 401 | n/a | n/a | n/a |
| `/redfish/v1/Managers` | 401 | n/a | n/a | n/a |
| `/redfish/v1/Managers/Self` | 401 | n/a | n/a | n/a |
| `/redfish/v1/SessionService` | 401 | n/a | n/a | n/a |
| `/redfish/v1/SessionService/Sessions` | 401 (GET and POST) | n/a | n/a | n/a |
| `/redfish/v1/AccountService` | 401 | n/a | n/a | n/a |
| `/redfish/v1/EventService` | 401 | n/a | n/a | n/a |
| `/redfish/v1/TaskService` | 401 | n/a | n/a | n/a |
| `/redfish/v1/UpdateService` | 401 | n/a | n/a | n/a |
| `/redfish/v1/TelemetryService` | 401 | n/a | n/a | n/a |
| `/redfish/v1/CompositionService` | 401 | n/a | n/a | n/a |
| `/redfish/v1/Registries` | 401 | n/a | n/a | n/a |
| `/redfish/v1/JsonSchemas` | 401 | n/a | n/a | n/a |
| `/redfish/v1/DynamicExtension` | 401 | n/a | n/a | n/a |
| `/redfish/v1/configurations` | 401 | n/a | n/a | n/a |
| `/redfish/v1/CertificateService` | 401 (not linked; no schema, so almost certainly absent) | n/a | n/a | n/a |
| `/redfish/v1/NoSuchThing` | 401 (so 404 is masked when unauthenticated) | n/a | n/a | n/a |

---

## Needs approval (next tests)

1. **Redfish credentials** (this blocks everything else). Either:
   - (a) the user supplies the password for the Redfish `Administrator` account, or another known Redfish-capable account; or
   - (b) approval to create a dedicated read-only or Administrator account with a password of at least 8 characters through `/api/settings/users` (an account change), then retry Redfish Basic and session login once.

   Testing hypothesis B (Redfish password policy) needs (b).
2. With credentials: the full authenticated crawl (600 cap). Measure `SessionTimeout` and its units, `MaxSessions`, the `X-Auth-Token` length and `Location`, the `$top`/`$skip`/`nextLink` behaviour on `Managers/Self/LogServices/SEL/Entries`, `$expand` cost, `Allow` per resource, and `ResetType` allowable values. Read `Managers/Self.FirmwareVersion`.
3. With credentials: the Redfish `Chassis/Self` `IndicatorLED` PATCH (Lit → Off) with and without `If-Match`, to learn about 412/428, the ETag granularity and whether a task is created.
4. **Regenerate the BMC TLS certificate** (`/api/operations/ssl/certificate/generate` or upload): a state change that fixes the expired, publicly keyed cert.
5. Confirm the `/api/logs/help-data` crash once, on purpose, with the coordinator's OK (it may restart `spx_restservice` and drop everyone's web sessions).
6. Identify the **TCP 623** listener (one benign probe, such as an RMCP+ Get Channel Auth Capabilities over TCP, or a banner read).
7. `DELETE /redfish/v1/GlobalLuaTables` without auth, on a lab unit only, to confirm whether unauthenticated cache-clearing works (a state change).
8. Upgrade a lab unit to 3.02.00 and repeat this probe, to separate version-related differences (TLS config, cert, Redfish account mirroring, `BIOS/get/status`, `sensor-threshold`) from mistakes in the static analysis.
9. Power-on tests (host inventory, `system_inventory/*`, sensors with the host running) need a host power-on, which is forbidden without approval.
   *Update (second probe):* one legacy power-on/off cycle was later done within the original scope (§13); the host was in POST only ~20 s, so host inventory and `system_inventory/*` with a booted OS are still untested.
10. **Not tested at all (second probe), and why:**
    - Redfish full walk, Allow/ETag per resource, actions and `@Redfish.AllowableValues`, SessionTimeout, max sessions, `$expand` cost, If-Match PATCH, Redfish boot override, Redfish `ComputerSystem.Reset`, `IndicatorLED` PATCH: Redfish rejects the credentials.
    - `/api` one-time boot override: no such endpoint exists on 2.32.00.
    - Every `ipmitool` command (`mc info`, `chassis status`, `sdr elist`, `sel info`, `fru`, `lan print 1`, cipher 3/17 logins, `-I lan`): ipmitool not installed, no passwordless sudo. Only sessionless raw RMCP queries were run (§11).
    - Virtual media mount (forbidden), H5Viewer "Start Media" and the `cd-server` websocket.
    - SSH login, SNMP, TCP 623 listener identity.
    - Killing the two leaked UI sessions via `DELETE /api/settings/service-sessions/<id>` (refused by the session's safety policy; they expire on their own).
    - Anything on the forbidden list (firmware, factory reset, BMC reset, network/user/service/cert/LDAP changes, log clearing).

---

## Method and hygiene

- Every request was sequential with 200–300 ms spacing and a 30 s timeout, except the deliberate 3-way parallel test. `-k` was always used. No 403 or 503 responses occurred.
- Sessions: 4 `/api` sessions were created and all were logged out (one was already killed by `help-data`). No Redfish session was created.
- Redfish auth failures: 2 credentialed attempts (Basic once, session POST once), about 3 minutes apart.
- Writes: only the UID LED (on, then restored to off).
- **Second probe (session `4c303595`):** one request at a time with 150–300 ms spacing (one deliberate 12-way parallel test on the unauthenticated Redfish root). `/api` sessions created: 9 (idle-test, walk, UI runs); all logged out except the two that were killed by `help-data` and the two leaked UI sessions noted in §14.4. Redfish credentialed failures: 4 (session POST, Basic ×2 at 06:38–06:42, Basic retry at 07:18). One `/api` login failure (06:38:26) was a client-side form-encoding mistake. Writes: UID LED on → off; one power on → (soft-off ignored) → hard off; host left OFF, LED off. Raw output (contains MACs, serials, tokens, not for `docs/`): `/tmp/claude-1000/-opt-sparked-asrockrack-api/4c303595-4244-4c22-8dc9-6968bfcb1de5/scratchpad/probe/` (`raw/`, `ui/*.har`, `ui/shots/`).
- Scripts and raw output (first probe): `/tmp/claude-1000/-opt-sparked-asrockrack-api/152d9ce5-3665-4f40-8521-6ab266196a9e/scratchpad/probe-x470/` (`api_sweep.py`, `rf_crawl.py`, `api1..5/`, `rf_unauth/`, `tls_full.txt`). These raw files contain MACs, the BMC IP and serial numbers, so do not copy them into `docs/`.
