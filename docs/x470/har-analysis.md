# X470D4U BMC web UI: HAR analysis

Source: `harfiles/x470d4u.har`. This is a preliminary assessment for the native Go Redfish proxy. It documents only what the capture shows. Where the bundled web UI JavaScript (`/source.min.js`, which is *inside* the HAR as a response body) explains a value, that is marked **[JS]**. No firmware binary was disassembled.

Companion machine-readable file: `docs/x470/har-endpoints.json`. It holds one object per method+path, with a redacted request/response example, call count, intervals and timings.

---

## 0. Key findings (TL;DR)

1. **The southbound API is AMI MegaRAC SPX "`/api`" JSON over HTTPS on lighttpd.** It is session based. You log in with `POST /api/session` (form-urlencoded), then send a session cookie (`QSESSIONID`) plus an `X-CSRFTOKEN` header on every call.
2. **The BMC handles requests effectively serially.** A request that normally takes ~80 ms can take 1.7–2 s if heavier calls are in flight (`settings/network`, `static-ipv6`, `eventlog`). The proxy must **serialise or tightly limit concurrency (1–2) per BMC** and serve northbound reads from cache.
3. **Cost table.** Cheap: `chassis`, `chassis-status`, `sdr`, `fru`, `firmware-info`, `asrr/*` (~80–400 ms). Expensive: `logs/eventlog` (475 KB, ~3 s), `settings/network` (~1–1.6 s), `settings/users` (~1.7 s), `session` login (~1.5 s), `settings/static-ipv6` (~0.9 s).
4. **Power control:** `POST /api/actions/power` with JSON `{"power_command":N}`. The codes are 0=off, 1=on, 2=cycle, 3=hard reset, 5=ACPI soft shutdown **[JS]**. The response echoes the body. `power_status` flipped 0→1 within ~1.7 s.
5. **Sensors** (`/api/sensors`) return 41 IPMI sensors with converted readings, thresholds, a threshold mask and state flags. `accessible == 213` (0xD5) means "no reading", for example host-side sensors while the host is off or absent PSUs.
6. **`/api/configuration/project` advertises `REDFISH` and `REDFISH_HOSTINTERFACE` feature flags.** That is evidence *against* the assumption that the X470D4U (BMC fw 2.32.00) has no Redfish. The web UI never calls `/redfish/v1`, so this needs a direct probe (see section 12).
7. Boot-device override, virtual media control and user CRUD were **not exercised**. Logout was not captured either. See section 13.
8. **Security note:** the original HAR contains the **admin password in cleartext**, in the `POST /api/session` body. Treat the raw HAR as a secret, and consider rotating that BMC password.

---

## 1. Capture metadata

| Item | Value |
|---|---|
| HAR version / creator | 1.2 / WebInspector 537.36 (Brave 157 / Chromium 157 on Windows) |
| Page | `https://<BMC_HOST>/` (BMC at RFC1918 address `10.0.0.87`; client `10.0.0.252`) |
| Time span | 2026-11-16 05:58:44.688Z → 06:02:38.165Z (~3 min 54 s) |
| Entries | 176 total: 122 `/api/*` XHR, 53 static (HTML/JS/CSS/fonts/images/favicon), 1 unrelated third-party request (`cache.agilebits.com`, the 1Password browser extension; returned 404 and is ignored) |
| Unique API method+path | 32 (30 GET, 2 POST) |
| BMC identity (from responses) | Board `X470D4U`; BMC fw `2.32.00` (built `May 11 2021 17:53:41 CST`); BIOS `P4.20`; ME `0.14.0.27`; microcode `0a201009`; CPLD `N/A`; IPMI 2.0; Mfg ID 0x00C1D6 (= 49622, ASRock); Product ID 514 |
| Host state | Powered **off** at start. The user issued Power On at 06:00:41.450Z, and the host was on for the rest of the capture |
| Cookies | **Not present in the HAR.** The `Cookie`/`Set-Cookie` headers and `cookies[]` arrays are all empty, consistent with Chromium's default "sanitised" HAR export. The session cookie therefore has to be inferred (section 4.3) |
| TLS details | Not recorded by Chromium HAR (no `_securityDetails`). Every BMC request is `https`, `HTTP/1.1`, port 443 |

### UI navigation sequence (reconstructed)
05:58:44 load login page → 05:59:57 login → 06:00:01 dashboard (burst of 11 API calls) → 06:00:11–16 maintenance/firmware pages (polling paused) → 06:00:19 event log page (`sdr`, `eventlog`, `date-time`, `getmcalog`) → 06:00:24 video log → 06:00:26–29 host inventory pages → 06:00:30 FRU → 06:00:32 power supply / sensors pages → 06:00:39 power control page → **06:00:41 Power On** → 06:01:52–02:15 inventory, KVM settings (`media/adviser`), FRU, fan page (`sensors`, `GetFanDuty`) → 06:02:25–28 network settings (`network-bond`, `static-ipv6`, `network`) → end.

---

## 2. Transport, server banner, headers, caching

### 2.1 Server and protocol
- `Server: lighttpd` (no version) on every BMC response.
- HTTP/1.1 over TLS on :443. HTTP/2 is not used. Browser connections show `connect` ~40–100 ms, of which TLS is ~23–57 ms. 16 distinct TCP connections were used, reused heavily (up to 17 requests each).
- **Clock skew:** the BMC `Date` header was consistently **~4–6 s behind** the client clock. Do not use BMC time for cache freshness.

### 2.2 API (`/api/*`) response headers (identical on all 122)
```
Cache-Control: no-store, no-cache, must-revalidate, private
Pragma: no-cache
Content-Type: application/json
Transfer-Encoding: chunked
Server: lighttpd
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Date: <RFC1123>
```
- **No `Content-Encoding`.** API JSON is sent **uncompressed** even though the client sent `Accept-Encoding: gzip, deflate, br, zstd`. The 475 KB event log came over the wire as 475 KB.
- No `ETag` or `Last-Modified` on API responses, so conditional GET is impossible. The proxy needs its own TTL cache.
- 8 API responses also carried `Connection: close`, at random: `chassis` (idx 16, 113), `asrr/product-name` (500), `host_interface_pcie_device_function_info`, `configuration/runtime` (x2), `host_interface_processor_info`, `settings/network-bond`. **The Go client must tolerate server-initiated connection close and retry idempotent GETs on a new connection.**
- No CORS headers, and no `X-Frame-Options` on API responses.
- JSON formatting: AMI style `{ "k": v, ... }` with spaces. Floats are printed with 6 decimals (`3.400000`), and `/` is escaped as `\/` (`"N\/A"`). Any standard JSON decoder copes.

### 2.3 Static asset headers
`Cache-Control: no-store, no-cache, must-revalidate, private`, `Pragma: no-cache`, `Content-Encoding: gzip`, `ETag: "<number>"`, `Last-Modified: Thu, 01 Jan 1970 00:00:00 GMT`, `Accept-Ranges: bytes`, `X-Frame-Options: SAMEORIGIN`, `X-Content-Type-Options: nosniff`, `X-XSS-Protection: 1; mode=block`. Unknown paths (`/favicon.ico`, `/fonts/ionicons.ttf`, `/fonts/fontawesome-webfont.woff2`) return **200 `text/html`** with the 1280-byte SPA index page, not 404. So you **cannot** rely on HTTP 404 to detect missing resources on the static side. `GET /` is served without authentication.

### 2.4 Request headers the UI sends on API calls
```
Accept: application/json, text/javascript, */*; q=0.01      (Backbone fetches of /api/chassis send "*/*")
X-Requested-With: XMLHttpRequest
X-CSRFTOKEN: <CSRFToken>                                  (all calls after login; absent on POST /api/session)
Referer: https://<BMC_HOST>/
Content-Type: application/json                             (JSON POST/PUT)
Content-Type: application/x-www-form-urlencoded; charset=UTF-8   (login only)
Origin: https://<BMC_HOST>                                 (POSTs only)
Cookie: QSESSIONID=<...>                                   (inferred: stripped from HAR)
```
Recommendation for the Go client: send `X-CSRFTOKEN`, the `QSESSIONID` cookie, `Accept: application/json` and `X-Requested-With: XMLHttpRequest`. Sending `Referer`/`Origin` matching the BMC host is cheap insurance in case lighttpd/AMI checks them; the capture can't tell us whether it does.

---

## 3. Request conventions summary for a Go client

