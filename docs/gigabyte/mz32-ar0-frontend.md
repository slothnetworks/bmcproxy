# Gigabyte MZ32-AR0 BMC: web frontend analysis

Status: preliminary. I fetched the frontend from the live unit (`10.0.5.38`, BMC firmware **12.61.39**) on 2026-11-16,
compared it with the firmware image (`firmware-dumped/MZ32-AR0-00/www_plain`), and read it statically. The only
`/api` calls made were one login, the read-only GETs listed in §4, and one logout. Companion docs:
`docs/gigabyte/mz32-ar0-live-probe.md` (the whole probe) and `docs/firmware/mz32-ar0.md` (firmware/handler inventory
and deny list).

Redaction: no password, CSRF token, `QSESSIONID` value or Redfish token appears here.

## 1. Stack identification

**AMI MegaRAC SP-X classic HTML5 web UI** (the "SP-X 12" generation), Gigabyte-branded. It is **not** the newer AMI
Angular UI, not Vertiv/Avocent, and not OpenBMC webui-vue.

| Evidence | Value |
|---|---|
| `GET /` | 200, `Server: lighttpd`, 1,587 B gzip of `index.html`: `<title>MegaRAC SP-X</title>`, AMI copyright banner "(C) COPYRIGHT American Megatrends Inc. 2016-2017", `<script data-main="/app/main" src="/source.min.js">` |
| Bundle | `/source.min.js` (11.7 MB plain, 2.36 MB gzip): almond 0.2.9 (RequireJS) + jQuery 2.1.1 + Backbone/LayoutManager + Underscore templates + i18next, with locales en / zh-CN / zh-TW. Same framework as the ASRock X470/B650 UIs (`docs/api/legacy-api-catalog.md` §1) |
| KVM | `/viewer.html` + `/viewer.min.js` (H5Viewer, websocket `wss://<host>/kvm` on 443, single-port mode), plus the legacy Java `JViewer.jar`, `JViewer-OEM.jar`, `JViewer-FolderRedir.jar`, `Win32.jar` and `Win64.jar` still shipped under `/Java/release/` |
| Image match | sha256 of live `source.min.js`, `styles.min.css`, `viewer.html` and `index.html` = the same files in `firmware-dumped/MZ32-AR0-00/www_plain`. **The live UI is byte-identical to the 12.61.39 image** |
| Version strings | No version file is exposed. `/manifest.json`, `/version.txt` and `/robots.txt` all return **200 + the SPA index** (lighttpd `server.error-handler-404 = "/index.html"`). The firmware version only becomes visible after login (`/api/firmware-info` `fw_ver "12.61.39"`, `date "Jul 2 2025"`, `time "11:14:05 CST"`) or on Redfish `Managers/Self.FirmwareVersion` |
| Gigabyte additions | `/api/system_inventory_gbt/*` (inventory pages), `/api/GSM`, `/api/MINIDP-SUPPORT-status`, `/api/bios-setupdata-status`, `/api/powerControl/*`, the fan-profile pages (`/api/settings/fanprofile/*`), the `h5banner_*` logos |

### 1.1 Unauthenticated surface

- Static assets only. Every `/api/*` route I tried without a session returns `401 {"cc":7,"error":"Invalid
  Authentication"}` (`/api/configuration/project`, `/api/firmware-info`, `/api/status/uptime`). `GET /api/session`
  returns `404 {"error":"Invalid API Call","code":1010}`.
- The JS contains no default-credential hint (no "admin/…", "sysadmin", "superuser" or "default password" strings).
  The login view has a forced password-change form (`#change_password_form`), but it only shows when the login
  response has `passwordStatus == 1`. I didn't record the value on this unit, but Redfish reports
  `PasswordChangeRequired false` for `admin`, whose password is a well-known default (see the security findings in
  the live probe).
- Security headers on static content: HSTS (`max-age=31536000; includeSubDomains; preload`), CSP (`default-src
  'self'; object-src 'none'; connect-src *; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-eval'; img-src
  'self' blob: data:; font-src 'self'`), `X-Frame-Options: SAMEORIGIN`, `X-Content-Type-Options: nosniff`,
  `X-XSS-Protection: 1; mode=block`, `Cache-Control: no-store`. `/api` responses carry the CSP/HSTS/no-store set but
  not `X-Frame-Options`. Plain HTTP on :80 returns a **307** to `https://<host>:443/<path>`, with the security headers
  already on the 307.

