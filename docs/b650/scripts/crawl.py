#!/usr/bin/env python3
# Redfish authenticated BFS crawl. One curl invocation per round (keep-alive,
# one request at a time via --rate). Basic auth; no server-side sessions.
import json, os, re, subprocess, sys, html

HOST = os.environ["BMC_HOST"]
BASE = f"https://{HOST}"
WD = os.path.dirname(os.path.abspath(__file__))
BODY = os.path.join(WD, "rf_body"); HDR = os.path.join(WD, "rf_hdr")
os.makedirs(BODY, exist_ok=True); os.makedirs(HDR, exist_ok=True)

DEPTH_CAP = 8
RES_CAP = 600

# action-path denylist for *invocation* (we never POST anyway); also skip GET of
# obviously-actionable singletons that could trigger side effects.
SKIP_GET = re.compile(r'(/Actions/|/upgrade|/flash|nm_activate|restore_defaults|bmccoldreset|SimpleUpdate|/upload\b|UploadCABundle|/SSE$)', re.I)

seen = {}          # uri -> record
frontier = [("/redfish/v1/", 0)]
queued = {"/redfish/v1/"}
order = 0

def run_round(items):
    """items: list of (uri, idx). Returns dict idx->(code,time)."""
    cfg_lines = []
    for uri, idx in items:
        cfg_lines.append(f'output = "{BODY}/{idx}.body"')
        cfg_lines.append(f'dump-header = "{HDR}/{idx}.hdr"')
        # url-encode spaces etc minimally; redfish uris are clean
        cfg_lines.append(f'url = "{BASE}{uri}"')
    cfg = "\n".join(cfg_lines) + "\n"
    cfgpath = os.path.join(WD, "round_urls.cfg")
    open(cfgpath, "w").write(cfg)
    # secret config via stdin
    secret = f'user = "{os.environ["BMC_USER"]}:{os.environ["BMC_PASS"]}"\n'
    wfmt = '%{json}\n'
    p = subprocess.run(
        ["curl", "-sk", "--rate", "300/m", "-K", "-", "-K", cfgpath,
         "-w", wfmt],
        input=secret, capture_output=True, text=True)
    res = {}
    # curl prints one json per transfer in order; map by url_effective
    url2idx = {f"{BASE}{u}": i for u, i in items}
    for line in p.stdout.splitlines():
        line = line.strip()
        if not line.startswith("{"): continue
        try: j = json.loads(line)
        except Exception: continue
        ue = j.get("url_effective", "").rstrip()
        idx = url2idx.get(ue) or url2idx.get(ue.rstrip("/"))
        if idx is None:
            # try match ignoring trailing slash differences
            for k, v in url2idx.items():
                if k.rstrip("/") == ue.rstrip("/"): idx = v; break
        if idx is not None:
            res[idx] = (int(j.get("http_code", 0)), float(j.get("time_total", 0)))
    return res

def parse_headers(path):
    etag = allow = None
    try:
        for ln in open(path, encoding="utf-8", errors="replace"):
            l = ln.strip()
            if l.lower().startswith("etag:"): etag = l.split(":", 1)[1].strip()
            elif l.lower().startswith("allow:"): allow = l.split(":", 1)[1].strip()
    except FileNotFoundError:
        pass
    return etag, allow

