# X470D4U endpoint reference (legacy `/api`)

> Assembled from the live probe scratch data (no fresh BMC calls at assembly time). The live unit runs **BMC 2.32.00**; the dumped firmware is **3.02.00**, so routes seen only in the 3.02 binary are candidates (`in_302_firmware`) and a live 404 may just mean the route is newer than 2.32. Body samples are redacted. Redfish is covered in `live-probe.md` / `redfish-uri-inventory.json`; this file is the `/api` surface.

- Endpoints catalogued: **298**
- Probed live on 2.32.00: **175**
- In the 3.02 firmware but 404 on 2.32 (version gap): **12**

## Behaviour that shapes the proxy

| Fact | Detail |
|---|---|
| CSRF on writes | ENFORCED: PUT with bogus or empty X-CSRFTOKEN -> 401 {cc:7}; correct token -> 200 |
| CSRF on GETs | NOT enforced: GET with a bogus token -> 200 (empty/no token -> 401) |
| Method override | X-HTTP-Method-Override honoured by the backend; a proxy MUST strip it |
| Invalid session | `401 {"cc":7,"error":"Invalid Authentication"}` |
| Unknown route | `404 {"error":"Invalid API Call","code":1010}` |
| Feature disabled | 405 code 8500 "Feature is not enabled" |
| OPTIONS/HEAD | always 200, empty, no Allow header, even for nonexistent paths |
| **help-data crash** | GET /api/logs/help-data (no ?file=) -> HTML 500 and crashes the backend; two-session test showed it invalidates ALL web sessions, not just the caller's |
| Other backend-crashers | logs/help-data, hpm/upgradestatus, hpm/verifyimagestatus |
| 502 handlers | asrr/mca-file, maintenance/download_config, settings/download_image |
| Write 500 (payload fmt) | settings/date-time, settings/pef/lan_destinations/1, settings/smtp/1 |
| Unauth static symlinks | /config.bak, /SELLog, /bsod, /capture, /video (served by lighttpd; auth-bypass risk) |
| Hidden accounts | sysadmin (uid0, enable_user_access=1), fwupd (ipmi user id 3) |
| Suspected mem leak in response | settings/dns server dns_iface, maintenance/firmware/dwldfwstatus-progress progress/state |


## ? (36)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/ae/results` | ? | — | 500 |
| `/api/ae/run` | ? | — | not probed |
| `/api/asrr/bios-model-name` | ? | — | 200 |
| `/api/asrr/inventory_info` | ? | — | 200 |
| `/api/asrr/inventory_status` | ? | — | 200 |
| `/api/asrr/maintenance/BIOS/export/current_settings` | ? | — | 404 |
| `/api/asrr/maintenance/BIOS/export/settings` | ? | — | 404 |
| `/api/asrr/maintenance/BIOS/import/settings` | ? | — | not probed |
| `/api/asrr/maintenance/BMC/coldreset` | ? | — | not probed |
| `/api/asrr/maintenance/BMC/configuration` | ? | — | not probed |
| `/api/asrr/maintenance/BMC/firmware` | ? | — | not probed |
| `/api/asrr/maintenance/BMC/flash-progress` | ? | — | 200 |
| `/api/asrr/maintenance/BMC/status` | ? | — | 200 |
| `/api/asrr/maintenance/BMC/upgrade` | ? | — | not probed |
| `/api/asrr/settings/BIOS/get/status` | ? | — | 404 |
| `/api/hsc-info` | ? | — | 500 |
| `/api/logs/recentevent` | ? | — | 200 |
| `/api/logs/selevent` | ? | — | 404 |
| `/api/maintenance/fwimage/info` | ? | — | 500 |
| `/api/maintenance/fwimage/remote/info` | ? | — | 404 |
| `/api/maintenance/hpm/biosfw` | ? | — | not probed |
| `/api/maintenance/hpm/mmcfw` | ? | — | not probed |
| `/api/previewer` | ? | — | 404 |
| `/api/scripts/:id` | ? | — | not probed |
| `/api/sensors/:id` | ? | — | not probed |
| `/api/sensors/basic` | ? | — | 200 |
| `/api/settings/dns/domain` | ? | — | 200 |
| `/api/settings/dns/general` | ? | — | 200 |
| `/api/settings/dns/host` | ? | — | 200 |
| `/api/settings/dns/server` | ? | — | 200 |
| `/api/settings/dns/tsig` | ? | — | 200 |
| `/api/settings/email-format` | ? | — | 404 |
| `/api/settings/media/remote/configuration` | ? | — | 404 |
| `/api/settings/sensor-threshold/:id` | ? | — | not probed |
| `/api/settings/service/web-restart` | ? | — | not probed |
| `/api/settings/ssl` | ? | — | 404 |

## Automation engine (scripts/tasks) (4)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/ae_test` | PUT | B650,B650,X470 | not probed |
| `/api/scripts` | GET,POST,PUT | B650,B650,X470 | 500 |
| `/api/tasks` | GET,POST | B650,B650,X470 | 500 |
| `/api/tasks/:id` | GET,POST | B650,B650,X470 | not probed |

## Dashboard & identity (6)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/asrr/fw-info` | GET | B650,B650,X470 | 200 |
| `/api/asrr/model-name` | GET | B650,B650,X470 | 200 |
| `/api/asrr/product-name` | GET | X470 | 200 |
| `/api/asrr/selsummary` | GET | B650,B650,X470 | 200 |
| `/api/firmware-info` | GET,PUT | B650,B650,X470 | 200 |
| `/api/logs/dashboardevent` | GET | B650,B650,X470 | 200 |

## FRU & PLDM inventory (1)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/fru` | GET | B650,B650,X470 | 200 |

## Feature discovery & UI configuration (3)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/configuration/project` | GET | B650,B650,X470 | 200 |
| `/api/configuration/runtime` | GET | B650,B650,X470 | 200 |
| `/api/status/uptime` | GET | B650,B650,X470 | 200 |

## Host inventory (17)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/host_inventory/host_interface_baseboard_info` | GET | B650,B650,X470 | 200 |
| `/api/host_inventory/host_interface_memory_info` | GET | B650,B650,X470 | 200 |
| `/api/host_inventory/host_interface_pcie_device_function_info` | GET | B650,B650,X470 | 200 |
| `/api/host_inventory/host_interface_power_info` | GET | B650,B650,X470 | 200 |
| `/api/host_inventory/host_interface_processor_info` | GET | B650,B650,X470 | 200 |
| `/api/host_inventory/host_interface_storage_info` | GET | B650,B650,X470 | 200 |
| `/api/host_inventory/host_interface_system_info` | GET | B650,B650,X470 | 200 |
| `/api/host_inventory/host_interface_thermal_info` | GET | B650,B650,X470 | 200 |
| `/api/system_inventory/component_status_update` | PUT | X470 | not probed |
| `/api/system_inventory/cpu_info` | GET | X470 | 500 |
| `/api/system_inventory/cpu_info/:id` | GET | X470 | not probed |
| `/api/system_inventory/device_count` | GET | X470 | 500 |
| `/api/system_inventory/device_count/:id` | GET | X470 | not probed |
| `/api/system_inventory/dimm_info` | GET | X470 | 500 |
| `/api/system_inventory/dimm_info/:id` | GET | X470 | not probed |
| `/api/system_inventory/pci_info` | GET | X470 | 500 |
| `/api/system_inventory/pci_info/:id` | GET | X470 | not probed |

## KVM, H5Viewer & SOL (6)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/asrr/java-console` | GET | X470 | 200 |
| `/api/kvm/token` | GET | B650,B650,X470 | 200 |
| `/api/settings/javasol` | GET | X470 | 500 |
| `/api/settings/media/adviser` | GET | B650,B650,X470 | 200 |
| `/api/settings/media/remotesession` | GET,PUT | B650,B650,X470 | 200 |
| `/api/settings/mouse` | GET,PUT | B650,B650,X470 | 200 |

## Logs, crash dumps & diagnostics (18)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/asrr/clearmcalog` | DELETE | X470 | not probed |
| `/api/asrr/getmcalog` | GET | B650,B650,X470 | 200 |
| `/api/asrr/mca-file` | GET | X470 | 502 |
| `/api/logs/audit` | GET | B650,B650,X470 | 200 |
| `/api/logs/event` | DELETE,GET | B650,B650,X470 | 200 |
| `/api/logs/event-file` | GET | B650,B650,X470 | 200 |
| `/api/logs/event/:id` | DELETE,GET | B650,B650,X470 | not probed |
| `/api/logs/eventlog` | GET | B650,B650,X470 | 200 |
| `/api/logs/getvideotag` | POST | X470 | not probed ·RT:POST=200 |
| `/api/logs/help-data` | GET | B650,B650,X470 | 200,500 |
| `/api/logs/raw-event-file` | GET | B650,B650,X470 | 200 |
| `/api/logs/system` | GET | B650,B650,X470 | 200 |
| `/api/logs/video` | GET | B650,B650,X470 | 200 |
| `/api/logs/video-data` | GET | B650,B650,X470 | not probed |
| `/api/logs/video-log` | GET | B650,B650,X470 | 200 |
| `/api/logs/video-log/delete` | DELETE | B650,B650,X470 | not probed |
| `/api/settings/log` | GET,PUT | B650,B650,X470 | 200 ·RT:GET=200,PUT=200,GET=200 |
| `/api/settings/log-policy` | GET,PUT | B650,B650,X470 | 200 ·RT:GET=200,PUT=200,GET=200,PUT=401 |

## Maintenance & firmware update (48)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/asrr/maintenance/BIOS/PublicKey` | POST | X470 | not probed |
| `/api/asrr/maintenance/BIOS/configuration` | POST | B650,B650,X470 | not probed |
| `/api/asrr/maintenance/BIOS/firmware` | POST | B650,B650,X470 | not probed |
| `/api/asrr/maintenance/BIOS/flash-progress` | GET | B650,B650,X470 | 200 |
| `/api/asrr/maintenance/BIOS/status` | GET | B650,B650,X470 | 200 |
| `/api/asrr/maintenance/BIOS/upgrade` | POST | B650,B650,X470 | not probed |
| `/api/asrr/maintenance/CPLD/configuration` | POST | B650,X470 | not probed |
| `/api/asrr/maintenance/CPLD/firmware` | POST | B650,X470 | not probed |
| `/api/asrr/maintenance/CPLD/status` | GET | B650,X470 | 404 |
| `/api/asrr/maintenance/CPLD/upgrade` | POST | B650,X470 | not probed |
| `/api/maintenance/ValidSignImage` | GET | B650,B650,X470 | 500 |
| `/api/maintenance/backup_config` | GET,PUT | B650,B650,X470 | 200 |
| `/api/maintenance/bmc_recovery` | GET,PUT | B650,B650,X470 | 405 |
| `/api/maintenance/download_config` | GET | B650,B650,X470 | 502 |
| `/api/maintenance/dual_image_config` | GET,PUT | B650,B650,X470 | 405 |
| `/api/maintenance/firmware` | POST | B650,B650,X470 | not probed |
| `/api/maintenance/firmware/dwldfwimg` | PUT | B650,B650,X470 | not probed |
| `/api/maintenance/firmware/dwldfwstatus-progress` | GET | B650,B650,X470 | 200 |
| `/api/maintenance/firmware/flash-progress` | GET | B650,B650,X470 | 200 |
| `/api/maintenance/firmware/upgrade` | PUT | B650,B650,X470 | not probed |
| `/api/maintenance/firmware/verification` | GET,POST | B650,B650,X470 | not probed |
| `/api/maintenance/flash` | PUT | B650,B650,X470 | not probed |
| `/api/maintenance/flash_image_config` | PUT | X470 | not probed |
| `/api/maintenance/fwimage_location` | GET,PUT | B650,B650,X470 | 200 |
| `/api/maintenance/hpm` | POST | — | not probed |
| `/api/maintenance/hpm/activatecomponents` | PUT | B650,B650,X470 | not probed |
| `/api/maintenance/hpm/componentversions` | GET | B650,B650,X470 | 500 |
| `/api/maintenance/hpm/exitupdatemode` | PUT | B650,B650,X470 | not probed |
| `/api/maintenance/hpm/flash` | PUT | B650,B650,X470 | not probed |
| `/api/maintenance/hpm/preparecomponents` | PUT | B650,B650,X470 | not probed |
| `/api/maintenance/hpm/updatemode` | PUT | B650,B650,X470 | not probed |
| `/api/maintenance/hpm/upgradestatus` | GET | B650,B650,X470 | 500 |
| `/api/maintenance/hpm/verifyimage` | PUT | B650,B650,X470 | not probed |
| `/api/maintenance/hpm/verifyimagestatus` | GET | B650,B650,X470 | 500 |
| `/api/maintenance/nm_activate` | GET | B650,B650,X470 | not probed |
| `/api/maintenance/nm_compareversion` | GET | B650,B650,X470 | not probed |
| `/api/maintenance/nm_deactivate` | GET | B650,B650,X470 | not probed |
| `/api/maintenance/nm_ecfstatus` | GET | B650,B650,X470 | not probed |
| `/api/maintenance/nm_eraseflash` | GET | B650,B650,X470 | not probed |
| `/api/maintenance/nm_firmware_update` | GET,PUT | B650,B650,X470 | not probed |
| `/api/maintenance/nm_upload` | GET,POST | B650,B650,X470 | not probed |
| `/api/maintenance/preserve` | GET,PUT | B650,B650,X470 | 200 |
| `/api/maintenance/reset` | POST | B650,B650,X470 | not probed |
| `/api/maintenance/restore_config` | GET,PUT | B650,B650,X470 | 200 |
| `/api/maintenance/restore_defaults` | GET,PUT | B650,B650,X470 | 200 |
| `/api/maintenance/signed-image-key` | GET,POST | X470 | 200 |
| `/api/maintenance/system_admin` | GET,PUT | B650,B650,X470 | 200 |
| `/api/maintenance/upload_restore` | POST | B650,B650,X470 | not probed |

