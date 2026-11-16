# Gigabyte MZ32-AR0 BMC: live probe

Status: preliminary assessment for the native Go management proxy. This is a strictly **read-only** live probe of
one Gigabyte **MZ32-AR0-00** BMC (`10.0.5.38`, credentials in `MZ32-AR0-00.env`) on 2026-11-16. The aim was to decide
whether this board is "plain Redfish" for the Gigabyte driver, which is being built from MC13-LE0. Companion files:

- `docs/gigabyte/mz32-ar0-redfish-inventory.json`: the sanitized authenticated Redfish crawl (561 resources, with bodies)
- `docs/gigabyte/mz32-ar0-frontend.md`: the web UI / legacy `/api` analysis and route list
- `docs/firmware/mz32-ar0.md`: static analysis of `bmc-firmware/MZ32-AR0-00.bin` (handler inventory, **deny list**, MC13 diff)
- For contrast: `docs/b650/live-probe.md` (**[B650]**, ASRock 4.10.00), `docs/x470/live-probe.md` (**[X470]**,
  ASRock 2.32.00), and `docs/gigabyte/mc13-le0-redfish-inventory.json` (**[MC13]**, the other Gigabyte board).

**Redaction.** MACs, UUIDs, serial/asset/part numbers, the MAC-derived hostname, session ids, IPv6 link-local and
non-lab addresses are replaced by `<REDACTED_*>`. In the JSON, `10.x.y.z` is mapped to `192.0.2.z`. This document
contains no password, X-Auth-Token, CSRF token or cookie value. Raw captures stayed in the session scratchpad, and
the ones that held session material were deleted.

---

## Live firmware (read this first)

> **The live unit runs BMC firmware `12.61.39`, the same build as the image we have** (`bmc-firmware/MZ32-AR0-00.bin`
> fwinfo `FW_VERSION=12.61.39`, `Jul 2 2025 11:14:05 CST`). The live web bundle is byte-identical to the image's
> `www` (same sha256 for `source.min.js`, `styles.min.css`, `index.html`, `viewer.html`). This is the first board in
> this repo where the static analysis and the live unit match exactly.

| Item | Value | Source |
|---|---|---|
| Board | **Gigabyte MZ32-AR0-00** (single-socket AMD EPYC SP3, rev 00) | Redfish `Systems/Self.Model "MZ32-AR0-00"`, `Manufacturer "GIGABYTE"`; `/api/system_inventory_gbt/bios_info`; FRU board area |
| BMC SoC | **ASPEED AST2500** | firmware FMH module `ast2500e`, PRJ |
| BMC stack | **AMI MegaRAC SP-X 12.x** ("WolfPass" reference project, AMI 12.2 base from 2019-12, rebuilt by Gigabyte 2025-07), Linux 3.14.17-ami | `docs/firmware/mz32-ar0.md` §1 |
| BMC firmware | **12.61.39**, built `Jul 2 2025 11:14:05 CST`, **active image 1** (dual image) | Redfish `Managers/Self.FirmwareVersion "12.61.39"`, `FirmwareInventory/BMC.Version "12.61.39"`; `/api/firmware-info` (`fw_rev_1 12`, `fw_rev_2 0x61`, `active_image 1`) |
| IPMI identity | manufacturer **IANA 15370 (0x3C0A, Giga-Byte)**, `prod_id 0x1008`, device id 32, rev 129, IPMI 2.0 | `/api/firmware-info` |
| Redfish | `RedfishVersion 1.8.0`, `ServiceRoot.v1_5_2`, `Product "AMI Redfish Server"`, `Vendor "AMI"`, `Oem.Ami.RtpVersion "1.8.0"`, `Oem.Vendor.VendorVersion "1.8.0.001"` | `GET /redfish/v1/` |
| BIOS | **M32** (release date 06/23/2026) | Redfish `Systems/Self.BiosVersion "M32"`, `FirmwareInventory/BIOS`; `/api/system_inventory_gbt/bios_info` |
| CPLD | Redfish `FirmwareInventory/MB_CPLD1.Version "ff"`, but `/api/system_inventory_gbt/cpld_info` `"12 "` (the two disagree) | both |
| Host | AMD EPYC 7B13 64-core (1 socket, 128 threads), 256 GiB, 2× NVMe (Samsung PM983 960 GB, Kioxia 960 GB), dual X540-AT2 10 GbE (onboard + offboard) | Redfish Systems/Self |
| Host power | **On** during the whole probe | `Systems/Self.PowerState "On"`, `/api/chassis-status power_status 1` |
| BMC clock | `2026-11-16T21:24:11+00:00` against the probe's HTTP `Date` of 21:23–21:24 GMT, so **no meaningful skew**. Timezone `Etc/GMT`. NTP servers configured (`pool.ntp.org`, `time.nist.gov`) but **NTP disabled** | `Managers/Self.DateTime`, `NetworkProtocol.NTP.ProtocolEnabled false`, `/api/settings/date-time ntp_auto_date 0` |

---

## Summary verdict

- **Yes, it's essentially plain AMI Redfish, and Redfish should be the primary southbound.** Basic auth with the
  admin account returns 200 across the whole tree. The crawl reached **561 resources (554× 200, 7× 404, 0× 5xx)**,
  and power state, reset (4 ResetTypes), boot override, LED, sensors (via `Thermal`/`Power`), SEL, inventory,
  firmware inventory, network protocol/NTP and accounts all come from standard DMTF resources.
