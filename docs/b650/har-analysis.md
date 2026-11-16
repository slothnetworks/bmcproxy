# B650-family BMC web UI: HAR analysis (B650D4U and "B650D4U-2T")

Sources: `harfiles/b650d4u.har` (tagged **[B650]** below) and `harfiles/b650d4u-2t.har` (tagged **[2T]**). This is a preliminary assessment for the native Go Redfish proxy. It documents only what the captures show. Where the web UI JavaScript bundled inside each HAR (`/source.min.js`) explains a value, the finding is marked **[JS]**. Firmware facts quoted from `docs/firmware/b650d4u.md` are marked **[FW]**. No binary was disassembled and no BMC was contacted.

This document follows the structure of `docs/x470/har-analysis.md` (referred to as **X470 §n**). Fields that behave exactly as on the X470 are referenced, not re-explained. Only differences are documented.

Companion machine-readable file: `docs/b650/har-endpoints.json`. It has one object per board + method + path, with a `board` and a `capture` field, a redacted request/response example, call counts, intervals and timings. Its format matches `docs/x470/har-endpoints.json`, plus those two fields.

---

## 0. Key findings (TL;DR)

1. **The "2T" capture is not a B650 board.** In `b650d4u-2t.har`, `/api/asrr/model-name` returns `mb_name: "B550D4U-2T"`. The SEL/sensor names include "B550 Temp", the CPU is a Ryzen 7 5700X (AM4, Zen 3, microcode `0a20120e`) and the DIMMs are DDR4. It is an **AM4 B550D4U-2T**, not the AM5 B650D4U_2L2T analysed in `docs/firmware/b650d4u.md`. Below it is called "2T" and the table columns say "B550D4U-2T". Confirm which physical box was captured. A real B650D4U_2L2T capture is still missing.
2. **Both captures run BMC firmware 1.10.00** (B650D4U built 16 Mar 2023). The firmware doc analyses images **7.02.00 / 9.05.00**, so the HARs come from much older firmware. Expect endpoint and field differences after an upgrade.
3. **Same southbound protocol as the X470:** AMI MegaRAC `/api` JSON over HTTPS/lighttpd, `POST /api/session` (form) → `CSRFToken` → `X-CSRFTOKEN` header + `QSESSIONID` cookie. Differences: login sends **no `certlogin`**, the response adds **`user_id`, `channel`, `passwordStatus`**, and the unauthenticated error body is **`{"cc":7,"error":"Invalid Authentication"}` with HTTP 401**.
4. **The 401 on `GET /api/settings/services` (both captures) is benign UI start-up behaviour (§4.6).** On page load the SPA's `app.initialize()` saw a non-empty `document.cookie` (a stale cookie from an earlier session), assumed it was logged in, and fetched `settings/services` with `X-CSRFTOKEN: null` before anyone logged in. The BMC returned 401 cc 7. After login the same call returned 200. **No retry/refresh logic is involved.** It does show that **the BMC enforces auth and answers 401 + cc 7 for a missing or invalid session**, and that is exactly what the proxy must detect to re-login.
5. **Writes captured [B650]:**
   - `PUT /api/settings/network-bond {"id":1,"bond_enable":0,"bond_mode":"active-backup","auto_configuration_enable":1,"bond_ifc":"eth0"}` **disabled NIC bonding**. That restarts the BMC network, and the capture ends there. **Operational warning: the B650D4U's LAN configuration was changed during this capture.**
   - `POST /api/asrr/settings/setopenlooptablenumber {"table_numble":1}` is a **server-side cursor** that selects which fan open-loop table the next `GET getfanopenloopcontroltable` returns. The UI sends it **just by opening the page**. That makes "read the fan table" a two-step, stateful, race-prone operation (§7.2).
6. **Sensor JSON changed shape vs the X470 (§8):**
   - `settable_readable_threshMask` became **`settable_flag`**.
   - An unset threshold is the **string `"NA"`** (X470: `0.0`).
   - Whole-number values are **JSON ints**, others are floats with 17 significant digits (`3.3399999999999999`).
   - The B650 reports `unit: "deg_c"`, while the 2T reports `"°C"`.
   - The B650 uses a new sensor naming and numbering scheme (`VOLT_*`, `TEMP_*`, `FANn_m`, `STS_*`, sensor numbers 1–216). The 2T keeps the AM4/X470-like scheme.
   - A Go decoder needs a custom "number-or-NA" type.
7. **Host inventory is now populated** (X470: all empty). [B650] returns `system_info`, `baseboard_info` (BMC NICs) and `power_info` (power limit + 15 voltage sensors), with empty processor/memory. [2T] returns processor, 4 DIMMs and NVMe/SATA/USB storage. Every value is a **string** (`"4650"`, `"32768"`, `"NA"`).
8. **The BMC still serialises requests.** Uncontended calls take ~75–90 ms. In the dashboard bursts, cheap calls waited 0.5–1.5 s. Login took 360 ms [B650] vs **3.5 s** [2T].
9. **BIOS UpdateOption [2T] is not a timer.** The UI calls it on **every route change** (21 calls in 36 s). It only asks whether a deferred BIOS flash is armed (`action:1`). It was always `{"action":0}`.
10. **Security notes:**
    - Both raw HARs contain the **admin password in cleartext** (login form body).
    - Both web UI bundles embed a **static RSA private key** (commented out, next to the public key used for the optional "encrypt sensitive data" feature, which is disabled: `encrypt_sensitive_data = 0`).
    - [B650] the factory **uid-0 `sysadmin` account is enabled** (`/api/maintenance/system_admin`).
    - [B650] the CSRF token is stored in a **JS-readable cookie `garc`**.
    - Both BMC clocks are **hours off** (B650 +5 h, 2T −8 h, NTP disabled on 2T), so SEL timestamps are unreliable.

---

## 1. Capture metadata

| Item | [B650] `b650d4u.har` | [2T] `b650d4u-2t.har` |
|---|---|---|
| Creator | WebInspector 537.36 (Brave 157 / Chromium 157, Windows) | same browser; the two captures overlap in time (tabs open side by side) |
| Page | `https://<BMC_HOST>/` (RFC1918 BMC `10.3.10.128`, client `10.3.10.252`) | `https://<BMC_HOST>/` (RFC1918 BMC `10.6.10.75`, client `10.6.10.250`) |
| Time span (client clock, UTC) | 06:27:45.875 → 06:28:57.57 (~72 s) | 06:27:36.708 → 06:29:59.23 (~2 min 23 s) |
| Entries | 63: 47 `/api` XHR, 16 static (2 of them 404) | 73: 51 `/api` XHR, 1 `HEAD /bios/Index.html`, 21 static (7 of them 404) |
| Unique API method+path | 28 (25 GET, 2 POST, 1 PUT) | 24 (23 GET, 1 POST) |
| Board (`asrr/model-name`) | `B650D4U`, `sys_name "N/A"` | **`B550D4U-2T`**, `sys_name "N/A"` |
| BMC fw | `1.10.00`, built `Mar 16 2023 03:32:09 UTC` (`dashboardinfo`) | `1.10.00` (`asrr/fw-info`; static files Last-Modified 8 Mar 2023) |
| BIOS / PSP / microcode | `2.17` / `0.2B.0.4E` / `0a601203` (Zen 4) | `L1.02` / `0.14.0.3E` / `0a20120e` (Zen 3) |
| ME / CPLD / BP | `N/A` / `N/A` / `N/A` | `N/A` / `N/A` / (field absent) |
| IPMI Device ID | device 32, rev 129 (0x81), fw 1.16 (BCD "1.10"), IPMI 2.0, mfg 0x00C1D6, **prod_id 8207 (0x200F)** | not fetched |
| Host | on throughout (`power_status 1`); power-on hours counter 7679 | on throughout; power-on hours 3548; Ryzen 7 5700X, 4x32 GiB DDR4-2666, 2x 1 TB NVMe |
| Cookies | **stripped**: every `Cookie`/`Set-Cookie` header and `cookies[]` array is empty (Chromium sanitised export, as on the X470) | same |
| TLS | not recorded (no `_securityDetails`). All `https`, `HTTP/1.1`, :443 | same. One new connection mid-burst showed connect 598 ms, of which TLS took 537 ms |
| Connections | 6 connection ids, up to 15 requests each | 7 connection ids, up to 16 requests each |

### UI navigation sequence (reconstructed)

**[B650]**
- 06:27:45 login page loads. `source.min.js` (12.3 MB raw, 2.45 MB gzip) takes 8.4 s.
- 06:27:54.76: **401** `settings/services`.
- 06:28:03.66: login.
- 06:28:04: dashboard burst (`dashboardinfo`, `services`, `chassis-status`, `dashboardsensors`, `logs/dashboardevent`, `status/uptime`, `logs/audit?cmd=1`, `settings/network`, `asrr/model-name`, `asrr/fw-info`).
- 06:28:09–17: system inventory pages (`system_info`, `processor`, `memory`, `baseboard`, `power`, `baseboard`, `pcie` x2).
- 06:28:19: sensors.
- 06:28:27: network bond page.
- 06:28:33: "Keep Share NIC Link Up" page (`getvetobit`).
- 06:28:36: fan open-loop table page (`setopenlooptablenumber`, then `getfanopenloopcontroltable`).
- 06:28:39: remote control (`sol/session`, `media/adviser`, `services`).
- 06:28:44: restore factory defaults page.
- 06:28:45–46: firmware update page (`fwimage_location` x2).
- 06:28:47: system administrator page.
- 06:28:50: vetobit again.
- 06:28:53: bond page.
- **06:28:56.45: `PUT network-bond` (disable bonding)**, then end of capture.

**[2T]**
- 06:27:36 login page loads (JS 11.4 MB, 7.4 s).
- 06:27:45.67: **401** `settings/services`.
- 06:29:16.9: login (3.5 s).
- 06:29:20: `settings/users`.
- 06:29:22.9: dashboard burst (`services`, `configuration/project`, `date-time`, `uptime`, `logs/audit`, `network`, `model-name`, `fw-info`, `dashboardevent`, `BIOS/UpdateOption`).
- 06:29:23.9: `HEAD /bios/Index.html`, `sensors`, `chassis-status`.
- 06:29:29.8: event log page (`getmcalog`, `logs/eventlog?EVENTID=-1`, `sdr`).
- 06:29:34–38: inventory (`processor`, `memory`, `pcie` x2, `storage`).
- 06:29:46.9: POST-code page (`getbioscode`).
- 06:29:49: sensors.
- 06:29:53: bond page.
- 06:29:56: IPMI interfaces page.
- 06:29:59: bond page, then end.
- `BIOS/UpdateOption` follows every navigation.

---

## 2. Transport, server banner, headers, caching

### 2.1 Server and protocol
As X470 §2.1: `Server: lighttpd` (no version), HTTP/1.1 over TLS, no HTTP/2. Differences:
- **Clock skew:** the BMC `Date` header was **+17 996 to +17 999 s** (≈ +5 h) vs the client on [B650], and **−28 803 to −28 807 s** (≈ −8 h) on [2T]. On the X470 it was −4 to −6 s. [B650] `dashboardinfo` reports `timezone "GMT-05"`, `utc_minutes -300`, which is consistent with the wall clock having been set to UTC while the timezone was GMT-5. [2T] `date-time` reports `ntp_auto_date 0` (NTP off), `timezone "Etc/GMT"`. **The proxy must not trust BMC time.** Compute an offset per BMC, for example `timestamp` from `settings/date-time` vs local UTC, and use it to correct SEL `Created` times.