## Power, chassis & UID (8)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/actions/chassis-led` | POST | B650,B650,X470 | not probed |
| `/api/actions/power` | POST | B650,B650,X470 | not probed |
| `/api/asrr/PowerSource` | GET | B650,B650,X470 | 200 |
| `/api/asrr/bmccoldreset` | PUT | B650,B650,X470 | not probed |
| `/api/asrr/getprochot` | GET | X470 | 200 ·RT:GET=200,GET=200 |
| `/api/asrr/setprochot` | PUT | X470 | not probed ·RT:PUT=200 |
| `/api/chassis` | GET | X470 | 200 |
| `/api/chassis-status` | GET | B650,B650,X470 | 200,401 ·RT:GET=401,GET=404 |

## Sensors & SDR (4)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/detail_sensors_readings` | GET | B650,B650,X470 | 200 |
| `/api/sdr` | GET | B650,B650,X470 | 200 |
| `/api/sensors` | GET | B650,B650,X470 | 200 |
| `/api/settings/sensor-threshold` | GET,PUT | B650,B650,X470 | 404 |

## Session & authentication (4)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/asrr/session` | POST | X470 | not probed |
| `/api/reset-pass` | POST | B650,B650,X470 | not probed |
| `/api/session` | DELETE,POST | B650,B650,X470 | not probed ·RT:DELETE=200 |
| `/api/viewerlogin` | POST | B650,B650,X470 | not probed |

## Settings: BIOS / platform OEM (1)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/asrr/getbioscode` | GET | B650,B650,X470 | 200 |

## Settings: LDAP / AD / RADIUS (9)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/settings/active-directory-settings` | GET,PUT | B650,B650,X470 | 200 |
| `/api/settings/active-directory-users` | GET | B650,B650,X470 | 500 |
| `/api/settings/active-directory-users/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/ldap-certificates` | POST | B650,B650,X470 | not probed |
| `/api/settings/ldap-settings` | GET,PUT | B650,B650,X470 | 200 |
| `/api/settings/ldap-users` | GET | B650,B650,X470 | 500 |
| `/api/settings/ldap-users/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/radius/advanced` | GET,PUT | B650,B650,X470 | 500 |
| `/api/settings/radius/general` | GET,PUT | B650,B650,X470 | 200 |

## Settings: SMTP, PEF, alerts & SNMP (12)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/settings/pef/alert_policies` | GET | B650,B650,X470 | 200 |
| `/api/settings/pef/alert_policies/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/pef/alert_string_keys` | GET | B650,B650,X470 | 200 |
| `/api/settings/pef/destinations` | GET | B650,B650,X470 | 200 |
| `/api/settings/pef/event_filters` | GET | B650,B650,X470 | 200 |
| `/api/settings/pef/event_filters/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/pef/lan_destinations` | GET | B650,B650,X470 | 200 |
| `/api/settings/pef/lan_destinations/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/pef/send_test_alert` | POST | B650,B650,X470 | not probed |
| `/api/settings/smtp` | GET | B650,B650,X470 | 200 |
| `/api/settings/smtp/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/smtp/smtpupload` | POST | B650,B650,X470 | not probed |

## Settings: SSL (3)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/operations/ssl/certificate/generate` | GET,PUT | B650,B650,X470 | 200 |
| `/api/settings/ssl/certificate` | GET,POST,PUT | B650,B650,X470 | 200 |
| `/api/settings/ssl/certificate-info` | GET,PUT | B650,B650,X470 | 200 |

## Settings: date & time / NTP (1)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/settings/date-time` | GET,PUT | B650,B650,X470 | 200 ·RT:GET=200,PUT=500,GET=200 |

## Settings: fan control (21)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/asrr/GetFanDuty` | GET | X470 | 200 |
| `/api/asrr/settings/getfancloseloopcontroltable` | GET | X470 | 200 ·RT:GET=200 |
| `/api/asrr/settings/getfancontrolmode` | GET | X470 | 200 ·RT:GET=200,GET=200 |
| `/api/asrr/settings/getfandutymanualmode` | GET | X470 | 200 |
| `/api/asrr/settings/getfanopenloopcontroltable` | GET | X470 | 200 ·RT:GET=200 |
| `/api/asrr/settings/getfantempsensorandcorrespondfantable` | GET | X470 | 200 |
| `/api/asrr/settings/getsupportfan` | GET | X470 | 200 |
| `/api/asrr/settings/gettscftsupportfan` | GET | X470 | 200 |
| `/api/asrr/settings/setclearallfansetting` | POST | X470 | not probed |
| `/api/asrr/settings/setcloselooptablenumber` | POST | X470 | not probed ·RT:POST=200 |
| `/api/asrr/settings/setfancloseloopcontroltable` | POST | X470 | not probed |
| `/api/asrr/settings/setfancontrolmode` | POST | X470 | not probed ·RT:POST=200 |
| `/api/asrr/settings/setfandutymanualmode` | POST | X470 | not probed |
| `/api/asrr/settings/setfanopenloopcontroltable` | POST | X470 | not probed |
| `/api/asrr/settings/setfantempsensorandcorrespondfantable` | POST | X470 | not probed |
| `/api/asrr/settings/setopenlooptablenumber` | POST | X470 | not probed ·RT:POST=200 |
| `/api/asrr/settings/settscftsensornumber` | POST | X470 | not probed |
| `/api/settings/fan/fan_closeloopctb` | GET | — | 404 |
| `/api/settings/fan/fan_mode` | GET | — | 404 |
| `/api/settings/fan/fan_openloopctb` | GET | — | 404 |
| `/api/settings/fan/fan_tscft` | GET | — | 404 |

## Settings: firewall (8)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/settings/firewall` | GET | B650,B650,X470 | 200 |
| `/api/settings/firewall-ip-rule` | GET,PUT | X470 | 200 |
| `/api/settings/firewall-ip-rules` | GET | B650,B650,X470 | 200 |
| `/api/settings/firewall-ip-rules/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/firewall-port-rule` | GET,PUT | X470 | 200 |
| `/api/settings/firewall-port-rules` | GET | B650,B650,X470 | 200 |
| `/api/settings/firewall-port-rules/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/firewall/:id` | DELETE | B650,B650,X470 | not probed |

## Settings: licensing (3)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/settings/license` | GET,PUT | B650,B650,X470 | 405 |
| `/api/settings/licenses` | GET | B650,B650,X470 | 405 |
| `/api/settings/licenses/:id` | GET | B650,B650,X470 | not probed |

## Settings: network (22)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/asrr/getvetobit` | GET | X470 | 200 |
| `/api/asrr/setvetobit` | PUT | X470 | not probed |
| `/api/set/dns/registration` | POST | B650,B650,X470 | not probed |
| `/api/settings/channels` | GET | B650,B650,X470 | 200 |
| `/api/settings/dns` | GET,PUT | X470 | 200 |
| `/api/settings/dns/domain-options` | GET | B650,B650,X470 | 200 |
| `/api/settings/dns/registration` | GET | B650,B650,X470 | 200 |
| `/api/settings/dns/registration/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/dns/restart` | PUT | B650,B650,X470 | not probed |
| `/api/settings/dns/server-options` | GET | B650,B650,X470 | 200 |
| `/api/settings/dns/tsig-private-key` | POST | B650,B650,X470 | not probed |
| `/api/settings/ncsi-interfaces` | GET | B650,B650,X470 | 200 |
| `/api/settings/ncsi/mode` | GET,PUT | B650,B650,X470 | 200 |
| `/api/settings/network` | GET | B650,B650,X470 | 200 |
| `/api/settings/network-bond` | GET,PUT | B650,B650,X470 | 200 |
| `/api/settings/network-interfaces` | GET | — | 200 |
| `/api/settings/network-link` | GET | B650,B650,X470 | 200 |
| `/api/settings/network-link/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/network/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/network/activelancfg` | GET | B650,B650,X470 | 200 |
| `/api/settings/packages` | GET | B650,B650,X470 | 200 |
| `/api/settings/static-ipv6` | GET | B650,B650,X470 | 200 |

## Settings: services & sessions (7)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/asrr/getWebEnableTimeout` | GET | B650,B650,X470 | 200 |
| `/api/asrr/setWebEnableTimeout` | POST | B650,B650,X470 | not probed |
| `/api/settings/service-sessions` | DELETE,GET | B650,B650,X470 | 200 |
| `/api/settings/service-sessions/:id` | DELETE,GET | B650,B650,X470 | not probed |
| `/api/settings/services` | GET | B650,B650,X470 | 200 |
| `/api/settings/services/:id` | GET | B650,B650,X470 | not probed |
| `/api/settings/web_server_instances` | GET,PUT | B650,B650,X470 | 200 |

## Settings: users & PAM (5)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/settings/pam-order` | GET | B650,B650,X470 | 200 |
| `/api/settings/user/ssh-key-upload/:id` | POST | B650,B650,X470 | not probed |
| `/api/settings/users` | GET | B650,B650,X470 | 200 |
| `/api/settings/users/:id` | DELETE | B650,B650,X470 | not probed |
| `/api/settings/users_read_only` | GET | B650,B650,X470 | 200 |

## Settings: video recording / BSOD / SOL recording (7)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/settings/download_image` | GET | B650,B650,X470 | 502 |
| `/api/settings/sol-recorded-video` | GET | B650,B650,X470 | 200 |
| `/api/settings/video/pre-event` | GET,PUT | B650,B650,X470 | 200 ·RT:GET=200,PUT=200,GET=200 |
| `/api/settings/video/remote-storage` | GET,PUT | B650,B650,X470 | 200 |
| `/api/settings/video/sol-remote-storage` | GET,PUT | B650,B650,X470 | 500 |
| `/api/settings/video/sol-triggers` | GET,PUT | B650,B650,X470 | 500 |
| `/api/settings/video/triggers` | GET,PUT | B650,B650,X470 | 200 ·RT:GET=200,PUT=200,GET=200 |

