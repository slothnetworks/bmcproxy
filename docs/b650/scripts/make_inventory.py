#!/usr/bin/env python3
import json, os, re
WD=os.path.dirname(os.path.abspath(__file__))
raw=json.load(open(f"{WD}/rf_inventory_raw.json"))
DOMINANT='"1793662187"'

# known ActionInfo allowable values gathered from ResetActionInfo / SimpleUpdateActionInfo
RESET_SYSCHASSIS=["ForceOff","ForceRestart","GracefulShutdown","On","PowerCycle"]
RESET_MGR=["ForceRestart"]
def allow_for(uri, name):
    if name in ("#ComputerSystem.Reset","#Chassis.Reset"): return RESET_SYSCHASSIS
    if name=="#Manager.Reset": return RESET_MGR
    if name=="#UpdateService.SimpleUpdate": return {"TransferProtocol":["HTTP","FTP","HTTPS"]}
    return None

out=[]
for r in sorted(raw, key=lambda x:x["uri"]):
    st=r.get("status")
    if st=="skipped":
        out.append({"uri":r["uri"],"status":"not called (action path)","type":None,
                    "etag_differs":None,"actions":[],"latency_ms":None,
                    "notes":"SSE stream / multipart upload target; GET skipped to stay read-only"})
        continue
    etag=r.get("etag")
    etag_differs = (etag is not None and etag!=DOMINANT) if st==200 else None
    acts=[]
    for a in r.get("actions",[]):
        allow=a.get("allowable")
        if allow is None:
            allow=allow_for(r["uri"], a.get("name"))
        acts.append({"name":a.get("name"),"target":a.get("target"),"allowable":allow})
    notes=[]
    if st==404: notes.append("linked but returns 404 (not implemented / host powered off)")
    if r.get("allow") and r["allow"]!="GET": notes.append(f"Allow: {r['allow']}")
    if etag_differs: notes.append(f"ETag {etag} (differs from service epoch)")
    out.append({"uri":r["uri"],"status":st,"type":r.get("type"),
                "etag":etag,"etag_differs":etag_differs,
                "actions":acts,"latency_ms":r.get("latency_ms"),
                "notes":"; ".join(notes) or None})
json.dump(out, open(f"{WD}/live-redfish-inventory.json","w"), indent=1)
print("wrote",len(out),"entries")
print("200:",sum(1 for o in out if o['status']==200),
      "404:",sum(1 for o in out if o['status']==404),
      "skipped:",sum(1 for o in out if isinstance(o['status'],str)))
print("with actions:",sum(1 for o in out if o['actions']))
print("etag_differs True:",sum(1 for o in out if o['etag_differs']))