| Concern | Behaviour |
|---|---|
| Base URL | `https://<bmc>/api/` |
| TLS | Self-signed BMC cert expected (not visible in HAR), so the proxy needs a configurable CA or pinning |
| Auth | `POST /api/session` form → JSON with `CSRFToken`; cookie `QSESSIONID` set by server |
| Per-request | Cookie `QSESSIONID` + header `X-CSRFTOKEN` |
| Body encodings | Login: form-urlencoded. Actions/settings: `application/json` |
| Success | HTTP 200 + JSON (object or array) |
| Error | HTTP 4xx/5xx + JSON `{"error": "<text>", "code": <int>}` (observed: 500 / code 16005) |
| Concurrency | Treat the BMC as single-threaded (section 10.3) |
| Caching | No validators; the proxy must cache with TTLs |
| Compression | None on API responses |

---

## 4. Authentication flow

### 4.1 Login request (HAR entry 8)
```
POST /api/session HTTP/1.1
Host: <BMC_HOST>
Content-Type: application/x-www-form-urlencoded; charset=UTF-8
Accept: application/json, text/javascript, */*; q=0.01
X-Requested-With: XMLHttpRequest
Origin: https://<BMC_HOST>
Referer: https://<BMC_HOST>/
Content-Length: 52

username=admin&password=<REDACTED_PASSWORD>&certlogin=0
```
| Field | Type | Example | Meaning |
|---|---|---|---|
| `username` | string | `admin` | BMC user name. `admin` is the stock account, not personal data |
| `password` | string | `<REDACTED_PASSWORD>` | Cleartext, URL-encoded. Original was 16 chars `[A-Za-z0-9]` |
| `certlogin` | `"0"`/`"1"` | `0` | `1` = client-certificate login, which the UI then POSTs to **`/api/asrr/session`** instead **[JS]** |

No cookie or CSRF header is sent on login. Latency: **1491 ms** server wait (expensive, so reuse sessions).

### 4.2 Login response
HTTP 200, `Content-Type: application/json`, no `Set-Cookie` in the HAR (stripped by export).
```json
{ "ok": 0, "privilege": 4, "extendedpriv": 259, "racsession_id": "<REDACTED_SESSION_ID>",
  "remote_addr": "10.0.0.252", "server_name": "10.0.0.87", "server_addr": "10.0.0.87",
  "HTTPSEnabled": 1, "CSRFToken": "<REDACTED_CSRF>" }
```
| Field | Type | Example | Meaning |
|---|---|---|---|
| `ok` | int | `0` | 0 = success. The UI treats any non-zero as "invalid username or password" **[JS]** |
| `privilege` | int | `4` | IPMI channel privilege: 1 Callback, 2 User, 3 Operator, 4 Administrator, 5 OEM **[JS `window.CONSTANTS`]** |
| `extendedpriv` | int (bitmask) | `259` (0x103) | bit0 = KVM allowed, bit1 = Virtual Media allowed **[JS]**. bit8 (0x100) is set but not decoded by the UI |
| `racsession_id` | int | redacted (a 1-digit integer in the original) | BMC session slot id. Useful for `/api/settings/service-sessions` management |
| `remote_addr` | string | `10.0.0.252` | Client IP as the BMC sees it |
| `server_name` | string | `10.0.0.87` | Host header value |
| `server_addr` | string | `10.0.0.87` | BMC IP that served the request |
| `HTTPSEnabled` | int | `1` | 1 = HTTPS in use |
| `CSRFToken` | string | `<REDACTED_CSRF>` | **8 chars, `[A-Za-z0-9]`**. Must be sent as `X-CSRFTOKEN` on every later request. It stayed constant for the whole session (121 requests) |

Login failure **[JS]**: a non-2xx response with JSON body `{"code": ...}`. `code == 15000` (`MAX_NUM_SESSIONS_ALREADY_IN_USE`) means the session table is full, and anything else means bad credentials. **The proxy should log out cleanly and reuse one long-lived session per BMC, so it doesn't exhaust session slots.**

### 4.3 Session cookie
- Name: **`QSESSIONID`**. It is not in the HAR (export stripped cookies). The UI JS clears it on logout and on 401 with `document.cookie="QSESSIONID=1;expires=Thu, 01 Jan 1970 00:00:00 GMT"` (23 occurrences) **[JS]**.
- The UI JS also sets its own non-HTTP cookie `refresh_disable=1` after login, and uses its presence to decide whether a 401 means "session expired" **[JS]**. Server-side code ignores it, so the Go client does not need it.
- Attributes (`Secure`, `HttpOnly`, `Path`, `Max-Age`) and the value format are **unknown from this capture**. The UI checks `if(document.cookie)` to decide whether you are logged in, and that works even if `QSESSIONID` is HttpOnly because `refresh_disable` is visible. So HttpOnly cannot be ruled out. **Recapture with cookies included** (section 13).
- Go: use a `net/http/cookiejar` per BMC.

### 4.4 Authenticated requests
Every later API call carried `X-CSRFTOKEN: <REDACTED_CSRF>` (121/121 calls) and, implicitly, the cookie. The UI stores the token in `sessionStorage/localStorage["garc"]` and installs it with `$.ajaxSetup({headers:{"X-CSRFTOKEN":...}})` **[JS]**.

### 4.5 Logout (not in capture, from JS)
```
DELETE /api/session        (synchronous XHR, with X-CSRFTOKEN + cookie)
```
Then the UI expires the `QSESSIONID` and `refresh_disable` cookies on the client side.

### 4.6 Session expiry / errors **[JS]**
UI constants: `HTTP.UNAUTHORIZED=401` (session expired: clear state, show `session_expired`, go to login), `NO_PRIVILEGE_ACCESS=403`, `NOT_FOUND=404`, `FEATURE_NOT_ENABLED=405`, `SERVER_ERROR=500`, plus the custom `LARGE_FILE_SIZE=12012`. The capture contains no 401, 403 or 404. **The proxy should re-login on 401 and retry once.** The idle session timeout value was not observed. `/api/settings/services` (not captured) usually exposes the web session timeout.

### 4.7 Other auth-adjacent endpoints (in JS, not captured)
`POST /api/reset-pass` (form `username`; SMTP password reset), `POST /api/asrr/session` (cert login), `/api/settings/service-sessions[?service_id=]` (list/kill sessions).

---

## 5. Endpoint catalog

Conventions: **n** = number of calls in the HAR. Timings are server `wait` (TTFB) in ms as min/median/max, from HAR `timings.wait`. "Total" includes browser queueing. All requests had no query string, all responses are `application/json`, and the headers are as in section 2.2 unless stated otherwise. Types: `int`, `float`, `string`, `bool-int` (0/1 int).

### 5.1 `POST /api/session`: login
See section 4. n=1, wait 1491 ms.