## Storage / PCIe device management (30)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/settings/raid_management/bbu_info` | GET | X470 | 405 |
| `/api/settings/raid_management/bbu_info/:id` | GET | X470 | not probed |
| `/api/settings/raid_management/create_virtual_drive` | GET,PUT | X470 | not probed |
| `/api/settings/raid_management/delete_virtual_drive` | DELETE | X470 | not probed |
| `/api/settings/raid_management/logical_consistency_check_cmd` | PUT | X470 | not probed |
| `/api/settings/raid_management/logical_device_info` | GET | X470 | 405 |
| `/api/settings/raid_management/logical_device_info/:id` | GET | X470 | not probed |
| `/api/settings/raid_management/logical_device_progress_cmd` | PUT | X470 | not probed |
| `/api/settings/raid_management/logical_initialize_check_cmd` | PUT | X470 | not probed |
| `/api/settings/raid_management/logical_start_stop_locate_cmd` | PUT | X470 | not probed |
| `/api/settings/raid_management/physical_dedicated_hotspare_cmd` | PUT | X470 | not probed |
| `/api/settings/raid_management/physical_device_info` | GET | X470 | 405 |
| `/api/settings/raid_management/physical_device_info/:id` | GET | X470 | not probed |
| `/api/settings/raid_management/physical_hotspare_cmd` | PUT | X470 | not probed |
| `/api/settings/raid_management/physical_onlineoffline_cmd` | PUT | X470 | not probed |
| `/api/settings/raid_management/physical_removal_cmd` | PUT | X470 | not probed |
| `/api/settings/raid_management/physical_start_stop_locate_cmd` | PUT | X470 | not probed |
| `/api/settings/raid_management/raid_controller_info` | GET | X470 | 405 |
| `/api/settings/raid_management/raid_controller_info/:id` | GET | X470 | not probed |
| `/api/settings/raid_management/raid_event_log` | DELETE,GET | X470 | 405 |
| `/api/settings/raid_management/raid_levels` | PUT | X470 | not probed |
| `/api/settings/raid_management/raid_pd_count` | PUT | X470 | not probed |
| `/api/settings/raid_management/storage_summary` | GET | X470 | 405 |
| `/api/settings/raid_management/storage_summary/:id` | GET | X470 | not probed |
| `/api/settings/sasit_management/sasit_controller_info` | GET | X470 | 405 |
| `/api/settings/sasit_management/sasit_controller_info/:id` | GET | X470 | not probed |
| `/api/settings/sasit_management/sasit_physical_device_info` | GET | X470 | 405 |
| `/api/settings/sasit_management/sasit_physical_device_info/:id` | GET | X470 | not probed |
| `/api/settings/sasit_management/sasit_topology_info` | GET | X470 | 405 |
| `/api/settings/sasit_management/sasit_topology_info/:id` | GET | X470 | not probed |

## Virtual media / remote storage (14)

| Path | Methods | Handler (boards) | Live 2.32 status |
|---|---|---|---|
| `/api/settings/media/general` | GET,PUT | B650,B650,X470 | 200 |
| `/api/settings/media/instance` | GET,PUT | B650,B650,X470 | 200 |
| `/api/settings/media/local/configurations` | GET | B650,B650,X470 | 200 |
| `/api/settings/media/local/file` | POST | B650,B650,X470 | not probed |
| `/api/settings/media/local/image` | DELETE | B650,B650,X470 | not probed |
| `/api/settings/media/local/images` | GET | B650,B650,X470 | 200 |
| `/api/settings/media/local/lmediasizecheck` | POST | B650,B650,X470 | not probed |
| `/api/settings/media/local/start-media` | POST | B650,B650,X470 | not probed |
| `/api/settings/media/local/stop-media` | POST | B650,B650,X470 | not probed |
| `/api/settings/media/remote/configurations` | GET | B650,B650,X470 | 200 |
| `/api/settings/media/remote/images` | GET | B650,B650,X470 | 200 |
| `/api/settings/media/remote/rmediarefreshlist` | GET,POST | B650,B650,X470 | 404 |
| `/api/settings/media/remote/start-media` | POST | B650,B650,X470 | not probed |
| `/api/settings/media/remote/stop-media` | POST | B650,B650,X470 | not probed |

## Field and purpose detail


### Automation engine (scripts/tasks)

**`/api/ae_test`** — PUT
  - request: code; id; name
  - response: error; log

**`/api/scripts`** — GET, POST, PUT
  - request: author; code; id (B650 only); name
  - response: id (B650 only); name (B650 only)

**`/api/tasks`** — GET, POST
  - request: event_filters; schedule; scripts; task_title
  - response: author (B650 only); event_filters; id (B650 only); name (B650 only); schedule; scripts; task_title; timestamp (B650 only)

**`/api/tasks/:id`** — GET, POST
  - request: event_filters; schedule; scripts; task_title
  - response: author (B650 only); event_filters; id (B650 only); name (B650 only); schedule; scripts; task_title; timestamp (B650 only)


### Dashboard & identity

**`/api/asrr/fw-info`** — GET
  - response: BIOS_fw_version; BMC_fw_version; BP_version (only in b650d4u.har); CPLD_version; ME_fw_version; Micro_Code_version; PSP_fw_version (only in b650d4u-2t.har, b650d4u.har); BPCPLD_version (read by B650 UI; absent from capture)
  - sample (redacted): `{ "BMC_fw_version": "2.32.00", "BIOS_fw_version": "P4.20", "ME_fw_version": "0.11.0.65", "Micro_Code_version": "0a50000b", "CPLD_version": "N\/A" }`
  - note: HAR-confirmed on X470: BMC_fw_version, BIOS_fw_version, ME_fw_version, Micro_Code_version, CPLD_version.

**`/api/asrr/model-name`** — GET
  - response: mb_name; sys_name (only in b650d4u-2t.har, b650d4u.har)
  - sample (redacted): `{ "mb_name": "X470D4U" }`
  - note: HAR-confirmed {"mb_name":"X470D4U"}.

**`/api/asrr/product-name`** — GET
  - response: sys_name
  - sample (redacted): `{ "sys_name": "N\/A" }`
  - note: Returned HTTP 500 {"error":"Error in Getting Model Name","code":16005} in the X470 HAR (no product FRU).

**`/api/asrr/selsummary`** — GET
  - response: freeentries; selentries
  - sample (redacted): `{ "selentries": 566, "freeentries": 3073 }`
  - note: Dashboard SEL usage donut. HAR-confirmed {"selentries":489,"freeentries":3150}.

**`/api/firmware-info`** — GET, PUT
  - response: active_image; aux; date; dev_support; device_id; fw_rev_1; fw_rev_2; fw_ver; ipmi_version; mfg_id_0; mfg_id_1; mfg_id_2
  - sample (redacted): `{ "device_id": 32, "revision": 1, "fw_rev_1": 2, "fw_rev_2": 50, "ipmi_version": 2, "dev_support": 191, "mfg_id_0": 214, "mfg_id_1": 193, "mfg_id_2": 0, "prod_id": 514, "aux": 0, "fw_ver": "2.32.00", "date": "May 11 2021", "time": "17:53:41…`
  - note: HAR-confirmed: device_id, revision, fw_rev_1, fw_rev_2, ipmi_version, dev_support, mfg_id_0..2, prod_id, aux, fw_ver, date, time, active_image. The PUT comes from a generic save() handler and is not reachable from the page.


### Feature discovery & UI configuration

**`/api/configuration/project`** — GET
  - response: feature
  - sample (redacted): `[ { "feature": "NWLINK" }, { "feature": "SYSTEM_FIREWALL" }, { "feature": "IPV6_SUPPORT" }, { "feature": "EXTENDED_PRIV" }, { "feature": "NWBONDING" }, { "feature": "SERVICES" }, { "feature": "SESSION_MANAGEMENT" }, { "feature": "FIRMWARE_F…`
  - note: Static compile-time feature list; drives data-feature visibility. HAR-confirmed.

**`/api/configuration/runtime`** — GET
  - response: enabled; feature
  - sample (redacted): `[ { "feature": "LDAP", "enabled": 0 }, { "feature": "AD", "enabled": 0 }, { "feature": "RADIUS", "enabled": 0 }, { "feature": "SINGLE_PORT_APP", "enabled": 1 }, { "feature": "SD_MEDIA_SUPPORT", "enabled": 0 } ]`
  - note: Runtime-toggleable features (LDAP, AD, RADIUS, SINGLE_PORT_APP, SD_MEDIA_SUPPORT ...). HAR-confirmed.

**`/api/status/uptime`** — GET
  - response: minutes_per_count; poh_counter_reading
  - sample (redacted): `{ "minutes_per_count": 60, "poh_counter_reading": 29011 }`
  - note: Power-on-hours counter (HAR-confirmed on B650D4U/B550D4U-2T).


### Host inventory

**`/api/system_inventory/component_status_update`** — PUT
  - request: DEV_INSTANCE; DEV_LED; DEV_TYPE; STATUS


### KVM, H5Viewer & SOL

**`/api/asrr/java-console`** — GET
  - sample (redacted): `<?xml version="1.0" encoding="UTF-8"?> <jnlp spec="1.0+" codebase="https://<REDACTED_IP>:443/Java"> <information> <title>JViewer</title> <vendor>American Megatrends, Inc.</vendor> <description kind="one-line">JViewer Console Redirection App…`
  - note: Returns the JNLP text; the UI saves it as jviewer.jnlp (Java KVM). B650 equivalent is /api/remote_control/get/kvm/launch (only when the kvm service viewer_count == 0).

**`/api/kvm/token`** — GET
  - response: client_ip; session (web session cookie value); token (KVM SSI hash)
  - sample (redacted): `{ "client_ip": "<REDACTED_IP>", "token": "<REDACTED>", "session": "<REDACTED>" }`
  - note: Fetched by viewer.html (models/kvm_token) when KVM starts. The result is stored in sessionStorage.token; token is written into the first /kvm WebSocket packet (CMD_CONNECTION_COMPLETE / validate-video-session packet, SSI_HASH_SIZE bytes) together with client_ip, username and MAC. The same token authenticates the /cd-server virtual media WebSocket. Error code 1000 = KVM_TOKEN_GEN_FAILED.

**`/api/settings/media/adviser`** — GET
  - response: keyboard_layout; kvm_client (only in b650d4u.har); kvm_port; license; mouse_mode; retry_count; retry_interval; secure_channel; singleport_status; status; web_port; keyboard_language (read by UI; absent from capture)
  - sample (redacted): `{ "license": 1, "status": 1, "secure_channel": 0, "kvm_port": 7578, "mouse_mode": 2, "web_port": 443, "singleport_status": 1, "retry_count": 3, "retry_interval": 10, "keyboard_layout": "<REDACTED>" }`
  - note: KVM service (adviserd) config. remotePopUpView disables the Launch button when status == 0. Field list HAR-confirmed on X470.

**`/api/settings/media/remotesession`** — GET, PUT
  - request: automatic_off; keyboard_language; kvm_client (B650 only); kvm_encryption; local_monitor_off; retry_count; retry_time_interval; single_port; vmedia_attach; vnc_non_secure (B650 only); vnc_over_ssh (B650 only); vnc_over_stunnel (B650 only)
  - response: automatic_off; keyboard_language; kvm_client (B650 only); kvm_encryption; local_monitor_off; retry_count; retry_time_interval; single_port; vmedia_attach; vnc_non_secure (B650 only); vnc_over_ssh (B650 only); vnc_over_stunnel (B650 only)
  - sample (redacted): `{ "id": 1, "single_port": 1, "kvm_encryption": 0, "keyboard_language": "<REDACTED>", "local_media_enable": 40, "remote_media_enable": 0, "vmedia_attach": 0, "retry_count": 3, "retry_time_interval": 10, "local_monitor_off": 1, "automatic_off…`

**`/api/settings/mouse`** — GET, PUT
  - request: mode
  - response: mode
  - sample (redacted): `{ "id": 1, "mode": "absolute" }`


### Logs, crash dumps & diagnostics

**`/api/asrr/clearmcalog`** — DELETE
  - note: DELETE, no body.

**`/api/asrr/getmcalog`** — GET
  - response: MCADump
  - sample (redacted): `{ "MCADump": 0 }`
  - note: Shows the MCA log save/clear buttons when MCADump is non-zero. HAR-confirmed.

**`/api/asrr/mca-file`** — GET
  - note: Text download (MCALog).

**`/api/logs/audit`** — GET
  - request: cmd
  - response: audit_count (only in b650d4u.har); hostname (only in b650d4u-2t.har); id (only in b650d4u-2t.har); message (only in b650d4u-2t.har); timestamp (only in b650d4u-2t.har)
  - sample (redacted): `[ { "id": 0, "timestamp": 1794668769, "hostname": "localhost", "message": "spx_restservice: spx_restservice - [3035 : 3035 INFO]http Login from IP:<REDACTED_IP> user:<REDACTED_PASSWORD> - " }, { "id": 1, "timestamp": 1794668780, "hostname":…`

