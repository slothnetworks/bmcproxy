#!/usr/bin/env python3
"""Assemble docs/x470/endpoint-reference.{json,md} from the probe scratch data.
No network. Read-only inputs. Redacts body samples defensively."""
import json, re, collections, html

S = "/tmp/claude-1000/-opt-sparked-asrockrack-api/4c303595-4244-4c22-8dc9-6968bfcb1de5/scratchpad/endpoints"
OUT = "/opt/sparked/asrockrack-api/docs/x470"

master = json.load(open(f"{S}/master.json"))
results = json.load(open(f"{S}/results.json"))
rt = [json.loads(l) for l in open(f"{S}/out_rt.jsonl") if l.strip()]

# ---- redaction ---------------------------------------------------------
MAC = re.compile(r'\b([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}\b')
IP = re.compile(r'\b(10|127|169\.254|172\.(1[6-9]|2\d|3[01])|192\.168)(\.\d{1,3}){2,3}\b')
LONGHEX = re.compile(r'\b[0-9a-fA-F]{16,}\b')
TOKENISH = re.compile(r'("(?:[A-Za-z_]*(?:token|csrf|session|passw|secret|key)[A-Za-z_]*)"\s*:\s*)"[^"]*"', re.I)
def redact(s):
    if not isinstance(s, str): return s
    s = MAC.sub("<REDACTED_MAC>", s)
    s = IP.sub("<REDACTED_IP>", s)
    s = TOKENISH.sub(r'\1"<REDACTED>"', s)
    s = LONGHEX.sub("<REDACTED_HEX>", s)
    return s
def sample(body, n=240):
    if body is None: return None
    b = redact(str(body)).strip()
    b = re.sub(r'\s+', ' ', b)
    return b[:n] + ("…" if len(b) > n else "")

# ---- index live results by (method,path) -------------------------------
live = collections.defaultdict(list)
for r in results:
    live[(r["method"], r["path"].rstrip("/"))].append(r)

# round-trip write outcomes keyed by path
rtmap = collections.defaultdict(list)
for r in rt:
    rtmap[r["path"].rstrip("/")].append((r.get("method"), r.get("status"), r.get("id")))

def param_family(path):
    # collapse /:id and numeric ids to a template for lookups
    return re.sub(r'/\d+$', '/:id', path)

def live_for(path, methods):
    p = path.rstrip("/")
    hits = []
    for m in methods or ["GET"]:
        for r in live.get((m, p), []):
            hits.append(r)
        # try :id -> 1 instantiation that the prober used
        if not live.get((m, p)):
            for (mm, pp), rs in live.items():
                if mm == m and param_family(pp) == param_family(p):
                    hits.extend(rs)
    return hits

