#!/usr/bin/env python3
"""Build docs/redfish/b650-uri-inventory.json from the (corrected) markdown section 2 table,
entity names from the previous agent's strings-derived inv.json, and the strings-derived
route sets of B650 7.02/9.05 and X470 3.02 (p702.json, p905.json, px470.json)."""
import json, re, sys, os

MD, INV, P702, P905, PX470, OUT = sys.argv[1:7]

md = open(MD).read()
sec = md.split('## 2. URI inventory (B650 9.05)')[1].split('### 2.1')[0]
rows = []
for line in sec.splitlines():
    if not re.match(r'^\| \d+ \|', line):
        continue
    cells = [c.strip() for c in re.split(r'(?<!\\)\|', line)[1:-1]]
    n, uri, meth, handler, actions, notes = cells
    rows.append(dict(n=int(n), uri=uri.strip('`'), methods=meth, handler=handler.strip('`'),
                     actions=actions.replace('\\|', '|'), notes=notes.replace('\\|', '|')))

inv = {u['key']: u for u in json.load(open(INV))['uris']}

# ---------- schema versions from section 5.2 ----------
ver = {}
for m in re.finditer(r'([A-Za-z]+) (\d+)\.(\d+)\.(\d+)', md.split('### 5.2')[1].split('### 5.3')[0]):
    ver.setdefault(m.group(1), f'v{m.group(2)}_{m.group(3)}_{m.group(4)}')

def schema_for(uri, key, entity):
    if 'ActionInfo' in uri.split('/')[-1]:
        return 'ActionInfo.' + ver.get('ActionInfo', '')
    if '/Actions/' in uri or uri.endswith('.Deep'):
        return None
    manual = {
        '/redfish/v1/': 'ServiceRoot', '/redfish/v1/$metadata': 'OData CSDL metadata (XML)',
        '/redfish/v1/odata': 'OData service document', '/redfish/v1/odata/': 'OData service document',
        '/redfish/v1/openapi.yaml': 'OpenAPI document (YAML)',
        '/redfish': 'Redfish version document',
        '/redfish/v1/UpdateService': 'UpdateService',
        '/redfish/v1/UpdateService/FirmwareInventory': 'SoftwareInventoryCollection',
        '/redfish/v1/UpdateService/FirmwareInventory/{InventoryId}': 'SoftwareInventory',
        '/redfish/v1/UpdateService/{ActionInfoName}': 'ActionInfo',
        '/redfish/v1/Managers/{ManagerId}/HostInterfaces': 'HostInterfaceCollection',
        '/redfish/v1/Managers/{ManagerId}/HostInterfaces/{HostInterfaceId}': 'HostInterface',
        '/redfish/v1/Managers/{ManagerId}/HostInterfaces/{HostInterfaceId}/HostEthernetInterfaces': 'EthernetInterfaceCollection',
        '/redfish/v1/EventService/SSE': 'text/event-stream (Event / MetricReport payloads)',
        '/redfish/v1/PrivilegeRegistry/Redfish_1.3.0_PrivilegeRegistry.json': 'PrivilegeRegistry',
        '/redfish/v1/Oem/Ami/Configurations': 'AMIConfigurations',
        '/redfish/v1/AccountService/Oem/Ami/Configurations': 'AMIAccountServiceConfigurations',
        '/redfish/v1/AccountService/ExternalAccountProviders/RADIUS/Oem/Ami/AdvancedRADIUSSetting': 'AMIAdvanceRADIUSSetting',
        '/redfish/v1/AccountService/{ServiceName}/Certificates/Oem/Ami/ClientCertificates': 'CertificateCollection',
        '/redfish/v1/AccountService/{ServiceName}/Certificates/Oem/Ami/ClientCertificates/{CertificateId}': 'Certificate',
        '/redfish/v1/EventService/Oem/Ami/SMTP/Certificates': 'AMISMTPCertificateCollection',
        '/redfish/v1/EventService/Oem/Ami/SMTP/Certificates/{CertificateId}': 'Certificate',
        '/redfish/v1/Oem/Ami/InventoryData/Status': 'AMIInventoryStatus',
        '/redfish/v1/Oem/Ami/InventoryData': 'BiosStaticFileCollection',
        '/redfish/v1/Systems/{SystemId}/Oem/Ami/BiosTable': 'AMIBiosTable',
        '/redfish/v1/Systems/{SystemId}/Oem/Ami/BiosTableTags': 'AMIBiosTableTags',
        '/redfish/v1/Systems/{SystemId}/Oem/Ami/Inventory/Crc': 'AMIBIOSInventoryCRC',
        '/redfish/v1/oem/ami/inventory/crc': 'AMIBIOSInventoryCRC',
        '/redfish/v1/oem/MappingOut': 'OemMappingOut',
        '/redfish/v1/bios/defaultSD': 'Bios',
        '/redfish/v1/Systems/{SystemId}/Pldm': 'BiosNVRam',
        '/redfish/v1/Managers/{ManagerId}/KCS': 'KCS.v1_0_0',
    }
    if uri in manual:
        t = manual[uri]
    elif entity:
        t = entity
    else:
        return None
    if '.' not in t and ' ' not in t and t in ver:
        t = f'{t}.{ver[t]}'
    return t

