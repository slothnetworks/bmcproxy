# ASRock Rack BMC AST2600 Generic Redfish v1.11 API User Guide — Transcription Part 1 (PDF pages 1–130)

Source: `/opt/sparked/asrockrack-api/redfishdocs/asrockrack-redfishdocs.pdf` (381 pages, "Microsoft Print To PDF", scanned/printed page images).
Transcribed from page images. Page references `(p.N)` are PDF page numbers; the printed page numbers in the document footer match the PDF page numbers (p.2 is printed "2", and so on).
Parts 2 (pp.131–260) and 3 (pp.261–381) are covered in separate files.

## Document identity (p.1)

- Title: **Baseboard Management Controller (BMC) AST2600 — Generic Redfish v1.11 API User Guide**
- Publisher: ASRock Rack (logo)
- Date: **December 2023**
- Revision: **1.00**
- It is a *generic* AST2600 guide (AMI MegaRAC-based); the cover names no specific board (B650D4U is not named on the cover). Applicable boards/firmware: see section 1.0 (p.22) below.

## Table of contents (pp.2–17)

Page numbers are the document's own (equal to the PDF page numbers). Each resource section 3.x follows the same template: x.1 URL, x.2 HTTP Method and Privileges, x.3 Supported Properties, x.4 Example (with x.4.n per-method examples).

### Chapter-level page ranges

| Chapter | Pages | Covered in |
|---|---|---|
| Front matter (cover, TOC, list of tables) | 1–21 | part 1 |
| 1.0 Introduction (1.1 Related Documents) | 22 | part 1 |
| 2.0 Redfish API (2.1–2.7) | 23–31 | part 1 |
| 3.0 Redfish Resources (3.1–3.126) | 32–377 | 3.1–3.33 in part 1 (pp.32–130); 3.34–3.78 roughly in part 2; 3.78–3.126 in part 3 |
| 4.0 Appendix (4.1 FixedBootOrder) | 378–381 | part 3 |

### Full TOC

| Section | Title | Page |
|---|---|---|
| 1.0 | Introduction | 22 |
| 1.1 | Related Documents | 22 |
| 2.0 | Redfish API | 23 |
| 2.1 | URL Support | 23 |
| 2.2 | HTTP Request Methods | 24 |
| 2.2.1 | GET (read requests) | 24 |
| 2.2.2 | POST (create/actions) | 24 |
| 2.2.3 | PATCH (update) | 24 |
| 2.2.4 | PUT (replace) | 24 |
| 2.2.5 | DELETE (delete) | 24 |
| 2.3 | HTTP Status Code | 25 |
| 2.4 | Redfish Schema | 26 |
| 2.5 | Authentication | 26 |
| 2.6 | Roles and Privileges | 26 |
| 2.7 | Available Redfish Resource | 27 |
| 3.0 | Redfish Resources | 32 |
| 3.1 | ODATA and Frequently Used Properties | 32 |
| 3.2 | ServiceRoot (GET ServiceRoot Instance 37) | 36 |
| 3.3 | AccountService (GET 42; PATCH AccountService 44; PATCH LDAP & LDAP Service 44; PATCH LDAP RemoteRoleMapping 45; PATCH Active Directory 45; PATCH Active Directory RemoteRoleMapping 46) | 40 |
| 3.4 | ManagerAccountCollection (GET 47; POST 47; POST 48) | 47 |
| 3.5 | Manager Account (GET 50; PATCH 50; PATCH Add SNMP Account 51; PATCH Delete SNMP Account 51; DELETE 51) | 49 |
| 3.6 | CertificateCollection (GET 53; POST 53) | 53 |
| 3.7 | Certificate (GET 58; POST Rekey 59; POST Renew 60; DELETE 61) | 57 |
| 3.8 | ExternalAccountProviderCollection (GET 62) | 62 |
| 3.9 | RADIUS Settings (GET 63; PATCH 65) | 63 |
| 3.10 | Advanced RADIUS Setting (GET 66; PATCH 67) | 66 |
| 3.11 | PAM Configuration (GET 68; PATCH 69) | 68 |
| 3.12 | RoleCollection (GET 70; POST 71) | 70 |
| 3.13 | Role (GET 73; PATCH 74; DELETE 74) | 73 |
| 3.14 | CertificateService (GET 75; POST Generate CSR 76; POST Replace Certificate 77) | 75 |
| 3.15 | CertificateLocations (GET 79) | 79 |
| 3.16 | ChassisCollection (GET 81) | 81 |
| 3.17 | Chassis (GET 84; PATCH 86; POST Chassis Power State 86) | 82 |
| 3.18 | LogServiceCollection (GET 88) | 88 |
| 3.19 | LogService (GET 89; POST Clear Log 90) | 89 |
| 3.20 | LogEntryCollection (GET 92) | 92 |
| 3.21 | LogEntry (GET 97) | 94 |
| 3.22 | MediaControllerCollection (GET 99) | 99 |
| 3.23 | MediaController (GET 100) | 100 |
| 3.24 | NetworkAdapterCollection (GET 102) | 102 |
| 3.25 | NetworkAdapter (GET 105) | 103 |
| 3.26 | NetworkDeviceFunctionCollection (GET 107) | 107 |
| 3.27 | NetworkDeviceFunction Instance (GET 113) | 109 |
| 3.28 | NetworkDeviceFunction Future State (SD) (GET 116; PATCH 118) | 116 |
| 3.29 | VLANNetworkInterfaceCollection (GET 120; POST 121) | 120 |
| 3.30 | VLANNetworkInterface (GET 122; DELETE 123) | 122 |
| 3.31 | VLANNetworkInterface Future State (SD) (GET 124; PATCH 125) | 124 |
| 3.32 | PortCollection (GET 126) | 126 |
| 3.33 | Port (GET 129) | 127 |
| 3.34 | PCIeDevice Collection (GET 131) | 131 |
| 3.35 | PCIeDevice (GET 136) | 135 |
| 3.36 | PCIeFunction Collection (GET 137) | 137 |
| 3.37 | PCIeFunction (GET 139) | 138 |
| 3.38 | PCIeSlots (GET 141) | 140 |
| 3.39 | Power (GET 149; PATCH 151) | 146 |
| 3.40 | SensorCollection (GET 153) | 153 |
| 3.41 | Sensor (GET 158) | 155 |
| 3.42 | Thermal (GET 163) | 161 |
| 3.43 | Composition Service (GET 166; PATCH 167) | 166 |
| 3.44 | ResourceBlocks Collection (GET 168) | 168 |
| 3.45 | ResourceBlocks (GET 170; PATCH 171) | 169 |
| 3.46 | ResourceZone Collection (GET 172) | 172 |
| 3.47 | ResourceZone (GET 173) | 173 |
| 3.48 | Event Service API (GET 176; PATCH 178; POST Submit Test Event 178) | 175 |
| 3.49 | Event Subscription Collection (GET 180; POST 180) | 180 |
| 3.50 | Event Subscription (GET 184; PATCH 185; POST Resume Event Subscription 185; DELETE 185) | 183 |
| 3.51 | SMTP Certificate OEM Properties (GET SMTP Certificates Collection 186; GET SMTP Certificates 187; POST Adding Certificates for StartTLS Connection Protocol SMTP 188; DELETE SMTP Certificates 191) | 186 |
| 3.52 | JSONSchemaFile Collection (GET 192) | 192 |
| 3.53 | JsonSchemaFile (GET 195) | 195 |
| 3.54 | Manager Collection (GET 197) | 197 |
| 3.55 | Manager (GET 200; PATCH 203; POST 203) | 198 |
| 3.56 | EthernetInterfaceCollection (GET 204) | 204 |
| 3.57 | Ethernet Interface (GET 208) | 206 |
| 3.58 | Manager EthernetInterface (GET 210) | 210 |
| 3.59 | HostInterface Collection (GET HostEthernetInterface Collection 212) | 212 |
| 3.60 | HostInterface (GET 214; PATCH 215) | 213 |
| 3.61 | HostEthernetInterface Collection (GET 216) | 216 |
| 3.62 | ManagerNetworkProtocol (GET 218; PATCH 220) | 217 |
| 3.63 | SerialInterfaceCollection (GET 221) | 221 |
| 3.64 | SerialInterface (GET 224; PATCH 224) | 223 |
| 3.65 | VirtualMediaCollection (GET 226) | 226 |
| 3.66 | VirtualMedia (GET 228; PATCH 229; POST 229; POST 230) | 227 |
| 3.67 | Configurations (GET 231; PATCH 231) | 231 |
| 3.68 | InventoryData Status URI (GET 234) | 233 |
| 3.69 | MessageRegistryFileCollection (GET 235) | 235 |
| 3.70 | MessageRegistryFile (GET 237) | 237 |
| 3.71 | MessageRegistry (GET 239) | 239 |
| 3.72 | AttributeRegistry (GET 244) | 241 |
| 3.73 | PrivilegeRegistry (GET 247) | 246 |
| 3.74 | Session Service (GET 250; PATCH 251) | 250 |
| 3.75 | SessionCollection (GET 252; POST 252) | 252 |
| 3.76 | Session (GET 254; DELETE 254) | 254 |
| 3.77 | ComputerSystemCollection (GET 255; POST 256) | 255 |
| 3.78 | ComputerSystem (GET 262; PATCH 265; POST 266) | 258 |
| 3.79 | BootOptionCollection (GET 267; POST 268) | 267 |
| 3.80 | BootOption (GET 271) | 270 |
| 3.81 | FRU Infomation [sic] (GET 273) | 272 |
| 3.83 | GraphicsControllerCollection (GET 274) — note: TOC skips 3.82 | 274 |
| 3.84 | GraphicsController (GET 275) | 275 |
| 3.85 | MemoryCollection (GET 276) | 276 |
| 3.86 | Memory (GET 282; POST 283) | 278 |
| 3.87 | MemoryDomainCollection (GET 285) | 285 |
| 3.88 | MemoryDomain (GET 287) | 287 |
| 3.89 | MemoryChunksCollection (GET 289) | 289 |
| 3.90 | MemoryChunks (GET 290) | 290 |
| 3.91 | NetworkInterfaceCollection (GET 292) | 292 |
| 3.92 | NetworkInterface (GET 293) | 293 |
| 3.93 | ProcessorCollection (GET 295) | 295 |
| 3.94 | Processor (GET 299) | 296 |
| 3.95 | Systems Instance FutureState (SD) (GET 302; PATCH 303) | 302 |
| 3.96 | SecureBoot (GET 305; PATCH 306) | 305 |
| 3.97 | SecureBootDatabaseCollection (GET 307) | 307 |
| 3.98 | SecureBootDatabase (GET 309; POST 310) | 309 |
| 3.99 | SignatureCollection (GET 311; POST 311) | 311 |
| 3.100 | Signature (GET 313; DELETE 314) | 313 |
| 3.101 | SimpleStorageCollection (GET 315) | 315 |
| 3.102 | SimpleStorage (GET 317) | 317 |
| 3.103 | StorageCollection (GET 319) | 319 |
| 3.104 | Storage (GET 322) | 320 |
| 3.105 | StorageControllerCollection (GET 326) | 326 |
| 3.106 | StorageController (GET 328) | 328 |
| 3.107 | Drive (GET 332) | 330 |
| 3.108 | VolumeCollection (GET 334; POST 334) | 334 |
| 3.109 | Volume (GET 338) | 336 |
| 3.110 | USBControllerCollection (GET 339) | 339 |
| 3.111 | USBController (GET 340) | 340 |
| 3.112 | TaskService (GET 342) | 342 |
| 3.113 | TaskCollection (GET 344) | 344 |
| 3.114 | Task (GET 346; DELETE 346) | 345 |
| 3.115 | TelemetryService (GET 347; POST 349; PATCH 350) | 347 |
| 3.116 | MetricDefinitionCollection (GET 351) | 351 |
| 3.117 | MetricDefinition (GET 353) | 352 |
| 3.118 | MetricReportDefinitionCollection (GET 354; POST with MetricProperties 354; POST with Metrics 355) | 354 |
| 3.119 | MetricReportDefinition (GET 358; PATCH 358; DELETE 359) | 357 |
| 3.120 | MetricReportCollection (GET 360) | 360 |
| 3.121 | MetricReport (GET 361) | 361 |
| 3.122 | TriggerCollection (GET 362; POST Create Discrete Trigger 362; POST Create Numeric Trigger 363) | 362 |
| 3.123 | Trigger (GET 366; DELETE 367) | 365 |
| 3.124 | UpdateService (GET 371; PATCH 372; POST Create Update with HTTP 372; POST Create Update with FTP 373; POST for MultipartHttpPush 374) | 368 |
| 3.125 | FirmwareInventoryCollection (GET 376) | 376 |
| 3.126 | FirmwareInventory (GET 377) | 377 |
| 4.0 | Appendix | 378 |
| 4.1 | FixedBootOrder | 378 |
| 4.1.1 | Getting current Boot Order and Changing Boot Order | 378 |
| 4.1.1.1 | Getting current FixedBootOrder information | 378 |
| 4.1.1.2 | GET Getting Current Boot Order with FBO | 379 |
| 4.1.1.3 | PATCH Changing Current Boot Order with FBO | 380 |

### List of tables (pp.18–20)

| # | Table | Page |
|---|---|---|
| 1 | HTTP Status Code and Description | 25 |
| 2 | Lists of Roles and Privileges | 26 |
| 3 | List of Available Redfish Resource | 27 |
| 4 | ODATA and Frequently Used Properties | 32 |
| 5 | Resource.v1_8_1 Schema Properties | 33 |
| 6 | Supported Properties for Service Root | 36 |
| 7 | Supported Properties for Account Service | 40 |
| 8 | Supported Properties for Manager Account | 49 |
| 9 | CertificateCollection POST Request Property | 54 |
| 10 | Supported Properties for Certificate | 57 |
| 11 | Rekey Action Request Body Property | 60 |
| 12 | Rekey Action Response Property | 60 |
| 13 | Renew Action Request Body Property | 60 |
| 14 | Renew Action Response Property | 61 |
| 15 | Supported Properties for RADIUS Settings | 63 |
| 16 | Supported Properties for Advanced RADIUS Setting | 66 |
| 17 | Supported Properties for PAM Configuration | 68 |
| 18 | Supported Properties for Role | 73 |
| 19 | Supported Properties for CertificateService | 75 |
| 20 | Generate CSR Request Body Property | 76 |
| 21 | Generate CSR Response Property | 77 |
| 22 | Supported Properties for CertificateLocations | 79 |
| 23 | Supported Properties for Chassis | 82 |
| 24 | Chassis Type Enum Properties | 83 |
| 25 | Chassis Links Properties | 83 |
| 26 | Supported Properties for Log Service | 89 |
| 27 | Supported Properties for Log Entry | 94 |
| 28 | Supported Properties for MediaController | 100 |
| 29 | Supported Properties for Network Adapter | 103 |
| 30 | Controller Properties | 103 |
| 31 | DataCenterBridging Properties | 104 |
| 32 | VirtualizationOffload Properties | 104 |
| 33 | NPIV Properties | 104 |
| 34 | NPAR Properties | 104 |
| 35 | Supported Properties for NetworkDeviceFunction Instance | 109 |
| 36 | iSCSIBoot Properties | 110 |
| 37 | FibreChannel Properties | 111 |
| 38 | InfiniBand Properties | 112 |
| 39 | Supported Properties for VLANNetworkInterface | 122 |
| 40 | Supported Properties for VLANNetworkInterface Future State (SD) | 124 |
| 41 | Supported Properties for Ports Instance | 127 |
| 42 | LLDPTransmit/LLDPReceive Properties | 128 |
| 43 | IEEE 802.1AB-2009 chassis ID subtype | 129 |
| 44 | Supported Properties for PCIeDevice | 135 |
| 45 | PCIeInterface Properties | 135 |
| 46 | PCIe Types Properties | 136 |
| 47 | Supported Properties for PCIeFunction | 138 |
| 48 | Supported Properties for PCIeSlots | 140 |
| 49 | Slots Properties | 140 |
| 50 | Links Properties | 140 |
| 51 | Supported Properties for Power | 146 |
| 52 | PowerControl Properties | 146 |
| 53 | Chassis PowerLimitExceptionEnum Properties | 147 |
| 54 | Voltages Properties | 147 |
| 55 | PowerSupply Properties | 148 |
| 56 | Supported Properties for Sensor | 155 |
| 57 | Threshold Properties | 157 |
| 58 | Supported Properties for Thermal | 161 |
| 59 | Temperature Properties | 161 |
| 60 | ThermalPhysicalContextEnum Properties | 162 |
| 61 | Fan Properties | 163 |
| 62 | Supported Properties for Composition Service | 166 |
| 63 | Supported Properties for ResourceBlocks | 169 |
| 64 | Supported Properties for ResourceZone | 173 |
| 65 | Supported Properties for Event Service API | 175 |
| 66 | Supported Properties for Event Subscription | 183 |
| 67 | Supported Properties for SMTP Certificate OEM Properties | 186 |
| 68 | Supported Properties for JsonSchemaFile | 195 |
| 69 | JSON Schema File Location Properties | 195 |
| 70 | Supported Properties for Manager | 198 |
| 71 | SerialConsole Properties | 199 |
| 72 | CommandShell Properties | 199 |
| 73 | GraphicalConsole Properties | 200 |
| 74 | Supported Properties for Ethernet Interface | 206 |
| 75 | Supported Properties for Manager EthernetInterface | 210 |
| 76 | Supported Properties for HostInterface | 213 |
| 77 | Supported Properties for ManagerNetworkProtocol | 217 |
| 78 | Protocol Properties | 218 |
| 79 | HTTPS Protocol Properties | 218 |
| 80 | SNMP Protocol Properties | 218 |
| 81 | SSDP Protocol Properties | 218 |
| 82 | Supported Properties for SerialInterface | 223 |
| 83 | Supported Properties for VirtualMedia | 227 |
| 84 | Supported Properties for Configurations | 231 |
| 85 | Supported Properties for InventoryData Status URI | 233 |
| 86 | Allowed Values for DeletedModules | 233 |
| 87 | Enum values for Status | 233 |
| 88 | Supported Properties for MessageRegistryFile | 237 |
| 89 | Supported Properties for MessageRegistry | 239 |
| 90 | Supported Properties for AttributeRegistry | 241 |
| 91 | Property Table of Attributes | 241 |
| 92 | Property Table of Dependencies | 242 |
| 93 | Property Table for MapFrom | 243 |
| 94 | Property Table for Menus | 244 |
| 95 | Supported Properties for PrivilegeRegistry | 246 |
| 96 | PrivilegeRegistry Mapping Property | 246 |
| 97 | PrivilegeRegistry OperationMap Property | 247 |
| 98 | PrivilegeRegistry Target Privilege Map | 247 |
| 99 | PrivilegeRegistry OperationPrivilege | 247 |
| 100 | Supported Properties for Session Service | 250 |
| 101 | Supported Properties for Session | 254 |
| 102 | Supported Properties for ComputerSystem | 258 |
| 103 | ComputerSystem – Boot Properties | 260 |
| 104 | ComputerSystem – MemorySummary – MemoryMirroringEnum Properties | 261 |
| 105 | Trusted Modules Properties | 262 |
| 106 | Supported Properties for BootOption | 270 |
| 107 | Supported Properties for FRU Infomation | 272 |
| 108 | Supported Properties for GraphicsController | 275 |
| 109 | Supported Properties for Memory | 278 |
| 110 | Security States | 281 |
| 111 | Supported Properties for MemoryDomain | 287 |
| 112 | Supported Properties for MemoryChunks | 290 |
| 113 | Supported Properties for NetworkInterface | 293 |
| 114 | Supported Properties for Processor | 296 |
| 115 | Property of FPGA | 298 |
| 116 | Property of FPGA Interface | 298 |
| 117 | Enum values of Memory Type | 298 |
| 118 | Supported Properties for SecureBoot | 305 |
| 119 | Supported Properties for SecureBootDatabase | 309 |
| 120 | Supported Properties for Signature | 313 |
| 121 | Supported Properties for SimpleStorage | 317 |
| 122 | Supported Properties for Storage | 320 |
| 123 | Storage Controller Properties | 320 |
| 124 | Protocol Properties | 322 |
| 125 | Supported Properties for StorageController | 328 |
| 126 | Supported Properties for Drive | 330 |
| 127 | Supported Properties for Volume | 336 |
| 128 | Properties of ReplicaInfo | 337 |
| 129 | Supported Properties for USBController | 340 |
| 130 | Supported Properties for TaskService | 342 |
| 131 | Supported Properties for Task | 345 |
| 132 | Payload Properties | 345 |
| 133 | Supported Properties for TelemetryService | 347 |
| 134 | Supported Properties for TelemetryService Post Service | 349 |
| 135 | Supported Properties for MetricDefinition | 352 |
| 136 | Supported Properties for MetricReportDefinition | 357 |
| 137 | Supported Properties for MetricReport | 361 |
| 138 | Supported Properties for Trigger | 365 |
| 139 | Numeric Thresholds Sub-Attributes | 366 |
| 140 | Supported Properties for UpdateService | 368 |
| 141 | UpdateService Oem Object | 368 |
| 142 | UpdateService – PreserveConfiguration | 368 |
| 143 | UpdateService – SimpleUpdateActionInfo Properties | 369 |
| 144 | UpdateService – SimpleUpdateActionInfo – Parameters Properties | 369 |
| 145 | UpdateService – Actions – SimpleUpdate Properties | 369 |
| 146 | UpdateService – Response Properties | 369 |
| 147 | UpdateService – Actions – MultipartHttpPush Properties | 369 |
| 148 | UpdateService – Actions – MultipartHttpPush UpdateParameters Properties | 370 |
| 149 | UpdateService – Actions – MultipartHttpPush OemParameters Properties | 370 |
| 150 | UpdateService – Actions – UpdateService.UploadCABundle Properties | 370 |
| 151 | Supported Properties for FirmwareInventory | 377 |

