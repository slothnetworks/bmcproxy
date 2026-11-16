import json,sys,collections
e=json.load(open(sys.argv[1]))
ks=collections.OrderedDict()
for x in e:
    for k,v in x.items(): ks.setdefault(k,[collections.Counter(),set(),0]); ks[k][0][repr(v)[:40]]+=1; ks[k][1].add(type(v).__name__); ks[k][2]+=1
for k,(c,t,n) in ks.items(): print(k,t,'present',n,'distinct',len(c),dict(c.most_common(8)))
print('record types',collections.Counter(x['record_type'] for x in e))
print('ids',min(x['id'] for x in e),max(x['id'] for x in e),'order first', [x['id'] for x in e[:5]])
ts=[x['timestamp'] for x in e]; print('ts range',min(ts),max(ts))
print(collections.Counter((x['sensor_type'],x['sensor_name'],x['event_description']) for x in e).most_common(40))
for x in e:
    if x['record_type']!='system_event_record': print(json.dumps(x)); break
