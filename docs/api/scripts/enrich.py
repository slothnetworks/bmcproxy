import re, sys, os, json, glob, collections
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from extract import scan_strings, mask, enclosing, topkeys  # noqa

board = sys.argv[1]
d = 'mods_' + board
ex = json.load(open('ex_%s.json' % board))

mods = {}
deps = {}
argmap = {}
HDR = re.compile(r'^[A-Za-z_$]{1,2}\("([^"]+)",\[([^\]]*)\],function\(([^)]*)\)')
for f in glob.glob(os.path.join(d, '*.js')):
    if f.endswith('.pretty.js'):
        continue
    s = open(f, encoding='utf-8', errors='replace').read()
    m = HDR.match(s)
    if not m:
        continue
    name = m.group(1)
    dl = re.findall(r'"([^"]*)"', m.group(2))
    al = [a.strip() for a in m.group(3).split(',')] if m.group(3).strip() else []
    # strip trailing i18n/string junk: cut at '}),X("strings/' etc
    cut = re.search(r'\}\),[A-Za-z_$]{1,2}\("(?:strings|i18n|text!)', s)
    if cut:
        s = s[:cut.start() + 2]
    mods[name] = s
    deps[name] = dl
    argmap[name] = dict(zip(al, dl))

def is_data(n):
    return n.startswith('models') or n.startswith('collection')

# url per model/collection
urls = collections.defaultdict(set)
DEFKEYS = {'defaults', 'validation', 'model', 'url', 'urlRoot', 'idAttribute', 'parse', 'initialize'}
def is_def(e):
    k = set(e.get('keys', []))
    return bool(k & {'defaults', 'validation', 'model', 'urlRoot', 'idAttribute'}) or (k <= DEFKEYS and 'type' not in k) or k == set()
for e in ex:
    nm = e['module']
    if (nm.startswith('models') or nm.startswith('collection')) and is_def(e):
        urls[nm].add(re.sub(r'^/', '', e['url']))

fname2mod = {}
for n in mods:
    fname2mod[n.replace('/', '__').replace('!', '_')] = n

def mod_urls(n):
    fn = n.replace('/', '__').replace('!', '_')
    return urls.get(fn, set())

GET = re.compile(r'\.get\("([A-Za-z0-9_\-]+)"\)')
SET1 = re.compile(r'\.set\("([A-Za-z0-9_\-]+)",')
def objkeys_after(s, m, pat):
    out = []
    for mm in re.finditer(pat, s):
        p = mm.end()
        if p + 1 < len(s) and s[p] == '{' and s[p + 1] != '}':
            e2 = enclosing(m, p + 1)
            if e2:
                out += topkeys(m, s, e2[0], e2[1])
    return out

info = {}
for n, s in mods.items():
    m = mask(s)
    gi = sorted(set(GET.findall(s)) | set(re.findall(r'"change:([A-Za-z0-9_]+)"', s)))
    si = sorted(set(SET1.findall(s)) | set(objkeys_after(s, m, r'\.set\(')))
    sv = sorted(set(objkeys_after(s, m, r'\.save\(')))
    defaults = objkeys_after(s, m, r'\bdefaults:')
    validation = objkeys_after(s, m, r'\bvalidation:')
    ops = sorted(set(re.findall(r'\.(save|destroy|fetch|create)\(', s)))
    sd = re.findall(r'JSON\.stringify\(\{', s)
    tmpl = [x for x in deps.get(n, []) if x.startswith('text!')]
    info[n] = dict(get=gi, set=si, save=sv, defaults=defaults, validation=validation, ops=ops, templates=tmpl)

# template ids
tmpl_ids = {}
for f in glob.glob(os.path.join(d, 'text_templates__*.js')):
    if f.endswith('.pretty.js'):
        continue
    s = open(f, encoding='utf-8', errors='replace').read()
    m = HDR.match(s) or re.match(r'^[A-Za-z_$]{1,2}\("([^"]+)"', s)
    ids = sorted(set(re.findall(r'id=\\?["\']id([A-Za-z0-9_]+)', s)))
    tmpl_ids[m.group(1)] = ids

