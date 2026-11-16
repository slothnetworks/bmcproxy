import json,re
JUNK={'/aeabi','/aep_inventory','/aes','/chassis_info_area_start_offset','/chassis_power_off_events_enable','/chassis_power_on_events_enable','/chassis_reset_events_enable','/confbkup','/extlog/archive/','/previewer_handler','/previewer_handler_data','/previewer_handler_func','/viewerlogin_handler','/viewerlogin_handler_data','/viewerlogin_handler_func'}
fw=['/api'+l.strip() for l in open('fw_routes.txt') if l.strip() and l.strip() not in JUNK]
cat=json.load(open('/opt/sparked/asrockrack-api/docs/api/legacy-api-endpoints.json'))
har=json.load(open('/opt/sparked/asrockrack-api/docs/x470/har-endpoints.json'))['endpoints']
M={}
def get(p):
    return M.setdefault(p,{'path':p,'sources':set(),'methods_js':set(),'cat':None})
for p in fw: get(p)['sources'].add('firmware')
for e in cat:
    inx='X470-3.02' in e.get('boards',[])
    if not inx and e.get('handler_present',{}).get('X470-3.02')!='yes': continue
    m=get(e['path']); 
    if inx: m['sources'].add('ui_js')
    else: m['sources'].add('catalog_fw_only')
    m['methods_js'] |= set(e['methods']); m['cat']=e
    if e.get('item_path'):
        mi=get(e['item_path']); mi['methods_js'] |= set(e.get('item_methods') or [])
        if inx: mi['sources'].add('ui_js')
        mi['cat']=mi['cat'] or e
for h in har:
    m=get(h['path']); m['sources'].add('har'); m.setdefault('har',[]).append(h)
out=[]
for p in sorted(M):
    m=M[p]; m['sources']=sorted(m['sources']); m['methods_js']=sorted(m['methods_js'])
    m['in_firmware']='firmware' in m['sources']
    out.append(m)
json.dump(out,open('master.json','w'),indent=1,default=str)
print(len(out))
for m in out:
    if not m['in_firmware']: print('NOFW',m['path'],m['sources'],m['methods_js'])
for m in out:
    if m['in_firmware'] and not ({'ui_js','har'}&set(m['sources'])): print('FWONLY',m['path'])
