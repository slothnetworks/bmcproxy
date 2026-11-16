import sys,os,json
sys.path.insert(0,os.path.dirname(os.path.abspath(__file__)))
from lstrlib import sfile
R=sys.argv[1]; d=R+'/'+sys.argv[2]
METH={'GET','POST','PATCH','DELETE','PUT'}
s=sfile(d+'/redfish-resource-URI_pattern.lua')
pats=[];
for i,x in enumerate(s):
    if x.startswith('/') and x.endswith('$'):
        k=s[i+2] if i+2<len(s) and s[i+1]=='key' else None
        pats.append([x,k])
hk=[x for x in sfile(d+'/redfish-resource-handler.lua')]
keys=set(k for _,k in pats if k)
# candidate keys from handler file
cand=[x for x in hk if not x.startswith('redfish.') and '.' not in x and x not in ('create_handler_table','redfish-resource','require','config','turbo','pcall','include','path','package','get','connect','redis_sock','timeout','scheme','unix','redis','utils','quit','hdel','hset','recursive_table_merge','Redfish:Services','TelemetryService','Redfish:Telemetrics:Support','odata-handler','redfish-handler','redirect-handler')]
missing=[c for c in cand if c not in keys]
# entities
es=sfile(d+'/redfish-resource-entities.lua')
allkeys=keys|set(cand)
ent={};cur=None
for x in es:
    if x in allkeys and x not in ('sensor',) or (x=='sensor' and cur!='sensor_collection_x'):
        if x in allkeys: cur=x; ent[cur]={'chain':[],'name':None,'sup':False}; nxt=False; continue
    if cur is None: continue
    if x=='name': ent[cur]['_n']=True; continue
    if x=='superordinate': ent[cur]['sup']=True; continue
    if ent[cur].pop('_n',False): ent[cur]['name']=x
    else: ent[cur]['chain'].append(x)
# allow
al=sfile(d+'/redfish-resource-allow_method.lua') if os.path.exists(d+'/redfish-resource-allow_method.lua') else []
label={};meth={};cur=None;phase=1
i=0
while i<len(al):
    x=al[i]
    if x in allkeys:
        if i+1<len(al) and (al[i+1] in METH or al[i+1] in allkeys or al[i+1]=='overrides' or al[i+1] in('Administrator','Capabilities')):
            cur=x; meth.setdefault(cur,[]); i+=1; continue
        elif i+1<len(al):
            label[x]=al[i+1]; cur=None; i+=2; continue
    if cur is not None:
        meth[cur].append(x)
    i+=1
out=[]
for p,k in pats:
    e=ent.get(k,{})
    out.append({'pattern':p,'key':k,'entity':e.get('name'),'chain':e.get('chain'),'label':label.get(k),'methods':meth.get(k)})
json.dump({'rows':out,'missing_keys':missing,'unrouted_ent':[k for k in ent if k not in keys]},open(sys.argv[3],'w'),indent=1)
for r in out: print(r['key'],'|',r['pattern'],'|',r['entity'],'|',r['label'],'|',' '.join(r['methods'] or []))
print('MISSING',missing)
print('ENT not in patterns',[k for k in ent if k not in keys])
