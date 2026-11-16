import json,re
M=json.load(open('master.json'))
DENY_EXACT={'/api/asrr/bmccoldreset','/api/asrr/maintenance/BMC/coldreset','/api/asrr/clearmcalog','/api/maintenance/reset',
'/api/maintenance/firmware/verification','/api/maintenance/flash','/api/maintenance/firmware/upgrade','/api/maintenance/firmware/dwldfwimg',
'/api/maintenance/firmware','/api/maintenance/upload_restore','/api/maintenance/flash_image_config','/api/maintenance/hpm',
'/api/asrr/maintenance/BIOS/import/settings','/api/settings/service/web-restart','/api/settings/dns/restart','/api/ae/run','/api/ae_test',
'/api/actions/power','/api/actions/chassis-led','/api/reset-pass','/api/viewerlogin','/api/asrr/session','/api/session',
'/api/logs/video-log/delete','/api/settings/pef/send_test_alert','/api/settings/smtp/smtpupload','/api/settings/dns/tsig-private-key',
'/api/settings/ldap-certificates','/api/set/dns/registration','/api/system_inventory/component_status_update',
'/api/asrr/setprochot','/api/asrr/setvetobit','/api/asrr/setWebEnableTimeout','/api/logs/video-data'}
DENY_RE=[r'/api/maintenance/nm_',r'/api/maintenance/hpm/(activatecomponents|exitupdatemode|flash|preparecomponents|updatemode|verifyimage$|biosfw|mmcfw)',
r'/api/asrr/maintenance/(BIOS|BMC|CPLD)/(upgrade|configuration|firmware|PublicKey)',r'/api/asrr/settings/set',r'/api/settings/media/(local|remote)/(start-media|stop-media|file|image$|lmediasizecheck)',
r'/api/settings/raid_management/.*(_cmd|create_virtual_drive|delete_virtual_drive|raid_levels|raid_pd_count)$',r':id',r'/api/settings/user/ssh-key-upload']
plan=[];skip=[]
for m in M:
    p=m['path']
    if p in DENY_EXACT or any(re.search(r,p) for r in DENY_RE): skip.append(p); continue
    mj=m['methods_js']
    if mj and 'GET' not in mj and m['in_firmware'] and p not in ('/api/settings/sensor-threshold',):
        # fw route without GET in JS: still try GET (unregistered method returns 404 1010) only if not write-ish
        pass
    plan.append({'id':'get:'+p,'method':'GET','path':p})
json.dump(plan,open('plan1.json','w'),indent=0)
print(len(plan),'get;',len(skip),'skipped'); print('\n'.join(skip))