## Revision history (p.21)

| Revision | Description | Date |
|---|---|---|
| 1.00 | Initial Release | December 2023 |

---

## 1.0 Introduction (p.22)

- Redfish (DMTF) RESTful, schema-based management API.
- "Redfish server follows Redfish **DSP0266 1.11.0** specification and Redfish **Schema 2019.02**."
- No board list or firmware version is given anywhere in the front matter or introduction. The document is generic to the ASRock Rack AST2600 BMC (AMI MegaRAC stack; ServiceRoot reports `Product: "AMI Redfish Server"`, `Vendor: "AMI"`, `Oem.Ami.RtpVersion: "13.04"`; see p.38).

### 1.1 Related Documents (p.22)

| Document | Version |
|---|---|
| Redfish Specification | 1.11.0 |
| Redfish Schema | 2019.2 |
| Redfish Schema Supplement | 2020.4 |

## 2.0 Redfish API (p.23)

### 2.1 URL Support (p.23)

- All resources live under `/redfish/v1/`. Pattern: `https://<BMC IP>/redfish/v1/<Path of Resource>`.
- Supported URL patterns:
  - `/redfish`: Redfish version.
  - `/redfish/v1`: version 1.x of the Redfish service (service root).
  - `/redfish/v1/odata`: OData service document.
  - `/redfish/v1/$metadata`: metadata document (XML).
  - `/redfish/v1/$metadata#<Collection or Resource Type>`: the form of `@odata.context` values.
  - `/redfish/v1/<Others Resource Specific URIs>`: all resources.

### 2.2 HTTP Request Methods (p.24)

GET (read a single resource or collection), POST (create a resource or invoke an action), PATCH (update existing resources), PUT ("replace the property value of resource"), DELETE (delete). The guide does not say which resources accept PUT; none of the resources in pp.32–130 list PUT in their method tables.

### 2.3 HTTP Status Codes (p.25, Table 1)

| Code | Description |
|---|---|
| 200 | OK |
| 201 | Created |
| 202 | Accepted |
| 204 | No Content |
| 301 | Moved Permanently |
| 302 | Found |
| 304 | Not Modified |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 405 | Method Not Allowed |
| 406 | Not Acceptable |
| 409 | Conflict |
| 410 | Gone |
| 411 | Length Required |
| 412 | Precondition Failed |
| 415 | Unsupported Media Type |
| 428 | Precondition Required |
| 431 | Request Header Field Too Large |
| 500 | Internal Server Error |
| 501 | Not Implemented |
| 503 | Service Unavailable |
| 507 | Insufficient Storage |

Note: 412/428 being listed implies ETag / `If-Match` precondition handling exists (428 = If-Match required but missing). The guide gives no further detail on ETags in chapter 2. `@odata.etag` is present in responses (for example `"\"1691843524\""` on ServiceRoot, a timestamp-like value; per Table 4, "This value gives the timestamp at which the resource properties have been initialized or modified").

### 2.4 Redfish Schema (p.26)

Schemas are defined in OData CSDL, translatable to JSON Schema.

### 2.5 Authentication (p.26)

- Supports both **Basic Authentication** (username and password on every request) and **Redfish Session Login Authentication** (for multiple requests; refers to DSP0266 1.11.0 chapter 13.3 "Redfish session login authentication", i.e. POST to `/redfish/v1/SessionService/Sessions` returning `X-Auth-Token` and `Location`). Session details are in section 3.74–3.76 (pp.250–254, part 2).

### 2.6 Roles and Privileges (p.26, Table 2)

| Predefined Role | Assigned Privileges |
|---|---|
| Administrator | Login, ConfigureManager, ConfigureUsers, ConfigureSelf, ConfigureComponents |
| Operator | Login, ConfigureSelf, ConfigureComponents |
| ReadOnly | Login, ConfigureSelf |

### 2.7 Available Redfish Resources (pp.27–31, Table 3)

The "Notes" column is empty except for `/redfish/v1` = "Service root". `Instnace` typos are reproduced from the source.

```
/redfish/v1                                                     (Service root)
/redfish/v1/AccountService
/redfish/v1/AccountService/Accounts
/redfish/v1/AccountService/Accounts/{Account-Instance}
/redfish/v1/AccountService/Accounts/{Account-Instance}/Certificates
/redfish/v1/AccountService/Accounts/{Account-Instance}/Certificates/{Certificates-Instance}
/redfish/v1/AccountService/ExternalAccountProviders
/redfish/v1/AccountService/ExternalAccountProviders/RADIUS
/redfish/v1/AccountService/ExternalAccountProviders/RADIUS/Oem/Ami/AdvancedRADIUSSetting
/redfish/v1/AccountService/LDAP/Certificates
/redfish/v1/AccountService/LDAP/Certificates/{Certificates-Instance}
/redfish/v1/AccountService/LDAP/Certificates/Oem/Ami/ClientCertificates
/redfish/v1/AccountService/LDAP/Certificates/Oem/Ami/ClientCertificates/{Certificates-Instance}
/redfish/v1/AccountService/Oem/Ami/Configurations
/redfish/v1/AccountService/Roles
/redfish/v1/AccountService/Roles/{Role-Instance}
/redfish/v1/CertificateService
/redfish/v1/CertificateService/Actions/CertificateService.GenerateCSR
/redfish/v1/CertificateService/Actions/CertificateService.ReplaceCertificate
/redfish/v1/CertificateService/CertificateLocations
/redfish/v1/Chassis
/redfish/v1/Chassis/{Chassis-Id}
/redfish/v1/Chassis/{Chassis-Id}/Actions/Chassis.Reset
/redfish/v1/Chassis/{Chassis-Id}/LogServices
/redfish/v1/Chassis/{Chassis-Id}/LogServices/{LogServices-Instance}
/redfish/v1/Chassis/{Chassis-Id}/LogServices/{LogServices-Instance}/Actions/LogService.ClearLog
/redfish/v1/Chassis/{Chassis-Id}/LogServices/{LogServices-Instance}/Entries
/redfish/v1/Chassis/{Chassis-Id}/LogServices/{LogServices-Instance}/Entries/{Chassis-LogEntry-Instance}
/redfish/v1/Chassis/{Chassis-Id}/MediaControllers
/redfish/v1/Chassis/{Chassis-Id}/MediaControllers/{MediaControllers-Instance}
/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters
/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}
/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions
/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}
/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/Ethernet/VLANs
/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/Ethernet/VLANs/{VLAN-Instance}
/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/Ethernet/VLANs/{VLAN-Instance}/SD
/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/SD
/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/Ports
/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/Ports/{Ports-Instance}
/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices
/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices/{PCIeDevice-Instance}
/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices/{PCIeDevice-Instance}/PCIeFunctions
/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices/{PCIeDevice-Instance}/PCIeFunctions/{PCIeFunctions-Instance}
/redfish/v1/Chassis/{Chassis-Id}/PCIeSlots
/redfish/v1/Chassis/{Chassis-Id}/Power
/redfish/v1/Chassis/{Chassis-Id}/ResetActionInfo
/redfish/v1/Chassis/{Chassis-Id}/Sensors
/redfish/v1/Chassis/{Chassis-Id}/Sensors/{Sensors-Instance}
/redfish/v1/Chassis/{Chassis-Id}/Thermal
/redfish/v1/CompositionService
/redfish/v1/CompositionService/ResourceBlocks
/redfish/v1/CompositionService/ResourceBlocks/ComputeBlock
/redfish/v1/CompositionService/ResourceBlocks/DrivesBlock
/redfish/v1/CompositionService/ResourceBlocks/NetworkBlock
/redfish/v1/CompositionService/ResourceZones
/redfish/v1/CompositionService/ResourceZones/{ResourceZones-Instance}
/redfish/v1/EventService
/redfish/v1/EventService/Subscriptions
/redfish/v1/EventService/Subscriptions/{Subscriptions-Instance}
/redfish/v1/EventService/Oem/Ami/SMTP/Certificates
/redfish/v1/EventService/Oem/Ami/SMTP/Certificates/{Certificates-Instance}
/redfish/v1/JsonSchemas
/redfish/v1/JsonSchemas/<json-schema-name>
/redfish/v1/Managers
/redfish/v1/Managers/{Manager-Instance}
/redfish/v1/Managers/{Manager-Instance}/Certificates
/redfish/v1/Managers/{Manager-Instance}/EthernetInterfaces
/redfish/v1/Managers/{Manager-Instance}/EthernetInterfaces/{EthernetInterfaces-Instance}
/redfish/v1/Managers/{Manager-Instance}/EthernetInterfaces/usb0
/redfish/v1/Managers/{Manager-Instance}/HostInterfaces
/redfish/v1/Managers/{Manager-Instance}/HostInterfaces/{HostInterfaces-Instance}
/redfish/v1/Managers/{Manager-Instance}/HostInterfaces/{HostInterfaces-Instance}/HostEthernetInterfaces
/redfish/v1/Managers/{Manager-Instance}/LogServices
/redfish/v1/Managers/{Manager-Instance}/LogServices/{LogServices-Instnace}
/redfish/v1/Managers/{Manager-Instance}/LogServices/{LogServices-Instnace}/Actions/LogService.ClearLog
/redfish/v1/Managers/{Manager-Instance}/LogServices/{LogServices-Instnace}/Entries
/redfish/v1/Managers/{Manager-Instance}/LogServices/{LogServices-Instnace}/Entries/{Manager-LogEntry-Instance}
/redfish/v1/Managers/{Manager-Instance}/NetworkProtocol
/redfish/v1/Managers/{Manager-Instance}/NetworkProtocol/HTTPS/Certificates
/redfish/v1/Managers/{Manager-Instance}/ResetActionInfo
/redfish/v1/Managers/{Manager-Instance}/SerialInterfaces
/redfish/v1/Managers/{Manager-Instance}/SerialInterfaces/{SerialInterfaces-Instance}
/redfish/v1/Managers/{Manager-Instance}/VirtualMedia
/redfish/v1/Managers/{Manager-Instance}/VirtualMedia/{VirtualMedia-Instance}
/redfish/v1/Managers/{Manager-Instance}/VirtualMedia/{VirtualMedia-Instance}/Actions/VirtualMedia.InsertMedia
/redfish/v1/Managers/{Manager-Instance}/VirtualMedia/{VirtualMedia-Instance}/Actions/VirtualMedia.EjectMedia
/redfish/v1/Oem/Ami/Configurations
/redfish/v1/Oem/Ami/InventoryData/Status
/redfish/v1/Registries
/redfish/v1/Registries/{Registries-Instance}
/redfish/v1/Registries/{AttributeRegistry-Instance.json}
/redfish/v1/Registries/{PrivilegeRegistry-Instance.json}
/redfish/v1/Registries/{Registry-Instance.json}
/redfish/v1/SessionService
/redfish/v1/SessionService/Sessions
/redfish/v1/SessionService/Sessions/{Sessions-Id}
/redfish/v1/Systems
/redfish/v1/Systems/{Systems-Instance}
/redfish/v1/Systems/{Systems-Instance}/Actions/ComputerSystem.Reset
/redfish/v1/Systems/{Systems-Instance}/Boot/Certificates
/redfish/v1/Systems/{Systems-Instance}/Boot/Certificates/{Certificates-Instance}
/redfish/v1/Systems/{Systems-Instance}/BootOptions
/redfish/v1/Systems/{Systems-Instance}/BootOptions/{BootOptions-Instance}
/redfish/v1/Systems/{Systems-Instance}/EthernetInterfaces
/redfish/v1/Systems/{Systems-Instance}/EthernetInterfaces/{EthernetInterfaces-Instance}
/redfish/v1/Systems/{Systems-Instance}/EthernetInterfaces/{EthernetInterfaces-Instance}/VLANs
/redfish/v1/Systems/{Systems-Instance}/EthernetInterfaces/{EthernetInterfaces-Instance}/VLANs/{VLAN-Instance}
/redfish/v1/Systems/{Systems-Instance}/FruInfo
/redfish/v1/Systems/{Systems-Instance}/GraphicsControllers
/redfish/v1/Systems/{Systems-Instance}/GraphicsControllers/{GraphicsController-Instance}
/redfish/v1/Systems/{Systems-Instance}/GraphicsControllers/{GraphicsController-Instance}/Ports
/redfish/v1/Systems/{Systems-Instance}/GraphicsControllers/{GraphicsController-Instance}/Ports/{Port-Instance}
/redfish/v1/Systems/{Systems-Instance}/LogServices
/redfish/v1/Systems/{Systems-Instance}/LogServices/{LogServices-Instnace}
/redfish/v1/Systems/{Systems-Instance}/LogServices/{LogServices-Instnace}/Entries
/redfish/v1/Systems/{Systems-Instance}/LogServices/{LogServices-Instnace}/Entries/{System-LogEntry-Instance}
/redfish/v1/Systems/{Systems-Instance}/Memory
/redfish/v1/Systems/{Systems-Instance}/Memory/{Memory-Instnace}
/redfish/v1/Systems/{Systems-Instance}/Memory/{Memory-Instnace}/Actions/AmiBios.ChangeState
/redfish/v1/Systems/{Systems-Instance}/MemoryDomains
/redfish/v1/Systems/{Systems-Instance}/MemoryDomains/{MemoryDomains-Instance}
/redfish/v1/Systems/{Systems-Instance}/MemoryDomains/{MemoryDomains-Instance}/MemoryChunks
/redfish/v1/Systems/{Systems-Instance}/MemoryDomains/{MemoryDomains-Instance}/MemoryChunks/{MemoryChunks-Instance}
/redfish/v1/Systems/{Systems-Instance}/NetworkInterfaces
/redfish/v1/Systems/{Systems-Instance}/NetworkInterfaces/{NetworkInterfaces-Instance}
/redfish/v1/Systems/{Systems-Instance}/NetworkInterfaces/{NetworkInterfaces-Instance}/NetworkDeviceFunctions
/redfish/v1/Systems/{Systems-Instance}/Processors
/redfish/v1/Systems/{Systems-Instance}/Processors/{Processors-Instance}
/redfish/v1/Systems/{Systems-Instance}/Processors/{Processors-Instance}/SubProcessors
/redfish/v1/Systems/{Systems-Instance}/Processors/{Processors-Instance}/SubProcessors/{SubProcessors-Instance}
/redfish/v1/Systems/{Systems-Instance}/ResetActionInfo
/redfish/v1/Systems/{Systems-Instance}/SD
/redfish/v1/Systems/{Systems-Instance}/SecureBoot
/redfish/v1/Systems/{Systems-Instance}/SecureBoot/ResetKeysActionInfo
/redfish/v1/Systems/{Systems-Instance}/SecureBoot/Actions/SecureBoot.ResetKeys
/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases
/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}
/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}/Signatures
/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}/Signatures/{Signatures-Instance}
/redfish/v1/Systems/{Systems-Instance}/SimpleStorage
/redfish/v1/Systems/{Systems-Instance}/SimpleStorage/{SimpleStorage-Instance}
/redfish/v1/Systems/{Systems-Instance}/Storage
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Controllers
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Controllers/{Controllers-Instance}
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Drives/{Drives-Instance}
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Volumes
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Volumes/{Volumes-Instance}
/redfish/v1/Systems/{Systems-Instance}/USBControllers
/redfish/v1/Systems/{Systems-Instance}/USBControllers/{USBControllers-Instance}
/redfish/v1/Systems/{Systems-Instance}/USBControllers/{USBControllers-Instance}/Ports
/redfish/v1/Systems/{Systems-Instance}/USBControllers/{USBControllers-Instance}/Ports/{Port-Instance}
/redfish/v1/TaskService
/redfish/v1/TaskService/Tasks
/redfish/v1/TaskService/Tasks/{Task-Instance}
/redfish/v1/TelemetryService
/redfish/v1/TelemetryService/LogService
/redfish/v1/TelemetryService/LogService/Entries
/redfish/v1/TelemetryService/LogService/Entries/{Entry-Instance}
/redfish/v1/TelemetryService/MetricDefinitions
/redfish/v1/TelemetryService/MetricDefinitions/{MetricDefinition-Instance}
/redfish/v1/TelemetryService/MetricReportDefinitions
/redfish/v1/TelemetryService/MetricReportDefinitions/{MetricReportDefinition-Instance}
/redfish/v1/TelemetryService/MetricReports
/redfish/v1/TelemetryService/MetricReports/{MetricReport-Instance}
/redfish/v1/TelemetryService/Triggers
/redfish/v1/TelemetryService/Triggers/{Trigger-Instnace}
/redfish/v1/UpdateService
/redfish/v1/UpdateService/FirmwareInventory
/redfish/v1/UpdateService/FirmwareInventory/{FirmwareInventory-Instance}
/redfish/v1/UpdateService/upload
```

