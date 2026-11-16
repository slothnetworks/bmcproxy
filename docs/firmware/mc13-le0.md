# Gigabyte MC13-LE0 BMC firmware: stack assessment

Static analysis of the Gigabyte **MC13-LE0-000** (AM5, Ryzen 7000) BMC image, done to build the `gigabyte` driver
(`vendors/gigabyte`). The live behaviour (firmware **13.04.15**) is in `docs/gigabyte/mc13-le0-live-probe.md`. Where the
two disagree, the live probe wins. The driver keys its behaviour off the **live** `Managers/Self.FirmwareVersion`.

## 0. Sources and path notation

| Item | Value |
|---|---|
| Image | `bmc-firmware/MC13-LE0-000.bin`, 66,060,552 bytes = 0x3F00000 bytes of flash + a 0x108-byte signed trailer (magic `0xDEADBEEF`) |
| Extraction | `firmware-dumped/MC13-LE0-000/` (gitignored): `raw/` holds the FMH sections carved by offset; `rootfs/` and `www/` come from `unsquashfs`; `conf/` comes from `docs/firmware/scripts/jffs2x.py` |
| Notation | `R/` = `rootfs/`, `W/` = `www/`, `C/` = `conf/` |

## 1. Identity and flash layout

| Item | Value | Source |
|---|---|---|
| FW version | **13.06.30**, `FW_DATE=Sep 4 2026`, `FW_DESC=AST2600_Channel`, `RELEASEID RR9`, `CODEBASE 5.X` | `raw/fwinfo.bin` (FMH `ast2600e`) |
| AMI base | MegaRAC SP-X **13.x**: git repo `v13_ast2600`, commit `af50883`, tag `v13.06.30`, PRJ `LTS13-AST2600EVB-FULL1.PRJ`, `MDS_VERSION 13.8` | `R/etc/*release*`, PRJ strings |
| SoC | **ASPEED AST2600** | U-Boot, FIT, device tree |
| Bootloader / kernel | U-Boot 2019.04; FIT `LTS13 SPX/5.4.85-ami/ast2600` | `raw/boot.bin`, `raw/kernel.fit` |
| Gigabyte layer | `CONFIG_SPX_FEATURE_GBT_2600_SUPPORT`, `gbt_module`, `gbt_redfish_extension`; `libgbt*.so`, `libgbtrest.so`, `gbtfw`, `S10gbt-init.sh` | PRJ, `R/usr/local/lib` |
| IPMI identity | manufacturer IANA **15370** (0x3C0A, Giga-Byte), `prod_id 4213` | live `/api/firmware-info` |

### 1.1 FMH sections

| Section | FMH offset | Data | Notes |
|---|---|---|---|
| boot | 0x000000 | size 0xcff00 | U-Boot |
| conf | 0x110000 | 0x120000, 0x190000 | JFFS2 (`/conf`) |
| bkupconf | 0x2b0000 | | JFFS2 backup |
| ec | 0x450000 | | |
| root | 0x550000 | 0x560000, 0x2f10000 | squashfs rootfs |
| osimage | 0x3470000 | | FIT kernel + dtb |
| www | 0x3920000 | 0x3930000 | squashfs web UI |
| testapps | 0x3e50000 | | |
| ast2600e (fwinfo) | 0x3ef0000 | | version block |

## 2. Network services, lighttpd and TLS

- lighttpd serves the UI and `/api` (FastCGI `spx_restservice`). `/redfish` is reverse-proxied to `127.0.0.1:9080`
  (the LuaJIT Redfish server), and Redfish SSE goes to :9090. `/api` crashes and Redfish fail independently.
- Unlike the ASRock builds, `lighttpd_ext.conf` **is** included (`/ext` → `spx_restservice_ext`).
- `mod_evasive`: 30 connections per IP, and 10 for `/redfish`. The proxy must stay at 1–2 concurrent requests per host.
- HTTP/2 is disabled. `server.http-parseopts` sets `method-get-body` (GET bodies are accepted).
- TLS cipher list `"TLSv1.2 !SHA384 !SHA256 !AES128 ..."`. Live the server **also** accepts ECDHE-RSA-AES128-GCM, so the
  string is not applied literally. Live: TLS 1.2 and 1.3 only, ECDHE/AEAD only, the AMI default certificate (CN=ami.com,
  issuer megarac.com, 2022-01-01 → 2031-12-30). Go's default `crypto/tls` works (with InsecureSkipVerify).
- HSTS on the web root.

## 3. Redfish implementation

- LuaJIT bytecode tree in `R/usr/local/redfish`, with the data model in redis. RtpVersion is 13.06 in the image and
  **13.04 live**. Live ServiceRoot is `v1_13_0`, RedfishVersion `1.11.0`, registry `Base.1.12`.
