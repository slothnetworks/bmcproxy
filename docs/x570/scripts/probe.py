#!/usr/bin/env python3
"""X570D4U strictly read-only live probe.

Rules enforced in code (not just by convention):
  * /api: GET only, plus POST/DELETE /api/session (login/logout).
  * /redfish: GET only, plus POST/DELETE on SessionService/Sessions.
  * deny patterns (crash/flash/download/action) are refused before sending.
  * one request at a time on one keep-alive connection, paced.
Credentials come from the environment (BMC_HOST, BMC_USER, BMC_PASS) and are
never printed or written. Raw bodies go to raw-<phase>/ next to the script, so
copy it to a scratch directory first (raw bodies hold serials/MACs; shred them).
Usage: set -a; . ./x570d4u.env; set +a; python3 -I probe.py unauth|api|redfish
"""
import base64, http.client, json, os, re, socket, ssl, sys, time, urllib.parse

HOST = os.environ["BMC_HOST"]
USER = os.environ["BMC_USER"]
PASS = os.environ["BMC_PASS"]
WD = os.path.dirname(os.path.abspath(__file__))
RAW = os.path.join(WD, "raw-" + "-".join(sys.argv[1:] or ["all"]))
os.makedirs(RAW, exist_ok=True)
PACE = 0.2
RF_CAP = int(os.environ.get("RF_CAP", "450"))

DENY = re.compile(r"(help-data|mca-file|download_image|download_config|upgradestatus|verifyimagestatus|"
                  r"/maintenance/|/Actions/|/SSE|upload|java-console|kvm/token|viewerlogin|remote_control|"
                  r"event-file|video-data|GlobalLuaTables|bmccoldreset|flash|factory|restore_|/reset)", re.I)


def allowed(method, path):
    p = path.split("?")[0]
    if method == "GET":
        return True
    if p == "/api/session" and method in ("POST", "DELETE"):
        return True
    if re.match(r"^/redfish/v1/SessionService/Sessions(/[^/]+)?$", p) and method in ("POST", "DELETE"):
        return True
    return False


def ctx():
    c = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    c.check_hostname = False
    c.verify_mode = ssl.CERT_NONE
    return c


class Conn:
    def __init__(self):
        self.c = None
        self.n = 0
        self.log = []
        self.cookie = None      # in memory only
        self.handshakes = 0
        self._last_token = None

    def _connect(self):
        self.c = http.client.HTTPSConnection(HOST, 443, context=ctx(), timeout=60)
        self.handshakes += 1

    def req(self, method, path, body=None, headers=None, tag="", allow_gl=False, timeout=60, save=True, send_cookie=True):
        if not allowed(method, path):
            raise SystemExit(f"REFUSED (not read-only): {method} {path}")
        pp = path.split("?")[0]
        if DENY.search(pp) and not allow_gl and not pp.endswith("ActionInfo"):
            raise SystemExit(f"REFUSED (denylist): {method} {path}")
        time.sleep(PACE)
        hdrs = {"Accept": "application/json"}
        if send_cookie and self.cookie and path.startswith("/api"):
            hdrs["Cookie"] = "QSESSIONID=" + self.cookie
        hdrs.update(headers or {})
        r = data = None
        for attempt in (1, 2):
            if self.c is None:
                self._connect()
            t0 = time.time()
            try:
                if self.c.sock is not None:
                    self.c.sock.settimeout(timeout)
                else:
                    self.c.timeout = timeout
                self.c.request(method, path, body=body, headers=hdrs)
                r = self.c.getresponse()
                data = r.read()
                break
            except (http.client.RemoteDisconnected, ConnectionResetError, BrokenPipeError,
                    http.client.CannotSendRequest, http.client.ResponseNotReady) as e:
                self.c.close(); self.c = None
                if attempt == 2 or method != "GET":
                    rec = {"method": method, "path": path, "status": 0, "error": type(e).__name__, "tag": tag}
                    self.log.append(rec); return rec, b""
            except (socket.timeout, TimeoutError):
                self.c.close(); self.c = None
                rec = {"method": method, "path": path, "status": 0, "error": "timeout",
                       "ms": round((time.time() - t0) * 1000), "tag": tag}
                self.log.append(rec); return rec, b""
        ms = round((time.time() - t0) * 1000)
        self.n += 1
        h = {}
        for k, v in r.getheaders():
            kl = k.lower()
            if kl == "set-cookie":
                name, _, rest = v.partition("=")
                val, _, attrs = rest.partition(";")
                if name.strip() == "QSESSIONID" and tag == "login":
                    self.cookie = val.strip()
                v = f"{name}=<REDACTED len={len(val.strip())}>;{attrs}"
            if kl == "x-auth-token":
                self._last_token = v
                v = f"<REDACTED len={len(v)}>"
            h.setdefault(k, []).append(v)
        rec = {"i": self.n, "method": method, "path": path, "status": r.status, "ms": ms, "bytes": len(data),
               "headers": h, "tag": tag}
        if save:
            fn = os.path.join(RAW, f"{self.n:04d}.body")
            open(fn, "wb").write(data)
            rec["file"] = fn
        self.log.append(rec)
        if (r.getheader("Connection") or "").lower() == "close":
            self.c.close(); self.c = None
        return rec, data

    def dump(self):
        json.dump(self.log, open(os.path.join(WD, "log.json"), "w"), indent=1)


