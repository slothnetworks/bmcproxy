# B650D4U BMC: live probe

Status: preliminary assessment for the native Go management proxy. This is a live,
strictly **read-only** probe of one real ASRock Rack B650D4U BMC (`10.7.12.55`, a
non-production unit) on 2026-11-16, done to confirm or refute the static findings in:

- `docs/firmware/b650d4u.md`, the static analysis of the **7.02.00 / 9.05.00** images (cited **[FW]**)
- `docs/b650/har-analysis.md` and `har-endpoints.json`, a web-UI capture from **1.10.00** (**[HAR]**)
- `docs/redfish/b650-implementation.md` and `b650-uri-inventory.json` (built from 7.02/9.05) (**[RF]**)
- `docs/api/legacy-api-catalog.md` / `legacy-api-endpoints.json` (**[CAT]**)
- Contrast is drawn throughout with the X470D4U live probe, `docs/x470/live-probe.md` (**[X470]**).

Companion file: `docs/b650/live-redfish-inventory.json` (the sanitized authenticated Redfish crawl:
`{uri, status, type, etag, etag_differs, actions, latency_ms, notes}` per resource).

**Redaction.** MACs, UUIDs/GUIDs, the BMC's MAC-derived hostname, IPv6 link-local addresses and
non-`10.7.x` IPs are replaced with `<REDACTED_*>` placeholders, with the format noted. `10.7.x`
addresses are kept (they are the lab management subnet). No password, session token, `X-Auth-Token`,
CSRF token or cookie value appears in this document. Raw dumps with unredacted values stay in the
probe scratchpad and are not copied here.

---

## Live firmware (read this first)

> **The live unit runs BMC firmware `4.10.00`** — a version that matches **none** of the static
> sources. Our dumped images are 7.02.00 and 9.05.00; the HAR capture was 1.10.00; this unit is
> 4.10.00, built **Jun 4 2024**. Treat every difference from the static docs below as a possible
> **version gap first**, not a doc error. The IPMI `prod_id 0x2019` confirms this is a genuine
> **B650D4U** (the same product id as the 7.02 `B650D4U.PRJ` in [FW §0]), not the `_2L2T` SKU and
> not the AM4 "B550D4U-2T" that the [HAR] capture turned out to be.

| Item | Value | Source |
|---|---|---|
| Board | `B650D4U` | `/api/asrr/model-name` → `{"mb_name":"B650D4U","sys_name":"N/A"}`; Redfish `Systems/Self.Manufacturer "AsrockRack"` |
| BMC firmware | **4.10.00**, built `Jun 4 2024 10:12:26 UTC`, active image 0 | `/api/asrr/fw-info` `BMC_fw_version`; `/api/firmware-info` (`fw_rev_1 4`, `fw_rev_2 16`=0x10, `fw_ver "4.10.00"`, device_id 32, mfg 0xC1D6=ASRock, `prod_id 8217`=0x2019, date `Jun 4 2024`) |
| BMC firmware (Redfish) | `Managers/Self.FirmwareVersion` **`4.10.0`**; `UpdateService/FirmwareInventory/BMC.Version` **`4.10.00`**, `Updateable:true` | Redfish |
| BIOS | `10.15` | `/api/asrr/fw-info` `BIOS_fw_version`; Redfish `Systems/Self.BiosVersion 10.15`; `FirmwareInventory/BIOS` |
| PSP | `00.2B.00.55` | `/api/asrr/fw-info` |
| CPU microcode | `0a601206` | `/api/asrr/fw-info` |
| ME / CPLD / BP | `N/A` | `/api/asrr/fw-info` (AM5 has no ME) |
| Host power | **Off** for the whole probe | `Systems/Self.PowerState "Off"`, `Chassis/Self.PowerState "Off"`, `/api/chassis-status power_status 0` |
| BMC clock | `2026-11-16T…-05:00` (offset `-05:00`, tz `GMT-05`). NTP servers configured (`pool.ntp.org`, `time.nist.gov`) but **NTP sync disabled** | `Managers/Self.DateTime`, `NetworkProtocol.NTP.ProtocolEnabled false`, `/api/settings/date-time` |

---

## Summary verdict

- **Redfish works with the admin credentials, over HTTP Basic — this is the proxy's B650 path.**
  `GET /redfish/v1/Systems/Self` with Basic auth returns **200**, and so does the entire tree. This
  is the **opposite of the X470** [X470 §2], whose Redfish rejected the same admin account with 401
  `Security.1.0.AccessDenied`. On the B650 the proxy can and should drive Redfish directly; the
  legacy `/api` is only needed for the handful of ASRock-OEM things Redfish does not expose.
- **The authenticated BFS crawl succeeded.** From `/redfish/v1/`, following every `@odata.id`,
  `@Redfish.ActionInfo` and `Members@odata.nextLink`, it reached **691 URIs: 677× `200`, 12× `404`,
  2 skipped** (SSE stream + multipart upload target). The crawl was stopped by the ~600 resource
  cap inside the large `Chassis/Self` sensor subtree, so a few deep leaf members past the cap were
  not fetched; everything structurally important was reached.
- **This is a much newer and more complete AMI MegaRAC stack than the X470.**
  `ServiceRoot.v1_7_0`, `RedfishVersion 1.15.1`, `Vendor "AMI"`, AMI `RtpVersion "13.05"` (the X470
  was `ServiceRoot.v1_3_1` / `1.5.0` / RtpVersion `1.5.b`). Modern schema versions throughout:
  `ComputerSystem.v1_16_0`, `Manager.v1_13_0`, `UpdateService.v1_6_0`, `EventService.v1_5_0`,
  `TelemetryService.v1_2_1`, `Sensor.v1_0_2`. CertificateService, Sensors, PowerSubsystem/
  ThermalSubsystem, TelemetryService-with-schema and SSE — all **present** here, all absent/empty on
  the X470.