Notable OEM/non-standard paths: `.../Oem/Ami/AdvancedRADIUSSetting`, `/AccountService/LDAP/Certificates/Oem/Ami/ClientCertificates`, `/AccountService/Oem/Ami/Configurations`, `/EventService/Oem/Ami/SMTP/Certificates`, `/Oem/Ami/Configurations`, `/Oem/Ami/InventoryData/Status`, `/Systems/{id}/FruInfo`, `.../SD` ("Settings Data"/future-state objects on Systems, NetworkDeviceFunctions, VLANs), `Memory/{id}/Actions/AmiBios.ChangeState`, `/UpdateService/upload`, `Managers/{id}/EthernetInterfaces/usb0`.

---

## 3.0 Redfish Resources (p.32)

### 3.1 ODATA and Frequently Used Properties (pp.32–33, Table 4)

| Property | Type | Read Only | Description |
|---|---|---|---|
| @odata.context | String | Yes | Context URL describing the resource per OData |
| @odata.id | String | Yes | Unique identifier (URI) of the resource |
| @odata.type | String | Yes | Absolute URL of the schema type |
| @odata.etag | (blank) | (blank) | ETags provide ability to conditionally retrieve or update a resource. "This value gives the timestamp at which the resource properties have been initialized or modified." |
| @odata.nextLink | String | Yes | URI of the next set of partial members (paging) |
| Id | String | Yes | Unique id within collection |
| Name | String | Yes | Name |
| Description | Null, String | Yes | Description |
| UUID | String | Yes | UUID |
| Oem | Object | No | OEM property / OemObject |
| Members | Array | Yes | Collection members |
| Members@odata.count | Number | Yes | Collection member count |
| Status.State | String | Yes | Enum: `Enabled`, `Disabled`, `StandbyOffline` (enabled but awaiting external action to activate), `StandbySpare` (part of redundancy set awaiting failover), `InTest`, `Starting`, `Absent` (not present or not detected), `UnavailableOffline` (present but cannot be used), `Deferring` (will not process commands but queues new requests), `Quiesced` (enabled, processes a restricted set of commands), `Updating` (updating, may be unavailable or degraded) |
| Reset (ResetType) | String | Yes | `On`, `ForceOff` (immediate non-graceful), `GracefulShutdown` (graceful shutdown and power off), `ForceRestart` (immediate shutdown then restart), `PowerCycle` |
| Status.HealthRollup | String | Yes | Health of resource and dependents: `OK`, `Warning`, `Critical` |
| Status.Health | String | Yes | Health of resource only: `OK`, `Warning`, `Critical` |
| IndicatorLED | String | Yes | `Lit`, `Blinking`, `Off` |

#### Resource.v1_8_1 Schema Properties (pp.33–35, Table 5)

Identifier:

| Property | Type | Read Only | Description |
|---|---|---|---|
| DurableName | String | True | World-wide unique identifier |
| DurableNameFormat | String | True | Enum: `NAA` (hex Name Address Authority, T11 FC-FS-3), `FC_WWN` (hex WWN per T11 FC-PI), `UUID` (hex UUID per ITU OSI), `EUI` (hex IEEE EUI-64), `iQN` (iSCSI Qualified Name, RFC 3720/3721) |

Location:

| Property | Type | Read Only | Description |
|---|---|---|---|
| AltitudeMeters | Number | True | Altitude (m) |
| Latitude | Number | True | Latitude |
| Longitude | Number | True | Longitude (degrees) |
| Contacts | Array | True | Contact info array |
| Contacts[].ContactName | String | False | Contact name |
| Contacts[].EmailAddress | String | False | Contact email |
| PartLocation | Object | True | Location of the part |
| PartLocation.LocationOrdinalValue | Number | True | Ordinal (e.g. slot 2 => 2) |
| PartLocation.LocationType | Object (enum) | True | `Slot`, `Bay`, `Connector`, `Socket` |
| PartLocation.Orientation | Object (enum) | True | `FrontToBack`, `BackToFront`, `TopToBottom`, `BottomToTop`, `LeftToRight`, `RightToLeft` |
| PartLocation.Reference | Object (enum) | True | `Top`, `Bottom`, `Front`, `Rear`, `Left`, `Right`, `Middle` |
| PartLocation.ServiceLabel | String | True | Service label |
| PostalAddress | Object | False | Postal address. All sub-fields String, writable: AdditionalInfo, AdditionalCode (RFC5139 ADDCODE), Building, City, Community, Country (RFC5139), District, Division, Floor (RFC5139 FLR), HouseNumber, HouseNumberSuffix, Landmark (RFC5139 LMK), LeadingStreetDirection, Location (room designation), Name (RFC5139 NAM, occupant), POBox, PlaceType, PostalCode, Road, RoadBranch, RoadPostModifier, RoadPreModifier, RoadSection, RoadSubBranch, Room, Seat, Street, StreetSuffix, Territory, TrailingStreetSuffix, Unit, Neighborhood |
| Placement | Object | False | Placement |
| Placement.AdditionalInfo | String | False | Additional info |
| Placement.Rack | String | False | Rack name within row |
| Placement.RackOffset | Number | False | Vertical location in RackOffsetUnits |
| Placement.RackOffsetUnits | String | False | `OpenU` (48 mm / 1.89 in), `EIA_310` (1.75 in / 44.45 mm) |
| Placement.Row | String | False | Row name |

### 3.2 ServiceRoot (pp.36–39)

- **URL:** `/redfish/v1/`
- **Methods/privileges:** GET: Login. (Note: the DMTF spec allows unauthenticated GET of the service root; this guide lists Login as required.)

Supported properties (Table 6, all read-only):

| Property | Type | Description |
|---|---|---|
| AccountService | Object | Link to Account Service |
| CertificateService | Object | Link to Certificate Service |
| Chassis | Object | Link to Chassis collection |
| CompositionService | Object | Link to Composition Service |
| EventService | Object | Link to Event Service |
| JsonSchemas | Object | Link to JsonSchemas collection |
| Managers | Object | Link to Managers collection |
| Product | String | Product name |
| ProtocolFeaturesSupported | Object | Protocol features |
| ProtocolFeaturesSupported.DeepOperations.DeepPATCH | Boolean | Deep PATCH supported |
| ProtocolFeaturesSupported.DeepOperations.DeepPOST | Boolean | Deep POST supported |
| ProtocolFeaturesSupported.DeepOperations.MaxLevels | Number | Max levels for deep operations |
| ProtocolFeaturesSupported.ExcerptQuery | Boolean | `excerpt` query supported |
| ProtocolFeaturesSupported.ExpandQuery.ExpandAll | Boolean | `$expand=*` supported |
| ProtocolFeaturesSupported.ExpandQuery.Levels | Boolean | `$levels` supported |
| ProtocolFeaturesSupported.ExpandQuery.Links | Boolean | `$expand=~` supported |
| ProtocolFeaturesSupported.ExpandQuery.MaxLevels | Number | Max `$levels` |
| ProtocolFeaturesSupported.ExpandQuery.NoLinks | Boolean | `$expand=.` supported |
| ProtocolFeaturesSupported.FilterQuery | Boolean | `$filter` supported |
| ProtocolFeaturesSupported.OnlyMemberQuery | Boolean | `only` supported |
| ProtocolFeaturesSupported.SelectQuery | Boolean | `$select` supported |
| RedfishVersion | String | Redfish version |
| SessionService | Object | Link to Session Service |
| Systems | Object | Link to Systems collection |
| Tasks | Object | Link to Task Service |
| TelemetryService | Object | Link to Telemetry Service |
| UpdateService | Object | Link to Update Service |
| Vendor | String | Vendor |

#### GET ServiceRoot (pp.37–39)

Request:
```
GET https://{BMC_IP}/redfish/v1/
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#ServiceRoot.ServiceRoot",
    "@odata.etag": "\"1691843524\"",
    "@odata.id": "/redfish/v1/",
    "@odata.type": "#ServiceRoot.v1_7_0.ServiceRoot",
    "AccountService": { "@odata.id": "/redfish/v1/AccountService" },
    "CertificateService": { "@odata.id": "/redfish/v1/CertificateService" },
    "Chassis": { "@odata.id": "/redfish/v1/Chassis" },
    "CompositionService": { "@odata.id": "/redfish/v1/CompositionService" },
    "Description": "The service root for all Redfish requests on this host",
    "EventService": { "@odata.id": "/redfish/v1/EventService" },
    "Id": "RootService",
    "JsonSchemas": { "@odata.id": "/redfish/v1/JsonSchemas" },
    "Links": {
        "Sessions": { "@odata.id": "/redfish/v1/SessionService/Sessions" }
    },
    "Managers": { "@odata.id": "/redfish/v1/Managers" },
    "Name": "Root Service",
    "Oem": {
        "Ami": {
            "@odata.type": "#AMIServiceRoot.v1_0_0.AMIServiceRoot",
            "Configurations": { "@odata.id": "/redfish/v1/Oem/Ami/Configurations" },
            "InventoryDataStatus": { "@odata.id": "/redfish/v1/Oem/Ami/InventoryData/Status" },
            "RtpVersion": "13.04"
        }
    },
    "Product": "AMI Redfish Server",
    "ProtocolFeaturesSupported": {
        "DeepOperations": { "DeepPATCH": true, "DeepPOST": false, "MaxLevels": 6 },
        "ExcerptQuery": true,
        "ExpandQuery": {
            "ExpandAll": true,
            "Levels": true,
            "Links": true,
            "MaxLevels": 5,
            "NoLinks": true
        },
        "FilterQuery": true,
        "OnlyMemberQuery": true,
        "SelectQuery": true
    },
    "RedfishVersion": "1.11.0",
    "Registries": { "@odata.id": "/redfish/v1/Registries" },
    "SessionService": { "@odata.id": "/redfish/v1/SessionService" },
    "Systems": { "@odata.id": "/redfish/v1/Systems" },
    "Tasks": { "@odata.id": "/redfish/v1/TaskService" },
    "TelemetryService": { "@odata.id": "/redfish/v1/TelemetryService" },
    "UUID": "6698714e-bfde-11d3-01ab-bce9f759a1a8",
    "UpdateService": { "@odata.id": "/redfish/v1/UpdateService" },
    "Vendor": "AMI"
}
```
Notes: the example sends `Content-Type` on a GET; this is harmless. DeepPATCH is true (max 6 levels) and DeepPOST is false. `$expand` is supported with MaxLevels 5. OEM namespace is `Oem.Ami`; no `Oem.ASRockRack` is seen here.

---

### 3.3 AccountService (pp.40–46)

- **URL:** `/redfish/v1/AccountService`
- **Methods/privileges:** GET: ConfigureUsers, ConfigureManager. PATCH: ConfigureUsers.

Supported properties (Table 7, pp.40–42):

| Property | Type | Read Only | Description |
|---|---|---|---|
| ServiceEnabled | Boolean | False | Whether the service is enabled |
| AuthFailureLoggingThreshold | Number | False | Threshold at which an authorization failure is logged |
| MinPasswordLength | Number | True | Minimum password length |
| MaxPasswordLength | Number | True | Maximum password length |
| AccountLockoutThreshold | Number | False | Failed login attempts before the account is locked |
| AccountLockoutDuration | Number | False | Seconds the account stays locked. Must be >= AccountLockoutCounterResetAfter. 0 = no lockout. Min 0 |
| AccountLockoutCounterResetAfter | Number | False | Seconds after the last failed login before the failure counter resets. Must be <= AccountLockoutDuration. The counter also resets after a successful login. Min 0 |
| Accounts | Object | True | Link to ManagerAccountCollection |
| Roles | Object | True | Link to RoleCollection |
| PrivilegeMap | Object | True | Link to Privilege Registry |
| Actions | Object | True | Actions (under Oem, if any) |
| LocalAccountAuth | String | False | How the service uses the local Accounts collection during authentication (example value `LocalFirst`) |
| AccountLockoutCounterResetEnabled | Boolean | False | Whether the counter resets after AccountLockoutCounterResetAfter expires |
| LDAP | Object | False | LDAP properties |
| LDAP.Authentication | Object | False | Authentication details |
| LDAP.Authentication.AuthenticationType | String | True | Authentication type |
| LDAP.Authentication.Username | String | False | Bind user name |
| LDAP.Authentication.Password | String | False | Bind password |
| LDAP.Authentication.Oem | Object | False | OEM extension |
| LDAP.LDAPService | Object | False | LDAP service properties |
| LDAP.LDAPService.SearchSettings | Object | False | Search settings |
| ...SearchSettings.BaseDistinguishedNames | Array | False | Base DNs |
| ...SearchSettings.GroupNameAttribute | String | False | Attribute holding group name |
| ...SearchSettings.GroupsAttribute | String | False | Attribute holding a user's groups |
| ...SearchSettings.UsernameAttribute | String | False | Attribute holding username |
| LDAP.RemoteRoleMapping | Array | False | Remote-to-local role mapping rules |
| RemoteRoleMapping[].LocalRole | String | False | Local Redfish role name |
| RemoteRoleMapping[].RemoteGroup | String | False | Remote group |
| RemoteRoleMapping[].RemoteUser | String | False | Remote user |
| RemoteRoleMapping[].Oem (Oem.Ami) | Object | False | OEM extension |
| Oem.Ami.KVMAccess | String | False | KVM access for the role mapping (example `"Enable"`) |
| Oem.Ami.VMediaAccess | String | False | Virtual media access for the role mapping (example `"Enable"`) |
| Oem.Ami.RoleID | Number | True | ID of the role in RoleMapping |
| Oem.Ami.EncryptionType | String | False | Encryption for username/password: `NoEncryption`, `SSL`, `StartTLS` |
| Oem.Ami.CommonNameType | String | False | Server name type: `IPAddress`, `FQDN`. Note: FQDN can be patched only when EncryptionType is StartTLS |
| LDAP.ServiceAddresses | Array | False | Provider addresses (format depends on provider; example `"192.168.235.125:389"`) |
| LDAP.ServiceEnabled | Boolean | True (as printed) | Whether the service is enabled. Printed as read-only, but the PATCH example sets it to `true` |
| ActiveDirectory | Object | False | Active Directory properties |
| ActiveDirectory.Authentication.Username | String | False | User name |
| ActiveDirectory.Authentication.Password | String | False | Password |
| ActiveDirectory.Authentication.AuthenticationType | String | False | Value is `"UsernameAndPassword"` for AD |
| ActiveDirectory.Authentication.Oem (Ami) | Object | False | OEM extension |
| ...Oem.Ami.DomainName | String | False | Domain name for the user |
| ...Oem.Ami.DomainControllerServerAddr1 / Addr2 / Addr3 | String | False | AD server IP (IPv4 or IPv6). At least one must be configured |
| ...Oem.Ami.KVMAccess | String | False | KVM access of the role in RoleMapping |
| ...Oem.Ami.VMediaAccess | String | False | VMedia access of the role in RoleMapping |
| ...Oem.Ami.GroupID | Number | True | GroupID of the five available roles in AD RoleMapping |
| ...Oem.Ami.RoleID | Number | True | ID of the five available roles in AD RoleMapping |
| AdditionalExternalAccountProviders | Object | False | Additional external account providers |

