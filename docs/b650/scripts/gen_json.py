"""Generate docs/b650/har-endpoints.json from the two B650-family HAR captures (read-only, redacted)."""
import json, re, sys, collections, statistics
from urllib.parse import urlsplit, parse_qsl
from datetime import datetime

H = '/opt/sparked/asrockrack-api/harfiles/'
OUT = sys.argv[1]

CAPTURES = [
    ('b650d4u.har', 'B650D4U'),
    ('b650d4u-2t.har', 'B550D4U-2T'),
]
KEEP_REQ = ['Accept', 'Content-Type', 'Origin', 'Referer', 'X-CSRFTOKEN', 'X-Requested-With']

MAC = re.compile(r'\b[0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5}\b')
AMIHOST = re.compile(r'\bAMI[0-9A-F]{12}\b')
SERIAL_KEYS = {'SerialNumber', 'pws_serial_number', 'serial_number'}
# values collected from the captures at runtime (never written out)
SECRETS = {}


def ts(s):
    return datetime.strptime(s.replace('Z', '+0000'), '%Y-%m-%dT%H:%M:%S.%f%z').timestamp()


def red_str(s):
    if not isinstance(s, str):
        return s
    for v, ph in SECRETS.items():
        if v and v in s:
            s = s.replace(v, ph)
    s = MAC.sub('<REDACTED_MAC>', s)
    s = AMIHOST.sub('<REDACTED_HOSTNAME>', s)
    return s


def red(o, key=None):
    if isinstance(o, dict):
        out = {}
        for k, v in o.items():
            if k == 'racsession_id':
                out[k] = '<REDACTED_SESSION_ID>'
            elif k == 'CSRFToken':
                out[k] = '<REDACTED_CSRF>'
            elif k == 'password' and v not in ('', None):
                out[k] = '<REDACTED_PASSWORD>'
            elif k in SERIAL_KEYS and isinstance(v, str) and v.strip() and v not in ('Not Available', 'N/A', 'NA'):
                out[k] = '<REDACTED_SERIAL>'
            elif k == 'Model' and isinstance(v, str) and re.fullmatch(r'\d{8,}', v):
                out[k] = '<REDACTED_SERIAL>'
            elif k == 'hostname':
                out[k] = '<REDACTED_HOSTNAME>'
            else:
                out[k] = red(v, k)
        return out
    if isinstance(o, list):
        return [red(x) for x in o]
    return red_str(o)


def body(e):
    t = e['response']['content'].get('text') or ''
    try:
        return json.loads(t)
    except Exception:
        return t if t else None


def reqbody(e):
    pd = e['request'].get('postData')
    if not pd:
        return None
    t = pd.get('text') or ''
    if 'urlencoded' in (pd.get('mimeType') or ''):
        return dict(parse_qsl(t, keep_blank_values=True))
    try:
        return json.loads(t)
    except Exception:
        return t


def collect_secrets(E, host):
    for e in E:
        p = urlsplit(e['request']['url']).path
        if p == '/api/session' and e['request']['method'] == 'POST':
            b = body(e)
            SECRETS[b['CSRFToken']] = '<REDACTED_CSRF>'
            pw = dict(parse_qsl(e['request']['postData']['text']))['password']
            SECRETS[pw] = '<REDACTED_PASSWORD>'
    SECRETS['https://' + host] = 'https://<BMC_HOST>'
    SECRETS['tos_1325_627'] = '<REDACTED_USER>'


def truncate(path, b):
    """Return (example, truncation note)."""
    if not isinstance(b, list):
        return b, None
    n = len(b)
    if path == '/api/logs/eventlog':
        seen, ex = set(), []
        for r in b:
            k = (r.get('record_type'), r.get('sensor_type'), r.get('event_description'))
            if 'id' in r and k not in seen:
                seen.add(k)
                ex.append(r)
        ex.append(b[-1])
        return ex, 'truncated: one record per (record_type,sensor_type,event_description) plus the trailing count element; full response had %d elements (%d records + {"event_log_count":N})' % (n, n - 1)
    if path == '/api/logs/dashboardevent':
        return b[:3] + b[-1:], 'truncated to first 3 records plus the trailing summary element; full response had %d elements' % n
    if path == '/api/logs/audit' and n > 3:
        return b[:3], 'truncated to 3 of %d records' % n
    if path == '/api/settings/users':
        ex = [r for r in b if r['name']][:3] + [r for r in b if not r['name']][:1]
        return ex, 'truncated: named users on channel 1 plus one empty slot; full response had %d entries (15 user ids x 2 LAN channels 1 and 8)' % n
    return b, None