**`/api/logs/event`** — DELETE, GET
  - response: event_description; event_direction (X470 only); event_format_ipmi_version (X470 only); event_reading_class (X470 only); EventData1 (X470 only); EventData1_Description (X470 only); EventData1_Description_raw (X470 only); EventData2 (X470 only); EventData2_Description (X470 only); EventData2_Description_raw (X470 only); EventData3 (X470 only); EventData3_Description (X470 only)
  - note: GET = SEL (collection/event-log; B650 polls it every 15 s with ?LASTEVENTID=n). DELETE /api/logs/event (no id) clears the SEL. Per-item DELETE is Backbone boilerplate.

**`/api/logs/event-file`** — GET
  - note: Text download of the decoded SEL (saved as SELLog).

**`/api/logs/event/:id`** — DELETE, GET
  - response: event_description; event_direction (X470 only); event_format_ipmi_version (X470 only); event_reading_class (X470 only); EventData1 (X470 only); EventData1_Description (X470 only); EventData1_Description_raw (X470 only); EventData2 (X470 only); EventData2_Description (X470 only); EventData2_Description_raw (X470 only); EventData3 (X470 only); EventData3_Description (X470 only)
  - note: GET = SEL (collection/event-log; B650 polls it every 15 s with ?LASTEVENTID=n). DELETE /api/logs/event (no id) clears the SEL. Per-item DELETE is Backbone boilerplate.

**`/api/logs/eventlog`** — GET
  - response: advanced_event_description (only in x470d4u.har); channel_number; event_description; event_direction; event_direction_table (only in b650d4u-2t.har); event_format_ipmi_version; event_log_count (only in b650d4u-2t.har); event_reading_class; EventData1; EventData1_Description; EventData1_Description_raw (only in x470d4u.har); EventData2
  - note: X470 EventLogView fetches the SEL from here (collection.fetch({url:"/api/logs/eventlog"})). HAR-confirmed array of SEL records (id, record_type, timestamp, GenID1, GenID2, sensor_type, sensor_name, sensor_number, event_direction, ...).

**`/api/logs/getvideotag`** — POST
  - request: file_name
  - response: CompleteTag

**`/api/logs/help-data`** — GET
  - request: file (query)
  - response: (text or image/jpeg blob)
  - sample (redacted): `KVM Remote Console Utility Version <=jviewerversion=> Plugin Version <=socversion=> for <=soc=> Copyright (c) 2016 American Megatrends, Inc.`

**`/api/logs/raw-event-file`** — GET
  - note: Text download of the raw SEL (saved as RawSELLog).

**`/api/logs/video-data`** — GET
  - note: Recorded-video binary (arraybuffer), ?file=<name>. HTTP 12012 = file too large for the HTML5 player.

**`/api/logs/video-log/delete`** — DELETE
  - request: file_name

**`/api/settings/log`** — GET, PUT
  - request: audit_log; file_size; local (B650 only); port; port_type (B650 only); remote; rotate_count; server_addr; system_log
  - response: audit_log; file_size; local (B650 only); port; port_type (B650 only); remote; rotate_count; server_addr; system_log
  - sample (redacted): `{ "id": 1, "audit_log": 1, "system_log": 1, "file_size": 50000, "rotate_count": 0, "port": 0, "server_addr": "", "remote": 1 }`

**`/api/settings/log-policy`** — GET, PUT
  - request: policy
  - response: policy
  - sample (redacted): `{ "id": 1, "policy": 0 }`


### Maintenance & firmware update

**`/api/asrr/maintenance/BIOS/PublicKey`** — POST
  - request: multipart:publickey
  - response: response (1 ok)
  - note: X470 only: upload a BIOS signing public key.

**`/api/asrr/maintenance/BIOS/configuration`** — POST
  - request: action (2 = preserve BIOS settings/NVRAM, 4 = clear)
  - response: response (1 ok, 2 error)
  - note: Body is JSON.stringify({action}) but sent with contentType:false, so the Content-Type header is the jQuery default for a string body (text/plain or none). Proxy should send the raw JSON string. X470 sends the value as a string ("2"/"4"), B650 as an int.

**`/api/asrr/maintenance/BIOS/firmware`** — POST
  - request: multipart:fwimage (BIOS ROM)
  - response: response (1 ok, 2 verify failed); curversion (B650 only); newversion (B650 only)
  - note: ASRock OEM BIOS image upload (multipart). X470 repeats every 90 s until success. Same shape for CPLD, PSU, BP, FPGA, RETIMER on B650 (CPLD also returns cpldbpcurversion).

**`/api/asrr/maintenance/BIOS/status`** — GET
  - response: response (4 in progress, 1 completed (X470 msg 1), 2 failed, 3 completed (B650: success)), anything else = keep polling
  - sample (redacted): `{ "response": 3 }`
  - note: Polled recursively; when response == 4 the UI also reads flash-progress.

**`/api/asrr/maintenance/BIOS/upgrade`** — POST
  - request: action (1 flash on next host power-off, 2 flash immediately without power action, 3 shut host down and flash)
  - response: response (1 started, 2 error)
  - note: Same raw-JSON/contentType:false quirk as configuration.

**`/api/asrr/maintenance/CPLD/configuration`** — POST
  - request: action
  - response: response

**`/api/asrr/maintenance/CPLD/firmware`** — POST
  - request: multipart:fwimage
  - response: cpldbpcurversion; curversion; newversion; response

**`/api/asrr/maintenance/CPLD/upgrade`** — POST
  - request: action
  - response: response

**`/api/maintenance/ValidSignImage`** — GET
  - response: public_key_info
  - note: Re-read after uploading a signing key to confirm it was accepted.

**`/api/maintenance/backup_config`** — GET, PUT
  - request: authentication; ipmi; kvm; network; ntp; snmp; syslog
  - response: authentication; ipmi; kvm; network; ntp; snmp; syslog
  - sample (redacted): `{ "id": 1, "snmp": 0, "kvm": 0, "network": 0, "ipmi": 0, "ntp": 0, "authentication": 0, "syslog": 0 }`

**`/api/maintenance/bmc_recovery`** — GET, PUT
  - request: bootRetryCount; forceRecovery; imageName; recoveryRetryCount; serverAddress
  - response: bootRetryCount; forceRecovery; imageName; recoveryRetryCount; serverAddress

**`/api/maintenance/dual_image_config`** — GET, PUT
  - request: boot_image
  - response: boot_image; fw_version_1; fw_version_2; state_1; state_2

**`/api/maintenance/firmware`** — POST
  - request: multipart:fwimage (the .ima BMC image)
  - note: X470 BMC image upload (multipart/form-data, field fwimage, contentType false, processData false). The UI re-issues the upload every 90 s until it succeeds (httpsInterval). B650 uses /api/maintenance/firmware/firmware for the same purpose (the dual-image and HPM views still use /api/maintenance/firmware).

**`/api/maintenance/firmware/dwldfwimg`** — PUT
  - request: PROTO_TYPE (1 = TFTP); preserve_config (B650 only)
  - note: TFTP path: after PUT /api/maintenance/flash, asks the BMC to download the image from the server configured in /api/maintenance/fwimage_location. Then poll GET dwldfwstatus-progress until progress == 100, then continue at verification.

**`/api/maintenance/firmware/flash-progress`** — GET
  - response: state (undefined until flashing starts; 1-5 while active); progress (string, e.g. "45% done" / "Complete"); action (string, e.g. "Flashing...")
  - sample (redacted): `{ "id": 1, "action": "", "progress": "", "state": 1 }`
  - note: Polled back-to-back (next request issued from the success handler, no delay). X470: done when parseInt(progress) == 100 or NaN; B650: continues until progress contains "Complete" then issues PUT /api/maintenance/firmware/VerifyImageFlash and polls again.

**`/api/maintenance/firmware/upgrade`** — PUT
  - request: preserve_config (0|1); preserve_network (X470 only, 0|1); preserve_user (X470 only, 0|1); flash_status (1 full/force flash, 2 section flash, 4 version-compare flash [B650]); section_count (when flash_status == 2); section_names (array of current_image_name, when flash_status == 2)
  - note: Starts flashing. JSON body. Follow with GET flash-progress polling.

**`/api/maintenance/firmware/verification`** — GET, POST
  - response: [] array of sections: current_image_name; current_image_version1; new_image_version; section_status (1 = section identical / not selectable); verification_status (bitmask on element 0: 1 differs, 2 size differs, 4 version differs, 16 = full flash required, 32 same version, 64 module size differs)
  - note: Validates the uploaded image (signature, board/platform match). Error codes in responseJSON.code: 14540, 14543, 14545, 14549, 14551, 14552, 14553 (signature/platform/size mismatch, see i18n error_code_*). On error the UI calls POST /api/maintenance/reset.

**`/api/maintenance/flash`** — PUT
  - request: (X470: empty body, data:{} with JSON content type); preserve_config (B650 only, 0|1)
  - note: Puts the BMC into firmware-update mode (stops services, reserves RAM). Must precede the image upload. Any later failure must be followed by POST /api/maintenance/reset to leave update mode.

**`/api/maintenance/flash_image_config`** — PUT
  - request: image_update; reboot_bmc
  - note: X470 dual-image page: JSON {image_update, reboot_bmc} before the flash. B650 equivalent: /api/maintenance/dualflashimageconfig.

**`/api/maintenance/fwimage_location`** — GET, PUT
  - request: image_name; protocol_type; retry_count; server_address
  - response: id; image_name; protocol_type; retry_count; server_address; controller_id (read by B650 UI; absent from capture); controller_name (read by B650 UI; absent from capture); firmware_version (read by B650 UI; absent from capture); globalFwUpdateStatus (read by B650 UI; absent from capture); serial_number (read by B650 UI; absent from capture); vendor_type (read by B650 UI; absent from capture)
  - sample (redacted): `{ "id": 1, "protocol_type": "http", "server_address": "", "image_name": "", "retry_count": 0 }`
  - note: Firmware image source. protocol_type "http" (web upload during flash) or "tftp" (server_address, image_name, retry_count 0-255). HAR-confirmed {"id":1,"protocol_type":"http","server_address":"","image_name":"","retry_count":0}. Saved with PUT (Backbone save, singleton with id).

**`/api/maintenance/hpm`** — POST
  - request: multipart:vendor_type (B650 only)

**`/api/maintenance/hpm/activatecomponents`** — PUT
  - request: COMPONENT_ID

**`/api/maintenance/hpm/componentversions`** — GET
  - request: VERS_ID

**`/api/maintenance/hpm/exitupdatemode`** — PUT
  - request: COMPONENT_DATA_LEN; COMPONENT_ID; FWUPDATEID; IS_MMC; RAID_CID (B650 only)

**`/api/maintenance/hpm/flash`** — PUT
  - request: COMPONENT_DATA_LEN; COMPONENT_ID; FWUPDATEID; SECTION_FLASH
  - response: COMPONENT_DATA_LEN (X470 only); COMPONENT_ID (X470 only); FWUPDATEID (X470 only)

**`/api/maintenance/hpm/preparecomponents`** — PUT
  - request: COMPONENT_DATA_LEN; COMPONENT_ID; FWUPDATEID; IS_MMC (X470 only)
  - response: COMPONENT_DATA_LEN; COMPONENT_ID; FWUPDATEID

**`/api/maintenance/hpm/upgradestatus`** — GET
  - request: COMPONENT_ID (X470 only); COMPONENT_ID (query) (B650 only); FWUPDATEID (query) (B650 only)
  - response: COMPONENT_ID (X470 only); PROGRESS (X470 only)

**`/api/maintenance/hpm/verifyimage`** — PUT
  - request: COMPONENT_DATA_LEN; COMPONENT_ID; FWUPDATEID
  - response: COMPONENT_ID

**`/api/maintenance/hpm/verifyimagestatus`** — GET
  - request: COMPONENT_DATA_LEN (B650 only); COMPONENT_ID; FWUPDATEID (B650 only)
  - response: COMPONENT_ID; PROGRESS

