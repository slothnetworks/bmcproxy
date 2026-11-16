# X470D4U Redfish authentication: why `admin` is rejected

Scope: owner's own non-production BMC. This documents the auth *model* and account provisioning. No bypass was attempted and no hash was recovered. The seeded hash is described by format only.

Analyzed: firmware **3.02.00** (static). The live board runs **2.32.00**, so every claim below is "per 3.02.00" (see the version caveat).

Paths are relative to `firmware-dumped/X470D4U_3.02.00/rootfs/` (`RF` = `usr/local/redfish/`) unless noted. B650 paths use `firmware-dumped/B650D4U_2L2T_9.05.00/rootfs/`.

## 1. Verdict

Redfish on the X470 only authenticates against its own redis account store (`Redfish:AccountService:Accounts:<user>:*`), and the only account in that store is the factory seed `Administrator` (id 1). The `admin` user lives in the IPMI/`/api` user database. The X470 build has no mechanism that copies IPMI users into the Redfish store. The B650 build has one (the "Unified User Account" feature plus an `ipmiredfishusersync` daemon). The only other route to IPMI users is PAM (`pam_ipmi`). The Redfish PAM path is gated on the redis flag `Redfish:AccountService:PAMEnabled`, which is not seeded anywhere in the X470 rootfs, so it is off by default. With PAM off, `Accounts:admin:MemberId` does not exist, the credential check fails, and the handler turns the failure into the generic `unauthorized()` response. That response is HTTP 401 with `Security.1.0.AccessDenied` and `WWW-Authenticate` headers, which matches what the live board returns for both Basic and session login. The B650 works because its sync daemon mirrors IPMI user `admin` into the Redfish store as a local account (live Redfish account id 4).

Confidence: high that the model (local redis store, PAM gated on `PAMEnabled`) is as described. High that there is no sync component in the 3.02.00 image. Medium that 2.32.00 behaves identically (not examined). Medium-low on the one claim that PAMEnabled is unset on the live board (needs a live check).

## 2. Evidence

### 2.1 Readable seed and config (confidence: high)

`RF/db_init/zzzaccountservice.rcmd` (mirrored in `RF/db_init/init-script/zzzaccountservice.sh`):

| Item | Value |
|---|---|
| Accounts seeded | exactly one: id 1, `UserName "Administrator"`, `RoleId Administrator`, `Enabled true`, `Locked false`, `StrongPassword 1` |
| Credential fields | `Password` = 128 hex chars (SHA-512, salted), `Salt` = base64 string. Values redacted. |
| `LastAccountIndex` | `"3"` (ids 2 and 3 are reserved for host-interface accounts, see below) |
| Lookup key | `Redfish:AccountService:Accounts:Administrator:MemberId = 1` (username to id map) |
| Policy | `MinPasswordLength 8`, `MaxPasswordLength 20`, `AccountLockoutThreshold 5`, `AccountLockoutDuration 30`, `AccountLockoutCounterResetAfter 30`, `AuthFailureLoggingThreshold 3` |
| Roles | Administrator (Login, ConfigureManager, ConfigureUsers, ConfigureSelf, ConfigureComponents); Operator (Login, ConfigureSelf, ConfigureComponents); ReadOnly (Login, ConfigureSelf) |
| Not present | `PAMEnabled`, `PAMOrder`, `LocalAccountAuth`, `PasswordChangeRequired`. None of these keys exists in any `.rcmd`. |

Seeding is first-boot only: `RF/db_init/redis-init.sh` runs the `.rcmd` files only when `GET Redfish:AccountService:ServiceEnabled` is empty, then `bgsave`s. After that the redis contents persist and are not re-applied. Changing them needs a Redfish PATCH or a direct redis write.

`RF/config.lua` (LuaJIT bytecode; constants decoded with a custom parser): `USE_SPX_PAM` is set by `if stat("/info") == nil then USE_SPX_PAM=false, SERVICE_HOST="0.0.0.0" else USE_SPX_PAM=true, SERVICE_HOST="127.0.0.1"`. `/info` exists on the BMC (`rootfs/info/`), so `USE_SPX_PAM=true` on the device. `SALT = "spx"`, `SUPPORTED_HASHING_ALGORITHM = {MD5, SHA512}`. There is no `PBKDF2` entry on the X470. B650 `config.lua` has the identical `/info` test.