(The table's nesting places DomainName, the DomainControllerServerAddr fields, KVMAccess, VMediaAccess, GroupID and RoleID under `ActiveDirectory...Oem`. In the examples, DomainName and the DC addresses sit under `ActiveDirectory.Authentication.Oem.Ami`, while KVMAccess and VMediaAccess sit under `RemoteRoleMapping[].Oem.Ami`.)

Caveat: RemoteRoleMapping is a **fixed 5-slot array** for both LDAP and AD. GET returns `[null,null,null,null,null]` when nothing is configured, and setting a slot to `null` in PATCH deletes that mapping.

#### GET AccountService (pp.42–44)

```
GET https://{BMC_IP}/redfish/v1/AccountService
Content-Type: application/json
```
```json
{
    "@odata.context": "/redfish/v1/$metadata#AccountService.AccountService",
    "@odata.etag": "\"1691848924\"",
    "@odata.id": "/redfish/v1/AccountService",
    "@odata.type": "#AccountService.v1_6_0.AccountService",
    "AccountLockoutCounterResetAfter": 30,
    "AccountLockoutCounterResetEnabled": true,
    "AccountLockoutDuration": 30,
    "AccountLockoutThreshold": 5,
    "Accounts": { "@odata.id": "/redfish/v1/AccountService/Accounts" },
    "ActiveDirectory": {
        "Authentication": {
            "AuthenticationType": null,
            "Password": null,
            "Username": ""
        },
        "RemoteRoleMapping": [ null, null, null, null, null ],
        "ServiceEnabled": false
    },
    "AdditionalExternalAccountProviders": {
        "@odata.id": "/redfish/v1/AccountService/ExternalAccountProviders"
    },
    "AuthFailureLoggingThreshold": 3,
    "Description": "Redfish User Accounts",
    "Id": "AccountService",
    "LDAP": {
        "Authentication": {
            "AuthenticationType": null,
            "Password": null,
            "Username": ""
        },
        "LDAPService": {
            "SearchSettings": {
                "BaseDistinguishedNames": [ null ],
                "UsernameAttribute": null
            }
        },
        "RemoteRoleMapping": [ null, null, null, null, null ],
        "ServiceAddresses": [ null ],
        "ServiceEnabled": false
    },
    "LocalAccountAuth": "LocalFirst",
    "MaxPasswordLength": 20,
    "MinPasswordLength": 8,
    "Name": "Account Service",
    "Oem": {
        "Ami": {
            "@odata.type": "#AMIAccountService.v1_0_0.AMIAccountService",
            "Configuration": {
                "@odata.id": "/redfish/v1/AccountService/Oem/Ami/Configurations"
            }
        }
    },
    "PrivilegeMap": {
        "@odata.id": "/redfish/v1/Registries/Redfish_1.0.3_PrivilegeRegistry.json"
    },
    "Roles": { "@odata.id": "/redfish/v1/AccountService/Roles" },
    "ServiceEnabled": true,
    "Status": { "Health": "OK", "State": "Enabled" }
}
```
Defaults shown: password length 8–20, lockout after 5 failures for 30 s, counter reset 30 s, AuthFailureLoggingThreshold 3. Passwords are returned as `null`.

#### PATCH AccountService (p.44)

`PATCH https://{BMC_IP}/redfish/v1/AccountService` (Content-Type: application/json)
```json
{
    "AccountLockoutCounterResetAfter": 60,
    "AccountLockoutCounterResetEnabled": true,
    "AccountLockoutDuration": 60,
    "AccountLockoutThreshold": 10
}
```

#### PATCH LDAP & LDAP Service (pp.44–45)

Same URL.
```json
{
    "LDAP": {
        "Authentication": {
            "Password": "test@123",
            "Username": "cn=admin,dc=testldap,dc=com"
        },
        "LDAPService": {
            "SearchSettings": {
                "BaseDistinguishedNames": [ "dc=testldap,dc=com" ],
                "UsernameAttribute": "cn"
            }
        },
        "ServiceAddresses": [ "192.168.235.125:389" ],
        "ServiceEnabled": true
    }
}
```

#### PATCH LDAP RemoteRoleMapping (p.45)

Same URL. The `//` comments are in the source; strip them in real requests.
```json
{
    "LDAP": {
        "RemoteRoleMapping": [
            {
                "LocalRole": "Administrator",
                "Oem": {
                    "Ami": {
                        "KVMAccess": "Enable",
                        "VMediaAccess": "Enable"
                    }
                },
                "RemoteGroup": "group1",
                "RemoteUser": "dc=com"
            },
            null,   // 2nd role if any, will be deleted.
            null,   // 3rd role if any, will be deleted.
            null,   // 4th role if any, will be deleted.
            null    // 5th role if any, will be deleted.
        ]
    }
}
```

#### PATCH Active Directory (pp.45–46)

Same URL.
```json
{
    "ActiveDirectory": {
        "Authentication": {
            "Password": "ad01@123",
            "Username": "AD01",
            "Oem": {
                "Ami": {
                    "DomainName": "ad01.com",
                    "DomainControllerServerAddr1": "192.168.235.111",
                    "DomainControllerServerAddr2": "",
                    "DomainControllerServerAddr3": ""
                }
            }
        },
        "ServiceEnabled": true
    }
}
```

#### PATCH Active Directory RemoteRoleMapping (p.46)

Same URL.
```json
{
    "ActiveDirectory": {
        "RemoteRoleMapping": [
            {
                "LocalRole": "Administrator",
                "RemoteGroup": "group1",
                "RemoteUser": "ADUser01",
                "Oem": {
                    "Ami": {
                        "KVMAccess": "Enable",
                        "VMediaAccess": "Enable"
                    }
                }
            },
            null,   // 2nd role if any, will be deleted.
            null,   // 3rd role if any, will be deleted.
            null,   // 4th role if any, will be deleted.
            null    // 5th role if any, will be deleted.
        ]
    }
}
```

---

### 3.4 ManagerAccountCollection (pp.47–48)

- **URL:** `/redfish/v1/AccountService/Accounts`
- **Methods/privileges:** GET: ConfigureUsers, ConfigureManager. POST: ConfigureUsers.
- Properties: common OData/collection properties only (section 3.1).

#### GET (p.47)

```json
{
    "@odata.context": "/redfish/v1/$metadata#ManagerAccountCollection.ManagerAccountCollection",
    "@odata.etag": "\"1691926015\"",
    "@odata.id": "/redfish/v1/AccountService/Accounts",
    "@odata.type": "#ManagerAccountCollection.ManagerAccountCollection",
    "Description": "Collection for Manager Accounts",
    "Members": [
        { "@odata.id": "/redfish/v1/AccountService/Accounts/1" },
        { "@odata.id": "/redfish/v1/AccountService/Accounts/4" }
    ],
    "Members@odata.count": 2,
    "Name": "Accounts Collection"
}
```
Note: account IDs are numeric slot IDs (IPMI-style user IDs; 1 and 4 in the example, so the IDs are not contiguous).

#### POST: create account (pp.47–48)

`POST https://{BMC_IP}/redfish/v1/AccountService/Accounts` (Content-Type: application/json)
```json
{
    "Name": "Test User Account",
    "Description": "Test User Account",
    "Enabled": true,
    "Password": "test@123",
    "UserName": "testuser01",
    "RoleId": "Operator",
    "Locked": false
}
```

#### POST: create SNMP account (p.48)

```json
{
    "Name": "Test User Account",
    "Description": "Test User Account",
    "Enabled": true,
    "Password": "snmptestuser@123",
    "UserName": "snmptestuser01",
    "RoleId": "Operator",
    "Locked": false,
    "AccountTypes": [ "Redfish", "SNMP" ],
    "Oem": {
        "Ami": {
            "SNMP": {
                "AuthenticationProtocol": "SHA256",
                "EncryptionProtocol": "DES",
                "Access": "ReadOnly"
            }
        }
    }
}
```
Response: **201**, and the body is the GET representation of the new Account.

---

### 3.5 Manager Account (pp.49–52)

- **URL:** `/redfish/v1/AccountService/Accounts/{Account-Instance}`
- **Methods/privileges:** GET: ConfigureSelf, ConfigureUsers, ConfigureManager. PATCH: ConfigureUsers. DELETE: ConfigureUsers.

Supported properties (Table 8, pp.49–50):

| Property | Type | Read Only | Description |
|---|---|---|---|
| UserName | String | False | User name |
| Password | String | False | Password (returned as null) |
| RoleId | String | False | Role ID |
| Locked | Boolean | False | True when the service auto-locked the account after AccountLockoutThreshold was exceeded |
| Enabled | Boolean | False | Enable or disable future logins. "The value of Enable overrides the locked property." |
| Actions | Object | True | Actions (under Oem) |
| PasswordChangeRequired | Boolean | True | Password must be changed before access is allowed |
| Certificates | Object | True | Link to the account's certificate collection |
| AccountTypes | Array | False | Allowed: `Redfish` (access to the Redfish service), `SNMP` (access to the SNMP service) |
| SNMP | Object | True | SNMP settings |
| SNMP.AuthenticationKeySet | Boolean | True | Authentication key is set |
| SNMP.EncryptionKeySet | Boolean | True | Encryption key is set |
| Oem.Ami.SNMP | Object | False | SNMP configuration (OEM) |
| Oem.Ami.SNMP.AuthenticationProtocol | String | False | SNMPv3 auth: `SHA256`, `SHA384`, `SHA512` |
| Oem.Ami.SNMP.EncryptionProtocol | String | False | SNMPv3 privacy: `DES`, `AES` |
| Oem.Ami.SNMP.Access | String | False | `ReadOnly`, `ReadWrite` |

DMTF difference: SNMPv3 settings are configured via `Oem.Ami.SNMP` rather than the standard `SNMP.AuthenticationProtocol` / `EncryptionProtocol` properties.

#### GET (p.50)

```json
{
    "@odata.context": "/redfish/v1/$metadata#ManagerAccount.ManagerAccount",
    "@odata.etag": "\"1691926015\"",
    "@odata.id": "/redfish/v1/AccountService/Accounts/1",
    "@odata.type": "#ManagerAccount.v1_5_0.ManagerAccount",
    "AccountTypes": [ "Redfish" ],
    "Certificates": { "@odata.id": "/redfish/v1/AccountService/Accounts/1/Certificates" },
    "Description": "Default Account",
    "Enabled": true,
    "Id": "1",
    "Links": {
        "Role": { "@odata.id": "/redfish/v1/AccountService/Roles/Administrator" }
    },
    "Locked": false,
    "Name": "Default Account",
    "Password": null,
    "PasswordChangeRequired": true,
    "RoleId": "Administrator",
    "UserName": "Administrator"
}
```
Note: the default account 1 is `Administrator` with `PasswordChangeRequired: true`. Until the password is changed, the service may restrict access (DMTF behaviour: only a PATCH of the password is allowed).

#### PATCH (pp.50–51)

```json
{
    "UserName": "testuser02",
    "Password": "testuser2@123",
    "RoleId": "Administrator",
    "Locked": false,
    "Enabled": true
}
```

#### PATCH: add SNMP to account (p.51)

```json
{
    "AccountTypes": [ "Redfish", "SNMP" ],
    "Oem": {
        "Ami": {
            "SNMP": {
                "AuthenticationProtocol": "SHA512",
                "EncryptionProtocol": "AES",
                "Access": "ReadOnly"
            }
        }
    }
}
```

#### PATCH: remove SNMP from account (p.51)

```json
{ "AccountTypes": [ "Redfish" ] }
```
Response: **204**, no body.

#### DELETE (pp.51–52)

`DELETE https://{BMC_IP}/redfish/v1/AccountService/Accounts/{Account-Instance}`. Response: **204**, no body.

---

### 3.6 CertificateCollection (pp.53–56)

- **URLs:**
  - `/redfish/v1/AccountService/Accounts/{Account-Instance}/Certificates`
  - `/redfish/v1/AccountService/LDAP/Certificates`
  - `/redfish/v1/AccountService/LDAP/Certificates/Oem/Ami/ClientCertificates`
  - `/redfish/v1/Managers/{Manager-Instance}/NetworkProtocol/HTTPS/Certificates`
  - `/redfish/v1/Systems/{Systems-Instance}/Boot/Certificates`
- **Methods/privileges:** GET: ConfigureSelf, ConfigureUsers, ConfigureManager. POST: ConfigureManager.

#### GET (p.53)

```json
{
    "@odata.context": "/redfish/v1/$metadata#CertificateCollection.CertificateCollection",
    "@odata.etag": "\"1694266264\"",
    "@odata.id": "/redfish/v1/AccountService/Accounts/1/Certificates",
    "@odata.type": "#CertificateCollection.CertificateCollection",
    "Description": "Certificates Collection for Account Administrator",
    "Members": [
        { "@odata.id": "/redfish/v1/AccountService/Accounts/1/Certificates/1" }
    ],
    "Members@odata.count": 1,
    "Name": "Certificates Collection"
}
```

#### POST: install certificate (pp.53–56)

`POST https://{BMC_IP}/redfish/v1/AccountService/Accounts/{Account-Instance}/Certificates` (Content-Type: application/json)

Request properties (Table 9):

| Property | Type | Description |
|---|---|---|
| CertificateString | String | Certificate string in the CertificateType format. Private keys are stripped from responses. If the service does not know the private key and needs it, the client includes the private key in the string. **Notes:** only **PKCS#1 and PKCS#8 (unencrypted)** private keys are supported; all end-of-line characters from the cert/key file must be kept and written as `\n` in the request body; only a certificate or private key in the correct CertificateType format is recognised, and other strings are ignored. |
| CertificateType | String | Per the table, "the link to the Certificate Resource Collection where the certificate is installed" (a doc error; it is actually the format). **Only `PEM` is supported.** |

Example request (PEM bodies elided here; the source shows a full cert plus a concatenated private key):
```json
{
    "CertificateString": "-----BEGIN CERTIFICATE-----\nMIIEITCCAwmgAwIBAgIUJLCjFTUASxoFVsyHU18s/+Geo6gw...[base64 elided]...\n-----END CERTIFICATE-----\n    -----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASC...[base64 elided]...\n-----END PRIVATE KEY-----",
    "CertificateType": "PEM"
}
```
Certificate and private key go in the same `CertificateString`, concatenated.

Response: **201**, and the body is the GET representation of the new certificate (pp.55–56). Note that the example `@odata.id` printed in the response is the collection URI.
```json
{
    "@odata.context": "/redfish/v1/$metadata#Certificate.Certificate",
    "@odata.etag": "\"1694266264\"",
    "@odata.id": "/redfish/v1/AccountService/Accounts/1/Certificates",
    "@odata.type": "#Certificate.v1_4_0.Certificate",
    "Actions": {
        "#Certificate.Rekey": {
            "@Redfish.ActionInfo": "/redfish/v1/AccountService/Accounts/1/Certificates/1/Certificate.RekeyActionInfo",
            "target": "/redfish/v1/AccountService/Accounts/1/Certificates/1/Actions/Certificate.Rekey"
        },
        "#Certificate.Renew": {
            "@Redfish.ActionInfo": "/redfish/v1/AccountService/Accounts/1/Certificates/1/Certificate.RenewActionInfo",
            "target": "/redfish/v1/AccountService/Accounts/1/Certificates/1/Actions/Certificate.Renew"
        }
    },
    "CertificateString": "-----BEGIN CERTIFICATE-----\n...[elided]...\n-----END  CERTIFICATE-----",
    "CertificateType": "PEM",
    "Description": "Certificate Instance 1",
    "Id": "1",
    "Issuer": {
        "City": "Taipei",
        "CommonName": "test.asrockrack.com",
        "Country": "TW",
        "Email": "test@asrockrack.com",
        "Organization": "ASRock Rack Inc.",
        "OrganizationalUnit": "Firmware",
        "State": "Taiwan"
    },
    "KeyUsage": [ null ],
    "Name": "Certificate Instance 1",
    "Subject": {
        "City": "Taipei",
        "CommonName": "test.asrockrack.com",
        "Country": "TW",
        "Email": "test@asrockrack.com",
        "Organization": "ASRock Rack Inc.",
        "OrganizationalUnit": "Firmware",
        "State": "Taiwan"
    },
    "ValidNotAfter": "2033-9-14T04:47:31+00:00",
    "ValidNotBefore": "2023-9-17T04:47:31+00:00"
}
```
Caveats: the dates are **not** zero-padded ISO 8601 (`2033-9-14T...`), so parsers must be lenient. ActionInfo URIs use a non-standard form, `.../Certificates/1/Certificate.RekeyActionInfo`.

---

### 3.7 Certificate (pp.57–61)

- **URLs:**
  - `/redfish/v1/AccountService/Accounts/{Account-Instance}/Certificates/{Certificates-Instance}`
  - `/redfish/v1/AccountService/LDAP/Certificates/{Certificates-Instance}`
  - `/redfish/v1/AccountService/LDAP/Certificates/Oem/Ami/ClientCertificates/{Certificates-Instance}`
  - `/redfish/v1/Managers/{Manager-Instance}/NetworkProtocol/HTTPS/Certificates/{Certificates-Instance}`
  - `/redfish/v1/Systems/{Systems-Instance}/Boot/Certificates/{Certificates-Instance}`
- **Methods/privileges:** GET: ConfigureManager. POST (actions): ConfigureManager. DELETE: ConfigureManager.

Supported properties (Table 10, all read-only):

| Property | Type | Description |
|---|---|---|
| Issuer | Object | Issuer (same sub-fields as Subject) |
| KeyUsage | Array | Enum: `CRLSigning`, `ClientAuthentication`, `CodeSigning`, `DataEncipherment`, `DecipherOnly`, `DigitalSignature`, `EmailProtection`, `EncipherOnly`, `KeyAgreement`, `KeyCertSign`, `KeyEncipherment`, `NonRepudiation`, `OCSPSigning`, `ServerAuthentication`, `Timestamping` |
| Subject | Object | Subject |
| Subject.City | String | City/locality |
| Subject.CommonName | String | FQDN |
| Subject.Country | String | Country |
| Subject.Email | String | Contact email |
| Subject.Organization | String | Organization |
| Subject.OrganizationalUnit | String | OU |
| Subject.State | String | State/province |
| ValidNotAfter | String | Expiry date |
| ValidNotBefore | String | Start date |
| CertificateString | String | Certificate string (private keys removed) |
| CertificateType | String | Format (PEM) |

#### GET (pp.58–59)

`GET https://{BMC_IP}/redfish/v1/AccountService/Accounts/{Account-Instance}/Certificates/{Certificates-Instance}`. The response is identical to the POST response above, except `"@odata.etag": "\"1694925101\""` and `"@odata.id": "/redfish/v1/AccountService/Accounts/1/Certificates/1"`.

#### POST Rekey (pp.59–60)

Generates a new key pair for an existing certificate from its data, and returns a CSR for a CA to sign.

`POST https://{BMC_IP}/redfish/v1/AccountService/Accounts/{Account-Instance}/Certificates/{Certificates-Instance}/Actions/Certificate.Rekey` (Content-Type: application/json)

Request (Table 11):

| Property | Type | Description |
|---|---|---|
| ChallengePassword | String | Challenge password for revocation requests |
| KeyBitLength | Number | Key length in bits. Default **2048** if omitted, **max 2048, min 512** |

Example (printed with unquoted keys and typographic quotes; valid JSON is shown here):
```json
{
    "KeyBitLength": 512,
    "ChallengePassword": "challengepassword"
}
```
Response **200**:
```json
{
    "CSRString": "-----BEGIN CERTIFICATE REQUEST-----\nMIIBezCCASUCAQAwgZ8xCzAJBgNVBAYTAlRXMQ8w...[elided]...\n-----END CERTIFICATE REQUEST-----\n",
    "Certificate": "/redfish/v1/AccountService/Accounts/1/Certificates/1"
}
```
Response properties (Table 12): `CSRString` (String, RO) is the CSR. `Certificate` (Object, RO) is a link to the rekeyed certificate. Note: the example returns `Certificate` as a plain string, not as an `{"@odata.id": ...}` object as DMTF requires.

#### POST Renew (pp.60–61)

Generates a CSR from the existing info and key pair. The service keeps the private key, and the response does not include it.

`POST https://{BMC_IP}/redfish/v1/AccountService/Accounts/{Account-Instance}/Certificates/{Certificates-Instance}/Actions/Certificate.Renew` (Content-Type: application/json)

Request (Table 13): `ChallengePassword` (String), the challenge password for revocation requests. Continued on p.61 (below).

Renew example request (p.61):
```json
{ "ChallengePassword": "renewchallenge" }
```
Response **200** (p.61):
```json
{
    "CSRString": "-----BEGIN CERTIFICATE REQUEST-----\nMIIBejCCASQCAQAwgZ8xCzAJBgNVBAYTAlRXMQ8w...[elided]...\n-----END CERTIFICATE REQUEST-----\n",
    "Certificate": "/redfish/v1/AccountService/Accounts/1/Certificates/1"
}
```
Response properties (Table 14): `CSRString` (String, RO); `Certificate` (Object, RO), described as "The link to the certificate being rekeyed" (sic). As with Rekey, it is returned as a plain string.

#### DELETE certificate (p.61)

`DELETE https://{BMC_IP}/redfish/v1/AccountService/Accounts/{Account-Instance}/Certificates/{Certificates-Instance}`. Response: **204**, no body.

---

### 3.8 ExternalAccountProviderCollection (p.62)

- **URL:** `/redfish/v1/AccountService/ExternalAccountProviders`
- **Methods/privileges:** GET: Login.
- Properties: common only. The guide gives no example body; it refers to section 3.1. Expected member: `/redfish/v1/AccountService/ExternalAccountProviders/RADIUS`.

---

### 3.9 RADIUS Settings (pp.63–65)

The RADIUS server is configured as an ExternalAccountProvider.

- **URL:** `/redfish/v1/AccountService/ExternalAccountProviders/RADIUS`
- **Methods/privileges:** GET: Login. PATCH: ConfigureUsers.

Supported properties (Table 15):

| Property | Type | Read Only | Description |
|---|---|---|---|
| ServiceEnabled | Boolean | False | Service enabled |
| AccountProviderTypes | String | True | Type of external provider: `RedfishService` (DMTF Redfish-conformant), `ActiveDirectoryService` (Microsoft AD), `LDAPService` (RFC4511), `OEM`. The example shows singular `AccountProviderType: "OEM"` |
| AMI (`Oem.Ami`) | Object | True | AMI features |
| ...AdvancedRadiusSetting | Object | True | Link to AdvancedRadiusSetting (example key: `AdvancedRADIUSSettings`) |
| ...ServiceAddress | String | False | RADIUS server IPv4/IPv6 address |
| ...ServicePort | String | False | RADIUS port. Typed String, but the examples use a number (1812) |
| ...Secret | String | False | Shared secret (returned as null) |
| ...Timeout | Integer | True | Default timeout (s) for RADIUS auth |
| ...ExtendedPrivilege | Object | False | Extended privileges for RADIUS |
| ......KVMAccess | Boolean | False | KVM access |
| ......VMediaAccess | Boolean | False | VMedia access |

#### GET when not configured (p.64)

```json
{
    "@odata.context": "/redfish/v1/$metadata#ExternalAccountProvider.ExternalAccountProvider",
    "@odata.etag": "\"1694237498\"",
    "@odata.id": "/redfish/v1/AccountService/ExternalAccountProviders/RADIUS",
    "@odata.type": "#ExternalAccountProvider.v1_1_2.ExternalAccountProvider",
    "AccountProviderType": "OEM",
    "Description": "RADIUS server settings",
    "Id": "RADIUS Server",
    "Name": "RADIUS Settings",
    "Oem": {
        "Ami": {
            "@odata.type": "#AMIExternalAccountProvider.v1_0_0.AMIExternalAccountProvider",
            "ExtendedPrivilege": {
                "KVMAccess": false,
                "VMediaAccess": false
            },
            "Secret": null,
            "ServiceAddress": null,
            "ServicePort": 1812,
            "Timeout": 3
        }
    },
    "ServiceEnabled": false
}
```
Note: `Id` is `"RADIUS Server"` (it contains a space), which does not match the URI segment `RADIUS`.

#### GET when configured (pp.64–65)

```json
{
    "@odata.context": "/redfish/v1/$metadata#ExternalAccountProvider.ExternalAccountProvider",
    "@odata.etag": "\"1694241254\"",
    "@odata.id": "/redfish/v1/AccountService/ExternalAccountProviders/RADIUS",
    "@odata.type": "#ExternalAccountProvider.v1_1_2.ExternalAccountProvider",
    "AccountProviderType": "OEM",
    "Description": "RADIUS server settings",
    "Id": "RADIUS Server",
    "Name": "RADIUS Settings",
    "Oem": {
        "Ami": {
            "@odata.type": "#AMIExternalAccountProvider.v1_0_0.AMIExternalAccountProvider",
            "AdvancedRADIUSSettings": {
                "@odata.id": "/redfish/v1/AccountService/ExternalAccountProviders/RADIUS/Oem/Ami/AdvancedRADIUSSetting"
            },
            "ExtendedPrivilege": {
                "KVMAccess": false,
                "VMediaAccess": false
            },
            "Secret": null,
            "ServiceAddress": "192.168.150.2",
            "ServicePort": 1812,
            "Timeout": 3
        }
    },
    "ServiceEnabled": true
}
```
The AdvancedRADIUSSettings link appears only once RADIUS is configured.

#### PATCH (p.65)

```json
{
    "Oem": {
        "Ami": {
            "ExtendedPrivilege": {
                "KVMAccess": false,
                "VMediaAccess": false
            },
            "Secret": "testkey123",
            "ServiceAddress": "192.168.150.2",
            "ServicePort": 1812
        }
    },
    "ServiceEnabled": true
}
```
Response: **204**, no body.

---

### 3.10 Advanced RADIUS Setting (pp.66–67)

- **URL:** `/redfish/v1/AccountService/ExternalAccountProviders/RADIUS/Oem/Ami/AdvancedRADIUSSetting`
- **Methods/privileges:** GET: Login. PATCH: ConfigureUsers.

Supported properties (Table 16):

| Property | Type | Read Only | Description |
|---|---|---|---|
| RADIUSAuthorization | Object | False | Privilege levels and associated values (vendor-specific attribute strings) for RADIUS users |
| ...Administrator | String | False | Name for the Administrator privilege on the RADIUS server and BMC |
| ...Operator | String | False | Operator privilege |
| ...User | String | False | User privilege |
| ...Oem | String | False | OEM privilege |
| ...NoAccess | String | False | No-access privilege |

#### GET (pp.66–67)

```json
{
    "@odata.context": "/redfish/v1/$metadata#AMIAdvanceRADIUSSetting.AMIAdvanceRADIUSSetting",
    "@odata.etag": "\"1694241254\"",
    "@odata.id": "/redfish/v1/AccountService/ExternalAccountProviders/RADIUS/Oem/Ami/AdvancedRADIUSSetting",
    "@odata.type": "#AMIAdvanceRADIUSSetting.v1_0_0.AMIAdvanceRADIUSSetting",
    "RADIUSAuthorization": {
        "Administrator": "H=4",
        "NoAccess": "H=0",
        "Oem": "H=1",
        "Operator": "H=3",
        "User": "H=2"
    }
}
```

#### PATCH (p.67)

```json
{
    "RADIUSAuthorization": {
        "Administrator": "H=4",
        "NoAccess": "H=0",
        "Oem": "H=1",
        "Operator": "H=3",
        "User": "H=2"
    }
}
```
Response: **204**, no body.

---

### 3.11 PAM Configuration (pp.68–69)

PAM (authentication-module order) settings for the BMC. OEM resource.

- **URL:** `/redfish/v1/AccountService/Oem/Ami/Configurations`
- **Methods/privileges:** GET: Login. PATCH: ConfigureComponents.

Supported properties (Table 17):

| Property | Type | Read Only | Description |
|---|---|---|---|
| Id | String | True | Id |
| Name | String | True | Name |
| PAMEnabled | Boolean | False | Use PAM authentication for Redfish requests |
| PAMOrder | Array | False | Order in which PAM modules are checked. The table lists `ACTIVE`, `DIRECTORY`, `IPMI`, `LDAP`, `RADIUS`; the examples use the single string `"ACTIVE DIRECTORY"`, so the table's ACTIVE/DIRECTORY split is a line-wrap artefact |

#### GET (pp.68–69)

```json
{
    "@odata.context": "/redfish/v1/$metadata#AMIAccountServiceConfigurations.AMIAccountServiceConfigurations",
    "@odata.etag": "\"1694234439\"",
    "@odata.id": "/redfish/v1/AccountService/Oem/Ami/Configurations",
    "@odata.type": "#AMIAccountServiceConfigurations.v1_0_0.AMIAccountServiceConfigurations",
    "Id": "Configurations",
    "Name": "AccountService Configurations",
    "PAMEnabled": false,
    "PAMOrder": [
        "IPMI",
        "LDAP",
        "ACTIVE DIRECTORY",
        "RADIUS"
    ]
}
```

#### PATCH (p.69)

```json
{
    "PAMEnabled": true,
    "PAMOrder": [
        "LDAP",
        "IPMI",
        "ACTIVE DIRECTORY",
        "RADIUS"
    ]
}
```
Response: **204**, no body.

---

### 3.12 RoleCollection (pp.70–72)

- **URL:** `/redfish/v1/AccountService/Roles`
- **Methods/privileges:** GET: Login. POST: ConfigureManager.

#### GET (p.70)

```json
{
    "@odata.context": "/redfish/v1/$metadata#RoleCollection.RoleCollection",
    "@odata.etag": "\"1694246474\"",
    "@odata.id": "/redfish/v1/AccountService/Roles",
    "@odata.type": "#RoleCollection.RoleCollection",
    "Description": "Details of Roles Available",
    "Members": [
        { "@odata.id": "/redfish/v1/AccountService/Roles/Administrator" },
        { "@odata.id": "/redfish/v1/AccountService/Roles/HostInterfaceAdministrator" },
        { "@odata.id": "/redfish/v1/AccountService/Roles/Operator" },
        { "@odata.id": "/redfish/v1/AccountService/Roles/ReadOnly" }
    ],
    "Members@odata.count": 4,
    "Name": "Roles Collection"
}
```
Note: there is an extra predefined role, `HostInterfaceAdministrator`, which is not in Table 2.

#### POST: create custom role (pp.71–72)

`RoleId` and `Name` are required, plus `AssignedPrivileges`, `OemPrivileges`, or both.

```json
{
    "AssignedPrivileges": [
        "Login",
        "ConfigureManager",
        "ConfigureUsers",
        "ConfigureSelf",
        "ConfigureComponents"
    ],
    "Description": "TestRole User Role",
    "Id": "TestRole",
    "RoleId": "TestRole",
    "Name": "TestRole Role",
    "OemPrivileges": [
        "OemPowerControl",
        "OemClearLog"
    ]
}
```
Response **201**, with the GET body of the new role:
```json
{
    "@odata.context": "/redfish/v1/$metadata#Role.Role",
    "@odata.etag": "\"1694250830\"",
    "@odata.id": "/redfish/v1/AccountService/Roles/TestRole",
    "@odata.type": "#Role.v1_2_4.Role",
    "AssignedPrivileges": [
        "Login",
        "ConfigureManager",
        "ConfigureUsers",
        "ConfigureSelf",
        "ConfigureComponents"
    ],
    "Description": "TestRole User Role",
    "Id": "TestRole",
    "IsPredefined": false,
    "Name": "TestRole Role",
    "OemPrivileges": [
        "OemPowerControl",
        "OemClearLog"
    ],
    "RoleId": "TestRole"
}
```
OEM privileges seen: `OemPowerControl`, `OemClearLog`, and `ConfigureHostInterface` (from Table 18). The guide does not list the full set of OEM privilege names.

---

### 3.13 Role (pp.73–74)

- **URL:** `/redfish/v1/AccountService/Roles/{Role-Instance}`
- **Methods/privileges:** GET: Login. PATCH: ConfigureManager. DELETE: ConfigureManager.

Supported properties (Table 18):

| Property | Type | Read Only | Description |
|---|---|---|---|
| IsPredefined | Boolean | True | Predefined (true) or custom role |
| AssignedPrivileges | Array | False | Redfish privileges: `Login` (log in and read resources), `ConfigureManager`, `ConfigureUsers`, `ConfigureSelf` (change own password), `ConfigureComponents` |
| OemPrivileges | Array | False | OEM privileges; listed: `ConfigureHostInterface` (configure HostInterface resources) |
| Actions | Object | True | Actions under Oem |
| RoleId | String | True | Role name, same as Id |

#### GET (pp.73–74)

```json
{
    "@odata.context": "/redfish/v1/$metadata#Role.Role",
    "@odata.etag": "\"1694246474\"",
    "@odata.id": "/redfish/v1/AccountService/Roles/Administrator",
    "@odata.type": "#Role.v1_2_4.Role",
    "AssignedPrivileges": [
        "Login",
        "ConfigureManager",
        "ConfigureUsers",
        "ConfigureSelf",
        "ConfigureComponents"
    ],
    "Description": "Admin User Role",
    "Id": "Administrator",
    "IsPredefined": true,
    "Name": "User Role",
    "RoleId": "Administrator"
}
```

#### PATCH (p.74)

```json
{ "AssignedPrivileges": [ "Login", "ConfigureSelf" ] }
```
Response: **204** (the guide adds "and the updated Role", which contradicts 204 having no body).

#### DELETE (p.74)

Response: **204**, no body. (Presumably only custom roles can be deleted; the guide does not say.)

---

### 3.14 CertificateService (pp.75–78)

- **URL:** `/redfish/v1/CertificateService`
- **Methods/privileges:** GET: Login. POST (actions): ConfigureManager.

Supported properties (Table 19): `CertificateLocations` (Object, RO), a link to the resource that locates all installed certificates.

#### GET (pp.75–76)

```json
{
    "@odata.context": "/redfish/v1/$metadata#CertificateService.CertificateService",
    "@odata.etag": "\"1694252367\"",
    "@odata.id": "/redfish/v1/CertificateService",
    "@odata.type": "#CertificateService.v1_0_2.CertificateService",
    "Actions": {
        "#CertificateService.GenerateCSR": {
            "@Redfish.ActionInfo": "/redfish/v1/CertificateService/CertificateService.GenerateCSRActionInfo",
            "target": "/redfish/v1/CertificateService/Actions/CertificateService.GenerateCSR"
        },
        "#CertificateService.ReplaceCertificate": {
            "@Redfish.ActionInfo": "/redfish/v1/CertificateService/CertificateService.ReplaceCertificateActionInfo",
            "target": "/redfish/v1/CertificateService/Actions/CertificateService.ReplaceCertificate"
        }
    },
    "CertificateLocations": {
        "@odata.id": "/redfish/v1/CertificateService/CertificateLocations"
    },
    "Description": "Get the available to manage certificates and link to certificates",
    "Id": "CertificateService",
    "Name": "Certificate Service"
}
```

#### POST Generate CSR (pp.76–77)

`POST https://{BMC_IP}/redfish/v1/CertificateService/Actions/CertificateService.GenerateCSR` (Content-Type: application/json)

Request (Table 20):

| Name | Type | Description |
|---|---|---|
| AlternativeNames | Array | Additional host names (SANs) |
| CertificateCollection | Object | Target certificate collection link. The table description is a copy-paste error; the example uses `{"@odata.id": ".../HTTPS/Certificates"}` |
| ChallengePassword | String | Challenge password for revocation |
| City | String | City/locality |
| CommonName | String | FQDN of the component |
| ContactPerson | String | Requesting user name |
| Country | String | Two-letter country code |
| Email | String | Contact email |
| GivenName | String | Given name |
| Initials | String | Initials |
| KeyBitLength | String (as printed; the example uses a number) | Key length. Default **2048**, **max 2048**, **min 512** |
| KeyUsage | Array | Key usages |
| Organization | String | Organization |
| OrganizationalUnit | String | OU |
| State | String | State/province |
| Surname | String | Surname |
| UnstructuredName | String | Unstructured name |

Example:
```json
{
    "Country": "TW",
    "State": "Taiwan",
    "City": "Taipei",
    "Organization": "ASRock Rack",
    "OrganizationalUnit": "Firmware",
    "CommonName": "test.asrockrack.com",
    "AlternativeNames": [ "test1.asrockrack.com", "test2.asrockrack.com", "192.168.149.211" ],
    "Email": "test1@asrockrack.com",
    "KeyBitLength": 2048,
    "KeyUsage": [ "DigitalSignature" ],
    "ChallengePassword": "challengepassword",
    "GivenName": "userGivenName",
    "ContactPerson": "Test User",
    "Initials": "userInitials",
    "Surname": "userSurname",
    "UnstructuredName": "userUnstructuredName",
    "CertificateCollection": { "@odata.id": "/redfish/v1/Managers/Self/NetworkProtocol/HTTPS/Certificates" }
}
```
Response properties (Table 21): `CSRString` (String, RO); `CertificateLocations` (Object, RO). The example actually returns `CertificateCollection`.

Response **200**:
```json
{
    "CSRString": "-----BEGIN CERTIFICATE REQUEST-----\nMIID6DCCAtACAQAwgfQxCzAJBgNVBAYTAlRXMQ8w...[elided]...\n-----END  CERTIFICATE REQUEST-----\n",
    "CertificateCollection": {
        "@odata.id": "/redfish/v1/Managers/Self/NetworkProtocol/HTTPS/Certificates"
    }
}
```
Note: the manager instance ID is **`Self`** (`/redfish/v1/Managers/Self`).

#### POST Replace Certificate (pp.77–78)

`POST https://{BMC_IP}/redfish/v1/CertificateService/Actions/CertificateService.ReplaceCertificate` (Content-Type: application/json)

Example (certificate and private key concatenated in one string, base64 elided):
```json
{
    "CertificateString": "-----BEGIN CERTIFICATE-----\nMIIEITCCAwmgAwIBAgIUULresH8I7jFkvyjzisKWH3Jm8pswDQYJKoZIhvcNAQEN...[elided]...\n-----END CERTIFICATE-----\n-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDRTGyOi...[elided]...\n-----END PRIVATE KEY-----",
    "CertificateType": "PEM",
    "CertificateUri": { "@odata.id": "/redfish/v1/Managers/Self/NetworkProtocol/HTTPS/Certificates/1" }
}
```
Response: **204**, no body. This is how the BMC HTTPS/TLS certificate is replaced. (DMTF specifies a response body for ReplaceCertificate; this implementation returns 204.)

---

### 3.15 CertificateLocations (pp.79–80)

- **URL:** `/redfish/v1/CertificateService/CertificateLocations`
- **Methods/privileges:** GET: Login.

Supported properties (Table 22):

| Property | Type | Read Only | Description |
|---|---|---|---|
| Links | Object | True | Related resources |
| Links.Certificates | Array | True | Links to installed certificates |
| Links.Certificates@odata.count | Number | True | Count |
| Links.Oem (Ami) | Object | True | OEM |
| ...PrimarySMTP | Object | True | SMTP StartTLS certificates (primary) |
| ......CACert | String | False | CA certificate for SMTP |
| ......Cert | String | False | Client certificate for SMTP |
| ......PrivateKey | String | False | Private key for SMTP |
| ...SecondarySMTP | Object | True | Secondary SMTP certificates |
| ......CACert / Cert / PrivateKey | String | False | As for PrimarySMTP |

#### GET (pp.79–80)

```json
{
    "@odata.context": "/redfish/v1/$metadata#CertificateLocations.CertificateLocations",
    "@odata.etag": "\"1694266264\"",
    "@odata.id": "/redfish/v1/CertificateService/CertificateLocations",
    "@odata.type": "#CertificateLocations.v1_0_2.CertificateLocations",
    "Description": "Certificate Locations",
    "Id": "CertificateLocations",
    "Links": {
        "Certificates": [
            { "@odata.id": "/redfish/v1/Managers/Self/NetworkProtocol/HTTPS/Certificates/1" }
        ],
        "Certificates@odata.count": 1,
        "Oem": {
            "Ami": {
                "PrimarySMTP": [],
                "SecondarySMTP": []
            }
        }
    },
    "Name": "Certificate Locations"
}
```
Note: PrimarySMTP and SecondarySMTP are documented as Objects but returned as empty arrays.

---

### 3.16 ChassisCollection (p.81)

- **URL:** `/redfish/v1/Chassis`
- **Methods/privileges:** GET: Login.

```json
{
    "@odata.context": "/redfish/v1/$metadata#ChassisCollection.ChassisCollection",
    "@odata.etag": "\"1694548802\"",
    "@odata.id": "/redfish/v1/Chassis",
    "@odata.type": "#ChassisCollection.ChassisCollection",
    "Description": "The Collection for Chassis",
    "Members": [
        { "@odata.id": "/redfish/v1/Chassis/Self" }
    ],
    "Members@odata.count": 1,
    "Name": "Chassis Collection"
}
```
Note: the single chassis ID is **`Self`**. Likewise the system is `/redfish/v1/Systems/Self` and the manager is `/redfish/v1/Managers/Self`.

---

### 3.17 Chassis (pp.82–87)

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}`
- **Methods/privileges:** GET: Login. PATCH: ConfigureComponents. POST (Chassis.Reset): ConfigureComponents.

Supported properties (Table 23):

| Property | Type | Read Only | Description |
|---|---|---|---|
| ChassisType | String | True | Physical form factor; see Table 24 |
| Manufacturer | String | True | Manufacturer |
| Model | String | True | Model |
| SKU | String | True | SKU |
| Sensors | Object | True | Link to Sensor collection |
| SerialNumber | String | True | Serial |
| PartNumber | String | True | Part number |
| PCIeDevices | Object | True | Link to PCIe devices collection |
| PCIeSlots | Object | True | Link to PCIeSlots |
| AssetTag | String | **False** | User-assigned asset tag |
| Assembly | Object | True | Link to Assembly |
| UUID | String | True | UUID |
| IndicatorLED | String | **False** | `Lit`, `Blinking`, `Off` |
| Links | Object | True | See Table 25 |
| LogServices | Object | True | Link to LogServices |
| Thermal | Object | True | Link to Thermal |
| Power | Object | True | Link to Power |
| PowerState | String | True | `On`; `Off` (components have no power except AUX such as the management controller) |
| Location | Object | True | See Table 5 |
| HeightMm / WidthMm / DepthMm / WeightKg | Number | True | Dimensions |
| NetworkAdapters | Object | True | Link to NetworkAdapters |
| MediaControllers | Object | True | Link to MediaControllers |
| EnvironmentalClass | String | True | ASHRAE class `A1`, `A2`, `A3`, `A4` |

Chassis Type enum (Table 24): `Rack`, `Blade`, `Enclosure`, `StandAlone`, `RackMount`, `Card`, `Cartridge`, `Row`, `Pod`, `Expansion`, `Sidecar`, `Zone`, `Sled`, `Shelf`, `Drawer`, `Module`, `Component`, `Other`, `StorageEnclosure` (descriptions follow DMTF).

Chassis Links (Table 25, all RO): `ComputerSystems` (Array) and `@odata.count`, `ManagedBy` and count, `Drives` and count, `Storage` and count, `ResourceBlocks` and count, `ContainedBy` (Array, as printed), `Contains` and count, `PoweredBy` and count, `CooledBy` and count, `ManagersInChassis` and count, `Processors` and count.

#### GET (pp.84–86)

```json
{
    "@odata.context": "/redfish/v1/$metadata#Chassis.Chassis",
    "@odata.etag": "\"1694985188\"",
    "@odata.id": "/redfish/v1/Chassis/Self",
    "@odata.type": "#Chassis.v1_14_0.Chassis",
    "Actions": {
        "#Chassis.Reset": {
            "@Redfish.ActionInfo": "/redfish/v1/Chassis/Self/ResetActionInfo",
            "@Redfish.OperationApplyTimeSupport": {
                "@odata.type": "#Settings.v1_2_2.OperationApplyTimeSupport",
                "MaintenanceWindowDurationInSeconds": 600,
                "MaintenanceWindowResource": { "@odata.id": "/redfish/v1/Chassis/Self" },
                "SupportedValues": [ "Immediate", "AtMaintenanceWindowStart" ]
            },
            "target": "/redfish/v1/Chassis/Self/Actions/Chassis.Reset"
        }
    },
    "AssetTag": "To Be Filled By O.E.M.",
    "ChassisType": "Other",
    "Description": "Chassis Self",
    "Id": "Self",
    "IndicatorLED": "Off",
    "IndicatorLED@Redfish.AllowableValues": [ "Lit", "Blinking", "Off" ],
    "Links": {
        "ComputerSystems": [ { "@odata.id": "/redfish/v1/Systems/Self" } ],
        "ComputerSystems@odata.count": 1,
        "ManagedBy": [ { "@odata.id": "/redfish/v1/Managers/Self" } ],
        "ManagedBy@odata.count": 1
    },
    "Location": {
        "PartLocation": [],
        "Placement": [],
        "PostalAddress": []
    },
    "LogServices": { "@odata.id": "/redfish/v1/Chassis/Self/LogServices" },
    "Manufacturer": "To Be Filled By O.E.M.",
    "MediaControllers": { "@odata.id": "/redfish/v1/Chassis/Self/MediaControllers" },
    "Model": " ",
    "Name": "Computer System Chassis",
    "NetworkAdapters": { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters" },
    "PCIeDevices": { "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices" },
    "PCIeSlots": { "@odata.id": "/redfish/v1/Chassis/Self/PCIeSlots" },
    "PartNumber": " ",
    "PhysicalSecurity": [],
    "Power": { "@odata.id": "/redfish/v1/Chassis/Self/Power" },
    "PowerState": "On",
    "SKU": "To Be Filled By O.E.M.",
    "Sensors": { "@odata.id": "/redfish/v1/Chassis/Self/Sensors" },
    "SerialNumber": "To Be Filled By O.E.M.",
    "Status": { "Health": "OK", "HealthRollup": "OK", "State": "Enabled" },
    "Thermal": { "@odata.id": "/redfish/v1/Chassis/Self/Thermal" }
}
```
Caveats: object-typed properties (`Location.PartLocation`, `Placement`, `PostalAddress`, `PhysicalSecurity`) are returned as **empty arrays `[]`** when unset, which violates the schema. Strict Go struct unmarshalling into structs will fail, so use `json.RawMessage` or lenient types. FRU strings are SMBIOS-style placeholders ("To Be Filled By O.E.M.", and `" "` with a single space). Chassis.Reset supports `@Redfish.OperationApplyTime` with `Immediate` or `AtMaintenanceWindowStart`, and a 600 s maintenance window.

#### PATCH (p.86)

`PATCH https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}` (printed "PATHC")
```json
{
    "AssetTag": "ABCDEFG",
    "IndicatorLED": "Lit"
}
```
Response: **204**, no body.

#### POST Chassis.Reset (pp.86–87)

`POST https://{BMC_IP}/redfish/v1/Chassis/Self/Actions/Chassis.Reset`

