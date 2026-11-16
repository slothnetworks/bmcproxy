# Gigabyte MC13-LE0 BMC: live probe

This is a strictly **read-only** live probe of one Gigabyte **MC13-LE0-000** BMC (credentials in `mc13-le0-000.env`,
loaded into the environment only) on 2026-11-16. It was done to build the `gigabyte` driver (`vendors/gigabyte`).
Companion files:

- `docs/gigabyte/mc13-le0-redfish-inventory.json`: the sanitized authenticated Redfish crawl (225 resources)
- `docs/gigabyte/mc13-le0-har-endpoints.json`: the sanitized request/response captures (253 endpoints, Redfish and `/api`)
- `docs/firmware/mc13-le0.md`: static analysis of the 13.06.30 image (handler inventory, deny list, async tasks)
- `vendors/gigabyte/testdata/mc13le0-13.04.15.json`: the test fixture generated from the same captures
- For contrast: `docs/x470/live-probe.md` (**[X470]**, ASRock 2.32.00), `docs/b650/live-probe.md` (**[B650]**, ASRock
  4.10.00), and `docs/gigabyte/mz32-ar0-live-probe.md` (**[MZ32]**, written by the research agent)

**Redaction.** MACs are mapped to `02:00:00:00:00:NN`. `10.x.y.z` is mapped to `192.0.2.z`. UUIDs, session ids, serials,
the SNMP EngineId, CSRF tokens and `racsession_id` are faked or redacted. The MAC-derived hostname becomes
`AMI020000000001`, the site domain becomes `example.com`, and non-admin user names become `user4..6`. No file contains
the password, an X-Auth-Token, a CSRF token or a cookie value. A grep-based leak check (`leakcheck.sh` in the session
scratchpad) found zero hits for the password, the BMC address, serials and MACs.

---

## Live firmware (read this first)

> **Live BMC firmware is `13.04.15`** (Redfish `Managers/Self.FirmwareVersion`; `/api/firmware-info` `fw_ver
> "13.04.15"`, `date "Mar 6 2023"`, `active_image 1`). The image analysed statically is **13.06.30** (Sep 2026), so it
> is newer than the live unit. The driver's MC13-LE0 profile is keyed off the live version (`13.04.15` = verified).
> Other versions get the same quirks, are flagged `degraded: not verified`, and the capability probing still applies.

| Item | Value |
|---|---|
| Board | `Systems/Self.Manufacturer "Giga Computing"`, `Model "MC13-LE0-000"`, Manager `Model "421310600"` |
| BMC | AST2600, AMI MegaRAC SP-X 13.x, ServiceRoot `v1_13_0`, `RedfishVersion 1.11.0`, `Oem.Ami.RtpVersion 13.04` |
| IPMI identity | `mfg_id_0..2` = 10, 60, 0 → IANA **15370** (Giga-Byte), `prod_id 4213` |
| BIOS | F15 |
| Host | Ryzen 7 7700X, 128 GiB DDR5 (4 DIMMs), 1 NVMe, powered **on** for the whole probe |
| BMC clock | UTC, NTP **disabled** (`pool.ntp.org`, `time.nist.gov` configured); about **4 h behind** the probe host (offset −14408 s) |

## Summary verdict

- **Primary southbound: Redfish with Basic auth.** The admin account works on Redfish Basic and sessions, and on `/api`
  (unified `libnss_redfish` account store). Every generic capability except bond, firmware update and KVM is served
  from standard DMTF resources.
- **AMI `/api`** is needed only for the fan-profile **mode** (and `firmware-info` active image, plus raw/api). The
  driver logs in lazily and logs out on Close.
- **No IPMI path** is needed.
- **Quirks the driver encodes** (`vendors/gigabyte/boards/mc13le0.go`):
  1. **Collections are truncated at 50 members without `Members@odata.nextLink`.** `Members@odata.count` equals the page
     length, and `$skip=0` is a 400. The SEL has 150 entries (ids 168–317) and the AuditLog 68. Fixed with the new
     `rf.Quirks.SkipPageSize`: when a page holds exactly 50 and has no nextLink, the client asks for `?$skip=50,100,…`.
  2. **`Chassis/Self/Sensors` only lists Energy and Power.** Temperatures (9), fans (6) and voltages (8) are only in
     `Thermal`/`Power`. Fixed in the generic driver, which merges the legacy sensors when the Sensors collection has no
     temperature or fan.
  3. **SEL `Created` is the import time**, identical for every entry (`2025-09-17T16:54:49`). `EventTimestamp` carries
     bogus offsets (`+00:01..+00:07`). The real time is in `Message` (`"Timestamp : 2025-03-25T16:07:47+00:00"`). The
     gigabyte `Events` override takes the time from Message and keeps `oem.created`.
  4. The real baseboard serial is in `Chassis/Self.Oem.GBTChassisOemProperty."Board Serial Number"` (the FRU board serial
     is a `0123456789` placeholder). FRU and Baseboard are overridden to use it.
  5. `$expand` is advertised, but members come back as bare links. Log entry collections inline full `LogEntry` bodies
     anyway, so `rf.Members` now uses whole inlined members without `$expand`.
