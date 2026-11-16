#!/usr/bin/env python3
import json, os, subprocess, time
HOST=os.environ["BMC_HOST"]; BASE=f"https://{HOST}"
WD=os.path.dirname(os.path.abspath(__file__))
OUT=os.path.join(WD,"api"); os.makedirs(OUT,exist_ok=True)
CSRF=open(os.path.join(WD,"csrf.txt")).read().strip()
# safe GET-only, B650 handler present, not action/download/session-launch
EP=[
 "/api/settings/services","/api/settings/dashboardinfo","/api/chassis-status",
 "/api/dashboardsensors","/api/logs/dashboardevent","/api/status/uptime",
 "/api/logs/audit","/api/settings/network","/api/asrr/model-name",
 "/api/host_inventory/host_interface_system_info","/api/host_inventory/host_interface_processor_info",
 "/api/host_inventory/host_interface_memory_info","/api/host_inventory/host_interface_baseboard_info",
 "/api/host_inventory/host_interface_power_info","/api/sensors","/api/settings/network-bond",
 "/api/settings/media/adviser","/api/sdr","/api/configuration/project","/api/configuration/runtime",
 "/api/fru","/api/settings/users","/api/settings/date-time","/api/settings/ssl/certificate-info",
 "/api/settings/services","/api/asrr/selsummary","/api/settings/sensor-threshold",
 "/api/settings/network/activelancfg","/api/extfeatures","/api/settings/dns-info",
 "/api/detail_sensors_readings","/api/smbiosdata","/api/settings/fan/mode",
]
# dedup preserve order
seen=set(); EP=[e for e in EP if not (e in seen or seen.add(e))]
res=[]
for e in EP:
    fn=OUT+"/"+e.replace("/","_")+".json"
    p=subprocess.run(["curl","-sk","-b",os.path.join(WD,"cookies.txt"),
        "-H",f"X-CSRFTOKEN: {CSRF}","-H","X-Requested-With: XMLHttpRequest",
        "-o",fn,"-w","%{http_code} %{size_download} %{time_total}",BASE+e],
        capture_output=True,text=True)
    code,size,t=p.stdout.split()
    res.append((e,code,size,t))
    print(f"  {code}  {int(size):7d}B  {float(t):.2f}s  {e}")
    time.sleep(0.15)
json.dump(res,open(OUT+"/_sweep.json","w"))