**`/api/maintenance/nm_upload`** — GET, POST
  - request: multipart:fwimage; multipart:vendor_type (B650 only)

**`/api/maintenance/preserve`** — GET, PUT
  - request: authentication; automationEngine (B650 only); extlog; fru; ipmi; kvm; network; ntp; redfish; rsd (B650 only); sdr; sel; snmp; ssh; syslog; web
  - response: authentication; automationEngine (B650 only); extlog; fru; ipmi; kvm; network; ntp; redfish; rsd (B650 only); sdr; sel
  - sample (redacted): `{ "id": 1, "sdr": 0, "fru": 0, "sel": 0, "ipmi": 0, "network": 0, "ntp": 0, "snmp": 0, "ssh": 0, "kvm": 0, "authentication": 0, "syslog": 0, "web": 0, "redfish": 0 }`

**`/api/maintenance/reset`** — POST
  - request: flash_done (B650 only); image_flashed (B650 only); xhr (B650 only)
  - note: Body {} (X470 sends data:{} which jQuery serialises as an empty body). Reboots the BMC after flashing, or aborts update mode on cancel/error. The UI logs out and reloads after 120 s. Also sent from window.onbeforeunload while window.fw_upgrade is set.

**`/api/maintenance/restore_config`** — GET, PUT
  - request: config
  - response: config
  - sample (redacted): `{ }`

**`/api/maintenance/restore_defaults`** — GET, PUT
  - request: authentication; automationEngine (B650 only); extlog; fru; ipmi; kvm; network; ntp; redfish; rsd (B650 only); sdr; sel; snmp; ssh; syslog; web
  - response: authentication; fru; id; ipmi; kvm; network; ntp; redfish; sdr; sel; snmp; ssh
  - sample (redacted): `{ "id": 1, "sdr": 0, "fru": 0, "sel": 0, "ipmi": 0, "network": 0, "ntp": 0, "snmp": 0, "ssh": 0, "kvm": 0, "authentication": 0, "syslog": 0, "web": 0, "redfish": 0 }`

**`/api/maintenance/signed-image-key`** — GET, POST
  - request: multipart:publicKey (POST)
  - response: id; key_exists; public_key_info
  - sample (redacted): `{ "id": 1, "key_exists": 0, "public_key_info": "<REDACTED>" }`
  - note: Signed-image public key (feature ENC_IMAGE). GET HAR-confirmed {"id":1,"key_exists":0,"public_key_info":"<REDACTED>"}. After POST the UI re-reads /api/maintenance/ValidSignImage.

**`/api/maintenance/system_admin`** — GET, PUT
  - request: cpassword; enable_change_password; enable_user_access; password
  - response: enable_change_password; enable_user_access; id; username; cpassword (read by UI; absent from capture); password (read by UI; absent from capture)
  - sample (redacted): `{ "id": 1, "enable_change_password": 0, "enable_user_access": 1, "username": "sys<REDACTED_PASSWORD>" }`

**`/api/maintenance/upload_restore`** — POST
  - request: multipart:config


### Power, chassis & UID

**`/api/actions/chassis-led`** — POST
  - request: uid_command (int: 0 off, 1 blink, 2 on); blink_time (B650 only, models/chassis_status.led()); force_on (B650 only)
  - note: JSON body. X470 and B650 UI (miscellaneous/uidcontrolView via models/uidcontrol.uidcontrol()) send {"uid_command":N}. B650 models/chassis_status also defines led(blink_time, force_on) for the IPMI Chassis Identify form.

**`/api/actions/power`** — POST
  - request: power_command (int)
  - response: power_command
  - note: JSON body {"power_command":N}. 0 power off (hard), 1 power on, 2 power cycle, 3 hard reset, 5 ACPI/soft shutdown (orderly). B650 UI also offers 6 = boot to BIOS setup, implemented client-side as POST /api/actions/accessbiossetup followed by power_command 1 (if off) or 2 (if on); 6 is never sent. UI then re-reads /api/chassis-status after 10 s (20 s for cycle, 30 s for ACPI), up to 3 retries, expec

**`/api/asrr/PowerSource`** — GET
  - response: ac_input_current; ac_input_current_fault; ac_input_power; ac_input_power_fault; ac_input_voltage; ac_input_voltage_fault; dc_12v_output_current; dc_12v_output_current_fault; dc_12v_output_power; dc_12v_output_power_fault; dc_12v_output_voltage; dc_12v_output_voltage_fault
  - sample (redacted): `[ { "power_supply_status": 2, "ac_input_voltage": "N\/A", "ac_input_voltage_fault": 0, "ac_input_current": "N\/A", "ac_input_current_fault": 0, "dc_12v_output_voltage": "N\/A", "dc_12v_output_voltage_fault": 0, "dc_12v_output_current": "N\/…`
  - note: Array of PSU objects (PMBus). HAR-confirmed on X470 (values "N/A" without a PMBus PSU).

**`/api/asrr/bmccoldreset`** — PUT
  - note: PUT with data:{} (empty body). Cold-resets the BMC; UI logs out after a delay.

**`/api/asrr/setprochot`** — PUT
  - request: prochot

**`/api/chassis`** — GET
  - response: led_status; power_status
  - sample (redacted): `{ "power_status": 0, "led_status": 0 }`
  - note: X470 only. Polled by app every 5 s (models/chassis_status_poll) to drive the header host/UID icons. HAR-confirmed: {"power_status":0,"led_status":0}.

**`/api/chassis-status`** — GET
  - response: led_status; power_cycle_interval (only in b650d4u.har); power_status; power_on_after_flash (read by B650 UI; absent from capture)
  - sample (redacted): `{ "power_status": 0, "led_status": 0 }`
  - note: GET only (models/chassis_status). On B650 this model is also the 5 s header poll (App.Chassis). HAR-confirmed on X470. B650D4U HAR: {"power_status":1,"led_status":0,"power_cycle_interval":2000}.


### Sensors & SDR

**`/api/sdr`** — GET
  - response: id; name; owner_lun (only in b650d4u-2t.har); sensor_number; type; type_number
  - sample (redacted): `[ { "id": 1, "sensor_number": 1, "name": "3VSB", "type": "voltage", "type_number": 2 }, { "id": 2, "sensor_number": 2, "name": "5VSB", "type": "voltage", "type_number": 2 }, { "id": 3, "sensor_number": 3, "name": "VCPU", "type": "voltage", …`
  - note: HAR-confirmed array {id, sensor_number, name, type, type_number}.

**`/api/sensors`** — GET
  - request: severity (B650 only)
  - response: accessible; discrete_state; higher_critical_threshold; higher_non_critical_threshold; higher_non_recoverable_threshold; id; lower_critical_threshold; lower_non_critical_threshold; lower_non_recoverable_threshold; name; owner_id; owner_lun
  - note: HAR-confirmed array: id, sensor_number, name, owner_id, owner_lun, raw_reading, type, type_number, reading, sensor_state, discrete_state, settable_readable_threshMask, lower/higher_*_threshold, accessible (213 = not accessible), unit. Polled every 10 s on #dashboard and #sensors.

**`/api/settings/sensor-threshold`** — GET, PUT
  - request: higher_critical_threshold; higher_critical_threshold_enable (X470 only); higher_non_critical_threshold; higher_non_critical_threshold_enable (X470 only); higher_non_recoverable_threshold; higher_non_recoverable_threshold_enable (X470 only); lower_critical_threshold; lower_critical_threshold_enable (X470 only); lower_non_critical_threshold; lower_non_critical_threshold_enable (X470 only); lower_non_recoverable_threshold; lower_non_recoverable_threshold_enable (X470 only); retain_threshold_value (B650 only); Settable_Readable_ThreshMask (X470 only)
  - response: higher_critical_threshold; higher_critical_threshold_enable (X470 only); higher_non_critical_threshold; higher_non_critical_threshold_enable (X470 only); higher_non_recoverable_threshold; higher_non_recoverable_threshold_enable (X470 only); id (B650 only); lower_critical_threshold; lower_critical_threshold_enable (X470 only); lower_non_critical_threshold; lower_non_critical_threshold_enable (X470 only); lower_non_recoverable_threshold


### Session & authentication

**`/api/asrr/session`** — POST
  - request: username (form, fixed "CERT_ADMIN"); password (form, empty); certlogin (form, "1")
  - response: CSRFToken; extendedpriv; ok; privilege; racsession_id; server_addr
  - note: ASRock variant used only when document.TRUSTED_CERT == 1 (index_trusted.html, client-certificate login). Same response as /api/session.

**`/api/reset-pass`** — POST
  - request: param (B650 only); Password (B650 only); RetypePassword (B650 only); username; verifyOTP (B650 only)
  - response: code (B650 only)
  - note: Forgot-password flow. X470 sends form {username}; response code & 0xFF: 0 ok, 17 SMTP disabled, 18-20 SMTP failures, 3/16/21 e-mail errors. B650 adds Password, RetypePassword, param, verifyOTP.

**`/api/session`** — DELETE, POST
  - request: username (form); password (form); certlogin (form, X470 only: "0" | "1")
  - response: ok (0 = success); privilege (1 callback, 2 user, 3 operator, 4 admin, 5 OEM); extendedpriv (bit0 = KVM, bit1 = VMedia); racsession_id; remote_addr; server_name; server_addr; HTTPSEnabled; CSRFToken; user_id (B650 only); channel (B650 only); passwordStatus (B650 only)
  - note: POST body is application/x-www-form-urlencoded (jQuery default), not JSON. Response CSRFToken must be echoed as X-CSRFTOKEN on every later request; the server sets the QSESSIONID cookie. Error body {code, error}; code 15000 = MAX_NUM_SESSIONS_ALREADY_IN_USE. DELETE (no body) logs out; X470 sends it synchronously (async:false) from app.logout and on window unload. HAR-confirmed on X470.

**`/api/viewerlogin`** — POST
  - request: token
  - response: CSRFToken; extendedpriv; ok; privilege; racsession_id; server_addr
  - note: H5Viewer standalone login: viewer.html#<token> (or ?<token>); body is form-encoded {token}. Same response shape as /api/session; sets X-CSRFTOKEN/garc. Lets a KVM window be opened without the main UI session.


### Settings: LDAP / AD / RADIUS

**`/api/settings/active-directory-settings`** — GET, PUT
  - request: domain_controller1; domain_controller2; domain_controller3; enable; enablessl (B650 only); secret_password; secret_username; user_domain_name
  - response: domain_controller1; domain_controller2; domain_controller3; enable; enablessl (B650 only); secret_password; secret_username; user_domain_name
  - sample (redacted): `{ "id": 1, "enable": 0, "user_domain_name": "", "secret_username": "<REDACTED>", "domain_controller1": "", "domain_controller2": "", "domain_controller3": "" }`

**`/api/settings/active-directory-users`** — GET
  - request: role_group_domain; role_group_kvm_privilege; role_group_name; role_group_privilege; role_group_vmedia_privilege; role_group_withoem_privilege
  - response: id; role_group_domain; role_group_kvm_privilege; role_group_name; role_group_privilege; role_group_vmedia_privilege; role_group_withoem_privilege

**`/api/settings/active-directory-users/:id`** — GET
  - request: role_group_domain; role_group_kvm_privilege; role_group_name; role_group_privilege; role_group_vmedia_privilege; role_group_withoem_privilege
  - response: id; role_group_domain; role_group_kvm_privilege; role_group_name; role_group_privilege; role_group_vmedia_privilege; role_group_withoem_privilege

**`/api/settings/ldap-certificates`** — POST
  - request: bind_dn (B650 only); multipart:ca_certificate_file; multipart:certificate_file; multipart:private_key; password (B650 only); port (B650 only); search_base (B650 only); server_address (B650 only)

**`/api/settings/ldap-settings`** — GET, PUT
  - request: bind_dn; common_name_type; enable; encryption_type; password; port; search_base; server_address; user_login_attribute
  - response: bind_dn; ca_certificate_file; certificate_file; common_name_type; enable; encryption_type; port; private_key; search_base; server_address; user_login_attribute
  - sample (redacted): `{ "id": 1, "enable": 0, "encryption_type": 0, "port": 389, "server_address": "", "bind_dn": "", "search_base": "", "user_login_attribute": "cn", "common_name_type": "ip" }`

