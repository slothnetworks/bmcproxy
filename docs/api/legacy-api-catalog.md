# Legacy AMI `/api` REST catalog (ASRock Rack MegaRAC SPX)

Preliminary assessment input for the native Go proxy. On boards without usable Redfish (X470D4U), the proxy drives the same legacy `/api` interface that the BMC web UI uses. This document catalogs that interface from the UI's point of view.

**Sources** (read-only; JavaScript read, no binaries disassembled):

| Tag | Board / firmware | UI bundle |
|---|---|---|
| X470-3.02 | X470D4U BMC 3.02 | `scratchpad/fw/x470/www_plain/source.min.js` (5.4 MB), `viewer.min.js` |
| B650-7.02 | B650D4U BMC 7.02 | `scratchpad/fw/b650_702/www_plain/source.min.js` (8.0 MB), `viewer.min.js` |
| B650-9.05 | B650D4U_2L2T BMC 9.05 | `scratchpad/fw/b650_905/www_plain/source.min.js` (8.0 MB), `viewer.min.js` (identical to 7.02) |

Validation sources:
- Live captures: `harfiles/x470d4u.har` (X470D4U, BMC fw **2.32.00**), `harfiles/b650d4u.har` (B650D4U, BMC fw **1.10.00**) and `harfiles/b650d4u-2t.har` (the board reports `mb_name` **B550D4U-2T**, BMC fw **1.10.00**). None of the three captures runs the firmware analysed here, so a capture confirms a request/response shape but cannot prove that an endpoint exists in 3.02/7.02/9.05. The X470 HAR analysis is in `docs/x470/har-analysis.md`. Every `/api` call in all three captures (32, 28 and 24 distinct method+path pairs) was compared field by field with the JS-inferred shapes; see section 6.3 and the per-endpoint "Live" lines.
- Handler inventories: `strings -n 4` of every `*restservice*` binary and library in each image's `firmware-dumped/<fw>/rootfs` (`spx_restservice`, `spx_restservice_ext`, `libspx_restservice*.so`, `libasrrspx_restservice.so` on B650, the hostinterface/sol/vnc plugins). Route strings are matched literally (with or without the leading `/`) and as POSIX-regex item routes such as `/settings/users/([[:digit:]]+)`. This was cross-checked against `docs/firmware/x470d4u.md` §5.2 and `docs/firmware/b650d4u.md` §5.3 / Appendix A. 7.02 is checked against its own image, not derived from 9.05; the result matches the documented 7.02/9.05 diff (7.02 lacks `asrr/maintenance/{CPLD,FPGA,RETIMER}/*` and has `asrr/PowerSourceID` and `logs/platform`).

Companion machine-readable file: `docs/api/legacy-api-endpoints.json`. It is an array of `{path, methods, boards[], area, request_fields, response_fields, item_path?, item_methods?, query_params?, validation?, allowed_values?, handler_present{board: yes|no|unknown}, seen_in_har{"x470d4u.har"|"b650d4u.har"|"b650d4u-2t.har": ["METHOD status", …]}, har_observed?{har: [{method, status, query_keys?, request_content_type?, request_shape?, response_shape?}]}, har_vs_js?, response_fields_source?, response_fields_js_inferred?, ui[], js_modules?, notes?}`. Shapes contain only key names and JSON types, never captured values.

How the catalog was produced. The bundles are almond/r.js builds: every module is `c("name",[deps],function…)` on X470 and `l("name",…)` on B650. Scripts in `scratchpad/api-agent/` (`split.py`, `extract.py`, `enrich.py`, `gen.py`) split them into modules and found every `"/api/…"` literal along with its enclosing `$.ajax({...})` or `Backbone.Model/Collection.extend({...})`. For each one they recorded `type`, `data`/`JSON.stringify` keys, `FormData.append` names, the success-callback fields, Backbone `defaults`/`validation`, and the `.get()`/`.set()`/`.save()` keys of the views that depend on the model. They also pulled `<select>`/radio option values from the views' templates. Key flows (session, power, KVM, virtual media, all firmware-update paths) were then read by hand and checked against the captures. The field lists are what the **UI reads or writes**, so the server may return more. Where a view has a generic `save()` handler (AMI boilerplate) but no visible save button, a `PUT` can show up for a read-only page (example: `/api/firmware-info`).

Counts: **416 distinct paths**: X470 3.02 references 224, B650 7.02 references 370 and 9.05 references 369 (counts include the dynamic HPM/RAID paths and the viewer-only calls). Only one capture endpoint (`/api/asrr/maintenance/BIOS/UpdateOption`) is referenced by none of the analysed bundles.

---

## 1. Client framework conventions

### 1.1 Stack
- RequireJS/almond + jQuery + Backbone + Backbone.LayoutManager + backbone-validation; Underscore templates (`text!templates/...html`); i18n bundles (`strings/nls/<page>` on X470, i18next `:<page>` namespaces on B650).
- Hash routing (`router.js`, `auto_router.js`): `#dashboard`, `#power-control`, `#settings/...`, `#maintenance/...`, `#remote-control`, etc. Each route renders one `views/...` module. The UI column in the catalog gives that module.
- Base URL: same origin, absolute paths `"/api/..."` (a few are relative `"api/..."`, for example the B650 10 s sensor poll uses `"api/sensors"`). No API version prefix. There is no `urlRoot` anywhere; models use `url:"/api/x"` or `url:function(){return "/api/x"}`.

### 1.2 Backbone method mapping (what the proxy must emulate)
| UI action | HTTP | URL |
|---|---|---|
| `model.fetch()` / `collection.fetch()` | `GET` | model/collection `url` (optionally `?`+`$.param(data)`, e.g. `GET /api/logs/event?LASTEVENTID=123`) |
| `model.save(attrs)` on a singleton settings model | `PUT` | the model `url`. Singletons always carry `"id":1` in the GET response, so Backbone `isNew()` is false and **save is always PUT, never POST**. The body is the **whole model** as JSON (`Content-Type: application/json`), not just the changed fields. HAR-confirmed: `PUT /api/settings/network-bond` with `{"id":1,"bond_enable":0,"bond_mode":"active-backup","auto_configuration_enable":1,"bond_ifc":"eth0"}`; the response echoes the object. |
| `model.save()` on an item of a collection | `PUT` | `collection.url + "/" + id`, e.g. `PUT /api/settings/users/3`, `PUT /api/settings/network/1`, `PUT /api/settings/pef/event_filters/5`. The catalog lists these as **Per-item**. |
| `model.destroy()` | `DELETE` | `collection.url + "/" + id` |
| `collection.create()` | `POST` | collection `url` (scripts, tasks only) |
| explicit `$.ajax` | as written | listed per endpoint |

Fixed-slot tables (users, LDAP/AD role groups, PEF filters/policies/destinations, firewall rules) are pre-allocated server-side. "Add" is a `PUT` on an empty slot id and "delete" is a `DELETE` on the slot (users: `DELETE /api/settings/users/:id` with JSON body `{"snmp_status":…, "id":…}`).

### 1.3 Body encodings (important for the proxy)
- `$.ajax({data:{...}})` without `contentType` is sent as **`application/x-www-form-urlencoded`**. Used by `POST /api/session`, `/api/asrr/session`, `/api/viewerlogin`, `/api/reset-pass`, `/api/verify_otp`, `/api/updatenew_password`. HAR-confirmed on all three boards: `username=admin&password=…&certlogin=0` on X470, without `certlogin` on B650.
- `contentType:"application/json"` + `JSON.stringify({...})` is a JSON body (most actions and settings).
- `contentType:"application/json"` + `data:{}` is **an empty body** with a JSON content type (jQuery serialises `{}` to `""`). Used by `PUT /api/maintenance/flash` (X470), `POST /api/maintenance/reset`, `PUT /api/asrr/bmccoldreset`.
- `contentType:false, processData:false, data:FormData` is `multipart/form-data` (all uploads). The field names are in the catalog as `multipart:<name>`.
- ASRR OEM update calls (`/api/asrr/maintenance/<C>/configuration|upgrade`) send `JSON.stringify({action:N})` **with `contentType:false`**. The body is a JSON string with no explicit JSON content type (the XHR default for a string, `text/plain;charset=UTF-8`). The proxy should send the same raw JSON.
- B650 `appext.rsaencrypt()` can wrap sensitive payloads as `{"endata": RSA(JSON)}`, but `window.encrypt_sensitive_data = 0` in both B650 builds, so it is a no-op.
- Error responses are JSON `{"error": "...", "code": N}`. The HAR has `GET /api/asrr/product-name` → 500 `{"error":"Error in Getting Model Name","code":16005}`. `HTTPErrorHandler` maps `code` to the i18n key `error_code_<code>`.

### 1.4 Authentication, CSRF and cookies
1. **Login:** `POST /api/session` (form) `username`, `password` (X470 also `certlogin=0`).
   Response (HAR, X470): `{"ok":0,"privilege":4,"extendedpriv":259,"racsession_id":9,"remote_addr":"…","server_name":"…","server_addr":"…","HTTPSEnabled":1,"CSRFToken":"<8-char token>"}`.
   B650 adds `user_id`, `channel` and `passwordStatus` (1 = must change password, which goes to `POST /api/updatenew_password`), and with two-factor auth enabled also `TFAEnabled`/`TFAStatus` (then `POST /api/verify_otp`).
   - `privilege`: 1 callback, 2 user, 3 operator, 4 administrator, 5 OEM (`window.CONSTANTS`). `extendedpriv` bit0 = KVM allowed, bit1 = VMedia allowed (259 = 0x103).
   - Failure: HTTP error with `code` 15000 = `MAX_NUM_SESSIONS_ALREADY_IN_USE`; anything else is shown as "invalid username or password".
2. **`QSESSIONID` cookie:** the session cookie is set by the BMC on the login response (the JS never writes it, it only deletes it with `document.cookie="QSESSIONID=1;expires=Thu, 01 Jan 1970…"` on logout, on 401 and on the login page). The captures have cookies stripped. The browser sends it on every `/api/*` call and on the `/kvm` and `/cd-server` WebSocket upgrades.
3. **CSRF:** every later request carries **`X-CSRFTOKEN: <CSRFToken>`** (set globally with `$.ajaxSetup({headers:{"X-CSRFTOKEN":…}})`; raw `XMLHttpRequest` downloads set it by hand). HAR-confirmed on all requests after login.
4. **What `garc` is:** it is only the UI's storage name for the CSRF token, not a server cookie with its own meaning.
   - X470 3.02: `localStorage.garc` and `sessionStorage.garc` = `CSRFToken` (`views/LoginView` success handler; `app.initialize` re-reads `localStorage.getItem("garc")` for `$.ajaxSetup`). Other stored items: `privilege_id`, `extended_privilege`, `session_id` (= `racsession_id`), `kvm_access`, `vmedia_access`, `features`.
   - B650: a JS-written **cookie** `garc` (named `__Host-garc; path=/; secure; SameSite=Lax` when served over HTTPS) with a 2-day expiry, written through `app.setCookie("garc", CSRFToken, 2)` and read through `app.getCookie("garc")`. Companion cookies `TFAEnabled`, `TFAStatus`, `fw_flash_state`, `isActiveKVM`, `selected_lang`/`lang`.
   - Both: `refresh_disable=1` is written after login and decides whether a later 401 shows the "session expired" alert.
   - A proxy needs to keep only `QSESSIONID` + `CSRFToken`. It must not forward `garc` to the BMC (harmless if it does).
5. **Logout:** `DELETE /api/session` (no body; X470 sends it `async:false`). X470 also sends it from `window.onbeforeunload` unless a settings reload, KVM or firmware update is in progress.
6. **Other session entry points:** `POST /api/asrr/session` (X470 client-certificate login; form `username=CERT_ADMIN&password=&certlogin=1`, used from `index_trusted.html` on the mutual-TLS port 4433). `POST /api/viewerlogin` (form `token=<…>`; lets `viewer.html#<token>` open a KVM window by itself). `GET /api/newtabinfo` (B650: rebuilds session info for a second tab that already has the cookie). `GET /api/settings/dashboardinfo?user_id=&channel_id=&user_name=` (B650 post-login user details).

### 1.5 Session expiry, 401 handling
- `HTTPErrorHandler` (`cc.js` codes: `HTTP.UNAUTHORIZED 401`, `NO_PRIVILEGE_ACCESS 403`, `NOT_FOUND 404`, `FEATURE_NOT_ENABLED 405`, `SERVER_ERROR 500`, `LARGE_FILE_SIZE 12012`, `CUSTOM_STATUS_MSG 519`, `KVM_TOKEN_GEN_FAILED 1000`, `MAX_NUM_SESSIONS 15000`, `RAID_HOST_POWER_DOWN_STATE 1365`).
- **401:** clear local/session storage, expire `QSESSIONID` (and `refresh_disable`), go to `#login`, `location.reload()`. Show "session expired" if `refresh_disable==1`.
- 403 → "non-privileged user" alert, redirect to the section root. 405 → "feature not enabled".
- Timeout or status 0 → probe `POST <host>` with a 6 s timeout. If that also fails, treat it as BMC loss and force a re-login (skipped while a KVM session with `KVM_SESSION_RECONNECT` is active).
- B650 `appext` adds a **client idle timer**: 1800 s, replaced by `GET /api/settings/services` → `[0].time_out` (the web service timeout) and reset on key/mouse input. On expiry it alerts and goes to `#login` unless KVM is active. Server-side, the session times out after the web service `time_out` (default 1800 s) of inactivity.

### 1.6 Polling and keepalive
There is no dedicated keepalive endpoint. Any authenticated call refreshes the session. The UI polls:
| Timer | X470 3.02 (`app.setPolls`) | B650 (`app.setPolls`/`initPollingEventHandlers`) |
|---|---|---|
| 5 s | `GET /api/chassis` (`models/chassis_status_poll`), header power/UID icons, `global:false`, 6 s timeout | `GET /api/chassis-status` (App.Chassis) |
| 10 s | on `#dashboard`/`#sensors`: `GET /api/sensors`; on `#sensors/<n>`: `GET /api/detail_sensors_readings` | same (re-armed `setTimeout`) |
| 15 s | (handler empty) | `GET /api/logs/event?LASTEVENTID=<max id>` (new-SEL notifications) |
| 6 s | n/a | during BMC flash: `GET /api/maintenance/flashtrack`. When this fails the BMC is rebooting, so the UI waits 120 s and goes to the login page. |

A header toggle (`#live-poll`) stops and starts these timers. **Proxy recommendation:** poll `GET /api/chassis-status` (both families; `/api/chassis` is X470-only) at an interval below the web service `time_out` to keep a pooled session alive, and re-login on any 401.

### 1.7 Feature gating
After login the UI fetches `GET /api/configuration/project` (static `[{"feature":"…"}]` list, e.g. `NWLINK`, `SYSTEM_FIREWALL`, `IPV6_SUPPORT`, `FIRMWARE_FLASH_SUPPORT`, `SECTIONFLASH`, `VERSION_CMP_FLASH`, `ENC_IMAGE`, `PRESERVECONF`, `KVM_SESSION_RECONNECT`, `CD_SERVER_APP`, `RUNTIME_LICENSE` …) and `GET /api/configuration/runtime` (`[{"feature":"LDAP","enabled":0},…]`). B650 also fetches `GET /api/extfeatures` (`{FEATURE: bool}`). DOM elements with `data-feature` / `data-not-feature` / `data-runtime-feature` / `data-license` are hidden accordingly. The B650 bundle is a superset gated this way, which is why about 150 B650 UI endpoints have no handler in the shipped firmware (see the handler column).

---

## 2. Power control and chassis status

**Read:** `GET /api/chassis-status` (both families; `models/chassis_status`, also `models/uidcontrol`) → `{"power_status":0|1,"led_status":0|1|2}`, plus `"power_cycle_interval":2000` on B650 (HAR). X470 also has `GET /api/chassis` with the same two fields (the 5 s header poll).
- `power_status`: 0 = off, 1 = on.
- `led_status` (UID): 0 = off, 1 = blinking, 2 = on.

**Act:** `POST /api/actions/power`, JSON `{"power_command":N}` (`models/chassis_status.power()`, `views/PowerControlView`, template `power-control.html` radios `name="power_command"`). The response echoes `{"power_command":N}` (HAR).
| `power_command` | Label (i18n) | UI expects `power_status` afterwards | UI wait before re-check |
|---|---|---|---|
| 0 | Power Off (immediate, hard) | 0 | 10 s |
| 1 | Power On | 1 | 10 s |
| 2 | Power Cycle | 1 | 20 s (B650: `power_cycle_interval` + 10 s) |
| 3 | Hard Reset | 1 | 10 s |
| 5 | ACPI shutdown (orderly / soft) | 0 | 30 s |
| 6 | B650 only: "Boot to BIOS setup". **Not sent**; the UI does `POST /api/actions/accessbiossetup` (no body), then sends 1 if the host is off or 2 if it is on | 1 | as above |

(4 is not used; IPMI "pulse diag" is not exposed.) The UI re-reads chassis-status up to 3 times. The page is administrator-only (`privilege_id < ADMIN` disables the form). The H5Viewer power menu sends the same actions over the KVM WebSocket (`CMD_POWER_CTRL_REQUEST`) instead of REST.

**UID LED:** `POST /api/actions/chassis-led`, JSON `{"uid_command":N}` with 0 = off, 1 = blink, 2 = on (`views/miscellaneous/uidcontrolView`, both families). B650 `models/chassis_status.led(blink_time, force_on)` sends `{"blink_time":…,"force_on":…}` (IPMI Chassis Identify semantics).

Related: `GET /api/asrr/PowerSource` (PSU PMBus telemetry array), `GET/POST /api/asrr/{Get,Set}PowerRestore` (B650 `{"powerRestore":…}`), `PUT /api/asrr/bmccoldreset` (BMC cold reset, empty body), `GET/PUT /api/asrr/{get,set}prochot` (X470 `{"prochot":…}`).

---

## 3. Firmware update flows (step by step)

All update pages set `window.fw_upgrade=1`, block navigation and, if the tab is closed during an update, send `POST /api/maintenance/reset` from `onbeforeunload`.

### 3.1 BMC firmware, X470 3.02 (`views/maintenance/FirmwareUpgradeView`, template `fwimage_upgrade.html`)
Before starting: `GET /api/maintenance/fwimage_location` (`{"id":1,"protocol_type":"http"|"tftp","server_address","image_name","retry_count"}`) and, with feature `ENC_IMAGE`, `GET /api/maintenance/signed-image-key` (`{"id":1,"key_exists":0,"public_key_info":""}`).

HTTP (web upload) path:
1. Optional signing key: `POST /api/maintenance/signed-image-key`, multipart field **`publicKey`**, then `GET /api/maintenance/ValidSignImage` (`public_key_info`).
2. Client check: the file extension must be `.ima`.
3. `PUT /api/maintenance/flash`, `Content-Type: application/json`, **empty body**. Enters update mode (the BMC stops services and reserves memory).
4. `POST /api/maintenance/firmware`, multipart field **`fwimage`** = the `.ima`. Upload progress comes from XHR `upload.onprogress` and is shown as 0-50% of the bar. The UI **re-posts the upload every 90 s** until one succeeds (`httpsInterval`).
5. `GET /api/maintenance/firmware/verification` returns an array of sections: `[{current_image_name, current_image_version1, new_image_version, section_status, verification_status}, …]`. `verification_status` is a bitmask on element 0 (16 = full flash required; see the catalog for the other bits).
   - On error `code` 14540/14543/14545/14549/14551/14552/14553 (image/signature/platform mismatch) the UI alerts, calls `POST /api/maintenance/reset` and logs out after 120 s.
   - With feature `SECTIONFLASH` the user picks sections. Otherwise the UI shows current/new versions and "Proceed".
6. `PUT /api/maintenance/firmware/upgrade`, JSON `{"preserve_config":0|1,"preserve_network":0|1,"preserve_user":0|1,"flash_status":1|2[, "section_count":N, "section_names":[…]]}` (1 = full flash, 2 = selected sections). The checkboxes `#idpreserveAll` / `#idpreserveNETWORK` / `#idpreserveUSER` feed the preserve flags. The detailed preserve set is the separate `GET/PUT /api/maintenance/preserve` page (`sdr, fru, sel, ipmi, network, ntp, snmp, ssh, kvm, authentication, syslog, web, extlog, redfish`: 0 overwrite / 1 preserve).
7. Poll `GET /api/maintenance/firmware/flash-progress` back-to-back (the next request goes out from the success handler with no delay) → `{state, progress:"NN% done", action:"Flashing..."}`. `state` is undefined until flashing starts. Done when `parseInt(progress) == 100` or NaN.
8. `POST /api/maintenance/reset` (empty body) reboots the BMC. The UI waits 120 s, then logs out and reloads.
- Cancel at any point: `POST /api/maintenance/reset` (aborts update mode).

TFTP path (`protocol_type == "tftp"`): `PUT /api/maintenance/flash`, then `PUT /api/maintenance/firmware/dwldfwimg` JSON `{"PROTO_TYPE":1}`, then poll `GET /api/maintenance/firmware/dwldfwstatus-progress` (`{state 1-5, progress, action}`) until progress is 100, then continue at step 5.

Dual-image builds (`dualFirmwareUpgradeView`, feature-gated): `PUT /api/maintenance/flash_image_config` JSON `{"image_update":…, "reboot_bmc":…}` first (B650: `/api/maintenance/dualflashimageconfig`); `GET/PUT /api/maintenance/dual_image_config` `{"boot_image":0..5}` selects the boot image.

### 3.2 BMC firmware, B650 7.02/9.05 differences
- Step 3 body: `{"preserve_config":0|1}`.
- Step 4 URL: **`POST /api/maintenance/firmware/firmware`** (multipart `fwimage`, uploaded once, no 90 s retry).
- `flash_status` adds 4 = version-compare flash (feature `VERSION_CMP_FLASH`). Constants: `CONS_FORCE_FLASH 1`, `SECTION_CMP 2`, `VERSION_CMP 4`, `FULL_FLASH 16`, `VERSION_SAME 32`, `MODULE_SIZE_DIFF 64`.
- Step 7: polls until `progress` contains `"Complete"`, then `PUT /api/maintenance/firmware/VerifyImageFlash` (`{"preserve_config","flash_status"}`), polls flash-progress again, and only then sends `POST /api/maintenance/reset` with body `{}`.
- During the flash the app polls `GET /api/maintenance/flashtrack` every 6 s. The first failure means the BMC is rebooting: clear the session, wait 120 s, go to the login page.

### 3.3 BIOS (and other ASRR components) via `/api/asrr/maintenance/<COMPONENT>/*`
`<COMPONENT>` is `BIOS` (all boards); `CPLD`, `FPGA`, `RETIMER` (B650 UI; firmware handlers only in 9.05); `PSU`, `BP` (B650 UI only; **no handler in either B650 image**). The handler strings also include `BMC` (9.05 and X470), which no UI page uses. Every component uses the same five calls (`views/maintenance/{Bios,Cpld,Fpga,Retimer,Psu,Bp}UpdateView`):
1. `POST /api/asrr/maintenance/<C>/firmware`, multipart field **`fwimage`** → `{"response":1}` ok / `2` image verify failed. B650 also returns `curversion`, `newversion` (CPLD: `cpldbpcurversion` for the backplane CPLD). X470 re-posts every 90 s until success.
2. User confirms. `POST /api/asrr/maintenance/<C>/configuration`, raw-JSON body `{"action":N}` (sent with `contentType:false`) → `{"response":1|2}`.
   - BIOS: `action` 2 = preserve BIOS settings (NVRAM), 4 = clear (checkbox `#idPreserveNvramSection`, default 4; X470 sends the value as the string `"2"`/`"4"`, B650 as an int).
   - CPLD: 1 = motherboard CPLD, 2 = backplane CPLD (`kind_cpld_radio`). BP: 0 = Expander, 1 = MCU. RETIMER: slot bitmask 1/2/4/8 (slots 4-7). PSU/FPGA: fixed 1.
3. `POST /api/asrr/maintenance/<C>/upgrade`, raw-JSON `{"action":N}` → `{"response":1}` started / `2` error.
   - BIOS `action` (radio `UpdateOption`): 1 = flash when the host is next powered off ("Flash BIOS after manually shutdown server"), 2 = flash immediately without power action, 3 = shut the host down and flash (B650 default). Other components send 1.
   - B650 + BIOS + option 1: the UI reads `/api/chassis-status`. If the host is on, it reports "will flash on next reboot" and stops.
4. Poll `GET /api/asrr/maintenance/<C>/status` → `{"response":N}`: 4 = flashing (then also `GET /api/asrr/maintenance/<C>/flash-progress` → `{"action":"Flashing...","progress":"NN%"|"About To Start..."}`), 1 = completed (X470 message 1), 2 = failed, 3 = completed (B650 success, X470 message 2), anything else = keep polling (recursively, no delay).
5. Done. The page reloads. No BMC reset is needed.
- X470 only: `POST /api/asrr/maintenance/BIOS/PublicKey` (multipart **`publickey`**, lower-case) → `{"response":1}` uploads the BIOS signing key.
- B650 only: `GET/PUT /api/asrr/maintenance/fwupdate` (`models/fwupdate`) holds global state `{flag, config, action}`. `flag` bits: 1 BMC, 2 BIOS, 4 YAFU-BIOS, 8 PSU, 16 CPLD, 32 BP, 64 RETIMER, 128 FPGA, 256 waiting-for-host-off. On page load, if a BIOS update is pending (waiting for host off), the UI resumes monitoring. Cancel = `PUT {"config":0,"action":0,"flag":0}`.
- Live capture note: the B550D4U-2T (fw 1.10.00) UI polls `GET /api/asrr/maintenance/BIOS/UpdateOption` → `{"action":0}`, an older variant of this state call.

### 3.4 HPM / NM / other update paths (generic AMI, mostly feature-gated off)
- `HpmFirmwareUpdateView`: `POST /api/maintenance/hpm` (multipart), then `PUT /api/maintenance/hpm/updatemode` (→ `unique_id`), `GET componentversions?VERS_ID=`, `PUT preparecomponents`, `PUT hpm/flash` `{COMPONENT_ID, COMPONENT_DATA_LEN, FWUPDATEID, SECTION_FLASH}`, poll `GET hpm/upgradestatus` (`PROGRESS`), `PUT verifyimage` and poll `GET verifyimagestatus`, `PUT activatecomponents`, `PUT exitupdatemode`. B650 builds the URL as `"/api/maintenance/" + ("hpm/…" | "raid_management/…")`.
- `NmFirmwareUpdateView` (Intel ME/NM): `nm_upload`, `nm_compareversion`, `nm_eraseflash`, `nm_firmware_update`, poll `nm_ecfstatus` (`nm_progress`), `nm_activate`/`nm_deactivate`.
- B650 also has PFR, PLDM, NVMe-MI, CPLD (`maintenance/cpld/*`) update pages. Their handlers are absent from both B650 images (see the handler column).

### 3.5 Configuration maintenance
`GET/PUT /api/maintenance/backup_config` then `GET /api/maintenance/download_config` (saved as `bmc-config.bak`). Restore: `POST /api/maintenance/upload_restore` (multipart **`config`**), then `PUT /api/maintenance/restore_config`. Factory defaults: `PUT /api/maintenance/restore_defaults` (same section flags as preserve). `PUT /api/maintenance/system_admin` (sysadmin user enable/password). `PUT /api/maintenance/bmc_recovery`. Cold reset: `PUT /api/asrr/bmccoldreset`.

---

## 4. KVM / H5Viewer / SOL
- Launch: `views/remotePopUpView` checks `GET /api/settings/media/adviser` (`status` 0 = KVM service disabled; HAR fields `license, status, secure_channel, kvm_port, mouse_mode, web_port, singleport_status, retry_count, retry_interval, keyboard_layout`, plus `kvm_client` on B650D4U; note the key is `keyboard_layout`, not `keyboard_language`), then `window.open(<origin>/viewer.html, "H5Viewer")`. Java: `GET /api/asrr/java-console` (X470) or `GET /api/remote_control/get/kvm/launch` (B650) returns JNLP text saved as `jviewer.jnlp`.
- The viewer (`viewer.min.js`, same origin, reuses the cookie and the `garc`/CSRF token; a token in `viewer.html#<t>` goes through `POST /api/viewerlogin`) loads `configuration/project|runtime`, `settings/media/adviser` and `settings/media/instance|remotesession`. On Start it gets the KVM token:
  - X470: `GET /api/kvm/token` → `{"client_ip","session","token"}`, stored as `sessionStorage.token`.
  - B650: `GET /api/settings/media/h5viewercfg` → token plus `kvm_service_status, client_ip, session, token, server_ip, automatic_off, local_monitor_off, keyboard_language, num_cd, num_hd, kvm_num_cd, kvm_num_hd, power_save_mode` (`/api/kvm/token` remains as a fallback for media).
- **KVM WebSocket:** `new WebSocket((https?"wss://":"ws://") + location.host + "/kvm", ["binary","base64"])`, `binaryType=arraybuffer`. AMI IVTP packets have an 8-byte little-endian header `{type:uint16, pkt_size:uint32, status:uint16}`. The first client packet (validate video session / `CMD_CONNECTION_COMPLETE_PKT`) carries `token` (`SSI_HASH_SIZE`), client IP, username and MAC.
- **Virtual media WebSocket:** `…/cd-server` (or `ws(s)://host:9999/` when the media port is 9999), also authenticated with the token. The browser-side ISO redirection uses this. Server-side image redirection uses the REST calls in the catalog's virtual-media section.
- Settings: `GET/PUT /api/settings/media/remotesession` (`single_port, kvm_encryption, keyboard_language` [AD, DA, NL-BE, NL-NL, GB, US, FI, FR-BE, FR, DE, DE-CH, IT, JP, NO, PT, ES, SV, TR_F, TR_Q], `retry_count` 1-6, `retry_time_interval` 5-30, `local_monitor_off, automatic_off, vmedia_attach`; B650 adds `kvm_client` 1 H5Viewer/2 VNC, `vnc_*`), `GET/PUT /api/settings/mouse` (`mode`), `GET/PUT /api/settings/media/instance` (`num_cd/fd/hd` 0-4, `kvm_num_*`, `secure_channel`, `power_save_mode`, `sd_media`). Active sessions: `GET/DELETE /api/settings/service-sessions?service_id=`.
- SOL (B650): `GET /api/sol/session` (`{"status":0}` HAR), `POST /api/sol/solcfg`, `GET/POST /api/sol/sol_configurations` (`baudrate` 6-10 = IPMI rate codes 9600…115200), `GET /api/settings/javasol?protocol=&externalIP=&externalport=`.

## 5. Virtual media / remote storage (server-side redirection)
- General: `GET/PUT /api/settings/media/general` (local/remote media support, `mount_cd/fd/hd`, `*_remote_server_address`, `*_remote_source_path`, `*_remote_share_type` nfs|cifs, credentials, `same_settings`; B650 adds Kerberos/ENNFS fields).
- Remote (NFS/CIFS share): `POST /api/settings/media/remote/rmediarefreshlist` `{"appType":1,"mediaType":1|2|4}` (1 CD, 2 FD, 4 HD; one call per enabled media server, 2 s apart), then `GET …/remote/images` / `…/remote/configurations` (`image_name, image_type/media_type, image_index, redirection_status, media_index, session_index`). Start: `POST /api/settings/media/remote/start-media` `{"image_name","image_type","image_index"}` (B650 also `image_redirection:1, image_share_type`). Stop: `POST …/remote/stop-media` with the same fields.
- Local (image stored on the BMC): `POST /api/settings/media/local/lmediasizecheck` `{"image_size":<bytes>}`, then `POST /api/settings/media/local/file` multipart **`lmedia_image`**, then `GET …/local/images`. Start and stop are the same as remote under `/local/`. Delete: `DELETE /api/settings/media/local/image?&image_index=<n>&image_name=<name>`.
- B650: `GET /api/settings/media/active_redirections`, `…/remote/lmediarefreshlist`.

---

## 6. Handler presence and live validation

### 6.1 Handler presence by area

"Handler in firmware" means the `/api` route string exists in that image's REST binaries (see Sources). It is a strings-level check: "yes" means the router knows the path, not that every method works. "no" means the UI calls the path but no shipped binary contains it; expect a 404 or a FastCGI error. "unknown" is used for the four X470 `settings/fan/fan_*` collections, which appear only as handler symbol names (`settings_fan_mode`, `settings_fan_tscft`, …) and not as route strings.

| Area | X470-3.02 UI refs (handler yes/no/unknown) | B650-7.02 UI refs (yes/no) | B650-9.05 UI refs (yes/no) | Seen in HAR |
|---|---|---|---|---|
| Session & authentication | 4 (4/0/0) | 7 (7/0) | 7 (7/0) | 1 |
| Feature discovery & UI configuration | 2 (2/0/0) | 5 (4/1) | 5 (4/1) | 3 |
| Dashboard & identity | 6 (6/0/0) | 7 (7/0) | 7 (7/0) | 8 |
| Power, chassis & UID | 8 (8/0/0) | 10 (9/1) | 9 (8/1) | 4 |
| Sensors & SDR | 4 (4/0/0) | 6 (4/2) | 6 (4/2) | 2 |
| FRU & PLDM inventory | 1 (1/0/0) | 6 (3/3) | 6 (3/3) | 1 |
| Logs, crash dumps & diagnostics | 14 (14/0/0) | 34 (19/15) | 34 (18/16) | 4 |
| Host inventory | 13 (13/0/0) | 23 (9/14) | 23 (9/14) | 7 |
| Storage / PCIe device management | 22 (22/0/0) | 64 (0/64) | 64 (0/64) | 0 |
| KVM, H5Viewer & SOL | 5 (5/0/0) | 12 (11/1) | 12 (11/1) | 2 |
| Virtual media / remote storage | 14 (14/0/0) | 15 (15/0) | 15 (15/0) | 0 |
| Settings: network | 18 (17/1/0) | 19 (18/1) | 19 (18/1) | 4 |
| Settings: users & PAM | 5 (5/0/0) | 6 (6/0) | 6 (6/0) | 1 |
| Settings: services & sessions | 5 (5/0/0) | 3 (3/0) | 3 (3/0) | 2 |
| Settings: SSL | 3 (3/0/0) | 3 (3/0) | 3 (3/0) | 0 |
| Settings: LDAP / AD / RADIUS | 7 (7/0/0) | 7 (7/0) | 7 (7/0) | 0 |
| Settings: date & time / NTP | 1 (1/0/0) | 1 (1/0) | 1 (1/0) | 1 |
| Settings: SMTP, PEF, alerts & SNMP | 8 (8/0/0) | 12 (9/3) | 12 (9/3) | 0 |
| Settings: firewall | 6 (6/0/0) | 4 (4/0) | 4 (4/0) | 0 |
| Settings: fan control | 21 (17/0/4) | 8 (8/0) | 8 (8/0) | 3 |
| Settings: video recording / BSOD / SOL recording | 7 (7/0/0) | 7 (7/0) | 7 (7/0) | 1 |
| Settings: licensing | 2 (2/0/0) | 2 (2/0) | 2 (2/0) | 0 |
| Settings: BIOS / platform OEM | 1 (1/0/0) | 3 (1/2) | 3 (1/2) | 1 |
| Maintenance & firmware update | 44 (43/1/0) | 103 (48/55) | 103 (63/40) | 5 |
| Automation engine (scripts/tasks) | 3 (3/0/0) | 3 (3/0) | 3 (3/0) | 0 |
| **TOTAL** | 224 (218/2/4) | 370 (208/162) | 369 (221/148) | 50 |

Per board, of the paths its own UI references: X470-3.02 224 (218 yes, 2 no, 4 unknown); B650-7.02 370 (208 yes, 162 no); B650-9.05 369 (221 yes, 148 no).

### 6.2 Why so many B650 paths have no handler

The B650 web UI is AMI's generic SPX bundle and is a superset gated by `configuration/project` features. `spx_restservice` `dlopen`s optional plugins, and 18 of them are not shipped (`docs/firmware/b650d4u.md` §5.1). The 148 unhandled 9.05 paths group as: `settings/raid_management` (32), `maintenance/pfr` (14), `settings` (9), `settings/pcie_switch_management` (9), `host` (8), `settings/brcm_pcie_switch_management` (7), `pldm` (6), `nvdimm_inventory` (6), `settings/sasit_management` (6), `settings/nvme_mi_management` (5), `asrr/maintenance/BP` (5), `asrr/maintenance/PSU` (5), `acd` (4), `maintenance/nvme` (4), `maintenance/pldm` (4), `system_diagnostics` (3), `gpu` (3), `settings/nvme_management` (3), `maintenance/raid` (3), `asrr` (2), `logs` (2), `maintenance/cpld` (2), `gpu_info` (1), `settings/nic_management` (1), `settings/pcie_switch_mgmt` (1), `maintenance/hpm` (1), `maintenance/memory` (1), `maintenance/signed` (1).

`docs/firmware/b650d4u.md` reports 151 of 366 B650-UI endpoints without a handler. This catalog finds 148 of 369 for 9.05. The differences: this catalog normalises trailing-slash and prefix forms (`/api/settings/users/`, `/api/maintenance/`, `/api/scripts/`) into their base paths, adds the dynamic HPM/RAID and viewer-only paths, and matches regex item routes, which shows that `/api/scripts` and `/api/settings/user/ssh-key-upload/:id` do have handlers (`/scripts`, `/settings/user/ssh-key-upload/([[:digit:]]+)`).

On X470-3.02 only two UI paths have no route string. The X470 image also contains route strings for many generic AMI pages its UI does not use (RAID, SAS-IT, NVMe, AE). The plugins behind several of those (`libspx_restservice_{acd,aep,nvme,pldm,raid,sasit,sol}.so`) are not shipped, so treat X470 "yes" for those groups as "route known, back end probably missing" (`docs/firmware/x470d4u.md` §5.2).

### 6.3 Validation against the live captures