def extract_links(obj, acc):
    """collect @odata.id strings and action targets."""
    if isinstance(obj, dict):
        for k, v in obj.items():
            if k == "@odata.id" and isinstance(v, str):
                acc["ids"].add(v)
            elif k == "@Redfish.ActionInfo" and isinstance(v, str):
                acc["ids"].add(v)
            elif k == "Members@odata.nextLink" and isinstance(v, str):
                acc["ids"].add(v)
            elif k == "Actions" and isinstance(v, dict):
                for an, av in v.items():
                    if isinstance(av, dict) and "target" in av:
                        acc["actions"].append({
                            "name": an,
                            "target": av.get("target"),
                            "allowable": av.get("ResetType@Redfish.AllowableValues")
                                         or av.get("TransferProtocol@Redfish.AllowableValues")
                                         or _any_allow(av),
                            "actioninfo": av.get("@Redfish.ActionInfo"),
                        })
                        if av.get("@Redfish.ActionInfo"):
                            acc["ids"].add(av["@Redfish.ActionInfo"])
                    elif an == "Oem" and isinstance(av, dict):
                        for on, ov in av.items():
                            if isinstance(ov, dict) and "target" in ov:
                                acc["actions"].append({"name": on, "target": ov.get("target"),
                                    "allowable": _any_allow(ov), "actioninfo": ov.get("@Redfish.ActionInfo")})
                                if ov.get("@Redfish.ActionInfo"): acc["ids"].add(ov["@Redfish.ActionInfo"])
            else:
                extract_links(v, acc)
    elif isinstance(obj, list):
        for it in obj: extract_links(it, acc)

def _any_allow(av):
    for k, v in av.items():
        if k.endswith("@Redfish.AllowableValues"): return v
    return None

pending = []  # global FIFO of (uri, depth) still to fetch
for uri, depth in frontier:
    pending.append((uri, depth))

while pending and len([u for u in seen if seen[u]["status"] != "skipped"]) < RES_CAP:
    # pull up to 100 un-seen, non-skip items off the front; keep remainder
    batch = []
    remainder = []
    for uri, depth in pending:
        if uri in seen:
            continue
        if SKIP_GET.search(uri):
            seen[uri] = {"uri": uri, "status": "skipped", "note": "not called (action path)", "depth": depth}
            continue
        if len(batch) < 100:
            batch.append((uri, depth))
        else:
            remainder.append((uri, depth))
    if not batch:
        break
    items = []
    idxmap = {}
    for uri, depth in batch:
        order += 1
        items.append((uri, order))
        idxmap[order] = (uri, depth)
    results = run_round(items)
    new_frontier = []
    for idx, (uri, depth) in idxmap.items():
        code, t = results.get(idx, (0, 0))
        etag, allow = parse_headers(f"{HDR}/{idx}.hdr")
        rec = {"uri": uri, "status": code, "etag": etag, "allow": allow,
               "latency_ms": round(t*1000, 1), "depth": depth, "type": None,
               "actions": [], "idx": idx}
        bodypath = f"{BODY}/{idx}.body"
        otype = None
        if code == 200 and os.path.exists(bodypath):
            try:
                j = json.load(open(bodypath, encoding="utf-8", errors="replace"))
            except Exception:
                j = None
            if isinstance(j, dict):
                otype = j.get("@odata.type")
                rec["type"] = otype
                acc = {"ids": set(), "actions": []}
                extract_links(j, acc)
                rec["actions"] = acc["actions"]
                if depth < DEPTH_CAP:
                    for nid in acc["ids"]:
                        if nid.startswith("/redfish/") and nid not in seen and nid not in queued:
                            queued.add(nid); new_frontier.append((nid, depth+1))
        seen[uri] = rec
    pending = remainder + new_frontier

# write inventory
inv = []
for uri in sorted(seen):
    r = seen[uri]
    if r.get("status") == "skipped":
        inv.append({"uri": uri, "status": "skipped (action path)", "type": None,
                    "etag_differs": None, "actions": [], "latency_ms": None,
                    "notes": "not called (action path)"})
    else:
        inv.append({"uri": uri, "status": r["status"], "type": r.get("type"),
                    "etag": r.get("etag"), "allow": r.get("allow"),
                    "actions": r.get("actions", []), "latency_ms": r.get("latency_ms"),
                    "depth": r.get("depth")})
json.dump(inv, open(os.path.join(WD, "rf_inventory_raw.json"), "w"), indent=1)
print("total uris:", len(seen))
from collections import Counter
print("status counts:", Counter(str(v.get("status")) for v in seen.values()))
# etag analysis
etags = Counter(v.get("etag") for v in seen.values() if v.get("status")==200 and v.get("etag"))
print("distinct etags (200):", len(etags), "— top:", etags.most_common(5))
print("resources with actions:", sum(1 for v in seen.values() if v.get("actions")))