### 2.2 API (`/api/*`) response headers
Identical on all 98 API responses in both captures:
```
Cache-Control: no-store, no-cache, must-revalidate, private
Pragma: no-cache
Content-Type: application/json
Transfer-Encoding: chunked
Server: lighttpd
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
X-Frame-Options: SAMEORIGIN                                   (new vs X470)
Referrer-Policy: no-referrer                                  (new)
Content-Security-Policy: default-src 'self';object-src 'none';connect-src 'self' ws: wss:;style-src 'self';script-src 'self'; img-src 'self' blob:;frame-ancestors 'self';font-src 'self'   (new)
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload                    (new)
Date: <RFC1123, skewed, see 2.1>
```
These match the lighttpd `$HTTP["url"] =~ "^/api..."` block in the firmware doc **[FW §3.2]**. As on the X470:
- No `Content-Encoding`: API JSON is uncompressed even though the client sent `Accept-Encoding: gzip, deflate, br, zstd`.
- No `ETag`/`Last-Modified`, so the proxy needs its own TTL cache.
- No CORS headers.

`Connection: close` appeared on 3 random API responses per capture: [B650] `host_interface_baseboard_info` (2nd call), `chassis-status` (idx 48), `fwimage_location` (idx 55); [2T] `asrr/fw-info`, `BIOS/UpdateOption` x2. **Retry idempotent GETs on a fresh connection**, as on the X470.

JSON formatting differs from the X470. Floats are printed with 17 significant digits (`3.3399999999999999`, X470: `3.400000`), integral values print as ints, and `/` is still escaped (`"N\/A"`, `"Etc\/GMT"`). Some arrays end with a **non-record summary element** (`eventlog`, `dashboardevent`, see §9).

### 2.3 Static asset headers and 404 behaviour
- HTML/JS/CSS/PNG: `Cache-Control: no-store…`, `Pragma`, `Content-Encoding: gzip` (except PNG), `ETag`, **a real `Last-Modified` (16 Mar 2023 [B650], 8 Mar 2023 [2T])** instead of the X470's 1970 epoch, plus HSTS/CSP/Referrer-Policy/XFO.
- `/fonts/*` responses carry **no** `Cache-Control`/`ETag`/`Last-Modified` (lighttpd excludes `^/fonts` **[FW]**).
- **Unknown static paths return a real `404`** with a 345-byte XHTML "404 - Not Found" page: [B650] `/res/oem/h5banner_{left,right}.png`; [2T] those two plus `ionicons.ttf`, `fontawesome-webfont.woff2`, `glyphicons-halflings-regular.woff2`, `footable.{woff,ttf}`. **This differs from the X470**, which returned the SPA index with 200. The proxy *can* rely on 404 here.
- `GET /` is unauthenticated (1642 B [B650] / 1707 B [2T] decompressed).
- [2T] `HEAD /bios/Index.html` → **200** (`text/html`, gzip, 2356 B, Last-Modified 18 May 2026). The UI probes it to decide whether to show the "BIOS setup (Redfish)" button **[JS]**. The BIOS's own web setup page is served from the BMC (feature `BIOS_WEBUI_STATIC_FILE` **[FW §6.4]**), and that implies Redfish BIOS/RTP data exists on this board.

### 2.4 Request headers the UI sends on API calls
As X470 §2.4, except:
- **No `Referer`** on any request (both captures), because pages are served with `Referrer-Policy: no-referrer`. The BMC evidently does not require `Referer`. `Origin` is still sent on POST/PUT.
- `X-CSRFTOKEN` is present on **every** API request, including login. Before login its value is the literal string **`null`** (see §4.6).
- A few GETs send `Content-Type: application/json` with no body (`dashboardinfo`, `logs/audit`, `host_inventory/baseboard|power|pcie|storage`, fan table). This is harmless.
- `Accept: */*` on `asrr/model-name`, `asrr/fw-info`, `BIOS/UpdateOption`, the fan-table calls and the third `services` call [B650]. Everything else sends `application/json, text/javascript, */*; q=0.01`.

---

## 3. Request conventions summary for a Go client (differences from X470 §3)

| Concern | B650-family behaviour |
|---|---|
| Auth | `POST /api/session` form `username`, `password` (no `certlogin`) → JSON incl. `CSRFToken`; cookie `QSESSIONID` set by server (inferred; stripped) |
| Per-request | Cookie `QSESSIONID` + `X-CSRFTOKEN`. `Referer` not needed |
| Unauthenticated | **HTTP 401** + `{"cc": 7, "error": "Invalid Authentication"}` (note: `cc`, not `code`) |
| Other error envelope | Not observed in these captures. Expect `{"error":..., "code":...}` or `{"cc":...}`, so accept both, as the UI does **[JS bond.save error handler]** |
| Numbers | Sensor thresholds and readings may be int, float or `"NA"`. Host-inventory numbers are strings |
| Arrays | Some lists end with a summary object (`{"event_log_count":N}`, `{"pending_deassert":N,"dashboard_event_log_count":0}`) |
| Concurrency | Serialise per BMC. FastCGI `/api` runs with `max-procs 1` **[FW §5.1]** |
| 404 | Real 404 for missing static files |

---

## 4. Authentication flow

### 4.1 Login request
[B650] entry 11; [2T] entry 18.
```
POST /api/session HTTP/1.1
Content-Type: application/x-www-form-urlencoded; charset=UTF-8
Accept: application/json, text/javascript, */*; q=0.01
X-Requested-With: XMLHttpRequest
X-CSRFTOKEN: null
Origin: https://<BMC_HOST>
Content-Length: 40            ([2T]: 34)

username=admin&password=<REDACTED_PASSWORD>
```
- **No `certlogin` field.** The X470 UI sent `certlogin=0`. The B650 UI has no certificate-login code path (`certlogin` occurs 0 times in either bundle).
- `X-CSRFTOKEN: null` is sent because `$.ajaxSetup` was already primed with a missing token (§4.6). The server ignores it on login.
- The password is cleartext in the form body. [B650] 16 chars `[A-Za-z0-9]`; [2T] 10 chars `[A-Za-z0-9]`.
- Latency: [B650] **360 ms**, [2T] **3509 ms**, X470 1491 ms. Login cost varies a lot, so reuse sessions.

### 4.2 Login response
```json
{ "ok": 0, "privilege": 4, "user_id": 2, "extendedpriv": 259, "racsession_id": "<REDACTED_SESSION_ID>",
  "remote_addr": "10.3.10.252", "server_name": "10.3.10.128", "server_addr": "10.3.10.128",
  "HTTPSEnabled": 1, "CSRFToken": "<REDACTED_CSRF>", "channel": 1, "passwordStatus": 0 }
```
`ok`, `privilege`, `extendedpriv`, `remote_addr`, `server_name`, `server_addr` and `HTTPSEnabled` are as in X470 §4.2. `CSRFToken` is again **8 chars `[A-Za-z0-9]`** and constant for the session (45/45 [B650], 50/50 [2T] post-login requests). `racsession_id` was a small integer ([B650] 1 digit, [2T] 2 digits). New fields:

| Field | Type | Example | Meaning |
|---|---|---|---|
| `user_id` | int | `2` | IPMI user id of the logged-in user. The UI passes it to `settings/dashboardinfo`. A `user_id >= 1000` (LDAP/AD/RADIUS) skips that call **[JS]** |
| `channel` | int | `1` | LAN channel the session is bound to (1 = primary LAN; users are per channel on this firmware, see §5.16) |
| `passwordStatus` | int | `0` | `1` together with `ok != 0` means "default/expired password, must change": the UI shows the change-password form and does not proceed **[JS]**. Related: `ASRR_PASSWORD_AGE`, `/api/updatenew_password` **[FW §5.2]** |

After login the UI sets the JS cookie `refresh_disable=1`.
- [B650] it stores the token in a **JS cookie `garc`** (`app.setCookie('garc', CSRFToken, 2)`). A code comment says this replaced `localStorage` "for security", but the cookie is readable from JS and is also sent to the server on every request.
- [2T] it stores the token in `localStorage["garc"]` (as on the X470).

Both builds then call `$.ajaxSetup({headers:{'X-CSRFTOKEN': token}})` **[JS]**.

### 4.3 Session cookie
As X470 §4.3: `QSESSIONID` (name from JS and **[FW §5.2]**). Cookies are stripped from both HARs, so attributes, length and format are still unknown. Go: use one `cookiejar` per BMC.

### 4.4 Session timeout
`/api/settings/services` → `web.time_out = 1800` (s), so the UI arms a client-side idle timer of the same length (`Executetimer`) **[JS]**. This matches `WEB_SESSION_TIMEOUT=0x708` **[FW §6.3]**. Max web sessions = `148 & 0x7F` = **20**, with 1 active (§5.2).

### 4.5 Logout
Not captured in either HAR. From JS: `DELETE /api/session`, as on the X470.

### 4.6 What happened around the 401 on `GET /api/settings/services`

Both captures show the same sequence:

| # | [B650] entry / time | [2T] entry / time | Request | `X-CSRFTOKEN` | Status / body |
|---|---|---|---|---|---|
| 1 | 6 / 06:27:54.763 | 11 / 06:27:45.670 | `GET /api/settings/services` | `null` (literal string) | **401** `{ "cc": 7, "error": "Invalid Authentication" }` (46 B, normal API headers, no `WWW-Authenticate`) |
| 2 | 11 / 06:28:03.655 (+8.9 s) | 18 / 06:29:16.908 (+91 s) | `POST /api/session` | `null` | 200, `CSRFToken` issued |
| 3 | 13 / 06:28:04.173 (+0.5 s) | 20 / 06:29:22.888 (+6 s) | `GET /api/settings/services` | `<REDACTED_CSRF>` | 200, 7-service array |
| 4 | 52 / 06:28:40.458 | 35 / 06:29:27.481 | `GET /api/settings/services` | `<REDACTED_CSRF>` | 200, identical body |

Explanation, from the bundled JS **[JS]**:
1. On page load the SPA runs `app.initialize()`. It returns early only `if (!document.cookie)`. The browser still held a JS-visible cookie for this origin from an earlier session, probably `refresh_disable=1` or a stale `QSESSIONID` (cookies are stripped from the HAR, so the exact cookie cannot be identified). So the UI behaved as if it were logged in.
2. It ran `$.ajaxSetup({headers:{'X-CSRFTOKEN': app.getCookie('garc')}})` ([B650]) or `localStorage.getItem("garc")` ([2T]). No token existed, so jQuery serialised `null` as the header value `"null"`.
3. The "client handle session expire" block then did `GET /api/settings/services` to read `data[0].time_out` for the idle timer. The server found no valid session or CSRF pair and answered **401 cc 7**.
4. The error callback is the no-op statement `app.HTTPErrorHandler;` (not called), so nothing visible happened and the user logged in normally. The login POST still carried `X-CSRFTOKEN: null` from step 2.
5. After a successful login `app.initialize()` runs again with the real token. Call #3 returns 200 and arms the 1800 s timer.
6. Call #4 is a later fetch of the same resource. [B650] it came on the remote-control page with `Accept: */*`, so it is a different code path. [2T] it came 4.6 s after call #3, during navigation. In both it carried the valid token and returned the same body.

