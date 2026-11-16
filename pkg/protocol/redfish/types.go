package redfish

import (
	"encoding/json"
	"strings"
)

// Typed views of common Redfish resources. Only fields the proxy uses are
// declared; Raw-style access is available through Client.Get into a map or
// json.RawMessage. Numeric readings are pointers (null/absent = nil).

// Link is an {"@odata.id": "..."} reference.
type Link struct {
	ID string `json:"@odata.id"`
}

// Resource carries the common OData/Redfish header fields.
type Resource struct {
	ODataID   string `json:"@odata.id"`
	ODataType string `json:"@odata.type"`
	ODataETag string `json:"@odata.etag"`
	ID        string `json:"Id"`
	Name      string `json:"Name"`
}

// Status is the common Status object.
type Status struct {
	State        string `json:"State"`
	Health       string `json:"Health"`
	HealthRollup string `json:"HealthRollup"`
}

// Collection is a resource collection page.
type Collection struct {
	Resource
	Members  []json.RawMessage `json:"Members"`
	Count    *int              `json:"Members@odata.count"`
	NextLink string            `json:"Members@odata.nextLink"`
}

// MemberLinks returns the @odata.id of each member.
func (c *Collection) MemberLinks() []string {
	out := make([]string, 0, len(c.Members))
	for _, m := range c.Members {
		var l Link
		if json.Unmarshal(m, &l) == nil && l.ID != "" {
			out = append(out, l.ID)
		}
	}
	return out
}

// Action is one entry of an Actions object. Allowable holds every
// "<Param>@Redfish.AllowableValues" annotation keyed by parameter name.
type Action struct {
	Target     string              `json:"target"`
	Title      string              `json:"title,omitempty"`
	ActionInfo string              `json:"@Redfish.ActionInfo,omitempty"`
	Allowable  map[string][]string `json:"-"`
}

// UnmarshalJSON collects the AllowableValues annotations.
func (a *Action) UnmarshalJSON(b []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	for k, v := range m {
		switch {
		case k == "target":
			_ = json.Unmarshal(v, &a.Target)
		case k == "title":
			_ = json.Unmarshal(v, &a.Title)
		case k == "@Redfish.ActionInfo":
			_ = json.Unmarshal(v, &a.ActionInfo)
		case strings.HasSuffix(k, "@Redfish.AllowableValues"):
			var vals []string
			if json.Unmarshal(v, &vals) == nil {
				if a.Allowable == nil {
					a.Allowable = map[string][]string{}
				}
				a.Allowable[strings.TrimSuffix(k, "@Redfish.AllowableValues")] = vals
			}
		}
	}
	return nil
}

// Actions maps "#Type.Action" to its Action; the "Oem" member is skipped.
type Actions map[string]Action

// UnmarshalJSON skips non-object members such as "Oem" sub-maps gracefully.
func (as *Actions) UnmarshalJSON(b []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	out := Actions{}
	for k, v := range m {
		if !strings.HasPrefix(k, "#") {
			continue
		}
		var a Action
		if json.Unmarshal(v, &a) == nil {
			out[k] = a
		}
	}
	*as = out
	return nil
}

// ActionInfo is the #ActionInfo resource.
type ActionInfo struct {
	Resource
	Parameters []struct {
		Name            string   `json:"Name"`
		Required        bool     `json:"Required"`
		DataType        string   `json:"DataType"`
		AllowableValues []string `json:"AllowableValues"`
	} `json:"Parameters"`
}

// Allowable returns AllowableValues for parameter name.
func (ai *ActionInfo) Allowable(name string) []string {
	if ai == nil {
		return nil
	}
	for _, p := range ai.Parameters {
		if p.Name == name {
			return p.AllowableValues
		}
	}
	return nil
}

// ---- ServiceRoot ----

// ExpandQuery is ProtocolFeaturesSupported.ExpandQuery.
type ExpandQuery struct {
	ExpandAll bool `json:"ExpandAll"`
	Levels    bool `json:"Levels"`
	Links     bool `json:"Links"`
	NoLinks   bool `json:"NoLinks"`
	MaxLevels int  `json:"MaxLevels"`
}

