import json, sys, collections, re
H='/opt/sparked/asrockrack-api/harfiles/'
def E(f): return json.load(open(H+f))['log']['entries']
def B(es,i): return json.loads(es[i]['response']['content']['text'])
from urllib.parse import urlsplit
b=E('b650d4u.har'); t=E('b650d4u-2t.har'); x=E('x470d4u.har')
def find(es,path): return [i for i,e in enumerate(es) if urlsplit(e['request']['url']).path==path]
u=B(t,19)
print('users n',len(u)); print(collections.Counter((r['channel'],r['channel_type']) for r in u))
for r in u:
    if r['name']: print({k:r[k] for k in ('id','channel','channel_type','userid','name','access','accessByChannel','kvm','vmedia','privilege','privilegeByChannel','fixed_user_count','creation_time','password_age','password_enddate','email_format','ssh_key')})
print('empty slot sample', [r for r in u if not r['name']][0])
xu=B(x,find(x,'/api/settings/users')[0]); print('x470 users keys',list(xu[0].keys())); print('2t keys',list(u[0].keys()))
pj=[r['feature'] for r in B(t,21)]; xp=[r['feature'] for r in B(x,find(x,'/api/configuration/project')[0])]
print('project 2T n',len(pj),'dups',[k for k,c in collections.Counter(pj).items() if c>1])
print(' 2T only',[f for f in pj if f not in xp]); print(' X470 only',[f for f in xp if f not in pj])
au=B(t,24); print('audit n',len(au)); 
for r in au: print('  ',r['id'],r['timestamp'],re.sub(r'IP:\S+','IP:<ip>',r['message'])[:150])
print('date-time 2t',B(t,22)); print('x470 date-time',B(x,find(x,'/api/settings/date-time')[0]))
pi=B(b,33)[0]; print('power_info keys',list(pi.keys()))
for k,v in pi.items():
    print(' ',k,len(v)); 
    for r in v: print('    ',r)
