# X570D4U BMC: live probe

Date: 2026-11-16. One X570D4U on the lab network. The probe was strictly read-only:

- `/api`: GET only, plus `POST`/`DELETE /api/session` for login and logout.
- Redfish: GET only, plus one SessionService session create and delete.
- One request at a time, on one keep-alive connection, paced 0.2 s.
- The method and deny lists are enforced in code (`docs/x570/scripts/probe.py`).
- The bmcprobe runs used `--max-inflight 1`, a new flag that caps southbound concurrency at 1.

Captures, all sanitized:

- `docs/x570/har-endpoints.json`: `/api` digest, in the same shape as
  `docs/x470/har-endpoints.json` and consumed by `internal/testsim/cmd/extract`.
- `docs/x570/live-redfish-inventory.json`: 472 Redfish requests with status, type, ETag,
  actions, latency and notes, plus the ServiceRoot.
- `docs/api/legacy-api-endpoints.json`, column `X570-2.11`: static handler presence.
- Static firmware analysis: `docs/firmware/x570d4u.md`.

Sanitization:

- Serials, MACs, UUIDs, host names, CSRF tokens, session ids and the password are
  `<REDACTED_*>`.
- RFC 1918 addresses are mapped to 192.0.2.0/24.
- The password was only ever passed through the environment
  (`set -a; . ./x570d4u.env; set +a`, then `--password-env BMC_PASS`).

## Live firmware (read this first)

| Item | Value | Source |
|---|---|---|
| Board | `X570D4U` | `asrr/model-name` `{"mb_name":"X570D4U","sys_name":"N/A"}`; FRU board `manufacturer "ASRockRack"` |
| BMC firmware | **1.20.00** (the dumped image is 2.11.00) | `asrr/fw-info`; `firmware-info` `fw_rev_1 1`, `fw_rev_2 32` (0x20); Redfish `Managers/Self.FirmwareVersion "1.20.0"` |
| BIOS | `L1.57` | `asrr/fw-info`, Redfish `Systems/Self.BiosVersion` |
| ME / PSP | `0.14.0.3E` | `asrr/fw-info` |
| CPU microcode | `0a20120e` | `asrr/fw-info` |
| CPLD | `N/A` | `asrr/fw-info` |
| Host power | **off** for the whole probe | `chassis-status` `{"power_status":0,"led_status":0}` |
| BMC clock | about **+5 h fast** (offset 17995 s). `date-time` `timezone "GMT-05"`, `utc_minutes -300`; `timestamp - localized_timestamp = 18000`. Redfish `DateTime` shows a `-05:00` label on a UTC wall time | `settings/date-time`, response `Date` header |
| Redfish | ServiceRoot `v1_5_2`, RedfishVersion **1.8.0**, Vendor AMI, `Oem.Ami.RtpVersion 1.8.0` | `GET /redfish/v1/` |

## Comparison with X470 2.32 and B650 4.10