// ProtocolFeatures is ServiceRoot.ProtocolFeaturesSupported.
type ProtocolFeatures struct {
	ExcerptQuery    bool         `json:"ExcerptQuery"`
	ExpandQuery     *ExpandQuery `json:"ExpandQuery"`
	FilterQuery     bool         `json:"FilterQuery"`
	OnlyMemberQuery bool         `json:"OnlyMemberQuery"`
	SelectQuery     bool         `json:"SelectQuery"`
	TopSkipQuery    bool         `json:"TopSkipQuery"`
}

// ServiceRoot is GET /redfish/v1/.
type ServiceRoot struct {
	Resource
	RedfishVersion            string            `json:"RedfishVersion"`
	Vendor                    string            `json:"Vendor"`
	Product                   string            `json:"Product"`
	UUID                      string            `json:"UUID"`
	ProtocolFeaturesSupported *ProtocolFeatures `json:"ProtocolFeaturesSupported"`
	Systems                   Link              `json:"Systems"`
	Chassis                   Link              `json:"Chassis"`
	Managers                  Link              `json:"Managers"`
	SessionService            Link              `json:"SessionService"`
	AccountService            Link              `json:"AccountService"`
	UpdateService             Link              `json:"UpdateService"`
	EventService              Link              `json:"EventService"`
	Tasks                     Link              `json:"Tasks"`
	CertificateService        Link              `json:"CertificateService"`
	TelemetryService          Link              `json:"TelemetryService"`
	Links                     struct {
		Sessions Link `json:"Sessions"`
	} `json:"Links"`
	Oem map[string]json.RawMessage `json:"Oem"`
}

// ---- ComputerSystem ----

// Boot is ComputerSystem.Boot.
type Boot struct {
	BootSourceOverrideEnabled string   `json:"BootSourceOverrideEnabled,omitempty"`
	BootSourceOverrideMode    string   `json:"BootSourceOverrideMode,omitempty"`
	BootSourceOverrideTarget  string   `json:"BootSourceOverrideTarget,omitempty"`
	TargetAllowable           []string `json:"BootSourceOverrideTarget@Redfish.AllowableValues,omitempty"`
	BootOrder                 []string `json:"BootOrder,omitempty"`
}

// ComputerSystem is a Systems member.
type ComputerSystem struct {
	Resource
	SystemType              string  `json:"SystemType"`
	Manufacturer            string  `json:"Manufacturer"`
	Model                   string  `json:"Model"`
	SKU                     string  `json:"SKU"`
	SerialNumber            string  `json:"SerialNumber"`
	PartNumber              string  `json:"PartNumber"`
	AssetTag                string  `json:"AssetTag"`
	UUID                    string  `json:"UUID"`
	HostName                string  `json:"HostName"`
	BiosVersion             string  `json:"BiosVersion"`
	PowerState              string  `json:"PowerState"`
	PowerRestorePolicy      *string `json:"PowerRestorePolicy"`
	IndicatorLED            *string `json:"IndicatorLED"`
	LocationIndicatorActive *bool   `json:"LocationIndicatorActive"`
	Status                  Status  `json:"Status"`
	Boot                    *Boot   `json:"Boot"`
	ProcessorSummary        struct {
		Count int    `json:"Count"`
		Model string `json:"Model"`
	} `json:"ProcessorSummary"`
	MemorySummary struct {
		TotalSystemMemoryGiB *float64 `json:"TotalSystemMemoryGiB"`
	} `json:"MemorySummary"`
	Processors         Link    `json:"Processors"`
	Memory             Link    `json:"Memory"`
	Storage            Link    `json:"Storage"`
	SimpleStorage      Link    `json:"SimpleStorage"`
	EthernetInterfaces Link    `json:"EthernetInterfaces"`
	LogServices        Link    `json:"LogServices"`
	Bios               Link    `json:"Bios"`
	VirtualMedia       Link    `json:"VirtualMedia"`
	PCIeDevices        []Link  `json:"PCIeDevices"`
	Actions            Actions `json:"Actions"`
	Links              struct {
		Chassis   []Link `json:"Chassis"`
		ManagedBy []Link `json:"ManagedBy"`
	} `json:"Links"`
	Oem map[string]json.RawMessage `json:"Oem"`
}