ResetType: `On`, `ForceOff`, `GracefulShutdown`, `ForceRestart` (printed "ForceRestar"), `PowerCycle`.
```json
{ "ResetType": "On" }
```
Response **202**, with a Task monitor body (asynchronous):
```json
{
    "@odata.context": "/redfish/v1/$metadata#Task.Task",
    "@odata.id": "/redfish/v1/TaskService/Tasks/1",
    "@odata.type": "#Task.v1_4_2.Task",
    "Description": "Task for Chassis Reset",
    "Id": "1",
    "Name": "Chassis Reset",
    "TaskState": "New"
}
```
Note: Chassis.Reset is asynchronous and returns a Task (poll `/redfish/v1/TaskService/Tasks/{id}`). Allowable values are also discoverable at `/redfish/v1/Chassis/Self/ResetActionInfo`.

---

### 3.18 LogServiceCollection (p.88)

- **URLs:** `/redfish/v1/Chassis/{Chassis-Id}/LogServices`, `/redfish/v1/Managers/{Manager-Instance}/LogServices`, `/redfish/v1/Systems/{Systems-Instance}/LogServices`
- **Methods/privileges:** GET: Login.

```json
{
    "@odata.context": "/redfish/v1/$metadata#LogServiceCollection.LogServiceCollection",
    "@odata.etag": "\"1695296170\"",
    "@odata.id": "/redfish/v1/Chassis/Self/LogServices",
    "@odata.type": "#LogServiceCollection.LogServiceCollection",
    "Description": "Collection of Log Services for this Chassis",
    "Members": [
        { "@odata.id": "/redfish/v1/Chassis/Self/LogServices/Logs" }
    ],
    "Members@odata.count": 1,
    "Name": "Log Service Collection"
}
```

