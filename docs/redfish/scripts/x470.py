import sys,subprocess,re,json,os
root=sys.argv[1]
files=['default_route.lua','extensions/routes/hi_routes.lua','oem/ami/route.lua','oem/ami_hi/route.lua']
out={}
for f in files:
    s=subprocess.run(['strings','-n','2',os.path.join(root,f)],capture_output=True,text=True).stdout
    out[f]=re.findall(r'((?:\^%s)?/[A-Za-z0-9_()\[\]^/%+.\-{}$]*\$)',s)
t=open(os.path.join(root,'extensions/routes/update_service_routes.lua')).read()
out['extensions/routes/update_service_routes.lua']=[m for line in t.splitlines() if not line.strip().startswith('--') for m in re.findall(r'"(/[^"]*\$)"',line)]
json.dump(out,open(sys.argv[2],'w'),indent=1)
for k,v in out.items(): print(k,len(v))
