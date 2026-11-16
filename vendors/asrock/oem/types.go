package oem

// Request and response bodies of the /api/v1/vendor/asrock/* routes. Field
// comments and example tags feed the generated OpenAPI document.

// BondingStatus is GET /api/v1/vendor/asrock/bonding (AMI settings/network-bond).
type BondingStatus struct {
	// Bond id on the BMC (always 1 on known boards).
	ID int `json:"id" example:"1"`
	// Whether NIC bonding is enabled.
	Enabled bool `json:"enabled" example:"false"`
	// Linux bonding mode.
	Mode string `json:"mode" example:"active-backup"`
	// Whether the bond configures itself automatically.
	AutoConfig bool `json:"autoConfig" example:"true"`
	// Interface reported by the BMC (B650 bond_ifc / interface_name), if any.
	Interface string `json:"interface,omitempty" example:"eth0"`
	// Raw is the BMC object as returned.
	Raw map[string]any `json:"raw,omitempty"`
} //@name AsrockBondingStatus

// EnableBondingRequest is POST /api/v1/vendor/asrock/enableBonding.
type EnableBondingRequest struct {
	// Bonding mode (default: the BMC's current mode, normally active-backup).
	Mode string `json:"mode,omitempty" example:"active-backup"`
	// Primary interface (B650 bond_ifc; ignored on X470, which has no such field).
	Interface string `json:"interface,omitempty" example:"eth0"`
} //@name AsrockEnableBondingRequest

// DisableBondingRequest is POST /api/v1/vendor/asrock/disableBonding.
type DisableBondingRequest struct {
	// Interface that keeps the BMC address after un-bonding (B650 bond_ifc; required there).
	Interface string `json:"interface,omitempty" example:"eth0"`
} //@name AsrockDisableBondingRequest

// NCSIMode is GET /api/v1/vendor/asrock/ncsiMode (AMI settings/ncsi/mode).
type NCSIMode struct {
	// NC-SI interface name.
	InterfaceName string `json:"interfaceName,omitempty" example:"eth1"`
	// NC-SI channel number.
	ChannelNumber int `json:"channelNumber" example:"0"`
	// NC-SI package id.
	PackageID int `json:"packageId" example:"0"`
	// Mode value as reported (AMI: 0 = auto failover, 1 = manual; UNVERIFIED on hardware).
	Mode *int `json:"mode,omitempty" example:"0"`
	// Active channel (B650 only).
	Active *int `json:"active,omitempty" example:"1"`
	// Raw is the BMC object as returned.
	Raw map[string]any `json:"raw,omitempty"`
} //@name AsrockNCSIMode

// SetNCSIModeRequest is POST /api/v1/vendor/asrock/setNcsiMode. Unset fields
// keep the current value (the driver reads, merges and writes back).
type SetNCSIModeRequest struct {
	// NC-SI interface name.
	InterfaceName *string `json:"interfaceName,omitempty" example:"eth1"`
	// Mode value (AMI: 0 = auto failover, 1 = manual; UNVERIFIED).
	Mode *int `json:"mode,omitempty" example:"1"`
	// NC-SI channel number (manual mode).
	ChannelNumber *int `json:"channelNumber,omitempty" example:"0"`
	// NC-SI package id (manual mode).
	PackageID *int `json:"packageId,omitempty" example:"0"`
} //@name AsrockSetNCSIModeRequest

// KeepNicLinkUp is GET /api/v1/vendor/asrock/keepNicLinkUp.
type KeepNicLinkUp struct {
	// Whether the shared NIC link is kept up while the host is off.
	Enabled bool `json:"enabled" example:"true"`
	// Backend dialect: "vetobit" (X470 asrr/getvetobit) or "ncsilinkup" (B650 settings/ncsilinkup).
	Dialect string `json:"dialect" example:"vetobit"`
	// B650: shared NIC 1 keep-link-up flag.
	SharedNIC *bool `json:"sharedNic,omitempty" example:"true"`
	// B650: shared NIC 2 keep-link-up flag.
	SharedNIC2 *bool `json:"sharedNic2,omitempty" example:"false"`
	// Raw is the BMC object as returned.
	Raw map[string]any `json:"raw,omitempty"`
} //@name AsrockKeepNicLinkUp

// SetKeepNicLinkUpRequest is POST /api/v1/vendor/asrock/setKeepNicLinkUp.
type SetKeepNicLinkUpRequest struct {
	// Keep the link up (X470: vetobit 1/0; B650: sets both shared NIC flags unless given below).
	Enabled *bool `json:"enabled,omitempty" example:"true"`
	// B650 only: shared NIC 1 flag (overrides enabled).
	SharedNIC *bool `json:"sharedNic,omitempty" example:"true"`
	// B650 only: shared NIC 2 flag (overrides enabled).
	SharedNIC2 *bool `json:"sharedNic2,omitempty" example:"true"`
} //@name AsrockSetKeepNicLinkUpRequest