# parents: view -> views requiring it
parents = collections.defaultdict(set)
for n, dl in deps.items():
    for x in dl:
        parents[x].add(n)

# collection -> model dep
coll_model = {}
for n, s in mods.items():
    if n.startswith('collection'):
        mm = re.search(r'model:([A-Za-z_$]+)', s)
        if mm and mm.group(1) in argmap[n]:
            coll_model[n] = argmap[n][mm.group(1)]

result = collections.OrderedDict()
def ep(u):
    if u not in result:
        result[u] = dict(path=u, methods=set(), modules=set(), views=set(), request=set(), response=set(), notes=[])
    return result[u]

# 1. direct ajax contexts
for e in ex:
    u = re.sub(r'^/', '', e['url'])
    r = ep(u)
    fn = e['module']
    mn = fname2mod.get(fn, fn)
    r['modules'].add(mn)
    if not (mn.startswith(('models', 'collection')) and is_def(e)):
        for t in e.get('type', []):
            r['methods'].add(t)
    obj = e.get('obj', '')
    # data keys
    if e.get('data'):
        # skip if data captured ajax-option keys incorrectly
        if not set(e['data']) & {'url', 'type'}:
            r['request'].update(e['data'])
    js = re.search(r'JSON\.stringify\(\{(.*?)\}\)', obj)
    if js and 'data' in e.get('keys', []):
        r['request'].update(re.findall(r'(?:^|,)\s*"?([A-Za-z0-9_]+)"?\s*:', js.group(1)))
    fd = [x for x in re.findall(r'\.append\("([^"]+)",', obj) if re.match(r'^[A-Za-z0-9_\-]+$', x)]
    if fd:
        r['request'].update('multipart:' + x for x in fd)
    # response fields from success callback
    sm = re.search(r'success:function\(([A-Za-z_$]+)', obj)
    if sm:
        v = re.escape(sm.group(1))
        body = obj[sm.end():]
        r['response'].update(f for f in re.findall(r'(?<![A-Za-z0-9_$.])' + v + r'(?:\[[^\]]{1,20}\])?\.([A-Za-z_][A-Za-z0-9_]*)', body)
                             if f not in ('length', 'each', 'forEach', 'push', 'toString', 'split', 'responseJSON', 'status', 'indexOf', 'map', 'filter', 'replace', 'trim', 'substr', 'substring', 'toLowerCase', 'toUpperCase', 'slice', 'charAt', 'join', 'sort', 'get', 'set', 'models', 'toJSON', 'where', 'find', 'pluck', 'at', 'concat'))
    # model definitions -> fetch GET
    if mn.startswith(('models', 'collection')) and is_def(e):
        r['methods'].add('GET (fetch)')

