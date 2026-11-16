# B650D4U Redfish implementation (AMI MegaRAC SPX, BMC 9.05 / 7.02)

This is a preliminary assessment for a native Go proxy in front of ASRock Rack BMCs. It covers the
Redfish service on the B650D4U, firmware 9.05, compared against 7.02, plus a short look at the X470 3.02
tree.

**How this was produced.** Everything comes from static analysis of the extracted root filesystems:

| Tag | Root filesystem |
|---|---|
| `B650 9.05` | `fw/b650_905/root` |
| `B650 7.02` | `fw/b650_702/root` |
| `X470 3.02` | `fw/x470/root` |

Paths in this document are relative to those roots. For example, `/usr/local/redfish/server.lua` means
`fw/b650_905/root/usr/local/redfish/server.lua`.

**The Lua is bytecode, not source.** All 494 `.lua` files under `/usr/local/redfish` on B650 9.05 (495 on
7.02) and all 229 under `/usr/local/sync-agent` are LuaJIT bytecode, not text. They start with
`1b 4c 4a 01`, and the dump flags are `0x02` (stripped, so there is no debug info). That has three
consequences:

- **Method used.** No source was available. Nothing was disassembled. The only extraction was a
  length-aware `strings` pass: a scan for printable runs that carry LuaJIT's `len+5` ULEB128 prefix, which
  is how string constants are stored. Text files were read directly: `db_init/*.rcmd`, shell init scripts,
  lighttpd configs, JSON schemas, the privilege-registry JSON, `static/metadata` and `openapi.yaml.gz`.
- **Numeric constants are missing.** They don't appear in a strings dump. Values such as `MAX_SESSIONS`,
  `MAX_REQUESTBODY_LENGTH` and `SESSION_TIME_*_LIMIT` in `config.lua` are therefore listed by name only.
  Where a `.rcmd` file seeds a default into redis, that value is given.
- **Some pairings are inferred.** The key→module pairing in `redfish-resource-handler.lua` can't be read
  directly from a stripped dump. Each handler file was matched to its URI by entity and module name.
  Lower-confidence matches are marked *(assumed)* or *(inferred)*.

Scripts used, all in the scratchpad `rf-agent/` folder:

| Script | Purpose |
|---|---|
| `lstrlib.py`, `lstr.py` | length-aware strings scan |
| `join.py` | route-table join |
| `gen.py` | inventory generator |

The machine-readable inventory is in [`b650-uri-inventory.json`](./b650-uri-inventory.json).

---

## 1. Server architecture

### 1.1 Process layout and boot order

| Component | Init script | Runlevel | What it does |
|---|---|---|---|
| redis | `/etc/init.d/redis-server` (`/usr/local/redis/redis-server`) | S25 | Runs `/usr/bin/redis-server /etc/redis/redis.conf`. Unix socket only (`port 0`, `unixsocket /run/redis/redis.sock`, perm 700). `maxmemory 20971520` (20 MiB), `noeviction`, `databases 16`, `notify-keyspace-events "KEA"`. RDB file is `/tmp/redis-dump.rdb`; there is no AOF. |
| redisrsync | `/usr/local/redis/redisrsync` | S24 | Watches `/tmp/redis-dump.rdb.gz` and copies it to `/conf/redis-dump.rdb.gz` (SPI flash, `CONFIG_SPX_FEATURE_SPI_REDIS_DUMP`) or `/mnt/sdmmc0p5/` (eMMC). This is how Redfish state persists across reboots. `savedb_to_conf.sh` forces `bgsave`. |
| event-service | `/usr/local/redfish/event-service/event-service` | S26 | `luajit ./event-service/event-service.lua` → `/var/run/event-service.pid`. Sets `Redfish:EventService:ServiceEnabled true`. |
| task-service | `/usr/local/redfish/task-service/task-service` | S26 | `luajit ./task-service/task-service.lua` → `/var/run/task-service.pid`. |
| sync-agent | `/usr/local/sync-agent/sync-agent` | S28 | IPMI/IPC → redis synchroniser (see 1.5). |
| redfish-server | `/usr/local/redfish/redfish-server` | S29 | `cd /usr/local/redfish; luajit server.lua`. Then starts `/usr/local/bin/redfish_pamauthd --daemonize --reg-with-procmgr`, runs `sync-agent/subagents/init-agent.lua`, and registers with procmonitor (`procmonitor-redfish.lua`). |
| redfish-settings | `/usr/local/redfish/redfish-settings` | S48 | `luajit run_settings.lua Boot` (and `Shutdown` on stop). Runs OnBoot/OnShutdown hooks. |
| redfish-nw-up | `/usr/local/redfish/redfish-nw-up` | S89 | Sends `kill -12` (SIGUSR2) to the server to rebind once the network is up, then re-runs init-agent. |
| lighttpd | `/usr/local/redfish/lighttpd.sh` | S90 | Front end. Exits if `/var/tmp/licstat/lighttpd_nolicense` exists. |

`redfish-monitor-script` starts the stack in this order: redis → event → task → sync-agent → redfish.

### 1.2 HTTP front end (lighttpd)

`/etc/defconfig/lighttpd.conf` includes `redfish_lighttpd.conf` (line 115) and `redfish_lighttpd_bios.conf`
(line 118). Global `evasive.max-conns-per-ip = 30`.

`/usr/local/redfish/redfish_lighttpd.conf`:

```
server.stream-request-body = 2
server.modules += ( "mod_proxy", "mod_evasive" )
server.stream-response-body = 1
$HTTP["url"] =~ "^/redfish" {
    proxy.server  = ( "" => (( "host" => "127.0.0.1", "port" => 9080 )) )
    server.errorfile-prefix = "/usr/local/redfish/redfish_error_"
    evasive.max-conns-per-ip = 10
}
$HTTP["url"] =~ "^/redfish/v1/EventService/SSE" {
    proxy.server  = ( "" => (( "host" => "127.0.0.1", "port" => 9090  )) )
    setenv.add-response-header += ( "Content-Encoding" => " ")
}
$SERVER["socket"] == ":443" {
    ssl.ca-file = "/conf/ca.pem"
    ssl.verifyclient.activate = "enable"
    ssl.verifyclient.enforce = "disable"
    ssl.verifyclient.username = "SSL_CLIENT_S_DN_CN"
    ssl.verifyclient.exportcert= "enable"
}
```

The config has these consequences for a proxy:

- **Concurrency cap.** Clients get at most 10 concurrent connections per IP to `/redfish` (`mod_evasive`).
  This is the only "rate limiting".
- **Streaming bodies.** Request bodies are streamed to the backend (`stream-request-body = 2`), which
  matters for multipart firmware upload.
- **Upload staging.** lighttpd may stage uploads in `/tmp/lighttpd-upload-*`.
  `update-service_utils.lua` cleans those up.
- **503 page.** When the backend is down, lighttpd serves `/usr/local/redfish/redfish_error_503.html`.
- **mTLS is optional.** Client certificates are requested on :443 but not enforced. lighttpd exports
  `SSL_CLIENT_S_DN_CN` and the PEM to the backend (see 3.5).

`redfish_lighttpd_bios.conf` serves the BIOS setup web UI under `/bios/`. The static files are copied from
`/etc/defconfig/redfish/bios/static/` (`Index.html`, `Index.js`, `Index.css`, two PNGs, a favicon) with
CSP, `X-Frame-Options` and gzip headers. This UI is not Redfish.

### 1.3 Redfish server (`server.lua`) and framework

- **Framework.** Turbo.lua (`turbo.web.Application`, `turbo.httpserver.HTTPServer`) on LuaJIT, using
  `turboredis` (`/usr/local/redfish/libs/turboredis/*`) for asynchronous redis over the Unix socket. JSON
  comes from cjson and `turbo.3rdparty.JSON`. FFI calls go to libipmi, libpam, OpenSSL, `libredfishinterface`
  and similar. Module search paths are set in `module_manager.lua`:
  `./?.lua;./libs/?;./oem/?;./redfish/?;./event-service/?`.
- **Listen address.** `config.SERVICE_HOST` is `127.0.0.1` (a `0.0.0.0` option exists), port
  `config.SERVICE_PORT` (9080 per lighttpd). There is also an SSL listener path (`ssl_options`,
  `cert_file=/conf/server.pem`, `key_file=/conf/server.pem`, `:443`). It is used when the server binds
  directly, for example to the host-interface address `DEFAULT_HI_USB0_ADDRESS`.
- **SSE listener.** `config.SSE_SERVICE_PORT` (9090) is served from the same process via
  `event-service/ServerSentEvent.lua` (`HTTP_SSE`, `SSE_CLIENT_HANDLER`).
- **Request limits.** `max_header_size` and `streaming_multipart_bytes` are passed to `listen()`.
- **Signals.** SIGUSR1/SIGTERM trigger a graceful exit, removing the PID files and SSE subscription keys.
  SIGUSR2 rebinds.
- **Server name and version.** `SERVER_NAME = "AMI MegaRAC Redfish Service"`.
  `SERVICE_PREFIX = "/redfish/v" .. MAJOR`, i.e. `/redfish/v1`.
- **Hard-coded UUID.** `config.UUID = "92384634-2938-2342-8820-489239905423"`. This is a default; the
  service root reads `Redfish:UUID` from redis.
- **Startup work.** At boot `server.lua`:
  - reads BIOS attribute registries from `/conf/redfish/bios` (`BiosAttributeRegistry*.json`, gzip),
    writes "stripped" and "readable" copies to `/tmp/bios_strip_files/` and `/tmp/bios_readable_files/`,
    and populates `Redfish:Systems:Self:BIOS:IntegerAttributes`;
  - sets `Redfish:ServiceRunningStatus=true` and `Redfish:LastModified`;
  - registers PLDM RDE resources if `CONFIG_SPX_FEATURE_SELECT_RDE_DEVICE_SUPPORT` is set.

### 1.4 Routing table: how URLs map to handler files

Routing is table-driven (`redfish-resource.lua`, called from `server.lua` as
`load_all_resource_table` → `create_all_route_table_to_global`). Each *package* directory contains five
or six table files:

| File | Content |
|---|---|
| `redfish-resource-URI_pattern.lua` | list of `{pattern = SERVICE_PREFIX .. "<lua pattern>$", key = "<resource key>"}` |
| `redfish-resource-handler.lua` | `key → require("<handler module>")` (a turbo `RequestHandler` subclass) |
| `redfish-resource-allow_method.lua` | `key → {"GET","PATCH",...}`, sometimes with `overrides = {allowed_methods=..., targets=...}`. It also holds a `key → entity label` table used for the `Allow` header. |
| `redfish-resource-entities.lua` | `key → {chain of Redfish entity types…, name = "<Entity>", superordinate=…}`. The chain is used for privilege-registry lookups. |
| `redfish-resource-route_options.lua` | per-key handler options; only `resource_001 = {redirection_location = SERVICE_PREFIX}` |
| `redfish-resource-operation_privilege_mappings.lua` | builds `operation_privilege_mappings` from `redfish.registry` and `config.PRIVILEGE_CHECKING_WHITELIST` |

Packages load in this order, and later packages can override keys from earlier ones:

1. `priority/redfish-resource`: only `.*%.Deep$` → `redfish/deep_operation.lua` when
   `DeepOperations_Enabled`.
2. `redfish-resource`: the core set, 249 patterns.
3. `extensions/redfish-resource/*`: `hostinterface` (8) and `updateservice` (7).
4. `oem/*/redfish-resource`, found via `utils.get_oem_dirs`:
   - `oem/ami` (11 literal patterns plus 10 built with `string.format("^%s/Managers/%s/...", SERVICE_PREFIX, get_env_managers_name())`)
   - `oem/ami_bios` (8)
   - `oem/ami_hi` (7)
   - `oem/plugin` (2)
   - `oem/plugin_hi` (2)

The legacy `route.lua` files (`oem/*/route.lua`, `extensions/routes/*.lua`, `priority/priority_route.lua`,
`default_route.lua`) now just return `RESOURCE.<pkg>.route_table`. The catch-all `^/redfish.*$` is mapped
to `redfish/default-handler.lua`.

Two optional behaviours:

- **Telemetry routes are conditional.** The `TelemetryService` entries in the core handler table are only
  added when redis `Redfish:Telemetrics:Support` is truthy (seeded to `1` by
  `db_init/telemetryservice.rcmd`). They also write `Redfish:Services TelemetryService`.
- **Host-interface overrides.** The `hostinterface` extension re-registers the core keys `resource_110`,
  `resource_109`, `resource_053`, `resource_049` and `resource_050` (Registries, Bios, Bios/SD,
  Bios/Actions) with host-interface-aware handlers from `extensions/host-interface/hi-handlers/*.lua`.

Every handler subclasses `RedfishHandler` from `redfish-handler.lua`, which provides:

- authentication, privilege checks, ETag and precondition handling;
- JSON validation (`MalformedJSON`, `NonASCIICharacterDetected`, `PropertyUnknown` and similar);
- the error builders `error_*`;
- the `Allow` and `Link` headers;
- redis transactions with `MULTI`/`EXEC`, plus keyspace-notification waiting with `PATCH_TIMEOUT`;
- audit-log entries.

The `odata-handler.lua` subclass handles OData query parameters and `$metadata`.

A second layer, `redfish/resource_metadata/*.lua` and `oem/*/resource_metadata/*.lua`, declares
per-property redis access. Each property gets a `redis_lua_access_prehook`/`posthook` and a key template.
`property-access/`, `property-type/` and `property-size/` declare which properties are readable or
writable, their types, and their string-length limits. These tables drive generic GET and PATCH handling.

### 1.5 Data sourcing

All resource data lives in **redis** (db 0, plus `config.resource_db_num` for the route tables
`Redfish:RESOURCE`). Handlers don't call IPMI on GET; they read redis. Several mechanisms populate it:

1. **`db_init/*.rcmd` files** (plain redis commands; see section 5 for the list). They're applied once at
   first boot or after a factory reset by the sync-agent's `redis_init_helper.lua`. Placeholders such as
   `$AMI_SYSTEMS_INSTANCE`, `$AMI_MANAGERS_INSTANCE`, `$AMI_CHASSIS_INSTANCE` and
   `$AMI_HOSTINTERFACE_INSTANCE` are substituted, normally with `Self`. They seed static properties,
   ActionInfo parameter lists, allowable values and defaults. Examples:
   - `SessionTimeout 30`
   - `AccountLockoutThreshold 5`
   - a default `Administrator` account with `PasswordChangeRequired true` (a salted SHA-512 and PBKDF2
     hash is shipped in `zzzaccountservice.rcmd`)
   - `HSET Redfish:Services …` links for the service root
2. **sync-agent** (`/usr/local/sync-agent`, also LuaJIT bytecode):
   - **init-agent** (`subagents/init-agent.lua` plus `extensions/*/init_agent_scripts/init-*.lua`) does
     one-shot population at boot over IPMI via FFI `libipmi`. It covers FRU, SEL, sensors/SDR, network,
     users, PAM, serial, services, boot, certificates, SMBIOS (`oem-init-SMBIOS.lua`), telemetry, update
     service and host interface.
   - **ipmi2redfish / sel2redfish** subagents (`extensions/ipmi/subagents/*`) handle periodic and event
     sync: sensor polling (`Redfish:SensorPollingInterval 3000` ms), SEL → LogEntries, and IPMI-side
     user changes (`ipmiredfishusersync.lua`).
   - **redfish → IPMI writes** (`notify_fn_map.lua`, `notify_mask_map.lua`, `redfish_group_map.lua`,
     `redfish_intermediate_map.lua`, `subagents/notify*.lua`, `redfish-2-redfishgroup.lua`,
     `redfishgroup-2-fns.lua`). Handlers stage writes as `PATCH:<key>` and `POST:<key>` redis keys (for
     example `PATCH:Redfish:AccountService:IPMI:AccountInfo`,
     `POST:Redfish:UpdateService:OemAction:StartFwUpdate:BMC`). The sync-agent sees the keyspace
     notification, calls the matching IPMI or library function (`extensions/ipmi/sync-helpers/*.lua`,
     `extensions/ami/sync-helpers_ami/*.lua`), and writes the result back. The handler waits on keyspace
     events (`Waiting on …`, `PATCH_TIMEOUT`, `PostTimeout`), then answers.
   - **libraries.** `libipmi_ffi`, `libcomponent_manager_ffi` (HPM/BIOS/CPLD), `libbackupconf_ffi`,
     `libnetwork_ffi` and `libpldmupdate_ffi`.
3. **Task service IPC.** Long-running actions (reset, virtual media, firmware update, maintenance-window
   operations) create `Redfish:TaskService:Tasks:<id>` with `TaskType` set to `LUA_SCRIPT`,
   `IPC_PROCESS`, `SYSTEM_CALL` or `CLIB_CALL`, and push to `Redfish:TaskService:TaskList`.
   `task-service.lua` pops the list and runs the script, for example `./update-service/update-service.lua`,
   `./post_maintenance_window_task.lua` or `task-service/lua_script/patch-networkprotcol.lua`.
4. **Host interface (in-band from BIOS).** BIOS posts its attribute registry, BIOS tables, SMBIOS, PLDM
   blobs and inventory data to `/redfish/v1/Oem/Ami/*`, `/Systems/Self/Smbios` and `/Systems/Self/Pldm`
   over the USB-NIC host interface. This is how `Bios`, `Memory`, `Processors`, `PCIeDevices` and similar
   resources get content.
5. **PAM daemon.** `/usr/local/bin/redfish_pamauthd` is reached through `/var/pipe/RedfishPAMReqQ` and
   `/var/pipe/RedfishPAMResQ` (`redfish-handler.lua`, struct `user_auth_info{username[17], password[21],
   remote_addr[46], server_addr[46]}`). It handles IPMI, LDAP, AD and RADIUS authentication.

### 1.6 Redis key layout (observed)

Keys mirror the URI path with `:` separators, rooted at `Redfish:`. Collection members are tracked with
`SortedIDs` zsets and `SRK` keys. Properties are scalar `SET` keys or hashes (for example
`Status`: `State`/`Health`). Highlights:

| Prefix | Content |
|---|---|
| `Redfish:Services`, `Redfish:Services:Links` | hash: service-root links (Systems, Chassis, Managers, Tasks, SessionService, AccountService, CompositionService, EventService, Registries, JsonSchemas, CertificateService, UpdateService, TelemetryService) |
| `Redfish:Systems:Self:*` | ComputerSystem. Includes `:Actions:ComputerSystem.Reset:*`, `:Bios:*`, `:Bios:SD:*` (pending settings), `:SecureBoot:*`, `:LogServices:{BIOS,SMI}:*`, `:Boot:Certificates:*`, `:Oem:Ami:*`, `:MaintenanceWindowStartTime` |
| `Redfish:Systems:DefaultResetAllowedValues` | set: On ForceOff GracefulShutdown ForceRestart PowerCycle |
| `Redfish:Chassis:Self:*`, `Redfish:Chassis:DefaultResetAllowedValues` | Chassis; `:LogServices:Logs:*` |
| `Redfish:Managers:Self:*` | Manager. Includes `:LogServices:{SEL,EventLog,AuditLog}:*`, `:EthernetInterfaces:*`, `:NetworkProtocol:*`, `:VirtualMedia:*`, `:VMedia:*`, `:HostInterfaces:*`, `:Oem:*` |
| `Redfish:AccountService:*` | Accounts `:Accounts:<n>:{UserName,Password(SHA512),PBKDF2Password,Salt,PBKDF2Salt,EncryptedPassword(AES),RoleId,Locked,Enabled,PasswordChangeRequired,FailedLoginCount,AccountTypes}`, `:Roles:<RoleId>:*`, `:LDAP:*`, `:ActiveDirectory:*`, `:RADIUS:*`, lockout settings, `:PAMOrder`, `:PAMEnabled`, `:Accounts:AESKey`/`:AESIV` |
| `Redfish:SessionService:*` | `:SessionTimeout`. Each session is `Redfish:SessionService:Sessions:<id>:{Token,UserName,…}` with a redis TTL (`expire`), so expiry is redis-driven |
| `Redfish:EventService:*` | `:Subscriptions:<id>:*`, `:SSE:NumClients`, `:SSE:KeepAlive`, `:SMTP:*`, `:Oem:Ami:SecondarySMTP:*`, `:EventTypesForSubscription`, `:RegistryPrefixes`, `:ResourceTypes`, `:EventFormatTypes`; `Redfish:EventServices:Events:PresentID` |
| `Redfish:TaskService:*` | `:Tasks:<id>:{TaskState,TaskStatus,Messages,PercentComplete,StartTime,EndTime,TaskType,TaskIPCData,TaskWebRequestData,Payload,…}`, `:TaskList`, `:TaskMonitors:*`, `:CompletedTaskOverWritePolicy` (Oldest) |
| `Redfish:UpdateService:*` | `:FirmwareInventory:<id>:{Name,Version,Updateable,Status}`, `:Oem:{UpdateStatus,UpdateTarget,CancelFwUpdate,FlashPercentage}`, `:Oem:BMC:DualImageConfigurations`, `:Oem:Ami:PreserveConfiguration`, `:ServiceEnabled`, `:MaxImageSizeBytes` |
| `Redfish:TelemetryService:*` | MetricDefinitions, MetricReportDefinitions, MetricReports, Triggers, LogService |
| `Redfish:Registries:*`, `Redfish:BiosAtrribute:Language`, `Redfish:oem:ami:bios:{RedfishVersion,RtpVersion}`, `Redfish:Ami:RtpVersion` (13.05) | registries and BIOS RTP metadata |
| `Dynamic:RedfishExtensions:*` | BIOS dynamic-extension (DRE) registration and MD5s (`db_init/bios_dre.rcmd`) |
| `RDE:*` | PLDM RDE tasks and events (`RDE:Task`, `RDE:Event`, `RDE:Collection`, `RDE:Instance`) |
| `PATCH:<key>`, `POST:<key>`, `DELETE:<key>` | write intents consumed by the sync-agent |
| `Redfish:DebugPrint`, `Redfish:DefaultSleep` (0.6), `Redfish:VmediaSleep` (2), `Redfish:ActiveSleep` (0.6), `Redfish:SensorPollingInterval` (3000) | tunables (`db_init/managers.rcmd`, `chassis.rcmd`) |

