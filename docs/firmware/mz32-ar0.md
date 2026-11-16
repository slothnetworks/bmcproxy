# Gigabyte MZ32-AR0 BMC firmware: stack assessment

Status: preliminary. I derived everything here from the static image. I read config files, init scripts, the web UI JS and
LuaJIT bytecode string tables, and ran `strings`/`file` on binaries. Nothing was disassembled or decompiled. The live
behaviour of a unit running this exact build is in `docs/gigabyte/mz32-ar0-live-probe.md`, and the frontend analysis is
in `docs/gigabyte/mz32-ar0-frontend.md`.

## 0. Sources

| Alias | Path |
|---|---|
| `IMG` | `/opt/sparked/asrockrack-api/bmc-firmware/MZ32-AR0-00.bin` (66,060,552 B, sha256 `43817b0f…42f5a9`) |
| `D` | `/opt/sparked/asrockrack-api/firmware-dumped/MZ32-AR0-00/` (`raw/` carved FMH sections, `rootfs/` cramfs root, `www/` cramfs www as shipped (gzip-encoded files), `www_plain/` the same tree with the 84 gzip files decompressed, `conf/` and `dre/` JFFS2 extracts) |
| `MC13` | `/opt/sparked/asrockrack-api/firmware-dumped/MC13-LE0-000/` (the other agent's extraction of `bmc-firmware/MC13-LE0-000.bin`) |

How it was extracted: `docs/firmware/scripts/fmh.py` lists the AMI FMH headers. Each section was carved with `dd` at
its FMH `mloc`/`msize`. The cramfs root and www were unpacked with `7z x` (7z won't follow the 6 `../../../var/...`
symlinks in www; those are runtime log/capture mounts). `conf`/`dre` were unpacked with `docs/firmware/scripts/jffs2x.py`.

**Live vs image version:** the live unit (2026-11-16) reports `Managers/Self.FirmwareVersion` **`12.61.39`**, which
matches this image's `FW_VERSION=12.61.39`. The live UI bundle is **byte-identical** to the image: `source.min.js`,
`styles.min.css`, `viewer.html` and `index.html` all have the same sha256 as `D/www_plain`. So, unlike every ASRock
board so far, **the image and the live unit run the same build**. See the live probe for details.

---

## 1. Identity

| Item | Value | Evidence |
|---|---|---|
| Firmware info module (`ast2500e`, @0x3EF0040) | `FW_VERSION=12.61.39`, `FW_DATE=Jul 2 2025`, `FW_BUILDTIME=11:14:05 CST`, `FW_DESC=WolfPass 32MB v12-update-2.03`, `FW_PRODUCTID=1`, `FW_RELEASEID=RR9`, `FW_CODEBASEVERSION=3.X` | `D/raw/fwinfo.bin` |
| SoC | **ASPEED AST2500** (ARM11, ARMv6) | FMH module name `ast2500e`; PRJ `CONFIG_SPX_FEATURE_GLOBAL_^AST2500^_MAX_ADC_CHANNELS=16`; U-Boot `basesoc_ast` |
| BMC stack | **AMI MegaRAC SP-X, 12.x codebase** ("LTS 12"; the 3.X codebase). The project is AMI's Intel **WolfPass** reference platform (`info/wolfpass.PRJ`), rebranded by Gigabyte for this AMD EPYC board | `D/rootfs/info/wolfpass.PRJ` ("SPX version: 4.0", generated `Wed, 26 Jun 2024 10:30:14 +0800`); `BuildScript.Ver 3.17.0`; build host path `/project/mike/2025/0702FW/V12.2_20191206/` (an AMI **12.2 drop of 2019-12-06** rebuilt in July 2025) |
| Kernel | `Linux-3.14.17-ami`, legacy uImage (uncompressed), built `Wed Jul 2 02:57:44 2025` | `file D/raw/kernel.uimage` |
| Userland | Debian 8 "jessie" base (`etc/target.ver` `Debian-8.0.0`), glibc 2.19, OpenSSL 1.0.x (`libssl.so.1.0.0`), Python 2.7 | PRJ `CROSSCC_JESSIE`, `SUPPORTS_PYTHON_2_7` |
| Web server | lighttpd + mod_fastcgi (`/api` → `spx_restservice`, `/ext` → `spx_restservice_ext`), mod_proxy (`/redfish` → 127.0.0.1:9080, SSE → :9090), mod_websocket (KVM), mod_evasive, mod_singleport | `D/rootfs/etc/defconfig/lighttpd*.conf`, `redfish_lighttpd.conf` |
| Redfish | AMI Redfish server (LuaJIT + turbo + redis), `Redfish:Ami:RtpVersion 1.8.0`; live `RedfishVersion 1.8.0`, `ServiceRoot.v1_5_2` | `D/rootfs/usr/local/redfish/db_init/*.rcmd` |
| Flash | PRJ: 2 banks × 64 MiB SPI (`FLASH_SIZE=0x8000000`, `FLASH_BANKS=2`, `GLOBAL_DUAL_IMAGE_SUPPORT=YES`), image uses `USED_FLASH_SIZE=0x3F00000` | PRJ |
| Image signing | `SIGNED_HASHED_IMAGE_SUPPORT=YES`; the 0x108-byte trailer after 0x3F00000 is the signature block (not verified) | PRJ, `D/raw/trailer.bin` |

### 1.1 FMH layout (FMH v1.8, section version 12.01)

| FMH @ | Name | Ver | Type | Alloc | Data @ | Size | Content |
|---|---|---|---|---|---|---|---|
| 0x0003FF00 | `boot` | 12.01 | 0x0001 | 0x50000 | 0x0 | 0x3B488 | U-Boot (DENX) |
| 0x00050000 | `conf` | 12.01 | 0x0011 | 0x200000 | 0x60000 | 0x1F0000 | JFFS2 `/conf` (factory seed) |
| 0x00250000 | `conf` (bkupconf) | 12.01 | 0x0011 | 0x200000 | 0x260000 | 0x1F0000 | JFFS2 backup conf |
| 0x00450000 | `ec` | 1.25 | 0x0011 | 0x100000 | 0x460000 | 0 (msize 0) | Gigabyte EC/CPLD blob slot (opaque, 0xF0000 carved) |
| 0x00550000 | `root` | 12.01 | 0x0012 | 0x2740000 | 0x560000 | 0x2728000 | **cramfs** root (6,619 files) |
| 0x02C90000 | `osimage` | 12.01 | 0x0006 | 0x2B0000 | 0x2C90040 | 0x2A9CC0 | uImage kernel |
| 0x02F40000 | `dre` | 12.01 | 0x0011 | 0x80000 | 0x2F50000 | 0x70000 | JFFS2: Redfish BIOS-config "DRE" store (`/dre`, `redfish_subextensions`) |
| 0x02FC0000 | `www` | 12.01 | 0x0011 | 0x5D0000 | 0x2FD0000 | 0x5B2000 | **cramfs** `/usr/local/www` (131 files, gzip-encoded) |
| 0x03590000 | `testapps` | 2.04 | 0x0005 | 0x30000 | 0x3590040 | 0x21FA0 | ARM test binary |
| 0x03EF0000 | `ast2500e` | 12.61 | 0x0002 | 0x10000 | 0x3EF0040 | 0x9E | firmware-info text |

**Why the file size matches MC13-LE0 exactly:** both Gigabyte images are `0x3F00000` bytes of used flash plus the
same 0x108-byte signature trailer. That's Gigabyte's packaging convention. **It does not mean they share a platform.**
They have different SoCs, AMI generations and filesystem formats (§7).

---

## 2. Web / API handler inventory (legacy AMI `/api`)

The UI is the classic **AMI MegaRAC SP-X HTML5 UI** (RequireJS/almond + jQuery + Backbone; `<title>MegaRAC SP-X</title>`).
It's the same framework the ASRock X470/B650 builds use, so `docs/api/legacy-api-catalog.md` describes the conventions
(Backbone singletons → `PUT` whole model, `X-CSRFTOKEN`, `QSESSIONID`). Full route list and login flow:
`docs/gigabyte/mz32-ar0-frontend.md`.

- `spx_restservice` (fastcgi, `max-procs 1`, so **single-threaded**: one slow handler blocks the whole `/api`) plus
  plugins `libspx_restservice_{hostinterface,nvme,raid,sasit,sol}.so`, plus Gigabyte's `libgbtrest.so`.
- `strings` of those binaries yields **329 distinct route strings** (276 in `spx_restservice`). The UI bundle references
  **272** `/api/...` literals, and 240 of those have a matching handler string. The 32 UI routes with no handler are
  dead UI code for features not built into this image: AEP/Optane (`aep_inventory/*`), Intel ACD crashdump
  (`acd/*`, `logs/acd`), PLDM firmware update, Microsemi PCIe switch, `signed-image-key`, `skuv2`, `oem/node_info`,
  `user-preference`, `raid_management/create_lu_cache`.
- **Gigabyte-specific `/api` handlers** (implemented in `spx_restservice` as `getSysInv*_gbt_*`, `getGSM_*`,
  `getBIOSSetup_gbt_*`):
  `/api/system_inventory_gbt/{bios_info, cpld_info, cpu_info, dimm_info, dimm_info_ex, fpga_info, gpu_info, hdd_info,
  nic_info, nvswitch_info, pci_info, scp_info, smbpbi, cmc_ip, iptable, ip6table, smbios-file, bios-setup-file,
  bios-setup-file/upload}`, `/api/GSM`, `/api/MINIDP-SUPPORT-status`, `/api/bios-setupdata-status`,
  `/api/powerControl/{getNodeInfo,SetNodeStatus}`, `/api/hsc-info`, `/api/save/{bios,bmc}brokenimage`.
- `libgbtrest.so` (`gbt_rest_service`, GET/POST method tables) carries `/GBT/GpuManager/NVAPI/...` (GPU/FPGA/NVSwitch
  telemetry from redis) and `/GBT/OEM/{BMC/System/Information, CMC/Chassis/Power/{PowerMetrics,PowerLimit,NodePowerControl}}`.
  The UI doesn't use these. They're meant for GPU servers and multi-node (CMC) chassis and are irrelevant on MZ32-AR0.
- The old AMI generic IPMI inventory (`/system_inventory/{cpu_info,dimm_info,pci_info,device_count}`) exists alongside
  the Gigabyte `system_inventory_gbt/*` variants.

### 2.1 Deny list (crash-prone, destructive or side-effecting GETs: never call)

Based on AMI SP-X behaviour seen on the ASRock boards (`docs/x470/live-probe.md`, `docs/b650/live-probe.md`) plus the
handler names in this image. None of these were called during the live probe.

| Route (under `/api`) | Why |
|---|---|
| `logs/help-data` | handler present. On X470 the GET crashed `spx_restservice` and dropped **every** session |
| `settings/download_image`, `maintenance/download_config`, `maintenance/backup_config` | config/image exfil, builds archives, heavy |
| `maintenance/firmware/*` (incl. `dump`, `dwldfwimg`, `verification`, `upgrade`, `flash-progress`), `maintenance/flash`, `maintenance/flashtrack`, `maintenance/fwupdate_*`, `maintenance/hpm/*` (esp. `upgradestatus`, `verifyimagestatus`, `freemem`, `updatemode`), `maintenance/nm_*`, `maintenance/dual_image_config`, `maintenance/bmc_recovery`, `maintenance/reset`, `maintenance/restore_*`, `maintenance/upload_restore`, `maintenance/preserve`, `maintenance/system_admin`, `maintenance/clear_model_patch`, `maintenance/sku$`, `maintenance/skuv2$`, `maintenance/redfish`, `maintenance/firmware/redfish` | firmware-update state machine. Several GETs put the BMC into update mode or 502 on ASRock |
| `save/biosbrokenimage`, `save/bmcbrokenimage` | dumps flash to `/var/tmp/BrokenBmc.bin` (named "save", side-effecting) |
| `system_inventory_gbt/smbios-file`, `system_inventory_gbt/bios-setup-file` | file download of SMBIOS / BIOS setup blobs (the latter has an `upload` twin) |
| `logs/*-file`, `logs/video-data`, `logs/video-log/delete`, `logs/acd-file`, `extlog/archive/` | file downloads / delete |
| `ae/run`, `ae_test`, `ae/results`, `ipmi_response` | Automation Engine script execution / raw IPMI |
| `sample/{get,post,put,delete}` | AMI sample handlers compiled into production: unknown side effects |
| `kvm/token`, `remote_control/get/kvm/launch` | mints KVM tokens / JNLP |
| `settings/dns/restart`, `settings/pef/send_test_alert`, `operations/ssl/certificate/generate`, `actions/*`, `powerControl/SetNodeStatus`, `settings/media/*/start-media`, `stop-media` | actions behind a URL. The UI uses POST/PUT, but don't risk a GET |
| `/GBT/OEM/CMC/Chassis/Power/*` | node power / power-limit (POST table) |
| Redfish: `/redfish/v1/EventService/SSE` (long-lived stream on :9090), anything under `/Actions/`, `UpdateService` upload targets, `Bios/BiosStaticFiles/*`, `Systems/Self/LogServices/*/Entries/*/attachment`-style downloads | not GET-safe or not resources |

---

## 3. Redfish implementation

- Lua tree under `D/rootfs/usr/local/redfish` (444 `.lua` files, all **LuaJIT bytecode** `\x1bLJ`). It runs as
  `redfish-server` on 127.0.0.1:9080 behind lighttpd, with the data model in redis (`db_init/*.rcmd`).
  `Server: AMI MegaRAC Redfish Service`.
- `RtpVersion 1.8.0` (`db_init` `SET Redfish:Ami:RtpVersion 1.8.0`). For comparison: X470 2.32 = `1.5.b`,
  B650 4.10 = `13.05`, MC13-LE0 13.06.30 = `13.06`.
- OEM namespaces: `oem/ami` (AMI: Configurations, InventoryData, BiosStaticFiles, `ami_hi` host interface), `oem/ipmi`,
  and **`oem/gbt`** (Gigabyte):
  - `GpuManager/NVAPI` (GPU slot/information/status/temperature/power/clock/memory; `GbtGPUManager*.v1_0_0` schemas)
  - `Oem/Ami/InventoryDataInfo`
  - a Gigabyte `Sensors` collection/instance override (`GbtSensorProperty.v1_0_0`) and `GBTThermal.v1_0_0` on `Chassis/{id}/Thermal`
  - `FanprofileService` (+ `Fanprofile`, `SupportPCIEDevice`; `GBTFanprofile*.v1_0_0`)
  - `Systems/{id}/Oem/.../SetPowerFloorInfo` / `SetPowerFloorTarget` actions (`powerfloor-actioninfo`)
  - `Systems/{id}/Storage/RaidConfig/{Create,Delete}LogicDeviceInfo` and `RaidConfig/Actions/{Create,Delete}LogicDevice`
    (these ActionInfo URIs return **404** live with no RAID card present)
  - `GBTSystemsOemProperty.v1_0_0` on `Systems/{id}`
- **Apply-time / async semantics** (from the bytecode string tables):
  - `system-actions.lua` accepts **`@Redfish.OperationApplyTime`** on `ComputerSystem.Reset` with values
    `Immediate` / `AtMaintenanceWindowStart` (+ `MaintenanceWindowStartTime`, `MaintenanceWindowDurationInSeconds`). The
    deferred variant creates a TaskService task ("Maintenance Window Task", `post_maintenance_window_task.lua`). Log
    services' `ClearLog` accept it too (`*/log-service-actions.lua`).
  - BIOS attributes: the firmware has the pending-settings machinery (`redfish/system/bios.lua` references `/SD`, and
    the `redfish-settings` init script runs `run_settings.lua Boot` / `Shutdown`, handing pending settings to the BIOS
    across a host reset; the `dre` JFFS2 partition holds the BIOS-config store). **Live, though,** `Systems/Self/Bios`
    is `Allow: GET` only, and its `@Redfish.Settings` is a stub: `{"ETag":"Dummyetag","Messages":[{"MessageId":
    "Base.1.0.SettingsFailed"}]}` with **no `SettingsObject` link**. So BIOS-attribute writes over Redfish are
    effectively not offered on this unit (or need the BIOS to publish its attribute set / SD first).
  - `config.lua` defines `PATCH_TIMEOUT`, `SESSION_TIMEOUT`, `MAX_SESSIONS` and `DEFAULT_COLLECTION_LIMIT`. Live:
    `SessionTimeout 30` s (schema minimum, same as B650).
- `CONFIG_SPX_FEATURE_REDFISH_PBKDF2_PASSWORD_HASH=YES`, `REDFISH_UPNP_ENABLED=YES` (SSDP advertisement).

---

## 4. Account store and auth model

- Local accounts are the IPMI user table (`/conf/BMC1/wolfpass/IPMI.conf`), which Redfish AccountService, `/api`,
  SSH and IPMI-over-LAN share. PAM stack: `pam_withunix`/`pam_wounix`, with LDAP, AD and RADIUS options
  (`pam_ldap.conf`, `pam_radius_auth.conf`, `activedir.conf`).
- **`/conf/shadow` ships a uid-0 `sysadmin` account with a traditional DES crypt hash** (13 chars), and
  `/conf/passwd` gives it `sysadmin:x:0:0:/root:/usr/local/bin/defshell`. This is AMI's well-known factory
  `sysadmin` shell account. Unless an operator changes it, it's a fixed credential shared by every unit on this
  firmware, and SSH is enabled by default (`ncml.conf [ssh] current_state=1, port 22`). I didn't test it (the live
  probe found TCP/22 filtered from the probe host). **Security finding.**
