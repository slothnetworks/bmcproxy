import json, collections, sys, re
d = json.load(open(sys.argv[1]))
bs = ['X470-3.02', 'B650-7.02', 'B650-9.05']
HF = ['x470d4u.har', 'b650d4u.har', 'b650d4u-2t.har']
HS = {'x470d4u.har': 'X', 'b650d4u.har': 'B', 'b650d4u-2t.har': '2T'}
areas = []
for e in d:
    if e['area'] not in areas: areas.append(e['area'])
cnt = {b: collections.Counter(e['handler_present'][b] for e in d if b in e['boards']) for b in bs}
nui = {b: sum(cnt[b].values()) for b in bs}
rows = ["| Area | X470-3.02 UI refs (handler yes/no/unknown) | B650-7.02 UI refs (yes/no) | B650-9.05 UI refs (yes/no) | Seen in HAR |", "|---|---|---|---|---|"]
for a in areas + ['TOTAL']:
    es = d if a == 'TOTAL' else [e for e in d if e['area'] == a]
    cells = []
    for b in bs:
        c = collections.Counter(e['handler_present'][b] for e in es if b in e['boards'])
        n = sum(c.values())
        cells.append('%d (%d/%d%s)' % (n, c['yes'], c['no'], '/%d' % c['unknown'] if b == 'X470-3.02' else ''))
    lv = sum(1 for e in es if any(e['seen_in_har'].values()))
    rows.append('| %s | ' % ('**TOTAL**' if a == 'TOTAL' else a) + ' | '.join(cells) + ' | %d |' % lv)
nohand = [e for e in d if 'B650-9.05' in e['boards'] and e['handler_present']['B650-9.05'] == 'no']
grp = collections.Counter()
for e in nohand:
    p = e['path'][5:]
    m = re.match(r'(settings/[a-z_]+_management|settings/[a-z_]+_mgmt|asrr/maintenance/[A-Z]+|maintenance/[a-z]+|[a-z_]+)', p)
    grp[m.group(1) if m else p] += 1
grp_txt = ', '.join('`%s` (%d)' % kv for kv in sorted(grp.items(), key=lambda kv: -kv[1]))
seen = [e for e in d if any(e['seen_in_har'].values())]
seen_unhandled = [e['path'] for e in seen if any(e['handler_present'][b] != 'yes' for b in bs if b in e['boards'])]

idx = ['| Path | Methods | X470-3.02 | B650-7.02 | B650-9.05 | HAR |', '|---|---|---|---|---|---|']
for e in d:
    cells = []
    for b in bs:
        ui = 'UI' if b in e['boards'] else '-'
        cells.append('%s / %s' % (ui, e['handler_present'][b]))
    har = ', '.join(HS[h] for h in HF if e['seen_in_har'].get(h)) or '-'
    meth = '/'.join(e['methods']) + (' (+item %s)' % '/'.join(e['item_methods']) if e.get('item_methods') else '')
    idx.append('| `%s` | %s | %s | %s |' % (e['path'], meth, ' | '.join(cells), har))