---

### 3.19 LogService (pp.89–91)

- **URLs:** `/redfish/v1/Chassis/{Chassis-Id}/LogServices/{LogServices-Instance}`, `/redfish/v1/Managers/{Manager-Instance}/LogServices/{LogServices-Instance}`, `/redfish/v1/Systems/{Systems-Instance}/LogServices/{LogServices-Instance}`, `/redfish/v1/TelemetryService/LogService`
- **Methods/privileges:** GET: Login. POST (ClearLog): ConfigureManager.

Supported properties (Table 26):

| Property | Type | Read Only | Description |
|---|---|---|---|
| ServiceEnabled | Boolean | True | Service enabled (default true) |
| MaxNumberOfRecords | Number | True | Max entries |
| OverWritePolicy | String | True | `WrapsWhenFull` (new entries overwrite old) |
| DateTime | String | False | Current log-service DateTime (Redfish timestamp) |
| DateTimeLocalOffset | String | False | UTC offset, format `+06:00` |
| Actions | Object | True | `LogService.ClearLog` and OEM actions |
| Entries | Object | True | Link to LogEntry collection |
| LogEntryType | String | True | `Event` (Redfish messages), `SEL` (IPMI SEL), `Multiple`, `OEM` |

#### GET (pp.89–90)

```json
{
    "@odata.context": "/redfish/v1/$metadata#LogService.LogService",
    "@odata.etag": "\"1695296170\"",
    "@odata.id": "/redfish/v1/Chassis/Self/LogServices/Logs",
    "@odata.type": "#LogService.v1_1_3.LogService",
    "Actions": {
        "#LogService.ClearLog": {
            "target": "/redfish/v1/Chassis/Self/LogServices/Logs/Actions/LogService.ClearLog"
        }
    },
    "DateTime": "2023-09-30T06:37:40+00:00",
    "DateTimeLocalOffset": "+00:00",
    "Description": "IPMI SEL events for this manager",
    "Entries": { "@odata.id": "/redfish/v1/Chassis/Self/LogServices/Logs/Entries" },
    "Id": "Logs",
    "LogEntryType": "SEL",
    "MaxNumberOfRecords": 150,
    "Name": "IPMI Chassis Logs",
    "OverWritePolicy": "WrapsWhenFull",
    "ServiceEnabled": true,
    "Status": { "Health": "OK", "State": "Enabled" }
}
```
Note: the chassis log is the IPMI SEL (max 150 records in the example, wraps when full).

#### POST ClearLog (pp.90–91)

`POST https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/LogServices/{LogServices-Instance}/Actions/LogService.ClearLog` (or the same under `/Managers/{Manager-Id}/...`)

Body: `{}` (an empty object).

Response **202** with a Task:
```json
{
    "@odata.context": "/redfish/v1/$metadata#Task.Task",
    "@odata.id": "/redfish/v1/TaskService/Tasks/3",
    "@odata.type": "#Task.v1_4_2.Task",
    "Description": "Task for Chassis LogService",
    "Id": "3",
    "Name": "Chassis LogService",
    "TaskState": "New"
}
```

---

### 3.20 LogEntryCollection (pp.92–93)

- **URLs:** `.../Chassis/{Chassis-Id}/LogServices/{LogServices-Instance}/Entries`, `.../Managers/{Manager-Id}/LogServices/{LogServices-Instance}/Entries`, `.../Systems/{Systems-Instance}/LogServices/{LogServices-Instance}/Entries`, `/redfish/v1/TelemetryService/LogService/Entries`
- **Methods/privileges:** GET: Login.

```json
{
    "@odata.context": "/redfish/v1/$metadata#LogEntryCollection.LogEntryCollection",
    "@odata.etag": "\"1696064015\"",
    "@odata.id": "/redfish/v1/Chassis/Self/LogServices/Logs/Entries",
    "@odata.type": "#LogEntryCollection.LogEntryCollection",
    "Description": "Collection of entries for this log service",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Chassis/Self/LogServices/Logs/Entries/1",
            "@odata.type": "#LogEntry.v1_4_3.LogEntry",
            "Created": "2023-09-30T08:53:33+00:00",
            "Description": "LOG 3",
            "EntryCode": "Lower Non-critical - going low",
            "EntryType": "SEL",
            "EventTimestamp": "2023-09-30T08:53:33+00:00",
            "Id": "1",
            "Links": {
                "OriginOfCondition": { "@odata.id": "/redfish/v1/Chassis/Self/Thermal" }
            },
            "Message": "Event_Data_1 : 80, Record_Type : system event record, Sensor_Number : 17, Event_Dir : Assertion event, Event_Data_2 : 0, Timestamp : 2023-09-30T08:53:33+00:00, EvM_Rev : IPMI v2.0 Event Messages, Generator_ID : 0x2000, Event_Data_3 : 1, Event_Type : Lower Non-critical - going low, Record_ID : 3, Sensor_Type : Fan, ",
            "MessageId": "0x500001",
            "Name": "LOG 3",
            "SensorNumber": 17,
            "SensorType": "Fan",
            "Severity": "Warning"
        },
        {
            "@odata.id": "/redfish/v1/Chassis/Self/LogServices/Logs/Entries/2",
            "@odata.type": "#LogEntry.v1_4_3.LogEntry",
            "Created": "2023-09-30T08:53:35+00:00",
            "Description": "LOG 4",
            "EntryCode": "Lower Non-critical - going low",
            "EntryType": "SEL",
            "EventTimestamp": "2023-09-30T08:53:35+00:00",
            "Id": "2",
            "Links": {
                "OriginOfCondition": { "@odata.id": "/redfish/v1/Chassis/Self/Thermal" }
            },
            "Message": "Event_Data_1 : 80, Record_Type : system event record, Sensor_Number : 17, Event_Dir : Deassertion event, Event_Data_2 : 6, Timestamp : 2023-09-30T08:53:35+00:00, EvM_Rev : IPMI v2.0 Event Messages, Generator_ID : 0x2000, Event_Data_3 : 1, Event_Type : Lower Non-critical - going low, Record_ID : 4, Sensor_Type : Fan, ",
            "MessageId": "0x500601",
            "Name": "LOG 4",
            "SensorNumber": 17,
            "SensorType": "Fan",
            "Severity": "Warning"
        }
    ],
    "Members@odata.count": 2,
    "Name": "Log Service Entries Collection"
}
```
Notes: collection members are **expanded inline** (full LogEntry objects, not just `@odata.id` links). The SEL `Message` is a flattened "key : value, " string, and `MessageId` is a hex string encoding the SEL event data (not a registry MessageId). The redfish entry Id ("1") differs from the SEL Record_ID (3). In the second entry, Event_Dir is "Deassertion event" while EntryCode still says "going low".

---

### 3.21 LogEntry (pp.94–98)

- **URLs:**
  - `/redfish/v1/Chassis/{Chassis-Id}/LogServices/{LogServices-Instance}/Entries/{Chassis-LogEntry-Instance}`
  - `/redfish/v1/Managers/{Manager-Id}/LogServices/{LogServices-Instance}/Entries/{Manager-LogEntry-Instance}`
  - `/redfish/v1/Systems/{Systems-Instance}/LogServices/{LogServices-Instance}/Entries/{System-LogEntry-Instance}`
  - `/redfish/v1/TelemetryService/LogService/Entries/{Entry-Instance}`
- **Methods/privileges:** GET: Login.

Supported properties (Table 27, all RO):

| Property | Type | Description |
|---|---|---|
| Severity | String | `OK`, `Warning`, `Critical` |
| Created | String | Entry creation time |
| EventId | String | Unique event id (implementation-dependent), if this records an Event |
| EventTimestamp | String | Time the event occurred |
| EntryType | String | `Event`, `SEL`, `Oem` |
| EntryCode | String | Present if EntryType=SEL; values from IPMI tables 42-1 and 42-2 (list below) |
| SensorType | String | Present if EntryType=SEL (list below) |
| SensorNumber | Number | SEL: sensor number; Event: n/a; otherwise OEM-specific |
| Message | String | Event: message text; SEL: SEL-specific message; otherwise OEM entry |
| MessageId | String | Event: Redfish registry MessageId; SEL: event data; otherwise OEM-specific |
| MessageArgs | Array | Message substitution arguments |
| OemLogEntryCode | String | OEM entry code if EntryCode type is OEM |
| OemSensorType | String | OEM sensor type if SensorType is OEM |

**EntryCode enum (pp.94–96):** `Assert`, `Deassert`, `Lower Non-critical - going low`, `Lower Non-critical - going high`, `Lower Critical - going low`, `Lower Critical - going high`, `Lower Non-recoverable - going low`, `Lower Non-recoverable - going high`, `Upper Non-critical - going low`, `Upper Non-critical - going high`, `Upper Critical - going low`, `Upper Critical - going high`, `Upper Non-recoverable - going low`, `Upper Non-recoverable - going high`, `Transition to Idle`, `Transition to Active`, `Transition to Busy`, `State Deasserted`, `State Asserted`, `Predictive Failure deasserted`, `Predictive Failure asserted`, `Limit Not Exceeded`, `Limit Exceeded`, `Performance Met`, `Performance Lags`, `Transition to OK`, `Transition to Non-Critical from OK`, `Transition to Critical from less severe`, `Transition to Non-recoverable from less severe`, `Transition to Critical from Non-recoverable`, `Transition to Non-recoverable`, `Monitor`, `Informational`, `Device Removed / Device Absent`, `Device Inserted / Device Present`, `Device Disabled`, `Device Enabled`, `Transition to Running`, `Transition to In Test`, `Transition to Power Off`, `Transition to On Line`, `Transition to Off Line`, `Transition to Off Duty`, `Transition to Degraded`, `Transition to Power Save`, `Install Error`, `Fully Redundant`, `Redundancy Lost`, `Redundancy Degraded`, `Non-redundant:Sufficient Resources from Redundant`, `Non-redundant:Sufficient Resources from Insufficient Resources`, `Non-redundant:Insufficient Resources`, `Redundancy Degraded from Fully Redundant`, `Redundancy Degraded from Non-redundant`, `D0 Power State`, `D1 Power State`, `D2 Power State`, `D3 Power State`.