Conclusions for the proxy:
- **Unauthenticated → HTTP 401 with `cc: 7`.** Treat `401` (or a body with `cc == 7`) as "session invalid": re-login once, then retry. The 401 was fast (76 ms wait), so it is cheap to detect.
- The server does not appear to need any cookie on login. Whether a stale `QSESSIONID` sent with login matters cannot be seen, because cookies are stripped.
- Never send the literal `null` token. It is harmless here but proves nothing.

---

## 5. Endpoint catalog

Conventions as X470 §5: **n** = calls, timings = server `wait` min/median/max ms. There were no query strings except where shown. Field tables list **new or changed fields only**. "As X470 §x" means identical semantics.

### 5.1 `POST /api/session`: login
See §4. [B650] n=1, 360 ms; [2T] n=1, 3509 ms.

### 5.2 `GET /api/settings/services`: network service configuration (not captured on X470)
[B650] n=3 (401, 200, 200), 76/119/173 ms. [2T] n=3 (401, 200, 200), 76/402/731 ms. The array has 7 entries and was identical on both boards:

| Field | Type | Example | Meaning |
|---|---|---|---|
| `id` | int | `1` | Row index |
| `service_id` | int (bit) | `1` | Service bitmask id: 1 web, 2 kvm, 4 cd-media, 16 hd-media, 32 ssh, 128 solssh, 256 vnc |
| `service_name` | string | `web` | |
| `state` | bool-int | `1` | Service enabled (`solssh` = 0) |
| `interface_name` | string | `bond0` | Bound interface; `NA` = all (ssh, vnc) |
| `non_secure_port` | int | `-1` | `-1` = disabled (no plain HTTP) |
| `secure_port` | int | `443` | kvm 7582, cd-media 5124, hd-media 5127, ssh 22, vnc 5901 |
| `time_out` | int (s) | `1800` | Idle timeout. web 1800, kvm 1800, ssh 60, solssh 60, vnc 600, media `-1` |
| `maximum_sessions` | int (encoded) | `148` | **Display value = `v & 0x7F`; 255 = N/A [JS]**. web 20, kvm 2, cd/hd 1, vnc 2, ssh/solssh N/A |
| `active_session` | int (encoded) | `129` | Same encoding: web 1, others 0 |
| `singleport_status` | bool-int | `1` | Service tunnelled over 443 (`mod_singleport` **[FW]**) |
| `viewer_count` | int | `0` | kvm only |

Proxy use: read `web.time_out` to schedule keep-alives (any API call resets the idle timer) and `web` max/active to avoid exhausting the 20 web sessions.

### 5.3 `GET /api/settings/dashboardinfo?user_id=2&channel_id=1&user_name=admin` [B650 only]
n=1, 143 ms. The post-login dashboard call. Its query is built from the login response (`user_id`, `channel`) and the typed user name. It merges four things into one object:
- **IPMI Get Device ID**: `device_id, revision, fw_rev_1, fw_rev_2, ipmi_version, dev_support, mfg_id_0..2, prod_id, aux, fw_ver, date, time, active_image`, the same fields as X470 `/api/firmware-info` (X470 §5.3). B650 values: `revision 129` (bit 7 = device provides SDRs), `fw_rev_2 16` (BCD 0x10 → "10"), `prod_id 8207` (0x200F), `fw_ver "1.10.00"`, `date "Mar 16 2023"`, `time "03:32:09 UTC"`.
- **User record**: `id 2`, `privilege "administrator"`, `creation_time 1325394605` (2012-01-01, i.e. created while the RTC was at its default).
- **Dashboard NTP and timezone**: `dashboardntp_id 1`, `utc_minutes -300`, `timezone "GMT-05"`.
- **Counters**: `audit_count 4`, `minutes_per_count 60`, `poh_counter_reading 7679` (same as `/api/status/uptime`).

It also has `bios_ver ""`. The UI expects it but it is empty here. Use `asrr/fw-info` instead. The [2T] UI has no `dashboardinfo`: it calls `settings/users`, `configuration/project` and `date-time` separately, as the X470 does.

### 5.4 `GET /api/chassis-status`: power/UID (primary poll)
[B650] **n=13**, 83/84/689 ms, polled on a **5.0 s timer** (7 of 12 intervals within 4.99–5.011 s). Outliers come from navigation-triggered fetches. [2T] n=3, 102/116/848 ms. The 5 s poll was only visible from 06:29:40.8 (next interval 5.2 s). The UI registers the `5sec` handler from inside the `30sec` one **[JS]**, which probably explains the late start.

Fields: `power_status`, `led_status` as X470 §5.6. [B650] adds:

| Field | Type | Example | Meaning |
|---|---|---|---|
| `power_cycle_interval` | int (ms) | `2000` | Power-cycle off-time. The Power Control page waits `power_cycle_interval + 10000` ms before re-checking state after a cycle **[JS]** |

The X470 UI polled `/api/chassis`. **Neither B650-family UI calls `/api/chassis`** (it may still exist; untested). Body unchanged over the whole capture (host on).

### 5.5 `GET /api/dashboardsensors` [B650 only]
n=1, 421 ms, 3.8 KB. A lightweight sensor list: 59 entries in the same order and with the same `sensor_number` set as `/api/sensors`, but **only `{id, sensor_number, name}`**. The exception is sensors in a non-normal state: they carry the **full `/api/sensors` record**. Here that was one entry, `FAN7_1` (`sensor_state 8` = Lower Non-Critical, `accessible 0`, reading 0 RPM). The UI's `getCriticalSensors()` filters on `has("owner_id")` **[JS]**. **This makes a cheap "anything abnormal?" probe** (421 ms vs 751 ms for full sensors under the same load).

### 5.6 `GET /api/logs/dashboardevent`
[B650] n=1, 389 ms; [2T] n=1, 137 ms (2.8 s total, queued). This is a SEL summary for the dashboard: **61 records + 1 summary element**.

| Field | Type | Example | Meaning |
|---|---|---|---|
| `id` | int | `1` | SEL record id (oldest first) |
| `record_type` | string | `system_event_record` | As X470 §9 |
| `timestamp` | int (epoch s) | `1325376099` | Raw SEL time. Small values (113, 106) = seconds since BMC init; `1325376xxx` = 2012-01-01 default RTC before sync |
| `sensor_name` | string | `system_event`, `fan`, `os_boot` [B650]; `system_event`, `temperature` [2T] | **This is the sensor *type* name**, not the sensor's ID string |
| `sensor_number` | int | `0` | |
| *(last element)* `pending_deassert` | int | `41` [B650] / `43` [2T] | Number of asserted events without a matching deassertion |
| *(last element)* `dashboard_event_log_count` | int | `0` | |

Go: decode as `[]json.RawMessage` and split off the element without `id`.

### 5.7 `GET /api/status/uptime`
[B650] 544 ms, [2T] 1175 ms (both inside the burst). Body `{ "minutes_per_count": 60, "poh_counter_reading": 7679 }` ([2T] 3548). This is the IPMI **power-on-hours** counter: hours = reading x minutes_per_count / 60. It is not BMC uptime. It maps to Redfish `Systems.Oem` or a `PowerOnHours` metric.

### 5.8 `GET /api/logs/audit`
- [B650] `?cmd=1`, 306 ms: `[ { "audit_count": 4 } ]`. `cmd=1` means "count only" **[JS]**.
- [2T] no query, 883 ms (2.7 s total), 5.2 KB, 29 records:

| Field | Type | Example | Meaning |
|---|---|---|---|
| `id` | int | `1` | Oldest first |
| `timestamp` | int (epoch s) | `1781583943` | Skewed BMC clock (§2.1) |
| `hostname` | string | `<REDACTED_HOSTNAME>` | BMC hostname `AMI` + MAC without colons (15 chars) |
| `message` | string | `spx_restservice: spx_restservice - -  [979 : 979 INFO]https Login from IP:10.6.10.250 user:admin - ` | Syslog line. Seen: `https Login`, `HTTPS logout` (spx_restservice), `kvm Login`, `KVM logout` (adviserd). Other source IPs seen are RFC1918 `172.18/172.29.x`. One non-stock user name appears (redacted `<REDACTED_USER>`; it is not in `settings/users`, so it is probably an LDAP/Redfish or automation account) |

The audit log shows each web login and logout. A proxy that logs in repeatedly will flood it, which is another reason to keep one long session.

### 5.9 `GET /api/settings/network`
[B650] 372 ms, [2T] 620 ms. Same fields as X470 §5.8, but **one entry: `interface_name "bond0"`, `channel_number 1`** (X470: eth0 + eth1). The MAC is redacted (ASRock OUI, `XX:XX:XX:XX:XX:XX` uppercase). Static IPv4, DHCPv6 on, VLAN off on both.

### 5.10 `GET /api/settings/network-bond` (and the PUT, §7.1)
[B650] n=2, 84–87 ms; [2T] n=2, 114–130 ms. Body as X470 §5.10 but **`bond_enable: 1`** on both boards (X470: 0). The GET does **not** return `bond_ifc`.

### 5.11 `GET /api/asrr/model-name`
829 ms [B650] / 1165 ms [2T] (in burst). `{ "mb_name": "B650D4U", "sys_name": "N\/A" }`. **New `sys_name`** (system/chassis product name, hidden in the UI by default **[JS]**). This replaces the X470's separate `asrr/product-name` (which returned 500 there). Neither board called `product-name`.

### 5.12 `GET /api/asrr/fw-info`
556 ms [B650] / 648 ms [2T]. As X470 §5.21 plus **`PSP_fw_version`** (AMD Platform Security Processor firmware; `ME_fw_version` is now `N/A`) and, on [B650] only, **`BP_version`** (backplane firmware, `N/A`). Redfish: add PSP (and BP when present) to `FirmwareInventory`.

### 5.13 `GET /api/host_inventory/*`
Compared in §10. Calls: [B650] `system_info` 1, `processor` 1, `memory` 1, `baseboard` 2, `power` 1, `pcie` 2; [2T] `processor` 1, `memory` 1, `pcie` 2, `storage` 1. All take 83–202 ms.

### 5.14 `GET /api/sensors`
[B650] n=1, 751 ms, 29 KB, 59 sensors. [2T] n=2 (25.5 s apart), 853 ms in burst / 232 ms alone, 25 KB, 51 sensors. See §8.

### 5.15 `GET /api/asrr/getvetobit` [B650]
n=2 (17.6 s apart), 84 ms. `{ "vetobit": 0 }`. See §7.3.

