#!/usr/bin/env python3
"""Build the sanitized docs/x570/{live-redfish-inventory.json,har-endpoints.json}
from the probe.py logs (log-*.json + raw-*/) in the scratch dir holding this
script. Usage: python3 -I build_docs.py <repo root>  (BMC_HOST in env, only
used to scrub)."""
import json, os, re, sys, statistics, ipaddress

WD = os.path.dirname(os.path.abspath(__file__))
REPO = sys.argv[1]
OUT = os.path.join(REPO, "docs", "x570")
os.makedirs(OUT, exist_ok=True)
HOST = os.environ["BMC_HOST"]  # only used to scrub; never written

ipmap = {}


def mapip(m):
    ip = m.group(0)
    try:
        a = ipaddress.ip_address(ip)
    except ValueError:
        return ip
    if not any(a in ipaddress.ip_network(n) for n in ("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16")):
        return ip
    if ip not in ipmap:
        ipmap[ip] = "192.0.2.%d" % (10 + len(ipmap))
    return ipmap[ip]


MAC = re.compile(r"\b[0-9A-Fa-f]{2}([:-])(?:[0-9A-Fa-f]{2}\1){4}[0-9A-Fa-f]{2}\b")
UUID = re.compile(r"\b[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\b")
IPV4 = re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b")
HOSTN = re.compile(r"\bAMI[0-9A-F]{12}\b")
SENSITIVE_KEYS = re.compile(r"(serial|uuid|guid|csrf|token|password|racsession|session_id|sessionid|macaddress|mac_address|^mac$|_mac$|hwaddr|permanentmac|asset_tag|assettag|hostname|host_name|fqdn|domainname|domain_name|\bip\b|_ip$|^ip_|ipaddress|ipv4|ipv6|gateway|address$|dns|server_name|server_addr|remote_addr|email)", re.I)
KEEP_KEYS = re.compile(r"^(ipmi_version|SerialConsole|serialoverlan|Serial_Console)$", re.I)
seen_serials = set()


def s_str(v):
    v = MAC.sub("<REDACTED_MAC>", v)
    v = UUID.sub("<REDACTED_UUID>", v)
    v = HOSTN.sub("<REDACTED_HOSTNAME>", v)
    v = IPV4.sub(mapip, v)
    for ser in seen_serials:
        if ser and len(ser) >= 6:
            v = v.replace(ser, "<REDACTED_SERIAL>")
    return v


def collect_serials(o, k=""):
    if isinstance(o, dict):
        for kk, vv in o.items():
            collect_serials(vv, kk)
    elif isinstance(o, list):
        for x in o:
            collect_serials(x, k)
    elif isinstance(o, str) and re.search(r"serial|uuid|guid", k, re.I) and o.strip() and o.strip().upper() not in ("N/A", "NA", "TO BE FILLED BY O.E.M.", "DEFAULT STRING", "NOT AVAILABLE", "NOT SPECIFIED") and not set(o.strip()) <= set("0 "):
        seen_serials.add(o.strip())


def tag_for(k):
    kl = k.lower()
    for word, tag in (("serial", "SERIAL"), ("uuid", "UUID"), ("guid", "UUID"), ("csrf", "CSRF"), ("token", "TOKEN"),
                      ("password", "PASSWORD"), ("racsession", "SESSION"), ("session", "SESSION"), ("mac", "MAC"),
                      ("hwaddr", "MAC"), ("asset", "ASSET_TAG"), ("email", "EMAIL"), ("host", "HOSTNAME"),
                      ("domain", "DOMAIN")):
        if word in kl:
            return tag
    return None


def san(o, k=""):
    if isinstance(o, dict):
        return {kk: san(vv, kk) for kk, vv in o.items()}
    if isinstance(o, list):
        return [san(x, k) for x in o]
    if isinstance(o, str):
        if k and not KEEP_KEYS.match(k) and o.strip() and o.strip().upper() not in ("N/A", "NA", ""):
            t = tag_for(k)
            if t and t not in ("HOSTNAME", "DOMAIN") or (t in ("HOSTNAME", "DOMAIN") and HOSTN.search(o)):
                if not (t == "SESSION" and k.lower() in ("sessiontimeout", "session_timeout")):
                    return "<REDACTED_%s>" % t
        return s_str(o)
    return o


def body(e):
    f = e.get("file")
    if not f or not os.path.exists(f):
        return None
    raw = open(f, "rb").read()
    try:
        return json.loads(raw.decode("utf-8", "replace"))
    except ValueError:
        return {"_non_json": raw[:200].decode("utf-8", "replace")}


def hdr(e, name):
    for k, v in (e.get("headers") or {}).items():
        if k.lower() == name.lower():
            return v[0] if isinstance(v, list) else v
    return None


def path_only(p):
    return s_str(re.sub(r"Sessions/[0-9a-f]{16,}", "Sessions/<REDACTED_SESSION>", p))


# ---------- pass 1: collect serial-like values everywhere ----------
logs = {n: json.load(open(os.path.join(WD, n))) for n in ("log-redfish.json", "log-api2.json", "log-unauth.json")}
for n in ("log-redfish.json", "log-api2.json"):
    for e in logs[n]:
        b = body(e)
        if b is not None:
            collect_serials(b)
