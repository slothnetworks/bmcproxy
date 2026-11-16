import json, sys, re, urllib.parse, collections
FILES = {'x470': 'x470d4u.har', 'b650': 'b650d4u.har', 'b650_2t': 'b650d4u-2t.har'}
D = '/opt/sparked/asrockrack-api/harfiles/'
def shape(v, depth=0):
    if isinstance(v, dict):
        if depth >= 2: return '{…}'
        return {k: shape(x, depth + 1) for k, x in v.items()}
    if isinstance(v, list):
        if not v: return []
        el = merge([shape(x, depth) for x in v])
        return [el] + (['…%d items' % len(v)] if len(v) > 1 else [])
    if isinstance(v, bool): return 'bool'
    if isinstance(v, int): return 'int'
    if isinstance(v, float): return 'float'
    if v is None: return 'null'
    return 'str'
def merge(items):
    if all(isinstance(i, dict) for i in items):
        o = {}
        for i in items:
            for k, t in i.items():
                if k not in o: o[k] = t
                elif o[k] != t and isinstance(o[k], str) and isinstance(t, str):
                    o[k] = '|'.join(sorted(set(o[k].split('|')) | set(t.split('|'))))
        return o
    ts = []
    for i in items:
        if i not in ts: ts.append(i)
    if all(isinstance(i, str) for i in ts):
        return '|'.join(sorted(set('|'.join(ts).split('|'))))
    return ts[0]
out = {}
for hb, f in FILES.items():
    h = json.load(open(D + f))
    calls = collections.OrderedDict()
    for e in h['log']['entries']:
        rq, rs = e['request'], e['response']
        u = urllib.parse.urlsplit(rq['url'])
        if not u.path.startswith('/api/'): continue
        path = u.path.rstrip('/')
        path = re.sub(r'/(\d+)$', '/:id', path)
        qk = sorted(urllib.parse.parse_qs(u.query, keep_blank_values=True).keys())
        key = '%s %s' % (rq['method'], path)
        c = calls.setdefault(key, dict(statuses=set(), query=set(), req_ct=set(), req=None, rsp_ct=set(), rsp=None, hdrs=set(), n=0))
        c['n'] += 1
        c['statuses'].add(rs['status'])
        if qk: c['query'].add('&'.join(qk))
        for hh in rq['headers']:
            n = hh['name'].lower()
            if n in ('x-csrftoken', 'cookie'): c['hdrs'].add(n)
        pd = rq.get('postData')
        if pd:
            ct = pd.get('mimeType', '')
            c['req_ct'].add(ct.split(';')[0])
            t = pd.get('text', '')
            if 'json' in ct or (t.startswith(('{', '['))):
                try: c['req'] = shape(json.loads(t))
                except Exception: c['req'] = '(non-JSON %d bytes)' % len(t)
            elif 'urlencoded' in ct:
                c['req'] = {k: 'form' for k in urllib.parse.parse_qs(t, keep_blank_values=True)}
            elif 'multipart' in ct:
                c['req'] = sorted(set(re.findall(r'name="([^"]+)"', t))) or '(multipart)'
            elif t:
                c['req'] = '(%s %d bytes)' % (ct, len(t))
        ct = rs.get('content', {}).get('mimeType', '')
        c['rsp_ct'].add(ct)
        t = rs.get('content', {}).get('text')
        if t and rs.get('content', {}).get('encoding') != 'base64':
            try:
                s = shape(json.loads(t))
                if c['rsp'] is None or rs['status'] < 300: c['rsp'] = s
            except Exception:
                if c['rsp'] is None: c['rsp'] = '(text %d bytes)' % len(t)
    for c in calls.values():
        for k in ('statuses', 'query', 'req_ct', 'rsp_ct', 'hdrs'): c[k] = sorted(c[k])
    out[hb] = calls
    print(hb, len(calls))
json.dump(out, open(sys.argv[1], 'w'), indent=1)