| Area | X470D4U 2.32.00 | B650D4U 4.10.00 | **X570D4U 1.20.00** |
|---|---|---|---|
| SoC / AMI stack | AST2500, SPX 2.x/3.x | AST2600, SPX 13 | AST2500, SPX 13 (`5.X` codebase) |
| ServiceRoot / RedfishVersion | v1_3_1 / 1.5.0, no Vendor | v1_7_0 / 1.15.1 | **v1_5_2 / 1.8.0**, Vendor AMI |
| `/redfish/v1` without slash | 302 | 200 | 200 |
| Redfish takes the `/api` admin | **no** (401 AccessDenied) | yes (synced account) | **yes** (synced account id 4; `PAMEnabled false`) |
| TLS | legacy: SSLv3 to TLS1.2, RSA key exchange only, expired cert | modern: TLS 1.2/1.3 ECDHE AEAD | modern: TLS 1.2 ECDHE/**DHE** AEAD, TLS 1.3; self-signed `CN=megarac.com` valid until 2034 |
| `/api` login | `certlogin=0`, about 1.6 to 3.0 s | no certlogin | no certlogin, **about 2.9 s** |
| Login response extras | none | `user_id`, `channel`, `passwordStatus` | same as B650 |
| Cookie | `QSESSIONID; path=/` | `secure; HttpOnly; Samesite=Lax` | `path=/; secure;HttpOnly` (**no SameSite**) |
| CSRF on GET | any non-empty token | any non-empty token | **enforced**: a missing or bogus token gives 401 cc:7 |
| Fan dialect | `asrr/settings/*` + `GetFanDuty` | `settings/fan/*` | `asrr/settings/*`, **no `GetFanDuty`** (404) |
| Keep NIC link up | `asrr/getvetobit` | `settings/ncsilinkup` | `asrr/getvetobit` |
| date-time NTP field | `auto_date` | `ntp_auto_date` (value 2 seen) | `ntp_auto_date` (0; Redfish NTP `ProtocolEnabled false`) |
| Sensors mask / unset thresholds | `settable_readable_threshMask` / 0.0 | `settable_flag` / `"NA"` | `settable_flag` / `"NA"`; 48 sensors |
| SEL trailer `{"event_log_count":N}` | no | yes | **no** |
| Restore policy | IPMI only | `asrr/GetPowerRestore` | `asrr/GetPowerRestore` `{"powerRestore":1}`; Redfish `PowerRestorePolicy` is **null** |
| Inventory | system_inventory (500 when off) + host_inventory | host_inventory | host_inventory; system_inventory gives 500 code 11272 when off |
| POST code | `asrr/getbioscode` | `logs/postcode` | `logs/postcode` (32 KB) and `asrr/getbioscode` |
| `settings/network-bond` | no `bond_ifc` | `bond_ifc` | no `bond_ifc` |
| prochot / clearmcalog | yes | no | no |
| `asrr/selsummary` | 200 (fast) | about 30 s, then 500 code 50012 | **30 s, then 500 code 1007** |
| Redfish `$select` / `$expand` | 501 for unknown query | honoured | **both silently ignored** (full body, no inlining); unknown params ignored |
| Redfish paging | none | 50 per page, nextLink | 50 per page, nextLink, `$skip` honoured |
| Redfish sensors | n/a | 60 `Chassis/Self/Sensors` with readings | **3** (PSU/discrete, null readings); Thermal 11 temps + 9 fans, Power 15 voltages |
| Redfish SEL | n/a | 11 entries | **2** entries (Created 1969) vs 3639 in `/api` |
| Redfish FirmwareInventory | BIOS, BMC with versions | BIOS, BMC | BIOS, BMC, BP, CPLD, FPGA, PSU, RETIMER; **only BMC has a Version** |
| `Systems/Self` Manufacturer / Model | n/a | AsrockRack / null | **"To Be Filled By O.E.M." / " "** |
| NetworkProtocol PATCH (static) | synchronous | 202 + NetworkProtocolTask | synchronous (not tested live) |
| Ports | 22, 80 (301 to https), 443, 623 | 22, 443, 623 (80 closed) | 22, 80, 443, 623, **5901**; 5120, 5900, 7578 and 9080 closed |

## 1. Ports and TLS

- **Open:** 22, 80, 443, 623, 5901.
- **Closed:** 5120, 5900, 7578, 9080. SINGLE_PORT_APP is enabled
  (`configuration/runtime`).
- **TLS 1.2:**
  - suites: ECDHE-RSA and DHE-RSA (4096-bit) with AES-GCM, CHACHA20, CCM and ARIA, all
    grade A;
  - the server chooses the suite.
- **TLS 1.3:** AES256, CHACHA20 and AES128. A Go client negotiated TLS 1.3
  `TLS_AES_256_GCM_SHA384`.
- **Certificate:** self-signed AMI `CN=megarac.com` (Service Processors), RSA-2048, valid
  Aug 27 2019 to Aug 23 2034. SAN `DNS:megarac.com, DNS:169.254.0.17`.
- **Proxy profile:** `transport.ProfileModern`.

## 2. Redfish

**Unauthenticated:**

- `/redfish/v1/`, `/redfish/v1`, `/redfish`, `odata` and `$metadata` all return 200.
- Everything below the root returns 401, including a nonexistent path, with
  `WWW-Authenticate: Basic` and `X-Auth-Token realm="AMI_Redfish_Server"`.
- `GlobalLuaTables` returns 200 unauthenticated, which is why it is denylisted.
- `$select` on the root is ignored.
- ProtocolFeaturesSupported advertises ExcerptQuery, ExpandQuery (MaxLevels 5), FilterQuery,
  OnlyMemberQuery and SelectQuery. DeepOperations is absent.

**Authenticated:**

- **Login:** Basic auth with the `/api` admin returns 200. A session POST returns 201 with
  X-Auth-Token and Location, and DELETE returns 204.
- **Accounts and policy:**
  - accounts: 1 `Administrator`, 4 `admin` (Administrator role), 5 `fwupd` (Administrator
    role, enabled);
  - `AccountService/Configurations`: `PAMEnabled false`,
    `PAMOrder [IPMI, LDAP, ACTIVE DIRECTORY, RADIUS]`;
  - `AccountService/Oem/Ami/Configurations` returns 404;
  - AccountService v1_6_0, lockout 5 / 30 s; SessionService v1_1_6, `SessionTimeout 30`.
- **Query handling:**
  - `$select`, `$expand` and unknown parameters are ignored;
  - collections page at 50 with a working `nextLink` (AuditLog 150, EventLog 86), and `$skip`
    is honoured.
- **Errors:**
  - 404 uses `Base.1.x.ResourceMissingAtURI`;
  - 405 for `Systems/Self/Pldm` and `Smbios`;
  - 404 for `BiosStaticFiles`, `Oem/Ami/InventoryData` and `Systems/Self/LogServices/SEL`.
- **ETags:** 113 distinct values (per resource group), so there is no single service ETag.
- **Crawl:** 460 resources fetched (cap 450 plus probes), 46 left in the queue. Median
  latency 285 ms, p95 914 ms.
- **Data quality, which drives the southbound choice (§7):**
  - `Chassis/Self/Sensors` has 3 members (112, 145, 160), all with null readings;
  - `Managers/Self/LogServices` has AuditLog, EventLog and SEL; `SEL/Entries` has 2;
    `Systems/Self/LogServices` has only BIOS;
  - only the BMC firmware component has a Version;
  - `Systems/Self` has a placeholder Manufacturer, Model `" "` and a null
    `PowerRestorePolicy`;
  - NetworkProtocol v1_4_2: NTP `ProtocolEnabled false`, servers `pool.ntp.org` and
    `time.nist.gov`.

## 3. `/api` session layer

- **Login:** `POST /api/session` takes 2.8 to 2.9 s.
- **Login response keys:** `ok`, `privilege 4`, `user_id 2`, `extendedpriv 259`,
  `racsession_id`, `remote_addr`, `server_name`, `server_addr`, `HTTPSEnabled`, `channel`,
  `passwordStatus` and `CSRFToken`. There are no TFA fields.
- **Cookie:** `QSESSIONID=<30 chars>; path=/; secure;HttpOnly`.
- **CSRF is enforced on GETs** (new; the X470 and B650 accept any non-empty token on GETs).
  These all return 401 `{cc:7}`:
  - no `X-CSRFTOKEN`;
  - a bogus token;
  - no cookie;
  - HTTP Basic instead of the cookie.

  The proxy always sends the real token, so nothing changed there. The simulator now
  models it (`Profile.CSRFOnGET`).
- **Unknown path:** 404 `{"error":"Invalid API Call","code":1010}`.
- **Logout:**
  - First run: `DELETE /api/session` returned 200 `{"ok":0}`, and replaying the cookie then
    gave 401.
  - Second run: it returned a **lighttpd HTML 500**, and the next GET with the same cookie
    returned 200, so the session was not closed (see §8).

## 4. `/api` endpoint results

All GET. Timings are from the second run.

**200:**

- Identity:
  - `asrr/model-name`, `asrr/fw-info`, `asrr/product-name`;
  - `firmware-info`;
  - `configuration/project`, `configuration/runtime`.
- Status:
  - `chassis-status` (no `power_cycle_interval`);
  - `status/uptime`.
- Sensors:
  - `sensors` and `detail_sensors_readings`: 48 sensors, `settable_flag`, `"NA"`
    thresholds, units `V`, `°C`, `RPM` and `unknown`;
  - `sensors/basic`, `sdr`;
  - `settings/sensor-threshold`, which returns `{}`.
- Logs and FRU:
  - `logs/audit`;
  - `logs/dashboardevent` (470 KB, 4.5 s);
  - `logs/postcode` (32 KB);
  - `fru`.
- Inventory: `host_inventory/*`, populated even with the host off.
- Network:
  - `settings/network`;
  - `settings/network-bond`: `{id, bond_enable, bond_mode "active-backup", auto_configuration_enable}`;
  - `settings/network-link`;
  - `settings/ncsi/mode` (`{id, mode}`) and `settings/ncsi-interfaces`;
  - `settings/static-ipv6`, `settings/dns-info`.
- Users and services:
  - `settings/users` (13.5 KB, 2.7 s), `settings/users_read_only`;
  - `settings/services`.
- Time: `settings/date-time`, with no `ptp_*` fields on 1.20.
- Security:
  - `settings/ssl/certificate-info`;
  - `settings/pam-order`.
- ASRock OEM:
  - `asrr/getvetobit` `{"vetobit":0}`;
  - `asrr/settings/getfancontrolmode` (16 modes, all 0);
  - `asrr/settings/getfanopenloopcontroltable` and `getfancloseloopcontroltable`;
  - `asrr/settings/getfandutymanualmode` (`duty1..16`);
  - `asrr/settings/getsupportfan`, `getsupportfanname`, `gettscftsupportfan`, `getfantempsensorandcorrespondfantable`.
- Power and diagnostics:
  - `asrr/GetPowerRestore` `{"powerRestore":1}`;
  - `asrr/PowerSource`;
  - `asrr/getbioscode`;
  - `asrr/getmcalog` `{"MCADump":0}`;
  - `asrr/getWebEnableTimeout`.

**404:**

- `extfeatures`, `chassis`;
- `asrr/smbiosdata`, `asrr/settings/getfan100druingbmcupdate` (both present in 2.11);
- `asrr/GetFanDuty`, `asrr/getprochot`;
- `settings/fan/mode`, `settings/ncsilinkup`.

**500:**

- `system_inventory/{device_count,cpu_info,dimm_info,pci_info}`: code 11272, host off.
- `host_inventory/host_interface_power_info`: 200 on the first run, then a **lighttpd HTML
  500** on the second, with the session still valid.

**SEL:**

- `logs/eventlog`:
  - 4.08 MB, **34 s**, 3639 records, newest first;
  - query strings are ignored (`?start=0&count=10` returned the same 4 MB in 21 s);
  - no `event_log_count` trailer;
  - early records have negative timestamps, for example -17965.
- `logs/event?LASTEVENTID=3634` returns ids 3639 down to 3635 in about 1 s.
  `LASTEVENTID=3639` returns `[]`.
- `asrr/selsummary` hangs for **30 s**, then returns 500
  `{"error":"Error in get SEL summary","code":1007}`.

## 5. bmcprobe

### 5.1 Before the X570 profile (the board fell to the "unknown" profile)

| Command | Result |
|---|---|
| `detect` (no creds) | asrock, score 35, family modern |
| `detect` (creds) | asrock, score 100, X570D4U 1.20.00, profile **unknown**, degraded "unknown board/firmware: capability probing"; Redfish authenticated (basic) |
| `capabilities` | identity, power, LED, boot, sensors, eventLog, eventLogClear, inventory, users, firmwareInventory on **redfish**; restore, audit, fru, network, bond, services, time, bmcReset on ami |
| `get events` | **0 entries** (Redfish), while `/api` holds 3639 |
| `get sensors` | 39 from Redfish Thermal/Power, readings null; `/api` has 48 |
| `get firmware` | BIOS and the others with **empty versions** |
| `get system`, `get inventory baseboard` | manufacturer **"To Be Filled By O.E.M."**, model `" "` |
| `get power`, `fru`, `network`, `users`, `services`, `time`, `inventory cpus/memory/pcie/storage` | OK; time offset 17994 s is real BMC skew |

### 5.2 After the X570 profile (same commands, `--max-inflight 1`)

| Command | Result |
|---|---|
| `detect` (creds) | asrock, score 100, X570D4U 1.20.00, profile **x570d4u**, not degraded; ami and redfish authenticated |
| `capabilities` | identity, power, LED, boot, inventory and users on redfish; **sensors, eventLog, eventLogClear and firmwareInventory on ami**; restore, fru, bond, network, services, time, audit and bmcReset on ami |
| `get system` | manufacturer `ASRockRack`, model empty, BIOS L1.57, power off |
| `get sensors` | 48 (`/api`) |
| `get firmware` | bmc 1.20.00, bios L1.57, me 0.14.0.3E, microcode 0a20120e |
| `get restore-policy` | `lastState` (asrr `powerRestore 1`) |
| `get events --since 3630` | 9 entries, lastId 3639 (`logs/event?LASTEVENTID=3630`) |
| `get inventory baseboard` | manufacturer `ASRockRack`, product X570D4U |

No stderr output, and every session was logged out.

## 6. Quirks

Dangerous quirks first:

1. **Redfish EventLog entry stall.** `GET Managers/Self/LogServices/EventLog/Entries/48` and
   `/49` each timed out at 60 s, and `/50` took 42 s. That stalled Redfish for about 3
   minutes; it recovered afterwards. The proxy reads the SEL over `/api` on this board and
   never touches these entries.
2. **Full SEL read is 4 MB / 34 s.** The first incremental read (`LASTEVENTID=0`) returns
   everything. The profile gives `/api/logs/event` a 90 s attempt timeout with no retry, so
   a slow read never doubles the load. Reads after that are incremental and small.
3. **`asrr/selsummary` holds the single FastCGI worker for 30 s.** The profile cuts it off
   at 8 s.
4. **HTML 500 that is not a backend restart.** It was seen on
   `host_inventory/host_interface_power_info` and on `DELETE /api/session`, and both times
   the session survived. The amimegarac client treats an HTML 500 as "backend restarted"
   and re-logs in, which is safe here, just one extra 2.9 s login. The driver never calls
   `host_interface_power_info`.
5. **Logout can fail with an HTML 500.** `amimegarac.Client.Logout` now retries a 5xx once
   after 1 s. Before this fix a failed logout left the session alive until the 1800 s idle
   timeout.
6. **CSRF is enforced on GETs.** A client that sends a stale token after a re-login gets
   401 cc:7, which already triggers the re-login path.
7. **Clock about 5 h fast.** The proxy's normal clock-offset correction handles this.
8. **Redfish ignores `$select`/`$expand`.** The generic client coped live: users and
   inventory read correctly through Redfish. The simulator models it with
   `RedfishIgnoresQuery`.

## 7. Southbound decision

**Redfish yes** (`RedfishYes`), with **`/api` first** for the four capabilities whose
Redfish data is poor on this board:

- **Sensors:** Redfish has 3 sensors with null readings, or 39 via Thermal/Power; `/api` has 48.
- **Event log and clear:** Redfish has 2 entries, plus the EventLog entry stall; `/api` has 3639.
- **Firmware inventory:** Redfish has versionless components; `asrr/fw-info` has BMC, BIOS,
  ME and microcode.

Identity, power, LED, boot override, inventory and users stay on Redfish. All of them worked
live. The ASRock gaps stay on `/api`, as on the B650: FRU, bond, restore policy, fan,
vetobit and POST code. The new `boards.Profile.AMIFirst` field moves `/api` to the front of
the preference list only, so Redfish remains the fallback if the `/api` login fails.

## 8. Hygiene

- No PATCH, PUT or POST actions; no LED, power, time or NTP writes. The only writes were
  `/api` and Redfish session create and delete.
- Never called:
  - `logs/help-data`, `asrr/mca-file`, `settings/download_image`,
    `maintenance/download_config`;
  - `hpm/upgradestatus`, `hpm/verifyimagestatus`;
  - anything firmware, flash, restore, reset or factory.
- The client enforced the deny list itself (`probe.py` refuses before sending), and every
  request went out one at a time.
- **One `/api` session leaked.** In the second API run, `DELETE /api/session` returned the
  HTML 500 and the session stayed valid. The probe did not persist the cookie, so it could
  not retry. The session expires after the 1800 s idle timeout. This motivated the
  logout-retry fix.
- The password never appeared on a command line or in any file. Raw bodies (serials, MACs)
  stayed in a scratch directory and were deleted after the sanitized captures were built.
