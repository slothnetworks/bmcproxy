# ASRock Rack (AMI MegaRAC) Redfish API Guide: transcription, part 3 (PDF pages 261-381)

Source: `/opt/sparked/asrockrack-api/redfishdocs/asrockrack-redfishdocs.pdf` (printed with "Microsoft Print To PDF"; every page is an image). This file transcribes PDF pages 261-381. Page references such as `(p.300)` use the PDF's own printed page numbers, which match the PDF page index.

Conventions used in the source:
- `{BMC_IP}` stands for the BMC address. `{Systems-Instance}` is `Self` in every example.
- Every example request carries the header `Content-Type: application/json`.
- "Read Only" columns are given as True/False. Enum values are listed as indented sub-rows with `-` in the Type and Read Only columns.
- "Please refer to section ODATA and Frequently Used Properties" points to a section in an earlier part of the PDF (see part 1/2).
- The sample responses come from a **GENOAD8UD-2T/X550** board (AMD Genoa), not a B650D4U, so expect differences in members and values.

---

## 3.78 ComputerSystem (continued from part 2)

### Table 103 (continued). ComputerSystem `Boot` properties (p.261)

The table begins in part 2. The first rows on p.261 are the remaining enum values of `BootSourceOverrideTarget`, the property whose header row ("The allowable values for this property are specified in the following:") falls at the end of part 2.

| Property | Type | Read Only | Description |
|---|---|---|---|
| *(BootSourceOverrideTarget, continued)* | | | The allowable values for this property are specified in the following: |
| &nbsp;&nbsp;None | - | - | Boot from the normal boot device. |
| &nbsp;&nbsp;Pxe | - | - | Boot from the Pre-Boot Execution (PXE) environment. |
| &nbsp;&nbsp;Floppy | - | - | Boot from the floppy disk drive. |
| &nbsp;&nbsp;Cd | - | - | Boot from the CD/DVD disc. |
| &nbsp;&nbsp;Usb | - | - | Boot from a USB device as specified by the system BIOS. |
| &nbsp;&nbsp;Hdd | - | - | Boot from a hard drive. |
| &nbsp;&nbsp;BiosSetup | - | - | Boot to the BIOS Setup Utility. |
| &nbsp;&nbsp;Utilities | - | - | Boot the manufacturer's Utilities program(s). |
| &nbsp;&nbsp;Diags | - | - | Boot the manufacturer's Diagnostics program. |
| &nbsp;&nbsp;UefiShell | - | - | Boot to the UEFI Shell. |
| &nbsp;&nbsp;UefiTarget | - | - | Boot to the UEFI Device specified in the UefiTargetBootSourceOverride property. |
| &nbsp;&nbsp;SDCard | - | - | Boot from an SD Card. |
| &nbsp;&nbsp;UefiHttp | - | - | Boot from a UEFI HTTP network location. |
| &nbsp;&nbsp;RemoteDrive | - | - | Boot from a remote drive (e.g. iSCSI). |
| &nbsp;&nbsp;UefiBootNext | - | - | Boot to the UEFI Device specified in the BootNext property. |
| BootSourceOverrideEnabled | String | False | The value of this property shall be Once if this is a one-time boot override and Continuous if this selection should remain active until cancelled. |
| &nbsp;&nbsp;Disabled | - | - | The system will boot normally. |
| &nbsp;&nbsp;Once | - | - | On its next boot cycle, the system will boot (one time) to the Boot Source Override Target. |
| &nbsp;&nbsp;Continuous | - | - | The system will boot to the target specified in the BootSourceOverrideTarget, **when bios reboots or if host restart not happen in 60 seconds then this property will set to Disabled.** |
| UefiTargetBootSourceOverride | String | False | The value of this property shall be the UEFI device path of the override boot target. |
| AliasBootOrder | Array | True | Ordered array of boot source aliases representing the persistent Boot Order associated with this computer system. |
| BootOrder | Array | False | Ordered array of BootOptionReference strings representing the persistent Boot Order associated with this computer system. |
| BootOrderPropertySelection | String | True | The value of this property shall indicate which boot order property the system uses when specifying the persistent boot order. |
| HttpBootUri | String | False | This property shall contain the URI to perform an HTTP or HTTPS boot. |
| BootSourceOverrideMode | String | False | The value of this property shall be Legacy for non-UEFI BIOS boot or UEFI for UEFI boot from boot source specified in BootSourceOverrideTarget property. |
| &nbsp;&nbsp;Legacy | - | - | The system will boot in non-UEFI boot mode to the Boot Source Override Target. |
| &nbsp;&nbsp;UEFI | - | - | The system will boot in UEFI boot mode to the Boot Source Override Target. |

> **Caveat (differs from DMTF):** `BootSourceOverrideEnabled: "Continuous"` is not persistent as in the DMTF schema. The BMC resets it to `Disabled` when the BIOS reboots, or when no host restart happens within 60 seconds (p.261). A proxy should not rely on Continuous persisting.

### Table 104. ComputerSystem - MemorySummary - MemoryMirroringEnum Properties (p.261)

| Enum | Description |
|---|---|
| System | The system supports DIMM mirroring at the System level. Individual DIMMs are not paired for mirroring in this mode. |
| DIMM | The system supports DIMM mirroring at the DIMM level. Individual DIMMs can be mirrored. |

### Table 105. Trusted Modules Properties (p.262)

| Property | Type | Read Only | Description |
|---|---|---|---|
| FirmwareVersion | String | True | The firmware version of this Trusted Module. |
| InterfaceType | String | True | This property indicates the interface type of the Trusted Module. |
| &nbsp;&nbsp;TPM1_2 | - | - | Trusted Platform Module (TPM) 1.2. |
| &nbsp;&nbsp;TPM2_0 | - | - | Trusted Platform Module (TPM) 2.0. |
| &nbsp;&nbsp;TCM1_0 | - | - | Trusted Cryptography Module (TCM) 1.0. |
| FirmwareVersion2 | String | True | The 2nd firmware version of this Trusted Module, if applicable. |
| InterfaceTypeSelection | String | True | The Interface Type selection supported by this Trusted Module. |
| &nbsp;&nbsp;None | - | - | The TrustedModule does not support switching the InterfaceType. |
| &nbsp;&nbsp;FirmwareUpdate | - | - | The TrustedModule supports switching InterfaceType via a firmware update. |
| &nbsp;&nbsp;BiosSetting | - | - | The TrustedModule supports switching InterfaceType via platform software, such as a BIOS configuration Attribute. |
| &nbsp;&nbsp;OemMethod | - | - | The TrustedModule supports switching InterfaceType via an OEM proprietary mechanism. |

### 3.78.4 Example (p.262)

#### 3.78.4.1 GET ComputerSystem Instance (p.262-265)

Request:
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}
Content-Type: application/json
```

Response (JSON):
```json
{
    "@Redfish.Settings": {
        "@odata.type": "#Settings.v1_2_2.Settings",
        "SettingsObject": {
            "@odata.id": "/redfish/v1/Systems/Self/SD"
        }
    },
    "@odata.context": "/redfish/v1/$metadata#ComputerSystem.ComputerSystem",
    "@odata.etag": "\"1702107620\"",
    "@odata.id": "/redfish/v1/Systems/Self",
    "@odata.type": "#ComputerSystem.v1_15_0.ComputerSystem",
    "Actions": {
        "#ComputerSystem.Reset": {
            "@Redfish.ActionInfo": "/redfish/v1/Systems/Self/ResetActionInfo",
            "@Redfish.OperationApplyTimeSupport": {
                "@odata.type": "#Settings.v1_2_2.OperationApplyTimeSupport",
                "MaintenanceWindowDurationInSeconds": 600,
                "MaintenanceWindowResource": {
                    "@odata.id": "/redfish/v1/Systems/Self"
                },
                "SupportedValues": [
                    "Immediate",
                    "AtMaintenanceWindowStart"
                ]
            },
            "target": "/redfish/v1/Systems/Self/Actions/ComputerSystem.Reset"
        }
    },
    "AssetTag": "",
    "BiosVersion": null,
    "Boot": {
        "BootNext": null,
        "BootOptions": {
            "@odata.id": "/redfish/v1/Systems/Self/BootOptions"
        },
        "BootOrderPropertySelection": null,
        "BootSourceOverrideEnabled": null,
        "BootSourceOverrideMode": null,
        "BootSourceOverrideTarget": null,
        "Certificates": {
            "@odata.id": "/redfish/v1/Systems/Self/Boot/Certificates"
        },
        "HttpBootUri": null,
        "UefiTargetBootSourceOverride": null
    },
    "Description": "System Self",
    "EthernetInterfaces": {
        "@odata.id": "/redfish/v1/Systems/Self/EthernetInterfaces"
    },
    "FruInfo": {
        "@odata.id": "/redfish/v1/Systems/Self/FruInfo"
    },
    "GraphicsControllers": {
        "@odata.id": "/redfish/v1/Systems/Self/GraphicsControllers"
    },
    "HostName": null,
    "HostingRoles": [
        "ApplicationServer"
    ],
    "Id": "Self",
    "IndicatorLED": "Off",
    "IndicatorLED@Redfish.AllowableValues": [
        "Lit",
        "Blinking",
        "Off"
    ],
    "Links": {
        "Chassis": [
            {
                "@odata.id": "/redfish/v1/Chassis/Self"
            }
        ],
        "Chassis@odata.count": 1,
        "ManagedBy": [
            {
                "@odata.id": "/redfish/v1/Managers/Self"
            }
        ],
        "ManagedBy@odata.count": 1
    },
    "LogServices": {
        "@odata.id": "/redfish/v1/Systems/Self/LogServices"
    },
    "Manufacturer": "",
    "Memory": {
        "@odata.id": "/redfish/v1/Systems/Self/Memory"
    },
    "MemoryDomains": {
        "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains"
    },
    "MemorySummary": {
        "MemoryMirroring": null,
        "Status": {
            "Health": null,
            "HealthRollup": null,
            "State": null
        },
        "TotalSystemMemoryGiB": null,
        "TotalSystemPersistentMemoryGiB": null
    },
    "Model": "GENOAD8UD-2T/X550",
    "Name": "System",
    "NetworkInterfaces": {
        "@odata.id": "/redfish/v1/Systems/Self/NetworkInterfaces"
    },
    "Oem": {
        "Ami": {
            "@odata.type": "#AMIBIOSInventoryCRC.v1_0_0.AMIBIOSInventoryCRC",
            "ManagerBootConfiguration": {
                "ManagerBootMode": null,
                "ManagerBootMode@Redfish.AllowableValues": [
                    "SoftReset",
                    "ResetTimeout"
                ]
            }
        }
    },
    "PartNumber": "",
    "PowerRestorePolicy": "LastState",
    "PowerState": "Off",
    "ProcessorSummary": {
        "Count": null,
        "Model": null,
        "Status": {
            "Health": null,
            "HealthRollup": null,
            "State": null
        }
    },
    "Processors": {
        "@odata.id": "/redfish/v1/Systems/Self/Processors"
    },
    "SKU": " ",
    "SecureBoot": {
        "@odata.id": "/redfish/v1/Systems/Self/SecureBoot"
    },
    "SerialNumber": "",
    "SimpleStorage": {
        "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage"
    },
    "Status": {
        "Health": "OK",
        "HealthRollup": "OK",
        "State": "Enabled"
    },
    "Storage": {
        "@odata.id": "/redfish/v1/Systems/Self/Storage"
    },
    "SubModel": null,
    "SystemType": "Physical",
    "USBControllers": {
        "@odata.id": "/redfish/v1/Systems/Self/USBControllers"
    },
    "UUID": null
}
```

Notes on this example:
- Pending settings use a Settings object at `/redfish/v1/Systems/Self/SD` (`@Redfish.Settings`, Settings v1_2_2).
- ETags are quoted epoch-like integers, e.g. `"\"1702107620\""`.
- `ComputerSystem.Reset` supports `@Redfish.OperationApplyTimeSupport` with `Immediate` and `AtMaintenanceWindowStart`, a 600 s maintenance window, and an ActionInfo at `/redfish/v1/Systems/Self/ResetActionInfo`.
- OEM: `Oem.Ami` of type `#AMIBIOSInventoryCRC.v1_0_0.AMIBIOSInventoryCRC` with `ManagerBootConfiguration.ManagerBootMode` (allowable values `SoftReset`, `ResetTimeout`).
- Non-standard links: `FruInfo` (`/redfish/v1/Systems/Self/FruInfo`, see 3.81) and `Boot.Certificates`.
- Many values are `null` because the host was powered off (`PowerState: "Off"`). BIOS-sourced inventory appears to be filled in only after the BIOS has pushed its data.

#### 3.78.4.2 PATCH ComputerSystem Instance (p.265-266)

Request:
```
PATCH https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}
Content-Type: application/json
```

Example request body 1:
```json
{
    "IndicatorLED": "Lit"
}
```

Response: success with status code **202** and a message body (a Task):
```json
{
    "@odata.context": "/redfish/v1/$metadata#Task.Task",
    "@odata.id": "/redfish/v1/TaskService/Tasks/1",
    "@odata.type": "#Task.v1_4_2.Task",
    "Description": "Task for Systems - IndicatorLED",
    "Id": "1",
    "Name": "Systems - IndicatorLED",
    "TaskState": "New"
}
```

Example request body 2:
```json
{
    "AssetTag": "Free form asset tag"
}
```

Response: success with status code **204** and no body (p.266).

> **Caveat:** A PATCH to `IndicatorLED` is asynchronous. It returns 202 with a Task (`/redfish/v1/TaskService/Tasks/{n}`), while a PATCH to `AssetTag` returns 204. Clients must handle both.

#### 3.78.4.3 POST ComputerSystem Instance (Reset action) (p.266)

Request:
```
POST https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Actions/ComputerSystem.Reset
Content-Type: application/json
```

Example request body:
```json
{
    "ResetType": "On"
}
```

Response: on success the status is **202** with a message body (a Task):
```json
{
    "@odata.context": "/redfish/v1/$metadata#Task.Task",
    "@odata.id": "/redfish/v1/TaskService/Tasks/2",
    "@odata.type": "#Task.v1_4_2.Task",
    "Description": "Task for Computer Reset",
    "Id": "2",
    "Name": "Computer Reset",
    "TaskState": "New"
}
```

> **Caveat (differs from common DMTF practice):** `ComputerSystem.Reset` returns 202 Accepted with a Task body rather than 204/200.

---

## 3.79 BootOptionCollection (p.267)

It displays a list of BootOption instances. This represents the collection of BootOption resources.

### 3.79.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/BootOptions
```

### 3.79.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| POST | ConfigureComponents |

### 3.79.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties" for the supported properties.

### 3.79.4 Example

#### 3.79.4.1 GET BootOptionCollection Instance (p.267-268)

Request:
```
GET https://{BMC_IP}/redfish/v1/Systems/Self/BootOptions
Content-Type: application/json
```

Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#BootOptionCollection.BootOptionCollection",
    "@odata.etag": "\"1702121796\"",
    "@odata.id": "/redfish/v1/Systems/Self/BootOptions",
    "@odata.type": "#BootOptionCollection.BootOptionCollection",
    "Description": "Collection of BootOption for this system",
    "Members": [
        { "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0000" },
        { "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0001" },
        { "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0002" },
        { "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0003" },
        { "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0004" },
        { "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0005" },
        { "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0006" },
        { "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0007" },
        { "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0008" },
        { "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0009" }
    ],
    "Members@odata.count": 10,
    "Name": "BootOption Collection"
}
```

#### 3.79.4.2 POST BootOptionCollection Instance (p.268-269)

Request:
```
POST https://{BMC_IP}/redfish/v1/Systems/Self/BootOptions
Content-Type: application/json
```

Example request body:
```json
{
    "Name": "Boot Option",
    "Description": "UEFI Boot Option",
    "BootOptionReference": "Boot0A",
    "DisplayName" : "Windows Boot Manager",
    "UefiDevicePath": "PciRoot(0x0)/Pci(0x14,0x0)/USB(0x4,0x0)/USB(0x0,0x0)/HD(1,MBR,0x006EC15D,0x40,0x1DFFFC0)",
    "Alias": "Hdd",
    "RelatedItem": [
        {
            "@odata.id": "/redfish/v1/Systems/1/SimpleStorage/1"
        }
    ]
}
```

Response: status **201**. The body is a GET response with the properties of the newly created BootOptions instance:
```json
{
    "@Redfish.Settings": {
        "@odata.type": "#Settings.v1_2_2.Settings",
        "SettingsObject": {
            "@odata.id": "/redfish/v1/Systems/Self/BootOptions/oob_Boot0A/SD"
        }
    },
    "@odata.context": "/redfish/v1/$metadata#BootOption.BootOption",
    "@odata.etag": "\"1702123643\"",
    "@odata.id": "/redfish/v1/Systems/Self/BootOptions",
    "@odata.type": "#BootOption.v1_0_3.BootOption",
    "Alias": "Hdd",
    "BootOptionEnabled": true,
    "BootOptionReference": "Boot0A",
    "Description": "UEFI Boot Option",
    "DisplayName": "Windows Boot Manager",
    "Id": "oob_Boot0A",
    "Name": "Boot Option",
    "RelatedItem": [
        {
            "@odata.id": "/redfish/v1/Systems/1/SimpleStorage/1"
        }
    ],
    "RelatedItem@odata.count": 1,
    "UefiDevicePath": "PciRoot(0x0)/Pci(0x14,0x0)/USB(0x4,0x0)/USB(0x0,0x0)/HD(1,MBR,0x006EC15D,0x40,0x1DFFFC0)"
}
```

> **Notes:** Boot options created out of band get the Id `oob_<BootOptionReference>`, e.g. `oob_Boot0A`, with a Settings object at `.../BootOptions/oob_Boot0A/SD`. The example's `@odata.id` in the 201 body is the collection URI, not the new member (likely a firmware or documentation bug). `RelatedItem` uses `/redfish/v1/Systems/1/...`, which is inconsistent with `Self`.

---

## 3.80 BootOption (p.270)

Displays the information about the BootOptions.

### 3.80.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/BootOptions/{BootOptions-Instance}
```

### 3.80.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

(PATCH is not listed here, even though `BootOptionEnabled` is marked writable and a `@Redfish.Settings` SD object exists. Changes are presumably made through `.../BootOptions/{id}/SD`.)

### 3.80.3 Supported Properties

#### Table 106. Supported Properties for BootOption (p.270)

| Property | Type | Read Only | Description |
|---|---|---|---|
| @Redfish.Settings | Object | True | A Reference to the FutureState URI for this resource. |
| &nbsp;&nbsp;@odata.type | String | True | Please refer to section ODATA and Frequently Used Properties for the supported properties. |
| &nbsp;&nbsp;SettingsObject | Object | True | A Reference to the FutureState URI for this resource. |
| Actions | Object | True | The available actions for this Resource. |
| BootOptionEnabled | Boolean | False | An indication of whether the boot option is enabled. |
| BootOptionReference | String | True | The unique boot option. |
| DisplayName | String | True | The user-readable display name of the boot option that appears in the boot order list in the user interface. |
| RelatedItem | Array | True | An array of one or more IDs for the Resources associated with this boot option. |
| RelatedItem@odata.count | Number | True | Number of RelatedItems |
| UefiDevicePath | String | True | The UEFI device path to access this UEFI Boot Option. |
| Alias | String | True | The alias of this boot source. |
| &nbsp;&nbsp;None | - | - | Boot from the normal boot device. |
| &nbsp;&nbsp;Pxe | - | - | Boot from the Pre-Boot Execution (PXE) environment. |
| &nbsp;&nbsp;Floppy | - | - | Boot from the floppy disk drive. |
| &nbsp;&nbsp;Cd | - | - | Boot from the CD/DVD disc. |
| &nbsp;&nbsp;Usb | - | - | Boot from a USB device as specified by the system BIOS |
| &nbsp;&nbsp;Hdd | - | - | Boot from a hard drive |
| &nbsp;&nbsp;BiosSetup | - | - | Boot to the BIOS Setup Utility. |
| &nbsp;&nbsp;Utilities | - | - | Boot the manufacturer's Utilities program(s). |
| &nbsp;&nbsp;Diags | - | - | Boot the manufacturer's Diagnostics program. |
| &nbsp;&nbsp;UefiShell | - | - | Boot to the UEFI Shell. |
| &nbsp;&nbsp;UefiTarget | - | - | Boot to the UEFI Device specified in the UefiTargetBootSourceOverride property. |
| &nbsp;&nbsp;SDCard | - | - | Boot from an SD Card. |
| &nbsp;&nbsp;UefiHttp | - | - | Boot from a UEFI HTTP network location. |
| &nbsp;&nbsp;RemoteDrive | - | - | Boot from a remote drive (e.g. iSCSI). |
| &nbsp;&nbsp;UefiBootNext | - | - | Boot to the UEFI Device specified in the BootNext property. |

### 3.80.4 Example

#### 3.80.4.1 GET BootOption Instance (p.271)

Request:
```
GET https://{BMC_IP}/redfish/v1/Systems/Self/BootOptions/{BootOptions-Instance}
Content-Type: application/json
```

Response:
```json
{
    "@Redfish.Settings": {
        "@odata.type": "#Settings.v1_2_2.Settings",
        "SettingsObject": {
            "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0000/SD"
        }
    },
    "@odata.context": "/redfish/v1/$metadata#BootOption.BootOption",
    "@odata.etag": "\"1702121793\"",
    "@odata.id": "/redfish/v1/Systems/Self/BootOptions/0000",
    "@odata.type": "#BootOption.v1_0_3.BootOption",
    "Alias": "Hdd",
    "BootOptionEnabled": false,
    "BootOptionReference": "Boot0000",
    "Description": "ubuntu",
    "DisplayName": "ubuntu",
    "Id": "0000",
    "Name": "Boot0000",
    "UefiDevicePath": "HD(1,GPT,B3515844-B068-48F5-99E5-FC9520D41772,0x800,0x100000)/\\EFI\\ubuntu\\shimx64.efi"
}
```

---

## 3.81 FRU Infomation [sic] (p.272)

Displays the FRU information of mainboard. (OEM resource, not part of DMTF Redfish.)

### 3.81.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/FruInfo
```

### 3.81.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.81.3 Supported Properties

#### Table 107. Supported Properties for FRU Infomation (p.272)

| Property | Type | Read Only | Description |
|---|---|---|---|
| DeviceID | String | True | The ID number of the FRU device. |
| DeviceName | String | True | The name of the FRU device. |
| Chassis | Object | True | This area is used to hold Serial Number, Part Number, and other information about the system chassis. |
| &nbsp;&nbsp;ChassisType | String | True | Indicates the System Enclosure and Chassis type of the chassis. |
| &nbsp;&nbsp;PartNumber | String | True | Indicates the part number of the chassis |
| &nbsp;&nbsp;SerialNumber | String | True | Indicates the serial number of the chassis |
| &nbsp;&nbsp;Manufacturer | String | True | Indicates the manufacturer of the chassis. |
| &nbsp;&nbsp;Version | String | True | Indicates the version of chassis. |
| &nbsp;&nbsp;AssetTag | String | True | Indicates the asset tag of the board. |
| &nbsp;&nbsp;SKU | String | True | Indicates the SKU information of the chassis. |
| &nbsp;&nbsp;Model | String | True | Indicates the model name of the chassis. |
| &nbsp;&nbsp;Info | String | True | Indicates he information of the Chassis object. |
| Board | Object | True | This area provides Serial Number, Part Number, and other information about the board that the FRU Information Device is located on. |
| &nbsp;&nbsp;Manufacturer | String | True | Indicates the nanufacturer of the board. |
| &nbsp;&nbsp;ProductName | String | True | Indicates the product name of the board. |
| &nbsp;&nbsp;SerialNumber | String | True | Indicates the serial number of the board |
| &nbsp;&nbsp;PartNumber | String | True | Indicates the part number of the board |
| &nbsp;&nbsp;Version | String | True | Indicates the version of the board. |
| &nbsp;&nbsp;AssetTag | String | True | Indicates the asset tag of the board. |
| &nbsp;&nbsp;Info | String | True | Indicates the information of the Board object. |
| Product | Object | True | This area is present if the FRU itself is a separate product. |
| &nbsp;&nbsp;Manufacturer | String | True | Indicates the manufacturer of the product. |
| &nbsp;&nbsp;ProductName | String | True | Indicates the product name of the product. |
| &nbsp;&nbsp;PartNumber | String | True | Indicates the part number of the product |
| &nbsp;&nbsp;Version | String | True | Indicates the version of the product. |
| &nbsp;&nbsp;SerialNumber | String | True | Indicates the serial number of the product |
| &nbsp;&nbsp;AssetTag | String | True | Indicates the asset tag of the product. |
| &nbsp;&nbsp;UUID | String | True | Indicates the UUID number of the product. |
| &nbsp;&nbsp;SKU | String | True | Indicates the SKU information of the product. |
| &nbsp;&nbsp;Family | String | True | Indicates the Family information of the product. |
| &nbsp;&nbsp;Info | String | True | Indicates the information of the Product object. |