- **But it's an older AMI Redfish (RtpVersion 1.8.0, Base.1.5 registry), and it has quirks a "generic Redfish" driver
  would trip over:**
  1. **The query parser does not percent-decode `%24`.** `$select`, `$expand`, `$top`, `$skip` and `$filter` work only
     with a **literal `$`**. Go's `url.Values.Encode()` produces `%24select=…`, which the server treats as an unknown
     non-`$` parameter and **silently ignores** (200, full body). The service's own nextLinks use a literal `$`
     (`?$skip=50`).
  2. **`$filter` is applied after paging**: the count reflects only the filtered first page, and the `nextLink` drops
     the filter.
  3. **`Chassis/Self/Sensors` has only `Energy` and `Power`** (Gigabyte override, both reading 0). Temperatures, fans
     and voltages are only in the legacy `Thermal` (12 temps, 6 fans) and `Power` (12 voltages, 4 PSU slots).
     There's no `ThermalSubsystem`/`PowerSubsystem`.
  4. **The SEL is split across three LogServices** (`Managers/Self/LogServices/SEL` 57, `Systems/Self/LogServices/BIOS`
     86, `Chassis/Self/LogServices/Logs` 4), and Redfish entries have only hex `Message`s (`"0x02FFFF"`,
     `AmiIpmiOem.1.0.GeneralEventData`). The legacy `/api/logs/event` returns **1,022** decoded SEL records.
  5. **`SessionTimeout` is 30 s**, so use Basic auth (it creates no session) as on [B650].
- **Plain Redfish is enough for every generic proxy capability except** a decoded/full event log, BMC uptime, the
  dual-image/active-image indicator, and Gigabyte fan-profile *writes* (`GBTFanprofile` is `GET,PUT` in Redfish, so
  possibly not even then). See the capability map in §8.
- **Security**: the admin account on this unit uses a **well-known vendor default password**, and Redfish reports
  `PasswordChangeRequired false` for it. The BMC also ships AMI's shared default TLS certificate, an image-
  wide uid-0 `sysadmin` shell account, and a Redfish Host Interface with **`AuthNone` → Administrator**. IPMI suite 0
  is advertised. Details in §9.

---

## 1. Ports

TCP connect scan (nmap `-sT`) from the probe host (172.16.0.0/12 → 10.0.5.0/24, i.e. through a router or firewall):

| Port | State | Notes |
|---|---|---|
| 80/tcp | open | lighttpd. **307** → `https://<host>:443/<path>` (the security headers are already on the 307) |
| 443/tcp | open | lighttpd: web UI, `/api`, `/redfish` (proxied to an internal :9080), KVM websocket `/kvm` (single-port mode) |
| 623/tcp | open | listener present (AMI). Not exercised |
| 623/udp | **reachable** | sessionless RMCP: ASF Presence Pong (IANA 4542, entities 0x81 = IPMI). Get Channel Auth Capabilities (v2): cc 0, channel 1, auth types `0x84` = **MD5 only** (+v2 bit), status `0x14` (non-null usernames, per-message auth off, no anonymous, **KG all-zero**), ext caps `0x03` (IPMI 1.5 + 2.0), OEM id 0. Cipher suites: **0, 1, 2, 3, 6, 7, 8, 11, 12, 15, 16, 17**, identical to [X470] |
| 22/tcp | filtered | SSH is **enabled** on the BMC (Redfish `NetworkProtocol.SSH.ProtocolEnabled true`, `ncml.conf [ssh]`). Filtered between the probe host and the BMC |
| 5900, 5901, 7578, 8443/tcp | filtered | KVM runs over 443 in single-port mode. 7578/7582 are the non-single-port KVM ports |

Redfish `NetworkProtocol` also shows SNMP 161 **enabled**, IPMI 623 enabled, SSDP 1900 disabled, KVMIP/VirtualMedia
on 443. UDP 161 wasn't probed.

## 2. TLS

| Item | Value |
|---|---|
| Protocols | **TLS 1.2 and TLS 1.3 only** (no SSLv3/TLS1.0/1.1) |
| TLS 1.2 ciphers (server preference) | `ECDHE-RSA-AES128-GCM-SHA256`, `ECDHE-RSA-AES256-GCM-SHA384` (secp384r1), `DHE-RSA-AES128/256-GCM` (dh 4096), `RSA-AES128/256-GCM` (static RSA, no FS) |
| TLS 1.3 ciphers | `AES_256_GCM_SHA384`, `CHACHA20_POLY1305_SHA256`, `AES_128_GCM_SHA256` (secp384r1) |
| Certificate | Self-signed **AMI default**: `C=US, O=American Megatrends Incorporated, OU=Service Processors, L=Norcross, ST=Georgia, CN=www.ami.com, emailAddress=support@ami.com`. RSA-2048, sha256WithRSA, `CA:FALSE`, no SAN. Valid **2018-04-23 → 2037-06-22**. SHA-256 fingerprint `ed4da7c3…b292bd18`. The same cert and key ship in every AMI image, so it provides no identity |
| Go client | default `crypto/tls` config negotiates TLS 1.3 fine. `InsecureSkipVerify` is needed for the self-signed cert (the policy in `docs/PLAN.md`). No legacy-cipher profile is needed (unlike [X470]) |
| Optional mTLS | `ssl.verifyclient.activate=enable, enforce=disable` (client certs accepted, not required) |

HTTP security headers (static paths): HSTS `max-age=31536000; includeSubDomains; preload`, CSP, `X-Frame-Options:
SAMEORIGIN`, `nosniff`, `X-XSS-Protection`, `Cache-Control: no-store`. Redfish responses add CORS:
`Access-Control-Allow-Origin: *` + `Access-Control-Allow-Credentials: true` + `Expose-Headers: X-Auth-Token`
(permissive. The browser won't actually combine `*` with credentials, but don't rely on that).

## 3. Frontend (summary; full analysis in `mz32-ar0-frontend.md`)

- **Stack: AMI MegaRAC SP-X classic HTML5 UI** (`<title>MegaRAC SP-X</title>`, almond 0.2.9 + jQuery 2.1.1 +
  Backbone), Gigabyte-branded, the same framework as the ASRock boards. KVM is the H5Viewer (`wss://host/kvm`) and
  Java JViewer jars still ship.
