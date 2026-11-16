import sys,subprocess,re,json,os
root=sys.argv[1]
files=['default_route.lua','extensions/routes/hi_routes.lua','oem/ami/route.lua','oem/ami_hi/route.lua']
out={}
for f in files:
    s=subprocess.run(['strings','-n','2',os.path.join(root,f)],capture_output=True,text=True).stdout
    res=[]
    for line in s.splitlines():
        # split concatenated constants: a '$' followed by one length byte then '/'
        parts=re.split(r'(?<=\$).(?=/)',line)
        for p in parts:
            p=p.strip()
            m=re.search(r'(/[A-Za-z0-9_()\[\]^/%+.\-$]*)$',p)
            if m and ('/' in m.group(1)): res.append(m.group(1))
    out[f]=[r for r in res if r.endswith('$') or 'AmiOemBMCUpdate' in r or 'SubmitDelayedTestEvent' in r]
t=open(os.path.join(root,'extensions/routes/update_service_routes.lua')).read()
out['extensions/routes/update_service_routes.lua']=[m for line in t.splitlines() if not line.strip().startswith('--') for m in re.findall(r'"(/[^"]*\$)"',line)]
json.dump(out,open(sys.argv[2],'w'),indent=1)
for k,v in out.items(): print(k,len(v),len(set(v)))
