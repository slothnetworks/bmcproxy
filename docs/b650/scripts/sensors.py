import json,sys,collections
def load(f,i):
    h=json.load(open(f)); return json.loads(h['log']['entries'][i]['response']['content']['text'])
f=sys.argv[1]; i=int(sys.argv[2])
d=load(f,i)
print('count',len(d))
kc=collections.Counter(tuple(r.keys()) for r in d); 
for k,c in kc.items(): print(c,k)
tc=collections.defaultdict(collections.Counter)
for r in d:
    for k,v in r.items(): tc[k][type(v).__name__]+=1
print({k:dict(v) for k,v in tc.items()})
print('accessible',collections.Counter(r['accessible'] for r in d))
print('sensor_state',collections.Counter(r['sensor_state'] for r in d))
print('discrete_state',collections.Counter(r['discrete_state'] for r in d))
print('unit',collections.Counter(r['unit'] for r in d))
print('settable_flag',collections.Counter(hex(r.get('settable_flag',-1)) for r in d))
def f2(x): 
    return '–' if x=='NA' else ('%g'%x if isinstance(x,(int,float)) else str(x))
for r in d:
    th='/'.join(f2(r[k]) for k in ['lower_non_recoverable_threshold','lower_critical_threshold','lower_non_critical_threshold','higher_non_critical_threshold','higher_critical_threshold','higher_non_recoverable_threshold'])
    print('| %s | %s | %s (%s) | %s | %s | %s | %s | ss=%s raw=%s mask=%s |'%(r['sensor_number'],r['name'],r['type'],r['type_number'],r['unit'],th,r['accessible'],f2(r['reading']),r['sensor_state'],r['raw_reading'],hex(r.get('settable_flag',0))))
