# ASRock Rack Redfish API document, part 2 (PDF pages 131 to 260)

Source: `/opt/sparked/asrockrack-api/redfishdocs/asrockrack-redfishdocs.pdf` (AMI MegaRAC Redfish guide as published for ASRock Rack). The pages are printed images. This file transcribes pages 131 to 260. Page references look like `(p.142)`. Unreadable text is marked `[illegible]`.

Conventions the PDF uses throughout:
- Each resource section has URL, "HTTP Method and Privileges", "Supported Properties", and "Example" subsections.
- "Read Only" columns are reproduced as RO = True / `-` (not specified).
- "Please refer to section ODATA and Frequently Used Properties" means the common `@odata.*`, `Id`, `Name`, `Description`, `Status`, `Oem` properties are described in part 1.
- Examples use `{BMC_IP}`. The Chassis ID in examples is `Self`.
- Example requests show `Content-Type: application/json` even on GET requests.
- ETags in examples are quoted Unix timestamps, e.g. `"@odata.etag": "\"1699174497\""`.

---

## 3.34 PCIeDevice Collection (p.131)

Lists the PCIeDevice resource instances in the Chassis.

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices`
- **Methods/privileges:** GET = Login
- **Properties:** only the common ODATA properties (see part 1).

### Example: GET PCIeDevice Collection (pp.131-134)

```
GET https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices
Content-Type: application/json
```

Response (the member list is shortened to one line per member here; every ID shown in the PDF is kept):

```json
{
    "@odata.context": "/redfish/v1/$metadata#PCIeDeviceCollection.PCIeDeviceCollection",
    "@odata.etag": "\"1699174497\"",
    "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices",
    "@odata.type": "#PCIeDeviceCollection.PCIeDeviceCollection",
    "Description": "The Collection of PCIeDevices",
    "Members": [
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_00"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_08"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_10"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_18"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_20"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_28"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_29"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_38"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_39"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_3A"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_A0"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_A3"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_C0"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_C1"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_C2"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_C3"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_C4"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_C5"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_C6"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_00_C7"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_00"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_01"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_02_00"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_02_04"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_02_05"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_03_00"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_03_01"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_00"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_08"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_10"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_18"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_19"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_1A"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_1B"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_1C"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_20"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_28"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_38"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_40_39"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_49_00"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_80_00"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_80_08"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_80_10"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_80_18"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_80_19"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_80_1A"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_80_1B"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_80_20"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_80_28"},
        {"@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_80_38"}
    ],
    "Members@odata.count": 73,
    "Members@odata.nextLink": "/redfish/v1/Chassis/Self/PCIeDevices?$skip=50",
    "Name": "PCIeDevice Collection"
}
```

Notes:
- **Server-side paging:** the collection returns 50 members per page. `Members@odata.count` is the total (73) and `Members@odata.nextLink` uses `?$skip=50`. A client or proxy must follow `nextLink`. (p.134)
- PCIeDevice IDs use the form `SS_BB_DF`, which looks like segment_bus_devfn in hex (for example `00_01_00`). (pp.131-134)

## 3.35 PCIeDevice (p.135)

Schema for the PCIeDevice resource: a PCIe device attached to a System.

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices/{PCIeDevice-Instance}`
- **Methods/privileges:** GET = Login

### Table 44. Supported Properties for PCIeDevice (p.135)

| Property | Type | RO | Description |
|---|---|---|---|
| Manufacturer | String | True | Manufacturer of this PCIe device. |
| Model | String | True | Model number of the PCIe device. |
| SKU | String | True | SKU of this PCIe device. |
| SerialNumber | String | True | Serial number of this PCIe device. |
| PartNumber | String | True | Part number of this PCIe device. |
| AssetTag | String | True | User-assigned asset tag for this PCIe device. Listed as read-only here. |
| DeviceType | String | True | Device type. Enum: `SingleFunction` (a single-function PCIe device), `MultiFunction` (a multi-function PCIe device), `Simulated` (not physically present now but simulated by the PCIe infrastructure). |
| FirmwareVersion | String | True | Firmware version of this PCIe device. |
| Links | Object | True | Links to related resources. |
| Links.Oem | Object | - | See ODATA and Frequently Used Properties. |
| Links.Chassis | Array | True | References to the chassis that contains the PCIe device. |
| Links.Chassis@odata.count | Number | True | Number of items in the collection. |
| PCIeFunctions | Object | True | Reference to the PCIeFunctionsCollection that this device exposes. |
| PCIeInterface | Object | True | PCI Interface object (Table 45). |
| Assembly | Object | True | Reference to the Assembly resource for this PCIe device. |

### Table 45. PCIeInterface Properties (pp.135-136)

| Property | Type | RO | Description |
|---|---|---|---|
| LanesInUse | Number | True | Number of PCIe lanes in use by this device. |
| MaxLanes | Number | True | Number of PCIe lanes this device supports. |
| MaxPCIeType | String | True | Highest PCIe spec version the device supports (Table 46). |
| PCIeType | String | True | PCIe spec version in use by this device (Table 46). |
| Oem | Object | - | See ODATA and Frequently Used Properties. |

### Table 46. PCIe Types Properties (p.136)

| Value | Description |
|---|---|
| GEN1 | A PCIe v1.0 slot. |
| GEN2 | A PCIe v2.0 slot. |
| GEN3 | A PCIe v3.0 slot. |
| GEN4 | A PCIe v4.0 slot. |
| GEN5 | A PCIe v5.0 slot. |

Caveat: the table spells the values in upper case (`GEN3`), but the example response returns `"Gen3"`, which matches the DMTF enum. Match case-insensitively or expect `Gen1`..`Gen5`. (p.136)

### Example: GET PCIeDevice Instance (p.136)

The PDF's request line shows the collection URL (`.../PCIeDevices`). This looks like a documentation error; the response is for an instance.

```
GET https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#PCIeDevice.PCIeDevice",
    "@odata.etag": "\"1699177554\"",
    "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_00",
    "@odata.type": "#PCIeDevice.v1_4_0.PCIeDevice",
    "AssetTag": "LAN1",
    "DeviceType": "MultiFunction",
    "FirmwareVersion": "1.2829.0",
    "Id": "00_01_00",
    "Name": "LAN1",
    "PCIeFunctions": {
        "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_00/PCIeFunctions"
    },
    "PCIeInterface": {
        "LanesInUse": 4,
        "MaxLanes": 4,
        "MaxPCIeType": "Gen3",
        "PCIeType": "Gen3"
    },
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    }
}
```

## 3.36 PCIeFunction Collection (p.137)

Lists the PCIeFunction resource instances under a PCIeDevice.

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices/{PCIeDevice-Instance}/PCIeFunctions`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET PCIeFunction Collection (p.137)

```
GET https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices/{PCIeDevice-Instance}/PCIeFunctions
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#PCIeFunctionCollection.PCIeFunctionCollection",
    "@odata.etag": "\"1699177557\"",
    "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_00/PCIeFunctions",
    "@odata.type": "#PCIeFunctionCollection.PCIeFunctionCollection",
    "Description": "The Collection of PCIeFunctions",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_00/PCIeFunctions/DevType3_LAN1_DevIndex16"
        }
    ],
    "Members@odata.count": 1,
    "Name": "PCIeFunction Collection"
}
```

Note: PCIeFunction IDs are not numeric. They take the form `DevType{n}_{Name}_DevIndex{n}`. (p.137)

## 3.37 PCIeFunction (p.138)

Schema for the PCIeFunction resource: a PCIe function attached to a System.

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices/{PCIeDevice-Instance}/PCIeFunctions/{PCIeFunctions-Instance}`
- **Methods/privileges:** GET = Login

### Table 47. Supported Properties for PCIeFunction (p.138)

| Property | Type | RO | Description |
|---|---|---|---|
| FunctionId | Number | True | The PCIe Function identifier. |
| FunctionType | String | True | Function type of the PCIe device function, such as Physical or Virtual. |
| DeviceClass | String | True | Device class of the function, such as Storage, Network, Memory. |
| DeviceId | String | True | Device ID of this PCIe function. |
| VendorId | String | True | Vendor ID of this PCIe function. |
| ClassCode | String | True | Class Code of this PCIe function. |
| RevisionId | String | True | Revision ID of this PCIe function. |
| SubsystemId | String | True | Subsystem ID of this PCIe function. |
| SubsystemVendorId | String | True | Subsystem Vendor ID of this PCIe function. |
| Links | Object | - | Links to related resources. |
| Links.Oem | Object | - | OEM extensions. |
| Links.Drives | Array | True | References to drives this PCIe device produces. |
| Links.Drives@odata.count | Number | True | Number of items. |
| Links.StorageControllers | Array | True | References to storage controllers this PCIe device produces. |
| Links.StorageControllers@odata.count | Number | True | Number of items. |
| Links.EthernetInterfaces | Array | True | References to ethernet interfaces this PCIe device produces. |
| Links.EthernetInterfaces@odata.count | Number | True | Number of items. |
| Links.NetworkDeviceFunctions | Array | True | References to Network Device Functions this PCIe device produces. |
| Links.NetworkDeviceFunctions@odata.count | Number | True | Number of items. |
| Links.PCIeDevice | Object | True | Reference to the PCIeDevice resource this function belongs to. |

### Example: GET PCIeFunction Instance (p.139)

```
GET https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/PCIeDevices/{PCIeDevice-Instance}/PCIeFunctions/{PCIeFunctions-Instance}
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#PCIeFunction.PCIeFunction",
    "@odata.etag": "\"1699177554\"",
    "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_00/PCIeFunctions/DevType3_LAN1_DevIndex16",
    "@odata.type": "#PCIeFunction.v1_2_3.PCIeFunction",
    "ClassCode": "0x020000",
    "DeviceClass": "NetworkController",
    "DeviceId": "0x1563",
    "FunctionId": 0,
    "FunctionType": "Physical",
    "Id": "DevType3_LAN1_DevIndex16",
    "Links": {
        "NetworkDeviceFunctions": [
            {
                "@odata.id": "/redfish/v1/Chassis/Self/NetworkAdapters/DevType7_NIC0/NetworkDeviceFunctions/NetworkDeviceFunction0"
            }
        ],
        "NetworkDeviceFunctions@odata.count": 1,
        "PCIeDevice": {
            "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_01_00"
        }
    },
    "Name": "LAN1",
    "RevisionId": "0x01",
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "SubsystemId": "0x1563",
    "SubsystemVendorId": "0x1849",
    "VendorId": "0x8086"
}
```

Note: IDs are strings in `0x` hex form. `SubsystemVendorId` 0x1849 is ASRock. NetworkAdapter IDs look like `DevType7_NIC0`. (p.139)

## 3.38 PCIeSlots (p.140)

Holds PCIe slot information.

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/PCIeSlots`
- **Methods/privileges:** GET = Login

### Table 48. Supported Properties for PCIeSlots (p.140)

| Property | Type | RO | Description |
|---|---|---|---|
| HotPluggable | Boolean | True | Whether the PCIe slot supports hotplug. (Placed at the top level in this table. In DMTF, HotPluggable belongs to each Slot.) |
| Actions | Object | True | Actions for this resource, under the Oem property if any. |
| Oem | Object | - | OEM extensions. |
| Slots | Array | - | PCI slot information (Table 49). |

### Table 49. Slots Properties (p.140)

| Property | Type | RO | Description |
|---|---|---|---|
| Lanes | Number | True | Maximum number of PCIe lanes the slot supports. |
| Links | Object | - | See Table 50. |
| Location | Object | - | Part location information, including the ServiceLabel of the PCIe slot. |
| Oem | Object | - | OEM extensions. |
| PCIeType | String | True | PCIe spec version this slot supports. Enum: `Gen1` (v1.0), `Gen2` (v2.0), `Gen3` (v3.0), `Gen4` (v4.0), `Gen5` (v5.0). |
| SlotType | String | True | PCIe slot type. Enum: `FullLength`, `HalfLength`, `LowProfile` (Low-Profile or Slim), `M2` (PCIe M.2 slot), `Mini` (Mini PCIe slot). |

### Table 50. Links Properties (p.140)

| Property | Type | RO | Description |
|---|---|---|---|
| Oem | Object | - | OEM extensions. |
| PCIeDevice | Array | True | PCIe devices connected to this slot. |
| PCIeDevice@odata.count | Number | True | Number of items in PCIeDevices. |

### Example: GET PCIeSlots Instance (pp.141-145)

```
GET https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/PCIeSlots
Content-Type: application/json
```

Response. The first two Slots entries are copied verbatim. The other 13 entries have the same shape and are listed in the table below.

```json
{
    "@odata.context": "/redfish/v1/$metadata#PCIeSlots.PCIeSlots",
    "@odata.etag": "\"1699179899\"",
    "@odata.id": "/redfish/v1/Chassis/Self/PCIeSlots",
    "@odata.type": "#PCIeSlots.v1_1_1.PCIeSlots",
    "Description": "PcieSlot_0",
    "Id": "PcieSlot_0",
    "Name": "PcieSlot_0",
    "Slots": [
        {
            "Links": {
                "PCIeDevice": [
                    { "@odata.id": "/redfish/v1/Chassis/Self/PCIeDevices/00_85_00" }
                ],
                "PCIeDevice@odata.count": 1
            },
            "Status": {
                "Health": "OK",
                "State": "Enabled"
            }
        },
        {
            "HotPluggable": false,
            "Lanes": 16,
            "Location": {
                "PartLocation": {
                    "LocationOrdinalValue": 19,
                    "LocationType": "Slot",
                    "ServiceLabel": "PCIE3"
                }
            },
            "PCIeType": "Gen1",
            "SlotType": "FullLength",
            "Status": {
                "State": "Absent"
            }
        }
    ]
}
```

All the other slots have `"HotPluggable": false`, `"LocationType": "Slot"`, `"PCIeType": "Gen1"`, `"SlotType": "FullLength"`, `"Status": {"State": "Absent"}`, in this order (pp.141-145):

| # | ServiceLabel | LocationOrdinalValue | Lanes |
|---|---|---|---|
| 2 | PCIE3 | 19 | 16 |
| 3 | PCIE4 | 20 | 16 |
| 4 | PCIE5 | 21 | 16 |
| 5 | PCIE6 | 22 | 16 |
| 6 | MCIO3-1 | 85 | 4 |
| 7 | MCIO3-2 | 86 | 4 |
| 8 | MCIO4-1 | 87 | 4 |
| 9 | MCIO4-2 | 88 | 4 |
| 10 | MCIO5 | 89 | 4 |
| 11 | MCIO1-1 | 81 | 4 |
| 12 | MCIO1-2 | 82 | 4 |
| 13 | MCIO2-1 | 83 | 4 |
| 14 | MCIO2-2 | 84 | 4 |
| 15 | M2_2 | 34 | 4 |

Caveats (pp.141-145):
- The example is from a board that is not the B650D4U. It has MCIO connectors and PCIE3 to PCIE6.
- The first (occupied) slot entry has only `Links` and `Status`. It has no `Location`, `Lanes`, `PCIeType`, or `SlotType`.
- Empty slots report `PCIeType: "Gen1"` and `SlotType: "FullLength"` as defaults, including the M.2 slot. Treat these values as unreliable when `State` is `Absent`.
- `HotPluggable` appears on each slot in the example, as in DMTF, not at the top level as Table 48 says.

## 3.39 Power (p.146)

Power metrics resource (legacy DMTF `Power` schema, v1_5_4 in the example).

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/Power`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureManager |

### Table 51. Supported Properties for Power (p.146)

| Property | Type | RO | Description |
|---|---|---|---|
| PowerControl | Array | False | Power control function (power reading and limiting). See Table 52. |
| PowerControl@odata.count | Number | True | Number of items. |
| Voltages | Array | True | Voltage sensors. See Table 54. |
| Voltages@odata.count | Number | True | Number of items. |
| PowerSupplies | Array | False | Power supplies for this system or device. See Table 55. |
| PowerSupplies@odata.count | Number | True | Number of items. |
| Redundancy | Array | True | Redundancy information for the power subsystem. |
| Redundancy@odata.count | Number | True | Number of items. |
| Actions | Object | True | Holds an Oem object under the Oem attribute. |

### Table 52. PowerControl Properties (pp.146-147)

| Property | Type | RO | Description |
|---|---|---|---|
| MemberId | String | True | Member identifier within the collection. |
| PowerConsumedWatts | Number | True | Actual power consumed by the chassis, in watts. Min 0. |
| PowerRequestedWatts | Number | True | Power, in watts, that the chassis is requesting to be budgeted for future use. Min 0. |
| PowerAvailableWatts | Number | True | Power capacity, in watts, not yet allocated. Equals PowerCapacityWatts minus PowerAllocatedWatts. Min 0. |
| PowerCapacityWatts | Number | True | Total power capacity available for allocation to the chassis resources. Min 0. |
| PowerAllocatedWatts | Number | True | Total power currently allocated to chassis resources. Min 0. |
| PowerMetrics | Object | True | Power readings for this chassis. |
| PowerMetrics.IntervalInMin | Number | - | Time interval (window) over which PowerMetrics are measured. Min 0. |
| PowerMetrics.MinConsumedWatts | Number | - | Lowest power consumption over the last IntervalInMin minutes. Min 0. |
| PowerMetrics.MaxConsumedWatts | Number | - | Highest power consumption over the last IntervalInMin minutes. Min 0. |
| PowerMetrics.AverageConsumedWatts | Number | - | Average power over the last IntervalInMin minutes. Min 0. |
| PowerLimit | Object | **False (writable)** | Power limit status and configuration for this chassis. |
| PowerLimit.LimitInWatts | Number | - | Power limit in watts. Min 0. **Note:** range 0 to 65535. |
| PowerLimit.LimitException | String | - | Action taken if power cannot be kept below LimitInWatts. See Table 53. Defaults to null. |
| PowerLimit.CorrectionInMs | Number | - | Time the limiting process needs to bring power consumption below the limit. Defaults to null. **Note:** range 0 to 4294967295. |
| RelatedItem | Array | True | IDs of the resources associated with this Power Limit. |
| PhysicalContext | Object | True | Affected device or region within the chassis. (The table says Object, but the example returns a string such as `"Intake"`.) |

### Table 53. Chassis PowerLimitExceptionEnum Properties (p.147)

| Enum | Description |
|---|---|
| NoAction | Take no action when the limit is exceeded. |
| HardPowerOff | Turn the power off immediately when the limit is exceeded. |
| LogEventOnly | Log an event when the limit is exceeded and take no further action. |
| Oem | Take an OEM-defined action. |

### Table 54. Voltages Properties (pp.147-148)

| Property | Type | RO | Description |
|---|---|---|---|
| Name | String | True | Name of the voltage sensor. |
| MemberId | String | True | Uniquely identifies the member in the collection. For Redfish v1.6 or later this is the zero-based array index. |
| SensorNumber | Number | True | Numeric identifier for this voltage sensor, unique within this resource. |
| ReadingVolts | Number | True | Current value of the voltage sensor. |
| UpperThresholdNonCritical | Number | True | Reading is above the normal range but not critical. Same units as ReadingVolts. |
| UpperThresholdCritical | Number | True | Reading is above the normal range but not yet fatal. |
| UpperThresholdFatal | Number | True | Reading is above the normal range and fatal. |
| LowerThresholdNonCritical | Number | True | Reading is below the normal range but not critical. |
| LowerThresholdCritical | Number | True | Reading is below the normal range but not yet fatal. |
| LowerThresholdFatal | Number | True | Reading is below the normal range and fatal. |
| MinReadingRange | Number | True | Lowest possible reading value. Same units as ReadingVolts. |
| MaxReadingRange | Number | True | Highest possible reading value. |
| PhysicalContext | Object | True | Affected device or region within the chassis. See the ThermalPhysicalContextEnum table (Thermal section). |
| RelatedItem | Array | True | IDs of the associated resources. |
| OwnerLUN | Number | True | **OEM attribute, implementation-specific.** Tells apart sensors that have the same Sensor number but different LUN numbers. (In the example it appears under `Oem.Ami`.) |

### Table 55. PowerSupply Properties (pp.148-149)

| Property | Type | RO | Description |
|---|---|---|---|
| Name | String | True | Name of the PowerSupply. |
| MemberId | String | True | Member identifier within the collection. |
| PowerSupplyType | String | True | Power supply type. Enum: `Unknown` (cannot be determined), `AC` (alternating current), `DC` (direct current), `ACorDC` (supports both). |
| LineInputVoltageType | String | True | Line input voltage at which the PSU operates. Enum: `Unknown`; `ACLowLine` (100-127V AC, deprecated, use AC120V); `ACMidLine` (200-240V AC, deprecated, use AC240V); `ACHighLine` (277V AC, deprecated, use AC277V); `DCNeg48V` (-48V DC); `DC380V` (high-voltage DC, 380V); `AC120V`; `AC240V`; `AC277V`; `ACandDCWideRange` (wide-range AC or DC); `ACWideRange` (wide-range AC); `DC240V` (DC 240V nominal). |
| LineInputVoltage | Number | True | Line input voltage at which the PSU operates. |
| PowerCapacityWatts | Number | True | Maximum capacity of this PSU. Min 0. |
| LastPowerOutputWatts | Number | True | Average power output of this PSU, in watts. Min 0. |
| Model | String | True | Model number. |
| FirmwareVersion | String | True | Firmware version. |
| SerialNumber | String | True | Serial number. |
| PartNumber | String | True | Part number. |
| SparePartNumber | String | True | Spare part number. |
| PowerInputWatts | Number | True | Measured input power of the PSU, in watts. |
| PowerOutputWatts | Number | True | Measured output power of the PSU, in watts. |
| EfficiencyPercent | Number | True | Measured power efficiency of the PSU, as a percentage. |
| HotPluggable | Boolean | True | Whether the device can be inserted or removed while the equipment stays in its current operational state (standard DMTF definition). |
| RelatedItem | Array | True | IDs of the associated resources. |
| Redundancy | Array | True | Redundancy for power supplies. Component IDs refer to members of the redundancy groups. |

### Example: GET Power Instance (pp.149-151)

```
GET https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/Power
Content-Type: application/json
```

Response. The `...` is in the PDF itself; PowerSupplies 1 to 7 were left out of the document.