### 1.7 Caching and refresh

- **Redis is the cache.** GETs serve whatever the sync-agent last wrote. Sensors refresh every 3000 ms
  (`Redfish:SensorPollingInterval`). SEL and LogEntries update as events arrive. FRU, SMBIOS and inventory
  load at boot or on host POST (via the host interface).
- **In-process cache.** `resource_metadata` entries carry `property_cache_enabled`. Cached
  resource-metadata UUIDs can be listed and cleared through `/redfish/v1/GlobalLuaTables`
  (`global-lua-tables.lua`: `clear_resource_cache`, `locate_resource_metadata_uuid_by_cache_existing`).
- **Write path.** PATCH stages commands (`:PatchCmds`) and waits for the sync-agent to apply them. A
  timeout returns `PostTimeout` or `TimeoutError`. Writes bump `Redfish:LastModified` and the per-resource
  `:LastModified`, which ETags and `If-Modified` checks use. `trigger_bgsave` then forces a redis `bgsave`
  so the change persists to flash.

### 1.8 Event service and SSE

**`event-service/event-service.lua`** (separate daemon). It psubscribes to redis keyspace events (`KEA`)
on:

- `Redfish:*:*:LogServices:*:Entries:*:EventService`
- `Redfish:Managers:*:LogServices:*:Entries:*:EventService`
- `Redfish:TelemetryService:MetricReportDefinitions:*`
- subscription keys

When an event fires, it builds `Event` or `MetricReport` payloads (`/redfish/v1/$metadata#Event.Event`) and
fans out to subscribers through `event-service/post_task.lua`. Delivery options:

- **HTTP(S) push.** POST with custom `HttpHeaders`, using `socket.http` and `ssl`.
  `DeliveryRetryAttempts` defaults to 3 and `DeliveryRetryIntervalSeconds` to 60. `DeliveryRetryPolicy`
  is `TerminateAfterRetries` or `SuspendRetries`; `EventDestination.ResumeSubscription` resumes a
  suspended subscription.
- **SNMP traps.** v1/v2c/v3 via `snmptrap`, enterprise OID `1.3.6.1.4.1.412.266.*`, plus the
  `Oem.Ami.SNMP` subscription block.
- **SMTP.** Primary and secondary SMTP via FFI `smtp_mail` (`AMI_SMTP_Redfish_Service`), with StartTLS or
  TLS_SSL and None/Plain authentication.

Subscription filters cover `EventTypes`, `RegistryPrefixes`, `ResourceTypes`, `MessageIds` and
`OriginResources`. Seeded allowable values (`db_init/eventservice.rcmd`):

| Setting | Values |
|---|---|
| EventTypes | StatusChange, ResourceUpdated, ResourceAdded, ResourceRemoved, Alert |
| EventFormatTypes | Event, MetricReport |
| RegistryPrefixes | Base, SyncAgent, Security, EventLog, IPMI, HttpStatus, Task |
| ResourceTypes | Chassis, Managers, Systems, AccountService, EventService, TelemetryService, TaskService |

`SubordinateResourcesSupported` is false. The subscription cap is `MAX_EVENTS_DESTINATION` (value not
recoverable; the error is `EventSubscriptionLimitExceeded` / "Maximum Event Destination Limit reached").

**SSE** (`event-service/ServerSentEvent.lua`, served on 127.0.0.1:9090 inside the redfish-server process,
proxied by lighttpd for `^/redfish/v1/EventService/SSE`):

- **Auth.** The client authenticates with Basic or `X-Auth-Token` on the GET (it reuses the
  `check_local_acc_auth` and `check_external_acc_auth` logic). It needs Login privilege.
- **Subscription.** A transient subscription `Subscription_SSE_<n>` with `SubscriptionType=SSE` is
  created and deleted when the socket closes.
- **Limits.** Clients are capped at `MAX_SSE_CLIENT_CONNECTION` (`CreateLimitReachedForResource`). The
  current count is in `Redfish:EventService:SSE:NumClients`.
- **Filtering.** `$filter` supports only `EventFormatType eq X`, `MessageId eq X`,
  `OriginResource eq X`, `RegistryPrefix eq X` and `ResourceType eq X`, joined with `and`. `$expand`,
  `$select`, `$top`, `$skip`, `$only` and `$excerpt` are rejected with `QueryNotSupportedOnResource`.
- **Wire format.** Frames are `id:%s\n` and `data:%s\n\n` with `Content-Type: text/event-stream`. A
  keep-alive comment (" Dummy string to keep the SSE clients Alive ") is sent every
  `SSE_KEEP_ALIVE_RECCURENCE_INTERVAL`, driven by `Redfish:EventService:SSE:KeepAlive`.
- **Headers.** lighttpd forces `Content-Encoding: " "` so that responses aren't compressed.

**Test events:**

- `EventService.SubmitTestEvent`: parameters EventId, EventTimestamp, MessageArgs, MessageId (required),
  OriginOfCondition and Severity. It fails with `Ami` registry messages if no active `Event`-format
  subscription matches.
- `TelemetryService.SubmitTestMetricReport`
- `submit_test_event_metricreport_task.lua` (task-based resubmission).

---

## 2. URI inventory (B650 9.05)

Notes on reading the table:

- **Sources.**
  - Methods come from the per-key lists in `redfish-resource-allow_method.lua` of each package.
  - Handler paths are relative to `/usr/local/redfish/`.
  - Action parameters come from `db_init/action-info.rcmd`, `db_init/actioninfo_amioem.rcmd` and the
    handler strings.
- **Placeholders.**
  - In practice there is one ComputerSystem, Chassis and Manager, each with Id `Self`.
  - `{Action}` is the action segment, for example `ComputerSystem.Reset`.
  - `{ServiceName}` is `LDAP` or `ActiveDirectory`.
