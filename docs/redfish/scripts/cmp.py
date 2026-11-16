import json,sys,re
inv=json.load(open(sys.argv[1]))['uris']
p=json.load(open(sys.argv[2]))
def shape(x):
    x=x.replace('^%s','/redfish/v1').replace('/Managers/%s/','/Managers/*/')
    if not x.startswith('/redfish'): x='/redfish/v1'+x
    x=x.rstrip('$')
    x=x.replace('([^/]+)','*').replace('($metadata)','$metadata')
    return x
mine={}
for k,v in p.items():
    for x in v: mine[shape(x)]=(k,x)
invs={}
for u in inv:
    s=re.sub(r'\{[^}]+\}','*',u['uri'])
    invs.setdefault(s,[]).append(u['key'])
print('FW not in inv:'); [print(' ',k,mine[k]) for k in sorted(set(mine)-set(invs))]
print('inv not in FW:'); [print(' ',k,invs[k]) for k in sorted(set(invs)-set(mine))]