def j(data):
    try:
        return json.loads(data.decode("utf-8", "replace"))
    except Exception:
        return None


API_GETS = [
    # identity
    "asrr/model-name", "asrr/fw-info", "asrr/product-name", "firmware-info", "configuration/project",
    "configuration/runtime", "extfeatures", "chassis-status", "chassis", "status/uptime",
    # sensors
    "sensors", "detail_sensors_readings", "sensors/basic", "sdr", "settings/sensor-threshold",
    # logs (eventlog/event handled separately)
    "logs/audit", "logs/dashboardevent", "logs/postcode",
    # fru / inventory
    "fru",
    "system_inventory/device_count", "system_inventory/cpu_info", "system_inventory/dimm_info", "system_inventory/pci_info",
    "host_inventory/host_interface_system_info", "host_inventory/host_interface_processor_info",
    "host_inventory/host_interface_memory_info", "host_inventory/host_interface_pcie_device_function_info",
    "host_inventory/host_interface_storage_info", "host_inventory/host_interface_baseboard_info",
    "host_inventory/host_interface_power_info", "host_inventory/host_interface_thermal_info",
    "asrr/smbiosdata",
    # network
    "settings/network", "settings/network-bond", "settings/network-link", "settings/ncsi/mode",
    "settings/ncsi-interfaces", "settings/static-ipv6", "settings/dns-info",
    # users / services / time / ssl
    "settings/users", "settings/users_read_only", "settings/services", "settings/date-time",
    "settings/ssl/certificate-info", "settings/pam-order",
    # OEM reads the driver uses
    "asrr/getvetobit", "asrr/settings/getfancontrolmode", "asrr/settings/getfanopenloopcontroltable",
    "asrr/settings/getfancloseloopcontroltable", "asrr/settings/getfandutymanualmode", "asrr/settings/getsupportfan",
    "asrr/settings/getsupportfanname", "asrr/settings/gettscftsupportfan",
    "asrr/settings/getfantempsensorandcorrespondfantable", "asrr/settings/getfan100druingbmcupdate",
    "asrr/GetPowerRestore", "asrr/PowerSource", "asrr/getbioscode", "asrr/getmcalog", "asrr/getWebEnableTimeout",
    # expected-absent handlers (dialect confirmation; unknown paths answer 404 code 1010)
    "asrr/GetFanDuty", "asrr/getprochot", "settings/fan/mode", "settings/ncsilinkup", "nosuchthing12345",
]