**SensorType enum (pp.96–97):** `Platform Security Violation Attempt`, `Temperature`, `Voltage`, `Current`, `Fan`, `Physical Chassis Security`, `Processor`, `Power Supply / Converter`, `PowerUnit`, `CoolingDevice`, `Other Units-based Sensor`, `Memory`, `Drive Slot/Bay`, `POST Memory Resize`, `System Firmware Progress`, `Event Logging Disabled`, `System Event`, `Critical Interrupt`, `Button / Switch`, `Module / Board`, `Microcontroller / Coprocessor`, `Add-in Card`, `Chassis`, `Chipset`, `Other FRU`, `Management Subsystem Health`, `Battery`, `Session Audit`, `Version Change`, `FRUState`, `OEM`.

#### GET (pp.97–98)

`GET https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/LogServices/Logs/Entries/{Chassis-LogEntry-Instance}`
```json
{
    "@odata.context": "/redfish/v1/$metadata#LogEntry.LogEntry",
    "@odata.etag": "\"1696064137\"",
    "@odata.id": "/redfish/v1/Chassis/Self/LogServices/Logs/Entries/1",
    "@odata.type": "#LogEntry.v1_4_3.LogEntry",
    "Created": "2023-09-30T08:53:33+00:00",
    "Description": "LOG 3",
    "EntryCode": "Lower Non-critical - going low",
    "EntryType": "SEL",
    "EventTimestamp": "2023-09-30T08:53:33+00:00",
    "Id": "1",
    "Links": {
        "OriginOfCondition": { "@odata.id": "/redfish/v1/Chassis/Self/Thermal" }
    },
    "Message": "Event_Data_1 : 80, Record_Type : system event record, Sensor_Number : 17, Event_Dir : Assertion event, Event_Data_2 : 0, Timestamp : 2023-09-30T08:53:33+00:00, EvM_Rev : IPMI v2.0 Event Messages, Generator_ID : 0x2000, Event_Data_3 : 1, Event_Type : Lower Non-critical - going low, Record_ID : 3, Sensor_Type : Fan, ",
    "MessageId": "0x500001",
    "Name": "LOG 3",
    "SensorNumber": 17,
    "SensorType": "Fan",
    "Severity": "Warning"
}
```

---

### 3.22 MediaControllerCollection (p.99)

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/MediaControllers`
- **Methods/privileges:** GET: Login.

```json
{
    "@odata.context": "/redfish/v1/$metadata#MediaControllerCollection.MediaControllerCollection",
    "@odata.etag": "\"1696080679\"",
    "@odata.id": "/redfish/v1/Chassis/Self/MediaControllers",
    "@odata.type": "#MediaControllerCollection.MediaControllerCollection",
    "Description": "The Collection of Media Controllers",
    "Members": [],
    "Members@odata.count": 0,
    "Name": " Media Controller Collection"
}
```
The collection is empty on real hardware (note the leading space in Name).

---

### 3.23 MediaController (pp.100–101)

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/MediaControllers/{MediaControllers-Instance}`
- **Methods/privileges:** GET: Login.

Supported properties (Table 28, all RO, String): SerialNumber, MediaControllerType, UUID, Model, PartNumber, Manufacturer.

GET example (pp.100–101). This is a **DMTF mockup** ("Contoso", `/Chassis/GenZ/...`), not real BMC output:
```json
{
    "@odata.type": "#MediaController.v1_1_0.MediaController",
    "Id": "MediaController1",
    "Name": "Media Controller 1",
    "MediaControllerType": "Memory",
    "Manufacturer": "Contoso",
    "Model": "Contoso MediaController",
    "SerialNumber": "2M220100SL",
    "Status": { "State": "Enabled", "Health": "OK" },
    "UUID": "41784113-ed6b-2284-1414-916520dc1dd1",
    "Ports": { "@odata.id": "/redfish/v1/Chassis/GenZ/MediaControllers/1/Ports" },
    "Actions": {
        "#MediaController.Reset": {
            "target": "/redfish/v1/Chassis/GenZ/MediaControllers/1/Actions/MediaController.Reset",
            "ResetType@Redfish.AllowableValues": [ "ForceRestart" ]
        }
    },
    "Links": {
        "Endpoints": [ { "@odata.id": "/redfish/v1/Fabrics/GenZ/Endpoints/1" } ],
        "MemoryDomains": [ { "@odata.id": "/redfish/v1/Chassis/GenZ/MemoryDomains/1" } ]
    },
    "@odata.id": "/redfish/v1/Chassis/GenZ/MediaControllers/1"
}
```
(The DMTF Gen-Z mockup is copied verbatim. Not applicable to the B650D4U in practice.)

---

