import json, re, collections, sys

BOARDS = [('x470', 'X470-3.02'), ('b650_702', 'B650-7.02'), ('b650_905', 'B650-9.05')]
FAMILY = {'x470': 'X470', 'b650_702': 'B650', 'b650_905': 'B650'}

# extra endpoints only referenced from viewer.min.js (H5Viewer)
VIEWER = {
    'api/kvm/token': dict(methods=['GET'], boards=['x470', 'b650_702', 'b650_905'], views=['viewer.min.js models/kvm_token'],
                          response=['client_ip', 'session', 'token'], request=[]),
    'api/viewerlogin': dict(methods=['POST'], boards=['x470', 'b650_702', 'b650_905'], views=['viewer.min.js login (viewer.html#<token>)'],
                            request=['token'], response=['ok', 'privilege', 'extendedpriv', 'racsession_id', 'server_addr', 'CSRFToken']),
    'api/logs/help-data': dict(methods=['GET'], boards=['x470', 'b650_702', 'b650_905'], views=['viewer.min.js RemoteControlView (help/about text, logo)'],
                               request=['file (query)'], response=['(text or image/jpeg blob)']),
    'api/settings/media/h5viewercfg': dict(methods=['GET'], boards=['b650_702', 'b650_905'], views=['viewer.min.js views/RemoteControlView (models/h5viewercfg)'],
                                           request=[], response=['kvm_service_status', 'client_ip', 'session', 'token', 'server_ip', 'automatic_off', 'local_monitor_off',
                                                                 'keyboard_language', 'num_cd', 'num_hd', 'kvm_num_cd', 'kvm_num_hd', 'power_save_mode']),
}
VIEWER_ALSO = {  # endpoints the viewer uses that also exist in source.min.js
    'api/configuration/project': ['x470', 'b650_702', 'b650_905'],
    'api/configuration/runtime': ['x470', 'b650_702', 'b650_905'],
    'api/session': ['x470', 'b650_702', 'b650_905'],
    'api/settings/media/adviser': ['x470', 'b650_702', 'b650_905'],
    'api/actions/accessbiossetup': ['b650_702', 'b650_905'],
}

def norm(p):
    p = p.lstrip('/')
    q = ''
    if '?' in p:
        p, q = p.split('?', 1)
    p = p.rstrip('/')
    return p, q