// ---- Manager ----

// Manager is a Managers member.
type Manager struct {
	Resource
	ManagerType         string  `json:"ManagerType"`
	FirmwareVersion     string  `json:"FirmwareVersion"`
	Model               string  `json:"Model"`
	Manufacturer        string  `json:"Manufacturer"`
	SerialNumber        string  `json:"SerialNumber"`
	UUID                string  `json:"UUID"`
	PowerState          string  `json:"PowerState"`
	DateTime            string  `json:"DateTime"`
	DateTimeLocalOffset string  `json:"DateTimeLocalOffset"`
	Status              Status  `json:"Status"`
	NetworkProtocol     Link    `json:"NetworkProtocol"`
	EthernetInterfaces  Link    `json:"EthernetInterfaces"`
	LogServices         Link    `json:"LogServices"`
	VirtualMedia        Link    `json:"VirtualMedia"`
	Actions             Actions `json:"Actions"`
	Links               struct {
		ManagerForServers []Link `json:"ManagerForServers"`
		ManagerForChassis []Link `json:"ManagerForChassis"`
		ManagerInChassis  Link   `json:"ManagerInChassis"`
	} `json:"Links"`
	Oem map[string]json.RawMessage `json:"Oem"`
}

// ---- Chassis ----

// Chassis is a Chassis member.
type Chassis struct {
	Resource
	ChassisType             string                     `json:"ChassisType"`
	Manufacturer            string                     `json:"Manufacturer"`
	Model                   string                     `json:"Model"`
	SKU                     string                     `json:"SKU"`
	SerialNumber            string                     `json:"SerialNumber"`
	PartNumber              string                     `json:"PartNumber"`
	AssetTag                string                     `json:"AssetTag"`
	Version                 string                     `json:"Version"`
	PowerState              string                     `json:"PowerState"`
	IndicatorLED            *string                    `json:"IndicatorLED"`
	IndicatorLEDAllowable   []string                   `json:"IndicatorLED@Redfish.AllowableValues"`
	LocationIndicatorActive *bool                      `json:"LocationIndicatorActive"`
	Status                  Status                     `json:"Status"`
	Thermal                 Link                       `json:"Thermal"`
	Power                   Link                       `json:"Power"`
	Sensors                 Link                       `json:"Sensors"`
	PowerSubsystem          Link                       `json:"PowerSubsystem"`
	ThermalSubsystem        Link                       `json:"ThermalSubsystem"`
	PCIeDevices             Link                       `json:"PCIeDevices"`
	LogServices             Link                       `json:"LogServices"`
	Assembly                Link                       `json:"Assembly"`
	Actions                 Actions                    `json:"Actions"`
	Oem                     map[string]json.RawMessage `json:"Oem"`
}

// ---- Sensors ----

// Threshold is one Sensor.Thresholds entry.
type Threshold struct {
	Reading *float64 `json:"Reading"`
}

// SensorThresholds is Sensor.Thresholds.
type SensorThresholds struct {
	LowerCaution  *Threshold `json:"LowerCaution"`
	LowerCritical *Threshold `json:"LowerCritical"`
	LowerFatal    *Threshold `json:"LowerFatal"`
	UpperCaution  *Threshold `json:"UpperCaution"`
	UpperCritical *Threshold `json:"UpperCritical"`
	UpperFatal    *Threshold `json:"UpperFatal"`
}

// Sensor is a #Sensor resource (Chassis/{id}/Sensors/{id}).
type Sensor struct {
	Resource
	ReadingType     string                     `json:"ReadingType"`
	Reading         *float64                   `json:"Reading"`
	ReadingUnits    string                     `json:"ReadingUnits"`
	PhysicalContext string                     `json:"PhysicalContext"`
	Status          Status                     `json:"Status"`
	Thresholds      SensorThresholds           `json:"Thresholds"`
	SensorNumber    *int                       `json:"SensorNumber"`
	Oem             map[string]json.RawMessage `json:"Oem"`
}

