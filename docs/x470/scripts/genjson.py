import json,sys,base64,collections,datetime,statistics,urllib.parse,re
h=json.load(open(sys.argv[1]));out=sys.argv[2]
SECRETS={}
MAC=re.compile(r'([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}')
def red(o,k=None):
    if isinstance(o,dict): return {kk:red(v,kk) for kk,v in o.items()}
    if isinstance(o,list): return [red(v) for v in o]
    if k=='CSRFToken' and o: return '<REDACTED_CSRF>'
    if k=='password' and o: return '<REDACTED_PASSWORD>'
    if k=='racsession_id': return '<REDACTED_SESSION_ID>'
    if k in ('serial_number','pws_serial_number') and o and o!='N/A': return '<REDACTED_SERIAL>'
    if isinstance(o,str) and MAC.fullmatch(o): return '<REDACTED_MAC>'
    return o
def body(e):
    c=e['response']['content'];x=c.get('text','')
    if c.get('encoding')=='base64': x=base64.b64decode(x).decode()
    try: return json.loads(x)
    except Exception: return x
def ts(e): return datetime.datetime.fromisoformat(e['startedDateTime'].replace('Z','+00:00')).timestamp()
KEEP_REQ={'content-type','x-csrftoken','x-requested-with','accept','origin','referer'}
g=collections.OrderedDict()
for i,e in enumerate(h['log']['entries']):
    u=urllib.parse.urlsplit(e['request']['url'])
    if not u.path.startswith('/api/') or u.hostname!='10.0.0.87': continue
    g.setdefault((e['request']['method'],u.path),[]).append((i,e))
res=[]
for (m,p),v in g.items():
    i,e=v[0]
    t=[ts(x) for _,x in v]
    iv=[round(b-a,3) for a,b in zip(t,t[1:])]
    w=[x['timings']['wait'] for _,x in v]; tot=[x['time'] for _,x in v]
    hdr={x['name']:('<REDACTED_CSRF>' if x['name'].lower()=='x-csrftoken' else x['value']) for x in e['request']['headers'] if x['name'].lower() in KEEP_REQ}
    hdr={k:v2.replace('10.0.0.87','<BMC_HOST>') for k,v2 in hdr.items()}
    pd=e['request'].get('postData')
    reqbody=None
    if pd:
        if 'urlencoded' in pd['mimeType']:
            reqbody={x['name']:('<REDACTED_PASSWORD>' if x['name']=='password' else x['value']) for x in pd['params']}
        else: reqbody=json.loads(pd['text'])
    b=red(body(e)); note=None
    if p=='/api/logs/eventlog':
        full=b; seen=set(); smp=[]
        for x in full:
            k=(x['record_type'],x['sensor_type'],x['event_description'])
            if k not in seen: seen.add(k); smp.append(x)
        b=smp; note='truncated: one example per (record_type,sensor_type,event_description); full response had %d records'%len(full)
    if p=='/api/sensors':
        b=red(body(h['log']['entries'][150])); note='example taken from entry 150 (host powered on); earlier samples (host off) show accessible=213 and zero readings for host-powered sensors'
    if p=='/api/settings/static-ipv6':
        note='truncated to 2 of %d entries (16 per interface, all empty)'%len(b); b=b[:2]
    if p=='/api/asrr/PowerSource':
        note='truncated to 1 of %d identical slot entries'%len(b); b=b[:1]
    st=collections.Counter(x['response']['status'] for _,x in v)
    res.append(collections.OrderedDict(
        method=m,path=p,query=e['request']['queryString'],calls=len(v),
        har_entry_indexes=[x for x,_ in v],
        status=dict(st),
        response_content_type=[x['value'] for x in e['response']['headers'] if x['name'].lower()=='content-type'][0],
        response_body_bytes_first=e['response']['content'].get('size'),
        timing_ms=dict(wait_min=round(min(w)),wait_median=round(statistics.median(w)),wait_max=round(max(w)),total_min=round(min(tot)),total_median=round(statistics.median(tot)),total_max=round(max(tot))),
        intervals_s=iv,
        request_example=dict(headers=hdr,body=reqbody),
        response_example=b,
        note=note))
doc=collections.OrderedDict(
  source='harfiles/x470d4u.har (Chrome/Brave WebInspector HAR 1.2, 176 entries, 2026-11-16T05:58:44Z..06:02:38Z)',
  bmc=dict(board='X470D4U',bmc_fw='2.32.00 (built May 11 2021)',bios='P4.20',server='lighttpd',http='HTTP/1.1 over TLS',host='<BMC_HOST> (RFC1918 address in capture)'),
  redaction='Passwords, CSRF tokens, session ids, serial numbers and MAC addresses replaced with <REDACTED_*> placeholders. See har-analysis.md for formats/lengths.',
  auth=dict(login='POST /api/session (application/x-www-form-urlencoded: username, password, certlogin=0)',
            csrf_header='X-CSRFTOKEN: <CSRFToken from login response>',
            session_cookie='QSESSIONID (name known from web UI JS; cookie values/attributes stripped from this HAR export)',
            logout='DELETE /api/session (from web UI JS; not exercised in capture)'),
  endpoints=res)
json.dump(doc,open(out,'w'),indent=2,ensure_ascii=False)
s=open(out).read()
for bad in ['l7psK4jX','4p58kGrfrpWedwTQ','217943410000474','D0:50:99','agilebits']:
    assert bad not in s,bad
print(len(res),'endpoints',len(s),'bytes')