## 2. Login flow (from `source.min.js`, confirmed live)

1. `POST /api/session` with `Content-Type: application/x-www-form-urlencoded`, fields **`username`**, **`password`**
   (no `certlogin` field, no JSON). Took 1.4 s live.
2. Response 200 JSON: `{ok:0, privilege:4, extendedpriv:259, racsession_id:<int>, remote_addr, server_name,
   server_addr, HTTPSEnabled:1, CSRFToken:<string>, channel:1, passwordStatus:<int>}`. There are no `TFAEnabled` /
   `user_id` fields (the ASRock B650 4.10 has them).
3. `Set-Cookie: QSESSIONID=<v>; path=/; secure;HttpOnly`. There's no `SameSite` attribute (B650 4.10 sets
   `Samesite=Lax`).
4. The UI stores the token in `sessionStorage["garc"]` ("garc - csrftoken.. just a name change here") and sends it on
   every request as **`X-CSRFTOKEN: <CSRFToken>`** via `$.ajaxSetup`.
5. If `passwordStatus == 1` it shows the change-password form, which posts to `/api/updatenew_password`.
6. Logout: `DELETE /api/session` (with cookie + `X-CSRFTOKEN`) → `200 {"ok":0}`. A replay afterwards → 401 `cc:7`.

**CSRF is enforced on GETs as well** (a difference from both ASRock boards, where GETs ignored the token): with a
valid `QSESSIONID`, `GET /api/status/uptime` with a bogus `X-CSRFTOKEN` → **401 `cc:7`**, and with no header → **401**.
The session stayed valid (the real-token logout afterwards returned 200). A proxy driver must send the header on
every `/api` call.

## 3. Backend routes referenced by the UI

`source.min.js` contains **272** distinct `/api/...` literals. Item routes are built as `collection.url + "/" + id`
(shown as `/:id`), so a few dynamic ones aren't listed. **240 of the 272 have a matching handler string** in
`spx_restservice` / `libspx_restservice_*.so` in the 12.61.39 image. The 32 without one are dead UI code for features
this build doesn't include (AEP/Optane, Intel ACD crashdump, PLDM, Microsemi PCIe switch, `skuv2`,
`signed-image-key`, `oem/node_info`, `user-preference`, `create_lu_cache`).

