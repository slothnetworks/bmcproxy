import json, sys, os, runpy, requests
P = runpy.run_path(os.path.join(os.path.dirname(os.path.abspath(__file__)), "probe.py"), run_name="lib")
do, login, scrub = P["do"], P["login"], P["scrub"]
A = requests.Session(); B = requests.Session()
ca, _, _ = login(A); cb, _, _ = login(B)
def g(s, c, path, tag, **kw):
    r, dt, e = do(s, "GET", path, c, **kw)
    print(tag, r.status_code if r is not None else e, dt, (r.text[:120] if r is not None else '').replace('\n', ' '))
    return r
g(A, ca, "/api/chassis-status", "A_pre"); g(B, cb, "/api/chassis-status", "B_pre")
g(A, ca, "/api/logs/help-data", "A_crash")
ra = g(A, ca, "/api/chassis-status", "A_post"); rb = g(B, cb, "/api/chassis-status", "B_post")
for s, c, t in ((A, ca, "A"), (B, cb, "B")):
    r, dt, e = do(s, "DELETE", "/api/session", c); print(t, "logout", r.status_code if r is not None else e)
