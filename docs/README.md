# ASRock Rack BMC proxy: preliminary assessment

Status: **preliminary research, not yet a plan** (2026-11-16).
Goal: a native Go proxy that presents a single Redfish API to clients and drives ASRock Rack BMCs underneath through Redfish, the legacy AMI `/api` REST interface, and IPMI where needed.

All findings come from four sources:

| Source | What it gave us | Limits |
|---|---|---|
| Static firmware analysis of the three `.ima` images in `bmc-firmware/` | configs, init scripts, web UI JS, Redfish schemas, `strings` of binaries | **No disassembly or decompilation.** Redfish Lua is stripped LuaJIT bytecode, so it was read with `strings` only. |
| ASRock Rack "BMC AST2600 Generic Redfish v1.11 API User Guide" (PDF, Rev 1.00, Dec 2023) | vendor reference for each resource | names no board or firmware version; some examples are DMTF samples |
| Three HAR captures of the web UI | real `/api` request/response shapes | cookies stripped from the browser export; no Redfish traffic |
| Live probe of one X470D4U, read-only apart from an identify-LED toggle | TLS, auth, latency, Redfish presence | Redfish rejected the credentials, so no authenticated Redfish crawl |

---

## 1. Read this first: deployed firmware ≠ analysed firmware

| Board | Firmware in the field (HAR / live probe) | Firmware statically analysed |
|---|---|---|
| X470D4U (AST2500) | **2.32.00** (May 2021): HAR + live probe | 3.02.00 |
| B650D4U (AST2600) | **1.10.00**: HAR | 7.02.00 |
| B550D4U-2T (AST2600?) | **1.10.00**: HAR (file is misnamed `b650d4u-2t.har`) | *none* |
| B650D4U_2L2T (AST2600) | not seen in the field | 9.05.00 |

Consequences:
- The proxy must detect **board + firmware version at runtime** (`GET /api/asrr/model-name`, `GET /api/asrr/fw-info`, Redfish `Managers/Self.FirmwareVersion`) and choose per-version behaviour. Field shapes already differ between generations (see §4).
- When a HAR or the live probe disagrees with a static doc, treat it as a possible **version difference** before assuming either is wrong.
- 7.02 and 9.05 are **not** two versions of one board. They are sibling builds for two boards, made the same day from the same codebase, and their core stack is identical.

---

## 2. Document index

| Path | Contents |
|---|---|
| [firmware/x470d4u.md](firmware/x470d4u.md) | X470D4U 3.02 stack: flash layout, init order, lighttpd, services/ports, Redfish enablement evidence, `/api` provider, auth, feature flags |
| [firmware/b650d4u.md](firmware/b650d4u.md) | B650D4U 7.02 / B650D4U_2L2T 9.05 stack, the same topics, plus a 7.02↔9.05 diff and security findings |
| [redfish/b650-implementation.md](redfish/b650-implementation.md) | AMI Redfish server internals (Turbo.lua on :9080, SSE :9090, redis, sync-agent), auth, OEM, UpdateService flow, quirks, X470 tree comparison |
| [redfish/b650-uri-inventory.json](redfish/b650-uri-inventory.json) | 307 Redfish URIs with methods, actions and handler (machine-readable) |
| [redfish/pdf-part1-p001-130.md](redfish/pdf-part1-p001-130.md) | Vendor guide pp.1–130: TOC, conventions, auth, AccountService, Chassis, logs, network adapters, … |
| [redfish/pdf-part2-p131-260.md](redfish/pdf-part2-p131-260.md) | pp.131–260: PCIe, Power/Thermal/Sensors, EventService, Manager, NetworkProtocol, VirtualMedia, registries, Sessions |
| [redfish/pdf-part3-p261-381.md](redfish/pdf-part3-p261-381.md) | pp.261–381: ComputerSystem, Boot, Memory/CPU/Storage, SecureBoot, TaskService, Telemetry, UpdateService, FixedBootOrder |
| [api/legacy-api-catalog.md](api/legacy-api-catalog.md) | Every legacy `/api` endpoint referenced by the web UIs (416), with request/response fields, firmware-update and power sequences, and handler presence per board |
| [api/legacy-api-endpoints.json](api/legacy-api-endpoints.json) | The same, machine-readable (`handler_present` per board, `seen_in_live_capture`) |
| [x470/har-analysis.md](x470/har-analysis.md) · [har-endpoints.json](x470/har-endpoints.json) | X470D4U 2.32 HAR: auth flow, 32 endpoints, polling, latency |
| [b650/har-analysis.md](b650/har-analysis.md) · [har-endpoints.json](b650/har-endpoints.json) | B650D4U 1.10 + B550D4U-2T 1.10 HARs: writes (network-bond, fan tables), the 401 explained, a cross-board table |
| [x470/live-probe.md](x470/live-probe.md) · [live-redfish-inventory.json](x470/live-redfish-inventory.json) | Live X470D4U 2.32 probe: TLS, auth, latency, ports, Redfish root, "needs approval" list |