def api_phase(c, out):
    form = urllib.parse.urlencode({"username": USER, "password": PASS})
    rec, data = c.req("POST", "/api/session", body=form, tag="login", save=False,
                      headers={"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
                               "X-Requested-With": "XMLHttpRequest"}, send_cookie=False)
    login = j(data) or {}
    tok = login.get("CSRFToken") if isinstance(login, dict) else None
    red = {k: ("<REDACTED len=%d>" % len(str(v)) if k in ("CSRFToken",) else v) for k, v in login.items()} if isinstance(login, dict) else login
    open(os.path.join(RAW, "login.json"), "w").write(json.dumps(red, indent=1))
    rec["file"] = os.path.join(RAW, "login.json")
    out["login"] = {"status": rec["status"], "ms": rec["ms"], "keys": sorted(login.keys()) if isinstance(login, dict) else None}
    if rec["status"] != 200 or not tok:
        print("API LOGIN FAILED", rec["status"]); return False
    A = {"X-CSRFTOKEN": tok, "X-Requested-With": "XMLHttpRequest"}
    try:
        # CSRF behaviour on GETs
        c.req("GET", "/api/asrr/fw-info", headers={"X-Requested-With": "XMLHttpRequest"}, tag="csrf-missing")
        c.req("GET", "/api/asrr/fw-info", headers={"X-CSRFTOKEN": "AAAAAAAA", "X-Requested-With": "XMLHttpRequest"}, tag="csrf-bogus")
        c.req("GET", "/api/asrr/fw-info", headers=A, tag="csrf-nocookie", send_cookie=False)
        c.req("GET", "/api/asrr/fw-info", headers={"Authorization": "Basic " + base64.b64encode(f"{USER}:{PASS}".encode()).decode()},
              tag="api-basic", send_cookie=False)
        for p in API_GETS:
            c.req("GET", "/api/" + p, headers=A, tag="api")
        # events: full dump once (for max id), then the incremental read the driver uses
        if os.environ.get("SKIP_SLOW"):
            c.req("GET", "/api/logs/event?LASTEVENTID=3634", headers=A, tag="api-event-incr")
            c.req("GET", "/api/logs/event?LASTEVENTID=3639", headers=A, tag="api-event-incr-empty")
            return True
        rec, data = c.req("GET", "/api/logs/eventlog", headers=A, tag="api-eventlog", timeout=90)
        ev = j(data)
        ids = [e.get("id") for e in ev if isinstance(e, dict) and isinstance(e.get("id"), int)] if isinstance(ev, list) else []
        mx = max(ids) if ids else 0
        out["sel"] = {"count": len(ids), "max_id": mx, "trailer": [e for e in (ev or []) if isinstance(e, dict) and "id" not in e][:2] if isinstance(ev, list) else None}
        c.req("GET", "/api/logs/event?LASTEVENTID=%d" % max(mx - 5, 0), headers=A, tag="api-event-incr")
        c.req("GET", "/api/logs/event?LASTEVENTID=%d" % mx, headers=A, tag="api-event-incr-empty")
        c.req("GET", "/api/logs/eventlog?start=0&count=10", headers=A, tag="api-eventlog-query", timeout=90)
        # slow one last
        c.req("GET", "/api/asrr/selsummary", headers=A, tag="api-selsummary", timeout=45)
        c.req("GET", "/api/chassis-status", headers=A, tag="api-after-selsummary")
    finally:
        rec, _ = c.req("DELETE", "/api/session", headers=A, tag="logout")
        out["logout"] = rec["status"]
        r2, _ = c.req("GET", "/api/chassis-status", headers=A, tag="after-logout")
        out["after_logout"] = r2["status"]
        c.cookie = None
    return True


def links(obj, acc):
    if isinstance(obj, dict):
        for k, v in obj.items():
            if k in ("@odata.id", "@Redfish.ActionInfo", "Members@odata.nextLink") and isinstance(v, str):
                acc.append(v)
            elif k == "Actions" and isinstance(v, dict):
                continue
            else:
                links(v, acc)
    elif isinstance(obj, list):
        for x in obj:
            links(x, acc)


SKIP_RF = re.compile(r"(/Actions/|/SSE$|upload|GlobalLuaTables|/JsonSchemas/.+|/Registries/.+|#)", re.I)