# ---------- OEM namespace ----------
def oem_for(uri, handler):
    if '/Actions/Oem/AMIManager.' in uri: return 'AMIManager'
    if '/Actions/Oem/AMIVirtualMedia.' in uri: return 'AMIVirtualMedia'
    if 'UploadCABundle' in uri: return 'AMIUpdateService'
    if 'SubmitDelayedTestEvent' in uri: return 'AMIEventService'
    if handler.startswith('oem/ami_bios'): return 'AmiBios'
    if handler.startswith('oem/ami_hi'): return 'Ami (host interface)'
    if handler.startswith('oem/ami'): return 'Ami'
    if handler.startswith('oem/plugin'): return 'ASRockRack plugin (non-standard resource, no Oem namespace)'
    if '/Oem/' in uri or '/oem/' in uri: return 'Ami'
    if uri.endswith('/GlobalLuaTables'): return 'AMI (non-standard debug resource)'
    return False

# ---------- structured actions ----------
RST = ['On', 'ForceOff', 'GracefulShutdown', 'ForceRestart', 'PowerCycle']
CERT = [dict(name='Certificate.Rekey', target='{uri}/Certificate.Rekey', parameters={'ChallengePassword': [], 'KeyBitLength': [], 'KeyCurveId': [], 'KeyPairAlgorithm': []}),
        dict(name='Certificate.Renew', target='{uri}/Certificate.Renew', parameters={'ChallengePassword': []})]