// ThermalTemperature is one Thermal.Temperatures entry.
type ThermalTemperature struct {
	MemberID                  string   `json:"MemberId"`
	Name                      string   `json:"Name"`
	SensorNumber              *int     `json:"SensorNumber"`
	ReadingCelsius            *float64 `json:"ReadingCelsius"`
	UpperThresholdNonCritical *float64 `json:"UpperThresholdNonCritical"`
	UpperThresholdCritical    *float64 `json:"UpperThresholdCritical"`
	UpperThresholdFatal       *float64 `json:"UpperThresholdFatal"`
	LowerThresholdNonCritical *float64 `json:"LowerThresholdNonCritical"`
	LowerThresholdCritical    *float64 `json:"LowerThresholdCritical"`
	LowerThresholdFatal       *float64 `json:"LowerThresholdFatal"`
	PhysicalContext           string   `json:"PhysicalContext"`
	Status                    Status   `json:"Status"`
}

// ThermalFan is one Thermal.Fans entry.
type ThermalFan struct {
	MemberID                  string   `json:"MemberId"`
	Name                      string   `json:"Name"`
	SensorNumber              *int     `json:"SensorNumber"`
	Reading                   *float64 `json:"Reading"`
	ReadingUnits              string   `json:"ReadingUnits"`
	UpperThresholdNonCritical *float64 `json:"UpperThresholdNonCritical"`
	UpperThresholdCritical    *float64 `json:"UpperThresholdCritical"`
	UpperThresholdFatal       *float64 `json:"UpperThresholdFatal"`
	LowerThresholdNonCritical *float64 `json:"LowerThresholdNonCritical"`
	LowerThresholdCritical    *float64 `json:"LowerThresholdCritical"`
	LowerThresholdFatal       *float64 `json:"LowerThresholdFatal"`
	Status                    Status   `json:"Status"`
}

// Thermal is the legacy Chassis/{id}/Thermal resource.
type Thermal struct {
	Resource
	Temperatures []ThermalTemperature `json:"Temperatures"`
	Fans         []ThermalFan         `json:"Fans"`
}

// PowerVoltage is one Power.Voltages entry.
type PowerVoltage struct {
	MemberID                  string   `json:"MemberId"`
	Name                      string   `json:"Name"`
	SensorNumber              *int     `json:"SensorNumber"`
	ReadingVolts              *float64 `json:"ReadingVolts"`
	UpperThresholdNonCritical *float64 `json:"UpperThresholdNonCritical"`
	UpperThresholdCritical    *float64 `json:"UpperThresholdCritical"`
	UpperThresholdFatal       *float64 `json:"UpperThresholdFatal"`
	LowerThresholdNonCritical *float64 `json:"LowerThresholdNonCritical"`
	LowerThresholdCritical    *float64 `json:"LowerThresholdCritical"`
	LowerThresholdFatal       *float64 `json:"LowerThresholdFatal"`
	Status                    Status   `json:"Status"`
}

// PowerControl is one Power.PowerControl entry.
type PowerControl struct {
	MemberID           string   `json:"MemberId"`
	Name               string   `json:"Name"`
	PowerConsumedWatts *float64 `json:"PowerConsumedWatts"`
	Status             Status   `json:"Status"`
}

// PowerSupply is one Power.PowerSupplies entry.
type PowerSupply struct {
	MemberID           string   `json:"MemberId"`
	Name               string   `json:"Name"`
	Manufacturer       string   `json:"Manufacturer"`
	Model              string   `json:"Model"`
	SerialNumber       string   `json:"SerialNumber"`
	PartNumber         string   `json:"PartNumber"`
	FirmwareVersion    string   `json:"FirmwareVersion"`
	PowerCapacityWatts *float64 `json:"PowerCapacityWatts"`
	LastPowerOutput    *float64 `json:"LastPowerOutputWatts"`
	Status             Status   `json:"Status"`
}