`viewer.min.js` (KVM) adds `/api/kvm/token`, `/api/settings/media/h5viewercfg`, `/api/settings/media/adviser`,
`/api/chassis/bootoption`, `/api/configuration/{project,runtime}`, `/api/session`, and **`/api/logs/help-data`** (the
KVM viewer's Help button). That's the route that crashed `spx_restservice` and invalidated all sessions on the X470.
**Never let a proxied KVM session or a crawler hit it.**

The UI uses **no Redfish, `/rpc` or `/ext` routes**. `/redfish` is a separate service (§5).

Legend: (live GET 200) = read-only GET confirmed during this probe; (no handler) = no handler string in the image;
(DENY) = crash-prone, side-effecting, file download or firmware/config path: do not call (see
`docs/firmware/mz32-ar0.md` §2.1). Routes with no tag exist but weren't exercised. Methods follow AMI conventions: GET
to read, `PUT` of the whole model to write (see `docs/api/legacy-api-catalog.md` §1.2).

| Prefix | Routes |
|---|---|
| `/api/GSM` | `GSM` (live GET **204**, empty) |
| `/api/MINIDP-SUPPORT-status` | `MINIDP-SUPPORT-status` |
| `/api/acd` | `/download_crashdump` (no handler) (DENY), `/getcrashdumpstatus` (no handler), `/set_crashdump_download` (no handler) (DENY), `/setcrashdumpstatus` (no handler) |
| `/api/actions` | `/bmc/reset` (DENY), `/chassis-led` (DENY), `/power` (DENY) |
| `/api/ae` | `/test` (no handler) (DENY) |
| `/api/ae_test` | `ae_test` (DENY) |
| `/api/aep_inventory` | `/aep_dimm_action` (no handler) (DENY), `/aep_dimm_info` (no handler) (DENY), `/aep_namespace_info` (no handler) (DENY), `/aep_pools_info` (no handler) (DENY), `/aepdimm_fw_flash_status` (no handler) (DENY) |
| `/api/bios-setupdata-status` | `bios-setupdata-status` |
| `/api/chassis-status` | `chassis-status` (live GET 200) |
| `/api/configuration` | `/project` (live GET 200), `/runtime` (live GET 200) |
| `/api/dcmi` | `/chassisPower`, `/power` |
| `/api/detail_sensors_readings` | `detail_sensors_readings` |
| `/api/firmware-info` | `firmware-info` (live GET 200) |
| `/api/fru` | `fru` (live GET 200) |
| `/api/host` | `/get_crash_dump` (no handler) (DENY) |
| `/api/host_inventory` | `/host_interface_baseboard_info`, `/host_interface_memory_info`, `/host_interface_pcie_device_function_info`, `/host_interface_power_info`, `/host_interface_processor_info`, `/host_interface_storage_info`, `/host_interface_system_info`, `/host_interface_thermal_info` |
| `/api/logs` | `/acd` (no handler), `/acd-file` (DENY), `/acd-log`, `/acd-status`, `/audit`, `/audit-file` (DENY), `/dashboardevent`, `/event` (live GET 200), `/event-file` (DENY), `/pending-Deassertions`, `/system`, `/system-file` (DENY), `/video`, `/video-data` (DENY), `/video-log`, `/video-log/delete` (DENY) |
| `/api/maintenance` | `/ValidFileSize` (DENY), `/ValidSignImage` (DENY), `/backup_config` (DENY), `/bmc_recovery` (DENY), `/clear_model_patch` (DENY), `/download_config` (DENY), `/dual_image_config` (DENY), `/dualflashimageconfig` (DENY), `/firmware` (DENY), `/firmware/abort` (DENY), `/firmware/dump` (DENY), `/firmware/dwldfwimg` (DENY), `/firmware/dwldfwstatus-progress` (DENY), `/firmware/flash-progress` (DENY), `/firmware/upgrade` (DENY), `/firmware/verification` (DENY), `/flash` (DENY), `/flashtrack` (DENY), `/fwimage_location` (DENY), `/fwupdate_check` (DENY), `/fwupdate_keepalived` (DENY), `/hpm/` (no handler) (DENY), `/hpm/activatecomponents` (DENY), `/hpm/componentversions` (DENY), `/hpm/exitupdatemode` (DENY), `/hpm/flash` (DENY), `/hpm/freemem` (DENY), `/hpm/preparecomponents` (DENY), `/hpm/updatemode` (DENY), `/hpm/upgradestatus` (DENY), `/hpm/verifyimage` (DENY), `/hpm/verifyimagestatus` (DENY), `/nm_activate` (DENY), `/nm_compareversion` (DENY), `/nm_deactivate` (DENY), `/nm_ecfstatus` (DENY), `/nm_eraseflash` (DENY), `/nm_firmware_update` (DENY), `/nm_upload` (DENY), `/pldm/` (no handler) (DENY), `/pldm/fwupdateprogress` (no handler) (DENY), `/pldm/fwupdatestatus` (no handler) (DENY), `/pldm/getpldmfwinfo` (no handler) (DENY), `/preserve` (DENY), `/reset` (DENY), `/restore_config` (DENY), `/restore_defaults` (DENY), `/signed-image-key` (no handler) (DENY), `/skuv2` (no handler) (DENY), `/system_admin` (DENY), `/upload_restore` (DENY) |
| `/api/oem` | `/node_info` (no handler) |
| `/api/operations` | `/ssl/certificate/generate` (DENY) |
| `/api/powerControl` | `/SetNodeStatus`, `/getNodeInfo` |
| `/api/remote_control` | `/get/kvm/launch` (DENY) |
| `/api/reset-pass` | `reset-pass` (DENY) |
| `/api/scripts` | `scripts`, `/` |
| `/api/sdr` | `sdr` |
| `/api/sensors` | `sensors` (live GET 200), `/` |
| `/api/session` | `session` (live: POST login 200, DELETE logout 200; unauth GET → 404 `{"code":1010}`) |
| `/api/set` | `/dns/registration` |
| `/api/settings` | `/active-directory-settings`, `/active-directory-users`, `/add-iptables-rules`, `/add-one-touch-settings`, `/channels`, `/date-time` (live GET 200), `/dns`, `/download_image` (DENY), `/fanprofile`, `/firewall`, `/firewall-ip-rules`, `/firewall-port-rules`, `/iptables-rules`, `/javasol`, `/ldap-certificates`, `/ldap-settings`, `/ldap-users`, `/license`, `/licenses`, `/log`, `/log-certificates`, `/log-policy`, `/mouse`, `/ncsi-interfaces`, `/network` (live GET 200), `/network-bond`, `/network-link`, `/one-touch-settings`, `/packages`, `/pam-order`, `/sensor-threshold`, `/service-sessions`, `/services` (live GET 200), `/smtp`, `/sol-recorded-video`, `/static-ipv6`, `/user-preference` (no handler), `/users` (live GET 200), `/users_read_only` |
| `/api/settings/dns` | `/domain-options`, `/registration`, `/restart` (DENY), `/server-options`, `/tsig-private-key` |
| `/api/settings/fanprofile` | `/collection`, `/device_define`, `/device_define/collection`, `/mode` (live GET 200) |
| `/api/settings/firewall` | `/:id` |
| `/api/settings/media` | `/active_redirections`, `/adviser`, `/general`, `/instance`, `/local/configurations`, `/local/file`, `/local/image`, `/local/images`, `/local/lmediasizecheck`, `/local/start-media`, `/local/stop-media`, `/remote/configurations`, `/remote/images`, `/remote/lmediarefreshlist`, `/remote/rmediarefreshlist`, `/remote/start-media`, `/remote/stop-media`, `/remotesession` |
| `/api/settings/ncsi` | `/mode` |
| `/api/settings/network` | `/activelancfg` |
| `/api/settings/nvme_management` | `/nvme_controller_info`, `/nvme_p3700_vital_product_data` |
| `/api/settings/nvme_mi_management` | `/nvme_mi_clear_status_flags`, `/nvme_mi_set_status_flags`, `/nvme_mi_subsystem_info`, `/nvme_mi_vpd_info` |
| `/api/settings/pcie_switch_management` | `/get_pcie_bifurcation_info` (no handler), `/pcie_mgmt_dev_info` (no handler), `/pcie_partition_count` (no handler), `/pcie_port_bind` (no handler), `/pcie_port_link_status` (no handler), `/pcie_port_unbind` (no handler), `/pcie_switch_reset` (no handler) (DENY), `/set_pcie_bifurcation_info` (no handler), `/validate_port_unbind` (no handler) |
| `/api/settings/pcie_switch_mgmt` | `/pcie_partition_info` (no handler) |
| `/api/settings/pef` | `/alert_policies`, `/alert_string_keys`, `/destinations`, `/event_filters`, `/lan_destinations`, `/send_test_alert` (DENY) |
| `/api/settings/radius` | `/advanced`, `/general` |
| `/api/settings/raid_management` | `/bbu_info`, `/clear_all_foreign_cmd`, `/create_lu_cache` (no handler), `/create_virtual_drive`, `/dedicated_hot_spare_info`, `/delete_virtual_drive`, `/logical_consistency_check_cmd`, `/logical_device_info`, `/logical_device_progress_cmd`, `/logical_initialize_check_cmd`, `/logical_start_stop_locate_cmd`, `/manage_array_info`, `/manage_data_drive_array`, `/manage_spare_drive_array`, `/patrol_read_properties`, `/patrol_read_start_stop_cmd`, `/physical_dedicated_hotspare_cmd`, `/physical_device_info`, `/physical_hotspare_cmd`, `/physical_onlineoffline_cmd`, `/physical_removal_cmd`, `/physical_start_stop_locate_cmd`, `/physical_state_cmd`, `/raid_controller_info`, `/raid_event_log`, `/raid_levels`, `/raid_pd_count`, `/sesenclosureinfo`, `/storage_summary` |
| `/api/settings/sasit_management` | `/sasit_controller_info`, `/sasit_event_log`, `/sasit_physical_device_info`, `/sasit_topology_info` |
| `/api/settings/sensor-threshold` | `/` |
| `/api/settings/smtp` | `/smtpupload` |
| `/api/settings/sol-recorded-video` | `/` |
| `/api/settings/ssl` | `/certificate`, `/certificate-info` (live GET 200) |
| `/api/settings/user` | `/ssh-key-upload/:id` |
| `/api/settings/users` | `/:id` |
| `/api/settings/video` | `/pre-event`, `/remote-storage`, `/sol-remote-storage`, `/sol-triggers`, `/triggers` |
| `/api/sol` | `/solcfg` |
| `/api/status` | `/uptime` (live GET 200) |
| `/api/system_inventory_gbt` | `/bios-setup-file` (DENY), `/bios_info` (live GET 200), `/cmc_ip`, `/cpld_info` (live GET 200), `/cpu_info` (live GET 200), `/dimm_info` (live GET 200), `/dimm_info_ex`, `/fpga_info`, `/gpu_info`, `/hdd_info`, `/nic_info` (live GET 200), `/nvswitch_info`, `/pci_info` (live GET 200), `/scp_info`, `/smbios-file` (DENY), `/smbpbi` |
| `/api/tasks` | `tasks` |
| `/api/updatenew_password` | `updatenew_password` (DENY) |

## 4. Read routes exercised live (one `/api` session, logged out at the end)

All were GETs with `QSESSIONID` + `X-CSRFTOKEN`, run one at a time with a 0.6 s gap. I confirmed every handler as a
plain getter from its function name in `spx_restservice` (`get*_func` / `getSysInv*_gbt_func`) before calling it.
Values that are serials or MACs are not reproduced.

| Route | Status / time | Shape / notable values |
|---|---|---|
| `/api/firmware-info` | 200, 0.3 s | IPMI Get Device ID view: `device_id 32`, `revision 129`, `fw_rev_1 12`, `fw_rev_2 97` (0x61), `ipmi_version 2`, `mfg_id` bytes `0A 3C 00` (= **IANA 15370, Giga-Byte**), `prod_id 4104` (0x1008), `aux 39`, **`fw_ver "12.61.39"`**, `bios_ver ""`, `date "Jul 2 2025"`, `time "11:14:05 CST"`, **`active_image 1`** (dual image, image 1 running), `sku_ver "1.0"`, `sdr_ver` |
| `/api/chassis-status` | 200, 1.6 s | `{"power_status":1,"led_status":0}`. There's no `power_cycle_interval` (B650 has it) |
| `/api/status/uptime` | 200 | `{"minutes_per_count":60,"poh_counter_reading":1992}` (host power-on hours, **not** BMC uptime) |
| `/api/configuration/project` | 200 | array of `{feature:"…"}`: compiled-in features (`NWBONDING`, `DUAL_IMAGE`, `SNMP`, `NTP_SERVER_SUPPORT`, `SINGLE_PORT_APP`, `EXTENDED_PRIV`, …). The UI gates its pages on this |
| `/api/configuration/runtime` | 200 | `[{feature:"LDAP",enabled:0},{AD:0},{RADIUS:0},{SINGLE_PORT_APP:1},{SD_MEDIA_SUPPORT:0}]` |
| `/api/fru` | 200 | 6 FRU devices: `{device:{id,name}, common_header, chassis{type,part_number,serial_number}, board{date,manufacturer "GIGABYTE",product_name "MZ32-AR0-00",serial_number,part_number}, product{…}}`. The chassis/product fields hold placeholders (`01234567…`) |
| `/api/sensors` | 200, 0.1 s | **36 sensors** (ASRock-style shape): `{id, sensor_number, name, owner_id, owner_lun, raw_reading, type, type_number, reading, sensor_state, discrete_state, accessible, settable_flag, lower_non_recoverable_threshold … higher_non_recoverable_threshold, unit}`. **`unit` is `"deg_c"`** (like the B650 1.10 HAR, unlike B650 4.10's `"°C"`). Thresholds are numbers (e.g. CPU0_TEMP LNR -128 / LC -10 / LNC -5 / UNC 97 / UC 100 / UNR 127) |
| `/api/settings/services` | 200 | AMI services list: web (80/443, `time_out 1800`, `maximum_sessions 148`, `active_session 130`), kvm, cd-media, hd-media, ssh, solssh …. All have `singleport_status 1` and show the 80/443 ports when single-port is on |
| `/api/settings/date-time` | 200 | `{primary_ntp:"pool.ntp.org", secondary_ntp:"time.nist.gov", ntp_auto_date:0, timestamp, localized_timestamp, utc_minutes:0, timezone:"Etc/GMT"}`. NTP auto-sync is **off**, and there are no PTP fields |
| `/api/settings/network` | 200, 1.1 s | one entry, `interface_name "bond0"` (bonding on by default), static IPv4, IPv6 DHCP, VLAN off |
| `/api/settings/ssl/certificate-info` | 200 | `certificate_date "Thu Dec 24 08:33:30 2020\n"` (literal `\n` quirk, as on ASRock). This is the date it was written into `/conf`. The certificate itself is the 2018 AMI default |
| `/api/settings/users` | 200 | 32 slots × channels. Named: `anonymous` (userid 1, access 0) and `admin` (userid 2, administrator). The body has no password hashes (`ssh_key "Not Available"`, `email_id`, `snmp_*`, `privilegeByChannel`) |
| `/api/system_inventory_gbt/bios_info` | 200 | `{strManufacturer:"GIGABYTE", strProductName:"MZ32-AR0-00", strReleaseDate:"06/23/2026", strVersion:"M32"}` |
| `/api/system_inventory_gbt/cpu_info` | 200 | SMBIOS type-4 view: `AMD EPYC 7B13 64-Core Processor`, socket `P0`, `iCoreCount 64`, `iThreadCount 128`, `fMaxSpeed "3.525000"` (GHz as a string), `arrCache[]` |
| `/api/system_inventory_gbt/dimm_info` | 200 | 16 slots `DIMM_P0_[A-H][01]`, SMBIOS type-17 fields (`iSize`, `iSpeed`, `strManufacturer`, `strSN`, `strPartNum`, …). Empty slots have `iSize 0`, `strType "Unknown"` |
| `/api/system_inventory_gbt/cpld_info` | 200, 2.1 s | `{"strVersion":"12 "}` (trailing space). Redfish `FirmwareInventory/MB_CPLD1.Version` says `"ff"`, so the two views disagree |
| `/api/system_inventory_gbt/pci_info` | 200, 1.6 s | SMBIOS type-9/41 PCI list (class, VID/DID, slot designation, link width/speed) |
| `/api/system_inventory_gbt/nic_info` | 200 | per-port MAC, slot, bus/device numbers |
| `/api/GSM` | **204**, empty body | Gigabyte Server Management hook, nothing configured |
| `/api/settings/fanprofile/mode` | 200 | `{"strMode":"NEW_PROFILE"}` (same as Redfish `…/FanprofileService/Fanprofile.strMode`) |
| `/api/logs/event` | 200, 1.8 s, **550 KB** | the full IPMI SEL as decoded JSON (**1,022 records**): `{id, record_type, timestamp, sensor_type, sensor_number, event_direction, event_reading_class, event_description, event_assertSeverity, …}`. It's unpaged and large. The UI supports `?LASTEVENTID=` for incremental reads (same as ASRock) |

## 5. Relationship to Redfish

The UI doesn't touch Redfish. `/redfish` is reverse-proxied by lighttpd to a separate LuaJIT/redis service
(`127.0.0.1:9080`), so the UI `/api` backend (`spx_restservice`, single fastcgi process) and Redfish fail
independently. Almost everything the UI shows is also in Redfish (see the capability map in the live probe). The
UI-only data are the raw IPMI SEL with decoded descriptions (Redfish's SEL view holds only 57 + 86 + 4 entries
across three log services, against 1,022 SEL records in `/api/logs/event`), the AMI services/session table, the
Gigabyte SMBIOS inventory, `firmware-info` (`active_image`), and the PEF/SMTP/firewall/one-touch settings.