**`/api/settings/ldap-users`** — GET
  - request: role_group_domain; role_group_kvm_privilege; role_group_name; role_group_privilege; role_group_vmedia_privilege; role_group_withoem_privilege
  - response: id; role_group_domain; role_group_kvm_privilege; role_group_name; role_group_privilege; role_group_vmedia_privilege; role_group_withoem_privilege

**`/api/settings/ldap-users/:id`** — GET
  - request: role_group_domain; role_group_kvm_privilege; role_group_name; role_group_privilege; role_group_vmedia_privilege; role_group_withoem_privilege
  - response: id; role_group_domain; role_group_kvm_privilege; role_group_name; role_group_privilege; role_group_vmedia_privilege; role_group_withoem_privilege

**`/api/settings/radius/advanced`** — GET, PUT
  - request: administrator; none; oem; operator; user
  - response: administrator; none; oem; operator; user

**`/api/settings/radius/general`** — GET, PUT
  - request: enable; kvm; port; secret; server_address; vmedia
  - response: enable; kvm; port; secret; server_address; vmedia
  - sample (redacted): `{ "id": 1, "enable": 0, "server_address": "", "port": 1812, "secret": "<REDACTED>", "kvm": 0, "vmedia": 0 }`


### Settings: SMTP, PEF, alerts & SNMP

**`/api/settings/pef/alert_policies`** — GET
  - request: alert_string; alert_string_key; channel_number; destination_id; enable_policy; policy_action; policy_group
  - response: channel_number; destination_id (B650 only); enable_policy (B650 only); id; policy_action (B650 only); policy_group (B650 only)
  - sample (redacted): `[ { "id": 1, "policy_group": "1", "enable_policy": 0, "policy_action": "always_send_alert", "channel_number": 1, "destination_id": "0", "alert_string": 0, "alert_string_key": "<REDACTED>" }, { "id": 2, "policy_group": "2", "enable_policy": …`

**`/api/settings/pef/alert_policies/:id`** — GET
  - request: alert_string; alert_string_key; channel_number; destination_id; enable_policy; policy_action; policy_group
  - response: channel_number; destination_id (B650 only); enable_policy (B650 only); id; policy_action (B650 only); policy_group (B650 only)

**`/api/settings/pef/event_filters`** — GET
  - request: alert (B650 only); channel_number (X470 only); enable_filter; event_data_1_and_mask; event_data_1_compare_1; event_data_1_compare_2; event_data_1_offset_mask; event_data_2_and_mask; event_data_2_compare_1; event_data_2_compare_2; event_data_3_and_mask; event_data_3_compare_1; event_data_3_compare_2; event_options; event_trigger; generator_id_1; generator_id_2; generator_type; ipmb_lun (X470 only); name; policy_group; power_action; raw_data; sensor_type; slave_addr_software_id; trigger_event_severity
  - response: alert (B650 only); discrete_state (B650 only); enable_filter; id; name; policy_group (B650 only); power_action (B650 only); sensor_state (B650 only); trigger_event_severity (B650 only); type_number

**`/api/settings/pef/event_filters/:id`** — GET
  - request: alert (B650 only); channel_number (X470 only); enable_filter; event_data_1_and_mask; event_data_1_compare_1; event_data_1_compare_2; event_data_1_offset_mask; event_data_2_and_mask; event_data_2_compare_1; event_data_2_compare_2; event_data_3_and_mask; event_data_3_compare_1; event_data_3_compare_2; event_options; event_trigger; generator_id_1; generator_id_2; generator_type; ipmb_lun (X470 only); name; policy_group; power_action; raw_data; sensor_type; slave_addr_software_id; trigger_event_severity
  - response: alert (B650 only); discrete_state (B650 only); enable_filter; id; name; policy_group (B650 only); power_action (B650 only); sensor_state (B650 only); trigger_event_severity (B650 only); type_number

**`/api/settings/pef/lan_destinations`** — GET
  - request: destination_address; destination_type; format; message; name; subject
  - response: channel_id; destination_address; destination_type; email_format; email_id; id; lan_channel; message; name; snmp (B650 only); snmp_access (B650 only); snmp_authentication_protocol (B650 only)
  - sample (redacted): `[ { "channel_id": 1, "id": 1, "destination_address": "", "name": "", "lan_channel": 1, "destination_type": "snmp", "subject": "", "message": "" }, { "channel_id": 2, "id": 2, "destination_address": "", "name": "", "lan_channel": 1, "destina…`

**`/api/settings/pef/lan_destinations/:id`** — GET
  - request: destination_address; destination_type; format; message; name; subject
  - response: channel_id; destination_address; destination_type; email_format; email_id; id; lan_channel; message; name; snmp (B650 only); snmp_access (B650 only); snmp_authentication_protocol (B650 only)

**`/api/settings/pef/send_test_alert`** — POST
  - request: channel_id; destination_address; destination_type; id; lan_channel; message; subject; username

**`/api/settings/smtp`** — GET
  - request: email_id; primary_password; primary_server_ip; primary_server_name; primary_smtp_authentication; primary_smtp_enable; primary_smtp_port; primary_smtp_secure_port; primary_ssltls_enable; primary_starttls_enable; primary_username; secondary_password; secondary_server_ip; secondary_server_name; secondary_smtp_authentication; secondary_smtp_enable; secondary_smtp_port; secondary_smtp_secure_port; secondary_ssltls_enable; secondary_starttls_enable; secondary_username; smtp_default_enable
  - response: ca_info1; ca_info2; cert_info1; cert_info2; channel_interface; email_id; key_info1; key_info2; primary_password; primary_server_ip; primary_server_name; primary_smtp_authentication
  - sample (redacted): `[ { "id": 1, "channel_interface": "eth0", "email_id": "", "primary_smtp_enable": 1, "primary_server_name": "", "primary_smtp_port": 25, "primary_smtp_secure_port": 465, "primary_smtp_authentication": 0, "primary_server_ip": "", "primary_use…`

**`/api/settings/smtp/:id`** — GET
  - request: email_id; primary_password; primary_server_ip; primary_server_name; primary_smtp_authentication; primary_smtp_enable; primary_smtp_port; primary_smtp_secure_port; primary_ssltls_enable; primary_starttls_enable; primary_username; secondary_password; secondary_server_ip; secondary_server_name; secondary_smtp_authentication; secondary_smtp_enable; secondary_smtp_port; secondary_smtp_secure_port; secondary_ssltls_enable; secondary_starttls_enable; secondary_username; smtp_default_enable
  - response: ca_info1; ca_info2; cert_info1; cert_info2; channel_interface; email_id; key_info1; key_info2; primary_password; primary_server_ip; primary_server_name; primary_smtp_authentication

**`/api/settings/smtp/smtpupload`** — POST
  - request: multipart:auth (B650 only); multipart:upload_cacert1; multipart:upload_cacert2; multipart:upload_cert1; multipart:upload_cert2; multipart:upload_key1; multipart:upload_key2


### Settings: SSL

**`/api/operations/ssl/certificate/generate`** — GET, PUT
  - request: city; common_name; country; email_id; key_length; organization; organization_unit; state; valid_days
  - response: city; common_name; country; email_id; key_length; organization; organization_unit; state; valid_days
  - sample (redacted): `{ }`

**`/api/settings/ssl/certificate`** — GET, POST, PUT
  - request: multipart:new_ca_certificate (X470 only); multipart:new_certificate; multipart:new_private_key; multipart:trusted_ca_certificates (B650 only); multipart:trusted_ca_support (B650 only)
  - response: certificate_version; from_city; from_common_name; from_country; from_email_id; from_organization; from_organization_unit; from_state; public_key; serial_number; signature_algorithm; to_city
  - sample (redacted): `{ "id": 1, "certificate_version": "3", "serial_number": "<REDACTED_HEX>", "signature_algorithm": "sha256WithRSAEncryption", "public_key": "<REDACTED>", "from_common_name": "AMI", "from_organization": "American Megatrends Inc", "from_organiz…`

**`/api/settings/ssl/certificate-info`** — GET, PUT
  - request: trusted_ca_support (B650 only)
  - response: ca_certificate_available (X470 only); ca_certificate_date (X470 only); certificate_available; certificate_date; current_private_key; private_key_date; trusted_ca_certificate_available (B650 only); trusted_ca_certificate_date (B650 only); trusted_ca_support (B650 only)
  - sample (redacted): `{ "id": 1, "certificate_available": 1, "certificate_date": "Tue May 11 09:45:32 2021\n", "private_key_date": "<REDACTED>" }`


### Settings: date & time / NTP

**`/api/settings/date-time`** — GET, PUT
  - request: auto_date (X470 only); daylight (X470 only); ntp_auto_date (B650 only); primary_ntp; ptp_auto_date (B650 only); ptp_delaymech (B650 only); ptp_inlatency (B650 only); ptp_interface (B650 only); ptp_ipmode (B650 only); ptp_logdelayint (B650 only); ptp_maxmasters (B650 only); ptp_outlatency (B650 only); ptp_panicmode (B650 only); ptp_preset (B650 only); ptp_priority1 (B650 only); ptp_transport (B650 only); ptp_unicastip (B650 only); secondary_ntp; timestamp; timezone; utc_minutes
  - response: auto_date (only in x470d4u.har); daylight (only in x470d4u.har); id; localized_timestamp; ntp_auto_date (only in b650d4u-2t.har); primary_ntp; secondary_ntp; timestamp; timezone; utc_minutes; interface_name (read by B650 UI; absent from capture); ntp_server_status (read by X470 UI; absent from capture)
  - sample (redacted): `{ "id": 1, "primary_ntp": "pool.ntp.org", "secondary_ntp": "time.nist.gov", "auto_date": 1, "localized_timestamp": 1794811789, "utc_minutes": 0, "daylight": 0, "timestamp": 1794811789, "timezone": "GMT+0" }`
  - note: HAR-confirmed {id, primary_ntp, secondary_ntp, auto_date, localized_timestamp, utc_minutes, daylight, timestamp, timezone}.


### Settings: fan control

**`/api/asrr/GetFanDuty`** — GET
  - response: FAN1_Duty; FAN2_Duty; FAN3_Duty; FAN4_Duty; FAN5_Duty; FAN6_Duty; FAN7_Duty; FAN8_Duty
  - sample (redacted): `{ "FAN1_Duty": "60", "FAN2_Duty": "60", "FAN3_Duty": "60", "FAN4_Duty": "60", "FAN5_Duty": "60", "FAN6_Duty": "60", "FAN7_Duty": "60", "FAN8_Duty": "60" }`
  - note: Read by host_interface_info. HAR-confirmed on X470.

**`/api/asrr/settings/setcloselooptablenumber`** — POST
  - request: table_numble

**`/api/asrr/settings/setfancloseloopcontroltable`** — POST
  - request: duty_cycle_percent_of_slowdown_fan_duty_cycle; duty_cycle_percent_of_speedup_fan_duty_cycle; table_numble; temp_of_slowdown_fan_duty_cycle; temp_of_speedup_fan_duty_cycle; time_of_slowdown_fan_duty_cycle; time_of_speedup_fan_duty_cycle

**`/api/asrr/settings/setfancontrolmode`** — POST
  - request: mode1; mode10; mode11; mode12; mode13; mode14; mode15; mode16; mode2; mode3; mode4; mode5; mode6; mode7; mode8; mode9

**`/api/asrr/settings/setfandutymanualmode`** — POST
  - request: duty1; duty10; duty11; duty12; duty13; duty14; duty15; duty16; duty2; duty3; duty4; duty5; duty6; duty7; duty8; duty9

**`/api/asrr/settings/setfanopenloopcontroltable`** — POST
  - request: duty1; duty10; duty11; duty12; duty13; duty14; duty15; duty16; duty17; duty18; duty19; duty2; duty20; duty21; duty22; duty23; duty24; duty3; duty4; duty5; duty6; duty7; duty8; duty9; table_numble; temp1; temp10; temp11; temp12; temp13; temp14; temp15; temp16; temp17; temp18; temp19; temp2; temp20; temp21; temp22; temp23; temp24; temp3; temp4; temp5; temp6; temp7; temp8; temp9