```json
{
    "@odata.context": "/redfish/v1/$metadata#Power.Power",
    "@odata.etag": "\"1699185299\"",
    "@odata.id": "/redfish/v1/Chassis/Self/Power",
    "@odata.type": "#Power.v1_5_4.Power",
    "Description": "Power sensor readings",
    "Id": "Power",
    "Name": "Power",
    "PowerControl": [
        {
            "@odata.id": "/redfish/v1/Chassis/Self/Power#/PowerControl/0",
            "MemberId": "0",
            "Name": "Chassis Power Control",
            "PhysicalContext": "Intake",
            "PowerLimit": {
                "CorrectionInMs": 1000,
                "LimitException": "HardPowerOff",
                "LimitInWatts": 500
            },
            "PowerMetrics": {
                "AverageConsumedWatts": 0,
                "CurConsumedWatts": 0,
                "IntervalInMin": 0,
                "MaxConsumedWatts": 0,
                "MinConsumedWatts": 0
            },
            "RelatedItem@odata.count": 0
        }
    ],
    "PowerControl@odata.count": 1,
    "PowerSupplies": [
        {
            "@odata.id": "/redfish/v1/Chassis/Self/Power#/PowerSupplies/0",
            "MemberId": "0",
            "Name": "PWR_PSU2_POUT",
            "RelatedItem@odata.count": 0,
            "Status": {
                "State": "Absent"
            }
        }, ...
        {
            "@odata.id": "/redfish/v1/Chassis/Self/Power#/PowerSupplies/8",
            "MemberId": "8",
            "Name": "STS_PSU2_AC_LOST",
            "RelatedItem@odata.count": 0,
            "Status": {
                "State": "Absent"
            }
        }
    ],
    "PowerSupplies@odata.count": 9,
    "Voltages": [
        {
            "@odata.id": "/redfish/v1/Chassis/Self/Power#/Voltages/0",
            "LowerThresholdCritical": 3.04,
            "LowerThresholdFatal": 2.86,
            "MaxReadingRange": 5.1,
            "MemberId": "0",
            "MinReadingRange": 0,
            "Name": "VOLT_3.3VSB",
            "Oem": {
                "Ami": {
                    "@odata.type": "#AMIChassisPowerThermal.v1_0_0.AMIChassisPowerThermal",
                    "OwnerLUN": 0
                }
            },
            "PhysicalContext": "VoltageRegulator",
            "ReadingVolts": 3.34,
            "RelatedItem@odata.count": 0,
            "SensorNumber": 1,
            "Status": {
                "Health": "OK",
                "State": "Enabled"
            },
            "UpperThresholdCritical": 3.72,
            "UpperThresholdFatal": 3.88
        }, ...
        {
            "@odata.id": "/redfish/v1/Chassis/Self/Power#/Voltages/14",
            "MaxReadingRange": 255,
            "MemberId": "14",
            "MinReadingRange": 0,
            "Name": "VOLT_PSU2_VIN",
            "Oem": {
                "Ami": {
                    "@odata.type": "#AMIChassisPowerThermal.v1_0_0.AMIChassisPowerThermal",
                    "OwnerLUN": 0
                }
            },
            "PhysicalContext": "VoltageRegulator",
            "RelatedItem@odata.count": 0,
            "SensorNumber": 120,
            "Status": {
                "State": "Absent"
            }
        }
    ],
    "Voltages@odata.count": 15
}
```

Notes on the Power example (pp.150-151):
- **OEM extension `Oem.Ami`** on each Voltages entry: `@odata.type` = `#AMIChassisPowerThermal.v1_0_0.AMIChassisPowerThermal`, with property `OwnerLUN` (number).
- `PowerMetrics` contains **`CurConsumedWatts`**, which is not a DMTF property (DMTF has `PowerConsumedWatts` on PowerControl). All metrics are 0 in the example.
- "PowerSupplies" are really IPMI sensors for the PSUs (e.g. `PWR_PSU2_POUT`, `STS_PSU2_AC_LOST`), not physical PSUs. There are 9 entries, all `Absent` on this example board. Do not assume one entry per physical PSU.
- Member `@odata.id` values use JSON-pointer fragments (`Power#/PowerControl/0`).
- When a sensor is Absent, it has no `ReadingVolts` or threshold properties at all.

### Example: PATCH Power Instance (pp.151-152)

Sets the chassis power limit. Privilege: ConfigureManager.

```
PATCH https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/Power
Content-Type: application/json
```

Request body:

```json
{
    "PowerControl": [
        {
            "PowerLimit": {
                "CorrectionInMs": 1000,
                "LimitException": "HardPowerOff",
                "LimitInWatts": 550
            }
        }
    ]
}
```

Response: **200 OK, with the GET response body** (the updated resource) (p.152). The PDF does not say whether an `If-Match` header is required.

## 3.40 SensorCollection (p.153)

Collection of Sensor instances.

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/Sensors`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET Sensor Collection (pp.153-154)

```
GET https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/Sensors
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#SensorCollection.SensorCollection",
    "@odata.etag": "\"1699703741\"",
    "@odata.id": "/redfish/v1/Chassis/Self/Sensors",
    "@odata.type": "#SensorCollection.SensorCollection",
    "Description": "Resource Collection of Sensor instances",
    "Members": [
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/113"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/121"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/176"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/177"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/178"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/179"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/184"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/185"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/186"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/208"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/209"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/212"},
        {"@odata.id": "/redfish/v1/Chassis/Self/Sensors/216"}
    ],
    "Members@odata.count": 13,
    "Name": "Sensor Collection"
}
```

Note: Sensor IDs are numeric strings, apparently IPMI sensor numbers in decimal. Only 13 sensors are listed here, while Power has 15 voltage entries plus PSU entries, so the Sensors collection does not appear to hold every IPMI sensor. (pp.153-154)

## 3.41 Sensor (p.155)

A single Sensor.

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/Sensors/{Sensors-Instance}`
- **Methods/privileges:** GET = Login

### Table 56. Supported Properties for Sensor (pp.155-157)

| Property | Type | RO | Description |
|---|---|---|---|
| Accuracy | Number | True | Estimated percent error of measured versus actual values. |
| AdjustedMaxAllowableOperatingValue | Number | True | Adjusted maximum allowable operating value, based on environmental conditions. |
| AdjustedMinAllowableOperatingValue | Number | True | Adjusted minimum allowable operating value, based on environmental conditions. |
| ApparentVA | Number | True | Voltage times current for an AC circuit, in volt-amperes. |
| ElectricalContext | String | True | Combination of current-carrying conductors. Enum: `Line1`, `Line1ToLine2`, `Line1ToNeutral`, `Line1ToNeutralAndL1L2`, `Line2`, `Line2ToLine3`, `Line2ToNeutral`, `Line2ToNeutralAndL1L2`, `Line2ToNeutralAndL2L3`, `Line3`, `Line3ToLine1`, `Line3ToNeutral`, `Line3ToNeutralAndL3L1`, `LineToLine`, `LineToNeutral`, `Neutral` (grounded return circuit), `Total` (all conductors). |
| LoadPercent | Number | True | Power load utilization for this sensor. |
| Location | Object | - | Location information for this sensor. |
| MaxAllowableOperatingValue | Number | True | Maximum allowable operating value for this equipment. |
| MinAllowableOperatingValue | Number | True | Minimum allowable operating value for this equipment. |
| PeakReading | Number | True | Peak sensor value. |
| PeakReadingTime | String | True | Time at which the peak sensor value occurred. |
| PhysicalContext | String | True | Area or device the measurement applies to. Enum: `Accelerator`, `ACInput`, `ACMaintenanceBypassInput`, `ACOutput`, `ACStaticBypassInput`, `ACUtilityInput`, `ASIC`, `Back`, `Backplane`, `Chassis`, `ComputeBay`, `CoolingSubsystem`, `CPU`, `CPUSubsystem`, `DCBus`, `Exhaust`, `ExpansionBay`, `Fan`, `FPGA`, `Front`, `GPU`, `GPUSubsystem`, `Intake`, `LiquidInlet`, `LiquidOutlet`, `Lower`, `Memory`, `MemorySubsystem`, `Motor`, `NetworkBay`, `NetworkingDevice`, `PowerSubsystem`, `PowerSupply`, `PowerSupplyBay`, `Rectifier`, `Room`, `StorageBay`, `StorageDevice`, `SystemBoard`, `Transformer`, `Upper`, `VoltageRegulator`. |
| PhysicalSubContext | String | True | Where within a device the measurement applies. Enum: `Input`, `Output`. |
| PowerFactor | Number | True | Power factor for this sensor. |
| Precision | Number | True | Number of significant digits in the reading. |
| ReactiveVAR | Number | True | Square root of (ApparentVA squared minus Power (Reading) squared) for a circuit, in VAR. |
| Reading | Number | True | Sensor value. |
| ReadingRangeMax | Number | True | Highest possible value for this sensor. |
| ReadingRangeMin | Number | True | Lowest possible value for this sensor. |
| ReadingType | String | True | Sensor type. Enum: `AirFlow`, `Altitude`, `Barometric`, `Current`, `EnergyJoules`, `EnergykWh`, `Frequency`, `Humidity` (relative), `LiquidFlow`, `LiquidLevel`, `Power`, `Pressure`, `Rotational`, `Temperature`, `Voltage` (AC or DC). |
| ReadingUnits | String | True | Units of the reading and thresholds. |
| SensingFrequency | Number | True | Time between readings of the physical sensor. |
| SensorResetTime | String | True | Date and time when the time-based properties were last reset. |
| Thresholds | Object | - | Set of thresholds for this sensor. Members: `LowerCaution`, `LowerCritical`, `LowerFatal`, `UpperCaution`, `UpperCritical`, `UpperFatal` (each an Object, see Table 57). |
| VoltageType | String | True | Voltage type. Enum: `AC`, `DC`. |

### Table 57. Threshold Properties (pp.157-158)

| Property | Type | RO | Description |
|---|---|---|---|
| Activation | String | True | Crossing direction that activates the threshold. Enum: `Decreasing` (value falls below), `Either` (crosses in either direction), `Increasing` (value rises above). |
| DwellTime | String | True | How long the value must violate the threshold before it activates. |
| Reading | Number | True | Threshold value. |

Note: all threshold fields are read-only. The PDF documents no PATCH on Sensors. (p.155)

### Example: GET Sensor Instance (pp.158-160)

```
GET https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/Sensors/{Sensors-Instance}
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#Sensor.Sensor",
    "@odata.etag": "\"1699708324\"",
    "@odata.id": "/redfish/v1/Chassis/Self/Sensors/113",
    "@odata.type": "#Sensor.v1_0_2.Sensor",
    "Accuracy": null,
    "AdjustedMaxAllowableOperatingValue": null,
    "AdjustedMinAllowableOperatingValue": null,
    "ApparentVA": null,
    "Description": null,
    "ElectricalContext": null,
    "Id": "113",
    "LoadPercent": null,
    "Location": {
        "AltitudeMeters": null,
        "Contacts": [
            null
        ],
        "Latitude": null,
        "Longitude": null,
        "PartLocation": {
            "LocationOrdinalValue": null,
            "LocationType": null,
            "Orientation": null,
            "Reference": null,
            "ServiceLabel": null
        },
        "Placement": {
            "AdditionalInfo": null,
            "Rack": null,
            "RackOffset": null,
            "RackOffsetUnits": null,
            "Row": null
        },
        "PostalAddress": {
            "AdditionalCode": null,
            "AdditionalInfo": null,
            "Building": null,
            "City": null,
            "Community": null,
            "Country": null,
            "District": null,
            "Division": null,
            "Floor": null,
            "HouseNumber": null,
            "HouseNumberSuffix": null,
            "Landmark": null,
            "LeadingStreetDirection": null,
            "Name": null,
            "Neighborhood": null,
            "POBox": null,
            "PlaceType": null,
            "PostalCode": null,
            "Road": null,
            "RoadBranch": null,
            "RoadPostModifier": null,
            "RoadPreModifier": null,
            "RoadSection": null,
            "RoadSubBranch": null,
            "Room": null,
            "Seat": null,
            "Street": null,
            "StreetSuffix": null,
            "Territory": null,
            "TrailingStreetSuffix": null,
            "Unit": null
        }
    },
    "MaxAllowableOperatingValue": null,
    "MinAllowableOperatingValue": null,
    "Name": "CUR_PSU1_IOUT",
    "PeakReading": null,
    "PeakReadingTime": null,
    "PhysicalContext": null,
    "PhysicalSubContext": null,
    "PowerFactor": null,
    "Precision": null,
    "ReactiveVAR": null,
    "Reading": 0,
    "ReadingRangeMax": null,
    "ReadingRangeMin": null,
    "ReadingType": null,
    "ReadingUnits": null,
    "SensingFrequency": null,
    "SensorResetTime": null,
    "Status": {
        "Health": null,
        "HealthRollup": null,
        "State": "Disabled"
    },
    "Thresholds": {
        "LowerCaution":  { "Activation": null, "DwellTime": null, "Reading": null },
        "LowerCritical": { "Activation": null, "DwellTime": null, "Reading": null },
        "LowerFatal":    { "Activation": null, "DwellTime": null, "Reading": null },
        "UpperCaution":  { "Activation": null, "DwellTime": null, "Reading": null },
        "UpperCritical": { "Activation": null, "DwellTime": null, "Reading": null },
        "UpperFatal":    { "Activation": null, "DwellTime": null, "Reading": null }
    },
    "VoltageType": null
}
```

Caveats (pp.158-160):
- Unpopulated properties come back as explicit `null`, including a `Contacts: [null]` array and a full `PostalAddress` object of nulls. DMTF practice is to leave such properties out. A strict client must accept nulls for nearly every field, and a proxy may want to strip them.
- `ReadingType` and `ReadingUnits` can be null even for a named current sensor (`CUR_PSU1_IOUT`).
- `Status.State` is `Disabled` with `Reading: 0`.
- Schema version is `#Sensor.v1_0_2.Sensor`, which is old.

## 3.42 Thermal (p.161)

Thermal metrics resource (legacy DMTF `Thermal` schema, v1_5_3 in the example).

- **URL:** `/redfish/v1/Chassis/{Chassis-Id}/Thermal`
- **Methods/privileges:** GET = Login. **No PATCH is documented.** Fan and threshold control through Thermal is not offered.

### Table 58. Supported Properties for Thermal (p.161)

| Property | Type | RO | Description |
|---|---|---|---|
| Temperatures | Array of Objects | True | Temperature sensors (Table 59). |
| Fans | Array of Objects | True | Fans (Table 61). |
| Redundancy | Array | True | Redundancy for fans. Component IDs refer to members of the redundancy groups. |
| Actions | Object | True | Holds actions under the Oem attribute, if any. |

### Table 59. Temperature Properties (pp.161-162)

| Property | Type | RO | Description |
|---|---|---|---|
| Name | String | True | Name of the temperature sensor. |
| MemberId | String | True | Uniquely identifies the member within the collection. |
| SensorNumber | Number | True | Numeric identifier for this temperature sensor, unique within this resource. |
| ReadingCelsius | Number | True | Current temperature reading. |
| UpperThresholdNonCritical | Number | True | Above normal range but not critical (same units as ReadingCelsius). |
| UpperThresholdCritical | Number | True | Above normal range, not yet fatal. |
| UpperThresholdFatal | Number | True | Above normal range and fatal. |
| LowerThresholdNonCritical | Number | True | Below normal range but not critical. |
| LowerThresholdCritical | Number | True | Below normal range, not yet fatal. |
| LowerThresholdFatal | Number | True | Below normal range and fatal. |
| MinReadingRangeTemp | Number | True | Lowest possible reading value. |
| MaxReadingRangeTemp | Number | True | Highest possible reading value. |
| PhysicalContext | Object | True | Affected device or region (Table 60). Returned as a string in the example. |
| RelatedItem | Array | True | IDs of the associated resources. |
| DeltaReadingCelsius | Number | True | Difference between this sensor's reading and the sensor at DeltaPhysicalContext. |
| DeltaPhysicalContext | Enum | True | Affected device or region that DeltaReadingCelsius is relative to. |
| MaxAllowableOperatingValue | Number | True | Maximum allowable operating temperature for the monitored equipment. |
| MinAllowableOperatingValue | Number | True | Minimum allowable operating temperature. |
| AdjustedMaxAllowableOperatingValue | Number | True | Maximum allowable operating temperature, adjusted for environmental conditions. |
| AdjustedMinAllowableOperatingValue | Number | True | Minimum allowable operating temperature, adjusted for environmental conditions. |
| OwnerLUN | (blank) | (blank) | **OEM attribute** (implementation-specific). Tells apart sensors that have the same Sensor number but different LUNs. Appears under `Oem.Ami`. |

### Table 60. ThermalPhysicalContextEnum Properties (pp.162-163)

| Enum | Description |
|---|---|
| Room | The room. |
| Intake | Intake point of the chassis. |
| Exhaust | Exhaust point of the chassis. |
| Front | Front of the chassis. |
| Back | Back of the chassis. |
| Upper | Upper portion of the chassis. |
| Lower | Lower portion of the chassis. |
| CPU | A processor (CPU). |
| GPU | A graphics processor (GPU). |
| Backplane | A backplane within the chassis. |
| SystemBoard | The system board (PCB). |
| PowerSupply | A power supply. |
| VoltageRegulator | A voltage regulator device. |
| StorageDevice | A storage device. |
| NetworkingDevice | A networking device. |
| ComputeBay | Within a compute bay. |
| StorageBay | Within a storage bay. |
| NetworkBay | Within a networking bay. |
| ExpansionBay | Within an expansion bay. |
| PowerSupplyBay | Within a power supply bay. |

### Table 61. Fan Properties (p.163)

| Property | Type | RO | Description |
|---|---|---|---|
| Name | String | True | Name of the fan. |
| MemberId | String | True | Uniquely identifies the member within the collection. |
| PhysicalContext | Object | True | Affected device or region (Table 60). |
| Reading | Number | True | Current fan sensor reading. |
| UpperThresholdNonCritical | Number | True | Above normal range, not critical (same units as Reading). |
| UpperThresholdCritical | Number | True | Above normal range, not yet fatal. |
| UpperThresholdFatal | Number | True | Above normal range and fatal. |
| LowerThresholdNonCritical | Number | True | Below normal range, not critical. |
| LowerThresholdCritical | Number | True | Below normal range, not yet fatal. |
| LowerThresholdFatal | Number | True | Below normal range and fatal. |
| MinReadingRange | Number | True | Lowest possible Reading value. |
| MaxReadingRange | Number | True | Highest possible Reading value. |
| RelatedItem | Array | True | IDs of the resources this fan serves. |
| Redundancy | Array | True | Redundancy information. |
| HotPluggable | Boolean | True | Whether the device can be inserted or removed while the equipment stays in operation. |
| SensorNumber | Number | True | Numeric identifier of the fan speed sensor. |
| OwnerLUN | Number | True | **OEM attribute.** Tells apart sensors that have the same sensor number but different LUNs. |

### Example: GET Thermal Instance (pp.163-165)

```
GET https://{BMC_IP}/redfish/v1/Chassis/{Chassis-Id}/Thermal
Content-Type: application/json
```

Response. The `...` elisions are in the PDF.

```json
{
    "@odata.context": "/redfish/v1/$metadata#Thermal.Thermal",
    "@odata.etag": "\"1699708324\"",
    "@odata.id": "/redfish/v1/Chassis/Self/Thermal",
    "@odata.type": "#Thermal.v1_5_3.Thermal",
    "Description": "Thermal sensor readings",
    "Fans": [
        {
            "@odata.id": "/redfish/v1/Chassis/Self/Thermal#/Fans/0",
            "LowerThresholdNonCritical": 200,
            "MaxReadingRange": 51000,
            "MemberId": "0",
            "MinReadingRange": 0,
            "Name": "FAN1",
            "Oem": {
                "Ami": {
                    "@odata.type": "#AMIChassisPowerThermal.v1_0_0.AMIChassisPowerThermal",
                    "OwnerLUN": 0
                }
            },
            "PhysicalContext": "Fan",
            "ReadingUnits": "RPM",
            "RelatedItem@odata.count": 0,
            "SensorNumber": 17,
            "Status": {
                "State": "Absent"
            }
        }, ...
        {
            "@odata.id": "/redfish/v1/Chassis/Self/Thermal#/Fans/13",
            "LowerThresholdNonCritical": 100,
            "MaxReadingRange": 25500,
            "MemberId": "13",
            "MinReadingRange": 0,
            "Name": "FAN_PSU2",
            "Oem": {
                "Ami": {
                    "@odata.type": "#AMIChassisPowerThermal.v1_0_0.AMIChassisPowerThermal",
                    "OwnerLUN": 0
                }
            },
            "PhysicalContext": "Fan",
            "ReadingUnits": "RPM",
            "RelatedItem@odata.count": 0,
            "SensorNumber": 125,
            "Status": {
                "State": "Absent"
            }
        }
    ],
    "Id": "Thermal",
    "Name": "Thermal",
    "Temperatures": [
        {
            "@odata.id": "/redfish/v1/Chassis/Self/Thermal#/Temperatures/0",
            "MemberId": "0",
            "Name": "TEMP_CPU",
            "Oem": {
                "Ami": {
                    "@odata.type": "#AMIChassisPowerThermal.v1_0_0.AMIChassisPowerThermal",
                    "OwnerLUN": 0
                }
            },
            "PhysicalContext": "Intake",
            "RelatedItem@odata.count": 0,
            "SensorNumber": 42,
            "Status": {
                "State": "Absent"
            },
            "UpperThresholdCritical": 99,
            "UpperThresholdNonCritical": 98
        }, ...
        {
            "@odata.id": "/redfish/v1/Chassis/Self/Thermal#/Temperatures/17",
            "MemberId": "17",
            "Name": "TEMP_M.2_2",
            "Oem": {
                "Ami": {
                    "@odata.type": "#AMIChassisPowerThermal.v1_0_0.AMIChassisPowerThermal",
                    "OwnerLUN": 0
                }
            },
            "PhysicalContext": "Intake",
            "RelatedItem@odata.count": 0,
            "SensorNumber": 92,
            "Status": {
                "State": "Absent"
            }
        }
    ]
}
```

Notes (pp.164-165):
- `Oem.Ami` (`#AMIChassisPowerThermal.v1_0_0.AMIChassisPowerThermal`, `OwnerLUN`) appears on every Fan and Temperature entry.
- `PhysicalContext` is **wrong in practice**: `TEMP_CPU` and `TEMP_M.2_2` both report `"Intake"`. Do not use PhysicalContext to classify sensors; use the sensor `Name` (IPMI SDR name).
- Fans have `ReadingUnits: "RPM"` and use `Reading`. Absent sensors leave out `Reading` and `ReadingCelsius` entirely.
- Fans has 14 entries (0..13), including PSU fans. Temperatures has 18 entries (0..17).
- **The example (taken while the host was off) shows every entry as `Absent`.** Plan for sensors that report `Absent` when the host is powered off.