def area(p):
    a = p[4:]
    rules = [
        (r'^(session|asrr/session|reset-pass|updatenew_password|verify_otp|tfa|newtabinfo|viewerlogin)$', 'Session & authentication'),
        (r'^(configuration/|extfeatures|status/uptime|settings/user-preference)', 'Feature discovery & UI configuration'),
        (r'^(dashboardsensors|settings/dashboardinfo|settings/dashboardntp|logs/dashboardevent|asrr/selsummary|asrr/fw-info|asrr/model-name|asrr/product-name|firmware-info)', 'Dashboard & identity'),
        (r'^(chassis|chassis-status|actions/|asrr/PowerSource|asrr/(Get|Set)PowerRestore|settings/amdpowercap|asrr/(get|set)prochot|asrr/bmccoldreset)', 'Power, chassis & UID'),
        (r'^(sensors|detail_sensors_readings|sdr|settings/sensor-threshold|pldm/(sensors|state_sensors))', 'Sensors & SDR'),
        (r'^(fru|frueditable|fruupdate|pldm/fru|pldm/entity_association|pldm/eids)', 'FRU & PLDM inventory'),
        (r'^(kvm/|remote_control/|asrr/java-console|settings/media/adviser|settings/media/h5viewercfg|settings/media/remotesession|settings/media/active_redirections|settings/mouse|settings/javasol|sol/|settings/video/sol_configurations)', 'KVM, H5Viewer & SOL'),
        (r'^settings/media/', 'Virtual media / remote storage'),
        (r'^(logs/|settings/log|asrr/(get|clear)mcalog|asrr/mca-file|acd/|system_diagnostics/|pldm/get_all_event_log|host/(get_crash_dump|get_addc|download_addc|addc_))', 'Logs, crash dumps & diagnostics'),
        (r'^(host_inventory/|system_inventory/|smbiosdata|gpu|nvdimm_inventory/|host/asd/)', 'Host inventory'),
        (r'^settings/(raid_management|sasit_management|nvme_management|nvme_mi_management|pcie_switch_management|pcie_switch_mgmt|brcm_pcie_switch_management|nic_management)/', 'Storage / PCIe device management'),
        (r'^(settings/(network|static-ipv6|dns|ncsi|channels|packages|ncsilinkup|ncsiselect)|set/dns|asrr/(get|set)vetobit)', 'Settings: network'),
        (r'^(settings/(users|user/|users_read_only|pam-order)|asrr/settings/passwordcfg)', 'Settings: users & PAM'),
        (r'^(settings/(services|service-sessions|network-interfaces|web_server_instances|ipmi_disable_interfaces)|asrr/(get|set)WebEnableTimeout)', 'Settings: services & sessions'),
        (r'^(settings/ssl|operations/ssl)', 'Settings: SSL'),
        (r'^settings/(ldap|active-directory|radius)', 'Settings: LDAP / AD / RADIUS'),
        (r'^settings/date-time', 'Settings: date & time / NTP'),
        (r'^(settings/(smtp|pef|lan_destination|getSnmpv2|setSnmpv2)|snmp/)', 'Settings: SMTP, PEF, alerts & SNMP'),
        (r'^settings/firewall', 'Settings: firewall'),
        (r'^(settings/fan|asrr/settings/|asrr/GetFanDuty)', 'Settings: fan control'),
        (r'^(settings/(video|sol-recorded-video|download_image))', 'Settings: video recording / BSOD / SOL recording'),
        (r'^settings/licen', 'Settings: licensing'),
        (r'^(asrr/getbioscode|settings/ocprofile|settings/telemetrycups)', 'Settings: BIOS / platform OEM'),
        (r'^(maintenance|asrr/maintenance/)', 'Maintenance & firmware update'),
        (r'^(scripts|tasks|ae_test)', 'Automation engine (scripts/tasks)'),
    ]
    for rx, name in rules:
        if re.search(rx, a):
            return name
    return 'Miscellaneous'

AREA_ORDER = ['Session & authentication', 'Feature discovery & UI configuration', 'Dashboard & identity', 'Power, chassis & UID',
              'Sensors & SDR', 'FRU & PLDM inventory', 'Logs, crash dumps & diagnostics', 'Host inventory',
              'Storage / PCIe device management', 'KVM, H5Viewer & SOL', 'Virtual media / remote storage', 'Settings: network',
              'Settings: users & PAM', 'Settings: services & sessions', 'Settings: SSL', 'Settings: LDAP / AD / RADIUS',
              'Settings: date & time / NTP', 'Settings: SMTP, PEF, alerts & SNMP', 'Settings: firewall', 'Settings: fan control',
              'Settings: video recording / BSOD / SOL recording', 'Settings: licensing', 'Settings: BIOS / platform OEM',
              'Maintenance & firmware update', 'Automation engine (scripts/tasks)', 'Miscellaneous']

