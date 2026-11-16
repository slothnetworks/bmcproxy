import json, sys, collections, statistics
from urllib.parse import urlsplit, parse_qsl
from datetime import datetime

def ts(s):
    return datetime.strptime(s.replace('Z', '+0000'), '%Y-%m-%dT%H:%M:%S.%f%z').timestamp()

def load(f):
    return json.load(open(f))['log']['entries']

def body(e):
    t = e['response']['content'].get('text') or ''
    try:
        return json.loads(t)
    except Exception:
        return None

def stats(xs):
    xs = sorted(xs)
    return (round(xs[0]), round(statistics.median(xs)), round(xs[-1])) if xs else None

def group(E):
    g = collections.OrderedDict()
    for i, e in enumerate(E):
        u = urlsplit(e['request']['url'])
        if not u.path.startswith('/api'):
            continue
        k = (e['request']['method'], u.path)
        g.setdefault(k, []).append(i)
    return g

def typeprof(obj, prefix='', out=None):
    if out is None:
        out = collections.OrderedDict()
    if isinstance(obj, list):
        for x in obj:
            typeprof(x, prefix + '[]', out)
    elif isinstance(obj, dict):
        for k, v in obj.items():
            p = prefix + '.' + k
            if isinstance(v, (dict, list)):
                typeprof(v, p, out)
            else:
                out.setdefault(p, collections.Counter())[type(v).__name__] += 1
    return out

if __name__ == '__main__':
    f = sys.argv[1]
    E = load(f)
    g = group(E)
    print('connection ids', len(set(e.get('_connectionId') for e in E)))
    for (m, p), idx in g.items():
        es = [E[i] for i in idx]
        st = collections.Counter(e['response']['status'] for e in es)
        w = stats([e['timings']['wait'] for e in es])
        tt = stats([e['time'] for e in es])
        t0 = [ts(e['startedDateTime']) for e in es]
        iv = [round(b - a, 3) for a, b in zip(t0, t0[1:])]
        qs = sorted(set(urlsplit(e['request']['url']).query for e in es))
        sizes = sorted(set(len(e['response']['content'].get('text') or '') for e in es))
        bodies = set((e['response']['content'].get('text') or '') for e in es)
        print('=== %s %s n=%d idx=%s status=%s wait=%s total=%s q=%s sizes=%s distinct_bodies=%d' % (m, p, len(es), idx, dict(st), w, tt, qs, sizes, len(bodies)))
        print('    intervals', iv)
        b = body(es[0])
        prof = typeprof(b)
        print('    fields', {k: dict(v) for k, v in prof.items()})