- **ASRock driver over-claims this board:** see §7. Report only. The gigabyte driver avoids it by priority.

## 1. TLS and ports

| Item | Value |
|---|---|
| Protocols | TLS 1.2 and 1.3 only (1.0/1.1 rejected) |
| TLS 1.2 suites | ECDHE-RSA-AES256-GCM-SHA384, ECDHE-RSA-AES128-GCM-SHA256, ECDHE-RSA-CHACHA20-POLY1305. No CBC, no static RSA kx, no DHE |
| Certificate | AMI default: CN=ami.com, issuer megarac.com, RSA 2048, 2022-01-01 → 2031-12-30, SAN megarac.com / 169.254.0.17 |
| Go client | default config, TLS 1.3 `TLS_AES_256_GCM_SHA384` (profile "modern") |
| Web root | lighttpd, HSTS; Redfish `Server: AMI MegaRAC Redfish Service` |

## 2. Unauthenticated surface

| Request | Result |
|---|---|
| `GET /redfish/v1/` | 200. ServiceRoot `v1_13_0`, Vendor `AMI`, Product `AMI Redfish Server`, `Oem.Ami.RtpVersion 13.04`. **No Gigabyte marker** |
| `GET /redfish/v1/$metadata` | 200, 115,551 bytes, declares the `GBT*`/`Gbt*` namespaces (GBTChassisOemProperty, GBTFanprofile*, GBTThermal, GbtPowerLimit, …). **This is the detection signal** |
| `GET /redfish/v1/GlobalLuaTables` | **200 unauthenticated** (194 bytes; body not recorded). Same leak as [X470]/[B650]; denied in raw/redfish |
| `GET /api/chassis-status` | 401 `{cc:7}` |
| `GET /api/asrr/model-name` | 404 `{"error":"Invalid API Call","code":1010}`: no ASRock namespace |

`ProtocolFeaturesSupported`: ExpandQuery (All/Levels/Links/NoLinks, MaxLevels 5), Filter, Select, OnlyMember, Excerpt,
DeepPATCH. TopSkipQuery is **not** advertised, but `$top`/`$skip` work.

## 3. Redfish authentication

| Test | Result |
|---|---|
| Basic, admin | 200 across the tree; creates no session |
| Basic, wrong password | 401 `Base.1.12.AccessDenied`. Lockout threshold 5, duration 30 |
| `POST SessionService/Sessions` | 201 + `X-Auth-Token` + `Location`. Token GET 200. `DELETE` 204. Token afterwards: 401 |
| `SessionTimeout` | 30 (seconds), so the driver uses Basic |

## 4. Redfish conformance and quirks

