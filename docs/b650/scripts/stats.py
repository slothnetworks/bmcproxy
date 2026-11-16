import json,sys,statistics,collections
from urllib.parse import urlsplit
from datetime import datetime
from email.utils import parsedate_to_datetime
h=json.load(open(sys.argv[1])); E=h['log']['entries']
def ts(e): return datetime.fromisoformat(e['startedDateTime'].replace('Z','+00:00'))
g=collections.OrderedDict()
skews=[];conns=collections.Counter();close=[]
for i,e in enumerate(E):
    u=urlsplit(e['request']['url'])
    conns[e.get('connection')]+=1
    rh={x['name'].lower():x['value'] for x in e['response']['headers']}
    if 'date' in rh:
        skews.append((parsedate_to_datetime(rh['date'])-ts(e)).total_seconds())
    if not u.path.startswith('/api'): continue
    if rh.get('connection')=='close': close.append((i,u.path))
    k=(e['request']['method'],u.path); g.setdefault(k,[]).append((i,e))
print('entries',len(E),'first',E[0]['startedDateTime'],'last',E[-1]['startedDateTime'])
print('connection ids',len(conns),conns.most_common(5))
print('date skew s min/med/max',min(skews),statistics.median(skews),max(skews))
print('Connection: close',close)
print('api calls',sum(len(v) for v in g.values()),'unique',len(g))
for k,v in g.items():
    w=[e['timings']['wait'] for i,e in v]; t=[e['time'] for i,e in v]
    st=collections.Counter(e['response']['status'] for i,e in v)
    tt=[ts(e) for i,e in v]; iv=[round((b-a).total_seconds(),2) for a,b in zip(tt,tt[1:])]
    q=sorted(set(urlsplit(e['request']['url']).query for i,e in v))
    sz=[len(e['response']['content'].get('text') or '') for i,e in v]
    print('%-5s %-55s n=%-2d st=%s w=%d/%d/%d tot=%d/%d/%d q=%s sz=%s idx=%s iv=%s'%(k[0],k[1],len(v),dict(st),min(w),statistics.median(w),max(w),min(t),statistics.median(t),max(t),q,sorted(set(sz)),[i for i,e in v],iv))