## 3.43 Composition Service (p.166)

Holds links to the ResourceBlocks and ResourceZones collections.

- **URL:** `/redfish/v1/CompositionService`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureComponents |

### Table 62. Supported Properties for Composition Service (p.166)

| Property | Type | RO | Description |
|---|---|---|---|
| ServiceEnabled | Boolean | **False (writable)** | Whether this service is enabled. |
| ResourceBlocks | Object | True | Resource blocks available on the service. |
| ResourceZones | Object | True | Resource zones available on the service. |

### Example: GET Composition Service Instance (pp.166-167)

```
GET https://{BMC_IP}/redfish/v1/CompositionService
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#CompositionService.CompositionService",
    "@odata.etag": "\"1700283939\"",
    "@odata.id": "/redfish/v1/CompositionService",
    "@odata.type": "#CompositionService.v1_1_2.CompositionService",
    "Description": "Composition Service",
    "Id": "CompositionService",
    "Name": "Composition Service",
    "ResourceBlocks": {
        "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks"
    },
    "ResourceZones": {
        "@odata.id": "/redfish/v1/CompositionService/ResourceZones"
    },
    "ServiceEnabled": true,
    "Status": {
        "Health": "OK",
        "HealthRollup": "OK",
        "State": "Enabled"
    }
}
```

### Example: PATCH Composition Service Instance (p.167)

```
PATCH https://{BMC_IP}/redfish/v1/CompositionService
Content-Type: application/json
```

```json
{
    "ServiceEnabled": false
}
```

Response: **204 No Content, no body** (p.167). This differs from PATCH Power, which returns 200 with a body. A client must handle both.

## 3.44 ResourceBlocks Collection (p.168)

- **URL:** `/redfish/v1/CompositionService/ResourceBlocks`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET ResourceBlocks Collection (p.168)

```
GET https://{BMC_IP}/redfish/v1/CompositionService/ResourceBlocks
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#ResourceBlockCollection.ResourceBlockCollection",
    "@odata.etag": "\"1700283939\"",
    "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks",
    "@odata.type": "#ResourceBlockCollection.ResourceBlockCollection",
    "Description": "Collection of ResourceBlocks",
    "Members": [
        { "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/ComputeBlock" },
        { "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/DrivesBlock" },
        { "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/NetworkBlock" }
    ],
    "Members@odata.count": 3,
    "Name": "ResourceBlock Collection"
}
```

## 3.45 ResourceBlocks (p.169)

Inventory of resource instances populated from `Systems/Self`:
- **ComputeBlock:** Memory and processor instances.
- **DrivesBlock:** Storage instances.
- **NetworkBlock:** EthernetInterface instances.

- **URLs:**
  - `/redfish/v1/CompositionService/ResourceBlocks/ComputeBlock`
  - `/redfish/v1/CompositionService/ResourceBlocks/DrivesBlock`
  - `/redfish/v1/CompositionService/ResourceBlocks/NetworkBlock`
- **Methods/privileges:** GET = Login. No PATCH is listed, even though some properties are marked writable.

### Table 63. Supported Properties for ResourceBlocks (pp.169-170)

| Property | Type | RO | Description |
|---|---|---|---|
| CompositionStatus | Object | False | Composition status details for this Resource Block. |
| CompositionStatus.Reserved | Boolean | False | Whether any client has reserved the block. Defaults to null. |
| CompositionStatus.SharingCapable | Boolean | True | Description in the PDF is copied from Reserved ("reserved by any client"). In DMTF it means the block can take part in multiple compositions. |
| CompositionStatus.SharingEnabled | Boolean | False | Whether the block may take part in multiple compositions at the same time. |
| CompositionStatus.MaxCompositions | Boolean (as printed; Number in the example and in DMTF) | True | Maximum number of compositions the block can take part in at the same time. |
| CompositionStatus.NumberOfCompositions | Number | True | Number of compositions the block currently takes part in. |
| CompositionStatus.CompositionState | String | True | Current state from a composition point of view. Enum: `Composed` (final successful state after taking part in a composition), `Unused` (free and available for composition). |
| ResourceBlockType | Array | True | Types of resources available on this block. |
| (Links).Oem | Object | - | OEM extensions. (The table nests these under ResourceBlockType, but they are really `Links` members.) |
| (Links).ComputerSystems | Array | True | References to Computer Systems composed from this block. |
| (Links).Chassis | Array | True | References to the Chassis that contains this block. |
| (Links).Zones | Array | True | References to the Zones this block is bound to. |
| Processors | Array | True | References to Processors in this block. |
| Processors@odata.count | Number | True | Number of items. |
| Memory | Array | True | References to Memory in this block. |
| Memory@odata.count | Number | True | Number of items. |
| Storage | Array | True | References to Storage in this block. |
| Storage@odata.count | Number | True | Number of items. |
| SimpleStorage | Array | True | References to SimpleStorage in this block. |
| SimpleStorage@odata.count | Number | True | Number of items. |
| Drives | Array | True | References to Drives in this block. |
| Drives@odata.count | Number | True | Number of items. |
| EthernetInterfaces | Array | True | References to EthernetInterfaces in this block. |
| EthernetInterfaces@odata.count | Number | True | Number of items. |

### Example: GET ResourceBlocks Instance (pp.170-171)

```
GET https://{BMC_IP}/redfish/v1/CompositionService/ResourceBlocks/ComputeBlock
Content-Type: application/json
(or)
GET https://{BMC_IP}/redfish/v1/CompositionService/ResourceBlocks/DrivesBlock
Content-Type: application/json
(or)
GET https://{BMC_IP}/redfish/v1/CompositionService/ResourceBlocks/NetworkBlock
Content-Type: application/json
```

Response (example for NetworkBlock):

```json
{
    "@odata.context": "/redfish/v1/$metadata#ResourceBlock.ResourceBlock",
    "@odata.etag": "\"1700283939\"",
    "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/NetworkBlock",
    "@odata.type": "#ResourceBlock.v1_3_2.ResourceBlock",
    "CompositionStatus": {
        "CompositionState": "Unused",
        "MaxCompositions": 1,
        "Reserved": false
    },
    "Description": "NetworkBlock",
    "EthernetInterfaces": [
        { "@odata.id": "/redfish/v1/Systems/Self/EthernetInterfaces/EthernetInterface0" },
        { "@odata.id": "/redfish/v1/Systems/Self/EthernetInterfaces/EthernetInterface1" }
    ],
    "EthernetInterfaces@odata.count": 2,
    "Id": "NetworkBlock",
    "Links": {
        "Chassis": [
            { "@odata.id": "/redfish/v1/Chassis/Self" }
        ],
        "Zones": [
            { "@odata.id": "/redfish/v1/CompositionService/ResourceZones/1" }
        ]
    },
    "Name": "NetworkBlock",
    "ResourceBlockType": [
        "Network"
    ],
    "Status": {
        "Health": "OK",
        "HealthRollup": "OK",
        "State": "Enabled"
    }
}
```

### Example: PATCH ResourceBlocks Instance (p.171)

PATCH is shown here even though the method table (3.45.2) lists only GET.

```
PATCH https://{BMC_IP}/redfish/v1/CompositionService/ResourceBlocks/ComputeBlock
Content-Type: application/json
(or)
PATCH https://{BMC_IP}/redfish/v1/CompositionService/ResourceBlocks/DrivesBlock
Content-Type: application/json
(or)
PATCH https://{BMC_IP}/redfish/v1/CompositionService/ResourceBlocks/NetworkBlock
Content-Type: application/json
```

```json
{
    "CompositionStatus": {
        "Reserved": false
    }
}
```

Response: **204, no body.**

## 3.46 ResourceZone Collection (p.172)

- **URL:** `/redfish/v1/CompositionService/ResourceZones`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET ResourceZone Collection (p.172)

```
GET https://{BMC_IP}/redfish/v1/CompositionService/ResourceZones
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#ZoneCollection.ZoneCollection",
    "@odata.etag": "\"1700283939\"",
    "@odata.id": "/redfish/v1/CompositionService/ResourceZones",
    "@odata.type": "#ZoneCollection.ZoneCollection",
    "Description": "Collection of Zones",
    "Members": [
        { "@odata.id": "/redfish/v1/CompositionService/ResourceZones/1" }
    ],
    "Members@odata.count": 1,
    "Name": "Zone Collection"
}
```

## 3.47 ResourceZone (p.173)

A simple fabric zone.

- **URL:** `/redfish/v1/CompositionService/ResourceZones/{ResourceZones-Instance}`
- **Methods/privileges:** GET = Login

### Table 64. Supported Properties for ResourceZone (p.173)

| Property | Type | RO | Description |
|---|---|---|---|
| Links | Object | True | References to related resources. |
| Links.Oem | Object | - | OEM extensions. |
| Links.ResourceBlocks | Array | True | References to the Resource Blocks used in this Zone. |

### Example: GET ResourceZone Instance (pp.173-174)

```
GET https://{BMC_IP}/redfish/v1/CompositionService/ResourceZones/{ResourceZones-Instance}
Content-Type: application/json
```

```json
{
    "@Redfish.CollectionCapabilities": {
        "@odata.type": "#CollectionCapabilities.v1_2_0.CollectionCapabilities",
        "Capabilities": [
            {
                "CapabilitiesObject": {
                    "@odata.id": "/redfish/v1/Systems/Capabilities"
                },
                "Links": {
                    "TargetCollection": {
                        "@odata.id": "/redfish/v1/Systems"
                    }
                },
                "UseCase": "ComputerSystemComposition"
            }
        ],
        "MaxMembers": 1
    },
    "@odata.context": "/redfish/v1/$metadata#Zone.Zone",
    "@odata.etag": "\"1700283939\"",
    "@odata.id": "/redfish/v1/CompositionService/ResourceZones/1",
    "@odata.type": "#Zone.v1_3_1.Zone",
    "Description": "Resource Zone 1",
    "Id": "1",
    "Links": {
        "ResourceBlocks": [
            { "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/ComputeBlock" },
            { "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/DrivesBlock" },
            { "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/NetworkBlock" }
        ]
    },
    "Name": "Resource Zone 1",
    "Status": {
        "Health": "OK",
        "HealthRollup": "OK",
        "State": "Enabled"
    }
}
```

Note: `@Redfish.CollectionCapabilities` points to `/redfish/v1/Systems/Capabilities` with UseCase `ComputerSystemComposition` and `MaxMembers: 1`. That suggests composing a system by POSTing to `/redfish/v1/Systems` is advertised, but this range of the PDF does not document it. (p.173)

## 3.48 Event Service API (p.175)

The Event Service manages event subscriptions and produces the events sent to subscribers. It links to the subscription collection (Event Destinations).

- **URL:** `/redfish/v1/EventService`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureManager |
| POST | ConfigureManager (for the SubmitTestEvent action) |

### Table 65. Supported Properties for Event Service API (pp.175-176)

| Property | Type | RO | Description |
|---|---|---|---|
| ServiceEnabled | Boolean | False | Whether this service is enabled. |
| DeliveryRetryAttempts | Number | False | Number of retries for an event to the subscription destination before the **subscription is terminated**. |
| DeliveryRetryIntervalSeconds | Number | False | Seconds between retry attempts for an event to the subscription destination. |
| EventFormatTypes | Array | True | Message types this service can send to the destination. `MetricReport`: the destination gets MetricReport JSON bodies, only when the TelemetryService has created a new Metric Report or updated an existing one. `Event`: the destination gets Event JSON bodies for all other event types. |
| RegistryPrefixes | Array | True | Message Registry prefixes allowed for an Event Subscription. **Supported: `EventLog`, `SyncAgent`, `Security`, `IPMI`, `HttpStatus`, `Base`, `Task`.** |
| ResourceTypes | Array | True | ResourceTypes values allowed for an Event Subscription. **Supported: `Systems`, `Chassis`, `AccountService`, `TelemetryService`, `Managers`, `EventService`, `TaskService`.** |
| SubordinateResourcesSupported | Boolean | True | Whether the SubordinateResource property is supported on Event Subscription. |
| Actions | Object | True | Custom actions available on this resource, such as SubmitTestEvent or any Oem action. |
| Subscriptions | Object | True | Reference to the EventDestinationCollection. |
| ServerSentEventUri | String | True | URI for receiving the Server-Sent Event representation of events. |
| SSEFilterPropertiesSupported | Object | True | Properties supported in the filter query parameter for the ServerSentEventUri: `EventFormatType` True, `MessageId` True, `MetricReportDefinition` **False**, `OriginResource` True, `RegistryPrefix` True, `ResourceType` True, `SubordinateResources` **False**. |
| SMTP | Object | True | Parameters for the SMTP service (members writable). |
| SMTP.Authentication | String | False | Authentication method for the SMTP server. **`Plain` and `None`** are the options. None means no authentication. Plain is basic authentication with the SMTP server. Setting None does **not** delete the Username and Password parameters. |
| SMTP.ConnectionProtocol | String | False | Connection type to the outgoing SMTP server. **`None`, `StartTLS`, `TLS_SSL`.** None sends mail over a non-secure port. TLS_SSL sends over a secure port. StartTLS sends over TLS. |
| SMTP.FromAddress | String | False | Address for the 'from' field of outgoing email. |
| SMTP.Password | String | False | Password for authenticating with the SMTP server. |
| SMTP.Port | Number | False | Destination port of the SMTP server. |
| SMTP.ServerAddress | String | False | Address of the SMTP server for outgoing email. |
| SMTP.ServiceEnabled | Boolean | False | Whether SMTP event delivery is enabled. |
| SMTP.Username | String | False | Username for authenticating with the SMTP server. |

### Example: GET Event Service Instance (pp.176-178)

```
GET https://{BMC_IP}/redfish/v1/EventService
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#EventService.EventService",
    "@odata.etag": "\"1700283939\"",
    "@odata.id": "/redfish/v1/EventService",
    "@odata.type": "#EventService.v1_5_0.EventService",
    "Actions": {
        "#EventService.SubmitTestEvent": {
            "@Redfish.ActionInfo": "/redfish/v1/EventService/SubmitTestEventActionInfo",
            "target": "/redfish/v1/EventService/Actions/EventService.SubmitTestEvent"
        }
    },
    "DeliveryRetryAttempts": 3,
    "DeliveryRetryIntervalSeconds": 60,
    "Description": "Event Service",
    "EventFormatTypes": [
        "Event",
        "MetricReport"
    ],
    "Id": "EventService",
    "Name": "Event Service",
    "Oem": {
        "Ami": {
            "@odata.type": "#AMIEventService.v1_0_0.AMIEventService",
            "Certificates": {
                "@odata.id": "/redfish/v1/EventService/Oem/Ami/SMTP/Certificates"
            },
            "SecondarySMTP": {
                "Authentication": "None",
                "ConnectionProtocol": "None",
                "FromAddress": null,
                "Password": null,
                "Port": 25,
                "ServerAddress": null,
                "ServiceEnabled": false,
                "Username": null
            }
        }
    },
    "RegistryPrefixes": [
        "EventLog",
        "IPMI",
        "HttpStatus",
        "SyncAgent",
        "Task",
        "Security",
        "Base"
    ],
    "ResourceTypes": [
        "AccountService",
        "Chassis",
        "TaskService",
        "Systems",
        "Managers",
        "EventService",
        "TelemetryService"
    ],
    "SMTP": {
        "Authentication": "None",
        "ConnectionProtocol": "None",
        "FromAddress": null,
        "Password": null,
        "Port": 25,
        "ServerAddress": null,
        "ServiceEnabled": false,
        "Username": null
    },
    "SSEFilterPropertiesSupported": {
        "EventFormatType": true,
        "MessageId": true,
        "MetricReportDefinition": false,
        "OriginResource": true,
        "RegistryPrefix": true,
        "ResourceType": true,
        "SubordinateResources": false
    },
    "ServerSentEventUri": "/redfish/v1/EventService/SSE",
    "ServiceEnabled": true,
    "Status": {
        "State": "Enabled"
    },
    "SubordinateResourcesSupported": false,
    "Subscriptions": {
        "@odata.id": "/redfish/v1/EventService/Subscriptions"
    }
}
```

Notes (pp.176-178):
- **OEM `Oem.Ami`** (`#AMIEventService.v1_0_0.AMIEventService`):
  - `Certificates` links to `/redfish/v1/EventService/Oem/Ami/SMTP/Certificates`, the SMTP TLS certificate collection.
  - `SecondarySMTP` is a second SMTP server object with the same fields as `SMTP` (`Authentication`, `ConnectionProtocol`, `FromAddress`, `Password`, `Port`, `ServerAddress`, `ServiceEnabled`, `Username`).
- **SSE endpoint:** `/redfish/v1/EventService/SSE`. Filtering on `MetricReportDefinition` and `SubordinateResources` is not supported.
- `SMTP.Password` is returned in the GET (null here). Whether a non-null password is echoed back is not stated. A proxy should consider redacting it.
- An ActionInfo for SubmitTestEvent exists at `/redfish/v1/EventService/SubmitTestEventActionInfo`.

### Example: PATCH Event Service Instance (p.178)

```
PATCH https://{BMC_IP}/redfish/v1/EventService
Content-Type: application/json
```

```json
{
    "ServiceEnabled": false
}
```

Response: **204, no body.**

### Example: POST Submit Test Event (pp.178-179)

The available MessageId, MessageArgs, and Severity values are in the registries at:

```
https://{BMC_IP}/redfish/v1/Registries/{{Registry_instance.json}}
```

Request:

```
POST https://{BMC_IP}/redfish/v1/EventService/Actions/EventService.SubmitTestEvent
Content-Type: application/json
```

POST EventLog example request body:

```json
{
    "MessageId": "EventLog.1.0.ResourceAdded"
}
```

Response: **202 Accepted, with the created Task instance as the body** (p.179):

```json
{
    "@odata.context": "/redfish/v1/$metadata#Task.Task",
    "@odata.id": "/redfish/v1/TaskService/Tasks/1",
    "@odata.type": "#Task.v1_4_2.Task",
    "Description": "Task for EventService SubmitTestEvent Action",
    "Id": "1",
    "Name": "EventService SubmitTestEvent Action",
    "TaskState": "New"
}
```

Caveat: SubmitTestEvent is **asynchronous**. It returns a Task at `/redfish/v1/TaskService/Tasks/{id}` and does not return 204 as most implementations do. The PDF does not show whether a `Location` header is returned; expect to poll the Task URI. Only `MessageId` is shown in the body; other parameters (MessageArgs, Severity, OriginOfCondition) are taken from the registry. (pp.178-179)

## 3.49 Event Subscription Collection (p.180)

Collection of subscription IDs on this EventService. Follows the EventDestinationCollection schema.

- **URL:** `/redfish/v1/EventService/Subscriptions`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| POST | ConfigureManager |

- **Properties:** common ODATA properties only.

### Example: GET Event Subscription Collection (p.180)

```
GET https://{BMC_IP}/redfish/v1/EventService/Subscriptions
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#EventDestinationCollection.EventDestinationCollection",
    "@odata.etag": "\"1700315201\"",
    "@odata.id": "/redfish/v1/EventService/Subscriptions",
    "@odata.type": "#EventDestinationCollection.EventDestinationCollection",
    "Description": "Collection for Event Subscriptions",
    "Members": [
        { "@odata.id": "/redfish/v1/EventService/Subscriptions/1" }
    ],
    "Members@odata.count": 1,
    "Name": "Event Subscriptions Collection"
}
```

### Example: POST Event Subscription Collection (pp.180-181)

The supported EventFormatType, RegistryPrefixes, and ResourceTypes values are listed at `https://{BMC_IP}/redfish/v1/EventService`.

Request (p.181):

```
POST https://{BMC_IP}/redfish/v1/EventService/Subscriptions
Content-Type: application/json
```

```json
{
    "Context": "TestEvent",
    "Destination": "http://192.168.246.103:5000/event",
    "EventFormatType": "Event",
    "RegistryPrefixes": [
        "SyncAgent",
        "Base",
        "EventLog"
    ],
    "ResourceTypes": [
        "Chassis",
        "AccountService",
        "Systems",
        "EventService"
    ],
    "Protocol": "Redfish",
    "OriginResources": [
        {"@odata.id": "/redfish/v1/AccountService"}
    ]
}
```

Response: **201, with the new EventDestination as the body** (pp.181-182):

```json
{
    "@odata.context": "/redfish/v1/$metadata#EventDestination.EventDestination",
    "@odata.etag": "\"1700315201\"",
    "@odata.id": "/redfish/v1/EventService/Subscriptions",
    "@odata.type": "#EventDestination.v1_7_0.EventDestination",
    "Context": "TestEvent",
    "DeliveryRetryPolicy": "TerminateAfterRetries",
    "Description": "Event Subscription",
    "Destination": "http://192.168.246.103:5000/event",
    "EventFormatType": "Event",
    "Id": "1",
    "Name": "Subscription 1",
    "OriginResources": [
        "Redfish:AccountService"
    ],
    "OriginResources@odata.count": 1,
    "Protocol": "Redfish",
    "RegistryPrefixes": [
        "SyncAgent",
        "EventLog",
        "Base"
    ],
    "ResourceTypes": [
        "Systems",
        "AccountService",
        "Chassis",
        "EventService"
    ],
    "Status": {
        "Health": "OK",
        "HealthRollup": "OK",
        "State": "Enabled"
    },
    "SubordinateResources": false,
    "SubscriptionType": "RedfishEvent"
}
```

Caveats on the POST response (pp.181-182):
- The `@odata.id` in the 201 body is the **collection URI** (`/redfish/v1/EventService/Subscriptions`), not `/Subscriptions/1`. Get the new resource URI from `Id` or from the `Location` header (the PDF does not show headers), not from the body's `@odata.id`.
- In the POST response, `OriginResources` is an **array of strings in a non-standard form `"Redfish:AccountService"`**. A later GET of the instance returns standard `{"@odata.id": ...}` objects (p.184).
- An `http://` (non-TLS) destination is accepted.
- `DeliveryRetryPolicy` defaults to `TerminateAfterRetries`, and `SubscriptionType` defaults to `RedfishEvent`.