### 3.24 NetworkAdapterCollection (p.102)

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters`
- **Methods/privileges:** GET: Login.

```json
{
    "@odata.context": "/redfish/v1/$metadata#NetworkAdapterCollection.NetworkAdapterCollection",
    "@odata.etag": "\"1696081571\"",
    "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters",
    "@odata.type": "#NetworkAdapterCollection.NetworkAdapterCollection",
    "Description": "The Collection of Network Adapters",
    "Members": [
        { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0" }
    ],
    "Members@odata.count": 1,
    "Name": "NetworkAdapter Collection"
}
```
Note: the IDs are AMI-generated (`DevType7_NIC0`), derived from the BIOS-reported inventory (the host BIOS pushes it via the Redfish Host Interface; see `/redfish/v1/Oem/Ami/InventoryData/Status`). The data is likely absent until the host has booted at least once.

---

### 3.25 NetworkAdapter (pp.103–106)

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}`
- **Methods/privileges:** GET: Login.

Supported properties (Table 29):

| Property | Type | Read Only | Description |
|---|---|---|---|
| Manufacturer | String | True | Manufacturer/OEM |
| Model | String | True | Model |
| SKU | String | True | SKU |
| SerialNumber | String | True | Serial |
| PartNumber | String | True | Part number |
| Controllers | Array | - | Network controller ASICs in this adapter (Table 30) |
| Actions | Object | True | Actions under Oem |
| NetworkDeviceFunctions | Object | True | Link to NetworkDeviceFunctionCollection |
| Assembly | Object | True | Link to Assembly |
| Ports | Object | True | Link to PortCollection |

Controller properties (Table 30):

| Property | Type | Read Only | Description |
|---|---|---|---|
| FirmwarePackageVersion | String | True | Described as "Resource Identifier" (sic); it is the firmware version |
| Location | Object | True | |
| PCIeInterface | Object | True | PCIe interface details |
| Identifiers | Object | True | Durable names |
| Identifiers.DurableName | String | True | World-wide persistent name |
| Identifiers.DurableNameFormat | String | True | Format of DurableName |
| Links | String (as printed) | True | Links |
| Links.PCIeDevices@odata.count | Number | True | Count |
| Links.PCIeDevices | Array | - | PCIeDevice links |
| Links.Oem | Object | True | - |
| Links.Ports | Array | True | Port links |
| Links.Ports@odata.count | Number | True | Count |
| Links.NetworkDeviceFunctions@odata.count | Number | True | Count |
| Links.NetworkDeviceFunctions | Array | - | NDF links |
| ControllerCapabilities | String (as printed; actually Object) | True | Capabilities |
| ...NetworkPortCount | Number | True | Physical port count |
| ...NetworkDeviceFunctionCount | Number | True | Max physical functions |
| ...DataCenterBridging | Object | - | DCB (Table 31) |
| ...VirtualizationOffload | Object | - | (Table 32) |
| ...NPAR | Object | - | NIC partitioning (Table 34) |
| ...NPIV | Object | - | N_Port ID virtualization (Table 33) |

- DataCenterBridging (Table 31): `Capable` (Boolean, RO).
- VirtualizationOffload (Table 32): `VirtualFunction` (Object, RO) with `DeviceMaxCount`, `NetworkPortMaxCount` and `MinAssignmentGroupSize` (all Number, RO); `SRIOV` (String as printed, RO) with `SRIOVVEPACapable` (Boolean, RO).
- NPIV (Table 33): `MaxDeviceLogins` (Number, RO), `MaxPortLogins` (Number, RO).
- NPAR (Table 34): `NparCapable` (Boolean, RO), `NparEnabled` (Boolean, RO).

#### GET (pp.105–106)

```json
{
    "@odata.context": "/redfish/v1/$metadata#NetworkAdapter.NetworkAdapter",
    "@odata.etag": "\"1696081559\"",
    "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0",
    "@odata.type": "#NetworkAdapter.v1_5_0.NetworkAdapter",
    "Controllers": [
        {
            "ControllerCapabilities": {
                "NetworkDeviceFunctionCount": 1,
                "NetworkPortCount": 1
            },
            "FirmwarePackageVersion": "1.2829.0",
            "Links": {
                "NetworkDeviceFunctions": [
                    { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0" }
                ],
                "NetworkDeviceFunctions@odata.count": 1,
                "PCIeDevices": [
                    { "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_00" }
                ],
                "PCIeDevices@odata.count": 1,
                "Ports": [
                    { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/Ports/DevTypeE_Slot0_Instance0_PORT0" }
                ],
                "Ports@odata.count": 1
            }
        },
        {
            "ControllerCapabilities": {
                "NetworkDeviceFunctionCount": 1,
                "NetworkPortCount": 1
            },
            "FirmwarePackageVersion": "nil",
            "Links": {
                "NetworkDeviceFunctions": [
                    { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction1" }
                ],
                "NetworkDeviceFunctions@odata.count": 1,
                "PCIeDevices": [
                    { "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_01" }
                ],
                "PCIeDevices@odata.count": 1,
                "Ports": [
                    { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/Ports/DevTypeE_Slot0_Instance1_PORT1" }
                ],
                "Ports@odata.count": 1
            }
        }
    ],
    "Id": "DevType7_NIC0",
    "Name": "DevType7_NIC0",
    "NetworkDeviceFunctions": {
        "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions"
    },
    "Ports": { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/Ports" },
    "Status": { "Health": "OK", "State": "Enabled" }
}
```
Caveat: `FirmwarePackageVersion` can be the literal string **`"nil"`** when the value is unknown. PCIeDevice IDs use the `BB_DD_FF` bus/device/function format (`00_01_00`).

---

### 3.26 NetworkDeviceFunctionCollection (pp.107–108)

- **URLs:** `/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions`, `/redfish/v1/Systems/{Systems-Instance}/NetworkInterfaces/{NetworkInterfaces-Instance}/NetworkDeviceFunctions`
- **Methods/privileges:** GET: Login.

```json
{
    "@odata.context": "/redfish/v1/$metadata#NetworkDeviceFunctionCollection.NetworkDeviceFunctionCollection",
    "@odata.etag": "\"1696161152\"",
    "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions",
    "@odata.type": "#NetworkDeviceFunctionCollection.NetworkDeviceFunctionCollection",
    "Description": "The Collection of Network Device Functions",
    "Members": [
        { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0" },
        { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction1" }
    ],
    "Members@odata.count": 2,
    "Name": "NetworkDeviceFunction Collection"
}
```

---

### 3.27 NetworkDeviceFunction Instance (pp.109–115)

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}`
- **Methods/privileges:** GET: Login. (Changes go through the `/SD` future-state resource, section 3.28.)

Supported properties (Table 35):

| Property | Type | Read Only | Description |
|---|---|---|---|
| @Redfish.Settings | Object | True | Settings descriptor |
| @Redfish.Settings.SettingsObject | Object | True | Link to the FutureState (`.../SD`) URI |
| NetDevFuncType | String | True | Configured capability: `Disabled`, `Ethernet`, `FibreChannel`, `iSCSI`, `InfiniBand`, `FibreChannelOverEthernet` |
| DeviceEnabled | Boolean | True | Function enabled; disabled functions are not enumerated by the OS |
| NetDevFuncCapabilities | Array | True | Items of NetDevFuncType |
| Ethernet | Object | - | Ethernet settings |
| Ethernet.PermanentMACAddress | String | True | Permanent MAC |
| Ethernet.MACAddress | String | True | Current MAC |
| Ethernet.VLANs | Object | True | Link to the VLAN collection |
| Ethernet.MTUSize | Number | **False** | MTU |
| iSCSIBoot | Object | - | Table 36 |
| FibreChannel | Object | - | Table 37 |
| BootMode | String | **False** | `Disabled`; `PXE` (only if type is Ethernet); `iSCSI` (only if type is iSCSI); `FibreChannel`; `FibreChannelOverEthernet` |
| VirtualFunctionsEnabled | Boolean | True | SR-IOV VFs enabled |
| MaxVirtualFunctions | Number | True | Number of VFs available |
| Actions | Object | True | Under Oem |
| AssignablePhysicalNetworkPorts | Array | True | Assignable physical ports |
| AssignablePhysicalNetworkPorts@odata.count | Number | True | Count |
| InfiniBand | Object | True | Table 38 |
| Links | Object | True | Links |
| Links.Endpoints@odata.count | Number | True | Count |
| Links.Endpoints | Array | True | Endpoints |
| Links.PhysicalPortAssignment | Object | True | Assigned physical port |
| Links.PhysicalNetworkPortAssignment | Object | True | Assigned physical port |
| Links.EthernetInterface | Object | True | Virtual Ethernet interface created for a VLAN (shows the VLAN's IP) |
| Links.PCIeFunction | Object | True | PCIe function link |

iSCSIBoot properties (Table 36; all writable, i.e. Read Only = False):

| Property | Type | Description |
|---|---|---|
| IPAddressType | String | `IPv4`, `IPv6` |
| InitiatorIPAddress | String | Initiator IP |
| InitiatorName | String | Initiator IQN |
| InitiatorDefaultGateway | String | Gateway |
| InitiatorNetmask | String | Netmask |
| TargetInfoViaDHCP | Boolean | Target name/LUN/IP/netmask from DHCP |
| PrimaryTargetName | String | Primary target name |
| PrimaryTargetIPAddress | String | Primary target IP |
| PrimaryTargetTCPPort | Number | Primary target port |
| PrimaryLUN | Number | Primary LUN |
| PrimaryVLANEnable | Boolean | Primary VLAN enabled |
| PrimaryVLANId | Number | Primary 802.1q VLAN ID |
| PrimaryDNS | String | Primary DNS |
| SecondaryTargetName | String | |
| SecondaryTarget (SecondaryTargetIPAddress) | String | Secondary target IP |
| SecondaryTargetTCPPort | Number | |
| SecondaryLUN | Number | |
| SecondaryVLANEnable | Boolean | |
| SecondaryVLANId | Number | |
| SecondaryDNS | String | |
| IPMaskDNSViaDHCP | Boolean | Initiator uses DHCP for name/IP/netmask |
| RouterAdvertisementEnabled | Boolean | Table row is garbled: it mixes RouterAdvertisementEnabled with AuthenticationMethod. The description "iSCSI boot authentication method" plus the enum `None`, `CHAP`, `MutualCHAP` belong to **AuthenticationMethod** |
| CHAPUsername | String | CHAP username |
| CHAPSecret | String | CHAP secret |
| MutualCHAPUsername | String | 2-way CHAP username |
| MutualCHAPSecret | String | 2-way CHAP secret |

FibreChannel properties (Table 37):

| Property | Type | Read Only | Description |
|---|---|---|---|
| PermanentWWPN | String | True | Permanent WWPN |
| PermanentWWNN | String | True | Permanent WWNN |
| FibreChannelId | String | True | FC Id assigned by switch |
| WWPN | String | False | Effective WWPN (read-only alias of PermanentWWPN if not assignable) |
| WWNN | String | False | Effective WWNN |
| WWNSource | String | False | `ConfiguredLocally`, `ProvidedByFabric` |
| FCoELocalVLANId | Number | False | Local FCoE VLAN ID, 0–4094 |
| AllowFIPVLANDiscovery | Boolean | False | Use FIP VLAN discovery |
| FCoEActiveVLANId | Number | True | Active FCoE VLAN (null when link down), 0–4094 |
| BootTargets | Array | False | FC boot targets |
| BootTargets[].WWPN | String | False | WWPN to boot from |
| BootTargets[].LUNID | String | False | LUN ID |
| BootTargets[].BootPriority | Number | False | Lower = higher priority, 0 = highest, unique |

InfiniBand properties (Table 38, all RO): MTUSize (Number), NodeGUID, PermanentNodeGUID, PermanentPortGUID, PermanentSystemGUID, PortGUID, SystemGUID (String), SupportedMTUSizes (Array of numbers).

#### GET (pp.113–115)

```json
{
    "@Redfish.Settings": {
        "@odata.type": "#Settings.v1_2_2.Settings",
        "SettingsObject": {
            "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0/SD"
        }
    },
    "@odata.context": "/redfish/v1/$metadata#NetworkDeviceFunction.NetworkDeviceFunction",
    "@odata.etag": "\"1696144808\"",
    "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0",
    "@odata.type": "#NetworkDeviceFunction.v1_5_0.NetworkDeviceFunction",
    "AssignablePhysicalNetworkPorts": [
        { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/Ports/DevTypeE_Slot0_Instance0_PORT0" }
    ],
    "AssignablePhysicalNetworkPorts@odata.count": 1,
    "AssignablePhysicalPorts@odata.count": 0,
    "BootMode@Redfish.AllowableValues": [
        "Disabled",
        "PXE",
        "iSCSI",
        "FibreChannel",
        "FibreChannelOverEthernet"
    ],
    "Ethernet": {
        "MACAddress": "A8:A1:59:F7:EC:08",
        "MTUSize": 1500,
        "PermanentMACAddress": "A8:A1:59:F7:EC:08",
        "VLANs": {
            "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0/Ethernet/VLANs"
        }
    },
    "FibreChannel": {
        "WWNSource@Redfish.AllowableValues": [ "ConfiguredLocally", "ProvidedByFabric" ]
    },
    "Id": "NetworkDeviceFunction0",
    "Links": {
        "Endpoints@odata.count": 0,
        "PCIeFunction": {
            "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_00/PCIeFunctions/DevType3_LAN1_DevIndex16"
        },
        "PhysicalNetworkPortAssignment": {
            "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/Ports/DevTypeE_Slot0_Instance0_PORT0"
        }
    },
    "Name": "NetworkDeviceFunction0",
    "NetDevFuncCapabilities": [ "Ethernet" ],
    "NetDevFuncType": "Ethernet",
    "NetDevFuncType@Redfish.AllowableValues": [
        "Disabled",
        "Ethernet",
        "FibreChannel",
        "iSCSI",
        "FibreChannelOverEthernet",
        "InfiniBand"
    ],
    "Status": { "Health": "OK", "State": "Enabled" },
    "iSCSIBoot": {
        "AuthenticationMethod": null,
        "AuthenticationMethod@Redfish.AllowableValues": [ "None", "CHAP", "MutualCHAP" ],
        "CHAPSecret": null,
        "CHAPUsername": null,
        "IPAddressType": null,
        "IPAddressType@Redfish.AllowableValues": [ "IPv4", "IPv6" ],
        "IPMaskDNSViaDHCP": null,
        "InitiatorDefaultGateway": null,
        "InitiatorIPAddress": null,
        "InitiatorName": null,
        "InitiatorNetmask": null,
        "MutualCHAPSecret": null,
        "MutualCHAPUsername": null,
        "PrimaryDNS": null,
        "PrimaryLUN": null,
        "PrimaryTargetIPAddress": null,
        "PrimaryTargetName": null,
        "PrimaryTargetTCPPort": null,
        "PrimaryVLANEnable": null,
        "PrimaryVLANId": null,
        "RouterAdvertisementEnabled": null,
        "SecondaryDNS": null,
        "SecondaryLUN": null,
        "SecondaryTargetIPAddress": null,
        "SecondaryTargetName": null,
        "SecondaryTargetTCPPort": null,
        "SecondaryVLANEnable": null,
        "SecondaryVLANId": null,
        "TargetInfoViaDHCP": null
    }
}
```
Note: `AssignablePhysicalPorts@odata.count` appears without its array (a stray annotation). The response contains no `BootMode` value, only its AllowableValues.

---

### 3.28 NetworkDeviceFunction Future State (SD) (pp.116–119)

"This resource has the property values which will be applied in the next boot." (The BIOS applies the pending settings on the next host boot.)

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/SD`
- **Methods/privileges:** GET: Login. PATCH: ConfigureComponents.
- Properties: same as Table 35.

#### GET (pp.116–118)

The response is identical to the 3.27 GET body, with these differences: there is no `@Redfish.Settings` block; `"@odata.etag": "\"1697900427\""`; `"@odata.id": ".../NetworkDeviceFunctions/NetworkDeviceFunction0/SD"`; and **`"Status": { "Health": "OK", "State": "Disabled" }`**. All other fields (BootMode AllowableValues, Ethernet MAC A8:A1:59:F7:EC:08 / MTU 1500, FibreChannel WWNSource allowables, Links, NetDevFunc*, and the iSCSIBoot block with all nulls) are the same.

#### PATCH (pp.118–119)

`PATCH https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/SD` (Content-Type: application/json)
```json
{
    "iSCSIBoot": {
        "AuthenticationMethod": "None",
        "IPAddressType": "IPv4",
        "IPMaskDNSViaDHCP": false,
        "InitiatorDefaultGateway": "192.168.233.2",
        "InitiatorIPAddress": "192.168.233.250",
        "InitiatorName": "iqn.2023-05.asrockrack.com:Initiator",
        "InitiatorNetmask": "255.255.255.0",
        "PrimaryDNS": "192.168.130.3",
        "PrimaryLUN": 0,
        "PrimaryTargetIPAddress": "192.168.233.2",
        "PrimaryTargetName": "iqn.2023-05.asrockrack.com:test01:iscsiboot3",
        "PrimaryTargetTCPPort": 3260,
        "PrimaryVLANEnable": false,
        "PrimaryVLANId": 0,
        "RouterAdvertisementEnabled": false,
        "TargetInfoViaDHCP": false
    }
}
```
Response: **204**, no body. The guide does not mention an If-Match requirement for SD PATCH. (AMI implementations commonly require `If-Match` with the ETag on the BIOS-applied SD resources; verify against hardware.)

---

### 3.29 VLANNetworkInterfaceCollection (pp.120–121)

- **URLs:**
  - `/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/Ethernet/VLANs`
  - `/redfish/v1/Systems/{Systems-Instance}/EthernetInterfaces/{EthernetInterfaces-Instance}/VLANs`
- **Methods/privileges:** GET: Login. POST: ConfigureManager.

#### GET (p.120)

```json
{
    "@odata.context": "/redfish/v1/$metadata#VLanNetworkInterfaceCollection.VLanNetworkInterfaceCollection",
    "@odata.etag": "\"1697896922\"",
    "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0/Ethernet/VLANs",
    "@odata.type": "#VLanNetworkInterfaceCollection.VLanNetworkInterfaceCollection",
    "Description": "Collection of VLANs for this system",
    "Members": [
        { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0/Ethernet/VLANs/oob_NetworkDeviceFunction0_100" }
    ],
    "Members@odata.count": 1,
    "Name": "VLAN Network Interface Collection"
}
```
VLAN member ID format: `oob_<NDF-Id>_<VLANId>`.

#### POST: create VLAN (p.121)

`POST https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/Ethernet/VLANs` (Content-Type: application/json)
```json
{
    "VLANId": 100,
    "VLANEnable": true,
    "VLANPriority": 2
}
```
Response: **204**, no body. DMTF expects 201 with a Location header; this implementation returns 204, so the new member URI must be discovered by re-reading the collection.

---

### 3.30 VLANNetworkInterface (pp.122–123)

- **URLs:**
  - `/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/Ethernet/VLANs/{VLAN-Instance}`
  - `/redfish/v1/Systems/{Systems-Instance}/EthernetInterfaces/{EthernetInterfaces-Instance}/VLANs/{VLAN-Instance}`
- **Methods/privileges:** GET: Login. DELETE: ConfigureManager.

Supported properties (Table 39, all listed RO; changes go through `/SD`):

| Property | Type | Read Only | Description |
|---|---|---|---|
| VLANEnable | Boolean | True | VLAN enabled for this interface |
| VLANId | Number | True | VLAN ID, 0–4094 |
| VLANPriority | Number | True | Priority, 0–7 |

#### GET (pp.122–123)

```json
{
    "@Redfish.Settings": {
        "@odata.type": "#Settings.v1_2_2.Settings",
        "SettingsObject": {
            "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0/Ethernet/VLANs/VLAN0001/SD"
        }
    },
    "@odata.context": "/redfish/v1/$metadata#VLanNetworkInterface.VLanNetworkInterface",
    "@odata.etag": "\"1697897404\"",
    "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0/Ethernet/VLANs/VLAN0001",
    "@odata.type": "#VLanNetworkInterface.v1_2_0.VLanNetworkInterface",
    "Id": "VLAN0001",
    "Name": "VLAN0001",
    "VLANEnable": true,
    "VLANId": 100,
    "VLANPriority": 2
}
```
Note: the member ID here is `VLAN0001`, while the collection example (p.120) showed `oob_NetworkDeviceFunction0_100`. The ID format is inconsistent within the doc, so do not assume either; follow links.

#### DELETE (p.123)

`DELETE https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/Ethernet/VLANs/{VLAN-Instance}`. Response: **204**, no body.

---

### 3.31 VLANNetworkInterface Future State (SD) (pp.124–125)

Property values applied at the next boot.

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/NetworkDeviceFunctions/{NetworkDeviceFunctions-Instance}/Ethernet/VLANs/{VLAN-Instance}/SD`
- **Methods/privileges:** GET: Login. PATCH: ConfigureManager.
- Properties (Table 40): same as Table 39 (VLANEnable Boolean; VLANId Number 0–4094; VLANPriority Number 0–7). They are printed RO but are writable via PATCH here.

#### GET (pp.124–125)

```json
{
    "@odata.context": "/redfish/v1/$metadata#VLanNetworkInterface.VLanNetworkInterface",
    "@odata.etag": "\"1697897404\"",
    "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0/Ethernet/VLANs/VLAN0001/SD",
    "@odata.type": "#VLanNetworkInterface.v1_2_0.VLanNetworkInterface",
    "Id": "VLAN0001",
    "Name": "VLAN0001",
    "VLANEnable": true,
    "VLANId": 100,
    "VLANPriority": 2
}
```

#### PATCH (p.125)

```json
{
    "VLANId": 103,
    "VLANEnable": true,
    "VLANPriority": 3
}
```
Response: **204**, no body.

---

### 3.32 PortCollection (p.126)

- **URLs:**
  - `/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/Ports`
  - `/redfish/v1/Systems/{Systems-Instance}/GraphicsControllers/{GraphicsController-Instance}/Ports`
  - `/redfish/v1/Systems/{Systems-Instance}/USBControllers/{USBControllers-Instance}/Ports`
- **Methods/privileges:** GET: Login.

```json
{
    "@odata.context": "/redfish/v1/$metadata#PortCollection.PortCollection",
    "@odata.etag": "\"1696150352\"",
    "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/Ports",
    "@odata.type": "#PortCollection.PortCollection",
    "Description": "The Collection of Port",
    "Members": [
        { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/Ports/DevTypeE_Slot0_Instance0_PORT0" },
        { "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/Ports/DevTypeE_Slot0_Instance1_PORT1" }
    ],
    "Members@odata.count": 2,
    "Name": "Port Collection"
}
```

---

### 3.33 Port (pp.127–130)

- **URLs:**
  - `/redfish/v1/Chassis/{Chassis-Id}/NetworkAdapters/{NetworkAdapters-Instance}/Ports/{Ports-Instance}`
  - `/redfish/v1/Systems/{Systems-Instance}/GraphicsControllers/{GraphicsController-Instance}/Ports/{Port-Instance}`
  - `/redfish/v1/Systems/{Systems-Instance}/USBControllers/{USBControllers-Instance}/Ports/{Port-Instance}`
- **Methods/privileges:** GET: Login.

Supported properties (Table 41, all RO):

| Property | Type | Description |
|---|---|---|
| ActiveWidth | Number | Active lanes |
| CurrentSpeedGbps | Number | Current speed |
| InterfaceEnabled | Boolean | Interface enabled |
| LinkTransitionIndicator | Boolean (as printed) | Number of link state transitions |
| LocationIndicatorActive | Boolean | Locator indicator |
| Location | Object | Port location |
| MaxFrameSize | Number | Max frame size |
| MaxSpeedGbps | Number | Max configured speed |
| PortId | String | Physical label |
| SignalDetected | Boolean | Signal detected |
| Ethernet | Object | Ethernet properties |
| Ethernet.FlowControlConfiguration | String | Configured 802.3x flow control |
| Ethernet.FlowControlStatus | String | Negotiated flow control |
| Ethernet.LLDPEnabled | Boolean | LLDP enabled |
| Ethernet.WakeOnLANEnabled | Boolean | WoL enabled |
| Ethernet.EEEEnabled | Boolean | 802.3az EEE enabled |
| Ethernet.AssociatedMACAddresses | Array of String | Configured MACs associated with the port |
| Ethernet.LLDPReceive | Object | LLDP data received (Table 42) |
| Ethernet.LLDPTransmit | Object | LLDP data transmitted (Table 42) |
| Ethernet.SupportedEthernetCapabilities | Array of String | `EEE`, `WakeOnLAN` |
| FibreChannel | Object | FC properties |
| FibreChannel.PortConnectionType | String | Connection type |
| FibreChannel.NumberDiscoveredRemotePorts | Number | Discovered remote ports |
| FibreChannel.FabricName | String | Fabric name |
| LinkConfiguration | Object | Link configuration |
| LinkConfiguration.AutoSpeedNegotiationCapable | Boolean | Autoneg capable |
| LinkConfiguration.AutoSpeedNegotiationEnabled | Number (as printed) | Autoneg enabled |
| LinkConfiguration.ConfiguredNetworkLinks | Array of Objects | Configured speed/width pairs |
| ...ConfiguredWidth | String | Configured width |
| ...ConfiguredLinkSpeedGbpsNumber | Integer | Configured speed per lane |
| LinkNetworkTechnology | String | Link technology |
| PortMedium | String | Physical medium |
| PortProtocol | String | Protocol |
| PortType | String | Port type |
| LinkState | String | Desired link state |
| LinkStatus | String | Link status |
| Links | Object | Related resources |
| Links.ConnectedSwitches (+ @odata.count) | Array of Links / Number | Switches |
| Links.ConnectedPorts (+ @odata.count) | Array of Links / Number | Remote ports |
| Links.AssociatedEndpoints (+ @odata.count) | Array of Links / Number | Endpoints |
| Links.ConnectedSwitchPorts (+ @odata.count) | Array of Links / Number | Switch ports |

LLDPTransmit/LLDPReceive (Table 42, all RO):

| Property | Type | Description |
|---|---|---|
| ChassisId | String | LLDP chassis ID |
| ChassisIdSubtype | String | See Table 43 |
| ManagementAddressIPv4 | String | IPv4 management address |
| ManagementAddressIPv6 | String | IPv6 management address |
| ManagementAddressMAC | String | Management MAC |
| ManagementVlanId | Number | Management VLAN |
| PortId | String | Colon-delimited hex octets |
| PortIdSubtype | String | See Table 43 |

IEEE 802.1AB-2009 chassis ID subtype (Table 43): `ChassisComp` (entPhysicalAlias, RFC4133), `IfAlias` (ifAlias MIB), `PortComp` (entPhysicalAlias of port), `MacAddr` (IEEE 802 unicast source), `NetworkAddr`, `IfName` (ifName MIB), `AgentId` (agent circuit ID, RFC3046), `LocalAssign` (locally assigned alphanumeric), `NotTransmitted`.

#### GET (pp.129–130)

```json
{
    "@odata.context": "/redfish/v1/$metadata#Ports.Ports",
    "@odata.etag": "\"1696150352\"",
    "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/Ports/DevTypeE_Slot0_Instance0_PORT0",
    "@odata.type": "#Port.v1_5_0.Port",
    "Id": "DevTypeE_Slot0_Instance0_PORT0",
    "InterfaceEnabled": true,
    "LinkNetworkTechnology": "Ethernet",
    "LinkState": "Enabled",
    "LinkStatus": "LinkUp",
    "Name": "DevTypeE_Slot0_Instance0_PORT0",
    "PortId": "0",
    "PortProtocol": "Ethernet",
    "SignalDetected": true,
    "Status": { "Health": "OK", "State": "Enabled" }
}
```
Note: `@odata.context` is `#Ports.Ports` (non-standard; DMTF uses `#Port.Port`). Section 3.33 ends on p.130. **3.34 PCIeDevice Collection starts on p.131 and is covered in part 2.**

---

## Implementer notes / deviations from DMTF observed in pp.1–130

1. **Resource IDs:** chassis, system and manager are all `Self` (`/redfish/v1/Chassis/Self`, `/Systems/Self`, `/Managers/Self`). Inventory IDs are AMI-generated (`DevType7_NIC0`, `DevTypeE_Slot0_Instance0_PORT0`, `NetworkDeviceFunction0`, PCIe `00_01_00`, PCIe function `DevType3_LAN1_DevIndex16`).
2. **Auth:** Basic auth and Redfish sessions (X-Auth-Token) are both supported (p.26). The guide lists GET `/redfish/v1/` as requiring Login. Default account `Administrator` (ID 1) has `PasswordChangeRequired: true`. Password length 8–20. Lockout defaults: 5 failures, 30 s.
3. **Privileges:** Administrator, Operator and ReadOnly per Table 2, plus a predefined `HostInterfaceAdministrator` role. Custom roles can be created via POST to `/AccountService/Roles`, with OemPrivileges such as `OemPowerControl`, `OemClearLog` and `ConfigureHostInterface`.
4. **ETag:** `@odata.etag` values are quoted Unix timestamps (`"\"1694985188\""`). Status codes 412/428 are listed, but chapters 1–3.33 never document an If-Match requirement. Verify on hardware, especially for `/SD` PATCH.
5. **Status codes in practice:** PATCH returns 204 with no body. POST create returns 201 with a body for accounts, certificates and roles, but **204** for VLAN create. Actions: Chassis.Reset and LogService.ClearLog return **202 with a Task** (`/redfish/v1/TaskService/Tasks/{n}`); Certificate Rekey/Renew/GenerateCSR return 200 with a body; ReplaceCertificate returns 204. DELETE returns 204.
6. **The error body format is not documented** in pp.1–130 (no `@Message.ExtendedInfo` examples). The MessageRegistry is in part 2 (pp.239+).
7. **Schema violations to tolerate in a Go client:** object properties returned as `[]` (Chassis Location/Placement/PostalAddress/PhysicalSecurity, CertificateLocations Oem SMTP); `"nil"` strings; non-zero-padded dates (`2033-9-14T04:47:31+00:00`); action responses returning a link as a bare string (`"Certificate": "/redfish/v1/..."`); stray `AssignablePhysicalPorts@odata.count`; `Id` with spaces (`"RADIUS Server"`); placeholder FRU strings (`"To Be Filled By O.E.M."`, `" "`); expanded members in the LogEntry collection; `@odata.context` `#Ports.Ports`.
8. **OEM namespace:** only `Oem.Ami` appears (AMIServiceRoot, AMIAccountService, AMIExternalAccountProvider, AMIAdvanceRADIUSSetting, AMIAccountServiceConfigurations). **No `Oem.ASRockRack` appears in pp.1–130.**
9. **Query support (ServiceRoot):** `$expand` (ExpandAll, Links, NoLinks, Levels; MaxLevels 5), `$filter`, `$select`, `only`, `excerpt`, DeepPATCH (MaxLevels 6). DeepPOST is not supported.
10. **Certificates:** PEM only. The private key is concatenated in the same `CertificateString` and must be unencrypted PKCS#1 or PKCS#8. KeyBitLength 512–2048 (max 2048, so no 4096-bit keys). The HTTPS certificate is replaced via `CertificateService.ReplaceCertificate` with `CertificateUri` set to `/redfish/v1/Managers/Self/NetworkProtocol/HTTPS/Certificates/1`.
11. **"SD" (Settings Data / future state):** NetworkDeviceFunction and VLAN changes are PATCHed to `.../SD` and applied at the next host boot (via BIOS). The live resources advertise them via `@Redfish.Settings.SettingsObject`.
12. **Documentation quality:** several examples are DMTF mockups (MediaController "Contoso"/GenZ). Some tables carry copy-paste errors (CertificateType description, RouterAdvertisementEnabled/AuthenticationMethod merged, ReadOnly flags contradicting the PATCH examples, such as LDAP.ServiceEnabled). Trust the examples over the tables where they conflict.
13. **Board/firmware applicability:** the doc is the generic "BMC AST2600 Generic Redfish v1.11 API User Guide", Rev 1.00, December 2023. No board model (B650D4U or otherwise) and no BMC firmware version is stated in pp.1–130. Example data shows `RtpVersion` 13.04, `AMI Redfish Server`, and RedfishVersion 1.11.0.