# 2. model/collection usage in views
for n in mods:
    if not is_data(n):
        continue
    us = [u for u in mod_urls(n)]
    if not us:
        continue
    modelname = coll_model.get(n)
    users = set(parents[n])
    if modelname:
        users |= set(parents[modelname])
    # item views of user views
    ext = set()
    for v in list(users):
        for x in deps.get(v, []):
            if x.startswith('views') and ('item' in x):
                ext.add(x)
    allv = users | ext
    for u in us:
        r = ep(u)
        req = set(info[n]['validation']) | set(info[n]['defaults'])
        if modelname and modelname in info:
            req |= set(info[modelname]['validation']) | set(info[modelname]['defaults'])
        for v in allv:
            if v not in info or not v.startswith('views'):
                continue
            r['views'].add(v)
            vs = mods[v]
            vm = mask(vs)
            ddeps = [x for x in deps[v] if is_data(x) and mod_urls(x)]
            direct = n in deps[v] or (modelname and modelname in deps[v])
            if direct and len(ddeps) == 1:
                r['response'].update(info[v]['get'])
                req.update(info[v]['set']); req.update(info[v]['save'])
                ops = info[v]['ops']
            elif direct:
                arg = [a for a, dn in argmap[v].items() if dn in (n, modelname)]
                ops = []
                for a in arg:
                    ae = re.escape(a)
                    r['response'].update(re.findall(r'(?<![A-Za-z0-9_$.])' + ae + r'\.get\("([A-Za-z0-9_\-]+)"\)', vs))
                    req.update(re.findall(r'(?<![A-Za-z0-9_$.])' + ae + r'\.set\("([A-Za-z0-9_\-]+)"', vs))
                    req.update(objkeys_after(vs, vm, r'(?<![A-Za-z0-9_$.])' + ae + r'\.(?:save|set)\('))
                    ops += re.findall(r'(?<![A-Za-z0-9_$.])' + ae + r'\.(save|destroy|fetch|create)\(', vs)
                    if re.search(r'this\.(?:collection|model)=' + ae + r'(?![A-Za-z0-9_$])', vs):
                        r['response'].update(re.findall(r'\.model\.get\("([A-Za-z0-9_\-]+)"\)', vs))
                        r['response'].update(re.findall(r'(?<![A-Za-z0-9_$.])[a-z]\.get\("([A-Za-z0-9_\-]+)"\)', vs))
                        req.update(re.findall(r'\.model\.set\("([A-Za-z0-9_\-]+)"', vs))
                        req.update(objkeys_after(vs, vm, r'\.model\.(?:save|set)\('))
                        ops += re.findall(r'\.model\.(save|destroy)\(', vs) + re.findall(r'\.collection\.(create)\(', vs)
            else:
                r['response'].update(re.findall(r'\.model\.get\("([A-Za-z0-9_\-]+)"\)', vs))
                r['response'].update(re.findall(r'\.model\.bind\("change:([A-Za-z0-9_]+)"', vs))
                req.update(re.findall(r'\.model\.set\("([A-Za-z0-9_\-]+)"', vs))
                req.update(objkeys_after(vs, vm, r'\.model\.(?:save|set)\('))
                ops = re.findall(r'\.model\.(save|destroy)\(', vs)
            if 'save' in ops:
                r['methods'].add('PUT (save)')
            if 'destroy' in ops:
                r['methods'].add('DELETE (destroy)')
            if 'create' in ops:
                r['methods'].add('POST (create)')
        r['request'].update(req)

for n in mods:
    if n.startswith('views'):
        for x in deps[n]:
            pass

out = []
for u, r in sorted(result.items()):
    out.append(dict(path=u, methods=sorted(r['methods']), modules=sorted(r['modules']), views=sorted(r['views']),
                    request=sorted(r['request']), response=sorted(r['response'])))
json.dump(out, open('enr_%s.json' % board, 'w'), indent=1)
json.dump(dict(deps=deps, info=info, tmpl_ids=tmpl_ids), open('meta_%s.json' % board, 'w'))
print(len(out))