seen_serials.discard(HOST)

# ---------- live-redfish-inventory.json ----------
rf = logs["log-redfish.json"]
root = next(body(e) for e in rf if e.get("path") == "/redfish/v1/" and e.get("status") == 200)
root_etag = next(hdr(e, "ETag") for e in rf if e.get("path") == "/redfish/v1/" and e.get("status") == 200)
resources = []
for e in rf:
    b = body(e) if e.get("status") == 200 else None
    notes = []
    allow = hdr(e, "Allow")
    if allow:
        notes.append("Allow: " + allow)
    et = hdr(e, "ETag")
    if et and et != root_etag:
        notes.append("ETag %s (differs from service root)" % et)
    if e.get("error"):
        notes.append("client %s after %d ms (no response)" % (e["error"], e["ms"]))
    elif e["ms"] > 5000:
        notes.append("slow: %d ms" % e["ms"])
    if e.get("tag") and e["tag"] != "crawl":
        notes.append("probe: " + e["tag"])
    if isinstance(b, dict):
        if "Members@odata.count" in b:
            notes.append("Members@odata.count %s, %d inline" % (b["Members@odata.count"], len(b.get("Members", []))))
        if "Members@odata.nextLink" in b:
            notes.append("nextLink " + path_only(b["Members@odata.nextLink"]))
        if e["status"] >= 400 or "error" in b:
            ext = (b.get("error") or {}).get("@Message.ExtendedInfo") or []
            if ext:
                notes.append("error " + ", ".join(x.get("MessageId", "") for x in ext))
    elif e.get("status", 0) >= 400:
        bb = body(e)
        if isinstance(bb, dict) and "error" in bb:
            ext = bb["error"].get("@Message.ExtendedInfo") or []
            notes.append("error " + ", ".join(x.get("MessageId", "") for x in ext))
    resources.append({
        "uri": path_only(e["path"]),
        "method": e["method"],
        "status": e["status"],
        "type": b.get("@odata.type") if isinstance(b, dict) else None,
        "etag": et,
        "etag_differs": bool(et and et != root_etag),
        "actions": sorted((b.get("Actions") or {}).keys()) if isinstance(b, dict) else [],
        "latency_ms": e["ms"],
        "notes": "; ".join(notes) or None,
    })
unauth = []
for e in logs["log-unauth.json"]:
    unauth.append({"uri": path_only(e["path"]), "method": e["method"], "status": e["status"], "latency_ms": e["ms"],
                   "www_authenticate": hdr(e, "WWW-Authenticate"), "location": hdr(e, "Location"), "tag": e.get("tag")})
lat = [e["ms"] for e in rf if e.get("status") == 200]
lat.sort()
inv = {
    "_meta": {
        "board": "X570D4U", "bmc_fw": "1.20.00", "bios": "L1.57", "host_power": "off",
        "captured": "2026-11-16, docs/x570/scripts/probe.py (read-only: GET only, plus one SessionService session create/delete)",
        "requests": len(rf), "crawl_cap": 450,
        "latency_ms_200": {"median": statistics.median(lat), "p95": lat[int(len(lat) * 0.95)], "max": lat[-1]},
        "service_root_etag": root_etag,
        "redaction": "UUIDs, serials, MACs, session ids and host names are <REDACTED_*>; private IPv4 mapped to 192.0.2.0/24. No credentials were ever written.",
    },
    "service_root": san(root),
    "unauthenticated": unauth,
    "resources": resources,
}
json.dump(inv, open(os.path.join(OUT, "live-redfish-inventory.json"), "w"), indent=1, ensure_ascii=False)

# ---------- har-endpoints.json ----------
api = logs["log-api2.json"]
groups = {}
order = []
for e in api:
    p = e["path"]
    path, _, q = p.partition("?")
    key = (e["method"], path)
    if key not in groups:
        groups[key] = []
        order.append(key)
    groups[key].append((e, q))