sec6 = '''## 6. Handler presence and live validation

### 6.1 Handler presence by area

"Handler in firmware" means the `/api` route string exists in that image's REST binaries (see Sources). It is a strings-level check: "yes" means the router knows the path, not that every method works. "no" means the UI calls the path but no shipped binary contains it; expect a 404 or a FastCGI error. "unknown" is used for the four X470 `settings/fan/fan_*` collections, which appear only as handler symbol names (`settings_fan_mode`, `settings_fan_tscft`, …) and not as route strings.

''' + '\n'.join(rows) + '''

Per board, of the paths its own UI references: X470-3.02 %(x)d (%(xy)d yes, %(xn)d no, %(xu)d unknown); B650-7.02 %(s)d (%(sy)d yes, %(sn)d no); B650-9.05 %(n)d (%(ny)d yes, %(nn)d no).

### 6.2 Why so many B650 paths have no handler

The B650 web UI is AMI's generic SPX bundle and is a superset gated by `configuration/project` features. `spx_restservice` `dlopen`s optional plugins, and 18 of them are not shipped (`docs/firmware/b650d4u.md` §5.1). The %(nn)d unhandled 9.05 paths group as: %(grp)s.

`docs/firmware/b650d4u.md` reports 151 of 366 B650-UI endpoints without a handler. This catalog finds %(nn)d of %(n)d for 9.05. The differences: this catalog normalises trailing-slash and prefix forms (`/api/settings/users/`, `/api/maintenance/`, `/api/scripts/`) into their base paths, adds the dynamic HPM/RAID and viewer-only paths, and matches regex item routes, which shows that `/api/scripts` and `/api/settings/user/ssh-key-upload/:id` do have handlers (`/scripts`, `/settings/user/ssh-key-upload/([[:digit:]]+)`).

On X470-3.02 only two UI paths have no route string. The X470 image also contains route strings for many generic AMI pages its UI does not use (RAID, SAS-IT, NVMe, AE). The plugins behind several of those (`libspx_restservice_{acd,aep,nvme,pldm,raid,sasit,sol}.so`) are not shipped, so treat X470 "yes" for those groups as "route known, back end probably missing" (`docs/firmware/x470d4u.md` §5.2).

### 6.3 Validation against the live captures

All three captures were parsed in full: every `/api` request's method, path, status, query keys, request content type and body keys, and the response's key names and JSON types. No values were copied. The results are on each endpoint as `Seen in HAR` and `Live (<file>)` lines, and in the JSON as `seen_in_har` / `har_observed` / `har_vs_js`. %(nseen)d catalog paths were seen live. For endpoints with a captured response, the **response field list is built from the capture** and then extended with fields the UI reads but the capture lacks (marked "absent from capture"). The original JS-only list is kept in the JSON as `response_fields_js_inferred`.

Corrections and findings from the captures:
- **The captures run older firmware** than the analysed bundles (X470D4U 2.32.00; B650D4U 1.10.00; the `b650d4u-2t.har` board reports `mb_name` `B550D4U-2T`, fw 1.10.00). A shape seen live is reliable. A field that is absent may simply be newer than the capture.
- `GET /api/settings/media/adviser`: the key is **`keyboard_layout`** (all captures), not `keyboard_language`. B650D4U also returns `kvm_client`.
- `GET /api/sensors`: static analysis had attached SEL/event fields (`EventData1…`, `sensor_type_table`, …) to this endpoint from sibling models. The capture shows a flat array of `{id, sensor_number, name, owner_id, owner_lun, raw_reading, type, type_number, reading, sensor_state, discrete_state, accessible, lower_/higher_{non_recoverable,critical,non_critical}_threshold, unit}`. X470 adds `settable_readable_threshMask`; B650-family firmware returns `settable_flag` instead. `raw_reading` is a float on X470 and an int on B650. On B650-family firmware an unset threshold is the **string `"NA"`** instead of a number (X470 returns numbers throughout), and `reading` may be an int or a float. A Go decoder needs a flexible number type for these fields. `GET /api/dashboardsensors` (B650) returns the same item shape, but some items carry only `{id, sensor_number, name}`.
- `GET /api/host_inventory/host_interface_{processor,system}_info`: the JS lists mixed the processor, memory and system models. Corrected from the capture (processor: `Id, Name, Manufacturer, MaxSpeedMHz, Model, ProcessorArchitecture, ProcessorType, Socket, State, EffectiveFamily, TotalCores`; system: `Name, Description, Model, IndicatorLED, Manufacturer, PowerState, SerialNumber, PartNumber, SystemType, State, AssetTag, BiosVersion, UUID`). Empty arrays are returned when the host has not pushed inventory (X470 and B650D4U captures). `host_interface_storage_info` returns the misspelled key `Storage_Contoller`, and returns `null` for both members when it has no data.
- `GET /api/chassis-status`: `power_cycle_interval` is present on B650D4U 1.10 but not on B550D4U-2T 1.10 or X470. A proxy should treat it as optional (2000 ms in the one capture that has it). `power_on_after_flash` (read by the B650 UI) did not appear in any capture.
- `POST /api/session`: B650-family responses add `user_id`, `channel` and `passwordStatus` (confirmed). `TFAEnabled`/`TFAStatus` appear only with two-factor enabled (not captured).
- `GET /api/settings/date-time`: X470 returns `auto_date` and `daylight`. B550D4U-2T returns `ntp_auto_date` and no `daylight`. A proxy must not assume one key set.
- `GET /api/settings/users`: the two families have different item shapes. X470: `network_privilege`, `privilege_limit_serial`. B650-family: `channel`, `channel_type`, `userid`, `accessByChannel`, `privilege`, `privilegeByChannel`, `password_age`, `password_enddate`.
- `GET /api/logs/audit?cmd=…` returns the count form `[{"audit_count":N}]` (B650D4U). Without `cmd` it returns the entries `[{id, timestamp, hostname, message}]` (B550D4U-2T).
- `GET /api/logs/eventlog?EVENTID=…` (B550D4U-2T) is the incremental form. B650-family entries add `sensor_type_table`, `record_type_table`, `event_direction_table`. X470 adds the `pci_*_num` and `*_Description_raw` fields.
- `POST /api/asrr/settings/setopenlooptablenumber`: the request key is literally **`table_numble`** (sic), and the response is `{}` (B650D4U capture, an endpoint the analysed B650 UIs no longer reference).
- `PUT /api/settings/network-bond`: the request is the whole model plus `bond_ifc`, and the response echoes it (confirms the singleton read-modify-write rule in 1.2).
- `GET /api/asrr/product-name` returns HTTP 500 `{"error":"Error in Getting Model Name","code":16005}` on X470 2.32. Use `asrr/model-name` (`mb_name`, plus `sys_name` on B650-family firmware).
- `GET /api/asrr/fw-info`: X470 returns `BMC/BIOS/ME/Micro_Code/CPLD`. B650-family firmware adds `PSP_fw_version`, and B650D4U adds `BP_version`. `BPCPLD_version` (read by the 9.05 UI) was not captured.
- `GET /api/settings/services` returned **401** before login in both B650 captures. It is the first call the B650 UI makes, and the 401 is expected.
- Every request after login carried `X-CSRFTOKEN` in all three captures. Cookies are stripped from the captures.
- Paths seen live whose handler column is not "yes" on every board whose UI references them: %(su)s.

### 6.4 Endpoint index (handler column)

Cell format: `<UI references it?> / <handler present in firmware>`. `-` = not referenced by that board's UI. HAR: `X` = x470d4u.har, `B` = b650d4u.har, `2T` = b650d4u-2t.har.

''' % dict(x=nui['X470-3.02'], xy=cnt['X470-3.02']['yes'], xn=cnt['X470-3.02']['no'], xu=cnt['X470-3.02']['unknown'],
           s=nui['B650-7.02'], sy=cnt['B650-7.02']['yes'], sn=cnt['B650-7.02']['no'],
           n=nui['B650-9.05'], ny=cnt['B650-9.05']['yes'], nn=cnt['B650-9.05']['no'], grp=grp_txt, nseen=len(seen),
           su=', '.join('`%s`' % p for p in seen_unhandled) or 'none') + '\n'.join(idx) + '\n'