## 3.50 Event Subscription (p.183)

A single event subscription. Follows the EventDestination schema.

- **URL:** `/redfish/v1/EventService/Subscriptions/{Subscriptions-Instance}`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureManager |
| POST | ConfigureManager (ResumeSubscription action) |
| DELETE | ConfigureManager |

### Table 66. Supported Properties for Event Subscription (pp.183-184)

| Property | Type | RO | Description |
|---|---|---|---|
| Destination | String | True | URI of the destination the events are sent to. When the protocol is SMTP, this is the destination e-mail address. |
| Context | String | **False** | Client-supplied string stored with the subscription. |
| SubscriptionType | String | True | Type of subscription. |
| Protocol | String | True | Protocol type of the event connection. |
| MessageIds | Array | True | The only MessageIds the service will send. |
| OriginResources | Array | True | The only resources the service will send related events for. |
| OriginResources@odata.count | Number | True | Number of items. |
| Actions | Object | True | Actions for this resource, under Oem if any. |
| SubordinateResources | Boolean | True | When true, OriginResources also covers subordinate resources. |
| EventFormatType (printed "EventFormarType") | String | True | Message type sent to the destination. `MetricReport`: MetricReport JSON bodies, only when the TelemetryService creates or updates a Metric Report. `Event`: Event JSON bodies for all other event types. |
| RegistryPrefixes | Array | True | Prefixes of the Message Registries that contain the MessageIds. |
| ResourceTypes | Array | True | Resource type values that correspond to the OriginOfCondition. |
| MetricReportDefinitions | Array | True | The only metric report definitions allowed to produce metric reports for this subscription. |
| DeliveryRetryPolicy | String | **False** | Retry policy for events when the subscription type is RedfishEvent. `RetryForever`: the subscription is never suspended or terminated, and delivery of future events continues after the maximum number of retries. `SuspendRetries`: the subscription is suspended after the maximum number of retries. `TerminateAfterRetries`: the subscription is terminated after the maximum number of retries, and **the subscription is deleted after the retry attempts**. |

Writable properties: only `Context` and `DeliveryRetryPolicy`. Destination, filters, and other properties cannot be PATCHed; delete the subscription and create it again. (pp.183-184)

### Example: GET Event Subscription Instance (pp.184-185)

```
GET https://{BMC_IP}/redfish/v1/EventService/Subscriptions/{Subscriptions-Instance}
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#EventDestination.EventDestination",
    "@odata.etag": "\"1700315201\"",
    "@odata.id": "/redfish/v1/EventService/Subscriptions/1",
    "@odata.type": "#EventDestination.v1_7_0.EventDestination",
    "Context": "TestEvent",
    "DeliveryRetryPolicy": "TerminateAfterRetries",
    "Description": "Event Subscription",
    "Destination": "http://192.168.246.103:5000/event",
    "EventFormatType": "Event",
    "Id": "1",
    "Name": "Subscription 1",
    "OriginResources": [
        {
            "@odata.id": "/redfish/v1/AccountService"
        }
    ],
    "OriginResources@odata.count": 1,
    "Protocol": "Redfish",
    "RegistryPrefixes": [
        "SyncAgent",
        "EventLog",
        "Base"
    ],
    "ResourceTypes": [
        "Systems",
        "AccountService",
        "Chassis",
        "EventService"
    ],
    "Status": {
        "Health": "OK",
        "HealthRollup": "OK",
        "State": "Enabled"
    },
    "SubordinateResources": false,
    "SubscriptionType": "RedfishEvent"
}
```

### Example: PATCH Event Subscription Instance (p.185)

```
PATCH https://{BMC_IP}/redfish/v1/EventService/Subscriptions/{Subscriptions-Instance}
Content-Type: application/json
```

```json
{
    "Context": "ChangeTestEventContext"
}
```

Response: **204, no body.**

### Example: POST Resume Event Subscription Instance (p.185)

Resumes a suspended event subscription, which changes the subscription status. **The action link appears in the subscription instance only while the subscription is suspended.**

```
POST https://{BMC_IP}/redfish/v1/EventService/Subscriptions/{{Subscriptions-Instance}}/Actions/EventDestination.ResumeSubscription
Content-Type: application/json
```

No request body is shown. Response: **204, no body.**

### Example: DELETE Event Subscription Instance (p.185)

```
DELETE https://{BMC_IP}/redfish/v1/EventService/Subscriptions/{Subscriptions-Instance}
Content-Type: application/json
```

Response: **204, no body.**

## 3.51 SMTP Certificate OEM Properties (p.186)

**AMI OEM resource** for adding, editing, or deleting the SMTP StartTLS certificates.

- **URL:** `/redfish/v1/EventService/Oem/Ami/SMTP/Certificates`
- **Methods/privileges:** Note that **even GET needs ConfigureManager**.

| Method | Privilege |
|---|---|
| GET | ConfigureManager |
| POST | ConfigureManager |
| DELETE | ConfigureManager |

### Table 67. Supported Properties for SMTP Certificate OEM Properties (p.186)

| Property | Type | RO | Description |
|---|---|---|---|
| PrimarySMTP | Object | True | Parameters for the SMTP certificates needed for StartTLS. |
| PrimarySMTP.CACert | String | False | CA certificate for SMTP (PEM). |
| PrimarySMTP.Cert | String | False | Client certificate for SMTP (PEM). |
| PrimarySMTP.PrivateKey | String | False | Private key for SMTP (PEM). **Not shown on GET.** |
| SecondarySMTP | Object | True | Parameters for the Secondary SMTP certificates needed for StartTLS. |
| SecondarySMTP.CACert | String | False | CA certificate for the secondary SMTP server. |
| SecondarySMTP.Cert | String | False | Client certificate for the secondary SMTP server. |
| SecondarySMTP.PrivateKey | String | False | Private key for the secondary SMTP server. **Not shown on GET.** |

### Example: GET SMTP Certificates Collection (pp.186-187)

```
GET https://{BMC_IP}/redfish/v1/EventService/Oem/Ami/SMTP/Certificates
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#AMISMTPCertificateCollection.AMISMTPCertificateCollection",
    "@odata.etag": "\"1700395016\"",
    "@odata.id": "/redfish/v1/EventService/Oem/Ami/SMTP/Certificates",
    "@odata.type": "#AMISMTPCertificateCollection.v1_0_0.AMISMTPCertificateCollection",
    "Certificates": [
        { "@odata.id": "/redfish/v1/EventService/Oem/Ami/SMTP/Certificates/1" },
        { "@odata.id": "/redfish/v1/EventService/Oem/Ami/SMTP/Certificates/2" }
    ],
    "Certificates@odata.count": 2,
    "Description": "Collection for SMTP Certificate Collection",
    "Id": "SMTP Certificate Collection",
    "Name": "SMTP Certificate Collection"
}
```

Note: the collection uses a **`Certificates` array instead of the standard `Members`**, and `Certificates@odata.count` instead of `Members@odata.count`. The `Id` contains spaces. (p.187)

### Example: GET SMTP Certificates instance (pp.187-188)

```
GET https://{BMC_IP}/redfish/v1/EventService/Oem/Ami/SMTP/Certificates/{Certificates-Instance}
Content-Type: application/json
```

Response. The base64 PEM bodies are shortened here because they are sample data only.

```json
{
    "@odata.context": "/redfish/v1/$metadata#AMISMTPCertificate.AMISMTPCertificate",
    "@odata.etag": "\"1700395016\"",
    "@odata.id": "/redfish/v1/EventService/Oem/Ami/SMTP/Certificates/1",
    "@odata.type": "#AMISMTPCertificate.v1_0_0.AMISMTPCertificate",
    "PrimarySMTP": {
        "CACert": "-----BEGIN CERTIFICATE-----\nMIIGMzCCBBugAwIBAgIUJTLvafPltmV8ks4Pf39WdiMLpswwDQYJKoZIhvcNAQEL\n...[sample base64 omitted]...\n+xu7Lz9rZA==\n-----END  CERTIFICATE-----",
        "Cert": "-----BEGIN CERTIFICATE-----\nMIIF3zCCA8cCFGNEbCleqtr/MQ2JTqvY0wc7/iG7MA0GCSqGSIb3DQEBCwUAMIGo\n...[sample base64 omitted]...\nnnfpSSCOHIml9DxHQOQg61Wr/1w==\n-----END  CERTIFICATE-----"
    }
}
```

Notes: PEM strings are sent as JSON strings with `\n` line breaks. `PrivateKey` is left out of GET responses. Instance 1 appears to hold PrimarySMTP and instance 2 SecondarySMTP (inferred from the two members). The PEM trailer in the example has two spaces (`-----END  CERTIFICATE-----`), probably a copy artefact. (pp.187-188)

### Example: POST Adding Certificates for StartTLS Connection Protocol SMTP (pp.188-191)

```
POST https://{BMC_IP}/redfish/v1/EventService/Oem/Ami/SMTP/Certificates/
Content-Type: application/json
```

Request body (PEM content shortened):

```json
{
    "PrimarySMTP": {
        "CACert": "-----BEGIN CERTIFICATE-----\n...\n-----END  CERTIFICATE-----",
        "Cert": "-----BEGIN CERTIFICATE-----\n...\n-----END  CERTIFICATE-----",
        "PrivateKey": "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"
    }
}
```

Response: **201, with a body** (pp.190-191):

```json
{
    "@odata.context": "/redfish/v1/$metadata#AMISMTPCertificate.AMISMTPCertificate",
    "@odata.etag": "\"1700394819\"",
    "@odata.id": "/redfish/v1/EventService/Oem/Ami/SMTP/Certificates",
    "@odata.type": "#AMISMTPCertificate.v1_0_0.AMISMTPCertificate",
    "PrimarySMTP": {
        "CACert": "-----BEGIN CERTIFICATE-----\n...[same CA cert as request]...\n-----END  CERTIFICATE-----",
        "Cert": "-----BEGIN CERTIFICATE-----\n...[same client cert as request]...\n-----END  CERTIFICATE-----"
    }
}
```

Notes: the 201 body's `@odata.id` is again the collection URI. `PrivateKey` is not echoed. The POST body has only `PrimarySMTP`; presumably a `SecondarySMTP` object is posted the same way. (p.191)

### Example: DELETE SMTP Certificates (p.191)

```
DELETE https://{BMC_IP}/redfish/v1/EventService/Oem/Ami/SMTP/Certificates/{Certificates-Instance}
Content-Type: application/json
```

Response: **204, no body.**

## 3.52 JSONSchemaFile Collection (p.192)

Collection of links to each OEM JSON schema hosted locally.

- **URL:** `/redfish/v1/JsonSchemas`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET JSONSchemaFile Collection (pp.192-194)

```
GET https://{BMC_IP}/redfish/v1/JsonSchemas
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#JsonSchemaFileCollection.JsonSchemaFileCollection",
    "@odata.etag": "\"1700283939\"",
    "@odata.id": "/redfish/v1/JsonSchemas",
    "@odata.type": "#JsonSchemaFileCollection.JsonSchemaFileCollection",
    "Description": "List of JSON Oem schemas and Extensions.",
    "Members": [
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIAccountService.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIAccountServiceConfigurations.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIAdvanceRADIUSSetting.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIBIOSInventoryCRC.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIBiosTable.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIBiosTableTags.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMICaCert.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMICertificateCollection.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIChassisPowerThermal.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIConfigurations.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIEventService"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIEventService.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIExternalAccountProvider.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIIPV6.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIInventoryStatus.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIManager.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIManagerAccount.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIRADIUS.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMISMTP.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMISMTPCertificate"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMISMTPCertificate.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMISMTPCertificateCollection"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMISMTPCertificateCollection.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIServiceRoot.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMITriggers.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIUpdateService.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AMIVirtualMedia.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/AmiBios.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/BiosStaticFile.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/BiosStaticFileCollection"},
        {"@odata.id": "/redfish/v1/JsonSchemas/ChangePassword.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/InventoryCRC.v1_0_0"},
        {"@odata.id": "/redfish/v1/JsonSchemas/ResetBios.v1_0_0"}
    ],
    "Members@odata.count": 33,
    "Name": "Schema Repository"
}
```

Note: this is the complete list of AMI OEM schemas the BMC hosts. It is a useful inventory of the OEM namespaces a proxy may see: `AMIManager`, `AMIUpdateService`, `AMIVirtualMedia`, `AmiBios`, `AMIBiosTable`, `AMIBIOSInventoryCRC`, `BiosStaticFile`, `ChangePassword`, `ResetBios`, `InventoryCRC`, `AMIRADIUS`, `AMIIPV6`, `AMITriggers`, and others. The JSON schema files themselves can be downloaded at `/redfish/v1/JsonSchemas/<name>.json`. **No ASRockRack-specific OEM schemas are listed**, only AMI ones. (pp.192-194)

## 3.53 JsonSchemaFile (p.195)

Schema File locator resource.

- **URL:** `/redfish/v1/JsonSchemas/<json-schema-name>`
- **Methods/privileges:** GET = Login

### Table 68. Supported Properties for JsonSchemaFile (p.195)

| Property | Type | RO | Description |
|---|---|---|---|
| Languages | Array | True | RFC 5646 language codes. |
| Schema | String | True | `@odata.type` value for that schema, in the Redfish Type syntax. |
| Location | Array | True | Location information for this schema file (Table 69). |
| Actions | Object | True | Actions under Oem, if any. |

### Table 69. JSON Schema File Location Properties (p.195)

| Property | Type | RO | Description |
|---|---|---|---|
| Language | String | True | Language code of the schema file. |
| Uri | String | True | Local URI of the schema. |

### Example: GET JsonSchemaFile Instance (pp.195-196)

```
GET https://{BMC_IP}/redfish/v1/JsonSchemas/<json-schema-name>
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#JsonSchemaFile.JsonSchemaFile",
    "@odata.etag": "\"1700396825\"",
    "@odata.id": "/redfish/v1/JsonSchemas/AMIAccountService.v1_0_0",
    "@odata.type": "#JsonSchemaFile.v1_1_4.JsonSchemaFile",
    "Description": "AMIAccountService.v1_0_0 schema",
    "Id": "AMIAccountService.v1_0_0",
    "Languages": [
        "en"
    ],
    "Location": [
        {
            "Language": "en",
            "Uri": "/redfish/v1/JsonSchemas/AMIAccountService.v1_0_0.json"
        }
    ],
    "Name": "AMIAccountService.v1_0_0 schema",
    "Schema": "AMIAccountService.v1_0_0.AMIAccountService"
}
```

Note: `Schema` leaves out the leading `#`. (p.196)

## 3.54 Manager Collection (p.197)

- **URL:** `/redfish/v1/Managers`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET Manager Collection (p.197)

```
GET https://{BMC_IP}/redfish/v1/Managers
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#ManagerCollection.ManagerCollection",
    "@odata.etag": "\"1700396825\"",
    "@odata.id": "/redfish/v1/Managers",
    "@odata.type": "#ManagerCollection.ManagerCollection",
    "Description": "The collection for Managers",
    "Members": [
        { "@odata.id": "/redfish/v1/Managers/Self" }
    ],
    "Members@odata.count": 1,
    "Name": "Manager Collection"
}
```

The manager ID is `Self`.

## 3.55 Manager (p.198)

Schema for a Manager (BMC).

- **URL:** `/redfish/v1/Managers/{Manager-Instance}` (that is, `/redfish/v1/Managers/Self`)
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureManager |
| POST | ConfigureManager (actions) |

### Table 70. Supported Properties for Manager (pp.198-199)

| Property | Type | RO | Description |
|---|---|---|---|
| ManagerType | String | True | Manager type. Value: `BMC` (a controller providing management functions for a single computer system). |
| Links | Object | True | References to related resources that this resource does not contain. |
| Links.ManagerForServers | Array | - | Systems this manager controls. |
| Links.ManagerForServers@odata.count | Number | True | Number of items. |
| Links.ManagerForSwitches | Array | - | Switches this manager controls. |
| Links.ManagerForSwitches@odata.count | Number | True | Number of items. |
| Links.ManagerForChassis | Array | True | Chassis this manager controls. |
| Links.ManagerForChassis@odata.count | Number | True | Number of items. |
| Links.ManagerInChassis | Array | True | Reference to the chassis this manager is located in. |
| Links.ActiveSoftwareImage | Array | True | Link to the SoftwareInventory resource for the active firmware image of this manager. |
| Links.ActiveSoftwareImage@odata.count | Number | True | Number of items. |
| Links.SoftwareImages | Array | True | Links to the SoftwareInventory resources for the firmware images that apply to this manager. |
| Links.SoftwareImages@odata.count | Number | True | Number of items. |
| ServiceEntryPointUUID | String | True | UUID of the Redfish Service this manager provides. |
| UUID | String | True | UUID of this Manager. |
| Model | String | True | Model number of this manager. |
| DateTime | String | **False** | Current DateTime (with offset) of the manager. Used to set or read the time. |
| DateTimeLocalOffset | String | **False** | Offset from UTC that DateTime is set to, in the format `+06:00`. |
| FirmwareVersion | String | True | Firmware version of this Manager. |
| SerialConsole | Object | - | Serial Console service information (Table 71). |
| CommandShell | Object | - | Command Shell service information (Table 72). |
| GraphicalConsole | Object | - | Graphical Console (KVM-IP) service information (Table 73). |
| Actions | Object | True | Manager actions, such as Reset. |
| EthernetInterfaces | Object | True | Link to the collection of NICs this manager uses. |
| SerialInterfaces | Object | True | Link to the collection of serial interfaces this manager uses. |
| NetworkProtocol | Object | True | Link to the network services and settings the Manager controls. |
| LogServices | Object | True | Link to the collection of Logs the manager uses. |
| VirtualMedia | Object | True | Link to the Virtual Media services of this manager. |
| Redundancy | Array | True | Redundancy information for the managers of this system. |
| Redundancy@odata.count | Number | True | Number of items. |
| HostInterfaces | Object | - | Link to the collection of Host Interfaces this manager uses for local host communication. |
| AutoDSTEnabled | Boolean | True | Whether automatic Daylight Saving Time adjustment of DateTime is enabled. (Read-only here. Writable in DMTF.) |
| PowerState | String | True | Power state of the Manager. |

### Table 71. SerialConsole Properties (p.199)

| Property | Type | RO | Description |
|---|---|---|---|
| ServiceEnabled | Boolean | False | Whether the service is enabled for this manager. |
| MaxConcurrentSessions | String (as printed; Number in DMTF and the example) | True | Maximum concurrent service sessions, across all protocols. Min 0. |
| ConnectTypesSupported | Array | True | Enum values: `Telnet` (Serial Console over Telnet), `IPMI` (Serial Console over IPMI Serial-over-LAN, SOL), `Oem` (OEM-specific protocol). |

### Table 72. CommandShell Properties (p.200)

| Property | Type | RO | Description |
|---|---|---|---|
| ServiceEnabled | Boolean | False | Whether the service is enabled for this manager. |
| MaxConcurrentSessions | String (as printed) | True | Maximum concurrent service sessions. Min 0. |
| ConnectTypesSupported | Array | True | Enum values: `SSH`, `Telnet`, `IPMI`, `Oem`. |

### Table 73. GraphicalConsole Properties (p.200)

| Property | Type | RO | Description |
|---|---|---|---|
| ServiceEnabled | Boolean | False | Whether the service is enabled for this manager. |
| MaxConcurrentSessions | String (as printed) | True | Maximum concurrent service sessions. Min 0. |
| ConnectTypesSupported | Array | True | Enum values: `KVMIP` (KVM-IP: Keyboard, Video, Mouse redirection over IP), `Oem`. |