CLEAR = [dict(name='LogService.ClearLog', target='{uri}/LogService.ClearLog', parameters={})]
CHG = [dict(name='AmiBios.ChangeState', target='{uri}/AmiBios.ChangeState', parameters={'State': ['Enabled', 'Disabled']})]
A = {
 'Certificates/{CertificateId}/Actions/{Action}': CERT,
 'ClientCertificates/{CertificateId}/Actions/{Action}': CERT,
 '/redfish/v1/CertificateService/Actions/{Action}': [
    dict(name='CertificateService.GenerateCSR', target='/redfish/v1/CertificateService/Actions/CertificateService.GenerateCSR',
         parameters={'CertificateCollection': [], 'CommonName': [], 'Country': [], 'State': [], 'City': [], 'Organization': [], 'OrganizationalUnit': [], 'KeyPairAlgorithm': [], 'KeyBitLength': [], 'KeyCurveId': [], 'AlternativeNames': [], 'Email': [], 'ChallengePassword': []}),
    dict(name='CertificateService.ReplaceCertificate', target='/redfish/v1/CertificateService/Actions/CertificateService.ReplaceCertificate',
         parameters={'CertificateUri': [], 'CertificateString': [], 'CertificateType': ['PEM']})],
 '/redfish/v1/Chassis/{ChassisId}/Actions/{Action}': [dict(name='Chassis.Reset', target='{uri}/Chassis.Reset', parameters={'ResetType': RST + ['GracefulRestart (action-info.rcmd only)']})],
 '/redfish/v1/Chassis/{ChassisId}/LogServices/{LogServiceId}/Actions/{Action}': CLEAR,
 '/redfish/v1/Chassis/{ChassisId}/NetworkAdapters/{NetworkAdapterId}/Actions/{Action}': [dict(name='NetworkAdapter.ResetSettingsToDefault', target='{uri}/NetworkAdapter.ResetSettingsToDefault', parameters={})],
 '/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}/PCIeFunctions/{PCIeFunctionId}/Actions/{Action}': CHG,
 '/redfish/v1/Systems/{SystemId}/Memory/{MemoryId}/Actions/{Action}': CHG,
 '/redfish/v1/EventService/Actions/EventService.SubmitDelayedTestEvent': [dict(name='EventService.SubmitDelayedTestEvent', target='{uri}', parameters={'DelayTime': [], 'EventType': []})],
 '/redfish/v1/EventService/Actions/EventService.SubmitTestEvent': [dict(name='EventService.SubmitTestEvent', target='{uri}', parameters={'EventId': [], 'EventTimestamp': [], 'MessageArgs': [], 'MessageId': [], 'OriginOfCondition': [], 'Severity': []})],
 '/redfish/v1/EventService/Subscriptions/{SubscriptionId}/Actions/EventDestination.ResumeSubscription': [dict(name='EventDestination.ResumeSubscription', target='{uri}', parameters={})],
 '/redfish/v1/Managers/{ManagerId}/Actions/Oem/AMIManager.BackupConfig': [dict(name='AMIManager.BackupConfig', target='{uri}', parameters={'BackupFeatures': []})],
 '/redfish/v1/Managers/{ManagerId}/Actions/Oem/AMIManager.RedfishPowerSaveMode': [dict(name='AMIManager.RedfishPowerSaveMode', target='{uri}', parameters={'PowerSaveMode': ['Enable', 'Disable']})],
 '/redfish/v1/Managers/{ManagerId}/Actions/Oem/AMIManager.RestoreConfig': [dict(name='AMIManager.RestoreConfig', target='{uri}', parameters={'conf_file (multipart part)': []})],
 '/redfish/v1/Managers/{ManagerId}/Actions/Oem/AMIVirtualMedia.ConfigureCDInstance': [dict(name='AMIVirtualMedia.ConfigureCDInstance', target='{uri}', parameters={'CDInstance': []})],
 '/redfish/v1/Managers/{ManagerId}/Actions/Oem/AMIVirtualMedia.EnableRMedia': [dict(name='AMIVirtualMedia.EnableRMedia', target='{uri}', parameters={'RMediaState': ['Enable', 'Disable']})],
 '/redfish/v1/Managers/{ManagerId}/Actions/{Action}': [dict(name='Manager.Reset', target='{uri}/Manager.Reset', parameters={'ResetType': ['ForceRestart']})],
 '/redfish/v1/Managers/{ManagerId}/LogServices/{LogServiceId}/Actions/{Action}': CLEAR,
 '/redfish/v1/Managers/{ManagerId}/VirtualMedia/{VirtualMediaId}/Actions/{Action}': [
    dict(name='VirtualMedia.InsertMedia', target='{uri}/VirtualMedia.InsertMedia',
         parameters={'Image': [], 'TransferProtocolType': ['NFS', 'CIFS', 'HTTPS'], 'TransferMethod': ['Stream'], 'Inserted': [], 'WriteProtected': [], 'UserName': [], 'Password': []}),
    dict(name='VirtualMedia.EjectMedia', target='{uri}/VirtualMedia.EjectMedia', parameters={})],
 '/redfish/v1/Systems/{SystemId}/Actions/{Action}': [dict(name='ComputerSystem.Reset', target='{uri}/ComputerSystem.Reset',
    parameters={'ResetType': RST, '@Redfish.OperationApplyTime': ['Immediate', 'AtMaintenanceWindowStart'], 'MaintenanceWindowStartTime': []})],
 '/redfish/v1/Systems/{SystemId}/Bios/Actions/{Action}': [
    dict(name='Bios.ResetBios', target='{uri}/Bios.ResetBios', parameters={}),
    dict(name='Bios.ChangePassword', target='{uri}/Bios.ChangePassword', parameters={'PasswordName': [], 'OldPassword': [], 'NewPassword': []})],
 '/redfish/v1/Systems/{SystemId}/LogServices/{LogServiceId}/Actions/{Action}': CLEAR,
 '/redfish/v1/Systems/{SystemId}/SecureBoot/Actions/{Action}': [dict(name='SecureBoot.ResetKeys', target='{uri}/SecureBoot.ResetKeys', parameters={'ResetKeysType': ['ResetAllKeysToDefault', 'DeleteAllKeys', 'DeletePK']})],
 '/redfish/v1/Systems/{SystemId}/SecureBoot/SecureBootDatabases/{DatabaseId}/Actions/{Action}': [dict(name='SecureBootDatabase.ResetKeys', target='{uri}/SecureBootDatabase.ResetKeys', parameters={'ResetKeysType': []})],
 '/redfish/v1/Systems/{SystemId}/Storage/{StorageId}/Volumes/{VolumeId}/Actions/{Action}': [dict(name='Volume.Initialize', target='{uri}/Volume.Initialize', parameters={})],
 '/redfish/v1/TelemetryService/Actions/{Action}': [dict(name='TelemetryService.SubmitTestMetricReport', target='{uri}/TelemetryService.SubmitTestMetricReport', parameters={'MetricReportName': [], 'GeneratedMetricReportValues': []})],
 '/redfish/v1/TelemetryService/LogService/Actions/{Action}': CLEAR,
 '/redfish/v1/UpdateService/Actions/Oem/UpdateService.UploadCABundle': [dict(name='UpdateService.UploadCABundle', target='{uri}', parameters={'ca_bundle (multipart part, PEM)': []})],
 '/redfish/v1/UpdateService/Actions/SimpleUpdate': [dict(name='UpdateService.SimpleUpdate', target='{uri}', parameters={'ImageURI': [], 'TransferProtocol': ['HTTP', 'FTP', 'HTTPS'], 'Username': [], 'Password': []})],
 '/redfish/v1/UpdateService/upload': [dict(name='UpdateService.MultipartPush (MultipartHttpPushUri)', target='{uri}',
    parameters={'UpdateParameters.Targets': ['/redfish/v1/Managers/Self', '/redfish/v1/Systems/Self/Bios', '/redfish/v1/Systems/Self/Cpld'],
                'UpdateParameters.@Redfish.OperationApplyTime': [],
                'UpdateParameters.Oem.FlashType': ['FULLFwUpdate', 'DUALIMAGEFwUpdate', 'HPMFwUpdate'],
                'UpdateParameters.Oem.UploadSelector': ['AutoInactive', 'Image1', 'Image2', 'ImageBoth', 'Default'],
                'UpdateParameters.Oem.ImageType': [], 'UpdateParameters.Oem.preserveconf': [], 'UpdateFile': []})],
}
def actions_for(uri):
    acts = A.get(uri)
    if acts is None:
        for k, v in A.items():
            if not k.startswith('/') and uri.endswith(k):
                acts = v; break
    if not acts:
        return []
    base = uri[:-len('/{Action}')] if uri.endswith('/{Action}') else uri
    return [dict(name=a['name'], target=a['target'].replace('{uri}', base), parameters=a['parameters']) for a in acts]