// Power is the legacy Chassis/{id}/Power resource.
type Power struct {
	Resource
	Voltages      []PowerVoltage `json:"Voltages"`
	PowerControl  []PowerControl `json:"PowerControl"`
	PowerSupplies []PowerSupply  `json:"PowerSupplies"`
}

// ---- logs ----

// LogService is a LogServices member.
type LogService struct {
	Resource
	LogEntryType       string  `json:"LogEntryType"`
	MaxNumberOfRecords *int    `json:"MaxNumberOfRecords"`
	OverWritePolicy    string  `json:"OverWritePolicy"`
	ServiceEnabled     *bool   `json:"ServiceEnabled"`
	DateTime           string  `json:"DateTime"`
	Entries            Link    `json:"Entries"`
	Actions            Actions `json:"Actions"`
	Status             Status  `json:"Status"`
}

// LogEntry is a LogEntries member.
type LogEntry struct {
	Resource
	Created           string                     `json:"Created"`
	EventTimestamp    string                     `json:"EventTimestamp"`
	EntryType         string                     `json:"EntryType"`
	EntryCode         string                     `json:"EntryCode"`
	Severity          string                     `json:"Severity"`
	Message           string                     `json:"Message"`
	MessageID         string                     `json:"MessageId"`
	MessageArgs       []string                   `json:"MessageArgs"`
	SensorType        string                     `json:"SensorType"`
	SensorNumber      *int                       `json:"SensorNumber"`
	EventType         string                     `json:"EventType"`
	EventID           string                     `json:"EventId"`
	GeneratorID       string                     `json:"GeneratorId"`
	OemRecordFormat   string                     `json:"OemRecordFormat"`
	OriginOfCondition Link                       `json:"OriginOfCondition"`
	Oem               map[string]json.RawMessage `json:"Oem"`
}

// ---- inventory ----

// Processor is a Processors member.
type Processor struct {
	Resource
	Socket                string   `json:"Socket"`
	ProcessorType         string   `json:"ProcessorType"`
	Manufacturer          string   `json:"Manufacturer"`
	Model                 string   `json:"Model"`
	TotalCores            *int     `json:"TotalCores"`
	TotalThreads          *int     `json:"TotalThreads"`
	MaxSpeedMHz           *float64 `json:"MaxSpeedMHz"`
	Status                Status   `json:"Status"`
	ProcessorArchitecture string   `json:"ProcessorArchitecture"`
}

// Memory is a Memory member (a DIMM).
type Memory struct {
	Resource
	DeviceLocator     string   `json:"DeviceLocator"`
	Manufacturer      string   `json:"Manufacturer"`
	PartNumber        string   `json:"PartNumber"`
	SerialNumber      string   `json:"SerialNumber"`
	CapacityMiB       *float64 `json:"CapacityMiB"`
	OperatingSpeedMhz *float64 `json:"OperatingSpeedMhz"`
	MemoryDeviceType  string   `json:"MemoryDeviceType"`
	MemoryLocation    struct {
		Slot   *int `json:"Slot"`
		Socket *int `json:"Socket"`
	} `json:"MemoryLocation"`
	Status Status `json:"Status"`
}

// PCIeDevice is a PCIeDevices member.
type PCIeDevice struct {
	Resource
	Manufacturer    string `json:"Manufacturer"`
	Model           string `json:"Model"`
	DeviceType      string `json:"DeviceType"`
	SerialNumber    string `json:"SerialNumber"`
	PartNumber      string `json:"PartNumber"`
	FirmwareVersion string `json:"FirmwareVersion"`
	PCIeInterface   struct {
		LanesInUse *int   `json:"LanesInUse"`
		MaxLanes   *int   `json:"MaxLanes"`
		PCIeType   string `json:"PCIeType"`
	} `json:"PCIeInterface"`
	Slot struct {
		Location struct {
			PartLocation struct {
				ServiceLabel string `json:"ServiceLabel"`
			} `json:"PartLocation"`
		} `json:"Location"`
	} `json:"Slot"`
	PCIeFunctions Link   `json:"PCIeFunctions"`
	Status        Status `json:"Status"`
}