| Area | Finding |
|---|---|
| Paging | **Truncated at 50, no nextLink, count = page length.** `$skip=50` works, and `$skip=0` → 400 `QueryParameterOutOfRange`. Handled by `SkipPageSize: 50` |
| `$select` | works on a resource; on a collection it returns **0 members** (never used on collections) |
| `$expand` | bare links (profile sets `NoExpand`) |
| `$filter` | 400 on a resource |
| Unknown query param | 200, ignored |
| ETags | service-wide `W/"1790857023"` with per-group etags. `If-None-Match` never gives 304 |
| Systems/Self | IndicatorLED `Off` (Lit/Blinking/Off), PowerRestorePolicy `AlwaysOn`, Boot override `Disabled`/`Legacy`/`None`, `@Redfish.Settings` → SD |
| ResetActionInfo | System and Chassis: ForceOff, ForceRestart, GracefulShutdown, On, PowerCycle. Manager: ForceRestart |
| Manager | KVMIP max 2, IPMI max 36. Oem.Ami actions: RedfishDBReset, PowerSaveMode, VirtualMedia ones. VirtualMedia CD1 |
| Network | `bond0` holds the static IP. `eth0`/`eth1` have MACs only. `usb0` is the host interface |
| Accounts | admin (Administrator) + 3 others |
| FirmwareInventory | BIOS F15, BMC 13.04.15, MB_CPLD1 `ff` |
| TaskService | empty |
| `Systems/Self/Bios` | **400** `ResourceAtUriInUnknownFormat` |
| Gigabyte OEM | `Thermal.Oem.Gbt.FanprofileService` → `Fanprofile` (Allow GET,PUT: `strMode`, `strVersion`, `arrProfile` = `default` with 4 policies and `NEW_PROFILE` with 1) and `SupportPCIEDevice` (GET,PUT; 88 devices). `FanMode`, `PoweronDelay`, `FrontPanelSettings`, the Gct Firewall and `RetimerInfo` return **404**. `Power.PowerControl.Oem.GbtPowerLimit` |

## 5. Legacy `/api`

All `/api` paths were checked against the 13.06.30 handler before calling (plain reads only).

| Request | Result |
|---|---|
| `POST /api/session` (form username/password) | 200 in about 0.55 s: `ok 0`, `privilege 4`, `extendedpriv 259`, `racsession_id`, `remote_addr`, `server_name`, `server_addr`, `HTTPSEnabled`, `CSRFToken`, `channel`, `passwordStatus`. Cookie `QSESSIONID; path=/; secure;HttpOnly;Samesite=Lax` |
| Reads (0.07–0.26 s each) | `chassis-status` `{power_status 1, led_status 0, power_cycle_interval 5000}`, `configuration/project`, `firmware-info`, `fru`, `settings/date-time` (AMI shape), `settings/fanprofile`, `/collection`, `/mode` `{"strMode":"NEW_PROFILE"}`, `settings/service-sessions`, `settings/services` (web/kvm/... on bond0) |
| GET with a bogus or missing `X-CSRFTOKEN` | **401 `cc:7`, and the session is invalidated** (the logout that followed got 401). This is the opposite of ASRock. I checked afterwards with a second login and a session listing: nothing leaked |
| `DELETE /api/session` | 200 |

## 6. Comparison

| Axis | ASRock X470D4U 2.32.00 | ASRock B650D4U 4.10.00 | **Gigabyte MC13-LE0 13.04.15** |
|---|---|---|---|
| SoC / AMI gen | AST2500 / SP-X 3.x | AST2600 / SP-X 13.x | **AST2600 / SP-X 13.x** |
| Image = live? | no (3.02 image) | no (7.02/9.05 images) | **no** (13.06.30 image) |
| Redfish admin auth | rejected (separate store) | Basic OK | **Basic OK** (unified store) |
| RedfishVersion / RtpVersion | 1.5.0 / 1.5.b | 1.15.1 / 13.05 | **1.11.0 / 13.04** |
| Message registry | Base.1.0 | Base.1.12 | **Base.1.12** |
| Sensors | n/a | 60 DMTF `Sensors` | **`Sensors` = Energy + Power only; `Thermal`/`Power`** |
| Paging | n/a | 50, nextLink, terminates | **50, no nextLink, count = page: truncated** |
| `$expand` | advertised | honoured | **bare links** |
| SEL time | n/a | Created OK | **Created = import time; time in Message** |
| ETags | single, service-wide | 48 distinct, epoch | **service-wide + per group, no 304** |
| SessionTimeout | n/a | 30 s | **30 s** |
| ResetType | n/a | full set | **On, ForceOff, GracefulShutdown, ForceRestart, PowerCycle** |
| Unauth leak | `GlobalLuaTables` | `GlobalLuaTables` | **`GlobalLuaTables`** |
| TLS | TLS 1.0–1.2, RSA kx, expired cert | TLS 1.2 + 1.3 | **TLS 1.2 + 1.3, ECDHE/AEAD only, AMI cert to 2031** |
| `/api` cookie | bare `path=/` | Secure;HttpOnly;SameSite=Lax | **secure;HttpOnly;Samesite=Lax** |
| `/api` CSRF on GET | not enforced | not enforced | **enforced; a bad token kills the session** |
| `asrr/*` namespace | yes | yes | **no (404 code 1010)** |
| Primary southbound | `/api` + IPMI | Redfish + `/api` | **Redfish (Basic); `/api` lazy for fan mode** |

