import json, re, sys
N = sys.argv[1]
d = json.load(open('/opt/sparked/asrockrack-api/docs/api/legacy-api-endpoints.json'))
B = {'x470': 'X470-3.02', 'b650_702': 'B650-7.02', 'b650_905': 'B650-9.05'}
for t, lbl in B.items():
    S = [l.rstrip('\n') for l in open('%s/str_%s.txt' % (N, t))]
    exact = set(s.rstrip('/') for s in S if s.startswith('/'))
    rx = []
    for s in S:
        if s.startswith('/') and ('(' in s or '[' in s):
            try: rx.append(re.compile('^' + s + '$'))
            except re.error: pass
    dis = []
    for e in d:
        p = e['path'][4:]
        q = re.sub(r'/:id$', '', p)
        hit = p in exact or q in exact or any(r.match(p) for r in rx)
        cur = e['handler_present'][lbl]
        if (hit and cur != 'yes') or (not hit and cur == 'yes'):
            dis.append((e['path'], cur, hit, lbl in e['boards']))
    print('==', lbl, len(dis))
    for x in dis: print('  ', x)