merged = collections.OrderedDict()
for b, label in BOARDS:
    for r in json.load(open('enr_%s.json' % b)):
        p, q = norm(r['path'])
        if not p.startswith('api/') or len(p) < 6:
            continue
        coll = r.get('collection')
        methods = set()
        item_methods = set()
        for m in r['methods']:
            base = m.split(' ')[0]
            if coll and m in ('PUT (save)', 'DELETE (destroy)'):
                item_methods.add(base)
            elif m == 'POST (create)':
                methods.add('POST')
            else:
                methods.add(base)
        if any(x.startswith(('models', 'collection')) for x in r['modules']) and not methods:
            methods.add('GET')
        if not methods and not item_methods:
            methods.add('GET')
        e = merged.setdefault(p, dict(path=p, methods=set(), item_methods=set(), boards=[], query=set(), views=set(),
                                      modules=set(), req={}, rsp={}, validation={}, options={}))
        e['methods'] |= methods
        e['item_methods'] |= item_methods
        if b not in e['boards']:
            e['boards'].append(b)
        if q:
            e['query'].add(re.sub(r'=.*?(&|$)', r'=…\1', q))
        e['views'] |= set(r['views'])
        e['modules'] |= set(r['modules'])
        for f in r['request']:
            e['req'].setdefault(f, set()).add(FAMILY[b])
        for f in r['response']:
            e['rsp'].setdefault(f, set()).add(FAMILY[b])
        for k, v in r.get('validation', {}).items():
            e['validation'].setdefault(k, v)
        for k, v in r.get('options', {}).items():
            e['options'].setdefault(k, v)

for p, v in VIEWER.items():
    e = merged.setdefault(p, dict(path=p, methods=set(), item_methods=set(), boards=[], query=set(), views=set(), modules=set(),
                                  req={}, rsp={}, validation={}, options={}))
    e['methods'] |= set(v['methods'])
    for b in v['boards']:
        if b not in e['boards']:
            e['boards'].append(b)
    e['views'] |= set(v['views'])
    for f in v['request']:
        e['req'].setdefault(f, set()).update(FAMILY[b] for b in v['boards'])
    for f in v['response']:
        e['rsp'].setdefault(f, set()).update(FAMILY[b] for b in v['boards'])
for p, bl in VIEWER_ALSO.items():
    if p in merged:
        merged[p]['views'].add('viewer.min.js (H5Viewer)')
        for b in bl:
            if b not in merged[p]['boards']:
                merged[p]['boards'].append(b)


merged.pop('api/maintenance', None)
for p, meth, req in (('api/maintenance/hpm/flash', 'PUT', ['COMPONENT_ID', 'COMPONENT_DATA_LEN', 'FWUPDATEID', 'SECTION_FLASH']),
                     ('api/maintenance/hpm/upgradestatus', 'GET', ['COMPONENT_ID (query)', 'FWUPDATEID (query)']),
                     ('api/maintenance/hpm/preparecomponents', 'PUT', []),
                     ('api/maintenance/raid_management/flash', 'PUT', ['COMPONENT_ID', 'COMPONENT_DATA_LEN', 'FWUPDATEID', 'SECTION_FLASH', 'RAID_CID']),
                     ('api/maintenance/raid_management/upgradestatus', 'GET', ['COMPONENT_ID (query)', 'FWUPDATEID (query)']),
                     ('api/maintenance/raid_management/preparecomponents', 'PUT', [])):
    e = merged.setdefault(p, dict(path=p, methods=set(), item_methods=set(), boards=[], query=set(), views=set(), modules=set(),
                                  req={}, rsp={}, validation={}, options={}))
    e['methods'].add(meth)
    for b in ('b650_702', 'b650_905'):
        if b not in e['boards']:
            e['boards'].append(b)
    e['views'].add('maintenance/HpmFirmwareUpdateView ("/api/maintenance/"+suffix)')
    for f in req:
        e['req'].setdefault(f, set()).add('B650')

HAR = json.load(open('har_seen.json'))
HARLBL = {'x470': 'X470D4U HAR (fw 2.32.00)', 'b650': 'B650D4U HAR (fw 1.10.00)', 'b650_2t': 'B550D4U-2T HAR (fw 1.10.00)'}
har_by_path = collections.defaultdict(dict)
for hb, calls in HAR.items():
    for k, v in calls.items():
        meth, path, status = k.split(' ')
        har_by_path[path].setdefault(hb, []).append('%s %s' % (meth, status))