## 7. bmcprobe with the drivers as they were (before the gigabyte driver)

`bmcprobe` was built into the scratchpad from the drivers before this change (asrock, generic redfish).

| Run | Result |
|---|---|
| detect, unauthenticated | `redfish` (score 10), manufacturer AMI |
| detect, with credentials | **`asrock` (score 30)**: `detail.ami "logged in; no asrr namespace"`. The ASRock session then reported AMI "not authenticated (no asrr namespace (board unknown))", `degraded: unknown board/firmware`, and **FRU 501** |
| caps, `--vendor redfish` | everything supported except bond, firmwareUpdate and kvm |
| get, `--vendor redfish` | sensors: only 2 (energy unit "W"). events: 50 of 150 (the oldest 50 only), all with the same timestamp. audit: 50 of 68. FRU board serial is a placeholder |

**Bug (report only, ASRock is owned by another agent):** `vendors/asrock` `DetectWithCreds` logs in to any MegaRAC
`/api` and returns score 30 ("logged in; no asrr namespace") even when the `asrr` namespace is absent. So it claims
non-ASRock MegaRAC boards. The gigabyte driver runs at priority 10 and returns ScoreCertain first, so `bmc.Resolve`
never reaches the ASRock probe on Gigabyte boards (covered by `TestResolvePrefersGigabyte`). The ASRock driver should
probably return at most a fallback score when `asrr` is missing.

Fixed in shared code (backward compatible, with tests):

| Problem | Fix |
|---|---|
| 50-of-150 events / 50-of-68 audit | `rf.Quirks.SkipPageSize` + `WalkCollection` `$skip` continuation (stops on a repeated first member) |
| 2 sensors | generic `Sensors` merges legacy Thermal/Power when the collection has no temperature or fan |
| energy unit "W" | generic `Unit()` maps EnergyJoules → `J`, EnergykWh → `kWh`, EnergyWh → `Wh` |
| 150 per-entry GETs | `rf.Members` uses whole inlined members (`@odata.type` + `Id`) without `$expand` |
| SEL times / board serial | gigabyte driver overrides (`Events`, `FRU`, `Baseboard`) |

## 8. Final live check through the gigabyte driver

`go run -tags gigabytelive ./vendors/gigabyte/internal/probe` (built into the scratchpad) with the env file loaded in
the same shell. Getters only, one request at a time. Output sanitized (address mapped):

```
Detect (unauth)              1070ms ok score=60 signal="$metadata GBT OEM namespaces" stack="AMI MegaRAC"
Resolve (creds)              1590ms ok driver=gigabyte score=100 board=MC13-LE0-000 firmware=13.04.15 profile=mc13le0 verified=true
Open                         3048ms ok
info: driver=gigabyte profile=mc13le0 degraded="" caps=auditLog,bmcReset,bootOverride,eventLog,eventLogClear,firmwareInventory,fru,identity,indicatorLed,inventory,network,power,powerRestorePolicy,sensors,services,time,users,virtualMedia
SystemInfo                    705ms ok manufacturer="Giga Computing" board="MC13-LE0-000" bmc=13.04.15 bios=F15 power=on health=ok
PowerState                    156ms ok on
RestorePolicy                 397ms ok alwaysOn
IndicatorLED                  209ms ok off
BootOverride                  156ms ok {Target:none Mode:disabled BootMode:legacy}
Sensors                       840ms ok n=26 types=map[fan:6 power:3 temperature:9 voltage:8] units=map[C:9 J:1 RPM:6 V:8 W:2]
Events                       1802ms ok n=150 ids=168..317 first=2025-03-25T16:07:47Z last=2026-10-01T12:17:04Z distinctTimes=124
AuditLog                      919ms ok n=68
FRU                           824ms ok n=1 boardSerialSet=true boardSerialPlaceholder=false product="MC13-LE0-000"
CPUs                          379ms ok n=1
Memory                        603ms ok n=4
PCIe                         1067ms ok n=5
Storage                       470ms ok n=1
Baseboard                     578ms ok manufacturer="Giga Computing" product="MC13-LE0-000" serialSet=true
Network                       617ms ok n=4 names=[bond0 eth0 eth1 usb0]
Users                         847ms ok n=4
Services                      294ms ok n=8
Time                          301ms ok offset=-14408s ntp=false tz="+00:00"
Firmware                      420ms ok BIOS=F15 BMC=13.04.15 MB_CPLD1=ff
VirtualMedia                  276ms ok n=1
vendor fanProfiles            772ms ok mode=NEW_PROFILE profiles=[default(4) NEW_PROFILE(1)]
vendor fanProfileDevices      674ms ok n=88
vendor fanMode                387ms ok mode=NEW_PROFILE source=ami
backend: redfish available=true authenticated=true detail="basic"
backend: ami available=true authenticated=true detail="logged in"
Close                         208ms ok
```