# ---------- route sets ----------
def shape(p):
    p = re.sub(r'^/[a-z0-9]+_[a-z0-9_]+[^/]?(?=/)', '', re.sub(r'^//', '/', p)).replace('^%s', '/redfish/v1').replace('/Managers/%s/', '/Managers/*/')
    if not p.startswith('/redfish'):
        p = '/redfish/v1' + p
    p = p.rstrip('$').replace('([^/]+)', '*').replace('($metadata)', '$metadata')
    p = re.sub(r'\((\w+)\)', r'\1', p)
    return p
def load(path):
    s = set()
    for v in json.load(open(path)).values():
        for x in v: s.add(shape(x))
    return s
S702, S905, SX = load(P702), load(P905), load(PX470)
SX |= {'/redfish/v1/EventService/Actions/EventService.SubmitDelayedTestEvent',
       '/redfish/v1/Managers/Self/FactoryResetActionInfo', '/redfish/v1/UpdateService/Actions/AmiOemBMCUpdate'}
def ushape(u):
    return re.sub(r'\{[^}]+\}', '*', u)

SPECIAL = {  # uri -> (b650 routed literally?, x470 value)
 '/redfish/*': (True, True), '/redfish/... trailing "/"': (True, True),
 '/redfish/v1': (True, 'unknown'),
 '/redfish/v1/Chassis/RDE{n}_*, /redfish/v1/Systems/RDE{n}_* (dynamic)': (True, False),
 '/redfish/v1/Chassis/{ChassisId}/Thermal/... (Thermal-Instance)': (True, 'unknown'),
 '/redfish/v1/EventService/Actions/EventService.SubmitTestEvent': (True, True),
 '/redfish/v1/{any}.Deep': (True, False),
 '/redfish/v1/PrivilegeRegistry/Redfish_1.3.0_PrivilegeRegistry.json': (True, False),
 '/redfish/v1/Oem/Ami/BiosStaticFiles/PostStatus/{Id}': (True, 'unknown'),
 '/redfish/v1/Oem/Ami/BiosStaticFiles/{FileId}': (True, 'unknown'),
 '/redfish/v1/Systems/{SystemId}/Memory/{MemoryId}/Actions/{Action}': (True, True),
 '/redfish/v1/EventService/SSE': (True, False),
}
X_NOTES = {
 '/redfish/v1/Systems/{SystemId}/Memory/{MemoryId}/Actions/{Action}': 'X470 registers the literal .../Actions/AmiBios.ChangeState (oem/ami/route.lua).',
 '/redfish/v1/Oem/Ami/BiosStaticFiles/{FileId}': 'X470 ami_hi has /BiosStaticFiles/{id} whose full prefix is not recoverable from strings.',
 '/redfish/v1/Oem/Ami/BiosStaticFiles/PostStatus/{Id}': 'X470 ami_hi has /BiosStaticFiles/PostStatus/{id} whose full prefix is not recoverable from strings.',
 '/redfish/v1/EventService/Actions/EventService.SubmitDelayedTestEvent (allow-table only)': 'X470 registers this route (oem/ami/route.lua); B650 9.05/7.02 do not.',
 '/redfish/v1/EventService/SubmitDelayedTestEventActionInfo (allow-table only)': 'X470 registers this route (oem/ami/route.lua); B650 9.05/7.02 do not.',
}