NOTES = {
    ("GET", "/api/asrr/fw-info"): "Calls 1-4 (401 cc:7) are the CSRF checks: no X-CSRFTOKEN, a bogus token, no cookie, and HTTP Basic instead of the cookie. X570 1.20 enforces the token on GETs (X470 2.32 / B650 4.10 accept any token on GETs).",
    ("DELETE", "/api/session"): "Returned a lighttpd HTML 500 in this run while the session stayed valid (next GET 200). An earlier run got 200 {\"ok\":0}. The proxy retries a 5xx logout once.",
    ("GET", "/api/host_inventory/host_interface_power_info"): "200 in the first run; lighttpd HTML 500 here, with the session still valid afterwards.",
    ("GET", "/api/logs/event"): "LASTEVENTID=3634 returns ids 3639..3635, newest first; LASTEVENTID=3639 returns []. No event_log_count trailer.",
    ("GET", "/api/asrr/settings/getfan100druingbmcupdate"): "404 on 1.20; the route exists in the 2.11.00 image.",
    ("GET", "/api/asrr/GetFanDuty"): "404: absent on X570 (present on X470).",
    ("GET", "/api/settings/fan/mode"): "404: X570 uses the X470 asrr/settings/* fan dialect.",
    ("GET", "/api/settings/ncsilinkup"): "404: X570 uses asrr/getvetobit.",
    ("GET", "/api/system_inventory/cpu_info"): "500 code 11272 with the host powered off.",
    ("GET", "/api/date-time"): "",
}
TRUNC = 6  # keep the first N elements of long arrays
endpoints = []
for key in order:
    items = groups[key]
    e0 = next((e for e, _ in items if e["status"] == 200), items[0][0])
    st = {}
    for e, _ in items:
        st[str(e["status"])] = st.get(str(e["status"]), 0) + 1
    qs = []
    for _, q in items:
        if q:
            d = {}
            for kv in q.split("&"):
                k, _, v = kv.partition("=")
                d[k] = v
            qs.append(d)
    b = body(e0)
    note = NOTES.get(key, "")
    if isinstance(b, list) and len(b) > TRUNC:
        note = (note + " " if note else "") + "response_example truncated to the first %d of %d elements." % (TRUNC, len(b))
        b = b[:TRUNC]
    if isinstance(b, dict) and "_non_json" in b:
        b = {"_non_json": "<lighttpd HTML error page>"} if "html" in b["_non_json"].lower() else b
    req_body = None
    if key == ("POST", "/api/session"):
        req_body = {"username": "admin", "password": "<REDACTED_PASSWORD>"}
    hdrs = {"Accept": "application/json"}
    if key != ("POST", "/api/session"):
        hdrs["X-CSRFTOKEN"] = "<REDACTED_CSRF>"
    else:
        hdrs["Content-Type"] = "application/x-www-form-urlencoded; charset=UTF-8"
    endpoints.append({
        "method": key[0], "path": key[1], "query": qs, "calls": len(items), "status": st,
        "response_content_type": (hdr(e0, "Content-Type") or "").split(";")[0] or None,
        "response_body_bytes_first": e0.get("bytes"),
        "timing_ms": {"min": min(e["ms"] for e, _ in items), "max": max(e["ms"] for e, _ in items)},
        "request_example": {"headers": hdrs, "body": req_body},
        "response_example": san(b),
        "note": note,
    })
# The slow SEL reads were measured in the first API run (log-api.json; bodies not kept there).
first = json.load(open(os.path.join(WD, "log-api.json")))
for e in first:
    if e["path"].split("?")[0] in ("/api/logs/eventlog", "/api/asrr/selsummary"):
        endpoints.append({
            "method": e["method"], "path": e["path"].split("?")[0], "query": [], "calls": 1, "status": {str(e["status"]): 1},
            "response_content_type": (hdr(e, "Content-Type") or "").split(";")[0] or None,
            "response_body_bytes_first": e.get("bytes"), "timing_ms": {"min": e["ms"], "max": e["ms"]},
            "request_example": {"headers": {"Accept": "application/json", "X-CSRFTOKEN": "<REDACTED_CSRF>"}, "body": None},
            "response_example": ({"error": "Error in get SEL summary", "code": 1007} if "selsummary" in e["path"] else None),
            "note": ("First API run; body not retained. 3639 SEL records, newest first; query strings ignored; no event_log_count trailer; early records have negative timestamps." if "eventlog" in e["path"]
                     else "Hangs ~30 s, then 500 code 1007 (first API run)."),
        })
        break_once = True
seen = set()
dedup = []
for ep in endpoints:
    k = (ep["method"], ep["path"], json.dumps(ep["query"]))
    if k in seen:
        continue
    seen.add(k)
    dedup.append(ep)
har = {
    "source": "docs/x570/scripts/probe.py live read-only probe (api phase, 74 requests, 2026-11-16), not a browser HAR; same digest shape as docs/x470/har-endpoints.json",
    "bmc": {"board": "X570D4U", "bmc_fw": "1.20.00", "bios": "L1.57", "server": "lighttpd", "http": "HTTP/1.1 over TLS 1.3",
            "host": "<BMC_HOST> (RFC1918 address in capture)"},
    "redaction": "Passwords, CSRF tokens, session ids, serial numbers, UUIDs, host names and MAC addresses replaced with <REDACTED_*> placeholders; private IPv4 addresses mapped to 192.0.2.0/24; long arrays truncated (see note).",
    "auth": {
        "login": "POST /api/session (application/x-www-form-urlencoded: username, password; no certlogin), ~2.9 s",
        "csrf_header": "X-CSRFTOKEN: <CSRFToken from login response> (enforced on GET too: missing/bogus -> 401 cc:7)",
        "session_cookie": "QSESSIONID=<30 chars>; path=/; secure;HttpOnly (no SameSite)",
        "logout": "DELETE /api/session -> 200 {\"ok\":0} (one run: lighttpd HTML 500, session still valid)",
    },
    "endpoints": dedup,
}
json.dump(har, open(os.path.join(OUT, "har-endpoints.json"), "w"), indent=1, ensure_ascii=False)
print("ipmap size", len(ipmap), "serials", len(seen_serials), "resources", len(resources), "endpoints", len(dedup))