### 3.81.4 Example

#### 3.81.4.1 GET FRU Information Instance (p.273)

Request:
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/FruInfo
Content-Type: application/json
```

Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#MbFru.MbFru",
    "@odata.etag": "\"1702123810\"",
    "@odata.id": "/redfish/v1/Systems/Self/FruInfo",
    "@odata.type": "#FruInfo.v1_0_0.FruInfo",
    "Board": {
        "Info": "Board FRU Information",
        "Manufacturer": "ASRockRack",
        "PartNumber": "",
        "ProductName": "GENOAD8UD-2T/X550",
        "SerialNumber": "80G400010019"
    },
    "Chassis": {
        "ChassisType": "table: 0x76304250",
        "Info": "Chassis FRU Information",
        "PartNumber": "table: 0x76304278",
        "SerialNumber": "table: 0x763042a0"
    },
    "Description": "Display BMC MotherBoard FRU INFO",
    "Id": "0",
    "Name": "BMC_FRU",
    "Product": {
        "AssetTag": "",
        "Info": "Product FRU Information",
        "Manufacturer": "",
        "PartNumber": "",
        "ProductName": "",
        "SerialNumber": "",
        "UUID": "6698714EBFDE11D301ABA8A159F7E9BC",
        "Version": ""
    }
}
```

> **Caveats:** The `Chassis` fields in this sample hold Lua table addresses (`"table: 0x76304250"`), which is a firmware bug (the Redfish server is Lua-based) that leaks internal pointers instead of values. A proxy should sanitise or ignore them. The `@odata.context` (`#MbFru.MbFru`) does not match the `@odata.type` (`#FruInfo.v1_0_0.FruInfo`). `DeviceID` and `DeviceName` are documented but absent from the sample.

---

**Note:** Section 3.82 is missing. The PDF jumps from 3.81 (p.273) to 3.83 (p.274).

## 3.83 GraphicsControllerCollection (p.274)

It displays the collection of GraphicControllers resources instances available in the system.

### 3.83.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/GraphicsControllers
```

### 3.83.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.83.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.83.4 Example
#### 3.83.4.1 GET GraphicsControllerCollection Instance (p.274)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/GraphicsControllers
Content-Type: application/json
```
Response: "The response of the request will be in JSON format." **The example response box is empty in the PDF.**

## 3.84 GraphicsController (p.275)

It displays the information of Graphic Controller instances available in the system.

### 3.84.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/GraphicsControllers/{GraphicsController-Instance}
```

### 3.84.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.84.3 Supported Properties

#### Table 108. Supported Properties for GraphicsController (p.275)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Links | Object | True | The links to other resources that are related to this resource |
| &nbsp;&nbsp;PCIeDevice | Object | True | A link to the PCIe device that represents this graphics controller. |
| &nbsp;&nbsp;Processors | Array of Object | True | An array of links to the processors that are a part of this graphics controller. |
| &nbsp;&nbsp;Processors@odata.count | Number | True | Number of Processor instance connected that are part of this graphics controller. |
| Location | Object | True | The location of the graphics controller. |
| Manufacturer | String | True | The manufacturer of this graphics controller. |
| Model | String | True | The product model number of this graphics controller. |
| PartNumber | String | True | The part number for this graphics controller. |
| Ports | Object | True | This property shall contain a link to a resource collection of type PortCollection. |
| SKU | String | True | The SKU for this graphics controller. |
| SerialNumber | String | True | The serial number for this graphics controller. |
| SparePartNumber | String | True | The spare part number of the graphics controller. |
| Status | Object | True | The status and health of the resource and its subordinate or dependent resources. |

### 3.84.4 Example
#### 3.84.4.1 GET GraphicsController Instance (p.275)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/GraphicsControllers/{GraphicsController-Instance}
Content-Type: application/json
```
Response: **The example response box is empty in the PDF.**

---

## 3.85 MemoryCollection (p.276)

It displays a list of Memory instances. This represents the collection of Memory resources.

### 3.85.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Memory
```

### 3.85.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.85.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.85.4 Example
#### 3.85.4.1 GET MemoryCollection Instance (p.276-277)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Memory
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#MemoryCollection.MemoryCollection",
    "@odata.etag": "\"1702365017\"",
    "@odata.id": "/redfish/v1/Systems/Self/Memory",
    "@odata.type": "#MemoryCollection.MemoryCollection",
    "Description": "Collection of Memories for this system",
    "Members": [
        { "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM0" },
        { "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM1" },
        { "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM10" },
        { "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM2" },
        { "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM4" },
        { "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM6" },
        { "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM7" },
        { "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM8" }
    ],
    "Members@odata.count": 8,
    "Name": "Memory Collection"
}
```
Note: member IDs follow the pattern `DevType2_DIMM<n>` and are sorted lexically, not numerically.

## 3.86 Memory (p.278)

Displays the information about the Memory devices like DIMM supported by the host connected to the BMC.

### 3.86.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Memory/{Memory-Instnace}
```

### 3.86.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| POST | ConfigureComponents |

### 3.86.3 Supported Properties

#### Table 109. Supported Properties for Memory (p.278-281)

| Property | Type | Read Only | Description |
|---|---|---|---|
| MemoryType | String | True | The Type of Memory. |
| &nbsp;&nbsp;DRAM | - | - | This memory type shall represent volatile DRAM. |
| &nbsp;&nbsp;IntelOptane | - | - | This memory type shall represent an Intel Optane DC Persistent Memory Module. |
| &nbsp;&nbsp;NVDIMM_F | - | - | This memory type shall represent NVDIMM_F as defined by JEDEC. |
| &nbsp;&nbsp;NVDIMM_N | - | - | This memory type shall represent NVDIMM_N as defined by JEDEC. |
| &nbsp;&nbsp;NVDIMM_P | - | - | This memory type shall represent NVDIMM_P as defined by JEDEC. |
| MemoryDeviceType | String | True | Type details of Memory. |
| &nbsp;&nbsp;DDR | - | - | DDR. |
| &nbsp;&nbsp;DDR_SDRAM | - | - | DDR SDRAM. |
| &nbsp;&nbsp;DDR_SGRAM | - | - | DDR SGRAM. |
| &nbsp;&nbsp;DDR2 | - | - | DDR2. |
| &nbsp;&nbsp;DDR2_SDRAM | - | - | DDR2 SDRAM. |
| &nbsp;&nbsp;DDR2_SDRAM_FB_DIMM | - | - | DDR2 SDRAM FB_DIMM. |
| &nbsp;&nbsp;DDR2_SDRAM_FB_DIMM_PROBE | - | - | DDR2 SDRAM FB_DIMM PROBE. |
| &nbsp;&nbsp;DDR3 | - | - | DDR3. |
| &nbsp;&nbsp;DDR3_SDRAM | - | - | DDR3 SDRAM. |
| &nbsp;&nbsp;DDR4 | - | - | DDR4. |
| &nbsp;&nbsp;DDR4_SDRAM | - | - | DDR4 SDRAM. |
| &nbsp;&nbsp;DDR4E_SDRAM | - | - | DDR4E SDRAM. |
| &nbsp;&nbsp;DDR5 | - | - | Double data rate type five synchronous dynamic random-access memory. |
| &nbsp;&nbsp;EDO | - | - | EDO. |
| &nbsp;&nbsp;FastPageMode | - | - | Fast Page Mode. |
| &nbsp;&nbsp;GDDR | - | - | Synchronous graphics random-access memory. |
| &nbsp;&nbsp;GDDR2 | - | - | Double data rate type two synchronous graphics random-access memory. |
| &nbsp;&nbsp;GDDR3 | - | - | Double data rate type three synchronous graphics random-access memory. |
| &nbsp;&nbsp;GDDR4 | - | - | Double data rate type four synchronous graphics random-access memory. |
| &nbsp;&nbsp;GDDR5 | - | - | Double data rate type five synchronous graphics random-access memory. |
| &nbsp;&nbsp;GDDR5X | - | - | Double data rate type five X synchronous graphics random-access memory. |
| &nbsp;&nbsp;GDDR6 | - | - | Double data rate type six synchronous graphics random-access memory. |
| &nbsp;&nbsp;HBM | - | - | High Bandwidth Memory. |
| &nbsp;&nbsp;HBM2 | - | - | The second generation of High Bandwidth Memory. |
| &nbsp;&nbsp;HBM3 | - | - | The third generation of High Bandwidth Memory. |
| &nbsp;&nbsp;Logical | - | - | Logical device, such as when the memory is fabric-attached. |
| &nbsp;&nbsp;LPDDR3_SDRAM | - | - | LPDDR3 SDRAM. |
| &nbsp;&nbsp;LPDDR4_SDRAM | - | - | LPDDR4 SDRAM. |
| &nbsp;&nbsp;OEM | - | - | OEM-defined. |
| &nbsp;&nbsp;PipelinedNibble | - | - | Pipelined Nibble. |
| &nbsp;&nbsp;ROM | - | - | ROM. |
| &nbsp;&nbsp;SDRAM | - | - | SDRAM. |
| BaseModuleType | String | True | The base module type of Memory. |
| &nbsp;&nbsp;Die | - | - | A die within a package. |
| &nbsp;&nbsp;LRDIMM | - | - | Load Reduced. |
| &nbsp;&nbsp;Mini_RDIMM | - | - | Mini_RDIMM. |
| &nbsp;&nbsp;Mini_UDIMM | - | - | Mini_UDIMM. |
| &nbsp;&nbsp;RDIMM | - | - | Registered DIMM. |
| &nbsp;&nbsp;SO_DIMM | - | - | SO_DIMM. |
| &nbsp;&nbsp;SO_DIMM_16b | - | - | SO_DIMM_16b. |
| &nbsp;&nbsp;SO_DIMM_32b | - | - | SO_DIMM_32b. |
| &nbsp;&nbsp;SO_RDIMM_72b | - | - | SO_RDIMM_72b. |
| &nbsp;&nbsp;SO_UDIMM_72b | - | - | SO_UDIMM_72b. |
| &nbsp;&nbsp;UDIMM | - | - | UDIMM. |
| MemoryMedia | Array | True | Media of this memory. |
| &nbsp;&nbsp;DRAM | - | - | DRAM media. |
| &nbsp;&nbsp;Intel3DXPoint | - | - | Intel 3D XPoint media. |
| &nbsp;&nbsp;NAND | - | - | NAND media. |
| &nbsp;&nbsp;Proprietary | - | - | Proprietary media. |
| CapacityMiB | Number | True | The value of this property shall be the Memory capacity in MiB. |
| DataWidthBits | Number | True | The value of this property shall be the bus width in bits. |
| BusWidthBits | Number | True | The value of this property shall be the bus width in bits. |
| Manufacturer | String | True | The manufacturer of the Memory. |
| SerialNumber | String | True | The serial number as provided by the manufacturer of this Memory. |
| PartNumber | String | True | The part number as provided by the manufacturer of this Memory. |
| AllowedSpeedsMHz | Array | True | Speed bins supported by this Memory. |
| Assembly | Object | True | The link to the assembly associated with this DIMM. |
| FirmwareRevision | String | True | Revision of firmware on the Memory controller. |
| FirmwareApiVersion | String | True | Version of API supported by the firmware. |
| FunctionClasses | Array of Items of type String | True | Function classes by the memory device. |
| MaxTDPMilliWatts | Array of Type Number | True | The value of this property shall be the maximum power budgets supported by the Memory in milli Watts. |
| SecurityCapabilities | Object | True | This object shall contain properties which describe the security capabilities of the Memory. |
| &nbsp;&nbsp;PassphraseCapable | Boolean | True | Memory passphrase set capability. |
| &nbsp;&nbsp;MaxPassphraseCount | Number | True | Maximum number of passphrases supported for this Memory. |
| &nbsp;&nbsp;PassphraseLockLimit | Number | True | Maximum number of incorrect passphrase attempts allowed before memory is locked. |
| &nbsp;&nbsp;ConfigurationLockCapable | Boolean | True | Support for locking the configuration. |
| &nbsp;&nbsp;DataLockCapable | Boolean | True | Support for data locking. |
| SpareDeviceCount | Number | True | The value of this property shall be the number of unused spare devices available in the Memory. |
| ConfigurationLocked | Boolean | True | Indicates that the configuration of this memory has been locked. |
| RankCount | Number | True | The value of this property shall be number of ranks available in the Memory. |
| MemoryLocation | object | - | Memory connection information to sockets and memory controllers. |
| &nbsp;&nbsp;Socket | Number | True | Socket number in which Memory is connected. |
| &nbsp;&nbsp;MemoryController | Number | True | Memory controller number in which Memory is connected. |
| &nbsp;&nbsp;Channel | Number | True | Channel number in which Memory is connected. |
| &nbsp;&nbsp;Slot | Number | True | Slot number in which Memory is connected. |
| ErrorCorrection | String | True | The value of this property shall be the error correction scheme supported for this memory. |
| &nbsp;&nbsp;NoECC | - | - | No ECC available. |
| &nbsp;&nbsp;SingleBitECC | - | - | Single bit Data error can be corrected by ECC. |
| &nbsp;&nbsp;MultiBitECC | - | - | Multi-bit Data errors can be corrected by ECC. |
| &nbsp;&nbsp;AddressParity | - | - | Address Parity errors can be corrected. |
| OperatingSpeedMhz | Number | True | Operating speed of Memory in MHz. |
| VolatileRegionSizeLimitMiB | Number | True | The value of this property shall be the total size of volatile regions in MiB. |
| PersistentRegionSizeLimitMiB | Number | True | The value of this property shall be the total size of persistent regions in MiB |
| Regions | Array | - | Memory regions information within the memory device. |
| &nbsp;&nbsp;RegionId | String | True | Unique region ID representing a specific region within the Memory |
| &nbsp;&nbsp;OffsetMiB | Number | True | Offset with in the Memory that corresponds to the starting of this memory region in MiB. |
| &nbsp;&nbsp;PassphraseEnabled | Number | True | The value of this property shall be a boolean indicating if the passphrase is enabled for this region. (Type column says Number; description says boolean.) |
| &nbsp;&nbsp;SizeMiB | Number | True | Size of this memory region in MiB. |
| &nbsp;&nbsp;MemoryClassification | String | True | The classification of memory that the memory region occupies. |
| &nbsp;&nbsp;&nbsp;&nbsp;Volatile | - | - | Volatile memory. |
| &nbsp;&nbsp;&nbsp;&nbsp;ByteAccessiblePersistent | - | - | Byte-accessible persistent memory. |
| &nbsp;&nbsp;&nbsp;&nbsp;Block | - | - | Block-accesible memory. |
| PowerManagementPolicy | Object | - | This object shall contain properties which describe the power management policy for the current resource. |
| &nbsp;&nbsp;PolicyEnabled | Boolean | True | Power management policy enabled status. |
| &nbsp;&nbsp;MaxTDPMilliWatts | Number | True | Maximum TDP in milli watts. |
| &nbsp;&nbsp;PeakPowerBudgetMilliWatts | Number | True | Peak power budget in milli watts. Unit is mW. |
| &nbsp;&nbsp;AveragePowerBudgetMilliWatts | Number | True | Average power budget in milli watts. Unit is mW. |
| IsSpareDeviceEnabled | Boolean | True | Spare device enabled status. |
| IsRankSpareEnabled | Boolean | True | Rank spare enabled status. |
| VolatileRegionNumberLimit | Number | True | Total number of volatile regions this Memory can support. |
| PersistentRegionNumberLimit | Number | True | Total number of persistent regions this Memory can support. |
| VolatileRegionSizeMaxMiB | Number | True | Maximum size of a single volatile region in MiB. |
| PersistentRegionSizeMaxMiB | Number | True | Maximum size of a single persistent region in MiB. |
| AllocationIncrementMiB | Number | True | The size of the smallest unit of allocation for a memory region, thus it is the multiple in which regions are actually reserved. |
| AllocationAlignmentMiB | Number | True | The boundary which memory regions are allocated on, measured in MiB. |
| Links | Object | - | Contains references to other resources that are related to this resource. |
| &nbsp;&nbsp;Oem | Object | - | This property shall contain the OEM extensions. |
| &nbsp;&nbsp;Chassis | Array | True | A reference to the Chassis which contains this Memory. |
| Status | Object | - | The status and health of the resource and its subordinate or dependent resources. |
| &nbsp;&nbsp;Oem | Object | - | This property shall contain the OEM extensions. |
| &nbsp;&nbsp;OriginOfCondition | Object | True | This is the URI of the resource that caused the log entry. |
| ModuleManufacturerID | String | True | The value of this property shall be the two bytes manufacturer ID of this memory module as defined by JEDEC in JEP-106. |
| ModuleProductID | String | True | The value of this property shall be the two bytes product ID of this memory module as defined by the manufacturer. |
| MemorySubsystemControllerManufacturerID | String | True | The value of this property shall be the two bytes product ID of the memory subsystem controller of this memory module as defined by the manufacturer. |
| VolatileSizeMiB | Number | True | The value of this property shall be the total size of the volatile portion memory in MiB. |
| NonVolatileSizeMiB | Number | True | The value of this property shall be the maximum size of a single volatile regions in MiB. |
| CacheSizeMiB | Number | True | The value of this property shall be the total size of the cache portion memory in MiB. |
| LogicalSizeMiB | Number | True | The value of this property shall be the total size of the logical memory in MiB. |
| Location | Array | True | The location of the memory device. |
| Metrics | Object | - | This property will have reference to the MemoryMetrics Resource which is populated under this MemoryInstance. |

(The row `VolatileRegionSizeMaxMiB` is split across p.280/p.281.)

#### Table 110. Security States (p.281)

| Enum | Description |
|---|---|
| Enabled | Secure mode is enabled. |
| Disabled | Secure mode is disabled. |
| Unlocked | Secure mode is enabled and access to the data is unlocked. |
| Locked | Secure mode is enabled and access to the data is locked. |
| Frozen | Secure state is frozen and cannot be modified until reset. |
| Passphraselimit | Number of attempts to unlock the Memory exceeded limit. |
| Volatile | Volatile Memory. |
| ByteAccessiblePersistent | Byte accessible persistent memory. |
| Block | Block accesible memory. |

### 3.86.4 Example

#### 3.86.4.1 GET Memory Instance (p.282-283)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Memory/{Memory-Instnace}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#Memory.Memory",
    "@odata.etag": "\"1702283684\"",
    "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM0",
    "@odata.type": "#Memory.v1_11_0.Memory",
    "Actions": {
        "Oem": {
            "#AmiBios.ChangeState": {
                "@Redfish.ActionInfo": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM0/ChangeStateActionInfo",
                "target": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM0/Actions/AmiBios.ChangeState"
            }
        }
    },
    "AllowedSpeedsMHz": [
        4800
    ],
    "BaseModuleType": "RDIMM",
    "BusWidthBits": 80,
    "CacheSizeMiB": 0,
    "CapacityMiB": 32768,
    "DataWidthBits": 64,
    "FirmwareRevision": "Unknown",
    "Id": "DevType2_DIMM0",
    "Links": {
        "Chassis": {
            "@odata.id": "/redfish/v1/Chassis/Self"
        }
    },
    "LogicalSizeMiB": 0,
    "Manufacturer": "Micron Technology",
    "MemoryDeviceType": "DDR5",
    "MemoryLocation": {
        "Channel": 0,
        "MemoryController": 0,
        "Slot": 0,
        "Socket": 0
    },
    "MemoryMedia": [
        "DRAM"
    ],
    "MemorySubsystemControllerManufacturerID": "0x0000",
    "MemorySubsystemControllerProductID": "0x0000",
    "MemoryType": "NVDIMM_P",
    "ModuleManufacturerID": "0x2C80",
    "ModuleProductID": "0x0000",
    "Name": "DDR5_A1",
    "NonVolatileSizeMiB": 0,
    "Oem": {
        "AmiBios": {
            "@odata.type": "#AmiBios.v1_0_0.AmiBios"
        }
    },
    "OperatingMemoryModes": [
        "Volatile"
    ],
    "OperatingSpeedMhz": 4800,
    "PartNumber": "MTC20F1045S1RC48BAX ",
    "PersistentRegionNumberLimit": 0,
    "PersistentRegionSizeLimitMiB": 0,
    "PersistentRegionSizeMaxMiB": 0,
    "RankCount": 1,
    "Regions": [
        {
            "OffsetMiB": 0,
            "RegionId": "0",
            "SizeMiB": 33554431
        }
    ],
    "SecurityCapabilities": {
        "ConfigurationLockCapable": false,
        "DataLockCapable": false,
        "PassphraseCapable": false
    },
    "SerialNumber": "2D58CC98",
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "VolatileRegionNumberLimit": 1,
    "VolatileRegionSizeLimitMiB": 32768,
    "VolatileRegionSizeMaxMiB": 32768,
    "VolatileSizeMiB": 32768
}
```

> **Caveats (differ from DMTF):** `Links.Chassis` is a single object reference here (consistent with DMTF Memory v1_11), but Table 109 calls it an Array. `MemoryType` reports `NVDIMM_P` for a plain DDR5 RDIMM (a firmware or BIOS data bug). `Regions[0].SizeMiB` is `33554431` (0x1FFFFFF), a nonsense value. `PartNumber` has a trailing space. `OperatingSpeedMhz` uses the DMTF spelling (lower-case "hz").

#### 3.86.4.2 POST Memory Instance (OEM action `AmiBios.ChangeState`) (p.283-284)
```
POST https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Memory/{Memory-Instnace}/Actions/AmiBios.ChangeState
Content-Type: application/json
```
Example request body:
```json
{
    "State": "Enabled"
}
```
Response: status **204** with no body.

OEM action: `#AmiBios.ChangeState`, parameter `State` (example `"Enabled"`; allowable values come from `.../ChangeStateActionInfo`, which is not shown). Requires `ConfigureComponents`. It presumably enables or disables a DIMM through the BIOS on the next boot.

---

## 3.87 MemoryDomainCollection (p.285)

It displays a list of Memory instances. This represents the collection of Memory resources. [sic: this describes MemoryDomains]

### 3.87.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/MemoryDomains
```
### 3.87.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.87.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.87.4 Example
#### 3.87.4.1 GET MemoryDomainCollection Instance (p.285-286)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/MemoryDomains
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#MemoryDomainCollection.MemoryDomainCollection",
    "@odata.etag": "\"1702709659\"",
    "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains",
    "@odata.type": "#MemoryDomainCollection.MemoryDomainCollection",
    "Description": "Collection of MemoryDomains",
    "Members": [
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_0" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_10" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_11" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_1" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_2" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_3" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_4" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_5" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_6" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_7" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_8" },
        { "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_9" }
    ],
    "Members@odata.count": 12,
    "Name": "MemoryDomain Collection"
}
```

## 3.88 MemoryDomain (p.287)

Displays the information about the Memory devices like DIMM supported by the host connected to the BMC.

