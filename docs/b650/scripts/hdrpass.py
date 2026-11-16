#!/usr/bin/env python3
# Re-fetch all discovered 200 URIs once more with -i (headers+body combined per
# file), paced + keep-alive, to capture ETag/Allow reliably per resource.
import json, os, subprocess
HOST=os.environ["BMC_HOST"]; BASE=f"https://{HOST}"
WD=os.path.dirname(os.path.abspath(__file__))
OUT=os.path.join(WD,"rf_full"); os.makedirs(OUT,exist_ok=True)
inv=json.load(open(os.path.join(WD,"rf_inventory_raw.json")))
uris=[r["uri"] for r in inv if r.get("status")==200]
cfg=[]
for i,u in enumerate(uris):
    cfg.append(f'output = "{OUT}/{i}.txt"')
    cfg.append(f'url = "{BASE}{u}"')
open(os.path.join(WD,"hdrpass.cfg"),"w").write("\n".join(cfg)+"\n")
secret=f'user = "{os.environ["BMC_USER"]}:{os.environ["BMC_PASS"]}"\n'
subprocess.run(["curl","-sk","-i","--rate","360/m","-K","-","-K",os.path.join(WD,"hdrpass.cfg")],
               input=secret,capture_output=True,text=True)
# parse
res={}
for i,u in enumerate(uris):
    p=f"{OUT}/{i}.txt"
    etag=allow=None
    try:
        data=open(p,encoding="utf-8",errors="replace").read()
    except FileNotFoundError:
        res[u]=(None,None); continue
    head=data.split("\r\n\r\n",1)[0] if "\r\n\r\n" in data else data.split("\n\n",1)[0]
    for ln in head.splitlines():
        l=ln.strip()
        if l.lower().startswith("etag:"): etag=l.split(":",1)[1].strip()
        elif l.lower().startswith("allow:"): allow=l.split(":",1)[1].strip()
    res[u]=(etag,allow)
# merge back
for r in inv:
    if r.get("status")==200 and r["uri"] in res:
        e,a=res[r["uri"]]
        r["etag"]=e; r["allow"]=a
json.dump(inv,open(os.path.join(WD,"rf_inventory_raw.json"),"w"),indent=1)
from collections import Counter
got=[r for r in inv if r.get("status")==200]
we=[r for r in got if r.get("etag")]
wa=[r for r in got if r.get("allow")]
print("200:",len(got)," with ETag:",len(we)," with Allow:",len(wa))
print("distinct ETag values:",len(Counter(r["etag"] for r in we)))
for v,c in Counter(r["etag"] for r in we).most_common(8): print("  ",c,v)
print("Allow variety:",dict(Counter(r.get("allow") for r in got)))