### 5.16 `GET /api/settings/users` [2T]
n=1, 1266 ms, 14.7 KB. **30 entries = 15 user ids x 2 LAN channels** (`channel` 1 and 8; ids 1–15 for channel 1, 16–30 for channel 8). The X470 had 10 slots with no channel split. Named users: `anonymous` (id 1, disabled), `admin` (id 2), `Administrator` (id 3, the Redfish default account, created with `UNIFIED_USER_ACCOUNT` **[FW §6.1]**, `kvm 0`, `vmedia 0`). Changes vs X470 §5.2:

| Field | Type | Example | Meaning |
|---|---|---|---|
| `channel` | int | `1` / `8` | LAN channel of this row |
| `channel_type` | int | `4` | IPMI channel medium (4 = 802.3 LAN) |
| `userid` | int | `2` | IPMI user id (same across both channel rows); `id` is the row index |
| `accessByChannel` | string | `"(1,1)"` | Enable flag per channel `(ch1,ch8)`. Empty slots use a different format: `"0,0,0"` |
| `privilege` | string | `administrator` | **Renamed from `network_privilege`**. Row 18 (`Administrator`, ch 8) shows `none` |
| `privilegeByChannel` | string | `"(administrator,none)"` | Privilege per channel; empty slots `" , , "` |
| `password_age` | int | `0` | Password expiry policy (days; 0 = never), `ASRR_PASSWORD_AGE` |
| `password_enddate` | int | `0` | Expiry epoch (0 = none) |
| *(removed)* `privilege_limit_serial` | | | Not present |

Go: parse the tuple strings defensively. Their format differs between populated and empty rows.

### 5.17 `GET /api/configuration/project` [2T]
n=1, 963 ms. **83 entries** (duplicates: `YES`, `CAPTURE_BSOD_JPEG`, `REDFISH_HOSTINTERFACE`). Both this board and the X470 list `REDFISH` and `REDFISH_HOSTINTERFACE`.
- **Only on the 2T:** `SYSTEM_FIREWALL_TIMEOUT, ENC_IMAGE, SOLSSH_SUPPORT, ASRR_FAN_TABLE, ASRR_PASSWORD_AGE, BIOS_CODE_2BYTES, BIOS_SUPPORT, BIOS_FW, BIOS_FLASH_ON_THE_FLY, DYNAMIC_KCS_LAN_SUPPORT, MEDIA_REDIR_READ_WRITE_ONLY, SKIP_REPOSITIONING_JVIEWER_WINDOW, FOLDER_REDIRECTION, VNC_SUPPORT, VNC_OVER_STUNNEL_SUPPORT, VNC_OVER_SSH_SUPPORT, CAPTURE_BSOD_JPEG, VMEDIA_MAX_COUNT_FOR_KVM, RMEDIA_MULTI_IMAGE_SUPPORT, RMEDIA_MULTI_CD_SUPPORT, RMEDIA_MULTI_HD_SUPPORT, RMEDIA_SESSION_RECONNECT, CPLD_FW, BP_FW, PSU_FW, RETIMER_FW, FPGA_FW, ASRR_REST_SUPPORT, UNIFIED_USER_ACCOUNT, KVM_OR_MEDIA_AVAILABLE, PRESERVE_SSH_SUPPORT`.
- **Only on the X470:** `VERSION_CMP_FLASH, CAPTURE_BSOD_RAW, FD_SERVER_APP, BONDING_SUPPORT`.

### 5.18 `GET /api/settings/date-time` [2T]
n=1, 1131 ms. As X470 §5.11 except:
- `auto_date` is **renamed `ntp_auto_date`** (0 here: NTP off).
- `daylight` is **absent**.
- `timezone` is an IANA-style name (`"Etc\/GMT"`; X470 `"GMT+0"`).
- `timestamp` 1794781758 = 2026-11-15 22:29:18Z, while real UTC was ~06:29:22Z on 11-16, so the BMC clock is 8 h behind.

### 5.19 `GET /api/asrr/maintenance/BIOS/UpdateOption` [2T]
**n=21**, 73/107/1457 ms, body always `{ "action": 0 }`. See §7.4.

### 5.20 `GET /api/asrr/getmcalog` [2T]
157 ms. `{ "MCADump": 0 }`, as X470 §5.21.

### 5.21 `GET /api/logs/eventlog?EVENTID=-1` [2T]
1278 ms + 309 ms receive, 50.7 KB. See §9.

### 5.22 `GET /api/sdr` [2T]
1442 ms (contended). See §8.5.

### 5.23 `GET /api/asrr/getbioscode` [2T]
97 ms. `{ "poststatus": 1, "postdata": 3242 }`.

| Field | Type | Meaning |
|---|---|---|
| `poststatus` | bool-int | 1 = a POST code is available |
| `postdata` | int | Last BIOS POST code (port 80h/81h). With feature `BIOS_CODE_2BYTES` it is 16-bit and the UI renders 4 hex digits **[JS]**: 3242 = `0CAA`. Without the feature, 2 digits |

Redfish: `Systems.BootProgress.OemLastState` or an Oem field. Cheap enough to poll during boot.

### 5.24 `GET /api/settings/ipmi_disable_interfaces` [2T]
114 ms. `{ "ipmi_over_LAN": 1, "ipmi_over_kcs": 1 }`. Bool-ints: whether IPMI-over-LAN (RMCP+) and in-band KCS are enabled. The UI warns against disabling both **[JS]**. **The proxy should read this before relying on IPMI southbound.** If `ipmi_over_LAN` is 0, IPMI fallbacks fail.

### 5.25 `GET /api/sol/session` [B650]
77 ms. `{ "status": 0 }`. SOL session state (0 = no active SOL session). Model `models/sol` **[JS]**.

### 5.26 `GET /api/settings/media/adviser` [B650]
132 ms. As X470 §5.12, except `kvm_port 65535` (X470: 7578; with single-port KVM the dedicated port is unused, and `services.kvm.secure_port` says 7582) and a new **`kvm_client: 1`** (KVM client type selector; not decoded by these captures).

### 5.27 `GET /api/maintenance/restore_defaults` [B650]
128 ms.
```json
{ "id": 1, "sdr": 0, "fru": 0, "sel": 0, "ipmi": 0, "network": 0, "ntp": 0, "snmp": 0,
  "ssh": 0, "kvm": 0, "authentication": 0, "syslog": 0, "web": 0, "redfish": 0 }
```
Each flag (bool-int) says whether that configuration area is **preserved** across "Restore Factory Defaults" **[JS locale]**. All 0 means **a factory restore would wipe everything, including the network config**, and the BMC would come back on factory network settings. The proxy must never expose a restore-defaults action without checking this. The `redfish` key confirms a Redfish config store exists on this firmware.

### 5.28 `GET /api/maintenance/fwimage_location` [B650]
n=2 (1.3 s apart), 83 ms. Identical to X470 §5.20. The X470's `signed-image-key` was not called.

### 5.29 `GET /api/maintenance/system_admin` [B650]
130 ms. `{ "id": 1, "enable_change_password": 0, "enable_user_access": 1, "username": "sysadmin" }`. `sysadmin` is the factory **uid-0 shell account** **[FW §6.1]**. `enable_user_access 1` = the account is **enabled** (SSH root-equivalent login, given `PermitRootLogin yes` **[FW §8.4]**). `enable_change_password` is the UI's "change password" checkbox state. A PUT with `password`/`cpassword` (5–20 non-space chars) sets the password **[JS validation]**. **Security: verify this account's password is not the factory one, or disable it.**

### 5.30 `GET /api/asrr/settings/getfanopenloopcontroltable`, `POST /api/asrr/settings/setopenlooptablenumber`, `PUT /api/settings/network-bond`
See §7.

---

## 6. Endpoint call counts at a glance

| Board | Method | Path | n | Status | wait min/med/max (ms) | Trigger / cadence |
|---|---|---|---|---|---|---|
| B650 | GET | /api/chassis-status | 13 | 200 | 83/84/689 | **5 s timer** |
| B650 | GET | /api/settings/services | 3 | 401, 200, 200 | 76/119/173 | app init (pre- and post-login), remote-control page |
| B650 | GET | /api/host_inventory/host_interface_baseboard_info | 2 | 200 | 91/91/92 | page loads (1.1 s apart) |
| B650 | GET | /api/host_inventory/host_interface_pcie_device_function_info | 2 | 200 | 83/85/87 | page loads |
| B650 | GET | /api/settings/network-bond | 2 | 200 | 84/85/87 | bond page |
| B650 | GET | /api/asrr/getvetobit | 2 | 200 | 84/84/84 | Keep-NCSI page |
| B650 | GET | /api/maintenance/fwimage_location | 2 | 200 | 83/83/84 | firmware page |
| B650 | POST | /api/session | 1 | 200 | 360 | login |
| B650 | GET | /api/settings/dashboardinfo | 1 | 200 | 143 | after login |
| B650 | GET | /api/dashboardsensors | 1 | 200 | 421 | dashboard |
| B650 | GET | /api/logs/dashboardevent | 1 | 200 | 389 | dashboard |
| B650 | GET | /api/status/uptime | 1 | 200 | 544 | dashboard |
| B650 | GET | /api/logs/audit?cmd=1 | 1 | 200 | 306 | dashboard |
| B650 | GET | /api/settings/network | 1 | 200 | 372 | dashboard |
| B650 | GET | /api/asrr/model-name | 1 | 200 | 829 | dashboard |
| B650 | GET | /api/asrr/fw-info | 1 | 200 | 556 | dashboard |
| B650 | GET | /api/host_inventory/host_interface_system_info | 1 | 200 | 85 | inventory |
| B650 | GET | /api/host_inventory/host_interface_processor_info | 1 | 200 | 85 | inventory |
| B650 | GET | /api/host_inventory/host_interface_memory_info | 1 | 200 | 85 | inventory |
| B650 | GET | /api/host_inventory/host_interface_power_info | 1 | 200 | 122 | inventory |
| B650 | GET | /api/sensors | 1 | 200 | 751 | sensors page |
| B650 | POST | /api/asrr/settings/setopenlooptablenumber | 1 | 200 | 83 | fan table page open |
| B650 | GET | /api/asrr/settings/getfanopenloopcontroltable | 1 | 200 | 92 | fan table page open |
| B650 | GET | /api/sol/session | 1 | 200 | 77 | remote control page |
| B650 | GET | /api/settings/media/adviser | 1 | 200 | 132 | remote control page |
| B650 | GET | /api/maintenance/restore_defaults | 1 | 200 | 128 | restore page |
| B650 | GET | /api/maintenance/system_admin | 1 | 200 | 130 | system admin page |
| B650 | PUT | /api/settings/network-bond | 1 | 200 | **1105** | user save |
| 2T | GET | /api/asrr/maintenance/BIOS/UpdateOption | **21** | 200 | 73/107/1457 | **every route change** |
| 2T | GET | /api/settings/services | 3 | 401, 200, 200 | 76/402/731 | app init |
| 2T | GET | /api/chassis-status | 3 | 200 | 102/116/848 | dashboard, then 5 s timer |
| 2T | GET | /api/sensors | 2 | 200 | 232/543/853 | sensors page |
| 2T | GET | /api/host_inventory/host_interface_pcie_device_function_info | 2 | 200 | 142/160/178 | inventory |
| 2T | GET | /api/settings/network-bond | 2 | 200 | 114/122/130 | bond page |
| 2T | POST | /api/session | 1 | 200 | **3509** | login |
| 2T | GET | /api/settings/users | 1 | 200 | 1266 | after login |
| 2T | GET | /api/configuration/project | 1 | 200 | 963 | dashboard |
| 2T | GET | /api/settings/date-time | 1 | 200 | 1131 | dashboard |
| 2T | GET | /api/status/uptime | 1 | 200 | 1175 | dashboard |
| 2T | GET | /api/logs/audit | 1 | 200 | 883 | dashboard |
| 2T | GET | /api/settings/network | 1 | 200 | 620 | dashboard |
| 2T | GET | /api/asrr/model-name | 1 | 200 | 1165 | dashboard |
| 2T | GET | /api/asrr/fw-info | 1 | 200 | 648 | dashboard |
| 2T | GET | /api/logs/dashboardevent | 1 | 200 | 137 | dashboard |
| 2T | GET | /api/asrr/getmcalog | 1 | 200 | 157 | event log page |
| 2T | GET | /api/logs/eventlog?EVENTID=-1 | 1 | 200 | 1278 (+309 receive) | event log page |
| 2T | GET | /api/sdr | 1 | 200 | 1442 | event log page |
| 2T | GET | /api/host_inventory/host_interface_processor_info | 1 | 200 | 124 | inventory |
| 2T | GET | /api/host_inventory/host_interface_memory_info | 1 | 200 | 155 | inventory |
| 2T | GET | /api/host_inventory/host_interface_storage_info | 1 | 200 | 202 | inventory |
| 2T | GET | /api/asrr/getbioscode | 1 | 200 | 97 | POST code page |
| 2T | GET | /api/settings/ipmi_disable_interfaces | 1 | 200 | 114 | IPMI interfaces page |