**`/api/asrr/settings/setfantempsensorandcorrespondfantable`** — POST
  - request: closetable_numble; opentable_numble; tscft_fan1; tscft_fan10; tscft_fan11; tscft_fan12; tscft_fan13; tscft_fan14; tscft_fan15; tscft_fan16; tscft_fan2; tscft_fan3; tscft_fan4; tscft_fan5; tscft_fan6; tscft_fan7; tscft_fan8; tscft_fan9; tscft_sensor_number

**`/api/asrr/settings/setopenlooptablenumber`** — POST
  - request: table_numble

**`/api/asrr/settings/settscftsensornumber`** — POST
  - request: tscft_sensor_number


### Settings: firewall

**`/api/settings/firewall`** — GET
  - request: block_all; end_time; flush_all; start_time; timeout
  - response: block_all; end_time; flush_all; id; start_time; timeout
  - sample (redacted): `[ ]`

**`/api/settings/firewall-ip-rule`** — GET, PUT
  - request: end_time; ip_end; ip_start; rule; start_time; timeout
  - response: end_time; ip_end; ip_start; rule; start_time; timeout
  - sample (redacted): `[ ]`

**`/api/settings/firewall-ip-rules`** — GET
  - request: end_time; ip_end; ip_start; rule; start_time; timeout
  - response: end_time; id; ip_end; ip_start; rule; start_time; timeout
  - sample (redacted): `[ ]`

**`/api/settings/firewall-ip-rules/:id`** — GET
  - request: end_time; ip_end; ip_start; rule; start_time; timeout
  - response: end_time; id; ip_end; ip_start; rule; start_time; timeout

**`/api/settings/firewall-port-rule`** — GET, PUT
  - request: data_flow; end_time; network_type; port_end; port_start; protocol; rule; start_time; timeout
  - response: data_flow; end_time; network_type; port_end; port_start; protocol; rule; start_time; timeout
  - sample (redacted): `[ ]`

**`/api/settings/firewall-port-rules`** — GET
  - request: end_time; network_type; port_end; port_start; protocol; rule; start_time; timeout
  - response: data_flow (X470 only); end_time; id; network_type; port_end; port_start; protocol; rule; start_time; timeout
  - sample (redacted): `[ ]`

**`/api/settings/firewall-port-rules/:id`** — GET
  - request: end_time; network_type; port_end; port_start; protocol; rule; start_time; timeout
  - response: data_flow (X470 only); end_time; id; network_type; port_end; port_start; protocol; rule; start_time; timeout


### Settings: licensing

**`/api/settings/license`** — GET, PUT
  - request: license_key
  - response: license_key


### Settings: network

**`/api/asrr/getvetobit`** — GET
  - response: vetobit
  - sample (redacted): `{ "vetobit": 0 }`
  - note: Keep NC-SI link up (veto bit). X470 UI; also seen live on B650D4U fw 1.10.00 ({"vetobit":0}) although the analysed B650 7.02/9.05 JS and binaries do not have it. setvetobit is PUT JSON {vetobit}.

**`/api/asrr/setvetobit`** — PUT
  - request: vetobit

**`/api/set/dns/registration`** — POST
  - request: reg_enable; reg_mdns; reg_option; reg_tsig

**`/api/settings/dns`** — GET, PUT
  - request: dns_iface; dns_manual; dns_priority; dns_server1; dns_server2; dns_server3; dns_status; domain_iface; domain_manual; domain_name; host_cfg; host_name; iface_name
  - response: dns_iface; dns_status; domain_iface; ethIndex; reg_enable; reg_ifc; reg_mdns; reg_option; reg_tsig0; reg_tsig1; reg_tsig2; tsig_both

**`/api/settings/dns/domain-options`** — GET
  - request: domain_name
  - sample (redacted): `[ { "id": 1, "domain_iface": "eth0_v6" }, { "id": 2, "domain_iface": "eth1_v4" }, { "id": 3, "domain_iface": "eth1_v6" } ]`

**`/api/settings/dns/registration`** — GET
  - request: iface_name; reg_enable; reg_mdns; reg_option; reg_tsig
  - response: reg_enable; reg_ifc; reg_mdns; reg_option; reg_tsig
  - sample (redacted): `[ { "id": 1, "ethIndex": 0, "reg_ifc": "eth0", "reg_enable": 1, "reg_option": 0, "reg_mdns": 0, "reg_tsig0": 0 }, { "id": 2, "ethIndex": 1, "reg_ifc": "eth1", "reg_enable": 0, "reg_option": 0, "reg_mdns": 0, "reg_tsig1": 0 } ]`

**`/api/settings/dns/registration/:id`** — GET
  - request: iface_name; reg_enable; reg_mdns; reg_option; reg_tsig
  - response: reg_enable; reg_ifc; reg_mdns; reg_option; reg_tsig

**`/api/settings/dns/restart`** — PUT
  - request: dns_status

**`/api/settings/dns/tsig-private-key`** — POST
  - request: multipart:index; multipart:tsig_newfile

**`/api/settings/ncsi/mode`** — GET, PUT
  - request: channel_number; interface_name; mode; package_id
  - response: active (B650 only); channel_number; interface_name; package_id
  - sample (redacted): `{ "id": 1, "mode": 0 }`

**`/api/settings/network`** — GET
  - request: interface_name; ipv4_address; ipv4_dhcp_enable; ipv4_enable; ipv4_gateway; ipv4_subnet; ipv6_address; ipv6_clear (B650 only); ipv6_dhcp_enable; ipv6_enable; ipv6_gateway (B650 only); ipv6_index; ipv6_prefix; lan_enable; mac_address; vlan_enable; vlan_id; vlan_priority
  - response: channel_number; id; interface_name; ipv4_address; ipv4_dhcp_enable; ipv4_enable; ipv4_gateway; ipv4_subnet; ipv6_address; ipv6_dhcp_enable; ipv6_enable; ipv6_gateway
  - sample (redacted): `[ { "id": 1, "interface_name": "eth0", "channel_number": 1, "lan_enable": 1, "mac_address": "<REDACTED_MAC>", "ipv4_enable": 1, "ipv4_dhcp_enable": 0, "ipv4_address": "<REDACTED_IP>", "ipv4_subnet": "255.255.255.0", "ipv4_gateway": "<REDACT…`
  - note: HAR-confirmed array per interface (id, interface_name, channel_number, lan_enable, mac_address, ipv4_*, ipv6_*, vlan_*). Saved per interface with PUT /api/settings/network/:id. Saving makes the BMC restart networking.

**`/api/settings/network-bond`** — GET, PUT
  - request: auto_configuration_enable; bond_enable; bond_ifc (B650 only)
  - response: auto_configuration_enable; bond_enable; bond_mode; id; interface_name (read by B650 UI; absent from capture)
  - sample (redacted): `{ "id": 1, "bond_enable": 0, "bond_mode": "active-backup", "auto_configuration_enable": 1 }`
  - note: HAR-confirmed {id, bond_enable, bond_mode, auto_configuration_enable}. B650D4U HAR confirms Backbone save semantics: PUT /api/settings/network-bond with the whole model as JSON ({"id":1,"bond_enable":0,"bond_mode":"active-backup","auto_configuration_enable":1,"bond_ifc":"eth0"}); the response echoes the saved object.

**`/api/settings/network-link`** — GET
  - request: auto_negotiation; duplex_mode; interface_name; link_speed
  - response: auto_negotiation; duplex_mode; duplex_mode_auto_negotiated; interface_name; link_speed; link_speed_auto_negotiated; ncsi_status (B650 only)
  - sample (redacted): `[ { "id": 1, "auto_negotiation": 1, "link_speed_auto_negotiated": 1000, "link_speed": 1000, "interface_name": "eth0", "duplex_mode_auto_negotiated": "FULL", "duplex_mode": "FULL" }, { "id": 2, "auto_negotiation": 1, "link_speed_auto_negotia…`

**`/api/settings/network-link/:id`** — GET
  - request: auto_negotiation; duplex_mode; interface_name; link_speed
  - response: auto_negotiation; duplex_mode; duplex_mode_auto_negotiated; interface_name; link_speed; link_speed_auto_negotiated; ncsi_status (B650 only)

**`/api/settings/network/:id`** — GET
  - request: interface_name; ipv4_address; ipv4_dhcp_enable; ipv4_enable; ipv4_gateway; ipv4_subnet; ipv6_address; ipv6_clear (B650 only); ipv6_dhcp_enable; ipv6_enable; ipv6_gateway (B650 only); ipv6_index; ipv6_prefix; lan_enable; mac_address; vlan_enable; vlan_id; vlan_priority
  - response: channel_number; id; interface_name; ipv4_address; ipv4_dhcp_enable; ipv4_enable; ipv4_gateway; ipv4_subnet; ipv6_address; ipv6_dhcp_enable; ipv6_enable; ipv6_gateway
  - note: HAR-confirmed array per interface (id, interface_name, channel_number, lan_enable, mac_address, ipv4_*, ipv6_*, vlan_*). Saved per interface with PUT /api/settings/network/:id. Saving makes the BMC restart networking.

**`/api/settings/static-ipv6`** — GET
  - response: interface_name; ipv6_address; ipv6_index; ipv6_prefix
  - sample (redacted): `[ { "ipv6_index": 0, "interface_name": "eth0", "ipv6_address": "", "ipv6_prefix": 0 }, { "ipv6_index": 1, "interface_name": "eth0", "ipv6_address": "", "ipv6_prefix": 0 }, { "ipv6_index": 2, "interface_name": "eth0", "ipv6_address": "", "ip…`
  - note: HAR-confirmed array {ipv6_index, interface_name, ipv6_address, ipv6_prefix}.


### Settings: services & sessions

**`/api/asrr/setWebEnableTimeout`** — POST
  - request: enable_timeout

**`/api/settings/services`** — GET
  - request: active_session; interface_name; maximum_sessions; mutual_port (X470 only); non_secure_port; secure_port; state; time_out
  - response: active_session; id; interface_name; maximum_sessions; non_secure_port; secure_port; service_id; service_name; singleport_status; state; time_out; viewer_count
  - sample (redacted): `[ { "id": 1, "service_id": 1, "service_name": "web", "state": 1, "interface_name": "both", "non_secure_port": 80, "secure_port": 443, "mutual_port": 4433, "time_out": 1800, "maximum_sessions": 148, "active_session": 129, "singleport_status"…`

**`/api/settings/services/:id`** — GET
  - request: active_session; interface_name; maximum_sessions; mutual_port (X470 only); non_secure_port; secure_port; state; time_out
  - response: active_session; id; interface_name; maximum_sessions; non_secure_port; secure_port; service_id; service_name; singleport_status; state; time_out; viewer_count

**`/api/settings/web_server_instances`** — GET, PUT
  - request: instances
  - response: instances
  - sample (redacted): `{ "id": 1, "instances": "1" }`


### Settings: users & PAM

**`/api/settings/pam-order`** — GET
  - request: pam_position
  - response: id; pam_module_name; pam_position
  - sample (redacted): `[ { "id": 1, "pam_position": 1, "pam_module_name": "IPMI" }, { "id": 2, "pam_position": 2, "pam_module_name": "LDAP" }, { "id": 3, "pam_position": 3, "pam_module_name": "ACTIVE DIRECTORY" }, { "id": 4, "pam_position": 4, "pam_module_name": …`

**`/api/settings/user/ssh-key-upload/:id`** — POST
  - request: multipart:upload_ssh_key
  - note: multipart field upload_ssh_key; :id = user id.

