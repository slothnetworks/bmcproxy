import sys,os
sys.path.insert(0,os.path.dirname(os.path.abspath(__file__)))
from lstrlib import scan
src,dst=sys.argv[1],sys.argv[2]
n=0;bc=0;txt=0
for dp,dn,fn in os.walk(src):
    for f in fn:
        if not f.endswith('.lua'):continue
        p=os.path.join(dp,f); rel=os.path.relpath(p,src)
        data=open(p,'rb').read(); n+=1
        o=os.path.join(dst,rel+'.txt'); os.makedirs(os.path.dirname(o),exist_ok=True)
        if data[:3]==b'\x1bLJ':
            bc+=1; open(o,'w').write('\n'.join(s.replace('\n','\\n') for s in scan(data)))
        else:
            txt+=1; open(o,'wb').write(data)
print(src,'lua',n,'bytecode',bc,'text',txt)
