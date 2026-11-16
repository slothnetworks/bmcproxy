import sys,re
cur=None;res={}
for line in open(sys.argv[1]):
    m=re.match(r'^  (/redfish\S*):\s*$',line)
    if m: cur=m.group(1); res[cur]=[]; continue
    if re.match(r'^\S',line): cur=None
    if cur:
        m=re.match(r'^    (get|post|patch|delete|put|head):',line)
        if m: res[cur].append(m.group(1).upper())
for k,v in res.items(): print(k,' '.join(v))