### 3.88.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/MemoryDomains/{MemoryDomains-Instance}
```
### 3.88.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.88.3 Supported Properties

#### Table 111. Supported Properties for MemoryDomain (p.287)

| Property | Type | Read Only | Description |
|---|---|---|---|
| AllowsMemoryChunkCreation | Boolean | True | Indicates if this Memory Domain supports the creation of Memory Chunks. |
| AllowsBlockProvisioning | Boolean | True | Indicates if this Memory Domain supports the provisioning of blocks of memory. |
| InterleavableMemorySets | Array | - | This is the interleave sets for the memory chunk. |
| &nbsp;&nbsp;MemorySet | Array of Objects | True | This is the collection of memory for a particular interleave set |
| &nbsp;&nbsp;MemorySet@odata.count | Number | True | Count of MemorySets |
| MemoryChunks | Object | True | A reference to the collection of Memory Chunks associated with this Memory Domain. |
| AllowsMirroring | Boolean | True | Indicates if this Memory Domain supports the creation of Memory Chunks with mirroring enabled. |
| AllowsSparing | Boolean | True | Indicates if this Memory Domain supports the creation of Memory Chunks with sparing enabled. |
| Actions | Object | True | This object will contain the actions for this resource under Oem property if any. |

### 3.88.4 Example
#### 3.88.4.1 GET MemoryDomain Instance (p.287-288)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/MemoryDomains/{MemoryDomains-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#MemoryDomain.MemoryDomain",
    "@odata.etag": "\"1702708587\"",
    "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_0",
    "@odata.type": "#MemoryDomain.v1_2_3.MemoryDomain",
    "AllowsBlockProvisioning": false,
    "AllowsMemoryChunkCreation": false,
    "AllowsMirroring": false,
    "AllowsSparing": false,
    "Id": "MemoryDomain_SystemMemory_0",
    "InterleavableMemorySets": [
        {
            "MemorySet": [
                {
                    "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM0"
                }
            ],
            "MemorySet@odata.count": 1
        }
    ],
    "MemoryChunks": {
        "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_0/MemoryChunks"
    },
    "Name": "MemoryDomain_SystemMemory_0"
}
```

## 3.89 MemoryChunksCollection (p.289)

It displays a list of Memory instances. This represents the collection of Memory resources. [sic]

### 3.89.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/MemoryDomains/{MemoryDomains-Instance}/MemoryChunks
```
### 3.89.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.89.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.89.4 Example
#### 3.89.4.1 GET MemoryChunksCollection Instance (p.289)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/MemoryDomains/{MemoryDomains-Instance}/MemoryChunks
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#MemoryChunksCollection.MemoryChunksCollection",
    "@odata.etag": "\"1702709659\"",
    "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_0/MemoryChunks",
    "@odata.type": "#MemoryChunksCollection.MemoryChunksCollection",
    "Description": "Collection of MemoryChunks",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_0/MemoryChunks/MemoryDomain_SystemMemory_0_Chunk_0"
        }
    ],
    "Members@odata.count": 1,
    "Name": "MemoryChunks Collection"
}
```

## 3.90 MemoryChunks (p.290)

The schema definition of a memory chunk and its configuration. This resource shall represent memory chunks and interleave sets in a Redfish implementation.

### 3.90.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/MemoryDomains/{MemoryDomains-Instance}/MemoryChunks/{MemoryChunks-Instance}
```
### 3.90.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.90.3 Supported Properties

#### Table 112. Supported Properties for MemoryChunks (p.290)

| Property | Type | Read Only | Description |
|---|---|---|---|
| MemoryChunkSizeMiB | Number | True | Size of the memory chunk in MiB. |
| AddressRangeType | String | True | Memory type of this memory chunk. |
| &nbsp;&nbsp;Volatile | - | - | Volatile memory. |
| &nbsp;&nbsp;PMEM | - | - | Byte accessible persistent memory. |
| &nbsp;&nbsp;Block | - | - | Block accessible memory. |
| InterleaveSets | Array | - | The interleave sets for the memory chunk. |
| &nbsp;&nbsp;RegionId | String | True | DIMM region identifier. |
| &nbsp;&nbsp;OffsetMiB | Number | True | Off set within the DIMM that corresponds to the start of this memory region, measured in mebibytes (MiB). |
| &nbsp;&nbsp;SizeMiB | Number | True | Size of this memory region measured in mebibytes (MiB). |
| &nbsp;&nbsp;MemoryLevel | Number | True | Level of the interleave set for multi-level tiered memory. |
| &nbsp;&nbsp;Memory | Objects | True | Describes a memory device of the interleave set. |
| IsSpare | Boolean | True | Spare enabled status. |
| IsMirrorEnabled | Boolean | True | Mirror Enabled status. |
| Actions | Object | True | This object will contain the actions for this resource under Oem property if any. |
| AddressRangeOffsetMiB | Number | True | Offset of the memory chunk in the address range in MiB. |
| DisplayName | String | True | A user-configurable string to name the memory chunk |

### 3.90.4 Example
#### 3.90.4.1 GET MemoryChunks Instnace (p.290-291)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/MemoryDomains/{MemoryDomains-Instance}/MemoryChunks/{MemoryChunks-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#MemoryChunks.MemoryChunks",
    "@odata.etag": "\"1702708587\"",
    "@odata.id": "/redfish/v1/Systems/Self/MemoryDomains/MemoryDomain_SystemMemory_0/MemoryChunks/MemoryDomain_SystemMemory_0_Chunk_0",
    "@odata.type": "#MemoryChunks.v1_4_0.MemoryChunks",
    "AddressRangeType": "Volatile",
    "Id": "MemoryDomain_SystemMemory_0_Chunk_0",
    "InterleaveSets": [
        {
            "Memory": {
                "@odata.id": "/redfish/v1/Systems/Self/Memory/DevType2_DIMM0"
            },
            "MemoryLevel": 0,
            "RegionId": "MemoryDomain_SystemMemory_0_Chunk_0_Volatile_0",
            "SizeMiB": 32768
        }
    ],
    "IsMirrorEnabled": false,
    "IsSpare": false,
    "MemoryChunkSizeMiB": 32768,
    "Name": "MemoryDomain_SystemMemory_0_Chunk_0"
}
```

---

## 3.91 NetworkInterfaceCollection (p.292)

It displays the collection of network interface resource instances available in the system.

### 3.91.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/NetworkInterfaces
```
### 3.91.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.91.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.91.4 Example
#### 3.91.4.1 GET NetworkInterfaceCollection Instance (p.292)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/NetworkInterfaces
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#NetworkInterfaceCollection.NetworkInterfaceCollection",
    "@odata.etag": "\"1702715060\"",
    "@odata.id": "/redfish/v1/Systems/Self/NetworkInterfaces",
    "@odata.type": "#NetworkInterfaceCollection.NetworkInterfaceCollection",
    "Description": "The Collection for Network Interfaces",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Systems/Self/NetworkInterfaces/DevType7_NIC0"
        }
    ],
    "Members@odata.count": 1,
    "Name": "NetworkInterface Collection"
}
```

## 3.92 NetworkInterface (p.293)

A NetworkInterface contains references linking NetworkAdapter, and NetworkDeviceFunction resources and represents the functionality available to the containing system.

### 3.92.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/NetworkInterfaces/{NetworkInterfaces-Instance}
```
### 3.92.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.92.3 Supported Properties

#### Table 113. Supported Properties for NetworkInterface (p.293)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Status | object | True | The status and health of the resource and its subordinate or dependent resources. |
| Links | object | True | The links to other resources that are related to this resource. |
| &nbsp;&nbsp;NetworkAdapter | object | True | The link to the network adapter that contains this network interface. |
| NetworkDeviceFunctions | object | True | The link to the network device functions associated with this network interface. |
| Actions | object | True | This object will contain the actions for this resource under OEM property if any. |

### 3.92.4 Example
#### 3.92.4.1 GET NetworkInterface Instance (p.293-294)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/NetworkInterfaces/{NetworkInterfaces-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#NetworkInterface.NetworkInterface",
    "@odata.etag": "\"1702708587\"",
    "@odata.id": "/redfish/v1/Systems/Self/NetworkInterfaces/DevType7_NIC0",
    "@odata.type": "#NetworkInterface.v1_2_0.NetworkInterface",
    "Description": "NetworkInterface instance",
    "Id": "DevType7_NIC0",
    "Links": {
        "NetworkAdapter": {
            "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0"
        }
    },
    "Name": "DevType7_NIC0",
    "NetworkDeviceFunctions": {
        "@odata.id": "/redfish/v1/Systems/Self/NetworkInterfaces/DevType7_NIC0/NetworkDeviceFunctions"
    },
    "Ports": {
        "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/Ports"
    },
    "Status": {
        "Health": "OK",
        "State": "Disabled"
    }
}
```
Note: `Ports` appears in the response but not in Table 113. It points into the Chassis tree.

---

## 3.93 ProcessorCollection (p.295)

It displays a list of Processor instances in the ComputerSystem (Host).

### 3.93.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Processors
/redfish/v1/Systems/{Systems-Instance}/Processors/{Processors-Instance}/SubProcessors
```
### 3.93.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.93.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.93.4 Example
#### 3.93.4.1 GET ProcessorCollection Instance (p.295)
```
GET hptts://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Processors
Content-Type: application/json
```
(The "hptts" typo is in the source.)

Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#ProcessorCollection.ProcessorCollection",
    "@odata.etag": "\"1702720459\"",
    "@odata.id": "/redfish/v1/Systems/Self/Processors",
    "@odata.type": "#ProcessorCollection.ProcessorCollection",
    "Description": "Collection of processors",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0"
        }
    ],
    "Members@odata.count": 1,
    "Name": "Processors Collection"
}
```

## 3.94 Processor (p.296)

This is the schema definition for the Processor resource. It represents the properties of a processor attached to a System.

### 3.94.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Processors/{Processors-Instance}
/redfish/v1/Systems/{Systems-Instance}/Processors/{Processors-Instance}/SubProcessors/{SubProcessors-Instance}
```
### 3.94.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.94.3 Supported Properties

#### Table 114. Supported Properties for Processor (p.296-298)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Socket | String | True | Identifies the physical location or socket of the processor. |
| ProcessorType | String | True | Identifies the type of processor contained in this Socket. |
| &nbsp;&nbsp;CPU | - | - | A Central Processing Unit. |
| &nbsp;&nbsp;GPU | - | - | A Graphics Processing Unit. |
| &nbsp;&nbsp;FPGA | - | - | A Field Programmable Gate Array. |
| &nbsp;&nbsp;DSP | - | - | A Digital Signal Processor. |
| &nbsp;&nbsp;Accelerator | - | - | An Accelerator |
| &nbsp;&nbsp;OEM | - | - | An OEM-defined Processing Unit. |
| &nbsp;&nbsp;Core | - | - | A Core in a Processor. |
| &nbsp;&nbsp;Thread | - | - | A Thread in a Processor. |
| ProcessorArchitecture | String | True | Identifies the architecture of the processor contained in this Socket |
| &nbsp;&nbsp;x86 | - | - | x86 or x86-64. |
| &nbsp;&nbsp;IA-64 | - | - | Intel Itanium. |
| &nbsp;&nbsp;ARM | - | - | ARM. |
| &nbsp;&nbsp;MIPS | - | - | MIPS. |
| &nbsp;&nbsp;OEM | - | - | OEM-defined |
| InstructionSet | String | True | This property shall contain the string which identifies the instruction set of the processor contained in this socket. |
| &nbsp;&nbsp;x86 | - | - | x86 32-bit. |
| &nbsp;&nbsp;x86-64 | - | - | x86 64-bit. |
| &nbsp;&nbsp;IA-64 | - | - | Intel IA-64. |
| &nbsp;&nbsp;ARM-A32 | - | - | ARM 32-bit. |
| &nbsp;&nbsp;ARM-A64 | - | - | ARM 64-bit. |
| &nbsp;&nbsp;MIPS32 | - | - | MIPS 32-bit. |
| &nbsp;&nbsp;MIPS64 | - | - | MIPS 64-bit. |
| &nbsp;&nbsp;OEM | - | - | OEM-defined. |
| ProcessorId | Object | - | This object shall contain identification information for this processor. |
| &nbsp;&nbsp;VendorId | String | True | This property shall indicate the Vendor Identification string information as provided by the manufacturer of this processor. |
| &nbsp;&nbsp;IdentificationRegisters | String | True | The contents of the Identification Registers (CPUID) for this processor. |
| &nbsp;&nbsp;EffectiveFamily | String | True | The effective Family for this processor. |
| &nbsp;&nbsp;EffectiveModel | String | True | This property shall indicate the effective Model information as provided by the manufacturer of this processor. |
| &nbsp;&nbsp;ProtectedIdentificationNumber | String | True | The Protected Processor Identification Number (PPIN) for this processor. |
| &nbsp;&nbsp;Step | String | True | This property shall indicate the Step or revision string information as provided by the manufacturer of this processor. |
| &nbsp;&nbsp;MicrocodeInfo | String | True | This property shall indicate the Microcode Information as provided by the manufacturer of this processor. |
| Manufacturer | String | True | The manufacturer of the processor. |
| OperatorConfigs [sic, = OperatingConfigs] | Object | True | The link to collection of OperatingConfigs under this porcessors. |
| AppliedOperatorConfig [sic, = AppliedOperatingConfig] | Object | True | The link to currently applied OperatingConfig. |
| MaxSpeedMHz | Number | True | The maximum clock speed of the processor. |
| Model | String | True | This property shall indicate the model information as provided by the manufacturer of this processor. |
| PartNumber | String | True | This property indicates a part number. |
| SerialNumber | String | True | The serial number of the processor. |
| Version | String | True | The hardware version of the processor. |
| TotalCores | Number | True | The total count of independent processor cores contained within this processor. |
| TotalThreads | Number | True | The total count of independent execution threads supported by this processor. |
| Links | Object | True | The links to other resources that are related to this resource. |
| &nbsp;&nbsp;Oem | Object | False | *(no description)* |
| &nbsp;&nbsp;ConnectedProcessors | Array | True | An array of links to the processors directly connected to this processor. |
| &nbsp;&nbsp;ConnectedProcessors@odata.count | Number | True | The count of Processors directly connected to this processor. |
| &nbsp;&nbsp;Chassis | Array | True | The value of this property shall be a reference to a resource of type Chassis that represent the physical container associated with this Processor. |
| Actions | Object | True | This object will contain the actions for this resource under Oem property if any. |
| SubProcessors | Object | True | The value of this property shall be a link to a collection of type ProcessorCollection. |
| Location | Object | True | The location of a resource. |
| AccelerationFunctions | Object | True | A reference to the collection of Acceleration Functions associated with this Processor. |
| Assembly | Object | True | A reference to the Assembly resourec associated with this Processor. |
| MaxTDPWatts | Number | True | The maximum Thermal Design Power (TDP) in watts. |
| Metrics | Object | True | A reference to the Metrics associated with this Processor. |
| TDPWatts | Number | True | The nominal Thermal Design Power (TDP) in watts. |
| TotalEnabledCores | Number | True | The total number of enabled cores contained in this processor. |
| UUID | String | True | The universal unique identifier (UUID) for this processor. |
| FPGA | Object | True | The properties specific for Processors of type FPGA. |
| ProcessorMemory | Array | True | The memory directly attached or integrated witin this Procesor. |
| &nbsp;&nbsp;CapacityMiB | Number | True | The memory capacity in MiB. |
| &nbsp;&nbsp;IntegratedMemory | Boolean | True | This indicates whether this memory is integrated within the Processor. |
| &nbsp;&nbsp;MemoryType | String | True | The type of memory used by this processor. Refer to the table Enum values of Memory Type. |
| &nbsp;&nbsp;SpeedMHz | Number | True | The operating speed of the memory in MHz. |

#### Table 115. Property of FPGA (p.298)

| Property | Type | Read Only | Description |
|---|---|---|---|
| FirmwareId | String | True | The value of this property shall contain a string describing the FPGA firmware identifier. |
| FirmwareManufacturer | String | True | The FPGA firmware manufacturer. |
| FirmwareVersion | String | True | The FPGA firmware version. |
| FpgaType | String | True | The value of this property shall be a type of the FPGA device. |
| &nbsp;&nbsp;Discrete | - | - | The discrete FPGA device. |
| &nbsp;&nbsp;Integrated | - | - | The FPGA device integrated with others processor in the single chip. |
| Model | String | True | The value of this property shall be a model of the FPGA device. |
| PCIeVirtualFunctions | Number | True | The number of the PCIe Virtual Functions. |
| ProgrammableFromHost | Boolean | True | This flag indicates if the FPGA firmware can be reprogrammed from the host using system software. |
| ReconfigurationSlots | Array | True | An array of the FPGA reconfiguration slots. |
| &nbsp;&nbsp;AccelerationFunction | Object | True | A link to the Acceleration Function provided by the code programmed into a reconfiguration slot. |
| &nbsp;&nbsp;ProgrammableFromHost | Boolean | True | This flag indicates if the reconfiguration slot can be reprogrammed from the host using system software. |
| &nbsp;&nbsp;SlotId | String | True | The FPGA reconfiguration slot identifier. |
| &nbsp;&nbsp;UUID | String | True | The universal unique identifier (UUID) for this reconfiguration slot. |
| HostInterface | Object | True | The FPGA interface to the host. Refer to the table Property of FPGA Interface. |
| ExternalInterfaces | Array | True | An array of the FPGA external interfaces. Refer to the table Property of FPGA Interface. |

#### Table 116. Property of FPGA Interface (p.298)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Ethernet | Object | True | Describes the Ethernet related information about this FPGA interface. |
| &nbsp;&nbsp;MaxLanes | Number | True | This is the number of lanes supported by this interface. |
| &nbsp;&nbsp;MaxSpeedMbps | Number | True | The maximum speed supported by this interface. |
| InterfaceType | String | True | The FPGA interface type. |
| &nbsp;&nbsp;Ethernet | - | - | An Ethernet Interface. |
| &nbsp;&nbsp;OEM | - | - | An OEM-defined interface. |
| &nbsp;&nbsp;PCIe | - | - | A PCI Express interface. |
| &nbsp;&nbsp;QPI | - | - | The Intel QuickPath Interconnect. |
| &nbsp;&nbsp;UPI | - | - | The Intel UltraPath Interconnect. |
| PCIe | Object | True | Describes the PCI-e related information about this FPGA interface. Refer to the table PCIeInterface Properties. |

#### Table 117. Enum values of Memory Type (p.298-299)

| Property | Description |
|---|---|
| DDR | Double data rate synchronous dynamic random-access memory. |
| DDR2 | Double data rate type two synchronous dynamic random-access memory. |
| DDR3 | Double data rate type three synchronous dynamic random-access memory. |
| DDR4 | Double data rate type four synchronous dynamic random-access memory. |
| DDR5 | Double data rate type five synchronous dynamic random-access memory. |
| Flash | Flash memory. |
| GDDR | Synchronous graphics random-access memory. |
| GDDR2 | Double data rate type two synchronous graphics random-access memory. |
| GDDR3 | Double data rate type three synchronous graphics random-access memory. |
| GDDR4 | Double data rate type four synchronous graphics random-access memory. |
| GDDR5 | Double data rate type five synchronous graphics random-access memory. |
| GDDR5X | Double data rate type five X synchronous graphics random-access memory. |
| GDDR6 | Double data rate type six synchronous graphics random-access memory. |
| HBM1 | High Bandwidth Memory. |
| HBM2 | The second generation of High Bandwidth Memory. |
| HBM3 | The third generation of High Bandwidth Memory. |
| L1Cache | L1 cache. |
| L2Cache | L2 cache. |
| L3Cache | L3 cache. |
| L4Cache | L4 cache. |
| L5Cache | L5 cache. |
| L6Cache | L6 cache. |
| L7Cache | L7 cache. |
| OEM | OEM-defined. |
| SDRAM | Synchronous dynamic random-access memory. |
| SGRAM | Synchronous graphics RAM. |
| SRAM | Static random-access memory. |

### 3.94.4 Example
#### 3.94.4.1 GET Processor Instance (p.299-301)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Processors/{Processors-Instance}
Content-Type: application/json
```
Response (the source itself elides part of the core list with `...`):
```json
{
    "@odata.context": "/redfish/v1/$metadata#Processor.Processor",
    "@odata.etag": "\"1702725861\"",
    "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0",
    "@odata.type": "#Processor.v1_10_0.Processor",
    "AccelerationFunctions": {
        "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/AccelerationFunctions"
    },
    "Id": "DevType1_CPU0",
    "Links": {
        "Chassis": {
            "@odata.id": "/redfish/v1/Chassis/Self"
        },
        "ConnectedProcessors": [
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core0" },
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core1" },
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core10" },
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core11" },
            ...
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core9" },
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core90" },
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core91" },
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core92" },
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core93" },
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core94" },
            { "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors/DevType1_CPU0_Core95" }
        ],
        "ConnectedProcessors@odata.count": 96
    },
    "Manufacturer": "Advanced Micro Devices, Inc.",
    "MaxSpeedMHz": 3700,
    "Model": "17",
    "Name": "CPU1",
    "ProcessorArchitecture": "x86",
    "ProcessorId": {
        "EffectiveFamily": "AMD EPYC 9654 96-Core Processor                "
    },
    "ProcessorMemory": [
        {
            "CapacityMiB": 6,
            "IntegratedMemory": true,
            "MemoryType": "L1Cache"
        },
        {
            "CapacityMiB": 96,
            "IntegratedMemory": true,
            "MemoryType": "L2Cache"
        },
        {
            "CapacityMiB": 384,
            "IntegratedMemory": true,
            "MemoryType": "L3Cache"
        }
    ],
    "ProcessorType": "CPU",
    "Socket": "P0",
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "SubProcessors": {
        "@odata.id": "/redfish/v1/Systems/Self/Processors/DevType1_CPU0/SubProcessors"
    },
    "TotalCores": 96,
    "TotalEnabledCores": 96,
    "TotalThreads": 192
}
```

> **Caveats:** `ProcessorId.EffectiveFamily` holds the brand string padded with trailing spaces, not a family number as DMTF intends. `Model` is `"17"`, a numeric family or model value, not the model name. `Links.Chassis` is a single object although Table 114 calls it an Array. `ConnectedProcessors` lists per-core SubProcessors (`DevType1_CPU0_Core<n>`, 96 entries, sorted lexically) rather than other sockets. `Name` is `CPU1` while `Id` is `DevType1_CPU0`.

---

## 3.95 Systems Instance FutureState (SD) (p.302)

Displays the information about the Boot Properties. This is the pending-settings (`@Redfish.Settings`) object for ComputerSystem.

### 3.95.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/SD
```

### 3.95.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureComponents |

### 3.95.3 Supported Properties
Please refer to section "Boot Properties" (Table 103, ComputerSystem Boot) for the supported properties.

### 3.95.4 Example

#### 3.95.4.1 GET Systems Instance FutureState (SD) Instance (p.302-303)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SD
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#ComputerSystem.ComputerSystem",
    "@odata.etag": "\"1702725861\"",
    "@odata.id": "/redfish/v1/Systems/Self/SD",
    "@odata.type": "#ComputerSystem.v1_15_0.ComputerSystem",
    "AssetTag": "",
    "Boot": {
        "AliasBootOrder": [],
        "BootNext": null,
        "BootOrder": [
            "Boot0002",
            "Boot0003",
            "Boot0004",
            "Boot0005",
            "Boot0006",
            "Boot0007",
            "Boot0008",
            "Boot0009",
            "Boot0001",
            "Boot0000"
        ],
        "BootOrderPropertySelection": "BootOrder",
        "BootSourceOverrideEnabled": "Disabled",
        "BootSourceOverrideEnabled@Redfish.AllowableValues": [
            "Disabled",
            "Once",
            "Continuous"
        ],
        "BootSourceOverrideMode": null,
        "BootSourceOverrideMode@Redfish.AllowableValues": [
            "Legacy",
            "UEFI"
        ],
        "BootSourceOverrideTarget": null,
        "BootSourceOverrideTarget@Redfish.AllowableValues": [
            "None",
            "Pxe",
            "Floppy",
            "Cd",
            "Usb",
            "Hdd",
            "BiosSetup",
            "Utilities",
            "Diags",
            "UefiShell",
            "UefiTarget",
            "SDCard",
            "UefiHttp",
            "RemoteDrive",
            "UefiBootNext"
        ],
        "HttpBootUri": null,
        "UefiTargetBootSourceOverride": null
    },
    "Description": "System Self",
    "Id": "Self",
    "Name": "System",
    "SystemType": "Physical"
}
```