Neither capture called `/api/configuration/runtime` (X470: 39 calls, one per route change) or `/api/chassis`.

---

## 7. Write operations and stateful reads in detail

### 7.1 `PUT /api/settings/network-bond` [B650, entry 62, 06:28:56.449]
```
PUT /api/settings/network-bond HTTP/1.1
Content-Type: application/json
X-CSRFTOKEN: <REDACTED_CSRF>
Origin: https://<BMC_HOST>
Content-Length: 100

{"id":1,"bond_enable":0,"bond_mode":"active-backup","auto_configuration_enable":1,"bond_ifc":"eth0"}
```
Response: HTTP 200 after **1105 ms**, echoing the body: `{ "id": 1, "bond_enable": 0, "bond_mode": "active-backup", "auto_configuration_enable": 1, "bond_ifc": "eth0" }`.

| Field | Type | Sent | Meaning |
|---|---|---|---|
| `id` | int | `1` | Bond config row (always 1). Carried over from the GET (Backbone `model.save()` sends all model attributes) |
| `bond_enable` | bool-int | `0` (was `1`) | Enable/disable NIC bonding. **This is the change: bonding was turned off** |
| `bond_mode` | string | `active-backup` | Bonding mode. Read-only in the UI ("not configurable"), sent back unchanged |
| `auto_configuration_enable` | bool-int | `1` | "Auto Configuration": when enabled, all network services (web, KVM, media, ...) are reconfigured and restarted automatically onto the new interface layout. When disabled, service-to-interface binding must be done via IPMI commands **[JS help text]** |
| `bond_ifc` | string | `eth0` | `eth0` or `eth1` from a `<select>`. The GET never returns it, so the UI sends the select's current value, which defaults to the first option `eth0`. The UI enables the field only when bonding is enabled, and only compares it for change detection when `bond_enable == 1` **[JS saveConfirm]**. Its effect when *disabling* bonding is not shown by the capture: it is probably the interface that keeps the configuration, or it is ignored. **Verify on hardware** |

Side effects **[JS locale]**: "Saving LAN settings will restart the BMC's network services. Please reconnect using new browser session". The success message says the interface comes back "with new IP". The capture ends 1 s later. **The B650D4U's management network was reconfigured: bond0 (which carried `10.3.10.128`) was dissolved.** Whether eth0 kept the address is unknown. Anyone probing that BMC now should check reachability and `GET /api/settings/network`.

Proxy implications:
- Treat network writes as connection-terminating. Expect the TCP session and possibly the IP to disappear, and re-discover the BMC.
- Before enabling bonding, the UI warns that VLAN must be disabled on the slave interfaces.
- Redfish mapping: `Managers/{id}/EthernetInterfaces` (bond as an aggregate). Do not expose this initially.

### 7.2 Fan open-loop control table [B650, entries 46–47]
**Step 1: select the table** (06:28:35.999, 83 ms):
```
POST /api/asrr/settings/setopenlooptablenumber HTTP/1.1
Content-Type: application/json
Accept: */*
X-CSRFTOKEN: <REDACTED_CSRF>
Content-Length: 18

{"table_numble":1}
```
Response 200, body `{ }`.

**Step 2: read it** (96 ms later, 92 ms): `GET /api/asrr/settings/getfanopenloopcontroltable` →
```json
[ { "Get_Temp1": 255, ..., "Get_Temp24": 255, "Get_Duty1": 255, ..., "Get_Duty24": 255 } ]
```

| Field | Type | Meaning |
|---|---|---|
| `table_numble` (sic) | int | **Table selector, i.e. a server-side cursor.** `1..8` = customized table N. `N \| 0x80` (129..136) = factory **default** table N (the "Default" button) **[JS]**. The 8 tables are referenced by the "Temperature Sensor and Corresponding Fan Table" page (`fan_tscft`), which maps each temperature sensor to a table and fans (endpoints `gettscftsupportfan` etc., not captured) |
| `Get_TempN` | int (°C) | Temperature breakpoint N (N = 1..24). Valid 0–120, non-decreasing across N |
| `Get_DutyN` | int (%) | Fan PWM duty at breakpoint N. Valid 20–100, non-decreasing |
| value `255` | | Unused point. The UI shows it blank. All 48 values = 255 → **customized table 1 is empty (never configured)** |

The table is a piecewise curve of up to 24 (temp → duty) points, read in order. UI validation **[JS CheckFunction]**:
- integers only;
- temp and duty set in pairs (both or neither);
- no gaps (a point after an unused one is invalid);
- monotonic non-decreasing temps and duties;
- point 1 must be set.

**Write (JS only, not captured):** `POST /api/asrr/settings/setfanopenloopcontroltable` with `{"table_numble":"1","temp1":"..",...,"temp24":"..","duty1":"..",...,"duty24":".."}`. The values are **strings** from the input boxes, 255 for unused. "Clear" sends the same body with every value `0xff` (255, as numbers).

Why it matters to the proxy:
- **Opening the UI page performs a write.** `afterRender` POSTs `{"table_numble":1}` unconditionally.
- "Read table N" = `POST set(N)` + `GET`. The selection is global BMC state, so concurrent clients (a human in the web UI, another proxy instance) can interleave and you read the wrong table. Serialise the pair under the per-BMC lock and never assume the cursor's current value.
- Open-loop tables are ASRock-specific (`ASRR_FAN_TABLE`). Redfish has no standard model for them, so expose them, if at all, under `Oem`.

### 7.3 `GET /api/asrr/getvetobit` [B650]
`{ "vetobit": 0 }` (n=2). This is the UI page **"Keep Share NIC Link Up"** **[JS locale]**. With `vetobit: 1` the shared NIC's (NC-SI) PHY stays linked up across host resets, so the BMC's shared-LAN connection survives host reboots. With 0 (current) the shared-NIC link may drop during a host reset. Write: `PUT /api/asrr/setvetobit` with `{"vetobit": 0|1}` (JSON, confirm dialog) **[JS]**, not captured. **Proxy relevance:** if a BMC is reached via the shared NIC, `vetobit 0` means it can go unreachable during host power actions, so increase timeouts or alert.

### 7.4 BIOS `UpdateOption` polling [2T]
`GET /api/asrr/maintenance/BIOS/UpdateOption` → `{ "action": 0 }`, 21 times. Intervals 0.33–6.2 s (median 1.45 s), all following navigation. **It is not a timer.** The SPA's `postSwitch()` (run after every route change) calls `app.GetBiosUpdateOption()` **[JS, "Vic: Add bios update notification in webui"]**.

What it waits for:
- `action` holds the **pending BIOS-flash mode** chosen on the BIOS update page. The radio values are `1` "Flash BIOS after manually shutdown server" (deferred), `2` "Immediately flash BIOS without power action" (flash on the fly, `BIOS_FLASH_ON_THE_FLY`), `3` "shut down host, flash, reboot" (default). `0` = nothing pending.
- Only `action == 1` triggers anything. The UI then schedules `GET /api/asrr/maintenance/BIOS/status` after 10 s:
  - `response == 4` → "Bios Flashing": call `GET .../BIOS/flash-progress` and re-check every 15 s.
  - `response == 3` → "Bios Update Successful".
  - `response == 2` → "Bios Update fail".
  - On 3 and 2 it resets the option with `POST api/asrr/maintenance/BIOS/upgrade {"action":0}`.
- So it is a notification check for a BIOS image staged earlier to flash at the next host shutdown. `{"action":0}` means no staged BIOS update.

Proxy relevance:
- If `action == 1`, **a BIOS flash will start when the host next powers off**. A Redfish `ComputerSystem.Reset` (GracefulShutdown/ForceOff) would trigger it. The proxy should check this before power-off actions, surface it (for example as an `UpdateService` task or a warning), and stop the host from being powered on mid-flash.
- [B650] the UI does not have this hook (the code is absent from its bundle), although `UpdateOption` appears there 76 times as the upload-page radio name.

---

## 8. Sensors, dashboardsensors and SDR vs the X470

### 8.1 Record schema differences (`/api/sensors`, both boards)
Fields as X470 §8.1, with these changes:

| Aspect | X470 | B650 / 2T |
|---|---|---|
| Threshold mask field | `settable_readable_threshMask` | **`settable_flag`** (same 16-bit value and meaning, e.g. 13878 = 0x3636; see X470 §8.2) |
| Unset threshold | `0.0` | **`"NA"` (string)** |
| Numeric typing | always float (`170.0`) | **int when integral** (`raw_reading 167`, `reading 17700`, thresholds `100`, `84`), float otherwise (`3.3399999999999999`) |
| `raw_reading` | float | int |
| Temperature unit | `°C` | [B650] **`deg_c`**; [2T] `°C` (the UI handles both) |
| Field order | `accessible` near the end | `accessible` after `discrete_state` (order is irrelevant to JSON) |

**Go:** define `type Num struct{ V float64; Valid bool }` with an `UnmarshalJSON` that accepts number or `"NA"`. Derive "threshold present" from `settable_flag` and/or `Valid`, never from `== 0`.

`sensor_state`, `discrete_state`, `accessible == 213` and the discrete `32768` (0x8000) "readable, no bits asserted" convention are unchanged (X470 §8.3–8.4).