**`/api/settings/users`** — GET
  - request: access; access_privilege_changed (B650 only); accessByChannel (B650 only); changepassword; confirm_password; email_format; email_id; kvm; loggedin_password (B650 only); name; network_privilege (X470 only); password; password_age (B650 only); password_enddate (B650 only); password_size; privilege (B650 only); privilege_limit_serial (X470 only); privilegeByChannel (B650 only); setpasswordage (B650 only); snmp; snmp_access; snmp_authentication_protocol; snmp_privacy_protocol; snmp_view_privileges (B650 only); ssh_key; UserOperation; vmedia
  - response: access; accessByChannel (only in b650d4u-2t.har); channel (only in b650d4u-2t.har); channel_type (only in b650d4u-2t.har); creation_time; email_format; email_id; fixed_user_count; id; kvm; name; network_privilege (only in x470d4u.har)
  - sample (redacted): `[ { "id": 1, "name": "anonymous", "access": 0, "kvm": 1, "vmedia": 1, "snmp": 0, "prev_snmp": 0, "network_privilege": "<REDACTED_PASSWORD>istrator", "fixed_user_count": 2, "snmp_access": "", "OEMProprietary_level_Privilege": 1, "privilege_l…`
  - note: Fixed user slots (ids 1..N; slot 1 = anonymous, 2 = admin). Adding a user = PUT /api/settings/users/:id on an empty slot. HAR-confirmed list shape includes id, name, access, kvm, vmedia, snmp, network_privilege, fixed_user_count, snmp_access, OEMProprietary_level_Privilege, privilege_limit_serial, snmp_authentication_protocol, snmp_privacy_protocol, email_id, email_format, ssh_key, creation_time.

**`/api/settings/users/:id`** — DELETE
  - request: id; snmp_status
  - note: URL built with "/api/settings/users/:id".replace(":id", id); JSON body {snmp_status, id}.


### Settings: video recording / BSOD / SOL recording

**`/api/settings/download_image`** — GET
  - note: BSOD screenshot as an image blob (XHR with X-CSRFTOKEN).

**`/api/settings/video/pre-event`** — GET, PUT
  - request: comp_mode; enable; fps; video_duration; video_quality
  - response: comp_mode; enable; fps; id; video_duration; video_quality
  - sample (redacted): `{ "id": 1, "enable": 0, "video_quality": 0, "comp_mode": 0, "fps": 1, "video_duration": 10 }`

**`/api/settings/video/remote-storage`** — GET, PUT
  - request: domain_name; ip_address; max_dumps; max_duration; max_size; password; remote_path; remote_support; share_type; user_name
  - response: domain_name; id; ip_address; max_dumps; max_duration; max_size; mount_status; password; remote_path; remote_support; share_type; user_name
  - sample (redacted): `{ "id": 1, "remote_support": 0, "mount_status": -1, "max_dumps": 2, "max_duration": 20, "max_size": 5, "ip_address": "", "remote_path": "", "share_type": "NFS", "user_name": "", "password": "<REDACTED>", "domain_name": "" }`
  - note: HAR-confirmed {id, remote_support, mount_status, max_dumps, max_duration, max_size, ip_address, remote_path, share_type, user_name, password, domain_name}.

**`/api/settings/video/sol-remote-storage`** — GET, PUT
  - request: allowed_file_count (B650 only); allowed_file_size (B650 only); domain_name; ip_address; log_file_count; log_size; password; remote_path; remote_support; share_type; user_name
  - response: allowed_file_count (B650 only); allowed_file_size (B650 only); domain_name; ip_address; log_file_count; log_size; password; remote_path; remote_support; share_type; user_name

**`/api/settings/video/sol-triggers`** — GET, PUT
  - request: chasis_power_off_events_enable; chasis_power_on_events_enable; chasis_reset_events_enable; critical_events_enable; date_time_events_enable; date_time_to_record; fan_state_events_enable; lpc_reset_events_enable; non_critical_events_enable; non_recoverable_events_enable; NTPtimestamp; watchdog_events_enable
  - response: date_time_events_enable (B650 only); localized_timestamp

**`/api/settings/video/triggers`** — GET, PUT
  - request: chassis_power_off_events_enable; chassis_power_on_events_enable; chassis_reset_events_enable; critical_events_enable; date_time_events_enable; date_time_to_record; fan_state_events_enable; lpc_reset_events_enable; non_critical_events_enable; non_recoverable_events_enable; NTPtimestamp; pre_event_video; watchdog_events_enable
  - response: date_time_events_enable (B650 only); localized_timestamp
  - sample (redacted): `{ "id": 1, "critical_events_enable": 0, "non_critical_events_enable": 0, "non_recoverable_events_enable": 0, "fan_state_events_enable": 0, "watchdog_events_enable": 0, "chassis_power_on_events_enable": 0, "chassis_power_off_events_enable": …`


### Storage / PCIe device management

**`/api/settings/raid_management/create_virtual_drive`** — GET, PUT
  - request: accelerator (B650 only); accesspolicy; array_number (B650 only); CTRLID; deviceId; diskCachePolicy; initState; iopolicy; numdrives; parity_group_count (B650 only); raid_level; readpolicy; spandepth; StripeSize; VDNAME; VDSize (X470 only); vendor_type (B650 only); writepolicy
  - response: array_number (B650 only); controller_id; controller_name; dev_id; pd_type (B650 only); predective_fail_count; size; state; type; vendor_type (B650 only)

**`/api/settings/raid_management/delete_virtual_drive`** — DELETE
  - request: CTRLID; DEVICEID; VENDOR_TYPE (B650 only)

**`/api/settings/raid_management/logical_consistency_check_cmd`** — PUT
  - request: ACTION; CTRLID; DEVICEID

**`/api/settings/raid_management/logical_device_progress_cmd`** — PUT
  - request: CTRLID; DEVICEID
  - response: CONSISTENCY_PROGRESS

**`/api/settings/raid_management/logical_initialize_check_cmd`** — PUT
  - request: ACTION; CTRLID; DEVICEID; INIT_TYPE

**`/api/settings/raid_management/logical_start_stop_locate_cmd`** — PUT
  - request: ACTION; CTRLID; DEVICEID; DURATION
  - response: action (B650 only)

**`/api/settings/raid_management/physical_dedicated_hotspare_cmd`** — PUT
  - request: CMDPARAM (X470 only); CTRLID; DEVICEID; DISKCOUNT; DISKID

**`/api/settings/raid_management/physical_hotspare_cmd`** — PUT
  - request: CMDPARAM; CTRLID; DEVICEID

**`/api/settings/raid_management/physical_onlineoffline_cmd`** — PUT
  - request: CMDPARAM; CTRLID; DEVICEID

**`/api/settings/raid_management/physical_removal_cmd`** — PUT
  - request: CMDPARAM; CTRLID; DEVICEID

**`/api/settings/raid_management/physical_start_stop_locate_cmd`** — PUT
  - request: ACTION; CTRLID; DEVICEID; DURATION; VENDORTYPE (B650 only)
  - response: action (B650 only)

**`/api/settings/raid_management/raid_event_log`** — DELETE, GET
  - request: vendor_type (B650 only)
  - response: controller_name (B650 only); event_class (B650 only); event_class_code (B650 only); event_code (B650 only); event_desc (B650 only); event_detail_code (B650 only); event_message (B650 only); event_subclass_code (B650 only); event_tag (B650 only); event_type (B650 only); id (B650 only); record_id (B650 only)

**`/api/settings/raid_management/raid_levels`** — PUT
  - request: CTRLID; VENDOR_TYPE (B650 only)
  - response: iopolicy (B650 only)

**`/api/settings/raid_management/raid_pd_count`** — PUT
  - request: CTRLID; VENDOR_TYPE (B650 only)


### Virtual media / remote storage

**`/api/settings/media/general`** — GET, PUT
  - request: cd_kerberos_default_realm (B650 only); cd_kerberos_domain_realm1 (B650 only); cd_kerberos_domain_realm2 (B650 only); cd_kerberos_encryption_type (B650 only); cd_kerberos_kdc_server_name (B650 only); cd_kerberos_key_version_number (B650 only); cd_kerberos_principal_name (B650 only); cd_kerberos_principal_password (B650 only); cd_remote_domain_name; cd_remote_password; cd_remote_server_address; cd_remote_share_type; cd_remote_source_path; cd_remote_user_name; fd_remote_domain_name (X470 only); fd_remote_password (X470 only); fd_remote_server_address (X470 only); fd_remote_share_type (X470 only); fd_remote_source_path (X470 only); fd_remote_user_name (X470 only); hd_kerberos_default_realm (B650 only); hd_kerberos_domain_realm1 (B650 only); hd_kerberos_domain_realm2 (B650 only); hd_kerberos_encryption_type (B650 only); hd_kerberos_kdc_server_name (B650 only); hd_kerberos_key_version_number (B650 only); hd_kerberos_principal_name (B650 only); hd_kerberos_principal_password (B650 only); hd_remote_domain_name; hd_remote_password; hd_remote_retry_count (B650 only); hd_remote_retry_interval (B650 only); hd_remote_server_address; hd_remote_share_type; hd_remote_source_path; hd_remote_user_name; local_media_support; mount_cd; mount_fd (X470 only); mount_hd; redirection_status (B650 only); remote_media_support; same_settings
  - response: cd_kerberos_default_realm (B650 only); cd_kerberos_domain_realm1 (B650 only); cd_kerberos_domain_realm2 (B650 only); cd_kerberos_encryption_type (B650 only); cd_kerberos_kdc_server_name (B650 only); cd_kerberos_key_version_number (B650 only); cd_kerberos_principal_name (B650 only); cd_kerberos_principal_password (B650 only); cd_remote_domain_name; cd_remote_password; cd_remote_server_address; cd_remote_share_type
  - sample (redacted): `{ "id": 1, "local_media_support": 0, "remote_media_support": 0, "same_settings": 0, "cd_remote_server_address": "", "cd_remote_source_path": "", "cd_remote_share_type": "", "cd_remote_domain_name": "", "cd_remote_user_name": "", "mount_cd":…`

**`/api/settings/media/instance`** — GET, PUT
  - request: kvm_num_cd; kvm_num_fd (X470 only); kvm_num_hd; num_cd; num_fd (X470 only); num_hd; power_save_mode; sd_media; secure_channel
  - response: kvm_num_cd; kvm_num_fd (X470 only); kvm_num_hd; num_cd; num_fd (X470 only); num_hd; power_save_mode; sd_media; secure_channel
  - sample (redacted): `{ "id": 1, "license": 1, "cd_status": 1, "cd_max_sessions": 132, "cd_active_sessions": 128, "cd_port": -2147478528, "cd_secure_port": -2147478524, "fd_status": 1, "fd_max_sessions": 132, "fd_active_sessions": 128, "fd_port": -2147478526, "f…`

**`/api/settings/media/local/file`** — POST
  - request: multipart:lmedia_image
  - note: multipart/form-data, field lmedia_image (ISO/IMG file). Preceded by POST lmediasizecheck {image_size: bytes}. Upload progress via XHR upload events.

**`/api/settings/media/local/image`** — DELETE
  - note: Query string built by hand: /api/settings/media/local/image?&image_index=<n>&image_name=<name>. No body.

**`/api/settings/media/local/lmediasizecheck`** — POST
  - request: image_size; multipart:lmedia_image

**`/api/settings/media/local/start-media`** — POST
  - request: image_index; image_name; image_type

**`/api/settings/media/local/stop-media`** — POST
  - request: image_index; image_type

**`/api/settings/media/remote/configurations`** — GET
  - request: redirection_status (B650 only)
  - response: error_code (B650 only); image_name; media_index; media_type; redirection_status; session_index (B650 only); share_type (B650 only)
  - sample (redacted): `[ ]`

**`/api/settings/media/remote/images`** — GET
  - request: redirection_status (B650 only)
  - response: error_code (B650 only); image_name; media_index; media_type; redirection_status; session_index (B650 only); share_type (B650 only)
  - sample (redacted): `[ ]`

**`/api/settings/media/remote/rmediarefreshlist`** — GET, POST
  - request: appType; mediaType; redirection_status (B650 only)
  - response: error_code (B650 only); image_name; media_index; media_type; redirection_status; session_index (B650 only); share_type (B650 only)
  - note: POST JSON {appType:1, mediaType:1|2|4} (1 = CD, 2 = FD, 4 = HD), one call per enabled media server (CD_SERVER_APP / FD_SERVER_APP / HD_SERVER_APP features), staggered by 2 s; then the collection is fetched (GET).

**`/api/settings/media/remote/start-media`** — POST
  - request: image_index; image_name; image_redirection (B650 only); image_share_type (B650 only); image_type
  - note: JSON {image_name, image_type (media_type: 1 CD, 2 FD, 4 HD), image_index}; B650 adds image_redirection:1 and image_share_type. stop-media takes the same fields.

**`/api/settings/media/remote/stop-media`** — POST
  - request: image_index; image_name; image_share_type (B650 only); image_type
