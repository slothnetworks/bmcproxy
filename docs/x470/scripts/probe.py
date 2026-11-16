#!/usr/bin/env python3
"""Sequential BMC /api prober. Usage: python3 -I probe.py plan.json out.jsonl
plan.json: list of {id, method, path, params?, json?, form?, data?, headers?, noauth?, raw_save?}
One session, one request at a time, >=250 ms gap, logout at end. Secrets are scrubbed."""
import json, os, re, sys, time, urllib3
import requests

urllib3.disable_warnings()
HOST = os.environ["BMC_HOST"]; USER = os.environ["BMC_USER"]; PASS = os.environ["BMC_PASS"]
BASE = f"https://{HOST}"
OUTDIR = os.path.dirname(os.path.abspath(sys.argv[2]))
RAW = os.path.join(OUTDIR, "raw"); os.makedirs(RAW, exist_ok=True)
SECRETS = set()
last = [0.0]
fails = [0]


def scrub(s):
    if not isinstance(s, str):
        return s
    for x in SECRETS:
        if x and len(x) >= 4:
            s = s.replace(x, "<REDACTED_SECRET>")
    if PASS:
        s = re.sub(r'(?<![A-Za-z0-9])' + re.escape(PASS) + r'(?![A-Za-z0-9])', "<REDACTED_PASSWORD>", s)
    s = re.sub(r'("?(CSRFToken|csrftoken|racsession_id|token|session_id|QSESSIONID|ViewerCsrftoken)"?\s*[:=]\s*"?)([A-Za-z0-9+/=_\-]{6,})', r'\1<REDACTED_TOKEN>', s)
    return s


def pace():
    d = time.time() - last[0]
    if d < 0.25:
        time.sleep(0.25 - d)


def do(sess, method, path, csrf=None, **kw):
    pace()
    h = dict(kw.pop("headers", {}) or {})
    h.setdefault("X-Requested-With", "XMLHttpRequest")
    if csrf:
        h["X-CSRFTOKEN"] = csrf
    t0 = time.time()
    err = None
    try:
        r = sess.request(method, BASE + path, headers=h, verify=False, timeout=90, allow_redirects=False, **kw)
    except Exception as e:  # timeout etc.
        r = None; err = repr(e)
    dt = int((time.time() - t0) * 1000)
    last[0] = time.time()
    if r is None or r.status_code >= 500:
        fails[0] += 1
        if fails[0] >= 3:
            print("  backoff 30s", file=sys.stderr); time.sleep(30); fails[0] = 0
    else:
        fails[0] = 0
    return r, dt, err


def login(sess):
    r, dt, err = do(sess, "POST", "/api/session", data={"username": USER, "password": PASS, "certlogin": "0"})
    j = r.json()
    for k in ("CSRFToken", "racsession_id"):
        if j.get(k):
            SECRETS.add(str(j[k]))
    for c in sess.cookies:
        SECRETS.add(c.value)
    return j.get("CSRFToken"), dt, r.status_code


def main():
    plan = json.load(open(sys.argv[1]))
    out = open(sys.argv[2], "a")
    sess = requests.Session()
    csrf, dt, st = login(sess)
    print(f"login {st} {dt}ms", file=sys.stderr)
    out.write(json.dumps({"id": "_login", "method": "POST", "path": "/api/session", "status": st, "latency_ms": dt}) + "\n")
    try:
        for p in plan:
            noauth = p.get("noauth")
            s2 = requests.Session() if noauth else sess
            kw = {}
            for k in ("params", "json", "data", "headers"):
                if k in p:
                    kw[k] = p[k]
            if "form" in p:
                kw["data"] = p["form"]
            r, dt, err = do(s2, p["method"], p["path"], None if noauth else csrf, **kw)
            relogin = False
            if r is not None and r.status_code == 401 and not noauth and not p.get("no_relogin"):
                try:
                    if 'Invalid Authentication' in r.text:
                        print("  session lost; re-login", file=sys.stderr)
                        sess.cookies.clear(); csrf, _, _ = login(sess); relogin = True
                        r, dt, err = do(sess, p["method"], p["path"], csrf, **kw)
                except Exception as e:
                    print("  relogin failed", e, file=sys.stderr)
            rec = {"id": p.get("id"), "method": p["method"], "path": p["path"], "params": p.get("params"),
                   "noauth": bool(noauth), "latency_ms": dt, "relogin": relogin}
            if r is None:
                rec.update(status=None, error=err)
            else:
                ct = r.headers.get("Content-Type", "")
                rec.update(status=r.status_code, headers={k: scrub(v) for k, v in r.headers.items()
                                                          if k.lower() in ("content-type", "allow", "content-length", "content-disposition", "location", "status", "server", "content-encoding")},
                           body_len=len(r.content))
                body = r.content
                txt = None
                try:
                    txt = body.decode("utf-8")
                except Exception:
                    pass
                if txt is not None and (len(txt) < 20000 or p.get("full")):
                    rec["body"] = scrub(txt)
                else:
                    fn = os.path.join(RAW, re.sub(r"[^A-Za-z0-9]+", "_", (p.get("id") or p["path"]))[:80] + ".bin")
                    if txt is not None:
                        open(fn, "w").write(scrub(txt))
                    else:
                        open(fn, "wb").write(body)
                    rec["body_file"] = fn
                    rec["body_head"] = scrub(txt[:3000]) if txt else repr(body[:200])
            out.write(json.dumps(rec) + "\n"); out.flush()
            print(f"{rec.get('status')} {dt:>6}ms {p['method']} {p['path']} {p.get('params') or ''}", file=sys.stderr)
    finally:
        r, dt, err = do(sess, "DELETE", "/api/session", csrf)
        print(f"logout {r.status_code if r is not None else err} {dt}ms", file=sys.stderr)
        out.write(json.dumps({"id": "_logout", "method": "DELETE", "path": "/api/session",
                              "status": r.status_code if r is not None else None, "latency_ms": dt,
                              "body": scrub(r.text) if r is not None else err}) + "\n")


if __name__ == "__main__":
    main()
