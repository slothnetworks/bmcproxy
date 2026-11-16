import json, sys, collections
H='/opt/sparked/asrockrack-api/harfiles/'
def E(f): return json.load(open(H+f))['log']['entries']
def B(es,i): return json.loads(es[i]['response']['content']['text'])
b=E('b650d4u.har'); t=E('b650d4u-2t.har'); x=E('x470d4u.har')
from urllib.parse import urlsplit
def find(es,path):
    return [i for i,e in enumerate(es) if urlsplit(e['request']['url']).path==path]
ds=B(b,19)
print('dashboardsensors len',len(ds)); print([r for r in ds if len(r)>3])
print('dashboardsensors sensor numbers == sensors?', [r['sensor_number'] for r in ds]==[r['sensor_number'] for r in B(b,37)])
for es,i,name in ((b,20,'b650'),(t,28,'2t')):
    de=B(es,i); print(name,'dashboardevent len',len(de),'last',de[-1], 'first', de[0]); print('  ids',[r.get('id') for r in de][:5],'...',[r.get('id') for r in de][-5:])
    print('  sensor_names',collections.Counter(r.get('sensor_name') for r in de))
    print('  keys',collections.Counter(tuple(r.keys()) for r in de))
# sensors diff 2t
s1=B(t,31); s2=B(t,62)
for a,c in zip(s1,s2):
    if a!=c: print('2t sensor change',a['name'],{k:(a[k],c[k]) for k in a if a[k]!=c[k]})
# sdr
sd=B(t,43); sn=set(r['sensor_number'] for r in s1)
print('sdr 2t len',len(sd),'extra',[ (r['sensor_number'],r['name'],r['type'],r['type_number']) for r in sd if r['sensor_number'] not in sn])
xs=find(x,'/api/sensors'); xsen=B(x,xs[-1]); xsd=B(x,find(x,'/api/sdr')[0])
print('x470 sensors',len(xsen),'x470 sdr keys',list(xsd[0].keys()))
xm={r['sensor_number']:r['name'] for r in xsen}; tm={r['sensor_number']:r['name'] for r in s1}
print('in 2T not X470',[(k,v) for k,v in tm.items() if k not in xm]); print('in X470 not 2T',[(k,v) for k,v in xm.items() if k not in tm])
print('renamed',[(k,xm[k],tm[k]) for k in tm if k in xm and xm[k]!=tm[k]])
print('x470 sensor keys', list(xsen[0].keys()))
# eventlog
ev=B(t,42); print('eventlog len',len(ev),'last',ev[-1]); 
print(' keys sets',collections.Counter(tuple(sorted(r.keys())) for r in ev[:-1]).most_common(3))
print(' ids',[r.get('id') for r in ev[:3]],[r.get('id') for r in ev[-4:]])
print(' types',collections.Counter((r.get('sensor_type'),r.get('event_description')) for r in ev[:-1]))
print(' ts range',min(r['timestamp'] for r in ev[:-1]),max(r['timestamp'] for r in ev[:-1]))
xev=B(x,find(x,'/api/logs/eventlog')[0]); xk=set().union(*[r.keys() for r in xev]); tk=set().union(*[r.keys() for r in ev[:-1]])
print(' fields in 2T not X470',tk-xk); print(' fields in X470 not 2T',xk-tk)
print(' sample',json.dumps(ev[0])[:1500])