### Example: GET Manager Instance (pp.200-204)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#Manager.Manager",
    "@odata.etag": "\"1700396825\"",
    "@odata.id": "/redfish/v1/Managers/Self",
    "@odata.type": "#Manager.v1_13_0.Manager",
    "Actions": {
        "#Manager.Reset": {
            "@Redfish.ActionInfo": "/redfish/v1/Managers/Self/ResetActionInfo",
            "@Redfish.OperationApplyTimeSupport": {
                "@odata.type": "#Settings.v1_2_2.OperationApplyTimeSupport",
                "MaintenanceWindowDurationInSeconds": 600,
                "MaintenanceWindowResource": {
                    "@odata.id": "/redfish/v1/Managers/Self"
                },
                "SupportedValues": [
                    "Immediate",
                    "AtMaintenanceWindowStart"
                ]
            },
            "target": "/redfish/v1/Managers/Self/Actions/Manager.Reset"
        },
        "Oem": {
            "#AMIManager.RedfishDBReset": {
                "@Redfish.ActionInfo": "/redfish/v1/Managers/Self/Oem/RedfishDBResetActionInfo",
                "target": "/redfish/v1/Managers/Self/Actions/Oem/AMIManager.RedfishDBReset"
            },
            "#AMIManager.RedfishPowerSaveMode": {
                "@Redfish.ActionInfo": "/redfish/v1/Managers/Self/Oem/RedfishPowerSaveModeActionInfo",
                "target": "/redfish/v1/Managers/Self/Actions/Oem/AMIManager.RedfishPowerSaveMode"
            },
            "#AMIVirtualMedia.ConfigureCDInstance": {
                "@Redfish.ActionInfo": "/redfish/v1/Managers/Self/Oem/ConfigureCDInstanceActionInfo",
                "target": "/redfish/v1/Managers/Self/Actions/Oem/AMIVirtualMedia.ConfigureCDInstance"
            },
            "#AMIVirtualMedia.EnableRMedia": {
                "@Redfish.ActionInfo": "/redfish/v1/Managers/Self/Oem/EnableRMediaActionInfo",
                "target": "/redfish/v1/Managers/Self/Actions/Oem/AMIVirtualMedia.EnableRMedia"
            }
        }
    },
    "Certificates": {
        "@odata.id": "/redfish/v1/Managers/Self/Certificates"
    },
    "CommandShell": {
        "ConnectTypesSupported": [
            "SSH",
            "IPMI"
        ],
        "MaxConcurrentSessions": 36,
        "ServiceEnabled": true
    },
    "DateTime": "2023-11-19T13:02:32-00:00",
    "DateTimeLocalOffset": "-00:00",
    "Description": "BMC",
    "EthernetInterfaces": {
        "@odata.id": "/redfish/v1/Managers/Self/EthernetInterfaces"
    },
    "FirmwareVersion": "5.06.0",
    "GraphicalConsole": {
        "ConnectTypesSupported": [
            "KVMIP"
        ],
        "MaxConcurrentSessions": 2,
        "ServiceEnabled": true
    },
    "HostInterfaces": {
        "@odata.id": "/redfish/v1/Managers/Self/HostInterfaces"
    },
    "Id": "Self",
    "Links": {
        "ManagerInChassis": {
            "@odata.id": "/redfish/v1/Chassis/Self"
        }
    },
    "LogServices": {
        "@odata.id": "/redfish/v1/Managers/Self/LogServices"
    },
    "ManagerType": "BMC",
    "Model": "462249622",
    "Name": "Manager",
    "NetworkProtocol": {
        "@odata.id": "/redfish/v1/Managers/Self/NetworkProtocol"
    },
    "Oem": {
        "Ami": {
            "@odata.type": "#AMIManager.v1_0_0.AMIManager",
            "ManagerServiceInfo": {
                "CommandShellServiceInfo": {
                    "IPMI": {
                        "MaxConcurrentSessions": 36
                    },
                    "SSH": []
                },
                "Links": {
                    "NetworkProtocol": {
                        "@odata.id": "/redfish/v1/Managers/Self/NetworkProtocol"
                    }
                }
            },
            "PowerSaveMode": "Enable",
            "VirtualMedia": {
                "CDInstances": 1,
                "RMediaStatus": "Disabled"
            }
        }
    },
    "PowerState": "On",
    "Redundancy@odata.count": 0,
    "SerialConsole": {
        "ConnectTypesSupported": [
            "IPMI"
        ],
        "MaxConcurrentSessions": 1,
        "ServiceEnabled": true
    },
    "SerialInterfaces": {
        "@odata.id": "/redfish/v1/Managers/Self/SerialInterfaces"
    },
    "ServiceEntryPointUUID": "6698714e-bfde-11d3-01ab-bce9f759a1a8",
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "UUID": "6698714e-bfde-11d3-01ab-bce9f759a1a8",
    "VirtualMedia": {
        "@odata.id": "/redfish/v1/Managers/Self/VirtualMedia"
    }
}
```

Notes on Manager (pp.200-203):
- **Manager.Reset** supports `@Redfish.OperationApplyTimeSupport` with `SupportedValues` `Immediate` and `AtMaintenanceWindowStart`, `MaintenanceWindowDurationInSeconds: 600`, and `MaintenanceWindowResource` = `/redfish/v1/Managers/Self`. ActionInfo: `/redfish/v1/Managers/Self/ResetActionInfo`.
- **AMI OEM actions on the Manager** (ActionInfo URIs are under `/redfish/v1/Managers/Self/Oem/...`):

  | Action | Target | ActionInfo |
  |---|---|---|
  | `#AMIManager.RedfishDBReset` | `/redfish/v1/Managers/Self/Actions/Oem/AMIManager.RedfishDBReset` | `/redfish/v1/Managers/Self/Oem/RedfishDBResetActionInfo` |
  | `#AMIManager.RedfishPowerSaveMode` | `/redfish/v1/Managers/Self/Actions/Oem/AMIManager.RedfishPowerSaveMode` | `/redfish/v1/Managers/Self/Oem/RedfishPowerSaveModeActionInfo` |
  | `#AMIVirtualMedia.ConfigureCDInstance` | `/redfish/v1/Managers/Self/Actions/Oem/AMIVirtualMedia.ConfigureCDInstance` | `/redfish/v1/Managers/Self/Oem/ConfigureCDInstanceActionInfo` |
  | `#AMIVirtualMedia.EnableRMedia` | `/redfish/v1/Managers/Self/Actions/Oem/AMIVirtualMedia.EnableRMedia` | `/redfish/v1/Managers/Self/Oem/EnableRMediaActionInfo` |

  The parameters for these actions are not shown in this range. Get them from the ActionInfo resources at runtime.
- **`Oem.Ami` (`#AMIManager.v1_0_0.AMIManager`)**: `ManagerServiceInfo.CommandShellServiceInfo` (`IPMI.MaxConcurrentSessions`, `SSH` array), `ManagerServiceInfo.Links.NetworkProtocol`, `PowerSaveMode` (`"Enable"`), and `VirtualMedia` with `CDInstances` (number) and `RMediaStatus` (`"Disabled"`). **Remote media (RMedia) is disabled by default.** It probably has to be enabled with `AMIVirtualMedia.EnableRMedia` before Redfish virtual media can be used (inference; see 3.65-3.66 below).
- `Certificates` link: `/redfish/v1/Managers/Self/Certificates`.
- `MaxConcurrentSessions` values: CommandShell 36, GraphicalConsole (KVM) **2**, SerialConsole (SOL) **1**.
- `Model` is a numeric string (`"462249622"`), not a product name. `FirmwareVersion` was `"5.06.0"` in the example.
- `ServiceEntryPointUUID` equals `UUID`.
- `DateTimeLocalOffset` was `"-00:00"`.
- Schema `#Manager.v1_13_0.Manager`.

### Example: PATCH Manager Instance (p.203)

```
PATCH https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}
Content-Type: application/json
```

```json
{
    "DateTime": "2023-11-01T10:00:00-00:00"
}
```

Response: **204, no body.**

### Example: POST Manager Instance, Manager.Reset (p.203)

```
POST https://{BMC_IP}/redfish/v1/Managers/Actions/Manager.Reset
Content-Type: application/json
```

(As printed, the URL leaves out `/Self`. The `target` in the GET is `/redfish/v1/Managers/Self/Actions/Manager.Reset`; use the target.)

```json
{
    "ResetType": "ForceRestart"
}
```

Response: **202, with a Task as the body**:

```json
{
    "@odata.context": "/redfish/v1/$metadata#Task.Task",
    "@odata.id": "/redfish/v1/TaskService/Tasks/2",
    "@odata.type": "#Task.v1_4_2.Task",
    "Description": "Task for Manager Reset",
    "Id": "2",
    "Name": "Manager Reset",
    "TaskState": "New"
}
```

Caveat: BMC reset is asynchronous and returns a Task (202). The BMC will go offline. A proxy should expect connection loss shortly after the 202 and should not depend on the task being reachable afterwards. Allowed ResetType values are in `/redfish/v1/Managers/Self/ResetActionInfo`; only `ForceRestart` is shown here. (p.203)

## 3.56 EthernetInterfaceCollection (p.204)

- **Managers:** collection of the NIC resources in the manager.
- **Systems:** collection of host-side NIC resources. **This needs host agent support from the OS and an in-band communication channel.**

- **URLs:**
  - `/redfish/v1/Managers/{Manager-Instance}/EthernetInterfaces`
  - `/redfish/v1/Systems/{Systems-Instance}/EthernetInterfaces`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET EthernetInterfaceCollection Instance (pp.204-205)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/EthernetInterfaces
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#EthernetInterfaceCollection.EthernetInterfaceCollection",
    "@odata.etag": "\"1701164885\"",
    "@odata.id": "/redfish/v1/Managers/Self/EthernetInterfaces",
    "@odata.type": "#EthernetInterfaceCollection.EthernetInterfaceCollection",
    "Description": "Collection of Ethernet Interfaces for this Manager",
    "Members": [
        { "@odata.id": "/redfish/v1/Managers/Self/EthernetInterfaces/bond0" },
        { "@odata.id": "/redfish/v1/Managers/Self/EthernetInterfaces/eth0" },
        { "@odata.id": "/redfish/v1/Managers/Self/EthernetInterfaces/eth1" },
        { "@odata.id": "/redfish/v1/Managers/Self/EthernetInterfaces/usb0" }
    ],
    "Members@odata.count": 4,
    "Name": "Ethernet Network Interface Collection"
}
```

Note: the BMC interface IDs are Linux interface names: `bond0`, `eth0`, `eth1`, and `usb0` (the host interface, i.e. the Redfish Host Interface over USB). (p.204)

## 3.57 Ethernet Interface (p.206)

The description is the same as 3.56 (Managers: manager NICs; Systems: host-side NICs, which need a host agent).

- **URLs:**
  - `/redfish/v1/Managers/{Manager-Instance}/EthernetInterfaces/{EthernetInterfaces-Instance}`
  - `/redfish/v1/Systems/{Systems-Instance}/EthernetInterfaces/{EthernetInterfaces-Instance}`
- **Methods/privileges:** **GET = Login only** in this table. No PATCH is listed here, even though several properties are marked writable. PATCH may be documented elsewhere (for example in part 3 or under NetworkProtocol); not confirmed.

### Table 74. Supported Properties for Ethernet Interface (pp.206-207)

| Property | Type | RO | Description |
|---|---|---|---|
| UefiDevicePath | String | True | UEFI device path of this interface (port). |
| InterfaceEnabled | Boolean | False | Whether this interface is enabled. |
| PermanentMACAddress | String | True | Permanent MAC address of this interface (port). |
| MACAddress | String | True | Effective current MAC address of this interface. |
| IPv4Addresses | Array of Objects | True | IPv4 connection characteristics of this interface. |
| IPv4Addresses[].Address | String | True | IPv4 address assigned to this interface. |
| IPv4Addresses[].SubnetMask | String | True | IPv4 subnet mask of this address. |
| IPv4Addresses[].AddressOrigin | String | True | IP address origin for this interface. |
| IPv4Addresses[].Gateway | String | True | IPv4 default gateway of this interface. |
| IPv6Addresses | Array of Objects | True | IPv6 connection characteristics of this interface. |
| IPv6Addresses[].Address | String | True | IPv6 address currently assigned on this interface. |
| IPv6Addresses[].PrefixLength | String (as printed; Number in the example) | True | IPv6 address prefix length. |
| IPv6Addresses[].AddressOrigin | String | True | IPv6 address origin. |
| IPv6Addresses[].AddressState | String | True | Current state of this address as defined in RFC 4862. |
| IPv6DefaultGateway | String | True | IPv6 default gateway currently in use on this interface. |
| VLANs | Object | True | Link to a collection of VLANs. Used only if the interface supports more than one VLAN. |
| LinkStatus | String | True | Link status of this interface (port). Enum: `LinkUp` (link available for communication), `NoLink` (no link or connection detected), `LinkDown` (no link, but the interface is connected). |
| Links | Object | True | Links to related resources. |
| Links.Oem | Object | True | OEM extensions. |
| Links.Chassis | Array | True | Reference to the Chassis that physically contains this interface. |
| Links.NetworkDeviceFunction | Object | True | Link to the NetworkDeviceFunction for this interface. |
| Links.Endpoints | Array | True | References to the Endpoint resources this interface is associated with. |
| Links.Endpoints@odata.count | Number | True | Number of items. |
| EthernetInterfaceType | Boolean (as printed; String in DMTF) | True | Interface type. |
| Actions | Object | True | Actions under Oem, if any. |
| DHCPv4 | Object | - | DHCPv4 configuration. |
| DHCPv4.DHCPEnabled | Boolean | False | Whether DHCPv4 is enabled on this interface. |
| DHCPv4.UseDNSServers | Boolean | False | Whether to use DNS servers supplied by DHCPv4. |
| DHCPv4.FallbackAddress | String | False | DHCPv4 fallback method. Enum: `Static` (fall back to a static address in IPv4StaticAddresses), `AutoConfig` (fall back to an autoconfigured address), `None` (keep trying DHCP with no fallback). |
| DHCPv4.UseGateway | Boolean | False | Whether to use a gateway supplied by DHCPv4. |
| DHCPv4.UseDomainName | Boolean | False | Whether to use a domain name supplied by DHCPv4. |
| DHCPv4.UseNTPServers | Boolean | False | Whether to use NTP servers supplied by DHCPv4. |
| DHCPv4.UseStaticRoutes | Boolean | False | Whether to use static routes supplied by DHCPv4. |
| DHCPv6 | Object | - | DHCPv6 configuration. |
| DHCPv6.UseDNSServer (sic) | Boolean | False | When enabled, DNS server addresses supplied through DHCPv6 stateless mode are used. |
| DHCPv6.OperatingMode | String | False | DHCPv6 operating mode for this interface. |
| DHCPv6.UseDomainName | Boolean | False | When enabled, the domain name supplied through DHCPv6 stateless mode is used. |
| DHCPv6.UseNTPServers | Boolean | False | When enabled, NTP server addresses supplied through DHCPv6 stateless mode are used. |
| DHCPv6.UseRapidCommit | Boolean | False | Whether to use DHCPv6 rapid commit in stateful mode address assignment. |

### Example: GET Ethernet Interface Instance (pp.208-209)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/EthernetInterfaces/{EthernetInterfaces-Instance}
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#EthernetInterface.EthernetInterface",
    "@odata.etag": "\"1701166237\"",
    "@odata.id": "/redfish/v1/Managers/Self/EthernetInterfaces/bond0",
    "@odata.type": "#EthernetInterface.v1_6_2.EthernetInterface",
    "AutoNeg": true,
    "DHCPv4": {
        "DHCPEnabled": true
    },
    "DHCPv6": {
        "OperatingMode": "Stateless"
    },
    "Description": "Ethernet Interface bond0",
    "FullDuplex": true,
    "HostName": "AMIA8A159F7E9BC",
    "IPv4Addresses": [
        {
            "Address": "192.168.36.56",
            "AddressOrigin": "DHCP",
            "Gateway": "192.168.36.1",
            "SubnetMask": "255.255.252.0"
        }
    ],
    "IPv6Addresses": [
        {
            "Address": "fe80::aaa1:59ff:fef7:e9bc",
            "AddressOrigin": "LinkLocal",
            "PrefixLength": 64
        }
    ],
    "IPv6DefaultGateway": null,
    "Id": "bond0",
    "InterfaceEnabled": true,
    "LinkStatus": "LinkUp",
    "MACAddress": "A8:A1:59:F7:E9:BC",
    "MTUSize": 1500,
    "MaxIPv6StaticAddresses": 16,
    "Name": "bond0",
    "NameServers": [
        "8.8.8.8",
        "168.95.1.1"
    ],
    "PermanentMACAddress": "A8:A1:59:F7:E9:BC",
    "SpeedMbps": 1000,
    "StatelessAddressAutoConfig": {
        "IPv4AutoConfigEnabled": false,
        "IPv6AutoConfigEnabled": false
    },
    "StaticNameServers": [
        null,
        null,
        null
    ],
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "VLAN": {
        "VLANEnable": false,
        "VLANId": 0
    }
}
```

Notes (pp.208-209):
- The default BMC hostname has the form `AMI<MAC without colons>`.
- `StaticNameServers` is padded with `null` entries (3 slots).
- `VLAN` is a single object (`VLANEnable`, `VLANId`), not a VLANs collection.
- The example includes properties that are not in Table 74: `AutoNeg`, `FullDuplex`, `HostName`, `MTUSize`, `MaxIPv6StaticAddresses`, `NameServers`, `SpeedMbps`, `StatelessAddressAutoConfig`, `StaticNameServers`, `VLAN`.
- The `bond0` example reports a MAC address and IPs, so on bonded setups the bond interface carries the IP.

## 3.58 Manager EthernetInterface (usb0) (p.210)

The manager's USB host-interface Ethernet.

- **URL:** `/redfish/v1/Managers/{Manager-Instance}/EthernetInterfaces/usb0`
- **Methods/privileges:** GET = Login

### Table 75. Supported Properties for Manager EthernetInterface (p.210)

| Property | Type | RO | Description |
|---|---|---|---|
| MACAddress | String | True | Effective current MAC address of this interface. |
| IPv4Addresses | Array of Objects | True | IPv4 address assigned to an interface. |
| IPv4Addresses[].Address | String | False | IPv4 address assigned to this interface. If DHCPv4 is enabled on the interface, this property becomes read-only. |
| IPv4Addresses[].SubnetMask | Object (as printed) | False | IPv4 subnet mask of this address. If DHCPv4 is enabled on the interface, this property becomes read-only. |
| InterfaceEnabled | Boolean | False | Whether this interface is enabled. |

### Example: GET Manager EthernetInterface Instance (pp.210-211)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/EthernetInterfaces/usb0
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#EthernetInterface.EthernetInterface",
    "@odata.etag": "\"1701328249\"",
    "@odata.id": "/redfish/v1/Managers/Self/EthernetInterfaces/usb0",
    "@odata.type": "#EthernetInterface.v1_6_2.EthernetInterface",
    "IPv4Addresses": [
        {
            "Address": "169.254.0.17",
            "SubnetMask": "255.255.0.0"
        }
    ],
    "Id": "usb0",
    "InterfaceEnabled": true,
    "MACAddress": "4A:B8:B3:8B:93:F0",
    "Name": "usb0",
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    }
}
```

Note: the usb0 host interface uses the link-local address 169.254.0.17/16. (pp.210-211)

## 3.59 HostInterface Collection (p.212)

Collection of the Host Interfaces available in Managers.

- **URL:** `/redfish/v1/Managers/{Manager-Instance}/HostInterfaces`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET HostEthernetInterface Collection Instance (p.212)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/HostInterfaces
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#HostInterfaceCollection.HostInterfaceCollection",
    "@odata.etag": "\"1701328249\"",
    "@odata.id": "/redfish/v1/Managers/Self/HostInterfaces",
    "@odata.type": "#HostInterfaceCollection.HostInterfaceCollection",
    "Description": "Collection of HostInterfaces",
    "Members": [
        { "@odata.id": "/redfish/v1/Managers/Self/HostInterfaces/Self" }
    ],
    "Members@odata.count": 1,
    "Name": "HostInterface Collection"
}
```

## 3.60 HostInterface (p.213)

**Under the HostInterface specification, `HostAutoFW` and `HostAutoOS` are the only two accounts allowed on this interface.**

- **URL:** `/redfish/v1/Managers/{Manager-Instance}/HostInterfaces/{HostInterfaces-Instance}`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureComponents |

### Table 76. Supported Properties for HostInterface (pp.213-214)

| Property | Type | RO | Description |
|---|---|---|---|
| HostInterfaceType | String | True | Type of the interface. |
| InterfaceEnabled | Boolean | False | Whether this interface is enabled. |
| ExternallyAccessible | Boolean | True | Whether external entities can reach this interface. |
| AuthenticationModes | Array | True | Authentication modes available on this interface. Enum: `AuthNone` (requests without any authentication are allowed), `BasicAuth` (HTTP Basic Authentication), `RedfishSessionAuth` (Redfish Session Authentication), `OemAuth` (OEM authentication mechanisms). |
| KernelAuthRoleId | String | False | Role for kernel authentication on this interface. |
| KernelAuthEnabled | Boolean | False | Whether kernel authentication is enabled on this interface. |
| FirmwareAuthRoleId | String | False | Role for firmware authentication on this interface. |
| FirmwareAuthEnabled | Boolean | False | Whether firmware authentication is enabled on this interface. |
| Links | Object | - | References to related resources. |
| Links.ComputerSystems | Array | True | Computer Systems connected to this Host Interface. |
| Links.ComputerSystems@odata.count | Number | True | Number of items. |
| Links.CredentialBootstrappingRole | Object | True | Link to the role that holds the privileges of the bootstrap account created for this interface. |
| Links.KernelAuthRole | Object | True | Role object that defines the privileges for this Host Interface under kernel authentication. |
| Links.FirmwareAuthRole | Object | True | Role object that defines the privileges for this Host Interface under firmware authentication. |
| HostEthernetInterfaces | Object | True | Link to the collection of NICs that Computer Systems use for network communication with this Host Interface. |
| ManagerEthernetInterface | Object | True | Link to the single NIC this Manager uses to communicate with this Host Interface. |
| NetworkProtocol | Object | True | Link to the network services and settings the Manager controls. |
| AuthNoneRoleId | String | False | Role used when the interface is used without authentication. |
| CredentialBootstrapping | Object | False | Credential bootstrapping settings for this interface. |
| CredentialBootstrapping.Enabled | Boolean | False | Whether credential bootstrapping is enabled on this interface. |
| CredentialBootstrapping.EnableAfterReset | Boolean | False | Whether credential bootstrapping is enabled after a reset on this interface. |
| CredentialBootstrapping.RoleId | String | False | Role used for the bootstrap account created for this interface. |

