import json,collections,sys
d=json.load(open('out.json'))
bs=['X470-3.02','B650-7.02','B650-9.05']
areas=[]
for e in d:
    if e['area'] not in areas: areas.append(e['area'])
rows=["| Area | X470 refs (handler yes/no/unk) | B650-7.02 refs (yes/no) | B650-9.05 refs (yes/no) | Seen live |","|---|---|---|---|---|"]
tot={b:collections.Counter() for b in bs}; live=0
for a in areas+['TOTAL']:
    es=d if a=='TOTAL' else [e for e in d if e['area']==a]
    cells=[]
    for b in bs:
        c=collections.Counter(e['handler_present'].get(b) for e in es if b in e['boards'])
        n=sum(c.values())
        cells.append(f"{n} ({c['yes']}/{c['no']}" + (f"/{c['unknown']})" if b=='X470-3.02' else ")"))
    lv=sum(1 for e in es if e.get('seen_in_live_capture'))
    rows.append(f"| {'**TOTAL**' if a=='TOTAL' else a} | "+" | ".join(cells)+f" | {lv} |")
head=open('head.md').read()
proxy='''
---

## 9. Proxy implementation notes (Go)

1. **Session pool per BMC.** Log in with a form-encoded `POST /api/session`, keep the `QSESSIONID` cookie and `CSRFToken`, and send `X-CSRFTOKEN` on every call. Log out with `DELETE /api/session` on shutdown. The BMC has a small session limit (code 15000), so leaked sessions lock the UI out until `time_out` expires.
2. **Keepalive.** There is no keepalive endpoint. Poll `GET /api/chassis-status` below the web service `time_out` (`GET /api/settings/services`, default 1800 s). On a 401, re-login once and retry.
3. **Body fidelity.** Use an empty body with a JSON content type for `PUT /api/maintenance/flash` (X470), `POST /api/maintenance/reset` and `PUT /api/asrr/bmccoldreset`. Send a raw JSON string without forcing a content type for the ASRR `configuration`/`upgrade` calls. Use multipart with the exact field names `fwimage`, `publicKey`, `publickey`, `lmedia_image` and `config`. Singleton `PUT`s should send the full object as previously read (read-modify-write).
4. **Board family detection.** `GET /api/asrr/fw-info` / `GET /api/asrr/model-name` (both), `/api/chassis` (X470 only) and `GET /api/configuration/project`. Choose the firmware upload URL (`/firmware` versus `/firmware/firmware`) by family.
5. **Do not trust B650 UI presence.** About 150 paths the B650 UI references have no handler in 9.05. Use the handler column, not the UI, to decide what the proxy exposes.
6. **Long operations.** BMC flash: the BMC goes away for about 2 minutes after `maintenance/reset`, and the session is invalidated. BIOS flash: poll `asrr/maintenance/BIOS/status` and `flash-progress`. The BMC session survives.
7. **KVM.** Proxying KVM means proxying the `/kvm` WebSocket (subprotocol `binary`) together with the cookie, and getting the token from `/api/kvm/token` (X470) or `/api/settings/media/h5viewercfg` (B650) on the same session.
'''
diff=open('diff.md').read()
cat=open('catalog.md').read()
out=head+"\n".join(rows)+"\n\nThe X470 'unknown' entries are the four `settings/fan/fan_*` collections. Their handler strings were not resolvable from the firmware doc.\n\n---\n\n## 7. Endpoint catalog\n\nEach endpoint block lists: boards whose UI references it; handler present per board; whether it was seen in a live capture; per-item path; query params; UI module(s); model; request fields (type / allowed values / validation); and response fields read by the UI.\n\n"+cat+"\n\n---\n\n## 8. Board diff (X470 3.02 versus B650)\n\nBased on the analysed JS bundles. Note: the B650D4U capture (fw 1.10.00) also calls several endpoints listed here as X470-only (`/api/asrr/getvetobit`, the asrr fan endpoints), so older B650 firmware shares more of the X470 OEM surface than 7.02/9.05 do.\n\n"+diff+proxy
open(sys.argv[1],'w').write(out)
print(len(out.splitlines()))