// FanModeResponse is GET /api/v1/vendor/asrock/fanMode.
type FanModeResponse struct {
	// Backend dialect: "asrr" (X470 asrr/settings/getfancontrolmode) or "settings/fan" (B650 settings/fan/mode).
	Dialect string `json:"dialect" example:"asrr"`
	// X470: per-fan mode value keyed by fan number ("1".."16"), from mode1..mode16.
	Modes map[string]int `json:"modes,omitempty"`
	// Raw is the BMC object as returned (B650: platform, num_of_fan, min_duty, max_duty, Pwm, ...).
	Raw any `json:"raw,omitempty"`
} //@name AsrockFanModeResponse

// SetFanModeRequest is POST /api/v1/vendor/asrock/setFanMode.
type SetFanModeRequest struct {
	// X470: per-fan mode value keyed by fan number ("1".."16"); merged into the current modes and written as mode1..mode16. Values are the BMC's own codes (0 is the factory default; other codes UNVERIFIED).
	Modes map[string]int `json:"modes,omitempty"`
	// B650: fields merged over the current settings/fan/mode object before the PUT.
	Raw map[string]any `json:"raw,omitempty"`
} //@name AsrockSetFanModeRequest

// FanTable is one fan control table.
type FanTable struct {
	// Table type: "open" (temperature -> duty curve) or "closed" (ramp control).
	Type string `json:"type" example:"open"`
	// Table number (X470: selected with set{open,close}looptablenumber).
	Table int `json:"table" example:"1"`
	// Open loop: temperature points (Get_Temp1..24; 255 = unused).
	Temps []int `json:"temps,omitempty"`
	// Open loop: duty points in percent (Get_Duty1..24; 255 = unused).
	Duties []int `json:"duties,omitempty"`
	// Raw is the BMC object as returned.
	Raw any `json:"raw,omitempty"`
} //@name AsrockFanTable

// FanTablesResponse is GET /api/v1/vendor/asrock/fanTables.
type FanTablesResponse struct {
	// Backend dialect: "asrr" or "settings/fan".
	Dialect string `json:"dialect" example:"asrr"`
	// Requested tables, in request order.
	Tables []FanTable `json:"tables"`
} //@name AsrockFanTablesResponse

// SetFanTableRequest is POST /api/v1/vendor/asrock/setFanTable.
type SetFanTableRequest struct {
	// "open" or "closed".
	Type string `json:"type" example:"open" enums:"open,closed"`
	// Table number (X470 table_numble; default 1).
	Table int `json:"table,omitempty" example:"1"`
	// Open loop (X470): temperature points, written as temp1..tempN.
	Temps []int `json:"temps,omitempty"`
	// Open loop (X470): duty points, written as duty1..dutyN.
	Duties []int `json:"duties,omitempty"`
	// Fields sent as-is (X470 closed-loop fields; B650 settings/fan/{openloop,closedloop} object).
	Raw map[string]any `json:"raw,omitempty"`
} //@name AsrockSetFanTableRequest

// FanDutyResponse is GET /api/v1/vendor/asrock/fanDuty.
type FanDutyResponse struct {
	// Backend dialect: "asrr" or "settings/fan".
	Dialect string `json:"dialect" example:"asrr"`
	// Source says what the duties are: "current" (X470 asrr/GetFanDuty) or "manual" (the configured manual-mode duty: X570 asrr/settings/getfandutymanualmode, B650 settings/fan/manual).
	Source string `json:"source,omitempty" example:"current" enums:"current,manual"`
	// Duty in percent keyed by fan number ("1".."16") (X470 GetFanDuty FANn_Duty; X570 getfandutymanualmode dutyN).
	Duties map[string]int `json:"duties,omitempty"`
	// Raw is the BMC object as returned.
	Raw any `json:"raw,omitempty"`
} //@name AsrockFanDutyResponse

// SetManualFanDutyRequest is POST /api/v1/vendor/asrock/setManualFanDuty.
type SetManualFanDutyRequest struct {
	// Duty in percent keyed by fan number ("1".."16"); merged into the current manual duties (X470 duty1..duty16).
	Duties map[string]int `json:"duties,omitempty"`
	// B650: fields merged over the current settings/fan/manual object.
	Raw map[string]any `json:"raw,omitempty"`
} //@name AsrockSetManualFanDutyRequest