#### 3.95.4.2 PATCH Systems Instance FutureState (SD) Instance (p.303-304)

> **Note (from the source):** *A PATCH request for Boot properties is blocked when the FixedBootOrder feature is enabled in the BIOS. This is handled with an internal flag that is populated from the BIOS. Refer to the Appendix FixedBootOrder.* (See section 4.1 below, p.378-381.)

Request:
```
PATCH https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SD
Content-Type: application/json
```
Example request body:
```json
{
    "Boot": {
        "BootOrder": [
            "Boot0001",
            "Boot0002",
            "Boot0003",
            "Boot0004",
            "Boot0005",
            "Boot0006",
            "Boot0007",
            "Boot0008",
            "Boot0009",
            "Boot0000",
            "Boot000A"
        ]
    }
}
```
Response: status **204** with no body.

> **Implementation note:** On this firmware, boot-order and boot-override changes are written to `/redfish/v1/Systems/Self/SD` (the FutureState object, applied by the BIOS on the next boot), not by PATCHing `/redfish/v1/Systems/Self` directly. The `@Redfish.AllowableValues` annotations for the Boot enums appear only on the SD resource.

---

## 3.96 SecureBoot (p.305)

This resource contains UEFI Secure Boot information. It represents properties for managing the UEFI Secure Boot functionality of a system.

### 3.96.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/SecureBoot
```
### 3.96.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| PATCH (written "PATHC" in the source) | ConfigureComponents |

(Source lists no POST privilege for the `SecureBoot.ResetKeys` action, although the action is advertised.)

### 3.96.3 Supported Properties

#### Table 118. Supported Properties for SecureBoot (p.305)

| Property | Type | Read Only | Description |
|---|---|---|---|
| SecureBootEnable | Boolean | False | Setting this property to true enables UEFI Secure Boot, and setting it to false disables it. |
| Actions | Object | True | This action is used to reset the Secure Boot keys. SecureBoot allows the user to perform the SecureBoot.ResetKeys action, whose allowable values are given below. It can also contain an Oem Object under the Oem attribute of this Actions object. |
| &nbsp;&nbsp;ResetKeys | - | - | Reset the content of all UEFI Secure Boot key databases to their default values. |
| SecureBootCurrentBoot | String | True | The value of this property shall indicate the UEFI Secure Boot state during the current boot cycle. |
| &nbsp;&nbsp;Enabled | - | - | Secure Boot is currently enabled. |
| &nbsp;&nbsp;Disabled | - | - | Secure Boot is currently disabled. |
| SecureBootMode | String | True | Current Secure Boot Mode as defined in the UEFI Specification. |
| &nbsp;&nbsp;SetupMode | - | - | Secure Boot is currently in Setup Mode. |
| &nbsp;&nbsp;UserMode | - | - | Secure Boot is currently in User Mode. |
| &nbsp;&nbsp;AuditMode | - | - | Secure Boot is currently in Audit Mode. |
| &nbsp;&nbsp;DeployedMode | - | - | Secure Boot is currently in Deployed Mode. |
| SecureBootDatabases | Object | True | The value of this property shall be a link to a resource collection of type SecureBootDatabaseCollection. |

### 3.96.4 Example

#### 3.96.4.1 GET SecureBoot Instance (p.305-306)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SecureBoot
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#SecureBoot.SecureBoot",
    "@odata.etag": "\"1702708590\"",
    "@odata.id": "/redfish/v1/Systems/Self/SecureBoot",
    "@odata.type": "#SecureBoot.v1_1_0.SecureBoot",
    "Actions": {
        "#SecureBoot.ResetKeys": {
            "@Redfish.ActionInfo": "/redfish/v1/Systems/Self/SecureBoot/ResetKeysActionInfo",
            "target": "/redfish/v1/Systems/Self/SecureBoot/Actions/SecureBoot.ResetKeys"
        }
    },
    "Description": "A reference to the UEFI SecureBoot resource associated with this system",
    "Id": "SecureBoot",
    "Name": "UEFI Secure Boot",
    "SecureBootCurrentBoot": "Disabled",
    "SecureBootDatabases": {
        "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases"
    },
    "SecureBootEnable": false,
    "SecureBootMode": "SetupMode"
}
```

#### 3.96.4.2 PATCH SecureBoot Instance (p.306)
```
PATCH https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SecureBoot
Content-Type: application/json
```
Example request body:
```json
{
    "SecureBootEnable": true
}
```
Response: status **204** with no body.

## 3.97 SecureBootDatabaseCollection (p.307)

This is the schema definition for the SecureBootDatabases collection.

### 3.97.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases
```
### 3.97.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.97.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.97.4 Example
#### 3.97.4.1 GET SecureBootDatabaseCollection Instance (p.307-308)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#SecureBootDatabaseCollection.SecureBootDatabaseCollection",
    "@odata.etag": "\"1702729059\"",
    "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases",
    "@odata.type": "#SecureBootDatabaseCollection.SecureBootDatabaseCollection",
    "Description": "UEFI SecureBoot Database Collection",
    "Members": [
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/KEK" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/KEKDefault" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/PK" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/PKDefault" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/db" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/dbDefault" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/dbr" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/dbrDefault" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/dbt" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/dbtDefault" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/dbx" },
        { "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/dbxDefault" }
    ],
    "Members@odata.count": 12,
    "Name": "UEFI SecureBoot Database Collection"
}
```

## 3.98 SecureBootDatabase (p.309)

This is the schema definition that describes a UEFI Secure Boot database used to store certificates or hashes.

### 3.98.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}
```
### 3.98.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| POST | ConfigureComponents |

### 3.98.3 Supported Properties

#### Table 119. Supported Properties for SecureBootDatabase (p.309)

| Property | Type | Read Only | Description |
|---|---|---|---|
| DatabaseId | String | True | This property shall contain the name of the UEFI Secure Boot database. The value shall be one of the UEFI-defined Secure Boot databases: "PK", "KEK", "db", "dbx", "dbr", "dbt", "PKDefault", "KEKDefault", "dbDefault", "dbxDefault", "dbrDefault", or "dbtDefault". |
| Certificates | Object | True | A link to the collection of certificates contained in this UEFI Secure Boot database. |
| Signatures | Object | True | A link to the collection of signatures contained in this UEFI Secure Boot database. |
| Actions | Object | True | This action is used to reset the UEFI Secure Boot keys of this database. The ResetKeys action and its allowable values are given below. |
| &nbsp;&nbsp;ResetKeys | - | - | The type of reset or delete to perform on this UEFI Secure Boot database. |
| &nbsp;&nbsp;&nbsp;&nbsp;DeleteAllKeys | - | - | Delete the content of this UEFI Secure Boot key database. |
| &nbsp;&nbsp;&nbsp;&nbsp;ResetAllKeysToDefault | - | - | Reset the content of this UEFI Secure Boot key database to the default values. |

### 3.98.4 Example

#### 3.98.4.1 GET SecureBootDatabase Instance (p.309-310)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#SecureBootDatabase.SecureBootDatabase",
    "@odata.etag": "\"1702708588\"",
    "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/PK",
    "@odata.type": "#SecureBootDatabase.v1_0_0.SecureBootDatabase",
    "Actions": {
        "#SecureBootDatabase.ResetKeys": {
            "ResetKeysType@Redfish.AllowableValues": [
                "ResetAllKeysToDefault",
                "DeleteAllKeys"
            ],
            "target": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/PK/Actions/SecureBootDatabase.ResetKeys"
        }
    },
    "Certificates": {
        "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/PK/Certificates"
    },
    "DatabaseId": "PK",
    "Id": "PK",
    "Name": "PK",
    "Signatures": {
        "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/PK/Signatures"
    }
}
```

#### 3.98.4.2 POST SecureBootDatabase Instance (ResetKeys action) (p.310)
```
POST https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}/Actions/SecureBootDatabase.ResetKeys
Content-Type: application/json
```
Example request body:
```json
{
    "ResetKeysType": "ResetAllKeysToDefault"
}
```
Response: status **204** with no body.

## 3.99 SignatureCollection (p.311)

This is the schema definition for the signature collection of a Secure Boot database instance.

### 3.99.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}/Signatures
```
### 3.99.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| POST | ConfigureComponents |

### 3.99.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.99.4 Example

#### 3.99.4.1 GET SignatureCollection Instance (p.311)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}/Signatures
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#SignatureCollection.SignatureCollection",
    "@odata.etag": "\"1702729059\"",
    "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/PK/Signatures",
    "@odata.type": "#SignatureCollection.SignatureCollection",
    "Description": "Signature Collection for SecureBoot Database PK",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/PK/Signatures/1"
        }
    ],
    "Members@odata.count": 1,
    "Name": "Signature Collection for SecureBoot Database PK"
}
```

#### 3.99.4.2 POST SignatureCollection Instance (p.311-312)
```
POST https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}/Signatures
Content-Type: application/json
```
Example request body:
```json
{
    "SignatureString": "80B4D96931BF0D02FD91A61E19D14F1DA452E66DB2408CA8604D411F92659F0A",
    "SignatureType": "EFI_CERT_SHA256_GUID",
    "SignatureTypeRegistry": "UEFI",
    "UEFISignatureOwner": "77FA9ABD-0359-4D32-BD60-28F4E78F784B"
}
```
Response: status **201**. The body is a GET response with the properties of the newly created Signature:
```json
{
    "@odata.context": "/redfish/v1/$metadata#Signature.Signature",
    "@odata.etag": "\"1702729059\"",
    "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/PK/Signatures",
    "@odata.type": "#Signature.v1_0_1.Signature",
    "Id": "Signatures",
    "Name": "SHA256 Signature",
    "SignatureString": "80B4D96931BF0D02FD91A61E19D14F1DA452E66DB2408CA8604D411F92659F0A",
    "SignatureType": "EFI_CERT_SHA256_GUID",
    "SignatureTypeRegistry": "UEFI"
}
```

> **Note (from the source):** *Creating (POST) Signatures is allowed only for **non-default** Secure Boot databases ("PK", "KEK", "db", "dbx", "dbr", "dbt"). The default databases ("PKdefault", "KEKDefault", "dbDefault", "dbxDefault", "dbrDefault", "dbtDefault") don't support POST.*
>
> **Caveats:** The request uses `UEFISignatureOwner` (upper-case UEFI) while Table 120 and DMTF use `UefiSignatureOwner`. The 201 body has the collection's `@odata.id` and `Id: "Signatures"`, not the new member's URI, so a client must re-list the collection to learn the new member ID (as with BootOptions).

## 3.100 Signature (p.313)

This is the schema definition for a signature or a hash in a Secure Boot database instance.

### 3.100.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}/Signatures/{Signatures-Instance}
```
### 3.100.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| DELETE | ConfigureComponents |

### 3.100.3 Supported Properties

#### Table 120. Supported Properties for Signature (p.313)

| Property | Type | Read Only | Description |
|---|---|---|---|
| SignatureString | String | True | This property shall contain the string of the signature, and the format shall follow the requirements specified by the value of the SignatureType property. |
| SignatureType | String | True | This property shall contain the format type for the signature. |
| SignatureTypeRegistry | String | True | This property shall contain the type for the signature. |
| UefiSignatureOwner | String | True | The value of this property shall contain the GUID of the UEFI signature owner for this signature as defined by the UEFI Specification. This property shall only be present if the SignatureTypeRegistry property is "UEFI". |

### 3.100.4 Example

#### 3.100.4.1 GET Signature Instance (p.313-314)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}/Signatures/{Signatures-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#Signature.Signature",
    "@odata.etag": "\"1702731261\"",
    "@odata.id": "/redfish/v1/Systems/Self/SecureBoot/SecureBootDatabases/PK/Signatures/1",
    "@odata.type": "#Signature.v1_0_1.Signature",
    "Id": "1",
    "Name": "SHA256 Signature",
    "SignatureString": "80B4D96931BF0D02FD91A61E19D14F1DA452E66DB2408CA8604D411F92659F0A",
    "SignatureType": "EFI_CERT_SHA256_GUID",
    "SignatureTypeRegistry": "UEFI"
}
```
(`UefiSignatureOwner` is absent from the response even though `SignatureTypeRegistry` is "UEFI".)

#### 3.100.4.2 DELETE Signature Instance (p.314)
```
DELETE https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SecureBoot/SecureBootDatabases/{SecureBootDatabases-Instance}/Signatures/{Signatures-Instance}
Content-Type: application/json
```
Response: status **204** with no response body.

(The `Certificates` sub-collection of a SecureBootDatabase is linked but not documented in pages 261-381.)

---

## 3.101 SimpleStorageCollection (p.315)

This represents the collection of Simple Storage resources.

### 3.101.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/SimpleStorage
```
### 3.101.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.101.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.101.4 Example
#### 3.101.4.1 GET SimpleStorageCollection (p.315-316)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SimpleStorage
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#SimpleStorageCollection.SimpleStorageCollection",
    "@odata.etag": "\"1702731261\"",
    "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage",
    "@odata.type": "#SimpleStorageCollection.SimpleStorageCollection",
    "Description": "Collection of simple storage for this system",
    "Members": [
        { "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage/0" },
        { "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage/1" },
        { "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage/2" },
        { "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage/3" },
        { "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage/4" },
        { "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage/5" },
        { "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage/6" }
    ],
    "Members@odata.count": 7,
    "Name": "Simple Storage Collection"
}
```

## 3.102 SimpleStorage (p.317)

This is the schema definition for the Simple Storage resource. It represents the properties of a storage controller and its directly-attached devices.

### 3.102.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/SimpleStorage/{SimpleStorage-Instance}
```
### 3.102.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.102.3 Supported Properties

#### Table 121. Supported Properties for SimpleStorage (p.317)

| Property | Type | Read Only | Description |
|---|---|---|---|
| UefiDevicePath | String | True | The UEFI device path to access this storage controller. |
| Status | Object | True | This property shall contain any status or health properties of the Resource. |
| Links | Object | True | The links to other Resources that are related to this Resource. |
| Actions | Object | True | This object will contain the actions for this resource under Oem property if any. |
| Devices | Array of Objects | True | The storage devices. |
| &nbsp;&nbsp;Oem | Object | False | This property shall contain the OEM extensions. |
| &nbsp;&nbsp;Name | String | True | The name of the Resource or array member. |
| &nbsp;&nbsp;Status | Object | True | The status and health of the Resource and its subordinate or dependent Resources. |
| &nbsp;&nbsp;Manufacturer | String | True | The name of the manufacturer of this device. |
| &nbsp;&nbsp;CapacityBytes | Number | True | The size, in bytes, of the storage device. |
| &nbsp;&nbsp;Model | String | True | The product model number of this device. |

### 3.102.4 Example
#### 3.102.4.1 GET SimpleStorage Instance (p.317-318)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/SimpleStorage/{SimpleStorage-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#SimpleStorage.SimpleStorage",
    "@odata.etag": "\"1702729058\"",
    "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage/0",
    "@odata.type": "#SimpleStorage.v1_3_0.SimpleStorage",
    "Devices": [
        {
            "CapacityBytes": 256060514304,
            "Manufacturer": "N/A",
            "Model": "KLEVV CRAS C710 M.2 NVMe SSD 256GB",
            "Name": "NVMe_Device0_NSID1",
            "Status": {
                "Health": "OK",
                "State": "Enabled"
            }
        }
    ],
    "Id": "0",
    "Links": {
        "Chassis": {
            "@odata.id": "/redfish/v1/Chassis/Self"
        },
        "Storage": {
            "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0"
        }
    },
    "Name": "NVMe_Controller_0",
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "UefiDevicePath": "PciRoot(0x2)/Pci(0x3,0x3)/Pci(0x0,0x0)"
}
```
(`Links.Storage` is a non-DMTF link from SimpleStorage to Storage.)

## 3.103 StorageCollection (p.319)

It displays the collection of storage resource instances available in the system.

### 3.103.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Storage
```
### 3.103.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.103.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.103.4 Example
#### 3.103.4.1 GET StorageCollection (p.319)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Storage
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#StorageCollection.StorageCollection",
    "@odata.etag": "\"1702731261\"",
    "@odata.id": "/redfish/v1/Systems/Self/Storage",
    "@odata.type": "#StorageCollection.StorageCollection",
    "Description": "Collection of Storage resource instances",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0"
        }
    ],
    "Members@odata.count": 1,
    "Name": "Storage Collection"
}
```

## 3.104 Storage (p.320)

Storage defines a storage subsystem and its respective properties. A storage subsystem represents a set of storage controllers (physical or virtual) and the resources such as volumes that can be accessed from that subsystem.

### 3.104.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}
```
### 3.104.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.104.3 Supported Properties

#### Table 122. Supported Properties for Storage (p.320)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Links | Object | True | Contains references to other resources that are related to this resource. |
| &nbsp;&nbsp;Oem | Object | - | This property shall contain the OEM extensions. |
| &nbsp;&nbsp;Enclosures | Array | True | An array of references to the chassis to which this storage subsystem is attached |
| &nbsp;&nbsp;Enclosures@odata.count | Number | True | An integer representing the number of items in a collection. |
| StorageControllers | Array | True | A collection that indicates all the storage controllers that this resource represents. |
| StorageControllers@odata.count | Number | True | An integer representing the number of items in a collection. |
| Drives | Array | True | A collection that indicates all the drives attached to the storage controllers that this resource represents. |
| Drives@odata.count | Number | True | An integer representing the number of items in a collection. |
| Volumes | Object | True | A collection that indicates all the volumes produced by the storage controllers that this resource represents. |
| Redundancy | Array | True | Redundancy information for the storage subsystem. |
| Redundancy@odata.count | Number | True | An integer representing the number of items in a collection. |
| Controllers | Object | True | A collection that indicates all the Controllers associated with this storage instance |
| SimpleStorage | Object | True | The link to the simple storage instance that corresponds to this storage. |

#### Table 123. Storage Controller Properties (p.320-...)

| Property | Type | Read Only | Description |
|---|---|---|---|
| MemberId | String | True | This is the identifier for the member within the collection. |
| Assembly | Object | True | The link to the assembly associated with this storage controller. |
| ControllerRates | object | - | This type describes the various controller rates used for processes such as Volume Rebuild or Consistency Checks. |
| &nbsp;&nbsp;ConsistencyCheckRatePercent | Number | True | The percentage of controller Resources used for performing a data consistency check on volumes. |
| &nbsp;&nbsp;RebuildRatePercent | Number | True | The percentage of controller Resources used for rebuilding/repairing volumes. |
| &nbsp;&nbsp;TransformationRatePercent | Number | True | The percentage of controller Resources used for transforming volumes from one configuration to another. |
| Name | String | True | The name of the Storage Controller. |
| SpeedGbps | Number | True | The value of this property shall represent the speed of the Storage bus interface (in Gigabits per second). |
| FirmwareVersion | String | True | The firmware version of this storage Controller. |
| Manufacturer | String | True | This is the manufacturer of this storage controller. |
| Model | String | True | This is the model number for the storage controller. |
| SKU | String | True | This is the SKU for this storage controller. |
| SerialNumber | String | True | The serial number for this storage controller. |
| PartNumber | String | True | The part number for this storage controller. |
| AssetTag | String | True | The user assigned asset tag for this storage controller. |
| SupportedControllerProtocols | Array | True | The supported set of protocols for communicating to this storage controller. Refer to the table Protocol Properties. |
| SupportedDeviceProtocols | Array | True | The protocols that the storage controller can use to communicate with attached devices. Refer to the table Protocol Properties. |
| Identifiers | Array | True | This property shall contain a list of all known durable names for the associated storage controller. |
| Location | Object | True | The location of the storage controller. |
| CacheSummary | Object | True | This object describes the cache memory of the storage controller in general detail. |
| &nbsp;&nbsp;PersistentCacheSizeMiB | Number | True | The portion of the cache memory that is persistent, measured in MiB. |
| &nbsp;&nbsp;TotalCacheSizeMiB | Number | True | The total configured cache memory, measured in MiB. |
| &nbsp;&nbsp;Status | Object | True | The status and health of the resource and its subordinate or dependent resources. |
| PCIeInterface | Object | - | The PCIe interface details for this controller. Refer to the table PCIeInterface Properties (in an earlier part). |
| SupportedRAIDTypes | Array | True | This object describes the RAID Types supported by the storage controller. |
| &nbsp;&nbsp;RAID0 | - | - | A placement policy where consecutive logical blocks of data are uniformly distributed across a set of independent storage devices without offering any form of redundancy. |
| &nbsp;&nbsp;RAID1 | - | - | A placement policy where each logical block of data is stored on more than one independent storage device. |
| &nbsp;&nbsp;RAID5 | - | - | A placement policy using parity-based protection for storing stripes of 'n' logical blocks of data and one logical block of parity across a set of 'n+1' independent storage devices where the parity and data blocks are interleaved across the storage devices. |
| &nbsp;&nbsp;RAID6 | - | - | A placement policy using parity-based protection for storing stripes of 'n' logical blocks of data and two logical blocks of independent parity across a set of 'n+2' independent storage devices where the parity and data blocks are interleaved across the storage devices. |
| Links | Object | True | Contains references to other resources that are related to this resource. |
| &nbsp;&nbsp;Endpoints | Array | True | The value of this property shall be a reference to the resources that this system is associated with and shall reference a resource of type Endpoint. |
| &nbsp;&nbsp;Endpoints@odata.count | Number | True | An integer representing the number of items in a collection. |

#### Table 124. Protocol Properties (p.322)

| Property | Description |
|---|---|
| PCIe | PCI Express. |
| AHCI | Advanced Host Controller Interface. |
| UHCI | Universal Host Controller Interface. |
| SAS | Serial Attached SCSI. |
| SATA | Serial AT Attachment. |
| USB | Universal Serial Bus. |
| NVMe | Non-Volatile Memory Express (NVMe). |
| FC | Fibre Channel. |
| iSCSI | Internet SCSI. |
| FCoE | Fibre Channel over Ethernet (FCoE). |
| NVMeOverFabrics | NVMe over Fabrics. |
| SMB | Server Message Block (SMB). Also known as the Common Internet File System (CIFS). |
| NFSv3 | Network File System (NFS) version 3. |
| NFSv4 | Network File System (NFS) version 4. |
| HTTP | Hypertext Transport Protocol (HTTP). |
| HTTPS | Hypertext Transfer Protocol Secure (HTTPS). |
| FTP | File Transfer Protocol (FTP). |
| SFTP | SSH File Transfer Protocol (SFTP). |
| FCP | Fibre Channel Protocol for SCSI. |
| FICON | FIbre CONnection (FICON). |
| TCP | Transmission Control Protocol (TCP). |
| UDP | User Datagram Protocol (UDP). |
| TFTP | Trivial File Transfer Protocol (TFTP). |
| GenZ | This value shall indicate conformance to the Gen-Z Core Specification. |
| MultiProtocol | This value shall indicate conformance to multiple protocols. |
| InfiniBand | This value shall indicate conformance to the InfiniBand Architecture Specification-defined InfiniBand protocol. |
| Ethernet | This value shall indicate conformance to the IEEE 802.3 Ethernet specification. |
| NVLink | This value shall indicate conformance to the NVIDIA NVLink protocol. |
| OEM | OEM-specific. |
| DisplayPort | This value shall indicate conformance to the VESA DisplayPort Specification. |
| HDMI | This value shall indicate conformance to the HDMI Forum HDMI Specification. |
| VGA | This value shall indicate conformance to the VESA SVGA Specification |
| DVI | This value shall indicate conformance to the Digital Display Working Group DVI-A, DVI-D, or DVI-I Specification |

### 3.104.4 Example
#### 3.104.4.1 GET Storage Instance (p.322-325)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}
Content-Type: application/json
```
Response (the source elides controllers 2-5 with `...`):
```json
{
    "@odata.context": "/redfish/v1/$metadata#Storage.Storage",
    "@odata.etag": "\"1702729058\"",
    "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0",
    "@odata.type": "#Storage.v1_9_0.Storage",
    "Controllers": {
        "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Controllers"
    },
    "Description": "Storage0",
    "Drives": [
        {
            "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Drives/NVMe_Device0_NSID1"
        },
        {
            "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Drives/USB_Device1_Port3"
        }
    ],
    "Drives@odata.count": 2,
    "Id": "StorageUnit_0",
    "Links": {
        "SimpleStorage": []
    },
    "Name": "StorageUnit_0",
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "StorageControllers": [
        {
            "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0#/StorageControllers/0",
            "AssetTag": "(NVMe_Controller_0)",
            "CacheSummary": {
                "Status": []
            },
            "ControllerRates": [],
            "FirmwareVersion": "U0914A0L",
            "Identifiers": [],
            "Links": [],
            "Location": {
                "PartLocation": [],
                "Placement": [],
                "PostalAddress": []
            },
            "MemberId": "0",
            "Model": "KLEVV CRAS C710 M.2 NVMe SSD 256GB",
            "Name": "NVMe_Controller_0",
            "PCIeInterface": [],
            "SerialNumber": "C710B1N12KEA00065",
            "SpeedGbps": 31.51,
            "Status": {
                "Health": "OK",
                "State": "Enabled"
            },
            "SupportedControllerProtocols": [
                "NVMe",
                "PCIe"
            ],
            "SupportedRAIDTypes": []
        },
        {
            "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0#/StorageControllers/1",
            "AssetTag": "(USB_Controller_1)",
            "CacheSummary": {
                "Status": []
            },
            "ControllerRates": [],
            "FirmwareVersion": "1.20",
            "Identifiers": [],
            "Links": [],
            "Location": {
                "PartLocation": [],
                "Placement": [],
                "PostalAddress": []
            },
            "MemberId": "1",
            "Model": "Not Available",
            "Name": "USB_Controller1",
            "PCIeInterface": [],
            "SerialNumber": "Not Available",
            "SpeedGbps": 3,
            "Status": {
                "Health": "OK",
                "State": "Enabled"
            },
            "SupportedControllerProtocols": [
                "USB",
                "PCIe"
            ],
            "SupportedRAIDTypes": []
        },
        ...
        {
            "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0#/StorageControllers/6",
            "AssetTag": "(SATA_Controller_6)",
            "CacheSummary": {
                "Status": []
            },
            "ControllerRates": [],
            "FirmwareVersion": "Not Available",
            "Identifiers": [],
            "Links": [],
            "Location": {
                "PartLocation": [],
                "Placement": [],
                "PostalAddress": []
            },
            "MemberId": "6",
            "Model": "Not Available",
            "Name": "SATA_Controller_6",
            "PCIeInterface": [],
            "SerialNumber": "Not Available",
            "SpeedGbps": 6,
            "Status": {
                "Health": "OK",
                "State": "Enabled"
            },
            "SupportedControllerProtocols": [
                "AHCI",
                "PCIe",
                "SATA"
            ],
            "SupportedRAIDTypes": []
        }
    ],
    "StorageControllers@odata.count": 7,
    "Volumes": {
        "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Volumes"
    }
}
```