NOTES = {
    ('B650D4U', 'GET', '/api/settings/services'): '1st call (entry 6) was made by the SPA app.initialize() before login with X-CSRFTOKEN: "null" and returned HTTP 401 {"cc":7,"error":"Invalid Authentication"}; the 2nd (after login) and 3rd returned 200. maximum_sessions/active_session are encoded: value & 0x7F, 255 = N/A [JS]. web time_out (1800 s) drives the UI idle-logout timer.',
    ('B550D4U-2T', 'GET', '/api/settings/services'): 'Same pre-login 401 pattern as B650D4U (entry 11, 97 s before the 200 at entry 20). Body identical to the B650D4U capture.',
    ('B650D4U', 'POST', '/api/session'): 'No certlogin field (X470 sent certlogin=0). Sent with X-CSRFTOKEN: "null" left over from the pre-login $.ajaxSetup. Response adds user_id, channel, passwordStatus vs X470.',
    ('B550D4U-2T', 'POST', '/api/session'): '3509 ms server wait (B650D4U: 360 ms; X470: 1491 ms).',
    ('B650D4U', 'GET', '/api/settings/dashboardinfo'): 'Consolidated post-login call: IPMI Get Device ID fields (same as X470 /api/firmware-info) + user record + dashboard NTP/timezone + power-on-hours + audit count. GET with JSON Content-Type and a query string built by jQuery from {user_id, channel_id, user_name}.',
    ('B650D4U', 'GET', '/api/chassis-status'): 'B650 UI polls chassis-status (not /api/chassis) on a 5 s timer. Extra field power_cycle_interval (ms) is used by the Power Control page to time the post-cycle re-check (interval + 10 s) [JS].',
    ('B550D4U-2T', 'GET', '/api/chassis-status'): 'No power_cycle_interval field on this board (same shape as X470).',
    ('B650D4U', 'GET', '/api/dashboardsensors'): 'Same order and sensor_number set as /api/sensors, but only {id,sensor_number,name} per entry, except sensors that are not in a normal state, which carry the full /api/sensors record (here FAN7_1, sensor_state 8 = Lower Non-Critical). The UI treats entries with owner_id as critical [JS].',
    ('B650D4U', 'GET', '/api/logs/audit'): 'cmd=1 returns only the audit record count [JS comment "audit count only"].',
    ('B550D4U-2T', 'GET', '/api/logs/audit'): 'Full audit log (syslog-style login/logout lines from spx_restservice and adviserd); hostname field is AMI+MAC.',
    ('B650D4U', 'GET', '/api/sensors'): 'Field renamed settable_readable_threshMask -> settable_flag; unset thresholds are the string "NA" (X470: 0.0); integral values serialise as JSON ints; unit "deg_c" (not degree sign). One active warning: FAN7_1 sensor_state 8 with accessible 0.',
    ('B550D4U-2T', 'GET', '/api/sensors'): 'Same shape as B650D4U (settable_flag, "NA" thresholds) but unit is "°C" and sensor numbering/naming follows the AM4 (X470-like) scheme.',
    ('B550D4U-2T', 'GET', '/api/asrr/maintenance/BIOS/UpdateOption'): 'Called from postSwitch() on every SPA route change (not a timer) [JS]. action 0 = no deferred BIOS flash pending; action 1 = flash armed for next host shutdown, in which case the UI chains GET /api/asrr/maintenance/BIOS/status (10 s, then 15 s) and flash-progress. Upload page radio values: 1 flash at next manual shutdown, 2 flash on the fly, 3 shut host down and flash.',
    ('B650D4U', 'POST', '/api/asrr/settings/setopenlooptablenumber'): 'Selects which open-loop table the next getfanopenloopcontroltable returns (server-side cursor). table_numble 1..8 = customized table N; N|0x80 (129..136) = factory default table N [JS]. The page sends {"table_numble":1} unconditionally in afterRender, so merely opening the page issues this POST. Response body is "{ }".',
    ('B650D4U', 'GET', '/api/asrr/settings/getfanopenloopcontroltable'): '24 (temperature degC, duty %) points of the table selected by the preceding setopenlooptablenumber. 255 = unused point. All 255 here = customized table 1 is empty.',
    ('B650D4U', 'GET', '/api/asrr/getvetobit'): '"Keep Share NIC Link Up" setting: vetobit 1 keeps the NC-SI shared-NIC PHY linked up across host resets. Write: PUT /api/asrr/setvetobit {"vetobit":0|1} [JS, not captured].',
    ('B650D4U', 'PUT', '/api/settings/network-bond'): 'User disabled bonding (bond_enable 1 -> 0). This restarts the BMC network stack; it is the last entry in the capture. bond_ifc is not returned by GET; the UI sends the <select> value (default first option "eth0").',
    ('B650D4U', 'GET', '/api/maintenance/system_admin'): 'Factory uid-0 shell account (sysadmin): enable_user_access 1 means the account is enabled.',
    ('B650D4U', 'GET', '/api/host_inventory/host_interface_baseboard_info'): 'BMC self-description plus BMC NICs (usb0 host interface 169.254.0.17, eth0, eth1, bond0). All values are strings, "NA" for unknown. Second call returned Connection: close.',
    ('B650D4U', 'GET', '/api/host_inventory/host_interface_power_info'): 'Powercontrol (power limit 500 W, HardPowerOff exception, consumption 0) and 15 voltage sensors with thresholds as strings ("NA" if unset); State "Absent" for PSU sensors.',
    ('B550D4U-2T', 'GET', '/api/host_inventory/host_interface_storage_info'): 'Populated on this board (X470: null/null). Key "Storage_Contoller" is still misspelled.',
    ('B550D4U-2T', 'GET', '/api/logs/eventlog'): 'EVENTID=-1 (UI sends EventId-1 with EventId=0) returned the whole SEL newest first; the final array element is {"event_log_count":N}, not a record.',
    ('B550D4U-2T', 'GET', '/api/sdr'): '58 entries: the 51 sensors plus SEL(207), DRAM ECC ErrorA1/A2/B1/B2 (208-211), PEF(232), Timestamp(233). Adds owner_lun vs X470.',
    ('B550D4U-2T', 'GET', '/api/asrr/getbioscode'): 'Last BIOS POST code. poststatus 1 = valid; postdata is 16-bit (feature BIOS_CODE_2BYTES: UI renders 4 hex digits, here 0x0CAA).',
    ('B550D4U-2T', 'GET', '/api/settings/users'): '15 user ids x 2 LAN channels (1, 8). New per-channel fields vs X470: channel, channel_type, userid, accessByChannel, privilegeByChannel, password_age, password_enddate; network_privilege renamed privilege; privilege_limit_serial dropped.',
}