// PowerSupply is one entry of GET /api/v1/vendor/asrock/powerSource.
type PowerSupply struct {
	// Index in the BMC list.
	Index int `json:"index" example:"0"`
	// power_supply_status as reported.
	Status int `json:"status" example:"2"`
	// AC input voltage (null = N/A).
	ACInputVoltage *float64 `json:"acInputVoltage" example:"230"`
	// AC input current (null = N/A).
	ACInputCurrent *float64 `json:"acInputCurrent" example:"0.8"`
	// AC input power in W (null = N/A).
	ACInputPower *float64 `json:"acInputPower" example:"180"`
	// 12 V output voltage (null = N/A).
	DC12VOutputVoltage *float64 `json:"dc12vOutputVoltage" example:"12.1"`
	// 12 V output current (null = N/A).
	DC12VOutputCurrent *float64 `json:"dc12vOutputCurrent" example:"14"`
	// 12 V output power in W (null = N/A).
	DC12VOutputPower *float64 `json:"dc12vOutputPower" example:"170"`
	// PSU serial number ("" = N/A).
	Serial string `json:"serial,omitempty" example:"PSU-SERIAL"`
	// Raw is the BMC object as returned (fault flags, temperatures, fans).
	Raw map[string]any `json:"raw,omitempty"`
} //@name AsrockPowerSupply

// PowerSourceResponse is GET /api/v1/vendor/asrock/powerSource.
type PowerSourceResponse struct {
	// PMBus power supplies (values N/A without a PMBus PSU).
	PowerSupplies []PowerSupply `json:"powerSupplies"`
} //@name AsrockPowerSourceResponse

// PostCodeResponse is GET /api/v1/vendor/asrock/postCode.
type PostCodeResponse struct {
	// Endpoint used: "asrr/getbioscode" or "logs/postcode".
	Source string `json:"source" example:"asrr/getbioscode"`
	// poststatus as reported (getbioscode).
	Status *int `json:"status,omitempty" example:"1"`
	// Last POST code in hex (getbioscode postdata).
	Code string `json:"code,omitempty" example:"0x0caa"`
	// Raw is the BMC object as returned.
	Raw any `json:"raw,omitempty"`
} //@name AsrockPostCodeResponse

// MCALogResponse is GET /api/v1/vendor/asrock/mcaLog.
type MCALogResponse struct {
	// Whether an MCA dump is stored (asrr/getmcalog MCADump != 0).
	DumpPresent bool `json:"dumpPresent" example:"false"`
	// MCADump value as reported.
	MCADump int `json:"mcaDump" example:"0"`
} //@name AsrockMCALogResponse

// ProchotResponse is GET /api/v1/vendor/asrock/prochot (X470 only).
type ProchotResponse struct {
	// PROCHOT assertion enabled (asrr/getprochot prochot != 0).
	Enabled bool `json:"enabled" example:"true"`
	// prochot value as reported.
	Value int `json:"value" example:"1"`
} //@name AsrockProchotResponse

// SetProchotRequest is POST /api/v1/vendor/asrock/setProchot.
type SetProchotRequest struct {
	// Enable (true -> prochot 1) or disable (false -> 0).
	Enabled bool `json:"enabled" example:"true"`
} //@name AsrockSetProchotRequest

// WebEnableTimeout is GET /api/v1/vendor/asrock/webEnableTimeout.
type WebEnableTimeout struct {
	// Whether the web session idle timeout is enforced (WebEnableTimeout != 0).
	Enabled bool `json:"enabled" example:"true"`
	// WebEnableTimeout value as reported.
	Value int `json:"value" example:"1"`
} //@name AsrockWebEnableTimeout

// SetWebEnableTimeoutRequest is POST /api/v1/vendor/asrock/setWebEnableTimeout.
type SetWebEnableTimeoutRequest struct {
	// Enforce the web session idle timeout (enable_timeout 1/0).
	Enabled bool `json:"enabled" example:"true"`
} //@name AsrockSetWebEnableTimeoutRequest

// BIOSUpdateOption is GET /api/v1/vendor/asrock/biosUpdateOption.
type BIOSUpdateOption struct {
	// action as reported by asrr/maintenance/BIOS/UpdateOption.
	Action int `json:"action" example:"0"`
} //@name AsrockBIOSUpdateOption

// ComponentStatus is GET /api/v1/vendor/asrock/componentStatus/{component}.
type ComponentStatus struct {
	// Component name.
	Component string `json:"component" example:"BIOS"`
	// asrr/maintenance/{C}/status response code (BIOS: 1/3 completed, 2 failed, 4 in progress).
	Response *int `json:"response,omitempty" example:"3"`
	// Flash progress text (flash-progress "progress"), if available.
	Progress string `json:"progress,omitempty" example:"45%"`
	// Flash action text (flash-progress "action"), if available.
	Action string `json:"action,omitempty" example:"Flashing..."`
	// Raw status object.
	Status any `json:"status,omitempty"`
	// Raw flash-progress object (absent when the board has no handler).
	FlashProgress any `json:"flashProgress,omitempty"`
} //@name AsrockComponentStatus

// ColdResetResult is POST /api/v1/vendor/asrock/bmcColdReset.
type ColdResetResult struct {
	// Always true when the BMC accepted the reset; every BMC session is gone afterwards.
	Accepted bool `json:"accepted" example:"true"`
	// Advice for the caller.
	Message string `json:"message" example:"BMC cold reset requested; the BMC is unreachable for ~1-3 minutes"`
} //@name AsrockColdResetResult