> **Caveats (schema violations to handle in a Go decoder):** Several object-typed properties are serialised as empty JSON **arrays** `[]` instead of objects or null: `CacheSummary.Status`, `ControllerRates`, `Links` (inside StorageControllers), `Location.PartLocation`, `Location.Placement`, `Location.PostalAddress`, `PCIeInterface`, and top-level `Links.SimpleStorage`. Strict typed decoding into DMTF structs will fail, so use lenient decoding (e.g. `json.RawMessage` or custom UnmarshalJSON that accepts `[]` as empty). Placeholder strings such as "Not Available" and "N/A" appear. `AssetTag` holds the controller name in parentheses.

## 3.105 StorageControllerCollection (p.326)

The StorageController schema describes a storage controller and its properties. A storage controller represents a physical or virtual storage device that produces volumes.

### 3.105.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Controllers
```
### 3.105.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.105.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.105.4 Example
#### 3.105.4.1 GET StorageControllerCollection Instance (p.326-327)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Controllers
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#StorageControllerCollection.StorageControllerCollection",
    "@odata.etag": "\"1702731261\"",
    "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Controllers",
    "@odata.type": "#StorageControllerCollection.StorageControllerCollection",
    "Description": "Collection of storage controllers",
    "Members": [
        { "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Controllers/0" },
        { "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Controllers/1" },
        { "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Controllers/2" },
        { "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Controllers/3" },
        { "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Controllers/4" },
        { "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Controllers/5" },
        { "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Controllers/6" }
    ],
    "Members@odata.count": 7,
    "Name": "Storage Controllers Collection"
}
```

## 3.106 StorageController (p.328)

The StorageController schema describes a storage controller and its properties. A storage controller represents a physical or virtual storage device that produces volumes.

### 3.106.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Controllers/{Controllers-Instance}
```
### 3.106.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.106.3 Supported Properties

#### Table 125. Supported Properties for StorageController (p.328)

| Property | Type | Read Only | Description |
|---|---|---|---|
| AssetTag | String | True | The user-assigned asset tag for this storage controller. |
| FirmwareVersion | String | True | The firmware version of this storage controller. |
| Manufacturer | String | True | The manufacturer of this storage controller. |
| Model | String | True | The model number for the storage controller. |
| SerialNumber | String | True | The serial number for this storage controller. |
| SpeedGbps | Number | True | The maximum speed of the storage controller's device interface. |
| Status | Object | True | The status and health of the resource and its subordinate or dependent resources. |
| SupportedControllerProtocols | Array of String | True | The supported set of protocols for communicating to this storage controller. Refer to the table Protocol Properties. |
| SupportedDeviceProtocols | Array of String | True | The protocols that the storage controller can use to communicate with attached devices. Refer to the table Protocol Properties. |
| SupportedRAIDTypes | Array of String | True | The set of RAID types supported by the storage controller. |

### 3.106.4 Example
#### 3.106.4.1 GET StorageController Instance (p.328-329)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Controllers/{Controllers-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#StorageController.v1_0_0.StorageController",
    "@odata.etag": "\"1702729058\"",
    "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Controllers/0",
    "@odata.type": "#StorageController.v1_0_0.StorageController",
    "AssetTag": "(NVMe_Controller_0)",
    "FirmwareVersion": "U0914A0L",
    "Id": "0",
    "Model": "KLEVV CRAS C710 M.2 NVMe SSD 256GB",
    "NVMeControllerProperties": {
        "MaxQueueSize": 255,
        "NVMeVersion": "NVMe Spec v1.3"
    },
    "Name": "NVMe_Controller_0",
    "SerialNumber": "C710B1N12KEA00065",
    "SpeedGbps": 31.51,
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "SupportedControllerProtocols": [
        "NVMe",
        "PCIe"
    ],
    "SupportedDeviceProtocols": [
        "NVMe"
    ]
}
```
(`NVMeControllerProperties` appears in the response but not in Table 125.)

## 3.107 Drive (p.330)

This is the schema definition for the Drives. It represents the properties of a Drive attached to a System.

### 3.107.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Drives/{Drives-Instance}
```
### 3.107.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.107.3 Supported Properties

#### Table 126. Supported Properties for Drive (p.330-332)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Manufacturer | String | True | This is the manufacturer of this Drive. |
| Model | String | True | This is the model number for the Drive. |
| SKU | String | True | This is the SKU for this Drive. |
| SerialNumber | String | True | The serial number for this Drive. |
| PartNumber | String | True | The part number for this Drive. |
| AssetTag | String | True | The user assigned asset tag for this Drive. |
| Assembly | Object | True | The link to the assembly associated with this Drive. |
| Revision | String | True | The revision of this Drive. This is typically the firmware/hardware version of the drive. |
| Links | Object | True | The links object contains the links to other resources that are related to this resource. |
| &nbsp;&nbsp;Endpoints | Array | True | An array of references to the endpoints that connect to this drive. |
| &nbsp;&nbsp;Endpoints@odata.count | Number | True | An integer representing the number of items in a collection. |
| &nbsp;&nbsp;Chassis | Object | True | A reference to the Chassis which contains this Drive. |
| &nbsp;&nbsp;PCIeFunctions | Array | True | An array of references to the PCIe Functions which the drive produces. |
| &nbsp;&nbsp;PCIeFunctions@odata.count | Number | True | An integer representing the number of items in a collection. |
| &nbsp;&nbsp;Volumes | Array | True | An array of references to the volumes contained in this drive. |
| &nbsp;&nbsp;Volumes@odata.count | Number | True | An integer representing the number of items in a collection. |
| Operations | Object | True | The operations currently running on the Drive. |
| &nbsp;&nbsp;OperationName | String | True | The name of the operation. |
| &nbsp;&nbsp;PercentageComplete | Number | True | The percentage of the operation that has been completed. |
| &nbsp;&nbsp;AssociatedTask | Object | True | A reference to the task associated with the operation if any. |
| StatusIndicator | String | True | The state of the status indicator, used to communicate status information about this drive. |
| &nbsp;&nbsp;OK | - | - | The drive is OK. |
| &nbsp;&nbsp;Fail | - | - | The drive has failed. |
| &nbsp;&nbsp;Rebuild | - | - | The drive is being rebuilt. |
| &nbsp;&nbsp;PredictiveFailureAnalysis | - | - | The drive is still working but predicted to fail soon. |
| &nbsp;&nbsp;Hotspare | - | - | The drive has been marked to automatically rebuild and replace a failed drive. |
| &nbsp;&nbsp;InACriticalArray | - | - | The array to which this drive belongs has been degraded. |
| &nbsp;&nbsp;InAFailedArray | - | - | The array to which this drive belongs has failed. |
| IndicatorLED | String | True | The state of the indicator LED, that identifies the drive. |
| &nbsp;&nbsp;Lit | - | - | The Indicator LED is lit |
| &nbsp;&nbsp;Blinking | - | - | The Indicator LED is blinking. |
| &nbsp;&nbsp;Off | - | - | The Indicator LED is off. |
| CapacityBytes | Number | True | The size in bytes of this Drive. |
| FailurePredicted | Boolean | True | An indication of whether this drive currently predicts a failure in the near future. |
| PhysicalLocation | Object | True | The location of the drive. |
| Protocol | String | True | The protocol that this drive currently uses to communicate to the storage controller. |
| &nbsp;&nbsp;PCIe | - | - | PCI Express |
| &nbsp;&nbsp;AHCI | - | - | Advanced Host Controller Interface |
| &nbsp;&nbsp;UHCI | - | - | Universal Host Controller Interface |
| &nbsp;&nbsp;SAS | - | - | Serial Attached SCSI |
| &nbsp;&nbsp;SATA | - | - | Serial AT Attachment |
| &nbsp;&nbsp;USB | - | - | Universal Serial Bus |
| &nbsp;&nbsp;NVMe | - | - | Non-Volatile Memory Express |
| &nbsp;&nbsp;FC | - | - | Fibre Channel |
| &nbsp;&nbsp;iSCSI | - | - | Internet SCSI |
| &nbsp;&nbsp;NVMeOverFabrics | - | - | NVMe over Fabrics |
| &nbsp;&nbsp;SMB | - | - | Server Message Block. Aka CIFS Common Internet File System. |
| &nbsp;&nbsp;NFSv3 | - | - | Network File System version 3 |
| &nbsp;&nbsp;NFSv4 | - | - | Network File System version 3 [sic] |
| &nbsp;&nbsp;HTTP | - | - | Hypertext Transport Protocol |
| &nbsp;&nbsp;HTTPS | - | - | Secure Hypertext Transport Protocol |
| &nbsp;&nbsp;SFTP | - | - | Secure File Transfer Protocol |
| &nbsp;&nbsp;FTP | - | - | File Transfer Protocol |
| MediaType | String | True | The type of media contained in this drive. |
| &nbsp;&nbsp;HDD | - | - | The drive media type is traditional magnetic platters. |
| &nbsp;&nbsp;SSD | - | - | The drive media type is solid state or flash memory. |
| &nbsp;&nbsp;SMR | - | - | The drive media type is shingled magnetic recording. |
| Identifiers | Array | True | Any additional identifiers for a resource. |
| EncryptionAbility | String | True | The encryption abilities of this drive. |
| &nbsp;&nbsp;None | - | - | The drive is not capable of self-encryption. |
| &nbsp;&nbsp;SelfEncryptingDrive | - | - | The drive is capable of self-encryption per the Trusted Computing Group's Self Encrypting Drive Standard. |
| &nbsp;&nbsp;Other | - | - | The drive is capable of self-encryption through some other means. |
| HotspareType | String | True | The type of hotspare this drive is currently serving as. |
| &nbsp;&nbsp;None | - | - | The drive is not currently a hotspare. |
| &nbsp;&nbsp;Global | - | - | The drive is currently serving as a hotspare for all other drives in the storage system. |
| &nbsp;&nbsp;Chassis | - | - | The drive is currently serving as a hotspare for all other drives in the chassis. |
| &nbsp;&nbsp;Dedicated | - | - | The drive is currently serving as a hotspare for a user defined set of drives. |
| EncryptionStatus | String | True | The status of the encryption of this drive. |
| &nbsp;&nbsp;Unecrypted [sic] | - | - | The drive is not currently encrypted. Deprecated in v1.1 and later. |
| &nbsp;&nbsp;Unlocked | - | - | The drive is currently encrypted but the data is accessible to the user in unencrypted form. |
| &nbsp;&nbsp;Locked | - | - | The drive is currently encrypted and the data is not accessible to the user. |
| &nbsp;&nbsp;Unencrypted | - | - | The drive is not currently encrypted. |
| &nbsp;&nbsp;Foreign | - | - | The drive is currently encrypted, the data is not accessible to the user, and the system requires user intervention to expose the data. |
| RotationSpeedRPM | Number | True | The rotation speed of this Drive in Revolutions per Minute (RPM). |
| BlockSizeBytes | Number | True | The size of the smallest addressable unit (Block) of this drive in bytes. |
| CapableSpeedGbs | Number | True | The speed which this drive can communicate to a storage controller in ideal conditions in Gigabits per second. |
| NegotiatedSpeedGbs | Number | True | The speed which this drive is currently communicating to the storage controller in Gigabits per second. |
| PredictedMediaLifeLeftPercent | Number | True | The percentage of reads and writes that are predicted to still be available for the media. |
| HotspareReplacementMode | String | True | The replacement mode for the hotspare drive. |
| &nbsp;&nbsp;NonRevertible | - | - | A hotspare drive that is commissioned due to a drive failure will remain as a data drive and will not revert to a hotspare if the failed drive is replaced. |
| &nbsp;&nbsp;Revertible | - | - | A hotspare drive that is commissioned due to a drive failure will revert to being a hotspare once the failed drive is replaced and rebuilt. |
| WriteCacheEnabled | Boolean | True | This property shall indicate whether the drive write cache is enabled. |
| LocationIndicatorActive | Boolean | True | An indicator allowing an operator to physically locate this resource. |
| ReadyToRemove | Boolean | True | An indication of whether the drive is prepared by the system for removal. |

### 3.107.4 Example
#### 3.107.4.1 GET Drive Instance (p.332-333)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Drives/{Drives-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#Drive.Drive",
    "@odata.etag": "\"1702729058\"",
    "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Drives/NVMe_Device0_NSID1",
    "@odata.type": "#Drive.v1_11_0.Drive",
    "Assembly": [],
    "AssetTag": null,
    "BlockSizeBytes": 512,
    "CapableSpeedGbs": 31.51,
    "CapacityBytes": 256060514304,
    "EncryptionAbility": "None",
    "Id": "NVMe_Device0_NSID1",
    "Identifiers": [],
    "IndicatorLED": null,
    "Links": {
        "Chassis": {
            "@odata.id": "/redfish/v1/Chassis/Self"
        },
        "Endpoints": [],
        "Endpoints@odata.count": 0,
        "Volumes": [
            {
                "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Volumes/VOL0"
            }
        ],
        "Volumes@odata.count": 1
    },
    "Manufacturer": "N/A",
    "MediaType": "SSD",
    "Model": "KLEVV CRAS C710 M.2 NVMe SSD 256GB",
    "Name": "NVMe_Device0_NSID1",
    "NegotiatedSpeedGbs": 31.51,
    "Operations": [],
    "PhysicalLocation": {
        "PartLocation": [],
        "Placement": [],
        "PostalAddress": []
    },
    "PredictedMediaLifeLeftPercent": 100,
    "Protocol": "NVMe",
    "Revision": "NVMe Spec v1.3",
    "RotationSpeedRPM": 0,
    "SerialNumber": "C710B1N12KEA00065",
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "StatusIndicator": "OK"
}
```
(Again `Assembly: []` and the `PhysicalLocation` sub-objects are empty arrays instead of objects. `Revision` carries the NVMe spec version, not the drive firmware.)

## 3.108 VolumeCollection (p.334)

It displays the collection of volume resource instances available in the system.

### 3.108.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Volumes
```
### 3.108.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| POST | ConfigureComponents |

### 3.108.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.108.4 Example

#### 3.108.4.1 GET VolumeCollection Instance (p.334)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Volumes
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#VolumeCollection.VolumeCollection",
    "@odata.etag": "\"1702731261\"",
    "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Volumes",
    "@odata.type": "#VolumeCollection.VolumeCollection",
    "Description": "The Collection for Volumes",
    "Members": [
        { "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Volumes/VOL0" },
        { "@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Volumes/VOL1" }
    ],
    "Members@odata.count": 2,
    "Name": "Volume Collection"
}
```

#### 3.108.4.2 POST VolumeCollection Instance (create a volume) (p.334-335)

The following properties are **mandatory** to create a volume:
- **Name**
- **RAIDType**: allowed values "RAID0", "RAID1", "RAID5", "RAID6".
- **StripSizeBytes**: allowed values "64K", "128K", "256K", "512K", "1MB". (**Note:** these are strings, not the DMTF integer byte count.)
- **ReadCachePolicyType**: allowed values "Off", "ReadAhead". (Non-DMTF name; DMTF uses `ReadCachePolicy`.)
- **WriteCachePolicyType**: allowed values "WriteThrough", "UnprotectedWriteBack". (Non-DMTF name; DMTF uses `WriteCachePolicy`.)
- **Oem/Ami/ControllerID**
- **Oem/Ami/Initialization**: allowed values "No_Initialization", "Quick_Initialization", "Full_Initialization".
- **Oem/Ami/DiskCachePolicy**: allowed values "Unchanged", "Enabled", "Disabled".
- **Oem/Ami/AccessPolicy**: allowed values "Read-Write", "Read_Only", "Blocked". (Note the inconsistent hyphen/underscore.)
- **Oem/Ami/Device**: array of Drive URIs.

Request:
```
POST https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Volumes
Content-Type: application/json
```
Example request body:
```json
{
    "Name": "Volume_Name",
    "RAIDType": "RAID0",
    "StripSizeBytes": "64K",
    "ReadCachePolicyType": "ReadAhead",
    "WriteCachePolicyType": "WriteThrough",
    "Oem": {
        "Ami": {
            "ControllerID": "/redfish/v1/Systems/Self/Storage/StorageUnit_0",
            "Initialization": "No_Initialization",
            "DiskCachePolicy": "Unchanged",
            "AccessPolicy": "Read-Write",
            "Device": [
                "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Drives/NVMe_Device0_NSID1",
                "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Drives/USB_Device1_Port1"
            ]
        }
    }
}
```
Response: status **204** with no body. (DMTF expects 201 with a Location header or 202 with a Task; this returns 204 and does not report the new volume URI.)

## 3.109 Volume (p.336)

Volume contains properties used to describe a volume, virtual disk, LUN, or other logical storage entity for any system.

### 3.109.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Volumes/{Volumes-Instance}
```
### 3.109.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

(DELETE is not listed and the `Volume.Initialize` action has no POST privilege entry, although Actions mentions it.)

### 3.109.3 Supported Properties

#### Table 127. Supported Properties for Volume (p.336-337)

| Property | Type | Read Only | Description |
|---|---|---|---|
| CapacityBytes | Number | True | This property shall contain the size in bytes of the associated volume. |
| VolumeType | String | True | This property shall contain the type of the associated Volume. |
| &nbsp;&nbsp;RawDevice | - | - | The volume is a raw physical device without any RAID or other virtualization applied. |
| &nbsp;&nbsp;NonRedundant | - | - | The volume is a non-redundant storage device. |
| &nbsp;&nbsp;Mirrored | - | - | The volume is a mirrored device. |
| &nbsp;&nbsp;StripedWithParity | - | - | The volume is a device which uses parity to retain redundant information. |
| &nbsp;&nbsp;SpannedMirrors | - | - | The volume is a spanned set of mirrored devices. |
| &nbsp;&nbsp;SpannedStripesWithParity | - | - | The volume is a spanned set of devices which uses parity to retain redundant information. |
| Encrypted | Boolean | True | This property shall contain a boolean indicator if the Volume is currently utilizing encryption or not. |
| EncryptionTypes | Array | True | This property shall contain the types of encryption used by this Volume. |
| &nbsp;&nbsp;NativeDriveEncryption | - | - | The volume is utilizing the native drive encryption capabilities of the drive hardware. |
| &nbsp;&nbsp;ControllerAssisted | - | - | The volume is being encrypted by the storage controller entity. |
| &nbsp;&nbsp;SoftwareAssisted | - | - | The volume is being encrypted by software running on the system or the operating system. |
| Identifiers | Array | True | This property shall contain a list of all known durable names for the associated volume. |
| BlockSizeBytes | Number | True | The size of the smallest addressible unit (Block) of this volume in bytes. |
| Operations | Array | - | The operations currently running on the Volume. |
| &nbsp;&nbsp;OperationName | String | True | The name of the operation. |
| &nbsp;&nbsp;PercentageComplete | Number | True | The percentage of the operation that has been completed. |
| &nbsp;&nbsp;AssociatedTask | Object | True | A reference to the task associated with the operation if any. |
| OptimumIOSizeBytes | Number | True | This property shall contain the optimum IO size to use when performing IO on this volume. For logical disks, this is the stripe size. For physical disks, this describes the physical sector size. |
| Links | object | - | An array of references to the drives which contain this volume. |
| &nbsp;&nbsp;Oem | object | - | This property shall contain the OEM extensions. |
| &nbsp;&nbsp;Drives | Array | True | An array of references to the chassis to which this storage subsystem is attached [sic] |
| &nbsp;&nbsp;Drives@odata.count | Number | True | An integer representing the number of items in a collection. |
| Actions | Object | True | Volume.Initialize is the available action for this resource |
| NVMeNamespaceProperties | Object | True | This property contains properties to use when Volume is used to describe an NVMe Namespace. |
| &nbsp;&nbsp;FormattedLBASize | String | True | The LBA data size and metadata size combination that the namespace has been formatted with. |
| &nbsp;&nbsp;MetadataTransferredAtEndOfDataLBA | Boolean | True | This property indicates whether or not the metadata is transferred at the end of the LBA creating an extended data LBA. |
| &nbsp;&nbsp;NumberLBAFormats | integer | True | The number of LBA data size and metadata size combinations supported by this namespace. |
| &nbsp;&nbsp;NVMeVersion | String | True | The version of the NVMe Base Specification supported. |
| VolumeUsage | String | True | Indicates the Volume usage type setting for the Volume. |
| StripSizeBytes | Number | True | The number of blocks (bytes) in a strip in a disk array that uses striped data mapping. |
| Compressed | Boolean | True | Indicator of whether or not the Volume has compression enabled. |
| Deduplicated | Boolean | True | Indicator of whether or not the Volume has deduplication enabled. |
| DisplayName | String | True | A user-configurable string to name the volume. |
| LogicalUnitNumber | Number | True | Indicates the host-visible LogicalUnitNumber assigned to this Volume. |
| Manufacturer | String | True | The manufacturer or OEM of this storage volume. |
| Model | String | True | The model number for this storage volume. |
| MediaSpanCount | Number | True | Indicates the number of media elements used per span in the secondary RAID for a hierarchical RAID type. |
| ProvisioningPolicy | String | True | This property specifies the volume's storage allocation, or provisioning policy. |
| RAIDType | String | True | The RAID type of this volume. |
| ReadCachePolicy | String | True | Indicates the read cache policy setting for the Volume. |
| RemainingCapacityPercent | Number | True | The percentage of the capacity remaining in the Volume. |
| WriteHoleProtectionPolicy | String | True | The policy that the RAID volume is using to address the write hole issue. |
| WriteCacheState | String | True | Indicates the WriteCacheState policy setting for the Volume. |
| WriteCachePolicy | String | True | Indicates the write cache policy setting for the Volume. |
| MaxBlockSizeByte | Number | True | Max Block size in bytes. |
| ReplicaInfo | Object | True | Describes this storage volume in its role as a target replica. |
| RecoverableCapacitySourceCount | Number | True | Current number of capacity source resources that are available as replacements. |

#### Table 128. Properties of ReplicaInfo (p.337-338)

| Property | Type | Read Only | Description |
|---|---|---|---|
| ConsistencyEnabled | Boolean | True | True if consistency is enabled. |
| ConsistencyState | String | True | The current state of consistency. |
| ConsistencyStatus | String | True | The current status of consistency. |
| ConsistencyType | String | True | Indicates the consistency type used by the source and its associated target group. |
| FailedCopyStopsHostIO | Boolean | True | If true, the storage array tells host to stop sending data to source element if copying to a remote element fails. |
| PercentSynced | Number | True | Specifies the percent of the work completed to reach synchronization. |
| ReplicaPriority | String | True | The priority of background copy engine I/O to be managed relative to host I/O operations during a sequential background copy operation. |
| ReplicaProgressStatus | String | True | The status of the session with respect to Replication activity. |
| ReplicaReadOnlyAccess | String | True | This property specifies whether the source, the target, or both elements are read only to the host. |
| ReplicaRecoveryMode | String | True | Describes whether the copy operation continues after a broken link is restored. |
| ReplicaRole | String | True | The source or target role of this replica. |
| ReplicaSkewBytes | String | True | Applies to Adaptive mode and it describes maximum number of bytes the SyncedElement (target) can be out of sync. |
| ReplicaState | String | True | ReplicaState describes the state of the relationship with respect to Replication activity. |
| ReplicaType | String | True | ReplicaType describes the intended outcome of the replication. |
| ReplicaUpdateMode | String | True | Describes whether the target elements will be updated synchronously or asynchronously. |
| RequestedReplicaState | String | True | The last requested or desired state for the relationship. |
| SyncMaintained | String | True | Synchronization is maintained. |
| UndiscoveredElement | String | True | This property specifies whether the source, the target, or both elements involved in a copy operation are undiscovered |
| WhenActivated | String | True | Specifies when point-in-time copy was taken or when the replication relationship is activated, reactivated, resumed or re-established. |
| WhenDeactivated | String | True | Specifies when the replication relationship is deactivated. |
| WhenEstablished | String | True | Specifies when the replication relationship is established. |
| WhenSuspended | String | True | Specifies when the replication relationship is suspended. |
| WhenSynced | String | True | The point in time that the Elements were synchronized. |
| WhenSynchronized | String | True | Specifies when the replication relationship is synchronized. |

### 3.109.4 Example
#### 3.109.4.1 GET Volume Instance (p.338)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/Storage/{Storage-Instance}/Volumes/{Volumes-Instance}
```
Response: **The example response box is empty in the PDF.**