// PCIeFunction is a PCIeFunctions member.
type PCIeFunction struct {
	Resource
	VendorID          string `json:"VendorId"`
	DeviceID          string `json:"DeviceId"`
	SubsystemVendorID string `json:"SubsystemVendorId"`
	SubsystemID       string `json:"SubsystemId"`
	ClassCode         string `json:"ClassCode"`
	DeviceClass       string `json:"DeviceClass"`
}

// Storage is a Storage member.
type Storage struct {
	Resource
	Drives []Link `json:"Drives"`
	Status Status `json:"Status"`
}

// Drive is a #Drive resource.
type Drive struct {
	Resource
	Manufacturer  string   `json:"Manufacturer"`
	Model         string   `json:"Model"`
	SerialNumber  string   `json:"SerialNumber"`
	Revision      string   `json:"Revision"`
	CapacityBytes *float64 `json:"CapacityBytes"`
	MediaType     string   `json:"MediaType"`
	Protocol      string   `json:"Protocol"`
	Status        Status   `json:"Status"`
}

// SimpleStorage is a SimpleStorage member.
type SimpleStorage struct {
	Resource
	Devices []struct {
		Name          string   `json:"Name"`
		Manufacturer  string   `json:"Manufacturer"`
		Model         string   `json:"Model"`
		CapacityBytes *float64 `json:"CapacityBytes"`
		Status        Status   `json:"Status"`
	} `json:"Devices"`
}

// ---- network ----

// IPv4Address is one IPv4Addresses entry.
type IPv4Address struct {
	Address       string `json:"Address,omitempty"`
	SubnetMask    string `json:"SubnetMask,omitempty"`
	Gateway       string `json:"Gateway,omitempty"`
	AddressOrigin string `json:"AddressOrigin,omitempty"`
}

// IPv6Address is one IPv6Addresses entry.
type IPv6Address struct {
	Address       string `json:"Address"`
	PrefixLength  *int   `json:"PrefixLength"`
	AddressOrigin string `json:"AddressOrigin"`
}

// EthernetInterface is an EthernetInterfaces member.
type EthernetInterface struct {
	Resource
	InterfaceEnabled    *bool  `json:"InterfaceEnabled"`
	LinkStatus          string `json:"LinkStatus"`
	SpeedMbps           *int   `json:"SpeedMbps"`
	MTUSize             *int   `json:"MTUSize"`
	MACAddress          string `json:"MACAddress"`
	PermanentMACAddress string `json:"PermanentMACAddress"`
	HostName            string `json:"HostName"`
	FQDN                string `json:"FQDN"`
	DHCPv4              *struct {
		DHCPEnabled *bool `json:"DHCPEnabled"`
	} `json:"DHCPv4"`
	DHCPv6 *struct {
		OperatingMode string `json:"OperatingMode"`
	} `json:"DHCPv6"`
	IPv4Addresses       []IPv4Address `json:"IPv4Addresses"`
	IPv4StaticAddresses []IPv4Address `json:"IPv4StaticAddresses"`
	IPv6Addresses       []IPv6Address `json:"IPv6Addresses"`
	IPv6DefaultGateway  string        `json:"IPv6DefaultGateway"`
	StaticNameServers   []*string     `json:"StaticNameServers"`
	NameServers         []string      `json:"NameServers"`
	VLAN                *struct {
		VLANEnable *bool `json:"VLANEnable"`
		VLANID     *int  `json:"VLANId"`
	} `json:"VLAN"`
	Status Status                     `json:"Status"`
	Oem    map[string]json.RawMessage `json:"Oem"`
}

// ProtocolSetting is one NetworkProtocol service entry.
type ProtocolSetting struct {
	ProtocolEnabled *bool `json:"ProtocolEnabled"`
	Port            *int  `json:"Port"`
}

// NTPSetting is NetworkProtocol.NTP.
type NTPSetting struct {
	ProtocolEnabled *bool     `json:"ProtocolEnabled"`
	Port            *int      `json:"Port"`
	NTPServers      []*string `json:"NTPServers"`
}