Every issue from §7 is resolved on the live unit: 26 sensors, all 150 SEL entries with 124 distinct real times,
68 audit entries, the real board serial, and energy in J.

## 9. Not verified (needs a write, with approval)

- `setFanMode` (POST `settings/fanprofile/mode {"strMode": …}`): the shape comes from the 13.06.30 UI JS and was never
  exercised.
- LED, power, boot override, restore policy and time/NTP writes on this board. The NetworkProtocol 202 task path is
  assumed from the firmware (`patch-networkprotcol.lua`) and covered by fake tests only.
- KVM (501, as for every driver), firmware update (501), BMC reset (not called).
- Firmware versions other than 13.04.15 (13.06.30 is newer than the live unit).

## Method and hygiene

- The password was loaded with `set -a; . ./mc13-le0-000.env; set +a` in the same shell command and passed through the
  environment (`--password-env BMC_PASS`, or read by the probe program). It was never printed, logged or written to
  any file, and no other `*.env` file was read.
- URLs were built in Python or Go, never in the shell. Requests were sequential, one at a time (`probe/rp.py` refuses
  any method other than GET, apart from `/api/session` and Redfish session POST/DELETE).
- Writes: **none.** The only non-GET requests were the `/api` login/logout and the Redfish session create/delete test.
  Every session was logged out (one `/api` session was invalidated by the bogus-CSRF test and confirmed gone).
- The deny-listed families (help-data, mca/crash files, downloads, hpm status, firmware/flash/restore/reset/factory,
  host diagnostics) were never called. `ResetActionInfo` GETs (ActionInfo reads) were explicitly allowed past the
  probe's own "reset" substring filter.

## Addendum: settings/network-bond (2026-11-16, read-only)

One read of the AMI bonding object, to pin the shape the gigabyte `BondManager` (generic `GET /api/v1/network/bond`) and
the vendor `bonding` routes parse. Program: `vendors/gigabyte/internal/probe/bond` (build tag `gigabytelive`).

| Step | Time | Result |
| --- | --- | --- |
| `POST /api/session` (login) | 709 ms | ok, CSRF token taken from the login body and sent on the GET |
| `GET /api/settings/network-bond` | 69 ms | 200, 111 bytes |
| `DELETE /api/session` (logout) | 211 ms | ok (no retry needed) |

Response (verbatim; the object holds no addresses, MACs or serials):

```json
{"id": 1, "bond_enable": 1, "bond_mode": "active-backup", "bond_ifc": "both", "auto_configuration_enable": 1}
```

All five values are JSON numbers or strings as shown (`id`, `bond_enable`, `auto_configuration_enable` numbers;
`bond_mode`, `bond_ifc` strings). The board is bonded in failover (`bond_ifc` "both", the web UI's "Failover";
"eth0" is "Shared", "eth1" "Dedicated"), which matches `bond0` holding the BMC address in `settings/network`.
The object carries an `id`, so the web UI's Backbone `model.save` is a `PUT` of the whole fetched object.

Mapping to `bmc.BondConfig`: `id`→`ID`, `bond_enable != 0`→`Enabled`, `bond_mode`→`Mode` (verbatim Linux mode
name, read-only on Gigabyte), `bond_ifc`→`Interfaces[0]`, `auto_configuration_enable != 0`→`AutoConfig`, the
whole object→`Oem`. Fixture: `vendors/gigabyte/testdata/network-bond-mc13le0-13.04.15.json`.

Not verified: the `PUT` (it restarts BMC networking and was not sent), its response body, and whether the firmware
accepts `bond_enable 0` with `bond_ifc "both"` (the driver refuses that combination).

Hygiene: credentials loaded with `set -a; . ./mc13-le0-000.env; set +a` in the same shell command and read from the
environment by the program; the password was never printed, logged or written. Exactly three requests, one at a time:
login, the GET, logout. **No writes.**
