import json,re,statistics,sys
from urllib.parse import urlsplit,parse_qsl
from datetime import datetime
H='/opt/sparked/asrockrack-api/harfiles/'
CAPS=[('b650d4u.har','B650D4U','1.10.00','2.17','harfiles/b650d4u.har'),('b650d4u-2t.har','B550D4U-2T','1.10.00','L1.02','harfiles/b650d4u-2t.har (filename says B650D4U-2T/2L2T; the board is a B550D4U-2T)')]
SECRETS={}
def collect(E):
    for e in E:
        pd=e['request'].get('postData') or {}
        for p in pd.get('params',[]) or []:
            if p['name']=='password': SECRETS[p['value']]='<REDACTED_PASSWORD>'
        if urlsplit(e['request']['url']).path=='/api/session' and e['response']['status']==200:
            d=json.loads(e['response']['content']['text']); SECRETS[d['CSRFToken']]='<REDACTED_CSRF>'
MAC=re.compile(r'\b[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}\b')
def red_s(s):
    for k,v in SECRETS.items(): s=s.replace(k,v)
    s=MAC.sub('<REDACTED_MAC>',s)
    s=re.sub(r'AMI[0-9A-F]{12}','<REDACTED_HOSTNAME>',s)
    s=re.sub(r'tos_\d+_\d+','<REDACTED_USER>',s)
    s=s.replace('82072141930','<REDACTED_ID>')
    return s
SERIALKEYS={'SerialNumber','serial_number','pws_serial_number','UUID'}
def red(o,key=None):
    if isinstance(o,dict): return {k:red(v,k) for k,v in o.items()}
    if isinstance(o,list): return [red(v) for v in o]
    if key=='racsession_id': return '<REDACTED_SESSION_ID>'
    if key in SERIALKEYS and isinstance(o,str) and o.strip() and o not in ('NA','N/A','Not Available'): return '<REDACTED_SERIAL>'
    if isinstance(o,str): return red_s(o)
    return o
COMMON={'accept-encoding','accept-language','connection','host','sec-fetch-dest','sec-fetch-mode','sec-fetch-site','sec-gpc','user-agent','sec-ch-ua','sec-ch-ua-mobile','sec-ch-ua-platform','content-length'}
NOTES={
 ('B650D4U','GET','/api/settings/services'):'entry 6 = 401 {"cc":7,"error":"Invalid Authentication"} sent BEFORE login with header X-CSRFTOKEN: null (UI boot probe, see har-analysis.md 4.6); later calls 200',
 ('B550D4U-2T','GET','/api/settings/services'):'entry 11 = 401 {"cc":7,"error":"Invalid Authentication"} sent BEFORE login with header X-CSRFTOKEN: null (UI boot probe); later calls 200',
 ('B650D4U','PUT','/api/settings/network-bond'):'WRITE: disables bonding and pins the BMC to eth0; BMC network services restart. Last entry of the capture.',
 ('B650D4U','POST','/api/asrr/settings/setopenlooptablenumber'):'selector, not a fan write: picks which of 8 open-loop tables later get/set calls address; bit7 (0x80) selects the factory-default copy. Not in the analysed B650 7.02/9.05 images (present in older/other firmware only)',
 ('B650D4U','GET','/api/asrr/settings/getfanopenloopcontroltable'):'24 (temp,duty) pairs for the selected table; 255 = empty slot. Not in the analysed B650 7.02/9.05 images (present in older/other firmware only)',
 ('B650D4U','GET','/api/asrr/getvetobit'):'Keep shared-NIC (NC-SI) PHY link up across host reset. Not in the analysed B650 7.02/9.05 images (present in older/other firmware only)',
 ('B550D4U-2T','GET','/api/asrr/maintenance/BIOS/UpdateOption'):'Called on every route render (not a timer); action 0 = no pending BIOS update, 1 = start polling BIOS/status after 10 s. Not in any analysed firmware image (present in older/other firmware only)',
 ('B550D4U-2T','GET','/api/logs/eventlog'):'paged: EVENTID=<id-1> (-1 = newest page, ~300 records/page); last array element is {"event_log_count": N}. Example truncated to 2 records + count',
 ('B550D4U-2T','GET','/api/settings/users'):'30 rows = 15 user slots x 2 LAN channels (1 and 8); example truncated to the 3 populated slots on channel 1',
 ('B650D4U','GET','/api/sensors'):'59 records; example truncated to one per type/kind. Thresholds are "NA" strings when unset',
 ('B550D4U-2T','GET','/api/sensors'):'51 records; example truncated to one per type/kind',
 ('B550D4U-2T','GET','/api/sdr'):'58 records; example truncated',
 ('B650D4U','GET','/api/dashboardsensors'):'59 records: 58 stubs {id,sensor_number,name} + full record only for sensors not in normal state (FAN7_1 here); example truncated',
 ('B650D4U','GET','/api/logs/dashboardevent'):'61 stub records + trailer {"pending_deassert":N,"dashboard_event_log_count":0}; truncated',
 ('B550D4U-2T','GET','/api/logs/dashboardevent'):'61 stub records + trailer; truncated',
 ('B550D4U-2T','GET','/api/logs/audit'):'29 syslog-style records; truncated to 3',
 ('B550D4U-2T','GET','/api/configuration/project'):'83 entries (80 unique); full list in har-analysis.md',
 ('B650D4U','GET','/api/host_inventory/host_interface_power_info'):'15 voltage entries; truncated to 2. Thresholds only, no live readings',
 ('B550D4U-2T','GET','/api/host_inventory/host_interface_memory_info'):'4 DIMMs; truncated to 1',
 ('B650D4U','GET','/api/host_inventory/host_interface_baseboard_info'):'NetworkInterface truncated to usb0 + bond0',
}
def trunc(board,m,p,d):
    k=(board,m,p)
    if not isinstance(d,list): 
        if p.endswith('baseboard_info'):
            d=json.loads(json.dumps(d)); d[0]['NetworkInterface']=[d[0]['NetworkInterface'][0],d[0]['NetworkInterface'][3]]
        return d
    if p=='/api/sensors':
        seen=set();out=[]
        for r in d:
            kk=(r['type'],r['accessible'],r['discrete_state']!=0)
            if kk not in seen: seen.add(kk);out.append(r)
        return out
    if p=='/api/logs/eventlog': return d[:2]+[d[-1]]
    if p=='/api/settings/users': return [r for r in d if r['name'] and r['channel']==1]
    if p in('/api/sdr',): return d[:3]+[r for r in d if r['type'] not in('voltage','fan','temperature','current','power_supply')][:3]
    if p=='/api/dashboardsensors': return d[:2]+[r for r in d if len(r)>3]
    if p=='/api/logs/dashboardevent': return d[:2]+[d[-1]]
    if p=='/api/logs/audit': return d[:3]
    if p=='/api/configuration/project': return d[:6]
    if p.endswith('power_info'):
        d=json.loads(json.dumps(d)); d[0]['Voltages']=d[0]['Voltages'][:1]+[v for v in d[0]['Voltages'] if v['State']=='Absent'][:1]; return d
    if p.endswith('memory_info'): return d[:1]
    return d
