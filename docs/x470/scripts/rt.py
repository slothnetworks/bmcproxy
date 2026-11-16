#!/usr/bin/env python3
"""Round-trip write tests. python3 -I rt.py out_rt.jsonl"""
import json, sys, runpy, os
P = runpy.run_path(os.path.join(os.path.dirname(os.path.abspath(__file__)), "probe.py"), run_name="lib")
import requests
do, login, scrub = P["do"], P["login"], P["scrub"]
out = open(sys.argv[1], "a")
sess = requests.Session()
csrf, _, _ = login(sess)


def rec(tag, method, path, r, dt, body=None, extra=None):
    d = {"id": tag, "method": method, "path": path, "status": r.status_code if r is not None else None,
         "latency_ms": dt, "request_body": body, "body": scrub(r.text)[:6000] if r is not None else None}
    if extra:
        d.update(extra)
    out.write(json.dumps(d) + "\n"); out.flush()
    print(d["status"], dt, method, path, tag, (d["body"] or "")[:150].replace("\n", " "), file=sys.stderr)
    return r


def call(tag, method, path, body=None, csrf_override=None, headers=None):
    kw = {}
    if body is not None:
        kw["data"] = json.dumps(body)
        kw["headers"] = {"Content-Type": "application/json"}
    if headers:
        kw.setdefault("headers", {}).update(headers)
    r, dt, err = do(sess, method, path, csrf if csrf_override is None else csrf_override, **kw)
    return rec(tag, method, path, r, dt, body)


def j(r):
    try:
        return r.json()
    except Exception:
        return None


def roundtrip(tag, get_path, put_path, method="PUT", pick=None, transform=None):
    g = j(call(tag + ":before", "GET", get_path))
    if g is None:
        return None
    body = pick(g) if pick else g
    if transform:
        body = transform(body)
    call(tag + ":write_same", method, put_path, body)
    call(tag + ":after", "GET", get_path)
    return body


try:
    # header behaviour tests
    call("csrf:no_header_get", "GET", "/api/chassis-status", csrf_override="")
    call("override:get_as_delete", "GET", "/api/chassis-status", headers={"X-HTTP-Method-Override": "DELETE"})
    # no-op round trips
    lp = roundtrip("log-policy", "/api/settings/log-policy", "/api/settings/log-policy")
    # CSRF enforcement on writes (same value)
    if lp:
        call("csrf:bogus_put", "PUT", "/api/settings/log-policy", lp, csrf_override="bogus0000")
        call("csrf:missing_put", "PUT", "/api/settings/log-policy", lp, csrf_override="")
        # harmless toggle linear(0) <-> circular(1) and restore
        t = dict(lp); t["policy"] = 1 - int(lp["policy"])
        call("log-policy:toggle", "PUT", "/api/settings/log-policy", t)
        call("log-policy:verify_toggle", "GET", "/api/settings/log-policy")
        call("log-policy:restore", "PUT", "/api/settings/log-policy", lp)
        call("log-policy:verify_restore", "GET", "/api/settings/log-policy")
    roundtrip("log", "/api/settings/log", "/api/settings/log")
    roundtrip("date-time", "/api/settings/date-time", "/api/settings/date-time")
    roundtrip("video-pre-event", "/api/settings/video/pre-event", "/api/settings/video/pre-event")
    roundtrip("video-triggers", "/api/settings/video/triggers", "/api/settings/video/triggers")
    roundtrip("pef-event-filter-1", "/api/settings/pef/event_filters/1", "/api/settings/pef/event_filters/1", pick=lambda g: g[0])
    roundtrip("pef-alert-policy-1", "/api/settings/pef/alert_policies/1", "/api/settings/pef/alert_policies/1", pick=lambda g: g[0])
    roundtrip("pef-lan-dest-1", "/api/settings/pef/lan_destinations/1", "/api/settings/pef/lan_destinations/1", pick=lambda g: g[0])
    roundtrip("smtp-1", "/api/settings/smtp/1", "/api/settings/smtp/1", pick=lambda g: g[0])
    roundtrip("sensor-threshold-1", "/api/settings/sensor-threshold/1", "/api/settings/sensor-threshold/1")
    roundtrip("prochot", "/api/asrr/getprochot", "/api/asrr/setprochot", pick=lambda g: {"prochot": g["prochot"]})
    roundtrip("fan-mode", "/api/asrr/settings/getfancontrolmode", "/api/asrr/settings/setfancontrolmode", method="POST", pick=lambda g: g[0])
    # selector POSTs used by the UI before table reads
    call("openloop-select", "POST", "/api/asrr/settings/setopenlooptablenumber", {"table_numble": 1})
    call("openloop-read", "GET", "/api/asrr/settings/getfanopenloopcontroltable")
    call("closeloop-select", "POST", "/api/asrr/settings/setcloselooptablenumber", {"table_numble": 1})
    call("closeloop-read", "GET", "/api/asrr/settings/getfancloseloopcontroltable")
    call("getvideotag", "POST", "/api/logs/getvideotag", {"file_name": "nonexistent"})
finally:
    r, dt, err = do(sess, "DELETE", "/api/session", csrf)
    rec("_logout", "DELETE", "/api/session", r, dt)