---

## 3.110 USBControllerCollection (p.339)

It displays the collection of USB Controller resource instances available in the system.

### 3.110.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/USBControllers
```
### 3.110.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.110.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.110.4 Example
#### 3.110.4.1 GET USBControllerCollection Instance (p.339)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/USBControllers
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#USBControllersCollection.USBControllersCollection",
    "@odata.etag": "\"1703164815\"",
    "@odata.id": "/redfish/v1/Systems/Self/USBControllers",
    "@odata.type": "#USBControllerCollection.USBControllerCollection",
    "Description": "Collection of USBControllers",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Systems/Self/USBControllers/DevTypeC_USB_0"
        }
    ],
    "Members@odata.count": 1,
    "Name": "USBControllers Collection"
}
```

## 3.111 USBController (p.340)

The USBController schema defines a Universal Serial Bus controller.

### 3.111.1 URL
```
/redfish/v1/Systems/{Systems-Instance}/USBControllers/{USBControllers-Instance}
```
(The source prints `/redfish/v1/Systems/{Systems-Instance}/ Manufacturer /{USBControllers-Instance}`, an editing error. The example request uses `USBControllers`.)

### 3.111.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.111.3 Supported Properties

#### Table 129. Supported Properties for USBController (p.340)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Links | Object | - | The links to other resources that are related to this resource. |
| &nbsp;&nbsp;PCIeDevice | Object | True | A link to the PCIe device that represents this usb controller. |
| &nbsp;&nbsp;Processors | Array of Object | True | An array of links to the processors that are a part of this usb controller. |
| &nbsp;&nbsp;Processors@odata.count | Number | True | Number of Processor instance connected that are part of this usb controller. |
| Manufacturer | String | True | The manufacturer of this usb controller. |
| Model | String | True | The product model number of this usb controller. |
| PartNumber | String | True | The part number for this usb controller. |
| Ports | Object | True | This property shall contain a link to a resource collection of type PortCollection. |
| SKU | String | True | The SKU for this usb controller. |
| SerialNumber | String | True | The serial number for this usb controller. |
| SparePartNumber | String | True | The spare part number of the usb controller. |
| Status | Object | True | The status and health of the resource and its subordinate or dependent resources. |