**Observed abnormal sensor [B650]:** `FAN7_1` (sensor 23) has `accessible 0`, `reading 0`, `sensor_state 8` (**Lower Non-Critical**, below the 100 RPM LNC threshold). It is a fan header with no fan that is not masked as absent. Other unpopulated fan headers report `accessible 213`. A proxy mapping this to Redfish will show a **Warning** health for that fan unless it is filtered by configuration.

### 8.2 Sensor inventory [B650] (59 sensors, host on)

| sensor_number | name | type (number) | unit | thresholds LNR/LC/LNC/UNC/UC/UNR | accessible | reading |
|---|---|---|---|---|---|---|
| 1 | VOLT_3VSB | voltage (2) | V | 2.86/3.04/–/–/3.72/3.88 | 0 | 3.34 |
| 2 | VOLT_5VSB | voltage (2) | V | 4.26/4.50/–/–/5.49/5.76 | 0 | 5.10 |
| 3 | VOLT_P0_VCORE | voltage (2) | V | 0.21/0.23/–/–/1.65/1.73 | 0 | 1.42 |
| 4 | VOLT_P0_VSOC | voltage (2) | V | 0.21/0.23/–/–/1.65/1.73 | 0 | 1.04 |
| 5 | VOLT_VMEM | voltage (2) | V | none (mask 0) | 0 | 1.10 |
| 6 | VOLT_VMISC | voltage (2) | V | 0.42/0.46/–/–/1.66/1.74 | 0 | 1.12 |
| 7 | VOLT_1.8V_PT21 | voltage (2) | V | 1.53/1.62/–/–/1.98/2.07 | 0 | 1.80 |
| 8 / 9 | VOLT_VSUS10 / VOLT_VDD10 | voltage (2) | V | 0.85/0.90/–/–/1.21/1.27 | 0 | 1.05 |
| 13 | VOLT_BAT | voltage (2) | V | 2.22/2.40/–/–/3.30/3.46 | 0 | 3.14 |
| 14 | VOLT_3V | voltage (2) | V | 2.80/2.96/–/–/3.62/3.80 | 0 | 3.36 |
| 15 | VOLT_5V | voltage (2) | V | 4.26/4.50/–/–/5.49/5.76 | 0 | 5.07 |
| 16 | VOLT_12V | voltage (2) | V | 10.2/10.8/–/–/13.2/13.8 | 0 | 12.0 |
| 17–23 | FAN1_1 … FAN7_1 | fan (4) | RPM | LNC 100 | FAN1_1 0, FAN3_1 0, FAN6_1 0, FAN7_1 0; others 213 | 17700, 8500, 17100, **0 (state 8)** |
| 25–31 | FAN1_2 … FAN7_2 | fan (4) | RPM | LNC 100 | 213 | – |
| 42 | TEMP_CPU | temperature (1) | deg_c | UNC 98, UC 99 | 0 | 73 |
| 48–51 | TEMP_DDR5_A1/A2/B1/B2 | temperature (1) | deg_c | UNC 84, UC 85 | A1, B1 0; A2, B2 213 | 45, 43 |
| 80 | TEMP_FCH | temperature (1) | deg_c | UNC 94, UC 95 | 0 | 41 |
| 81 / 83 | TEMP_MB / TEMP_CARD_SIDE | temperature (1) | deg_c | none (mask 0) | 0 | 38 / 39 |
| 84 | TEMP_M.2_1 | temperature (1) | deg_c | UNC 69, UC 70 | 213 | – |
| 112–117 | VOLT_PSU1_VIN, CUR_PSU1_IOUT, PWR_PSU1_PIN, PWR_PSU1_POUT, TEMP_PSU1, FAN_PSU1 | 2/3/8/8/1/4 | V/A/W/W/deg_c/RPM | FAN LNC 100 only | 213 | – (no PMBus PSU) |
| 120–125 | same for PSU2 | | | | 213 | – |
| 176 | ChassisIntr | physical_security (5) | unknown | discrete 0x6F | 0 | 0x8000 |
| 177 / 184 | STS_PSU1 / STS_PSU2 | power_supply (8) | unknown | discrete 0x6F | 0 | 0x8000 |
| 178/179, 185/186 | STS_PSUn_FAN, STS_PSUn_AC_LOST | power_supply (8) | unknown | discrete | 213 | – |
| 208 | PROCHOT_CPU | processor (7) | unknown | discrete 0x03 | 0 | 0x8000 |
| 209 | THERMTRIP_CPU | processor (7) | unknown | discrete 0x6F | 0 | 0x8000 |
| 212 | STS_CPU_MCE | processor (7) | unknown | discrete 0x6F | 0 | 0x8000 (new: machine-check sensor) |
| 216 | WATCHDOG2 | watchdog_2 (35) | unknown | discrete 0x6F | 0 | 0x8000 (new) |

Masks seen: 0x3636 (12 voltages), 0x0101 (16 fans), 0x1818 (7 temps), 0x0000 (24). There is no 0x3030 or 0x0808 on the B650: unlike the X470, MB and card-side temps have **no thresholds**. Fans have **two tach inputs per header** (`FANn_1`/`FANn_2`), 7 headers.

### 8.3 Sensor inventory [2T] (51 sensors)
This board follows the AM4/X470 numbering with additions. Compared with X470 §8.5:
- **Same numbers and names:** 1–13 voltages (3VSB … 12V), 24/25 PSU VIN, 32/33 PSU IOUT, 49 MB Temp (UNC 55), 64–67 DDR4 temps, 96 FAN1, 144 ChassisIntr, 145 CPU_PROCHOT, 160/161/168/169 PSU status/AC lost.
- **Renamed or renumbered:**
  - 50 = `B550 Temp` (UNC 93); X470 50 = Card Side Temp, which moves to 51 here.
  - 52 = `TR1` (UNC 55, absent); X470 52 = CPU Temp, which moves to 54 here (UNC 95, UC 96).
  - DDR4 64–67 have A1/A2 and B1/B2 swapped vs X470.
  - Fans: 96–101 = FAN1–FAN6 contiguous (X470: 96, 98–102), plus 102–105 = FAN1_1–FAN4_1.
  - PSU PIN/POUT at 118/119/122/123 (X470 112/113/116/117).
  - CPU_THERMTRIP at 146 (X470 147).
- **New:** 57 TEMP_M.2_1, 92/93 PSU1/PSU2 Temp, 249 WATCHDOG2.
- Masks: 0x3636 (12), 0x3030 (VCPU), 0x0808 (5 temps), 0x1818 (5), 0x0101 (10 fans), 0 (18).
- The host was on, yet `VCPU` read **0.19 V**. That is plausible only in a low-power idle state. Worth a second sample before treating it as a fault.
- Between the two samples (25 s apart) only 5VSB, VCCM, 3V and CPU Temp changed (±1 raw count).

**Proxy implication:** sensor numbers and names are **board-specific, not family-specific**. Build the Redfish sensor IDs from `/api/sdr` or `/api/sensors` at runtime (key `sensor_number`, display `name`). Never hard-code maps per family.

### 8.4 `/api/dashboardsensors` [B650]
See §5.5. It is the same list as `/api/sensors`, slimmed to `{id, sensor_number, name}` except for abnormal sensors. The X470 UI had no equivalent.

### 8.5 `/api/sdr` [2T]
58 entries: the 51 sensors plus 7 SDR-only records: **207 `SEL` (event_logging_disabled, 16), 208–211 `DRAM ECC ErrorA1/A2/B1/B2` (memory, 12), 232 `PEF` (system_event, 18), 233 `Timestamp` (system_event, 18)**. The X470 had 1 extra (PEF). Record shape as X470 §5.15 plus **`owner_lun`** (int, 0). The DRAM ECC sensors exist only as SEL event generators. Map their SEL events to `Memory` health in Redfish. 1442 ms here was contention (§11.3). Cache per BMC boot.

---

## 9. Event log, dashboard events, audit

### 9.1 `/api/logs/eventlog?EVENTID=-1` [2T]
The UI sends `EVENTID = context.EventId - 1` with `EventId` initially 0, so `-1` means "all from the newest" **[JS]**. The UI pages further with `EVENTID` when it has >299 records and more exist, which implies a **server-side page size of about 300**, and incremental fetches use `LASTEVENTID` (15 s dashboard poll, commented out in parts). That differs from X470 §5.17 (no query, full dump).

Response: **62 elements = 61 records (newest first, id 61 → 1) + a trailing `{ "event_log_count": 61 }`**, 50.7 KB (~830 B/record). Field differences vs X470 §9:

| Change | Detail |
|---|---|
| New | `sensor_type_table` (int, = `sensor_type_num`), `record_type_table` (int, = `record_type_num`), `event_direction_table` (int, = `EventDirType`). These are lookup keys for the UI's string tables |
| Removed | `advanced_event_description`, `EventData{1,2,3}_Description_raw`, `pci_bus_num/pci_device_num/pci_function_num` |
| Changed | `event_description` now holds **human text** (`"Timestamp Clock Synch"`, `"Upper Non-critical - going high"`) instead of the X470 machine key (`upper_non_critical_going_high`). Do not match on it across boards; use `sensor_type_num` + `EventDirType` + `EventData1 & 0x0F` |
| Optional fields | None: all 30 fields present on all 61 records (X470 had optional ones) |

Content: 42 `Timestamp Clock Synch` (BIOS, `GenID1 1`) and 19 `temperature / Upper Non-critical going high`. Timestamps range from 106 s (pre-sync) to 2026-11-12, on a clock that is 8 h behind.

Go: decode the array as `[]json.RawMessage`, treat the element with `event_log_count` as the total, and model records with a struct.

### 9.2 `/api/logs/dashboardevent` (both)
See §5.6. This is the cheapest SEL change detector seen on this family: 61 records, ~7.7 KB, ids ascending, plus `pending_deassert`. The X470's `/api/asrr/selsummary` was not called on these boards (its handler exists in 9.05 **[FW §5.3]**; untested on 1.10.00).

### 9.3 `/api/logs/audit` (both)
See §5.8.

---

## 10. Host inventory comparison (`/api/host_inventory/*`)