### Example: GET HostInterface Instance (pp.214-215)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/HostInterfaces/{HostInterfaces-Instance}
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#HostInterface.HostInterface",
    "@odata.etag": "\"1701328249\"",
    "@odata.id": "/redfish/v1/Managers/Self/HostInterfaces/Self",
    "@odata.type": "#HostInterface.v1_3_0.HostInterface",
    "AuthNoneRoleId": "HostInterfaceAdministrator",
    "AuthenticationModes": [
        "AuthNone"
    ],
    "CredentialBootstrapping": {
        "EnableAfterReset": false,
        "Enabled": true,
        "RoleId": "HostInterfaceAdministrator"
    },
    "Description": "Host Interface",
    "ExternallyAccessible": false,
    "HostEthernetInterfaces": {
        "@odata.id": "/redfish/v1/Managers/Self/HostInterfaces/Self/HostEthernetInterfaces"
    },
    "HostInterfaceType": "NetworkHostInterface",
    "Id": "Self",
    "InterfaceEnabled": true,
    "Links": {
        "ComputerSystems": [
            { "@odata.id": "/redfish/v1/Systems/Self" }
        ],
        "ComputerSystems@odata.count": 1,
        "CredentialBootstrappingRole": {
            "@odata.id": "/redfish/v1/AccountService/Roles/HostInterfaceAdministrator"
        }
    },
    "ManagerEthernetInterface": {
        "@odata.id": "/redfish/v1/Managers/Self/EthernetInterfaces/usb0"
    },
    "Name": "Host Interface",
    "NetworkProtocol": {
        "@odata.id": "/redfish/v1/Managers/Self/NetworkProtocol"
    },
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    }
}
```

**Security note (pp.214-215):** with the default configuration the host interface accepts `AuthenticationModes: ["AuthNone"]`, with `AuthNoneRoleId: "HostInterfaceAdministrator"`, and `CredentialBootstrapping.Enabled: true`. Any software on the host OS can therefore reach the Redfish API over usb0 (169.254.0.17) without credentials and with administrator-level rights. The custom role `HostInterfaceAdministrator` lives at `/redfish/v1/AccountService/Roles/HostInterfaceAdministrator`. Consider disabling it (PATCH `CredentialBootstrapping.Enabled: false` or `InterfaceEnabled: false`).

### Example: PATCH HostInterface Instance (p.215)

```
PATCH https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/HostInterfaces/{HostInterfaces-Instance}
Content-Type: application/json
```

```json
{
    "CredentialBootstrapping": {
        "Enabled": false
    }
}
```

Response: **204, no body.**

## 3.61 HostEthernetInterface Collection (p.216)

Collection of host-side Ethernet interfaces.

- **URL:** `/redfish/v1/Managers/{Manager-Instance}/HostInterfaces/{HostInterfaces-Instance}/HostEthernetInterfaces`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET HostEthernetInterface Collection Instance (p.216)

```json
{
    "@odata.context": "/redfish/v1/$metadata#EthernetInterfaceCollection.EthernetInterfaceCollection",
    "@odata.etag": "\"1701354446\"",
    "@odata.id": "/redfish/v1/Managers/Self/HostInterfaces/Self/HostEthernetInterfaces",
    "@odata.type": "#EthernetInterfaceCollection.EthernetInterfaceCollection",
    "Description": "Collection of EthernetInterfaces hosts connected to this HostInterface on the Manager",
    "Members": [],
    "Members@odata.count": 0,
    "Name": "Ethernet Network Interface Collection"
}
```

The collection is empty in the example.

## 3.62 ManagerNetworkProtocol (p.217)

Reads or changes the network services managed by a manager.

- **URL:** `/redfish/v1/Managers/{Manager-Instance}/NetworkProtocol`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureManager |

### Table 77. Supported Properties for ManagerNetworkProtocol (p.217)

| Property | Type | RO | Description |
|---|---|---|---|
| HostName | String | True | DNS host name of this manager, without domain information. |
| FQDN | String | True | Fully qualified domain name of the manager obtained by DNS, including host name and top-level domain. |
| HTTPS | Object | False | HTTPS/SSL protocol settings (Table 79). |
| SNMP | Object | **True** | SNMP protocol settings (Table 80). Read-only. |
| VirtualMedia | Object | False | Virtual Media protocol settings (Table 78). |
| Telnet | Object | False | Telnet protocol settings (Table 78). |
| SSDP | Object | False | SSDP protocol settings (Table 81). |
| IPMI | Object | False | IPMI over LAN protocol settings. |
| SSH | Object | False | SSH protocol settings. |
| KVMIP | Object | False | KVM-IP protocol settings (Table 78). |
| NTP | Object | False | NTP protocol settings. |
| NTP.ProtocolEnabled | Boolean | False | Whether the protocol is enabled. |
| NTP.Port | Number | False | Protocol port. |
| NTP.NTPServers | Array | False | NTP servers this manager uses. |
| Actions | Object | True | Actions under Oem, if any. |

### Table 78. Protocol Properties (p.218)

| Property | Type | RO | Description |
|---|---|---|---|
| ProtocolEnabled | Boolean | False | Whether the protocol is enabled. |
| Port | Number | False | Port assigned to the protocol. |

### Table 79. HTTPS Protocol Properties (p.218)

| Property | Type | RO | Description |
|---|---|---|---|
| ProtocolEnabled | Boolean | False | Whether the protocol is enabled. |
| Port | Number | False | Port assigned to the protocol. |
| Certificates | Object | True | Link to the collection of certificates used for HTTPS. |
| Certificates.@odata.id | String | True | Link to a CertificateCollection. |

### Table 80. SNMP Protocol Properties (p.218)

| Property | Type | RO | Description |
|---|---|---|---|
| ProtocolEnabled | Boolean | False | Whether the protocol is enabled. |
| Port | Number | False | Port assigned to the protocol. |
| AuthenticationProtocol | String | True | Authentication protocol for SNMP access to the manager. Enum: `Account` (set by account settings), `CommunityString`, `HMAC_MD5`, `HMAC_SHA96`. |
| EncryptionProtocol | String | True | Encryption protocol for SNMP access to the manager. Enum: `None`, `Account` (set by account settings), `CBC_DES`, `CFB128_AES128`. |
| EngineId | Object | True | RFC 3411 engine ID. |
| EngineId.ArchitectureId | String | True | Item 3 of the RFC 3411 snmpEngineID syntax. Note: not present if the most significant bit of PrivateEnterpriseId is zero. |
| EngineId.EnterpriseSpecificMethod | String | True | Item 2 of the RFC 3411 snmpEngineID syntax. Note: not present if the most significant bit of PrivateEnterpriseId is one. |
| EngineId.PrivateEnterpriseId | String | True | RFC 3411 private enterprise ID. |
| EnableSNMPv1 | Boolean | True | Whether SNMPv1 access is enabled. |
| EnableSNMPv2c | Boolean | True | Whether SNMPv2c access is enabled. |
| EnableSNMPv3 | Boolean | True | Whether SNMPv3 access is enabled. |

### Table 81. SSDP Protocol Properties (p.218)

| Property | Type | RO | Description |
|---|---|---|---|
| ProtocolEnabled | Boolean | False | Whether the protocol is enabled. |
| Port | Number | False | Port assigned to the protocol. |

### Example: GET ManagerNetworkProtocol Instance (pp.218-220)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/NetworkProtocol
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#ManagerNetworkProtocol.ManagerNetworkProtocol",
    "@odata.etag": "\"1701501062\"",
    "@odata.id": "/redfish/v1/Managers/Self/NetworkProtocol",
    "@odata.type": "#ManagerNetworkProtocol.v1_6_0.ManagerNetworkProtocol",
    "Description": "Network Protocol Details",
    "HTTPS": {
        "Certificates": {
            "@odata.id": "/redfish/v1/Managers/Self/NetworkProtocol/HTTPS/Certificates"
        },
        "Port": 443,
        "ProtocolEnabled": true
    },
    "HostName": "AMIA8A159F7E9BC",
    "IPMI": {
        "Port": 623,
        "ProtocolEnabled": true
    },
    "Id": "NetworkProtocol",
    "KVMIP": {
        "Port": 443,
        "ProtocolEnabled": true
    },
    "NTP": {
        "NTPServers": [
            "pool.ntp.org",
            "time.nist.gov"
        ],
        "Port": 123,
        "ProtocolEnabled": true
    },
    "Name": "Manager Network Protocol",
    "SNMP": {
        "AuthenticationProtocol": "Account",
        "EnableSNMPv1": true,
        "EnableSNMPv2c": true,
        "EnableSNMPv3": true,
        "EncryptionProtocol": "Account",
        "EngineId": {
            "ArchitectureId": "80 f0 07 57 6d 41 97 60 65",
            "PrivateEnterpriseId": "80 00 1f 88"
        },
        "Port": 161,
        "ProtocolEnabled": true
    },
    "SSH": {
        "Port": 22,
        "ProtocolEnabled": true
    },
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "VirtualMedia": {
        "Port": 443,
        "ProtocolEnabled": true
    }
}
```

Notes (pp.219-220):
- HTTPS certificates live at `/redfish/v1/Managers/Self/NetworkProtocol/HTTPS/Certificates`.
- KVMIP and VirtualMedia **share port 443** with HTTPS. KVM and virtual media are tunnelled over HTTPS/WebSocket, which matters for a proxy that must pass them through.
- IPMI 623, SSH 22, NTP 123, SNMP 161 (v1, v2c, and v3 all enabled by default).
- Telnet and SSDP do not appear in the example output.

### Example: PATCH ManagerNetworkProtocol Instance (p.220)

```
PATCH https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/NetworkProtocol
Content-Type: application/json
```

(Printed as "PATHC".)

```json
{
    "KVMIP": {
        "Port": 443,
        "ProtocolEnabled": false
    }
}
```

Response: **202, with a Task as the body** (asynchronous PATCH):

```json
{
    "@odata.context": "/redfish/v1/$metadata#Task.Task",
    "@odata.id": "/redfish/v1/TaskService/Tasks/1",
    "@odata.type": "#Task.v1_4_2.Task",
    "Description": "Task for NetworkProtocol Patch Task",
    "Id": "1",
    "Name": "NetworkProtocol Patch Task",
    "TaskState": "New"
}
```

Caveat: PATCH on NetworkProtocol is **asynchronous (202 plus a Task)**, unlike most PATCHes here, which return 204. Services probably restart. A client must poll `/redfish/v1/TaskService/Tasks/{Id}`. (p.220)

## 3.63 SerialInterfaceCollection (p.221)

Collection of asynchronous serial interface resources.

- **URL:** `/redfish/v1/Managers/{Manager-Instance}/SerialInterfaces`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET SerialInterfaceCollection Instance (pp.221-222)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/SerialInterfaces
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#SerialInterfaceCollection.SerialInterfaceCollection",
    "@odata.etag": "\"1701505569\"",
    "@odata.id": "/redfish/v1/Managers/Self/SerialInterfaces",
    "@odata.type": "#SerialInterfaceCollection.SerialInterfaceCollection",
    "Description": "Collection of Serial Interfaces for this System",
    "Members": [
        { "@odata.id": "/redfish/v1/Managers/Self/SerialInterfaces/IPMI-SOL" },
        { "@odata.id": "/redfish/v1/Managers/Self/SerialInterfaces/ttyS0" },
        { "@odata.id": "/redfish/v1/Managers/Self/SerialInterfaces/ttyS2" },
        { "@odata.id": "/redfish/v1/Managers/Self/SerialInterfaces/ttyS3" },
        { "@odata.id": "/redfish/v1/Managers/Self/SerialInterfaces/ttyS4" }
    ],
    "Members@odata.count": 5,
    "Name": "Serial Interface Collection"
}
```

## 3.64 SerialInterface (p.223)

Asynchronous serial interface resource. Per the text, **the supported instances are IPMI-SOL, ttyS0, ttyS1, ttyS3 and ttyS4.** The collection example lists ttyS2, not ttyS1; trust the collection at runtime.

- **URL:** `/redfish/v1/Managers/{Manager-Instance}/SerialInterfaces/{SerialInterfaces-Instance}`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureManager |

### Table 82. Supported Properties for SerialInterface (pp.223-224)

| Property | Type | RO | Description / enum |
|---|---|---|---|
| InterfaceEnabled | Boolean | False | Whether this interface is enabled. |
| BitRate | String | **False** | Transmit and receive speed. Enum (strings): `9600`, `19200`, `38400`, `57600`, `115200`. |
| Parity | String | True | Parity. Enum: `None`, `Even`, `Odd`, `Mark`, `Space`. |
| SignalType | String | True | Signal type. Enum: `RS232`, `RS485`. |
| ConnectorType | String | True | Connector type. Enum: `RJ11`, `RJ45`, `DB9 Female`, `DB9 Male`, `DB25 Female`, `DB25 Male`, `USB`, `mUSB`, `uUSB`. |
| PinOut | String | True | Physical pin configuration. Enum: `Cisco`, `Cyclades`, `Digi`. |
| DataBits | String | True | Data bits after the start bit. Enum: `5`, `6`, `7`, `8`. |
| StopBits | String | True | Enum: `1`, `2`. |
| FlowControl | String | True | Enum: `None`, `Software` (XON/XOFF in-band), `Hardware` (out-of-band). |

Only `InterfaceEnabled` and `BitRate` are writable.

### Example: GET SerialInterface Instance (p.224)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/SerialInterfaces/{SerialInterfaces-Instance}
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#SerialInterface.SerialInterface",
    "@odata.etag": "\"1701505569\"",
    "@odata.id": "/redfish/v1/Managers/Self/SerialInterfaces/IPMI-SOL",
    "@odata.type": "#SerialInterface.v1_1_5.SerialInterface",
    "BitRate": "115200",
    "DataBits": "8",
    "Description": "IPMI Serial over LAN",
    "FlowControl": "Hardware",
    "Id": "IPMI-SOL",
    "InterfaceEnabled": true,
    "Name": "IPMI-SOL",
    "Parity": "None",
    "StopBits": "1"
}
```

### Example: PATCH SerialInterface Instance (pp.224-225)

```
PATCH https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/SerialInterfaces/{SerialInterfaces-Instance}
```

```json
{
    "BitRate": "57600"
}
```

Response: **204, no body.** BitRate is a string, not a number.

## 3.65 VirtualMediaCollection (p.226)

**Virtual Media members are shown only when VirtualMedia is enabled in the BMC.** If RMedia is disabled (`Oem.Ami.VirtualMedia.RMediaStatus: "Disabled"` on the Manager), expect an empty collection. Enable it with `#AMIVirtualMedia.EnableRMedia` (Manager OEM action, see 3.55). The number of CD instances is set with `#AMIVirtualMedia.ConfigureCDInstance` (`Oem.Ami.VirtualMedia.CDInstances`).

- **URL:** `/redfish/v1/Managers/{Manager-Instance}/VirtualMedia`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET VirtualMediaCollection Instance (p.226)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/VirtualMedia
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#VirtualMediaCollection.VirtualMediaCollection",
    "@odata.etag": "\"1701506463\"",
    "@odata.id": "/redfish/v1/Managers/Self/VirtualMedia",
    "@odata.type": "#VirtualMediaCollection.VirtualMediaCollection",
    "Description": "Collection of Virtual Media redirected to host via this Manager",
    "Members": [
        { "@odata.id": "/redfish/v1/Managers/Self/VirtualMedia/CD1" },
        { "@odata.id": "/redfish/v1/Managers/Self/VirtualMedia/CD2" },
        { "@odata.id": "/redfish/v1/Managers/Self/VirtualMedia/CD3" },
        { "@odata.id": "/redfish/v1/Managers/Self/VirtualMedia/CD4" }
    ],
    "Members@odata.count": 4,
    "Name": "Virtual Media Collection"
}
```

Instance IDs are `CD1`..`CD4` (CD only; no floppy or USB stick instances appear).

## 3.66 VirtualMedia (p.227)

Virtual Media service resource.

- **URL:** `/redfish/v1/Managers/{Manager-Instance}/VirtualMedia/{VirtualMedia-Instance}`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureManager |
| POST | ConfigureManager (InsertMedia / EjectMedia) |

### Table 83. Supported Properties for VirtualMedia (pp.227-228)

| Property | Type | RO | Description |
|---|---|---|---|
| Oem | Object | - | OEM-defined properties (`Oem.Ami`). |
| Oem.Ami.@odata.type | String | True | (`#AMIVirtualMedia.v1_0_0.AMIVirtualMedia`) |
| Oem.Ami.RedirectionStatus | String | True | Redirection status of the image. |
| Oem.Ami.BoostMode | Boolean | **False** | Boost mode support status. |
| Oem.Ami.TransferProtocolType | String | True | Shows that CD redirection was started with the OEM CIFS encryption method. The value is `SMBE` (CIFS encryption). |
| ImageName | String | True | Name of the current image. |
| Image | String | True | URI of the selected image's location. |
| ConnectedVia | String | True | Current connection method from a client to the virtual media. Enum: `NotConnected`, `URI` (connected to a URI location), `Applet` (connected to a client application), `Oem` (OEM-defined method). |
| Inserted | Boolean | True | Whether virtual media is inserted in the virtual device. Usually applies only to remoted devices, not image virtual media. |
| WriteProtected | Boolean | True | Whether the media is write-protected. |
| MediaType (MediaTypes in the example) | Array | True | Supported media types for this connection. Value: `CD` (CD-ROM / ISO image). |
| TransferProtocolType | String | True | Network protocol for the image. Enum: `NFS`, `CIFS`, `HTTPS`, `OEM` (manufacturer-defined: CIFS Encryption, SMBE). |
| TransferMethod | String | True | Transfer method for the given image. Value: `Stream` (stream image data from the source URI). |
| UserName | String | True | User name for accessing the URI in Image. |
| Actions | Object | True | Actions, including any under Oem. |
| Actions InsertMedia | - | - | Attaches remote media to virtual media. Parameters below. |
| InsertMedia.Image | String | - | URI of the remote media to attach. |
| InsertMedia.Inserted | Boolean | - | Whether the image is treated as inserted when the action completes. |
| InsertMedia.TransferMethod | String | - | Transfer method. Allowed value: `Stream`. |
| InsertMedia.TransferProtocolType | String | - | Network protocol. Allowed values: `NFS`, `CIFS`, `HTTPS`, `OEM`. |
| InsertMedia.WriteProtected | String (as printed; Boolean in DMTF) | - | Whether the remote media is treated as write-protected. |
| InsertMedia.UserName | String | - | User name for the Image URI. **Required for CIFS, OEM and HTTPS.** |
| InsertMedia.Password | String | - | Password for the Image URI. **Required for CIFS, OEM and HTTPS.** |
| Actions EjectMedia | - | - | Detaches remote media from virtual media. |

Caveats (pp.227-228):
- **Plain HTTP is not supported.** Only NFS, CIFS, HTTPS, and OEM (SMBE) are listed, and HTTPS needs a UserName and Password.
- The only TransferMethod is `Stream`; `Upload` is not supported.
- Only `MediaTypes: ["CD"]` is supported (ISO).

### Example: GET VirtualMedia Instance (pp.228-229)

```
GET https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/VirtualMedia/{VirtualMedia-Instance}
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#VirtualMedia.VirtualMedia",
    "@odata.etag": "\"1701506463\"",
    "@odata.id": "/redfish/v1/Managers/Self/VirtualMedia/CD1",
    "@odata.type": "#VirtualMedia.v1_3_2.VirtualMedia",
    "Actions": {
        "#VirtualMedia.EjectMedia": {
            "@Redfish.ActionInfo": "/redfish/v1/Managers/Self/VirtualMedia/CD1/EjectMediaActionInfo",
            "target": "/redfish/v1/Managers/Self/VirtualMedia/CD1/Actions/VirtualMedia.EjectMedia"
        },
        "#VirtualMedia.InsertMedia": {
            "@Redfish.ActionInfo": "/redfish/v1/Managers/Self/VirtualMedia/CD1/InsertMediaActionInfo",
            "target": "/redfish/v1/Managers/Self/VirtualMedia/CD1/Actions/VirtualMedia.InsertMedia"
        }
    },
    "ConnectedVia": "URI",
    "Description": "Virtual Media Instance redirected to host via this Manager",
    "Id": "CD1",
    "Image": "//192.168.36.224/OS/ubuntu-22.04.3-desktop-amd64.iso",
    "ImageName": "ubuntu-22.04.3-desktop-amd64.iso",
    "Inserted": true,
    "MediaTypes": [
        "CD"
    ],
    "Name": "CD1",
    "Oem": {
        "Ami": {
            "@odata.type": "#AMIVirtualMedia.v1_0_0.AMIVirtualMedia",
            "BoostMode": true,
            "RedirectionStatus": "Redirection Started With Media Boost"
        }
    },
    "TransferMethod": "Stream",
    "TransferProtocolType": "CIFS",
    "UserName": "user_123",
    "WriteProtected": true
}
```

Notes: the CIFS `Image` format is `//<host>/<share>/<path>.iso`, with no `smb://` or `cifs://` scheme. `RedirectionStatus` is free text (`"Redirection Started With Media Boost"`). (p.229)

### Example: PATCH VirtualMedia Instance (p.229)

The request line is printed as `GET`, which is a documentation error; the method is PATCH.

```
PATCH https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/VirtualMedia/{VirtualMedia-Instance}
Content-Type: application/json
```

```json
{
    "Oem": {
        "Ami": {
            "BoostMode": true
        }
    }
}
```

Response: **204, no body.** The only writable property shown is `Oem.Ami.BoostMode`.

### Example: POST VirtualMedia InsertMedia, CIFS (pp.229-230)

```
POST https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/VirtualMedia/{VirtualMedia-Instance}/Actions/VirtualMedia.InsertMedia
Content-Type: application/json
```

```json
{
    "Image": "//192.168.36.224/OS/ubuntu-22.04.3-desktop-amd64.iso",
    "TransferProtocolType": "CIFS",
    "UserName": "user_123",
    "Password": "password123"
}
```

Response: **202, with a Task as the body**:

```json
{
    "@odata.context": "/redfish/v1/$metadata#Task.Task",
    "@odata.id": "/redfish/v1/TaskService/Tasks/3",
    "@odata.type": "#Task.v1_4_2.Task",
    "Description": "Task for InsertMedia Action",
    "Id": "3",
    "Name": "InsertMedia Action",
    "TaskState": "New"
}
```

Caveat: InsertMedia is **asynchronous (202 plus a Task)**. Poll the task to find out whether mounting worked; a 202 does not mean the image is mounted. (p.230)

### Example: POST VirtualMedia EjectMedia (p.230)

Ejects the CD media image redirection.

```
POST https://{BMC_IP}/redfish/v1/Managers/{Manager-Instance}/VirtualMedia/{VirtualMedia-Instance}/Actions/VirtualMedia.EjectMedia
Content-Type: application/json
```

```json
{
}
```

Response: **204** (synchronous; send an empty JSON body `{}`).

## 3.67 Configurations (AMI OEM) (p.231)

Supports Certificate Authentication (CA): the user uploads a public certificate to the Redfish server, which allows extra configuration of the Redfish service. In practice this resource holds the URL from which the BMC downloads a CA certificate.

- **URL:** `/redfish/v1/Oem/Ami/Configurations`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureComponents |

### Table 84. Supported Properties for Configurations (p.231)

| Property | Type | RO | Description |
|---|---|---|---|
| CertificateAuthorityUrl | String | False | URL of the web server where the CA certificate is stored. |

### Example: GET Configurations Instance (p.231)

```
GET https://{BMC_IP}/redfish/v1/Oem/Ami/Configurations
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#AMIConfigurations.AMIConfigurations",
    "@odata.etag": "\"1701515674\"",
    "@odata.id": "/redfish/v1/Oem/Ami/Configurations",
    "@odata.type": "#AMIConfigurations.v1_0_0.AMIConfigurations",
    "CertificateAuthorityUrl": "",
    "Id": "Configurations",
    "Name": "Configurations"
}
```

### Example: PATCH Configurations Instance (pp.231-232)

The request line is printed as `GET`; the method is PATCH.

```
PATCH https://{BMC_IP}/redfish/v1/Oem/Ami/Configurations
Content-Type: application/json
```

```json
{
    "CertificateAuthorityUrl": "https://<server_ip>/cert/ca-cert.pem"
}
```

**DHCP option alternative (p.232):** the URL can also be delivered with a custom DHCP option, sent when a lease is given. Add a custom option to the DHCP server named `REDFISH_CA_CERTIFICATE`, **code 248**, whose value is the URL the CA certificate can be downloaded from.

`CertificateAuthorityUrl` takes priority over the DHCP option. Once a URL has been PATCHed, the URL from the DHCP option is not used until `CertificateAuthorityUrl` is PATCHed to null:

```json
{
    "CertificateAuthorityUrl": null
}
```

