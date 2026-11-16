import sys,os,json,glob
sys.path.insert(0,os.path.dirname(os.path.abspath(__file__)))
from lstrlib import sfile
root=sys.argv[1]
dirs=sorted(glob.glob(root+'/**/redfish-resource-URI_pattern.lua',recursive=True))
res={}
for p in dirs:
    d=os.path.dirname(p); rel=os.path.relpath(d,root)
    s=sfile(p)
    pats=[]
    for i,x in enumerate(s):
        if x.startswith('/') and i+2<len(s) and s[i+1]=='key':
            pats.append((x,s[i+2]))
        elif x.startswith('/') and x.endswith('$'):
            pats.append((x,None))
    # entities
    ent={}
    es=sfile(d+'/redfish-resource-entities.lua') if os.path.exists(d+'/redfish-resource-entities.lua') else []
    # allow
    al={}
    if os.path.exists(d+'/redfish-resource-allow_method.lua'):
        a=sfile(d+'/redfish-resource-allow_method.lua')
        res.setdefault('_allow',{})[rel]=a
    res[rel]={'patterns':pats,'entities_raw':es}
json.dump(res,open(sys.argv[2],'w'),indent=1)
for rel,v in res.items():
    if rel.startswith('_'):continue
    print(rel,len(v['patterns']),sum(1 for p in v['patterns'] if p[1] is None))