Host-interface accounts: `libipmiamioemrestiface.so.6.10.0` (strings only) writes `Redfish:AccountService:Accounts:%d:*` for ids 2 and 3 (`HostAutoFW`, `HostAutoOS`, with `AMIGeneratePassword`). These are the in-band host-interface users, not IPMI users.

Correction to `docs/firmware/x470d4u.md` §4.6, last bullet ("IPMI users such as admin are mirrored into Redfish... the same credentials work for /api and /redfish"). The string evidence does not support this. The only writers of `Accounts:%d:*` on X470 are the host-interface code paths. The statement is contradicted by the live 401.

### 2.2 No IPMI-to-Redfish user sync on X470 (confidence: high for 3.02.00)

Build config, `info/X470D4U.PRJ` vs `info/B650D4U_2L2T.PRJ`:

| Feature | X470 3.02.00 | B650 9.05.00 |
|---|---|---|
| `CONFIG_SPX_FEATURE_UNIFIED_USER_ACCOUNT` / `_ANY` | absent | `=YES` (lines 1145, 1286) |
| `CONFIG_SPX_redfish_pam` | absent | `-13.11.0.0.0=YES` (line 766) |
| `CONFIG_SPX_libnss_redfish` | absent | `-13.3.0.0.0=YES` (line 1102) |
| PBKDF2 hashing for Redfish | absent | present (line 1158) |
| `pam_ipmi`, `pam_helperd`, Pamreorder | present | present |

Sync-agent contents, `usr/local/sync-agent/extensions/ipmi/`:

- X470 has none of: `subagents/ipmiredfishusersync.lua`, `init_agent_scripts/init-user.lua`, `libipmi/user.lua`, `sync-helpers/user.lua`, `user_privilege_map.lua`. `find ... -iname '*user*'` under the X470 sync-agent returns nothing. Its `init-pam.lua` only reads PAM order.
- B650 has all of them. Constants from `sync-helpers/user.lua` and `subagents/ipmiredfishusersync.lua`:
  - `":MappingID:IPMI:"` and `":MappingID:Redfish:"` map IPMI user ids to Redfish ids.
  - `Description "User Account sync from IPMI"`.
  - `/var/pipe/sync_usr_act_req` and `/var/pipe/sync_usr_act_res`.
  - `CONFIG_SPX_FEATURE_UNIFIED_USER_ACCOUNT`.
  - It writes `:Password`, `:Salt`, `:EncryptedPassword`, `:StrongPassword`, `:PasswordChangeRequired`. The Redfish password hash is derived from the IPMI password at change time.
- Grep of the whole X470 rootfs for `Redfish:AccountService:Accounts` outside `RF/` hits only `libipmiamioemrestiface`, `libipmiredfish.so`, and the sync-agent `ami.lua`/`redfish_intermediate_map.lua`/`session_common.lua`. None of these creates user accounts. `ami.lua` only references `Accounts:*:UserName/RoleId` for the PAM-order and host-interface helpers (strings only, medium confidence).

### 2.3 PAM FFI (confidence: high for the cdef; the binding use is from decoded bytecode)

`RF/pam_ffi.lua` starts with a readable C cdef block: `pam_handle_t`, `struct pam_message`, `pam_response`, `pam_conv`, `pam_data`, `pam_environ`, `_pam_fail_delay`, `pam_xauth_data`, `loaded_module`, `handler`, `handlers`, plus `time_t`. It is a copy of Linux-PAM internals so that LuaJIT can bind to the vendor wrapper. The bytecode after it binds two functions (strings in `redfish-handler.lua`): `DoPAMAuthentication` (credential check, with the PAM service name `"HTTP"` and the client IP from `X-Forwarded-For`) and `WebAuditLogout`. Also bound: `GetUsrLANChPriv`, which returns the IPMI LAN-channel privilege level; the handler maps 1/2 to ReadOnly, 3 to Operator and 4 to Administrator via the `Redfish:AccountService:Roles:*` table.

PAM service `HTTP` resolves via `etc/pam.d/HTTP -> /conf/pam_wounix`, with defaults in `etc/defconfig/pam_wounix`:

```
auth sufficient /lib/security/pam_ipmi.so
```

(plus `pam_nologin` and the same module for account/password/session). So if PAM were enabled, `admin`/IPMI credentials would work through `pam_ipmi`.