HAR_ONLY = []
for path in har_by_path:
    if path not in merged:
        HAR_ONLY.append(path)
        e = merged.setdefault(path, dict(path=path, methods=set(), item_methods=set(), boards=[], query=set(), views=set(), modules=set(),
                                         req={}, rsp={}, validation={}, options={}))
        for hb, ms in har_by_path[path].items():
            for m in ms:
                e['methods'].add(m.split(' ')[0])
        e['views'].add('(not referenced by the analysed 3.02/7.02/9.05 JS; seen only in a live capture)')

HANDLERS = json.load(open('handlers.json'))
HUNKNOWN = {'x470': {'api/settings/fan/fan_mode', 'api/settings/fan/fan_closeloopctb', 'api/settings/fan/fan_openloopctb', 'api/settings/fan/fan_tscft'}}
def handler(b, p):
    hs = HANDLERS[b]
    if p in HUNKNOWN.get(b, ()):
        return 'unknown'
    q = re.sub(r'/:id$', '', p)
    if p in hs or q in hs:
        return 'yes'
    for h in hs:
        if h.endswith('/*') and p.startswith(h[:-1]):
            return 'yes'
    return 'no'
# drop noise fields
NOISE = {'HTTPErrorHandler', 'ajax', 'responseText', 'code', 'error'}
def fields(dct, both_fams):
    out = []
    for f in sorted(dct, key=str.lower):
        if f in NOISE and f not in ('code', 'error'):
            continue
        fam = dct[f]
        tag = ''
        if both_fams and len(fam) == 1:
            tag = ' (%s only)' % list(fam)[0]
        out.append(f + tag)
    return out

with open('manual_notes.json') as fh:
    MANUAL = json.load(fh)

entries = []
for p, e in merged.items():
    fams = set(FAMILY[b] for b in e['boards'])
    both = len(fams) == 2
    m = MANUAL.get(p, {})
    methods = sorted(set(m.get('methods', [])) | e['methods'])
    rec = dict(path='/' + p, methods=methods, item_methods=sorted(e['item_methods']),
               boards=[l for b, l in BOARDS if b in e['boards']], area=m.get('area', area(p)),
               query_params=sorted(e['query']),
               request_fields=m.get('request_fields') or fields(e['req'], both),
               response_fields=m.get('response_fields') or fields(e['rsp'], both),
               validation=e['validation'], options=e['options'],
               ui=sorted(x for x in (e['views'] | e['modules']) if not x.startswith(('models', 'collection'))),
               js_modules=sorted(x for x in e['modules'] if x.startswith(('models', 'collection'))),
               notes=m.get('notes', ''),
               handler={lbl: handler(b, p) for b, lbl in BOARDS},
               har={HARLBL[hb]: sorted(set(ms)) for hb, ms in har_by_path.get(p, {}).items()})
    if rec['item_methods']:
        rec['item_path'] = rec['path'] + '/:id'
    entries.append(rec)

entries.sort(key=lambda r: (AREA_ORDER.index(r['area']) if r['area'] in AREA_ORDER else 99, r['path']))

# JSON output
js = []
for r in entries:
    o = dict(path=r['path'], methods=r['methods'], boards=r['boards'], area=r['area'],
             request_fields=r['request_fields'], response_fields=r['response_fields'])
    if r['item_methods']:
        o['item_path'] = r['item_path']; o['item_methods'] = r['item_methods']
    if r['query_params']:
        o['query_params'] = r['query_params']
    if r['validation']:
        o['validation'] = r['validation']
    if r['options']:
        o['allowed_values'] = r['options']
    o['handler_present'] = r['handler']
    if r['har']:
        o['seen_in_live_capture'] = r['har']
    o['ui'] = r['ui']
    if r['js_modules']:
        o['js_modules'] = r['js_modules']
    if r['notes']:
        o['notes'] = r['notes']
    js.append(o)
json.dump(js, open(sys.argv[1], 'w'), indent=1, ensure_ascii=False)

# Markdown catalog
def short_ui(u):
    return u.replace('views/', '')