def main():
    out = {
        'source': [
            'harfiles/b650d4u.har (Brave 157/Chromium WebInspector HAR 1.2, 63 entries, 2026-11-16T06:27:45Z..06:28:57Z)',
            'harfiles/b650d4u-2t.har (Brave 157/Chromium WebInspector HAR 1.2, 73 entries, 2026-11-16T06:27:36Z..06:29:59Z)',
        ],
        'bmc': [
            {'capture': 'b650d4u.har', 'board': 'B650D4U', 'bmc_fw': '1.10.00 (built Mar 16 2023 03:32:09 UTC)', 'bios': '2.17', 'psp': '0.2B.0.4E', 'microcode': '0a601203', 'ipmi_prod_id': 8207, 'server': 'lighttpd', 'http': 'HTTP/1.1 over TLS', 'host': '<BMC_HOST> (RFC1918 address in capture)'},
            {'capture': 'b650d4u-2t.har', 'board': 'B550D4U-2T', 'board_note': 'captured as "B650D4U_2L2T" but /api/asrr/model-name returns mb_name "B550D4U-2T"; Ryzen 7 5700X, DDR4, sensor "B550 Temp" (an AM4 board)', 'bmc_fw': '1.10.00', 'bios': 'L1.02', 'psp': '0.14.0.3E', 'microcode': '0a20120e', 'server': 'lighttpd', 'http': 'HTTP/1.1 over TLS', 'host': '<BMC_HOST> (RFC1918 address in capture)'},
        ],
        'redaction': 'Passwords, CSRF tokens, session ids, serial numbers (board/DIMM/NVMe), MAC addresses, AMI<MAC> hostnames and a non-stock audit-log username replaced with <REDACTED_*> placeholders; BMC host in Origin headers replaced with <BMC_HOST>. RFC1918 addresses kept. See har-analysis.md section 15 for formats/lengths.',
        'auth': {
            'login': 'POST /api/session (application/x-www-form-urlencoded: username, password; no certlogin field)',
            'login_response_extra_vs_x470': ['user_id', 'channel', 'passwordStatus'],
            'csrf_header': 'X-CSRFTOKEN: <CSRFToken from login response> (8 chars [A-Za-z0-9]); pre-login requests carried the literal string "null"',
            'session_cookie': 'QSESSIONID (name from web UI JS; Cookie/Set-Cookie stripped from both HAR exports)',
            'unauthenticated_error': 'HTTP 401 {"cc": 7, "error": "Invalid Authentication"}',
            'logout': 'DELETE /api/session (from web UI JS; not exercised in either capture)',
        },
        'endpoints': [],
    }
    for fname, board in CAPTURES:
        E = json.load(open(H + fname))['log']['entries']
        host = urlsplit(E[0]['request']['url']).netloc
        collect_secrets(E, host)
        groups = collections.OrderedDict()
        for i, e in enumerate(E):
            u = urlsplit(e['request']['url'])
            if not u.path.startswith('/api'):
                continue
            groups.setdefault((e['request']['method'], u.path), []).append(i)
        for (m, p), idx in groups.items():
            es = [E[i] for i in idx]
            ok = [e for e in es if e['response']['status'] == 200] or es
            ex = ok[-1] if p == '/api/sensors' else ok[0]
            hdrs = {h['name']: h['value'] for h in ex['request']['headers'] if h['name'] in KEEP_REQ}
            waits = sorted(e['timings']['wait'] for e in es)
            tots = sorted(e['time'] for e in es)
            t0 = [ts(e['startedDateTime']) for e in es]
            b = body(ex)
            b, tn = truncate(p, b)
            note = NOTES.get((board, m, p))
            if tn:
                note = (note + ' ' if note else '') + '[' + tn + ']'
            qs = sorted(set(urlsplit(e['request']['url']).query for e in es) - {''})
            query = []
            for q in qs:
                query.append(red(dict(parse_qsl(q, keep_blank_values=True))))
            out['endpoints'].append({
                'board': board,
                'capture': fname,
                'method': m,
                'path': p,
                'query': query,
                'calls': len(es),
                'har_entry_indexes': idx,
                'status': {str(k): v for k, v in collections.Counter(e['response']['status'] for e in es).items()},
                'response_content_type': ex['response']['content'].get('mimeType'),
                'response_body_bytes_first': len(es[0]['response']['content'].get('text') or ''),
                'timing_ms': {
                    'wait_min': round(waits[0]), 'wait_median': round(statistics.median(waits)), 'wait_max': round(waits[-1]),
                    'total_min': round(tots[0]), 'total_median': round(statistics.median(tots)), 'total_max': round(tots[-1]),
                },
                'intervals_s': [round(b2 - a, 3) for a, b2 in zip(t0, t0[1:])],
                'request_example': {'headers': red(hdrs), 'body': red(reqbody(ex))},
                'response_example': red(b),
                'note': note,
            })
    s = json.dumps(out, indent=1, ensure_ascii=False)
    # final safety: no collected secret may remain
    for v in SECRETS:
        assert v not in s, 'secret leaked'
    open(OUT, 'w').write(s + '\n')
    print('wrote', OUT, len(out['endpoints']), 'endpoints', len(s), 'bytes')


main()