### 5.2 `GET /api/settings/users`: user table
n=1 (right after login; the UI uses it to map the user's KVM/VMedia rights). **wait 1678 ms (slow).** The response is an array of **10 fixed slots** (ids 1–10). Slots 4–10 were empty in the capture.

| Field | Type | Example | Meaning |
|---|---|---|---|
| `id` | int | `2` | User slot / IPMI user id (1-based) |
| `name` | string | `admin` | User name; `""` = empty slot. Slot 1 = `anonymous` (IPMI null user), slot 2 = `admin`, slot 3 = `fwupd` (disabled; created 2026-09-03) |
| `access` | bool-int | `1` | 1 = user enabled |
| `kvm` | bool-int | `1` | KVM permission |
| `vmedia` | bool-int | `1` | Virtual media permission |
| `snmp` | bool-int | `0` | SNMP v3 user enabled |
| `prev_snmp` | bool-int | `0` | Previous SNMP state (UI bookkeeping) |
| `network_privilege` | string | `administrator` | Channel privilege name (`administrator`, `operator`, `user`, ...; `""` for empty slot) |
| `fixed_user_count` | int | `2` | Number of reserved, non-deletable slots |
| `snmp_access` | string | `""` | SNMP access (`read_only`/`read_write` when set) |
| `OEMProprietary_level_Privilege` | int | `1` | AMI OEM privilege flag |
| `privilege_limit_serial` | string | `none` | Serial-channel privilege limit |
| `snmp_authentication_protocol` | string | `""` | e.g. `sha`/`md5` when SNMP enabled |
| `snmp_privacy_protocol` | string | `""` | e.g. `des`/`aes` when SNMP enabled |
| `email_id` | string | `""` | Alert e-mail address |
| `email_format` | string | `ami_format` | Alert e-mail format |
| `ssh_key` | string | `Not Available` | SSH public key status |
| `creation_time` | int (epoch s) | `1649421370` | Creation time; `0` for empty. `anonymous` showed "now", which looks synthetic |

Redfish mapping: `AccountService/Accounts`. Writes go through `PUT /api/settings/users/{id}` (path from JS, body not captured).

### 5.3 `GET /api/firmware-info`: IPMI Get Device ID
n=1, wait 79 ms.

| Field | Type | Example | Meaning |
|---|---|---|---|
| `device_id` | int | `32` | IPMI Device ID |
| `revision` | int | `1` | Device revision |
| `fw_rev_1` | int | `2` | Major firmware revision |
| `fw_rev_2` | int | `50` | Minor firmware revision (0x32 in BCD = "32", hence 2.32) |
| `ipmi_version` | int | `2` | IPMI version (2 = 2.0) |
| `dev_support` | int | `191` | Additional device support bitmask (0xBF) |
| `mfg_id_0/1/2` | int | `214`,`193`,`0` | IANA manufacturer id bytes, LSB first: 0x00C1D6 = 49622 (ASRock) |
| `prod_id` | int | `514` | Product id |
| `aux` | int | `0` | Aux firmware revision |
| `fw_ver` | string | `2.32.00` | Firmware version string |
| `date` | string | `May 11 2021` | Build date |
| `time` | string | `17:53:41 CST` | Build time |
| `active_image` | int | `0` | Active flash image (dual-image) |

Redfish: `Managers/{id}.FirmwareVersion`, `UpdateService/FirmwareInventory/BMC`. Static, so cache indefinitely (refresh after a firmware update).

### 5.4 `GET /api/configuration/project`: compiled-in feature list
n=1, wait 123 ms. The response is an array of `{ "feature": string }` objects (54 entries, one duplicate). Values: `NWLINK, SYSTEM_FIREWALL, IPV6_SUPPORT, EXTENDED_PRIV, NWBONDING, SERVICES, SESSION_MANAGEMENT, FIRMWARE_FLASH_SUPPORT, VERSION_CMP_FLASH, FW_IMAGE, BACKUP_CONFIG, PRESERVECONF, NTP_SERVER_SUPPORT, SINGLE_PORT_APP, RUNTIME_SINGLEPORT_SUPPORT, SNMP, SAVE_SELLOG, WEB_APP_PROXY, TSIG, MDNS, TIMEZONE_SUPPORT, CIRCULAR_SEL, DISABLE_EMPTY_PASSWORD, OEM_PROPRIETARY_LEVEL_SUPPORT, IPV6_COMPLIANCE, INVENTORY_SUPPORT, YES, AUTOVDORECORD, AUTOVDORECORD_REMOTE, ADVISER_SUPPORT, KB_LANG_SELECT_SUPPORT, RUNTIME_HOST_LOCK, HOST_LOCK_AUTO, KVM_SESSION_TIMEOUT, KVM_SESSION_RECONNECT, HTML5_KVM, AUTO_RESIZE_KVM_CLIENT_WINDOW, JAVA_CD_IMAGE_REDIR, CAPTURE_BSOD_RAW (x2), MULTIPLE_USER_VMEDIA, CD_SERVER_APP, FD_SERVER_APP, HD_SERVER_APP, IMG_REDIRECTION, RMEDIA, NCSI_SUPPORT, PAM_REORDERING, AD_SUPPORT, LDAP_SUPPORT, RADIUS_SUPPORT, REDFISH, REDFISH_HOSTINTERFACE, BONDING_SUPPORT`.
Notable: **`REDFISH`, `REDFISH_HOSTINTERFACE`** (section 12), `CIRCULAR_SEL` (the SEL wraps, see 5.17), and `INVENTORY_SUPPORT` (host_inventory). Static, so cache per boot.

### 5.5 `GET /api/configuration/runtime`: runtime feature toggles
**n=39**, wait 212/335/1895 ms. The UI fetches it **on every route change** (each call is paired with a `favicon.ico` request). It is not timer driven. Intervals: 0.13–73.2 s, median ≈1.1 s. The body was identical all 39 times.
```json
[ { "feature": "LDAP", "enabled": 0 }, { "feature": "AD", "enabled": 0 },
  { "feature": "RADIUS", "enabled": 0 }, { "feature": "SINGLE_PORT_APP", "enabled": 1 },
  { "feature": "SD_MEDIA_SUPPORT", "enabled": 0 } ]
```
| Field | Type | Meaning |
|---|---|---|
| `feature` | string | Feature name |
| `enabled` | bool-int | Runtime state |

The proxy does not need this except to know whether LDAP/AD/RADIUS are active. Cache for ≥10 min.

### 5.6 `GET /api/chassis`: power + UID LED status (primary poll)
**n=31**, wait 68/84/1724 ms. The UI polls it on a **5 s timer** (`setInterval 5000`, request timeout 6 s) **[JS]**. Observed intervals: 4.99–5.01 s in steady state (24 intervals). Outliers: one 16.73 s gap while on the firmware-maintenance pages (the UI stops polling there), plus ad-hoc fetches 4.09 s and 0.90 s apart (the Power Supply page also fetches `/api/chassis`).
```json
{ "power_status": 0, "led_status": 0 }
```
| Field | Type | Example | Meaning |
|---|---|---|---|
| `power_status` | int | `0` → `1` | 0 = host off, 1 = host on (the UI shows "online"/"offline") |
| `led_status` | int | `0` | UID/identify LED: 0 = off, 1 = blinking, other (2) = on **[JS]** |

The body changed from `power_status:0` (7 samples) to `power_status:1` (24 samples) between 06:00:39.5 and 06:00:43.2, after the Power On at 06:00:41.45. Redfish: `Systems/{id}.PowerState`, `Chassis/{id}.IndicatorLED`/`LocationIndicatorActive`. **Cheapest liveness/power probe: poll at 5–10 s in the proxy.**

### 5.7 `GET /api/chassis-status`
n=5, wait 74/107/111 ms. **Same body shape as `/api/chassis`** (`power_status`, `led_status`). The Power Control / UID pages use it (Backbone model `models/chassis_status`) **[JS]**. Calls: page visits at 06:00:01, 06:00:06, 06:00:18 and 06:00:39, then **06:00:51.55, exactly 10.1 s after the power-on POST**. That matches the JS post-action re-check (`inittmout` = 10 s for on/off/reset, 20 s for cycle, 30 s for ACPI shutdown).

### 5.8 `GET /api/settings/network`: LAN interfaces
n=2 (146.6 s apart), **wait 952 / 1643 ms (slow)**. The response is an array with one entry per interface (2 here).

| Field | Type | Example | Meaning |
|---|---|---|---|
| `id` | int | `1` | Interface index (1-based) |
| `interface_name` | string | `eth0` | `eth0` (dedicated, channel 1) / `eth1` (channel 8, likely the NC-SI shared port) |
| `channel_number` | int | `1`, `8` | IPMI LAN channel |
| `lan_enable` | bool-int | `1` | Interface enabled |
| `mac_address` | string | `<REDACTED_MAC>` | `XX:XX:XX:XX:XX:XX` uppercase (ASRock OUI) |
| `ipv4_enable` | bool-int | `1` | |
| `ipv4_dhcp_enable` | bool-int | `0` | 0 = static |
| `ipv4_address` | string | `10.0.0.87` | `0.0.0.0` when none |
| `ipv4_subnet` | string | `255.255.255.0` | Dotted mask |
| `ipv4_gateway` | string | `10.0.0.1` | |
| `ipv6_enable` | bool-int | `1` | |
| `ipv6_dhcp_enable` | bool-int | `1` | |
| `ipv6_address` | string | `::` | |
| `ipv6_index` | int | `0` | Index into the static IPv6 table |
| `ipv6_prefix` | int | `0` | Prefix length |
| `ipv6_gateway` | string | `::` | |
| `vlan_enable` | bool-int | `0` | |
| `vlan_id` | int | `0` | |
| `vlan_priority` | int | `0` | |

Redfish: `Managers/{id}/EthernetInterfaces`. Cache ≥5 min.

### 5.9 `GET /api/settings/static-ipv6`
n=1, wait 869 ms. The response is an array of 32 entries (16 per interface), all empty in the capture.

| Field | Type | Example | Meaning |
|---|---|---|---|
| `ipv6_index` | int | `0`..`15` | Static slot index |
| `interface_name` | string | `eth0` | |
| `ipv6_address` | string | `""` | Static address, or empty |
| `ipv6_prefix` | int | `0` | Prefix length |

### 5.10 `GET /api/settings/network-bond`
n=1, wait 197 ms.
`{ "id": 1, "bond_enable": 0, "bond_mode": "active-backup", "auto_configuration_enable": 1 }`
Fields: `id` int, `bond_enable` bool-int, `bond_mode` string (`active-backup`, ...), `auto_configuration_enable` bool-int.

### 5.11 `GET /api/settings/date-time`
n=1, wait 114 ms.
| Field | Type | Example | Meaning |
|---|---|---|---|
| `id` | int | `1` | |
| `primary_ntp` | string | `pool.ntp.org` | |
| `secondary_ntp` | string | `time.nist.gov` | |
| `auto_date` | bool-int | `1` | 1 = NTP sync |
| `localized_timestamp` | int (epoch s) | `1794808815` | BMC local time |
| `utc_minutes` | int | `0` | UTC offset in minutes |
| `daylight` | bool-int | `0` | DST |
| `timestamp` | int (epoch s) | `1794808815` | BMC UTC time (2026-11-16 06:00:15Z; client clock was ~5 s ahead) |
| `timezone` | string | `GMT+0` | |

Redfish: `Managers/{id}.DateTime`, `DateTimeLocalOffset`, `NetworkProtocol.NTP`.

### 5.12 `GET /api/settings/media/adviser`: KVM (H5Viewer) settings
n=1, wait 161 ms.
| Field | Type | Example | Meaning |
|---|---|---|---|
| `license` | bool-int | `1` | KVM licensed |
| `status` | bool-int | `1` | KVM service enabled |
| `secure_channel` | bool-int | `0` | KVM encryption |
| `kvm_port` | int | `7578` | Non-single-port KVM port |
| `mouse_mode` | int | `2` | AMI mouse mode (1 relative, 2 absolute, 3 other, as usually defined) |
| `web_port` | int | `443` | |
| `singleport_status` | bool-int | `1` | KVM tunnelled through 443 |
| `retry_count` | int | `3` | |
| `retry_interval` | int | `10` | Seconds |
| `keyboard_layout` | string | `AD` | `AD` = auto-detect |

### 5.13 `GET /api/settings/video/remote-storage`
n=1, wait 239 ms. Settings for storing crash/auto video recordings remotely.
`id` int 1; `remote_support` bool-int 0; `mount_status` int -1 (not mounted); `max_dumps` int 2; `max_duration` int 20 (s); `max_size` int 5 (MB); `ip_address` string; `remote_path` string; `share_type` string `NFS`; `user_name` string; `password` string (empty in capture; **the BMC returns stored share passwords in plaintext, so redact them in any proxy logs**); `domain_name` string.

### 5.14 `GET /api/logs/video`
n=1, wait 119 ms. Body `[ ]` (no recordings). The element shape is not observed. JS downloads files via `GET /api/logs/video-data/?file=<name>` (binary).

### 5.15 `GET /api/sdr`: SDR index
n=1, wait 155 ms. The response is an array of **42** entries, one more than `/api/sensors`: the extra entry is `PEF` (sensor 232, type `system_event`/18, which has no reading).
| Field | Type | Example | Meaning |
|---|---|---|---|
| `id` | int | `1` | Record index (1-based) |
| `sensor_number` | int | `1` | IPMI sensor number. **This is the stable key that joins to sensors and SEL** |
| `name` | string | `3VSB` | Sensor ID string |
| `type` | string | `voltage` | Sensor type name (section 8.1) |
| `type_number` | int | `2` | IPMI sensor type code |

The SEL page uses it to map SEL `sensor_number` to names. Static, so cache per BMC boot.

### 5.16 `GET /api/sensors`: sensor readings
n=5, wait 105/229/389 ms, body ≈22 KB. Fetched on navigation to the dashboard, Power Supply, Sensors and Fan pages. On the dashboard and sensor pages, the JS also refreshes it every **10 s** (`10sec` poll, 6 s timeout). Single-sensor history pages use `/api/detail_sensors_readings` instead (not captured). Observed intervals 31.3, 2.3, 3.1 and 97.8 s. The 4 host-off samples were byte-identical; the host-on sample (idx 150) differs. **Full field documentation is in section 8.**

### 5.17 `GET /api/logs/eventlog`: full SEL dump
n=1, **wait 2383 ms + receive 602 ms = 2986 ms; body 474 940 bytes** for 489 records. No paging and no query parameters. Records are returned **newest first** (id 489 → 1). Full field documentation is in section 9. **The proxy should never fetch this on a northbound request path.** Refresh it in the background only when `/api/asrr/selsummary.selentries` (5.21) or the top id changes.

### 5.18 `GET /api/fru`: FRU inventory
n=2 (97.7 s apart), wait 365–372 ms. The response is an array with one element per FRU device (1 here: id 0 `BMC_FRU`).
| Path | Type | Example | Meaning |
|---|---|---|---|
| `device.id` | int | `0` | FRU device id |
| `device.name` | string | `BMC_FRU` | |
| `common_header.version` | int | `1` | FRU spec format version |
| `common_header.internal_use_area_start_offset` | int | `0` | Offsets in 8-byte multiples; 0 = area absent |
| `common_header.chassis_info_area_start_offset` | int | `0` | Chassis area absent |
| `common_header.board_info_area_start_offset` | int | `1` | Board area present |
| `common_header.product_info_area_start_offset` | int | `0` | Product area absent |
| `common_header.multi_record_area_start_offset` | int | `0` | |
| `chassis.version/length` | int | `0` | |
| `chassis.type` | string | `""` | Chassis type |
| `chassis.part_number/serial_number/custom_fields` | string | `""` | |
| `board.version` | int | `1` | |
| `board.length` | int | `9` | Area length (x8 bytes) |
| `board.language` | int | `0` | 0 = English |
| `board.date` | string | `"Thu Jan 13 06:21:00 2022\\n"` | Manufacturing date. **ctime()-style, with a literal backslash-n escape inside the string**, so trim it |
| `board.manufacturer` | string | `ASRockRack` | |
| `board.product_name` | string | `X470D4U` | |
| `board.serial_number` | string | `<REDACTED_SERIAL>` | Original: 15 decimal digits |
| `board.part_number` | string | `""` | |
| `board.fru_file_id` | string | `""` | |
| `board.custom_fields` | string | `""` | |
| `product.*` | | | `version,length,language` int; `manufacturer, product_name, part_number, product_version, serial_number, asset_tag, fru_file_id, custom_fields` string (all empty here) |

Redfish: `Chassis/{id}` (Manufacturer, Model, SerialNumber, PartNumber), `Systems/{id}`. With no chassis/product area, the system serial must come from the board serial or SMBIOS (host_inventory is empty, see 5.19).

### 5.19 `GET /api/host_inventory/*`: host inventory via Redfish Host Interface
All were called with the host off and then on, and every one was **empty**. The BIOS presumably never pushed inventory (feature `INVENTORY_SUPPORT` and `REDFISH_HOSTINTERFACE` are present). Bodies were identical before and after power-on.

| Path | n | wait ms | Body |
|---|---|---|---|
| `/api/host_inventory/host_interface_processor_info` | 5 | 81/91/203 | `[ ]` |
| `/api/host_inventory/host_interface_memory_info` | 4 | 76/77/103 | `[ ]` |
| `/api/host_inventory/host_interface_pcie_device_function_info` | 3 | 114/121/136 | `[ { "PCIE_Devices": [ ], "PCIE_Function": [ ] } ]` |
| `/api/host_inventory/host_interface_storage_info` | 1 | 78 | `[ { "Storage_Drives": null, "Storage_Contoller": null } ]` (note the misspelling **`Storage_Contoller`**, and `null`, not `[]`) |

Not called but present in JS: `host_interface_baseboard_info`, `host_interface_power_info`, `host_interface_system_info`, `host_interface_thermal_info`. **Element schemas are unknown.** Recapture after a full POST, possibly with the BIOS "Redfish host interface / inventory" option enabled. Until then the proxy cannot populate `Processors`/`Memory` from this BMC except via IPMI or SMBIOS-type OEM commands.

### 5.20 `GET /api/maintenance/fwimage_location`, `GET /api/maintenance/signed-image-key`
- `fwimage_location`: n=3, wait 93/99/125. `{ "id": 1, "protocol_type": "http", "server_address": "", "image_name": "", "retry_count": 0 }`. These are the remote firmware-image fetch settings: `protocol_type` string (`http`/`tftp`/...), `server_address` string, `image_name` string, `retry_count` int.
- `signed-image-key`: n=2 (fired twice in the same ms), wait 82/174/266. `{ "id": 1, "key_exists": 0, "public_key_info": "" }`. `key_exists` bool-int is whether a firmware-signing public key is installed; `public_key_info` string.
The UI calls nothing else while on the update page (no upload was attempted).

### 5.21 OEM `api/asrr/*` endpoints seen

| Endpoint | n | Status | wait ms | Body |
|---|---|---|---|---|
| `GET /api/asrr/model-name` | 1 | 200 | 1260 (inside the dashboard burst; probably ~100 ms alone) | `{ "mb_name": "X470D4U" }` |
| `GET /api/asrr/product-name` | 1 | **500** | 363 | `{ "error": "Error in Getting Model Name", "code": 16005 }`, with `Connection: close`. This is the only error in the capture. Expect it on boards without a product FRU area, and treat it as "unset", not a fault |
| `GET /api/asrr/fw-info` | 1 | 200 | 376 | `{ "BMC_fw_version": "2.32.00", "BIOS_fw_version": "P4.20", "ME_fw_version": "0.14.0.27", "Micro_Code_version": "0a201009", "CPLD_version": "N\/A" }` (all strings; `N/A` when absent). **The best single source for the Redfish FirmwareInventory (BMC, BIOS, ME, microcode)** |
| `GET /api/asrr/selsummary` | 1 | 200 | 311 | `{ "selentries": 489, "freeentries": 3150 }`. `selentries` int = records in SEL; `freeentries` int = remaining capacity (total 3639). **Use as the cheap change detector for the event log** |
| `GET /api/asrr/getmcalog` | 1 | 200 | 81 | `{ "MCADump": 0 }`. Whether an AMD MCA (machine-check) dump is available (0 = none). Related (JS only): `GET /api/asrr/mca-file`, `/api/asrr/clearmcalog` |
| `GET /api/asrr/PowerSource` | 1 | 200 | 458 | Array of **4** PSU slot objects (see below) |
| `GET /api/asrr/GetFanDuty` | 1 | 200 | 83 | `{ "FAN1_Duty": "55", ..., "FAN8_Duty": "55" }`. **The values are strings** holding the PWM duty %. FAN1..FAN8 are keyed but only 6 fan tachs exist in `/api/sensors` (FAN1–FAN6 with numbering gaps). The UI (buggily) shows `FAN1_Duty` for every fan **[JS]** |

`/api/asrr/PowerSource` element (PMBus PSU telemetry; this board has no PMBus PSU, so all values are `N/A`):

| Field | Type | Example | Meaning |
|---|---|---|---|
| `power_supply_status` | int | `2` | 1 = OK, otherwise error **[JS]**. 2 appears to mean "not present": the UI hides slots 3/4 when both report 2 |
| `ac_input_voltage` / `_fault` | string / int | `"N/A"` / `0` | AC in volts; fault flag |
| `ac_input_current` / `_fault` | string / int | `"N/A"` / `0` | |
| `dc_12v_output_voltage` / `_fault` | string / int | | |
| `dc_12v_output_current` / `_fault` | string / int | | |
| `temperature1`, `temperature2` / `_fault` | string / int | | PSU temperatures |
| `fan1`, `fan2` / `_fault` | string / int | | PSU fan speeds |
| `dc_12v_output_power` / `_fault` | string / int | | |
| `ac_input_power` / `_fault` | string / int | | |
| `pws_serial_number` | string | `"N/A"` | PSU serial |

**Numeric telemetry is encoded as strings** (or `"N/A"`), so parse with `strconv.ParseFloat` and fall back to null.

Other `asrr` endpoints referenced by the UI but **not captured**: `bmccoldreset`, `getbioscode` (POST code), `getprochot`/`setprochot`, `getvetobit`/`setvetobit`, `getWebEnableTimeout`/`setWebEnableTimeout`, `java-console`, `maintenance/BIOS/{PublicKey,configuration,firmware,flash-progress,status,upgrade}` (BIOS flashing from the BMC), and the fan-control family `settings/{getfancontrolmode,setfancontrolmode,getfandutymanualmode,setfandutymanualmode,getfanopenloopcontroltable,...,getsupportfan,gettscftsupportfan,setclearallfansetting}`.

### 5.22 `POST /api/actions/power`
See section 7.

---

## 6. Endpoint call counts at a glance

| Method | Path | n | Status | wait min/med/max (ms) | Trigger / cadence |
|---|---|---|---|---|---|
| GET | /api/configuration/runtime | 39 | 200 | 212/335/1895 | Every route change |
| GET | /api/chassis | 31 | 200 | 68/84/1724 | **5 s timer** (6 s timeout) |
| GET | /api/chassis-status | 5 | 200 | 74/107/111 | Power page + 10 s after power action |
| GET | /api/sensors | 5 | 200 | 105/229/389 | Page loads; 10 s timer on dashboard/sensors |
| GET | /api/host_inventory/host_interface_processor_info | 5 | 200 | 81/91/203 | Page loads |
| GET | /api/host_inventory/host_interface_memory_info | 4 | 200 | 76/77/103 | Page loads |
| GET | /api/host_inventory/host_interface_pcie_device_function_info | 3 | 200 | 114/121/136 | Page loads |
| GET | /api/maintenance/fwimage_location | 3 | 200 | 93/99/125 | Page loads |
| GET | /api/settings/network | 2 | 200 | 952/1297/1643 | Dashboard + network page |
| GET | /api/maintenance/signed-image-key | 2 | 200 | 82/174/266 | Page load (duplicate) |
| GET | /api/fru | 2 | 200 | 365/369/372 | Page loads |
| POST | /api/session | 1 | 200 | 1491 | Login |
| GET | /api/settings/users | 1 | 200 | 1678 | After login |
| GET | /api/firmware-info | 1 | 200 | 79 | Dashboard |
| GET | /api/configuration/project | 1 | 200 | 123 | Dashboard |
| GET | /api/asrr/model-name | 1 | 200 | 1260 | Dashboard |
| GET | /api/asrr/product-name | 1 | **500** | 363 | Dashboard |
| GET | /api/asrr/fw-info | 1 | 200 | 376 | Dashboard |
| GET | /api/asrr/selsummary | 1 | 200 | 311 | Dashboard |
| GET | /api/asrr/getmcalog | 1 | 200 | 81 | Event log page |
| GET | /api/settings/date-time | 1 | 200 | 114 | Event log page |
| GET | /api/sdr | 1 | 200 | 155 | Event log page |
| GET | /api/logs/eventlog | 1 | 200 | **2383** (+602 receive) | Event log page |
| GET | /api/logs/video | 1 | 200 | 119 | Video log page |
| GET | /api/settings/video/remote-storage | 1 | 200 | 239 | Video log page |
| GET | /api/asrr/PowerSource | 1 | 200 | 458 | PSU page |
| POST | /api/actions/power | 1 | 200 | 87 | User action |
| GET | /api/settings/media/adviser | 1 | 200 | 161 | KVM settings page |
| GET | /api/host_inventory/host_interface_storage_info | 1 | 200 | 78 | Page load |
| GET | /api/asrr/GetFanDuty | 1 | 200 | 83 | Fan page |
| GET | /api/settings/network-bond | 1 | 200 | 197 | Network page |
| GET | /api/settings/static-ipv6 | 1 | 200 | 869 | Network page |

---

## 7. Power control: `POST /api/actions/power`

### Request (HAR entry 105)
```
POST /api/actions/power HTTP/1.1
Content-Type: application/json
X-CSRFTOKEN: <REDACTED_CSRF>
X-Requested-With: XMLHttpRequest
Origin: https://<BMC_HOST>
Content-Length: 19

{"power_command":1}
```
### Response
HTTP 200, wait 87 ms, body `{ "power_command": 1 }` (echo). The POST returns as soon as the command is queued; it does **not** wait for the state change.

### Command codes (from the Power Control page radio buttons **[JS]**)

| `power_command` | UI label | Expected final `power_status` (JS) | JS re-check delay | Redfish `ResetType` mapping |
|---|---|---|---|---|
| `0` | Power Off (immediate) | 0 | 10 s | `ForceOff` |
| `1` | Power On | 1 | 10 s | `On` (also `ForceOn`) |
| `2` | Power Cycle | 1 | 20 s | `PowerCycle` |
| `3` | Hard Reset | 1 | 10 s | `ForceRestart` |
| `5` | ACPI Shutdown (soft) | 0 | 30 s (20 s retries) | `GracefulShutdown` |

These match IPMI Chassis Control (00 down, 01 up, 02 cycle, 03 hard reset, 05 soft shutdown). Code `4` (diagnostic interrupt / NMI in IPMI) is not offered by the UI. Whether the API accepts it is untested, and it would map to `Nmi`. `GracefulRestart` has no single command: the proxy would have to issue 5, poll until off, then send 1.

UI enable rules **[JS]**: when the host is off only `1` is enabled. When it is on, `1` is disabled and 0/2/3/5 are enabled. Users below Administrator get a disabled Save button. The UI re-polls `chassis-status` after the delay above, with `max_retries=3`.

Observed timing: POST at 06:00:41.450Z, then `/api/chassis` reported `power_status: 1` at 06:00:43.162Z (the next 5 s poll), so it took ≤1.7 s.

UID LED (JS only, not captured): `POST /api/actions/chassis-led` with `{"uid_command": N}`, where N is 0 = off, 1 = blink, 2 = on. This maps to Redfish `IndicatorLED` Off/Blinking/Lit.

---

## 8. Sensors (`/api/sensors`) in detail

### 8.1 Record schema (41 records, all fields always present)

| Field | Type | Example | Meaning |
|---|---|---|---|
| `id` | int | `1` | List index (1-based; not stable across firmware) |
| `sensor_number` | int | `1` | IPMI sensor number (**stable key**) |
| `name` | string | `3VSB` | Sensor name (the UI strips non-printable chars) |
| `owner_id` | int | `32` | Sensor owner = 0x20 (BMC IPMB address) |
| `owner_lun` | int | `0` | |
| `raw_reading` | float | `170.0` | Raw 8-bit reading byte |
| `type` | string | `voltage` | Type name |
| `type_number` | int | `2` | IPMI sensor type code |
| `reading` | float | `3.4` | Converted reading in `unit`. **For discrete sensors this is a state bitmask** (8.4) |
| `sensor_state` | int (bitmask) | `1` | Threshold state bits (8.3); `0` for discrete sensors |
| `discrete_state` | int | `0` | IPMI Event/Reading Type code: `0` = threshold sensor; 2–12 generic discrete; `111` (0x6F) sensor-specific; 112–127 OEM |
| `settable_readable_threshMask` | int (16-bit) | `13878` (0x3636) | Threshold mask (8.2) |
| `lower_non_recoverable_threshold` | float | `2.88` | LNR |
| `lower_critical_threshold` | float | `3.04` | LC |
| `lower_non_critical_threshold` | float | `0.0` | LNC |
| `higher_non_critical_threshold` | float | `0.0` | UNC |
| `higher_critical_threshold` | float | `3.7` | UC |
| `higher_non_recoverable_threshold` | float | `3.88` | UNR |
| `accessible` | int | `0` / `213` | `0` = reading valid; **`213` (0xD5, IPMI CC "not supported in present state") = no reading / sensor unavailable**. The UI lists these as "disabled sensors" |
| `unit` | string | `V` | `V`, `A`, `°C` (UTF-8 U+00B0 + `C`), `RPM`, `W`, `unknown` (discrete). JS also handles `deg_c` |

**An unset threshold is reported as `0.0`, not null.** Use the mask (8.2) to tell "unset" from a real 0 threshold.

### 8.2 `settable_readable_threshMask`
These are the two IPMI SDR mask bytes. The bit layout per byte is bit0 LNC, bit1 LC, bit2 LNR, bit3 UNC, bit4 UC, bit5 UNR. Both bytes were identical for every sensor, so the capture can't tell which byte is "settable" and which is "readable". Observed:

| Mask | Bits | Sensors | Matches non-zero thresholds |
|---|---|---|---|
| `0x3636` | LC, LNR, UC, UNR | 12 voltages | yes (e.g. 3VSB: LNR 2.88, LC 3.04, UC 3.7, UNR 3.88) |
| `0x3030` | UC, UNR | VCPU | yes (1.65/1.73) |
| `0x1818` | UNC, UC | CPU Temp, DDR4_A1/A2/B1/B2 | yes (93/94, 84/85) |
| `0x0808` | UNC | MB Temp (55), Card Side Temp (68) | yes |
| `0x0101` | LNC | FAN1–FAN6 | yes (100 RPM) |
| `0x0000` | none | PSU VIN/IOUT/PIN/POUT, discrete sensors | all thresholds 0 |

Redfish mapping: `Thresholds.LowerCaution` = LNC, `LowerCritical` = LC, `LowerFatal` = LNR, `UpperCaution` = UNC, `UpperCritical` = UC, `UpperFatal` = UNR. Include a threshold only if its mask bit is set.

### 8.3 `sensor_state` (threshold sensors) **[JS `threshold_states`]**
A bitmask. Each set bit adds a label: `1` Normal, `2` Upper Non-Critical, `4` Upper Critical, `8` Lower Non-Critical, `16` Lower Critical, `32` Access Failed, `64` Upper Non-Recoverable, `128` Lower Non-Recoverable. `0` = Uninitialized (or a discrete sensor). Only `1` (Normal) was observed on threshold sensors.
UI classification **[JS]**: *disabled* if `accessible==213`; *discrete* if `discrete_state!=0`; *normal* if `sensor_state==1`; *critical* otherwise. Redfish `Status.Health`: 1 = OK; 2/8 = Warning; 4/16/64/128 = Critical; `accessible==213` = `Status.State: "Absent"` or `"UnavailableOffline"` with a null `Reading`.

### 8.4 Discrete sensors
For `discrete_state != 0`, the UI decodes **bits 0–7 of `reading`** as asserted offsets **[JS]**:
- `discrete_state` 2–12 maps to the generic table `generic_discrete_<type>_<bit>`. Example: CPU_PROCHOT uses 3 (0x03, "digital discrete": bit0 State Deasserted, bit1 State Asserted).
- `discrete_state` 111 (0x6F) maps to the sensor-specific table `sensor_specific_<type_number>_<bit>`. Example: ChassisIntr type 5: bit0 General Chassis Intrusion, bit1 Drive Bay Intrusion, ...
- No bits set means "No event generated" (`sensor_specific_noevent`).
- Observed discrete readings were `0.0` while unavailable, or **`32768.0` (0x8000)** when readable. Bits 0–7 are clear, so no states were asserted. Bit 15 seems to be a "reading valid / present" marker from the AMI library and is ignored by the UI.

### 8.5 Sensor inventory (X470D4U)

| sensor_number | name | type (number) | unit | thresholds LNR/LC/LNC/UNC/UC/UNR | host-off accessible | host-on reading |
|---|---|---|---|---|---|---|
| 1 | 3VSB | voltage (2) | V | 2.88/3.04/–/–/3.70/3.88 | 0 | 3.40 |
| 2 | 5VSB | voltage (2) | V | 4.26/4.50/–/–/5.49/5.73 | 0 | 5.10 (4.95 off) |
| 3 | VCPU | voltage (2) | V | –/–/–/–/1.65/1.73 | 213 | 1.30 |
| 4 | VSOC | voltage (2) | V | 0.34/0.36/–/–/1.54/1.61 | 213 | 0.98 |
| 5 | VCCM | voltage (2) | V | 1.02/1.08/–/–/1.32/1.38 | 213 | 1.19 |
| 6 | APU_VDDP | voltage (2) | V | 0.77/0.81/–/–/1.16/1.21 | 213 | 0.95 |
| 7 | 1.05V_PROM_S5 | voltage (2) | V | 0.89/0.95/–/–/1.16/1.21 | 213 | 1.06 |
| 8 | 2.5V_PROM | voltage (2) | V | 2.12/2.26/–/–/2.74/2.88 | 213 | 2.56 |
| 9 | 1.05V_PROM_RUN | voltage (2) | V | 0.85/0.88/–/–/1.16/1.21 | 213 | 1.04 |
| 10 | BAT | voltage (2) | V | 2.00/2.70/–/–/3.40/3.56 | 0 | 3.14 |
| 11 | 3V | voltage (2) | V | 2.80/2.98/–/–/3.62/3.78 | 213 | 3.36 |
| 12 | 5V | voltage (2) | V | 4.26/4.50/–/–/5.49/5.73 | 213 | 5.07 |
| 13 | 12V | voltage (2) | V | 10.2/10.8/–/–/13.2/13.8 | 213 | 12.00 |
| 24 / 25 | PSU1 VIN / PSU2 VIN | voltage (2) | V | none | 213 | 213 (no PMBus PSU) |
| 32 / 33 | PSU1 IOUT / PSU2 IOUT | current (3) | A | none | 213 | 213 |
| 49 | MB Temp | temperature (1) | °C | UNC 55 | 213 | 29 |
| 50 | Card Side Temp | temperature (1) | °C | UNC 68 | 213 | 31 |
| 52 | CPU Temp | temperature (1) | °C | UNC 93, UC 94 | 213 | 52 |
| 64–67 | DDR4_A2/A1/B2/B1_Temp | temperature (1) | °C | UNC 84, UC 85 | 213 | 213 (no DIMM thermal data) |
| 96, 98–102 | FAN1–FAN6 | fan (4) | RPM | LNC 100 | 213 | FAN4 12700, FAN5 9600; others 213 (not populated) |
| 112/113 | PSU1/PSU2 PIN | power_supply (8) | W | none | 213 | 213 |
| 116/117 | PSU1/PSU2 POUT | power_supply (8) | W | none | 213 | 213 |
| 144 | ChassisIntr | physical_security (5) | unknown | discrete 0x6F | 0 | 0x8000 (no intrusion) |
| 145 | CPU_PROCHOT | processor (7) | unknown | discrete 0x03 | 213 | 0x8000 (deasserted) |
| 147 | CPU_THERMTRIP | processor (7) | unknown | discrete 0x6F | 213 | 0x8000 |
| 160 / 168 | PSU1 / PSU2 Status | power_supply (8) | unknown | discrete 0x6F | 0 | 0x8000 |
| 161 / 169 | PSU1 / PSU2 AC lost | power_supply (8) | unknown | discrete 0x6F | 213 | 213 |
| 232 | PEF (SDR only) | system_event (18) | n/a | not in /api/sensors | n/a | n/a |

Conversion: `reading = f(raw_reading)` via the SDR M/B/exponent (e.g. 3VSB 170 x 0.02 = 3.40; 12V 120 x 0.1; fans raw x 100 RPM; temps 1:1). **Fan resolution is therefore 100 RPM.**

Redfish mapping: `Chassis/{id}/Sensors` (or legacy `Thermal`/`Power`), keyed on `sensor_number`. Temps → `ReadingType: Temperature`/`Cel`; fans → `Fan`/`RPM`; voltages → `Voltage`/`V`; current → `A`; power → `W`. Discrete sensors go to `Status`/`Oem`, or to `Chassis.PhysicalSecurity.IntrusionSensor` for ChassisIntr.

---

## 9. Event log (`/api/logs/eventlog`) in detail

Array of 489 records, newest first. Distribution: 416 `system_event_record` and 73 `oem_record`. **Some fields are optional:** `event_reading_class` appears on 416/489, and `pci_bus_num`/`pci_device_num`/`pci_function_num` on 52/489. Model these as pointers or `omitempty` in Go.

| Field | Type | Example | Meaning |
|---|---|---|---|
| `id` | int | `489` | SEL record id (1..N; with `CIRCULAR_SEL` expect wrap-around) |
| `record_type` | string | `system_event_record` / `oem_record` | |
| `record_type_num` | int | `2` / `193` | IPMI record type (0x02 system event; 0xC0–0xDF OEM timestamped, here 0xC1) |
| `timestamp` | int (epoch s) | `1794446550` | Event time. **Small values (13, 71, 130, 143) are pre-time-sync "seconds since BMC init"** (IPMI ≤0x20000000 rule), so render them as unknown/relative |
| `GenID1` | int | `32` | Generator id: 0x20 = BMC; `1` = system software id 0x01 (BIOS); `0` on OEM records |
| `GenID2` | int | `0` | Channel/LUN byte |
| `event_format_ipmi_version` | int | `4` | EvMRev (0x04 = IPMI 2.0; 0 on OEM records) |
| `sensor_type` | string | `temperature` | Sensor type name. Seen: `temperature`, `processor`, `system_event`, `oem`, `version_change` |
| `sensor_type_num` | int | `1` | IPMI sensor type code (1, 7, 18, 192, 43) |
| `sensor_name` | string | `CPU Temp` | Resolved via SDR; `Unknown` if no SDR match, `BIOS` for BIOS-generated |
| `sensor_number` | int | `52` | |
| `generator_type` | string | `ipmb_slave_address` / `system_software_id` | Decoded GenID1 bit0 |
| `system_software_type` | string | `system_management_software` / `bios` | |
| `channel_number` | int | `0` | |
| `ipmb_lun` | int | `0` | |
| `event_direction` | string | `asserted` / `deasserted` | |
| `EventDirType` | int | `129` | Raw event dir/type byte: bit7 = deassertion, bits0–6 = event/reading type (1 threshold, 3 digital, 0x6F sensor-specific; 211 = 0xD3 on OEM) |
| `event_reading_class` | string? | `threshold` / `discrete` | Absent on OEM records |
| `EventData1/2/3` | int | `87`, `92`, `93` | Raw event data bytes. For threshold events: ED2 = trigger reading (raw), ED3 = threshold (raw) |
| `sensor_reading_value` | int | `64` / `0` | Unclear; 64 on all CPU Temp events |
| `triggered_value` | int | `0` | Always 0 here |
| `event_description` | string | `upper_non_critical_going_high` | Machine key for ED1 offset |
| `advanced_event_description` | string | `unknown` | Always `unknown` here |
| `EventData1_Description` | string | `Upper Non-critical - going high` | Human text |
| `EventData2_Description` | string | `Reading that triggered event : 92 °C` | Human text with unit (`N/A` otherwise) |
| `EventData3_Description` | string | `Threshold value that triggered event : 93 °C` | |
| `EventData{1,2,3}_Description_raw` | string | `Reading that triggered event : 92 ` | Same text without unit |
| `pci_bus_num`, `pci_device_num`, `pci_function_num` | int? | `128`, `31`, `7` | Present only on some BIOS / system-event / version-change records |

Event types in this SEL: CPU Temp UNC going high (118) and UC going high (86), CPU_PROCHOT state asserted/deasserted (110), timestamp clock sync from BIOS/BMC (100), OEM timestamped (73), firmware change detected (2). Timestamps run 2022-04 → 2026-11-12.

Redfish mapping: `Managers/{id}/LogServices/SEL/Entries` (`EntryType: SEL`, `SensorNumber`, `SensorType`, `EntryCode` from direction, `Message` from descriptions, `Created` from timestamp). `/api/logs/event` (JS) is the clear/delete endpoint, and `/api/logs/event-file` / `raw-event-file` are downloads. None of these were captured.

---

## 10. Timing, polling cadence and caching guidance

### 10.1 Observed polling by the stock UI
- `api/chassis`: **5.00 s ± 0.01 s** timer, 31 calls in ~157 s of active session (paused on the maintenance pages).
- `api/configuration/runtime`: 39 calls, driven by navigation (not a timer).
- `api/sensors`: 10 s timer only while on the dashboard or sensor pages. Here, 5 navigation-driven calls.
- Timer constants in JS: 2 s, 5 s, 10 s and 15 s buckets (the 2 s and 15 s ones are unused on these pages).

### 10.2 Latency (server wait, ms, uncontended where possible)

| Class | Endpoints | Typical |
|---|---|---|
| Fast (<150) | chassis, chassis-status, firmware-info, sdr, getmcalog, GetFanDuty, host_inventory/*, date-time, maintenance/*, logs/video, power action | 70–150 |
| Medium (150–500) | sensors (~105–390), fru (~370), PowerSource (~460), configuration/runtime (~220–500), selsummary (~310), asrr/fw-info (~380), network-bond (~200), remote-storage (~240), media/adviser (~160) | 150–500 |
| Slow (>800) | session login (1491), settings/users (1678), settings/network (952–1643), static-ipv6 (869), logs/eventlog (2383 + 602 transfer) | 0.9–3 s |

### 10.3 Contention evidence (the BMC serialises work)
- The dashboard burst at 06:00:01.43 sent 11 concurrent calls. `firmware-info` and `project` came back in ~80–120 ms, but `settings/network` took a 952 ms wait and `model-name` 1260 ms. Total completion took ~2 s.
- At 06:02:27.98, `static-ipv6`, `network` and `runtime` ran in parallel, and the following `api/chassis` poll had a **1724 ms wait** (normally ~80 ms). `runtime` went to 1895 ms (normally ~300).
- Conclusion: request handling on the BMC is effectively single-threaded or serialised per resource. **Parallel southbound calls only increase tail latency.**

### 10.4 Recommendations for the proxy (preliminary)
- One authenticated session per BMC, with a per-BMC request queue (concurrency 1, maybe 2) and a 6–10 s timeout. Retry an idempotent GET once on connection reset (the server sends `Connection: close` at random).
- Suggested background refresh/TTL:
  - power state (`/api/chassis`): 5–10 s, plus an immediate refresh 2 s and 10 s after a power action.
  - sensors: 10–30 s.
  - selsummary: 30–60 s; fetch eventlog only when it changes.
  - fru / firmware-info / asrr/fw-info / sdr / configuration/project: on startup and then hourly (they change only on firmware update or FRU edit).
  - network / users / date-time / settings/*: 5–15 min, or on demand with stale-while-revalidate.
- Never let a northbound Redfish GET trigger a synchronous `logs/eventlog` or `settings/users` fetch.

---

## 11. Error responses observed
Only one: `GET /api/asrr/product-name` → **HTTP 500**, `Connection: close`, body `{ "error": "Error in Getting Model Name", "code": 16005 }`. So the error envelope is `{error: string, code: int}`. Failures in the AMI framework often come back as 500 with an application `code`, so the Go client should surface `code` and not treat every 500 as a transport failure. The JS also references `code` 15000 (max sessions), 7 (unauthorized), 1000 (KVM token failure), 519 (custom message), 1365 (RAID: host powered down) and HTTP 12012 (large file).

---

## 12. Redfish on the X470D4U: needs verification
`/api/configuration/project` lists `REDFISH` and `REDFISH_HOSTINTERFACE`. Those are AMI build-time feature flags, and they suggest that BMC firmware 2.32.00 ships a Redfish service, at least for the BIOS host interface. The web UI bundle contains **no `/redfish` URLs**, so the HAR neither confirms nor denies a northbound-usable `/redfish/v1`. **Before choosing `/api`+IPMI as the only southbound path, probe `GET https://<bmc>/redfish/v1/` (unauthenticated service root) and `/redfish/v1/Systems` with basic auth.** Even if Redfish turns out to exist, it may be partial on this generation (inventory, sensors, power reset), so keep this document as the fallback.

---

## 13. What the HAR does NOT cover (capture next)

1. **Cookies:** re-export the HAR **with cookies/sensitive data** (Chromium DevTools: "Export HAR (with sensitive data)"), or log with `curl -v`. We need the `Set-Cookie: QSESSIONID=...` attributes (Secure/HttpOnly/Path/expiry), value length and format.
2. **Logout:** `DELETE /api/session` request/response.
3. **Session behaviour:** idle timeout, the response to an expired/invalid session (expected 401 body), a wrong `X-CSRFTOKEN`, a missing CSRF header, and login with bad credentials (status and `code`). Also the max concurrent sessions (code 15000) and `/api/settings/service-sessions` listing/kill.
4. **TLS:** protocol version, cipher, certificate (self-signed? key size? SAN). Use `openssl s_client -connect <bmc>:443`, since the HAR does not record it.
5. **Other power actions:** 0 (off), 2 (cycle), 3 (reset), 5 (ACPI), and whether 4 (NMI) is accepted. Include the responses when the action is invalid for the current state.
6. **UID LED:** `POST /api/actions/chassis-led` (`uid_command` 0/1/2) and `led_status` values when lit/blinking.
7. **Boot override:** no boot-device endpoint appears in the UI. Confirm whether one exists (the Redfish `Boot.BootSourceOverrideTarget` likely needs IPMI `chassis bootdev` instead).
8. **Virtual media:** `/api/settings/media/remote/*`, `/api/settings/media/instance`, start/stop-media, plus the KVM/H5Viewer websocket handshake (not needed for Redfish core, but needed for `VirtualMedia`).
9. **Writes:** sensor thresholds (`/api/settings/sensor-threshold/`), users (`PUT/POST/DELETE /api/settings/users/{id}`), network, NTP/date-time, SEL clear (`/api/logs/event` DELETE), BMC reset (`/api/maintenance/reset`, `/api/asrr/bmccoldreset`).
10. **Firmware update flow:** `/api/maintenance/firmware`, `flash`, `firmware/upgrade`, `flash-progress`, `firmware/verification`, `dual_image_config`; BIOS flash via `/api/asrr/maintenance/BIOS/*`.
11. **Host inventory with data:** all `host_interface_*` were empty. Capture after a full POST with BIOS inventory push enabled, and also call `host_interface_system_info`, `baseboard_info`, `power_info` and `thermal_info`. `/api/system_inventory/*` (cpu_info, dimm_info, pci_info, device_count) is referenced in JS but was never called and is worth probing.
12. **Single-sensor detail:** `/api/detail_sensors_readings` and the `/api/sensors` shape when a threshold is crossed (non-1 `sensor_state`) or a discrete sensor asserts (ChassisIntr bit0).
13. **Fan control:** `/api/settings/fan/*` and `/api/asrr/settings/get*fan*`; POST code via `/api/asrr/getbioscode`; `getprochot`.
14. **Services / ports:** `/api/settings/services` (web session timeout, ports), `/api/settings/channels`, `/api/settings/ncsi/mode`, `/api/settings/dns`, `/api/settings/smtp`, PEF and SNMP settings.
15. **SEL edge cases:** eventlog behaviour at wrap (CIRCULAR_SEL), and whether `eventlog` accepts paging or filter query parameters (none seen).
16. **IPMI over LAN:** not visible in HAR. Capture `ipmitool -I lanplus` sdr/sel/chassis/power/bootdev, and the `ipmitool raw 0x3a ...` ASRock OEM commands, to see what the proxy must cover via IPMI.
17. **Concurrency limits:** a controlled test of N parallel `GET /api/chassis` to quantify serialisation, and the behaviour under load (timeouts, 503s?).
18. **Redfish probe** (section 12).

---

## 14. Redaction log

| Item | Where | Placeholder | Original format |
|---|---|---|---|
| Password | `POST /api/session` form body | `<REDACTED_PASSWORD>` | 16 chars, `[A-Za-z0-9]` (**cleartext in the raw HAR**) |
| CSRF token | login response `CSRFToken`; `X-CSRFTOKEN` header on 121 requests | `<REDACTED_CSRF>` | 8 chars, `[A-Za-z0-9]`, constant for the session |
| Session id | login response `racsession_id` | `<REDACTED_SESSION_ID>` | small integer (1 digit) |
| Session cookie | (not present in HAR) | `QSESSIONID=<...>` | unknown |
| MAC addresses | `settings/network[].mac_address` (eth0, eth1) | `<REDACTED_MAC>` | `XX:XX:XX:XX:XX:XX`, uppercase hex, ASRock OUI |
| Board serial | `fru[0].board.serial_number` | `<REDACTED_SERIAL>` | 15 decimal digits |
| IP addresses | BMC `10.0.0.87`, client `10.0.0.252`, gateway `10.0.0.1` | kept | RFC1918, not public |
| Username | `admin` (stock account), `anonymous`, `fwupd` | kept | not personal |
| 3rd-party request | `cache.agilebits.com/...` (1Password extension, entry 9) | excluded | contains an opaque signed path |

---

## 15. Method
Python 3 (`python3 -I`) scripts in the session scratchpad parsed the HAR read-only. They extracted every entry's headers, timings and bodies, grouped `/api/*` calls by method+path, computed call intervals from `startedDateTime` and latency from `timings.wait`/`time`, profiled every JSON field (types, presence, distinct values), and diffed the host-off and host-on sensor snapshots. The web UI's JavaScript bundle (a text response inside the HAR) was searched as text for endpoint paths and enum meanings, and those are marked **[JS]**. No binaries were disassembled.