Response: **200 OK. The body is the GET response with the changed values from the request body**, plus an annotation:

```json
{
    "@odata.context": "/redfish/v1/$metadata#AMIConfigurations.AMIConfigurations",
    "@odata.etag": "\"1701517211\"",
    "@odata.id": "/redfish/v1/Oem/Ami/Configurations",
    "@odata.type": "#AMIConfigurations.v1_0_0.AMIConfigurations",
    "CertificateAuthorityUrl": "",
    "CertificateAuthorityUrl@Message.ExtendedInfo": {
        "@odata.type": "#Message.v1_0_8.Message",
        "Message": "Web server is being restarted. It will be unreachable for a few seconds.",
        "MessageId": "AmiOem.1.0.WebServerRestarting",
        "Resolution": "Wait a few seconds before sending additional requests.",
        "Severity": "OK"
    },
    "Id": "Configurations",
    "Name": "Configurations"
}
```

Caveats (p.232):
- **Changing this setting restarts the BMC web server**, so it is briefly unreachable. A proxy must tolerate connection resets and retry after a few seconds.
- `@Message.ExtendedInfo` is returned as a **single object, not an array** as DMTF requires. Parse both forms.
- Message ID `AmiOem.1.0.WebServerRestarting` comes from the `AmiOem` registry.

## 3.68 InventoryData Status URI (AMI OEM) (p.233)

Shows the status of BIOS inventory file processing (the host BIOS pushes its inventory to the BMC). **This URI exists in the service root only after an inventory file has been posted.**

- **URL:** `/redfish/v1/Oem/Ami/InventoryData/Status`
- **Methods/privileges:** GET = Login

### Table 85. Supported Properties for InventoryData Status URI (p.233)

| Property | Type | RO | Description |
|---|---|---|---|
| InventoryData | Object | True | |
| InventoryData.DeletedModules | Array | True | CrcElements updated in this inventory population (Table 86). |
| InventoryData.Messages | Array | True | Messages created during inventory population. |
| InventoryData.ProcessingTime | Number | True | Time taken to process and populate the inventory file. |
| InventoryData.LastModifiedTime | String | True | Date and time the InventoryData was updated. |
| InventoryData.Status | String | True | Status of inventory file processing (Table 87). |

### Table 86. Allowed Values for DeletedModules (p.233)

| Enum | Description |
|---|---|
| CPU | All data related to the Processor, such as Processors and ProcessorsMetrics. |
| DIMM | All data related to Memory, such as Memory, MemoryDomains, MemoryChunks and MemoryMetrics. |
| PCIE | All data related to Storage, PCIeDevices, NetworkInterfaces, and NetworkAdapters. |
| CERTIFICATES | All data related to Boot Certificates. |
| SECURBOOT (sic) | All data related to SecureBoot data. |

### Table 87. Enum values for Status (p.233)

| Enum | Description |
|---|---|
| BootInProgress | The host system has started booting. |
| Ready | Inventory file processing is complete. |
| Queued | The inventory processing task is queued in SouthBound. |
| In-Progress | The inventory file is being processed in the background. |
| Failed | Inventory file processing failed. |
| Completed | Inventory file processing is complete. |

### Example: GET InventoryData Status Instance (p.234)

```
GET https://{BMC_IP}/redfish/v1/Oem/Ami/InventoryData/Status
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#AMIInventoryStatus.v1_0_0.AMIInventoryStatus",
    "@odata.etag": "\"1701413649\"",
    "@odata.id": "/redfish/v1/Oem/Ami/InventoryData/Status",
    "@odata.type": "#AMIInventoryStatus.v1_0_0.AMIInventoryStatus",
    "Description": "Detailed Post Status for Bios Inventory file",
    "Id": "InventoryDataStatus",
    "InventoryData": {
        "DeletedModules": [
            "DIMM",
            "PCIE",
            "CPU"
        ],
        "LastModifiedTime": "2023-12-01T06:54:09+00:00",
        "Messages": [
            "Inventory Backup file updated successfully.",
            "BiosAttributeRegistryA2994.en-US.5.2.0.json Processed Successfully",
            "Warning PropertyMissing (Id : Redfish:Systems)",
            "Warning PropertyMissing (Id : Redfish:Chassis)"
        ],
        "ProcessingTime": 6087.01,
        "Status": "Completed"
    },
    "Name": "PostStatus for Bios Inventory file"
}
```

Implementation note: Processor, Memory, PCIe, Storage, and NIC inventory, plus SecureBoot and boot certificates, are filled in from a file the BIOS posts during POST. Until a POST has completed (`Status` = `Completed`/`Ready`), these resources may be missing or stale. Use this endpoint to gate inventory reads. `ProcessingTime` was 6087.01 in the example (units not given; probably ms). (pp.233-234)

## 3.69 MessageRegistryFileCollection (p.235)

Lists the registries the Redfish service supports, including AttributeRegistry.

- **URL:** `/redfish/v1/Registries`
- **Methods/privileges:** GET = Login
- **Properties:** common ODATA properties only.

### Example: GET MessageRegistryFileCollection Instance (pp.235-236)

```
GET https://{BMC_IP}/redfish/v1/Registries
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#MessageRegistryFileCollection.MessageRegistryFileCollection",
    "@odata.etag": "\"1701517264\"",
    "@odata.id": "/redfish/v1/Registries",
    "@odata.type": "#MessageRegistryFileCollection.MessageRegistryFileCollection",
    "Description": "Registry Repository",
    "Members": [
        {"@odata.id": "/redfish/v1/Registries/Redfish_1.0.3_PrivilegeRegistry"},
        {"@odata.id": "/redfish/v1/Registries/BiosAttributeRegistryA2994.5.2.0"},
        {"@odata.id": "/redfish/v1/Registries/Base.1.12.0"},
        {"@odata.id": "/redfish/v1/Registries/UpdateService.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/AmiOem.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/RedfishDeviceEnablement.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/CertificateService.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/Security.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/Task.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/SyncAgent.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/StorageDevice.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/Ami.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/EventLog.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/HttpStatus.1.0.0"},
        {"@odata.id": "/redfish/v1/Registries/TaskEvent.1.0.1"},
        {"@odata.id": "/redfish/v1/Registries/IPMI.1.0.0"}
    ],
    "Members@odata.count": 16,
    "Name": "Registry Repository"
}
```

Notes (pp.235-236):
- **BIOS attribute registry:** `BiosAttributeRegistryA2994.5.2.0`. The ID depends on the board and BIOS version (the file is `BiosAttributeRegistryA2994.en-US.5.2.0.json`, per the inventory messages). Discover it at runtime instead of hard-coding it.
- The privilege registry `Redfish_1.0.3_PrivilegeRegistry` is available, which helps for mapping the privileges a proxy needs per operation.
- Base registry version is 1.12.0. The AMI registries are `AmiOem.1.0.0` and `Ami.1.0.0`. There is also an `UpdateService.1.0.0` registry.

## 3.70 MessageRegistryFile (p.237)

Schema File locator resource for a registry. For example, `/redfish/v1/Registries/Base.1.12.0` represents the Base registry file.

- **URL:** `/redfish/v1/Registries/{Registry-Instance}`
- **Methods/privileges:** GET = Login

### Table 88. Supported Properties for MessageRegistryFile (p.237)

| Property | Type | RO | Description |
|---|---|---|---|
| Languages | Array | True | Language codes of the available schemas (RFC 5646). |
| Registry | String | True | Registry name with major and minor version, in the MessageId syntax without the MessageKey. |
| Location | Array | True | Location information for this schema file. |

### Example: GET MessageRegistryFile Instance (pp.237-238)

```
GET https://{BMC_IP}/redfish/v1/Registries/{Registry-Instance}
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#MessageRegistryFile.MessageRegistryFile",
    "@odata.etag": "\"1701517264\"",
    "@odata.id": "/redfish/v1/Registries/Base.1.12.0",
    "@odata.type": "#MessageRegistryFile.v1_1_3.MessageRegistryFile",
    "Description": "Registry for Base.1.12.0",
    "Id": "Base.1.12.0",
    "Languages": [
        "en"
    ],
    "Location": [
        {
            "Language": "en",
            "Uri": "/redfish/v1/Registries/Base.1.12.0.json"
        }
    ],
    "Name": "Base.1.12.0 Registry",
    "Registry": "Base.1.12.0"
}
```

## 3.71 MessageRegistry (p.239)

Schema for all Message Registries; the properties of the registries themselves. For example, `/redfish/v1/Registries/Base.1.12.0.json` is the Base Registry with the messages for the Redfish server.

- **URL:** `/redfish/v1/Registries/{Registry-Instance.json}`
- **Methods/privileges:** GET = Login

### Table 89. Supported Properties for MessageRegistry (p.239)

| Property | Type | RO | Description |
|---|---|---|---|
| Language | String | True | RFC 5646 language code of the registry. |
| RegistryPrefix | String | True | Single-word prefix used in MessageIds that marks every message as belonging to this registry. |
| RegistryVersion | String | True | Message registry version, used in the middle part of a MessageId. |
| OwningEntity | String | True | Publisher of this registry. |
| Messages | Object | True | Pattern property: each key is the MessageId suffix and is unique within the registry. |

### Example: GET MessageRegistry Instance (pp.239-240)

```
GET https://{BMC_IP}/redfish/v1/Registries/{Registry-Instance.json}
Content-Type: application/json
```

Response (the `...` is in the PDF; only the first message is shown):

```json
{
    "@odata.type": "#MessageRegistry.v1_5_0.MessageRegistry",
    "Description": "This registry defines the base messages for Redfish",
    "Id": "Base.1.12.0",
    "Language": "en",
    "Messages": {
        "AccessDenied": {
            "ArgDescriptions": [
                "The URI provided."
            ],
            "ArgLongDescriptions": [
                "This argument shall contain the URI provided for this operation."
            ],
            "Description": "Indicates that while attempting to access, connect to, or transfer to or from another resource, the service denied access.",
            "LongDescription": "This message shall be used to indicate that while attempting to access, connect to, or transfer to or from another resource, the service denied access.",
            "Message": "While attempting to establish a connection to '%1', the service denied access.",
            "MessageSeverity": "Critical",
            "NumberOfArgs": 1,
            "ParamTypes": [
                "string"
            ],
            "Resolution": "Attempt to ensure that the URI is correct and that the service has the appropriate credentials.",
            "Severity": "Critical"
        }, ...
    },
    "Name": "Base Message Registry",
    "OwningEntity": "DMTF",
    "RegistryPrefix": "Base",
    "RegistryVersion": "1.12.0"
}
```

Note: the registry JSON has no `@odata.id` or `@odata.etag`, only `@odata.type`. Messages have both `Severity` (deprecated) and `MessageSeverity`. (p.239)

## 3.72 AttributeRegistry (BIOS attributes) (p.241)

Schema for Attribute Registries. For example, `/redfish/v1/Registries/BiosAttributeRegistryA2994.en-US.5.2.0.json` is the BIOS Attribute Registry, which lists the supported attributes and their dependencies.

- **URL:** `/redfish/v1/Registries/{AttributeRegistry-Instance.json}`
- **Methods/privileges:** GET = Login

Note: the registry *file* URL (`...A2994.en-US.5.2.0.json`) includes the locale. The MessageRegistryFile collection member ID (`BiosAttributeRegistryA2994.5.2.0`) does not. Resolve the file through the MessageRegistryFile's `Location[].Uri`. (pp.235, 241)

### Table 90. Supported Properties for AttributeRegistry (p.241)

| Property | Type | RO | Description |
|---|---|---|---|
| Language | String | True | RFC 5646 language code of the registry. |
| OwningEntity | String | True | Organization or company that publishes this Attribute Registry. |
| RegistryVersion | String | True | Version of this Attribute Registry. |
| RegistryEntries | Object | True | All attributes and metadata for this component. |
| RegistryEntries.Attributes | Array of Objects | True | Attributes and their possible values (Table 91). |
| RegistryEntries.Dependencies | Array of Objects | True | Dependencies between attributes (Table 92). |
| RegistryEntries.Menus | Array of Objects | True | Attribute menus and their hierarchy (Table 94). |
| SupportedSystems | Array of Objects | True | Systems this Attribute Registry supports. |
| SupportedSystems[].FirmwareVersion | String | True | Version of the component firmware image this registry applies to. |
| SupportedSystems[].ProductName | String | True | Product name of the computer system this registry applies to. |
| SupportedSystems[].SystemId | String | True | ID of the systems this registry applies to. |

### Table 91. Property Table of Attributes (pp.241-242)

| Property | Type | RO | Description |
|---|---|---|---|
| AttributeName | String | True | Unique name of the attribute. |
| CurrentValue | String/Boolean/Number/Null | True | Placeholder for the attribute's current value. |
| DefaultValue | String/Boolean/Number/Null | True | Default value of the attribute. |
| DisplayName | String | True | User-readable display string in the defined language. |
| DisplayOrder | Number | True | Ascending order in which this attribute appears relative to other attributes. |
| GrayOut | Boolean | True | Whether the attribute is greyed out. |
| HelpText | String | True | Help text for the attribute. |
| Hidden | Boolean | True | Whether the attribute is hidden in user interfaces. |
| Immutable | Boolean | True | Whether the attribute is immutable. |
| IsSystemUniqueProperty | Boolean | True | Whether this attribute is unique. |
| LowerBound | Number | True | Lower limit of an integer attribute. |
| MaxLength | Number | True | Maximum character length of a string attribute. |
| MenuPath | String | True | Path that describes the attribute's menu hierarchy. |
| MinLength | Number | True | Minimum character length of a string attribute. |
| ReadOnly | Boolean | True | Whether this attribute is read-only. |
| ResetRequired | Boolean | True | Whether a system or device reset is needed for a value change to take effect. |
| ScalarIncrement | Number | True | Step for incrementing or decrementing an integer attribute on each change request. |
| Type | String | True | Attribute type. Enum: `Enumeration` (list of known possible values), `String` (free-form text), `Integer`, `Boolean` (true or false), `Password` (values not shown as plain text). |
| UefiDevicePath | String | True | UEFI device path that qualifies this attribute. |
| UpperBound | Number | True | Upper limit of an integer attribute. |
| UefiKeywordName | String | True | UEFI keyword string of this attribute. |
| UefiNamespaceId | String | True | UEFI namespace ID of the attribute. |
| ValueExpression | String | True | Regular expression (Perl dialect) that validates the attribute value. |
| WarningText | String | True | Warning text for the attribute. |
| WriteOnly | Boolean | True | Whether this attribute is write-only. |
| Value | Array of Objects | True | Possible values of enumerated attributes. |
| Value[].ValueDisplayName | String | True | User-readable display string of the value in the defined language. |
| Value[].ValueName | Boolean (as printed; String in DMTF) | True | Unique value name of the attribute. |

### Table 92. Property Table of Dependencies (pp.242-243)

| Property | Type | RO | Description |
|---|---|---|---|
| Dependency | Object | True | Dependency expression for one or more attributes in this registry. |
| Dependency.MapFrom | Object (array in the example) | True | Map-from conditions for a mapping dependency (Table 93). |
| Dependency.MapToAttribute | String | True | AttributeName of the attribute this dependency expression affects. |
| Dependency.MapToProperty | String | True | Metadata property of the attribute that the map-from condition changes. Enum: `CurrentValue`, `DefaultValue`, `DisplayName`, `DisplayOrder`, `GrayOut`, `HelpText`, `Immutable`, `Hidden`, `LowerBound`, `MaxLength`, `MinLength`, `ReadOnly`, `ScalarIncrement`, `UpperBound`, `ValueExpression`, `WarningText`, `WriteOnly`. |
| Dependency.MapToValue | String/Boolean/Number/Null | True | Value the map-to property changes to if the dependency expression is `true`. |
| DependencyFor | String | True | AttributeName of the attribute whose change triggers evaluation of this dependency expression. |
| Type | String | True | Dependency structure type. Enum: `Map` (a simple mapping dependency). |

### Table 93. Property Table for MapFrom (pp.243-244)

| Property | Type | RO | Description |
|---|---|---|---|
| MapFromAttribute | String | True | Attribute used to evaluate this dependency expression. |
| MapFromCondition | String | True | Condition used to evaluate this dependency expression. Enum: `EQU` (Equal), `GEQ` (Greater than or Equal), `GTR` (Greater than), `LEQ` (Less than or Equal), `LSS` (Less than), `NEQ` (Not Equal). |
| MapFromProperty | String | True | Metadata property of MapFromAttribute used for evaluation. Enum: `CurrentValue`, `DefaultValue`, `GrayOut`, `Hidden`, `LowerBound`, `MaxLength`, `MinLength`, `ReadOnly`, `ScalarIncrement`, `UpperBound`, `WriteOnly`. |
| MapFromValue | String/Boolean/Number | True | Value used to evaluate this dependency expression. |
| MapTerms | String | True | Logical term that combines two or more map-from conditions. Enum: `AND`, `OR`. |

### Table 94. Property Table for Menus (p.244)

| Property | Type | RO | Description |
|---|---|---|---|
| DisplayName | String | True | User-readable display string of the menu in the defined language. |
| DisplayOrder | Number | True | Ascending order in which this menu appears relative to other menus. |
| GrayOut | Boolean | True | Whether the menu is greyed out. A greyed-out menu is not accessible in user interfaces. |
| Hidden | Boolean | True | Whether the menu is hidden in user interfaces. |
| MenuName | String | True | Unique name of the menu. |
| MenuPath | String | True | Path to the menu names that describes the menu hierarchy relative to other menus. |
| ReadOnly | Boolean | True | Whether the menu is read-only. A read-only menu, its properties, and its sub-menus are not accessible in user interfaces. |

### Example: GET AttributeRegistry Instance (pp.244-245)

```
GET https://{BMC_IP}/redfish/v1/Registries/{AttributeRegistry-Instance.json}
Content-Type: application/json
```

Response (the `...` is in the PDF; only one Dependency entry is shown, and no Attributes or Menus):

```json
{
    "RegistryVersion": "5.2.0",
    "OwningEntity": "AMI",
    "Name": "A2994 BIOS Attribute Registry",
    "@odata.context": "/redfish/v1/$metadata#AttributeRegistry.AttributeRegistry",
    "RegistryEntries": {
        "Dependencies": [
            {
                "Type": "Map",
                "DependencyFor": "SYSACS",
                "Dependency": {
                    "MapToValue": "true",
                    "MapToProperty": "GrayOut",
                    "MapFrom": [
                        {
                            "MapFromValue": 1,
                            "MapFromProperty": "CurrentValue",
                            "MapFromAttribute": "SYSACS",
                            "MapFromCondition": "EQU"
                        }
                    ],
                    "MapToAttribute": "TCG003"
                }
            },
...
    },
    "@odata.type": "#AttributeRegistry.v1_3_1.AttributeRegistry",
    "@odata.id": "/redfish/v1/Registries/BiosAttributeRegistryA2994.en-US.5.2.0.json",
    "Language": "en-US",
    "@odata.etag": "Dummyetag",
    "Id": "BiosAttributeRegistryA2994.en-US.5.2.0",
    "Description": "This registry defines a representation of BIOS Attribute instances"
}
```

Caveats (pp.244-245):
- BIOS attribute names are **opaque short codes** (e.g. `SYSACS`, `TCG003`), the AMI setup-question style, not friendly names. Map them to meanings through `DisplayName`, `HelpText`, and `MenuPath` from this registry.
- In the dependency, `MapToValue` is the **string `"true"`** and not a JSON boolean. `MapFromValue` is a number (`1`). Compare loosely or convert types.
- `@odata.etag` is the literal string **`"Dummyetag"`**, so ETags on registry files are meaningless. Do not use them for caching or If-Match.
- `OwningEntity` is `"AMI"`. The registry name prefix `A2994` looks like the AMI BIOS project ID.

## 3.73 PrivilegeRegistry (p.246)

Operation-to-privilege mappings.

- **URL:** `/redfish/v1/Registries/{PrivilegeRegistry-Instance.json}`
- **Methods/privileges:** GET = Login

### Table 95. Supported Properties for PrivilegeRegistry (p.246)

| Property | Type | RO | Description |
|---|---|---|---|
| OEMPrivilegesUsed | Array | True | OEM privileges used in this mapping. |
| PrivilegesUsed | Array | True | Redfish standard privileges used in this mapping. Values: `Login` (log in to the service and read resources), `ConfigureManager` (configure Manager resources), `ConfigureUsers` (configure Users and their Accounts), `ConfigureSelf` (change the password of the current user account), `ConfigureComponents` (configure components managed by this service). |
| Mappings | Array | True | Mappings between entities and the privileges needed to access them (Table 96). |

### Table 96. PrivilegeRegistry Mapping Property (p.246)

| Property | Type | RO | Description |
|---|---|---|---|
| Entity | String | True | Resource name, such as "Manager". |
| OperationMap | Object | True | Mapping between HTTP methods and the privilege required for the Resource (Table 97). |
| PropertyOverrides | Array | True | Privilege overrides of properties, such as the Password property in the ManagerAccount resource (Table 98). |
| ResourceURIOverrides | Array | True | Privilege overrides of Resource URIs. The target lists the Resource URI and the new privileges (Table 98). |
| SubordinateOverrides | Array | True | Privilege overrides of the subordinate Resource. Target lists are identified by Resource type (Table 98). |

### Table 97. PrivilegeRegistry OperationMap Property (p.247)

| Property | Type | RO | Description |
|---|---|---|---|
| DELETE | Array | True | Privilege required for HTTP DELETE (Table 99). |
| GET | Array | True | Privilege required for HTTP GET. |
| HEAD | Array | True | Privilege required for HTTP HEAD. |
| PATCH | Array | True | Privilege required for HTTP PATCH. |
| POST | Array | True | Privilege required for HTTP POST. |
| PUT | Array | True | Privilege required for HTTP PUT. |

### Table 98. PrivilegeRegistry Target Privilege Map (p.247)

| Property | Type | RO | Description |
|---|---|---|---|
| OperationMap | Object | True | Mapping between the HTTP operation and the privilege required to complete it. |
| Targets | Array | True | URIs, Resource types, or properties. For example `/redfish/v1/Systems/1`, `Manager`, or `Password`. |

### Table 99. PrivilegeRegistry OperationPrivilege (p.247)

| Property | Type | RO | Description |
|---|---|---|---|
| Privilege | Array | True | Privileges required to complete a specific HTTP operation on a Resource. These strings match zero or more strings in PrivilegesUsed and OEMPrivilegesUsed. |