- Web `/api` login: `POST /api/session` (form `username`, `password`) → `QSESSIONID` cookie + `CSRFToken` in the body,
  echoed as `X-CSRFTOKEN`. Web session `session_timeout=1800`, `max_sessions=148` (`ncml.conf [web]`).
- Redfish: Basic **and** `X-Auth-Token` (`WWW-Authenticate: Basic realm="AMI_Redfish_Server", X-Auth-Token realm=...`).
- IPMI: v1.5 + v2.0, cipher suites 0,1,2,3,6,7,8,11,12,15,16,17 (live, sessionless query). **Suite 0 is advertised.**
- Default service ports (`/conf/ncml.conf`): web 80/443, KVM 7578/7582, CD-media 5120/5124, HD-media 5123/5127, SSH 22.
  There's a telnetd init script, but `ncml.conf` has no `[telnet]` section, so it doesn't start by default.

## 5. TLS configuration

- `ssl.pemfile=/conf/server.pem` is the **shared AMI default certificate**: `CN=www.ami.com, O=American Megatrends
  Incorporated, OU=Service Processors, L=Norcross, ST=Georgia, C=US, emailAddress=support@ami.com`, RSA-2048,
  sha256WithRSA, valid **2018-04-23 → 2037-06-22**, self-signed. Every unit shares it and the private key ships in the
  image, so pinning it proves nothing.