// NetworkProtocol is Managers/{id}/NetworkProtocol. Services holds every
// member that looks like a ProtocolSetting (HTTP, HTTPS, SSH, IPMI, KVMIP,
// VirtualMedia, SNMP, NTP, ...), keyed by property name.
type NetworkProtocol struct {
	Resource
	HostName string                     `json:"HostName"`
	FQDN     string                     `json:"FQDN"`
	NTP      *NTPSetting                `json:"NTP"`
	Services map[string]ProtocolSetting `json:"-"`
}

// UnmarshalJSON fills Services generically.
func (np *NetworkProtocol) UnmarshalJSON(b []byte) error {
	type plain NetworkProtocol
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	p.Services = map[string]ProtocolSetting{}
	for k, v := range m {
		if len(v) == 0 || v[0] != '{' || strings.HasPrefix(k, "@") || k == "Oem" || k == "Status" || k == "Links" {
			continue
		}
		var probe map[string]json.RawMessage
		if json.Unmarshal(v, &probe) != nil {
			continue
		}
		if _, ok := probe["ProtocolEnabled"]; !ok {
			if _, ok := probe["Port"]; !ok {
				continue
			}
		}
		var ps ProtocolSetting
		_ = json.Unmarshal(v, &ps)
		p.Services[k] = ps
	}
	*np = NetworkProtocol(p)
	return nil
}

// ---- accounts ----

// AccountService is the AccountService resource.
type AccountService struct {
	Resource
	Accounts Link `json:"Accounts"`
	Roles    Link `json:"Roles"`
}

// ManagerAccount is an Accounts member. Password is always null on read
// and is deliberately not declared.
type ManagerAccount struct {
	Resource
	UserName    string `json:"UserName"`
	RoleID      string `json:"RoleId"`
	Enabled     *bool  `json:"Enabled"`
	Locked      *bool  `json:"Locked"`
	Description string `json:"Description"`
}

// ---- update ----

// UpdateService is the UpdateService resource.
type UpdateService struct {
	Resource
	ServiceEnabled       *bool   `json:"ServiceEnabled"`
	HttpPushURI          string  `json:"HttpPushUri"`
	MultipartHttpPushURI string  `json:"MultipartHttpPushUri"`
	FirmwareInventory    Link    `json:"FirmwareInventory"`
	SoftwareInventory    Link    `json:"SoftwareInventory"`
	Actions              Actions `json:"Actions"`
}

// SoftwareInventory is a FirmwareInventory member.
type SoftwareInventory struct {
	Resource
	Version     string `json:"Version"`
	Updateable  *bool  `json:"Updateable"`
	ReleaseDate string `json:"ReleaseDate"`
	SoftwareID  string `json:"SoftwareId"`
	Status      Status `json:"Status"`
}

// ---- virtual media ----

// VirtualMedia is a VirtualMedia member.
type VirtualMedia struct {
	Resource
	MediaTypes           []string `json:"MediaTypes"`
	Image                string   `json:"Image"`
	ImageName            string   `json:"ImageName"`
	Inserted             *bool    `json:"Inserted"`
	WriteProtected       *bool    `json:"WriteProtected"`
	ConnectedVia         string   `json:"ConnectedVia"`
	TransferProtocolType string   `json:"TransferProtocolType"`
	Actions              Actions  `json:"Actions"`
}

// ---- tasks ----

// Task is a TaskService Tasks member.
type Task struct {
	Resource
	TaskState       string        `json:"TaskState"`
	TaskStatus      string        `json:"TaskStatus"`
	PercentComplete *int          `json:"PercentComplete"`
	StartTime       string        `json:"StartTime"`
	EndTime         string        `json:"EndTime"`
	TaskMonitor     string        `json:"TaskMonitor"`
	Messages        []MessageInfo `json:"Messages"`
}

// Terminal reports whether TaskState is final.
func (t *Task) Terminal() bool {
	switch t.TaskState {
	case "Completed", "Killed", "Exception", "Cancelled", "Canceled":
		return true
	}
	return false
}

// Succeeded reports a final, successful task.
func (t *Task) Succeeded() bool {
	return t.TaskState == "Completed" && (t.TaskStatus == "" || t.TaskStatus == "OK" || t.TaskStatus == "Warning")
}