# ---- build entries -----------------------------------------------------
STATIC_PATHS = ["/config.bak", "/SELLog", "/bsod", "/capture", "/video"]
entries = []
for e in master:
    path = e["path"]
    cat = e.get("cat") or {}
    methods = cat.get("methods") or e.get("methods_js") or []
    hp = cat.get("handler_present") or {}
    in302 = "firmware" in (e.get("sources") or [])
    hits = live_for(path, methods)
    statuses = sorted({h["status"] for h in hits})
    lat = [h["latency_ms"] for h in hits if h.get("latency_ms")]
    body = None
    for h in hits:
        if h["status"] == 200 and h.get("body"):
            body = sample(h["body"]); break
    rth = rtmap.get(path.rstrip("/"), [])
    entries.append({
        "surface": "api",
        "path": path,
        "area": cat.get("area", "?"),
        "methods_known": methods,
        "handler_present": hp,
        "in_302_firmware": in302,
        "sources": e.get("sources"),
        "request_fields": cat.get("request_fields") or [],
        "response_fields": cat.get("response_fields") or [],
        "live_2_32_00": {
            "called": bool(hits),
            "statuses": statuses,
            "latency_ms_median": (sorted(lat)[len(lat)//2] if lat else None),
            "sample_200": body,
            "roundtrip_writes": [{"method": m, "status": s, "test": t} for m, s, t in rth] or None,
        },
        "notes": cat.get("notes", ""),
        "js_modules": cat.get("js_modules") or [],
    })

json.dump(entries, open(f"{OUT}/endpoint-reference.json", "w"), indent=1)

# ---- behavioural facts (from the probe) --------------------------------
facts = {
    "firmware_live": "2.32.00 (built 2021-05-11), BIOS P4.20; dumped image is 3.02.00",
    "auth": "form POST /api/session -> QSESSIONID cookie (30 chars, path=/, no HttpOnly/Secure/SameSite) + 8-char CSRF",
    "csrf_on_writes": "ENFORCED: PUT with bogus or empty X-CSRFTOKEN -> 401 {cc:7}; correct token -> 200",
    "csrf_on_gets": "NOT enforced: GET with a bogus token -> 200 (empty/no token -> 401)",
    "invalid_session": '401 {"cc":7,"error":"Invalid Authentication"}',
    "unknown_route": '404 {"error":"Invalid API Call","code":1010}',
    "feature_disabled": '405 code 8500 "Feature is not enabled"',
    "options_head": "always 200, empty, no Allow header, even for nonexistent paths",
    "method_override": "X-HTTP-Method-Override honoured by the backend; a proxy MUST strip it",
    "help_data_crash": "GET /api/logs/help-data (no ?file=) -> HTML 500 and crashes the backend; two-session test showed it invalidates ALL web sessions, not just the caller's",
    "html500_crashers": ["logs/help-data", "hpm/upgradestatus", "hpm/verifyimagestatus"],
    "bad_gateway_502": ["asrr/mca-file", "maintenance/download_config", "settings/download_image"],
    "mem_leak_suspect": ["settings/dns server dns_iface", "maintenance/firmware/dwldfwstatus-progress progress/state"],
    "write_500_payload_format": ["settings/date-time", "settings/pef/lan_destinations/1", "settings/smtp/1"],
    "static_no_auth_symlinks": STATIC_PATHS,
    "hidden_accounts": ["sysadmin (uid0, enable_user_access=1)", "fwupd (ipmi user id 3)"],
    "tls_cert": "CN=AMI self-signed, expired 2026-05-30; private key ships in firmware",
}
json.dump(facts, open(f"{OUT}/endpoint-behaviour.json", "w"), indent=1)

# ---- markdown ----------------------------------------------------------
byarea = collections.defaultdict(list)
for e in entries:
    byarea[e["area"]].append(e)

called = sum(1 for e in entries if e["live_2_32_00"]["called"])
nf_live = sum(1 for e in entries
              if 404 in e["live_2_32_00"]["statuses"] and e["in_302_firmware"])

L = []
L.append("# X470D4U endpoint reference (legacy `/api`)\n")
L.append("> Assembled from the live probe scratch data (no fresh BMC calls at assembly time). "
         "The live unit runs **BMC 2.32.00**; the dumped firmware is **3.02.00**, so routes seen only "
         "in the 3.02 binary are candidates (`in_302_firmware`) and a live 404 may just mean the route "
         "is newer than 2.32. Body samples are redacted. Redfish is covered in "
         "`live-probe.md` / `redfish-uri-inventory.json`; this file is the `/api` surface.\n")
L.append(f"- Endpoints catalogued: **{len(entries)}**")
L.append(f"- Probed live on 2.32.00: **{called}**")
L.append(f"- In the 3.02 firmware but 404 on 2.32 (version gap): **{nf_live}**\n")

L.append("## Behaviour that shapes the proxy\n")
L.append("| Fact | Detail |\n|---|---|")
L.append(f"| CSRF on writes | {facts['csrf_on_writes']} |")
L.append(f"| CSRF on GETs | {facts['csrf_on_gets']} |")
L.append(f"| Method override | {facts['method_override']} |")
L.append(f"| Invalid session | `{facts['invalid_session']}` |")
L.append(f"| Unknown route | `{facts['unknown_route']}` |")
L.append(f"| Feature disabled | {facts['feature_disabled']} |")
L.append(f"| OPTIONS/HEAD | {facts['options_head']} |")
L.append(f"| **help-data crash** | {facts['help_data_crash']} |")
L.append(f"| Other backend-crashers | {', '.join(facts['html500_crashers'])} |")
L.append(f"| 502 handlers | {', '.join(facts['bad_gateway_502'])} |")
L.append(f"| Write 500 (payload fmt) | {', '.join(facts['write_500_payload_format'])} |")
L.append(f"| Unauth static symlinks | {', '.join(STATIC_PATHS)} (served by lighttpd; auth-bypass risk) |")
L.append(f"| Hidden accounts | {', '.join(facts['hidden_accounts'])} |")
L.append(f"| Suspected mem leak in response | {', '.join(facts['mem_leak_suspect'])} |")
L.append("")

def row(e):
    m = ",".join(e["methods_known"]) or "?"
    boards = ",".join(sorted(k.split("-")[0] for k, v in e["handler_present"].items() if v == "yes")) or "—"
    lv = e["live_2_32_00"]
    st = ",".join(str(s) for s in lv["statuses"]) if lv["called"] else "not probed"
    rt_ = ""
    if lv["roundtrip_writes"]:
        rt_ = " ·RT:" + ",".join(f"{w['method']}={w['status']}" for w in lv["roundtrip_writes"][:4])
    return f"| `{e['path']}` | {m} | {boards} | {st}{rt_} |"

for area in sorted(byarea):
    L.append(f"\n## {area} ({len(byarea[area])})\n")
    L.append("| Path | Methods | Handler (boards) | Live 2.32 status |\n|---|---|---|---|")
    for e in sorted(byarea[area], key=lambda x: x["path"]):
        L.append(row(e))

# detail section for endpoints with request fields or notable notes
L.append("\n## Field and purpose detail\n")
for area in sorted(byarea):
    rich = [e for e in byarea[area] if e["request_fields"] or (e["notes"] and len(e["notes"]) > 10)]
    if not rich: continue
    L.append(f"\n### {area}\n")
    for e in sorted(rich, key=lambda x: x["path"]):
        L.append(f"**`{e['path']}`** — {', '.join(e['methods_known']) or '?'}")
        if e["request_fields"]:
            L.append("  - request: " + "; ".join(e["request_fields"]))
        if e["response_fields"]:
            L.append("  - response: " + "; ".join(e["response_fields"][:12]))
        if e["live_2_32_00"]["sample_200"]:
            L.append("  - sample (redacted): `" + e["live_2_32_00"]["sample_200"] + "`")
        if e["notes"]:
            L.append("  - note: " + redact(e["notes"])[:400])
        L.append("")

open(f"{OUT}/endpoint-reference.md", "w").write("\n".join(L))
print("wrote endpoint-reference.{json,md} and endpoint-behaviour.json")
print("entries", len(entries), "called", called, "version-gap-404", nf_live)