- **Auth model.** Redfish advertises `WWW-Authenticate: Basic realm="AMI_Redfish_Server"` **and**
  `X-Auth-Token`. Basic works; prefer it so the proxy never creates server-side sessions. The legacy
  `/api` still uses form login → `QSESSIONID` cookie + `X-CSRFTOKEN` header; **the cookie is now
  `secure; HttpOnly; SameSite=Lax`** (the X470's was bare `path=/`). Auth failures on `/api` return
  `401 {"cc":7,"error":"Invalid Authentication"}` as before.
- **Top quirks the proxy must handle (details in §3–4):**
  1. **Query options WORK (corrected).** An earlier pass reported `$select`/`$top`/`$skip`/`$expand`
     as no-ops; that was a probe bug (unescaped `$` in shell commands, so the params were never sent).
     Re-verified from the main session with proper escaping: `$select=Id` returns only `Id`; `$top=3`
     on Sensors returns 3; `$top=2` on SEL returns 2 of 11; `$expand=.($levels=1)` on
     `AccountService/Accounts` inlines the member bodies (1946 B vs 535 B plain). FilterQuery not
     re-verified. An unknown query param returns **200** (silently ignored), not the X470's 501.
  2. **Pagination WORKS (corrected).** Collections page at **50 members** with
     `Members@odata.nextLink = ?$skip=50`; following it (properly escaped) returns the remaining
     members and a null nextLink. Verified on `Sensors`: page 1 = 50 (ids 112…50), page 2 = 10
     (ids 51…9), all **60/60** reachable, no loop. The earlier "infinite loop / 10 sensors
     unreachable" finding was the same `$`-escaping bug. The proxy should still cap its own walk
     depth defensively, but the BMC's nextLink terminates correctly.
  3. **TLS is modern** (§2): TLS 1.2 + 1.3, ECDHE/AEAD only, forward secrecy, grade A, cert valid to
     2035. Go's default `tls.Config` works as-is — the opposite of the X470, which needed explicit
     RSA-kex cipher suites.
  4. **`SessionTimeout` is `30`** (seconds, per the DMTF SessionService schema — the schema minimum).
  5. **Security:** the factory **`Administrator` account is enabled** (id 1, Administrator role); a
     second random-named account is the host-interface auto account; `admin` (id 4) is the one in
     use. `GlobalLuaTables` is readable **unauthenticated** and must be blocked by the proxy.
  6. `OPTIONS` → **501** (JSON); HTTP on port **80 is closed** (no redirect); `/redfish/v1` returns
     **200** directly (the X470 302-redirected it).

---

## 1. Ports

TCP connect test, one port at a time, 3 s timeout.

| Port | Result | Note vs [X470] / [FW §3.1] |
|---|---|---|
| 22 | **open** (SSH) | same as X470 |
| 80 | **closed** | **differs**: the X470 had 80 open → 301 to https. Here there is no HTTP listener at all |
| 443 | **open** (lighttpd; `Server: AMI MegaRAC Redfish Service` on `/redfish`, `lighttpd` on `/api`) | same |
| 623 | **open (TCP)** | X470 also had TCP 623 open (unidentified); UDP RMCP not re-tested here |
| 5120, 5123, 7578 | closed | single-port mode is on (`/api/configuration/runtime SINGLE_PORT_APP`, `settings/services singleport_status 1`), KVM/media ride 443 |
| 9080, 9090 | closed | good (loopback-only / absent) |

Note: `/api/settings/services` lists a **`vnc` service on secure port 5901** (new vs X470); it was
not in the scan list and, under single-port mode, is not necessarily externally reachable.

---

## 2. TLS

**Status: modern TLS — the single biggest operational difference from the X470.**

`nmap --script ssl-enum-ciphers -p 443`:

| | Live B650 4.10.00 | [X470] 2.32.00 |
|---|---|---|
| Protocols | **TLS 1.2 and TLS 1.3 only** (1.0/1.1 not offered) | TLS 1.0/1.1/1.2, no 1.3 |
| TLS 1.2 ciphers | `ECDHE_RSA_WITH_AES_256_GCM_SHA384`, `…CHACHA20_POLY1305_SHA256`, `…ARIA_256_GCM`, `…AES_128_GCM`, `…ARIA_128_GCM` (all secp256r1, grade **A**) | RSA-kex only, no ECDHE |
| TLS 1.3 ciphers | `AES_256_GCM_SHA384`, `CHACHA20_POLY1305_SHA256`, `AES_128_GCM_SHA256` | none |
| Forward secrecy | **Yes** (ECDHE) | **No** |
| Cipher preference | server | server |

**Go client:** the default `tls.Config` works unchanged (ECDHE-RSA + AES-GCM are in Go's defaults).
`MinVersion: tls.VersionTLS12` is fine. None of the X470's RSA-kex workarounds (`CipherSuites` /
`GODEBUG=tlsrsakex=1`) are needed.

**Certificate** (`openssl s_client`, single handshake):

| Item | Value |
|---|---|
| Subject | `C=US, ST=Georgia, L=Duluth, O=AMI, OU=MEGARAC, CN=ami.com, emailAddress=megarac@ami.com` |
| Issuer | `C=US, ST=Georgia, L=Norcross, O=American Megatrends International LLC (AMI), OU=Service Processors, CN=megarac.com` |
| Validity | **2025-04-16 → 2035-04-14 (NOT expired)** |
| Key / sig | RSA-2048, `sha256WithRSAEncryption`, serial `784454F7A5087F786C62769B248FBD3D349A3BE2` |
| SAN | `DNS:megarac.com`, `DNS:169.254.0.17`, `IP:169.254.0.17` (the USB host-interface link-local address) |
| SHA-256 fp | `22:A0:A7:88:05:DB:62:AF:9C:0C:C1:E1:28:A0:55:E7:38:BA:E9:A4:2C:42:91:73:EE:A7:26:B4:CC:06:A3:67` |
| Is it unit-specific? | **No.** It is the AMI MEGARAC shared firmware cert (CN `ami.com`, SAN `megarac.com` / `169.254.0.17`), issued by an AMI CA. `/api/settings/ssl/certificate-info` dates the cert `Apr 16 2025` and the **private key `Jun 4 2024`** (= the firmware build date), so the key ships in the image |

So the cert still provides **no real authentication of this host** (shared across units, CN/SAN do
not match the BMC address, key is in the firmware), but — unlike the X470's expired-2026 cert — it
is **not expired**. The proxy should still pin per-BMC (trust-on-first-use) or skip verification.

---

## 3. Redfish — authenticated findings (redacted JSON)

All bodies below were fetched with HTTP Basic auth. Latencies are ~60–250 ms per GET on a reused
keep-alive connection (TLS handshake ~170 ms amortised; see the inventory JSON for per-URI numbers).

### 3.1 ServiceRoot (`GET /redfish/v1/`)

```json
{"@odata.type":"#ServiceRoot.v1_7_0.ServiceRoot","RedfishVersion":"1.15.1",
 "Product":"AMI Redfish Server","Vendor":"AMI","Id":"RootService","Name":"Root Service",
 "ProtocolFeaturesSupported":{"DeepOperations":{"DeepPATCH":true,"DeepPOST":false,"MaxLevels":6},
   "ExcerptQuery":true,"ExpandQuery":{"ExpandAll":true,"Levels":true,"Links":true,"MaxLevels":5,"NoLinks":true},
   "FilterQuery":true,"OnlyMemberQuery":true,"SelectQuery":true},
 "Oem":{"Ami":{"@odata.type":"#AMIServiceRoot.v1_0_0.AMIServiceRoot","RtpVersion":"13.05",
   "Configurations":{"@odata.id":"/redfish/v1/Oem/Ami/Configurations"},
   "InventoryDataStatus":{"@odata.id":"/redfish/v1/Oem/Ami/InventoryData/Status"}}}}
```

Top-level links: AccountService, CertificateService, Chassis, CompositionService, EventService,
JsonSchemas, Managers, Registries, SessionService, Systems, Tasks (→ TaskService), TelemetryService,
UpdateService. `RtpVersion 13.05` matches the [FW] `MDS_VERSION 13.5` codebase. The root is served
**200 unauthenticated** (as are `/redfish`, `/redfish/v1`, `$metadata`, `GlobalLuaTables`); every
other resource requires auth.

> **Conformance caveat:** `ProtocolFeaturesSupported` advertises Select/Expand/Filter/Excerpt/
> OnlyMember/Deep, but none of the query modifiers are actually applied (§4). Do not trust the
> advertisement.

### 3.2 Systems/Self (`#ComputerSystem.v1_16_0`, `Allow: GET,PATCH`)

```json
{"Id":"Self","Name":"System","SystemType":"Physical","Manufacturer":"AsrockRack",
 "Model":null,"BiosVersion":"10.15","PowerState":"Off",
 "Boot":{"BootSourceOverrideEnabled":"Disabled","BootSourceOverrideMode":"Legacy",
   "BootSourceOverrideTarget":"None","BootOrderPropertySelection":"BootOrder",
   "BootOrder":["Boot0003","Boot0004","Boot0005","Boot0006","Boot0002","Boot0001"],
   "BootOptions":{"@odata.id":"/redfish/v1/Systems/Self/BootOptions"},
   "Certificates":{"@odata.id":"/redfish/v1/Systems/Self/Boot/Certificates"},
   "BootNext":null,"HttpBootUri":null,"UefiTargetBootSourceOverride":null},
 "Bios":{"@odata.id":"/redfish/v1/Systems/Self/Bios"},
 "Links":{"Chassis":[…"/redfish/v1/Chassis/Self"],"ManagedBy":[…"/redfish/v1/Managers/Self"]}}
```

- Sub-collections present: Bios, EthernetInterfaces, GraphicsControllers, LogServices, Memory,
  MemoryDomains, NetworkInterfaces, Processors, SecureBoot, SimpleStorage, Storage, USBControllers.
  Memory exposes 4 DIMM slots (`DevType2_DIMM0..3`, each with an `AmiBios.ChangeState` action).
  Because the host is **off**, Processor/Memory/Storage readings are mostly `Absent`/null.
- **`BootSourceOverrideTarget@Redfish.AllowableValues` is NOT present inline** (the X470 analysis
  expected it to be) — the allowable targets are only in the schema, not advertised on the resource.
- `Boot` mode is **Legacy** here. `BiosVersion 10.15`.
- **`Systems/Self/ResetActionInfo` → `ResetType` allowable: `["ForceOff","ForceRestart",
  "GracefulShutdown","On","PowerCycle"]`** (no `GracefulRestart`, `Nmi`, `PushPowerButton`, `ForceOn`).
- `Bios` resource is large (~64 KB of attributes) and exists even with the host off.

### 3.3 Chassis/Self (`#Chassis.v1_x`, `Allow: GET,PATCH`)

```json
{"Id":"Self","ChassisType":"Other","Name":"Computer System Chassis",
 "Manufacturer":"To be filled by O.E.M.","PowerState":"Off",
 "IndicatorLED":"Off","IndicatorLED@Redfish.AllowableValues":["Lit","Blinking","Off"]}
```

Both the **legacy** `Power`/`Thermal` and the **modern** `Sensors` + `PowerSubsystem` +
`ThermalSubsystem` trees are present and populated (the X470 had none of the modern ones):

- `Chassis/Self/Power` (`#Power.v1_5_4`): **15 Voltages**, 1 PowerControl, 0 PowerSupplies.
  e.g. `{"Name":"VOLT_3VSB","ReadingVolts":3.32,"UpperThresholdCritical":3.72,"SensorNumber":1,…}`.
- `Chassis/Self/Thermal` (`#Thermal.v1_5_3`): **12 Temperatures, 16 Fans**. With the host off,
  `TEMP_CPU` reads null / `Status.State "Absent"` and fans read null RPM / Absent.
- `Chassis/Self/Sensors` (`#SensorCollection`): **60 members**. Representative
  `Sensors/13` (`#Sensor.v1_0_2`): `{"Name":"VOLT_BAT","ReadingType":"Voltage","Reading":3.08,
  "PhysicalContext":"VoltageRegulator","Thresholds":{"LowerCritical":{"Reading":2.4},
  "LowerFatal":{"Reading":2.22},"UpperCritical":{"Reading":3.3},"UpperFatal":{"Reading":3.46}}}`.
  `ReadingUnits` is `null` even for voltage sensors (rely on `ReadingType`).
- Sensor names use the B650 scheme (`VOLT_*`, `TEMP_*`, `FANn_m`, `STS_*`) — matches [HAR §6].
- **`Chassis/Self/ResetActionInfo` → `ResetType`: `["ForceOff","ForceRestart","GracefulShutdown",
  "On","PowerCycle"]`.**

### 3.4 Managers/Self (`#Manager.v1_13_0`, `Allow: GET,PATCH`)

```json
{"Id":"Self","ManagerType":"BMC","FirmwareVersion":"4.10.0","Model":"821749622","PowerState":"On",
 "DateTime":"2026-11-16T…-05:00","DateTimeLocalOffset":"-05:00",
 "UUID":"<REDACTED_UUID>","ServiceEntryPointUUID":"<REDACTED_UUID>",
 "CommandShell":{"ConnectTypesSupported":["SSH","IPMI"],"MaxConcurrentSessions":36,"ServiceEnabled":true},
 "GraphicalConsole":{"ConnectTypesSupported":["KVMIP"],"MaxConcurrentSessions":2,"ServiceEnabled":true},
 "SerialConsole":{"MaxConcurrentSessions":1,"ServiceEnabled":true},
 "Links":{"ManagerInChassis":{"@odata.id":"/redfish/v1/Chassis/Self"}},
 "Oem":{"Ami":{"PowerSaveMode":"Enable","VirtualMedia":{"CDInstances":1,"RMediaStatus":"Disabled"}}},
 "Actions":{"#Manager.Reset":{"@Redfish.OperationApplyTimeSupport":{"SupportedValues":["Immediate","AtMaintenanceWindowStart"],"MaintenanceWindowDurationInSeconds":600},…}}}
```
`UUID` format is `8-4-4-4-12`; **its last 12 hex digits equal the BMC MAC** (and the host-interface
`Systems` UUID from `/api/host_inventory` embeds the MAC the same way) — redacted.
`Model "821749622"` is a numeric model code (same across the SKU; not a serial).

- **`Managers/Self/ResetActionInfo` → `ResetType`: `["ForceRestart"]`** only (BMC can only be cold-reset).
- Sub-resources: `NetworkProtocol`, `EthernetInterfaces` (bond0/eth0/eth1/usb0), `HostInterfaces`
  (`Self`), `SerialInterfaces`, `VirtualMedia` (empty), `LogServices`, `Certificates`.

**NetworkProtocol** (`GET …/NetworkProtocol`): HTTPS 443 on, KVMIP 443 on, VirtualMedia 443 on,
IPMI 623 on, SSH 22 on, SNMP 161 on, **NTP 123 off**. No HTTP (consistent with port 80 closed).

**EthernetInterfaces/eth0** (`#EthernetInterface.v1_6_2`, `Allow: GET,PATCH`):
```json
{"Id":"eth0","InterfaceEnabled":true,"LinkStatus":"LinkUp","SpeedMbps":1000,"MTUSize":1500,
 "MACAddress":"<REDACTED_MAC>","PermanentMACAddress":"<REDACTED_MAC>",
 "HostName":"<REDACTED_HOSTNAME (format \"AMI\"+MAC)>","FQDN":null,
 "DHCPv4":{"DHCPEnabled":false},
 "IPv4Addresses":[{"Address":"10.7.12.55","AddressOrigin":"Static","Gateway":"10.7.12.1","SubnetMask":"255.255.255.0"}],
 "IPv6Addresses":[{"Address":"<REDACTED_IPV6_LL>","AddressOrigin":"LinkLocal","PrefixLength":64}],
 "StaticNameServers":["<REDACTED_IP (192.168.x.x)>",null,null],
 "VLAN":{"VLANEnable":false,"VLANId":0},
 "Oem":{"Ami":{"@odata.type":"#AMIDNSConfiguration.v1_0_0.AMIDNSConfiguration","DNSEnabled":true,"HostNameSetting":"Auto"}}}
```
`HostInterfaces/Self` exists (USB NIC `usb0`, link-local `169.254.0.17` family — the Redfish-over-
host-interface path [RF `REDFISH_HOSTINTERFACE`]).

**LogServices** (Manager): `AuditLog`, `EventLog`, `SEL`. `SEL` (`#LogService.v1_1_3`):
`MaxNumberOfRecords 150`, `OverWritePolicy "WrapsWhenFull"`, `LogEntryType "SEL"`, `ClearLog` action.
`SEL/Entries` currently holds **11** `#LogEntry.v1_4_3` records, each with a decoded `Message`,
`SensorType`, `SensorNumber`, `MessageId` and an `OriginOfCondition` link. SEL/EventLog timestamps
are inconsistent (records dated 2022, 2025 and 2026 with `Created` ≠ `EventTimestamp`), reflecting
the drifting clock (NTP off) — the [HAR §10] clock warning still applies.

### 3.5 AccountService + accounts + roles (no secrets)

`AccountService` policy: `AccountLockoutThreshold 5`, `AccountLockoutDuration 30`,
`AccountLockoutCounterResetAfter 30`, `MinPasswordLength 8`, `MaxPasswordLength 20`,
`AuthFailureLoggingThreshold 3`, `LocalAccountAuth "LocalFirst"`.

`Accounts` has 3 members (ids 1, 2, 4); `Password` is present but always `null` (no secret exposed):

| Id | UserName | RoleId | Enabled | Locked | Note |
|---|---|---|---|---|---|
| 1 | `Administrator` | Administrator | **true** | false | **factory default admin account, enabled** |
| 2 | `<REDACTED_HOSTIF_USER>` (16-char random) | HostInterfaceAdministrator | true | false | auto-generated host-interface account |
| 4 | `admin` | Administrator | true | false | the account this probe used |

`Roles`: `Administrator`, `HostInterfaceAdministrator`, `Operator`, `ReadOnly`.

> **Security:** the enabled factory `Administrator` account (id 1) is a default-credential risk; the
> proxy/operators should confirm it has a non-default password or is disabled.

### 3.6 SessionService

```json
{"@odata.type":"#SessionService.v1_1_6.SessionService","ServiceEnabled":true,
 "SessionTimeout":30,"Sessions":{"@odata.id":"/redfish/v1/SessionService/Sessions"}}
```
**`SessionTimeout` = `30`. Units: seconds** — the DMTF `SessionService` schema defines this property
in seconds (allowed range 30–86400), so 30 s is the **schema minimum**: a Redfish session goes idle-
stale after 30 s of inactivity. `SessionService/Sessions` was empty during the probe (we used Basic
auth and created no session). The proxy should either re-auth frequently or (preferred) use Basic.

### 3.7 UpdateService

```json
{"@odata.type":"#UpdateService.v1_6_0.UpdateService","ServiceEnabled":true,
 "HttpPushUri":null,"MultipartHttpPushUri":"/redfish/v1/UpdateService/upload","HttpPushUriTargets":null,
 "FirmwareInventory":{"@odata.id":"/redfish/v1/UpdateService/FirmwareInventory"},
 "Actions":{"#UpdateService.SimpleUpdate":{"target":"/redfish/v1/UpdateService/Actions/SimpleUpdate"},
            "Oem":{"#UpdateService.UploadCABundle":{…}}}}
```
- `HttpPushUri` is **null**; firmware push is via **`MultipartHttpPushUri = /redfish/v1/UpdateService/upload`**.
- `SimpleUpdateActionInfo`: `ImageURI` (required), `TransferProtocol` allowable **`["HTTP","FTP","HTTPS"]`**,
  optional `User`/`Password`.
- `FirmwareInventory` members: **`BIOS`** and **`BMC`** (`BMC.Version "4.10.00"`, `Updateable:true`).

### 3.8 EventService — SSE and subscriptions

```json
{"@odata.type":"#EventService.v1_5_0.EventService","ServiceEnabled":true,
 "ServerSentEventUri":"/redfish/v1/EventService/SSE",
 "DeliveryRetryAttempts":3,"DeliveryRetryIntervalSeconds":60,
 "EventFormatTypes":["Event","MetricReport"],
 "RegistryPrefixes":["EventLog","SyncAgent","HttpStatus","Security","Base","IPMI","Task"],
 "ResourceTypes":["EventService","Systems","AccountService","TelemetryService","Chassis","Managers","TaskService"],
 "SSEFilterPropertiesSupported":{"EventFormatType":true,"MessageId":true,"OriginResource":true,"RegistryPrefix":true,"ResourceType":true,"MetricReportDefinition":false,"SubordinateResources":false},
 "SMTP":{"ServiceEnabled":false,"Port":25,…},"Subscriptions":{"@odata.id":"/redfish/v1/EventService/Subscriptions"},
 "Actions":{"#EventService.SubmitTestEvent":{…}}}
```
**SSE exists** (`/redfish/v1/EventService/SSE`) — the X470 had no SSE. Push subscriptions
(`Subscriptions`, `EventDestination`) and `SubmitTestEvent` are available; `Subscriptions` was empty.
SMTP (and an OEM SecondarySMTP) are configured-but-disabled.

### 3.9 TaskService / TelemetryService

- `TaskService` (`#TaskService.v1_1_4`): `ServiceEnabled true`, `CompletedTaskOverWritePolicy "Oldest"`,
  `LifeCycleEventOnTaskStateChange true`. `Tasks` empty (no async op in flight).
- `TelemetryService` (`#TelemetryService.v1_2_1`): `MaxReports 5`, `MinCollectionInterval "PT10M"`,
  `SupportedCollectionFunctions ["Minimum","Average","Summation","Maximum"]`, with
  `MetricDefinitions`, `MetricReportDefinitions`, `MetricReports`, `Triggers`, `LogService`.
  **`MetricReports` and `MetricReportDefinitions` are empty** (no report configured) — telemetry is
  present and functional but unprovisioned out of the box. The X470 only linked TelemetryService with
  no schema; here it is a full implementation.

### 3.10 Actions enumerated (not invoked)

34 resources carry Actions. Allowable values, where the server advertises them:

| Resource | Action | Allowable |
|---|---|---|
| `Systems/Self` | `#ComputerSystem.Reset` | ResetType: ForceOff, ForceRestart, GracefulShutdown, On, PowerCycle |
| `Chassis/Self` | `#Chassis.Reset` | ResetType: ForceOff, ForceRestart, GracefulShutdown, On, PowerCycle |
| `Managers/Self` | `#Manager.Reset` | ResetType: **ForceRestart** only |
| `Managers/Self` | Oem `#AMIManager.{BackupConfig,RestoreConfig,RedfishPowerSaveMode}`, `#AMIVirtualMedia.{ConfigureCDInstance,EnableRMedia}` | (ActionInfo descriptors) |
| `UpdateService` | `#UpdateService.SimpleUpdate` | TransferProtocol: HTTP, FTP, HTTPS |
| `UpdateService` | Oem `#UpdateService.UploadCABundle` | — |
| `Systems/Self/Bios` | `#Bios.ChangePassword`, `#Bios.ResetBios` | — |
| `Systems/Self/SecureBoot` | `#SecureBoot.ResetKeys` | (ActionInfo) |
| `Systems/Self/SecureBoot/SecureBootDatabases/{PK,KEK,db,dbx,dbt,dbr,…+Default}` | `#SecureBootDatabase.ResetKeys` | ResetKeysType: **ResetAllKeysToDefault, DeleteAllKeys** |
| `Systems/Self/Memory/DevType2_DIMM{0..3}` | `#AmiBios.ChangeState` | (ActionInfo) |
| `CertificateService` | `#CertificateService.{GenerateCSR,ReplaceCertificate}` | (ActionInfo) |
| `Managers/Self/NetworkProtocol/HTTPS/Certificates/1`, `Systems/Self/Boot/Certificates/1` | `#Certificate.{Rekey,Renew}` | (ActionInfo) |
| `EventService` | `#EventService.SubmitTestEvent` | (ActionInfo) |
| `TelemetryService` | `#TelemetryService.SubmitTestMetricReport` | (ActionInfo) |
| `*/LogServices/{SEL,BIOS,SMI,Logs,TelemetryService/LogService}` | `#LogService.ClearLog` | — |

The complete per-URI list (with targets and ActionInfo links) is in
`docs/b650/live-redfish-inventory.json`. **None were invoked.**

---

## 4. Read-only behaviour tests

| Test | Result | Verdict |
|---|---|---|
| `$select=Id` on `Chassis/Self` | returned **only `Id`** (plus `@odata.*`) | **$select WORKS** |
| `$expand=.($levels=1)` on `AccountService/Accounts` | member bodies **inlined** (UserName, RoleId, …); 1946 B vs 535 B plain | **$expand WORKS** |
| `$top=3` on `Sensors`; `$top=2` on `SEL/Entries` (11) | returned **3**, and **2** | **$top WORKS** |
| `$skip=50` on `Sensors` (60, page size 50) | returned the **remaining 10** (ids 51…9), nextLink **null** | **$skip WORKS; nextLink terminates; all 60 reachable** |
| Large-collection pagination | pages at **50**, `Members@odata.nextLink=?$skip=50`; the escaped follow-up returns the rest and a null nextLink | **works correctly** (page size 50) |
| ⚠️ Correction | The earlier rows claiming these were no-ops / an infinite loop were a **probe bug**: unescaped `$` in the shell meant the params were never sent. Re-verified from the main session with proper escaping. | query options + paging are functional |
| Unknown query param `$foobar=1` | **200**, full body | unknown params silently ignored (the X470 returned **501 QueryNotSupported**) |
| `GET /redfish/v1/NoSuchThing12345` (authenticated) | **404** `Base.1.12.ResourceMissingAtURI` (`@Message.ExtendedInfo`, `Message.v1_0_8`) | genuine 404s (newer registry `Base.1.12`; X470 was `Base.1.0` and masked 404 as 401) |
| Unauthenticated sub-resource | **401** `Security.1.x AccessDenied`, `WWW-Authenticate: Basic` + `X-Auth-Token` | auth enforced |
| `GlobalLuaTables` unauthenticated | **200** (lists Configurations members) | **must be blocked by the proxy** (same leak as X470) |
| `OPTIONS /redfish/v1/` | **501** (JSON, `Content-Type: application/json`) | differs from X470's 405 text/html |
| `/redfish` , `/redfish/v1` (no trailing slash) | both **200** | X470 302-redirected `/redfish/v1` → `/redfish/v1/` |
| ETag granularity | **48 distinct ETag values** across 677 `200` resources; the dominant `"1793662187"` (config/boot epoch) covers 569, and **108 resources carry a different ETag** that tracks the subtree's own LastModified epoch (e.g. cert/account subtrees `"1641013241"`, `"1744820014/15"`) | **per-resource-group ETags**, not one service-wide ETag (the X470 reused a single ETag everywhere). Still coarse (epoch-based, not a content hash) |

> **Net (corrected):** `$select`, `$top`, `$skip`, `$expand` and `nextLink` all work on the live 4.10
> firmware. The proxy MAY forward them, and MAY follow `nextLink` (it terminates), though a defensive
> loop/depth cap is still wise. It must not rely on an unknown query param being rejected (returns 200,
> silently ignored). No need to fall back to `/api` for large collections — the earlier claim that
> Sensors paging was broken was a probe-side `$`-escaping bug, since disproven.

---

## 5. Legacy `/api` dialect confirmation (live 4.10.00)

One `/api` session was created (`POST /api/session`), used read-only, and **deleted at the end**
(`DELETE /api/session` → `200 {"ok":0}`; replay → 401). Basic auth does **not** work for `/api`
(`/api/asrr/fw-info` with Basic → 401), so `/api` needs the session; Redfish does not.

**Login** (`POST /api/session`, form `username`/`password`, no `certlogin`): 200. Body adds, vs the
X470, the fields the [HAR §0.3] predicted and more: `user_id`, `channel`, `passwordStatus`,
`TFAEnabled (0)`, `TFAStatus (0)` alongside `ok, privilege(4), extendedpriv(259), racsession_id,
remote_addr(10.7.12.5), server_addr, HTTPSEnabled(1), CSRFToken (8 chars)`.
**Cookie: `QSESSIONID=…; path=/; secure; HttpOnly; Samesite=Lax`** — a real security improvement over
the X470's bare `path=/` cookie [X470 §5.1].

| Endpoint | Status | Notes / dialect |
|---|---|---|
| `/api/asrr/fw-info`, `/api/firmware-info`, `/api/asrr/model-name` | 200 | firmware header above |
| `/api/chassis-status` | 200 | `{"power_status":0,"led_status":0,"power_cycle_interval":2000,"power_on_after_flash":0}` |
| `/api/sensors`, `/api/detail_sensors_readings` | 200 | **60 sensors**. `settable_flag` (was `settable_readable_threshMask` on X470 — confirms [HAR §6]); unset thresholds are the string **`"NA"`**; whole values are 17-sig-digit floats (`3.3200000000000003`); **`unit` is `"V"`/`"°C"`/`"RPM"`/`"W"`/`"A"`/`"unknown"`** — the live unit reports **`"°C"`**, whereas the [HAR] B650 reported `"deg_c"` (a version change) |
| `/api/sdr` | 200 | 6985 B |
| `/api/settings/services` | 200 | `web time_out 1800`, `maximum_sessions 148`, `active_session 129` (≈1 over the 128 baseline = us); disabled ports encoded as **`non_secure_port:-1`** (cleaner than the X470's bit-31 `-2147…`); **new `vnc` service, secure_port 5901**; `singleport_status 1` on every service |
| `/api/settings/network` | 200 | eth0 static `10.7.12.55`/`10.7.12.1`; eth1 DHCP; MACs/foreign DNS redacted |
| `/api/settings/date-time` | 200 | `utc_minutes -300` (= −05:00), `timezone "GMT-05"`; **new PTP fields** (`ptp_*`) not on the X470; `ntp_auto_date 2` (servers set, auto-sync not on) |
| `/api/settings/ssl/certificate-info` | 200 | `certificate_date "Wed Apr 16 11:08:55 2025\n"`, `private_key_date "Tue Jun  4 01:37:39 2024\n"` (literal `\n` quirk persists; key = firmware build date) |
| `/api/host_inventory/host_interface_*` (system/processor/memory/baseboard/power) | 200 | populated; `system_info` UUID embeds the BMC MAC (redacted); Model/Serial = `"To be filled by O.E.M."` placeholders |
| `/api/configuration/project` | 200 | 90 features incl. `REDFISH`, `REDFISH_HOSTINTERFACE`, `REDFISH_BIOS_WEB_STATIC_SUPPORT`, `SINGLE_PORT_APP` |
| `/api/configuration/runtime` | 200 | `[{"feature":"SINGLE_PORT_APP","enabled":-1}]` |
| `/api/extfeatures` | 200 | `{"REDFISH_BIOS_SETTING":1}` |
| `/api/settings/fan/mode` | 200 | **new path** (fan control moved under `settings/fan/*`): `platform "AMD_platform"`, `num_of_fan 16`, `min_duty 20`, `max_duty 100` |
| `/api/settings/sensor-threshold` | **200** | empty `{}` (was **404** on the X470 2.32) |
| `/api/settings/network-bond`, `/settings/media/adviser`, `/settings/users`, `/settings/dns-info`, `/settings/dashboardinfo`, `/status/uptime`, `/logs/audit`, `/logs/dashboardevent`, `/dashboardsensors`, `/fru` | 200 | all readable |
| `/api/asrr/selsummary` | **500** (after a ~30 s hang) | `{"error":"Error in get SEL summary","code":50012}` — slow backend error on this firmware (X470 returned 200) |
| `/api/settings/network/activelancfg` | **500** | `{"error":"Error getting active lan count","code":1432}` |
| `/api/smbiosdata` | **404** | host off / not populated |

**Not fetched deliberately:** `/api/logs/help-data` (crashes the backend and kills all sessions,
per [X470 §5.2] — never called), `/api/logs/eventlog` (full-SEL dump, costly), and every
download / session-launch / action endpoint (`sol/session`, `remote_control/*`, `maintenance/*`,
`*/kvm/*`, `logs/*-file`, `video-data`, etc.).

---

## 6. Live vs static (7.02/9.05) and vs X470

| Dimension | Live B650 4.10.00 | Static [FW] 7.02/9.05 | [HAR] 1.10.00 | [X470] live 2.32.00 |
|---|---|---|---|---|
| Redfish admin auth | **Basic works (200)** | enabled by default | n/a | **401 — admin rejected** |
| ServiceRoot / RedfishVersion | v1_7_0 / 1.15.1 | (newer AMI gen) | n/a | v1_3_1 / 1.5.0 |
| AMI RtpVersion | 13.05 | MDS 13.5 codebase | n/a | 1.5.b |
| Query $select/$top/$skip/$expand | advertised and **honored** (verified) | — | n/a | advertised; unknown param → 501 |
| Collection paging | pages at 50, **nextLink terminates** (verified) | — | n/a | not testable (401) |
| CertificateService / Sensors / Telemetry+SSE | **present & functional** | predicted present | n/a | absent / empty |
| TLS | 1.2+1.3, ECDHE, FS, cert→2035 | (defconfig) | n/a | 1.0–1.2, RSA-kex only, cert expired 2026 |
| Port 80 | **closed** | — | n/a | open → 301 |
| `/api` cookie | `secure; HttpOnly; SameSite=Lax` | — | bare | bare `path=/` |
| Sensor unit string | **`"°C"`** | — | `"deg_c"` (B650) | `0.0`/°C |
| Sensor threshold mask field | `settable_flag` | — | `settable_flag` | `settable_readable_threshMask` |
| Factory `Administrator` acct | **enabled** | seeded Administrator/HostAuto* | sysadmin enabled | admin account present |

**Redfish URI set vs `docs/redfish/b650-uri-inventory.json`** (301 concrete static patterns):

- **161 static patterns matched** a live URI. The ~140 "absent" are almost entirely **not GET-
  crawlable rather than missing**: POST `…/Actions/{Action}` targets, ActionInfo descriptors for
  empty collections, and instance members of collections that are empty on this unit
  (`VirtualMedia/{id}`, `Subscriptions/{id}`, `*/Certificates/{id}`). A few are genuinely not present
  on 4.10: **Fabrics/** (no `Fabrics` link in the root), `Managers/Self/IKVM`, `Managers/Self/KCS`,
  `Chassis/Self/Oem/Ami/MapoutPorting`, and `Oem/Ami/BiosStaticFiles/*` (returns 404).
- **No genuinely new top-level resources** appeared. The 60 "added" live URIs are JSON-pointer
  fragments inside `Power`/`Thermal` arrays (`…/Power#/Voltages/0`) and the server's own pagination
  links (`?$skip=50`). The deprecated `Systems/Self/Storage/StorageUnit_0/StorageControllers/{0..6}`
  path returns **404** (controllers now live under `…/Controllers`).
- Conclusion: the 7.02/9.05 static inventory **predicts the live 4.10 structure well**; the live unit
  is effectively a **populated subset** — single `Self` instances for System/Chassis/Manager, empty
  VirtualMedia/Subscriptions/Certificates, no Fabrics/NetworkAdapters populated with the host off.

---

## Not called (actions / writes)

No POST/PUT/PATCH/DELETE to Redfish, no Redfish Action, no same-value writes. Enumerated-but-not-
invoked (targets and ActionInfo links in the inventory JSON):

- **Power/reset:** `Systems/Self #ComputerSystem.Reset`, `Chassis/Self #Chassis.Reset`,
  `Managers/Self #Manager.Reset`.
- **Manager OEM:** `AMIManager.BackupConfig`, `AMIManager.RestoreConfig`,
  `AMIManager.RedfishPowerSaveMode`, `AMIVirtualMedia.ConfigureCDInstance`, `AMIVirtualMedia.EnableRMedia`.
- **Update:** `UpdateService.SimpleUpdate`, Oem `UpdateService.UploadCABundle`, and the
  `MultipartHttpPushUri` target `/redfish/v1/UpdateService/upload` (skipped — upload path).
- **BIOS / SecureBoot:** `Bios.ChangePassword`, `Bios.ResetBios`, `SecureBoot.ResetKeys`,
  `SecureBootDatabase.ResetKeys` on PK/KEK/db/dbx/dbt/dbr(+Default) [allowable ResetAllKeysToDefault,
  DeleteAllKeys].
- **Certificates:** `CertificateService.GenerateCSR` / `ReplaceCertificate`; `Certificate.Rekey` /
  `Renew` on the HTTPS and Boot certs.
- **Memory:** `AmiBios.ChangeState` on each DIMM.
- **Logs/telemetry/event:** `LogService.ClearLog` (SEL, BIOS, SMI, Chassis `Logs`, Telemetry
  LogService), `EventService.SubmitTestEvent`, `TelemetryService.SubmitTestMetricReport`,
  `EventDestination.ResumeSubscription`.
- **SSE stream** `/redfish/v1/EventService/SSE` — not called (would open a long-lived stream).
- **Legacy `/api`** action/maintenance/download/session-launch routes — not called (see §5).

## Needs a write to confirm (next, with approval)

1. **Redfish `PATCH`** round-trips the proxy will rely on, each Lit→Off / set→restore and with/
   without `If-Match`, to learn ETag granularity and 412/428 behaviour: `Chassis/Self.IndicatorLED`
   (allowable Lit/Blinking/Off), `Systems/Self.Boot.BootSourceOverride*`, an `EthernetInterface`.
2. **`ComputerSystem.Reset`** (`On`/`GracefulShutdown`) — needs a host power-on window; currently off.
3. **One Redfish session** via `POST /redfish/v1/SessionService/Sessions` to measure `X-Auth-Token`
   length, `Location`, and whether the 30 s `SessionTimeout` is wall-clock or idle — then DELETE it.
   (Not done this probe: Basic auth avoided creating sessions.)
4. **EventService subscription** (`POST /Subscriptions`) and an **SSE** connect, to confirm delivery
   shape and whether MetricReport events require a provisioned `MetricReportDefinition`.
5. **TelemetryService**: create a `MetricReportDefinition` to see whether reports populate (both
   report collections are empty out of the box).
6. ~~Confirm whether 50-member pagination can be defeated~~ — RESOLVED: `$skip`/`$top`/`$select`/
   `$expand` and `nextLink` all work; the earlier "no-op/loop" finding was a probe `$`-escaping bug.
7. **Rotate the shared TLS cert** (`CertificateService.GenerateCSR`/`ReplaceCertificate` or
   `/api` upload) and **disable or re-secure the factory `Administrator` account** — both change state.

## Method and hygiene

- All requests sequential, keep-alive, paced (curl `--rate 300–360/m`, ≈ one every 150–200 ms). No
  `403` connection-cap bursts were seen. Redfish used **HTTP Basic only** — **zero** server-side
  Redfish sessions created.
- Exactly **one** `/api` session was created and was **deleted** at the end (verified invalid on replay).
- The TLS cipher enum (`nmap`) was run only after the crawl finished, to avoid competing for the
  ~10-connection cap.
- Scripts and raw (unredacted) dumps: probe scratchpad `…/scratchpad/b650probe/` (`crawl.py`,
  `hdrpass.py`, `capture.py`, `api_sweep.py`, `compare.py`, `make_inventory.py`, `rf_body/`, `key/`,
  `api/`, `tls_enum.txt`). These contain MACs, UUIDs and the host interface account name and are not
  copied into `docs/`. The session cookie and CSRF token were shredded after logout.
