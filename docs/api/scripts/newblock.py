HAR = json.load(open('har_shapes.json'))
HARLBL = {'x470': 'x470d4u.har', 'b650': 'b650d4u.har', 'b650_2t': 'b650d4u-2t.har'}
HARDESC = {'x470': 'X470D4U fw 2.32.00', 'b650': 'B650D4U fw 1.10.00', 'b650_2t': 'B550D4U-2T fw 1.10.00'}
har_by_path = collections.defaultdict(dict)
har_obs = collections.defaultdict(dict)
for hb, calls in HAR.items():
    for k, v in calls.items():
        meth, path = k.split(' ')
        p = path.lstrip('/')
        if p.endswith('/:id'):
            p = p[:-4]
        for st in v['statuses']:
            har_by_path[p].setdefault(hb, []).append('%s %s' % (meth, st))
        o = dict(method=meth, status=v['statuses'], count=v['n'])
        if v['query']: o['query_keys'] = v['query']
        if v['req_ct']: o['request_content_type'] = v['req_ct']
        if v['req'] is not None: o['request_shape'] = v['req']
        if v['rsp'] is not None: o['response_shape'] = v['rsp']
        har_obs[p].setdefault(HARLBL[hb], []).append(o)
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

# handler presence: route strings in the shipped REST binaries (strings -n 4 of every *restservice* file in rootfs)
STR = {}
for b, _ in BOARDS:
    S = [l.rstrip('\n') for l in open('str_%s.txt' % b, encoding='utf-8', errors='replace')]
    exact = set(s.rstrip('/') for s in S if s.startswith('/'))
    bare = set(s.rstrip('/') for s in S if '/' in s and not s.startswith('/') and ' ' not in s)
    rx = []
    for s in S:
        if s.startswith('/') and ('(' in s or '[[' in s):
            try:
                import warnings; warnings.simplefilter('ignore')
                rx.append(re.compile('^' + s + '$'))
            except re.error:
                pass
    STR[b] = (exact, bare, rx)
HUNKNOWN = {'x470': {'api/settings/fan/fan_mode', 'api/settings/fan/fan_closeloopctb', 'api/settings/fan/fan_openloopctb', 'api/settings/fan/fan_tscft'}}
def handler(b, p):
    if p in HUNKNOWN.get(b, ()):
        return 'unknown'
    exact, bare, rx = STR[b]
    r = p[4:]
    q = re.sub(r'/:id$', '', r)
    if '/' + r in exact or '/' + q in exact or (('/' in q) and q in bare):
        return 'yes'
    if any(x.match('/' + r) or x.match('/' + q + '/1') for x in rx):
        return 'yes'
    return 'no'