All three captures were parsed in full: every `/api` request's method, path, status, query keys, request content type and body keys, and the response's key names and JSON types. No values were copied. The results are on each endpoint as `Seen in HAR` and `Live (<file>)` lines, and in the JSON as `seen_in_har` / `har_observed` / `har_vs_js`. 50 catalog paths were seen live. For endpoints with a captured response, the **response field list is built from the capture** and then extended with fields the UI reads but the capture lacks (marked "absent from capture"). The original JS-only list is kept in the JSON as `response_fields_js_inferred`.

Corrections and findings from the captures:
- **The captures run older firmware** than the analysed bundles (X470D4U 2.32.00; B650D4U 1.10.00; the `b650d4u-2t.har` board reports `mb_name` `B550D4U-2T`, fw 1.10.00). A shape seen live is reliable. A field that is absent may simply be newer than the capture.
- `GET /api/settings/media/adviser`: the key is **`keyboard_layout`** (all captures), not `keyboard_language`. B650D4U also returns `kvm_client`.
- `GET /api/sensors`: static analysis had attached SEL/event fields (`EventData1…`, `sensor_type_table`, …) to this endpoint from sibling models. The capture shows a flat array of `{id, sensor_number, name, owner_id, owner_lun, raw_reading, type, type_number, reading, sensor_state, discrete_state, accessible, lower_/higher_{non_recoverable,critical,non_critical}_threshold, unit}`. X470 adds `settable_readable_threshMask`; B650-family firmware returns `settable_flag` instead. `raw_reading` is a float on X470 and an int on B650. On B650-family firmware an unset threshold is the **string `"NA"`** instead of a number (X470 returns numbers throughout), and `reading` may be an int or a float. A Go decoder needs a flexible number type for these fields. `GET /api/dashboardsensors` (B650) returns the same item shape, but some items carry only `{id, sensor_number, name}`.
- `GET /api/host_inventory/host_interface_{processor,system}_info`: the JS lists mixed the processor, memory and system models. Corrected from the capture (processor: `Id, Name, Manufacturer, MaxSpeedMHz, Model, ProcessorArchitecture, ProcessorType, Socket, State, EffectiveFamily, TotalCores`; system: `Name, Description, Model, IndicatorLED, Manufacturer, PowerState, SerialNumber, PartNumber, SystemType, State, AssetTag, BiosVersion, UUID`). Empty arrays are returned when the host has not pushed inventory (X470 and B650D4U captures). `host_interface_storage_info` returns the misspelled key `Storage_Contoller`, and returns `null` for both members when it has no data.
- `GET /api/chassis-status`: `power_cycle_interval` is present on B650D4U 1.10 but not on B550D4U-2T 1.10 or X470. A proxy should treat it as optional (2000 ms in the one capture that has it). `power_on_after_flash` (read by the B650 UI) did not appear in any capture.
- `POST /api/session`: B650-family responses add `user_id`, `channel` and `passwordStatus` (confirmed). `TFAEnabled`/`TFAStatus` appear only with two-factor enabled (not captured).
- `GET /api/settings/date-time`: X470 returns `auto_date` and `daylight`. B550D4U-2T returns `ntp_auto_date` and no `daylight`. A proxy must not assume one key set.
- `GET /api/settings/users`: the two families have different item shapes. X470: `network_privilege`, `privilege_limit_serial`. B650-family: `channel`, `channel_type`, `userid`, `accessByChannel`, `privilege`, `privilegeByChannel`, `password_age`, `password_enddate`.
- `GET /api/logs/audit?cmd=…` returns the count form `[{"audit_count":N}]` (B650D4U). Without `cmd` it returns the entries `[{id, timestamp, hostname, message}]` (B550D4U-2T).
- `GET /api/logs/eventlog?EVENTID=…` (B550D4U-2T) is the incremental form. B650-family entries add `sensor_type_table`, `record_type_table`, `event_direction_table`. X470 adds the `pci_*_num` and `*_Description_raw` fields.
- `POST /api/asrr/settings/setopenlooptablenumber`: the request key is literally **`table_numble`** (sic), and the response is `{}` (B650D4U capture, an endpoint the analysed B650 UIs no longer reference).
- `PUT /api/settings/network-bond`: the request is the whole model plus `bond_ifc`, and the response echoes it (confirms the singleton read-modify-write rule in 1.2).
- `GET /api/asrr/product-name` returns HTTP 500 `{"error":"Error in Getting Model Name","code":16005}` on X470 2.32. Use `asrr/model-name` (`mb_name`, plus `sys_name` on B650-family firmware).
- `GET /api/asrr/fw-info`: X470 returns `BMC/BIOS/ME/Micro_Code/CPLD`. B650-family firmware adds `PSP_fw_version`, and B650D4U adds `BP_version`. `BPCPLD_version` (read by the 9.05 UI) was not captured.
- `GET /api/settings/services` returned **401** before login in both B650 captures. It is the first call the B650 UI makes, and the 401 is expected.
- Every request after login carried `X-CSRFTOKEN` in all three captures. Cookies are stripped from the captures.
- Paths seen live whose handler column is not "yes" on every board whose UI references them: `/api/maintenance/signed-image-key`.

### 6.4 Endpoint index (handler column)

Cell format: `<UI references it?> / <handler present in firmware>`. `-` = not referenced by that board's UI. HAR: `X` = x470d4u.har, `B` = b650d4u.har, `2T` = b650d4u-2t.har.