md = []
cur = None
for r in entries:
    if r['area'] != cur:
        cur = r['area']
        n = sum(1 for x in entries if x['area'] == cur)
        md.append('\n### %s (%d)\n' % (cur, n))
    meth = '/'.join(r['methods'])
    md.append('#### `%s %s`' % (meth, r['path']))
    bl = ', '.join(r['boards']) or '(none of the analysed images)'
    lines = ['- Boards (UI references): %s' % bl,
             '- Handler in firmware: ' + ', '.join('%s %s' % (k, v) for k, v in r['handler'].items())]
    if r['har']:
        lines.append('- Seen in live capture: ' + '; '.join('%s: %s' % (k, ', '.join(v)) for k, v in r['har'].items()))
    if r['item_methods']:
        lines.append('- Per-item: `%s %s` (Backbone collection model, `:id` = item `id`)' % ('/'.join(r['item_methods']), r['item_path']))
    if r['query_params']:
        lines.append('- Query: `?%s`' % '`, `?'.join(r['query_params']))
    if r['ui']:
        lines.append('- UI: ' + ', '.join('`%s`' % short_ui(u) for u in r['ui'][:8]) + (' …' if len(r['ui']) > 8 else ''))
    if r['js_modules']:
        lines.append('- Model: ' + ', '.join('`%s`' % u for u in r['js_modules']))
    if r['request_fields']:
        lines.append('- Request: ' + ', '.join('`%s`' % f for f in r['request_fields']))
    if r['response_fields']:
        lines.append('- Response (fields read by UI): ' + ', '.join('`%s`' % f for f in r['response_fields']))
    for k, v in r['options'].items():
        lines.append('  - `%s` values: %s' % (k, ', '.join('`%s`' % x for x in v[:30]) + (' …' if len(v) > 30 else '')))
    for k, v in r['validation'].items():
        if v:
            lines.append('  - `%s` validation: %s' % (k, v.replace('|', ' / ').replace('`', "'")))
    if r['notes']:
        lines.append('- Notes: ' + r['notes'])
    md.append('\n'.join(lines) + '\n')
open(sys.argv[2], 'w').write('\n'.join(md))

# diff table
def fam(r):
    s = set(x.split('-')[0] for x in r['boards'])
    return s
x_only = [r['path'] for r in entries if fam(r) == {'X470'}]
b_only = [r for r in entries if fam(r) == {'B650'}]
both = [r['path'] for r in entries if fam(r) == {'X470', 'B650'}]
d = []
d.append('| Category | Count |\n|---|---|')
d.append('| Both X470 and B650 | %d |' % len(both))
d.append('| X470 3.02 only | %d |' % len(x_only))
d.append('| B650 only (7.02 and 9.05) | %d |' % len([r for r in b_only if len(r['boards']) == 2]))
d.append('| B650 7.02 only | %d |' % len([r for r in b_only if r['boards'] == ['B650-7.02']]))
d.append('| B650 9.05 only | %d |' % len([r for r in b_only if r['boards'] == ['B650-9.05']]))
d.append('| Total distinct paths | %d |' % len(entries))
d.append('\n#### Present on X470 3.02 only (%d)\n' % len(x_only))
d.append('| Path | Area |\n|---|---|')
for r in entries:
    if fam(r) == {'X470'}:
        d.append('| `%s` | %s |' % (r['path'], r['area']))
d.append('\n#### Present on B650 only (%d)\n' % len(b_only))
d.append('| Path | Boards | Area |\n|---|---|---|')
for r in b_only:
    d.append('| `%s` | %s | %s |' % (r['path'], ', '.join(r['boards']), r['area']))
d.append('\n#### Present on both families (%d)\n' % len(both))
d.append('| Path | Boards | Area |\n|---|---|---|')
for r in entries:
    if fam(r) == {'X470', 'B650'}:
        d.append('| `%s` | %s | %s |' % (r['path'], ', '.join(r['boards']), r['area']))
open(sys.argv[3], 'w').write('\n'.join(d) + '\n')
print(len(entries), len(x_only), len(b_only), len(both))