- No version file is exposed. Unknown static paths return **200 + index.html** (`error-handler-404`).
- 272 `/api/*` routes in the bundle, 240 with handlers. The Gigabyte-specific ones are `system_inventory_gbt/*`,
  `GSM`, `powerControl/*`, `fanprofile/*`.
- Login: `POST /api/session` (form `username`/`password`) → `QSESSIONID` cookie (`secure;HttpOnly`, **no SameSite**) +
  `CSRFToken`, sent back as `X-CSRFTOKEN`. **CSRF is enforced on GETs too** (bogus or missing token → 401 `cc:7`). On
  ASRock boards GETs ignore it.
- The KVM viewer's Help button calls **`/api/logs/help-data`** (crash-prone on AMI SP-X, see [X470]). It's on the deny
  list.
- I exercised 21 read routes live (all 200 except `/api/GSM` → 204) and logged out cleanly.

## 4. Authentication

| Test | Result |
|---|---|
| `GET /redfish/v1/` unauthenticated | 200 (ServiceRoot, includes `UUID` and `Oem.Ami.MACAddress`: a minor info leak) |
| `GET /redfish/v1/odata`, `/redfish/v1/$metadata` unauthenticated | 200 (normal) |
| `GET /redfish/v1/configurations` unauthenticated | **401** (the [X470]/[B650] `GlobalLuaTables`-style leak isn't there; `/configurations` is the AMI OEM resource, which needs auth) |
| Any other resource unauthenticated | 401 `Base.1.5.AccessDenied`, `WWW-Authenticate: Basic realm="AMI_Redfish_Server", X-Auth-Token realm="AMI_Redfish_Server"` |
| **Basic auth, admin** | **200** on `Systems` and the whole tree (as on [B650]; [X470] rejected it). Basic requests create **no** session (`Sessions` stayed empty during the Basic-auth crawl) |
| Basic auth, wrong password | 401 `Base.1.5.AccessDenied`. An AuditLog `Critical` entry is written per failure. `AccountLockoutThreshold 5`, `AccountLockoutDuration 30` s, `AccountLockoutCounterResetAfter 30` s |
| Session login `POST /redfish/v1/SessionService/Sessions` `{UserName,Password}` | **201** in 1.0 s. `X-Auth-Token` = **100 chars `[A-Za-z0-9]`**. `Location: /redfish/v1/SessionService/Sessions/<32-hex>` and `@odata.id` agree. Body `Session.v1_2_1` (`UserName`, no `Oem`). Quirk: `@odata.context` is malformed (`/redfish/v1/$metadataSessionService/Sessions`) |
| `SessionService.SessionTimeout` | **30** (seconds, the schema minimum, same as [B650]) |
| Token GET | 200. `DELETE` session → **204**. The token is rejected afterwards (401) |
| After logout | EventLog entries still reference the deleted session URI (`OriginOfCondition`/`EventId`) and 404 when followed. Harmless, but a crawler shouldn't treat those links as resources |
| Legacy `/api` | **Basic is not accepted**. Needs `POST /api/session` + cookie + `X-CSRFTOKEN` on every call (GETs included) |

## 5. Redfish feature / quirk table

| Area | Finding |
|---|---|
| Versions | `RedfishVersion 1.8.0`; schemas `ServiceRoot.v1_5_2`, `ComputerSystem.v1_8_0`, `Manager.v1_6_0`, `Chassis.v1_10_0`, `Thermal.v1_5_3`, `Power.v1_5_4`, `Sensor.v1_0_0`, `LogEntry.v1_4_3`, `UpdateService.v1_6_0`, `EventService.v1_5_0`, `TaskService.v1_1_4`, `TelemetryService.v1_1_2`, `AccountService.v1_6_0`, `SessionService.v1_1_6`, `ManagerNetworkProtocol.v1_4_2`, `VirtualMedia.v1_3_2`, `Bios.v1_1_0`, `Processor.v1_8_0`, `Memory.v1_8_0`, `Storage.v1_7_1`, `Drive.v1_7_0`, `PCIeDevice.v1_4_0`; message registry **`Base.1.5`** |
| `ProtocolFeaturesSupported` | `ExpandQuery{ExpandAll, Levels, Links, NoLinks, MaxLevels 5}`, `FilterQuery`, `SelectQuery`, `OnlyMemberQuery`, `ExcerptQuery` all `true` |
| **Query parameter escaping** | **Literal `$` only.** `?%24select=…`, `?%24top=…`, `?%24skip=…`, `?%24expand=…`, `?%24filter=…` → 200 with the **full, unmodified body** (treated as an unknown non-`$` param). This is the trap that bit the [B650] probe in reverse: there the shell dropped the `$`, here a correct client-side encoder breaks it |
| `$select` (literal) | **honoured**: `?$select=PowerState,BiosVersion` → 6 keys (4 `@odata.*` + the 2). An unknown property → 200 with `@Message.ExtendedInfo` and only the `@odata.*` keys |
| `$expand` (literal) | `.($levels=1)` and `*` **honoured** on `Systems` (members inlined, 5.8–6.3 KB vs 676 B). `~` returns the plain collection (links-only expand: nothing extra to show) |
| `$top` / `$skip` (literal) | **honoured** on log entries: `$top=2` → 2, `$skip=1&$top=2` → 2 starting at the 2nd, `$skip=55` → last 2 of 57, `$skip=1000` → 0 members. `$top=0` → 0 members. `$top=abc` → **400** `Base.1.5.QueryParameterValueTypeError` |
| `$filter` (literal) | **partly honoured, buggy.** `Severity eq 'Critical'` on `Chassis/Self/LogServices/Logs/Entries` → the 2 of 4 matching. On the 57-entry SEL, `Severity eq 'OK'` returns 50 members, `Members@odata.count 50` and `nextLink ?$skip=50` **without the filter**: the filter is applied to the current page only. A non-matching filter on the SEL returns 0 members **and still a nextLink**. An unknown property → 0 members, no error. **Don't forward `$filter`; filter in the proxy** |
| `only` | `/redfish/v1/Systems?only` → returns the single member's body (**honoured**) |
| `excerpt` | ignored (full body) |
| Unknown `$` param (`?$foobar=1`, literal) | **501** `Base.1.5.QueryNotSupported` (like [X470]; [B650] ignored it) |
| Unknown plain param (`?foo=bar`) | 200, ignored |
| Paging | page size **50**, `Members@odata.nextLink` = `<uri>?$skip=50` (literal `$`). Following it returns the rest and **no nextLink**, so **paging terminates**. `Members@odata.count` = total. Seen on SEL (57), BIOS log (86), `Processors/CPU0/SubProcessors` (64 cores) |
| Log entry collections | members are **inlined full `LogEntry` bodies** (no need to GET each entry). LogEntry has `Created`, `EventTimestamp`, `Severity`, `SensorNumber`, `SensorType`, `EntryCode`, `MessageId` (`AmiIpmiOem.1.0.GeneralEventData`) and a hex `Message` |
| 404 | genuine `404 Base.1.5.ResourceMissingAtURI` (no 401 masking as on [X470]) |
| `/redfish`, `/redfish/v1` (no slash) | both 200 |
| **ETags** | weak `W/"<epoch>"` (looks like a redis LastModified epoch), on the header **and** `@odata.etag`. **33 distinct values over 554 resources**, with one value covering 438. Per-resource-group, coarse (like [B650]). `If-None-Match` with the current ETag → **200 full body** (no 304). Conditional GET isn't implemented. `If-Match` on PATCH is untested (needs a write) |
| Latency | median **0.64 s**, p95 1.2 s, max 6.6 s (`TelemetryService/MetricDefinitions/*`). `Systems/Self` 1.4–4.2 s. A full crawl takes ~10 min single-threaded. The proxy should cache reads and never crawl the core/thread tree |
| Allow headers | `Systems/Self`, `Chassis/Self`, `Managers/Self`, `AccountService`, `NetworkProtocol`, LogServices, `SecureBoot`, `bond0`: `GET, PATCH`. `Bios`: **GET only**. `GBTFanprofile`: `GET,PUT`. `Sensors`: `GET,PATCH,POST` (odd) |
| `@Redfish.Settings` (pending/apply-time) | **Only on `Systems/Self/Bios`, and it's a stub**: `{"@odata.type":"#Settings.v1_2_0.Settings","ETag":"Dummyetag","Messages":[{"MessageId":"Base.1.0.SettingsFailed"}]}`. There's **no `SettingsObject`**, so no `Bios/SD` to PATCH. No other resource advertises `@Redfish.Settings`, so `Systems/Self` Boot/LED/PowerRestorePolicy PATCHes appear to be immediate (needs a write to confirm) |
| `@Redfish.OperationApplyTimeSupport` | on `ComputerSystem.Reset`, `Chassis.Reset`, `Manager.Reset` and every `LogService.ClearLog`: `SupportedValues ["Immediate","AtMaintenanceWindowStart"]`, `MaintenanceWindowDurationInSeconds 600`, `MaintenanceWindowResource` = the resource itself. The firmware schedules the deferred variant as a TaskService task |
| `@Redfish.ActionInfo` | present for every action. `ResetActionInfo` allowable values below |
| Gigabyte OEM | `Systems/Self.Oem.Gbt` (`GBTSystemsOemProperty` → `/redfish/v1/GpuManager/NVAPI` with empty GPU/FPGA/NVSwitch collections), `Chassis/Self.Oem.GBTChassisOemProperty."Board Serial Number"`, `Thermal.Oem.Gbt.FanprofileService` (`Fanprofile` `GET,PUT` with full curve tables, `SupportPCIEDevice`), `Power.PowerControl[0].Oem.Vendor` (`GbtPowerLimit`), `Chassis/Self/Power/SetPowerFloorInfo` (ActionInfo; Allow `POST`), `Memory.Oem.GbtMemoryOemProperty`, `Storage.Actions.Oem.#CreateLogicDevice/#DeleteLogicDevice` (their ActionInfos **404** with no RAID card). Registry `GBTOem.1.0.0` |
| AMI OEM | `ServiceRoot.Oem.Ami` (`Configurations`, `MACAddress`, `RtpVersion`), `Manager` OEM actions `RedfishDBReset`, `UploadLDAPCertificates`, `ConfigureCDInstance`, `EnableRMedia`, `#RestoreDefault`; `Manager.Oem.Ami.VirtualMedia {CDInstances "1", RMediaStatus "Disabled"}`; `UpdateService.Oem.AMIUpdateService.UpdateInformation` |

### 5.1 Resources the proxy cares about

| Resource | What's there |
|---|---|
| `Systems/Self` | `PowerState "On"`, `PowerRestorePolicy "LastState"`, `IndicatorLED "Off"` with `@Redfish.AllowableValues ["Lit","Blinking","Off"]` (**no `LocationIndicatorActive`**), `Boot{BootSourceOverrideEnabled "Once", Target "Hdd", Mode "UEFI", BootNext "", BootOrder[13], UefiTargetBootSourceOverride}` + AllowableValues: Target `None, Pxe, Floppy, Cd, Usb, Hdd, BiosSetup, Utilities, UefiShell, UefiTarget, SDCard, UefiHttp, RemoteDrive, UefiBootNext`; Enabled `Disabled, Once, Continuous`; Mode `Legacy, UEFI`. **A one-time `Hdd` override was already pending on this unit.** `ProcessorSummary`, `MemorySummary.TotalSystemMemoryGiB 256`, `BiosVersion "M32"`, `UUID` |
| `Systems/Self/ResetActionInfo` | `ResetType` ∈ **`ForceRestart`, `On`, `GracefulShutdown`, `ForceOff`**. There's no `PowerCycle`, `GracefulRestart`, `Nmi` or `PushPowerButton`. `Chassis/Self/ResetActionInfo` is the same. `Managers/Self/ResetActionInfo`: `ForceRestart` only |
| `Systems/Self/BootOptions` | 13 `BootOption.v1_0_3` (`Allow: GET, PATCH`, `BootOptionEnabled`, `UefiDevicePath`) |
| `Systems/Self/Processors/CPU0` | `Processor.v1_8_0` + `SubProcessors` (64 cores, each with 2 thread `SubProcessors`). **Crawl trap: ~200 resources.** The proxy should read only `CPU0` |
| `Systems/Self/Memory` | 16 `DIMM_P0_[A-H][01]` (absent slots `Status.State "Absent"`, 65535 widths) |
| `Systems/Self/Storage/StorageUnit_0` | 2 NVMe controllers inline (`StorageControllers[]`), 2 `Drives`, 2 `Volumes`. `SimpleStorage/1,2`. `Storage/1/Drives/*` links (from somewhere in the tree) **404** |
| `Systems/Self/EthernetInterfaces`, `NetworkInterfaces` | host NICs (`Onboard_0_*`, `Offboard_0_*`) with MAC/link state |
| `Systems/Self/LogServices/BIOS` | BIOS SEL view (86) |
| `Systems/Self/Bios` | 200, attribute dump (`GBT*`, `FBO*`, `IPMI*`, `MAPIDS` …). Settings stub (above). `Bios.ChangePassword` / `Bios.ResetBios` ActionInfos |
| `Systems/Self/SecureBoot` | `SecureBootEnable false`, `SecureBootMode "SetupMode"` |
| `Chassis/Self` | `ChassisType "Sled"`, `IndicatorLED` (same as Systems), `PowerState`, PCIeDevices (6) + PCIeFunctions (8), `NetworkAdapters` |
| `Chassis/Self/Thermal` | **12 `Temperatures`** (`ReadingCelsius`, full LNR/LC/LNC/UNC/UC/UNR thresholds, `SensorNumber`), **6 `Fans`** (`Reading` RPM). `Oem.Gbt.FanprofileService` |
| `Chassis/Self/Power` | `PowerControl[0]` (`PowerConsumedWatts 0`, `Status.Health "Critical"`, `State "Disabled"`; no PSU PMBus telemetry on this open-frame unit), **12 `Voltages`**, 4 `PowerSupplies` (all `N/A`), `Redundancy` (`NotRedundant`) |
| `Chassis/Self/Sensors` | **only `Energy` and `Power`** (`Sensor.v1_0_0`, `Reading 0`). Same Gigabyte override as [MC13] |
| `Managers/Self` | `FirmwareVersion "12.61.39"`, `DateTime`, `DateTimeLocalOffset "+00:00"`, `Model "410410600"`, `CommandShell{SSH,IPMI}`, `GraphicalConsole{KVMIP, MaxConcurrentSessions 2}`, `SerialConsole{IPMI}` |
| `Managers/Self/NetworkProtocol` | `HostName`/`FQDN` (MAC-derived), HTTPS 443, IPMI 623, SSH 22, SNMP 161 (**on**), SSDP off, KVMIP/VirtualMedia 443, **`NTP{ProtocolEnabled false, NTPServers ["time.nist.gov","pool.ntp.org"]}`** |
| `Managers/Self/EthernetInterfaces` | `bond0` (static IPv4, `Allow: GET, PATCH`), `usb0` (Host Interface) |
| `Managers/Self/HostInterfaces/Self` | **`AuthenticationModes ["AuthNone"]`, `AuthNoneRoleId "Administrator"`**, Firmware/Kernel auth → Administrator, `ExternallyAccessible false`, via `usb0` |
| `Managers/Self/LogServices` | `SEL` (IPMI SEL, `MaxNumberOfRecords 150`, `WrapsWhenFull`; 57 entries), `AuditLog` (Redfish auth failures etc.), `EventLog` (Redfish security events: logins). Each has a `ClearLog` ActionInfo + OperationApplyTime |
| `Managers/Self/VirtualMedia/CD1` | one CD instance, `Inserted false`, `InsertMedia`/`EjectMedia` actions, RMedia disabled |
| `Managers/Self/SerialInterfaces/IPMI-SOL` | 115200 8N1, hardware flow control, ETag `W/"999999999"` (constant) |
| `AccountService` | `MinPasswordLength 6`, `MaxPasswordLength 20`, lockout 5 / 30 s, `LocalAccountAuth "LocalFirst"`, LDAP/AD present but null. `Accounts` lists only **`1` = `admin`** (Administrator, `Enabled`, `Locked false`, `PasswordChangeRequired false`, `Password ""` echoed empty). The IPMI table has `anonymous` at userid 1 and `admin` at userid 2, and Redfish renumbers them. Roles `Administrator`/`Operator`/`ReadOnly`. `ExternalAccountProviders/RADIUS` |
| `UpdateService` | `SimpleUpdate` action (+ActionInfo), `MultipartHttpPushUri "/redfish/v1/UpdateService/upload"`, `MaxImageSizeBytes 199217152`. `FirmwareInventory`: `BMC 12.61.39`, `BIOS M32`, `MB_CPLD1 "ff"`, all `Updateable true` |
| `TaskService` | `Tasks` empty. `CompletedTaskOverWritePolicy "Oldest"`, `LifeCycleEventOnTaskStateChange true` |
| `EventService` | `ServerSentEventUri /redfish/v1/EventService/SSE` (not opened), `EventFormatTypes [Event, MetricReport]`, `Subscriptions` empty, SMTP configured off |
| `TelemetryService` | 3 `MetricDefinitions` (Temperature/Fan/Voltage reading) pointing at `Thermal#/…` and `Power#/…`. No reports or triggers |
| `CertificateService` | `GenerateCSR` / `ReplaceCertificate` ActionInfos. `Managers/Self/NetworkProtocol/HTTPS/Certificates/1` (the AMI cert) |
| `CompositionService`, `JobService` | present, empty / boilerplate |

## 6. Read-only behaviour tests (all GET)

| Test | Result |
|---|---|
| ServiceRoot ETag vs resource ETags | ServiceRoot `W/"1788106196"` unauthenticated and `W/"1794864199/250"` later. ETags move with redis writes (sessions, audit-log entries bump the epoch). Don't treat an ETag change as a content change |
| `Systems/Self` repeated GETs | ETag stable `W/"1788105423"` across the probe |
| SEL paging | 57 entries: page 1 = 50 + nextLink, page 2 = 7, no nextLink |
| BIOS log paging | 86 entries: 50 + 36 |
| Unauth `/redfish/v1/Registries`, `/JsonSchemas`, `/SessionService`, `/Managers/Self` | 401 |
| `GET /api/session` unauth | 404 `{"error":"Invalid API Call","code":1010}` |
| `/api` GET with bogus / no `X-CSRFTOKEN` | 401 `cc:7`. The session survived |
| SPA fallback | `/manifest.json`, `/version.txt`, `/robots.txt`, `/libs/js/html5shiv.js` → 200 + index.html |

## 7. Comparison

| Axis | ASRock X470D4U 2.32.00 | ASRock B650D4U 4.10.00 | **Gigabyte MZ32-AR0 12.61.39** | Gigabyte MC13-LE0 13.06.30 ([MC13] inventory + image) |
|---|---|---|---|---|
| SoC / AMI gen | AST2500 / SP-X, `FW_CODEBASEVERSION=3.X` (same generation as MZ32) | AST2600 / SP-X 13.x | **AST2500 / SP-X 12.x (12.2 base)** | AST2600 / SP-X 13.x |
| Image = live? | no (3.02 image) | no (7.02/9.05 images) | **yes, byte-identical UI** | n/a |
| Redfish admin auth | **rejected** (401) | Basic OK | **Basic OK** | (see MC13 docs) |
| RedfishVersion / RtpVersion | 1.5.0 / 1.5.b | 1.15.1 / 13.05 | **1.8.0 / 1.8.0** | ServiceRoot v1_13_0 / 13.06 |
| Message registry | Base.1.0 | Base.1.12 | **Base.1.5** | Base.1.12 |
| Sensors | n/a | 60 DMTF `Sensors` + Power/ThermalSubsystem | **`Sensors` = Energy+Power only; use `Thermal`/`Power`** | same Gigabyte override; `Thermal`/`Power` |
| Query escaping | n/a | `%24` decoded | **`%24` NOT decoded, literal `$` required** | unknown, check |
| `$select/$expand/$top/$skip` | advertised | all honoured | **honoured (literal `$`)** | |
| `$filter` | n/a | n/a | **page-local, nextLink drops filter** | |
| Unknown `$` param | 501 | 200 ignored | **501** | |
| Paging | n/a | 50, terminates | **50, terminates** | |
| ETags | single service-wide | 48 distinct, epoch | **33 distinct, weak epoch, no 304** | |
| SessionTimeout | n/a | 30 s | **30 s** | |
| ResetType | n/a | full set | **On, ForceOff, GracefulShutdown, ForceRestart** | |
| Unauth leak | `GlobalLuaTables` | `GlobalLuaTables` | **none beyond ServiceRoot UUID/MAC** | |
| TLS | TLS1.0–1.2, RSA-kx only, expired cert | TLS1.2+1.3 | **TLS1.2+1.3, ECDHE/DHE/RSA-GCM, AMI cert to 2037** | |
| `/api` cookie | bare `path=/` | Secure;HttpOnly;SameSite=Lax | **secure;HttpOnly (no SameSite)** | |
| `/api` CSRF on GET | not enforced | not enforced | **enforced** | |
| `/api` sensor unit | n/a | `"°C"` | **`"deg_c"`** | |
| Legacy `help-data` | crashes backend | (denylisted) | **handler present, called by KVM viewer: deny** | |

## 8. What a proxy driver needs

**Recommended primary southbound: Redfish, Basic auth.** Use the legacy `/api` only for the gaps listed below. The
board needs no IPMI path.

**Detection strings** (all from the unauthenticated ServiceRoot, plus one authenticated GET):
- `Server: AMI MegaRAC Redfish Service`, ServiceRoot `Vendor "AMI"`, `Product "AMI Redfish Server"`,
  `Oem.Ami.RtpVersion "1.8.0"` (→ SP-X 12 generation), `RedfishVersion "1.8.0"`, `@odata.type
  "#ServiceRoot.v1_5_2.ServiceRoot"`.
- Gigabyte: `Systems/Self.Manufacturer "GIGABYTE"`, `Systems/Self.Oem.Gbt`, registry `GBTOem.1.0.0` in `/Registries`,
  `/api/firmware-info` IANA 15370. The UI title alone (`MegaRAC SP-X`) doesn't separate it from ASRock.
- Generation switch: `Managers/Self.FirmwareVersion` major **12** (MZ32-AR0) vs **13** (MC13-LE0) / RtpVersion
  `1.8.0` vs `13.06`.

**Quirks to encode:**
1. **Build query strings by hand with a literal `$`** (`?$top=50&$skip=100`). Never use `url.Values.Encode()` for
   OData params. Follow `Members@odata.nextLink` verbatim (it already contains a literal `$`).
2. Don't forward `$filter` (page-local bug). Page with `$skip`/`$top` and filter in the proxy. Don't send unknown
   `$`-params (501).
3. Sensors: read `Chassis/Self/Thermal` (Temperatures/Fans) + `Chassis/Self/Power` (Voltages/PowerControl). Ignore
   `Chassis/Self/Sensors` (two dummy members).
4. Events: Redfish SEL lives in `Managers/Self/LogServices/SEL` + `Systems/Self/LogServices/BIOS` +
   `Chassis/Self/LogServices/Logs`. Merge the three and sort by `Created`. Entries are inlined in the collection.
   `Message` is hex only, so for human text use `/api/logs/event` (decoded, 1,022 records, `?LASTEVENTID=` for
   incremental reads).
5. `SessionTimeout 30 s`: use Basic. If sessions are used, refresh constantly and DELETE on eviction.
6. Read latency is 0.5–1.5 s per resource and up to 4 s for `Systems/Self`. Cache reads and cap concurrency at 1–2
   per host. Never walk `Processors/CPU0/SubProcessors`.
7. ETags are weak and coarse, and `If-None-Match` is ignored. Don't build caching on 304.
8. Reset mapping: `on→On`, `off→ForceOff`, `shutdown→GracefulShutdown`, `reset→ForceRestart`. **`cycle` has no
   native ResetType**: emulate it as ForceOff → poll → On, or return `unsupported`. `nmi` is unsupported.
9. LED: `IndicatorLED` (`Lit`/`Blinking`/`Off`) on `Systems/Self` or `Chassis/Self`. There's no
   `LocationIndicatorActive`.
10. Legacy `/api` (if used): `POST /api/session` form login, keep `QSESSIONID`, send `X-CSRFTOKEN` on **every**
    request, serialise per host (single fastcgi process), `DELETE /api/session` on eviction. Honour the deny list
    (`docs/firmware/mz32-ar0.md` §2.1), especially `/api/logs/help-data`.

**Capability → resource map:**

| Capability | Redfish (primary) | Gap / fallback |
|---|---|---|
| Identity / firmware | `Managers/Self.FirmwareVersion`, `Systems/Self.{Model,Manufacturer,BiosVersion}`, `UpdateService/FirmwareInventory/{BMC,BIOS,MB_CPLD1}` | dual-image `active_image` only in `/api/firmware-info` |
| Power state / control | `Systems/Self.PowerState`; `ComputerSystem.Reset` (4 types) | power-cycle: emulate |
| Boot override | `Systems/Self.Boot.BootSourceOverride{Target,Enabled,Mode}` (PATCH, `Allow: GET,PATCH`), `BootOptions`, `BootOrder` | whether PATCH is immediate is untested |
| LED | `IndicatorLED` PATCH | none |
| Sensors | `Chassis/Self/Thermal`, `Chassis/Self/Power` | `/api/sensors` has the same 36 sensors with IPMI state bits |
| Event log | 3 LogServices (above) | decoded text / full history: `/api/logs/event` |
| Clear log | `LogService.ClearLog` (Immediate / AtMaintenanceWindowStart) | disruptive tier |
| FRU / baseboard | `Chassis/Self` + `Systems/Self` serial/part/asset (placeholders on this unit) | full FRU areas: `/api/fru` |
| CPU / memory / PCIe / storage / NIC inventory | `Processors/CPU0`, `Memory/*`, `Chassis/Self/PCIeDevices/*`, `Storage/StorageUnit_0`, `SimpleStorage`, `EthernetInterfaces`, `NetworkAdapters` | SMBIOS detail: `/api/system_inventory_gbt/*` |
| BMC time / NTP | `Managers/Self.DateTime`; `NetworkProtocol.NTP` (PATCH) | timezone: `/api/settings/date-time` |
| BMC network | `Managers/Self/EthernetInterfaces/bond0` (PATCH) | bonding/NCSI/VLAN detail: `/api/settings/network*` |
| Accounts | `AccountService/Accounts` (POST/PATCH/DELETE allowed) | SNMP/email/KVM per-user flags: `/api/settings/users` |
| Virtual media | `Managers/Self/VirtualMedia/CD1` Insert/Eject; `Oem/Ami` `EnableRMedia` (RMedia currently Disabled) | |
| Fan control | `Chassis/Self/Thermal/FanprofileService/Fanprofile` (`GET,PUT`, Gigabyte schema) | `/api/settings/fanprofile*` |
| BMC reset | `Manager.Reset` (`ForceRestart`) | |
| Firmware update | `UpdateService` SimpleUpdate / multipart push | out of scope (disruptive) |
| Uptime | none (Redfish has no BMC uptime) | `/api/status/uptime` gives **host** POH, not BMC uptime |
| Services / sessions / single-port | `NetworkProtocol` (enabled flags + ports) | `/api/settings/services` |
| BIOS settings | read: `Systems/Self/Bios.Attributes`; write: **not available** (Settings stub, no SD) | none found |

## 9. Security findings

1. **Default admin credential in use.** The `admin` account on this unit authenticates with a **well-known vendor
   default password** (Redfish, `/api` and presumably IPMI/SSH, since they share one account store). Redfish reports
   `PasswordChangeRequired false` (the `/api` login's `passwordStatus` value was not recorded). **Change it
   before this BMC is reachable from anything but an isolated management network.** (The password is deliberately
   not written in this repository.)
2. **Image-wide `sysadmin` shell account.** The firmware's `/conf/shadow` seeds a uid-0 `sysadmin` user with a fixed
   DES crypt hash (AMI's well-known factory account), and SSH is enabled. TCP/22 was filtered from the probe host, so
   I didn't test it. Restrict SSH at the network, or confirm the account has been changed or disabled.
3. **Redfish Host Interface `AuthNone` → Administrator.** Any software on the host OS that brings up the `usb0`
   interface gets **unauthenticated Administrator Redfish** (`AuthenticationModes ["AuthNone"]`,
   `AuthNoneRoleId "Administrator"`). For multi-tenant or bare-metal-as-a-service use, disable the host interface or
   require credentials.
4. **Shared AMI default TLS certificate** (`CN=www.ami.com`, 2018–2037). Its private key ships in every AMI image, so
   TLS gives confidentiality only against passive observers, not authentication. Static-RSA GCM suites (no forward
   secrecy) are still offered.
5. **IPMI over LAN**: cipher suite **0** advertised, IPMI 1.5 enabled, KG all-zero, MD5 auth type. With the default
   password this is a direct path to power control and SOL. Disable suite 0 / IPMI 1.5 and set KG.
6. **SNMP enabled** (161). The community and version weren't checked.
7. **Unauthenticated ServiceRoot** leaks the service `UUID` and the BMC `MACAddress` (`Oem.Ami.MACAddress`). Minor.
8. `/api` cookie has no `SameSite`. Redfish sends `Access-Control-Allow-Origin: *` with `Allow-Credentials: true`.
9. Positive: no unauthenticated `/redfish/v1/configurations` / GlobalLuaTables leak (unlike both ASRock boards),
   TLS 1.0/1.1 disabled, HSTS/CSP on the UI, CSRF enforced on all `/api` methods, account lockout (5 failures / 30 s),
   and failed logins land in the AuditLog.

## Not called (actions / writes)

Every `/Actions/` target (Reset ×3, ClearLog ×5, SimpleUpdate, InsertMedia/EjectMedia, Bios.ChangePassword/ResetBios,
SecureBoot.ResetKeys, Certificate Rekey/Renew/ReplaceCertificate/GenerateCSR, SubmitTestEvent,
SubmitTestMetricReport, all AMI and Gigabyte OEM actions including `SetPowerFloorTarget` and RAID Create/Delete),
the SSE stream, the multipart upload URI, `Managers/Self/Oem/LDAPCertificateUploadActionInfo` (skipped by
the crawler's deny pattern), every PATCH/PUT/POST/DELETE except the Redfish session create/delete and the `/api`
login/logout, every route on the firmware deny list (including `/api/logs/help-data`), and every
download/backup/firmware/HPM/maintenance route.

## Needs a write to confirm (next, with approval)

1. `PATCH Systems/Self` `Boot.BootSourceOverrideTarget/Enabled`: immediate or pending? Does it clear after one
   boot? Does `If-Match` with the weak ETag work, and is a stale ETag rejected (412)?
2. `PATCH Systems/Self` `IndicatorLED` `Lit`/`Blinking`/`Off`: does it reflect immediately, and does `Chassis/Self` mirror it?
3. `ComputerSystem.Reset` `GracefulShutdown` while the OS is running (does it get ignored like on ASRock?), and the
   convergence time to `PowerState Off`. Also `ForceRestart` vs `On`, and the `AtMaintenanceWindowStart` task flow.
4. `PATCH NetworkProtocol.NTP` (enable plus servers): does `Managers/Self.DateTime` follow?
5. `PATCH Systems/Self.PowerRestorePolicy`. The allowable values aren't advertised, so check which are accepted.
6. Whether `Systems/Self/Bios` ever exposes a `SettingsObject` (after a BIOS POST with Redfish BIOS config enabled)
   and how pending settings apply on reset.
7. `PUT .../FanprofileService/Fanprofile` semantics (whole-object PUT?) and its interaction with `/api/settings/fanprofile/mode`.
8. `LogService.ClearLog` on each of the 3 SEL views: does clearing one clear the others (same IPMI SEL)?
9. `/api` write CSRF/`X-HTTP-Method-Override` behaviour (it's enforced for GET, so presumably for writes too).
10. Session limits: how many concurrent Redfish sessions, and whether Basic-auth bursts trip the 5-failure lockout
    when the password is wrong (lockout is per account, so a misconfigured proxy could lock the admin out for 30 s).

## Method and hygiene

- One shell per step, credentials loaded with `set -a; . ./MZ32-AR0-00.env; set +a` and passed to `python3 -I`
  scripts as environment variables. URLs were built in Python, and query strings with a literal `$` were written
  explicitly. I verified via `requests`' `PreparedRequest.url` what was actually sent (`%24…` vs `$…`) for every
  query test.
- Strictly one request at a time with 0.25–0.6 s gaps. No 5xx and no timeouts were seen at any point (561-resource
  crawl + ~60 query tests + 25 `/api` calls + 10 static fetches).
- Sessions created: **1 Redfish session** (created, used for 3 GETs, `DELETE` → 204, verified 401 afterwards) and
  **1 `/api` session** (login, 21 GETs + 2 CSRF tests, `DELETE /api/session` → 200 `{"ok":0}`, verified 401
  afterwards). Everything else used Basic auth, which creates no session.
- Network: one nmap TCP connect scan of 8 ports, one nmap `ssl-enum-ciphers`/`ssl-cert` run, one `openssl s_client`
  handshake, and 7 sessionless IPMI datagrams (ASF ping, Get Channel Auth Capabilities ×2, Get Channel Cipher Suites ×4).
- **Nothing was written to the BMC**: no PATCH/PUT/POST other than the two logins, and no actions.
- Raw captures (crawl bodies, query results, `/api` responses) were kept in the session scratchpad only for
  sanitizing. The files holding session ids or cookies were deleted at the end.

## Addendum: settings/network-bond (2026-11-16, read-only)

One read of the AMI bonding object, to pin the shape the gigabyte `BondManager` (generic `GET /api/v1/network/bond`) and
the vendor `bonding` routes parse. Program: `vendors/gigabyte/internal/probe/bond` (build tag `gigabytelive`).

| Step | Time | Result |
| --- | --- | --- |
| `POST /api/session` (login) | 1900 ms | ok, CSRF token taken from the login body and sent on the GET |
| `GET /api/settings/network-bond` | 68 ms | 200, 111 bytes |
| `DELETE /api/session` (logout) | 500 ms | ok (no retry needed) |

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
whole object→`Oem`. Fixture: `vendors/gigabyte/testdata/network-bond-mz32ar0-12.61.39.json`.

Not verified: the `PUT` (it restarts BMC networking and was not sent), its response body, and whether the firmware
accepts `bond_enable 0` with `bond_ifc "both"` (the driver refuses that combination).

Hygiene: credentials loaded with `set -a; . ./MZ32-AR0-00.env; set +a` in the same shell command and read from the
environment by the program; the password was never printed, logged or written. Exactly three requests, one at a time:
login, the GET, logout. **No writes.**