head = open('head2.md').read()
src = open(sys.argv[2]).read()
i = src.index('## 9. Proxy implementation notes (Go)')
proxy = '\n---\n\n' + src[i:]
proxy = proxy.replace('About 150 paths the B650 UI references have no handler in 9.05.',
                      '%d paths the B650 9.05 UI references (%d on 7.02) have no handler.' % (cnt['B650-9.05']['no'], cnt['B650-7.02']['no']))
diff = open(sys.argv[3]).read()
cat = open(sys.argv[4]).read()
out = (head + sec6 + "\n---\n\n## 7. Endpoint catalog\n\nEach endpoint block lists: the boards whose UI references it; whether a handler is present in each board's firmware; whether it was seen in a HAR capture, with the captured request/response shape (key names and types only); the per-item path; query params; UI module(s); model; request fields (type / allowed values / validation); and response fields (from the capture where one exists, otherwise the fields the UI reads).\n\n"
       + cat + "\n\n---\n\n## 8. Board diff (X470 3.02 versus B650)\n\nBased on the analysed JS bundles; see 6.4 for handler presence per board. Note: the B650D4U capture (fw 1.10.00) also calls several endpoints listed here as X470-only (`/api/asrr/getvetobit`, the asrr fan endpoints), so older B650 firmware shares more of the X470 OEM surface than 7.02/9.05 do.\n\n"
       + diff + proxy)
open(sys.argv[5], 'w').write(out)
print(len(out.splitlines()), len(out))