- `ssl.use-sslv2/sslv3 = disable`, `ssl.ec-curve=secp384r1`, `ssl.dh-file=/conf/certs/dhparam.pem` (4096-bit), and a
  cipher list that keeps ECDHE/DHE-GCM plus static-RSA `AES*-GCM`. Live: TLS 1.2 and 1.3 only. TLS 1.2 offers
  ECDHE/DHE/RSA AES-GCM and TLS 1.3 offers AES-GCM and CHACHA20. Go's defaults negotiate fine.
- Client-cert support: `ssl.ca-file=/conf/ca.pem`, `verifyclient.activate=enable`, `enforce=disable` (optional
  mTLS mapped to `SSL_CLIENT_S_DN_CN`).
- Security headers on static paths: HSTS (`max-age=31536000; includeSubDomains; preload`), CSP, `X-Frame-Options
  SAMEORIGIN`, `nosniff`. `/api` and `/redfish` don't get the frame/CSP set from lighttpd.

## 6. Notable / deny-worthy facts for the proxy

- `spx_restservice` runs as a single fastcgi process. Serialise `/api` per host, exactly like X470.
- `server.error-handler-404 = "/index.html"`: any unknown static path returns **200 + the SPA index**
  (`/manifest.json`, `/version.txt`, `/robots.txt` all do this live). Don't treat 200 as proof that a path exists.