# ---- validation hints per endpoint ----
def val_hints(n):
    s = mods[n]; m = mask(s)
    out = {}
    for mm in re.finditer(r'\bvalidation:', s):
        p = mm.end()
        if p + 1 >= len(s) or s[p] != '{' or s[p + 1] == '}':
            continue
        e2 = enclosing(m, p + 1)
        if not e2:
            continue
        a, b = e2
        # iterate top-level key: value
        depth = 0; i = a + 1; start = i
        parts = []
        while i < b - 1:
            c = m[i]
            if c in '{[(':
                depth += 1
            elif c in '}])':
                depth -= 1
            elif c == ',' and depth == 0:
                parts.append((start, i)); start = i + 1
            i += 1
        parts.append((start, b - 1))
        for x, y in parts:
            seg = s[x:y]
            km = re.match(r'\s*"?([A-Za-z0-9_\-]+)"?\s*:(.*)$', seg, re.S)
            if not km:
                continue
            k, v = km.group(1), km.group(2)
            h = []
            for kw in ('required', 'min', 'max', 'minLength', 'maxLength', 'length'):
                q = re.search(r'\b' + kw + r':(!0|!1|-?[0-9.e]+)', v)
                if q:
                    h.append(kw + '=' + q.group(1).replace('!0', 'true').replace('!1', 'false'))
            for kw in ('range', 'rangeLength', 'oneOf'):
                q = re.search(r'\b' + kw + r':(\[[^\]]{0,120}\])', v)
                if q:
                    h.append(kw + '=' + q.group(1))
            q = re.search(r'\bpattern:("[a-z]+"|/[^/]{1,60}/)', v)
            if q:
                h.append('pattern=' + q.group(1))
            if 'fn:' in v:
                h.append('custom-fn')
            msgs = re.findall(r'\.t\("[a-z_0-9]+:([A-Za-z0-9_]+)"\)|\b[a-z]\.([a-z][A-Za-z0-9_]{12,})', v)
            msgs = [a or b for a, b in msgs]
            if msgs:
                h.append('msg=' + '|'.join(sorted(set(msgs)))[:200])
            out[k] = '; '.join(h)
    return out

vh = {}
for n in mods:
    if is_data(n):
        for u in mod_urls(n):
            d2 = val_hints(n)
            mn = coll_model.get(n)
            if mn and mn in mods:
                d2.update(val_hints(mn))
            if d2:
                vh.setdefault(u, {}).update(d2)
for o in out:
    o['validation'] = vh.get(o['path'], {})
    # collection item path hint
    o['collection'] = any(x.startswith('collection') for x in o['modules'])
json.dump(out, open('enr_%s.json' % board, 'w'), indent=1)

# ---- template option values ----
def tmpl_src(name):
    fn = name.replace('/', '__').replace('!', '_')
    p = os.path.join(d, fn + '.js')
    if os.path.exists(p):
        return open(p, encoding='utf-8', errors='replace').read().replace("\\'", "'").replace('\\"', '"').replace('\\n', ' ')
    return ''
def options(src):
    res = {}
    for sm in re.finditer(r'<select[^>]*id=["\']id([A-Za-z0-9_]+)["\'][^>]*>(.*?)</select>', src, re.S):
        vals = re.findall(r'<option[^>]*value=["\']([^"\']*)["\'][^>]*>\s*([^<]{0,80})', sm.group(2))
        if vals:
            res[sm.group(1)] = ['%s=%s' % (v, re.sub(r'<%=\s*locale\.(?:t\(["\'][a-z_]+:)?([A-Za-z0-9_]+).*', r'\1', l.strip()).strip()[:40]) for v, l in vals][:40]
    for rm in re.finditer(r'<input[^>]*type=["\']radio["\'][^>]*>', src):
        tag = rm.group(0)
        nm = re.search(r'name=["\']([A-Za-z0-9_]+)', tag); vv = re.search(r'value=["\']([^"\']*)', tag)
        if nm and vv:
            after = src[rm.end():rm.end() + 120]
            lab = re.search(r'locale\.(?:t\(["\'][a-z_]+:)?([A-Za-z0-9_]+)', after)
            res.setdefault(nm.group(1), [])
            item = vv.group(1) + ('=' + lab.group(1) if lab else '')
            if item not in res[nm.group(1)]:
                res[nm.group(1)].append(item)
    return res
for o in out:
    opts = {}
    for v in o['views'] + [m for m in o['modules'] if m.startswith('views')]:
        for t in deps.get(v, []):
            if t.startswith('text!'):
                for k, vals in options(tmpl_src(t)).items():
                    if k in o['request'] or k in o['response'] or k.replace('id', '', 1) in o['request']:
                        opts[k] = vals
    o['options'] = opts
json.dump(out, open('enr_%s.json' % board, 'w'), indent=1)