| Endpoint | X470 | [B650] | [2T] |
|---|---|---|---|
| `host_interface_system_info` | not called | 1 record, mostly blank: `Name "System"`, `Description "System Self"`, `Model ""`, `IndicatorLED "Off"`, `Manufacturer " "`, `PowerState "On"`, `SerialNumber " "`, `PartNumber ""`, `SystemType "Physical"`, `State "Enabled"`, `AssetTag ""`, `BiosVersion "NA"`, `UUID "NA"` | not called |
| `host_interface_processor_info` | `[ ]` | `[ ]` | 1 record: `Id/Name "DevType1_CPU0"`, `Manufacturer "Advanced Micro Devices, Inc."`, `MaxSpeedMHz "4650"`, `Model "AMD Ryzen 7 5700X 8-Core Processor             , 4650 Mhz, 8 Core(s), 16 Logical Processor(s)"`, `ProcessorArchitecture "x86"`, `ProcessorType "CPU"`, `Socket "CPUSocket"`, `State "Enabled"`, `EffectiveFamily` (= model string, space-padded), `TotalCores "8"` |
| `host_interface_memory_info` | `[ ]` | `[ ]` | 4 records: `Id/Name "DevType2_DIMMn"` (order 2, 3, 0, 1), `CapacityMiB "32768"`, `Manufacturer` (`"Unknown"` x3, `"Micron Technology"`), `SerialNumber <REDACTED_SERIAL>` (8 hex), `PartNumber` (space-padded), `State "Enabled"`, `DeviceLocator "DIMM 0"`/`"DIMM 1"` (**duplicates: not unique, so use `Id`**), `OperatingSpeedMhz "2666"`, `Description "NA"`, `MemoryDeviceType "DDR4"`, `AllowedSpeedMHz "NA"` |
| `host_interface_pcie_device_function_info` | `[{PCIE_Devices:[],PCIE_Function:[]}]` | same (empty) | same (empty) |
| `host_interface_storage_info` | `[{Storage_Drives:null, Storage_Contoller:null}]` | not called | `Storage_Drives`: 2 NVMe (`Name "NVMe_Device0_NSID1"`, `SerialNumber <REDACTED_SERIAL>` (12 chars), `Manufacturer "N/A"`, `Location "NA"`, `Protocol "NVMe"`, `Model "CT1000P3SSD8"`, `Revision "NVMe Spec v1.4"`, `EncryptionStatus "Unlocked"`, `MediaType "SSD"`, `State "Enabled"`). `Storage_Contoller` (sic): 6 entries (`MemberId` "0".."5", `Name` `SATA_Controller_n`/`USB_Controllern`/`NVMe_Controller_n`, `SerialNumber`, `Model`, `FirmwareVersion` (`"P9CR30A"`, `"1.10"`, `"Not Available"`), `SpeedGbps` "3"/"6"/"8", `State`) |
| `host_interface_baseboard_info` | not called | `[{ baseboard_info:[{Name "Manager", Description "BMC", FirmwareVersion "1.10.0", Model <REDACTED_SERIAL> (11 digits), State "Enabled", PowerState "On"}], NetworkInterface:[4 x {id, Name, MACAddress, InterfaceEnabled ("true"/"false" strings), IPv4Addresses, HostName, State, FullDuplex, PermanentMACAddress, WWPN}] }]`. NICs: `usb0` (host interface, link-local `169.254.0.17`, locally administered MAC), `eth0`/`eth1` (disabled, bond slaves), `bond0` (enabled, BMC IPv4, `HostName <REDACTED_HOSTNAME>`) | not called |
| `host_interface_power_info` | not called | `[{ Powercontrol:[{id 0, Name "Chassis Power Control", Average/Max/MinConsumedWatts "0", IntervalInMinutes "0", LimitInWatts "500", LimitException "HardPowerOff"}], Voltages:[15 x {id, Name (= sensor name), MemberId (= sensor_number as string), State "Enabled"/"Absent", MinReadingRange, MaxReadingRange, Upper/LowerThreshold{Fatal,Critical,NonCritical}}] }]`. All strings, `"NA"` if unset. Thresholds equal `/api/sensors`. Order is not by sensor number | not called |
| `host_interface_thermal_info` | not called | not called | not called |

Observations:
- The field names are **Redfish property names** (`MaxSpeedMHz`, `CapacityMiB`, `LimitException`, ...). This is the BIOS-pushed Redfish host-interface inventory, flattened, so it maps almost 1:1 to `Processors`, `Memory`, `Storage/Drives`, `Power.PowerControl` and `Power.Voltages`. Parse every number with `strconv` (strings) and treat `"NA"`, `"N/A"`, `"Not Available"`, `" "` and `""` as null.
- [B650] processor/memory are empty even with the host on. Either the BIOS (2.17) did not push them, or the push happened before the last BMC reboot. [2T] has them. **This depends on board, BIOS and boot state, so the proxy must fall back** (IPMI FRU/SMBIOS) when the arrays are empty.
- `LimitInWatts "500"` with `HardPowerOff` is a power-limit setting. If it is enforced, a proxy exposing `PowerControl.PowerLimit` must treat it as a write that can power the host off.
- `Model` in `baseboard_info` is an 11-digit number, not a model name (likely a board or BMC serial). It was redacted as a precaution.

---

## 11. Timing, polling cadence and latency

### 11.1 Polling by the stock UI
- [B650] `chassis-status`: **5.00 s ± 0.01 s** timer. `/api/sensors`: 10 s timer, active only on `#sensors`/`#dashboard` routes **[JS]**. Here sensors were fetched once.
- [2T] `chassis-status` reached the 5 s cadence only ~18 s after login. `BIOS/UpdateOption`: once per route change (21x).
- Both: `/api/maintenance/flashtrack` on a 6 s timer exists in JS but was never called in these captures.
- There was no `configuration/runtime` per route (X470: 39 calls).

### 11.2 Latency (server wait, ms)