def handler_file(h):
    if h == 'none registered':
        return None
    return re.split(r' [\(/+]| \(|, ', h)[0].strip()

entries = []
for r in rows:
    uri = r['uri']
    keym = re.search(r'\(key `([^`]+)`\)', r['notes'])
    key = keym.group(1) if keym else None
    notes = re.sub(r'\s*\(key `[^`]+`\)\s*$', '', r['notes']).strip()
    ent = inv.get(key, {}).get('entity') if key else None
    meth = [m for m in r['methods'].split() if m]
    extra = []
    if any(m.endswith('?') for m in meth):
        extra.append('methods marked ? are not in the allow table; the handler decides')
    meth = [m.rstrip('?') for m in meth]
    if '(assumed)' in r['handler'] or '(inferred)' in r['handler']:
        extra.append('handler attribution is inferred, not confirmed')
    if handler_file(r['handler']) != r['handler']:
        extra.append('handler detail: ' + r['handler'])
    orphan = '(allow-table only)' in uri
    clean = uri.replace(' (allow-table only)', '')
    sh = ushape(clean)
    if uri in SPECIAL:
        b905, x = SPECIAL[uri]
        b702 = b905
    elif orphan:
        b905 = b702 = False
        x = sh in SX
    else:
        b905 = sh in S905 or clean in ('/redfish/v1/', '/redfish', '/redfish/v1/$metadata')
        b702 = sh in S702 or clean in ('/redfish/v1/', '/redfish', '/redfish/v1/$metadata')
        if not b905:
            print('WARN not matched in 9.05 strings:', uri, file=sys.stderr)
        x = sh in SX
    if orphan:
        extra.append('handler and allow entry ship in B650 but no URI pattern is registered (unreachable on B650)')
    if uri in X_NOTES: extra.append(X_NOTES[uri])
    if key: extra.append(f'route key {key}')
    entries.append(dict(uri=clean, methods=meth, handler_file=handler_file(r['handler']),
                        actions=actions_for(clean), schema_type=schema_for(clean, key, ent),
                        oem=oem_for(clean, r['handler']),
                        firmware=dict(b650_702=b702, b650_905=b905, x470_302=x),
                        notes='; '.join([n for n in [notes] + extra if n])))