Extracted filesystems live in `../firmware-dumped/<firmware>/{raw,rootfs,www,conf,extlog}` (gitignored because it contains private keys). See §7.

---

## 3. Headline conclusions for the Go proxy

1. **Every board in scope runs AMI MegaRAC SPX**, so one family of drivers covers them all:
   - **Redfish** is the primary interface: AMI LuaJIT server, `Oem.Ami` only, no ASRock OEM namespace. All top-level IDs are `Self`.
   - **Legacy `/api`** (`spx_restservice` over FastCGI) is needed for ASRock-specific features (`/api/asrr/*`: BIOS/CPLD/PSU/FPGA/retimer flashing, fan tables, POST codes, MCA log, power-restore policy) and for anything older Redfish lacks.
   - **IPMI** is a fallback only. Not yet probed: `ipmitool` isn't installed.
2. **The X470D4U *does* run Redfish, on the LAN, by default**, contrary to the original assumption. Static evidence (3.02): `CONFIG_SPX_FEATURE_REDFISH_ENABLED`, `S29redfish-server`, `lighttpd.conf` always includes `redfish_lighttpd.conf`. Live evidence (2.32): `/redfish/v1/` returns RedfishVersion 1.5.0. It is an older generation: no SSE, TaskMonitor, FirmwareInventory or CertificateService. **The probe account got 401 from Redfish while `/api` accepted it**, probably because the 5-character factory password falls below the 8-character Redfish password policy. How much of the X470 Redfish tree is usable is still **unknown**.
3. **Serialise per BMC; cache reads.**
   - `/api` is a single FastCGI process (`max-procs 1`).
   - Redfish queues parallel requests.
   - B650 lighttpd `mod_evasive` caps connections at **10 per IP on `/redfish`** (SSE included) and **30 overall**, and **answers excess with HTTP 403** (not 429). The proxy must not treat that 403 as an auth failure.
   - Slow calls: the event log dump (~3–5 s, ~500 KB), the users list (~1.7 s), network settings (~1–1.6 s), login (~1.5 s).
   - The TLS handshake (230–500 ms) costs more than most requests, so keep connections alive.
4. **Session management:**
   - Hold **one long-lived session per BMC per interface** and log out cleanly. Session slots are finite (128 web sessions, 36 IPMI on X470), and other clients (KVM, humans) share the BMC.
   - `/api` login is a form `POST /api/session`, which returns the `QSESSIONID` cookie (`path=/`, no HttpOnly or Secure) and a `CSRFToken`, sent back as `X-CSRFTOKEN`.
   - Polling `chassis-status` keeps the session alive.
   - An expired or invalid session gets `401 {"cc":7,"error":"Invalid Authentication"}`.
   - Redfish offers Basic and `X-Auth-Token`. Lockout is 5 failures for 30 s, and the IP can be locked as well.
5. **TLS client must be configured per generation.**
   - X470 2.32 offers **RSA key exchange only** (no ECDHE) and TLS 1.0–1.2. Go ≥1.22 excludes those suites by default, so set `CipherSuites` explicitly.
   - B650 is TLS 1.2, ECDHE+AEAD only.
   - Certificates are shared AMI defaults (one expired 2026-05-30), so **pin per BMC on first contact; never trust the chain**.
6. **Lenient decoding is mandatory.** Seen in the PDF, HARs and code:
   - `[]` where objects belong
   - `"nil"` and `"table: 0x…"` strings
   - `"NA"` for unset sensor thresholds (B650/B550)
   - dates that aren't zero-padded
   - explicit nulls
   - wrong `PhysicalContext`
   - 201 responses that return the collection URI
7. **Normalise write semantics.** The same operation may return 204, 200 with a body, or **202 with a Task**:
   - 202: Reset, IndicatorLED, InsertMedia, Manager.Reset, NetworkProtocol PATCH.
   - Firmware update returns 202 with the task URI in `Messages[].MessageArgs`.
   - ETags are timestamps: one service-wide ETag on X470, `"Dummyetag"` for the BIOS registry. About 30 B650 PATCH handlers check `If-Match`, which may mean 428 without it; unverified.