### 2.4 Decoded auth decision path (confidence: medium-high; stripped bytecode, no names)

The 3.02.00 `.lua` files are stripped LuaJIT 2.0 bytecode (flags `0x02`). No third-party decompiler was used. I wrote a small parser and disassembler to dump per-function constants and opcodes (scratch only, not kept). Function numbers below are the order in the dump of `RF/redfish-handler.lua`. I reconstructed the logic by hand from opcodes plus constants, not a full decompile, so treat the exact control flow as reconstructed.

1. **Request entry (fn 60, ~790 opcodes).** For each request it checks, in order:
   - a client-certificate header: `X-Username` and a `CN=` match, then `check_client_cert`, then `check_login`, which authenticates as `CERT_AUTH` and logs `AccessAllowed`;
   - an `Authorization` header: it uppercases and compares the first token to `BASIC_AUTH`, base64-decodes, splits on `:`, sets the auth mode to `BASIC_AUTH`, and calls `check_login(user, pass)`;
   - an `X-Auth-Token` header: `SESSION_AUTH` against `Redfish:SessionService:Sessions:*`.

   If `check_login` fails (`ISNEP ... true` falls through), it calls `unauthorized()`. If the account's `Enabled` is `false` or the user is a host-interface name, it calls `error_access_denied`.
2. **`POST /SessionService/Sessions`** is allowed through unauthenticated, and `session-service.lua` (`login_user`, `check_login`, `LoginSuccess`) calls the same `check_login`. On failure it also calls `unauthorized` (`#/UserName`, `#/Password` strings in fn 6 of `session-service.lua`).
3. **`unauthorized` (fn 28, 18 opcodes).** Adds `WWW-Authenticate: X-Auth-Token realm="AMI_Redfish_Server"` and `Basic realm="AMI_Redfish_Server"`, then `error_access_denied`. This is the `401 Security.1.0.AccessDenied` seen live. A wrong-password or unknown-user login does not use `LoginFailure` here; `error_login_failure` is only raised for lockout. (High confidence on the AccessDenied mapping.)
4. **`check_login` (fn 85, 227 opcodes).** In order:
   1. If this is not a host-interface connection, call `check_pam_login(user, pass)`. If it returns true, the login succeeds.
   2. Otherwise read `Redfish:AccountService:Accounts:<user>:MemberId`. If nil, it jumps to the failure exit. This is where `admin` dies on the X470.
   3. Otherwise apply the lockout counters (`AccountLockoutThreshold`, `Duration`, `CounterResetAfter`), check `:Locked` and `:Enabled == "true"`, then compute `sumhexa(sha512(<password>..<Salt>..SALT))` (SHA-512, with `StrongPassword` selecting the scheme) and compare against `:Password`.
5. **`check_pam_login` (fn 63, 221 opcodes).** Returns false immediately unless both of these hold:
   - `bool(db.get("Redfish:AccountService:PAMEnabled"))` is true, and
   - `config.USE_SPX_PAM` is true.

   Only then does it build the PAM handle, call `DoPAMAuthentication(...)` (0 means success), fetch `GetUsrLANChPriv`, map it to a role, and set `PAM_AUTH` mode.

`PAMEnabled` is read and written only by `RF/oem/ami/accountservice-config-handler.lua` (`GET/PATCH /redfish/v1/AccountService/Oem/Ami/Configurations`, `PAMEnabled`, `PAMOrder`; PATCH requires `ConfigureComponents` and restarts the web server per the strings `WebServerRestarting`). The schema text says "Indicates whether or not PAM authentication should be used when authenticating Redfish requests". `docs/redfish/pdf-part1-p001-130.md:1540` documents the default as `False`. No `.rcmd`, init script, or native lib in the X470 rootfs seeds it (`grep -rla PAMEnabled` hits only Lua, schema files and the schema docs).

### 2.5 Native binaries

Not disassembled and not needed. The credential logic is in Lua (`check_login`, `check_pam_login`) and delegates to the native PAM wrapper only through `DoPAMAuthentication`. I ran `strings` only on `libipmiamioemrestiface`, `libipmiredfish`, `libipmiamioempamreorder`, `libipmi`, `spx_restservice` to rule out a seeder or user mirror. The `PamOrder` hits there are the AMI OEM PAM-order IPMI commands, not the Redfish `PAMEnabled` flag.