def ts(e): return datetime.fromisoformat(e['startedDateTime'].replace('Z','+00:00'))
out={'source':'harfiles/b650d4u.har (63 entries, 2026-11-16T06:27:45Z..06:28:56Z) and harfiles/b650d4u-2t.har (73 entries, 2026-11-16T06:27:36Z..06:29:59Z); Chromium/Brave 157 WebInspector HAR 1.2',
 'bmc':[],'redaction':'Passwords, CSRF tokens, session ids, serial numbers (board/DIMM/NVMe), MACs, BMC hostnames (AMI+MAC), an 11-digit BMC Model id and a non-stock username replaced with <REDACTED_*> placeholders. RFC1918 IPs kept. See har-analysis.md section 15.',
 'auth':{'login':'POST /api/session (application/x-www-form-urlencoded: username, password; no certlogin field)','csrf_header':'X-CSRFTOKEN: <CSRFToken from login response> (UI sends the literal string "null" before login)','session_cookie':'QSESSIONID (name from UI JS; Cookie/Set-Cookie stripped from both HARs). B650D4U UI also keeps the CSRF token in a JS cookie __Host-garc','logout':'DELETE /api/session via fetch(keepalive) with X-CSRFTOKEN (from UI JS; not exercised)','unauthenticated_error':'HTTP 401 {"cc": 7, "error": "Invalid Authentication"}'},
 'endpoints':[]}
for fn,board,fw,bios,src in CAPS:
    E=json.load(open(H+fn))['log']['entries']; collect(E)
    out['bmc'].append({'board':board,'capture':src,'bmc_fw':fw,'bios':bios,'server':'lighttpd','http':'HTTP/1.1 over TLS','host':'<BMC_HOST> (RFC1918 address in capture)'})
    g={}
    for i,e in enumerate(E):
        p=urlsplit(e['request']['url']).path
        if not p.startswith('/api'): continue
        g.setdefault((e['request']['method'],p),[]).append((i,e))
    for (m,p),v in g.items():
        w=[e['timings']['wait'] for i,e in v]; t=[e['time'] for i,e in v]
        st={}
        for i,e in v: st[str(e['response']['status'])]=st.get(str(e['response']['status']),0)+1
        tt=[ts(e) for i,e in v]
        okv=[(i,e) for i,e in v if e['response']['status']==200] or v
        i0,e0=okv[0]
        hd={x['name']:x['value'] for x in e0['request']['headers'] if x['name'].lower() not in COMMON}
        hd=red(hd)
        if "Origin" in hd: hd["Origin"]="https://<BMC_HOST>"
        pd=e0['request'].get('postData')
        body=None
        if pd:
            txt=pd.get('text','')
            if 'json' in pd.get('mimeType',''): body=red(json.loads(txt))
            else: body=red(dict(parse_qsl(txt)))
        q=[]
        for i,e in v:
            qs=urlsplit(e['request']['url']).query
            if qs:
                d=red(dict(parse_qsl(qs)))
                if d not in q: q.append(d)
        txt=e0['response']['content'].get('text') or ''
        try: resp=json.loads(txt)
        except Exception: resp=txt
        resp=red(trunc(board,m,p,resp))
        out['endpoints'].append({'board':board,'method':m,'path':p,'query':q,'calls':len(v),'har_entry_indexes':[i for i,e in v],'status':st,
          'response_content_type':e0['response']['content'].get('mimeType'),'response_body_bytes_first':len(txt),
          'timing_ms':{'wait_min':round(min(w)),'wait_median':round(statistics.median(w)),'wait_max':round(max(w)),'total_min':round(min(t)),'total_median':round(statistics.median(t)),'total_max':round(max(t))},
          'intervals_s':[round((b-a).total_seconds(),2) for a,b in zip(tt,tt[1:])],
          'request_example':{'headers':hd,'body':body},'response_example':resp,'note':NOTES.get((board,m,p))})
s=json.dumps(out,indent=2,ensure_ascii=False)
# final secret scrub sanity
for k in SECRETS: assert k not in s, k
open('/opt/sparked/asrockrack-api/docs/b650/har-endpoints.json','w').write(s+'\n')
print(len(out['endpoints']),len(s))