- `db_init` sessionservice: `SessionTimeout 30` (seconds). The driver uses **Basic auth**, which creates no session.
- **Gigabyte OEM** (`R/usr/local/redfish/oem/gbt`) defines: FanprofileService/Fanprofile/SupportPCIEDevice/FanMode,
  PoweronDelay, FrontPanelSettings, Firewall/PortRules/IPRules, GPUManager (NVAPI), RAID, BiosLogo, SearchMac, the Gct
  license and more. **Live, only FanprofileService, Fanprofile and SupportPCIEDevice exist.** FanMode, PoweronDelay,
  FrontPanelSettings, the Gct Firewall and RetimerInfo all return 404 on 13.04.15. The `$metadata` (unauthenticated,
  115 KB) declares the `GBT*`/`Gbt*` namespaces, and the driver detects on that.
- Chassis `Sensors` is replaced by a Gigabyte two-member override (Energy, Power). Temperatures, fans and voltages only
  exist in the legacy `Thermal`/`Power` (shared with MZ32-AR0).
- **Async writes:** a NetworkProtocol PATCH goes through `task-service/lua_script/patch-networkprotcol.lua`, which
  answers 202 with a Task and applies the change later (the same pattern as B650). The generic driver waits for the
  task and then verifies the read-back (`verifyNTP`). This is assumed from the firmware and was not exercised live.

## 4. Legacy `/api` inventory

`spx_restservice` registers **362** multi-segment route strings, and the UI bundle references **401** `/api` paths.
The ones specific to Gigabyte / AMI 13.x:

| Area | Routes | Proxy use |
|---|---|---|
| Fan profiles | `settings/fanprofile`, `/collection`, `/mode`, `/device_define`, `/device_define/collection` (separate Get/Post/Put/Delete handlers, e.g. `FanProFileModeGet`, `FanProFileModePost_func`) | `fanMode` reads it (GET `/mode`). `setFanMode` writes it (POST `{"strMode": name}`, taken from the UI JS: **UNVERIFIED**) |
| GPU manager | `GBT/GpuManager/NVAPI/{GPU/Slot,FPGA/Information,FPGA/Temperature,NVSwitch/Temperature}` | none (raw/api) |
| Host diagnostics | `hostinterface/diag/*` (about 70 routes: start-test, abort, cpu/memory/pcie/psu/HDD/hgx diagnose) | **deny**: these run tests on the host |
| Actions | `actions/power`, `actions/bmc/reset`, `actions/chassis-led*`, `actions/chassis-case*`, `actions/button-enable`, `actions/leakage-detection-status`, `actions/set-leakage-detection` | denied in raw/api (actions behind URLs). Redfish covers power, LED and reset |
| Misc | `settings/one-touch-settings[/n]`, `status/power_on_time`, `maintenance/getPlatformID`, `pldm/sensors`, `pldm/state_sensors`, `save/biosbrokenimage`, `save/bmcbrokenimage` | `save/*` is denied |

## 5. Deny list (applied by the driver)

These come on top of `amimegarac.DefaultDenyRules`, which already covers `logs/help-data`, `download_image`,
`download_config`, `hpm`, firmware/flash/restore/reset/factory and `GlobalLuaTables`. The rules are in
`vendors/gigabyte/boards.CommonDeny`.

| Pattern | Why |
|---|---|
| `hostinterface/diag` | runs host diagnostics |
| `logs/audit-file`, `logs/system-file`, `logs/event-file` | FastCGI file downloads |
| `maintenance/firmware/dump`, `maintenance/bios_presv_update_log/dump` | image/log dumps |
| `save/biosbrokenimage`, `save/bmcbrokenimage` | write a broken-image capture |
| `acd/`, `host/get_crash_dump`, `download_addc` | crash dump collection |
| `settings/network/conftdownload` | config download |

The raw passthrough applies the union of every Gigabyte profile's rules (`boards.AllExtraDeny`), so the MZ32-AR0
additions (Automation Engine, raw IPMI, sample handlers, KVM token minting, `/api/actions/`, CMC power) also apply here.

## 6. Accounts and auth

- `nsswitch` includes `redfish` (`libnss_redfish`), which gives a **unified account store**. The web admin works on
  Redfish (Basic and sessions), on `/api` and, presumably, on IPMI. This differs from X470, where Redfish has its own
  store.
- `/api`: form `POST /api/session` (username and password only), then a `QSESSIONID` cookie
  (`secure;HttpOnly;Samesite=Lax`) and `CSRFToken`. Live, **CSRF is enforced on GETs as well, and a bad token
  invalidates the session** (ASRock ignores it on GETs). `amimegarac` sends the token on every request.
- Lockout: threshold 5, duration 30.

## 7. How much of amimegarac applies

All of the client applies unchanged: form login, cookie plus CSRF on every method, `cc:7` re-login, the backend-restart
detection and the deny list. The driver uses it **lazily**, for the fan-profile mode, `firmwareInfo` and `raw/api`. No
change to `pkg/protocol/amimegarac` was needed. The ASRock `asrr/*` namespace does not exist (`404 {"code":1010}`).

## 8. Differences vs MZ32-AR0

See `docs/firmware/mz32-ar0.md` §7. In short, MZ32-AR0 is AST2500 / SP-X **12.x** (Redfish 1.8.0) and MC13-LE0 is
AST2600 / SP-X **13.x**. They share the Gigabyte `oem/gbt` layer, the two-member Sensors override and the
Gigabyte-board-serial OEM block. The driver separates them by board model, and for unknown boards by firmware major
(12 vs 13).