8. **Firmware update paths are non-standard:**
   - Redfish: `POST /redfish/v1/UpdateService/Actions/SimpleUpdate` and multipart `/redfish/v1/UpdateService/upload`.
   - Legacy: BMC image to `/firmware` (X470) or `/firmware/firmware` (B650). Components go through `/api/asrr/maintenance/<BIOS|CPLD|PSU|BP|FPGA|RETIMER>/{firmware,configuration,upgrade,status,flash-progress}`.
9. **Endpoints the proxy must block or gate:**
   - `/redfish/v1/GlobalLuaTables`: debug, unauthenticated.
   - `GET /api/logs/help-data`: crashed `spx_restservice` on the live X470.
   - `PUT /api/maintenance/restore_defaults`: one-shot factory reset.
   - Network/bond changes (restart BMC networking).
   - User/account changes.
10. **Web-UI endpoint lists overstate the API.** 150 of 369 endpoints referenced by the B650 9.05 UI have **no handler** in the firmware (18 optional plugins aren't shipped). Use the `handler_present` column in `legacy-api-endpoints.json`.
11. **Events:** B650 has Redfish SSE (`/redfish/v1/EventService/SSE` → :9090, which sends a bogus `Content-Encoding: " "` header to strip). X470 has none, so poll there: `GET /api/logs/event?LASTEVENTID=n` returns only new SEL records (3.7 KB instead of a 589 KB dump), and `asrr/selsummary` is a cheap change detector.

---

## 4. Capability matrix (current evidence)

Legend: ✅ confirmed · 🟡 present statically, unverified live · ❌ absent · ❓ unknown

| Capability | X470D4U 2.32 (live) | X470D4U 3.02 (static) | B650D4U 1.10 (HAR) | B650 7.02 / 2L2T 9.05 (static) | B550D4U-2T 1.10 (HAR) |
|---|---|---|---|---|---|
| Redfish service on LAN | ✅ root (v1.5.0); auth ❓ | 🟡 enabled by default | ❓ not captured | 🟡 (307 URIs) | ❓ |
| Redfish SSE events | ❓ | ❌ | ❓ | 🟡 | ❓ |
| Redfish UpdateService tasks | ❓ | ❌ (HTTP/FTP SimpleUpdate only) | ❓ | 🟡 SimpleUpdate + multipart | ❓ |
| Legacy `/api` login + CSRF | ✅ | 🟡 | ✅ | 🟡 | ✅ |
| Power control via `/api` (`{"power_command":0/1/2/3/5}`) | ✅ (HAR) | 🟡 | 🟡 | 🟡 | 🟡 |
| Identify LED via `/api` | ✅ synchronous, no ETag | 🟡 | ❓ | 🟡 | ❓ |
| Sensors via `/api` | ✅ 41 sensors | 🟡 | ✅ (`"NA"` thresholds) | 🟡 | ✅ |
| Host inventory via `/api` | ✅ but empty | 🟡 | ⚠️ CPU/memory empty | 🟡 | ✅ populated |
| Event log paging | ❌ full dump; ✅ incremental `logs/event?LASTEVENTID` | 🟡 | ✅ paged + count record | 🟡 | ✅ paged |
| Fan open-loop tables (`asrr/settings/*openloop*`) | ❓ | 🟡 | ✅ | ❓ not in analysed image | ❓ |
| Component flash (`asrr/maintenance/<C>/*`) | ❓ | 🟡 BIOS | ❓ | 🟡 BIOS/CPLD/…; 9.05 adds FPGA/RETIMER | `BIOS/UpdateOption` seen |
| KVM / virtual media | single-port mode: 7578/5120 closed; via websockets on 443 | 🟡 H5Viewer websockets | ❓ | 🟡 | ❓ |
| TLS | TLS1.0–1.2, RSA kx only, AMI cert expired | OpenSSL 1.0.1t | ❓ | TLS1.2 ECDHE+AEAD, OpenSSL 1.1.1n | ❓ |
| IPMI (623) | UDP ❓, **TCP 623 open** (unexpected) | 🟡 | ❓ | 🟡 | ❓ |

---

## 5. Security findings (consolidated)

These are outside the proxy's scope, but they shape its design and should be raised with whoever owns the fleet.

| # | Finding | Where |
|---|---|---|
| S1 | **Shared TLS private key** (`/conf/server.pem`, `certs/privkey.pem`) in every image; the live X470 serves the expired shared `CN=AMI` cert | firmware/*, live-probe §8 |
| S2 | Same AES key/IV for stored Redfish account passwords on every unit (B650) | firmware/b650d4u.md |
| S3 | Redfish host interface (USB LAN 169.254.x) defaults to **no auth with the Administrator role**, so any host OS process gets BMC admin | PDF part 2, firmware docs |
| S4 | DHCP option 248 can replace the client-cert CA (`/conf/ca.pem`) (B650) | firmware/b650d4u.md |
| S5 | SSH `PermitRootLogin yes`; `sysadmin` shell account enabled (B650) | firmware/b650d4u.md, b650/har-analysis.md |
| S6 | `/redfish/v1/GlobalLuaTables` reachable without auth | redfish/b650-implementation.md, live-probe |
| S7 | `GET /api/logs/help-data` crashes the REST backend (denial of service) | live-probe |
| S8 | Login isn't CSRF-checked; the `QSESSIONID` cookie lacks HttpOnly/Secure; Redfish sends `CORS *` with credentials | HAR analyses, live-probe |
| S9 | RSA private key left in a commented-out line of the web UI JS (B650/B550) | b650/har-analysis.md |
| S10 | **The probed X470 still uses factory credentials**; **all three HARs contain admin passwords in cleartext** | live-probe, HAR analyses |

Implications for the proxy:
- keep BMCs on an isolated management network that only the proxy can reach
- pin BMC certificates per device
- the proxy stores BMC credentials and is the only client
- block S6 and S7 paths
- report host-interface auth mode and factory-cert usage as health warnings

---

## 6. Open questions and next inputs

Ordered by how much each would unblock:

1. **Working Redfish credentials for the X470** (≥8-character password, or a dedicated Redfish-capable account), then rerun the probe's authenticated crawl, SessionTimeout units (PDF says 30 *seconds*, `ncml.conf` says 1800), `If-Match` behaviour, and `Managers/Self.FirmwareVersion`.
2. **A live probe of a B650D4U** (and the B550D4U-2T): Redfish crawl, the mod_evasive 403 behaviour, SSE.
3. **Firmware strategy:** either supply images matching the field versions (X470D4U 2.32, B650D4U 1.10, B550D4U-2T 1.10) for static analysis, or upgrade lab units to the analysed versions and re-probe.
4. HAR recaptures **with cookies**, plus flows not yet seen: power on the B650, BMC + BIOS firmware update, KVM launch, virtual media, logout, Redfish usage.
5. IPMI: install `ipmitool` (or use a Go IPMI library) to check cipher suites, SOL and the unexpected TCP 623 listener.
6. Optional: **decompiling the LuaJIT bytecode** of the Redfish service would recover the exact routes, validation and numeric limits (`MAX_SESSIONS`, body size, collection limits). This needs explicit approval: so far, nothing has been disassembled or decompiled.

The full list of state-changing tests waiting for approval is in [x470/live-probe.md § Needs approval](x470/live-probe.md).

---

## 7. Extracted firmware layout (`firmware-dumped/`)

```
firmware-dumped/<image>/
  raw/      carved partitions: boot.bin, conf.jffs2, rootfs.{squashfs|cramfs},
            kernel.{fit|uimage}, www.{squashfs|cramfs}, (X470) extlog.jffs2
  rootfs/   root filesystem   (B650: squashfs-xz; X470: cramfs, extracted with fsck.cramfs to keep symlinks)
  www/      web UI partition  (files are gzip-compressed without a .gz extension)
  conf/     factory /conf JFFS2 (defaults, certs incl. private keys, ncml.conf, IPMI.conf, fan maps)
  extlog/   (X470 only) /extlog JFFS2: Redfish sub-extension Lua (ami_bios, ami_inventory, ami_util)
```

| Image | Flash | Partition offsets |
|---|---|---|
| B650D4U_2L2T_9.05.00 | 64 MiB | boot 0x0 · conf 0x110000 · root 0x510000 · kernel FIT 0x2680000 · www 0x2AD0000 |
| B650D4U_7.02.00 | 64 MiB | boot 0x0 · conf 0x110000 · root 0x510000 · kernel FIT 0x26A0000 · www 0x2AF0000 |
| X470D4U_3.02.00 | 32 MiB | boot 0x0 · conf 0x60000 · root 0x360000 · kernel uImage 0x16C0000 · extlog 0x1990000 · www 0x1A10000 |

The JFFS2 extraction was cross-checked: jefferson 0.4.8 and an in-house extractor produced byte-identical output.
