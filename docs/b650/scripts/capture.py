#!/usr/bin/env python3
import json, os, subprocess
HOST=os.environ["BMC_HOST"]; BASE=f"https://{HOST}"
WD=os.path.dirname(os.path.abspath(__file__))
OUT=os.path.join(WD,"key"); os.makedirs(OUT,exist_ok=True)
KEY=[
 "/redfish/v1/","/redfish/v1/Systems/Self","/redfish/v1/Systems/Self/ResetActionInfo",
 "/redfish/v1/Systems/Self/Bios",
 "/redfish/v1/Chassis/Self","/redfish/v1/Chassis/Self/ResetActionInfo",
 "/redfish/v1/Chassis/Self/Power","/redfish/v1/Chassis/Self/Thermal",
 "/redfish/v1/Chassis/Self/Sensors",
 "/redfish/v1/Managers/Self","/redfish/v1/Managers/Self/ResetActionInfo",
 "/redfish/v1/Managers/Self/NetworkProtocol","/redfish/v1/Managers/Self/EthernetInterfaces",
 "/redfish/v1/Managers/Self/HostInterfaces","/redfish/v1/Managers/Self/VirtualMedia",
 "/redfish/v1/Managers/Self/LogServices","/redfish/v1/Managers/Self/SerialInterfaces",
 "/redfish/v1/AccountService","/redfish/v1/AccountService/Accounts","/redfish/v1/AccountService/Roles",
 "/redfish/v1/SessionService","/redfish/v1/SessionService/Sessions",
 "/redfish/v1/UpdateService","/redfish/v1/UpdateService/FirmwareInventory",
 "/redfish/v1/UpdateService/SimpleUpdateActionInfo",
 "/redfish/v1/EventService","/redfish/v1/EventService/Subscriptions",
 "/redfish/v1/TaskService","/redfish/v1/TaskService/Tasks",
 "/redfish/v1/TelemetryService","/redfish/v1/TelemetryService/MetricReports",
 "/redfish/v1/TelemetryService/MetricReportDefinitions",
]
def slug(u): return u.strip("/").replace("/","__") or "root"
cfg=[]
for u in KEY:
    cfg.append(f'output = "{OUT}/{slug(u)}.json"'); cfg.append(f'url = "{BASE}{u}"')
open(os.path.join(WD,"key.cfg"),"w").write("\n".join(cfg)+"\n")
secret=f'user = "{os.environ["BMC_USER"]}:{os.environ["BMC_PASS"]}"\n'
subprocess.run(["curl","-sk","--rate","360/m","-K","-","-K",os.path.join(WD,"key.cfg")],
               input=secret,capture_output=True,text=True)
print("captured",len(KEY),"key resources into",OUT)
for u in KEY:
    p=f"{OUT}/{slug(u)}.json"
    try: sz=os.path.getsize(p)
    except OSError: sz=-1
    print(f"  {sz:7d}  {u}")
