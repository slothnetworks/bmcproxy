#!/usr/bin/env python3
import json, re, os
WD=os.path.dirname(os.path.abspath(__file__))
DOCS="/opt/sparked/asrockrack-api/docs"
static=json.load(open(f"{DOCS}/redfish/b650-uri-inventory.json"))
live=json.load(open(f"{WD}/rf_inventory_raw.json"))
live_uris=[r["uri"] for r in live if isinstance(r.get("status"),int)]  # 200 or 404

# static patterns present in b650 firmware
pats=[s for s in static if s.get("firmware",{}).get("b650_702") or s.get("firmware",{}).get("b650_905")]

def to_regex(uri):
    # turn {X} into [^/]+ ; ignore descriptive/dynamic entries
    u=uri.strip()
    if any(c in u for c in [" ","(","*","..."]) or u.endswith("Deep") or "{any}" in u:
        return None
    segs=u.split("/")
    out=[]
    for s in segs:
        if "{" in s:
            # escape literal bits, replace {..} tokens with [^/]+
            parts=re.split(r"(\{[^}]+\})", s)
            seg="".join("[^/]+" if (p.startswith("{") and p.endswith("}")) else re.escape(p) for p in parts)
            out.append(seg)
        else:
            out.append(re.escape(s))
    return re.compile("^"+"/".join(out)+"$")

# match: which static patterns have >=1 live uri
matched_pat=set(); live_matched=set()
pat_re=[]
for s in pats:
    rx=to_regex(s["uri"])
    pat_re.append((s["uri"],rx))
for lu in live_uris:
    for name,rx in pat_re:
        if rx and rx.match(lu):
            matched_pat.add(name); live_matched.add(lu); break

absent=[s["uri"] for s in pats if s["uri"] not in matched_pat
        and to_regex(s["uri"]) is not None]
# live URIs not matching any static pattern = ADDED
added=sorted(set(live_uris)-live_matched)

print("static b650 patterns (concrete, matchable):", sum(1 for _,rx in pat_re if rx))
print("matched by live:", len(matched_pat))
print("absent on live (static pattern, no live match):", len(absent))
print("added on live (no static pattern):", len(added))
print()
print("=== ABSENT (static pattern not seen live) ===")
for a in sorted(absent): print("  -", a)
print()
print("=== ADDED (live URI with no static pattern) ===")
for a in added: print("  +", a)
json.dump({"absent":sorted(absent),"added":added,
           "matched":len(matched_pat),"static_matchable":sum(1 for _,rx in pat_re if rx)},
          open(f"{WD}/uri_compare.json","w"),indent=1)