- **Collection POST.** `…/Members` POST keys are aliases for creating a member of the parent collection.
- **Methods marked `?`.** These are not listed in the OEM allow table; the handler decides.
- **`HEAD` and `OPTIONS`.** They aren't in the per-key lists. `config.SAFE_METHOD` is `{GET, HEAD}`, and
  `redfish-handler.lua` handles `HEAD` and `OPTIONS` generically (the `Allow` header comes from the entity
  label table in `allow_method_header_table.lua` plus the key's methods).
- **Telemetry rows.** `TelemetryService` rows exist only if `Redfish:Telemetrics:Support` is truthy (it is
  by default).
- **Host-interface overrides.** The Registries and Bios rows are re-registered by the host-interface
  extension with `extensions/host-interface/hi-handlers/*-hi.lua` handlers (same URI and key).

| # | URI | Methods | Handler (under /usr/local/redfish) | Actions / parameters | Notes |
|---|---|---|---|---|---|
| 1 | `/redfish` | GET | `redfish-handler.lua` |  | Version document {"v1": "/redfish/v1/"}; literal pattern `^/redfish$`; key is in PRIVILEGE_CHECKING_WHITELIST (key `resource_127`) |
| 2 | `/redfish/*` | * | `redfish/default-handler.lua` |  | Authenticates then returns 404 ResourceMissingAtURI / 405 for unknown URIs (key `default`) |
| 3 | `/redfish/... trailing "/"` | GET POST PATCH PUT DELETE | `redirect-handler.lua` |  | Pattern `^/redfish(.*)/+$`; strips trailing slashes and redirects (Location header); key is in PRIVILEGE_CHECKING_WHITELIST (key `resource_002`) |
| 4 | `/redfish/v1` | GET POST PATCH PUT DELETE | `redirect-handler.lua` |  | No trailing slash; pattern built as SERVICE_PREFIX .. "$"; route_options sets redirection_location from SERVICE_PREFIX, so it redirects to the service root (methods share the redirect list; inferred) (key `resource_001`) |
| 5 | `/redfish/v1/` | GET | `redfish/service-root.lua` |  | NoAuth GET (privilege registry ServiceRoot GET: Login\|NoAuth) (key `resource_000`) |
| 6 | `/redfish/v1/$metadata` | GET | `odata-handler.lua (static/metadata + static/oem_metadata)` |  | in PRIVILEGE_CHECKING_WHITELIST (key `resource_115`) |
| 7 | `/redfish/v1/AccountService` | GET PATCH | `redfish/account-service.lua` |  |  (key `resource_094`) |
| 8 | `/redfish/v1/AccountService/Accounts` | GET POST | `redfish/account-service/account.lua` |  |  (key `resource_091`) |
| 9 | `/redfish/v1/AccountService/Accounts/Members` | POST | `redfish/account-service/account.lua` |  | POST alias for creating a member of the parent collection (key `accounts_members`) |
| 10 | `/redfish/v1/AccountService/Accounts/{AccountId}` | GET PATCH DELETE | `redfish/account-service/account.lua` |  |  (key `resource_090`) |
| 11 | `/redfish/v1/AccountService/Accounts/{AccountId}/Certificates` | GET POST | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_collection_for_accounts`) |
| 12 | `/redfish/v1/AccountService/Accounts/{AccountId}/Certificates/Members` | POST | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_members_for_accounts`) |
| 13 | `/redfish/v1/AccountService/Accounts/{AccountId}/Certificates/{CertificateId}` | GET DELETE | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_instance_for_accounts`) |
| 14 | `/redfish/v1/AccountService/Accounts/{AccountId}/Certificates/{CertificateId}/Actions/{Action}` | POST | `redfish/certificate-service/certificate-actions.lua` | Certificate.Rekey, Certificate.Renew (db_init/certificate-service.rcmd). |  (key `certificate_actions_for_accounts`) |
| 15 | `/redfish/v1/AccountService/Accounts/{AccountId}/Certificates/{CertificateId}/{Id}ActionInfo` | GET | `redfish/actioninfo/certificate-actioninfo.lua` |  |  (key `certificate_actioninfo_for_accounts`) |
| 16 | `/redfish/v1/AccountService/ExternalAccountProviders` | GET | `redfish/account-service/additionalexternalaccountproviders.lua` |  |  (key `resource_radius_collection`) |
| 17 | `/redfish/v1/AccountService/ExternalAccountProviders/RADIUS/Oem/Ami/AdvancedRADIUSSetting` | GET PATCH | `oem/ami/advanced_radius_setting.lua` |  | RADIUSAuthorization: Administrator/Operator/User/Oem/NoAccess vendor strings (key `aimoem_redfish_extensions_radius_advanced_settings`) |
| 18 | `/redfish/v1/AccountService/ExternalAccountProviders/{ProviderId}` | GET PATCH | `redfish/account-service/additionalexternalaccountproviders.lua` |  |  (key `resource_radius`) |
| 19 | `/redfish/v1/AccountService/LDAP/Certificates` | GET POST | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_collection_for_ldap_accounts`) |
| 20 | `/redfish/v1/AccountService/LDAP/Certificates/Members` | POST | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_members_for_ldap_accounts`) |
| 21 | `/redfish/v1/AccountService/Oem/Ami/Configurations` | GET PATCH | `oem/ami/accountservice-config-handler.lua` |  | AMIAccountServiceConfigurations: PAMEnabled, PAMOrder (IPMI, LDAP, ACTIVE DIRECTORY, RADIUS); PATCH restarts web server (key `aimoem_redfish_extensions_account_service_config`) |
| 22 | `/redfish/v1/AccountService/Roles` | GET POST | `redfish/account-service/role.lua` |  |  (key `resource_093`) |
| 23 | `/redfish/v1/AccountService/Roles/Members` | POST | `redfish/account-service/role.lua` |  | POST alias for creating a member of the parent collection (key `roles_members`) |
| 24 | `/redfish/v1/AccountService/Roles/{RoleId}` | GET PATCH DELETE | `redfish/account-service/role.lua` |  | predefined roles Administrator/Operator/ReadOnly are GET-only; custom roles GET/PATCH/DELETE (key `resource_092`) |
| 25 | `/redfish/v1/AccountService/{ServiceName}/Certificates/Oem/Ami/ClientCertificates` | GET POST | `oem/ami/ldap_client_certificates.lua` |  | LDAP client certificate+key upload (key `aimoem_redfish_extensions_upload_ldap_client_certificates`) |
| 26 | `/redfish/v1/AccountService/{ServiceName}/Certificates/Oem/Ami/ClientCertificates/{CertificateId}` | GET DELETE | `oem/ami/ldap_client_certificates.lua` |  |  (key `aimoem_redfish_extensions_upload_ldap_client_certificates_instance`) |
| 27 | `/redfish/v1/AccountService/{ServiceName}/Certificates/Oem/Ami/ClientCertificates/{CertificateId}/Actions/{Action}` | POST | `redfish/certificate-service/certificate-actions.lua` | Certificate.Rekey / Certificate.Renew |  (key `aimoem_redfish_extensions_clientcertificate_actions_for_ldap_accounts`) |
| 28 | `/redfish/v1/AccountService/{ServiceName}/Certificates/Oem/Ami/ClientCertificates/{CertificateId}/{Action}ActionInfo` | GET | `redfish/actioninfo/certificate-actioninfo.lua` |  |  (key `aimoem_redfish_extensions_upload_ldap_certificates_action_info`) |
| 29 | `/redfish/v1/AccountService/{ServiceName}/Certificates/{CertificateId}` | GET DELETE | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_instance_for_ldap_accounts`) |
| 30 | `/redfish/v1/AccountService/{ServiceName}/Certificates/{CertificateId}/Actions/{Action}` | POST | `redfish/certificate-service/certificate-actions.lua` | Certificate.Rekey, Certificate.Renew (db_init/certificate-service.rcmd). |  (key `certificate_actions_for_ldap_accounts`) |
| 31 | `/redfish/v1/AccountService/{ServiceName}/Certificates/{CertificateId}/{Id}ActionInfo` | GET | `redfish/actioninfo/certificate-actioninfo.lua` |  |  (key `certificate_actioninfo_for_ldap_accounts`) |
| 32 | `/redfish/v1/CertificateService` | GET | `redfish/certificate-service.lua` |  |  (key `certificate_service`) |
| 33 | `/redfish/v1/CertificateService/Actions/{Action}` | POST | `redfish/certificate-service/certificate-service-actions.lua` | CertificateService.GenerateCSR, CertificateService.ReplaceCertificate. |  (key `certificate_service_actions`) |
| 34 | `/redfish/v1/CertificateService/CertificateLocations` | GET | `redfish/certificate-service/certificate-locations.lua` |  |  (key `certificate_locations`) |
| 35 | `/redfish/v1/CertificateService/{Id}ActionInfo` | GET | `redfish/actioninfo/certificate-service-actioninfo.lua` |  |  (key `certificate_service_actioninfo`) |
| 36 | `/redfish/v1/Chassis` | GET | `redfish/chassis.lua` |  |  (key `resource_069`) |
| 37 | `/redfish/v1/Chassis/RDE{n}_*, /redfish/v1/Systems/RDE{n}_* (dynamic)` | GET PATCH POST DELETE HEAD PUT | `pldm-rde-handler.lua + redfish/pldm-rde.lua` |  | PLDM RDE device resources, only when CONFIG_SPX_FEATURE_SELECT_RDE_DEVICE_SUPPORT is enabled (key `RDE:Collection / RDE:Instance`) |
| 38 | `/redfish/v1/Chassis/{ChassisId}` | GET PATCH | `redfish/chassis.lua` |  |  (key `resource_067`) |
| 39 | `/redfish/v1/Chassis/{ChassisId}/$ref` | GET PATCH | `redfish/chassis.lua` |  | odata $ref form of the chassis instance (key `resource_068`) |
| 40 | `/redfish/v1/Chassis/{ChassisId}/Actions/{Action}` | POST | `redfish/chassis.lua` | Chassis.Reset: ResetType from Redfish:Chassis:DefaultResetAllowedValues = On, ForceOff, GracefulShutdown, ForceRestart, PowerCycle (db_init/chassis.rcmd; action-info.rcmd also lists GracefulRestart). |  (key `resource_073`) |
| 41 | `/redfish/v1/Chassis/{ChassisId}/Assembly` | GET | `redfish/assembly.lua` |  |  (key `resource_717`) |
| 42 | `/redfish/v1/Chassis/{ChassisId}/LogServices` | GET | `redfish/chassis/log-service.lua` |  |  (key `resource_080`) |
| 43 | `/redfish/v1/Chassis/{ChassisId}/LogServices/{LogServiceId}` | GET PATCH | `redfish/chassis/log-service.lua` |  |  (key `resource_079`) |
| 44 | `/redfish/v1/Chassis/{ChassisId}/LogServices/{LogServiceId}/Actions/{Action}` | POST | `redfish/chassis/log-service-actions.lua` | LogService.ClearLog (Logs). |  (key `resource_077`) |
| 45 | `/redfish/v1/Chassis/{ChassisId}/LogServices/{LogServiceId}/Entries` | GET | `redfish/chassis/log-service-entries.lua` |  |  (key `resource_075`) |
| 46 | `/redfish/v1/Chassis/{ChassisId}/LogServices/{LogServiceId}/Entries/{EntryId}` | GET | `redfish/chassis/log-service-entries.lua` |  |  (key `resource_076`) |
| 47 | `/redfish/v1/Chassis/{ChassisId}/MediaControllers` | GET | `redfish/chassis/media-controllers.lua` |  |  (key `resource_148`) |
| 48 | `/redfish/v1/Chassis/{ChassisId}/MediaControllers/{MediaControllerId}` | GET | `redfish/chassis/media-controllers.lua` |  |  (key `resource_147`) |
| 49 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters` | GET | `redfish/chassis/networkadapter-collection.lua` |  |  (key `resource_083`) |
| 50 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}` | GET | `redfish/chassis/networkadapter-instance.lua` |  |  (key `resource_082`) |
| 51 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Actions/{Action}` | POST | `redfish/chassis/networkadapter-actions.lua` | NetworkAdapter.ResetSettingsToDefault. |  (key `resource_081`) |
| 52 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Assembly` | GET | `redfish/assembly.lua` |  |  (key `resource_710`) |
| 53 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/NetworkDeviceFunctions` | GET | `redfish/chassis/networkdevicefunction-collection.lua` |  |  (key `resource_087`) |
| 54 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/NetworkDeviceFunctions/{NetworkDeviceFunctionId}` | GET | `redfish/chassis/networkdevicefunction-instance.lua` |  |  (key `resource_086`) |
| 55 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/NetworkDeviceFunctions/{NetworkDeviceFunctionId}/Ethernet/VLANs` | GET POST | `redfish/system/vlan.lua` |  |  (key `resource_129`) |
| 56 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/NetworkDeviceFunctions/{NetworkDeviceFunctionId}/Ethernet/VLANs/Members` | POST | `redfish/system/vlan.lua` |  | POST alias for creating a member of the parent collection (key `resource_129_members`) |
| 57 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/NetworkDeviceFunctions/{NetworkDeviceFunctionId}/Ethernet/VLANs/{VlanId}` | GET DELETE | `redfish/system/vlan.lua` |  |  (key `resource_128`) |
| 58 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/NetworkDeviceFunctions/{NetworkDeviceFunctionId}/Ethernet/VLANs/{VlanId}/SD` | GET PATCH | `redfish/system/vlan.lua` |  | Settings object (@Redfish.Settings "SD" = pending settings, applied later) (key `resource_128_SD`) |
| 59 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/NetworkDeviceFunctions/{NetworkDeviceFunctionId}/SD` | GET PATCH | `redfish/chassis/networkdevicefunction-instance.lua` |  | Settings object (@Redfish.Settings "SD" = pending settings, applied later) (key `resource_086_SD`) |
| 60 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/NetworkPorts` | GET | `redfish/chassis/networkport-collection.lua` |  |  (key `resource_085`) |
| 61 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/NetworkPorts/{NetworkPortId}` | GET | `redfish/chassis/networkport-instance.lua` |  |  (key `resource_084`) |
| 62 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Ports` | GET | `redfish/chassis/ports.lua` |  |  (key `resource_142`) |
| 63 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Ports/{PortId}` | GET | `redfish/chassis/ports.lua` |  |  (key `resource_141`) |
| 64 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Processors` | GET | `redfish/system/processor-collection.lua / processor-instance.lua (assumed)` |  |  (key `NetworkAdaptersProcessorsCollection`) |
| 65 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Processors/{ProcessorId}` | GET | `redfish/system/processor-collection.lua / processor-instance.lua (assumed)` |  |  (key `NetworkAdaptersProcessorsInstance`) |
| 66 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Processors/{ProcessorId}/AccelerationFunctions` | GET | `redfish/system/acceleration-function.lua (assumed)` |  |  (key `NetworkAdaptersProcessorsAccFunCollection`) |
| 67 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Processors/{ProcessorId}/AccelerationFunctions/{AccelFnId}` | GET | `redfish/system/acceleration-function.lua (assumed)` |  |  (key `NetworkAdaptersProcessorsAccFunInstance`) |
| 68 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Processors/{ProcessorId}/OperatingConfigs` | GET | `redfish/system/operatingconfig.lua (assumed)` |  |  (key `NetworkAdaptersProcessorsOptCfgCollection`) |
| 69 | `/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Processors/{ProcessorId}/OperatingConfigs/{OpConfigId}` | GET | `redfish/system/operatingconfig.lua (assumed)` |  |  (key `NetworkAdaptersProcessorsOptCfgInstance`) |
| 70 | `/redfish/v1/Chassis/{ChassisId}/Oem/Ami/MapoutPorting` | GET? | `oem/ami_bios/pciefunlist-handler.lua (assumed)` |  |  (key `ami_bios_dre_chassis_mapoutporting`) |
| 71 | `/redfish/v1/Chassis/{ChassisId}/PCIeDevices` | GET | `redfish/chassis/pcie-devices.lua` |  |  (key `resource_131`) |
| 72 | `/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}` | GET | `redfish/chassis/pcie-devices.lua` |  |  (key `resource_088`) |
| 73 | `/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}/Assembly` | GET | `redfish/assembly.lua` |  |  (key `resource_709`) |
| 74 | `/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}/PCIeFunctions` | GET | `redfish/chassis/pcie-device-functions.lua` |  |  (key `resource_132`) |
| 75 | `/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}/PCIeFunctions/{PCIeFunctionId}` | GET | `redfish/chassis/pcie-device-functions.lua` |  |  (key `resource_089`) |
| 76 | `/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}/PCIeFunctions/{PCIeFunctionId}/Actions/{Action}` | POST | `oem/ami_bios/mapout-handler.lua (assumed)` | AmiBios.ChangeState (Enabled\|Disabled) - map out a PCIe function |  (key `ami_bios_dre_chassis_pciefunctions_actions`) |
| 77 | `/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}/PCIeFunctions/{PCIeFunctionId}/ChangeStateActionInfo` | GET | `oem/ami_bios/changestate-actioninfo.lua` |  |  (key `ami_bios_dre_chassis_pciefunctions_changestateactioninfo`) |
| 78 | `/redfish/v1/Chassis/{ChassisId}/PCIeSlots` | GET | `redfish/chassis/pcie-slots.lua` |  |  (key `resource_708`) |
| 79 | `/redfish/v1/Chassis/{ChassisId}/Power` | GET PATCH | `redfish/chassis/power.lua` |  |  (key `resource_070`) |
| 80 | `/redfish/v1/Chassis/{ChassisId}/PowerSubsystem` | GET | `redfish/chassis/power-subsystem.lua` |  |  (key `PowerSubSys_1`) |
| 81 | `/redfish/v1/Chassis/{ChassisId}/PowerSubsystem/PowerSupplies` | GET | `redfish/chassis/powersupplies.lua` |  |  (key `PowerSubSys_2`) |
| 82 | `/redfish/v1/Chassis/{ChassisId}/PowerSubsystem/PowerSupplies/{PowerSupplyId}` | GET | `redfish/chassis/powersupplies.lua` |  |  (key `PowerSubSys_3`) |
| 83 | `/redfish/v1/Chassis/{ChassisId}/PowerSubsystem/PowerSupplies/{PowerSupplyId}/Metrics` | GET | `redfish/chassis/powersupplies-metrics.lua` |  |  (key `PowerSubSys_4`) |
| 84 | `/redfish/v1/Chassis/{ChassisId}/ResetActionInfo` | GET | `redfish/actioninfo/reset-actioninfo.lua` |  |  (key `resource_074`) |
| 85 | `/redfish/v1/Chassis/{ChassisId}/Sensors` | GET | `redfish/chassis/sensor.lua` |  |  (key `sensor_collection`) |
| 86 | `/redfish/v1/Chassis/{ChassisId}/Sensors/{SensorId}` | GET | `redfish/chassis/sensor.lua` |  |  (key `sensor`) |
| 87 | `/redfish/v1/Chassis/{ChassisId}/Thermal` | GET | `redfish/chassis/thermal.lua` |  |  (key `resource_071`) |
| 88 | `/redfish/v1/Chassis/{ChassisId}/Thermal/... (Thermal-Instance)` | PATCH | `redfish/chassis/thermal-instance.lua` |  | Key has allow entry (PATCH, label Thermal-Instance) but no literal URI pattern; likely threshold PATCH on Thermal members (key `resource_072`) |
| 89 | `/redfish/v1/Chassis/{ChassisId}/ThermalSubsystem` | GET | `redfish/chassis/thermal-subsystem.lua` |  |  (key `ThermalSubSys_1`) |
| 90 | `/redfish/v1/Chassis/{ChassisId}/ThermalSubsystem/Fans` | GET | `redfish/chassis/fans.lua` |  |  (key `ThermalSubSys_2`) |
| 91 | `/redfish/v1/Chassis/{ChassisId}/ThermalSubsystem/Fans/{FanId}` | GET | `redfish/chassis/fans.lua` |  |  (key `ThermalSubSys_3`) |
| 92 | `/redfish/v1/Chassis/{ChassisId}/ThermalSubsystem/ThermalMetrics` | GET | `redfish/chassis/thermal-metrics.lua` |  |  (key `ThermalSubSys_4`) |
| 93 | `/redfish/v1/CompositionService` | GET PATCH | `redfish/compositionservice.lua` |  |  (key `resource_118`) |
| 94 | `/redfish/v1/CompositionService/ResourceBlocks` | GET | `redfish/compositionservice/resourceblock-collection.lua / resourceblock-instance.lua` |  |  (key `resource_120`) |
| 95 | `/redfish/v1/CompositionService/ResourceBlocks/{ResourceBlockId}` | GET PATCH | `redfish/compositionservice/resourceblock-collection.lua / resourceblock-instance.lua` |  |  (key `resource_119`) |
| 96 | `/redfish/v1/CompositionService/ResourceZones` | GET | `redfish/compositionservice/zone-collection.lua / zone-instance.lua` |  |  (key `resource_122`) |
| 97 | `/redfish/v1/CompositionService/ResourceZones/{ZoneId}` | GET | `redfish/compositionservice/zone-collection.lua / zone-instance.lua` |  |  (key `resource_121`) |
| 98 | `/redfish/v1/EventService` | GET PATCH | `redfish/event-service.lua` |  |  (key `resource_103`) |
| 99 | `/redfish/v1/EventService/Actions/EventService.SubmitDelayedTestEvent (allow-table only)` | POST | `oem/ami/delayed-test-event.lua` | Oem SubmitDelayedTestEvent (DelayTime, EventType) | Allow entry + handler file exist but no URI pattern registered in 9.05 (orphan) (key `aimoem_redfish_extensions_delayed_event`) |
| 100 | `/redfish/v1/EventService/Actions/EventService.SubmitTestEvent` | POST | `redfish/event-service.lua` | EventService.SubmitTestEvent (EventId, EventTimestamp, MessageArgs, MessageId (required), OriginOfCondition, Severity). Requires an active Event subscription. | Pattern built from default_constants.SEND_TEST_TARGET (key `resource_098`) |
| 101 | `/redfish/v1/EventService/Oem/Ami/SMTP/Certificates` | GET POST | `oem/ami/event-service-certificate.lua` |  | AMISMTPCertificateCollection (Primary/Secondary SMTP CACert, Cert, PrivateKey) (key `aimoem_redfish_extensions_eventservice_smtp_certificate_collection`) |
| 102 | `/redfish/v1/EventService/Oem/Ami/SMTP/Certificates/Members` | POST | `oem/ami/event-service-certificate.lua` |  | POST alias (key `aimoem_redfish_extensions_eventservice_smtp_certificate_collection_members`) |
| 103 | `/redfish/v1/EventService/Oem/Ami/SMTP/Certificates/{CertificateId}` | GET POST DELETE | `oem/ami/event-service-certificate.lua` |  | Instances 1 (primary) and 2 (secondary) exist by default (key `aimoem_redfish_extensions_eventservice_smtp_certificate_instance`) |
| 104 | `/redfish/v1/EventService/SSE` | GET | `event-service/ServerSentEvent.lua (lighttpd -> 127.0.0.1:9090)` |  | Server-Sent Events stream; proxied by lighttpd to 127.0.0.1:9090; text/event-stream; supports $filter on EventFormatType/MessageId/OriginResource/RegistryPrefix/ResourceType (key `resource_140`) |
| 105 | `/redfish/v1/EventService/SubmitDelayedTestEventActionInfo (allow-table only)` | GET | `oem/ami/actioninfo/submitdelayedtestevent-actioninfo.lua` |  | orphan (no URI pattern) (key `aimoem_redfish_extensions_submit_delayed_test_event_action_info`) |
| 106 | `/redfish/v1/EventService/SubmitTestEventActionInfo` | GET | `redfish/actioninfo/submittestevent-actioninfo.lua` |  |  (key `resource_099`) |
| 107 | `/redfish/v1/EventService/Subscriptions` | GET POST | `redfish/event-service.lua` |  |  (key `resource_102`) |
| 108 | `/redfish/v1/EventService/Subscriptions/Members` | GET POST | `redfish/event-service.lua` |  |  (key `resource_101`) |
| 109 | `/redfish/v1/EventService/Subscriptions/Members` | POST | `redfish/event-service.lua` |  | POST alias for creating a member of the parent collection (key `subscriptions_members`) |
| 110 | `/redfish/v1/EventService/Subscriptions/{SubscriptionId}` | GET PATCH DELETE | `redfish/event-service.lua` |  |  (key `resource_100`) |
| 111 | `/redfish/v1/EventService/Subscriptions/{SubscriptionId}/Actions/EventDestination.ResumeSubscription` | POST | `redfish/resumesubscription-action.lua` | EventDestination.ResumeSubscription. |  (key `resumesubscription_action_handler`) |
| 112 | `/redfish/v1/Fabrics/{FabricId}/Endpoints` | GET | `redfish/endpoint-collection.lua` |  |  (key `resource_117`) |
| 113 | `/redfish/v1/Fabrics/{FabricId}/Endpoints/{EndpointId}` | GET PATCH | `redfish/endpoint-instance.lua` |  |  (key `resource_116`) |
| 114 | `/redfish/v1/GlobalLuaTables` | GET DELETE | `redfish/global-lua-tables.lua` |  | debug: lists/clears the server resource-metadata cache; key is in config.PRIVILEGE_CHECKING_WHITELIST (key `resource_126`) |
| 115 | `/redfish/v1/JsonSchemas` | GET | `redfish/json-schema.lua` |  |  (key `resource_112`) |
| 116 | `/redfish/v1/JsonSchemas/{SchemaFile}` | GET | `redfish/json-schema.lua` |  |  (key `resource_111`) |
| 117 | `/redfish/v1/Managers` | GET | `redfish/manager.lua` |  |  (key `resource_020`) |
| 118 | `/redfish/v1/Managers/{ManagerId}` | GET PATCH | `redfish/manager.lua` |  |  (key `resource_019`) |
| 119 | `/redfish/v1/Managers/{ManagerId}/Actions/Oem/AMIManager.BackupConfig` | POST | `oem/ami/backup-config.lua` | AMIManager.BackupConfig (BackupFeatures[]) -> returns config archive | Pattern built with get_env_managers_name() (only the real manager id) (key `amioem_redfish_extensions_backupconfig`) |
| 120 | `/redfish/v1/Managers/{ManagerId}/Actions/Oem/AMIManager.RedfishPowerSaveMode` | POST | `oem/ami/power-saver.lua` | AMIManager.RedfishPowerSaveMode (PowerSaveMode Enable\|Disable) |  (key `amioem_redfish_extensions_reset_powersavemode`) |
| 121 | `/redfish/v1/Managers/{ManagerId}/Actions/Oem/AMIManager.RestoreConfig` | POST | `oem/ami/restore-config.lua` | AMIManager.RestoreConfig (multipart conf_file) |  (key `amioem_redfish_extensions_restoreconfig`) |
| 122 | `/redfish/v1/Managers/{ManagerId}/Actions/Oem/AMIVirtualMedia.ConfigureCDInstance` | POST | `oem/ami/configure-cdinstance.lua` | AMIVirtualMedia.ConfigureCDInstance (CDInstance count) |  (key `aimoem_redfish_extensions_configure_cd_instance`) |
| 123 | `/redfish/v1/Managers/{ManagerId}/Actions/Oem/AMIVirtualMedia.EnableRMedia` | POST | `oem/ami/rmedia-support.lua` | AMIVirtualMedia.EnableRMedia (RMediaState Enable\|Disable) - must be enabled before VirtualMedia.InsertMedia |  (key `aimoem_redfish_extensions_rmedia_support`) |
| 124 | `/redfish/v1/Managers/{ManagerId}/Actions/{Action}` | POST | `redfish/manager.lua` | Manager.Reset: ResetType ForceRestart only (action-info.rcmd, reset-actioninfo.lua). OEM actions under /Actions/Oem/* are routed by oem/ami (see OEM rows). |  (key `resource_017`) |
| 125 | `/redfish/v1/Managers/{ManagerId}/Certificates` | GET | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_collection_for_managers`) |
| 126 | `/redfish/v1/Managers/{ManagerId}/Certificates/Members` |  | `redfish/certificate-service/certificate.lua` |  | no method list in allow table (effectively not allowed) (key `certificate_members_for_managers`) |
| 127 | `/redfish/v1/Managers/{ManagerId}/Certificates/{CertificateId}` | GET | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_instance_for_managers`) |
| 128 | `/redfish/v1/Managers/{ManagerId}/Certificates/{CertificateId}/Actions/{Action}` | POST | `redfish/certificate-service/certificate-actions.lua` | Certificate.Rekey, Certificate.Renew (db_init/certificate-service.rcmd). |  (key `certificate_actions_for_managers`) |
| 129 | `/redfish/v1/Managers/{ManagerId}/Certificates/{CertificateId}/{Id}ActionInfo` | GET | `redfish/actioninfo/certificate-actioninfo.lua` |  |  (key `certificate_actioninfo_for_managers`) |
| 130 | `/redfish/v1/Managers/{ManagerId}/EthernetInterfaces` | GET | `redfish/manager/ethernet-interface.lua` |  |  (key `resource_016`) |
| 131 | `/redfish/v1/Managers/{ManagerId}/EthernetInterfaces/{EthernetInterfaceId}` | GET PATCH | `redfish/manager/ethernet-interface.lua` |  | usb0 (host-interface NIC) is GET-only via allowed_methods override (key `resource_015`) |
| 132 | `/redfish/v1/Managers/{ManagerId}/EthernetInterfaces/{EthernetInterfaceId}/{Id}` | GET PATCH | `redfish/manager/ethernet-interface.lua` |  |  (key `resource_014`) |
| 133 | `/redfish/v1/Managers/{ManagerId}/HostInterfaces` | GET | `extensions/host-interface/hostinterface-collection.lua` |  |  (key `hi_resource_02`) |
| 134 | `/redfish/v1/Managers/{ManagerId}/HostInterfaces/{HostInterfaceId}` | GET PATCH | `extensions/host-interface/hostinterface-instance.lua` |  | Redfish Host Interface (usb0, 169.254.0.17/18); FirmwareAuthEnabled/KernelAuthEnabled/AuthNoneRoleId (key `hi_resource_01`) |
| 135 | `/redfish/v1/Managers/{ManagerId}/HostInterfaces/{HostInterfaceId}/HostEthernetInterfaces` | GET | `extensions/host-interface/host-ethernet-interface-collection.lua` |  |  (key `hi_resource_05`) |
| 136 | `/redfish/v1/Managers/{ManagerId}/IKVM` | GET | `oem/plugin/ikvm.lua` |  | ASRock-Rack-added plugin (no Oem namespace); linked from Manager.IKVM (key `plugin_redfish_ikvm`) |
| 137 | `/redfish/v1/Managers/{ManagerId}/LogServices` | GET | `redfish/manager/log-service.lua` |  |  (key `resource_013`) |
| 138 | `/redfish/v1/Managers/{ManagerId}/LogServices/{LogServiceId}` | GET PATCH | `redfish/manager/log-service.lua` |  |  (key `resource_012`) |
| 139 | `/redfish/v1/Managers/{ManagerId}/LogServices/{LogServiceId}/Actions/{Action}` | POST | `redfish/manager/log-service-actions.lua` | LogService.ClearLog (SEL, EventLog, AuditLog). |  (key `resource_010`) |
| 140 | `/redfish/v1/Managers/{ManagerId}/LogServices/{LogServiceId}/Entries` | GET | `redfish/manager/log-service-entries.lua` |  |  (key `resource_009`) |
| 141 | `/redfish/v1/Managers/{ManagerId}/LogServices/{LogServiceId}/Entries/{EntryId}` | GET | `redfish/manager/log-service-entries.lua` |  |  (key `resource_008`) |
| 142 | `/redfish/v1/Managers/{ManagerId}/NetworkProtocol` | GET PATCH | `redfish/manager/network-protocol.lua` |  |  (key `resource_007`) |
| 143 | `/redfish/v1/Managers/{ManagerId}/NetworkProtocol/HTTPS/Certificates` | GET | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_collection_for_https`) |
| 144 | `/redfish/v1/Managers/{ManagerId}/NetworkProtocol/HTTPS/Certificates/{CertificateId}` | GET | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_instance_for_https`) |
| 145 | `/redfish/v1/Managers/{ManagerId}/NetworkProtocol/HTTPS/Certificates/{CertificateId}/Actions/{Action}` | POST | `redfish/certificate-service/certificate-actions.lua` | Certificate.Rekey, Certificate.Renew (db_init/certificate-service.rcmd). |  (key `certificate_actions_for_https`) |
| 146 | `/redfish/v1/Managers/{ManagerId}/NetworkProtocol/HTTPS/Certificates/{CertificateId}/{Id}ActionInfo` | GET | `redfish/actioninfo/certificate-actioninfo.lua` |  |  (key `certificate_actioninfo_for_https`) |
| 147 | `/redfish/v1/Managers/{ManagerId}/Oem/BackupConfigActionInfo` | GET | `oem/ami/actioninfo/backupconfig-actioninfo.lua` |  |  (key `amioem_redfish_extensions_backupconfig_action_info`) |
| 148 | `/redfish/v1/Managers/{ManagerId}/Oem/ConfigureCDInstanceActionInfo` | GET POST | `oem/ami/actioninfo/configurecdinstance-actioninfo.lua` |  | allow table lists GET, POST (key `aimoem_redfish_extensions_configure_cd_instance_action_info`) |
| 149 | `/redfish/v1/Managers/{ManagerId}/Oem/EnableRMediaActionInfo` | GET | `oem/ami/actioninfo/rmediasupport-actioninfo.lua` |  |  (key `aimoem_redfish_extensions_rmedia_support_action_info`) |
| 150 | `/redfish/v1/Managers/{ManagerId}/Oem/RedfishPowerSaveModeActionInfo` | GET | `oem/ami/actioninfo/powersaver-actioninfo.lua` |  |  (key `amioem_redfish_extensions_reset_powersavemode_action_info`) |
| 151 | `/redfish/v1/Managers/{ManagerId}/Oem/RestoreConfigActionInfo` | GET | `oem/ami/actioninfo/restoreconfig-actioninfo.lua` |  |  (key `amioem_redfish_extensions_restoreconfig_action_info`) |
| 152 | `/redfish/v1/Managers/{ManagerId}/ResetActionInfo` | GET | `redfish/actioninfo/reset-actioninfo.lua` |  |  (key `resource_018`) |
| 153 | `/redfish/v1/Managers/{ManagerId}/SerialInterfaces` | GET | `redfish/manager/serial-interface.lua` |  |  (key `resource_006`) |
| 154 | `/redfish/v1/Managers/{ManagerId}/SerialInterfaces/{SerialInterfaceId}` | GET PATCH | `redfish/manager/serial-interface.lua` |  |  (key `resource_005`) |
| 155 | `/redfish/v1/Managers/{ManagerId}/VirtualMedia` | GET | `redfish/manager/virtual-media.lua` |  |  (key `resource_004`) |
| 156 | `/redfish/v1/Managers/{ManagerId}/VirtualMedia/{VirtualMediaId}` | GET PATCH | `redfish/manager/virtual-media.lua` |  |  (key `resource_003`) |
| 157 | `/redfish/v1/Managers/{ManagerId}/VirtualMedia/{VirtualMediaId}/Actions/{Action}` | POST | `redfish/manager/virtual-media-actions.lua` | VirtualMedia.InsertMedia (Image req., TransferProtocolType req. NFS\|CIFS\|HTTPS, TransferMethod Stream, Inserted, WriteProtected, UserName, Password; .iso/.nrg images) and VirtualMedia.EjectMedia. Runs as a task. |  (key `resource_700`) |
| 158 | `/redfish/v1/Managers/{ManagerId}/VirtualMedia/{VirtualMediaId}/EjectMediaActionInfo` | GET | `redfish/actioninfo/virtualmedia-actioninfo.lua` |  |  (key `resource_701`) |
| 159 | `/redfish/v1/Managers/{ManagerId}/VirtualMedia/{VirtualMediaId}/InsertMediaActionInfo` | GET | `redfish/actioninfo/virtualmedia-actioninfo.lua` |  |  (key `resource_702`) |
| 160 | `/redfish/v1/Oem/Ami/BiosStaticFiles` | GET? POST? | `oem/ami_hi/static-file-collection-hi.lua` |  |  (key `amioem_hi_redfish_extension_bios_static_file_collection`) |
| 161 | `/redfish/v1/Oem/Ami/BiosStaticFiles/PostStatus/{Id}` | GET? | `oem/ami_hi/post-status.lua` |  |  (key `amioem_hi_redfish_extension_bios_static_file_post_status`) |
| 162 | `/redfish/v1/Oem/Ami/BiosStaticFiles/{FileId}` | GET | `oem/ami_hi/static-file-instance-hi.lua` |  |  (key `amioem_hi_redfish_extension_bios_static_file_instance`) |
| 163 | `/redfish/v1/Oem/Ami/Configurations` | GET PATCH | `oem/ami/config-handler.lua` |  | AMIConfigurations: CertificateAuthorityUrl (key `aimoem_redfish_extensions_config`) |
| 164 | `/redfish/v1/Oem/Ami/InventoryData` | GET? POST? | `oem/ami_hi/hi-inventory-data.lua` |  | BIOS inventory upload (host interface); entity BiosStaticFileCollection (key `amioem_hi_redfish_extension_inventory_data`) |
| 165 | `/redfish/v1/Oem/Ami/InventoryData/Status` | GET | `oem/ami_hi/inventory-data-status.lua` |  | AMIInventoryStatus (key `amioem_hi_redfish_extension_inventory_data_status`) |
| 166 | `/redfish/v1/Registries` | GET | `redfish/registry.lua (overridden by extensions/host-interface/hi-handlers/registry-*-hi.lua)` |  |  (key `resource_110`) |
| 167 | `/redfish/v1/PrivilegeRegistry/Redfish_1.3.0_PrivilegeRegistry.json` |  | none registered |  | Pattern built as SERVICE_PREFIX .. "/PrivilegeRegistry/" .. PRIVILEGE_REGISTRY_JSON_FILE, but the key has no handler or allow entry, so the URI is probably unreachable (falls through to 404). The privilege registries are served under `/Registries/{RegistryFile}` instead (key `resource_130`) |
| 168 | `/redfish/v1/Registries/Static/{RegistryFile}` | GET | `redfish/registry.lua (overridden by extensions/host-interface/hi-handlers/registry-*-hi.lua)` |  |  (key `resource_108`) |
| 169 | `/redfish/v1/Registries/{RegistryFile}` | GET | `redfish/registry.lua (overridden by extensions/host-interface/hi-handlers/registry-*-hi.lua)` |  |  (key `resource_109`) |
| 170 | `/redfish/v1/SessionService` | GET PATCH | `redfish/session-service.lua` |  |  (key `resource_097`) |
| 171 | `/redfish/v1/SessionService/Sessions` | GET POST | `redfish/session-service.lua` |  |  (key `resource_096`) |
| 172 | `/redfish/v1/SessionService/Sessions/Members` | POST | `redfish/session-service.lua` |  | POST alias for creating a member of the parent collection (key `sessions_members`) |
| 173 | `/redfish/v1/SessionService/Sessions/{SessionId}` | GET DELETE | `redfish/session-service.lua` |  |  (key `resource_095`) |
| 174 | `/redfish/v1/Systems` | GET POST | `redfish/system.lua` |  |  (key `resource_057`) |
| 175 | `/redfish/v1/Systems/Members` | POST | `redfish/system.lua` |  | POST alias for creating a member of the parent collection (key `systems_members`) |
| 176 | `/redfish/v1/Systems/{SystemId}` | GET PATCH DELETE | `redfish/system.lua` |  | override table: target "Capabilities" listed; physical system GET/PATCH, composed systems also DELETE (inferred from table shape) (key `resource_056`) |
| 177 | `/redfish/v1/Systems/{SystemId}/Actions/{Action}` | POST | `redfish/system-actions.lua` | ComputerSystem.Reset: ResetType in redis set Redfish:Systems:DefaultResetAllowedValues = On, ForceOff, GracefulShutdown, ForceRestart, PowerCycle (db_init/systems.rcmd). Supports @Redfish.OperationApplyTime Immediate\|AtMaintenanceWindowStart (+ MaintenanceWindowStartTime). Returns a task (TaskMonitor). |  (key `resource_054`) |
| 178 | `/redfish/v1/Systems/{SystemId}/Bios` | GET | `redfish/system/bios.lua (HI override: extensions/host-interface/hi-handlers/bios-hi.lua)` |  |  (key `resource_053`) |
| 179 | `/redfish/v1/Systems/{SystemId}/Bios/Actions/{Action}` | POST | `redfish/system/bios-actions.lua (HI override: extensions/host-interface/hi-handlers/bios-actions-hi.lua)` | Bios.ResetBios (no params) and Bios.ChangePassword (PasswordName, OldPassword, NewPassword; all required). 503 ServiceTemporarilyUnavailableDueToHostBooting while host is in POST. |  (key `resource_050`) |
| 180 | `/redfish/v1/Systems/{SystemId}/Bios/ChangePasswordActionInfo` | GET | `redfish/actioninfo/changepassword-actioninfo.lua` |  |  (key `resource_052`) |
| 181 | `/redfish/v1/Systems/{SystemId}/Bios/SD` | GET PATCH | `redfish/system/bios.lua (HI override: extensions/host-interface/hi-handlers/bios-hi.lua)` |  |  (key `resource_049`) |
| 182 | `/redfish/v1/Systems/{SystemId}/Boot/Certificates` | GET POST | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_collection_for_boot`) |
| 183 | `/redfish/v1/Systems/{SystemId}/Boot/Certificates/Members` | POST | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_members_for_boot`) |
| 184 | `/redfish/v1/Systems/{SystemId}/Boot/Certificates/{CertificateId}` | GET DELETE | `redfish/certificate-service/certificate.lua` |  |  (key `certificate_instance_for_boot`) |
| 185 | `/redfish/v1/Systems/{SystemId}/Boot/Certificates/{CertificateId}/Actions/{Action}` | POST | `redfish/certificate-service/certificate-actions.lua` | Certificate.Rekey, Certificate.Renew (db_init/certificate-service.rcmd). |  (key `certificate_actions_for_boot`) |
| 186 | `/redfish/v1/Systems/{SystemId}/Boot/Certificates/{CertificateId}/{Id}ActionInfo` | GET | `redfish/actioninfo/certificate-actioninfo.lua` |  |  (key `certificate_actioninfo_for_boot`) |
| 187 | `/redfish/v1/Systems/{SystemId}/BootOptions` | GET POST | `redfish/system/bootoption.lua` |  |  (key `resource_124`) |
| 188 | `/redfish/v1/Systems/{SystemId}/BootOptions/Members` | POST | `redfish/system/bootoption.lua` |  |  (key `bootoption_member`) |
| 189 | `/redfish/v1/Systems/{SystemId}/BootOptions/{BootOptionId}` | GET | `redfish/system/bootoption.lua` |  |  (key `resource_123`) |
| 190 | `/redfish/v1/Systems/{SystemId}/BootOptions/{BootOptionId}/SD` | GET PATCH | `redfish/system/bootoption.lua` |  | Settings object (@Redfish.Settings "SD" = pending settings, applied later) (key `resource_123_SD`) |
| 191 | `/redfish/v1/Systems/{SystemId}/EthernetInterfaces` | GET | `redfish/system/ethernet-interface.lua` |  |  (key `resource_048`) |
| 192 | `/redfish/v1/Systems/{SystemId}/EthernetInterfaces/{EthernetInterfaceId}` | GET | `redfish/system/ethernet-interface.lua` |  |  (key `resource_047`) |
| 193 | `/redfish/v1/Systems/{SystemId}/EthernetInterfaces/{EthernetInterfaceId}/VLANs` | GET POST | `redfish/system/vlan.lua` |  |  (key `resource_046`) |
| 194 | `/redfish/v1/Systems/{SystemId}/EthernetInterfaces/{EthernetInterfaceId}/VLANs/Members` | POST | `redfish/system/vlan.lua` |  | POST alias for creating a member of the parent collection (key `resource_046_members`) |
| 195 | `/redfish/v1/Systems/{SystemId}/EthernetInterfaces/{EthernetInterfaceId}/VLANs/{VlanId}` | GET DELETE | `redfish/system/vlan.lua` |  |  (key `resource_045`) |
| 196 | `/redfish/v1/Systems/{SystemId}/EthernetInterfaces/{EthernetInterfaceId}/VLANs/{VlanId}/SD` | GET PATCH | `redfish/system/vlan.lua` |  | Settings object (@Redfish.Settings "SD" = pending settings, applied later) (key `resource_045_SD`) |
| 197 | `/redfish/v1/Systems/{SystemId}/FruInfo` | GET | `oem/plugin/fruinfo.lua` |  | Plugin FRU dump (key `plugin_redfish_fru_info`) |
| 198 | `/redfish/v1/Systems/{SystemId}/GraphicsControllers` | GET | `redfish/system/graphics-controllers.lua` |  |  (key `resource_150`) |
| 199 | `/redfish/v1/Systems/{SystemId}/GraphicsControllers/{ControllerId}` | GET | `redfish/system/graphics-controllers.lua` |  |  (key `resource_149`) |
| 200 | `/redfish/v1/Systems/{SystemId}/GraphicsControllers/{ControllerId}/Ports` | GET | `redfish/system/port.lua` |  |  (key `resource_156`) |
| 201 | `/redfish/v1/Systems/{SystemId}/GraphicsControllers/{ControllerId}/Ports/{PortId}` | GET | `redfish/system/port.lua` |  |  (key `resource_155`) |
| 202 | `/redfish/v1/Systems/{SystemId}/LogServices` | GET | `redfish/system/log-service.lua` |  |  (key `resource_044`) |
| 203 | `/redfish/v1/Systems/{SystemId}/LogServices/{LogServiceId}` | GET PATCH | `redfish/system/log-service.lua` |  |  (key `resource_043`) |
| 204 | `/redfish/v1/Systems/{SystemId}/LogServices/{LogServiceId}/Actions/{Action}` | POST | `redfish/system/log-service-actions.lua` | LogService.ClearLog (BIOS, SMI). |  (key `resource_041`) |
| 205 | `/redfish/v1/Systems/{SystemId}/LogServices/{LogServiceId}/Entries` | GET | `redfish/system/log-service-entries.lua` |  |  (key `resource_040`) |
| 206 | `/redfish/v1/Systems/{SystemId}/LogServices/{LogServiceId}/Entries/{EntryId}` | GET | `redfish/system/log-service-entries.lua` |  |  (key `resource_039`) |
| 207 | `/redfish/v1/Systems/{SystemId}/Memory` | GET | `redfish/system/memory.lua` |  |  (key `resource_031`) |
| 208 | `/redfish/v1/Systems/{SystemId}/Memory/{MemoryId}` | GET | `redfish/system/memory.lua` |  |  (key `resource_030`) |
| 209 | `/redfish/v1/Systems/{SystemId}/Memory/{MemoryId}/Actions/{Action}` | POST | `oem/ami_bios/mapout-handler.lua (assumed)` | AmiBios.ChangeState (Enabled\|Disabled) - map out a DIMM |  (key `ami_bios_dre_system_memory_actions`) |
| 210 | `/redfish/v1/Systems/{SystemId}/Memory/{MemoryId}/Assembly` | GET | `redfish/assembly.lua` |  |  (key `resource_713`) |
| 211 | `/redfish/v1/Systems/{SystemId}/Memory/{MemoryId}/ChangeStateActionInfo` | GET | `oem/ami_bios/changestate-actioninfo.lua` |  |  (key `ami_bios_dre_system_memory_changestateactioninfo`) |
| 212 | `/redfish/v1/Systems/{SystemId}/Memory/{MemoryId}/MemoryMetrics` | GET | `redfish/system/memorymetrics-instance.lua` |  |  (key `resource_036`) |
| 213 | `/redfish/v1/Systems/{SystemId}/MemoryDomains` | GET | `redfish/system/memorydomain.lua` |  |  (key `resource_034`) |
| 214 | `/redfish/v1/Systems/{SystemId}/MemoryDomains/{MemoryDomainId}` | GET | `redfish/system/memorydomain.lua` |  |  (key `resource_035`) |
| 215 | `/redfish/v1/Systems/{SystemId}/MemoryDomains/{MemoryDomainId}/MemoryChunks` | GET | `redfish/system/memorychunks-collection.lua` |  |  (key `resource_033`) |
| 216 | `/redfish/v1/Systems/{SystemId}/MemoryDomains/{MemoryDomainId}/MemoryChunks/{MemoryChunkId}` | GET | `redfish/system/memorychunks-instance.lua` |  |  (key `resource_032`) |
| 217 | `/redfish/v1/Systems/{SystemId}/MemorySummary/MemoryMetrics` | GET | `redfish/system/memorymetrics-instance.lua` |  |  (key `resource_133`) |
| 218 | `/redfish/v1/Systems/{SystemId}/NetworkInterfaces` | GET | `redfish/system/networkinterface-collection.lua` |  |  (key `resource_060`) |
| 219 | `/redfish/v1/Systems/{SystemId}/NetworkInterfaces/{NetworkInterfaceId}` | GET | `redfish/system/networkinterface-instance.lua` |  |  (key `resource_059`) |
| 220 | `/redfish/v1/Systems/{SystemId}/NetworkInterfaces/{NetworkInterfaceId}/NetworkDeviceFunctions` | GET | `redfish/system/networkdevicefunction-collection.lua` |  |  (key `resource_058`) |
| 221 | `/redfish/v1/Systems/{SystemId}/Oem/Ami/BiosTable` | GET? | `oem/ami_hi/bios-table-handler.lua` |  | Written by BIOS over host interface (key `amioem_hi_redfish_extension_bios_table`) |
| 222 | `/redfish/v1/Systems/{SystemId}/Oem/Ami/BiosTableTags` | GET? | `oem/ami_hi/bios-tabletags-handler.lua` |  |  (key `amioem_hi_redfish_extension_bios_table_tags`) |
| 223 | `/redfish/v1/Systems/{SystemId}/Oem/Ami/Inventory/Crc` | GET POST DELETE | `oem/ami/inventory/crc-handler.lua` |  | BIOS<->BMC inventory CRC (GroupCrcList CERTIFICATE/CPU/DIMM/PCIE/SECUREBOOT); used by BIOS over host interface (key `aimoem_redfish_extensions_inventory_crc`) |
| 224 | `/redfish/v1/Systems/{SystemId}/Pldm` | POST GET DELETE | `oem/plugin_hi/pldm-hi.lua` |  | Host-interface: BIOS PLDM/NVRAM blob (entity BiosNVRam) (key `plugin_hi_redfish_bios_nvram`) |
| 225 | `/redfish/v1/Systems/{SystemId}/ProcessorSummary/ProcessorMetrics` | GET | `redfish/system/processor-metrics.lua` |  |  (key `ProcessorSummaryMetrics`) |
| 226 | `/redfish/v1/Systems/{SystemId}/Processors` | GET | `redfish/system/processor-collection.lua` |  |  (key `resource_026`) |
| 227 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}` | GET | `redfish/system/processor-instance.lua` |  |  (key `resource_025`) |
| 228 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/AccelerationFunctions` | GET | `redfish/system/acceleration-function.lua` |  |  (key `resource_707`) |
| 229 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/AccelerationFunctions/{AccelFnId}` | GET | `redfish/system/acceleration-function.lua` |  |  (key `resource_706`) |
| 230 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/Assembly` | GET | `redfish/assembly.lua` |  |  (key `resource_716`) |
| 231 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/OperatingConfigs` | GET | `redfish/system/operatingconfig.lua` |  |  (key `resource_146`) |
| 232 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/OperatingConfigs/{OpConfigId}` | GET | `redfish/system/operatingconfig.lua` |  |  (key `resource_145`) |
| 233 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/ProcessorMetrics` | GET | `redfish/system/processor-metrics.lua` |  |  (key `resource_705`) |
| 234 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/SubProcessors` | GET | `redfish/system/processor-collection.lua` |  |  (key `resource_024`) |
| 235 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/SubProcessors/{SubProcessorId}` | GET | `redfish/system/processor-instance.lua` |  |  (key `resource_023`) |
| 236 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/SubProcessors/{SubProcessorId}/AccelerationFunctions` | GET | `redfish/system/acceleration-function.lua` |  | shares key with the non-SubProcessor pattern (key `resource_707`) |
| 237 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/SubProcessors/{SubProcessorId}/AccelerationFunctions/{AccelFnId}` | GET | `redfish/system/acceleration-function.lua` |  | shares key with the non-SubProcessor pattern (key `resource_706`) |
| 238 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/SubProcessors/{SubProcessorId}/Assembly` | GET | `redfish/assembly.lua` |  |  (key `resource_714`) |
| 239 | `/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/SubProcessors/{SubProcessorId}/ProcessorMetrics` | GET | `redfish/system/processor-metrics.lua` |  |  (key `resource_704`) |
| 240 | `/redfish/v1/Systems/{SystemId}/ResetActionInfo` | GET | `redfish/actioninfo/reset-actioninfo.lua` |  |  (key `resource_055`) |
| 241 | `/redfish/v1/Systems/{SystemId}/SD` | GET PATCH | `redfish/system.lua` |  | Settings object (@Redfish.Settings "SD" = pending settings, applied later) (key `resource_056_SD`) |
| 242 | `/redfish/v1/Systems/{SystemId}/SecureBoot` | GET PATCH | `redfish/system/secureboot.lua` |  |  (key `resource_027`) |
| 243 | `/redfish/v1/Systems/{SystemId}/SecureBoot/Actions/{Action}` | POST | `redfish/system/secureboot-actions.lua` | SecureBoot.ResetKeys: ResetKeysType ResetAllKeysToDefault \| DeleteAllKeys \| DeletePK. |  (key `resource_028`) |
| 244 | `/redfish/v1/Systems/{SystemId}/SecureBoot/ResetKeysActionInfo` | GET | `redfish/actioninfo/resetkeys-actioninfo.lua` |  |  (key `resource_029`) |
| 245 | `/redfish/v1/Systems/{SystemId}/SecureBoot/SecureBootDatabases` | GET | `redfish/system/securebootdatabases.lua` |  |  (key `secureboot_database_collection`) |
| 246 | `/redfish/v1/Systems/{SystemId}/SecureBoot/SecureBootDatabases/{DatabaseId}` | GET | `redfish/system/securebootdatabases.lua` |  |  (key `secureboot_database_instance`) |
| 247 | `/redfish/v1/Systems/{SystemId}/SecureBoot/SecureBootDatabases/{DatabaseId}/Actions/{Action}` | POST | `redfish/system/securebootdatabases-actions.lua` | SecureBootDatabase.ResetKeys (per database). |  (key `secureboot_database_action`) |
| 248 | `/redfish/v1/Systems/{SystemId}/SecureBoot/SecureBootDatabases/{DatabaseId}/Certificates` | GET POST | `redfish/system/securebootdatabases.lua (certificate sub-resources; assumed)` |  |  (key `secureboot_database_certificate_collection`) |
| 249 | `/redfish/v1/Systems/{SystemId}/SecureBoot/SecureBootDatabases/{DatabaseId}/Certificates/Members` | POST | `redfish/system/securebootdatabases.lua (certificate sub-resources; assumed)` |  | POST alias for creating a member of the parent collection (key `secureboot_database_certificate_members`) |
| 250 | `/redfish/v1/Systems/{SystemId}/SecureBoot/SecureBootDatabases/{DatabaseId}/Certificates/{CertificateId}` | GET DELETE | `redfish/system/securebootdatabases.lua (certificate sub-resources; assumed)` |  |  (key `secureboot_database_certificate_instance`) |
| 251 | `/redfish/v1/Systems/{SystemId}/SecureBoot/SecureBootDatabases/{DatabaseId}/Signatures` | GET POST | `redfish/system/securebootdatabases_signature.lua` |  |  (key `secureboot_database_signature_collection`) |
| 252 | `/redfish/v1/Systems/{SystemId}/SecureBoot/SecureBootDatabases/{DatabaseId}/Signatures/Members` | POST | `redfish/system/securebootdatabases_signature.lua` |  | POST alias for creating a member of the parent collection (key `secureboot_database_signature_members`) |
| 253 | `/redfish/v1/Systems/{SystemId}/SecureBoot/SecureBootDatabases/{DatabaseId}/Signatures/{SignatureId}` | GET DELETE | `redfish/system/securebootdatabases_signature.lua` |  |  (key `secureboot_database_signature_instance`) |
| 254 | `/redfish/v1/Systems/{SystemId}/SimpleStorage` | GET | `redfish/system/simple-storage.lua` |  |  (key `resource_038`) |
| 255 | `/redfish/v1/Systems/{SystemId}/SimpleStorage/{SimpleStorageId}` | GET | `redfish/system/simple-storage.lua` |  |  (key `resource_037`) |
| 256 | `/redfish/v1/Systems/{SystemId}/Smbios` | POST DELETE | `oem/plugin_hi/smbios-hi.lua` |  | Host-interface: BIOS pushes SMBIOS tables (key `plugin_hi_redfish_bios_smbios`) |
| 257 | `/redfish/v1/Systems/{SystemId}/Storage` | GET | `redfish/system/storage-collection.lua` |  |  (key `resource_062`) |
| 258 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}` | GET | `redfish/system/storage-instance.lua` |  |  (key `resource_061`) |
| 259 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Assembly` | GET | `redfish/assembly.lua` |  |  (key `resource_712`) |
| 260 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Controllers` | GET | `redfish/system/controllers.lua` |  |  (key `resource_144`) |
| 261 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Controllers/{ControllerId}` | GET | `redfish/system/controllers.lua` |  |  (key `resource_143`) |
| 262 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Controllers/{ControllerId}/Ports` | GET | `redfish/system/port.lua` |  |  (key `resource_181`) |
| 263 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Controllers/{ControllerId}/Ports/{PortId}` | GET | `redfish/system/port.lua` |  |  (key `resource_180`) |
| 264 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Drives/{DriveId}` | GET | `redfish/system/drives.lua` |  |  (key `resource_066`) |
| 265 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Drives/{DriveId}/Assembly` | GET | `redfish/assembly.lua` |  |  (key `resource_711`) |
| 266 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Volumes` | GET POST | `redfish/system/volume-collection.lua` |  |  (key `resource_065`) |
| 267 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Volumes/{VolumeId}` | GET DELETE | `redfish/system/volume-instance.lua` |  |  (key `resource_064`) |
| 268 | `/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Volumes/{VolumeId}/Actions/{Action}` | POST | `redfish/system/volume-actions.lua` | Volume.Initialize. |  (key `resource_063`) |
| 269 | `/redfish/v1/Systems/{SystemId}/USBControllers` | GET | `redfish/system/usb-controllers.lua` |  |  (key `resource_152`) |
| 270 | `/redfish/v1/Systems/{SystemId}/USBControllers/{ControllerId}` | GET | `redfish/system/usb-controllers.lua` |  |  (key `resource_151`) |
| 271 | `/redfish/v1/Systems/{SystemId}/USBControllers/{ControllerId}/Ports` | GET | `redfish/system/port.lua` |  |  (key `resource_154`) |
| 272 | `/redfish/v1/Systems/{SystemId}/USBControllers/{ControllerId}/Ports/{PortId}` | GET | `redfish/system/port.lua` |  |  (key `resource_153`) |
| 273 | `/redfish/v1/TaskService` | GET | `redfish/task-service.lua` |  |  (key `resource_107`) |
| 274 | `/redfish/v1/TaskService/TaskMonitors/{TaskMonitorId}` | GET DELETE | `redfish/task-monitor.lua` |  | GET returns 202 + Location/Retry-After while running, final response when done; DELETE cancels (key `TaskMonitor_Instance`) |
| 275 | `/redfish/v1/TaskService/Tasks` | GET | `redfish/task-service.lua` |  |  (key `resource_106`) |
| 276 | `/redfish/v1/TaskService/Tasks/Members` | GET | `redfish/task-service.lua` |  |  (key `resource_105`) |
| 277 | `/redfish/v1/TaskService/Tasks/{TaskId}` | GET DELETE | `redfish/task-service.lua` |  |  (key `resource_104`) |
| 278 | `/redfish/v1/TelemetryService` | GET PATCH | `redfish/telemetry-service.lua` |  |  (key `telemetry_resource_01`) |
| 279 | `/redfish/v1/TelemetryService/Actions/{Action}` | POST | `redfish/telemetry-service/telemetry-actions.lua` | TelemetryService.SubmitTestMetricReport (MetricReportName, GeneratedMetricReportValues[MetricId, MetricProperty, MetricValue, Timestamp]). |  (key `telemetry_resource_71`) |
| 280 | `/redfish/v1/TelemetryService/LogService` | GET PATCH | `redfish/telemetry-service/log-service.lua` |  |  (key `telemetry_resource_10`) |
| 281 | `/redfish/v1/TelemetryService/LogService/Actions/{Action}` | POST | `redfish/telemetry-service/log-service-actions.lua` | LogService.ClearLog. |  (key `telemetry_resource_14`) |
| 282 | `/redfish/v1/TelemetryService/LogService/Entries` | GET | `redfish/telemetry-service/log-service-entries.lua` |  |  (key `telemetry_resource_12`) |
| 283 | `/redfish/v1/TelemetryService/LogService/Entries/{EntryId}` | GET | `redfish/telemetry-service/log-service-entries.lua` |  |  (key `telemetry_resource_13`) |
| 284 | `/redfish/v1/TelemetryService/MetricDefinitions` | GET | `redfish/telemetry-service/metricdefinitions.lua` |  |  (key `telemetry_resource_02`) |
| 285 | `/redfish/v1/TelemetryService/MetricDefinitions/{MetricDefinitionId}` | GET PATCH | `redfish/telemetry-service/metricdefinitions.lua` |  | Voltage_Reading/Fan_Reading/Temperature_Reading are GET-only; others GET/PATCH (inferred from override table) (key `telemetry_resource_03`) |
| 286 | `/redfish/v1/TelemetryService/MetricReportDefinitions` | GET POST | `redfish/telemetry-service/metricreportdefinitions.lua` |  |  (key `telemetry_resource_04`) |
| 287 | `/redfish/v1/TelemetryService/MetricReportDefinitions/Members` | POST | `redfish/telemetry-service/metricreportdefinitions.lua` |  | POST alias for creating a member of the parent collection (key `metric_report_definition_members`) |
| 288 | `/redfish/v1/TelemetryService/MetricReportDefinitions/{MetricReportDefinitionId}` | GET PATCH DELETE | `redfish/telemetry-service/metricreportdefinitions.lua` |  |  (key `telemetry_resource_05`) |
| 289 | `/redfish/v1/TelemetryService/MetricReports` | GET | `redfish/telemetry-service/metricreports.lua` |  |  (key `telemetry_resource_06`) |
| 290 | `/redfish/v1/TelemetryService/MetricReports/{MetricReportId}` | GET | `redfish/telemetry-service/metricreports.lua` |  |  (key `telemetry_resource_07`) |
| 291 | `/redfish/v1/TelemetryService/SubmitTestMetricReportActionInfo` | GET | `redfish/actioninfo/submittestmetricreport-actioninfo.lua` |  |  (key `telemetry_resource_72`) |
| 292 | `/redfish/v1/TelemetryService/Triggers` | GET POST | `redfish/telemetry-service/triggers.lua` |  |  (key `telemetry_resource_08`) |
| 293 | `/redfish/v1/TelemetryService/Triggers/Members` | POST | `redfish/telemetry-service/triggers.lua` |  | POST alias for creating a member of the parent collection (key `triggers_members`) |
| 294 | `/redfish/v1/TelemetryService/Triggers/{TriggerId}` | GET DELETE | `redfish/telemetry-service/triggers.lua` |  |  (key `telemetry_resource_09`) |
| 295 | `/redfish/v1/UpdateService` | GET PATCH | `redfish/update-service.lua` | Advertises #UpdateService.SimpleUpdate (target /redfish/v1/UpdateService/Actions/SimpleUpdate, @Redfish.ActionInfo SimpleUpdateActionInfo), MultipartHttpPushUri=/redfish/v1/UpdateService/upload, Oem.AMIUpdateService.UploadCABundle | PATCH: Oem.AMIUpdateService.PreserveConfiguration.*; MaxImageSizeBytes from redis (key `updateservice`) |
| 296 | `/redfish/v1/UpdateService/Actions/Oem/UpdateService.UploadCABundle` | POST | `redfish/upload-cabundle.lua` | UpdateService.UploadCABundle: multipart part named ca_bundle (PEM) -> /tmp/redfishfwupdate/redfish_ca_bundle.pem; used to verify HTTPS ImageURI |  (key `updateservice_upload_cabundle_action`) |
| 297 | `/redfish/v1/UpdateService/Actions/SimpleUpdate` | POST | `redfish/update-service.lua` | UpdateService.SimpleUpdate: ImageURI (required), TransferProtocol HTTP\|FTP\|HTTPS (or scheme in ImageURI), User/Username, Password. BMC image only ("This action is used to perform BMC Firmware update"). Returns 202 + Task. |  (key `updateservice_simpleupdate_action`) |
| 298 | `/redfish/v1/UpdateService/FirmwareInventory` | GET | `redfish/update-service/firmware-inventory.lua` |  | Members from redis Redfish:UpdateService:FirmwareInventory:* (key `updateservice_firmware_inventory_collection`) |
| 299 | `/redfish/v1/UpdateService/FirmwareInventory/{InventoryId}` | GET | `redfish/update-service/firmware-inventory.lua` |  | SoftwareInventory v1_2_3: Name, Version, Updateable, Status (BMC, BIOS, CPLD, ME, BP, PSU, FPGA, RETIMER... depending on features) (key `updateservice_firmware_inventory`) |
| 300 | `/redfish/v1/UpdateService/upload` | POST | `redfish/multipart-push.lua` | UpdateService.MultipartPush: multipart/form-data with parts UpdateParameters (JSON: Targets[], @Redfish.OperationApplyTime, Oem.Ami/OemParameters: FlashType, UploadSelector, ImageType, preserveconf) and UpdateFile. Components: BMC (.ima), BIOS, CPLD, BP, FPGA, PSU, RETIMER, PFR, PLDM, HPM bundles (.hpm). Returns 202 + Task. | Feature flags CONFIG_SPX_FEATURE_*_FW gate each component (key `updateservice_multipart_push_action`) |
| 301 | `/redfish/v1/UpdateService/{ActionInfoName}` | GET | `redfish/update-service.lua` |  | SimpleUpdateActionInfo (ImageURI req., TransferProtocol HTTP\|FTP\|HTTPS, User, Password), UploadCABundleActionInfo (key `updateservice_update_action_info`) |
| 302 | `/redfish/v1/bios/defaultSD` | GET? | `oem/ami_bios/load-default-handler.lua` |  | BIOS default settings (Bios entity); methods not in allow table (key `ami_bios_dre_bios_defaultsd`) |
| 303 | `/redfish/v1/odata` | GET | `redfish/odata-service-root.lua` |  | in PRIVILEGE_CHECKING_WHITELIST (key `resource_114`) |
| 304 | `/redfish/v1/odata/` | GET | `redfish/odata-service-root.lua` |  | in PRIVILEGE_CHECKING_WHITELIST (key `resource_113`) |
| 305 | `/redfish/v1/oem/MappingOut` | GET? | `oem/ami_bios/mapstatus-handler.lua` |  | OemMappingOut (key `ami_bios_dre_mappingout`) |
| 306 | `/redfish/v1/oem/ami/inventory/crc` | GET? | `oem/ami_bios/inventory-crc-handler.lua` |  | AMIBiosInventoryCRC (key `ami_bios_dre_inventory_crc`) |
| 307 | `/redfish/v1/openapi.yaml` | GET | `redfish/openapi.lua (static/openapi.yaml.gz)` |  | in PRIVILEGE_CHECKING_WHITELIST (key `resource_openapi`) |
| 308 | `/redfish/v1/{any}.Deep` | POST PATCH | `redfish/deep_operation.lua` |  | Redfish DeepPATCH/DeepPOST; ServiceRoot advertises ProtocolFeaturesSupported.DeepOperations with MaxLevels (key `deep_operation`) |

### 2.1 Action reference (allowable values)

| Action | Target | Parameters / AllowableValues | Source |
|---|---|---|---|
| `ComputerSystem.Reset` | `/redfish/v1/Systems/Self/Actions/ComputerSystem.Reset` | `ResetType`: On, ForceOff, GracefulShutdown, ForceRestart, PowerCycle (redis set `Redfish:Systems:DefaultResetAllowedValues`). Optional `@Redfish.OperationApplyTime`: `Immediate` \| `AtMaintenanceWindowStart` (with `MaintenanceWindowStartTime`, regex `YYYY-MM-DDThh:mm:ss±hh:mm`). The mapping to IPMI chassis control is in `sync-agent/extensions/ipmi/sync-helpers/Chassis.lua` / `libipmi/chassisStatus.lua` (`LIBIPMI_HL_PowerOff`, `SoftOff`, `HardReset`, `PowerCycle`, power-up). It returns errors if the host is already On or Off. | `db_init/systems.rcmd`, `redfish/system-actions.lua` |
| `Chassis.Reset` | `/redfish/v1/Chassis/Self/Actions/Chassis.Reset` | `ResetType`: same set (`Redfish:Chassis:DefaultResetAllowedValues`). `action-info.rcmd` also declares GracefulRestart. | `db_init/chassis.rcmd`, `db_init/action-info.rcmd` |
| `Manager.Reset` | `/redfish/v1/Managers/Self/Actions/Manager.Reset` | `ResetType`: ForceRestart. Done through the task service (`:Actions:Manager:Reset:TaskPrefix`). | `db_init/action-info.rcmd`, `redfish/actioninfo/reset-actioninfo.lua` |
| `VirtualMedia.InsertMedia` | `/redfish/v1/Managers/Self/VirtualMedia/{id}/Actions/VirtualMedia.InsertMedia` | `Image` (required; `.iso`/`.nrg`), `TransferProtocolType` (required: NFS, CIFS, HTTPS), `TransferMethod` (Stream), `Inserted`, `WriteProtected`, `UserName`, `Password`. Needs RMedia enabled (`AMIVirtualMedia.EnableRMedia`). HTTPS and CIFS-encryption support are feature-gated (`VMedia:RMEDIA_HTTPS_SUPPORT`, `RMEDIA_CIFS_ENC_SUPPORT`). Runs as a task. | `db_init/action-info.rcmd`, `redfish/manager/virtual-media-actions.lua` |
| `VirtualMedia.EjectMedia` | `…/Actions/VirtualMedia.EjectMedia` | none | same |
| PATCH VirtualMedia | `/redfish/v1/Managers/Self/VirtualMedia/{id}` | Image, Inserted, TransferMethod, TransferProtocolType, WriteProtected, UserName, Password (writable list in `property-access/VirtualMedia.lua`) | |
| `Bios.ResetBios` | `/redfish/v1/Systems/Self/Bios/Actions/Bios.ResetBios` | none. Writes `BIOS_RESET_PATH` (`/conf/redfish/bios/bios_reset`), applied on the next POST. | `redfish/system/bios-actions.lua` |
| `Bios.ChangePassword` | `/redfish/v1/Systems/Self/Bios/Actions/Bios.ChangePassword` | `PasswordName`, `OldPassword`, `NewPassword` (all required). Stored AES-encrypted in `bios_pass.json`. Fails with 503 `ServiceTemporarilyUnavailableDueToHostBooting` while the host is in POST. | `db_init/action-info.rcmd`, `bios-actions.lua` |
| BIOS settings | PATCH `/redfish/v1/Systems/Self/Bios/SD` | `Attributes{…}` validated against the BIOS attribute registry (`/Registries/BiosAttributeRegistry*`). Pending settings go to `bios_future_settings.json` and are consumed by BIOS through the host interface. | `redfish/system/bios.lua` |
| `SecureBoot.ResetKeys` | `/redfish/v1/Systems/Self/SecureBoot/Actions/SecureBoot.ResetKeys` | `ResetKeysType`: ResetAllKeysToDefault, DeleteAllKeys, DeletePK | `db_init/action-info.rcmd` |
| `LogService.ClearLog` | Managers `SEL`, `EventLog`, `AuditLog`; Systems `BIOS`, `SMI`; Chassis `Logs`; TelemetryService LogService | none (`ClearAll`) | `db_init/action-info.rcmd` |
| `UpdateService.SimpleUpdate` | `/redfish/v1/UpdateService/Actions/SimpleUpdate` | `ImageURI` (required), `TransferProtocol`: HTTP, FTP, HTTPS; `User`/`Username`, `Password` | `redfish/update-service.lua` |
| `UpdateService.MultipartPush` | `/redfish/v1/UpdateService/upload` (`MultipartHttpPushUri`) | see section 7 | `redfish/multipart-push.lua` |
| `UpdateService.UploadCABundle` (Oem) | `/redfish/v1/UpdateService/Actions/Oem/UpdateService.UploadCABundle` | multipart part `ca_bundle` (PEM) | `redfish/upload-cabundle.lua` |
| `EventService.SubmitTestEvent` | `/redfish/v1/EventService/Actions/EventService.SubmitTestEvent` | EventId, EventTimestamp, MessageArgs[], MessageId (required), OriginOfCondition, Severity | `db_init/action-info.rcmd` |
| `EventDestination.ResumeSubscription` | `/redfish/v1/EventService/Subscriptions/{id}/Actions/EventDestination.ResumeSubscription` | none | |
| `TelemetryService.SubmitTestMetricReport` | `/redfish/v1/TelemetryService/Actions/TelemetryService.SubmitTestMetricReport` | MetricReportName, GeneratedMetricReportValues[] (MetricId, MetricProperty, MetricValue, Timestamp) | `db_init/action-info.rcmd` |
| `CertificateService.GenerateCSR` / `.ReplaceCertificate` | `/redfish/v1/CertificateService/Actions/…` | standard DMTF parameters; certificate locations are HTTPS (`/Managers/Self/NetworkProtocol/HTTPS/Certificates`), LDAP, Accounts, Boot, Managers | `redfish/certificate-service/*.lua`, `config.lua` |
| `Certificate.Rekey` / `Certificate.Renew` | `…/Certificates/{id}/Actions/…` | | `db_init/certificate-service.rcmd` |
| `Volume.Initialize` | `/Systems/Self/Storage/{id}/Volumes/{id}/Actions/Volume.Initialize` | | |
| `NetworkAdapter.ResetSettingsToDefault` | `/Chassis/Self/NetworkAdapters/{id}/Actions/…` | | |
| `AmiBios.ChangeState` (Oem) | `/Systems/Self/Memory/{id}/Actions/AmiBios.ChangeState`, `/Chassis/Self/PCIeDevices/{d}/PCIeFunctions/{f}/Actions/AmiBios.ChangeState` | Enabled \| Disabled (DIMM or PCIe function map-out at next boot) | `db_init/action-info.rcmd`, `oem/ami_bios/*` |
| `AMIManager.BackupConfig` / `RestoreConfig` / `RedfishPowerSaveMode` (Oem) | `/Managers/Self/Actions/Oem/AMIManager.*` | `BackupFeatures`; multipart `conf_file`; `PowerSaveMode` | `schemas/AMIManager.v1_0_0.json` |
| `AMIVirtualMedia.EnableRMedia` / `ConfigureCDInstance` (Oem) | `/Managers/Self/Actions/Oem/AMIVirtualMedia.*` | `RMediaState`; `CDInstance` | same |
| `AMIManager.RedfishDBReset` (Oem) | advertised in `oem/ami/resource_metadata/ManagersInstance.lua`. The handler `oem/ami/redfish_db_reset_task.lua` and `factory-reset.lua` exist, but **no URI pattern is registered in 9.05**. | | |

---

## 3. Authentication and authorization

Sources: `redfish-handler.lua`, `redfish/default-handler.lua`, `redfish/session-service.lua`,
`redfish/resource_metadata/SessionService.lua`, `redfish/account-service/account.lua`, `pam_ffi.lua`,
`config.lua`, `db_init/zzzaccountservice.rcmd`, `db_init/sessionservice.rcmd`,
`privilege_registry/*.json`.

### 3.1 Authentication modes

`get_auth_mode`/`set_auth_mode` distinguish five modes:

| Mode | Source |
|---|---|
| `BASIC_AUTH` | `Authorization: Basic …` header. Realm `Basic realm="AMI_Redfish_Server"`. |
| `SESSION_AUTH` | `X-Auth-Token`. Realm `X-Auth-Token realm="AMI_Redfish_Server"`. |
| `CERT_AUTH` | client certificate (3.5) |
| `PAM_AUTH` | external accounts (LDAP, AD, RADIUS, IPMI) via `redfish_pamauthd` |
| `NoAuth` | allowed only where the privilege registry lists `NoAuth`, plus host-interface no-auth |

`NoAuth` resources:

- `ServiceRoot` GET (`Login | NoAuth` in the registry)
- the `/redfish` version document
- `POST /redfish/v1/SessionService/Sessions` (login)

Unauthenticated requests get 401 with `WWW-Authenticate` (`check_basic_authorization`,
`check_session_authorization`).

Local accounts are checked against `Redfish:AccountService:Accounts:<n>:Password` (SHA-512 with salt) or
`PBKDF2Password` (`isPBKDF2HashEnabled`; `config.SUPPORTED_HASHING_ALGORITHM` is `{MD5, SHA512, PBKDF2}`).
`LocalAccountAuth` defaults to `LocalFirst`; it controls the order of local and PAM checks.
`AccountService.Oem.Ami.Configurations` sets `PAMEnabled` and `PAMOrder` (IPMI, LDAP, ACTIVE DIRECTORY,
RADIUS).

`PasswordChangeRequired=true` (the default for the seeded `Administrator`, `Accounts/1`) restricts the
account. Everything except a PATCH of its own password returns `PasswordChangeRequired`
(`error_password_change_required`).

### 3.2 Sessions

- **Create.** `POST /redfish/v1/SessionService/Sessions` with `{"UserName","Password"}`. The response is
  201 with `X-Auth-Token` and `Location: /redfish/v1/SessionService/Sessions/<id>`. The token is stored at
  `Redfish:SessionService:Sessions:<id>:Token`, with `:UserName` and `:RoleId`. Sessions are also mirrored
  into the SPX web session store (`SESSION_PATH=/tmp/`, `SALT=spx`).
- **Delete.** `DELETE /redfish/v1/SessionService/Sessions/<id>` needs `ConfigureManager`, or
  `ConfigureSelf` for your own session.
- **Timeout.** `SessionService.SessionTimeout` is seeded to **30** (`db_init/sessionservice.rcmd`; per the
  Redfish schema this is seconds). It's implemented as the redis TTL on the session keys, refreshed on each
  request (`expire`/`ttl` in `redfish-handler.lua`). PATCHing `SessionTimeout` re-applies `expire` to every
  existing session. The PATCH is range-checked against `config.SESSION_TIME_LOWER_LIMIT` and
  `SESSION_TIME_UPPER_LIMIT`; values outside return `PropertyValueOutOfRange`. The numeric bounds aren't in
  the strings dump; the DMTF range is 30–86400.
- **Host-interface sessions.** Sessions created over the host interface (`HostAutoFW` and `HostAutoOS`
  accounts, roles `HI_Session_FW` and `HI_Session_OS`) **do not expire** ("SessionRequest Through HI, This
  session will not expire").
- **Max sessions.** `config.MAX_SESSIONS` caps sessions (value not recoverable from strings). Hitting it
  returns `SessionLimitExceeded`.
- **Polling.** `SESSION_TIMEOUT_POLL_INTERVAL` exists for legacy polling.

### 3.3 Privileges and roles

- **Registry.** `config.PRIVILEGE_REGISTRY_JSON_FILE = Redfish_1.3.0_PrivilegeRegistry.json` (195
  mappings, `#PrivilegeRegistry.v1_1_4`). `Redfish_1.0.3_PrivilegeRegistry.json` (159 mappings, with OEM
  privilege `ConfigureHostInterface`) also ships and is exposed under `/Registries`.
- **Lookup.** Privileges are resolved per *entity chain* from `redfish-resource-entities.lua`. They are
  checked per operation, per property (`PropertyOverrides`, `split_properties_by_privilege`,
  `InsufficientPrivilegeForProperty`), per subordinate and per resource URI override.
- **Whitelist.** `config.PRIVILEGE_CHECKING_WHITELIST` skips the privilege check for these keys:

  | Key | URI |
  |---|---|
  | `resource_002` | trailing-slash redirect `^/redfish(.*)/+$` |
  | `resource_openapi` | `/redfish/v1/openapi.yaml` |
  | `resource_113` | `/redfish/v1/odata/` |
  | `resource_114` | `/redfish/v1/odata` |
  | `resource_115` | `/redfish/v1/$metadata` |
  | `deep_operation` | DeepPOST/DeepPATCH |
  | `resource_126` | `/redfish/v1/GlobalLuaTables` (**a debug/cache endpoint with GET and DELETE**) |
  | `resource_127` | `/redfish` (version document) |

  Authentication still appears to apply to these; only the privilege lookup is skipped.

Predefined roles (`db_init/zzzaccountservice.rcmd`):

| RoleId | AssignedPrivileges | Notes |
|---|---|---|
| Administrator | Login, ConfigureManager, ConfigureUsers, ConfigureSelf, ConfigureComponents | |
| Operator | Login, ConfigureSelf, ConfigureComponents | |
| ReadOnly | Login, ConfigureSelf | |
| HostInterfaceAdministrator | all five + OEM `ConfigureHostInterface` | Used for host-interface auth (`AuthNoneRoleId`, `FirmwareAuthRoleId` = Administrator in `db_init/host_interface.rcmd`) |

Predefined roles are GET-only. Custom roles can be created with `POST /AccountService/Roles` (up to
`MAX_ROLES`) and support GET, PATCH and DELETE. `RestrictedRole` and `RestrictedPrivileges` exist.

Representative mappings from the 1.3.0 registry:

| Entity | GET | PATCH/POST/DELETE |
|---|---|---|
| ComputerSystem, Bios, UpdateService | Login | ConfigureComponents |
| Manager, EventService, VirtualMedia, SessionService | Login | ConfigureManager |
| ManagerAccount | ConfigureManager, ConfigureUsers or ConfigureSelf (property and self overrides) | ConfigureUsers |
| SessionCollection | Login | POST: Login (login itself is handled as NoAuth) |
| Session | Login | DELETE: ConfigureManager or ConfigureSelf |
| ServiceRoot | Login or NoAuth | ConfigureManager |

IPMI channel privileges are mapped to Redfish roles through
`sync-agent/extensions/ipmi/user_privilege_map.lua` and the `usrpriv_t` struct in `pam_ffi.lua`
(lan, serial, lan1 and lan2 privileges, plus `Extendedprivilege` for KVM and VMedia).

Accounts:

- **Limits.** Up to `MAX_ACCOUNTS`, with UIDs from `REDFISH_FIRST_UID`/`IPMI_FIRST_UID`. Redfish users are
  unified with IPMI users (`Redfish:UNIFIED_USER_ACCOUNT`, `PATCH:Redfish:AccountService:IPMI:AccountInfo`).
- **Usernames and passwords.** `MaxUserNameLength` 16. `MinPasswordLength` 8 and `MaxPasswordLength` 20
  (`user_auth_info` has `username[17]`, `password[21]`). `StrongPassword` is enforced when enabled.
- **AccountTypes.** Redfish, SNMP (with `Oem.Ami.SNMP` AuthenticationProtocol SHA/SHA256/…,
  EncryptionProtocol DES/AES, Access ReadOnly/ReadWrite), IPMI and others.
- **Last administrator.** `is_last_available_administrator_account` / `DEFAULT_LOCK_LAST_ADMINISTRATOR`
  stop the last admin from being deleted, disabled or locked.

### 3.4 Account lockout

Defaults from `db_init/zzzaccountservice.rcmd`; each is PATCHable on `/AccountService` up to the
`config.DEFAULT_*_MAX_LIMIT` caps:

| Property | Default |
|---|---|
| `AccountLockoutThreshold` | 5 failed attempts |
| `AccountLockoutDuration` | 30 s |
| `AccountLockoutCounterResetAfter` | 30 s (and `AccountLockoutCounterResetEnabled`) |
| `AuthFailureLoggingThreshold` | 3 |

Failures are counted in `Accounts:<n>:FailedLoginCount` and `:AuthFailureLogCount`, then the account is
set to `Locked`. The handler strings also show **IP-based lockout** ("…exceeded the set threshold. IP has
been locked."). The last Administrator is never locked ("Because the account is the last Administrator,
the account won't be locked."). Failed logins add audit-log entries and `Security` registry messages
(`LoginFailure`, `AccessDenied`).

### 3.5 mTLS / client-certificate mapping

lighttpd on :443 requests a client certificate validated against `/conf/ca.pem` (`ssl.verifyclient.enforce
= "disable"`, so it's optional). The subject CN is exported as the username (`SSL_CLIENT_S_DN_CN`), and the
certificate as PEM.

The backend `check_client_cert` / `check_client_certificate_authorization` in `redfish-handler.lua` and
`default-handler.lua` does the following:

1. Extracts `CN=([a-zA-Z0-9_]+)` from the forwarded certificate (`X-Username` and the
   `-----BEGIN CERTIFICATE-----` header data).
2. Verifies it in-process with OpenSSL FFI (`X509_STORE_add_cert`, `X509_verify_cert` against
   `CA_CERT_PATH=/conf/ca.pem`).
3. Maps the CN to a local account `Redfish:AccountService:Accounts:*:UserName`.
4. Compares the presented certificate with the account's `Accounts/<n>/Certificates/<id>`
   (`:CertificateString`; certificate files in `/conf/redfish_cert/accounts`, up to
   `MAX_ACCOUNTS_CERTIFICATE_COUNT` per account).

On success the request runs as that account with mode `CERT_AUTH`.

> **Proxy note.** A Go proxy that terminates TLS loses this unless it re-originates TLS with the client's
> certificate, which is impossible, or the backend is reached directly on 9080. Port 9080 binds to
> 127.0.0.1 only, so in practice certificate-to-account mapping is a BMC-edge feature. Treat it as
> optional and unsupported through a TLS-terminating proxy.

### 3.6 Host interface

`extensions/host-interface/*` and `Managers/Self/HostInterfaces/{id}`:

- **Network.** USB NIC `usb0`; BMC side `DEFAULT_HI_USB0_ADDRESS` (from
  `CONFIG_SPX_FEATURE_USB0_IP_ADDRESS`, typically 169.254.0.17), host side `169.254.0.18`
  (`DEFAULT_SYSTEMS_HI_ETH_ADDR`).
- **Auth.** `FirmwareAuthEnabled`/`KernelAuthEnabled` and `AuthNoneRoleId` (default Administrator) let BIOS
  and the OS talk without credentials, or with the auto accounts `HostAutoFW`/`HostAutoOS`.
- **Restrictions.** `is_host_interface_connection` changes behaviour in several handlers: the Bios PATCH
  path, non-expiring sessions, and making `usb0` GET-only.

---

## 4. OEM extensions

There is **no ASRock Rack (`AsrockRack`/`ASRR`) Oem namespace** in the Redfish service. `grep -i asrock`
over all strings, schemas and `static/oem_metadata` finds nothing. The only ASRock Rack customisation
visible is the `oem/plugin` package:

- `/Managers/Self/IKVM`, `/Systems/Self/FruInfo` (and `/Managers/Self/KCS` in 7.02 only)
- `oem/plugin_hi`: `/Systems/Self/Smbios`, `/Systems/Self/Pldm`

`lighttpd.conf` also carries `#ASRR:` comments. All other OEM content is AMI's.

### 4.1 `Oem.Ami` on standard resources

`oem/ami/query/*`, `oem/ami/resource_metadata/*`, `oem/ami/property-type/*`, `oem/ami/patch/*`:

| Resource | `Oem.Ami` properties (`@odata.type`) |
|---|---|
| ServiceRoot (`#AMIServiceRoot.v1_0_0.AMIServiceRoot`) | `Configurations` → `/redfish/v1/Oem/Ami/Configurations`; `RtpVersion` (redis `Redfish:Ami:RtpVersion`, seeded 13.05); `InventoryData` → `/redfish/v1/Oem/Ami/InventoryData` and `InventoryDataStatus` → `/Oem/Ami/InventoryData/Status` (only on host-interface connections); `CupsService` (only if `Redfish:IntelCUPS:FeatureSupport`). The schema also lists `AutonomousCrashDump`. |
| Manager (`#AMIManager.v1_0_0.AMIManager`) | `ManagerServiceInfo.CommandShellServiceInfo.{SSH,IPMI}.MaxConcurrentSessions` (`:Oem:SSH_MaxAllowSession`, `:Oem:IPMI_MaxAllowSession`); `ManagerServiceInfo.Links.NetworkProtocol`; `PowerSaveMode` (Enable/Disable); `VirtualMedia.CDInstances` and `VirtualMedia.RMediaStatus` (Enabled/Disabled); `NodeManager` link (if `Redfish:IntelNM:FeatureSupport`). `Actions.Oem`: `#AMIManager.RedfishPowerSaveMode`, `#AMIVirtualMedia.EnableRMedia`, `#AMIVirtualMedia.ConfigureCDInstance`, `#AMIManager.BackupConfig`, `#AMIManager.RestoreConfig`, `#AMIManager.RedfishDBReset`, each with `@Redfish.ActionInfo`. The `oem/plugin` metadata adds a top-level `IKVM` link. |
| ComputerSystem (`AMIComputerSystem` / `AMIBIOSInventoryCRC`) | `Oem.Ami.Bios.{RedfishVersion,RtpVersion}` (from `Redfish:oem:ami:bios:*`); `Oem.Ami.Inventory.Crc.GroupCrcList` (CERTIFICATE, CPU, DIMM, PCIE, SECUREBOOT) → `/Systems/Self/Oem/Ami/Inventory/Crc`; `Oem.Ami.ManagerBootConfiguration.ManagerBootMode` (`None`, `SoftReset`, `ResetTimeout`; with `@Redfish.AllowableValues`); `BiosTable` and `BiosTableTags` links |
| Memory | `Oem.AmiBios` (`#AmiBios.v1_0_0.AmiBios`) with action `#AmiBios.ChangeState` |
| VirtualMedia (`#AMIVirtualMedia.v1_0_0`) | `BoostMode`, `RedirectionStatus` (Redirection Started, Stopped – Connection Lost, Image File Not Found, Login Failed, MAX Session Reached, and others; see `oem/ami/query/manager/manager-virtualmedia-instance.lua`) |
| EthernetInterface (Managers) (`#AMIDNSConfiguration.v1_0_0`) | `Oem.Ami.DNSEnabled`, `Oem.Ami.HostNameSetting` (Auto/Manual); `AMIIPV6.StaticIPAddressIndex` |
| AccountService (`#AMIAccountService.v1_0_0`) | `LDAP.Authentication.Oem.Ami.{CommonNameType, EncryptionType (NoEncryption/SSL/StartTLS)}`; `ActiveDirectory.Authentication.Oem.Ami.{DomainName, DomainControllerServerAddr1..3}`; `RemoteRoleMapping[].Oem.Ami.{GroupID, KVMAccess, VMediaAccess, RoleID}`; `Oem.Ami.Configuration` → `/AccountService/Oem/Ami/Configurations`; LDAP `Certificates.Oem.Ami.ClientCertificates`; `CACertificateId`/`CertificateId` |
| ManagerAccount (`#AMIManagerAccount.v1_0_0`) | `Oem.Ami.SNMP.{Access, AuthenticationProtocol, EncryptionProtocol}` |
| ExternalAccountProvider RADIUS (`#AMIRADIUS.v1_0_0`) | `ServiceAddress`, `ServicePort`, `Secret`, `ExtendedPrivilege.{KVMAccess,VMediaAccess}`, `AdvancedRADIUSSetting` link (`RADIUSAuthorization.{Administrator,Operator,User,Oem,NoAccess}`) |
| EventService (`#AMIEventService.v1_0_0`) | `Oem.Ami.SecondarySMTP.{ServiceEnabled, Authentication (None/Plain), ConnectionProtocol (None/StartTLS/TLS_SSL), FromAddress, ServerAddress, Port, Username, Password}`; `Oem.Ami.Certificates` → `/EventService/Oem/Ami/SMTP/Certificates`. EventDestination: `Oem.Ami.SNMP` (trap destination, SNMPv1/v2c/v3) |
| UpdateService (`#AMIUpdateService.v1_0_0`, `#AMIBMC…`) | `Oem.AMIUpdateService.{UpdateStatus, UpdateTarget, FlashPercentage (from /tmp/flasher.redfishprogress or /tmp/redfish.hpmprogress), PreserveConfiguration.{SDR,FRU,SEL,IPMI,Network,NTP,SNMP,SSH,KVM,Authentication,Syslog,CMX,WEB,EXTLOG,REDFISH,AUTOMATION_ENGINE}}` (PATCHable); `Oem.BMC.DualImageConfigurations.{ActiveImage, BootImage, FirmwareImage1Name/Version, FirmwareImage2Name/Version}`; `Actions.Oem.#UpdateService.UploadCABundle` |
| Triggers (`#AMITriggers.v1_0_0`) | `Oem.Ami.ControlTriggerAlerts` |
| Chassis Power/Thermal (`#AMIChassisPowerThermal.v1_0_0`) | `OwnerLUN` |

### 4.2 AMI-specific resources

Full list in section 2 (OEM rows):

- `/redfish/v1/Oem/Ami/Configurations` (`AMIConfigurations`: `CertificateAuthorityUrl`)
- `/redfish/v1/AccountService/Oem/Ami/Configurations` (`AMIAccountServiceConfigurations`: `PAMEnabled`,
  `PAMOrder[]`)
- `/redfish/v1/Oem/Ami/InventoryData[/Status]`, `/redfish/v1/Oem/Ami/BiosStaticFiles[/{id}|/PostStatus/{id}]`
  (BIOS ↔ BMC inventory and static-file exchange over the host interface)
- `/redfish/v1/Systems/Self/Oem/Ami/{BiosTable,BiosTableTags,Inventory/Crc}`
- `/redfish/v1/bios/defaultSD`, `/redfish/v1/oem/MappingOut`, `/redfish/v1/oem/ami/inventory/crc`,
  `/redfish/v1/Chassis/Self/Oem/Ami/MapoutPorting` (the `ami_bios` dynamic-extension package)
- `/redfish/v1/EventService/Oem/Ami/SMTP/Certificates[/{1,2}]`
- `/redfish/v1/AccountService/ExternalAccountProviders/RADIUS/Oem/Ami/AdvancedRADIUSSetting`
- `/redfish/v1/AccountService/LDAP/Certificates/Oem/Ami/ClientCertificates[/{id}]`
- Manager OEM actions and ActionInfos under `/Managers/Self/Actions/Oem/*` and `/Managers/Self/Oem/*ActionInfo`
- the `/redfish/v1/odata` service document, which lists `DynamicExtension` and `configurations` entries
  for which **no routes exist** in 9.05. They're leftovers from the older DRE design still present on X470.

### 4.3 AMI message registries

`message_registries/`:

| Registry | Notes |
|---|---|
| `Base.1.12.0` | |
| `Ami.1.0.0` | AMI-specific Base-like messages, for example `PreconditionHeaderMissing`, `SubmitTestEventPreconditionsFailed`, `ServiceTemporarilyUnavailableDueToHostBooting`, `PAMOrderRadiusNotAllowed` |
| `AmiOem.1.0.0` | firmware update, certificates and SNMP messages |
| `CertificateService.1.0.0` | |
| `EventLog.1.0.0` | |
| `HttpStatus.1.0.0` | |
| `IPMI.1.0.0` | |
| `RedfishDeviceEnablement.1.0.0` | PLDM RDE |
| `Security.1.0.0` | |
| `StorageDevice.1.0.0` | |
| `SyncAgent.1.0.0` | |
| `Task.1.0.0` | |
| `TaskEvent.1.0.1` | |
| `UpdateService.1.0.0` | |

They're served as `/redfish/v1/Registries/{name}` plus `/Registries/Static/{file}`, together with the
BIOS `BiosAttributeRegistry*` files from `/conf/redfish/bios` and the privilege registry.

---

## 5. Schema versions

### 5.1 RedfishVersion

`ServiceRoot.RedfishVersion` comes from `config.REDFISH_VERSION`, built from `MAJOR`/`MINOR`/`ERRATA`.
`SERVICE_PREFIX` is `/redfish/v1` (MAJOR = 1). The minor and errata numbers are numeric constants that the
strings dump can't recover (a `"15"` string sits next to them in `config.lua`, but that's not conclusive).

The DMTF schema versions referenced suggest a **2021–2022 bundle (roughly Redfish 1.11–1.15)**: ServiceRoot
v1_7_0, ComputerSystem v1_15_0, Chassis v1_15_0, Drive v1_13_0, UpdateService v1_6_0, Processor v1_10_0,
Base registry 1.12.0. **Confirm on a live board with `GET /redfish/v1` → `RedfishVersion`.**

`ServiceRoot.Vendor` is `AMI` (`default_constants.SERVICEROOT_VENDOR`) and `Product` is
"AMI Redfish Server". `ProtocolFeaturesSupported` (from `config.SUPPORTED_ODATA_QUERY_PARAMETERS`) has:

- `ExpandQuery.{ExpandAll, Levels, MaxLevels, Links, NoLinks}`
- `SelectQuery`, `FilterQuery`, `OnlyMemberQuery`, `ExcerptQuery`
- `DeepOperations.{DeepPATCH, DeepPOST, MaxLevels}` when enabled

### 5.2 DMTF schema versions emitted

From `@odata.type` and `Link: rel=describedby` in `default_constants.lua` and
`extensions/constants/update_service_constants.lua`:

| Group | Versions |
|---|---|
| Service | ServiceRoot 1.7.0, SessionService 1.1.6, Session 1.2.1, AccountService 1.8.0, ManagerAccount 1.5.0, Role 1.3.0, ExternalAccountProvider 1.3.1 (also 1.1.2), TaskService 1.1.4, Task 1.4.2, EventService 1.4.0, EventDestination 1.7.0, JsonSchemaFile 1.1.4, MessageRegistryFile 1.1.3, MessageRegistry 1.3.1, PrivilegeRegistry 1.1.4, Message 1.0.8, Settings 1.2.2, CollectionCapabilities 1.2.0, ActionInfo 1.1.2 |
| Systems | ComputerSystem 1.15.0, Processor 1.10.0, ProcessorMetrics 1.0.2, AccelerationFunction 1.0.2, Memory 1.8.0, MemoryMetrics 1.3.0, MemoryDomain 1.2.3, MemoryChunks 1.4.0, Bios 1.1.0, SecureBoot 1.1.0, SecureBootDatabase 1.0.0, Signature 1.0.1, BootOption 1.0.3, Storage 1.7.1, StorageController 1.0.0, Drive 1.13.0, Volume 1.4.1, SimpleStorage 1.3.0, NetworkInterface 1.2.0, EthernetInterface 1.6.2, VLanNetworkInterface 1.2.0, USBController 1.0.0, GraphicsController 1.0.0, Port 1.5.0 |
| Chassis | Chassis 1.15.0, Power 1.5.4, Thermal 1.5.3, PowerSubsystem 1.1.0, PowerSupply, PowerSupplyMetrics, ThermalSubsystem 1.0.0, ThermalMetrics 1.0.1, Fan 1.1.1, Sensor 1.0.2, PCIeDevice 1.4.0, PCIeFunction 1.2.3, PCIeSlots 1.1.1, NetworkAdapter 1.8.0, NetworkDeviceFunction 1.5.0, NetworkPort 1.2.3, MediaController 1.1.0, Assembly 1.2.2 |
| Managers | Manager 1.5.0, ManagerNetworkProtocol 1.6.0, SerialInterface 1.1.5, VirtualMedia 1.3.2, LogService 1.1.3, LogEntry 1.4.3, HostInterface |
| Other | UpdateService 1.6.0, SoftwareInventory 1.2.3, CertificateService 1.0.2, Certificate 1.2.1, CertificateLocations 1.0.2, TelemetryService, MetricDefinition 1.0.3, MetricReportDefinition 1.3.0, MetricReport 1.2.0, Triggers 1.1.1, CompositionService 1.1.2, ResourceBlock 1.3.2, Zone 1.3.1, Endpoint 1.3.2 |

### 5.3 `schemas/` directory (OEM schemas served under `/redfish/v1/JsonSchemas`)

There are 61 files: JSON plus CSDL `_v1.xml`. All are AMI OEM; no DMTF standard schemas are bundled
locally (links point to `http://redfish.dmtf.org/schemas/v1/`).

<details><summary>file list</summary>

```
AMIAccountService.v1_0_0.json  AMIAccountService_v1.xml  AMIAccountServiceConfigurations.v1_0_0.json
AMIAccountServiceConfigurations_v1.xml  AMIAdvanceRADIUSSetting.v1_0_0.json  AMIAdvanceRADIUSSetting_v1.xml
AMIAutonomousCrashDump_v1.xml  AMIBIOSInventoryCRC.v1_0_0.json  AMIBIOSInventoryCRC_v1.xml
AMIBiosTable.v1_0_0.json  AMIBiosTable_v1.xml  AMIBiosTableTags.v1_0_0.json  AMIBiosTableTags_v1.xml
AMICaCert.v1_0_0.json  AMICaCert_v1.xml  AMICertificateCollection.v1_0_0.json  AMICertificateCollection_v1.xml
AMIChassisPowerThermal.v1_0_0.json  AMIChassisPowerThermal_v1.xml  AMIConfigurations.v1_0_0.json
AMIConfigurations_v1.xml  AMIDNSConfiguration.v1_0_0.json  AMIDNSConfiguration_v1.xml  AMIEventService.json
AMIEventService.v1_0_0.json  AMIEventService_v1.xml  AMIExternalAccountProvider.v1_0_0.json
AMIExternalAccountProvider_v1.xml  AMIIPV6.v1_0_0.json  AMIIPV6_v1.xml  AMIInventoryStatus.v1_0_0.json
AMIInventoryStatus_v1.xml  AMIManager.v1_0_0.json  AMIManager_v1.xml  AMIManagerAccount.v1_0_0.json
AMIManagerAccount_v1.xml  AMIRADIUS.v1_0_0.json  AMISMTP.v1_0_0.json  AMISMTP_v1.xml  AMISMTPCertificate.json
AMISMTPCertificate.v1_0_0.json  AMISMTPCertificate_v1.xml  AMISMTPCertificateCollection.json
AMISMTPCertificateCollection.v1_0_0.json  AMISMTPCertificateCollection_v1.xml  AMIServiceRoot.v1_0_0.json
AMIServiceRoot_v1.xml  AMITriggers.v1_0_0.json  AMITriggers_v1.xml  AMIUpdateService.v1_0_0.json
AMIUpdateService_v1.xml  AMIVirtualMedia.v1_0_0.json  AMIVirtualMedia_v1.xml  AmiBios.v1_0_0.json
AmiBios_v1.xml  BiosStaticFile.v1_0_0.json  BiosStaticFileCollection.json  ChangePassword.v1_0_0.json
InventoryCRC.v1_0_0.json  InventoryCRC_v1.xml  ResetBios.v1_0_0.json
```
</details>

`static/metadata` (93 KB CSDL) is served at `/$metadata`, merged with `static/oem_metadata`. A final copy
is cached at `/conf/oem_final_metadata`.

`static/openapi.yaml.gz` is served at `/redfish/v1/openapi.yaml`. It's a **generic 2019.2 DMTF/AMI OpenAPI
document** (395 paths) that is byte-identical in 7.02 and 9.05. It does **not** reflect the routes
actually registered, so don't use it as an inventory.

### 5.4 `db_init/` seed files

`ComposabilityService`, `action-info`, `actioninfo_amioem`, `bios_dre`, `certificate-service`, `chassis`,
`default-services`, `eventservice`, `eventservice-certificate`, `eventservice_secondarysmtp`, `general`,
`host_interface`, `managers`, `sessionservice`, `systems`, `taskservice`, `telemetryservice`,
`update_service_AmiOem`, `update_service_serviceroot`, `zzzaccountservice` (`.rcmd`; 625 lines in total).

---

## 6. Quirks and limitations visible in the code

### 6.1 ETags and preconditions

- **ETags on responses.** Responses carry `@odata.etag` and an `ETag` header (`set_etag`; 88 handler files
  reference it). Values derive from `LastModified` timestamps and content. "Error setting ETag. Make sure
  you did a set_scope" shows the ETag depends on the handler's scope.
- **If-Match on PATCH.** `validate_precondition_headers` with `If-Match`/`If-None-Match` handling;
  mismatches return `412 PreconditionFailed`. The `Ami` registry message `PreconditionHeaderMissing` says
  "the server requires the request to be conditional for PUT/PATCH". About 30 PATCH handlers call
  `validate_precondition_headers`, including:
  - `system.lua`, `system/bios.lua`, `bootoption.lua`, `secureboot.lua`
  - `account-service.lua`, `account.lua`, `role.lua`
  - `manager/ethernet-interface.lua`, `serial-interface.lua`, `virtual-media.lua`, `log-service.lua`
  - `chassis/power.lua`, `thermal-instance.lua`, `networkdevicefunction-instance.lua`
  - `event-service.lua`, `compositionservice.lua`, `endpoint-instance.lua`
  - telemetry
  - `oem/ami/config-handler.lua`, `accountservice-config-handler.lua`, `radius_setting.lua`,
    `advanced_radius_setting.lua`
  - `hostinterface-instance.lua`

  **A Go proxy should forward `If-Match` and be ready for 428/412.** Whether a missing header is rejected
  unconditionally needs live verification.
- **Conditional GET.** `If-None-Match` on GET/HEAD (written `IF-None-Match` in one place) gives a 304.
  `If_Modified` handling exists.
- **Caching headers.** `Cache-Control: no-cache, must-revalidate`; `OData-Version: 4.0` (the request header
  is validated: "OData-Version == 4.0", otherwise a header error).

### 6.2 Query parameters ($expand, $select, $filter, …)

The supported set (`config.SUPPORTED_ODATA_QUERY_PARAMETERS`, `odata-handler.lua`) is:

- `$expand` with `*`, `.`, `~` and `$levels=N`, up to `MAX_LEVELS`. Duplicate expansion is guarded by
  `Redfish:OData:ExpandDuplicateData`.
- `$select`
- `$filter` with `eq ne gt ge lt le and or not` and parentheses (`evaluate_filter_expression`)
- `$top`, `$skip`
- `only`, `excerpt` (excerpt only on `/Chassis/{id}/Sensors/{id}`, per
  `config.ODATA_EXCERPT_SUPPORT_URI_PATTERN_LIST`)
- `$count`/`$inlinecount`, `$orderby` and `$format` are recognised but rejected (`QueryNotSupported`)

Errors map to `QueryParameterValueTypeError`, `QueryParameterOutOfRange`, `QueryCombinationInvalid`,
`QueryNotSupportedOnResource` and `QueryNotSupportedOnOperation`. Query parameters on PATCH, POST and
DELETE are rejected.

### 6.3 Pagination

Collections are capped at `config.DEFAULT_COLLECTION_LIMIT` members (`get_limited_collection`). Beyond
that the response includes `Members@odata.nextLink` with `$skip`, along with `Members@odata.count`
(`handle_pagination` in `odata-handler.lua`; `ChassisCollection` metadata also sets it). Expect it on
LogEntries (SEL, EventLog and AuditLog hold up to 150 records each by default), Tasks and Sensors.

### 6.4 Request and response size

| Limit | Source | Error |
|---|---|---|
| Request body | `config.MAX_REQUESTBODY_LENGTH` | `RequestBodySizeExceeded` / `PayloadTooLarge` (413) |
| Response body | `config.RESPONSE_SIZE_LIMIT` | `InsufficientStorageForResponse` (507) |
| Header size | `max_header_size` | |
| Multipart streaming chunk | `streaming_multipart_bytes` | |
| Multipart parse errors | `redfish/multipart-handler.lua` | `MultipartBoundaryRequired`, `MultipartPartHeaderTooLong`, `MultipartWritingFileFailed`, `RequestBodyImcomplete` |
| Strings | property-size tables (`redfish/property-size/*.lua`; `config.DefaultAllowableStringSize`) | |
| Firmware image | `UpdateService.MaxImageSizeBytes` from redis | |

Request validation also rejects:

- non-ASCII characters (`NonASCIICharacterDetected`)
- empty bodies or objects (`EmptyJSON`, `EmptyObjectOrArrayInRequest`)
- wrong content types (`UnsupportedMediaType`; `set_allowed_content_types` per handler: JSON, or
  `multipart/form-data` for upload)
- `@odata.*` annotations in request bodies (`handle_odata_annotation_from_request_body`)
- infinite or NaN numbers

### 6.5 Rate limiting and concurrency

There's no application-level rate limiter; only lighttpd `mod_evasive` with 10 connections per IP on
`/redfish` (30 globally). The Lua server is single-threaded (Turbo ioloop): long synchronous waits on the
sync-agent (`PATCH_TIMEOUT`, `PostTimeout`) block other requests. Redis transactions return "DB is busy"
when contention occurs. Mutually exclusive operations return:

- `ServiceTemporarilyUnavailableDueToMutex`
- `FWUpdateInProgress` (firmware flashing blocks most writes)
- `TaskAlreadyExists`, `AlreadyScheduledTask` (maintenance window)

### 6.6 Errors

Errors use the DMTF `{"error":{"code","message","@Message.ExtendedInfo":[…]}}` envelope (`add_error_body`,
`create_message`). Message IDs come from the registries in 4.3 (mostly `Base.1.12.0`, `Ami.1.0.0` and
`AmiOem.1.0.0`). `@Message.ExtendedInfo` is also attached to successful PATCH responses (for example
`PropertyValueModified`, `ResourceModified`). Some messages carry the non-standard `Registry`/`RelatedProperties`
fields that the sync-agent passes back.

### 6.7 Other quirks

- **Trailing slashes.** Trailing `/` is redirected rather than accepted (`redirect-handler.lua`). Both
  `/redfish/v1` and `/redfish/v1/` work.
- **`/Members` POST aliases.** POSTs to `/…/Members` (Accounts, Roles, Sessions, Subscriptions, Systems,
  BootOptions, VLANs, Certificates, Triggers, MetricReportDefinitions, SecureBoot DBs) are accepted as
  creates.
- **Settings objects.** Writes to BIOS go to `/Systems/Self/Bios/SD` (`@Redfish.Settings`). There are SD
  variants for `/Systems/Self/SD`, `/BootOptions/{id}/SD`, NetworkDeviceFunctions and VLANs. PATCHing
  `/Systems/Self/Bios` directly isn't allowed (GET only).
- **Manager NICs.** `/Managers/Self/EthernetInterfaces/usb0` (the host-interface NIC) is GET-only via
  an allow-method override. `/EthernetInterfaces/{id}/{sub}` (`resource_014`) exists for sub-objects.
- **Composition service.** `/Systems/Capabilities` exists, and POST `/Systems` / `/Systems/Members`
  composes systems (`CompositionService`, physical only; `db_init/ComposabilityService.rcmd`). Probably
  irrelevant for a single-node board.
- **Debug endpoint.** `/redfish/v1/GlobalLuaTables` (GET, DELETE) exposes and clears internal caches and
  skips privilege checks. **Block it at the proxy.**
- **Advertised but unrouted.** `AMIManager.RedfishDBReset` and `EventService.SubmitDelayedTestEvent` are
  advertised or have handlers but no route. `/redfish/v1/odata` lists `DynamicExtension` and
  `configurations`, which return 404.
- **Missing standard resources.** There's no `/redfish/v1/UpdateService/SoftwareInventory` route, even
  though `redfish/update-service/software-inventory.lua` exists and the Allow table lists it.
- **Telemetry data.** `MinCollectionInterval` is PT10M and `MetricReports:MaxReports` is 5. Only
  Fan/Temperature/Voltage reading MetricDefinitions are predefined.
- **Default credentials.** The seeded Redfish `Administrator` account has `PasswordChangeRequired=true`. In
  practice the sync-agent unifies it with the IPMI/web users (`ipmiredfishusersync.lua`).

---

## 7. Firmware update via Redfish

### 7.1 Entry points

1. **`POST /redfish/v1/UpdateService/Actions/SimpleUpdate`** (`redfish/update-service.lua`).
   - **Body.** `{"ImageURI": "...", "TransferProtocol": "HTTP"|"FTP"|"HTTPS", "Username"/"User": "...",
     "Password": "..."}`. The protocol can also come from the URI scheme (`^(.*)://(.*)$`).
   - **Scope.** BMC image only ("This action is used to perform BMC Firmware update").
   - **HTTPS.** Verified with the CA bundle uploaded via `UpdateService.UploadCABundle`
     (`/tmp/redfishfwupdate/redfish_ca_bundle.pem`). Errors: `SSLCertificateBadFile`, `SSLVerifyFailed`,
     `CouldNotEstablishConnection`, `AccessDenied` (mapped from curl codes).
   - **Response.** 202 Accepted with a Task and a `Location: /redfish/v1/TaskService/TaskMonitors/<id>`.
2. **`POST /redfish/v1/UpdateService/upload`** (`MultipartHttpPushUri`, `redfish/multipart-push.lua` +
   `redfish/multipart-handler.lua`). Content type `multipart/form-data`, with these parts:
   - **`UpdateParameters`** (JSON):
     - `Targets: [ "<FirmwareInventory/Manager/System URI>" ]`. The target map is in
       `update-service_config.lua`: `/redfish/v1/Managers/Self` → BMC, `/redfish/v1/Systems/Self/Bios`
       → BIOS, `/redfish/v1/Systems/Self/Cpld` → CPLD. The FirmwareInventory URIs are also accepted
       (`get_firmware_inventory_uri_map`).
     - `@Redfish.OperationApplyTime`
     - `Oem` / `OemParameters`: `FlashType` (`FULLFwUpdate` | `DUALIMAGEFwUpdate` | `HPMFwUpdate`),
       `UploadSelector` (`AutoInactive` | `Image1` | `Image2` | `ImageBoth` | `Default`), `ImageType`,
       `preserveconf`
   - **`UpdateFile`**: the image. Extension and format are checked (`.ima` BMC; `.bin` BIOS, CPLD, BP,
     FPGA, PSU, RETIMER, PFR, PLDM; `.hpm` HPM bundles) by `is_bmc_image_valid`, `is_bios_image_valid`,
     `is_hpm_image_valid`, `is_pfr_update_capsule_valid` and `is_pldm_fwupdate_package_valid`. The image
     is saved under `/tmp/redfishfwupdate/` as `rom.ima`, `bios.bin`, `cpld.bin`, `bp.bin`, `fpga.bin`,
     `psu.bin`, `retimer.bin`, `pfr.bin`, `pldm.bin` or `rom.hpm`.
   - **Errors.** Upload errors map to `AmiOem` messages: `InvalidFirmwareImageFormat`,
     `error_update_no_file_existed`, `RequiredKeyParametersMissing` and similar.
3. **`POST /redfish/v1/UpdateService/Actions/Oem/UpdateService.UploadCABundle`**: uploads the CA bundle for
   HTTPS ImageURI.
4. **`PATCH /redfish/v1/UpdateService`**: `Oem.AMIUpdateService.PreserveConfiguration.*`, which selects
   which configuration sections survive a BMC flash.

### 7.2 Updatable components

`multipart-push.lua` and `sync-agent/extensions/update_service/init_agent_scripts/init-updateservice.lua`
list the components. Each is feature-gated by a `CONFIG_SPX_FEATURE_*` flag in `/etc/core_features`:

| Component | Feature flag | sync-agent helper |
|---|---|---|
| BMC (`.ima`, full or dual image) | always | `extensions/update_service/sync-helpers_update/FwUpdate.lua` |
| BIOS | `CONFIG_SPX_FEATURE_BIOS_COMPONENT_SUPPORT` | `extensions/ami/sync-helpers_ami/bios_update.lua` (HPM component manager; `/var/bios_update_selection`) |
| CPLD | `CONFIG_SPX_FEATURE_CPLD_FW` / `_CPLD_COMPONENT_SUPPORT` | `cpld_update.lua` |
| BP (backplane) | `CONFIG_SPX_FEATURE_BP_FW` | `bp_update.lua` |
| FPGA | `CONFIG_SPX_FEATURE_FPGA_FW` | `fpga_update.lua` |
| PSU | `CONFIG_SPX_FEATURE_PSU_FW` | `psu_update.lua` |
| Retimer | `CONFIG_SPX_FEATURE_RETIMER_FW` | `retimer_update.lua` |
| Intel PFR capsule | `CONFIG_SPX_FEATURE_INTEL_PFR_SUPPORT` | `pfr_update.lua` |
| PLDM FW update package | `CONFIG_SPX_FEATURE_PLDM_FWUPDATE_REDFISH_SUPPORT` | `pldm_update.lua` (`libpldmupdate_ffi`) |
| HPM bundle | (FlashType `HPMFwUpdate`) | `hpm_fwupdate.lua`, `hpm_fwupdate_compmngr.lua` |
| ME (inventory only) | `CONFIG_SPX_FEATURE_ME_COMPONENT_SUPPORT` | |

Which flags are actually set on B650D4U depends on `/etc/core_features` in the image. Check it before
promising BIOS or CPLD updates through Redfish. FirmwareInventory members are created by
`init-updateservice.lua` (`Redfish:UpdateService:FirmwareInventory:<id>:{Name,Version,Updateable}`, with
BMC Image1/Image2 versions from `GetIMG1FWInfo`/`GetIMG2FWInfo` for dual image).

### 7.3 Steps (BMC)

From `update-service/update-service.lua`, `FwUpdate.lua` and the task service:

1. **Accept.** The handler validates, creates `Redfish:TaskService:Tasks:<n>` (`TaskType=LUA_SCRIPT`,
   script `./update-service/update-service.lua`, `TaskIPCData`/`TaskWebRequestData` holding the request)
   and a TaskMonitor, then returns **202** with `Location` (task monitor) and a Task body.
2. **Start.** The task-service runs the script. It writes `POST:Redfish:UpdateService:Actions:SimpleUpdate`,
   `POST:Redfish:UpdateService:MultipartPush` or
   `POST:Redfish:UpdateService:OemAction:StartFwUpdate:<COMPONENT>`. The sync-agent's update extension
   (`sync-agent_update`) picks it up.
3. **Flash mode.** "1. Set to FlashMode…" (`/var/flasher.redfishstart`, `PrepareFlashArea`).
4. **Download.** "2. Download Image from Remote Path…" (SimpleUpdate only; minicurl → `/mnt/fwupdate/rom.ima`
   or `/tmp/redfishfwupdate/rom.ima`; `/var/flasher.redfishdownload`).
5. **Verify.** "3. Verify Image…": signature and checksum. Errors are listed as "Error because …", for
   example "Not a signed image", "Image checksum does not match" or "Different Code Base Version".
6. **Flash.** "4. Flash Image…" (`/var/flasher.redfishflashing`). Progress is in
   `/tmp/flasher.redfishprogress` → `Oem.AMIUpdateService.FlashPercentage` and the Task `PercentComplete`.
   Component (HPM) flashing uses `/tmp/redfish.hpmprogress`.
7. **Finish.** The configuration sections chosen in `PreserveConfiguration` are preserved
   (`PreserveConfig.lua`, `savedb_to_conf.sh`/`savedb_to_newconf.sh`). The sync-agent and procmanager stop
   and the BMC reboots ("Finish update … Ready to reboot with new firmware").

During the update, `Oem.AMIUpdateService.UpdateStatus` and `UpdateTarget` reflect progress, and other
writes may return `FWUpdateInProgress`. Cancel is possible through `/var/flasher.redfishcancel` /
`Redfish:UpdateService:Oem:CancelFwUpdate` (BMC only, no public Redfish action in 9.05). Messages emitted:
`UpdateService.1.0.0`/`AmiOem`/`Task` registry entries such as `DownloadFirmware*`, `VerifyFirmware*`,
`PrepareFlashArea*`, `FlashFirmware*`, `FirmwareUpdateCompleted`, `FirmwareUpdateFailed`, `HpmInvalidImage`
and `PfrInvalidUpdateCapsule`. Audit-log entries are added.

### 7.4 Task monitor

- **Task monitor.** `GET /redfish/v1/TaskService/TaskMonitors/{id}` (`redfish/task-monitor.lua`) returns
  **202 Accepted** with `Location` and `Retry-After` while running, then the final response of the
  original operation once complete. A Link header points to the task. `DELETE` on the monitor cancels or
  kills the task (`kill -9` of the task process) where supported.
- **Task resource.** `GET /redfish/v1/TaskService/Tasks/{id}` gives `TaskState` (`New`, `Starting`,
  `Running`, `Suspended`, `Interrupted`, `Pending`, `Stopping`, `Completed`, `Killed`, `Exception`,
  `Service`, `Cancelled`), `TaskStatus`, `PercentComplete`, `Messages[]`, `StartTime`, `EndTime`,
  `Payload` (`HttpOperation`, `TargetUri`, `JsonBody`, `HttpHeaders`, `HidePayload`) and `TaskMonitor`.
- **Limits and events.** At most `MAX_TASKS` tasks; completed ones are overwritten `Oldest` first.
  `LifeCycleEventOnTaskStateChange=true` emits `TaskEvent.1.0.1` events (TaskStarted, TaskCompletedOK,
  TaskAborted, TaskCancelled).

---

## 8. 7.02 vs 9.05

The two Redfish trees are **almost identical**:

- **Files.** `diff -r` of `/usr/local/redfish` finds one removed file and six changed ones. Everything else
  is byte-identical, including all 489 other Lua bytecode files, `db_init/`, `schemas/`, `static/` and the
  privilege registries. `/usr/local/sync-agent`, `/usr/local/redfish_utils`, `/usr/local/redis`,
  `/etc/redis` and `/etc/defconfig/redfish` are byte-identical.
- **Removed in 9.05: the KCS resource.**
  - URI: `/redfish/v1/Managers/{ManagerId}/KCS`, GET and PATCH (allow table "GET, PATCH"), key
    `plugin_redfish_kcs`, entity `KCS`, type `KCS.v1_0_0.KCS`.
  - Handler: `oem/plugin/kcs.lua` (7.02 only). It used `libipmi` raw IPMI
    (`LIBIPMI_Send_RAW_IPMI2_0_Command`) with `SetAccessBMCByKCSReq_T`/`GetAccessBMCByKCSRes_T`
    (`Control`) to read and toggle in-band KCS access (`InterfaceEnabled`).
  - The link `Managers/Self.KCS` was dropped from `oem/plugin/resource_metadata/ManagersInstance.lua`.
- **Changed files.**
  - `oem/plugin/constants.lua`: KCS constants removed.
  - `oem/plugin/redfish-resource/redfish-resource-{URI_pattern,allow_method,entities,handler}.lua`: the
    KCS entry was removed.
  - `oem/plugin/resource_metadata/ManagersInstance.lua`: KCS link removed. It now contains a leftover
    debug `print("jeff access_posthook_params")` in the IKVM property hook, so it logs on every Manager
    GET.
- **Unchanged.** Auth, sessions, update flow, schemas, registries, defaults and the message registries.
  `static/openapi.yaml.gz` is identical too (and stale in both).

For a proxy, 7.02 and 9.05 can be treated as the same API. The only exception is `Managers/Self/KCS`; if
it's needed, the IPMI "Set/Get KCS access" OEM command can replace it.

---

## 9. X470 (3.02) Redfish tree

Location `firmware-dumped/X470D4U_3.02.00/rootfs/usr/local/redfish/`. `redfish/` has 31 entries. There are 778 `.lua` files in total;
275 are LuaJIT bytecode and **503 are plain-text Lua**. The text files are almost all `oem/dre/**` (493
files), plus `redfish/update-service.lua`, `redfish/upload-file.lua`, `redfish/update-service/*.lua`,
`extensions/routes/update_service_routes.lua`, `extensions/constants/*`, `extensions/allow-header/*`, one
property-type file, one property-access file and `message_registries/DynamicExtension.1.0.0.lua`.

### 9.1 What's there

- **Older AMI generation, same architecture.** Turbo.lua + redis + sync-agent, and the same lighttpd
  include (`redfish_lighttpd.conf`: `proxy.server = ("/redfish" => 127.0.0.1:9080)`, the same optional
  mTLS block, `/bios/` gzip). Init scripts exist and are linked: `S26redisrsync`, `S27redis-server`,
  `S28sync-agent`, `S29redfish-server`, `S48redfish-settings`, `S89redfish-nw-up`.
  `/etc/defconfig/lighttpd.conf` includes `redfish_lighttpd.conf`. **There's no separate event-service or
  task-service init script, no SSE proxy and no 9090 listener.**
- **Routing.** The older style: one large `default_route.lua` table (`{pattern, Handler}` pairs) with 142
  patterns, `extensions/routes/hi_routes.lua` (52), `oem/ami/route.lua` (10), `oem/ami_hi/route.lua` (7)
  and the text `extensions/routes/update_service_routes.lua`.
- **Coverage.** ServiceRoot, odata/$metadata, Systems:
  - Processors, SubProcessors (nested twice), Memory with `Memory/{id}/Metrics`, MemoryDomains/Chunks
  - EthernetInterfaces/VLANs, NetworkInterfaces, SimpleStorage, Storage/Drives/Volumes
  - Bios (+SD, Actions, ChangePassword/ResetBios ActionInfo), SecureBoot, BootOptions
  - LogServices (+ClearlogActionInfo), Actions/ResetActionInfo

  Chassis:
  - Power, Thermal, Thermal/Fans/{id}, LogServices, NetworkAdapters/NDF/NetworkPorts
  - `PCIeDevices/{id}` and `PCIeDevices/{id}/Functions/{id}` (older path, not `PCIeFunctions`)
  - Assembly, `$ref`

  Managers:
  - EthernetInterfaces, LogServices, NetworkProtocol, SerialInterfaces
  - VirtualMedia (no `/Actions` route; insert via PATCH)
  - Actions/ResetActionInfo, HostInterfaces

  Services:
  - AccountService/Accounts/Roles, SessionService/Sessions, EventService/Subscriptions (+SubmitTestEventActionInfo)
  - TaskService/Tasks (no TaskMonitors), JsonSchemas, Registries (+Static)
  - CompositionService (ResourceBlocks/Zones), Fabrics/Endpoints
  - TelemetryService (MetricDefinitions, MRD, MetricReports, Triggers, `LogServices/{id}` plural)
  - GlobalLuaTables
- **Absent compared with B650 9.05.** CertificateService and every `/Certificates` resource;
  ExternalAccountProviders; Sensors; PowerSubsystem/ThermalSubsystem; PCIeSlots; USB, Graphics and Media
  controllers; storage Controllers and Ports; OperatingConfigs; AccelerationFunctions; ProcessorMetrics;
  SecureBootDatabases; SSE; TaskMonitors; FirmwareInventory; DeepOperations; PLDM RDE; OEM Inventory CRC
  under `ami_bios`; `Managers/Self/IKVM`, `Systems/Self/FruInfo`, Smbios/Pldm plugins; `openapi.yaml`.
- **UpdateService** (plain text, readable: `redfish/update-service.lua`, 897 lines):
  - routes: `UpdateService`, `UpdateService/{ActionInfo}`, `Actions/SimpleUpdate`,
    `Actions/Oem/UpdateService.UploadFirmwareImage` (`upload-file.lua`), `Actions/Oem/{action}`
  - OEM action `UpdateService.BMCFwUpdate` (`BMCFwUpdateActionInfo`); SimpleUpdate `allowed_protocols =
    {"HTTP","FTP"}` (no HTTPS)
  - `allowed_bmcflashtype = {"FULLFwUpdate","DUALIMAGEFwUpdate"}`, `allowed_uploadselector =
    {"AutoInactive","Image1","Image2","ImageBoth","Default"}`
  - the FirmwareInventory/SoftwareInventory routes are **commented out** ("TODO Not supporting
    FirmwareInventory and SoftwareInventory"). There's no multipart push URI.
  - `oem/ami/route.lua` also has `/UpdateService/Actions/AmiOemBMCUpdate`.
- **OEM.**
  - `oem/ami`: `/AccountService/Configurations`, `/configurations`, `/Managers/Self/Actions/RedfishDBReset`
    + `FactoryResetActionInfo`, `SubmitDelayedTestEvent` (+ActionInfo), `AmiBios.ChangeState` on Memory
    and PCIeDevices, `/Systems/{id}/Oem/Ami/Inventory/Crc`
  - `oem/ami_hi`: `BiosStaticFiles`, `Oem/Ami/InventoryData[/Status]`, `Systems/{id}/Bios/Actions`
  - `oem/dre`: the plain-text **AMI Dynamic Redfish Extension** framework, 493 files, with
    `dynamic-root-handler.lua` → `/redfish/v1/DynamicExtension`, `dynamic-extension-{collection,instance}`,
    `dynamic-log-*`, and `subextensions/`, `query/`, `patch/`, `post/`, `delete/`. It lets BIOS register
    extensions at runtime; B650 replaces it with the `ami_bios`/`ami_hi` packages and leaves a stub link in
    `/redfish/v1/odata`. `oem/dre/route.lua` has no literal patterns (routes are built at runtime).
- **Versions.** ServiceRoot v1_3_1 (also v1_0_0), ComputerSystem v1_5_0, Chassis v1_7_0, Manager v1_4_0.
  Registries: `Base.1.0.0`, `AmiOem`, `DynamicExtension`, `EventLog`, `HttpStatus`, `IPMI`, `Security`,
  `SyncAgent`. The 20 schemas are AMI OEM only. This is an approximately 2017–2018 bundle (Redfish
  ~1.1–1.4).
- **Defaults** (`db_init/*.rcmd`, plus `db_init/init-script/*.sh` and `redis-init.sh`): identical policy to
  B650. `SessionTimeout 30`, `AccountLockoutThreshold 5`, `AccountLockoutDuration 30`,
  `AccountLockoutCounterResetAfter 30`, `MinPasswordLength 8`.
- **Bundled sample keys.** `certs/` contains `ca-key.pem`, `server-key.pem`, `server.pem`, `user.pem`. No
  Lua strings reference `certs/`, so they look like unused development artifacts. They're still private
  keys shipped in firmware: don't trust them.
- `unit-test/` is empty.

### 9.2 Assessment

The X470 tree is a **complete, wired-up, functional older-generation AMI Redfish service**, not a stub.
The init links, lighttpd include, redis, sync-agent and route tables are all present and consistent. It
should answer the core DMTF resources:

- Systems/Chassis/Managers
- Power/Thermal
- logs
- accounts and sessions
- virtual media (PATCH)
- BIOS settings via host interface
- SimpleUpdate (HTTP/FTP)

Its gaps for a proxy are:

- no SSE, TaskMonitors or FirmwareInventory
- no CertificateService
- no Sensors or PowerSubsystem/ThermalSubsystem (only legacy Power/Thermal)
- PCIe functions at `/PCIeDevices/{id}/Functions/{id}`
- older schema versions and the older Base 1.0.0 message registry

Treat it as a separate "AMI-legacy" dialect in the Go proxy (normalise `Functions` ↔ `PCIeFunctions`,
`TelemetryService/LogServices/{id}` ↔ `TelemetryService/LogService`, synthesise FirmwareInventory from
other data). Live verification is recommended, because the strings-only analysis can't tell whether
individual handlers are fully implemented or return placeholders.

---

## Appendix A: implications for the Go proxy

- **Ports.** Front the BMC on HTTPS :443 only. 9080 and 9090 are loopback.
- **SSE.** Maintain a long-lived stream for `/redfish/v1/EventService/SSE` (B650 only). It's subject to
  `MAX_SSE_CLIENT_CONNECTION` and to the 10-connections-per-IP lighttpd cap; fan out locally rather than
  opening one SSE per downstream client.
- **Sessions.** Keep one `X-Auth-Token` session per BMC and refresh it more often than `SessionTimeout`
  (default 30, redis TTL renewed on each request). Re-login on 401. Respect `MAX_SESSIONS`.
- **Lockout.** Five failures in 30 s locks the account for 30 s, and can IP-lock the proxy's address, which
  affects every user behind it. Validate credentials before forwarding bursts.
- **Conditional writes.** Forward `If-Match` and use the GET `ETag` for PATCH. Handle 412 and 428.
- **Long operations.** Expect 202 + Task monitor for Reset, InsertMedia, SimpleUpdate, MultipartPush, BIOS
  actions and maintenance-window operations. Poll the TaskMonitor with `Retry-After`.
- **Blocking.** Block `/redfish/v1/GlobalLuaTables` and possibly `/redfish/v1/openapi.yaml` (misleading).
  Treat `/redfish/v1/odata` `DynamicExtension`/`configurations` links as dead.
- **Pagination.** Follow `Members@odata.nextLink`. `$expand` and `$select` work but are costly on a
  single-threaded server; prefer targeted GETs.
- **Version dialects.** Use the firmware version to pick the X470 legacy or B650 dialect. For B650, 7.02
  and 9.05 differ only by `/Managers/Self/KCS`.

---

Revision note (2026-11-16): cross-checking §2 against the 9.05/7.02 route tables (`strings` only) found that `/redfish` is key `resource_127` (literal `^/redfish$`), not `resource_002`; the trailing-slash redirect is `resource_002`, not `resource_001`; added the missing `/redfish/v1` (no slash, `resource_001`) redirect row, so §2 now has 308 rows; corrected the privilege-registry row to `/redfish/v1/PrivilegeRegistry/Redfish_1.3.0_PrivilegeRegistry.json` (`resource_130`, pattern only, no handler); updated the §3.3 whitelist table to match; fixed the X470 path in §9. Machine-readable companion: `b650-uri-inventory.json`.