- Redfish is proxied separately, so `/api` crashes don't take Redfish down, and the reverse also holds. Prefer Redfish.

---

## 7. Differences vs MC13-LE0 (`bmc-firmware/MC13-LE0-000.bin`)

| Axis | MZ32-AR0 (this image) | MC13-LE0 |
|---|---|---|
| FW version / date | **12.61.39**, Jul 2 2025 | **13.06.30**, Sep 4 2026 |
| FW_DESC / codebase | `WolfPass 32MB v12-update-2.03`, `FW_CODEBASEVERSION=3.X` | `AST2600_Channel`, `5.X` |
| AMI generation | MegaRAC SP-X **12.x** (12.2 base, 2019-12) | MegaRAC SP-X **13.x** (git `v13_ast2600`, tag `v13.06.30`, `LTS13-AST2600EVB-FULL1.PRJ`) |
| SoC | **AST2500** | **AST2600** |
| Kernel | 3.14.17-ami uImage | 5.4.251-ami, FIT image |
| Root / www FS | **cramfs** / **cramfs** (FMH type 0x0012/0x0011) | squashfs / squashfs (type 0x0060) |
| Extra sections | `ec`, `dre` | `ec` |
| Redfish RtpVersion | **1.8.0** (live RedfishVersion 1.8.0, `ServiceRoot.v1_5_2`) | **13.06** (ServiceRoot up to `v1_13_0` in db_init) |
| Redfish OEM dirs | `ami`, `ami_hi`, `gbt`, `ipmi` | `ami`, `ami_bios`, `ami_cper`, `ami_dot`, `ami_hi`, `gbt`, `ipmi` |
| `/api` handler strings | 329 | 572 |
| Only in MC13 `/api` | n/a | PLDM (`pldm/*`, `maintenance/pldm/*`), host diagnostics (`hostinterface/diag/*`, `system_diagnostics/*`), lockdown (`get/set-lockdown`), `get/set-redfish`, TFA/OTP (`tfa`, `verify_otp`), chassis-intrusion / leakage detection, `psu_info`, `cups_info`, NVIDIA HGX/Delta-Next GPU, `settings/users/password_policy`, `bios-post-code`, `health-status`, many more `raid_management/sl8/*` |
| Only in MZ32 `/api` | `system_inventory/*` (old AMI IPMI inventory), `system_inventory_gbt/{iptable,ip6table,smbpbi}`, NVMe-MI (`nvme_mi_management/*`, `nvme_management/*`), `settings/javasol`, `settings/web_server_instances`, `powerControl/*`, `keep_session`, `chassis/bootoption`, `dcmi/chassisPower` | n/a |
| UI | SP-X HTML5 (Backbone) | same framework, newer (adds `systemdiagnostics/`) |

**Implication for the Gigabyte driver:** MZ32-AR0 and MC13-LE0 share the vendor layer (Gigabyte `oem/gbt` Redfish
extensions, `system_inventory_gbt` `/api` handlers, `GSM`) but sit on **different AMI generations**. Some things
match: neither exposes `PowerSubsystem`/`ThermalSubsystem`, and both replace the DMTF `Sensors` collection with
Gigabyte's two-member `Energy`/`Power` override, so the legacy `Thermal`/`Power` resources are the sensor source on
both (`docs/gigabyte/mc13-le0-redfish-inventory.json`). The schema and registry generation differs: MZ32 has
`ServiceRoot.v1_5_2`, `ComputerSystem.v1_8_0`, `Manager.v1_6_0` and the `Base.1.5` registry, while MC13 has `v1_13_0`,
`v1_15_0`, `v1_13_0` and `Base.1.12`. On MZ32 `Systems/Self/Bios` is a readable 200, but on MC13 it returns 400
`ResourceAtUriInUnknownFormat`. The query/escaping behaviour also needs confirming per generation (live probe §5).
Key behaviour off `Manager.FirmwareVersion` major (12 vs 13) / `RtpVersion`, not off the vendor.