## 3. Account store contents summary

Redis (`Redfish:AccountService:*`) at first boot on X470 3.02.00:

- Accounts: `1 -> Administrator` (Administrator role, enabled, unlocked, salted SHA-512 hash). Ids 2 and 3 are reserved for the host-interface users created at runtime by `libipmiamioemrestiface`.
- Roles: Administrator, Operator, ReadOnly (privileges as in §2.1).
- Policy values as in §2.1.
- No `admin`, no IPMI users, no `PAMEnabled`.

## 4. B650 contrast

- B650 builds `UNIFIED_USER_ACCOUNT`, `redfish_pam` and `libnss_redfish`, and ships `ipmiredfishusersync.lua` plus `sync-helpers/user.lua`. IPMI user `admin` is mirrored into redis as a normal local account. The live B650 shows Redfish account id 4 `admin`, Administrator role, enabled (`docs/b650/live-probe.md` §3.5). Basic auth therefore resolves through the local-account path (step 4.3 above), the same path the factory `Administrator` uses. PAM is not required.
- The B650 PAM path also differs: it talks to a PAM service over named pipes (`/var/pipe/RedfishPAMReqQ`, `/var/pipe/RedfishPAMResQ`) instead of calling the PAM FFI directly, and it adds `LocalAccountAuth` (`LocalFirst` default) to order local vs PAM checks. The X470 has no `LocalAccountAuth`.
- B650 also adds `PasswordChangeRequired` and PBKDF2. Neither exists on X470.

So the difference is a feature of the build (account sync), not a configuration flag that could be flipped on the X470.

## 5. Proxy rule and provisioning options

**Rule: for the X470, the proxy must not use Redfish with the `admin` credentials.** Use `/api` plus IPMI, as already decided. Do not treat a Redfish 401 on X470 as a bad password; it is expected.

Is Redfish usable at all?
- With the seeded `Administrator`: it exists in the store, but its default password is **not** in readable config (only the salted SHA-512 hash and salt are seeded; the hash was not attacked). Its default is not documented in the extracted tree. If the live board was never changed, it would work with the vendor default, which has to be learned from the owner or AMI/ASRock documentation, not from this firmware. Also check that it has not been changed.
- Via PAM: it works with `admin` only after `PAMEnabled` is set to true. Setting it needs either an authenticated Redfish PATCH (circular: needs a working Redfish login first, e.g. `Administrator`) or a direct write to the BMC's redis (shell access). `/api` has `settings/pam-order` (the PAM-order command), but no `/api` route was found that sets the Redfish `PAMEnabled` flag.
- Provisioning a Redfish account via `/api`: unlikely to work, because the X470 build has no IPMI-to-Redfish sync, so `/api` user creation only touches the IPMI/`pam_ipmi` side. This is testable later: create a test user in `/api`, then try Basic against `/redfish/v1/` and check whether `Accounts` lists it.
- Practical conclusion: effectively unusable for the proxy without extra provisioning that the owner does not need, since `/api`+IPMI covers it.

## 6. Version caveat

- Analyzed: 3.02.00. Live: 2.32.00 (older). Everything above is per 3.02.00.
- The older release could differ in: whether `PAMEnabled` is seeded or defaults on, whether it ships a user-sync piece (unlikely given the build flags, but unverified), and the error mapping (`AccessDenied` vs `LoginFailure`). The 401 behavior reported live matches the 3.02.00 `unauthorized` path, which is mild corroboration only.
- The earlier doc claim about mirrored IPMI users (`docs/firmware/x470d4u.md` §4.6) should be corrected to match §2.2 here.

## 7. Needs live confirmation

1. `GET /redfish/v1/AccountService/Accounts` with a working login (if any): confirm only `Administrator` (plus host-interface users) exist. Needs the `Administrator` password.
2. `GET /redfish/v1/AccountService/Oem/Ami/Configurations`: confirm `PAMEnabled` is `false`/absent. (An unauthenticated read gets the same 401.)
3. Create a test user via `/api` and test Basic against Redfish to confirm there is no sync on 2.32.00.
4. Compare the unauthenticated `401` body for a bad password vs an unknown user (both expected `AccessDenied` per 3.02.00).
5. Confirm 2.32.00 has no `ipmiredfishusersync`/`init-user` files, if that image is ever extracted.
