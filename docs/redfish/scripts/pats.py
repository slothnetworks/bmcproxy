import sys,subprocess,re,json,os,glob
# usage: pats.py <redfish root> <out.json>; URI patterns from `strings` output only
root=sys.argv[1]; out={}
for p in sorted(glob.glob(root+'/**/redfish-resource-URI_pattern.lua',recursive=True)):
    rel=os.path.relpath(os.path.dirname(p),root)
    s=subprocess.run(['strings','-n','2',p],capture_output=True,text=True).stdout
    res=[]
    for line in s.splitlines():
        for part in re.split(r'(?<=\$).(?=/|\^%s)',line):
            m=re.search(r'((?:\^%s)?/[A-Za-z0-9_()\[\]^/%+.\-$]*\$)$',part)
            if m: res.append(m.group(1))
    out[rel]=res
json.dump(out,open(sys.argv[2],'w'),indent=1)
for k,v in out.items(): print(k,len(v))