### 3.111.4 Example
#### 3.111.4.1 GET USBController Instance (p.340-...)
```
GET https://{BMC_IP}/redfish/v1/Systems/{Systems-Instance}/USBControllers/{USBControllers-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#USBController.USBController",
    "@odata.etag": "\"1703161932\"",
    "@odata.id": "/redfish/v1/Systems/Self/USBControllers/DevTypeC_USB_0",
    "@odata.type": "#USBController.v1_0_0.USBController",
    "Id": "DevTypeC_USB_0",
    "Links": {
        "PCIeDevice": {
            "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_CB_04"
        }
    },
    "Name": "DevTypeC_USB_0",
    "Ports": {
        "@odata.id": "/redfish/v1/Systems/Self/USBControllers/DevTypeC_USB_0/Ports"
    },
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    }
}
```
(p.341. The collection's `@odata.context` is `#USBControllersCollection...` (plural) while its `@odata.type` is `#USBControllerCollection...`. PCIe device IDs take the form `bus_device_function` in hex, e.g. `00_CB_04`.)

---

## 3.112 TaskService (p.342)

This resource shall be used to represent a task service for a Redfish implementation. It represents the properties for the service itself and has links to the actual list of tasks.

### 3.112.1 URL
```
/redfish/v1/TaskService
```
### 3.112.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.112.3 Supported Properties

#### Table 130. Supported Properties for TaskService (p.342)

| Property | Type | Read Only | Description |
|---|---|---|---|
| CompletedTaskOverWritePolicy | String | True | The value of this property shall indicate how completed tasks are handled should the task service need to track more tasks. |
| &nbsp;&nbsp;Manual | - | - | Completed tasks are not automatically overwritten. |
| &nbsp;&nbsp;Oldest | - | - | Oldest completed tasks are overwritten. |
| DateTime | String | True | The current DateTime value for the TaskService, with offset from UTC, in Redfish Timestamp format. |
| LifeCycleEventOnTaskStateChange | Boolean | True | An indication of whether a task state change sends an event. |
| ServiceEnabled | Boolean | True | This indicates whether this service is enabled. |
| Status | Object | True | The status and health of the resource and its subordinate or dependent resources. |
| Tasks | Object | True | The links to the collection of tasks. |
| Actions | Object | True | This object will contain the actions for this resource under Oem property if any. |

### 3.112.4 Example
#### 3.112.4.1 GET TaskService Instance (p.342-343)
```
GET https://{BMC_IP}/redfish/v1/TaskService
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#TaskService.TaskService",
    "@odata.etag": "\"1703164815\"",
    "@odata.id": "/redfish/v1/TaskService",
    "@odata.type": "#TaskService.v1_1_4.TaskService",
    "CompletedTaskOverWritePolicy": "Oldest",
    "DateTime": "2023-12-21T14:07:04+00:00",
    "Description": "Task Service",
    "Id": "TaskService",
    "LifeCycleEventOnTaskStateChange": true,
    "Name": "Task Service",
    "ServiceEnabled": true,
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "Tasks": {
        "@odata.id": "/redfish/v1/TaskService/Tasks"
    }
}
```

## 3.113 TaskCollection (p.344)

It displays the collection of links to each task.

### 3.113.1 URL
```
/redfish/v1/TaskService/Tasks
```
### 3.113.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.113.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.113.4 Example
#### 3.113.4.1 GET TaskCollection Instance (p.344)
```
GET https://{BMC_IP}/redfish/v1/TaskService/Tasks
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#TaskCollection.TaskCollection",
    "@odata.etag": "\"1703168332\"",
    "@odata.id": "/redfish/v1/TaskService/Tasks",
    "@odata.type": "#TaskCollection.TaskCollection",
    "Description": "Task Collection",
    "Members": [
        {
            "@odata.id": "/redfish/v1/TaskService/Tasks/1"
        }
    ],
    "Members@odata.count": 1,
    "Name": "Task Collection"
}
```

## 3.114 Task (p.345)

The Task schema contains information about a task that the Redfish task service schedules or executes.

### 3.114.1 URL
```
/redfish/v1/TaskService/Tasks/{Task-Instance}
```
### 3.114.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| DELETE | ConfigureManager |

### 3.114.3 Supported Properties

#### Table 131. Supported Properties for Task (p.345)

| Property | Type | Read Only | Description |
|---|---|---|---|
| TaskState | String | True | The value of this property shall indicate the state of the task. |
| &nbsp;&nbsp;New | - | - | A new task. |
| &nbsp;&nbsp;Starting | - | - | Task is starting. |
| &nbsp;&nbsp;Running | - | - | Task is running normally. |
| &nbsp;&nbsp;Suspended | - | - | Task has been suspended. |
| &nbsp;&nbsp;Interrupted | - | - | Task has been interrupted. |
| &nbsp;&nbsp;Pending | - | - | Task is pending and has not started. |
| &nbsp;&nbsp;Stopping | - | - | Task is in the process of stopping. |
| &nbsp;&nbsp;Completed | - | - | Task was completed. |
| &nbsp;&nbsp;Killed | - | - | Task was terminated. |
| &nbsp;&nbsp;Exception | - | - | Task has stopped due to an exception condition. |
| &nbsp;&nbsp;Service | - | - | Task is running as a service. |
| StartTime | String | True | The date-time stamp that the task was last started. |
| EndTime | String | True | The value of this property shall indicate the time the task was completed. |
| TaskStatus | String | True | The completion status of the task. |
| Messages | Array | True | This is an array of messages associated with the task. |
| Actions | Object | True | This object will contain the actions for this resource under Oem property if any. |
| HidePayload | Boolean | True | If value of this property is true will hide the contents of the Payload otherwise the Payload contents can be returned normally. |
| PercentComplete | Integer | True | Completion percentage of this Task. |
| Payload | Object | True | Refer Below table for Payload property details. |

#### Table 132. Payload Properties (p.345)

| Property | Type | Read Only | Description |
|---|---|---|---|
| HttpHeaders | Array | True | HTTP Headers used in the execution of this Task. |
| HttpOperation | String | True | HTTP Operation to execution for this Task. |
| JsonBody | String | True | JSON Payload used for this Task. |
| TargetUri | String | True | URI of the Target for this Task. |

### 3.114.4 Example

#### 3.114.4.1 GET Task Instance (p.346)
```
GET https://{BMC_IP}/redfish/v1/TaskService/Tasks/{Task-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#Task.Task",
    "@odata.etag": "\"1703168332\"",
    "@odata.id": "/redfish/v1/TaskService/Tasks/1",
    "@odata.type": "#Task.v1_4_2.Task",
    "Description": "Task for Systems - IndicatorLED",
    "EndTime": "2023-12-21T14:18:52+00:00",
    "Id": "1",
    "Messages": [
        {
            "@odata.type": "#Message.v1_0_8.Message",
            "Message": "The task with id 1 has completed.",
            "MessageArgs": [
                "1"
            ],
            "MessageId": "TaskEvent.1.0.TaskCompletedOK",
            "Resolution": "None.",
            "Severity": "OK"
        }
    ],
    "Name": "Systems - IndicatorLED",
    "StartTime": "2023-12-21T14:18:52+00:00",
    "TaskState": "Completed",
    "TaskStatus": "OK"
}
```
Task messages use the DMTF `TaskEvent.1.0` registry (e.g. `TaskEvent.1.0.TaskCompletedOK`) with `Message.v1_0_8`.

#### 3.114.4.2 DELETE Task Instance (p.346)
```
DELETE https://{BMC_IP}/redfish/v1/TaskService/Tasks/{Task-Instance}
Content-Type: application/json
```
Response: status **204** with no response body.

> **Notes for implementers:** Task IDs are small integers that start at 1 and appear to be reused or reset (the TelemetryService example also returns Task 1), and `CompletedTaskOverWritePolicy` is `Oldest`. Poll `/redfish/v1/TaskService/Tasks/{id}` for `TaskState` = `Completed` / `Exception` / `Killed`. The 202 responses in this document carry the Task in the body. A `Location` header is not documented.

---

## 3.115 TelemetryService (p.347)

This resource shall be used to represent a Metrics Service for a Redfish implementation. It represents the properties for the service itself and has links to collections of metric definitions and metric report definitions.

### 3.115.1 URL
```
/redfish/v1/TelemetryService
```
### 3.115.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| POST | ConfigureComponents |

(The PATCH example in 3.115.4.3 has no privilege row in the source. ConfigureComponents or ConfigureManager is presumably required.)

### 3.115.3 Supported Properties

#### Table 133. Supported Properties for TelemetryService (p.347)

| Property | Type | Read Only | Description |
|---|---|---|---|
| MetricDefinitions | Array | True | The link to the collection of metric definitions. |
| MetricReportDefinitions | Array | True | The link to the collection of metric report definitions. |
| MetricReports | Array | True | The link to the collection of metric reports. |
| Triggers | Array | True | The link to the collection of triggers that apply to metrics. |
| Status | Object | True | The status and health of the resource and its subordinate or dependent resources. |
| Name | String | True | Name of the Collection. |
| MaxReports | Number | True | The maximum number of MetricReports that are supported by this service. **Limited to 5.** |
| MinCollectionInterval | String | True | The minimum supported interval between collections. The value shall be an ISO 8601 duration specifying the minimum time between collections. **Limited to PT10M.** |
| SupportedCollectionFunctions | Array | True | Function to perform over each sample. |
| SupportedCollectionFunctions@Redfish.AllowableValues | Array | True | This attribute signifies the set of allowable values for the SupportedCollectionFunctions property. |
| Actions | Object | True | The Actions object contains the available custom actions on this resource like SubmitTestMetricReport or any Oem Action. |
| LogService | Object | True | This is a reference to a Log Service used by the Telemetry Service. |
| ServiceEnabled | Boolean | False | This property shall indicate whether the telemetry service is in enabled state or not. |

### 3.115.4 Example

#### 3.115.4.1 GET TelemetryService Instance (p.347-349)
```
GET https://{BMC_IP}/redfish/v1/TelemetryService
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#TelemetryService.TelemetryService",
    "@odata.etag": "\"1703168332\"",
    "@odata.id": "/redfish/v1/TelemetryService",
    "@odata.type": "#TelemetryService.v1_2_1.TelemetryService",
    "Actions": {
        "#TelemetryService.SubmitTestMetricReport": {
            "@Redfish.ActionInfo": "/redfish/v1/TelemetryService/SubmitTestMetricReportActionInfo",
            "target": "/redfish/v1/TelemetryService/Actions/TelemetryService.SubmitTestMetricReport"
        }
    },
    "Description": "TelemetryService",
    "Id": "TelemetryService",
    "LogService": {
        "@odata.id": "/redfish/v1/TelemetryService/LogService"
    },
    "MaxReports": 5,
    "MetricDefinitions": {
        "@odata.id": "/redfish/v1/TelemetryService/MetricDefinitions"
    },
    "MetricReportDefinitions": {
        "@odata.id": "/redfish/v1/TelemetryService/MetricReportDefinitions"
    },
    "MetricReports": {
        "@odata.id": "/redfish/v1/TelemetryService/MetricReports"
    },
    "MinCollectionInterval": "PT10M",
    "Name": "TelemetryService",
    "ServiceEnabled": true,
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "SupportedCollectionFunctions": [
        "Minimum",
        "Summation",
        "Maximum",
        "Average"
    ],
    "SupportedCollectionFunctions@Redfish.AllowableValues": [
        "Average",
        "Maximum",
        "Summation",
        "Minimum"
    ],
    "Triggers": {
        "@odata.id": "/redfish/v1/TelemetryService/Triggers"
    }
}
```

#### 3.115.4.2 POST TelemetryService Instance (SubmitTestMetricReport action) (p.349)

##### Table 134. Supported Properties for TelemetryService Post Service (p.349)

| Property | Type | Read Only | Description |
|---|---|---|---|
| MetricReportName | String | False | The Name property of the metric report in generated metric report. This parameter shall be the value of the Name property in the generated metric report. |
| GeneratedMetricReportValues | Array | False | This parameter shall contain the contents of the MetricReportValues array property in the generated metric report. |
| MetricId | String | False | The metric definitions identifier for this metric. |
| MetricProperty | String | False | The URI for the property from which this metric is derived. This property shall contain a URI following RFC6901-specified JSON pointer notation to the property from which this metric is derived. |
| MetricValue | String | False | The value of the metric represented as a string. |
| Timestamp | String | False | The time when the value of the metric is obtained. |

Request:
```
POST https://{BMC_IP}/redfish/v1/TelemetryService/Actions/TelemetryService.SubmitTestMetricReport
Content-Type: application/json
```
Example request body:
```json
{
    "MetricReportName": "Average2",
    "GeneratedMetricReportValues": [
        {
            "MetricId": "Temp_average_reading_Average",
            "MetricProperty": "/redfish/v1/Chassis/Self/Thermal#/Temperatures/1/ReadingCelsius",
            "MetricValue": "23",
            "Timestamp": "2023-12-23T15:11:00+08:00"
        }
    ]
}
```
Response: status **202 Accepted**, with the created Task instance as the body:
```json
{
    "@odata.context": "/redfish/v1/$metadata#Task.Task",
    "@odata.id": "/redfish/v1/TaskService/Tasks/1",
    "@odata.type": "#Task.v1_4_2.Task",
    "Description": "Task for TelemetryService SubmitTestMetricReport Action",
    "Id": "1",
    "Name": "TelemetryService SubmitTestMetricReport Action",
    "TaskState": "New"
}
```

#### 3.115.4.3 PATCH TelemetryService Instance (p.350)
```
PATCH https://{BMC_IP}/redfish/v1/TelemetryService
Content-Type: application/json
```
Example request body:
```json
{
    "ServiceEnabled": true
}
```
Response: status **204** with no body.

## 3.116 MetricDefinitionCollection (p.351)

Metric Definitions are the metadata of the metrics and provide details about the characteristics of readings and the calculations for statistics.

### 3.116.1 URL
```
/redfish/v1/TelemetryService/MetricDefinitions
```
### 3.116.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.116.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.116.4 Example
#### 3.116.4.1 GET MetricDefinitionCollection Instance (p.351)
```
GET https://{BMC_IP}/redfish/v1/TelemetryService/MetricDefinitions
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#MetricDefinitionCollection.MetricDefinitionCollection",
    "@odata.etag": "\"1703168332\"",
    "@odata.id": "/redfish/v1/TelemetryService/MetricDefinitions",
    "@odata.type": "#MetricDefinitionCollection.MetricDefinitionCollection",
    "Description": "Metric Definitions Collection",
    "Members": [
        {
            "@odata.id": "/redfish/v1/TelemetryService/MetricDefinitions/Voltage_Reading"
        }
    ],
    "Members@odata.count": 1,
    "Name": "MetricDefinitions"
}
```

## 3.117 MetricDefinition (p.352)

Metric Definitions are the metadata of the metrics and provide details about the characteristics of readings and the calculations for statistics.

### 3.117.1 URL
```
/redfish/v1/TelemetryService/MetricDefinitions/{MetricDefinition-Instance}
```
### 3.117.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.117.3 Supported Properties

#### Table 135. Supported Properties for MetricDefinition (p.352-353)

| Property | Type | Read Only | Description |
|---|---|---|---|
| MetricDataType | String | True | This property shall specify the data-type of the metric. |
| &nbsp;&nbsp;Boolean | - | - | The JSON boolean definition. |
| &nbsp;&nbsp;DateTime | - | - | The JSON string definition with the date-time format. |
| &nbsp;&nbsp;Decimal | - | - | The JSON decimal definition. |
| &nbsp;&nbsp;Enumeration | - | - | The JSON string definition with a set of defined enumerations. |
| &nbsp;&nbsp;Integer | - | - | The JSON integer definition. |
| &nbsp;&nbsp;String | - | - | The JSON string definition. |
| MetricType | String | True | This property shall specify the type of metric. |
| &nbsp;&nbsp;Numeric | - | - | The metric is a numeric metric. The metric value is any real number. |
| Implementation | String | True | This value of this property shall designate how the sensor is implemented. |
| &nbsp;&nbsp;Calculated | - | - | The metric is implemented by applying a calculation on another metric property. |
| &nbsp;&nbsp;DigitalMeter | - | - | The metric is implemented as digital meter. |
| &nbsp;&nbsp;PhysicalSensor | - | - | The metric is implemented as a physical sensor. |
| &nbsp;&nbsp;Synthesized | - | - | The metric is implemented by applying a calculation on one or more metric properties. |
| Units | String | True | Units of measure for this metric. |
| IsLinear | Boolean | True | The value shall specify that the corresponding metric values shall be linear or non-linear. |
| MetricProperties | Array | True | A collection of URI for the properties on which this metric definition is defined. |
| Precision | Number | True | The value of the property shall specify the number of significant digits in the MetricValue. |
| Accuracy | Number | True | The estimated percent error of measured versus actual values. |
| Calculable | String | True | An indication of whether the metric can be used in a calculation. |
| CalculationAlgorithm | String | True | The calculation that is performed on a source metric to obtain the metric being defined. |
| &nbsp;&nbsp;Average | - | - | The metric is calculated as the average metric reading over a sliding time interval. |
| &nbsp;&nbsp;Maximum | - | - | The metric is calculated as the maximum metric reading over during a time interval. |
| &nbsp;&nbsp;Minimum | - | - | The metric is calculated as the minimum metric reading over a sliding time interval. |
| CalculationTimeInterval | String | True | The value of this property shall specify the time interval over the metric calculation is performed. |
| PhysicalContext | String | True | The value of this property shall specify the physical context of the metric. |

### 3.117.4 Example
#### 3.117.4.1 GET MetricDefinition Instance (p.353)
```
GET https://{BMC_IP}/redfish/v1/TelemetryService/MetricDefinitions/{MetricDefinition-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#MetricDefinition.MetricDefinition",
    "@odata.etag": "\"1703148667\"",
    "@odata.id": "/redfish/v1/TelemetryService/MetricDefinitions/Voltage_Reading",
    "@odata.type": "#MetricDefinition.v1_0_3.MetricDefinition",
    "Accuracy": 5,
    "Description": "Voltage Reading",
    "Id": "Voltage_Reading",
    "Implementation": "PhysicalSensor",
    "IsLinear": true,
    "MetricDataType": "Decimal",
    "MetricProperties": [
        "/redfish/v1/Chassis/Self/Power#/Voltages/0/ReadingVolts",
        "/redfish/v1/Chassis/Self/Power#/Voltages/1/ReadingVolts",
        "/redfish/v1/Chassis/Self/Power#/Voltages/9/ReadingVolts"
    ],
    "MetricType": "Numeric",
    "Name": "Voltage Reading",
    "Precision": 3,
    "Units": "V"
}
```

## 3.118 MetricReportDefinitionCollection (p.354)

This resource specifies a set of metrics that shall be collected into a metric report. It displays a collection of Metric Report Definitions.

### 3.118.1 URL
```
/redfish/v1/TelemetryService/MetricReportDefinitions
```
### 3.118.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| POST | ConfigureComponents |

### 3.118.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.118.4 Example

#### 3.118.4.1 GET MetricReportDefinitionCollection Instance (p.354)
```
GET https://{BMC_IP}/redfish/v1/TelemetryService/MetricReportDefinitions
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#MetricReportDefinitionCollection.MetricReportDefinitionCollection",
    "@odata.etag": "\"1703170216\"",
    "@odata.id": "/redfish/v1/TelemetryService/MetricReportDefinitions",
    "@odata.type": "#MetricReportDefinitionCollection.MetricReportDefinitionCollection",
    "Description": "Metric Report Definitions Collection",
    "Members": [],
    "Members@odata.count": 0,
    "Name": "MetricReportDefinitions"
}
```

#### 3.118.4.2 POST MetricReportDefinitionCollection Instance with MetricProperties (p.354-355)

The following properties are mandatory to create a Metric Report Definition:
- **Id**
- **Name**
- **Schedule.RecurrenceInterval**
- **MetricReport**
- **MetricReportDefinitionType**
- **MetricProperties or Metrics**

Request (the source mistakenly prints `GET`; the operation is a POST):
```
POST https://{BMC_IP}/redfish/v1/TelemetryService/MetricReportDefinitions
Content-Type: application/json
```
Example request body (as printed; note the invalid trailing comma after the last array element in the source):
```json
{
    "Id": "TemperatureReport",
    "Name": "Temperature_Report",
    "Description": "Temperature Report",
    "Schedule": {
        "RecurrenceInterval": "PT10M"
    },
    "MetricReportDefinitionType": "Periodic",
    "MetricReport": {
        "@odata.id": "/redfish/v1/TelemetryService/MetricReports/TemperatureReport"
    },
    "MetricProperties": [
        "/redfish/v1/Chassis/Self/Thermal#/Temperatures/13/ReadingCelsius",
        "/redfish/v1/Chassis/Self/Thermal#/Temperatures/14/ReadingCelsius",
    ]
}
```
Response: "The response status is 201 and the response body." **The example response box is empty in the PDF.**

#### 3.118.4.3 POST MetricReportDefinitionCollection Instance with Metrics (p.355-356)

Mandatory properties are the same as above (Id, Name, Schedule.RecurrenceInterval, MetricReport, MetricReportDefinitionType, MetricProperties or Metrics).

Request (again printed as `GET` in the source; it is a POST):
```
POST https://{BMC_IP}/redfish/v1/TelemetryService/MetricReportDefinitions
Content-Type: application/json
```
Example request body:
```json
{
    "Id": "AverageTemperatureReport",
    "Name": "Average_Temperature_Report",
    "Description": "Average Temperature Report",
    "Schedule": {
        "RecurrenceInterval": "PT10M"
    },
    "MetricReportDefinitionType": "Periodic",
    "MetricReport": {
        "@odata.id": "/redfish/v1/TelemetryService/MetricReports/AverageTemperatureReport"
    },
    "Metrics": [
        {
            "MetricId": "AverageTemperature",
            "CollectionTimeScope": "Interval",
            "MetricProperties": [
                "/redfish/v1/Chassis/Self/Thermal#/Temperatures/13/ReadingCelsius",
                "/redfish/v1/Chassis/Self/Thermal#/Temperatures/14/ReadingCelsius"
            ],
            "CollectionDuration": "PT10M",
            "CollectionFunction": "Average"
        }
    ]
}
```
Response: status **201** with a response body. **The example response box is empty in the PDF.**

## 3.119 MetricReportDefinition (p.357)

The Metric Report Definition resource specifies the metric report that the Redfish service will create. The Metric Reports are updated periodically based on the recurrence interval specified.

### 3.119.1 URL
```
/redfish/v1/TelemetryService/MetricReportDefinitions/{MetricReportDefinitions-Instance}
```
### 3.119.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureComponents |
| DELETE | ConfigureComponents |

### 3.119.3 Supported Properties

#### Table 136. Supported Properties for MetricReportDefinition (p.357-358)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Schedule | Object | True | The schedule for generating the metric report. Only **RecurrenceInterval** is supported currently. **Limited to PT10M.** |
| MetricReportDefinitionType | String | True | The value shall specify when the corresponding metric values are collected by the underlying instrumentation. Only **Periodic** is supported currently. |
| MetricProperties | Array | True | This property shall list the metric properties to include in the metric report. |
| MetricReport | Object | True | The value of this property shall be a reference to the resource where the resultant metric report is placed. |
| MetricReportDefinitionEnabled | Boolean | False | The value of this property shall be a boolean indicating whether this MetricReportDefinition is enabled for generating new MetricReports. |
| SuppressRepeatedMetricValue | Boolean | False | The value of this property shall indicate whether suppression of Metric information has been enabled or not. |
| MetricReportHeartbeatInterval | String | False | This property specifies an interval to send complete MetricReport regardless of whether values have changed. |
| ReportTimespan | String | False | This property shall specify the timespan duration that this metric report covers. |
| AppendLimit | Integer | True | This property shall contain a number that indicates the maximum number of entries that can be appended to a metric report. |
| ReportActions | Array | True | This property specifies the set of actions to perform when a metric report is generated and should be any one of the enum values: **LogToMetricReportsCollection**, **RedfishEvent**. |
| ReportUpdates | String | True | This property shall contain how subsequent metric reports are handled in relationship to an existing metric report created from the metric report definition. It specifies whether to overwrite, append, or create a report Resource. The allowable values for this property are: **Overwrite**, **AppendWrapsWhenFull**, **AppendStopsWhenFull**, **NewReport**. |
| Links | Object | True | The Links property, as described by the Redfish Specification, shall contain references to resources that are related to, but not contained by (subordinate to), this resource. |
| &nbsp;&nbsp;Oem | Object | - | This property shall contain the OEM extensions. |
| &nbsp;&nbsp;Triggers | Array | True | The value shall be a set of references to triggers that will cause this metric report definition to generate a new metric report upon a trigger occurrence when the TriggerActions property contains the value RedfishMetricReport. |
| &nbsp;&nbsp;Triggers@odata.count | Integer | True | An integer representing the number of items in a collection. |
| Metrics | Object | True | A collection of metrics specifying the CollectionFunction and the MetricProperties to apply these functions. |
| &nbsp;&nbsp;MetricId | | | Id of the metric. |
| &nbsp;&nbsp;CollectionDuration | | | Time interval. **Limited to greater than or equal to PT10M.** |
| &nbsp;&nbsp;CollectionFunction | String | False | Specifies the function to perform on each of the metric properties listed in the MetricProperties property or the metric properties specified in the MetricDefinition referenced by the MetricId property. |
| &nbsp;&nbsp;&nbsp;&nbsp;Average | - | - | The metric is calculated as the average metric reading over a duration. |
| &nbsp;&nbsp;&nbsp;&nbsp;Maximum | - | - | The metric is calculated as the maximum metric reading over a duration. |
| &nbsp;&nbsp;&nbsp;&nbsp;Minimum | - | - | The metric is calculated as the minimum metric reading over a duration. |
| &nbsp;&nbsp;&nbsp;&nbsp;Summation | - | - | The metric is calculated as the sum of the values over a duration. |
| &nbsp;&nbsp;CollectionTimeScope | | | The time scope of the corresponding metric values. Only **Interval** is supported. |
| &nbsp;&nbsp;MetricProperties | | | A collection of URI for the metric properties to include in the metric report. |

> **Telemetry limits (differ from DMTF flexibility):** at most 5 reports (`MaxReports`). `Schedule.RecurrenceInterval` and `MinCollectionInterval` are limited to PT10M, and `CollectionDuration` must be at least PT10M. Only the `Periodic` type and the `Interval` time scope are supported.

### 3.119.4 Example

#### 3.119.4.1 GET MetricReportDefinition Instance (p.358)
```
GET https://{BMC_IP}/redfish/v1/TelemetryService/MetricReportDefinitions/{MetricReportDefinitions-Instance}
Content-Type: application/json
```
Response: **The example response box is empty in the PDF.**

#### 3.119.4.2 PATCH MetricReportDefinition Instance (p.358-359)
```
PATCH https://{BMC_IP}/redfish/v1/TelemetryService/MetricReportDefinitions/{MetricReportDefinitions-Instance}
Content-Type: application/json
```
Example request body:
```json
{
    "MetricReportDefinitionEnabled": true,
    "SuppressRepeatedMetricValue": true,
    "MetricReportHeartbeatInterval": "PT15M"
}
```
Response: status **204** with no body.

#### 3.119.4.3 DELETE MetricReportDefinition Instance (p.359)
```
DELETE https://{BMC_IP}/redfish/v1/TelemetryService/MetricReportDefinitions/{MetricReportDefinitions-Instance}
Content-Type: application/json
```
Response: status **204** with no body.

## 3.120 MetricReportCollection (p.360)

This resource specifies an abstract metric value. It displays a collection of Metric Reports.

### 3.120.1 URL
```
/redfish/v1/TelemetryService/MetricReports
```
### 3.120.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.120.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.120.4 Example
#### 3.120.4.1 GET MetricReportCollection Instance (p.360)
```
GET https://{BMC_IP}/redfish/v1/TelemetryService/MetricReports
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#MetricReportCollection.MetricReportCollection",
    "@odata.etag": "\"1703170216\"",
    "@odata.id": "/redfish/v1/TelemetryService/MetricReports",
    "@odata.type": "#MetricReportCollection.MetricReportCollection",
    "Description": "Metric Reports Collection",
    "Members": [],
    "Members@odata.count": 0,
    "Name": "MetricReports"
}
```

## 3.121 MetricReport (p.361)

The Metric Report Instance resource specifies the metric report that the Redfish service will create, corresponding to the Metric Report Definition. The Metric Reports are updated periodically based on the recurrence interval specified.

### 3.121.1 URL
```
/redfish/v1/TelemetryService/MetricReports/{MetricReport-Instance}
```
### 3.121.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.121.3 Supported Properties

#### Table 137. Supported Properties for MetricReport (p.361)

| Property | Type | Read Only | Description |
|---|---|---|---|
| MetricReportDefinition | Object | True | The value shall be reference to the metric definition for this metric. |
| Timestamp | String | True | This property represents time associated with the metric report in its entirety. This property shall contain the time when the metric report was generated. |
| ReportSequence | String | True | This property shall contain the current sequence identifier for this metric report. |
| MetricValues | Object | True | The values shall be metric values for this MetricReport. |
| &nbsp;&nbsp;MetricId | String | True | The value shall be the Identifier of the source metric within the associated MetricDefinition. |
| &nbsp;&nbsp;MetricValue | String | True | The value of the metric represented as a string. |
| &nbsp;&nbsp;TimeStamp | String | True | The value shall be an ISO 8601 date time for when the metric value was computed. |
| &nbsp;&nbsp;MetricProperty | String | True | The value shall be an OData conformant URI to a property contained in the scope of the MetricScope. |

### 3.121.4 Example
#### 3.121.4.1 GET MetricReport Instance (p.361)
```
GET https://{BMC_IP}/redfish/v1/TelemetryService/MetricReports/{MetricReport-Instance}
Content-Type: application/json
```
Response: **The example response box is empty in the PDF.**

## 3.122 TriggerCollection (p.362)

Triggers are entities that sense or measure any sort of change or deviation in the properties of various Redfish resources.

There are two types of triggers: numeric and discrete.

Triggers are not limited to Voltage, Temp and Fan sensors. Any property can be monitored through triggers.

The Triggers Collection resource displays a collection of triggers, both numeric and discrete.

### 3.122.1 URL
```
/redfish/v1/TelemetryService/Triggers
```
### 3.122.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| POST | ConfigureComponents |

### 3.122.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.122.4 Example

#### 3.122.4.1 GET TriggerCollection Instance (p.362)
```
GET https://{BMC_IP}/redfish/v1/TelemetryService/Triggers
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#TriggersCollection.TriggersCollection",
    "@odata.etag": "\"1703322914\"",
    "@odata.id": "/redfish/v1/TelemetryService/Triggers",
    "@odata.type": "#TriggersCollection.TriggersCollection",
    "Description": "Triggers Collection",
    "Members": [],
    "Members@odata.count": 0,
    "Name": "Triggers"
}
```

#### 3.122.4.2 POST TriggerCollection Instance to Create Discrete Trigger (p.362-363)

The following properties are mandatory to create a trigger:
- **Id**
- **Name**
- **MetricType**
- **TriggerActions**
- **NumericThresholds or (DiscreteTriggerCondition & DiscreteTriggers)**
- **MetricProperties**

Request:
```
POST https://{BMC_IP}/redfish/v1/TelemetryService/Triggers
Content-Type: application/json
```
Example request body:
```json
{
    "Id": "ChassisIndicatorLED",
    "Name": "Chassis_IndicatorLED",
    "MetricType": "Discrete",
    "TriggerActions": [
        "LogToLogService",
        "RedfishEvent"
    ],
    "DiscreteTriggerCondition": "Specified",
    "DiscreteTriggers": [
        {
            "Name": "LED_Blinking",
            "Value": "Blinking",
            "Severity": "Critical",
            "DwellTime": "PT30S"
        }
    ],
    "MetricProperties": [
        "/redfish/v1/Chassis/Self#/IndicatorLED"
    ]
}
```
Response: status **201**. The body is a GET response with the properties of the newly created instance. **The example response box is empty in the PDF.**

#### 3.122.4.3 POST TriggerCollection Instance to Create Numeric Trigger (p.363-364)

Mandatory properties are the same as for a discrete trigger (Id, Name, MetricType, TriggerActions, NumericThresholds or (DiscreteTriggerCondition & DiscreteTriggers), MetricProperties).

Request:
```
POST https://{BMC_IP}/redfish/v1/TelemetryService/Triggers
Content-Type: application/json
```
Example request body:
```json
{
    "Id": "TemperatureTrigger",
    "Name": "Temperature Trigger",
    "Description": "Temperature_Trigger",
    "MetricType": "Numeric",
    "TriggerActions": [
        "LogToLogService",
        "RedfishEvent"
    ],
    "NumericThresholds": {
        "LowerCritical": {
            "Activation": "Either",
            "DwellTime": "PT1M",
            "Reading": 30
        },
        "LowerWarning": {
            "Activation": "Decreasing",
            "DwellTime": "PT1M30S",
            "Reading": 50
        },
        "UpperCritical": {
            "Activation": "Increasing",
            "DwellTime": "PT1M50S",
            "Reading": 80
        },
        "UpperWarning": {
            "Activation": "Increasing",
            "DwellTime": "PT30S",
            "Reading": 70
        }
    },
    "MetricProperties": [
        "/redfish/v1/Chassis/Self/Thermal#/Temperatures/13/ReadingCelsius",
        "/redfish/v1/Chassis/Self/Thermal#/Temperatures/14/ReadingCelsius"
    ]
}
```
Response: status **201**. The body is a GET response with the properties of the newly created instance. **The example response box is empty in the PDF.**

(The example sends `Reading` as a JSON number while Table 139 types it as String.)

## 3.123 Trigger (p.365)

It displays a trigger instance, which can be either numeric or discrete.

### 3.123.1 URL
```
/redfish/v1/TelemetryService/Triggers/{Trigger-Instnace}
```
### 3.123.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| DELETE | ConfigureComponents |

(No PATCH. To modify a trigger, delete it and re-create it.)

### 3.123.3 Supported Properties

#### Table 138. Supported Properties for Trigger (p.365-366)

| Property | Type | Read Only | Description |
|---|---|---|---|
| MetricType | String | True | The value of this property shall specify the type of trigger and should be any one of the enum values: **Numeric**, **Discrete**. *Note: When the value of MetricType is Numeric, the DiscreteTriggerCondition and DiscreteTriggers attributes are not allowed. When the value of MetricType is Discrete, the NumericThresholds attribute is not allowed.* |
| Oem | Object | False | This object represents the attributes supported by OEM under the Oem property. It can also contain an object of type OemObject. |
| &nbsp;&nbsp;@odata.type | String | True | The value of this property shall be a URI fragment that specifies the type of the resource and it shall be of the form defined in the Redfish specification. |
| &nbsp;&nbsp;ControlTriggerAlerts | String | True | This property is only applicable in the case of Numeric Triggers. This property controls the trigger actions (LogToLogService/RedfishEvent/RedfishMetricReport) for Numeric Triggers. (OEM extension) |
| TriggerActions | Array | True | This property denotes the actions to be performed when a trigger condition is met. |
| &nbsp;&nbsp;LogToLogService | - | - | When a trigger condition is met, record in a log. |
| &nbsp;&nbsp;RedfishEvent | - | - | When a trigger condition is met, the service sends an event to subscribers. |
| &nbsp;&nbsp;RedfishMetricReport | - | - | When a trigger condition is met, force an update of the specified metric reports. |
| NumericThresholds | Object | True | This property shall contain list of triggers to which a sensor reading will be compared. |
| &nbsp;&nbsp;LowerCritical | Object | True | The value of this property shall indicate the Reading is below the normal range and may require attention. Refer to Table 139, Numeric Thresholds Sub-Attributes. |
| &nbsp;&nbsp;LowerWarning | Object | True | The value of this property shall indicate the Reading is below the normal range. Refer to Table 139. |
| &nbsp;&nbsp;UpperCritical | Object | True | The value of this property shall indicate the Reading is above the normal range and may require attention. Refer to Table 139. |
| &nbsp;&nbsp;UpperWarning | Object | True | The value of this property shall indicate the Reading is above the normal range. Refer to Table 139. |
| Links | Object | True | The Links property, as described by the Redfish Specification, shall contain references to resources that are related to, but not contained by (subordinate to), this resource. |
| &nbsp;&nbsp;Oem | Object | - | This property shall contain the OEM extensions. |
| &nbsp;&nbsp;MetricReportDefinitions | Array | True | The value shall be a set of references to existing MetricReportDefinitions that will generate new metric reports when the TriggerActions property contains the value RedfishMetricReport and a trigger condition is met. |
| &nbsp;&nbsp;MetricReportDefinitions@odata.count | Integer | True | An integer representing the number of items in a collection. |
| DiscreteTriggerCondition | String | True | This property shall contain the conditions when a discrete metric needs to trigger. |
| &nbsp;&nbsp;Changed | - | - | A discrete trigger condition is met whenever the metric value changes. |
| &nbsp;&nbsp;Specified | - | - | A discrete trigger condition is met when the metric value becomes one of the values that the DiscreteTriggers property lists. |
| Metricproperties [sic] | Array | True | A collection of URI for the properties on which this metric definition is defined. |
| DiscreteTriggers | Array | True | This property shall contain list of triggers to which a sensor reading will be compared. |
| &nbsp;&nbsp;Name | String | True | The name of trigger. |
| &nbsp;&nbsp;Value | String | True | The discrete metric value that constitutes a trigger event. |
| &nbsp;&nbsp;Severity | String | True | The severity of the event message. |
| &nbsp;&nbsp;DwellTime | String | True | The amount of time that a trigger event persists before the metric action is performed. Supported format for DwellTime is `"-?(P)(T(\\d+H)?(\\d+M)?(\\d+(.\\d+)?S)?)?"`. *Note: Days (D) are not supported; only Hours (H), Minutes (M) and Seconds (S) are supported.* |

#### Table 139. Numeric Thresholds Sub-Attributes (p.366)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Reading | String | False | This property shall indicate the Reading value of this Sensor that triggers the threshold. |
| DwellTime | String | False | This property shall indicate the time interval over which the sensor reading must have passed through this Threshold value before the threshold is considered to be violated. |
| Activation | String | False | This property shall indicate the direction of crossing of the Reading value for this Sensor that triggers the threshold. |
| &nbsp;&nbsp;Increasing | - | - | Value increases above the threshold. |
| &nbsp;&nbsp;Decreasing | - | - | Value decreases below the threshold. |
| &nbsp;&nbsp;Either | - | - | Value crosses the threshold in either direction. |

> **Caveat:** ISO 8601 durations with a day component (e.g. `P1D`, `P1DT2H`) are rejected for `DwellTime`. Only `PT#H#M#S` forms are accepted.

### 3.123.4 Example

#### 3.123.4.1 GET Trigger Instance (p.366-367)
```
GET https://{BMC_IP}/redfish/v1/TelemetryService/Triggers/{Trigger-Instnace}
Content-Type: application/json
```
Response: **The example response box is empty in the PDF.**

#### 3.123.4.2 DELETE Trigger Instance (p.367)
```
DELETE https://{BMC_IP}/redfish/v1/TelemetryService/Triggers/{Trigger-Instnace}
Content-Type: application/json
```
Response: status **204** with no body.

---

## 3.124 UpdateService (p.368)

This is the schema definition for the Update Service and its properties.

### 3.124.1 URL
```
/redfish/v1/UpdateService/
```
### 3.124.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureComponents |
| POST | ConfigureComponents |

### 3.124.3 Supported Properties

#### Table 140. Supported Properties for UpdateService (p.368)

| Property | Type | Read Only | Description |
|---|---|---|---|
| ServiceEnabled | Boolean | False | This indicates whether this service is enabled. |
| Actions | Object | True | UpdateService allows the user to perform the UpdateService.SimpleUpdate Action. |
| &nbsp;&nbsp;UpdateService.UploadCABundle | - | - | Upload CA Bundle. |
| FirmwareInventory | Object | True | This property shall contain a link to a Resource of type SoftwareInventoryCollection. |
| MaxImageSizeBytes | Integer | False | The maximum size in bytes of the software update image that this Service supports. |
| MultipartHttpPushUri | String | False | The URI used to perform a Redfish Specification-defined Multipart HTTP or HTTPS push update to the Update Service. |

#### Table 141. UpdateService Oem Object (p.368)

| Property | Type | Read Only | Description |
|---|---|---|---|
| AMIUpdateService | Object | True | Contains information related to AMI features supported by the Update Service. |
| &nbsp;&nbsp;@odata.type | String | True | The value of this property shall be a URI fragment that specifies the type of the resource and it shall be of the form defined in the Redfish specification. |
| &nbsp;&nbsp;FlashPercentage | String | True | Percentage of flash done. |
| &nbsp;&nbsp;UpdateStatus | String | True | Stage of UpdateService. |
| &nbsp;&nbsp;UpdateTarget | String | True | Update Target. |
| &nbsp;&nbsp;PreserveConfiguration | Object | True | The Preserve Configuration info. (Writable via PATCH in practice; see 3.124.4.2.) |

#### Table 142. UpdateService - PreserveConfiguration (p.368-369)

All entries are Boolean, Read Only = False. Each one sets whether that configuration area is preserved across a BMC firmware flash.

| Property | Type | Read Only | Description |
|---|---|---|---|
| SDR | Boolean | False | To preserve SDR. |
| FRU | Boolean | False | To preserve FRU. |
| SEL | Boolean | False | To preserve SEL. |
| IPMI | Boolean | False | To preserve IPMI. |
| Network | Boolean | False | To preserve Network. |
| NTP | Boolean | False | To preserve NTP. |
| SNMP | Boolean | False | To preserve SNMP. |
| SSH | Boolean | False | To preserve SSH. |
| KVM | Boolean | False | To preserve KVM. |
| Authentication | Boolean | False | To preserve Authentication. |
| Syslog | Boolean | False | To preserve Syslog. |
| CMX | Boolean | False | To preserve CMX. |
| WEB | Boolean | False | To preserve WEB. |
| EXTLOG | Boolean | False | To preserve EXTLOG. |
| REDFISH | Boolean | False | To preserve REDFISH. |
| AUTOMATION_ENGINE | Boolean | False | To preserve AUTOMATION_ENGINE. |

#### Table 143. UpdateService - SimpleUpdateActionInfo Properties (p.369)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Parameters | Object | | A parameter associated with the specified Redfish Action. |

#### Table 144. UpdateService - SimpleUpdateActionInfo - Parameters Properties (p.369)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Name | String | True | The name of the parameter for this Action. |
| Required | Boolean | True | Indicates whether the parameter is required to perform this Action. |
| DataType | String | True | The JSON property type used for this parameter. |
| AllowableValues | Array | True | A list of values for this parameter supported by this Action target. |

#### Table 145. UpdateService - Actions - SimpleUpdate Properties (p.369)

| Property | Type | Read Only | Description |
|---|---|---|---|
| TransferProtocol | String | False | The network protocol that the update service uses to retrieve the software image file located at the URI provided in ImageURI. |
| &nbsp;&nbsp;HTTP | - | - | HTTP protocol. |
| &nbsp;&nbsp;HTTPS | - | - | HTTPS protocol. |
| &nbsp;&nbsp;FTP | - | - | File Transfer Protocol. |
| ImageURI | String | False | The URI of the software image to install. |
| User | String | False | User for FTP TransferProtocol. |
| Password | String | False | Password for FTP TransferProtocol. |

(`User`/`Password` are non-DMTF parameter names. DMTF SimpleUpdate uses `Username`/`Password`.)

#### Table 146. UpdateService - Response Properties (p.369)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Messages | Array | True | This property shall contain an array of messages associated with the settings. |

#### Table 147. UpdateService - Actions - MultipartHttpPush Properties (p.369)

| Property | Type | Read Only | Description |
|---|---|---|---|
| UpdateFile | File | False | Image binary for update. |
| UpdateParameters | File | False | DMTF defined standard parameters in json format. Refer to Table 148. |
| OemParameters | File | False | OEM parameters in json format. Refer to Table 149. |

#### Table 148. UpdateService - Actions - MultipartHttpPush UpdateParameters Properties (p.370)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Targets | Array | False | An array of URIs that indicate where to apply the update image. If this parameter is not present or contains no targets, the Service shall apply the software image to all applicable targets, as determined by the Service. |
| &nbsp;&nbsp;/redfish/v1/UpdateService/FirmwareInventory/BMC | - | - | Indicate to update BMC firmware. |
| &nbsp;&nbsp;/redfish/v1/UpdateService/FirmwareInventory/BMCImage1 | - | - | Indicate to update BMC Image1 for dual image. |
| &nbsp;&nbsp;/redfish/v1/UpdateService/FirmwareInventory/BMCImage2 | - | - | Indicate to update BMC Image2 for dual image. |
| &nbsp;&nbsp;/redfish/v1/UpdateService/FirmwareInventory/BIOS | - | - | Indicate to update BIOS component. |
| &nbsp;&nbsp;/redfish/v1/UpdateService/FirmwareInventory/ME | - | - | Indicate to update ME component. |
| &nbsp;&nbsp;/redfish/v1/UpdateService/FirmwareInventory/CPLD | - | - | Indicate to update CPLD component. |
| &nbsp;&nbsp;/redfish/v1/UpdateService/FirmwareInventory/MMC | - | - | Indicate to update MMC component. |
| &nbsp;&nbsp;/redfish/v1/UpdateService/FirmwareInventory/RAID_MSCC_CONTROLLER | - | - | Indicate to update RAID MSCC CONTROLLER component. |
| &nbsp;&nbsp;/redfish/v1/UpdateService/FirmwareInventory/RAID_BRCM_CONTROLLER | - | - | Indicate to update RAID BRCM CONTROLLER component. |

#### Table 149. UpdateService - Actions - MultipartHttpPush OemParameters Properties (p.370)

| Property | Type | Read Only | Description |
|---|---|---|---|
| ImageType | String | False | Indicate the image type. |
| &nbsp;&nbsp;BMC | - | - | Indicate uploaded file is a signed BMC image. |
| &nbsp;&nbsp;HPM | - | - | Indicate uploaded file is an image in HPM format. |
| &nbsp;&nbsp;BIOS | - | - | Indicate uploaded file is a signed BIOS image. |
| &nbsp;&nbsp;PFR | - | - | Indicate uploaded file is a PFR update capsule. |
| defer | Boolean | False | Indicate PFR update is deferred until reboot. |
| active | Boolean | False | Indicate PFR update image on Active region. |
| recovery | Boolean | False | Indicate PFR update image on Recovery region. |

#### Table 150. UpdateService - Actions - UpdateService.UploadCABundle Properties (p.370-371)

| Property | Type | Read Only | Description |
|---|---|---|---|
| ca_bundle | File | False | Indicate the CA bundle file. |
| CA bundle path | - | - | Provides a CA bundle file in pem format which contains all required RootCA certificates and intermediate certificates for verifying the SSL certificate of the HTTPS server during the SSL handshake. |

(The UploadCABundle action is an OEM action. It is presumably a multipart upload with a form field named `ca_bundle`; no request example is given. Use it before a `SimpleUpdate` with `TransferProtocol: "HTTPS"` so the BMC can verify the image server.)

### 3.124.4 Example

#### 3.124.4.1 GET UpdateService Instance (p.371-372)
```
GET https://{BMC_IP}/redfish/v1/UpdateService
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#UpdateService.UpdateService",
    "@odata.etag": "\"1703332226\"",
    "@odata.id": "/redfish/v1/UpdateService",
    "@odata.type": "#UpdateService.v1_6_0.UpdateService",
    "Actions": {
        "#UpdateService.SimpleUpdate": {
            "@Redfish.ActionInfo": "/redfish/v1/UpdateService/SimpleUpdateActionInfo",
            "target": "/redfish/v1/UpdateService/Actions/SimpleUpdate"
        },
        "Oem": {
            "#UpdateService.UploadCABundle": {
                "@Redfish.ActionInfo": "/redfish/v1/UpdateService/UploadCABundleActionInfo",
                "target": "/redfish/v1/UpdateService/Actions/Oem/UpdateService.UploadCABundle"
            }
        }
    },
    "Description": "Redfish Update Service",
    "FirmwareInventory": {
        "@odata.id": "/redfish/v1/UpdateService/FirmwareInventory"
    },
    "Id": "UpdateService",
    "MaxImageSizeBytes": 213278720,
    "MultipartHttpPushUri": "/redfish/v1/UpdateService/upload",
    "Name": "Update Service",
    "Oem": {
        "AMIUpdateService": {
            "@odata.type": "#AMIUpdateService.v1_0_0.AMIUpdateService",
            "FlashPercentage": null,
            "PreserveConfiguration": {
                "Authentication": false,
                "FRU": true,
                "IPMI": false,
                "KVM": false,
                "NTP": false,
                "Network": false,
                "REDFISH": false,
                "SDR": false,
                "SEL": false,
                "SNMP": false,
                "SSH": false,
                "Syslog": false,
                "WEB": false
            },
            "UpdateStatus": null,
            "UpdateTarget": null
        },
        "BMC": {
            "@odata.type": "#AMIUpdateService.v1_0_0.BMC",
            "DualImageConfigurations": []
        }
    },
    "ServiceEnabled": true,
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    }
}
```

> **Notes:** The SimpleUpdate target is `/redfish/v1/UpdateService/Actions/SimpleUpdate`, **not** the DMTF-conventional `.../Actions/UpdateService.SimpleUpdate`. The multipart push URI is `/redfish/v1/UpdateService/upload`. `MaxImageSizeBytes` is 213278720 (about 203.4 MiB). `Oem.BMC.DualImageConfigurations` (an AMI OEM extension) is empty on this single-image board. The sample's PreserveConfiguration omits CMX, EXTLOG and AUTOMATION_ENGINE, which Table 142 lists.

#### 3.124.4.2 PATCH UpdateService Instance (p.372)
```
PATCH https://{BMC_IP}/redfish/v1/UpdateService
Content-Type: application/json
```
Example request body:
```json
{
    "ServiceEnabled": true,
    "Oem": {
        "AMIUpdateService": {
            "PreserveConfiguration": {
                "IPMI": true,
                "KVM": true,
                "NTP": true
            }
        }
    }
}
```
Response: status **204** with no body.

#### 3.124.4.3 POST UpdateService Instance to Create Update with HTTP (p.372-373)
```
POST https://{BMC_IP}/redfish/v1/UpdateService/Actions/SimpleUpdate
Content-Type: application/json
```
Example request body:
```json
{
    "TransferProtocol": "HTTP",
    "ImageURI": "http://test.asrockrack.com/test_image.ima"
}
```
Response: status **202** with this response message:
```json
{
    "@odata.type": "#UpdateService.v1_6_0.UpdateService",
    "Messages": [
        {
            "@odata.type": "#Message.v1_0_8.Message",
            "Message": "A new task /redfish/v1/TaskService/Tasks/1 was created.",
            "MessageArgs": [
                "/redfish/v1/TaskService/Tasks/1"
            ],
            "MessageId": "Task.1.0.New",
            "Resolution": "None",
            "Severity": "OK"
        },
        {
            "@odata.type": "#Message.v1_0_8.Message",
            "Message": "The action SimpleUpdate was submitted to do firmware update.",
            "MessageArgs": [
                "SimpleUpdate"
            ],
            "MessageId": "UpdateService.1.0.StartFirmwareUpdate",
            "Resolution": "None",
            "Severity": "OK"
        }
    ]
}
```

> **Caveat (differs from DMTF):** Unlike the other 202 responses in this document (which return a Task body), firmware update actions return a body with `@odata.type` UpdateService and a `Messages` array. The task URI must be parsed from `Messages[].MessageArgs[0]`, where `MessageId` == `Task.1.0.New`. Message registries referenced: `Task.1.0` (`Task.1.0.New`) and an AMI/OEM `UpdateService.1.0` (`UpdateService.1.0.StartFirmwareUpdate`).

#### 3.124.4.4 POST UpdateService Instance to Create Update with FTP (p.373-374)
```
POST https://{BMC_IP}/redfish/v1/UpdateService/Actions/SimpleUpdate
Content-Type: application/json
```
Example request body:
```json
{
    "TransferProtocol": "FTP",
    "ImageURI": "ftp://test.asrockrack.com/test_image.ima",
    "User": "user_account",
    "Password": "user_password"
}
```
Response: status **202** with this response message:
```json
{
    "@odata.type": "#UpdateService.v1_6_0.UpdateService",
    "Messages": [
        {
            "@odata.type": "#Message.v1_0_8.Message",
            "Message": "A new task /redfish/v1/TaskService/Tasks/2 was created.",
            "MessageArgs": [
                "/redfish/v1/TaskService/Tasks/2"
            ],
            "MessageId": "Task.1.0.New",
            "Resolution": "None",
            "Severity": "OK"
        },
        {
            "@odata.type": "#Message.v1_0_8.Message",
            "Message": "The action SimpleUpdate was submitted to do firmware update.",
            "MessageArgs": [
                "SimpleUpdate"
            ],
            "MessageId": "UpdateService.1.0.StartFirmwareUpdate",
            "Resolution": "None",
            "Severity": "OK"
        }
    ]
}
```

#### 3.124.4.5 POST UpdateService Instance for MultipartHttpPush (p.374-375)
```
POST https://{BMC_IP}/redfish/v1/UpdateService/upload
Content-Type: multipart/form-data; boundary=-------493918603359346570222237
```
Example request body:
```
-------493918603359346570222237
Content-Disposition: form-data; name="UpdateFile"; filename="encrypted_rom.ima_enc"
Content-Type: application/octet-stream

<image_binary>
-------493918603359346570222237
Content-Disposition: form-data; name="UpdateParameters"; filename="parameters.json"
Content-Type: application/json

{
    "Targets": [
        "/redfish/v1/UpdateService/FirmwareInventory/BMC"
    ]
}
-------493918603359346570222237
Content-Disposition: form-data; name="OemParameters"; filename="oem_parameters.json"
Content-Type: application/json

{
    "ImageType": "BMC"
}
-------493918603359346570222237--
```
(As printed, the delimiter lines are the boundary string itself with no extra leading `--`. Because the boundary already begins with dashes, this happens to be well-formed only if the real boundary has two fewer dashes. A Go implementation should use `mime/multipart.Writer` and not copy the literal. Note the parts are sent as *file* parts with filenames, including the JSON ones. The example BMC image file is named `encrypted_rom.ima_enc`, i.e. an encrypted/signed image.)

Response: status **202** with this response message:
```json
{
    "@odata.type": "#UpdateService.v1_6_0.UpdateService",
    "Messages": [
        {
            "@odata.type": "#Message.v1_0_8.Message",
            "Message": "A new task /redfish/v1/TaskService/Tasks/2 was created.",
            "MessageArgs": [
                "/redfish/v1/TaskService/Tasks/2"
            ],
            "MessageId": "Task.1.0.New",
            "Resolution": "None",
            "Severity": "OK"
        },
        {
            "@odata.type": "#Message.v1_0_8.Message",
            "Message": "The action SimpleUpdate was submitted to do firmware update.",
            "MessageArgs": [
                "SimpleUpdate"
            ],
            "MessageId": "UpdateService.1.0.StartFirmwareUpdate",
            "Resolution": "None",
            "Severity": "OK"
        }
    ]
}
```
(The multipart response also says "SimpleUpdate".)

## 3.125 FirmwareInventoryCollection (p.376)

This resource shall be used to represent a collection of firmware inventory.

### 3.125.1 URL
```
/redfish/v1/UpdateService/FirmwareInventory
```
### 3.125.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.125.3 Supported Properties
Please refer to section "ODATA and Frequently Used Properties".

### 3.125.4 Example
#### 3.125.4.1 GET FirmwareInventoryCollection Instance (p.376)
```
GET https://{BMC_IP}/redfish/v1/UpdateService/FirmwareInventory
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#SoftwareInventoryCollection.SoftwareInventoryCollection",
    "@odata.etag": "\"1703337670\"",
    "@odata.id": "/redfish/v1/UpdateService/FirmwareInventory",
    "@odata.type": "#SoftwareInventoryCollection.SoftwareInventoryCollection",
    "Description": "Collection of Firmware Inventory resources available to the UpdateService",
    "Members": [
        { "@odata.id": "/redfish/v1/UpdateService/FirmwareInventory/BIOS" },
        { "@odata.id": "/redfish/v1/UpdateService/FirmwareInventory/BMC" },
        { "@odata.id": "/redfish/v1/UpdateService/FirmwareInventory/CPLD" }
    ],
    "Members@odata.count": 3,
    "Name": "Firmware Inventory Collection"
}
```

## 3.126 FirmwareInventory (p.377)

The FirmwareInventory schema contains an inventory of software components. This can include firmware components such as BIOS, BMC firmware, firmware for other devices, system drivers or provider firmware.

### 3.126.1 URL
```
/redfish/v1/UpdateService/FirmwareInventory/{FirmwareInventory-Instance}
```
### 3.126.2 HTTP Method and Privileges

| HTTP Method | Required Privilege |
|---|---|
| GET | Login |

### 3.126.3 Supported Properties

#### Table 151. Supported Properties for FirmwareInventory (p.377)

| Property | Type | Read Only | Description |
|---|---|---|---|
| Updateable | Boolean | True | An indication of whether the Update Service can update this firmware. |
| Version | String | True | The version of this firmware. |

### 3.126.4 Example
#### 3.126.4.1 GET FirmwareInventory Instance (p.377)
```
GET https://{BMC_IP}/redfish/v1/UpdateService/FirmwareInventory/{FirmwareInventory-Instance}
Content-Type: application/json
```
Response:
```json
{
    "@odata.context": "/redfish/v1/$metadata#SoftwareInventory.SoftwareInventory",
    "@odata.etag": "\"1703337670\"",
    "@odata.id": "/redfish/v1/UpdateService/FirmwareInventory/BMC",
    "@odata.type": "#SoftwareInventory.v1_2_3.SoftwareInventory",
    "Id": "BMC",
    "Name": "BMC",
    "Updateable": true,
    "Version": "5.06.00"
}
```

---

# 4.0 Appendix (p.378)

## 4.1 FixedBootOrder (p.378)

If the system BIOS supports the FixedBootOrder (FBO) feature, the Redfish standard method of changing the boot order has no effect.

Below is the method for getting and changing the boot order when the system BIOS implements the FBO feature.

### 4.1.1 Getting current Boot Order and Changing Boot Order (p.378)

#### 4.1.1.1 Getting current FixedBootOrder information (p.378-379)

Get the current FBO information from the BiosAttributeRegistry.json.

Check the FBO data in the JSON file. The attribute `AttributeName: FBO101` holds the **Legacy mode** FBO information. The attribute `AttributeName: FBO201` holds the **UEFI mode** FBO information.

Request:
```
GET https://{BMC_IP}/redfish/v1/Registries/<BiosAttributeRegistry.json>
Content-Type: application/json
```
(The registry file name is a placeholder; find the actual name from `/redfish/v1/Registries`.)

Response (excerpt, `...` as in the source):
```
...
        {
            "DefaultValue": "Hard Disk",
            "UefiNamespaceId": "x-UEFI-AMI",
            "DisplayName": "Boot Option #1",
            "HelpText": "Sets the system boot order",
            "AttributeName": "FBO101",
            "Value": [
                {
                    "ValueName": "FBO101HardDisk",
                    "ValueDisplayName": "Hard Disk"
                },
                {
                    "ValueName": "FBO101NVME",
                    "ValueDisplayName": "NVME"
                },
                {
                    "ValueName": "FBO101CDDVD",
                    "ValueDisplayName": "CD/DVD"
                },
                {
                    "ValueName": "FBO101USBDevice",
                    "ValueDisplayName": "USB Device"
                },
                {
                    "ValueName": "FBO101Network",
                    "ValueDisplayName": "Network"
                },
                {
                    "ValueName": "FBO101Disabled",
                    "ValueDisplayName": "Disabled"
                }
            ],
...
        {
            "DefaultValue": "Hard Disk",
            "UefiNamespaceId": "x-UEFI-AMI",
            "DisplayName": "Boot Option #1",
            "HelpText": "Sets the system boot order",
            "AttributeName": "FBO201",
            "Value": [
                {
                    "ValueName": "FBO201HardDisk",
                    "ValueDisplayName": "Hard Disk"
                },
                {
                    "ValueName": "FBO201NVME",
                    "ValueDisplayName": "NVME"
                },
                {
                    "ValueName": "FBO201CDDVD",
                    "ValueDisplayName": "CD/DVD"
                },
                {
                    "ValueName": "FBO201USBDeviceUEFIKingstonDataTraveler30PMAPPartition1",
                    "ValueDisplayName": "USB Device:UEFI: KingstonDataTraveler 3.0PMAP, Partition 1"
                },
                {
                    "ValueName": "FBO201NetworkUEFIHTTPIP4P0IntelREthernetControllerX550",
                    "ValueDisplayName": "Network:UEFI: HTTP IP4 P0 Intel(R) Ethernet Controller X550"
                },
                {
                    "ValueName": "FBO201UEFIAPUEFIBuiltinEFIShell",
                    "ValueDisplayName": "UEFI AP:UEFI: Built-in EFI Shell"
                },
                {
                    "ValueName": "FBO201Disabled",
                    "ValueDisplayName": "Disabled"
                }
            ],
...
```

#### 4.1.1.2 GET Getting Current Boot Order with FBO (p.379-380)

Example of checking the current boot order with the FBO feature.

Request:
```
GET https://{BMC_IP}/redfish/v1/Systems/Self/Bios
Content-Type: application/json
```
Response (excerpt of `Attributes`, `...` as in the source):
```
...
        "FBO001": "FBO001UEFI",
        "FBO101": "FBO101HardDisk",
        "FBO102": "FBO102NVME",
        "FBO103": "FBO103CDDVD",
        "FBO104": "FBO104USBDevice",
        "FBO105": "FBO105Network",
        "FBO201": "FBO201HardDisk",
        "FBO202": "FBO202NVME",
        "FBO203": "FBO203CDDVD",
        "FBO204": "FBO204USBDeviceUEFIKingstonDataTraveler30PMAPPartition1",
        "FBO205": "FBO205NetworkUEFIHTTPIP4P0IntelREthernetControllerX550",
        "FBO206": "FBO206UEFIAPUEFIBuiltinEFIShell",
...
```
Check the attributes FBO201 to FBO20x, which hold the current boot order for UEFI mode. (By inference: `FBO001` is the boot mode (`FBO001UEFI`), `FBO10x` is the Legacy order, and `FBO20x` is the UEFI order. Each value is prefixed with its own slot's attribute name, so the value for slot N is `FBO20N<Device>`.)

#### 4.1.1.3 PATCH Changing Current Boot Order with FBO (p.380-381)

Example of changing the boot order with the FBO feature.

Suppose the current UEFI-mode boot order has NVME as the 2nd boot option and CD-ROM as the 3rd, and the user wants CD-ROM 2nd and NVME 3rd.

Current boot order:
```
...
        "FBO001": "FBO001UEFI",
        "FBO101": "FBO101HardDisk",
        "FBO102": "FBO102NVME",
        "FBO103": "FBO103CDDVD",
        "FBO104": "FBO104USBDevice",
        "FBO105": "FBO105Network",
        "FBO201": "FBO201HardDisk",
        "FBO202": "FBO202NVME",
        "FBO203": "FBO203CDDVD",
        "FBO204": "FBO204USBDeviceUEFIKingstonDataTraveler30PMAPPartition1",
        "FBO205": "FBO205NetworkUEFIHTTPIP4P0IntelREthernetControllerX550",
        "FBO206": "FBO206UEFIAPUEFIBuiltinEFIShell",
...
```
Request:
```
PATCH https://{BMC_IP}/redfish/v1/Systems/Self/Bios/SD
Content-Type: application/json
```
Example request body (p.381):
```json
{
    "Attributes": {
        "FBO202": "FBO202CDDVD",
        "FBO203": "FBO203NVME"
    }
}
```
Response: status **204** with no body.

> **Implementation notes (FBO):**
> - With FBO enabled in the BIOS, PATCHing `Boot.BootOrder` on `/redfish/v1/Systems/Self/SD` is blocked or has no effect (see the note in 3.95.4.2). The boot order must be changed through BIOS attributes on `/redfish/v1/Systems/Self/Bios/SD` (the BIOS pending-settings object, applied on the next host boot).
> - The value strings carry the slot prefix: when you move a device into slot `FBO202`, the value must be `FBO202<Device>` (e.g. `FBO202CDDVD`), not the value string from its previous slot.
> - Device suffixes for USB and network entries are derived from the device name (e.g. `USBDeviceUEFIKingstonDataTraveler30PMAPPartition1`), so they change with attached hardware. Always read the allowable values from the BIOS attribute registry first.
> - The swap is done by PATCHing both affected slots in one request. The source does not say whether duplicate assignments are rejected.

---

## Coverage notes for part 3 (p.261-381)

- Sections covered: end of 3.78 ComputerSystem (Boot table continued from part 2, TrustedModules, examples, Reset), 3.79-3.81, 3.83-3.126, and Appendix 4.0/4.1 FixedBootOrder. **Section 3.82 does not exist** in the PDF (numbering skips from 3.81 to 3.83). The PDF ends at p.381 with Appendix 4.1. There is no message-registry, privilege-mapping or supported-schema appendix in this range.
- Topics **not** present in p.261-381 (look in parts 1/2): sessions and authentication, AccountService, ETag/If-Match semantics, error-response format, EventService, LogServices, VirtualMedia, Managers, Chassis, and the BIOS resource itself (only its FBO use appears here).
- Example responses that are **empty** in the PDF: GraphicsControllerCollection, GraphicsController, Volume, MetricReportDefinition (GET and both POST 201 bodies), MetricReport, Trigger (GET and both POST 201 bodies).
- No text was illegible in this range.