| Class | Endpoints | Uncontended | Contended (in bursts) |
|---|---|---|---|
| Fast | chassis-status, sol/session, getvetobit, fan table, fwimage_location, host_inventory/*, getbioscode, UpdateOption, network-bond GET, ipmi_disable_interfaces, services (401) | **73–135** | 400–1460 |
| Medium | dashboardinfo 143, media/adviser 132, restore_defaults 128, system_admin 130, getmcalog 157, dashboardevent 137–389, sensors 232 (2T alone), audit?cmd=1 306, network 372 | 130–400 | 600–1200 |
| Slow | **login 360 [B650] / 3509 [2T]**, settings/users 1266, sensors 751 (B650, burst), eventlog 1278 + 309 receive, sdr 1442 (contended), project 963, date-time 1131, **PUT network-bond 1105** | 0.4–3.5 s | |

The uncontended floor (~75–90 ms) is about the same as the X470 (68–84 ms).

### 11.3 Contention evidence (serialised backend)
- [B650] 06:28:04.27: 7 calls issued together. Waits: `logs/audit` 306, `dashboardevent` 389, `network` 372, `dashboardsensors` 421, `uptime` 544, `fw-info` 556, `model-name` **829** ms. The concurrent `chassis-status` (normally 84) waited 564 ms.
- [B650] 06:28:19.67: `sensors` (751 ms) overlapped `chassis-status`, which waited **689 ms**.
- [2T] 06:29:22.89: 10 calls. Waits ranged from 137 to 1175 ms, and Chrome queued up to 2.6 s on top (6-connection limit), so totals reached 2.8 s.
- [2T] 06:29:29.8: `eventlog` + `sdr` + `UpdateOption` in parallel took 1278/1442/**1457** ms. `UpdateOption` alone is 75 ms.
- This matches `/api` FastCGI `max-procs 1` **[FW §5.1]**. **Recommendation unchanged from X470 §10.4:** one session, a per-BMC queue (concurrency 1), 6–10 s timeouts, and northbound reads served from cache.

### 11.4 Suggested refresh/TTL (deltas vs X470 §10.4)
- Power state: `chassis-status` every 5–10 s (not `/api/chassis`).
- Health summary: `dashboardsensors` (cheap abnormal-only detail) every 10–30 s, and full `sensors` every 30–60 s.
- SEL change detection: `dashboardevent` (or `selsummary` if present) every 30–60 s. Fetch `eventlog?EVENTID=-1` only on change.
- Inventory (`host_inventory/*`, `fw-info`, `model-name`, `sdr`): at startup, hourly, and after host power-on.
- `services`, `ipmi_disable_interfaces`, `restore_defaults`, `vetobit`, `BIOS/UpdateOption`: every 5–15 min. Check `UpdateOption` **immediately before any power-off action** (§7.4).

---

## 12. Error responses observed

| Capture | Request | Status | Body | Notes |
|---|---|---|---|---|
| both | `GET /api/settings/services` (pre-login, `X-CSRFTOKEN: null`) | **401** | `{ "cc": 7, "error": "Invalid Authentication" }` | Normal API headers; no `WWW-Authenticate`. JS constant `CC.UNAUTHORIZED = 7` |
| [B650] | `GET /res/oem/h5banner_{left,right}.png` | 404 | 345-byte XHTML "404 - Not Found" | OEM banner override absent (cosmetic) |
| [2T] | `/fonts/*.ttf`, `*.woff2`, `footable.*`, `/res/oem/*.png` | 404 | same page (HAR stored 0 bytes) | Browser font fallbacks |

No 403, 500 or 503 occurred. The X470's `asrr/product-name` 500 was not reproduced because it was not called. JS error codes are as on the X470, plus `LOGINERR_PAM_PERM_DENIED 15010`, `FIRMWARE_UPDATE_IS_IN_PROGRESS 17000`, `RAID_HOST_POWER_DOWN_STATE 11287`, `RAID_INFO_NOT_AVAILABLE 1366`, HTTP `LARGE_FILE_SIZE 523` (X470 build: 12012) and `LICENSE_NOT_AVAILABLE 524`. lighttpd's `mod_evasive` answers excess concurrent connections (>30 per IP) with **HTTP 403** **[FW §3.2]**, so a 403 from this family can mean "too many connections" rather than "no privilege".

---

## 13. Cross-board comparison (X470D4U vs B650D4U vs B550D4U-2T)

### 13.1 Endpoints seen

| Endpoint | X470 | B650 | 2T |
|---|---|---|---|
| POST /api/session | ✔ | ✔ | ✔ |
| GET /api/settings/services | – | ✔ (401 + 200) | ✔ (401 + 200) |
| GET /api/settings/users | ✔ | – | ✔ |
| GET /api/settings/dashboardinfo | – | ✔ | – |
| GET /api/firmware-info | ✔ | – (folded into dashboardinfo) | – |
| GET /api/configuration/project | ✔ | – | ✔ |
| GET /api/configuration/runtime | ✔ (39) | – | – |
| GET /api/chassis | ✔ (31, 5 s poll) | – | – |
| GET /api/chassis-status | ✔ (5) | ✔ (13, 5 s poll) | ✔ (3) |
| GET /api/sensors | ✔ | ✔ | ✔ |
| GET /api/dashboardsensors | – | ✔ | – |
| GET /api/sdr | ✔ | – | ✔ |
| GET /api/logs/eventlog | ✔ (no query) | – | ✔ (`EVENTID=-1`) |
| GET /api/logs/dashboardevent | – | ✔ | ✔ |
| GET /api/logs/audit | – | ✔ (`cmd=1`) | ✔ (full) |
| GET /api/logs/video, settings/video/remote-storage | ✔ | – | – |
| GET /api/status/uptime | – | ✔ | ✔ |
| GET /api/settings/network | ✔ | ✔ | ✔ |
| GET /api/settings/network-bond | ✔ | ✔ | ✔ |
| PUT /api/settings/network-bond | – | ✔ | – |
| GET /api/settings/static-ipv6 | ✔ | – | – |
| GET /api/settings/date-time | ✔ | – | ✔ |
| GET /api/settings/media/adviser | ✔ | ✔ | – |
| GET /api/settings/ipmi_disable_interfaces | – | – | ✔ |
| GET /api/sol/session | – | ✔ | – |
| GET /api/fru | ✔ | – | – |
| GET /api/host_inventory/system_info | – | ✔ | – |
| GET /api/host_inventory/processor_info, memory_info | ✔ (empty) | ✔ (empty) | ✔ (data) |
| GET /api/host_inventory/pcie_device_function_info | ✔ (empty) | ✔ (empty) | ✔ (empty) |
| GET /api/host_inventory/storage_info | ✔ (null) | – | ✔ (data) |
| GET /api/host_inventory/baseboard_info, power_info | – | ✔ | – |
| GET /api/maintenance/fwimage_location | ✔ | ✔ | – |
| GET /api/maintenance/signed-image-key | ✔ | – | – |
| GET /api/maintenance/restore_defaults, system_admin | – | ✔ | – |
| GET /api/asrr/model-name | ✔ | ✔ | ✔ |
| GET /api/asrr/product-name | ✔ (500) | – | – |
| GET /api/asrr/fw-info | ✔ | ✔ | ✔ |
| GET /api/asrr/selsummary | ✔ | – | – |
| GET /api/asrr/getmcalog | ✔ | – | ✔ |
| GET /api/asrr/PowerSource, GetFanDuty | ✔ | – | – |
| GET /api/asrr/getvetobit | – | ✔ | – |
| POST /api/asrr/settings/setopenlooptablenumber, GET getfanopenloopcontroltable | – | ✔ | – |
| GET /api/asrr/getbioscode | – | – | ✔ |
| GET /api/asrr/maintenance/BIOS/UpdateOption | – | – | ✔ (21) |
| POST /api/actions/power | ✔ | – | – |
| HEAD /bios/Index.html | – | – | ✔ (200) |

"–" means not called in that capture, not that it does not exist.

### 13.2 Field differences in shared endpoints

| Endpoint | X470 (fw 2.32.00) | B650D4U (fw 1.10.00) | B550D4U-2T (fw 1.10.00) |
|---|---|---|---|
| `POST session` request | `username, password, certlogin=0`; no CSRF header | `username, password`; `X-CSRFTOKEN: null` | same as B650 |
| `POST session` response | 9 fields | + `user_id`, `channel`, `passwordStatus` | same as B650 |
| Login latency | 1491 ms | 360 ms | 3509 ms |
| Unauth error | not observed | 401 `{"cc":7,"error":"Invalid Authentication"}` | same |
| CSRF storage (UI) | localStorage `garc` | **cookie `garc`** | localStorage `garc` |
| `chassis-status` | `power_status, led_status` | + `power_cycle_interval` (ms) | as X470 |
| `sensors` | `settable_readable_threshMask`; unset = 0.0; floats; `°C` | `settable_flag`; `"NA"`; int/float; **`deg_c`**; new naming; 59 sensors | `settable_flag`; `"NA"`; int/float; `°C`; AM4 naming; 51 sensors |
| `sdr` | `id, sensor_number, name, type, type_number` (42) | – | + `owner_lun` (58) |
| `eventlog` | full dump, optional fields, machine-key `event_description` | – | `EVENTID` paging, + `*_table` fields, human-text `event_description`, trailing `{event_log_count}` |
| `settings/users` | 10 slots, `network_privilege`, `privilege_limit_serial` | – | 30 rows (15 x 2 channels), `privilege`, `*ByChannel`, `password_age/enddate` |
| `settings/network` | 2 entries (eth0 ch1, eth1 ch8) | 1 entry `bond0` ch1 | 1 entry `bond0` ch1 |
| `network-bond` | `bond_enable 0` | `bond_enable 1`; PUT adds `bond_ifc` | `bond_enable 1` |
| `date-time` | `auto_date`, `daylight`, tz `GMT+0` | – | `ntp_auto_date` (0), no `daylight`, tz `Etc/GMT` |
| `media/adviser` | `kvm_port 7578`, 10 fields | `kvm_port 65535`, + `kvm_client` | – |
| `asrr/model-name` | `mb_name` | + `sys_name` | + `sys_name` |
| `asrr/fw-info` | BMC, BIOS, ME, Micro_Code, CPLD | + `PSP_fw_version`, `BP_version`; ME `N/A` | + `PSP_fw_version` (no BP) |
| `configuration/project` | 54 | – | 83 (adds BIOS_FW, ASRR_FAN_TABLE, VNC, UNIFIED_USER_ACCOUNT, ...) |
| `host_inventory/*` | all empty or null | system/baseboard/power populated; CPU/DIMM empty | CPU/DIMM/storage populated |
| API security headers | no HSTS/CSP/Referrer-Policy | HSTS, CSP, Referrer-Policy, XFO | same as B650 |
| Unknown static path | 200 + SPA index | **404** | **404** |
| `Referer` sent by UI | yes | no | no |
| Float formatting | `3.400000` | `3.3399999999999999` | same as B650 |
| BMC clock vs client | −5 s | **+5 h** | **−8 h** (NTP off) |

---

## 14. What the HARs do NOT cover (capture next)

1. **A real B650D4U_2L2T**, plus confirmation of which box `b650d4u-2t.har` came from (it reports B550D4U-2T).
2. **Current firmware.** Re-capture on BMC 7.02.00 / 9.05.00 (the versions in `docs/firmware/b650d4u.md`), since 1.10.00 may differ.
3. **Cookies:** export "with sensitive data" to get `Set-Cookie: QSESSIONID` attributes, and the stale cookie that triggered the pre-login 401.
4. **Logout** (`DELETE /api/session`), expired-session behaviour after `time_out` 1800 s, wrong-CSRF (does it also return 401 cc 7?), bad credentials, and max sessions (20, code 15000).
5. **B650D4U network state after the bond PUT** (`GET /api/settings/network`, `network-bond`), and the semantics of `bond_ifc` when disabling or enabling bonding.
6. **Power control on the B650 family:** `POST /api/actions/power` (codes 0/1/2/3/5), `power_cycle_interval` effect, UID `POST /api/actions/chassis-led`, and whether `/api/chassis` exists.
7. **Fan control:** `setfanopenloopcontroltable` (body type: strings vs ints), default tables (`table_numble | 0x80`), `getfancontrolmode`/`setfancontrolmode`, manual duty, `fan_tscft` sensor→table mapping, close-loop tables, and `GetFanDuty` (absent from both captures).
8. **Vetobit write** (`PUT /api/asrr/setvetobit`) and its effect on shared-NIC reachability during host reset.
9. **BIOS update flow:** `BIOS/UpdateOption` with `action 1`, `BIOS/status` codes, `flash-progress`, `BIOS/upgrade`; BMC firmware update; `maintenance/flashtrack`.
10. **[B650] `settings/users`, `configuration/project`, `date-time`, `sdr`, `eventlog`, `fru`, `selsummary`, `getbioscode`, `getmcalog`**, none of which were called on the B650D4U.
11. **[2T] `dashboardinfo`, `dashboardsensors`, `system_info`/`baseboard`/`power` inventory, fan pages**, to see whether the differences are firmware or UI-build differences.
12. **Host inventory with data on the B650** (processor/memory empty), `host_interface_thermal_info`, and PCIe (empty on all three boards).
13. **Event log paging:** behaviour with >300 records, `EVENTID=n` and `LASTEVENTID=n` semantics, and the SEL clear endpoint.
14. **Redfish:** `GET /redfish/v1/` on both boards (the UI never calls it, but `REDFISH` flags, the `redfish` restore key and `/bios/Index.html` all indicate it exists), and comparison of Redfish `Sensors`/`Processors` with `/api`.
15. **TLS** (`openssl s_client`: protocol, ciphers, whether the default AMI cert/key is in use, **[FW §8.3]**), and IPMI-over-LAN reachability (`ipmi_over_LAN 1` on 2T).
16. **Concurrency:** a controlled N-parallel test against the 30-connection `mod_evasive` limit (403).

---

## 15. Redaction log

| Item | Where | Placeholder | Original format |
|---|---|---|---|
| Password | `POST /api/session` form body (both) | `<REDACTED_PASSWORD>` | [B650] 16 chars `[A-Za-z0-9]`; [2T] 10 chars `[A-Za-z0-9]`. **Cleartext in both raw HARs** |
| CSRF token | login `CSRFToken`; `X-CSRFTOKEN` on 45 [B650] / 50 [2T] requests | `<REDACTED_CSRF>` | 8 chars `[A-Za-z0-9]`, constant per session. The pre-login literal `null` is kept |
| Session id | login `racsession_id` | `<REDACTED_SESSION_ID>` | small integer: [B650] 1 digit, [2T] 2 digits |
| Session cookie | not present in either HAR | `QSESSIONID=<...>` | unknown |
| MAC addresses | `settings/network.mac_address`; `baseboard_info.NetworkInterface[].MACAddress/PermanentMACAddress` | `<REDACTED_MAC>` | `XX:XX:XX:XX:XX:XX` uppercase hex. BMC NICs use the ASRock OUI; `usb0` uses a locally administered MAC |
| BMC hostname | `baseboard_info...HostName`; `logs/audit[].hostname` | `<REDACTED_HOSTNAME>` | `AMI` + 12 uppercase hex (MAC without colons), 15 chars |
| Serial numbers | DIMM `SerialNumber` (8 hex), NVMe drive/controller `SerialNumber` (12 alnum), `baseboard_info.Model` (11 digits, serial-like) | `<REDACTED_SERIAL>` | as noted |
| Non-stock username | `logs/audit` message text [2T] (`user:...`) | `<REDACTED_USER>` | lowercase + digits with underscores, 12 chars |
| BMC host in headers | `Origin`, page URL | `<BMC_HOST>` | RFC1918 IPv4 |
| IP addresses | BMC `10.3.10.128` / `10.6.10.75`, clients `10.3.10.252` / `10.6.10.250`, gateways `.1`, audit-log `172.18.0.2`, `172.29.0.5/6`, host-interface `169.254.0.17` | kept | RFC1918 or link-local only; **no public IPs** appear in API traffic |
| Usernames kept | `admin`, `anonymous`, `Administrator`, `sysadmin` | kept | stock or factory accounts |
| UUIDs | `system_info.UUID` = `"NA"` | n/a | none present |
| RSA private key in UI JS | `source.min.js` in both HARs (commented `setPrivateKey(...)`) | not reproduced | PEM `BEGIN RSA PRIVATE KEY`; static, ships with firmware |

Final leak check: see §16.

---

## 16. Method
Python 3 scripts run with `python3 -I`, kept in the session scratchpad, parsed both HARs read-only:
- `overview.py`: entry table.
- `compact.py`: per-API-entry headers and bodies.
- `analyze.py`: per-endpoint counts, intervals, timings, field types.
- `sensors.py`, `details.py`, `details2.py`: sensor, SDR, event log, user, feature and inventory diffs against `harfiles/x470d4u.har`.
- `sens.py`: IP/MAC/hostname inventory and `Date`-header clock skew.
- `gen_json.py`: redacted `har-endpoints.json`, with an assert that no collected secret remains.

Each HAR's bundled `source.min.js` was extracted as text and searched with `jsgrep.py`. Findings from it are marked **[JS]**. No binary was disassembled and no BMC was contacted. A final `grep` over both output files for the original passwords, CSRF tokens, MACs, hostnames, serials, the audit username, numeric `racsession_id` values, UUID patterns, MAC patterns and non-RFC1918 IPv4 addresses returned no matches. The only PEM marker hit is the descriptive text in the §15 table, and no key material is reproduced.
