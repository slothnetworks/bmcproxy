#!/usr/bin/env python3
"""Add the X570D4U 2.11.00 column to docs/api/legacy-api-endpoints.json.

Same method as handlers.py used for X470/B650: `strings` on the shipped
/api FastCGI binary and its plugins, keep the route strings (relative to
/api), and mark each catalog path handler_present "yes"/"no". Paths with an
":id" suffix match their base route. X570 routes missing from the catalog are
appended (boards/handler_present filled for every board with the same
strings method, using the dumped images under firmware-dumped/).

Usage: python3 -I docs/api/scripts/x570_handlers.py   (from the repo root)
"""
import json, os, re, subprocess, gzip

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
CAT = os.path.join(ROOT, "docs", "api", "legacy-api-endpoints.json")
DUMPS = os.path.join(ROOT, "firmware-dumped")
COL = "X570-2.11"

PREFIXES = ("actions|asrr|logs|settings|maintenance|session|sensors|sdr|fru|chassis|configuration|firmware-info|kvm|"
            "host_inventory|system_inventory|status|tasks|remote_control|newtabinfo|reset-pass|operations|set|scripts|"
            "extfeatures|file|previewer|frueditable|fruupdate|smbiosdata|snmp|viewerlogin|sol|ae|ae_test|dashboardsensors|"
            "detail_sensors_readings|chassis-status|updatenew_password|verify_otp|tfa|config|hsc-info")
ROUTE = re.compile(r"^/(" + PREFIXES + r")(/[A-Za-z0-9_.-]+)*$")


def strings(path):
    out = subprocess.run(["strings", "-n", "3", path], capture_output=True, text=True, check=True).stdout
    return out.splitlines()


def routes(rootfs):
    bins = [os.path.join(rootfs, "usr/local/bin/spx_restservice")]
    lib = os.path.join(rootfs, "usr/local/lib")
    for f in sorted(os.listdir(lib)):
        if re.match(r"lib(asrr)?spx_restservice.*\.so\.\d+\.\d+\.\d+$", f) and not os.path.islink(os.path.join(lib, f)):
            bins.append(os.path.join(lib, f))
    found = set()
    for b in bins:
        for s in strings(b):
            s = s.strip()
            if not s.startswith("/"):
                # ASRock stores maintenance routes without the leading '/'
                if s.startswith("asrr/maintenance/"):
                    s = "/" + s
                else:
                    continue
            if ROUTE.match(s):
                found.add("/api" + s.rstrip("/"))
    return found


def ui_paths(www):
    src = os.path.join(www, "source.min.js")
    raw = open(src, "rb").read()
    try:
        raw = gzip.decompress(raw)
    except OSError:
        pass
    txt = raw.decode("utf-8", "replace")
    return {"/" + m.lstrip("/").rstrip("/") for m in re.findall(r'["\'](/?api/[A-Za-z0-9_/.-]+)', txt)}


def main():
    x570 = os.path.join(DUMPS, "X570D4U_2.11.00")
    h570 = routes(os.path.join(x570, "rootfs"))
    ui570 = ui_paths(os.path.join(x570, "www"))
    others = {
        "X470-3.02": routes(os.path.join(DUMPS, "X470D4U_3.02.00", "rootfs")),
        "B650-7.02": routes(os.path.join(DUMPS, "B650D4U_7.02.00", "rootfs")),
        "B650-9.05": routes(os.path.join(DUMPS, "B650D4U_2L2T_9.05.00", "rootfs")),
    }
    cat = json.load(open(CAT))
    known = set()
    for e in cat:
        p = e["path"]
        base = re.sub(r"/:id$", "", p).rstrip("/")
        known.add(base)
        e["handler_present"][COL] = "yes" if base in h570 else "no"
        if base in ui570 and COL not in e["boards"]:
            e["boards"].append(COL)
    added = 0
    for p in sorted(h570 - known):
        hp = {b: ("yes" if p in s else "no") for b, s in others.items()}
        hp[COL] = "yes"
        cat.append({
            "path": p, "methods": [], "boards": [COL] if p in ui570 else [],
            "area": "X570 additions", "request_fields": [], "response_fields": [],
            "handler_present": hp,
            "seen_in_har": {"x470d4u.har": [], "b650d4u.har": [], "b650d4u-2t.har": []},
            "ui": [],
            "notes": "Route string found in the X570D4U 2.11.00 /api binaries (x570_handlers.py); not in the original UI-derived catalog. Methods unknown from strings.",
        })
        added += 1
    json.dump(cat, open(CAT, "w", encoding="utf-8"), indent=1, ensure_ascii=False)
    yes = sum(1 for e in cat if e["handler_present"][COL] == "yes")
    print(f"x570 routes={len(h570)} ui={len(ui570)} catalog={len(cat)} yes={yes} added={added}")


if __name__ == "__main__":
    main()