# merge rows that share a URI (two route keys for one URI)
merged = []
for e in entries:
    prev = next((m for m in merged if m['uri'] == e['uri']), None)
    if prev:
        prev['methods'] += [m for m in e['methods'] if m not in prev['methods']]
        prev['notes'] += '; also registered under a second key: ' + e['notes']
    else:
        merged.append(e)
entries = merged

# ---------- 7.02-only ----------
entries.append(dict(uri='/redfish/v1/Managers/{ManagerId}/KCS', methods=['GET', 'PATCH'], handler_file='oem/plugin/kcs.lua',
    actions=[], schema_type='KCS.v1_0_0', oem='ASRockRack plugin (non-standard resource, no Oem namespace)',
    firmware=dict(b650_702=True, b650_905=False, x470_302=False),
    notes='Removed in 9.05 (section 8). Reads/toggles in-band KCS access (InterfaceEnabled) via raw IPMI; route key plugin_redfish_kcs'))

# ---------- X470-only ----------
XO = [
 ('/redfish/v1/Chassis/{ChassisId}/PCIeDevices', ['GET'], 'redfish/chassis/pcie-devices.lua', 'PCIeDeviceCollection', False, [], 'X470 registers it in hi_routes.lua; B650 exposes PCIeDevices only under the route below with key resource_131 (/Chassis/{id}/PCIeDevices is present in B650 too -- see main row)'),
 ('/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}/Functions/{FunctionId}', ['GET'], 'redfish/chassis/pcie-device-functions.lua', 'PCIeFunction', False, [], 'Older path; B650 uses .../PCIeFunctions/{id}. Normalise in the proxy'),
 ('/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}/Actions/AmiBios.ChangeState', ['POST'], 'oem/ami (bios-handler, assumed)', None, 'AmiBios',
    [dict(name='AmiBios.ChangeState', target='/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}/Actions/AmiBios.ChangeState', parameters={'State': ['Enabled', 'Disabled']})], 'B650 moved this to .../PCIeFunctions/{id}/Actions/{Action}'),
 ('/redfish/v1/Chassis/{ChassisId}/PCIeDevices/{PCIeDeviceId}/ChangeStateActionInfo', ['GET'], 'oem/ami/actioninfo/changestate-actioninfo.lua', 'ActionInfo', 'AmiBios', [], ''),
 ('/redfish/v1/Chassis/{ChassisId}/Thermal/Fans/{FanId}', ['GET', 'PATCH'], 'redfish/chassis/thermal-instance.lua (assumed)', None, False, [], 'Probably the Thermal-Instance (PATCH) route that has no literal pattern on B650'),
 ('/redfish/v1/Chassis/{ChassisId}/LogServices/{LogServiceId}/ClearlogActionInfo', ['GET'], 'redfish/actioninfo/clearlog-actioninfo.lua', 'ActionInfo', False, [], ''),
 ('/redfish/v1/Systems/{SystemId}/LogServices/{LogServiceId}/ClearlogActionInfo', ['GET'], 'redfish/actioninfo/clearlog-actioninfo.lua', 'ActionInfo', False, [], ''),
 ('/redfish/v1/Managers/{ManagerId}/LogServices/{LogServiceId}/ClearlogActionInfo', ['GET'], 'redfish/actioninfo/clearlog-actioninfo.lua', 'ActionInfo', False, [], ''),
 ('/redfish/v1/TelemetryService/LogServices/{LogServiceId}/ClearlogActionInfo', ['GET'], 'redfish/actioninfo/clearlog-actioninfo.lua', 'ActionInfo', False, [], ''),
 ('/redfish/v1/Systems/{SystemId}/Bios/ResetBiosActionInfo', ['GET'], 'redfish/actioninfo/biosreset-actioninfo.lua', 'ActionInfo', False, [], ''),
 ('/redfish/v1/Systems/{SystemId}/Memory/{MemoryId}/Metrics', ['GET'], 'redfish/system/memorymetrics-instance.lua (assumed)', 'MemoryMetrics', False, [], 'B650 uses .../MemoryMetrics'),
 ('/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/SubProcessors/{SubProcessorId}/SubProcessors', ['GET'], 'redfish/system/processor-collection.lua', 'ProcessorCollection', False, [], 'Second-level SubProcessors'),
 ('/redfish/v1/Systems/{SystemId}/Processors/{ProcessorId}/SubProcessors/{SubProcessorId}/SubProcessors/{SubProcessorId2}', ['GET'], 'redfish/system/processor-instance.lua', 'Processor', False, [], 'Second-level SubProcessors'),
 ('/redfish/v1/TelemetryService/LogServices', ['GET'], 'redfish/telemetry-service/log-service.lua (assumed)', 'LogServiceCollection', False, [], 'B650 uses singular TelemetryService/LogService'),
 ('/redfish/v1/TelemetryService/LogServices/{LogServiceId}', ['GET', 'PATCH'], 'redfish/telemetry-service/log-service.lua (assumed)', 'LogService', False, [], 'B650 uses singular TelemetryService/LogService'),
 ('/redfish/v1/TelemetryService/LogServices/{LogServiceId}/Actions/{Action}', ['POST'], 'redfish/telemetry-service/log-service-actions.lua (assumed)', None, False,
    [dict(name='LogService.ClearLog', target='/redfish/v1/TelemetryService/LogServices/{LogServiceId}/Actions/LogService.ClearLog', parameters={})], ''),
 ('/redfish/v1/TelemetryService/LogServices/{LogServiceId}/Entries', ['GET'], 'redfish/telemetry-service/log-service-entries.lua (assumed)', 'LogEntryCollection', False, [], ''),
 ('/redfish/v1/TelemetryService/LogServices/{LogServiceId}/Entries/{EntryId}', ['GET'], 'redfish/telemetry-service/log-service-entries.lua (assumed)', 'LogEntry', False, [], ''),
 ('/redfish/v1/AccountService/Accounts/HI-FW', ['GET'], 'extensions/host-interface (assumed)', None, False, [], 'Host-interface firmware account collection (hi_routes.lua)'),
 ('/redfish/v1/AccountService/Accounts/HI-FW/{AccountId}', ['GET'], 'extensions/host-interface (assumed)', None, False, [], 'hi_routes.lua'),
 ('/redfish/v1/oem/ami/configuredbsnapshot', ['GET'], 'extensions/host-interface (assumed)', None, 'Ami (host interface)', [], 'hi_routes.lua'),
 ('/redfish/v1/AccountService/Configurations', ['GET', 'PATCH'], 'oem/ami/accountservice-config-handler.lua', None, 'Ami', [], 'B650 moved this to /AccountService/Oem/Ami/Configurations'),
 ('/redfish/v1/configurations', ['GET', 'PATCH'], 'oem/ami/config-handler.lua', None, 'Ami', [], 'B650 moved this to /Oem/Ami/Configurations; still linked from the B650 /redfish/v1/odata document (dead link there)'),
 ('/redfish/v1/Managers/Self/Actions/RedfishDBReset', ['POST'], 'oem/ami/factory-reset.lua', None, 'AMIManager',
    [dict(name='AMIManager.RedfishDBReset', target='/redfish/v1/Managers/Self/Actions/RedfishDBReset', parameters={})], 'Literal Self; advertised but unrouted on B650 9.05'),
 ('/redfish/v1/Managers/Self/FactoryResetActionInfo', ['GET'], 'oem/ami/actioninfo/factoryreset-actioninfo.lua', 'ActionInfo', 'AMIManager', [], ''),
 ('/redfish/v1/UpdateService/Actions/AmiOemBMCUpdate', ['POST'], 'oem/ami/update-service_AmiOem.lua', None, 'AMIUpdateService', [], 'Pattern appears unanchored in strings'),
 ('/redfish/v1/UpdateService/Actions/Oem/UpdateService.UploadFirmwareImage', ['POST'], 'redfish/upload-file.lua', None, 'AMIUpdateService', [], 'Plain-text Lua (extensions/routes/update_service_routes.lua)'),
 ('/redfish/v1/UpdateService/Actions/Oem/{Action}', ['POST'], 'redfish/update-service.lua', None, 'AMIUpdateService',
    [dict(name='UpdateService.BMCFwUpdate', target='/redfish/v1/UpdateService/Actions/Oem/UpdateService.BMCFwUpdate',
          parameters={'FlashType': ['FULLFwUpdate', 'DUALIMAGEFwUpdate'], 'UploadSelector': ['AutoInactive', 'Image1', 'Image2', 'ImageBoth', 'Default']})], 'Plain-text Lua; BMCFwUpdateActionInfo'),
 ('/redfish/v1/BiosStaticFiles', ['GET', 'POST'], 'oem/ami_hi/static-file-collection-hi.lua', None, 'Ami (host interface)', [], 'Full prefix not recoverable from strings (may be under /Oem/Ami)'),
 ('/redfish/v1/BiosStaticFiles/{FileId}', ['GET'], 'oem/ami_hi/static-file-instance-hi.lua', None, 'Ami (host interface)', [], 'Full prefix not recoverable from strings (may be under /Oem/Ami)'),
 ('/redfish/v1/BiosStaticFiles/PostStatus/{Id}', ['GET'], 'oem/ami_hi/post-status.lua', None, 'Ami (host interface)', [], 'Full prefix not recoverable from strings (may be under /Oem/Ami)'),
 ('/redfish/v1/DynamicExtension', ['GET'], 'oem/dre/dynamic-root-handler.lua', None, 'Ami (Dynamic Redfish Extension)', [], 'oem/dre routes are built at runtime (plain-text Lua); sub-resources depend on what BIOS registers'),
]
known = {e['uri'] for e in entries}
for uri, meth, h, st, oem, acts, note in XO:
    if uri in known:
        continue
    if st and st in ver: st = f'{st}.{ver[st]}'
    hf = handler_file(h)
    n = [note] if note else []
    if hf != h: n.append('handler detail: ' + h)
    n.append('X470-only; methods inferred from X470 allow_method_header_table labels, not verified')
    entries.append(dict(uri=uri, methods=meth, handler_file=hf, actions=acts, schema_type=st, oem=oem,
                        firmware=dict(b650_702=False, b650_905=False, x470_302=True), notes='; '.join(n)))

# sanity: every X470 route shape is covered
cover = {ushape(e['uri']).replace('/Managers/Self/', '/Managers/*/') for e in entries if e['firmware']['x470_302'] is True}
cover |= {ushape(e['uri']) for e in entries if e['firmware']['x470_302'] is True}
miss = sorted(s for s in SX if s not in cover)
print('X470 shapes not covered:', miss, file=sys.stderr)

json.dump(entries, open(OUT, 'w'), indent=1, ensure_ascii=False)
open(OUT, 'a').write('\n')
print(len(entries), 'entries', file=sys.stderr)