### Example: GET PrivilegeRegistry Instance (pp.247-249)

```
GET https://{BMC_IP}/redfish/v1/Registries/{PrivilegeRegistry-Instance.json}
```

Response (the `...` is in the PDF; only the Manager entity is shown):

```json
{
    "@odata.type": "#PrivilegeRegistry.v1_1_4.PrivilegeRegistry",
    "Id": "Redfish_1.0.3_PrivilegeRegistry",
    "Mappings": [
        {
            "Entity": "Manager",
            "OperationMap": {
                "DELETE": [ { "Privilege": [ "ConfigureManager" ] } ],
                "GET":    [ { "Privilege": [ "Login" ] } ],
                "HEAD":   [ { "Privilege": [ "Login" ] } ],
                "PATCH":  [ { "Privilege": [ "ConfigureManager" ] } ],
                "POST":   [ { "Privilege": [ "ConfigureManager" ] } ],
                "PUT":    [ { "Privilege": [ "ConfigureManager" ] } ]
            }
        },
...
    ],
    "Name": "Privilege Mapping array collection",
    "OEMPrivilegesUsed": [
        "ConfigureHostInterface"
    ],
    "PrivilegesUsed": [
        "Login",
        "ConfigureManager",
        "ConfigureUsers",
        "ConfigureComponents",
        "ConfigureSelf"
    ]
}
```

Notes (pp.248-249):
- **OEM privilege `ConfigureHostInterface`** exists. It is probably needed for HostInterface configuration, used together with the `HostInterfaceAdministrator` role (see 3.60).
- A proxy implementing RBAC can GET this registry at runtime and derive the required privilege for each entity and method instead of hard-coding the method tables in this document.

## 3.74 Session Service (p.250)

Session Service properties, with a link to the list of sessions.

- **URL:** `/redfish/v1/SessionService`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureManager |

### Table 100. Supported Properties for Session Service (p.250)

| Property | Type | RO | Description |
|---|---|---|---|
| ServiceEnabled | Boolean | False | Whether this service is enabled. Null by default. |
| SessionTimeout | Number | False | Seconds of inactivity before the session service closes the session. |
| Sessions | Object | True | Link to the collection of Sessions. |
| Actions | Object | True | Actions under the OEM property, if any. |

### Example: GET Session Service Instance (pp.250-251)

```
GET https://{BMC_IP}/redfish/v1/SessionService
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#SessionService.SessionService",
    "@odata.etag": "\"1701522664\"",
    "@odata.id": "/redfish/v1/SessionService",
    "@odata.type": "#SessionService.v1_1_6.SessionService",
    "Description": "Session Service",
    "Id": "SessionService",
    "Name": "Session Service",
    "ServiceEnabled": true,
    "SessionTimeout": 30,
    "Sessions": {
        "@odata.id": "/redfish/v1/SessionService/Sessions"
    },
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    }
}
```

Note: **`SessionTimeout` defaults to 30 seconds** of inactivity, which is very short. A proxy that holds a BMC session must send keepalive requests faster than that, PATCH a longer timeout, or re-authenticate when it gets a 401. (p.250)

### Example: PATCH Session Service Instance (p.251)

```
PATCH https://{BMC_IP}/redfish/v1/SessionService
Content-Type: application/json
```

```json
{
    "ServiceEnabled": true,
    "SessionTimeout": 120
}
```

Response: **204.**

## 3.75 SessionCollection (p.252)

Collection of links to each session.

- **URL:** `/redfish/v1/SessionService/Sessions`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| POST | **NoAuth** (login; no credentials needed) |

- **Properties:** common ODATA properties only.

### Example: GET SessionCollection Instance (p.252)

```
GET https://{BMC}/redfish/v1/SessionService/Sessions
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#SessionCollection.SessionCollection",
    "@odata.etag": "\"1701527487\"",
    "@odata.id": "/redfish/v1/SessionService/Sessions",
    "@odata.type": "#SessionCollection.SessionCollection",
    "Description": "Collection of Sessions",
    "Members": [
        {
            "@odata.id": "/redfish/v1/SessionService/Sessions/879d6db31a24cb717a090f5831a0aa3b"
        }
    ],
    "Members@odata.count": 1,
    "Name": "Session Collection"
}
```

### Example: POST SessionCollection Instance (login) (pp.252-253)

```
POST https://{BMC}/redfish/v1/SessionService/Sessions
Content-Type: application/json
```

```json
{
    "UserName": "username",
    "Password": "password"
}
```

Response: **201. The body is a GET response with the properties of the new Session**:

```json
{
    "@odata.context": "/redfish/v1/$metadata#Session.Session",
    "@odata.etag": "\"1701527340\"",
    "@odata.id": "/redfish/v1/SessionService/Sessions/0b34753ae5b5e95a5f08f1bad69e07f0",
    "@odata.type": "#Session.v1_2_1.Session",
    "Description": "Session for user admin",
    "Id": "0b34753ae5b5e95a5f08f1bad69e07f0",
    "Name": "admin Session",
    "UserName": "admin"
}
```

Notes (p.253):
- Session IDs are 32 hex characters (128-bit).
- This range of the PDF does not show the response headers. Under the Redfish standard the token comes back in the `X-Auth-Token` header and the session URI in `Location`; send `X-Auth-Token` on later requests. Check part 1 for any auth notes specific to this firmware.
- Unlike the Subscription and SMTP-cert POSTs, the 201 body's `@odata.id` here is the instance URI. Use it (or `Location`) to DELETE the session on logout.

## 3.76 Session (p.254)

Details of a given session.

- **URL:** `/redfish/v1/SessionService/Sessions/{Session-Id}`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| DELETE | ConfigureManager |

Caveat: DELETE is listed as needing **ConfigureManager**. Taken literally, a non-admin user (e.g. ReadOnly/Operator) may be unable to delete (log out) even their own session; DMTF lets the session owner do so via ConfigureSelf. Verify on hardware. If true, a proxy using low-privilege accounts must rely on session timeout. (p.254)

### Table 101. Supported Properties for Session (p.254)

| Property | Type | RO | Description |
|---|---|---|---|
| UserName | String | True | UserName of the account for this session. |
| Password | String | True | Used in a POST to give the password when creating a new session. |

### Example: GET Session Instance (p.254)

```
GET https://{BMC_IP}/redfish/v1/SessionService/Sessions/{Session-Id}
Content-Type: application/json
```

```json
{
    "@odata.context": "/redfish/v1/$metadata#Session.Session",
    "@odata.etag": "\"1701527905\"",
    "@odata.id": "/redfish/v1/SessionService/Sessions/8a6d8bb8a48f289844abcb993f0795ce",
    "@odata.type": "#Session.v1_2_1.Session",
    "Description": "Session for user admin",
    "Id": "8a6d8bb8a48f289844abcb993f0795ce",
    "Name": "admin Session",
    "UserName": "admin"
}
```

### Example: DELETE Session Instance (logout) (p.254)

```
DELETE https://{BMC_IP}/redfish/v1/SessionService/Sessions/{Session-Id}
Content-Type: application/json
```

Response: **204, no body.**

## 3.77 ComputerSystemCollection (p.255)

Collection of links, each to a Computer System resource instance.

- **URL:** `/redfish/v1/Systems`
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| POST | ConfigureComponents (composition) |

- **Properties:** common ODATA properties only.

### Example: GET ComputerSystemCollection Instance (pp.255-256)

```
GET https://{BMC_IP}/redfish/v1/Systems
Content-Type: application/json
```

```json
{
    "@Redfish.CollectionCapabilities": {
        "@odata.type": "#CollectionCapabilities.v1_2_0.CollectionCapabilities",
        "Capabilities": [
            {
                "CapabilitiesObject": {
                    "@odata.id": "/redfish/v1/Systems/Capabilities"
                },
                "Links": {
                    "TargetCollection": {
                        "@odata.id": "/redfish/v1/Systems"
                    }
                },
                "UseCase": "ComputerSystemComposition"
            }
        ]
    },
    "@odata.context": "/redfish/v1/$metadata#ComputerSystemCollection.ComputerSystemCollection",
    "@odata.etag": "\"1702101951\"",
    "@odata.id": "/redfish/v1/Systems",
    "@odata.type": "#ComputerSystemCollection.ComputerSystemCollection",
    "Description": "Collection of Computer Systems",
    "Members": [
        { "@odata.id": "/redfish/v1/Systems/Self" }
    ],
    "Members@odata.count": 1,
    "Name": "Systems Collection"
}
```

The system ID is `Self`.

### Example: POST ComputerSystemCollection Instance (composition) (pp.256-257)

```
POST https://{BMC_IP}/redfish/v1/Systems
Content-Type: application/json
```

```json
{
    "Name": "NewSystem",
    "Links": {
        "ResourceBlocks": [
            { "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/ComputeBlock" },
            { "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/DrivesBlock" }
        ]
    },
    "HostName": "Demo01"
}
```

Response: **201, with a body**:

```json
{
    "@odata.context": "/redfish/v1/$metadata#ComputerSystem.ComputerSystem",
    "@odata.etag": "W/\"1702107268\"",
    "@odata.id": "/redfish/v1/Systems",
    "@odata.type": "#ComputerSystem.v1_15_0.ComputerSystem",
    "Actions": {
        "#ComputerSystem.Reset": {
            "@Redfish.ActionInfo": "/redfish/v1/Systems/Self/ResetActionInfo",
            "target": "/redfish/v1/Systems/Self/Actions/ComputerSystem.Reset"
        }
    },
    "Boot": {
        "BootSourceOverrideEnabled": "Disabled",
        "BootSourceOverrideEnabled@Redfish.AllowableValues": [
            "Disabled",
            "Once",
            "Continuous"
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
        "UefiTargetBootSourceOverride": null
    },
    "Description": "Computer System",
    "HostName": "Demo01",
    "Id": "NewSystem",
    "Links": {
        "ResourceBlocks": [
            { "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/ComputeBlock" },
            { "@odata.id": "/redfish/v1/CompositionService/ResourceBlocks/DrivesBlock" }
        ]
    },
    "Memory": {
        "@odata.id": "/redfish/v1/Systems/Self/Memory"
    },
    "Name": "NewSystem",
    "PowerState": "On",
    "Processors": {
        "@odata.id": "/redfish/v1/Systems/Self/Processors"
    },
    "SimpleStorage": {
        "@odata.id": "/redfish/v1/Systems/Self/SimpleStorage"
    },
    "Storage": {
        "@odata.id": "/redfish/v1/Systems/Self/Storage"
    },
    "SystemType": "Composed",
    "UUID": "f9f786bd-ab1e-40ee-b492-c385d6017a2c"
}
```

Caveats (pp.256-257):
- The ETag here is a **weak ETag** (`W/"1702107268"`); elsewhere ETags are strong. If-Match handling must accept both.
- The 201 body's `@odata.id` is the collection (`/redfish/v1/Systems`), while `Id` is `NewSystem`. The sub-resource links (Memory, Processors, Reset target) still point at `Systems/Self`. A composed system appears to be only a logical view over the single physical system. Composition is of little practical use on a single-node board.
- **`BootSourceOverrideTarget@Redfish.AllowableValues`** lists `None, Pxe, Floppy, Cd, Usb, Hdd, BiosSetup, Utilities, Diags, UefiShell, UefiTarget, SDCard, UefiHttp, RemoteDrive, UefiBootNext`. `BootSourceOverrideEnabled` allows `Disabled, Once, Continuous`. Use these values when setting a boot override on `Systems/Self` (PATCH `Boot`, ConfigureComponents).
- The Reset action ActionInfo is `/redfish/v1/Systems/Self/ResetActionInfo`, and the target is `/redfish/v1/Systems/Self/Actions/ComputerSystem.Reset`.

## 3.78 ComputerSystem (p.258) (continues in part 3)

A computer system: a machine (physical or virtual) and the local resources that can be reached from it, such as memory, CPU, and other devices. Information on these resources or subsystems is also linked from this resource.

- **URL:** `/redfish/v1/Systems/{Systems-Instance}` (that is, `/redfish/v1/Systems/Self`)
- **Methods/privileges:**

| Method | Privilege |
|---|---|
| GET | Login |
| PATCH | ConfigureComponents |
| POST | ConfigureComponents (actions, e.g. ComputerSystem.Reset) |

### Table 102. Supported Properties for ComputerSystem (pp.258-260)

| Property | Type | RO | Description |
|---|---|---|---|
| @Redfish.Settings | Object | True | Reference to the FutureState URI for this resource. **The system has a settings object (pending-settings / FutureState pattern).** |
| @Redfish.Settings.@odata.type | String | True | See ODATA and Frequently Used Properties. |
| @Redfish.Settings.SettingsObject | Object | True | Reference to the FutureState URI for this resource. |
| SystemType | String | True | Kind of system. Enum: `Physical` (a computer system), `Composed` (created by binding resource blocks together). |
| Links | Object | True | References to related resources that this resource does not contain. |
| Links.Oem | Object | - | OEM extensions. |
| Links.Chassis | Array | True | Chassis that contain this system. |
| Links.Chassis@odata.count | Number | True | Number of items. |
| Links.ManagedBy | Array | True | Managers responsible for this system. |
| Links.ManagedBy@odata.count | Number | True | Number of items. |
| Links.PoweredBy | Array | True | IDs of resources that power this system. |
| Links.PoweredBy@odata.count | Number | True | Number of items. |
| Links.CooledBy | Array | True | IDs of resources that cool this system. |
| Links.CooledBy@odata.count | Number | True | Number of items. |
| Links.Endpoints | Array | True | Endpoints that connect to this system. |
| AssetTag | String | **False** | User-definable tag for inventory tracking. |
| Manufacturer | String | True | Manufacturer or OEM of this system. |
| Model | String | True | Model number of this system. |
| SKU | String | True | Manufacturer SKU of the system. |
| SerialNumber | String | True | Serial number of this system. |
| PartNumber | String | True | Manufacturer part number. |
| SubModel | String | True | Sub-model (or configuration) of the system. |
| UUID | String | True | UUID of the system. |
| HostName | String | True | Host name of this system as reported by the OS or hypervisor. Usually provided to the Manager by a service running in the host OS. |
| IndicatorLED | String | **False** | State of the system indicator light. Enum: `Lit`, `Blinking`, `Off`. |
| PowerState | String | True | Current power state. Enum: `On`, `Off` (some components such as the management controller may still have AUX power). |
| Boot | Object | **False** | Boot information for the system (Table 103). |
| BiosVersion | String | True | Version of the system BIOS or primary system firmware. |
| ProcessorSummary | Object | - | Summary of the central processors. |
| ProcessorSummary.Count | Number | True | Number of processors. |
| ProcessorSummary.Model | String | True | Processor model of the primary or majority of processors. |
| ProcessorSummary.Status | Object | True | Status and health of the resource and its dependents. |
| MemorySummary | Object | - | Summary of central memory. |
| MemorySummary.TotalSystemMemoryGiB | Number | True | Total installed, OS-accessible memory (RAM), in GiB. |
| MemorySummary.Status | Object | True | Status and health. |
| MemorySummary.MemoryMirroring | String | True | Ability and type of memory mirroring supported. |
| MemorySummary.Metrics | Object | True | Link to metrics for all memory in this system. |
| MemorySummary.TotalSystemPersistentMemoryGiB | Number | True | Total configured, system-accessible persistent memory, in GiB. |
| Actions | Object | True | Available actions for this resource. |
| Processors | Object | True | Link to the Processors collection. |
| EthernetInterfaces | Object | True | Link to the collection of Ethernet interfaces for this system. |
| SimpleStorage | Object | True | Link to the collection of storage devices. |
| LogServices | Object | True | Link to the Log Services collection. |
| Status | Object | True | Status and health of the resource and its dependents. |
| TrustedModules | Array | True | Trusted Modules (TPM) in the system. |
| SecureBoot | Object | True | Link to the UEFI SecureBoot resource. |
| Bios | Object | True | Link to the BIOS settings for this system. |
| Memory | Object | True | Link to the Memory collection. |
| Storage | Object | True | Link to the collection of storage devices. |
| NetworkInterfaces | Object | True | Link to the Network Interfaces collection. |
| USBControllers | Object | True | Link to the USB controller devices collection. |
| GraphicsControllers | Object | True | Link to the Graphics controller devices collection. |
| HostingRoles | Array | True | Hosting roles this system supports. Enum: `ApplicationServer` (general-purpose applications), `StorageServer` (acts as a storage server), `Switch` (acts as a switch). |
| HostedServices | Object | True | Services this computer system supports. |
| PCIeDevices | Object | True | Link to the collection of PCIe Devices used by this system. |
| PCIeDevices@odata.count | Number | True | Number of PCIeDevices used by this system. |
| PCIeFunctions | Object | True | Link to the collection of PCIe Functions used by this system. |
| PCIeFunctions@odata.count | Number | True | Number of PCIeFunctions used by this system. |
| PowerRestorePolicy | String | **False** | Desired PowerState when power is applied to the system. Enum: `AlwaysOn` (always power on), `AlwaysOff` (always stay off), `LastState` (return to the last power state). |

Writable properties on ComputerSystem (from Table 102): `AssetTag`, `IndicatorLED`, `Boot` (sub-properties in Table 103), and `PowerRestorePolicy`. Note: the table lists `PCIeDevices` and `PCIeFunctions` as Objects (links to collections). In DMTF they are arrays of links. Check the actual GET in part 3.

### Table 103. ComputerSystem Boot Properties (p.260) (continues in part 3)

| Property | Type | RO | Description |
|---|---|---|---|
| BootOptions | Object | True | Link to the collection of UEFI Boot Options for this Computer System. |
| CertificateService | Object | True | Link to the collection of CertificateServices. (Non-standard placement inside Boot.) |
| BootNext | String | **False** | BootOptionReference of the Boot Option for a one-time boot when BootSourceOverrideTarget is `UefiBootNext`. |
| BootSourceOverrideTarget | String | **False** | Boot source used at next boot instead of the normal boot device, if BootSourceOverrideEnabled is true. (Allowed values listed in 3.77 above.) |

*(Table 103 continues on p.261: see part 3.)*

---

## Cross-cutting summary for implementers (pages 131 to 260)

### Status codes observed

| Operation | Status | Body |
|---|---|---|
| Most PATCH (CompositionService, ResourceBlocks, EventService, Subscription, Manager, HostInterface, SerialInterface, VirtualMedia, SessionService) | 204 | none |
| PATCH Chassis Power | 200 | full resource |
| PATCH Oem/Ami/Configurations | 200 | full resource plus `@Message.ExtendedInfo` (single object); web server restarts |
| **PATCH Managers/Self/NetworkProtocol** | **202** | Task |
| POST EventService.SubmitTestEvent | 202 | Task |
| POST Manager.Reset | 202 | Task |
| POST VirtualMedia.InsertMedia | 202 | Task |
| POST VirtualMedia.EjectMedia | 204 | none |
| POST EventDestination.ResumeSubscription | 204 | none |
| POST Subscriptions | 201 | resource, but `@odata.id` = collection URI |
| POST SMTP Certificates | 201 | resource, `@odata.id` = collection URI |
| POST Systems (composition) | 201 | resource, `@odata.id` = collection URI |
| POST SessionService/Sessions | 201 | session resource (`@odata.id` = instance) |
| DELETE Subscription / SMTP cert / Session | 204 | none |

### Task monitor

Asynchronous operations return 202 with a Task body: `@odata.id` `/redfish/v1/TaskService/Tasks/{n}`, `@odata.type` `#Task.v1_4_2.Task`, `TaskState: "New"`. Task IDs are small sequential integers. The PDF does not show `Location` headers or a separate TaskMonitor URI. Poll the Task resource.

### ETags

- Strong ETags that are quoted Unix timestamps (e.g. `"1701527905"`), the same across many resources at a moment. They look like a global or last-modified timestamp rather than a content hash.
- A weak ETag `W/"..."` appears on the composed ComputerSystem POST response.
- `"Dummyetag"` appears on the BIOS AttributeRegistry file.
- No page in this range states that `If-Match` is required on PATCH. None of the PATCH examples send it.

### Privileges

`Login`, `ConfigureManager`, `ConfigureUsers`, `ConfigureComponents`, and `ConfigureSelf`, plus the OEM privilege `ConfigureHostInterface`. Session POST is `NoAuth`. The SMTP certificate resource needs ConfigureManager even for GET. Session DELETE needs ConfigureManager.

### Collections and paging

Server-side paging with `Members@odata.nextLink` (`?$skip=50`) on large collections (PCIeDevices). The AMI SMTP certificate collection uses `Certificates` instead of `Members`.

### AMI OEM extensions seen in this range

- `Oem.Ami` on Power Voltages and Thermal Fans/Temperatures: `#AMIChassisPowerThermal.v1_0_0` `OwnerLUN`.
- `Oem.Ami` on EventService: `#AMIEventService.v1_0_0` `SecondarySMTP`, `Certificates`, and the `/redfish/v1/EventService/Oem/Ami/SMTP/Certificates` resource.
- `Oem.Ami` on Manager: `#AMIManager.v1_0_0` `ManagerServiceInfo`, `PowerSaveMode`, and `VirtualMedia {CDInstances, RMediaStatus}`. OEM actions: `AMIManager.RedfishDBReset`, `AMIManager.RedfishPowerSaveMode`, `AMIVirtualMedia.ConfigureCDInstance`, `AMIVirtualMedia.EnableRMedia`.
- `Oem.Ami` on VirtualMedia: `#AMIVirtualMedia.v1_0_0` `BoostMode` (writable), `RedirectionStatus`, and `TransferProtocolType` (`SMBE`).
- `/redfish/v1/Oem/Ami/Configurations` (CertificateAuthorityUrl; DHCP option 248 `REDFISH_CA_CERTIFICATE`).
- `/redfish/v1/Oem/Ami/InventoryData/Status` (BIOS inventory push status).
- `AmiOem` message registry (e.g. `AmiOem.1.0.WebServerRestarting`).
- **No `Oem.ASRockRack` extensions appear in pages 131 to 260.**

### Not covered in this range

UpdateService and firmware upload (HttpPushUri, multipart), Bios resource and settings PATCH, AccountService, TaskService, TelemetryService, and LogServices. These sections are not in pages 131 to 260; expect them in part 1 or part 3. The pages here cover only Event subscriptions/SSE, Sessions, VirtualMedia, and the BIOS AttributeRegistry from the requested list.