| Path | Methods | X470-3.02 | B650-7.02 | B650-9.05 | HAR |
|---|---|---|---|---|---|
| `/api/asrr/session` | POST | UI / yes | - / no | - / no | - |
| `/api/newtabinfo` | GET | - / no | UI / yes | UI / yes | - |
| `/api/reset-pass` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/session` | DELETE/POST | UI / yes | UI / yes | UI / yes | X, B, 2T |
| `/api/tfa` | DELETE/GET/POST | - / no | UI / yes | UI / yes | - |
| `/api/updatenew_password` | POST | - / no | UI / yes | UI / yes | - |
| `/api/verify_otp` | POST | - / no | UI / yes | UI / yes | - |
| `/api/viewerlogin` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/configuration/project` | GET | UI / yes | UI / yes | UI / yes | X, 2T |
| `/api/configuration/runtime` | GET | UI / yes | UI / yes | UI / yes | X |
| `/api/extfeatures` | GET | - / no | UI / yes | UI / yes | - |
| `/api/settings/user-preference` | GET | - / no | UI / no | UI / no | - |
| `/api/status/uptime` | GET | - / yes | UI / yes | UI / yes | B, 2T |
| `/api/asrr/fw-info` | GET | UI / yes | UI / yes | UI / yes | X, B, 2T |
| `/api/asrr/model-name` | GET | UI / yes | UI / yes | UI / yes | X, B, 2T |
| `/api/asrr/product-name` | GET | UI / yes | - / no | - / no | X |
| `/api/asrr/selsummary` | GET | UI / yes | - / yes | - / yes | X |
| `/api/dashboardsensors` | GET | - / no | UI / yes | UI / yes | B |
| `/api/firmware-info` | GET/PUT | UI / yes | UI / yes | UI / yes | X |
| `/api/logs/dashboardevent` | GET | UI / yes | UI / yes | UI / yes | B, 2T |
| `/api/settings/dashboardinfo` | GET | - / no | UI / yes | UI / yes | B |
| `/api/settings/dashboardntp` | GET | - / no | UI / yes | UI / yes | - |
| `/api/actions/accessbiossetup` | POST | - / no | UI / yes | UI / yes | - |
| `/api/actions/chassis-led` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/actions/power` | POST | UI / yes | UI / yes | UI / yes | X |
| `/api/asrr/GetPowerRestore` | GET | - / no | UI / yes | UI / yes | - |
| `/api/asrr/PowerSource` | GET | UI / yes | UI / yes | UI / yes | X |
| `/api/asrr/PowerSourceID` | GET | - / no | UI / yes | - / no | - |
| `/api/asrr/SetPowerRestore` | POST | - / no | UI / yes | UI / yes | - |
| `/api/asrr/bmccoldreset` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/asrr/getprochot` | GET | UI / yes | - / no | - / no | - |
| `/api/asrr/setprochot` | PUT | UI / yes | - / no | - / no | - |
| `/api/chassis` | GET | UI / yes | - / no | - / no | X |
| `/api/chassis-status` | GET | UI / yes | UI / yes | UI / yes | X, B, 2T |
| `/api/settings/amdpowercap` | GET/PUT | - / no | UI / no | UI / no | - |
| `/api/detail_sensors_readings` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/pldm/sensors` | GET | - / no | UI / no | UI / no | - |
| `/api/pldm/state_sensors` | GET | - / no | UI / no | UI / no | - |
| `/api/sdr` | GET | UI / yes | UI / yes | UI / yes | X, 2T |
| `/api/sensors` | GET | UI / yes | UI / yes | UI / yes | X, B, 2T |
| `/api/settings/sensor-threshold` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/fru` | GET | UI / yes | UI / yes | UI / yes | X |
| `/api/frueditable` | GET | - / no | UI / yes | UI / yes | - |
| `/api/fruupdate` | PUT | - / no | UI / yes | UI / yes | - |
| `/api/pldm/eids` | GET | - / no | UI / no | UI / no | - |
| `/api/pldm/entity_association` | GET | - / no | UI / no | UI / no | - |
| `/api/pldm/fru` | GET | - / no | UI / no | UI / no | - |
| `/api/acd/download_crashdump` | GET | - / no | UI / no | UI / no | - |
| `/api/acd/getcrashdumpstatus` | GET | - / no | UI / no | UI / no | - |
| `/api/acd/set_crashdump_download` | PUT | - / no | UI / no | UI / no | - |
| `/api/acd/setcrashdumpstatus` | PUT | - / no | UI / no | UI / no | - |
| `/api/asrr/clearmcalog` | DELETE | UI / yes | UI / no | UI / no | - |
| `/api/asrr/getmcalog` | GET | UI / yes | UI / yes | UI / yes | X, 2T |
| `/api/asrr/mca-file` | GET | UI / yes | UI / no | UI / no | - |
| `/api/host/addc_autonomous_debug_data_collection/error_recovery_mode_and_retry_count` | GET/PUT | - / no | UI / no | UI / no | - |
| `/api/host/download_addc_autonomous_debug_data_collection` | GET | - / no | UI / no | UI / no | - |
| `/api/host/get_addc_autonomous_debug_data_collection` | GET | - / no | UI / no | UI / no | - |
| `/api/host/get_crash_dump` | GET | - / no | UI / no | UI / no | - |
| `/api/logs/acd` | GET | - / no | UI / no | UI / no | - |
| `/api/logs/audit` | GET | - / yes | UI / yes | UI / yes | B, 2T |
| `/api/logs/debuglogs` | GET | - / no | UI / yes | UI / yes | - |
| `/api/logs/event` | DELETE/GET (+item DELETE) | UI / yes | UI / yes | UI / yes | - |
| `/api/logs/event-file` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/logs/eventlog` | GET | UI / yes | UI / yes | UI / yes | X, 2T |
| `/api/logs/getvideotag` | POST | UI / yes | - / no | - / no | - |
| `/api/logs/help-data` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/logs/platform` | GET | - / no | UI / yes | UI / no | - |
| `/api/logs/postcode` | GET | - / no | UI / yes | UI / yes | - |
| `/api/logs/raw-event-file` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/logs/savepostcode` | GET | - / no | UI / yes | UI / yes | - |
| `/api/logs/system` | GET | - / yes | UI / yes | UI / yes | - |
| `/api/logs/video` | GET | UI / yes | UI / yes | UI / yes | X |
| `/api/logs/video-data` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/logs/video-log` | GET | - / yes | UI / yes | UI / yes | - |
| `/api/logs/video-log/delete` | DELETE | UI / yes | UI / yes | UI / yes | - |
| `/api/pldm/get_all_event_log` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/log` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/log-certificates` | POST | - / no | UI / yes | UI / yes | - |
| `/api/settings/log-policy` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/system_diagnostics/download_system_diagnostics_log` | GET | - / no | UI / no | UI / no | - |
| `/api/system_diagnostics/generate_system_diagnostics_logs` | GET | - / no | UI / no | UI / no | - |
| `/api/system_diagnostics/log` | GET | - / no | UI / no | UI / no | - |
| `/api/gpu/nvfpga` | GET | - / no | UI / no | UI / no | - |
| `/api/gpu/nvgpu` | GET | - / no | UI / no | UI / no | - |
| `/api/gpu/status` | GET | - / no | UI / no | UI / no | - |
| `/api/gpu_info` | GET | - / no | UI / no | UI / no | - |
| `/api/host/asd/getasdjtaginfo` | GET | - / no | UI / no | UI / no | - |
| `/api/host/asd/getasdremotedebuginfo` | GET | - / no | UI / no | UI / no | - |
| `/api/host/asd/setasdremotedebuginfo` | PUT | - / no | UI / no | UI / no | - |
| `/api/host/asd/tls_certificate` | POST | - / no | UI / no | UI / no | - |
| `/api/host_inventory/host_interface_baseboard_info` | GET | UI / yes | UI / yes | UI / yes | B |
| `/api/host_inventory/host_interface_memory_info` | GET | UI / yes | UI / yes | UI / yes | X, B, 2T |
| `/api/host_inventory/host_interface_pcie_device_function_info` | GET | UI / yes | UI / yes | UI / yes | X, B, 2T |
| `/api/host_inventory/host_interface_power_info` | GET | UI / yes | UI / yes | UI / yes | B |
| `/api/host_inventory/host_interface_processor_info` | GET | UI / yes | UI / yes | UI / yes | X, B, 2T |
| `/api/host_inventory/host_interface_storage_info` | GET | UI / yes | UI / yes | UI / yes | X, 2T |
| `/api/host_inventory/host_interface_system_info` | GET | UI / yes | UI / yes | UI / yes | B |
| `/api/host_inventory/host_interface_thermal_info` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/nvdimm_inventory/last_action_info` | GET | - / no | UI / no | UI / no | - |
| `/api/nvdimm_inventory/nvdimm_dimm_action` | POST | - / no | UI / no | UI / no | - |
| `/api/nvdimm_inventory/nvdimm_dimm_info` | GET | - / no | UI / no | UI / no | - |
| `/api/nvdimm_inventory/nvdimm_namespace_info` | GET | - / no | UI / no | UI / no | - |
| `/api/nvdimm_inventory/nvdimm_pools_info` | GET | - / no | UI / no | UI / no | - |
| `/api/nvdimm_inventory/nvdimmdimm_fw_flash_status` | GET | - / no | UI / no | UI / no | - |
| `/api/smbiosdata` | GET | - / no | UI / yes | UI / yes | - |
| `/api/system_inventory/component_status_update` | PUT | UI / yes | - / no | - / no | - |
| `/api/system_inventory/cpu_info` | GET (+item DELETE) | UI / yes | - / no | - / no | - |
| `/api/system_inventory/device_count` | GET (+item DELETE) | UI / yes | - / no | - / no | - |
| `/api/system_inventory/dimm_info` | GET (+item DELETE) | UI / yes | - / no | - / no | - |
| `/api/system_inventory/pci_info` | GET (+item DELETE) | UI / yes | - / no | - / no | - |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_info` | GET (+item PUT) | - / no | UI / no | UI / no | - |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_port_assign_unassign` | PUT | - / no | UI / no | UI / no | - |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_port_error_counters` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_port_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_reset_port_error_counters` | PUT | - / no | UI / no | UI / no | - |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_station_ram_error_counters` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_station_reset_ram_error_counters` | PUT | - / no | UI / no | UI / no | - |
| `/api/settings/nic_management/nic_controller_info` | GET (+item PUT) | - / no | UI / no | UI / no | - |
| `/api/settings/nvme_management/nvme_basic_mi_vpd_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/nvme_management/nvme_controller_info` | GET (+item PUT) | - / no | UI / no | UI / no | - |
| `/api/settings/nvme_management/nvme_p3700_vital_product_data` | GET (+item PUT) | - / no | UI / no | UI / no | - |
| `/api/settings/nvme_mi_management/nvme_mi_clear_status_flags` | PUT | - / no | UI / no | UI / no | - |
| `/api/settings/nvme_mi_management/nvme_mi_set_status_flags` | PUT | - / no | UI / no | UI / no | - |
| `/api/settings/nvme_mi_management/nvme_mi_ssd_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/nvme_mi_management/nvme_mi_subsystem_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/nvme_mi_management/nvme_mi_vpd_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/pcie_switch_management/get_pcie_bifurcation_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/pcie_switch_management/pcie_mgmt_dev_info` | GET (+item PUT) | - / no | UI / no | UI / no | - |
| `/api/settings/pcie_switch_management/pcie_partition_count` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/pcie_switch_management/pcie_port_bind` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/pcie_switch_management/pcie_port_link_status` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/pcie_switch_management/pcie_port_unbind` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/pcie_switch_management/pcie_switch_reset` | POST | - / no | UI / no | UI / no | - |
| `/api/settings/pcie_switch_management/set_pcie_bifurcation_info` | PUT | - / no | UI / no | UI / no | - |
| `/api/settings/pcie_switch_management/validate_port_unbind` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/pcie_switch_mgmt/pcie_partition_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/bbu_info` | GET (+item PUT) | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/clear_all_foreign_cmd` | POST | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/create_lu_cache` | GET/PUT | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/create_virtual_drive` | GET/PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/dedicated_hot_spare_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/delete_virtual_drive` | DELETE | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/logical_consistency_check_cmd` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/logical_device_info` | GET (+item DELETE/PUT) | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/logical_device_progress_cmd` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/logical_initialize_check_cmd` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/logical_start_stop_locate_cmd` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/manage_array_info` | GET (+item PUT) | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/manage_data_drive_array` | PUT | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/manage_spare_drive_array` | PUT | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/patrol_read_properties` | GET (+item PUT) | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/patrol_read_start_stop_cmd` | PUT | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/physical_dedicated_hotspare_cmd` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/physical_device_info` | GET (+item DELETE/PUT) | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/physical_device_smart_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/physical_hotspare_cmd` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/physical_onlineoffline_cmd` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/physical_removal_cmd` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/physical_start_stop_locate_cmd` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/physical_state_cmd` | POST | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/raid_controller_info` | GET (+item PUT) | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/raid_event_log` | DELETE/GET | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/raid_levels` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/raid_pd_count` | PUT | UI / yes | UI / no | UI / no | - |
| `/api/settings/raid_management/raid_topology_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/sesenclosureinfo` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/sesenclosureinfobrcm` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/raid_management/storage_summary` | GET (+item PUT) | UI / yes | UI / no | UI / no | - |
| `/api/settings/sasit_management/sasit_controller_info` | GET (+item DELETE/PUT) | UI / yes | UI / no | UI / no | - |
| `/api/settings/sasit_management/sasit_enclosure_info` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/sasit_management/sasit_event_log` | DELETE/GET | - / no | UI / no | UI / no | - |
| `/api/settings/sasit_management/sasit_physical_device_info` | GET (+item PUT) | UI / yes | UI / no | UI / no | - |
| `/api/settings/sasit_management/sasit_physical_start_stop_locate_cmd` | PUT | - / no | UI / no | UI / no | - |
| `/api/settings/sasit_management/sasit_topology_info` | GET (+item DELETE/PUT) | UI / yes | UI / no | UI / no | - |
| `/api/asrr/java-console` | GET | UI / yes | - / no | - / no | - |
| `/api/kvm/token` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/remote_control/get/kvm/launch` | GET | - / no | UI / yes | UI / yes | - |
| `/api/settings/javasol` | GET | - / yes | UI / no | UI / no | - |
| `/api/settings/media/active_redirections` | GET | - / no | UI / yes | UI / yes | - |
| `/api/settings/media/adviser` | GET | UI / yes | UI / yes | UI / yes | X, B |
| `/api/settings/media/h5viewercfg` | GET | - / no | UI / yes | UI / yes | - |
| `/api/settings/media/remotesession` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/mouse` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/video/sol_configurations` | GET/PUT | - / no | UI / yes | UI / yes | - |
| `/api/sol/session` | GET | - / no | UI / yes | UI / yes | B |
| `/api/sol/sol_configurations` | GET/POST | - / no | UI / yes | UI / yes | - |
| `/api/sol/solcfg` | POST | - / no | UI / yes | UI / yes | - |
| `/api/settings/media/general` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/instance` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/local/configurations` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/local/file` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/local/image` | DELETE | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/local/images` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/local/lmediasizecheck` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/local/start-media` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/local/stop-media` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/remote/configurations` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/remote/images` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/remote/lmediarefreshlist` | GET/POST | - / no | UI / yes | UI / yes | - |
| `/api/settings/media/remote/rmediarefreshlist` | GET/POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/remote/start-media` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/media/remote/stop-media` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/asrr/getvetobit` | GET | UI / yes | - / no | - / no | B |
| `/api/asrr/setvetobit` | PUT | UI / yes | - / no | - / no | - |
| `/api/set/dns/registration` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/channels` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/dns` | GET/PUT | UI / yes | - / no | - / no | - |
| `/api/settings/dns-info` | GET/PUT | - / no | UI / yes | UI / yes | - |
| `/api/settings/dns/domain-options` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/dns/registration` | GET (+item PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/dns/restart` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/dns/server-options` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/dns/tsig-private-key` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/ncsi-interfaces` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/ncsi/mode` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/ncsilinkup` | GET/PUT | - / no | UI / yes | UI / yes | - |
| `/api/settings/ncsiselect` | GET/PUT | - / no | UI / no | UI / no | - |
| `/api/settings/network` | GET (+item PUT) | UI / yes | UI / yes | UI / yes | X, B, 2T |
| `/api/settings/network-bond` | GET/PUT | UI / yes | UI / yes | UI / yes | X, B, 2T |
| `/api/settings/network-interfaces` | GET | UI / no | - / no | - / no | - |
| `/api/settings/network-link` | GET (+item PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/network/activelancfg` | GET | - / yes | UI / yes | UI / yes | - |
| `/api/settings/network/availablelancfg` | GET | - / no | UI / yes | UI / yes | - |
| `/api/settings/packages` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/static-ipv6` | GET | UI / yes | UI / yes | UI / yes | X |
| `/api/asrr/settings/passwordcfg` | GET/PUT | - / no | UI / yes | UI / yes | - |
| `/api/settings/pam-order` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/user/ssh-key-upload/:id` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/users` | GET (+item PUT) | UI / yes | UI / yes | UI / yes | X, 2T |
| `/api/settings/users/:id` | DELETE | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/users_read_only` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/asrr/getWebEnableTimeout` | GET | UI / yes | - / yes | - / yes | - |
| `/api/asrr/setWebEnableTimeout` | POST | UI / yes | - / yes | - / yes | - |
| `/api/settings/ipmi_disable_interfaces` | GET/PUT | - / no | UI / yes | UI / yes | 2T |
| `/api/settings/service-sessions` | DELETE/GET (+item DELETE) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/services` | GET (+item DELETE/PUT) | UI / yes | UI / yes | UI / yes | B, 2T |
| `/api/settings/web_server_instances` | GET/PUT | UI / yes | - / yes | - / yes | - |
| `/api/operations/ssl/certificate/generate` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/ssl/certificate` | GET/POST/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/ssl/certificate-info` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/active-directory-settings` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/active-directory-users` | GET (+item DELETE/PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/ldap-certificates` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/ldap-settings` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/ldap-users` | GET (+item DELETE/PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/radius/advanced` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/radius/general` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/date-time` | GET/PUT | UI / yes | UI / yes | UI / yes | X, 2T |
| `/api/settings/getSnmpv2` | GET | - / no | UI / no | UI / no | - |
| `/api/settings/lan_destination/snmptrap_user` | GET/PUT | - / no | UI / no | UI / no | - |
| `/api/settings/pef/alert_policies` | GET (+item DELETE/PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/pef/alert_string_keys` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/pef/destinations` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/pef/event_filters` | GET (+item DELETE/PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/pef/lan_destinations` | GET (+item DELETE/PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/pef/send_test_alert` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/setSnmpv2` | POST | - / no | UI / no | UI / no | - |
| `/api/settings/smtp` | GET (+item PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/smtp/smtpupload` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/snmp/mibfile` | GET | - / no | UI / yes | UI / yes | - |
| `/api/settings/firewall` | GET (+item PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/firewall-ip-rule` | GET/PUT | UI / yes | - / no | - / no | - |
| `/api/settings/firewall-ip-rules` | GET (+item DELETE/PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/firewall-port-rule` | GET/PUT | UI / yes | - / no | - / no | - |
| `/api/settings/firewall-port-rules` | GET (+item DELETE/PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/firewall/:id` | DELETE | UI / yes | UI / yes | UI / yes | - |
| `/api/asrr/GetFanDuty` | GET | UI / yes | - / no | - / no | X |
| `/api/asrr/settings/getfancloseloopcontroltable` | GET | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/getfancontrolmode` | GET | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/getfandutymanualmode` | GET | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/getfanopenloopcontroltable` | GET | UI / yes | - / no | - / no | B |
| `/api/asrr/settings/getfantempsensorandcorrespondfantable` | GET | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/getsupportfan` | GET | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/gettscftsupportfan` | GET | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/setclearallfansetting` | POST | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/setcloselooptablenumber` | POST | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/setfancloseloopcontroltable` | POST | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/setfancontrolmode` | POST | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/setfandutymanualmode` | POST | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/setfanopenloopcontroltable` | POST | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/setfantempsensorandcorrespondfantable` | POST | UI / yes | - / no | - / no | - |
| `/api/asrr/settings/setopenlooptablenumber` | POST | UI / yes | - / no | - / no | B |
| `/api/asrr/settings/settscftsensornumber` | POST | UI / yes | - / no | - / no | - |
| `/api/settings/fan/assignment` | GET (+item PUT) | - / no | UI / yes | UI / yes | - |
| `/api/settings/fan/closedloop` | GET (+item PUT) | - / no | UI / yes | UI / yes | - |
| `/api/settings/fan/fan_closeloopctb` | GET | UI / unknown | - / no | - / no | - |
| `/api/settings/fan/fan_mode` | GET | UI / unknown | - / no | - / no | - |
| `/api/settings/fan/fan_openloopctb` | GET | UI / unknown | - / no | - / no | - |
| `/api/settings/fan/fan_tscft` | GET | UI / unknown | - / no | - / no | - |
| `/api/settings/fan/load_default` | POST | - / no | UI / yes | UI / yes | - |
| `/api/settings/fan/manual` | GET/PUT | - / no | UI / yes | UI / yes | - |
| `/api/settings/fan/mode` | GET/PUT | - / no | UI / yes | UI / yes | - |
| `/api/settings/fan/names` | GET | - / no | UI / yes | UI / yes | - |
| `/api/settings/fan/openloop` | GET (+item PUT) | - / no | UI / yes | UI / yes | - |
| `/api/settings/fan/temp_sensors` | GET | - / no | UI / yes | UI / yes | - |
| `/api/settings/download_image` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/sol-recorded-video` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/video/pre-event` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/video/remote-storage` | GET/PUT | UI / yes | UI / yes | UI / yes | X |
| `/api/settings/video/sol-remote-storage` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/video/sol-triggers` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/video/triggers` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/license` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/settings/licenses` | GET (+item PUT) | UI / yes | UI / yes | UI / yes | - |
| `/api/asrr/getbioscode` | GET | UI / yes | UI / yes | UI / yes | 2T |
| `/api/settings/ocprofile` | GET/PUT | - / no | UI / no | UI / no | - |
| `/api/settings/telemetrycups` | GET/PUT | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/BIOS/PublicKey` | POST | UI / yes | - / no | - / no | - |
| `/api/asrr/maintenance/BIOS/UpdateOption` | GET | - / no | - / no | - / no | 2T |
| `/api/asrr/maintenance/BIOS/configuration` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/asrr/maintenance/BIOS/firmware` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/asrr/maintenance/BIOS/flash-progress` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/asrr/maintenance/BIOS/status` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/asrr/maintenance/BIOS/upgrade` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/asrr/maintenance/BP/configuration` | POST | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/BP/firmware` | POST | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/BP/flash-progress` | GET | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/BP/status` | GET | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/BP/upgrade` | POST | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/CPLD/configuration` | POST | - / yes | UI / no | UI / yes | - |
| `/api/asrr/maintenance/CPLD/firmware` | POST | - / yes | UI / no | UI / yes | - |
| `/api/asrr/maintenance/CPLD/flash-progress` | GET | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/CPLD/status` | GET | - / yes | UI / no | UI / yes | - |
| `/api/asrr/maintenance/CPLD/upgrade` | POST | - / yes | UI / no | UI / yes | - |
| `/api/asrr/maintenance/FPGA/configuration` | POST | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/FPGA/firmware` | POST | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/FPGA/flash-progress` | GET | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/FPGA/status` | GET | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/FPGA/upgrade` | POST | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/PSU/configuration` | POST | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/PSU/firmware` | POST | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/PSU/flash-progress` | GET | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/PSU/status` | GET | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/PSU/upgrade` | POST | - / no | UI / no | UI / no | - |
| `/api/asrr/maintenance/RETIMER/configuration` | POST | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/RETIMER/firmware` | POST | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/RETIMER/flash-progress` | GET | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/RETIMER/status` | GET | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/RETIMER/upgrade` | POST | - / no | UI / no | UI / yes | - |
| `/api/asrr/maintenance/fwupdate` | GET/PUT | - / no | UI / yes | UI / yes | - |
| `/api/maintenance/ValidSignImage` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/backup_config` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/bmc_recovery` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/cpld/enableupdatemode` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/cpld/getinfo` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/download_config` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/dual_image_config` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/dualflashimageconfig` | PUT | - / no | UI / yes | UI / yes | - |
| `/api/maintenance/firmware` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/firmware/VerifyImageFlash` | PUT | - / no | UI / yes | UI / yes | - |
| `/api/maintenance/firmware/dwldfwimg` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/firmware/dwldfwstatus-progress` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/firmware/firmware` | POST | - / no | UI / yes | UI / yes | - |
| `/api/maintenance/firmware/flash-progress` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/firmware/upgrade` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/firmware/verification` | GET/POST | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/flash` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/flash_image_config` | PUT | UI / yes | - / no | - / no | - |
| `/api/maintenance/flashtrack` | GET | - / no | UI / yes | UI / yes | - |
| `/api/maintenance/fwimage_location` | GET/PUT | UI / yes | UI / yes | UI / yes | X, B |
| `/api/maintenance/getPlatformID` | GET | - / no | UI / yes | UI / yes | - |
| `/api/maintenance/hpm` | POST | UI / no | UI / no | UI / no | - |
| `/api/maintenance/hpm/activatecomponents` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/hpm/componentversions` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/hpm/exitupdatemode` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/hpm/flash` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/hpm/freemem` | GET | - / no | UI / yes | UI / yes | - |
| `/api/maintenance/hpm/getbiosversions` | GET | - / no | UI / yes | UI / yes | - |
| `/api/maintenance/hpm/preparecomponents` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/hpm/updatemode` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/hpm/upgradestatus` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/hpm/verifyimage` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/hpm/verifyimagestatus` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/memory_resilience_technology_information` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/nm_activate` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/nm_compareversion` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/nm_deactivate` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/nm_ecfstatus` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/nm_eraseflash` | GET | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/nm_firmware_update` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/nm_upload` | GET/POST | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/nvme/activatecomponents` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/nvme/fwupdate` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/nvme/fwupdateprogress` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/nvme/nvmemissdsfw` | POST | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/activatecomponents` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/exitupdatemode` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/flash` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/flashimagestatus` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/fwupdateoptions` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/pfrfw` | POST | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/pfrfwimagesize` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/pfrgetreadystatus` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/pfrversioninfo` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/preparecomponents` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/seamlessflashstatus` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/updatemode` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/verifyimage` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/pfr/verifyimagestatus` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/pldm` | POST | - / no | UI / no | UI / no | - |
| `/api/maintenance/pldm/fwupdateprogress` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/pldm/fwupdatestatus` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/pldm/getpldmfwinfo` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/preserve` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/raid_management/flash` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/raid_management/preparecomponents` | PUT | - / no | UI / no | UI / no | - |
| `/api/maintenance/raid_management/upgradestatus` | GET | - / no | UI / no | UI / no | - |
| `/api/maintenance/reset` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/restore_config` | GET/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/maintenance/restore_defaults` | GET/PUT | UI / yes | UI / yes | UI / yes | B |
| `/api/maintenance/signed-image-key` | GET/POST | UI / yes | UI / no | UI / no | X |
| `/api/maintenance/system_admin` | GET/PUT | UI / yes | UI / yes | UI / yes | B |
| `/api/maintenance/upload_restore` | POST | UI / yes | UI / yes | UI / yes | - |
| `/api/ae_test` | PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/scripts` | GET/POST/PUT | UI / yes | UI / yes | UI / yes | - |
| `/api/tasks` | GET/POST (+item DELETE/PUT) | UI / yes | UI / yes | UI / yes | - |

---

## 7. Endpoint catalog

Each endpoint block lists: the boards whose UI references it; whether a handler is present in each board's firmware; whether it was seen in a HAR capture, with the captured request/response shape (key names and types only); the per-item path; query params; UI module(s); model; request fields (type / allowed values / validation); and response fields (from the capture where one exists, otherwise the fields the UI reads).


### Session & authentication (8)

#### `POST /api/asrr/session`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `LoginView`
- Request: `username (form, fixed "CERT_ADMIN")`, `password (form, empty)`, `certlogin (form, "1")`
- Response (fields read by UI): `CSRFToken`, `extendedpriv`, `ok`, `privilege`, `racsession_id`, `server_addr`
- Notes: ASRock variant used only when document.TRUSTED_CERT == 1 (index_trusted.html, client-certificate login). Same response as /api/session.

#### `GET /api/newtabinfo`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `app`, `LoginView`
- Response (fields read by UI): `channel`, `creation_time`, `extendedpriv`, `privilege`, `privilege_text`, `racsession_id`, `tfa_enaled`, `TFAEnabled`, `TFAStatus`, `user_id`, `username`

#### `POST /api/reset-pass`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `LoginView`
- Request: `param (B650 only)`, `Password (B650 only)`, `RetypePassword (B650 only)`, `username`, `verifyOTP (B650 only)`
- Response (fields read by UI): `code (B650 only)`
- Notes: Forgot-password flow. X470 sends form {username}; response code & 0xFF: 0 ok, 17 SMTP disabled, 18-20 SMTP failures, 3/16/21 e-mail errors. B650 adds Password, RetypePassword, param, verifyOTP.

#### `DELETE/POST /api/session`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har POST 200; b650d4u.har POST 200; b650d4u-2t.har POST 200
- UI: `app`, `viewer.min.js (H5Viewer)`, `LoginView`
- Request: `username (form)`, `password (form)`, `certlogin (form, X470 only: "0" | "1")`
- Response (fields read by UI): `ok (0 = success)`, `privilege (1 callback, 2 user, 3 operator, 4 admin, 5 OEM)`, `extendedpriv (bit0 = KVM, bit1 = VMedia)`, `racsession_id`, `remote_addr`, `server_name`, `server_addr`, `HTTPSEnabled`, `CSRFToken`, `user_id (B650 only)`, `channel (B650 only)`, `passwordStatus (B650 only)`, `TFAEnabled (B650 only)`, `TFAStatus (B650 only)`
  - Live (x470d4u.har): POST 200; request application/x-www-form-urlencoded `{username:form,password:form,certlogin:form}`; response `{ok:int,privilege:int,extendedpriv:int,racsession_id:int,remote_addr:str,server_name:str,server_addr:str,HTTPSEnabled:int,CSRFToken:str}`
  - Live (b650d4u.har): POST 200; request application/x-www-form-urlencoded `{username:form,password:form}`; response `{ok:int,privilege:int,user_id:int,extendedpriv:int,racsession_id:int,remote_addr:str,server_name:str,server_addr:str,HTTPSEnabled:int,CSRFToken:str,channel:int,passwordStatus:int}`
  - Live (b650d4u-2t.har): POST 200; request application/x-www-form-urlencoded `{username:form,password:form}`; response `{ok:int,privilege:int,user_id:int,extendedpriv:int,racsession_id:int,remote_addr:str,server_name:str,server_addr:str,HTTPSEnabled:int,CSRFToken:str,channel:int,passwordStatus:int}`
- Notes: POST body is application/x-www-form-urlencoded (jQuery default), not JSON. Response CSRFToken must be echoed as X-CSRFTOKEN on every later request; the server sets the QSESSIONID cookie. Error body {code, error}; code 15000 = MAX_NUM_SESSIONS_ALREADY_IN_USE. DELETE (no body) logs out; X470 sends it synchronously (async:false) from app.logout and on window unload. HAR-confirmed on X470.

#### `DELETE/GET/POST /api/tfa`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `app`
- Request: `channel`, `name`
- Response (fields read by UI): `codes`, `tfa_enaled`, `url`

#### `POST /api/updatenew_password`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `LoginView`
- Request: `password`, `username`
- Response (fields read by UI): `code`

#### `POST /api/verify_otp`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `LoginView`
- Request: `channel`, `username`, `verifycode`

#### `POST /api/viewerlogin`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `viewer.min.js login (viewer.html#<token>)`
- Request: `token`
- Response (fields read by UI): `CSRFToken`, `extendedpriv`, `ok`, `privilege`, `racsession_id`, `server_addr`
- Notes: H5Viewer standalone login: viewer.html#<token> (or ?<token>); body is form-encoded {token}. Same response shape as /api/session; sets X-CSRFTOKEN/garc. Lets a KVM window be opened without the main UI session.


### Feature discovery & UI configuration (5)

#### `GET /api/configuration/project`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `viewer.min.js (H5Viewer)`
- Model: `collection/project-configurations`
- Response fields (from capture + UI): `feature`
  - Live (x470d4u.har): GET 200; response `[{feature:str},…54 items]`
  - Live (b650d4u-2t.har): GET 200; response `[{feature:str},…83 items]`
- Notes: Static compile-time feature list; drives data-feature visibility. HAR-confirmed.

#### `GET /api/configuration/runtime`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200
- UI: `viewer.min.js (H5Viewer)`
- Model: `collection/runtime-configurations`
- Response fields (from capture + UI): `enabled`, `feature`
  - Live (x470d4u.har): GET 200; response `[{feature:str,enabled:int},…5 items]`
- Notes: Runtime-toggleable features (LDAP, AD, RADIUS, SINGLE_PORT_APP, SD_MEDIA_SUPPORT ...). HAR-confirmed.

#### `GET /api/extfeatures`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `appext`

#### `GET /api/settings/user-preference`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `dashboard/StatsView`
- Model: `models/user_config`

#### `GET /api/status/uptime`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `dashboard/StatsView`
- Model: `models/app_config`
- Response fields (from capture + UI): `minutes_per_count`, `poh_counter_reading`
  - Live (b650d4u.har): GET 200; response `{minutes_per_count:int,poh_counter_reading:int}`
  - Live (b650d4u-2t.har): GET 200; response `{minutes_per_count:int,poh_counter_reading:int}`
- Notes: Power-on-hours counter (HAR-confirmed on B650D4U/B550D4U-2T).


### Dashboard & identity (9)

#### `GET /api/asrr/fw-info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `appext`, `dashboard/SummaryInfoView`, `dashboard/SummaryView`
- Response fields (from capture + UI): `BIOS_fw_version`, `BMC_fw_version`, `BP_version (only in b650d4u.har)`, `CPLD_version`, `ME_fw_version`, `Micro_Code_version`, `PSP_fw_version (only in b650d4u-2t.har, b650d4u.har)`, `BPCPLD_version (read by B650 UI; absent from capture)`
  - Live (x470d4u.har): GET 200; response `{BMC_fw_version:str,BIOS_fw_version:str,ME_fw_version:str,Micro_Code_version:str,CPLD_version:str}`
  - Live (b650d4u.har): GET 200; response `{BMC_fw_version:str,BIOS_fw_version:str,ME_fw_version:str,PSP_fw_version:str,Micro_Code_version:str,CPLD_version:str,BP_version:str}`
  - Live (b650d4u-2t.har): GET 200; response `{BMC_fw_version:str,BIOS_fw_version:str,ME_fw_version:str,PSP_fw_version:str,Micro_Code_version:str,CPLD_version:str}`
- Notes: HAR-confirmed on X470: BMC_fw_version, BIOS_fw_version, ME_fw_version, Micro_Code_version, CPLD_version.

#### `GET /api/asrr/model-name`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `appext`, `dashboard/SummaryInfoView`, `dashboard/SummaryView`
- Response fields (from capture + UI): `mb_name`, `sys_name (only in b650d4u-2t.har, b650d4u.har)`
  - Live (x470d4u.har): GET 200; response `{mb_name:str}`
  - Live (b650d4u.har): GET 200; response `{mb_name:str,sys_name:str}`
  - Live (b650d4u-2t.har): GET 200; response `{mb_name:str,sys_name:str}`
- Notes: HAR-confirmed {"mb_name":"X470D4U"}.

#### `GET /api/asrr/product-name`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: x470d4u.har GET 500
- UI: `dashboard/SummaryInfoView`
- Response (fields read by UI): `sys_name`
  - Live (x470d4u.har): GET 500; response `{error:str,code:int}`
- Notes: Returned HTTP 500 {"error":"Error in Getting Model Name","code":16005} in the X470 HAR (no product FRU).

#### `GET /api/asrr/selsummary`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200
- UI: `dashboard/SummaryLogView`
- Response fields (from capture + UI): `freeentries`, `selentries`
  - Live (x470d4u.har): GET 200; response `{selentries:int,freeentries:int}`
- Notes: Dashboard SEL usage donut. HAR-confirmed {"selentries":489,"freeentries":3150}.

#### `GET /api/dashboardsensors`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200
- UI: `DashboardView`, `dashboard/LogDetailsView`, `dashboard/QuickSensorView`
- Model: `collection/dashboard_sensors`
- Response fields (from capture + UI): `accessible`, `discrete_state`, `higher_critical_threshold`, `higher_non_critical_threshold`, `higher_non_recoverable_threshold`, `id`, `lower_critical_threshold`, `lower_non_critical_threshold`, `lower_non_recoverable_threshold`, `name`, `owner_id`, `owner_lun`, `raw_reading`, `reading`, `sensor_number`, `sensor_state`, `settable_flag`, `type`, `type_number`, `unit`, `pending_deassert (read by UI; absent from capture)`
  - Live (b650d4u.har): GET 200; response `[{id:int,sensor_number:int,name:str,owner_id:int,owner_lun:int,raw_reading:int,type:str,type_number:int,reading:int,sensor_state:int,discrete_state:int,accessible:int,settable_flag:int,lower_non_recoverable_threshold:str,lower_critical_threshold:str,lower_non_critical_threshold:int,higher_non_critical_threshold:str,higher_critical_threshold:str,higher_non_recoverable_threshold:str,unit:str},…59 items]`

#### `GET/PUT /api/firmware-info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200
- UI: `app`, `maintenance/firmware_information`, `maintenance/fwinfo`
- Model: `models//firmware_information`, `models//fwinfo`, `models/firmware_information`
- Response fields (from capture + UI): `active_image`, `aux`, `date`, `dev_support`, `device_id`, `fw_rev_1`, `fw_rev_2`, `fw_ver`, `ipmi_version`, `mfg_id_0`, `mfg_id_1`, `mfg_id_2`, `prod_id`, `revision`, `time`
  - Live (x470d4u.har): GET 200; response `{device_id:int,revision:int,fw_rev_1:int,fw_rev_2:int,ipmi_version:int,dev_support:int,mfg_id_0:int,mfg_id_1:int,mfg_id_2:int,prod_id:int,aux:int,fw_ver:str,date:str,time:str,active_image:int}`
- Notes: HAR-confirmed: device_id, revision, fw_rev_1, fw_rev_2, ipmi_version, dev_support, mfg_id_0..2, prod_id, aux, fw_ver, date, time, active_image. The PUT comes from a generic save() handler and is not reachable from the page.

#### `GET /api/logs/dashboardevent`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `dashboard/LogDetailsView`
- Model: `collection/summarylog`
- Response fields (from capture + UI): `dashboard_event_log_count`, `id`, `pending_deassert`, `record_type`, `sensor_name`, `sensor_number`, `timestamp`
  - Live (b650d4u.har): GET 200; response `[{id:int,record_type:str,timestamp:int,sensor_name:str,sensor_number:int,pending_deassert:int,dashboard_event_log_count:int},…62 items]`
  - Live (b650d4u-2t.har): GET 200; response `[{id:int,record_type:str,timestamp:int,sensor_name:str,sensor_number:int,pending_deassert:int,dashboard_event_log_count:int},…62 items]`

#### `GET /api/settings/dashboardinfo`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200
- UI: `LoginView`
- Request: `channel_id`, `user_id`, `user_name`
- Response fields (from capture + UI): `active_image`, `audit_count`, `aux`, `bios_ver`, `creation_time`, `dashboardntp_id`, `date`, `dev_support`, `device_id`, `fw_rev_1`, `fw_rev_2`, `fw_ver`, `id`, `ipmi_version`, `mfg_id_0`, `mfg_id_1`, `mfg_id_2`, `minutes_per_count`, `poh_counter_reading`, `privilege`, `prod_id`, `revision`, `time`, `timezone`, `utc_minutes`
  - Live (b650d4u.har): GET 200; query keys `channel_id&user_id&user_name`; response `{id:int,privilege:str,creation_time:int,dashboardntp_id:int,utc_minutes:int,audit_count:int,timezone:str,device_id:int,revision:int,fw_rev_1:int,fw_rev_2:int,ipmi_version:int,dev_support:int,mfg_id_0:int,mfg_id_1:int,mfg_id_2:int,prod_id:int,aux:int,fw_ver:str,bios_ver:str,date:str,time:str,active_image:int,minutes_per_count:int,poh_counter_reading:int}`

#### `GET /api/settings/dashboardntp`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `DashboardView`
- Model: `models/datetime/dashboard_ntp`


### Power, chassis & UID (13)

#### `POST /api/actions/accessbiossetup`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `viewer.min.js (H5Viewer)`, `PowerControlView`
- Notes: No body (Content-Type application/json). Sets the boot-to-BIOS-setup flag; the UI follows it with a power on/cycle. Used by B650 PowerControlView and by the H5Viewer power menu.

#### `POST /api/actions/chassis-led`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Model: `models/chassis_status`, `models/uidcontrol`
- Request: `uid_command (int: 0 off, 1 blink, 2 on)`, `blink_time (B650 only, models/chassis_status.led())`, `force_on (B650 only)`
- Notes: JSON body. X470 and B650 UI (miscellaneous/uidcontrolView via models/uidcontrol.uidcontrol()) send {"uid_command":N}. B650 models/chassis_status also defines led(blink_time, force_on) for the IPMI Chassis Identify form.

#### `POST /api/actions/power`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har POST 200
- UI: `maintenance/NVMeMISSDSControllerFirmwareUpdateView`
- Model: `models/chassis_status`, `models/uidcontrol`
- Request: `power_command (int)`
- Response fields (from capture + UI): `power_command`
  - Live (x470d4u.har): POST 200; request application/json `{power_command:int}`; response `{power_command:int}`
- Notes: JSON body {"power_command":N}. 0 power off (hard), 1 power on, 2 power cycle, 3 hard reset, 5 ACPI/soft shutdown (orderly). B650 UI also offers 6 = boot to BIOS setup, implemented client-side as POST /api/actions/accessbiossetup followed by power_command 1 (if off) or 2 (if on); 6 is never sent. UI then re-reads /api/chassis-status after 10 s (20 s for cycle, 30 s for ACPI), up to 3 retries, expecting power_status 0 for commands 0/5 and 1 for 1/2/3. Only shown to Administrator. HAR-confirmed: request {"power_command":1} -> {"power_command":1}. UI: PowerControlView (models/chassis_status.power()).

#### `GET /api/asrr/GetPowerRestore`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/power_restore`
- Response (fields read by UI): `powerRestore`

#### `GET /api/asrr/PowerSource`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200
- UI: `PowerSourceView`
- Response fields (from capture + UI): `ac_input_current`, `ac_input_current_fault`, `ac_input_power`, `ac_input_power_fault`, `ac_input_voltage`, `ac_input_voltage_fault`, `dc_12v_output_current`, `dc_12v_output_current_fault`, `dc_12v_output_power`, `dc_12v_output_power_fault`, `dc_12v_output_voltage`, `dc_12v_output_voltage_fault`, `fan1`, `fan1_fault`, `fan2`, `fan2_fault`, `power_supply_status`, `pws_serial_number`, `temperature1`, `temperature1_fault`, `temperature2`, `temperature2_fault`, `psu_id (read by B650 UI; absent from capture)`, `psu_model (read by B650 UI; absent from capture)`, `psu_revision (read by B650 UI; absent from capture)`, `psu_serial_number (read by B650 UI; absent from capture)`
  - Live (x470d4u.har): GET 200; response `[{power_supply_status:int,ac_input_voltage:str,ac_input_voltage_fault:int,ac_input_current:str,ac_input_current_fault:int,dc_12v_output_voltage:str,dc_12v_output_voltage_fault:int,dc_12v_output_current:str,dc_12v_output_current_fault:int,temperature1:str,temperature1_fault:int,temperature2:str,temperature2_fault:int,fan1:str,fan1_fault:int,fan2:str,fan2_fault:int,dc_12v_output_power:str,dc_12v_output_power_fault:int,ac_input_power:str,ac_input_power_fault:int,pws_serial_number:str},…4 items]`
- Notes: Array of PSU objects (PMBus). HAR-confirmed on X470 (values "N/A" without a PMBus PSU).

#### `GET /api/asrr/PowerSourceID`
- Boards (UI references): B650-7.02
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 no
- Seen in HAR: no
- UI: `SystemInformationView`
- Response (fields read by UI): `thisPlatform`

#### `POST /api/asrr/SetPowerRestore`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/power_restore`
- Request: `powerRestore`

#### `PUT /api/asrr/bmccoldreset`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/ColdResetView`
- Notes: PUT with data:{} (empty body). Cold-resets the BMC; UI logs out after a delay.

#### `GET /api/asrr/getprochot`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/prochot`
- Model: `models/prochot`
- Response (fields read by UI): `prochot`

#### `PUT /api/asrr/setprochot`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/prochot`
- Request: `prochot`

#### `GET /api/chassis`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: x470d4u.har GET 200
- UI: `app`, `PowerSourceView`
- Model: `models/chassis_status_poll`
- Response fields (from capture + UI): `led_status`, `power_status`
  - Live (x470d4u.har): GET 200; response `{power_status:int,led_status:int}`
- Notes: X470 only. Polled by app every 5 s (models/chassis_status_poll) to drive the header host/UID icons. HAR-confirmed: {"power_status":0,"led_status":0}.

#### `GET /api/chassis-status`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `DashboardView`, `PowerControlView`, `maintenance/BiosUpdateView`, `maintenance/BpUpdateView`, `maintenance/CpldUpdateView`, `maintenance/FpgaUpdateView`, `maintenance/PsuUpdateView`, `maintenance/RetimerUpdateView`, `miscellaneous/uidcontrolView`
- Model: `models/chassis_status`, `models/uidcontrol`
- Response fields (from capture + UI): `led_status`, `power_cycle_interval (only in b650d4u.har)`, `power_status`, `power_on_after_flash (read by B650 UI; absent from capture)`
  - Live (x470d4u.har): GET 200; response `{power_status:int,led_status:int}`
  - Live (b650d4u.har): GET 200; response `{power_status:int,led_status:int,power_cycle_interval:int}`
  - Live (b650d4u-2t.har): GET 200; response `{power_status:int,led_status:int}`
- Notes: GET only (models/chassis_status). On B650 this model is also the 5 s header poll (App.Chassis). HAR-confirmed on X470. B650D4U HAR: {"power_status":1,"led_status":0,"power_cycle_interval":2000}.

#### `GET/PUT /api/settings/amdpowercap`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/amdpowercap`
- Model: `models//amdpowercap`
- Request: `enable_powercap`, `power_cap_value`
- Response (fields read by UI): `current_power_consumption`, `enable_powercap`, `maximum_power_cap_value`, `power_cap_value`
  - `power_cap_value` validation: custom-fn; msg=max_power_cap_depends_on_validation / power_cap_value


### Sensors & SDR (6)

#### `GET /api/detail_sensors_readings`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `app`
- Model: `collection/sensors-reading`
- Response (fields read by UI): `reading (B650 only)`

#### `GET /api/pldm/sensors`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `pldm/PLDMSensorView`
- Model: `collection/pldm_numeric_sensors`

#### `GET /api/pldm/state_sensors`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `pldm/PLDMSensorView`
- Model: `collection/pldm_state_sensors`

#### `GET /api/sdr`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `EventLogView`, `settings/fan/fan_tscft`
- Model: `collection/sdr`
- Response fields (from capture + UI): `id`, `name`, `owner_lun (only in b650d4u-2t.har)`, `sensor_number`, `type`, `type_number`
  - Live (x470d4u.har): GET 200; response `[{id:int,sensor_number:int,name:str,type:str,type_number:int},…42 items]`
  - Live (b650d4u-2t.har): GET 200; response `[{id:int,sensor_number:int,name:str,owner_lun:int,type:str,type_number:int},…58 items]`
- Notes: HAR-confirmed array {id, sensor_number, name, type, type_number}.

#### `GET /api/sensors`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `app`, `EventLogView`, `PowerSourceView`, `SelEventView`, `SensorDetailsView`, `SensorView`, `TimelineItemView`, `dashboard/DashboardSensorView`, `dashboard/QuickSensorView`, `host_interface_info`, `settings/pef/event_filters_edit_item`
- Model: `collection//sensors`, `collection/sensors`
- Request: `severity (B650 only)`
- Response fields (from capture + UI): `accessible`, `discrete_state`, `higher_critical_threshold`, `higher_non_critical_threshold`, `higher_non_recoverable_threshold`, `id`, `lower_critical_threshold`, `lower_non_critical_threshold`, `lower_non_recoverable_threshold`, `name`, `owner_id`, `owner_lun`, `raw_reading`, `reading`, `sensor_number`, `sensor_state`, `settable_flag (only in b650d4u-2t.har, b650d4u.har)`, `settable_readable_threshMask (only in x470d4u.har)`, `type`, `type_number`, `unit`
  - Correction: static JS analysis also listed `EventData1`, `EventData1_Description`, `EventData2`, `EventData2_Description`, `EventData3`, `EventData3_Description`, `EventDirType`, `GenID1`, `GenID2`, `MaxReading`, `advanced_event_description`, `event_description`, `event_direction`, `event_direction_table`, `event_reading_class`, `ipmb_lun`, `record_type`, `sel_data`, `sensor_name`, `sensor_type`, `sensor_type_num`, `sensor_type_table`, `timestamp`; these belong to sibling models and are not returned by this endpoint (capture).
  - Live (x470d4u.har): GET 200; response `[{id:int,sensor_number:int,name:str,owner_id:int,owner_lun:int,raw_reading:float,type:str,type_number:int,reading:float,sensor_state:int,discrete_state:int,settable_readable_threshMask:int,lower_non_recoverable_threshold:float,lower_critical_threshold:float,lower_non_critical_threshold:float,higher_non_critical_threshold:float,higher_critical_threshold:float,higher_non_recoverable_threshold:float,accessible:int,unit:str},…41 items]`
  - Live (b650d4u.har): GET 200; response `[{id:int,sensor_number:int,name:str,owner_id:int,owner_lun:int,raw_reading:int,type:str,type_number:int,reading:float|int,sensor_state:int,discrete_state:int,accessible:int,settable_flag:int,lower_non_recoverable_threshold:float|str,lower_critical_threshold:float|str,lower_non_critical_threshold:int|str,higher_non_critical_threshold:int|str,higher_critical_threshold:float|int|str,higher_non_recoverable_threshold:float|str,unit:str},…59 items]`
  - Live (b650d4u-2t.har): GET 200; response `[{id:int,sensor_number:int,name:str,owner_id:int,owner_lun:int,raw_reading:int,type:str,type_number:int,reading:float|int,sensor_state:int,discrete_state:int,accessible:int,settable_flag:int,lower_non_recoverable_threshold:float|int|str,lower_critical_threshold:float|str,lower_non_critical_threshold:int|str,higher_non_critical_threshold:int|str,higher_critical_threshold:float|int|str,higher_non_recoverable_threshold:float|str,unit:str},…51 items]`
  - `sensor_type` values: `Any=`, `temperature=`, `fan=`, `voltage=`, `discrete=`
  - `ipmb_lun` values: `0=`, `1=`, `2=`, `3=`
- Notes: HAR-confirmed array: id, sensor_number, name, owner_id, owner_lun, raw_reading, type, type_number, reading, sensor_state, discrete_state, settable_readable_threshMask, lower/higher_*_threshold, accessible (213 = not accessible), unit. Polled every 10 s on #dashboard and #sensors.

#### `GET/PUT /api/settings/sensor-threshold`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/sensor_thresholds`
- Model: `models//sensor_thresholds`
- Request: `higher_critical_threshold`, `higher_critical_threshold_enable (X470 only)`, `higher_non_critical_threshold`, `higher_non_critical_threshold_enable (X470 only)`, `higher_non_recoverable_threshold`, `higher_non_recoverable_threshold_enable (X470 only)`, `lower_critical_threshold`, `lower_critical_threshold_enable (X470 only)`, `lower_non_critical_threshold`, `lower_non_critical_threshold_enable (X470 only)`, `lower_non_recoverable_threshold`, `lower_non_recoverable_threshold_enable (X470 only)`, `retain_threshold_value (B650 only)`, `Settable_Readable_ThreshMask (X470 only)`
- Response (fields read by UI): `higher_critical_threshold`, `higher_critical_threshold_enable (X470 only)`, `higher_non_critical_threshold`, `higher_non_critical_threshold_enable (X470 only)`, `higher_non_recoverable_threshold`, `higher_non_recoverable_threshold_enable (X470 only)`, `id (B650 only)`, `lower_critical_threshold`, `lower_critical_threshold_enable (X470 only)`, `lower_non_critical_threshold`, `lower_non_critical_threshold_enable (X470 only)`, `lower_non_recoverable_threshold`, `lower_non_recoverable_threshold_enable (X470 only)`, `name`, `owner_lun (B650 only)`, `sensor_number`, `sensor_type_number`, `settable_flag (B650 only)`, `Settable_Readable_ThreshMask (X470 only)`
  - `higher_non_recoverable_threshold` validation: custom-fn; msg=higher_non_recoverable_threshold_enable / invalid_upper_nonrecoverable_value_refer_help_content
  - `higher_critical_threshold` validation: custom-fn; msg=higher_critical_threshold_enable / higher_non_recoverable_threshold / higher_non_recoverable_threshold_enable / invalid_upper_critical_value_or_upper_critical_value_cannot_be_bigger_than_upper_nonrecoverabl
  - `higher_non_critical_threshold` validation: custom-fn; msg=higher_critical_threshold / higher_critical_threshold_enable / higher_non_critical_threshold_enable / invalid_upper_noncritical_value_or_upper_noncritical_value_cannot_be_bigger_than_upper_critical_value
  - `lower_non_critical_threshold` validation: custom-fn; msg=higher_non_critical_threshold / higher_non_critical_threshold_enable / invalid_lower_noncritical_value_or_lower_noncritical_value_cannot_be_bigger_than_upper_noncritical_value / lower_non_critical_threshold
  - `lower_critical_threshold` validation: custom-fn; msg=invalid_lower_critical_value_or_lower_critical_value_cannot_be_bigger_than_lower_noncritical_value / lower_critical_threshold_enable / lower_non_critical_threshold / lower_non_critical_threshold_enable
  - `lower_non_recoverable_threshold` validation: custom-fn; msg=invalid_lower_nonrecoverable_value_or_lower_nonrecoverable_value_cannot_be_bigger_than_lower_critical_value / lower_critical_threshold / lower_critical_threshold_enable / lower_non_recoverable_threshold_ena


### FRU & PLDM inventory (6)

#### `GET /api/fru`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200
- UI: `FRUView`
- Model: `collection/fru`
- Response fields (from capture + UI): `board`, `chassis`, `common_header`, `device`, `product`
  - Live (x470d4u.har): GET 200; response `[{device:{id:int,name:str},common_header:{version:int,internal_use_area_start_offset:int,chassis_info_area_start_offset:int,board_info_area_start_offset:int,product_info_area_start_offset:int,multi_record_area_start_offset:int},chassis:{version:int,length:int,type:str,part_number:str,serial_number:str,custom_fields:str},board:{version:int,length:int,language:int,date:str,manufacturer:str,product_name:str,serial_number:str,part_number:str,fru_file_id:str,custom_fields:str},product:{version:int,length:int,language:int,manufacturer:str,product_name:str,part_number:str,product_version:str,serial_number:str,asset_tag:str,fru_file_id:str,custom_fields:str}}]`

#### `GET /api/frueditable`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `FRUView`

#### `PUT /api/fruupdate`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `FRUView`

#### `GET /api/pldm/eids`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `pldm/PLDMFRUView`, `pldm/PLDMSensorView`
- Model: `collection/pldm_eids`

#### `GET /api/pldm/entity_association`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `pldm/PLDMSensorView`
- Model: `collection/pldm_entity_association`

#### `GET /api/pldm/fru`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `pldm/PLDMFRUView`
- Model: `collection/pldm_fru`
- Response (fields read by UI): `eid`, `fru_data_version`, `fru_field`, `fru_record_set_id`, `fru_table_max_size`


### Logs, crash dumps & diagnostics (35)

#### `GET /api/acd/download_crashdump`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ACDView`

#### `GET /api/acd/getcrashdumpstatus`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ACDView`
- Response (fields read by UI): `acd_monitor_status`

#### `PUT /api/acd/set_crashdump_download`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ACDView`

#### `PUT /api/acd/setcrashdumpstatus`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ACDView`
- Request: `acd_monitor_status`

#### `DELETE /api/asrr/clearmcalog`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `EventLogView`
- Notes: DELETE, no body.

#### `GET /api/asrr/getmcalog`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `EventLogView`
- Response fields (from capture + UI): `MCADump`
  - Live (x470d4u.har): GET 200; response `{MCADump:int}`
  - Live (b650d4u-2t.har): GET 200; response `{MCADump:int}`
- Notes: Shows the MCA log save/clear buttons when MCADump is non-zero. HAR-confirmed.

#### `GET /api/asrr/mca-file`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `EventLogView`
- Notes: Text download (MCALog).

#### `GET/PUT /api/host/addc_autonomous_debug_data_collection/error_recovery_mode_and_retry_count`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ADDCView`
- Model: `models//addc`
- Request: `addc_error_recovery_mode`, `addc_retry_count`
- Response (fields read by UI): `addc_error_recovery_mode`, `addc_retry_count`

#### `GET /api/host/download_addc_autonomous_debug_data_collection`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Query: `?file=…`
- UI: `ADDCView`

#### `GET /api/host/get_addc_autonomous_debug_data_collection`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ADDCView`

#### `GET /api/host/get_crash_dump`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ACDView`

#### `GET /api/logs/acd`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ACDView`

#### `GET /api/logs/audit`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `AuditLogView`, `dashboard/StatsView`
- Model: `collection/audit-log`
- Request: `cmd`
- Response fields (from capture + UI): `audit_count (only in b650d4u.har)`, `hostname (only in b650d4u-2t.har)`, `id (only in b650d4u-2t.har)`, `message (only in b650d4u-2t.har)`, `timestamp (only in b650d4u-2t.har)`
  - Live (b650d4u.har): GET 200; query keys `cmd`; response `[{audit_count:int}]`
  - Live (b650d4u-2t.har): GET 200; response `[{id:int,timestamp:int,hostname:str,message:str},…29 items]`

#### `GET /api/logs/debuglogs`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `DebugLogView`

#### `DELETE/GET /api/logs/event`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `DELETE /api/logs/event/:id` (Backbone collection model, `:id` = item `id`)
- UI: `/system_inventory_info_item`, `EventLogView`, `SensorDetailsView`, `dashboard/LogDetailsView`, `dashboard/QuickSensorItemView`, `system_inventory_info`
- Model: `collection/event-log`
- Response (fields read by UI): `event_description`, `event_direction (X470 only)`, `event_format_ipmi_version (X470 only)`, `event_reading_class (X470 only)`, `EventData1 (X470 only)`, `EventData1_Description (X470 only)`, `EventData1_Description_raw (X470 only)`, `EventData2 (X470 only)`, `EventData2_Description (X470 only)`, `EventData2_Description_raw (X470 only)`, `EventData3 (X470 only)`, `EventData3_Description (X470 only)`, `EventData3_Description_raw (X470 only)`, `EventDirType (X470 only)`, `GenID1 (X470 only)`, `GenID2 (X470 only)`, `id`, `name`, `owner_lun (B650 only)`, `reading (B650 only)`, `record_type (X470 only)`, `record_type_num (X470 only)`, `sensor_name (X470 only)`, `sensor_number`, `sensor_type (X470 only)`, `sensor_type_num (X470 only)`, `timestamp (X470 only)`
- Notes: GET = SEL (collection/event-log; B650 polls it every 15 s with ?LASTEVENTID=n). DELETE /api/logs/event (no id) clears the SEL. Per-item DELETE is Backbone boilerplate.

#### `GET /api/logs/event-file`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `EventLogView`
- Notes: Text download of the decoded SEL (saved as SELLog).

#### `GET /api/logs/eventlog`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `EventLogView`
- Response fields (from capture + UI): `advanced_event_description (only in x470d4u.har)`, `channel_number`, `event_description`, `event_direction`, `event_direction_table (only in b650d4u-2t.har)`, `event_format_ipmi_version`, `event_log_count (only in b650d4u-2t.har)`, `event_reading_class`, `EventData1`, `EventData1_Description`, `EventData1_Description_raw (only in x470d4u.har)`, `EventData2`, `EventData2_Description`, `EventData2_Description_raw (only in x470d4u.har)`, `EventData3`, `EventData3_Description`, `EventData3_Description_raw (only in x470d4u.har)`, `EventDirType`, `generator_type`, `GenID1`, `GenID2`, `id`, `ipmb_lun`, `pci_bus_num (only in x470d4u.har)`, `pci_device_num (only in x470d4u.har)`, `pci_function_num (only in x470d4u.har)`, `record_type`, `record_type_num`, `record_type_table (only in b650d4u-2t.har)`, `sensor_name`, `sensor_number`, `sensor_reading_value`, `sensor_type`, `sensor_type_num`, `sensor_type_table (only in b650d4u-2t.har)`, `system_software_type`, `timestamp`, `triggered_value`
  - Live (x470d4u.har): GET 200; response `[{id:int,record_type:str,timestamp:int,GenID1:int,GenID2:int,event_format_ipmi_version:int,sensor_type:str,sensor_name:str,sensor_number:int,generator_type:str,channel_number:int,ipmb_lun:int,event_direction:str,sensor_reading_value:int,triggered_value:int,record_type_num:int,sensor_type_num:int,EventDirType:int,EventData1:int,EventData2:int,EventData3:int,event_reading_class:str,pci_bus_num:int,pci_device_num:int,pci_function_num:int,event_description:str,advanced_event_description:str,EventData1_Description:str,EventData2_Description:str,EventData3_Description:str,EventData1_Description_raw:str,EventData2_Description_raw:str,EventData3_Description_raw:str,system_software_type:str},…489 items]`
  - Live (b650d4u-2t.har): GET 200; query keys `EVENTID`; response `[{id:int,record_type:str,timestamp:int,GenID1:int,GenID2:int,event_format_ipmi_version:int,sensor_type:str,sensor_type_table:int,sensor_name:str,sensor_number:int,generator_type:str,channel_number:int,ipmb_lun:int,event_direction:str,sensor_reading_value:int,triggered_value:int,record_type_num:int,sensor_type_num:int,EventDirType:int,EventData1:int,EventData2:int,EventData3:int,record_type_table:int,event_direction_table:int,event_reading_class:str,event_description:str,EventData1_Description:str,EventData2_Description:str,EventData3_Description:str,system_software_type:str,event_log_count:int},…62 items]`
- Notes: X470 EventLogView fetches the SEL from here (collection.fetch({url:"/api/logs/eventlog"})). HAR-confirmed array of SEL records (id, record_type, timestamp, GenID1, GenID2, sensor_type, sensor_name, sensor_number, event_direction, ...).

#### `POST /api/logs/getvideotag`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `VideoLogItemView`
- Request: `file_name`
- Response (fields read by UI): `CompleteTag`

#### `GET /api/logs/help-data`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `viewer.min.js RemoteControlView (help/about text, logo)`
- Request: `file (query)`
- Response (fields read by UI): `(text or image/jpeg blob)`

#### `GET /api/logs/platform`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 no
- Seen in HAR: no
- UI: `PlatformLogView`
- Model: `collection/platform-log`
- Response (fields read by UI): `timestamp`

#### `GET /api/logs/postcode`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `PostcodeLogView`
- Model: `collection/postcode-log`

#### `GET /api/logs/raw-event-file`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `EventLogView`
- Notes: Text download of the raw SEL (saved as RawSELLog).

#### `GET /api/logs/savepostcode`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `PostcodeLogView`

#### `GET /api/logs/system`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `SystemLogView`
- Model: `collection/system-log`
- Response (fields read by UI): `timestamp`

#### `GET /api/logs/video`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200
- UI: `VideoLogView`
- Model: `collection/video-log`
- Response (fields read by UI): `timestamp`
  - Live (x470d4u.har): GET 200; response `[]`

#### `GET /api/logs/video-data`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Query: `?file=…`
- UI: `VideoLogItemView`
- Notes: Recorded-video binary (arraybuffer), ?file=<name>. HTTP 12012 = file too large for the HTML5 player.

#### `GET /api/logs/video-log`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Query: `?file_name=…`
- UI: `VideoLogItemView`

#### `DELETE /api/logs/video-log/delete`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `VideoLogItemView`
- Request: `file_name`

#### `GET /api/pldm/get_all_event_log`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `pldm/PLDMEventLogView`
- Model: `collection/pldm_event_log`

#### `GET/PUT /api/settings/log`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/log/advanced_log`
- Model: `models//advanced_log`
- Request: `audit_log`, `file_size`, `local (B650 only)`, `port`, `port_type (B650 only)`, `remote`, `rotate_count`, `server_addr`, `system_log`
- Response (fields read by UI): `audit_log`, `file_size`, `local (B650 only)`, `port`, `port_type (B650 only)`, `remote`, `rotate_count`, `server_addr`, `system_log`
  - `remote` values: `1=advanced_loglocal_log`, `2=advanced_logremote_log`
  - `port_type` values: `1=advanced_logudp`, `2=advanced_logtcp`
  - `server_addr` validation: custom-fn; msg=invalid_remote_log_server_address_refer_help_for_more_information
  - `port` validation: custom-fn; msg=invalid_port_refer_help_for_more_information
  - `file_size` validation: custom-fn; msg=invalid_file_size_refer_help_for_more_information
  - `rotate_count` validation: custom-fn; msg=invalid_rotate_count_value_refer_help_for_more_information
  - `local` validation: custom-fn; msg=either_one_of_the_log_is_mandatory
  - `port_type` validation: custom-fn; msg=either_one_of_the_port_is_mandatory

#### `POST /api/settings/log-certificates`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/log/advanced_log`
- Request: `multipart:ca_certificate`

#### `GET/PUT /api/settings/log-policy`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/log/SEL_log_settings_policy`, `settings/log/general_log`
- Model: `models//SEL_log_settings_policy`, `models//general_log`
- Request: `policy`
- Response (fields read by UI): `policy`
  - `policy` values: `0=general_loglinear_storage_policy`, `1=general_logcircular_storage_policy`

#### `GET /api/system_diagnostics/download_system_diagnostics_log`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Query: `?file=…`
- UI: `maintenance/SystemDiagnosticsView`

#### `GET /api/system_diagnostics/generate_system_diagnostics_logs`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/SystemDiagnosticsView`

#### `GET /api/system_diagnostics/log`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/SystemDiagnosticsView`
- Response (fields read by UI): `SD_CARD_NOT_FOUND`


### Host inventory (28)

#### `GET /api/gpu/nvfpga`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `NvGpuView`
- Response (fields read by UI): `partnum`, `product`, `serialnum`, `version`

#### `GET /api/gpu/nvgpu`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `NvGpuView`
- Model: `collection/nvgpu`
- Response (fields read by UI): `dbe`, `did`, `guid`, `id`, `inforom`, `name`, `reset`, `sbe`, `sn`, `vbios`, `vid`

#### `GET /api/gpu/status`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `dashboard/SummaryView`

#### `GET /api/gpu_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/RetimerUpdateView`
- Response (fields read by UI): `retimer_ver_slot4`, `retimer_ver_slot5`, `retimer_ver_slot6`, `retimer_ver_slot7`

#### `GET /api/host/asd/getasdjtaginfo`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ASDView`
- Response (fields read by UI): `asd_jtag_test_logs`

#### `GET /api/host/asd/getasdremotedebuginfo`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ASDView`
- Response (fields read by UI): `service_status`, `tls_certificate_file`, `tls_status`, `version`

#### `PUT /api/host/asd/setasdremotedebuginfo`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ASDView`
- Request: `service_status`

#### `POST /api/host/asd/tls_certificate`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `ASDView`
- Request: `multipart:asd_tls_certificate`, `service_status`

#### `GET /api/host_inventory/host_interface_baseboard_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200
- UI: `host_interface_info`
- Response fields (from capture + UI): `baseboard_info`, `NetworkInterface`
  - Live (b650d4u.har): GET 200; response `[{baseboard_info:[{Name:str,Description:str,FirmwareVersion:str,Model:str,State:str,PowerState:str}],NetworkInterface:[{id:int,Name:str,MACAddress:str,InterfaceEnabled:str,IPv4Addresses:str,HostName:str,State:str,FullDuplex:str,PermanentMACAddress:str,WWPN:str},…4 items]}]`

#### `GET /api/host_inventory/host_interface_memory_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `host_interface_info`
- Model: `collection/host_interface_memory_info`
- Response fields (from capture + UI): `AllowedSpeedMHz`, `CapacityMiB`, `Description`, `DeviceLocator`, `Id`, `Manufacturer`, `MemoryDeviceType`, `Name`, `OperatingSpeedMhz`, `PartNumber`, `SerialNumber`, `State`
  - Live (x470d4u.har): GET 200; response `[]`
  - Live (b650d4u.har): GET 200; response `[]`
  - Live (b650d4u-2t.har): GET 200; response `[{Id:str,Name:str,CapacityMiB:str,Manufacturer:str,SerialNumber:str,PartNumber:str,State:str,DeviceLocator:str,OperatingSpeedMhz:str,Description:str,MemoryDeviceType:str,AllowedSpeedMHz:str},…4 items]`

#### `GET /api/host_inventory/host_interface_pcie_device_function_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `host_interface_info`
- Response fields (from capture + UI): `PCIE_Devices`, `PCIE_Function`
  - Live (x470d4u.har): GET 200; response `[{PCIE_Devices:[],PCIE_Function:[]}]`
  - Live (b650d4u.har): GET 200; response `[{PCIE_Devices:[],PCIE_Function:[]}]`
  - Live (b650d4u-2t.har): GET 200; response `[{PCIE_Devices:[],PCIE_Function:[]}]`

#### `GET /api/host_inventory/host_interface_power_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200
- UI: `host_interface_info`
- Response fields (from capture + UI): `Powercontrol`, `Voltages`
  - Live (b650d4u.har): GET 200; response `[{Powercontrol:[{id:int,Name:str,AverageConsumedWatts:str,MaxConsumedWatts:str,MinConsumedWatts:str,IntervalInMinutes:str,LimitInWatts:str,LimitException:str}],Voltages:[{id:int,Name:str,MemberId:str,State:str,MinReadingRange:str,MaxReadingRange:str,UpperThresholdFatal:str,UpperThresholdCritical:str,UpperThresholdNonCritical:str,LowerThresholdNonCritical:str,LowerThresholdCritical:str,LowerThresholdFatal:str},…15 items]}]`

#### `GET /api/host_inventory/host_interface_processor_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `host_interface_info`
- Model: `collection/host_interface_processor_info`
- Response fields (from capture + UI): `EffectiveFamily`, `Id`, `Manufacturer`, `MaxSpeedMHz`, `Model`, `Name`, `ProcessorArchitecture`, `ProcessorType`, `Socket`, `State`, `TotalCores`
  - Correction: static JS analysis also listed `CapacityMiB`, `Description`, `DeviceLocator`, `IndicatorLED`, `OperatingSpeed`, `PartNumber`, `PowerState`, `SerialNumber`, `SystemType`, `Type`, `name`, `reading`, `type`; these belong to sibling models and are not returned by this endpoint (capture).
  - Live (x470d4u.har): GET 200; response `[]`
  - Live (b650d4u.har): GET 200; response `[]`
  - Live (b650d4u-2t.har): GET 200; response `[{Id:str,Name:str,Manufacturer:str,MaxSpeedMHz:str,Model:str,ProcessorArchitecture:str,ProcessorType:str,Socket:str,State:str,EffectiveFamily:str,TotalCores:str}]`

#### `GET /api/host_inventory/host_interface_storage_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `host_interface_info`
- Response fields (from capture + UI): `Storage_Contoller`, `Storage_Drives`
  - Live (x470d4u.har): GET 200; response `[{Storage_Drives:null,Storage_Contoller:null}]`
  - Live (b650d4u-2t.har): GET 200; response `[{Storage_Drives:[{Name:str,SerialNumber:str,Manufacturer:str,Location:str,Protocol:str,Model:str,Revision:str,EncryptionStatus:str,MediaType:str,State:str},…2 items],Storage_Contoller:[{MemberId:str,Name:str,SerialNumber:str,Model:str,FirmwareVersion:str,SpeedGbps:str,State:str},…6 items]}]`

#### `GET /api/host_inventory/host_interface_system_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200
- UI: `host_interface_info`
- Model: `collection/host_interface_system_info`
- Response fields (from capture + UI): `AssetTag`, `BiosVersion`, `Description`, `IndicatorLED`, `Manufacturer`, `Model`, `Name`, `PartNumber`, `PowerState`, `SerialNumber`, `State`, `SystemType`, `UUID`
  - Correction: static JS analysis also listed `AllowedSpeedsMHz`, `CapacityMiB`, `EffectiveFamily`, `Id`, `MaxSpeedMHz`, `MemoryDeviceType`, `OperatingSpeedMhz`, `ProcessorArchitecture`, `ProcessorType`, `Socket`, `TotalCores`; these belong to sibling models and are not returned by this endpoint (capture).
  - Live (b650d4u.har): GET 200; response `[{Name:str,Description:str,Model:str,IndicatorLED:str,Manufacturer:str,PowerState:str,SerialNumber:str,PartNumber:str,SystemType:str,State:str,AssetTag:str,BiosVersion:str,UUID:str}]`

#### `GET /api/host_inventory/host_interface_thermal_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `host_interface_info`

#### `GET /api/nvdimm_inventory/last_action_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/cps`
- Response (fields read by UI): `ActionName`, `ActionUri`, `LastModifiedTimestamp`, `Status`, `StatusMessage`

#### `POST /api/nvdimm_inventory/nvdimm_dimm_action`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/cps`
- Request: `avgPwrTimeConst`, `coreTempThreshold`, `enableThrehsholds`, `mediaTempThreshold`, `nvdimmactionId`, `nvdimmactionName`, `NVDIMMDimmId`, `nvdimmMemoryInstance`, `percentageRemThreshold`

#### `GET /api/nvdimm_inventory/nvdimm_dimm_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/cps`
- Model: `collection/nvdimm_dimminfo`
- Response (fields read by UI): `addressrangescrubinfo`, `alarmthreshold`, `BlockSizeBytes`, `CapacityMiB`, `ConfigurationLockdown`, `devicecharacteristic`, `DimmId`, `dimmpartitioninfo`, `extendedadrinfo`, `FirmwareFlashStatus`, `firmwareimageinfo`, `FreeCapacityMiB`, `globalFwUpdateStatus`, `HealthState`, `identifydimminfo`, `LabelVersion`, `LatchSysShutdownState`, `linked_DimmNames`, `longoperationstatusinfo`, `ManageabilityState`, `Name`, `NamespaceID`, `NamespaceName`, `NVDIMMDimmId`, `PersistentMemoryType`, `platformconfigdata`, `pollName`, `poolId`, `powermanagementpolicyinfo`, `securityoptininfo`, `SecurityState`, `smarthealthinfo`, `StagedFWRevision`, `viralpolicyinfo`

#### `GET /api/nvdimm_inventory/nvdimm_namespace_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/cps`
- Model: `collection/nvdimm_namespaceinfo`

#### `GET /api/nvdimm_inventory/nvdimm_pools_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/cps`
- Model: `collection/nvdimm_poolsinfo`

#### `GET /api/nvdimm_inventory/nvdimmdimm_fw_flash_status`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/FirmwareUpdateWizardView`, `settings/cps`
- Model: `collection/nvdimm_flashstatus`

#### `GET /api/smbiosdata`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `SmbiosInfoView`
- Response (fields read by UI): `arrayBuffer`

#### `PUT /api/system_inventory/component_status_update`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `system_inventory_info`
- Request: `DEV_INSTANCE`, `DEV_LED`, `DEV_TYPE`, `STATUS`

#### `GET /api/system_inventory/cpu_info`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `DELETE /api/system_inventory/cpu_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `/system_inventory_info_item`, `system_inventory_info`, `system_inventory_info_edit_item`
- Model: `collection//system_inventory_cpu_info`
- Response (fields read by UI): `ch_num`, `dimm_num`, `id`, `pci_bus_num`, `pci_device_num`, `pci_function_num`, `sensor_number`, `socket_num`

#### `GET /api/system_inventory/device_count`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `DELETE /api/system_inventory/device_count/:id` (Backbone collection model, `:id` = item `id`)
- UI: `/system_inventory_info_item`, `system_inventory_info`, `system_inventory_info_edit_item`
- Model: `collection//system_inventory_device_info`
- Response (fields read by UI): `id`

#### `GET /api/system_inventory/dimm_info`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `DELETE /api/system_inventory/dimm_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `/system_inventory_info_item`, `system_inventory_info`, `system_inventory_info_edit_item`
- Model: `collection//system_inventory_dimm_info`
- Response (fields read by UI): `id`

#### `GET /api/system_inventory/pci_info`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `DELETE /api/system_inventory/pci_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `/system_inventory_info_item`, `system_inventory_info`, `system_inventory_info_edit_item`
- Model: `collection//system_inventory_pci_info`
- Response (fields read by UI): `id`


### Storage / PCIe device management (64)

#### `GET /api/settings/brcm_pcie_switch_management/brcm_pcie_switch_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/brcm_pcie_switch_management/brcm_pcie_switch_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/brcm_pcie_switch_management/brcm_pcie_switch_info`
- Model: `collection//brcm_pcie_switch_info`
- Response (fields read by UI): `fw_info`, `name`, `sl_id`, `swich_chip_id`, `swich_chip_rev`, `swich_station_count`, `swich_station_mask`, `switch_attr`, `switch_mgmt_port_number`, `switch_mode`, `switch_ports_per_station`, `switch_temp`

#### `PUT /api/settings/brcm_pcie_switch_management/brcm_pcie_switch_port_assign_unassign`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/brcm_pcie_switch_management/brcm_pcie_switch_port_info_item`

#### `GET /api/settings/brcm_pcie_switch_management/brcm_pcie_switch_port_error_counters`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/brcm_pcie_switch_management/brcm_pcie_switch_port_info_item`

#### `GET /api/settings/brcm_pcie_switch_management/brcm_pcie_switch_port_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/brcm_pcie_switch_management/brcm_pcie_switch_port_info`, `settings/brcm_pcie_switch_management/brcm_pcie_switch_port_info_item`
- Model: `collection//brcm_pcie_switch_port_info`
- Response (fields read by UI): `name`, `port_attr_info`

#### `PUT /api/settings/brcm_pcie_switch_management/brcm_pcie_switch_reset_port_error_counters`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/brcm_pcie_switch_management/brcm_pcie_switch_port_info_item`
- Request: `port_gid`, `sl_id`

#### `GET /api/settings/brcm_pcie_switch_management/brcm_pcie_switch_station_ram_error_counters`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/brcm_pcie_switch_management/brcm_pcie_switch_info`
- Response (fields read by UI): `rdr_misc_alut_ram_error_counter`, `rdr_misc_tec_bgl2lcl_ram_error_counter`, `rdr_misc_tec_bgl2lcl_ram_error_status`, `tec_pld_ram0_instance_0_error_counter`, `tec_pld_ram0_instance_0_error_status`, `tec_pld_ram0_instance_1_error_counter`, `tec_pld_ram0_instance_1_error_status`, `tec_pld_ram1_instance_0_error_status`

#### `PUT /api/settings/brcm_pcie_switch_management/brcm_pcie_switch_station_reset_ram_error_counters`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/brcm_pcie_switch_management/brcm_pcie_switch_info`
- Request: `sl_id`, `station_num`

#### `GET /api/settings/nic_management/nic_controller_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/nic_management/nic_controller_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/nic_management/nic_controller_info`
- Model: `collection//nic_controller_info`
- Response (fields read by UI): `aenControlSupport`, `broadcastCapabilities`, `bufferingCapability`, `capabilitiesFlags`, `channelCount`, `configFlags`, `eid`, `firmwareName1`, `firmwareName2`, `firmwareName3`, `firmwareVersion`, `flowControl`, `linkSetting`, `linkStatus`, `mac_info`, `macAddrCount`, `macAddrFlags`, `multicastCapabilities`, `ncsiversionAlpha1`, `ncsiversionAlpha2`, `vlanCount`, `vlanSupport`, `vlanTagCount`, `vlanTagFlags`

#### `GET /api/settings/nvme_management/nvme_basic_mi_vpd_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/nvme_mi_management/nvme_mi_vpd_info`

#### `GET /api/settings/nvme_management/nvme_controller_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/nvme_management/nvme_controller_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/nvme_management/nvme_controller_info`
- Model: `collection//nvme_controller_info`
- Response (fields read by UI): `bootloader_version`, `controller_name`, `firmware_version`, `pdlu`, `sensor_temperature`, `serial_no`, `smart_warning`, `status_flag`, `vendor_id`

#### `GET /api/settings/nvme_management/nvme_p3700_vital_product_data`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/nvme_management/nvme_p3700_vital_product_data/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/nvme_management/nvme_p3700_vpd_info`
- Model: `collection//nvme_p3700_vpd_info`
- Response (fields read by UI): `controller_name`, `ini_power_req`, `max_power_req`, `model_num`, `pci_sig_vendor_id`, `port0_max_link_speed`, `port0_max_link_width`, `port1_max_link_speed`, `port1_max_link_width`, `serial_no`

#### `PUT /api/settings/nvme_mi_management/nvme_mi_clear_status_flags`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/nvme_mi_management/nvme_mi_subsystem_info`
- Request: `clear_flag`, `crtwarn`, `csts`, `ctemp`, `fa`, `nac`, `pdlu`, `spare`, `ssd_id`

#### `PUT /api/settings/nvme_mi_management/nvme_mi_set_status_flags`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/nvme_mi_management/nvme_mi_subsystem_info`
- Request: `ceco`, `crtwarn`, `csts`, `ctemp`, `fa`, `fatal`, `nac`, `nssro`, `pdlu`, `ready`, `shutdown`, `spare`, `ssd_id`

#### `GET /api/settings/nvme_mi_management/nvme_mi_ssd_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/NVMeMISSDSControllerFirmwareUpdateView`
- Model: `models//nvme_mi_ssd_info`
- Response (fields read by UI): `count`

#### `GET /api/settings/nvme_mi_management/nvme_mi_subsystem_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/nvme_mi_management/nvme_mi_subsystem_info`
- Model: `collection//nvme_mi_subsystem_info`
- Response (fields read by UI): `controller_id`, `controller_name`, `major_version`, `minor_version`, `no_of_ports`, `port_info`, `sub_sys_hlth_temp_cels_comp_temp`, `subsystem_health_nssdf`, `subsystem_health_nssport_0_la`, `subsystem_health_nssport_1_la`, `subsystem_health_nssrnr`, `subsystem_health_pdlu`

#### `GET /api/settings/nvme_mi_management/nvme_mi_vpd_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/nvme_mi_management/nvme_mi_vpd_info`

#### `GET /api/settings/pcie_switch_management/get_pcie_bifurcation_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/pcie_management/pcie_partition_info`
- Response (fields read by UI): `bif_code_raw`

#### `GET /api/settings/pcie_switch_management/pcie_mgmt_dev_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/pcie_switch_management/pcie_mgmt_dev_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/pcie_management/pcie_partition_info`, `settings/pcie_management/pcie_partition_info_item`, `settings/pcie_switch_management/pcie_mgmt_dev_info`
- Model: `collection//pcie_mgmt_dev_info`
- Response (fields read by UI): `cfg_version`, `controller_name`, `dev_id`, `dev_version`, `firmware_information`, `id`, `port_num`, `port_state`

#### `GET /api/settings/pcie_switch_management/pcie_partition_count`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/pcie_management/pcie_partition_info`, `settings/pcie_management/pcie_partition_info_item`
- Model: `models//pcie_partition_count`
- Response (fields read by UI): `id`, `port_num`, `port_state`

#### `GET /api/settings/pcie_switch_management/pcie_port_bind`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/pcie_management/pcie_partition_info_item`

#### `GET /api/settings/pcie_switch_management/pcie_port_link_status`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/pcie_management/pcie_partition_info_item`

#### `GET /api/settings/pcie_switch_management/pcie_port_unbind`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/pcie_management/pcie_partition_info_item`

#### `POST /api/settings/pcie_switch_management/pcie_switch_reset`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/pcie_switch_management/pcie_mgmt_dev_info`

#### `PUT /api/settings/pcie_switch_management/set_pcie_bifurcation_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/pcie_management/pcie_partition_info`
- Request: `bif_code`, `stack_id`

#### `GET /api/settings/pcie_switch_management/validate_port_unbind`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/pcie_management/pcie_partition_info`
- Response (fields read by UI): `is_unbind`

#### `GET /api/settings/pcie_switch_mgmt/pcie_partition_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/pcie_management/pcie_partition_info`, `settings/pcie_management/pcie_partition_info_item`
- Model: `collection//pcie_partition_info`
- Response (fields read by UI): `id`, `port_num`, `port_state`

#### `GET /api/settings/raid_management/bbu_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/raid_management/bbu_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/raid_management/bbu_info`
- Model: `collection//bbu_info`
- Response (fields read by UI): `bbu_type (X470 only)`, `controller_name (X470 only)`, `current (X470 only)`, `status (X470 only)`, `temperature (X470 only)`, `voltage (X470 only)`

#### `POST /api/settings/raid_management/clear_all_foreign_cmd`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/physical_device_info`
- Request: `CTRLID`

#### `GET/PUT /api/settings/raid_management/create_lu_cache`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/create_lu_cache`
- Model: `models//create_lu_cache`
- Request: `array_number`, `CTRLID`, `deviceId`, `LDID`, `numdrives`, `parity_group_count`, `raid_level`, `vendor_type`, `writepolicy`
- Response (fields read by UI): `accelerator_type`, `array_number`, `controller_id`, `controller_name`, `dev_id`, `ld_name`, `logical_drive_type`, `predective_fail_count`, `size`, `state`, `type`, `vendor_type`

#### `GET/PUT /api/settings/raid_management/create_virtual_drive`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/create_logical_device`
- Model: `models//create_logical_device`
- Request: `accelerator (B650 only)`, `accesspolicy`, `array_number (B650 only)`, `CTRLID`, `deviceId`, `diskCachePolicy`, `initState`, `iopolicy`, `numdrives`, `parity_group_count (B650 only)`, `raid_level`, `readpolicy`, `spandepth`, `StripeSize`, `VDNAME`, `VDSize (X470 only)`, `vendor_type (B650 only)`, `writepolicy`
- Response (fields read by UI): `array_number (B650 only)`, `controller_id`, `controller_name`, `dev_id`, `pd_type (B650 only)`, `predective_fail_count`, `size`, `state`, `type`, `vendor_type (B650 only)`
  - `initState` values: `0=`, `1=`, `2=`
  - `StripeSize` values: `9=`

#### `GET /api/settings/raid_management/dedicated_hot_spare_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/physical_device_info_item`
- Request: `CTRLID`, `DEVICEID`
- Response (fields read by UI): `virtual_disk_id`

#### `DELETE /api/settings/raid_management/delete_virtual_drive`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/logical_device_info_item`
- Request: `CTRLID`, `DEVICEID`, `VENDOR_TYPE (B650 only)`

#### `PUT /api/settings/raid_management/logical_consistency_check_cmd`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/logical_device_info_item`
- Request: `ACTION`, `CTRLID`, `DEVICEID`

#### `GET /api/settings/raid_management/logical_device_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/raid_management/logical_device_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/raid_management/create_lu_cache`, `settings/raid_management/logical_device_info`, `settings/raid_management/logical_device_info_edit_item`, `settings/raid_management/logical_device_info_item`, `settings/raid_management/physical_device_info`, `settings/raid_management/physical_device_info_item`, `settings/raid_management/raid_logical_physical_device_info`, `settings/raid_management/raid_logical_physical_device_info_item`
- Model: `collection//logical_device_info`
- Response (fields read by UI): `accelerator_type (B650 only)`, `access_policy (B650 only)`, `bad_blocks`, `bgi`, `cache_policy`, `controller_id`, `controller_name`, `dev_id`, `device_group (B650 only)`, `elements_num (B650 only)`, `fw_version (B650 only)`, `id`, `initstate (B650 only)`, `ld_name`, `link_speed (B650 only)`, `logical_drive_type (B650 only)`, `model (B650 only)`, `pd_type (B650 only)`, `percentage (B650 only)`, `physical_drive_boot_property (B650 only)`, `Physical_Drive_Flags_Info (B650 only)`, `Physical_Drive_Location (B650 only)`, `physical_drive_max_link_speed (B650 only)`, `Physical_Drive_Status_Info (B650 only)`, `predective_fail_count`, `processing_status (B650 only)`, `progress`, `read_policy`, `relatedLDID (B650 only)`, `removal_status`, `size`, `slot (B650 only)`, `smart (B650 only)`, `speed (B650 only)`, `ssd_caching (X470 only)`, `state`, `Stripe_size`, `temperature (B650 only)`, `type`, `write_policy`
  - `write_policy` values: `2=`, `3=`

#### `PUT /api/settings/raid_management/logical_device_progress_cmd`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/logical_device_info`, `settings/raid_management/logical_device_info_item`
- Request: `CTRLID`, `DEVICEID`
- Response (fields read by UI): `CONSISTENCY_PROGRESS`

#### `PUT /api/settings/raid_management/logical_initialize_check_cmd`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/logical_device_info_item`
- Request: `ACTION`, `CTRLID`, `DEVICEID`, `INIT_TYPE`

#### `PUT /api/settings/raid_management/logical_start_stop_locate_cmd`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/logical_device_info_item`
- Request: `ACTION`, `CTRLID`, `DEVICEID`, `DURATION`
- Response (fields read by UI): `action (B650 only)`

#### `GET /api/settings/raid_management/manage_array_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/raid_management/manage_array_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/raid_management/create_logical_device`, `settings/raid_management/create_lu_cache`, `settings/raid_management/manage_array_info`, `settings/raid_management/manage_array_info_edit_item`, `settings/raid_management/manage_array_info_item`
- Model: `collection//manage_array_info`
- Request: `CTRLID`, `vendor_type`
- Response (fields read by UI): `acceleration_mode`, `array_number`, `block_size`, `free_size`, `id`, `interface`, `media`, `physical_device_count`, `physical_id_strings`, `spare_device_count`, `spare_id_strings`, `spare_type`, `total_size`, `type`

#### `PUT /api/settings/raid_management/manage_data_drive_array`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/manage_array_info_item`
- Request: `ACTION`, `ARRAY_NUMBER`, `CTRLID`, `PD_DEVICE_IDS`, `VENDOR_TYPE`

#### `PUT /api/settings/raid_management/manage_spare_drive_array`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/manage_array_info_item`
- Request: `ACTION`, `ARRAY_NUMBER`, `CTRLID`, `PD_ID`, `SPARE_TYPE`, `VENDOR_TYPE`

#### `GET /api/settings/raid_management/patrol_read_properties`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/raid_management/patrol_read_properties/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/raid_management/logical_device_info`, `settings/raid_management/logical_device_info_item`
- Model: `collection//patrol_read`
- Request: `mode`
- Response (fields read by UI): `accelerator_type`, `access_policy`, `cache_policy`, `controller_id`, `controller_name`, `dev_id`, `elements_num`, `id`, `initstate`, `ld_name`, `logical_drive_type`, `mode`, `read_policy`, `relatedLDID`, `state`, `type`, `write_policy`

#### `PUT /api/settings/raid_management/patrol_read_start_stop_cmd`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/logical_device_info`
- Request: `CTRLID`, `OPERATION`

#### `PUT /api/settings/raid_management/physical_dedicated_hotspare_cmd`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/physical_device_info_item`
- Request: `CMDPARAM (X470 only)`, `CTRLID`, `DEVICEID`, `DISKCOUNT`, `DISKID`

#### `GET /api/settings/raid_management/physical_device_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/raid_management/physical_device_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/raid_management/create_logical_device`, `settings/raid_management/create_lu_cache`, `settings/raid_management/manage_array_info`, `settings/raid_management/manage_array_info_item`, `settings/raid_management/physical_device_info`, `settings/raid_management/physical_device_info_edit_item`, `settings/raid_management/physical_device_info_item`, `settings/raid_management/raid_logical_physical_device_info`, `settings/raid_management/raid_logical_physical_device_info_edit_item`, `settings/raid_management/raid_logical_physical_device_info_item`
- Model: `collection//physical_device_info`, `collection//raid_logical_physical_device_info`
- Response (fields read by UI): `array_number (B650 only)`, `cache`, `controller_id`, `controller_name`, `dev_id`, `fw_version (B650 only)`, `id`, `interface_type`, `link_speed`, `media (B650 only)`, `model (B650 only)`, `pd_type (B650 only)`, `physical_device_count (B650 only)`, `physical_drive_boot_property (B650 only)`, `Physical_Drive_Flags_Info (B650 only)`, `Physical_Drive_Location (B650 only)`, `physical_drive_max_link_speed (B650 only)`, `Physical_Drive_Status_Info (B650 only)`, `physical_id_strings (B650 only)`, `power_sate`, `predective_fail_count`, `product_id`, `removal_status`, `rotation_speed (B650 only)`, `serial_num`, `size`, `slot`, `smart (B650 only)`, `spare_device_count (B650 only)`, `spare_id_strings (B650 only)`, `spare_type (B650 only)`, `speed`, `state`, `temperature`, `type`, `unique_id (B650 only)`, `usgae_info (B650 only)`, `vendor_id`

#### `GET /api/settings/raid_management/physical_device_smart_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/physical_device_info_item`
- Request: `CTRLID`, `DEVICEID`

#### `PUT /api/settings/raid_management/physical_hotspare_cmd`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/physical_device_info_item`
- Request: `CMDPARAM`, `CTRLID`, `DEVICEID`

#### `PUT /api/settings/raid_management/physical_onlineoffline_cmd`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/raid_logical_physical_device_info_item`
- Request: `CMDPARAM`, `CTRLID`, `DEVICEID`

#### `PUT /api/settings/raid_management/physical_removal_cmd`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/physical_device_info_item`
- Request: `CMDPARAM`, `CTRLID`, `DEVICEID`

#### `PUT /api/settings/raid_management/physical_start_stop_locate_cmd`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/physical_device_info_item`
- Request: `ACTION`, `CTRLID`, `DEVICEID`, `DURATION`, `VENDORTYPE (B650 only)`
- Response (fields read by UI): `action (B650 only)`

#### `POST /api/settings/raid_management/physical_state_cmd`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/physical_device_info_item`
- Request: `CTRLID`, `DEVICEID`

#### `GET /api/settings/raid_management/raid_controller_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/raid_management/raid_controller_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `maintenance/HpmFirmwareUpdateView`, `settings/raid_management/bbu_info`, `settings/raid_management/create_logical_device`, `settings/raid_management/create_lu_cache`, `settings/raid_management/logical_device_info`, `settings/raid_management/logical_device_info_item`, `settings/raid_management/manage_array_info`, `settings/raid_management/manage_array_info_item`, `settings/raid_management/physical_device_info`, `settings/raid_management/physical_device_info_item`, `settings/raid_management/raid_controller_info`, `settings/raid_management/raid_event_log`, `settings/raid_management/raid_event_log_item`, `settings/raid_management/raid_topology_info`, `settings/raid_management/ses_enclosure_info`, `settings/raid_management/ses_enclosure_info_item`, `settings/raid_management/storage_summary`
- Model: `collection//raid_controller_info`
- Response (fields read by UI): `accelerator_type (B650 only)`, `access_policy (B650 only)`, `array_number (B650 only)`, `bios_version`, `cache_policy (B650 only)`, `cache_size (B650 only)`, `controller_id`, `controller_name`, `cpld_version`, `dev_id`, `driver_version (B650 only)`, `elements_num (B650 only)`, `event_class (B650 only)`, `event_class_code (B650 only)`, `event_code (B650 only)`, `event_desc (B650 only)`, `event_detail_code (B650 only)`, `event_message (B650 only)`, `event_subclass_code (B650 only)`, `event_tag (B650 only)`, `event_type`, `expander_temp`, `expander_version`, `firmware_version (B650 only)`, `fw_version (B650 only)`, `hardware_revision (B650 only)`, `id`, `initstate (B650 only)`, `ld_name (B650 only)`, `link_speed (B650 only)`, `logical_drive_type (B650 only)`, `media (B650 only)`, `mode (B650 only)`, `model (B650 only)`, `nvdata_version (B650 only)`, `package_version`, `pci_device_id`, `pci_subsystem_id`, `pci_subvendor_id`, `pci_vendor_id`, `pd_type (B650 only)`, `physical_device_count (B650 only)`, `physical_drive_boot_property (B650 only)`, `Physical_Drive_Flags_Info (B650 only)`, `Physical_Drive_Location (B650 only)`, `physical_drive_max_link_speed (B650 only)`, `Physical_Drive_Status_Info (B650 only)`, `physical_id_strings (B650 only)`, `predective_fail_count`, `read_policy (B650 only)`, `record_id (B650 only)`, `relatedLDID (B650 only)`, `removal_status`, `roc_temp`, `seeprom_version (X470 only)`, `serial_number`, `ses_alarm_count (B650 only)`, `ses_alarm_status (B650 only)`, `ses_bay_device_count (B650 only)`, `ses_encl_alarm_status (B650 only)`, `ses_encl_fan_status (B650 only)`, `ses_encl_number (B650 only)`, `ses_encl_power_supply_status (B650 only)`, `ses_encl_temp_sensor_status (B650 only)`, `ses_enclosure_id (B650 only)`, `ses_enclosure_type (B650 only)`, `ses_exp_device_count (B650 only)`, `ses_fan_count (B650 only)`, `ses_fan_status (B650 only)`, `ses_path_count (B650 only)`, `ses_pd_count (B650 only)`, `ses_physical_drive_device_count (B650 only)`, `ses_power_supply_count (B650 only)`, `ses_power_supply_status (B650 only)`, `ses_product_id (B650 only)`, `ses_sep_device_count (B650 only)`, `ses_serial_number_id (B650 only)`, `ses_storage_encl_status (B650 only)`, `ses_storage_encl_status_fan (B650 only)`, `ses_storage_encl_status_path (B650 only)`, `ses_storage_encl_status_powersupply (B650 only)`, `ses_storage_encl_status_temp (B650 only)`, `ses_temp_sensor_count (B650 only)`, `ses_temp_sensor_status (B650 only)`, `ses_vendor_id (B650 only)`, `size (B650 only)`, `slot (B650 only)`, `smart (B650 only)`, `spare_device_count (B650 only)`, `spare_id_strings (B650 only)`, `spare_type (B650 only)`, `speed (B650 only)`, `speed_list (B650 only)`, `state`, `support_interface_list (B650 only)`, `temperature (B650 only)`, `timestamp (B650 only)`, `tmm_status`, `total_memory_size (B650 only)`, `type (B650 only)`, `uefi_version`, `vendor_type (B650 only)`, `write_policy (B650 only)`, `wwn (B650 only)`
  - `read_policy` values: `0=`, `1=`
  - `write_policy` values: `2=`, `3=`
  - `access_policy` values: `0=`, `1=`, `2=`

#### `DELETE/GET /api/settings/raid_management/raid_event_log`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/raid_controller_info`, `settings/raid_management/raid_event_log`, `settings/raid_management/raid_event_log_item`
- Model: `collection//raid_event_log`
- Request: `vendor_type (B650 only)`
- Response (fields read by UI): `controller_name (B650 only)`, `event_class (B650 only)`, `event_class_code (B650 only)`, `event_code (B650 only)`, `event_desc (B650 only)`, `event_detail_code (B650 only)`, `event_message (B650 only)`, `event_subclass_code (B650 only)`, `event_tag (B650 only)`, `event_type (B650 only)`, `id (B650 only)`, `record_id (B650 only)`, `timestamp (B650 only)`

#### `PUT /api/settings/raid_management/raid_levels`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/create_logical_device`, `settings/raid_management/create_lu_cache`
- Request: `CTRLID`, `VENDOR_TYPE (B650 only)`
- Response (fields read by UI): `iopolicy (B650 only)`

#### `PUT /api/settings/raid_management/raid_pd_count`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/create_logical_device`, `settings/raid_management/create_lu_cache`
- Request: `CTRLID`, `VENDOR_TYPE (B650 only)`

#### `GET /api/settings/raid_management/raid_topology_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/raid_topology_info`
- Model: `collection//raid_topology_info`

#### `GET /api/settings/raid_management/sesenclosureinfo`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/ses_enclosure_info`, `settings/raid_management/ses_enclosure_info_item`
- Model: `collection//ses_enclosure_info`
- Response (fields read by UI): `ses_alarm_count`, `ses_alarm_status`, `ses_bay_device_count`, `ses_encl_alarm_status`, `ses_encl_fan_status`, `ses_encl_number`, `ses_encl_power_supply_status`, `ses_encl_temp_sensor_status`, `ses_enclosure_id`, `ses_enclosure_type`, `ses_exp_device_count`, `ses_fan_count`, `ses_fan_status`, `ses_path_count`, `ses_pd_count`, `ses_physical_drive_device_count`, `ses_power_supply_count`, `ses_power_supply_status`, `ses_product_id`, `ses_sep_device_count`, `ses_serial_number_id`, `ses_storage_encl_status`, `ses_storage_encl_status_fan`, `ses_storage_encl_status_path`, `ses_storage_encl_status_powersupply`, `ses_storage_encl_status_temp`, `ses_temp_sensor_count`, `ses_temp_sensor_status`, `ses_vendor_id`

#### `GET /api/settings/raid_management/sesenclosureinfobrcm`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/raid_management/ses_enclosure_info`, `settings/raid_management/ses_enclosure_info_item`
- Model: `collection//ses_enclosure_info_brcm`
- Response (fields read by UI): `ses_alarm_count`, `ses_alarm_status`, `ses_bay_device_count`, `ses_encl_alarm_status`, `ses_encl_fan_status`, `ses_encl_number`, `ses_encl_power_supply_status`, `ses_encl_temp_sensor_status`, `ses_enclosure_id`, `ses_enclosure_type`, `ses_exp_device_count`, `ses_fan_count`, `ses_fan_status`, `ses_path_count`, `ses_pd_count`, `ses_physical_drive_device_count`, `ses_power_supply_count`, `ses_power_supply_status`, `ses_product_id`, `ses_sep_device_count`, `ses_serial_number_id`, `ses_storage_encl_status`, `ses_storage_encl_status_fan`, `ses_storage_encl_status_path`, `ses_storage_encl_status_powersupply`, `ses_storage_encl_status_temp`, `ses_temp_sensor_count`, `ses_temp_sensor_status`, `ses_vendor_id`

#### `GET /api/settings/raid_management/storage_summary`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/raid_management/storage_summary/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/raid_management/storage_summary`
- Model: `collection//storage_summary`
- Response (fields read by UI): `controller_id (B650 only)`, `controller_name`, `hotspare`, `logical_count`, `physical_count`, `vendor_type (B650 only)`

#### `GET /api/settings/sasit_management/sasit_controller_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/sasit_management/sasit_controller_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/sasit_management/sasit_controller_info`, `settings/sasit_management/sasit_enclosure_info`, `settings/sasit_management/sasit_enclosure_info_item`, `settings/sasit_management/sasit_event_log`, `settings/sasit_management/sasit_event_log_item`, `settings/sasit_management/sasit_physical_device_info`, `settings/sasit_management/sasit_physical_device_info_item`, `settings/sasit_management/sasit_topology_info`, `settings/sasit_management/sasit_topology_info_item`
- Model: `collection//sasit_controller_info`
- Response (fields read by UI): `bios_version (B650 only)`, `board_name`, `controller_name`, `dev_id (B650 only)`, `event_class (B650 only)`, `event_code (B650 only)`, `event_desc (B650 only)`, `event_type (B650 only)`, `firmware_version`, `id (B650 only)`, `ioc_temp`, `nvdata_version`, `package_version (B650 only)`, `pci_device_id`, `pci_subsystem_id`, `pci_subvendor_id`, `pci_vendor_id`, `record_id (B650 only)`, `sas_address (X470 only)`, `serial_number (B650 only)`, `ses_alarm_count (B650 only)`, `ses_alarm_status (B650 only)`, `ses_encl_alarm_status (B650 only)`, `ses_encl_fan_status (B650 only)`, `ses_encl_power_supply_status (B650 only)`, `ses_encl_temp_sensor_status (B650 only)`, `ses_enclosure_id (B650 only)`, `ses_enclosure_type (B650 only)`, `ses_fan_count (B650 only)`, `ses_fan_status (B650 only)`, `ses_pd_count (B650 only)`, `ses_power_supply_count (B650 only)`, `ses_power_supply_status (B650 only)`, `ses_product_id (B650 only)`, `ses_temp_sensor_count (B650 only)`, `ses_temp_sensor_status (B650 only)`, `ses_vendor_id (B650 only)`, `size (B650 only)`, `slot (B650 only)`, `speed`, `state (B650 only)`, `temperature (B650 only)`, `timestamp (B650 only)`, `type (B650 only)`

#### `GET /api/settings/sasit_management/sasit_enclosure_info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/sasit_management/sasit_enclosure_info`, `settings/sasit_management/sasit_enclosure_info_item`
- Model: `collection//sasit_enclosure_info`
- Response (fields read by UI): `ses_alarm_count`, `ses_alarm_status`, `ses_encl_alarm_status`, `ses_encl_fan_status`, `ses_encl_power_supply_status`, `ses_encl_temp_sensor_status`, `ses_enclosure_id`, `ses_enclosure_type`, `ses_fan_count`, `ses_fan_status`, `ses_pd_count`, `ses_power_supply_count`, `ses_power_supply_status`, `ses_product_id`, `ses_temp_sensor_count`, `ses_temp_sensor_status`, `ses_vendor_id`

#### `DELETE/GET /api/settings/sasit_management/sasit_event_log`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/sasit_management/sasit_event_log`, `settings/sasit_management/sasit_event_log_item`
- Model: `collection//sasit_event_log`
- Request: `vendor_type`
- Response (fields read by UI): `controller_name`, `event_class`, `event_code`, `event_desc`, `event_type`, `record_id`, `timestamp`

#### `GET /api/settings/sasit_management/sasit_physical_device_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `PUT /api/settings/sasit_management/sasit_physical_device_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/sasit_management/sasit_physical_device_info`, `settings/sasit_management/sasit_physical_device_info_edit_item`, `settings/sasit_management/sasit_physical_device_info_item`
- Model: `collection//sasit_physical_device_info`
- Response (fields read by UI): `controller_id`, `controller_name`, `dev_id`, `firmware_revision`, `id`, `interface_type`, `media_type`, `power_sate`, `product_id`, `serial_num`, `size`, `slot`, `smart`, `speed`, `state`, `temperature`, `type (B650 only)`, `vendor_id`

#### `PUT /api/settings/sasit_management/sasit_physical_start_stop_locate_cmd`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/sasit_management/sasit_physical_device_info_item`
- Request: `ACTION`, `CTRLID`, `DEVICEID`, `DURATION`
- Response (fields read by UI): `action`

#### `GET /api/settings/sasit_management/sasit_topology_info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/sasit_management/sasit_topology_info/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/sasit_management/sasit_topology_info`, `settings/sasit_management/sasit_topology_info_edit_item`, `settings/sasit_management/sasit_topology_info_item`
- Model: `collection//sasit_topology_info`
- Response (fields read by UI): `connected_dev_id`, `connected_phy_id`, `ConnectedDevID (X470 only)`, `ConnectedPhyID (X470 only)`, `controller_id`, `controller_name`, `dev_id`, `dev_type`, `DeviceID (X470 only)`, `DevType (X470 only)`, `id`, `phy_count`, `phy_enabled`, `PhyCount (X470 only)`, `PhyEnabled (X470 only)`


### KVM, H5Viewer & SOL (13)

#### `GET /api/asrr/java-console`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `remotePopUpView`
- Notes: Returns the JNLP text; the UI saves it as jviewer.jnlp (Java KVM). B650 equivalent is /api/remote_control/get/kvm/launch (only when the kvm service viewer_count == 0).

#### `GET /api/kvm/token`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `viewer.min.js models/kvm_token`
- Response (fields read by UI): `client_ip`, `session (web session cookie value)`, `token (KVM SSI hash)`
- Notes: Fetched by viewer.html (models/kvm_token) when KVM starts. The result is stored in sessionStorage.token; token is written into the first /kvm WebSocket packet (CMD_CONNECTION_COMPLETE / validate-video-session packet, SSI_HASH_SIZE bytes) together with client_ip, username and MAC. The same token authenticates the /cd-server virtual media WebSocket. Error code 1000 = KVM_TOKEN_GEN_FAILED.

#### `GET /api/remote_control/get/kvm/launch`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `remotePopUpView`
- Notes: B650: returns JNLP text saved as jviewer.jnlp.

#### `GET /api/settings/javasol`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- Query: `?protocol=…`
- UI: `settings/video/sol/javasol`

#### `GET /api/settings/media/active_redirections`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/media/active_redirections`, `settings/media/active_redirections_item`
- Model: `collection/media/active_redirections`
- Response (fields read by UI): `client_ip`, `client_type`, `image_name`, `media_index`, `media_type`, `state`

#### `GET /api/settings/media/adviser`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200
- UI: `viewer.min.js (H5Viewer)`, `remotePopUpView`
- Model: `models//adviser`
- Response fields (from capture + UI): `keyboard_layout`, `kvm_client (only in b650d4u.har)`, `kvm_port`, `license`, `mouse_mode`, `retry_count`, `retry_interval`, `secure_channel`, `singleport_status`, `status`, `web_port`, `keyboard_language (read by UI; absent from capture)`
  - Live (x470d4u.har): GET 200; response `{license:int,status:int,secure_channel:int,kvm_port:int,mouse_mode:int,web_port:int,singleport_status:int,retry_count:int,retry_interval:int,keyboard_layout:str}`
  - Live (b650d4u.har): GET 200; response `{license:int,status:int,secure_channel:int,kvm_port:int,mouse_mode:int,web_port:int,singleport_status:int,retry_count:int,retry_interval:int,keyboard_layout:str,kvm_client:int}`
- Notes: KVM service (adviserd) config. remotePopUpView disables the Launch button when status == 0. Field list HAR-confirmed on X470.

#### `GET /api/settings/media/h5viewercfg`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `viewer.min.js RemoteControlView (models/h5viewercfg)`
- Response (fields read by UI): `automatic_off`, `client_ip`, `keyboard_language`, `kvm_num_cd`, `kvm_num_hd`, `kvm_service_status`, `local_monitor_off`, `num_cd`, `num_hd`, `power_save_mode`, `server_ip`, `session`, `token`
- Notes: B650 H5Viewer replacement for /api/kvm/token: one call returns the KVM token plus KVM/VMedia settings. kvm_service_status == 0 means KVM disabled.

#### `GET/PUT /api/settings/media/remotesession`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/media/remotesession`
- Model: `models/media/remotesession`
- Request: `automatic_off`, `keyboard_language`, `kvm_client (B650 only)`, `kvm_encryption`, `local_monitor_off`, `retry_count`, `retry_time_interval`, `single_port`, `vmedia_attach`, `vnc_non_secure (B650 only)`, `vnc_over_ssh (B650 only)`, `vnc_over_stunnel (B650 only)`
- Response (fields read by UI): `automatic_off`, `keyboard_language`, `kvm_client (B650 only)`, `kvm_encryption`, `local_monitor_off`, `retry_count`, `retry_time_interval`, `single_port`, `vmedia_attach`, `vnc_non_secure (B650 only)`, `vnc_over_ssh (B650 only)`, `vnc_over_stunnel (B650 only)`
  - `keyboard_language` values: `AD=`, `DA=`, `NL-BE=`, `NL-NL=`, `GB=`, `US=`, `FI=`, `FR-BE=`, `FR=`, `DE=`, `DE-CH=`, `IT=`, `JP=`, `NO=`, `PT=`, `ES=`, `SV=`, `TR_F=`, `TR_Q=`
  - `vmedia_attach` values: `0=`, `1=`
  - `kvm_client` values: `1=remotesessionjviewerh5viewer`, `2=remotesessionvnc`
  - `retry_count` validation: min=1; max=6; pattern=/^\d+$/; msg=retry_count_must_be_a_number_from_1_to_6
  - `retry_time_interval` validation: min=5; max=30; pattern=/^\d+$/; msg=retry_time_interval_must_be_a_number_from_5_to_30

#### `GET/PUT /api/settings/mouse`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/mouse`
- Model: `models//mouse`
- Request: `mode`
- Response (fields read by UI): `mode`

#### `GET/PUT /api/settings/video/sol_configurations`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/video/sol/sol_configurations`
- Model: `models//sol_configurations`
- Request: `baudrate`, `volatile_baudrate`
- Response (fields read by UI): `baudrate`, `volatile_baudrate`
  - `volatile_baudrate` values: `6=`, `7=`, `8=`, `9=`, `10=`
  - `baudrate` values: `6=`, `7=`, `8=`, `9=`, `10=`

#### `GET /api/sol/session`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200
- UI: `remotePopUpView`
- Model: `models//sol`
- Response fields (from capture + UI): `status`
  - Live (b650d4u.har): GET 200; response `{status:int}`
- Notes: HAR (B650D4U): {"status":0}. Checked before opening the HTML5 SOL window.

#### `GET/POST /api/sol/sol_configurations`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/video/sol/sol_configurations`
- Request: `baudrate`
- Response (fields read by UI): `baudrate`
  - `baudrate` values: `6=`, `7=`, `8=`, `9=`, `10=`

#### `POST /api/sol/solcfg`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `remotePopUpView`


### Virtual media / remote storage (15)

#### `GET/PUT /api/settings/media/general`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/rmedia`, `image_redirection/rmedia_item`, `settings/media/general`
- Model: `models/media/general`
- Request: `cd_kerberos_default_realm (B650 only)`, `cd_kerberos_domain_realm1 (B650 only)`, `cd_kerberos_domain_realm2 (B650 only)`, `cd_kerberos_encryption_type (B650 only)`, `cd_kerberos_kdc_server_name (B650 only)`, `cd_kerberos_key_version_number (B650 only)`, `cd_kerberos_principal_name (B650 only)`, `cd_kerberos_principal_password (B650 only)`, `cd_remote_domain_name`, `cd_remote_password`, `cd_remote_server_address`, `cd_remote_share_type`, `cd_remote_source_path`, `cd_remote_user_name`, `fd_remote_domain_name (X470 only)`, `fd_remote_password (X470 only)`, `fd_remote_server_address (X470 only)`, `fd_remote_share_type (X470 only)`, `fd_remote_source_path (X470 only)`, `fd_remote_user_name (X470 only)`, `hd_kerberos_default_realm (B650 only)`, `hd_kerberos_domain_realm1 (B650 only)`, `hd_kerberos_domain_realm2 (B650 only)`, `hd_kerberos_encryption_type (B650 only)`, `hd_kerberos_kdc_server_name (B650 only)`, `hd_kerberos_key_version_number (B650 only)`, `hd_kerberos_principal_name (B650 only)`, `hd_kerberos_principal_password (B650 only)`, `hd_remote_domain_name`, `hd_remote_password`, `hd_remote_retry_count (B650 only)`, `hd_remote_retry_interval (B650 only)`, `hd_remote_server_address`, `hd_remote_share_type`, `hd_remote_source_path`, `hd_remote_user_name`, `local_media_support`, `mount_cd`, `mount_fd (X470 only)`, `mount_hd`, `redirection_status (B650 only)`, `remote_media_support`, `same_settings`
- Response (fields read by UI): `cd_kerberos_default_realm (B650 only)`, `cd_kerberos_domain_realm1 (B650 only)`, `cd_kerberos_domain_realm2 (B650 only)`, `cd_kerberos_encryption_type (B650 only)`, `cd_kerberos_kdc_server_name (B650 only)`, `cd_kerberos_key_version_number (B650 only)`, `cd_kerberos_principal_name (B650 only)`, `cd_kerberos_principal_password (B650 only)`, `cd_remote_domain_name`, `cd_remote_password`, `cd_remote_server_address`, `cd_remote_share_type`, `cd_remote_source_path`, `cd_remote_user_name`, `error_code (B650 only)`, `fd_remote_domain_name (X470 only)`, `fd_remote_password (X470 only)`, `fd_remote_server_address (X470 only)`, `fd_remote_share_type (X470 only)`, `fd_remote_source_path (X470 only)`, `fd_remote_user_name (X470 only)`, `hd_kerberos_default_realm (B650 only)`, `hd_kerberos_domain_realm1 (B650 only)`, `hd_kerberos_domain_realm2 (B650 only)`, `hd_kerberos_encryption_type (B650 only)`, `hd_kerberos_kdc_server_name (B650 only)`, `hd_kerberos_key_version_number (B650 only)`, `hd_kerberos_principal_name (B650 only)`, `hd_kerberos_principal_password (B650 only)`, `hd_remote_domain_name`, `hd_remote_password`, `hd_remote_server_address`, `hd_remote_share_type`, `hd_remote_source_path`, `hd_remote_user_name`, `image_name (B650 only)`, `local_media_support`, `media_index (B650 only)`, `media_type (B650 only)`, `mount_cd`, `mount_fd (X470 only)`, `mount_hd`, `redirection_status (B650 only)`, `remote_media_support`, `rmedia_retry_count (B650 only)`, `rmedia_retry_interval (B650 only)`, `same_settings`, `session_index (B650 only)`, `share_type (B650 only)`
  - `cd_remote_share_type` values: `nfs=generalnfs`, `cifs=generalcifs`
  - `fd_remote_share_type` values: `nfs=generalnfs`, `cifs=generalcifs`
  - `hd_remote_share_type` values: `nfs=generalnfs`, `cifs=generalcifs`
  - `cd_kerberos_encryption_type` values: `0=`, `1=`, `2=`, `3=`
  - `hd_kerberos_encryption_type` values: `0=`, `1=`, `2=`, `3=`
  - `mount_cd` validation: custom-fn; msg=atleast_one_media_should_be_mounted_when_remote_media_support_is_enabled / remote_media_support
  - `cd_remote_server_address` validation: custom-fn; msg=remote_media_support / valid_server_address_is_required
  - `cd_remote_source_path` validation: msg=remote_media_support / source_path_is_required
  - `cd_remote_user_name` validation: msg=cd_remote_share_type / remote_media_support / username_is_required_for_cifs_location
  - `cd_remote_password` validation: custom-fn; msg=cd_remote_share_type / password_is_required_for_cifs_location / remote_media_support
  - `fd_remote_user_name` validation: msg=fd_remote_share_type / remote_media_support / same_settings / username_is_required_for_cifs_location
  - `fd_remote_password` validation: custom-fn; msg=fd_remote_share_type / password_is_required_for_cifs_location / remote_media_support / same_settings
  - `fd_remote_server_address` validation: custom-fn; msg=remote_media_support / valid_server_address_is_required
  - `hd_remote_user_name` validation: msg=hd_remote_share_type / remote_media_support / same_settings / username_is_required_for_cifs_location
  - `hd_remote_server_address` validation: custom-fn; msg=remote_media_support / valid_server_address_is_required
  - `hd_remote_password` validation: custom-fn; msg=hd_remote_share_type / password_is_required_for_cifs_location / remote_media_support / same_settings
  - `cd_kerberos_default_realm` validation: msg=cd_remote_share_type / default_realm_is_required_for_ennfs / remote_media_support
  - `cd_kerberos_kdc_server_name` validation: msg=cd_remote_share_type / kdc_server_name_is_required_for_ennfs / remote_media_support
  - `cd_kerberos_domain_realm1` validation: msg=cd_remote_share_type / domain_realm1_is_required_for_ennfs / remote_media_support
  - `cd_kerberos_domain_realm2` validation: msg=cd_remote_share_type / domain_realm2_is_required_for_ennfs / remote_media_support
  - `cd_kerberos_principal_name` validation: msg=cd_remote_share_type / principal_name_is_required_for_ennfs_location / remote_media_support
  - `cd_kerberos_principal_password` validation: custom-fn; msg=cd_remote_share_type / password_is_required_for_cifs_location / remote_media_support
  - `cd_kerberos_key_version_number` validation: msg=cd_remote_share_type / key_version_number_is_required_for_ennfs_location / remote_media_support
  - `cd_kerberos_encryption_type` validation: msg=cd_remote_share_type / encryption_type_is_required_for_ennfs_location / remote_media_support
  - `hd_remote_source_path` validation: custom-fn; msg=remote_media_support / source_path_is_required
  - `hd_kerberos_default_realm` validation: msg=default_realm_is_required_for_ennfs / hd_remote_share_type / remote_media_support
  - `hd_kerberos_kdc_server_name` validation: msg=hd_remote_share_type / kdc_server_name_is_required_for_ennfs / remote_media_support
  - `hd_kerberos_domain_realm1` validation: msg=domain_realm1_is_required_for_ennfs / hd_remote_share_type / remote_media_support
  - `hd_kerberos_domain_realm2` validation: msg=domain_realm2_is_required_for_ennfs / hd_remote_share_type / remote_media_support
  - `hd_kerberos_principal_name` validation: msg=hd_remote_share_type / principal_name_is_required_for_ennfs_location / remote_media_support
  - `hd_kerberos_principal_password` validation: custom-fn; msg=hd_remote_share_type / password_is_required_for_cifs_location / remote_media_support
  - `hd_kerberos_key_version_number` validation: msg=hd_remote_share_type / kerberos_key_version_number_is_required_for_ennfs_location / remote_media_support
  - `hd_kerberos_encryption_type` validation: msg=hd_remote_share_type / kerberos_encryption_type_is_required_for_ennfs_location / remote_media_support
  - `hd_remote_retry_interval` validation: custom-fn; msg=isFeatureEnabled / remote_media_support / retry_interval_required
  - `hd_remote_retry_count` validation: custom-fn; msg=isFeatureEnabled / remote_media_support / retry_count_required

#### `GET/PUT /api/settings/media/instance`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/media/instance`
- Model: `models/media/instance`
- Request: `kvm_num_cd`, `kvm_num_fd (X470 only)`, `kvm_num_hd`, `num_cd`, `num_fd (X470 only)`, `num_hd`, `power_save_mode`, `sd_media`, `secure_channel`
- Response (fields read by UI): `kvm_num_cd`, `kvm_num_fd (X470 only)`, `kvm_num_hd`, `num_cd`, `num_fd (X470 only)`, `num_hd`, `power_save_mode`, `sd_media`, `secure_channel`
  - `num_fd` values: `0=`, `1=`, `2=`, `3=`, `4=`
  - `num_cd` values: `0=`, `1=`, `2=`, `3=`, `4=`
  - `num_hd` values: `0=`, `1=`, `2=`, `3=`, `4=`
  - `kvm_num_fd` values: `0=`, `1=`, `2=`, `3=`, `4=`
  - `kvm_num_cd` values: `0=`, `1=`, `2=`, `3=`, `4=`
  - `kvm_num_hd` values: `0=`, `1=`, `2=`, `3=`, `4=`
  - `kvm_num_fd` validation: custom-fn; msg=remote_kvm_floppy_device_instance_value_should_be_less_than_or_equal_to_virtual_floppy_device_count
  - `kvm_num_cd` validation: custom-fn; msg=remote_kvm_cddvd_device_instance_value_should_be_less_than_or_equal_to_virtual_cddvd_device_count
  - `kvm_num_hd` validation: custom-fn; msg=remote_kvm_hard_disk_instance_value_should_be_less_than_or_equal_to_virtual_hard_disk_count

#### `GET /api/settings/media/local/configurations`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/lmedia`, `image_redirection/lmedia_item`
- Model: `collection//lmedia_cfg`
- Response (fields read by UI): `image_name`, `media_index`, `media_type`, `redirection_status`, `session_index (B650 only)`

#### `POST /api/settings/media/local/file`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/lmedia_item`
- Request: `multipart:lmedia_image`
- Notes: multipart/form-data, field lmedia_image (ISO/IMG file). Preceded by POST lmediasizecheck {image_size: bytes}. Upload progress via XHR upload events.

#### `DELETE /api/settings/media/local/image`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Query: `?&image_index=…`
- UI: `image_redirection/lmedia_item`
- Notes: Query string built by hand: /api/settings/media/local/image?&image_index=<n>&image_name=<name>. No body.

#### `GET /api/settings/media/local/images`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/lmedia`, `image_redirection/lmedia_item`
- Model: `collection/lmedia_images`
- Response (fields read by UI): `image_name`, `media_index`, `media_type`, `redirection_status`, `session_index (B650 only)`

#### `POST /api/settings/media/local/lmediasizecheck`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/lmedia_item`
- Request: `image_size`, `multipart:lmedia_image`

#### `POST /api/settings/media/local/start-media`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/lmedia_item`
- Request: `image_index`, `image_name`, `image_type`

#### `POST /api/settings/media/local/stop-media`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/lmedia_item`
- Request: `image_index`, `image_type`

#### `GET /api/settings/media/remote/configurations`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/rmedia`, `image_redirection/rmedia_item`
- Model: `collection//rmedia_cfg`
- Request: `redirection_status (B650 only)`
- Response (fields read by UI): `error_code (B650 only)`, `image_name`, `media_index`, `media_type`, `redirection_status`, `session_index (B650 only)`, `share_type (B650 only)`

#### `GET /api/settings/media/remote/images`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/rmedia`, `image_redirection/rmedia_item`
- Model: `collection//rmedia_images`
- Request: `redirection_status (B650 only)`
- Response (fields read by UI): `error_code (B650 only)`, `image_name`, `media_index`, `media_type`, `redirection_status`, `session_index (B650 only)`, `share_type (B650 only)`

#### `GET/POST /api/settings/media/remote/lmediarefreshlist`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/lmedia`, `image_redirection/lmedia_item`
- Model: `collection//lmediaRefreshList`
- Request: `appType`, `mediaType`
- Response (fields read by UI): `image_name`, `media_index`, `media_type`, `redirection_status`, `session_index`

#### `GET/POST /api/settings/media/remote/rmediarefreshlist`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/rmedia`, `image_redirection/rmedia_item`
- Model: `collection//rmediaRefreshList`
- Request: `appType`, `mediaType`, `redirection_status (B650 only)`
- Response (fields read by UI): `error_code (B650 only)`, `image_name`, `media_index`, `media_type`, `redirection_status`, `session_index (B650 only)`, `share_type (B650 only)`
- Notes: POST JSON {appType:1, mediaType:1|2|4} (1 = CD, 2 = FD, 4 = HD), one call per enabled media server (CD_SERVER_APP / FD_SERVER_APP / HD_SERVER_APP features), staggered by 2 s; then the collection is fetched (GET).

#### `POST /api/settings/media/remote/start-media`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/rmedia_item`
- Request: `image_index`, `image_name`, `image_redirection (B650 only)`, `image_share_type (B650 only)`, `image_type`
- Notes: JSON {image_name, image_type (media_type: 1 CD, 2 FD, 4 HD), image_index}; B650 adds image_redirection:1 and image_share_type. stop-media takes the same fields.

#### `POST /api/settings/media/remote/stop-media`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `image_redirection/rmedia_item`
- Request: `image_index`, `image_name`, `image_share_type (B650 only)`, `image_type`


### Settings: network (23)

#### `GET /api/asrr/getvetobit`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: b650d4u.har GET 200
- UI: `settings/keep_ncsi_link_up`
- Model: `models/keep_ncsi_link_up`
- Response fields (from capture + UI): `vetobit`
  - Live (b650d4u.har): GET 200; response `{vetobit:int}`
- Notes: Keep NC-SI link up (veto bit). X470 UI; also seen live on B650D4U fw 1.10.00 ({"vetobit":0}) although the analysed B650 7.02/9.05 JS and binaries do not have it. setvetobit is PUT JSON {vetobit}.

#### `PUT /api/asrr/setvetobit`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/keep_ncsi_link_up`
- Request: `vetobit`

#### `POST /api/set/dns/registration`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/dns`
- Request: `reg_enable`, `reg_mdns`, `reg_option`, `reg_tsig`

#### `GET /api/settings/channels`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/ncsi`
- Model: `collection/channels`

#### `GET/PUT /api/settings/dns`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/network/dns`
- Model: `models//dns_settings`
- Request: `dns_iface`, `dns_manual`, `dns_priority`, `dns_server1`, `dns_server2`, `dns_server3`, `dns_status`, `domain_iface`, `domain_manual`, `domain_name`, `host_cfg`, `host_name`, `iface_name`
- Response (fields read by UI): `dns_iface`, `dns_status`, `domain_iface`, `ethIndex`, `reg_enable`, `reg_ifc`, `reg_mdns`, `reg_option`, `reg_tsig0`, `reg_tsig1`, `reg_tsig2`, `tsig_both`
  - `host_cfg` values: `1=dns_settingsautomatic`, `0=dns_settingsmanual`
  - `domain_manual` values: `0=dns_settingsautomatic`, `1=dns_settingsmanual`
  - `dns_manual` values: `0=dns_settingsautomatic`, `1=dns_settingsmanual`
  - `dns_priority` values: `4=dns_settingsipv4`, `6=dns_settingsipv6`
  - `host_name` validation: custom-fn; msg=invalid_host_name_refer_help_for_more_information
  - `domain_name` validation: custom-fn; msg=domain_manual / invalid_domain_name_refer_help_for_more_information
  - `dns_server1` validation: custom-fn; msg=invalid_dns_server_address_1refer_help_for_more_information
  - `dns_server2` validation: custom-fn; msg=invalid_dns_server_address_2refer_help_for_more_information
  - `dns_server3` validation: custom-fn; msg=invalid_dns_server_address_3refer_help_for_more_information
  - `domain_iface` validation: msg=domain_manual / required_domain_interface
  - `dns_iface` validation: msg=required_dns_interface
  - `dns_priority` validation: custom-fn; msg=required_ip_priority

#### `GET/PUT /api/settings/dns-info`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/dns`
- Model: `models//dns_settings`
- Request: `dns_iface`, `dns_manual`, `dns_priority`, `dns_server1`, `dns_server2`, `dns_server3`, `dns_status`, `domain_iface`, `domain_manual`, `domain_name`, `host_cfg`, `host_name`
- Response (fields read by UI): `dns_iface`, `dns_status`, `domain_iface`, `domain_name`, `ethIndex`, `reg_enable`, `reg_ifc`, `reg_mdns`, `reg_option`, `reg_tsig0`, `reg_tsig1`, `reg_tsig2`, `tsig_both`
  - `host_cfg` values: `1=dns_settingsautomatic`, `0=dns_settingsmanual`
  - `domain_manual` values: `0=dns_settingsautomatic`, `1=dns_settingsmanual`
  - `dns_manual` values: `0=dns_settingsautomatic`, `1=dns_settingsmanual`
  - `dns_priority` values: `4=dns_settingsipv4`, `6=dns_settingsipv6`
  - `host_name` validation: custom-fn; msg=invalid_host_name_refer_help_for_more_information
  - `domain_name` validation: custom-fn; msg=domain_manual / invalid_domain_name_refer_help_for_more_information
  - `dns_server1` validation: custom-fn; msg=invalid_dns_server_address_1refer_help_for_more_information
  - `dns_server2` validation: custom-fn; msg=invalid_dns_server_address_2refer_help_for_more_information
  - `dns_server3` validation: custom-fn; msg=invalid_dns_server_address_3refer_help_for_more_information
  - `domain_iface` validation: msg=domain_manual / required_domain_interface
  - `dns_iface` validation: msg=required_dns_interface
  - `dns_priority` validation: custom-fn; msg=required_ip_priority

#### `GET /api/settings/dns/domain-options`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/dns`
- Model: `collection/domain_options`
- Request: `domain_name`
  - `domain_name` validation: maxLength=256; msg=domain_name_must_consist_of_two_or_more_alphanumeric_labels_separated_by_periods_eg_subdomainexamplecom_the_total_length_of_the_domain_name_string_must_be_less_than_256_characters

#### `GET /api/settings/dns/registration`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `PUT /api/settings/dns/registration/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/dns/dns_registration`, `settings/network/dns`
- Model: `collection//dns_registration`, `collection/dns_registration`
- Request: `iface_name`, `reg_enable`, `reg_mdns`, `reg_option`, `reg_tsig`
- Response (fields read by UI): `reg_enable`, `reg_ifc`, `reg_mdns`, `reg_option`, `reg_tsig`
  - `reg_option` values: `0=dns_registrationnsupdate`, `1=dns_registrationdhcp_client_fqdn`, `2=dns_registrationhostname`

#### `PUT /api/settings/dns/restart`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/dns`
- Request: `dns_status`

#### `GET /api/settings/dns/server-options`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/dns`
- Model: `collection/dns_options`

#### `POST /api/settings/dns/tsig-private-key`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/dns`
- Request: `multipart:index`, `multipart:tsig_newfile`

#### `GET /api/settings/ncsi-interfaces`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/ncsi`
- Model: `collection/ncsi_interfaces`

#### `GET/PUT /api/settings/ncsi/mode`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/ncsi`
- Model: `models//ncsi`
- Request: `channel_number`, `interface_name`, `mode`, `package_id`
- Response (fields read by UI): `active (B650 only)`, `channel_number`, `interface_name`, `package_id`

#### `GET/PUT /api/settings/ncsilinkup`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/ncsi_link_up`
- Model: `models/ncsi_link_up`
- Request: `sharednic`, `sharednic2`
- Response (fields read by UI): `sharednic`, `sharednic2`

#### `GET/PUT /api/settings/ncsiselect`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/network/ncsi_select`
- Model: `models/ncsi_select`
- Request: `value`
- Response (fields read by UI): `value`

#### `GET /api/settings/network`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200; b650d4u-2t.har GET 200
- Per-item: `PUT /api/settings/network/:id` (Backbone collection model, `:id` = item `id`)
- UI: `dashboard/SummaryInfoView`, `dashboard/SummaryView`, `settings/network/ip`, `settings/pef/alert_policies_edit_item`, `settings/services_edit_item`
- Model: `collection//ip`, `collection//network_interfaces`, `collection//network_ip`
- Request: `interface_name`, `ipv4_address`, `ipv4_dhcp_enable`, `ipv4_enable`, `ipv4_gateway`, `ipv4_subnet`, `ipv6_address`, `ipv6_clear (B650 only)`, `ipv6_dhcp_enable`, `ipv6_enable`, `ipv6_gateway (B650 only)`, `ipv6_index`, `ipv6_prefix`, `lan_enable`, `mac_address`, `vlan_enable`, `vlan_id`, `vlan_priority`
- Response fields (from capture + UI): `channel_number`, `id`, `interface_name`, `ipv4_address`, `ipv4_dhcp_enable`, `ipv4_enable`, `ipv4_gateway`, `ipv4_subnet`, `ipv6_address`, `ipv6_dhcp_enable`, `ipv6_enable`, `ipv6_gateway`, `ipv6_index`, `ipv6_prefix`, `lan_enable`, `mac_address`, `vlan_enable`, `vlan_id`, `vlan_priority`
  - Live (x470d4u.har): GET 200; response `[{id:int,interface_name:str,channel_number:int,lan_enable:int,mac_address:str,ipv4_enable:int,ipv4_dhcp_enable:int,ipv4_address:str,ipv4_subnet:str,ipv4_gateway:str,ipv6_enable:int,ipv6_dhcp_enable:int,ipv6_address:str,ipv6_index:int,ipv6_prefix:int,ipv6_gateway:str,vlan_enable:int,vlan_id:int,vlan_priority:int},…2 items]`
  - Live (b650d4u.har): GET 200; response `[{id:int,interface_name:str,channel_number:int,lan_enable:int,mac_address:str,ipv4_enable:int,ipv4_dhcp_enable:int,ipv4_address:str,ipv4_subnet:str,ipv4_gateway:str,ipv6_enable:int,ipv6_dhcp_enable:int,ipv6_address:str,ipv6_index:int,ipv6_prefix:int,ipv6_gateway:str,vlan_enable:int,vlan_id:int,vlan_priority:int}]`
  - Live (b650d4u-2t.har): GET 200; response `[{id:int,interface_name:str,channel_number:int,lan_enable:int,mac_address:str,ipv4_enable:int,ipv4_dhcp_enable:int,ipv4_address:str,ipv4_subnet:str,ipv4_gateway:str,ipv6_enable:int,ipv6_dhcp_enable:int,ipv6_address:str,ipv6_index:int,ipv6_prefix:int,ipv6_gateway:str,vlan_enable:int,vlan_id:int,vlan_priority:int}]`
  - `ipv4_enable` validation: custom-fn; msg=please_choose_ipv4_enable_or_ipv6_enable
  - `ipv6_enable` validation: custom-fn; msg=please_choose_ipv4_enable_or_ipv6_enable
  - `ipv4_address` validation: custom-fn; msg=invalid_ipv4_address_refer_help_for_more_information / ipv4_dhcp_enable
  - `ipv4_subnet` validation: custom-fn; msg=invalid_ipv4_subnet_refer_help_for_more_information / ipv4_dhcp_enable
  - `ipv4_gateway` validation: custom-fn; msg=invalid_ipv4_gateway_refer_help_for_more_information / ipv4_dhcp_enable
  - `ipv6_address` validation: custom-fn; msg=invalid_ipv6_address_refer_help_for_more_information / ipv6_dhcp_enable
  - `ipv6_prefix` validation: custom-fn; msg=invalid_ipv6_prefix_refer_help_for_more_information / ipv6_dhcp_enable
  - `vlan_id` validation: custom-fn; msg=invalid_vlan_id_refer_help_for_more_information
  - `vlan_priority` validation: custom-fn; msg=invalid_vlan_priority_refer_help_for_more_information
  - `ipv6_gateway` validation: custom-fn; msg=invalid_ipv6_gateway_refer_help_for_more_information / ipv6_dhcp_enable
- Notes: HAR-confirmed array per interface (id, interface_name, channel_number, lan_enable, mac_address, ipv4_*, ipv6_*, vlan_*). Saved per interface with PUT /api/settings/network/:id. Saving makes the BMC restart networking.

#### `GET/PUT /api/settings/network-bond`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200, PUT 200; b650d4u-2t.har GET 200
- UI: `settings/network/bond`
- Model: `models//bond`
- Request: `auto_configuration_enable`, `bond_enable`, `bond_ifc (B650 only)`
- Response fields (from capture + UI): `auto_configuration_enable`, `bond_enable`, `bond_mode`, `id`, `interface_name (read by B650 UI; absent from capture)`
  - Live (x470d4u.har): GET 200; response `{id:int,bond_enable:int,bond_mode:str,auto_configuration_enable:int}`
  - Live (b650d4u.har): GET 200; response `{id:int,bond_enable:int,bond_mode:str,auto_configuration_enable:int}`
  - Live (b650d4u.har): PUT 200; request application/json `{id:int,bond_enable:int,bond_mode:str,auto_configuration_enable:int,bond_ifc:str}`; response `{id:int,bond_enable:int,bond_mode:str,auto_configuration_enable:int,bond_ifc:str}`
  - Live (b650d4u-2t.har): GET 200; response `{id:int,bond_enable:int,bond_mode:str,auto_configuration_enable:int}`
- Notes: HAR-confirmed {id, bond_enable, bond_mode, auto_configuration_enable}. B650D4U HAR confirms Backbone save semantics: PUT /api/settings/network-bond with the whole model as JSON ({"id":1,"bond_enable":0,"bond_mode":"active-backup","auto_configuration_enable":1,"bond_ifc":"eth0"}); the response echoes the saved object.

#### `GET /api/settings/network-interfaces`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/services_edit_item`
- Model: `collection//network_interfaces`

#### `GET /api/settings/network-link`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `PUT /api/settings/network-link/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/network/link`
- Model: `collection//link`
- Request: `auto_negotiation`, `duplex_mode`, `interface_name`, `link_speed`
- Response (fields read by UI): `auto_negotiation`, `duplex_mode`, `duplex_mode_auto_negotiated`, `interface_name`, `link_speed`, `link_speed_auto_negotiated`, `ncsi_status (B650 only)`
  - `link_speed` values: `100=`, `10=`
  - `duplex_mode` values: `FULL=linkfull_duplex`, `HALF=linkhalf_duplex`

#### `GET /api/settings/network/activelancfg`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/date_time.js`
- Model: `collection/active_lan_interfaces`

#### `GET /api/settings/network/availablelancfg`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/bond`
- Model: `collection/availablelancfg`

#### `GET /api/settings/packages`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/network/ncsi`
- Model: `collection/packages`

#### `GET /api/settings/static-ipv6`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200
- UI: `settings/network/ip`
- Model: `collection//ipv6_static`
- Response fields (from capture + UI): `interface_name`, `ipv6_address`, `ipv6_index`, `ipv6_prefix`
  - Live (x470d4u.har): GET 200; response `[{ipv6_index:int,interface_name:str,ipv6_address:str,ipv6_prefix:int},…32 items]`
- Notes: HAR-confirmed array {ipv6_index, interface_name, ipv6_address, ipv6_prefix}.


### Settings: users & PAM (6)

#### `GET/PUT /api/asrr/settings/passwordcfg`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/password`
- Model: `models/password`
- Request: `complexity`, `lockout_interval`, `minlength`, `reset_interval`, `retry_count`
- Response (fields read by UI): `complexity`, `lockout_interval`, `minlength`, `reset_interval`, `retry_count`
  - `retry_count` validation: required=true; min=0; max=50; pattern=/^\d+$/; msg=invalid_setting
  - `reset_interval` validation: required=true; min=0; max=160; pattern=/^\d+$/; msg=invalid_setting
  - `lockout_interval` validation: required=true; min=0; max=160; pattern=/^\d+$/; msg=invalid_setting
  - `minlength` validation: required=true; min=1; max=20; pattern=/^\d+$/; msg=invalid_setting

#### `GET /api/settings/pam-order`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/pam_order`, `settings/pamorder`
- Model: `collection//pam_order`, `collection//pamorder`
- Request: `pam_position`
- Response (fields read by UI): `id`, `pam_module_name`, `pam_position`

#### `POST /api/settings/user/ssh-key-upload/:id`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/users_edit_item`
- Request: `multipart:upload_ssh_key`
- Notes: multipart field upload_ssh_key; :id = user id.

#### `GET /api/settings/users`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u-2t.har GET 200
- Per-item: `PUT /api/settings/users/:id` (Backbone collection model, `:id` = item `id`)
- UI: `LoginView`, `settings/users`, `settings/users_edit_item`, `settings/users_item`
- Model: `collection//users`, `collection/users`
- Request: `access`, `access_privilege_changed (B650 only)`, `accessByChannel (B650 only)`, `changepassword`, `confirm_password`, `email_format`, `email_id`, `kvm`, `loggedin_password (B650 only)`, `name`, `network_privilege (X470 only)`, `password`, `password_age (B650 only)`, `password_enddate (B650 only)`, `password_size`, `privilege (B650 only)`, `privilege_limit_serial (X470 only)`, `privilegeByChannel (B650 only)`, `setpasswordage (B650 only)`, `snmp`, `snmp_access`, `snmp_authentication_protocol`, `snmp_privacy_protocol`, `snmp_view_privileges (B650 only)`, `ssh_key`, `UserOperation`, `vmedia`
- Response fields (from capture + UI): `access`, `accessByChannel (only in b650d4u-2t.har)`, `channel (only in b650d4u-2t.har)`, `channel_type (only in b650d4u-2t.har)`, `creation_time`, `email_format`, `email_id`, `fixed_user_count`, `id`, `kvm`, `name`, `network_privilege (only in x470d4u.har)`, `OEMProprietary_level_Privilege`, `password_age (only in b650d4u-2t.har)`, `password_enddate (only in b650d4u-2t.har)`, `prev_snmp`, `privilege (only in b650d4u-2t.har)`, `privilege_limit_serial (only in x470d4u.har)`, `privilegeByChannel (only in b650d4u-2t.har)`, `snmp`, `snmp_access`, `snmp_authentication_protocol`, `snmp_privacy_protocol`, `ssh_key`, `userid (only in b650d4u-2t.har)`, `vmedia`, `confirm_password (read by UI; absent from capture)`, `password (read by UI; absent from capture)`, `password_size (read by UI; absent from capture)`, `snmp_view_privileges (read by B650 UI; absent from capture)`, `ssh_status (read by UI; absent from capture)`, `tfa (read by B650 UI; absent from capture)`
  - Live (x470d4u.har): GET 200; response `[{id:int,name:str,access:int,kvm:int,vmedia:int,snmp:int,prev_snmp:int,network_privilege:str,fixed_user_count:int,snmp_access:str,OEMProprietary_level_Privilege:int,privilege_limit_serial:str,snmp_authentication_protocol:str,snmp_privacy_protocol:str,email_id:str,email_format:str,ssh_key:str,creation_time:int},…10 items]`
  - Live (b650d4u-2t.har): GET 200; response `[{id:int,channel:int,channel_type:int,userid:int,name:str,access:int,accessByChannel:str,kvm:int,vmedia:int,snmp:int,prev_snmp:int,privilege:str,privilegeByChannel:str,fixed_user_count:int,snmp_access:str,OEMProprietary_level_Privilege:int,snmp_authentication_protocol:str,snmp_privacy_protocol:str,email_id:str,email_format:str,ssh_key:str,creation_time:int,password_age:int,password_enddate:int},…30 items]`
  - `password_size` values: `bytes_16=`, `bytes_20=`
  - `network_privilege` values: `user=`, `administrator=`, `operator=`, `none=`
  - `privilege_limit_serial` values: `user=`, `administrator=`, `operator=`, `none=`
  - `snmp_access` values: `read_only=`, `read_write=`
  - `snmp_authentication_protocol` values: `sha=`, `md5=`
  - `snmp_privacy_protocol` values: `des=`, `aes=`
  - `email_format` values: `ami_format=`, `fixedsubject_format=`
  - `name` validation: required=true; maxLength=16; pattern=/^[a-zA-Z][a-zA-Z0-9\-\_\@]+$/; msg=invalid_username_refer_help_content
  - `password` validation: custom-fn; msg=changepassword / password_field_must_be_8_characters_atleast / password_size
  - `confirm_password` validation: custom-fn; msg=changepassword / password_size / passwords_do_not_match
  - `email_id` validation: custom-fn; msg=invalid_email_format_refer_help_content
  - `loggedin_password` validation: custom-fn; msg=current_user_id / password_field_must_be_8_characters_atleast
  - `snmp_view_privileges` validation: custom-fn; msg=invalid_usersnmp_view_refer_help_content / snmp_view_privileges
  - `password_age` validation: custom-fn; msg=invalid_password_age / setpasswordage
- Notes: Fixed user slots (ids 1..N; slot 1 = anonymous, 2 = admin). Adding a user = PUT /api/settings/users/:id on an empty slot. HAR-confirmed list shape includes id, name, access, kvm, vmedia, snmp, network_privilege, fixed_user_count, snmp_access, OEMProprietary_level_Privilege, privilege_limit_serial, snmp_authentication_protocol, snmp_privacy_protocol, email_id, email_format, ssh_key, creation_time.

#### `DELETE /api/settings/users/:id`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/users_edit_item`, `settings/users_item`
- Request: `id`, `snmp_status`
- Notes: URL built with "/api/settings/users/:id".replace(":id", id); JSON body {snmp_status, id}.

#### `GET /api/settings/users_read_only`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/pef/lan_destinations_edit_item`
- Model: `collection//users_read_only`
- Response (fields read by UI): `id (B650 only)`


### Settings: services & sessions (6)

#### `GET /api/asrr/getWebEnableTimeout`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/services_edit_item`
- Response (fields read by UI): `WebEnableTimeout`

#### `POST /api/asrr/setWebEnableTimeout`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/services_edit_item`
- Request: `enable_timeout`

#### `GET/PUT /api/settings/ipmi_disable_interfaces`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u-2t.har GET 200
- UI: `settings/disable_interface`
- Model: `models//disable_interface`
- Request: `ipmi_over_kcs`, `ipmi_over_LAN`
- Response fields (from capture + UI): `ipmi_over_kcs`, `ipmi_over_LAN`, `is_kcs_ifc_support (read by UI; absent from capture)`
  - Live (b650d4u-2t.har): GET 200; response `{ipmi_over_LAN:int,ipmi_over_kcs:int}`
- Notes: HAR (B550D4U-2T): {"ipmi_over_LAN":1,"ipmi_over_kcs":1}.

#### `DELETE/GET /api/settings/service-sessions`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `DELETE /api/settings/service-sessions/:id` (Backbone collection model, `:id` = item `id`)
- Query: `?service_id=…`
- UI: `DashboardView`, `settings/service_sessions`, `settings/service_sessions_item`
- Model: `collection//service_sessions`
- Response (fields read by UI): `client_ip (B650 only)`, `id`, `session_id`, `session_type (B650 only)`, `user_id (B650 only)`, `user_name (B650 only)`, `user_privilege (B650 only)`

#### `GET /api/settings/services`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200, GET 401; b650d4u-2t.har GET 200, GET 401
- Per-item: `DELETE/PUT /api/settings/services/:id` (Backbone collection model, `:id` = item `id`)
- UI: `appext`, `remotePopUpView`, `settings/service_sessions`, `settings/service_sessions_item`, `settings/services`, `settings/services_edit_item`, `settings/services_item`
- Model: `collection//services`
- Request: `active_session`, `interface_name`, `maximum_sessions`, `mutual_port (X470 only)`, `non_secure_port`, `secure_port`, `state`, `time_out`
- Response fields (from capture + UI): `active_session`, `id`, `interface_name`, `maximum_sessions`, `non_secure_port`, `secure_port`, `service_id`, `service_name`, `singleport_status`, `state`, `time_out`, `viewer_count`, `client_ip (read by B650 UI; absent from capture)`, `mutual_port (read by X470 UI; absent from capture)`, `session_id (read by UI; absent from capture)`, `session_type (read by B650 UI; absent from capture)`, `user_id (read by B650 UI; absent from capture)`, `user_name (read by B650 UI; absent from capture)`, `user_privilege (read by B650 UI; absent from capture)`
  - Live (b650d4u.har): GET 200/401; response `[{id:int,service_id:int,service_name:str,state:int,interface_name:str,non_secure_port:int,secure_port:int,time_out:int,maximum_sessions:int,active_session:int,singleport_status:int,viewer_count:int},…7 items]`
  - Live (b650d4u-2t.har): GET 200/401; response `[{id:int,service_id:int,service_name:str,state:int,interface_name:str,non_secure_port:int,secure_port:int,time_out:int,maximum_sessions:int,active_session:int,singleport_status:int,viewer_count:int},…7 items]`
  - `time_out` validation: custom-fn; msg=invalid_time_out_refer_help_for_more_information
  - `non_secure_port` validation: custom-fn; msg=invalid_non_secure_port_refer_help_for_more_information
  - `secure_port` validation: custom-fn; msg=invalid_secure_port_refer_help_for_more_information
  - `mutual_port` validation: custom-fn; msg=invalid_secure_port_refer_help_for_more_information

#### `GET/PUT /api/settings/web_server_instances`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/web_server_instances`
- Model: `models//web_server_instances`
- Request: `instances`
- Response (fields read by UI): `instances`
  - `instances` validation: required=true; range=[1,4]; msg=invalid_instances_refer_help_for_more_information


### Settings: SSL (3)

#### `GET/PUT /api/operations/ssl/certificate/generate`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/ssl/generate_ssl`
- Model: `models//generate_ssl`
- Request: `city`, `common_name`, `country`, `email_id`, `key_length`, `organization`, `organization_unit`, `state`, `valid_days`
- Response (fields read by UI): `city`, `common_name`, `country`, `email_id`, `key_length`, `organization`, `organization_unit`, `state`, `valid_days`
  - `key_length` values: `2048=`
  - `common_name` validation: required=true; maxLength=64; pattern=/(^[a-zA-Z0-9%=@!{},'~&*()'<>?.:;_ / ^\/; msg=invalid_common_name
  - `organization` validation: required=true; maxLength=64; pattern=/(^[a-zA-Z0-9%=@!{},'~&*()'<>?.:;_ / ^\/; msg=invalid_organization
  - `organization_unit` validation: required=true; maxLength=64; pattern=/(^[a-zA-Z0-9%=@!{},'~&*()'<>?.:;_ / ^\/; msg=invalid_organization_unit
  - `city` validation: required=true; maxLength=128; pattern=/(^[a-zA-Z0-9%=@!{},'~&*()'<>?.:;_ / ^\/; msg=invalid_city_or_locality
  - `state` validation: required=true; maxLength=128; pattern=/(^[a-zA-Z0-9%=@!{},'~&*()'<>?.:;_ / ^\/; msg=invalid_state_or_province
  - `country` validation: required=true; length=2; pattern=/^[a-zA-Z0-9]*$/; msg=invalid_country
  - `email_id` validation: required=true; pattern="email"; msg=invalid_email_address
  - `valid_days` validation: required=true; range=[1,3650]; pattern="number"; msg=invalid_time_until_expiry

#### `GET/POST/PUT /api/settings/ssl/certificate`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/ssl/upload_ssl`, `settings/ssl/view_ssl`
- Model: `models//view_ssl`
- Request: `multipart:new_ca_certificate (X470 only)`, `multipart:new_certificate`, `multipart:new_private_key`, `multipart:trusted_ca_certificates (B650 only)`, `multipart:trusted_ca_support (B650 only)`
- Response (fields read by UI): `certificate_version`, `from_city`, `from_common_name`, `from_country`, `from_email_id`, `from_organization`, `from_organization_unit`, `from_state`, `public_key`, `serial_number`, `signature_algorithm`, `to_city`, `to_common_name`, `to_country`, `to_email_id`, `to_organization`, `to_organization_unit`, `to_state`, `valid_from`, `valid_till`

#### `GET/PUT /api/settings/ssl/certificate-info`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/ssl/upload_ssl`
- Model: `models//upload_ssl`
- Request: `trusted_ca_support (B650 only)`
- Response (fields read by UI): `ca_certificate_available (X470 only)`, `ca_certificate_date (X470 only)`, `certificate_available`, `certificate_date`, `current_private_key`, `private_key_date`, `trusted_ca_certificate_available (B650 only)`, `trusted_ca_certificate_date (B650 only)`, `trusted_ca_support (B650 only)`


### Settings: LDAP / AD / RADIUS (7)

#### `GET/PUT /api/settings/active-directory-settings`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/ext_users/active_directory/general_active_directory`
- Model: `models//general_active_directory`
- Request: `domain_controller1`, `domain_controller2`, `domain_controller3`, `enable`, `enablessl (B650 only)`, `secret_password`, `secret_username`, `user_domain_name`
- Response (fields read by UI): `domain_controller1`, `domain_controller2`, `domain_controller3`, `enable`, `enablessl (B650 only)`, `secret_password`, `secret_username`, `user_domain_name`
  - `secret_username` validation: custom-fn; msg=invalid_user_name_refer_help_for_more_information / secret_password
  - `secret_password` validation: custom-fn; msg=invalid_password_refer_help_for_more_information / secret_username
  - `user_domain_name` validation: custom-fn; msg=invalid_user_domain_name_refer_help_for_more_information
  - `domain_controller1` validation: custom-fn; msg=invalid_domain_controller_server_address_1refer_help_for_more_information
  - `domain_controller2` validation: custom-fn; msg=domain_controller1 / domain_controller3 / invalid_domain_controller_server_address_2refer_help_for_more_information
  - `domain_controller3` validation: custom-fn; msg=domain_controller1 / domain_controller2 / invalid_domain_controller_server_address_3refer_help_for_more_information

#### `GET /api/settings/active-directory-users`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/active-directory-users/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/ext_users/active_directory/rolegroup_active_directory`, `settings/ext_users/active_directory/rolegroup_active_directory_edit_item`, `settings/ext_users/active_directory/rolegroup_active_directory_item`
- Model: `collection//rolegroup_active_directory`
- Request: `role_group_domain`, `role_group_kvm_privilege`, `role_group_name`, `role_group_privilege`, `role_group_vmedia_privilege`, `role_group_withoem_privilege`
- Response (fields read by UI): `id`, `role_group_domain`, `role_group_kvm_privilege`, `role_group_name`, `role_group_privilege`, `role_group_vmedia_privilege`, `role_group_withoem_privilege`
  - `role_group_privilege` values: `user=`, `administrator=`, `operator=`, `none=`
  - `role_group_withoem_privilege` values: `user=`, `administrator=`, `operator=`, `oem=`, `none=`
  - `role_group_name` validation: maxLength=64; pattern=/^([a-zA-Z0-9_\-])+$/; msg=invalid_role_group_name_refer_help_for_more_information
  - `role_group_domain` validation: maxLength=255; msg=invalid_role_group_domain_refer_help_for_more_information
  - `role_group_privilege` validation: required=true; msg=invalid_group_privilege_selected
  - `role_group_withoem_privilege` validation: required=true; msg=invalid_group_privilege_selected

#### `POST /api/settings/ldap-certificates`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/ext_users/ldap/general_ldap`
- Request: `bind_dn (B650 only)`, `multipart:ca_certificate_file`, `multipart:certificate_file`, `multipart:private_key`, `password (B650 only)`, `port (B650 only)`, `search_base (B650 only)`, `server_address (B650 only)`

#### `GET/PUT /api/settings/ldap-settings`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/ext_users/ldap/general_ldap`
- Model: `models//general_ldap`
- Request: `bind_dn`, `common_name_type`, `enable`, `encryption_type`, `password`, `port`, `search_base`, `server_address`, `user_login_attribute`
- Response (fields read by UI): `bind_dn`, `ca_certificate_file`, `certificate_file`, `common_name_type`, `enable`, `encryption_type`, `port`, `private_key`, `search_base`, `server_address`, `user_login_attribute`
  - `user_login_attribute` values: `cn=`, `uid=`
  - `encryption_type` values: `0=general_ldapno_encryption`, `1=general_ldapssl`, `2=general_ldapstarttls`
  - `common_name_type` values: `ip=general_ldapip_address`, `fqdn=general_ldapfqdn`
  - `server_address` validation: msg=invalid_server_address_refer_help_for_more_information
  - `port` validation: range=[1,65535]; msg=invalid_port_refer_help_for_more_information
  - `bind_dn` validation: custom-fn; msg=invalid_bind_dn_refer_help_for_more_information
  - `password` validation: minLength=1; maxLength=48; pattern=/^[^ ]+$/; msg=invalid_password_refer_help_for_more_information
  - `search_base` validation: minLength=4; maxLength=64; pattern=/^(((cn=) / (uid=) / (ou=) / (dc=) / (o=))[a-z]{1}[\w\-\.]{1,63},?)+$/; msg=invalid_search_base_refer_help_for_more_information

#### `GET /api/settings/ldap-users`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/ldap-users/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/ext_users/ldap/rolegroup_ldap`, `settings/ext_users/ldap/rolegroup_ldap_edit_item`, `settings/ext_users/ldap/rolegroup_ldap_item`
- Model: `collection//rolegroup_ldap`
- Request: `role_group_domain`, `role_group_kvm_privilege`, `role_group_name`, `role_group_privilege`, `role_group_vmedia_privilege`, `role_group_withoem_privilege`
- Response (fields read by UI): `id`, `role_group_domain`, `role_group_kvm_privilege`, `role_group_name`, `role_group_privilege`, `role_group_vmedia_privilege`, `role_group_withoem_privilege`
  - `role_group_withoem_privilege` values: `user=`, `administrator=`, `operator=`, `oem=`, `none=`
  - `role_group_privilege` values: `user=`, `administrator=`, `operator=`, `none=`
  - `role_group_name` validation: maxLength=64; pattern=/^([a-zA-Z0-9_\-])+$/; msg=invalid_role_group_name_refer_help_for_more_information
  - `role_group_domain` validation: minLength=4; maxLength=64; pattern=/^(((cn=) / (uid=) / (ou=) / (dc=) / (o=))[a-z]{1}[\w\-\.]{1,63},?)+$/; msg=invalid_role_group_domain_refer_help_for_more_information
  - `role_group_privilege` validation: required=true; msg=invalid_group_privilege_selected
  - `role_group_withoem_privilege` validation: required=true; msg=invalid_group_privilege_selected

#### `GET/PUT /api/settings/radius/advanced`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/ext_users/radius/advanced_radius`
- Model: `models//advanced_radius`
- Request: `administrator`, `none`, `oem`, `operator`, `user`
- Response (fields read by UI): `administrator`, `none`, `oem`, `operator`, `user`
  - `administrator` validation: custom-fn; msg=invalid_vendor_specific_stringrefer_help_for_more_information
  - `operator` validation: custom-fn; msg=invalid_vendor_specific_stringrefer_help_for_more_information
  - `user` validation: custom-fn; msg=invalid_vendor_specific_stringrefer_help_for_more_information
  - `oem` validation: custom-fn; msg=invalid_vendor_specific_stringrefer_help_for_more_information
  - `none` validation: custom-fn; msg=invalid_vendor_specific_stringrefer_help_for_more_information

#### `GET/PUT /api/settings/radius/general`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/ext_users/radius/general_radius`
- Model: `models//general_radius`
- Request: `enable`, `kvm`, `port`, `secret`, `server_address`, `vmedia`
- Response (fields read by UI): `enable`, `kvm`, `port`, `secret`, `server_address`, `vmedia`
  - `server_address` validation: custom-fn
  - `port` validation: range=[1,65535]; msg=invalid_port_number
  - `secret` validation: custom-fn; msg=invalid_secret_phrase


### Settings: date & time / NTP (1)

#### `GET/PUT /api/settings/date-time`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u-2t.har GET 200
- UI: `EventLogView`, `SmbiosInfoView`, `settings/date_time.js`, `settings/video/auto/trigger_settings`, `settings/video/sol/sol_trigger_settings`
- Model: `models/datetime/date_time`
- Request: `auto_date (X470 only)`, `daylight (X470 only)`, `ntp_auto_date (B650 only)`, `primary_ntp`, `ptp_auto_date (B650 only)`, `ptp_delaymech (B650 only)`, `ptp_inlatency (B650 only)`, `ptp_interface (B650 only)`, `ptp_ipmode (B650 only)`, `ptp_logdelayint (B650 only)`, `ptp_maxmasters (B650 only)`, `ptp_outlatency (B650 only)`, `ptp_panicmode (B650 only)`, `ptp_preset (B650 only)`, `ptp_priority1 (B650 only)`, `ptp_transport (B650 only)`, `ptp_unicastip (B650 only)`, `secondary_ntp`, `timestamp`, `timezone`, `utc_minutes`
- Response fields (from capture + UI): `auto_date (only in x470d4u.har)`, `daylight (only in x470d4u.har)`, `id`, `localized_timestamp`, `ntp_auto_date (only in b650d4u-2t.har)`, `primary_ntp`, `secondary_ntp`, `timestamp`, `timezone`, `utc_minutes`, `interface_name (read by B650 UI; absent from capture)`, `ntp_server_status (read by X470 UI; absent from capture)`, `ptp_auto_date (read by B650 UI; absent from capture)`
  - Live (x470d4u.har): GET 200; response `{id:int,primary_ntp:str,secondary_ntp:str,auto_date:int,localized_timestamp:int,utc_minutes:int,daylight:int,timestamp:int,timezone:str}`
  - Live (b650d4u-2t.har): GET 200; response `{id:int,primary_ntp:str,secondary_ntp:str,ntp_auto_date:int,localized_timestamp:int,utc_minutes:int,timestamp:int,timezone:str}`
  - `timezone` values: `GMT+14=GMT+14:00`, `GMT+13:45=GMT+13:45`, `GMT+13=GMT+13:00`, `GMT+12:45=GMT+12:45`, `GMT+12=GMT+12:00`, `GMT+11:30=GMT+11:30`, `GMT+11=GMT+11:00`, `GMT+10:30=GMT+10:30`, `GMT+10=GMT+10:00`, `GMT+9:45=GMT+09:45`, `GMT+9:30=GMT+09:30`, `GMT+9=GMT+09:00`, `GMT+8:45=GMT+08:45`, `GMT+8:30=GMT+08:30`, `GMT+8=GMT+08:00`, `GMT+7:30=GMT+07:30`, `GMT+7=GMT+07:00`, `GMT+6:30=GMT+06:30`, `GMT+6=GMT+06:00`, `GMT+5:45=GMT+05:45`, `GMT+5:30=GMT+05:30`, `GMT+5=GMT+05:00`, `GMT+4:30=GMT+04:30`, `GMT+4=GMT+04:00`, `GMT+3:30=GMT+03:30`, `GMT+3=GMT+03:00`, `GMT+2:30=GMT+02:30`, `GMT+2=GMT+02:00`, `GMT+1:30=GMT+01:30`, `GMT+1=GMT+01:00`, `GMT+0=GMT+00:00`, `GMT-1=GMT-01:00`, `GMT-2=GMT-02:00`, `GMT-2:30=GMT-02:30`, `GMT-3=GMT-03:00`, `GMT-3:30=GMT-03:30`, `GMT-4=GMT-04:00`, `GMT-4:30=GMT-04:30`, `GMT-5=GMT-05:00`, `GMT-6=GMT-06:00`
  - `ptp_preset` values: `0=`, `1=`
  - `ptp_transport` values: `0=`
  - `ptp_ipmode` values: `1=`
  - `ptp_delaymech` values: `0=`, `1=`
  - `primary_ntp` validation: custom-fn; msg=date_time_setting_valid_ntp
  - `secondary_ntp` validation: custom-fn; msg=date_time_setting_valid_ntp
  - `ptp_unicastip` validation: custom-fn; msg=ptp_auto_date / valid_ptp_unicaste_is_required
  - `ptp_interface` validation: msg=primary_ptp_interface_is_required / ptp_auto_date
- Notes: HAR-confirmed {id, primary_ntp, secondary_ntp, auto_date, localized_timestamp, utc_minutes, daylight, timestamp, timezone}.


### Settings: SMTP, PEF, alerts & SNMP (12)

#### `GET /api/settings/getSnmpv2`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/snmp_v2`
- Response (fields read by UI): `read_community`, `snmp_v2_access`, `write_community`

#### `GET/PUT /api/settings/lan_destination/snmptrap_user`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/pef/lan_destinations_edit_item`
- Model: `models/snmptrap_user`
- Request: `channel`, `lan_channel`, `lan_destination_number`, `snmp_trap_dest_id`, `snmp_trap_user`, `snmp_trap_version`

#### `GET /api/settings/pef/alert_policies`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/pef/alert_policies/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/pef/alert_policies`, `settings/pef/alert_policies_edit_item`, `settings/pef/alert_policies_item`
- Model: `collection//alert_policies`
- Request: `alert_string`, `alert_string_key`, `channel_number`, `destination_id`, `enable_policy`, `policy_action`, `policy_group`
- Response (fields read by UI): `channel_number`, `destination_id (B650 only)`, `enable_policy (B650 only)`, `id`, `policy_action (B650 only)`, `policy_group (B650 only)`
  - `policy_group` values: `1=`, `2=`, `3=`, `4=`, `5=`, `6=`, `7=`, `8=`, `9=`, `10=`, `11=`, `12=`, `13=`, `14=`, `15=`
  - `policy_action` values: `always_send_alert=`, `skip_on_prev_success_and_proceed=`, `stop_on_prev_success=`, `switch_channel_on_prev_success=`, `switch_alert_type_on_prev_success=`
  - `destination_id` values: `1=`, `2=`, `3=`, `4=`, `5=`, `6=`, `7=`, `8=`, `9=`, `10=`, `11=`, `12=`, `13=`, `14=`, `15=`
  - `alert_string_key` values: `1=1`, `2=2`, `3=3`, `4=4`, `5=5`, `6=6`, `7=7`, `8=8`, `9=9`, `10=10`, `11=11`, `12=12`, `13=13`, `14=14`, `15=15`, `16=16`, `17=17`, `18=18`, `19=19`, `20=20`, `21=21`, `22=22`, `23=23`, `24=24`, `25=25`, `26=26`, `27=27`, `28=28`, `29=29`, `30=30`, `31=31`, `32=32`, `33=33`, `34=34`, `35=35`, `36=36`, `37=37`, `38=38`, `39=39`, `40=40`
  - `destination_id` validation: required=true; msg=invalid_destination_selected
  - `channel_number` validation: required=true; msg=required_channel_number
  - `policy_group` validation: required=true; msg=invalid_policy_group_selected

#### `GET /api/settings/pef/alert_string_keys`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/pef/alert_policies_edit_item`
- Model: `collection//alert_string_keys`

#### `GET /api/settings/pef/destinations`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/pef/alert_policies_edit_item`
- Model: `collection//pef_destinations`

#### `GET /api/settings/pef/event_filters`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/pef/event_filters/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/pef/event_filters`, `settings/pef/event_filters_edit_item`, `settings/pef/event_filters_item`
- Model: `collection//event_filters`
- Request: `alert (B650 only)`, `channel_number (X470 only)`, `enable_filter`, `event_data_1_and_mask`, `event_data_1_compare_1`, `event_data_1_compare_2`, `event_data_1_offset_mask`, `event_data_2_and_mask`, `event_data_2_compare_1`, `event_data_2_compare_2`, `event_data_3_and_mask`, `event_data_3_compare_1`, `event_data_3_compare_2`, `event_options`, `event_trigger`, `generator_id_1`, `generator_id_2`, `generator_type`, `ipmb_lun (X470 only)`, `name`, `policy_group`, `power_action`, `raw_data`, `sensor_type`, `slave_addr_software_id`, `trigger_event_severity`
- Response (fields read by UI): `alert (B650 only)`, `discrete_state (B650 only)`, `enable_filter`, `id`, `name`, `policy_group (B650 only)`, `power_action (B650 only)`, `sensor_state (B650 only)`, `trigger_event_severity (B650 only)`, `type_number`
  - `trigger_event_severity` values: `unspecified=`, `monitor=`, `information=`, `normal=`, `non_critical=`, `critical=`, `non_recoverable=`
  - `power_action` values: `none=`, `power_down=`, `power_cycle=`, `power_reset=`
  - `policy_group` values: `1=`, `2=`, `3=`, `4=`, `5=`, `6=`, `7=`, `8=`, `9=`, `10=`, `11=`, `12=`, `13=`, `14=`, `15=`
  - `channel_number` values: `0=`, `1=`, `2=`, `3=`, `4=`, `5=`, `6=`, `7=`, `8=`, `9=`, `10=`, `11=`, `15=`
  - `ipmb_lun` values: `0=`, `1=`, `2=`, `3=`
  - `sensor_type` values: `Any=`, `temperature=`, `fan=`, `voltage=`, `discrete=`
  - `event_options` values: `all=`
  - `generator_type` values: `slave=event_filtersslave`, `software=event_filterssoftware`
  - `policy_group` validation: required=true; msg=required_policy_group
  - `sensor_type` validation: required=true; msg=required_sensor_type
  - `generator_id_1` validation: custom-fn; msg=enable_filter / invalid_generate_id_1
  - `generator_id_2` validation: custom-fn; msg=enable_filter / invalid_generate_id_2
  - `slave_addr_software_id` validation: custom-fn; msg=enable_filter / invalid_software_id

#### `GET /api/settings/pef/lan_destinations`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/pef/lan_destinations/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/pef/lan_destinations`, `settings/pef/lan_destinations_edit_item`, `settings/pef/lan_destinations_item`
- Model: `collection//lan_destinations`
- Request: `destination_address`, `destination_type`, `format`, `message`, `name`, `subject`
- Response (fields read by UI): `channel_id`, `destination_address`, `destination_type`, `email_format`, `email_id`, `id`, `lan_channel`, `message`, `name`, `snmp (B650 only)`, `snmp_access (B650 only)`, `snmp_authentication_protocol (B650 only)`, `snmp_privacy_protocol (B650 only)`, `subject`
  - `destination_type` values: `snmp=lan_destinationssnmp_trap`, `email=lan_destinationsemail`
  - `destination_address` validation: msg=destination_type / invalid_snmp_destiantion_address_refer_help_for_more_information
  - `name` validation: msg=destination_type / invalid_bmc_user_name_refer_help_for_more_information
  - `subject` validation: msg=destination_type / invalid_subject_refer_help_for_more_information
  - `message` validation: msg=destination_type / invalid_message_refer_help_for_more_information

#### `POST /api/settings/pef/send_test_alert`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/pef/lan_destinations_item`
- Request: `channel_id`, `destination_address`, `destination_type`, `id`, `lan_channel`, `message`, `subject`, `username`

#### `POST /api/settings/setSnmpv2`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/snmp_v2`
- Request: `read_community`, `snmp_v2_access`, `write_community`

#### `GET /api/settings/smtp`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `PUT /api/settings/smtp/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/smtp`
- Model: `collection//smtp`
- Request: `email_id`, `primary_password`, `primary_server_ip`, `primary_server_name`, `primary_smtp_authentication`, `primary_smtp_enable`, `primary_smtp_port`, `primary_smtp_secure_port`, `primary_ssltls_enable`, `primary_starttls_enable`, `primary_username`, `secondary_password`, `secondary_server_ip`, `secondary_server_name`, `secondary_smtp_authentication`, `secondary_smtp_enable`, `secondary_smtp_port`, `secondary_smtp_secure_port`, `secondary_ssltls_enable`, `secondary_starttls_enable`, `secondary_username`, `smtp_default_enable`
- Response (fields read by UI): `ca_info1`, `ca_info2`, `cert_info1`, `cert_info2`, `channel_interface`, `email_id`, `key_info1`, `key_info2`, `primary_password`, `primary_server_ip`, `primary_server_name`, `primary_smtp_authentication`, `primary_smtp_enable`, `primary_smtp_port`, `primary_smtp_secure_port`, `primary_ssltls_enable`, `primary_starttls_enable`, `primary_username`, `secondary_password`, `secondary_server_ip`, `secondary_server_name`, `secondary_smtp_authentication`, `secondary_smtp_enable`, `secondary_smtp_port`, `secondary_smtp_secure_port`, `secondary_ssltls_enable`, `secondary_starttls_enable`, `secondary_username`, `smtp_default_enable`
  - `email_id` validation: pattern="email"; msg=smtp_invalid_email_format_refer_help_content
  - `primary_smtp_enable` validation: custom-fn; msg=primary_server_ip / primary_server_ip_is_required_valid
  - `primary_server_name` validation: custom-fn; msg=primary_server_name_is_required / primary_smtp_enable
  - `primary_smtp_port` validation: custom-fn; msg=primary_server_port_is_required / primary_smtp_enable
  - `primary_smtp_secure_port` validation: custom-fn; msg=primary_server_secure_port_is_required / primary_smtp_enable / primary_ssltls_enable
  - `primary_server_ip` validation: custom-fn; msg=primary_server_ip_is_required_valid / primary_smtp_enable
  - `primary_username` validation: custom-fn; msg=primary_smtp_authentication / primary_smtp_enable / username_for_primary_server_is_required
  - `primary_password` validation: custom-fn; msg=password_for_primary_server_is_required / primary_smtp_authentication / primary_smtp_enable
  - `secondary_server_name` validation: custom-fn; msg=secondary_server_name_is_required / secondary_smtp_enable
  - `secondary_smtp_port` validation: custom-fn; msg=secondary_server_port_is_required / secondary_smtp_enable
  - `secondary_smtp_secure_port` validation: custom-fn; msg=secondary_server_secure_port_is_required / secondary_smtp_enable / secondary_ssltls_enable
  - `secondary_server_ip` validation: custom-fn; msg=secondary_server_ip_is_required / secondary_smtp_enable
  - `secondary_username` validation: custom-fn; msg=secondary_smtp_authentication / secondary_smtp_enable / username_for_secondary_server_is_required
  - `secondary_password` validation: custom-fn; msg=password_for_secondary_server_is_required / secondary_smtp_authentication / secondary_smtp_enable

#### `POST /api/settings/smtp/smtpupload`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/smtp`
- Request: `multipart:auth (B650 only)`, `multipart:upload_cacert1`, `multipart:upload_cacert2`, `multipart:upload_cert1`, `multipart:upload_cert2`, `multipart:upload_key1`, `multipart:upload_key2`

#### `GET /api/snmp/mibfile`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `miscellaneous/MibFileView`


### Settings: firewall (6)

#### `GET /api/settings/firewall`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `PUT /api/settings/firewall/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/firewall/general_firewall_settings/add_firewall_settings`, `settings/firewall/general_firewall_settings/general_firewall`, `settings/firewall/general_firewall_settings/general_firewall_rules`, `settings/firewall/general_firewall_settings/general_firewall_rules_edit_item`, `settings/firewall/general_firewall_settings/general_firewall_rules_item`
- Model: `collection//general_firewall_rules`, `models//add_firewall_settings`, `models//general_firewall`
- Request: `block_all`, `end_time`, `flush_all`, `start_time`, `timeout`
- Response (fields read by UI): `block_all`, `end_time`, `flush_all`, `id`, `start_time`, `timeout`
  - `block_all` values: `IPv4=`, `IPv6=`, `Both=`
  - `start_time` validation: msg=start_time_required_when_timeout_is_set
  - `end_time` validation: msg=end_time_required_when_timeout_is_set

#### `GET/PUT /api/settings/firewall-ip-rule`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/firewall/ip_firewall/add_ip_rule`
- Model: `models//add_ip_rule`
- Request: `end_time`, `ip_end`, `ip_start`, `rule`, `start_time`, `timeout`
- Response (fields read by UI): `end_time`, `ip_end`, `ip_start`, `rule`, `start_time`, `timeout`
  - `rule` values: `allow=`, `block=`
  - `ip_start` validation: custom-fn; msg=invalid_ip_address_refer_help_for_more_information
  - `ip_end` validation: custom-fn; msg=invalid_ip_address_refer_help_for_more_information
  - `start_time` validation: custom-fn; msg=start_time_required_when_timeout_is_set
  - `end_time` validation: custom-fn; msg=end_time_required_when_timeout_is_set_and_end_time_should_be_greater_than_the_start_time
  - `rule` validation: required=true; msg=invalid_rule_selected

#### `GET /api/settings/firewall-ip-rules`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/firewall-ip-rules/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/firewall/ip_firewall/add_ip_rule`, `settings/firewall/ip_firewall/ip_rules`, `settings/firewall/ip_firewall/ip_rules_edit_item`, `settings/firewall/ip_firewall/ip_rules_item`
- Model: `collection//ip_rules`, `models//add_ip_rule`
- Request: `end_time`, `ip_end`, `ip_start`, `rule`, `start_time`, `timeout`
- Response (fields read by UI): `end_time`, `id`, `ip_end`, `ip_start`, `rule`, `start_time`, `timeout`
  - `rule` values: `allow=`, `block=`
  - `start_time` validation: msg=start_time_required_when_timeout_is_set
  - `end_time` validation: msg=end_time_required_when_timeout_is_set
  - `ip_start` validation: custom-fn; msg=invalid_ip_address_refer_help_for_more_information
  - `ip_end` validation: custom-fn; msg=invalid_ip_address_refer_help_for_more_information
  - `rule` validation: required=true; msg=invalid_rule_selected

#### `GET/PUT /api/settings/firewall-port-rule`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/firewall/port_firewall/add_port_rule`
- Model: `models//add_port_rule`
- Request: `data_flow`, `end_time`, `network_type`, `port_end`, `port_start`, `protocol`, `rule`, `start_time`, `timeout`
- Response (fields read by UI): `data_flow`, `end_time`, `network_type`, `port_end`, `port_start`, `protocol`, `rule`, `start_time`, `timeout`
  - `protocol` values: `0=`, `1=`, `2=`
  - `network_type` values: `IPv4=`, `IPv6=`, `Both=`
  - `rule` values: `allow=`, `block=`
  - `data_flow` values: `1=`, `2=`, `0=`
  - `port_start` validation: custom-fn; msg=enter_valid_port_number_refer_help_for_more_information
  - `port_end` validation: custom-fn; msg=enter_valid_port_number_refer_help_for_more_information
  - `start_time` validation: msg=start_time_required_when_timeout_is_set
  - `end_time` validation: custom-fn; msg=end_time_required_when_timeout_is_set

#### `GET /api/settings/firewall-port-rules`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/settings/firewall-port-rules/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/firewall/port_firewall/add_port_rule`, `settings/firewall/port_firewall/port_rules`, `settings/firewall/port_firewall/port_rules_edit_item`, `settings/firewall/port_firewall/port_rules_item`
- Model: `collection//port_rules`, `models//add_port_rule`
- Request: `end_time`, `network_type`, `port_end`, `port_start`, `protocol`, `rule`, `start_time`, `timeout`
- Response (fields read by UI): `data_flow (X470 only)`, `end_time`, `id`, `network_type`, `port_end`, `port_start`, `protocol`, `rule`, `start_time`, `timeout`
  - `protocol` values: `0=`, `1=`, `2=`
  - `network_type` values: `IPv4=`, `IPv6=`, `Both=`
  - `rule` values: `allow=`, `block=`
  - `start_time` validation: msg=start_time_required_when_timeout_is_set
  - `end_time` validation: msg=end_time_required_when_timeout_is_set
  - `rule` validation: required=true; msg=invalid_rule_selected
  - `protocol` validation: required=true; msg=invalid_protocol_selected
  - `network_type` validation: required=true; msg=invalid_network_type_selected
  - `port_start` validation: custom-fn; msg=enter_valid_port_number_refer_help_for_more_information
  - `port_end` validation: custom-fn; msg=enter_valid_port_number_refer_help_for_more_information

#### `DELETE /api/settings/firewall/:id`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/firewall/general_firewall_settings/general_firewall_rules_edit_item`, `settings/firewall/general_firewall_settings/general_firewall_rules_item`


### Settings: fan control (29)

#### `GET /api/asrr/GetFanDuty`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: x470d4u.har GET 200
- UI: `host_interface_info`
- Response fields (from capture + UI): `FAN1_Duty`, `FAN2_Duty`, `FAN3_Duty`, `FAN4_Duty`, `FAN5_Duty`, `FAN6_Duty`, `FAN7_Duty`, `FAN8_Duty`
  - Live (x470d4u.har): GET 200; response `{FAN1_Duty:str,FAN2_Duty:str,FAN3_Duty:str,FAN4_Duty:str,FAN5_Duty:str,FAN6_Duty:str,FAN7_Duty:str,FAN8_Duty:str}`
- Notes: Read by host_interface_info. HAR-confirmed on X470.

#### `GET /api/asrr/settings/getfancloseloopcontroltable`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_closeloopctb`
- Response (fields read by UI): `duty_cycle_percent_of_slowdown_fan_duty_cycle`, `duty_cycle_percent_of_speedup_fan_duty_cycle`, `temp_of_slowdown_fan_duty_cycle`, `temp_of_speedup_fan_duty_cycle`, `time_of_slowdown_fan_duty_cycle`, `time_of_speedup_fan_duty_cycle`

#### `GET /api/asrr/settings/getfancontrolmode`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_mode`
- Response (fields read by UI): `mode1`, `mode10`, `mode11`, `mode12`, `mode13`, `mode14`, `mode15`, `mode16`, `mode2`, `mode3`, `mode4`, `mode5`, `mode6`, `mode7`, `mode8`, `mode9`

#### `GET /api/asrr/settings/getfandutymanualmode`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_mode`
- Response (fields read by UI): `duty1`, `duty10`, `duty11`, `duty12`, `duty13`, `duty14`, `duty15`, `duty16`, `duty2`, `duty3`, `duty4`, `duty5`, `duty6`, `duty7`, `duty8`, `duty9`

#### `GET /api/asrr/settings/getfanopenloopcontroltable`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: b650d4u.har GET 200
- UI: `settings/fan/fan_openloopctb`
- Response fields (from capture + UI): `Get_Duty1`, `Get_Duty10`, `Get_Duty11`, `Get_Duty12`, `Get_Duty13`, `Get_Duty14`, `Get_Duty15`, `Get_Duty16`, `Get_Duty17`, `Get_Duty18`, `Get_Duty19`, `Get_Duty2`, `Get_Duty20`, `Get_Duty21`, `Get_Duty22`, `Get_Duty23`, `Get_Duty24`, `Get_Duty3`, `Get_Duty4`, `Get_Duty5`, `Get_Duty6`, `Get_Duty7`, `Get_Duty8`, `Get_Duty9`, `Get_Temp1`, `Get_Temp10`, `Get_Temp11`, `Get_Temp12`, `Get_Temp13`, `Get_Temp14`, `Get_Temp15`, `Get_Temp16`, `Get_Temp17`, `Get_Temp18`, `Get_Temp19`, `Get_Temp2`, `Get_Temp20`, `Get_Temp21`, `Get_Temp22`, `Get_Temp23`, `Get_Temp24`, `Get_Temp3`, `Get_Temp4`, `Get_Temp5`, `Get_Temp6`, `Get_Temp7`, `Get_Temp8`, `Get_Temp9`
  - Live (b650d4u.har): GET 200; response `[{Get_Temp1:int,Get_Temp2:int,Get_Temp3:int,Get_Temp4:int,Get_Temp5:int,Get_Temp6:int,Get_Temp7:int,Get_Temp8:int,Get_Temp9:int,Get_Temp10:int,Get_Temp11:int,Get_Temp12:int,Get_Temp13:int,Get_Temp14:int,Get_Temp15:int,Get_Temp16:int,Get_Temp17:int,Get_Temp18:int,Get_Temp19:int,Get_Temp20:int,Get_Temp21:int,Get_Temp22:int,Get_Temp23:int,Get_Temp24:int,Get_Duty1:int,Get_Duty2:int,Get_Duty3:int,Get_Duty4:int,Get_Duty5:int,Get_Duty6:int,Get_Duty7:int,Get_Duty8:int,Get_Duty9:int,Get_Duty10:int,Get_Duty11:int,Get_Duty12:int,Get_Duty13:int,Get_Duty14:int,Get_Duty15:int,Get_Duty16:int,Get_Duty17:int,Get_Duty18:int,Get_Duty19:int,Get_Duty20:int,Get_Duty21:int,Get_Duty22:int,Get_Duty23:int,Get_Duty24:int}]`

#### `GET /api/asrr/settings/getfantempsensorandcorrespondfantable`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_tscft`
- Response (fields read by UI): `tscft_closetablenumber`, `tscft_fan1`, `tscft_fan10`, `tscft_fan11`, `tscft_fan12`, `tscft_fan13`, `tscft_fan14`, `tscft_fan15`, `tscft_fan16`, `tscft_fan2`, `tscft_fan3`, `tscft_fan4`, `tscft_fan5`, `tscft_fan6`, `tscft_fan7`, `tscft_fan8`, `tscft_fan9`, `tscft_opentablenumber`
  - `tscft_opentablenumber` values: `0=`, `1=`, `2=`, `3=`, `4=`, `5=`, `6=`, `7=`, `8=`
  - `tscft_closetablenumber` values: `0=`, `1=`, `2=`, `3=`, `4=`, `5=`, `6=`, `7=`, `8=`, `9=`, `10=`

#### `GET /api/asrr/settings/getsupportfan`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_mode`
- Response (fields read by UI): `tscft_support_fan1`, `tscft_support_fan10`, `tscft_support_fan11`, `tscft_support_fan12`, `tscft_support_fan13`, `tscft_support_fan14`, `tscft_support_fan15`, `tscft_support_fan16`, `tscft_support_fan2`, `tscft_support_fan3`, `tscft_support_fan4`, `tscft_support_fan5`, `tscft_support_fan6`, `tscft_support_fan7`, `tscft_support_fan8`, `tscft_support_fan9`

#### `GET /api/asrr/settings/gettscftsupportfan`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_tscft`
- Response (fields read by UI): `tscft_support_fan1`, `tscft_support_fan10`, `tscft_support_fan11`, `tscft_support_fan12`, `tscft_support_fan13`, `tscft_support_fan14`, `tscft_support_fan15`, `tscft_support_fan16`, `tscft_support_fan2`, `tscft_support_fan3`, `tscft_support_fan4`, `tscft_support_fan5`, `tscft_support_fan6`, `tscft_support_fan7`, `tscft_support_fan8`, `tscft_support_fan9`

#### `POST /api/asrr/settings/setclearallfansetting`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_mode`

#### `POST /api/asrr/settings/setcloselooptablenumber`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_closeloopctb`
- Request: `table_numble`
  - `table_numble` values: `1=`, `2=`, `3=`, `4=`, `5=`, `6=`, `7=`, `8=`, `9=`, `10=`

#### `POST /api/asrr/settings/setfancloseloopcontroltable`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_closeloopctb`
- Request: `duty_cycle_percent_of_slowdown_fan_duty_cycle`, `duty_cycle_percent_of_speedup_fan_duty_cycle`, `table_numble`, `temp_of_slowdown_fan_duty_cycle`, `temp_of_speedup_fan_duty_cycle`, `time_of_slowdown_fan_duty_cycle`, `time_of_speedup_fan_duty_cycle`
  - `table_numble` values: `1=`, `2=`, `3=`, `4=`, `5=`, `6=`, `7=`, `8=`, `9=`, `10=`

#### `POST /api/asrr/settings/setfancontrolmode`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_mode`
- Request: `mode1`, `mode10`, `mode11`, `mode12`, `mode13`, `mode14`, `mode15`, `mode16`, `mode2`, `mode3`, `mode4`, `mode5`, `mode6`, `mode7`, `mode8`, `mode9`

#### `POST /api/asrr/settings/setfandutymanualmode`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_mode`
- Request: `duty1`, `duty10`, `duty11`, `duty12`, `duty13`, `duty14`, `duty15`, `duty16`, `duty2`, `duty3`, `duty4`, `duty5`, `duty6`, `duty7`, `duty8`, `duty9`

#### `POST /api/asrr/settings/setfanopenloopcontroltable`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_openloopctb`
- Request: `duty1`, `duty10`, `duty11`, `duty12`, `duty13`, `duty14`, `duty15`, `duty16`, `duty17`, `duty18`, `duty19`, `duty2`, `duty20`, `duty21`, `duty22`, `duty23`, `duty24`, `duty3`, `duty4`, `duty5`, `duty6`, `duty7`, `duty8`, `duty9`, `table_numble`, `temp1`, `temp10`, `temp11`, `temp12`, `temp13`, `temp14`, `temp15`, `temp16`, `temp17`, `temp18`, `temp19`, `temp2`, `temp20`, `temp21`, `temp22`, `temp23`, `temp24`, `temp3`, `temp4`, `temp5`, `temp6`, `temp7`, `temp8`, `temp9`
  - `table_numble` values: `1=`, `2=`, `3=`, `4=`, `5=`, `6=`, `7=`, `8=`

#### `POST /api/asrr/settings/setfantempsensorandcorrespondfantable`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_tscft`
- Request: `closetable_numble`, `opentable_numble`, `tscft_fan1`, `tscft_fan10`, `tscft_fan11`, `tscft_fan12`, `tscft_fan13`, `tscft_fan14`, `tscft_fan15`, `tscft_fan16`, `tscft_fan2`, `tscft_fan3`, `tscft_fan4`, `tscft_fan5`, `tscft_fan6`, `tscft_fan7`, `tscft_fan8`, `tscft_fan9`, `tscft_sensor_number`

#### `POST /api/asrr/settings/setopenlooptablenumber`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: b650d4u.har POST 200
- UI: `settings/fan/fan_openloopctb`
- Request: `table_numble`
  - Live (b650d4u.har): POST 200; request application/json `{table_numble:int}`; response `{}`
  - `table_numble` values: `1=`, `2=`, `3=`, `4=`, `5=`, `6=`, `7=`, `8=`

#### `POST /api/asrr/settings/settscftsensornumber`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_tscft`
- Request: `tscft_sensor_number`

#### `GET /api/settings/fan/assignment`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `PUT /api/settings/fan/assignment/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/fan/fan_assignment`
- Model: `collection/fan_assignment`
- Response (fields read by UI): `ClosedLoopTable`, `Name`, `name`, `num_of_assignment`, `num_of_closed`, `num_of_open`, `OpenLoopTable`, `Pwm`, `SelectedFan`, `sensor_number`, `TempSensor`

#### `GET /api/settings/fan/closedloop`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `PUT /api/settings/fan/closedloop/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/fan/fan_closedloop`
- Model: `collection/fan_closedloop`
- Request: `RampDownDuty`, `RampDownInterval`, `RampDownTemp`, `RampUpDuty`, `RampUpInterval`, `RampUpTemp`, `ThreshouldTemp`
- Response (fields read by UI): `0`, `max_ramp_duty`, `max_ramp_interval`, `max_temp`, `min_ramp_duty`, `min_ramp_interval`, `min_temp`, `num_of_closed`, `RampDownDuty`, `RampDownInterval`, `RampDownTemp`, `RampUpDuty`, `RampUpInterval`, `RampUpTemp`, `ThreshouldTemp`

#### `GET /api/settings/fan/fan_closeloopctb`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 unknown, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_closeloopctb`
- Model: `collection//fan_closeloopctb`

#### `GET /api/settings/fan/fan_mode`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 unknown, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_mode`
- Model: `collection//fan_mode`

#### `GET /api/settings/fan/fan_openloopctb`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 unknown, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_openloopctb`
- Model: `collection//fan_openloopctb`

#### `GET /api/settings/fan/fan_tscft`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 unknown, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/fan/fan_tscft`
- Model: `collection//fan_tscft`
- Response (fields read by UI): `name`, `sensor_number`, `type_number`

#### `POST /api/settings/fan/load_default`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/fan/fan_mode`

#### `GET/PUT /api/settings/fan/manual`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/fan/fan_manual`
- Model: `models/fan_manual`
- Response (fields read by UI): `max_duty`, `min_duty`, `Name`, `num_of_fan`, `Pwm`

#### `GET/PUT /api/settings/fan/mode`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/fan/fan_mode`
- Model: `models/fan_mode`
- Response (fields read by UI): `default_mode`, `fw_update`, `max_duty`, `min_duty`, `misc_flag`, `Name`, `num_of_fan`, `platform`, `Pwm`

#### `GET /api/settings/fan/names`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/fan/fan_assignment`, `settings/fan/fan_manual`, `settings/fan/fan_mode`
- Model: `collection/fan_names`
- Response (fields read by UI): `num_of_assignment`

#### `GET /api/settings/fan/openloop`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `PUT /api/settings/fan/openloop/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/fan/fan_openloop`
- Model: `collection/fan_openloop`
- Request: `Hysteresis`, `RampDownDuty`, `RampDownInterval`, `RampUpDuty`, `RampUpInterval`
- Response (fields read by UI): `0`, `Hysteresis`, `max_duty`, `max_ramp_duty`, `max_ramp_interval`, `max_temp`, `min_duty`, `min_ramp_duty`, `min_ramp_interval`, `min_temp`, `num_of_entry`, `num_of_open`, `RampDownDuty`, `RampDownInterval`, `RampUpDuty`, `RampUpInterval`

#### `GET /api/settings/fan/temp_sensors`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/fan/fan_assignment`
- Model: `collection/fan_temp_sensors`


### Settings: video recording / BSOD / SOL recording (7)

#### `GET /api/settings/download_image`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/bsod`
- Notes: BSOD screenshot as an image blob (XHR with X-CSRFTOKEN).

#### `GET /api/settings/sol-recorded-video`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/video/sol/sol_recorded_video`, `settings/video/sol/sol_recorded_video_item`
- Model: `collection//sol_recorded_video`
- Response (fields read by UI): `file_name`, `id (B650 only)`, `timestamp (B650 only)`

#### `GET/PUT /api/settings/video/pre-event`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/video/auto/pre_event`
- Model: `models//pre_event`
- Request: `comp_mode`, `enable`, `fps`, `video_duration`, `video_quality`
- Response (fields read by UI): `comp_mode`, `enable`, `fps`, `id`, `video_duration`, `video_quality`
  - `video_quality` values: `0=`, `1=`, `2=`, `3=`, `4=`
  - `comp_mode` values: `0=`, `1=`, `2=`, `3=`
  - `fps` values: `1=`, `2=`, `3=`, `4=`
  - `video_duration` values: `10=`, `20=`, `30=`, `40=`, `50=`, `60=`

#### `GET/PUT /api/settings/video/remote-storage`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200
- UI: `VideoLogView`, `settings/video/auto/remote_storage`
- Model: `models//remote_storage`, `models/remote_storage`
- Request: `domain_name`, `ip_address`, `max_dumps`, `max_duration`, `max_size`, `password`, `remote_path`, `remote_support`, `share_type`, `user_name`
- Response fields (from capture + UI): `domain_name`, `id`, `ip_address`, `max_dumps`, `max_duration`, `max_size`, `mount_status`, `password`, `remote_path`, `remote_support`, `share_type`, `user_name`
  - Live (x470d4u.har): GET 200; response `{id:int,remote_support:int,mount_status:int,max_dumps:int,max_duration:int,max_size:int,ip_address:str,remote_path:str,share_type:str,user_name:str,password:str,domain_name:str}`
  - `share_type` values: `NFS=remote_storagenfs`, `CIFS=remote_storagecifs`
  - `max_dumps` validation: custom-fn; msg=invalid_maximum_dumps_refer_help_for_more_information / remote_support
  - `max_duration` validation: custom-fn; msg=invalid_maximum_duration_refer_help_for_more_information / remote_support
  - `max_size` validation: custom-fn; msg=invalid_maximum_size_refer_help_for_more_information / remote_support
  - `ip_address` validation: custom-fn; msg=invalid_server_address_refer_help_for_more_information / remote_support
  - `remote_path` validation: pattern=/(^[a-zA-Z0-9~@#$^()_+=[\]{}\/; msg=invalid_path_refer_help_for_more_information / remote_support
  - `user_name` validation: msg=remote_support / username_is_required_for_cifs_location
  - `password` validation: msg=password_is_required_for_cifs_location / remote_support
  - `domain_name` validation: custom-fn; msg=invalid_domain_name_refer_help_for_more_information / remote_support
- Notes: HAR-confirmed {id, remote_support, mount_status, max_dumps, max_duration, max_size, ip_address, remote_path, share_type, user_name, password, domain_name}.

#### `GET/PUT /api/settings/video/sol-remote-storage`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/video/sol/sol_remote_storage`
- Model: `models//sol_remote_storage`
- Request: `allowed_file_count (B650 only)`, `allowed_file_size (B650 only)`, `domain_name`, `ip_address`, `log_file_count`, `log_size`, `password`, `remote_path`, `remote_support`, `share_type`, `user_name`
- Response (fields read by UI): `allowed_file_count (B650 only)`, `allowed_file_size (B650 only)`, `domain_name`, `ip_address`, `log_file_count`, `log_size`, `password`, `remote_path`, `remote_support`, `share_type`, `user_name`
  - `share_type` values: `NFS=sol_remote_storagenfs`, `CIFS=sol_remote_storagecifs`
  - `log_size` validation: required=true; pattern="number"; msg=invalid_log_sizerefer_help_for_more_information
  - `log_file_count` validation: required=true; range=[1,10]; msg=invalid_log_file_countrefer_help_for_more_information
  - `ip_address` validation: custom-fn; msg=invalid_server_address_refer_help_for_more_information / remote_support
  - `remote_path` validation: pattern=/(^[a-zA-Z0-9~@#$^()_+=[\]{}\/; msg=invalid_path_refer_help_for_more_information / remote_support
  - `user_name` validation: msg=username_is_required_for_cifs_location
  - `password` validation: msg=password_is_required_for_cifs_location

#### `GET/PUT /api/settings/video/sol-triggers`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/video/sol/sol_trigger_settings`
- Model: `models//sol_trigger_settings`
- Request: `chasis_power_off_events_enable`, `chasis_power_on_events_enable`, `chasis_reset_events_enable`, `critical_events_enable`, `date_time_events_enable`, `date_time_to_record`, `fan_state_events_enable`, `lpc_reset_events_enable`, `non_critical_events_enable`, `non_recoverable_events_enable`, `NTPtimestamp`, `watchdog_events_enable`
- Response (fields read by UI): `date_time_events_enable (B650 only)`, `localized_timestamp`
  - `date_time_to_record` validation: custom-fn; msg=date_time_events_enable / recording_time_must_be_in_future

#### `GET/PUT /api/settings/video/triggers`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/video/auto/trigger_settings`
- Model: `models//trigger_settings`
- Request: `chassis_power_off_events_enable`, `chassis_power_on_events_enable`, `chassis_reset_events_enable`, `critical_events_enable`, `date_time_events_enable`, `date_time_to_record`, `fan_state_events_enable`, `lpc_reset_events_enable`, `non_critical_events_enable`, `non_recoverable_events_enable`, `NTPtimestamp`, `pre_event_video`, `watchdog_events_enable`
- Response (fields read by UI): `date_time_events_enable (B650 only)`, `localized_timestamp`
  - `date_time_to_record` validation: custom-fn; msg=date_time_events_enable / recording_time_must_be_in_future


### Settings: licensing (2)

#### `GET/PUT /api/settings/license`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `settings/license/license_add`
- Model: `models//license_add`
- Request: `license_key`
- Response (fields read by UI): `license_key`
  - `license_key` validation: custom-fn; msg=the_length_of_license_key_is_invalid_refer_help_for_more_information

#### `GET /api/settings/licenses`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `PUT /api/settings/licenses/:id` (Backbone collection model, `:id` = item `id`)
- UI: `settings/license/view_license`, `settings/license/view_license_edit_item`, `settings/license/view_license_item`
- Model: `collection//licenses`
- Response (fields read by UI): `feature`, `id`, `validity`


### Settings: BIOS / platform OEM (3)

#### `GET /api/asrr/getbioscode`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u-2t.har GET 200
- UI: `miscellaneous/postsnoopView`
- Model: `models/bios_post_snoop`
- Response fields (from capture + UI): `postdata`, `poststatus`, `postdata1 (read by X470 UI; absent from capture)`
  - Live (b650d4u-2t.har): GET 200; response `{poststatus:int,postdata:int}`

#### `GET/PUT /api/settings/ocprofile`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/oc_profile`
- Model: `models/oc_profile`
- Request: `action`
- Response (fields read by UI): `numberofprofiles`, `profileselection`

#### `GET/PUT /api/settings/telemetrycups`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `settings/telemetry_cups/cups_config`
- Model: `models//cups_config`
- Request: `AveragingPeriod`, `Interval`
- Response (fields read by UI): `AveragingPeriod`, `DynamicCoreLoadFactor`, `DynamicIioLoadFactor`, `DynamicMemoryLoadFactor`, `Interval`, `StaticCoreLoadFactor`, `StaticIioLoadFactor`, `StaticMemoryLoadFactor`
  - `Interval` validation: custom-fn; msg=invalid_interval_timerefer_help_for_more_information
  - `AveragingPeriod` validation: custom-fn; msg=invalid_averaging_period_timerefer_help_for_more_information


### Maintenance & firmware update (106)

#### `POST /api/asrr/maintenance/BIOS/PublicKey`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/BiosUpdateView`
- Request: `multipart:publickey`
- Response (fields read by UI): `response (1 ok)`
- Notes: X470 only: upload a BIOS signing public key.

#### `GET /api/asrr/maintenance/BIOS/UpdateOption`
- Boards (UI references): (none of the analysed images)
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: b650d4u-2t.har GET 200
- UI: `(not referenced by the analysed 3.02/7.02/9.05 JS; seen only in a live capture)`
- Response fields (from capture + UI): `action`
  - Live (b650d4u-2t.har): GET 200; response `{action:int}`
- Notes: Not referenced by the analysed 3.02/7.02/9.05 JS. Seen polled 21 times in the B550D4U-2T capture (fw 1.10.00), returning {"action":0}; probably the older ASRR UI's equivalent of /api/asrr/maintenance/fwupdate. No handler string in the 9.05 or X470 binaries.

#### `POST /api/asrr/maintenance/BIOS/configuration`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/BiosUpdateView`
- Request: `action (2 = preserve BIOS settings/NVRAM, 4 = clear)`
- Response (fields read by UI): `response (1 ok, 2 error)`
- Notes: Body is JSON.stringify({action}) but sent with contentType:false, so the Content-Type header is the jQuery default for a string body (text/plain or none). Proxy should send the raw JSON string. X470 sends the value as a string ("2"/"4"), B650 as an int.

#### `POST /api/asrr/maintenance/BIOS/firmware`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/BiosUpdateView`
- Request: `multipart:fwimage (BIOS ROM)`
- Response (fields read by UI): `response (1 ok, 2 verify failed)`, `curversion (B650 only)`, `newversion (B650 only)`
- Notes: ASRock OEM BIOS image upload (multipart). X470 repeats every 90 s until success. Same shape for CPLD, PSU, BP, FPGA, RETIMER on B650 (CPLD also returns cpldbpcurversion).

#### `GET /api/asrr/maintenance/BIOS/flash-progress`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/BiosUpdateView`
- Response (fields read by UI): `action ("Flashing...")`, `progress ("NN%" or "About To Start...")`

#### `GET /api/asrr/maintenance/BIOS/status`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/BiosUpdateView`
- Response (fields read by UI): `response (4 in progress, 1 completed (X470 msg 1), 2 failed, 3 completed (B650: success)), anything else = keep polling`
- Notes: Polled recursively; when response == 4 the UI also reads flash-progress.

#### `POST /api/asrr/maintenance/BIOS/upgrade`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/BiosUpdateView`
- Request: `action (1 flash on next host power-off, 2 flash immediately without power action, 3 shut host down and flash)`
- Response (fields read by UI): `response (1 started, 2 error)`
- Notes: Same raw-JSON/contentType:false quirk as configuration.

#### `POST /api/asrr/maintenance/BP/configuration`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/BpUpdateView`
- Request: `action`
- Response (fields read by UI): `response`

#### `POST /api/asrr/maintenance/BP/firmware`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/BpUpdateView`
- Request: `multipart:fwimage`
- Response (fields read by UI): `curversion`, `newversion`, `response`

#### `GET /api/asrr/maintenance/BP/flash-progress`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/BpUpdateView`
- Response (fields read by UI): `action`, `progress`

#### `GET /api/asrr/maintenance/BP/status`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/BpUpdateView`
- Response (fields read by UI): `action`, `progress`, `response`

#### `POST /api/asrr/maintenance/BP/upgrade`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/BpUpdateView`
- Request: `action`
- Response (fields read by UI): `response`

#### `POST /api/asrr/maintenance/CPLD/configuration`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/CpldUpdateView`
- Request: `action`
- Response (fields read by UI): `response`

#### `POST /api/asrr/maintenance/CPLD/firmware`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/CpldUpdateView`
- Request: `multipart:fwimage`
- Response (fields read by UI): `cpldbpcurversion`, `curversion`, `newversion`, `response`

#### `GET /api/asrr/maintenance/CPLD/flash-progress`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/CpldUpdateView`
- Response (fields read by UI): `action`, `progress`

#### `GET /api/asrr/maintenance/CPLD/status`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/CpldUpdateView`
- Response (fields read by UI): `action`, `progress`, `response`

#### `POST /api/asrr/maintenance/CPLD/upgrade`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/CpldUpdateView`
- Request: `action`
- Response (fields read by UI): `response`

#### `POST /api/asrr/maintenance/FPGA/configuration`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FpgaUpdateView`
- Request: `action`
- Response (fields read by UI): `response`

#### `POST /api/asrr/maintenance/FPGA/firmware`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FpgaUpdateView`
- Request: `multipart:fwimage`
- Response (fields read by UI): `curversion`, `newversion`, `response`

#### `GET /api/asrr/maintenance/FPGA/flash-progress`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FpgaUpdateView`
- Response (fields read by UI): `action`, `progress`

#### `GET /api/asrr/maintenance/FPGA/status`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FpgaUpdateView`
- Response (fields read by UI): `action`, `progress`, `response`

#### `POST /api/asrr/maintenance/FPGA/upgrade`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FpgaUpdateView`
- Request: `action`
- Response (fields read by UI): `response`

#### `POST /api/asrr/maintenance/PSU/configuration`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/PsuUpdateView`
- Request: `action`
- Response (fields read by UI): `response`

#### `POST /api/asrr/maintenance/PSU/firmware`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/PsuUpdateView`
- Request: `multipart:fwimage`
- Response (fields read by UI): `curversion`, `newversion`, `response`

#### `GET /api/asrr/maintenance/PSU/flash-progress`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/PsuUpdateView`
- Response (fields read by UI): `action`, `progress`

#### `GET /api/asrr/maintenance/PSU/status`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/PsuUpdateView`
- Response (fields read by UI): `action`, `progress`, `response`

#### `POST /api/asrr/maintenance/PSU/upgrade`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/PsuUpdateView`
- Request: `action`
- Response (fields read by UI): `response`

#### `POST /api/asrr/maintenance/RETIMER/configuration`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/RetimerUpdateView`
- Request: `action`
- Response (fields read by UI): `response`

#### `POST /api/asrr/maintenance/RETIMER/firmware`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/RetimerUpdateView`
- Request: `multipart:fwimage`
- Response (fields read by UI): `curversion`, `newversion`, `response`

#### `GET /api/asrr/maintenance/RETIMER/flash-progress`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/RetimerUpdateView`
- Response (fields read by UI): `action`, `progress`

#### `GET /api/asrr/maintenance/RETIMER/status`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/RetimerUpdateView`
- Response (fields read by UI): `action`, `progress`, `response`

#### `POST /api/asrr/maintenance/RETIMER/upgrade`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/RetimerUpdateView`
- Request: `action`
- Response (fields read by UI): `response`

#### `GET/PUT /api/asrr/maintenance/fwupdate`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/BiosUpdateView`, `maintenance/BpUpdateView`, `maintenance/CpldUpdateView`, `maintenance/FpgaUpdateView`, `maintenance/PsuUpdateView`, `maintenance/RetimerUpdateView`
- Model: `models/fwupdate`
- Request: `action`, `config`, `flag`
- Response (fields read by UI): `action`, `config`, `flag`, `power_status`
  - `config` values: `0`, `1`
- Notes: B650 global OEM update state (models/fwupdate). flag bitmask: 1 BMC, 2 BIOS, 4 YAFU BIOS, 8 PSU, 16 CPLD, 32 BP, 64 RETIMER, 128 FPGA, 256 waiting for host off. config: 2 preserve, 4 delete. action: 0 none/cancel, 1 flash on next boot, 2 on the fly, 3 shut down host to flash. Cancel while waiting = PUT {config:0, action:0, flag:0}.

#### `GET /api/maintenance/ValidSignImage`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Model: `models//fw_signValid`
- Response (fields read by UI): `public_key_info`
- Notes: Re-read after uploading a signing key to confirm it was accepted.

#### `GET/PUT /api/maintenance/backup_config`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/backup_config`
- Model: `models//backup_config`
- Request: `authentication`, `ipmi`, `kvm`, `network`, `ntp`, `snmp`, `syslog`
- Response (fields read by UI): `authentication`, `ipmi`, `kvm`, `network`, `ntp`, `snmp`, `syslog`

#### `GET/PUT /api/maintenance/bmc_recovery`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/bmc_recovery`
- Model: `models//bmc_recovery`
- Request: `bootRetryCount`, `forceRecovery`, `imageName`, `recoveryRetryCount`, `serverAddress`
- Response (fields read by UI): `bootRetryCount`, `forceRecovery`, `imageName`, `recoveryRetryCount`, `serverAddress`
  - `bootRetryCount` validation: min=1; max=5; msg=boot_retry_count_must_be_a_number_between_1_and_5
  - `recoveryRetryCount` validation: min=1; max=5; msg=recovery_retry_count_must_be_a_number_between_1_and_5
  - `serverAddress` validation: msg=invalid_server_address_refer_help_for_more_information
  - `imageName` validation: pattern=/^[a-zA-Z0-9-_.]{0,10}$/; msg=invalid_image_name_refer_help_for_more_information

#### `PUT /api/maintenance/cpld/enableupdatemode`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `BIOS_TABLE_ROW`, `idcode`, `interface`, `interfacenumber`

#### `GET /api/maintenance/cpld/getinfo`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Response (fields read by UI): `idcode`, `interface`, `interfacenumber`, `name`, `version`

#### `GET /api/maintenance/download_config`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/backup_config`

#### `GET/PUT /api/maintenance/dual_image_config`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`, `maintenance/dual_image_config`
- Model: `models//dual_image_config`
- Request: `boot_image`
- Response (fields read by UI): `boot_image`, `fw_version_1`, `fw_version_2`, `state_1`, `state_2`
  - `boot_image` values: `1=boot_image_to_be_booted_from_upon_reset`, `2=boot_image_to_be_booted_from_upon_r`, `0=boot_imagehigher_firmware_version_`, `3=boot_imagelower_firmware_version`, `4=boot_imagemost_recently_updated_f`, `5=boot_imageleast_recently_upda`

#### `PUT /api/maintenance/dualflashimageconfig`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Request: `image_update`, `reboot_bmc`
  - `image_update` values: `0=`, `1=`, `2=`, `3=`

#### `POST /api/maintenance/firmware`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Request: `multipart:fwimage (the .ima BMC image)`
- Notes: X470 BMC image upload (multipart/form-data, field fwimage, contentType false, processData false). The UI re-issues the upload every 90 s until it succeeds (httpsInterval). B650 uses /api/maintenance/firmware/firmware for the same purpose (the dual-image and HPM views still use /api/maintenance/firmware).

#### `PUT /api/maintenance/firmware/VerifyImageFlash`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Request: `preserve_config`, `flash_status`
- Notes: B650 only: post-flash read-back verification; progress is reported through flash-progress again.

#### `PUT /api/maintenance/firmware/dwldfwimg`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/dualFirmwareUpgradeView`
- Request: `PROTO_TYPE (1 = TFTP)`, `preserve_config (B650 only)`
  - `image_update` values: `0=`, `1=`, `2=`, `3=`
- Notes: TFTP path: after PUT /api/maintenance/flash, asks the BMC to download the image from the server configured in /api/maintenance/fwimage_location. Then poll GET dwldfwstatus-progress until progress == 100, then continue at verification.

#### `GET /api/maintenance/firmware/dwldfwstatus-progress`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/dualFirmwareUpgradeView`
- Response (fields read by UI): `state (1-5 while downloading)`, `progress`, `action`

#### `POST /api/maintenance/firmware/firmware`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Request: `multipart:fwimage`
- Notes: B650 BMC image upload; same semantics as /api/maintenance/firmware on X470.

#### `GET /api/maintenance/firmware/flash-progress`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Response (fields read by UI): `state (undefined until flashing starts; 1-5 while active)`, `progress (string, e.g. "45% done" / "Complete")`, `action (string, e.g. "Flashing...")`
- Notes: Polled back-to-back (next request issued from the success handler, no delay). X470: done when parseInt(progress) == 100 or NaN; B650: continues until progress contains "Complete" then issues PUT /api/maintenance/firmware/VerifyImageFlash and polls again.

#### `PUT /api/maintenance/firmware/upgrade`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Request: `preserve_config (0|1)`, `preserve_network (X470 only, 0|1)`, `preserve_user (X470 only, 0|1)`, `flash_status (1 full/force flash, 2 section flash, 4 version-compare flash [B650])`, `section_count (when flash_status == 2)`, `section_names (array of current_image_name, when flash_status == 2)`
- Notes: Starts flashing. JSON body. Follow with GET flash-progress polling.

#### `GET/POST /api/maintenance/firmware/verification`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Response (fields read by UI): `[] array of sections: current_image_name`, `current_image_version1`, `new_image_version`, `section_status (1 = section identical / not selectable)`, `verification_status (bitmask on element 0: 1 differs, 2 size differs, 4 version differs, 16 = full flash required, 32 same version, 64 module size differs)`
- Notes: Validates the uploaded image (signature, board/platform match). Error codes in responseJSON.code: 14540, 14543, 14545, 14549, 14551, 14552, 14553 (signature/platform/size mismatch, see i18n error_code_*). On error the UI calls POST /api/maintenance/reset.

#### `PUT /api/maintenance/flash`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Request: `(X470: empty body, data:{} with JSON content type)`, `preserve_config (B650 only, 0|1)`
  - `image_update` values: `0=`, `1=`, `2=`, `3=`
- Notes: Puts the BMC into firmware-update mode (stops services, reserves RAM). Must precede the image upload. Any later failure must be followed by POST /api/maintenance/reset to leave update mode.

#### `PUT /api/maintenance/flash_image_config`
- Boards (UI references): X470-3.02
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/dualFirmwareUpgradeView`
- Request: `image_update`, `reboot_bmc`
  - `image_update` values: `0=`, `1=`, `2=`, `3=`
- Notes: X470 dual-image page: JSON {image_update, reboot_bmc} before the flash. B650 equivalent: /api/maintenance/dualflashimageconfig.

#### `GET /api/maintenance/flashtrack`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `app`

#### `GET/PUT /api/maintenance/fwimage_location`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: x470d4u.har GET 200; b650d4u.har GET 200
- UI: `maintenance/BiosUpdateView`, `maintenance/FirmwareUpdateWizardView`, `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/NmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`, `maintenance/firmware_image_location`, `maintenance/fwimage_location`
- Model: `models//firmware_image_location`, `models//fwimage_location`
- Request: `image_name`, `protocol_type`, `retry_count`, `server_address`
- Response fields (from capture + UI): `id`, `image_name`, `protocol_type`, `retry_count`, `server_address`, `controller_id (read by B650 UI; absent from capture)`, `controller_name (read by B650 UI; absent from capture)`, `firmware_version (read by B650 UI; absent from capture)`, `globalFwUpdateStatus (read by B650 UI; absent from capture)`, `serial_number (read by B650 UI; absent from capture)`, `vendor_type (read by B650 UI; absent from capture)`
  - Live (x470d4u.har): GET 200; response `{id:int,protocol_type:str,server_address:str,image_name:str,retry_count:int}`
  - Live (b650d4u.har): GET 200; response `{id:int,protocol_type:str,server_address:str,image_name:str,retry_count:int}`
  - `protocol_type` values: `http=fwimage_locationweb_upload_during_flash`, `tftp=fwimage_locationtftp_server`
  - `server_address` validation: msg=invalid_tftp_server_address / protocol_type
  - `image_name` validation: msg=image_name_is_required / protocol_type
  - `retry_count` validation: required=false; range=[0,255]; msg=invalid_tftp_retry_count
- Notes: Firmware image source. protocol_type "http" (web upload during flash) or "tftp" (server_address, image_name, retry_count 0-255). HAR-confirmed {"id":1,"protocol_type":"http","server_address":"","image_name":"","retry_count":0}. Saved with PUT (Backbone save, singleton with id).

#### `GET /api/maintenance/getPlatformID`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpdateWizardView`
- Response (fields read by UI): `platformID`

#### `POST /api/maintenance/hpm`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `multipart:vendor_type (B650 only)`

#### `PUT /api/maintenance/hpm/activatecomponents`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `COMPONENT_ID`

#### `GET /api/maintenance/hpm/componentversions`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `VERS_ID`

#### `PUT /api/maintenance/hpm/exitupdatemode`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`, `maintenance/NVMeMISSDSControllerFirmwareUpdateView`, `maintenance/PldmFirmwareUpdateView`
- Request: `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `FWUPDATEID`, `IS_MMC`, `RAID_CID (B650 only)`

#### `PUT /api/maintenance/hpm/flash`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView ("/api/maintenance/"+suffix)`, `maintenance/HpmFirmwareUpdateView`
- Request: `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `FWUPDATEID`, `SECTION_FLASH`
- Response (fields read by UI): `COMPONENT_DATA_LEN (X470 only)`, `COMPONENT_ID (X470 only)`, `FWUPDATEID (X470 only)`

#### `GET /api/maintenance/hpm/freemem`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Model: `models//free_mem`

#### `GET /api/maintenance/hpm/getbiosversions`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`

#### `PUT /api/maintenance/hpm/preparecomponents`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView ("/api/maintenance/"+suffix)`, `maintenance/HpmFirmwareUpdateView`, `maintenance/PldmFirmwareUpdateView`
- Request: `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `FWUPDATEID`, `IS_MMC (X470 only)`
- Response (fields read by UI): `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `FWUPDATEID`

#### `PUT /api/maintenance/hpm/updatemode`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`, `maintenance/NVMeMISSDSControllerFirmwareUpdateView`, `maintenance/PldmFirmwareUpdateView`
- Response (fields read by UI): `unique_id`

#### `GET /api/maintenance/hpm/upgradestatus`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView ("/api/maintenance/"+suffix)`, `maintenance/HpmFirmwareUpdateView`
- Request: `COMPONENT_ID (X470 only)`, `COMPONENT_ID (query) (B650 only)`, `FWUPDATEID (query) (B650 only)`
- Response (fields read by UI): `COMPONENT_ID (X470 only)`, `PROGRESS (X470 only)`

#### `PUT /api/maintenance/hpm/verifyimage`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `FWUPDATEID`
- Response (fields read by UI): `COMPONENT_ID`

#### `GET /api/maintenance/hpm/verifyimagestatus`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `COMPONENT_DATA_LEN (B650 only)`, `COMPONENT_ID`, `FWUPDATEID (B650 only)`
- Response (fields read by UI): `COMPONENT_ID`, `PROGRESS`

#### `GET /api/maintenance/memory_resilience_technology_information`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/mrt`, `maintenance/mrt_item`
- Model: `collection//mrt`
- Request: `non_secure_port`, `secure_port`, `time_out`
- Response (fields read by UI): `channel`, `dim_id`, `imc`, `score`, `slot`, `socket`
  - `time_out` validation: custom-fn; msg=invalid_time_out_refer_help_for_more_information
  - `non_secure_port` validation: custom-fn; msg=invalid_non_secure_port_refer_help_for_more_information / isFeatureEnabled
  - `secure_port` validation: custom-fn; msg=invalid_secure_port_refer_help_for_more_information

#### `GET /api/maintenance/nm_activate`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/NmFirmwareUpdateView`
- Model: `models//nm_activate`

#### `GET /api/maintenance/nm_compareversion`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/NmFirmwareUpdateView`
- Model: `models//nm_compareversion`

#### `GET /api/maintenance/nm_deactivate`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/NmFirmwareUpdateView`
- Model: `models//nm_deactivate`

#### `GET /api/maintenance/nm_ecfstatus`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/NmFirmwareUpdateView`
- Model: `models//nm_ecfstatus`
- Response (fields read by UI): `nm_progress`

#### `GET /api/maintenance/nm_eraseflash`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/NmFirmwareUpdateView`
- Model: `models//nm_eraseflash`

#### `GET/PUT /api/maintenance/nm_firmware_update`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/NmFirmwareUpdateView`
- Model: `models//nm_firmware_update`

#### `GET/POST /api/maintenance/nm_upload`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`, `maintenance/NmFirmwareUpdateView`
- Model: `models//nm_upload`
- Request: `multipart:fwimage`, `multipart:vendor_type (B650 only)`

#### `PUT /api/maintenance/nvme/activatecomponents`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/NVMeMISSDSControllerFirmwareUpdateView`
- Request: `ssd_id`, `ssd_type`

#### `PUT /api/maintenance/nvme/fwupdate`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/NVMeMISSDSControllerFirmwareUpdateView`
- Request: `ssd_id`, `ssd_type`

#### `GET /api/maintenance/nvme/fwupdateprogress`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/NVMeMISSDSControllerFirmwareUpdateView`
- Request: `ssd_id`, `ssd_type`
- Response (fields read by UI): `ERR`, `RESULT`, `SENT_SIZE`, `TOTAL_SIZE`

#### `POST /api/maintenance/nvme/nvmemissdsfw`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/NVMeMISSDSControllerFirmwareUpdateView`
- Request: `multipart:nvmemissdsfw`

#### `PUT /api/maintenance/pfr/activatecomponents`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `COMPONENT_ID`

#### `PUT /api/maintenance/pfr/exitupdatemode`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `FWUPDATEID`

#### `PUT /api/maintenance/pfr/flash`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`

#### `GET /api/maintenance/pfr/flashimagestatus`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `FWUPDATEID`, `SECTION_FLASH`, `uploadedValue`
- Response (fields read by UI): `PROGRESS`

#### `PUT /api/maintenance/pfr/fwupdateoptions`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`

#### `POST /api/maintenance/pfr/pfrfw`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`

#### `GET /api/maintenance/pfr/pfrfwimagesize`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/FirmwareUpdateWizardView`
- Response (fields read by UI): `pfr_recovery_image_size`

#### `GET /api/maintenance/pfr/pfrgetreadystatus`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`, `maintenance/pfr_information`
- Response (fields read by UI): `BIOS_STATUS`, `CPLD_STATE`, `CPLD_STATUS`

#### `GET /api/maintenance/pfr/pfrversioninfo`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`, `maintenance/pfr_information`
- Model: `models//pfr_information`
- Response (fields read by UI): `BMC_ACTIVE_MAJOR_VER`, `BMC_ACTIVE_MINOR_VER`, `BMC_ACTIVE_SVN`, `BMC_RECOVERY_MAJOR_VER`, `BMC_RECOVERY_MINOR_VER`, `BMC_RECOVERY_SVN`, `CPLD_RELEASE_VER`, `CPLD_SVN`, `PCH_ACTIVE_MAJOR_VER`, `PCH_ACTIVE_MINOR_VER`, `PCH_ACTIVE_SVN`, `PCH_RECOVERY_MAJOR_VER`, `PCH_RECOVERY_MINOR_VER`, `PCH_RECOVERY_SVN`

#### `PUT /api/maintenance/pfr/preparecomponents`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `capsuleCode`, `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `firmwareActivation`, `FLASH_TYPE`, `FWUPDATEID`, `ImageFlag`, `IS_MMC`, `param`

#### `GET /api/maintenance/pfr/seamlessflashstatus`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `COMPONENT_ID`
- Response (fields read by UI): `PROGRESS`

#### `PUT /api/maintenance/pfr/updatemode`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `capsuleCode`, `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `firmwareActivation`, `FLASH_TYPE`, `FWUPDATEID`, `ImageFlag`, `IS_MMC`, `param`
- Response (fields read by UI): `unique_id`

#### `PUT /api/maintenance/pfr/verifyimage`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `FWUPDATEID`

#### `GET /api/maintenance/pfr/verifyimagestatus`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView`
- Request: `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `FWUPDATEID`
- Response (fields read by UI): `PROGRESS`

#### `POST /api/maintenance/pldm`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/PldmFirmwareUpdateView`
- Request: `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `FWUPDATEID`, `multipart:pldm`

#### `GET /api/maintenance/pldm/fwupdateprogress`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/PldmFirmwareUpdateView`
- Request: `FW_EID`, `FWUPDATEID`
- Response (fields read by UI): `activation_method`, `hideLoader`, `progress`, `update_state`, `update_status`

#### `PUT /api/maintenance/pldm/fwupdatestatus`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/PldmFirmwareUpdateView`
- Request: `FW_EID`, `FWUPDATEID`

#### `GET /api/maintenance/pldm/getpldmfwinfo`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/PldmFirmwareUpdateView`
- Request: `FIRMWARE_ID`, `multipart:pldm`
- Response (fields read by UI): `history`

#### `GET/PUT /api/maintenance/preserve`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/NmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`, `maintenance/preserve_configuration`
- Model: `models//preserve_configuration`
- Request: `authentication`, `automationEngine (B650 only)`, `extlog`, `fru`, `ipmi`, `kvm`, `network`, `ntp`, `redfish`, `rsd (B650 only)`, `sdr`, `sel`, `snmp`, `ssh`, `syslog`, `web`
- Response (fields read by UI): `authentication`, `automationEngine (B650 only)`, `extlog`, `fru`, `ipmi`, `kvm`, `network`, `ntp`, `redfish`, `rsd (B650 only)`, `sdr`, `sel`, `snmp`, `ssh`, `syslog`, `web`

#### `PUT /api/maintenance/raid_management/flash`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView ("/api/maintenance/"+suffix)`
- Request: `COMPONENT_DATA_LEN`, `COMPONENT_ID`, `FWUPDATEID`, `RAID_CID`, `SECTION_FLASH`

#### `PUT /api/maintenance/raid_management/preparecomponents`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView ("/api/maintenance/"+suffix)`

#### `GET /api/maintenance/raid_management/upgradestatus`
- Boards (UI references): B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 no, B650-7.02 no, B650-9.05 no
- Seen in HAR: no
- UI: `maintenance/HpmFirmwareUpdateView ("/api/maintenance/"+suffix)`
- Request: `COMPONENT_ID (query)`, `FWUPDATEID (query)`

#### `POST /api/maintenance/reset`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `app`, `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/PldmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Request: `flash_done (B650 only)`, `image_flashed (B650 only)`, `xhr (B650 only)`
- Notes: Body {} (X470 sends data:{} which jQuery serialises as an empty body). Reboots the BMC after flashing, or aborts update mode on cancel/error. The UI logs out and reloads after 120 s. Also sent from window.onbeforeunload while window.fw_upgrade is set.

#### `GET/PUT /api/maintenance/restore_config`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/restore_config`, `maintenance/restore_configuration`
- Model: `models//restore_config`, `models//restore_configuration`
- Request: `config`
- Response (fields read by UI): `config`
  - `config` validation: required=true; msg=upload_config_file

#### `GET/PUT /api/maintenance/restore_defaults`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200
- UI: `maintenance/restore_configuration`, `maintenance/restore_factory_defaults`
- Model: `models//restore_configuration`, `models//restore_factory_defaults`
- Request: `authentication`, `automationEngine (B650 only)`, `extlog`, `fru`, `ipmi`, `kvm`, `network`, `ntp`, `redfish`, `rsd (B650 only)`, `sdr`, `sel`, `snmp`, `ssh`, `syslog`, `web`
- Response fields (from capture + UI): `authentication`, `fru`, `id`, `ipmi`, `kvm`, `network`, `ntp`, `redfish`, `sdr`, `sel`, `snmp`, `ssh`, `syslog`, `web`, `automationEngine (read by B650 UI; absent from capture)`, `extlog (read by UI; absent from capture)`, `rsd (read by B650 UI; absent from capture)`
  - Live (b650d4u.har): GET 200; response `{id:int,sdr:int,fru:int,sel:int,ipmi:int,network:int,ntp:int,snmp:int,ssh:int,kvm:int,authentication:int,syslog:int,web:int,redfish:int}`

#### `GET/POST /api/maintenance/signed-image-key`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 no, B650-9.05 no
- Seen in HAR: x470d4u.har GET 200
- UI: `maintenance/FirmwareUpgradeView`, `maintenance/HpmFirmwareUpdateView`, `maintenance/dualFirmwareUpgradeView`
- Model: `models//fw_signImage`
- Request: `multipart:publicKey (POST)`
- Response fields (from capture + UI): `id`, `key_exists`, `public_key_info`
  - Live (x470d4u.har): GET 200; response `{id:int,key_exists:int,public_key_info:str}`
- Notes: Signed-image public key (feature ENC_IMAGE). GET HAR-confirmed {"id":1,"key_exists":0,"public_key_info":""}. After POST the UI re-reads /api/maintenance/ValidSignImage.

#### `GET/PUT /api/maintenance/system_admin`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: b650d4u.har GET 200
- UI: `maintenance/system_admin`
- Model: `models//system_admin`
- Request: `cpassword`, `enable_change_password`, `enable_user_access`, `password`
- Response fields (from capture + UI): `enable_change_password`, `enable_user_access`, `id`, `username`, `cpassword (read by UI; absent from capture)`, `password (read by UI; absent from capture)`
  - Live (b650d4u.har): GET 200; response `{id:int,enable_change_password:int,enable_user_access:int,username:str}`
  - `password` validation: custom-fn; msg=enable_change_password / password_field_must_be_8_characters_atleast_and_white_space_is_not_allowed
  - `cpassword` validation: custom-fn; msg=enable_change_password / password_and_confirm_password_do_not_match

#### `POST /api/maintenance/upload_restore`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `maintenance/restore_config`, `maintenance/restore_configuration`
- Request: `multipart:config`


### Automation engine (scripts/tasks) (3)

#### `PUT /api/ae_test`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `ScriptEditView`
- Request: `code`, `id`, `name`
- Response (fields read by UI): `error`, `log`

#### `GET/POST/PUT /api/scripts`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- UI: `ScriptEditView`, `ScriptsView`, `TaskDetailsView`
- Model: `collection/ae_scripts`, `collection/scripts`
- Request: `author`, `code`, `id (B650 only)`, `name`
- Response (fields read by UI): `id (B650 only)`, `name (B650 only)`

#### `GET/POST /api/tasks`
- Boards (UI references): X470-3.02, B650-7.02, B650-9.05
- Handler in firmware: X470-3.02 yes, B650-7.02 yes, B650-9.05 yes
- Seen in HAR: no
- Per-item: `DELETE/PUT /api/tasks/:id` (Backbone collection model, `:id` = item `id`)
- UI: `ScriptItemView`, `TaskDetailsView`, `TaskEventsView`, `dashboard/CalendarView`
- Model: `collection/tasks`
- Request: `event_filters`, `schedule`, `scripts`, `task_title`
- Response (fields read by UI): `author (B650 only)`, `event_filters`, `id (B650 only)`, `name (B650 only)`, `schedule`, `scripts`, `task_title`, `timestamp (B650 only)`


---

## 8. Board diff (X470 3.02 versus B650)

Based on the analysed JS bundles; see 6.4 for handler presence per board. Note: the B650D4U capture (fw 1.10.00) also calls several endpoints listed here as X470-only (`/api/asrr/getvetobit`, the asrr fan endpoints), so older B650 firmware shares more of the X470 OEM surface than 7.02/9.05 do.

| Category | Count |
|---|---|
| Both X470 and B650 | 179 |
| X470 3.02 only | 45 |
| B650 only (7.02 and 9.05) | 190 |
| B650 7.02 only | 1 |
| B650 9.05 only | 0 |
| Total distinct paths | 416 |

#### Present on X470 3.02 only (45)

| Path | Area |
|---|---|
| `/api/asrr/session` | Session & authentication |
| `/api/asrr/product-name` | Dashboard & identity |
| `/api/asrr/selsummary` | Dashboard & identity |
| `/api/asrr/getprochot` | Power, chassis & UID |
| `/api/asrr/setprochot` | Power, chassis & UID |
| `/api/chassis` | Power, chassis & UID |
| `/api/logs/getvideotag` | Logs, crash dumps & diagnostics |
| `/api/system_inventory/component_status_update` | Host inventory |
| `/api/system_inventory/cpu_info` | Host inventory |
| `/api/system_inventory/device_count` | Host inventory |
| `/api/system_inventory/dimm_info` | Host inventory |
| `/api/system_inventory/pci_info` | Host inventory |
| `/api/asrr/java-console` | KVM, H5Viewer & SOL |
| `/api/asrr/getvetobit` | Settings: network |
| `/api/asrr/setvetobit` | Settings: network |
| `/api/settings/dns` | Settings: network |
| `/api/settings/network-interfaces` | Settings: network |
| `/api/asrr/getWebEnableTimeout` | Settings: services & sessions |
| `/api/asrr/setWebEnableTimeout` | Settings: services & sessions |
| `/api/settings/web_server_instances` | Settings: services & sessions |
| `/api/settings/firewall-ip-rule` | Settings: firewall |
| `/api/settings/firewall-port-rule` | Settings: firewall |
| `/api/asrr/GetFanDuty` | Settings: fan control |
| `/api/asrr/settings/getfancloseloopcontroltable` | Settings: fan control |
| `/api/asrr/settings/getfancontrolmode` | Settings: fan control |
| `/api/asrr/settings/getfandutymanualmode` | Settings: fan control |
| `/api/asrr/settings/getfanopenloopcontroltable` | Settings: fan control |
| `/api/asrr/settings/getfantempsensorandcorrespondfantable` | Settings: fan control |
| `/api/asrr/settings/getsupportfan` | Settings: fan control |
| `/api/asrr/settings/gettscftsupportfan` | Settings: fan control |
| `/api/asrr/settings/setclearallfansetting` | Settings: fan control |
| `/api/asrr/settings/setcloselooptablenumber` | Settings: fan control |
| `/api/asrr/settings/setfancloseloopcontroltable` | Settings: fan control |
| `/api/asrr/settings/setfancontrolmode` | Settings: fan control |
| `/api/asrr/settings/setfandutymanualmode` | Settings: fan control |
| `/api/asrr/settings/setfanopenloopcontroltable` | Settings: fan control |
| `/api/asrr/settings/setfantempsensorandcorrespondfantable` | Settings: fan control |
| `/api/asrr/settings/setopenlooptablenumber` | Settings: fan control |
| `/api/asrr/settings/settscftsensornumber` | Settings: fan control |
| `/api/settings/fan/fan_closeloopctb` | Settings: fan control |
| `/api/settings/fan/fan_mode` | Settings: fan control |
| `/api/settings/fan/fan_openloopctb` | Settings: fan control |
| `/api/settings/fan/fan_tscft` | Settings: fan control |
| `/api/asrr/maintenance/BIOS/PublicKey` | Maintenance & firmware update |
| `/api/maintenance/flash_image_config` | Maintenance & firmware update |

#### Present on B650 only (191)

| Path | Boards | Area |
|---|---|---|
| `/api/newtabinfo` | B650-7.02, B650-9.05 | Session & authentication |
| `/api/tfa` | B650-7.02, B650-9.05 | Session & authentication |
| `/api/updatenew_password` | B650-7.02, B650-9.05 | Session & authentication |
| `/api/verify_otp` | B650-7.02, B650-9.05 | Session & authentication |
| `/api/extfeatures` | B650-7.02, B650-9.05 | Feature discovery & UI configuration |
| `/api/settings/user-preference` | B650-7.02, B650-9.05 | Feature discovery & UI configuration |
| `/api/status/uptime` | B650-7.02, B650-9.05 | Feature discovery & UI configuration |
| `/api/dashboardsensors` | B650-7.02, B650-9.05 | Dashboard & identity |
| `/api/settings/dashboardinfo` | B650-7.02, B650-9.05 | Dashboard & identity |
| `/api/settings/dashboardntp` | B650-7.02, B650-9.05 | Dashboard & identity |
| `/api/actions/accessbiossetup` | B650-7.02, B650-9.05 | Power, chassis & UID |
| `/api/asrr/GetPowerRestore` | B650-7.02, B650-9.05 | Power, chassis & UID |
| `/api/asrr/PowerSourceID` | B650-7.02 | Power, chassis & UID |
| `/api/asrr/SetPowerRestore` | B650-7.02, B650-9.05 | Power, chassis & UID |
| `/api/settings/amdpowercap` | B650-7.02, B650-9.05 | Power, chassis & UID |
| `/api/pldm/sensors` | B650-7.02, B650-9.05 | Sensors & SDR |
| `/api/pldm/state_sensors` | B650-7.02, B650-9.05 | Sensors & SDR |
| `/api/frueditable` | B650-7.02, B650-9.05 | FRU & PLDM inventory |
| `/api/fruupdate` | B650-7.02, B650-9.05 | FRU & PLDM inventory |
| `/api/pldm/eids` | B650-7.02, B650-9.05 | FRU & PLDM inventory |
| `/api/pldm/entity_association` | B650-7.02, B650-9.05 | FRU & PLDM inventory |
| `/api/pldm/fru` | B650-7.02, B650-9.05 | FRU & PLDM inventory |
| `/api/acd/download_crashdump` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/acd/getcrashdumpstatus` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/acd/set_crashdump_download` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/acd/setcrashdumpstatus` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/host/addc_autonomous_debug_data_collection/error_recovery_mode_and_retry_count` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/host/download_addc_autonomous_debug_data_collection` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/host/get_addc_autonomous_debug_data_collection` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/host/get_crash_dump` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/acd` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/audit` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/debuglogs` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/platform` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/postcode` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/savepostcode` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/system` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/video-log` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/pldm/get_all_event_log` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/settings/log-certificates` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/system_diagnostics/download_system_diagnostics_log` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/system_diagnostics/generate_system_diagnostics_logs` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/system_diagnostics/log` | B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/gpu/nvfpga` | B650-7.02, B650-9.05 | Host inventory |
| `/api/gpu/nvgpu` | B650-7.02, B650-9.05 | Host inventory |
| `/api/gpu/status` | B650-7.02, B650-9.05 | Host inventory |
| `/api/gpu_info` | B650-7.02, B650-9.05 | Host inventory |
| `/api/host/asd/getasdjtaginfo` | B650-7.02, B650-9.05 | Host inventory |
| `/api/host/asd/getasdremotedebuginfo` | B650-7.02, B650-9.05 | Host inventory |
| `/api/host/asd/setasdremotedebuginfo` | B650-7.02, B650-9.05 | Host inventory |
| `/api/host/asd/tls_certificate` | B650-7.02, B650-9.05 | Host inventory |
| `/api/nvdimm_inventory/last_action_info` | B650-7.02, B650-9.05 | Host inventory |
| `/api/nvdimm_inventory/nvdimm_dimm_action` | B650-7.02, B650-9.05 | Host inventory |
| `/api/nvdimm_inventory/nvdimm_dimm_info` | B650-7.02, B650-9.05 | Host inventory |
| `/api/nvdimm_inventory/nvdimm_namespace_info` | B650-7.02, B650-9.05 | Host inventory |
| `/api/nvdimm_inventory/nvdimm_pools_info` | B650-7.02, B650-9.05 | Host inventory |
| `/api/nvdimm_inventory/nvdimmdimm_fw_flash_status` | B650-7.02, B650-9.05 | Host inventory |
| `/api/smbiosdata` | B650-7.02, B650-9.05 | Host inventory |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_port_assign_unassign` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_port_error_counters` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_port_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_reset_port_error_counters` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_station_ram_error_counters` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/brcm_pcie_switch_management/brcm_pcie_switch_station_reset_ram_error_counters` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/nic_management/nic_controller_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/nvme_management/nvme_basic_mi_vpd_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/nvme_management/nvme_controller_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/nvme_management/nvme_p3700_vital_product_data` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/nvme_mi_management/nvme_mi_clear_status_flags` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/nvme_mi_management/nvme_mi_set_status_flags` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/nvme_mi_management/nvme_mi_ssd_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/nvme_mi_management/nvme_mi_subsystem_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/nvme_mi_management/nvme_mi_vpd_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/pcie_switch_management/get_pcie_bifurcation_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/pcie_switch_management/pcie_mgmt_dev_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/pcie_switch_management/pcie_partition_count` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/pcie_switch_management/pcie_port_bind` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/pcie_switch_management/pcie_port_link_status` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/pcie_switch_management/pcie_port_unbind` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/pcie_switch_management/pcie_switch_reset` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/pcie_switch_management/set_pcie_bifurcation_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/pcie_switch_management/validate_port_unbind` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/pcie_switch_mgmt/pcie_partition_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/clear_all_foreign_cmd` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/create_lu_cache` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/dedicated_hot_spare_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/manage_array_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/manage_data_drive_array` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/manage_spare_drive_array` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/patrol_read_properties` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/patrol_read_start_stop_cmd` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/physical_device_smart_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/physical_state_cmd` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/raid_topology_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/sesenclosureinfo` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/sesenclosureinfobrcm` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/sasit_management/sasit_enclosure_info` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/sasit_management/sasit_event_log` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/sasit_management/sasit_physical_start_stop_locate_cmd` | B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/remote_control/get/kvm/launch` | B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/settings/javasol` | B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/settings/media/active_redirections` | B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/settings/media/h5viewercfg` | B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/settings/video/sol_configurations` | B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/sol/session` | B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/sol/sol_configurations` | B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/sol/solcfg` | B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/settings/media/remote/lmediarefreshlist` | B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/dns-info` | B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/ncsilinkup` | B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/ncsiselect` | B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/network/activelancfg` | B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/network/availablelancfg` | B650-7.02, B650-9.05 | Settings: network |
| `/api/asrr/settings/passwordcfg` | B650-7.02, B650-9.05 | Settings: users & PAM |
| `/api/settings/ipmi_disable_interfaces` | B650-7.02, B650-9.05 | Settings: services & sessions |
| `/api/settings/getSnmpv2` | B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/lan_destination/snmptrap_user` | B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/setSnmpv2` | B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/snmp/mibfile` | B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/fan/assignment` | B650-7.02, B650-9.05 | Settings: fan control |
| `/api/settings/fan/closedloop` | B650-7.02, B650-9.05 | Settings: fan control |
| `/api/settings/fan/load_default` | B650-7.02, B650-9.05 | Settings: fan control |
| `/api/settings/fan/manual` | B650-7.02, B650-9.05 | Settings: fan control |
| `/api/settings/fan/mode` | B650-7.02, B650-9.05 | Settings: fan control |
| `/api/settings/fan/names` | B650-7.02, B650-9.05 | Settings: fan control |
| `/api/settings/fan/openloop` | B650-7.02, B650-9.05 | Settings: fan control |
| `/api/settings/fan/temp_sensors` | B650-7.02, B650-9.05 | Settings: fan control |
| `/api/settings/ocprofile` | B650-7.02, B650-9.05 | Settings: BIOS / platform OEM |
| `/api/settings/telemetrycups` | B650-7.02, B650-9.05 | Settings: BIOS / platform OEM |
| `/api/asrr/maintenance/BP/configuration` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/BP/firmware` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/BP/flash-progress` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/BP/status` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/BP/upgrade` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/CPLD/configuration` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/CPLD/firmware` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/CPLD/flash-progress` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/CPLD/status` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/CPLD/upgrade` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/FPGA/configuration` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/FPGA/firmware` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/FPGA/flash-progress` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/FPGA/status` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/FPGA/upgrade` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/PSU/configuration` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/PSU/firmware` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/PSU/flash-progress` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/PSU/status` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/PSU/upgrade` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/RETIMER/configuration` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/RETIMER/firmware` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/RETIMER/flash-progress` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/RETIMER/status` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/RETIMER/upgrade` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/fwupdate` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/cpld/enableupdatemode` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/cpld/getinfo` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/dualflashimageconfig` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/firmware/VerifyImageFlash` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/firmware/firmware` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/flashtrack` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/getPlatformID` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/freemem` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/getbiosversions` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/memory_resilience_technology_information` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nvme/activatecomponents` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nvme/fwupdate` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nvme/fwupdateprogress` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nvme/nvmemissdsfw` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/activatecomponents` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/exitupdatemode` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/flash` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/flashimagestatus` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/fwupdateoptions` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/pfrfw` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/pfrfwimagesize` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/pfrgetreadystatus` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/pfrversioninfo` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/preparecomponents` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/seamlessflashstatus` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/updatemode` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/verifyimage` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pfr/verifyimagestatus` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pldm` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pldm/fwupdateprogress` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pldm/fwupdatestatus` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/pldm/getpldmfwinfo` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/raid_management/flash` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/raid_management/preparecomponents` | B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/raid_management/upgradestatus` | B650-7.02, B650-9.05 | Maintenance & firmware update |

#### Present on both families (179)

| Path | Boards | Area |
|---|---|---|
| `/api/reset-pass` | X470-3.02, B650-7.02, B650-9.05 | Session & authentication |
| `/api/session` | X470-3.02, B650-7.02, B650-9.05 | Session & authentication |
| `/api/viewerlogin` | X470-3.02, B650-7.02, B650-9.05 | Session & authentication |
| `/api/configuration/project` | X470-3.02, B650-7.02, B650-9.05 | Feature discovery & UI configuration |
| `/api/configuration/runtime` | X470-3.02, B650-7.02, B650-9.05 | Feature discovery & UI configuration |
| `/api/asrr/fw-info` | X470-3.02, B650-7.02, B650-9.05 | Dashboard & identity |
| `/api/asrr/model-name` | X470-3.02, B650-7.02, B650-9.05 | Dashboard & identity |
| `/api/firmware-info` | X470-3.02, B650-7.02, B650-9.05 | Dashboard & identity |
| `/api/logs/dashboardevent` | X470-3.02, B650-7.02, B650-9.05 | Dashboard & identity |
| `/api/actions/chassis-led` | X470-3.02, B650-7.02, B650-9.05 | Power, chassis & UID |
| `/api/actions/power` | X470-3.02, B650-7.02, B650-9.05 | Power, chassis & UID |
| `/api/asrr/PowerSource` | X470-3.02, B650-7.02, B650-9.05 | Power, chassis & UID |
| `/api/asrr/bmccoldreset` | X470-3.02, B650-7.02, B650-9.05 | Power, chassis & UID |
| `/api/chassis-status` | X470-3.02, B650-7.02, B650-9.05 | Power, chassis & UID |
| `/api/detail_sensors_readings` | X470-3.02, B650-7.02, B650-9.05 | Sensors & SDR |
| `/api/sdr` | X470-3.02, B650-7.02, B650-9.05 | Sensors & SDR |
| `/api/sensors` | X470-3.02, B650-7.02, B650-9.05 | Sensors & SDR |
| `/api/settings/sensor-threshold` | X470-3.02, B650-7.02, B650-9.05 | Sensors & SDR |
| `/api/fru` | X470-3.02, B650-7.02, B650-9.05 | FRU & PLDM inventory |
| `/api/asrr/clearmcalog` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/asrr/getmcalog` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/asrr/mca-file` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/event` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/event-file` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/eventlog` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/help-data` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/raw-event-file` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/video` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/video-data` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/logs/video-log/delete` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/settings/log` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/settings/log-policy` | X470-3.02, B650-7.02, B650-9.05 | Logs, crash dumps & diagnostics |
| `/api/host_inventory/host_interface_baseboard_info` | X470-3.02, B650-7.02, B650-9.05 | Host inventory |
| `/api/host_inventory/host_interface_memory_info` | X470-3.02, B650-7.02, B650-9.05 | Host inventory |
| `/api/host_inventory/host_interface_pcie_device_function_info` | X470-3.02, B650-7.02, B650-9.05 | Host inventory |
| `/api/host_inventory/host_interface_power_info` | X470-3.02, B650-7.02, B650-9.05 | Host inventory |
| `/api/host_inventory/host_interface_processor_info` | X470-3.02, B650-7.02, B650-9.05 | Host inventory |
| `/api/host_inventory/host_interface_storage_info` | X470-3.02, B650-7.02, B650-9.05 | Host inventory |
| `/api/host_inventory/host_interface_system_info` | X470-3.02, B650-7.02, B650-9.05 | Host inventory |
| `/api/host_inventory/host_interface_thermal_info` | X470-3.02, B650-7.02, B650-9.05 | Host inventory |
| `/api/settings/raid_management/bbu_info` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/create_virtual_drive` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/delete_virtual_drive` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/logical_consistency_check_cmd` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/logical_device_info` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/logical_device_progress_cmd` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/logical_initialize_check_cmd` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/logical_start_stop_locate_cmd` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/physical_dedicated_hotspare_cmd` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/physical_device_info` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/physical_hotspare_cmd` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/physical_onlineoffline_cmd` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/physical_removal_cmd` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/physical_start_stop_locate_cmd` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/raid_controller_info` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/raid_event_log` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/raid_levels` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/raid_pd_count` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/raid_management/storage_summary` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/sasit_management/sasit_controller_info` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/sasit_management/sasit_physical_device_info` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/settings/sasit_management/sasit_topology_info` | X470-3.02, B650-7.02, B650-9.05 | Storage / PCIe device management |
| `/api/kvm/token` | X470-3.02, B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/settings/media/adviser` | X470-3.02, B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/settings/media/remotesession` | X470-3.02, B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/settings/mouse` | X470-3.02, B650-7.02, B650-9.05 | KVM, H5Viewer & SOL |
| `/api/settings/media/general` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/instance` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/local/configurations` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/local/file` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/local/image` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/local/images` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/local/lmediasizecheck` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/local/start-media` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/local/stop-media` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/remote/configurations` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/remote/images` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/remote/rmediarefreshlist` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/remote/start-media` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/settings/media/remote/stop-media` | X470-3.02, B650-7.02, B650-9.05 | Virtual media / remote storage |
| `/api/set/dns/registration` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/channels` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/dns/domain-options` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/dns/registration` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/dns/restart` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/dns/server-options` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/dns/tsig-private-key` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/ncsi-interfaces` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/ncsi/mode` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/network` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/network-bond` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/network-link` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/packages` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/static-ipv6` | X470-3.02, B650-7.02, B650-9.05 | Settings: network |
| `/api/settings/pam-order` | X470-3.02, B650-7.02, B650-9.05 | Settings: users & PAM |
| `/api/settings/user/ssh-key-upload/:id` | X470-3.02, B650-7.02, B650-9.05 | Settings: users & PAM |
| `/api/settings/users` | X470-3.02, B650-7.02, B650-9.05 | Settings: users & PAM |
| `/api/settings/users/:id` | X470-3.02, B650-7.02, B650-9.05 | Settings: users & PAM |
| `/api/settings/users_read_only` | X470-3.02, B650-7.02, B650-9.05 | Settings: users & PAM |
| `/api/settings/service-sessions` | X470-3.02, B650-7.02, B650-9.05 | Settings: services & sessions |
| `/api/settings/services` | X470-3.02, B650-7.02, B650-9.05 | Settings: services & sessions |
| `/api/operations/ssl/certificate/generate` | X470-3.02, B650-7.02, B650-9.05 | Settings: SSL |
| `/api/settings/ssl/certificate` | X470-3.02, B650-7.02, B650-9.05 | Settings: SSL |
| `/api/settings/ssl/certificate-info` | X470-3.02, B650-7.02, B650-9.05 | Settings: SSL |
| `/api/settings/active-directory-settings` | X470-3.02, B650-7.02, B650-9.05 | Settings: LDAP / AD / RADIUS |
| `/api/settings/active-directory-users` | X470-3.02, B650-7.02, B650-9.05 | Settings: LDAP / AD / RADIUS |
| `/api/settings/ldap-certificates` | X470-3.02, B650-7.02, B650-9.05 | Settings: LDAP / AD / RADIUS |
| `/api/settings/ldap-settings` | X470-3.02, B650-7.02, B650-9.05 | Settings: LDAP / AD / RADIUS |
| `/api/settings/ldap-users` | X470-3.02, B650-7.02, B650-9.05 | Settings: LDAP / AD / RADIUS |
| `/api/settings/radius/advanced` | X470-3.02, B650-7.02, B650-9.05 | Settings: LDAP / AD / RADIUS |
| `/api/settings/radius/general` | X470-3.02, B650-7.02, B650-9.05 | Settings: LDAP / AD / RADIUS |
| `/api/settings/date-time` | X470-3.02, B650-7.02, B650-9.05 | Settings: date & time / NTP |
| `/api/settings/pef/alert_policies` | X470-3.02, B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/pef/alert_string_keys` | X470-3.02, B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/pef/destinations` | X470-3.02, B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/pef/event_filters` | X470-3.02, B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/pef/lan_destinations` | X470-3.02, B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/pef/send_test_alert` | X470-3.02, B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/smtp` | X470-3.02, B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/smtp/smtpupload` | X470-3.02, B650-7.02, B650-9.05 | Settings: SMTP, PEF, alerts & SNMP |
| `/api/settings/firewall` | X470-3.02, B650-7.02, B650-9.05 | Settings: firewall |
| `/api/settings/firewall-ip-rules` | X470-3.02, B650-7.02, B650-9.05 | Settings: firewall |
| `/api/settings/firewall-port-rules` | X470-3.02, B650-7.02, B650-9.05 | Settings: firewall |
| `/api/settings/firewall/:id` | X470-3.02, B650-7.02, B650-9.05 | Settings: firewall |
| `/api/settings/download_image` | X470-3.02, B650-7.02, B650-9.05 | Settings: video recording / BSOD / SOL recording |
| `/api/settings/sol-recorded-video` | X470-3.02, B650-7.02, B650-9.05 | Settings: video recording / BSOD / SOL recording |
| `/api/settings/video/pre-event` | X470-3.02, B650-7.02, B650-9.05 | Settings: video recording / BSOD / SOL recording |
| `/api/settings/video/remote-storage` | X470-3.02, B650-7.02, B650-9.05 | Settings: video recording / BSOD / SOL recording |
| `/api/settings/video/sol-remote-storage` | X470-3.02, B650-7.02, B650-9.05 | Settings: video recording / BSOD / SOL recording |
| `/api/settings/video/sol-triggers` | X470-3.02, B650-7.02, B650-9.05 | Settings: video recording / BSOD / SOL recording |
| `/api/settings/video/triggers` | X470-3.02, B650-7.02, B650-9.05 | Settings: video recording / BSOD / SOL recording |
| `/api/settings/license` | X470-3.02, B650-7.02, B650-9.05 | Settings: licensing |
| `/api/settings/licenses` | X470-3.02, B650-7.02, B650-9.05 | Settings: licensing |
| `/api/asrr/getbioscode` | X470-3.02, B650-7.02, B650-9.05 | Settings: BIOS / platform OEM |
| `/api/asrr/maintenance/BIOS/configuration` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/BIOS/firmware` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/BIOS/flash-progress` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/BIOS/status` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/asrr/maintenance/BIOS/upgrade` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/ValidSignImage` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/backup_config` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/bmc_recovery` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/download_config` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/dual_image_config` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/firmware` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/firmware/dwldfwimg` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/firmware/dwldfwstatus-progress` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/firmware/flash-progress` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/firmware/upgrade` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/firmware/verification` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/flash` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/fwimage_location` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/activatecomponents` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/componentversions` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/exitupdatemode` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/flash` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/preparecomponents` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/updatemode` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/upgradestatus` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/verifyimage` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/hpm/verifyimagestatus` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nm_activate` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nm_compareversion` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nm_deactivate` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nm_ecfstatus` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nm_eraseflash` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nm_firmware_update` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/nm_upload` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/preserve` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/reset` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/restore_config` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/restore_defaults` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/signed-image-key` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/system_admin` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/maintenance/upload_restore` | X470-3.02, B650-7.02, B650-9.05 | Maintenance & firmware update |
| `/api/ae_test` | X470-3.02, B650-7.02, B650-9.05 | Automation engine (scripts/tasks) |
| `/api/scripts` | X470-3.02, B650-7.02, B650-9.05 | Automation engine (scripts/tasks) |
| `/api/tasks` | X470-3.02, B650-7.02, B650-9.05 | Automation engine (scripts/tasks) |

---

## 9. Proxy implementation notes (Go)

1. **Session pool per BMC.** Log in with a form-encoded `POST /api/session`, keep the `QSESSIONID` cookie and `CSRFToken`, and send `X-CSRFTOKEN` on every call. Log out with `DELETE /api/session` on shutdown. The BMC has a small session limit (code 15000), so leaked sessions lock the UI out until `time_out` expires.
2. **Keepalive.** There is no keepalive endpoint. Poll `GET /api/chassis-status` below the web service `time_out` (`GET /api/settings/services`, default 1800 s). On a 401, re-login once and retry.
3. **Body fidelity.** Use an empty body with a JSON content type for `PUT /api/maintenance/flash` (X470), `POST /api/maintenance/reset` and `PUT /api/asrr/bmccoldreset`. Send a raw JSON string without forcing a content type for the ASRR `configuration`/`upgrade` calls. Use multipart with the exact field names `fwimage`, `publicKey`, `publickey`, `lmedia_image` and `config`. Singleton `PUT`s should send the full object as previously read (read-modify-write).
4. **Board family detection.** `GET /api/asrr/fw-info` / `GET /api/asrr/model-name` (both), `/api/chassis` (X470 only) and `GET /api/configuration/project`. Choose the firmware upload URL (`/firmware` versus `/firmware/firmware`) by family.
5. **Do not trust B650 UI presence.** 148 paths the B650 9.05 UI references (162 on 7.02) have no handler. Use the handler column, not the UI, to decide what the proxy exposes.
6. **Long operations.** BMC flash: the BMC goes away for about 2 minutes after `maintenance/reset`, and the session is invalidated. BIOS flash: poll `asrr/maintenance/BIOS/status` and `flash-progress`. The BMC session survives.
7. **KVM.** Proxying KVM means proxying the `/kvm` WebSocket (subprotocol `binary`) together with the cookie, and getting the token from `/api/kvm/token` (X470) or `/api/settings/media/h5viewercfg` (B650) on the same session.