def redfish_phase(c, out):
    B = {"Authorization": "Basic " + base64.b64encode(f"{USER}:{PASS}".encode()).decode()}
    rec, data = c.req("GET", "/redfish/v1/Systems/Self", headers=B, tag="rf-basic-auth")
    out["redfish_basic"] = rec["status"]
    if rec["status"] == 401:
        # one session attempt only (lockout is 5 failures)
        body = json.dumps({"UserName": USER, "Password": PASS})
        rec, data = c.req("POST", "/redfish/v1/SessionService/Sessions", body=body, save=False,
                          headers={"Content-Type": "application/json"}, tag="rf-session-create")
        out["redfish_session"] = rec["status"]
        return False
    # one session round trip (create, use, delete) to see token/Location shape
    body = json.dumps({"UserName": USER, "Password": PASS})
    rec, data = c.req("POST", "/redfish/v1/SessionService/Sessions", body=body, save=False,
                      headers={"Content-Type": "application/json"}, tag="rf-session-create")
    hdrs = {}
    # http.client: token is only in the live response; grab it from the redacted header? -> re-read raw
    out["redfish_session"] = rec["status"]
    sess = j(data) or {}
    sess_red = {k: v for k, v in sess.items() if k not in ("Password",)} if isinstance(sess, dict) else None
    open(os.path.join(RAW, "session.json"), "w").write(json.dumps(sess_red, indent=1))
    tok = c._last_token if hasattr(c, "_last_token") else None
    loc = (rec["headers"].get("Location") or [None])[0]
    if rec["status"] in (200, 201) and loc:
        if tok:
            c.req("GET", "/redfish/v1/SessionService", headers={"X-Auth-Token": tok}, tag="rf-session-use")
            r2, _ = c.req("DELETE", urllib.parse.urlparse(loc).path, headers={"X-Auth-Token": tok}, tag="rf-session-delete")
        else:
            r2, _ = c.req("DELETE", urllib.parse.urlparse(loc).path, headers=B, tag="rf-session-delete")
        out["redfish_session_delete"] = r2["status"]
    # behaviour tests
    for p, tag in [("/redfish/v1/Chassis/Self?%24select=Id", "q-select"),
                   ("/redfish/v1/AccountService/Accounts?%24expand=.(%24levels%3D1)", "q-expand"),
                   ("/redfish/v1/Systems/Self/LogServices/SEL/Entries?%24top=2", "q-top"),
                   ("/redfish/v1/Chassis/Self/Sensors?%24skip=50", "q-skip"),
                   ("/redfish/v1/Chassis/Self?%24foobar=1", "q-unknown"),
                   ("/redfish/v1/NoSuchThing12345", "rf-404"),
                   ("/redfish/v1/AccountService/Oem/Ami/Configurations", "rf-pam"),
                   ("/redfish/v1/AccountService/Configurations", "rf-pam2")]:
        c.req("GET", p, headers=B, tag=tag)
    # BFS crawl
    seen = set()
    queue = ["/redfish/v1/"]
    inv = []
    while queue and len(seen) < RF_CAP:
        u = queue.pop(0)
        if u in seen:
            continue
        seen.add(u)
        up = u.split("?")[0]
        if SKIP_RF.search(u) or (DENY.search(up) and not up.endswith("ActionInfo")):
            inv.append({"uri": u, "status": "skipped"})
            continue
        rec, data = c.req("GET", u, headers=B, tag="crawl")
        body = j(data)
        inv.append({"uri": u, "status": rec["status"], "i": rec.get("i")})
        if rec["status"] == 200 and body is not None:
            acc = []
            links(body, acc)
            for l in acc:
                l = l.split("#")[0]
                if l.startswith("/redfish/v1") and l not in seen:
                    queue.append(l)
    out["crawl"] = {"fetched": len([x for x in inv if x["status"] != "skipped"]), "queued_left": len(queue)}
    json.dump(inv, open(os.path.join(WD, "crawl.json"), "w"), indent=1)
    return True


def main():
    c = Conn()
    # capture X-Auth-Token unredacted in memory only
    orig = c.req

    def req(*a, **kw):
        rec, data = orig(*a, **kw)
        return rec, data
    out = {"started": time.strftime("%Y-%m-%dT%H:%M:%S%z")}
    phases = sys.argv[1:] or ["unauth", "api", "redfish"]
    try:
        if "unauth" in phases:
            for p in ["/redfish/v1/", "/redfish/v1", "/redfish", "/redfish/v1/odata", "/redfish/v1/$metadata",
                      "/redfish/v1/Systems", "/redfish/v1/NoSuchThing12345", "/api/asrr/model-name",
                      "/redfish/v1/?%24select=RedfishVersion"]:
                c.req("GET", p, tag="unauth")
            c.req("GET", "/redfish/v1/GlobalLuaTables", tag="unauth-gl", allow_gl=True)
        if "api" in phases:
            api_phase(c, out)
        if "redfish" in phases:
            redfish_phase(c, out)
    finally:
        out["requests"] = c.n
        out["handshakes"] = c.handshakes
        c.dump()
        json.dump(out, open(os.path.join(WD, "summary-%s.json" % "-".join(phases)), "w"), indent=1)
        print(json.dumps(out, indent=1))


if __name__ == "__main__":
    main()
