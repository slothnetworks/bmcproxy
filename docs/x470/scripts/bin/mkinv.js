// Builds docs/x470/redfish-uri-inventory.json from live observations (no secrets included).
const fs = require('fs');
const live = [
  // uri, status_unauth, status_basic_auth, type, allow, notes
  ['/redfish', 200, null, null, null, 'Version document {"v1":"/redfish/v1/"}; no auth'],
  ['/redfish/v1', 302, null, null, null, 'Redirects to /redfish/v1/ (Location: /redfish/v1/)'],
  ['/redfish/v1/', 200, null, '#ServiceRoot.v1_3_1.ServiceRoot', 'GET', 'No auth. ETag W/"<number>", supports If-None-Match -> 304, $select works; $expand needs auth; $top/$skip/$filter -> 501 on this resource'],
  ['/redfish/v1/odata', 200, null, null, null, 'OData service document; no auth; lists 16 singletons'],
  ['/redfish/v1/$metadata', 200, null, null, null, 'CSDL XML, 46694 bytes, ~1.2 s; no auth; references 98 DMTF + 7 AMI OEM schema files'],
  ['/redfish/v1/Systems', 401, 401, '(ComputerSystemCollection expected)', null, 'Basic auth with valid web/IPMI admin credentials rejected (Security.1.0.AccessDenied)'],
  ['/redfish/v1/SessionService/Sessions', 401, 401, '(SessionCollection expected)', null, 'POST {UserName,Password} with valid credentials -> 401 Security.1.0.AccessDenied; no X-Auth-Token issued'],
  ['/redfish/v1/SessionService', 401, null, '(SessionService expected)', null, 'Linked from ServiceRoot'],
  ['/redfish/v1/Registries', 401, null, '(MessageRegistryFileCollection expected)', null, 'Linked from ServiceRoot'],
  ['/redfish/v1/JsonSchemas', 401, null, '(JsonSchemaFileCollection expected)', null, 'Linked from ServiceRoot'],
  ['/redfish/v1/Systems/Self', 401, null, '(ComputerSystem expected)', null, 'Not reachable: auth fails'],
  ['/redfish/v1/Chassis/Self/Sensors', 401, null, null, null, 'Auth is checked before routing: 401 for any path, existing or not'],
  ['/redfish/v1/CertificateService', 401, null, null, null, 'Not in $metadata; 401 (auth before routing)'],
  ['/redfish/v1/Nonexistent', 401, null, null, null, 'Proves auth-before-routing: no 404 oracle without credentials'],
];
const rootLinks = ['/redfish/v1/Chassis', '/redfish/v1/Managers', '/redfish/v1/TaskService', '/redfish/v1/AccountService', '/redfish/v1/EventService',
  '/redfish/v1/CompositionService', '/redfish/v1/TelemetryService', '/redfish/v1/UpdateService', '/redfish/v1/DynamicExtension', '/redfish/v1/configurations'];
const entries = live.map(([uri, su, sb, type, allow, notes]) => ({ uri, status_unauthenticated: su, status_basic_auth_valid_creds: sb, odata_type: type, allow, source: 'live', notes }));
for (const u of rootLinks) entries.push({ uri: u, status_unauthenticated: 401, status_basic_auth_valid_creds: null, odata_type: null, allow: null, source: 'linked from live ServiceRoot / odata; not fetched individually (auth required)', notes: 'Children not walked: Redfish authentication rejected the account' });
const out = {
  meta: {
    bmc: 'ASRock Rack X470D4U, BMC firmware 2.32.00 (built May 11 2021) — NOT 3.02.00 as expected',
    probed: '2026-11-16',
    host: '10.3.10.207',
    server_header: 'AMI MegaRAC Redfish Service',
    redfish_version: '1.5.0', service_root_type: 'ServiceRoot.v1_3_1', oem_ami_rtp_version: '1.5.b', product: 'AMI Redfish Server',
    protocol_features_supported: { ExpandQuery: { ExpandAll: true, Levels: true, Links: true, MaxLevels: 5, NoLinks: true }, FilterQuery: true, SelectQuery: true },
    auth_result: 'Basic and session (POST /SessionService/Sessions) both return 401 Security.1.0.AccessDenied for the admin account whose credentials work on /api/session. Full BFS walk was therefore impossible. Only 3 Redfish auth attempts were made to stay under the 5-failure lockout threshold.',
    method: 'curl, one request at a time, 150-300 ms spacing. Statuses are from unauthenticated requests unless noted.'
  },
  entries,
  metadata_entity_types: fs.readFileSync(process.argv[2], 'utf8').match(/<edmx:Include Namespace="[^".]+"/g).map(s => s.replace(/.*="/, '').replace(/"$/, '')),
};
fs.writeFileSync(process.argv[3], JSON.stringify(out, null, 1) + '\n');
console.log(entries.length, 'entries', out.metadata_entity_types.length, 'schemas');
