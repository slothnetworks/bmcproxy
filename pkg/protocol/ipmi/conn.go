package ipmi

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// Conn is the small transport-level interface the Client drives. The
// production implementation wraps github.com/bougou/go-ipmi (lanplus); tests
// use a fake. Implementations need not be safe for concurrent use: the
// Client serialises every call per host.
//
// Errors carrying an IPMI completion code should implement CompletionCoder
// (see CCError) so the Client can log and classify them.
type Conn interface {
	ChassisStatus(ctx context.Context) (ChassisStatus, error)
	ChassisControl(ctx context.Context, c ChassisControl) error
	ChassisIdentify(ctx context.Context, interval uint8, force bool) error
	SetBootDevice(ctx context.Context, dev BootDevice, efi, persist bool) error
	BootFlags(ctx context.Context) (BootFlags, error)
	SetPowerRestorePolicy(ctx context.Context, p RestorePolicyCode) error
	Sensors(ctx context.Context) ([]SensorRecord, error)
	SEL(ctx context.Context) ([]SELRecord, error)
	ClearSEL(ctx context.Context) error
	FRUs(ctx context.Context) ([]bmc.FRU, error)
	DeviceID(ctx context.Context) (DeviceInfo, error)
	ColdReset(ctx context.Context) error
	WarmReset(ctx context.Context) error
	LANConfig(ctx context.Context, channel uint8) (LANConfig, error)
	Users(ctx context.Context, channel uint8) ([]UserRecord, error)
	Close(ctx context.Context) error
}

// Dialer opens a Conn to addr ("host:623").
type Dialer func(ctx context.Context, addr string, creds bmc.Creds, o Options) (Conn, error)

// CompletionCoder is implemented by errors that carry an IPMI completion
// code.
type CompletionCoder interface {
	IPMICompletionCode() uint8
}

// CCError is a non-zero completion code returned by the BMC.
type CCError struct {
	Code uint8
	Desc string
}

func (e *CCError) Error() string {
	if e.Desc != "" {
		return fmt.Sprintf("IPMI completion code 0x%02x (%s)", e.Code, e.Desc)
	}
	return fmt.Sprintf("IPMI completion code 0x%02x", e.Code)
}

// IPMICompletionCode implements CompletionCoder.
func (e *CCError) IPMICompletionCode() uint8 { return e.Code }

// Well-known completion codes.
const (
	CCNodeBusy             = 0xC0
	CCInvalidCommand       = 0xC1
	CCInvalidCommandForLUN = 0xC2
	CCTimeout              = 0xC3
	CCReservationCanceled  = 0xC5
	CCParameterOutOfRange  = 0xC9
	CCDataNotPresent       = 0xCB
	CCInvalidDataField     = 0xCC
	CCInsufficientPriv     = 0xD4
	CCNotSupportedInState  = 0xD5
	CCSubfunctionDisabled  = 0xD6
)

// ChassisControl values (IPMI Chassis Control command, NetFn 0x00 Cmd 0x02).
type ChassisControl uint8

const (
	ControlPowerDown  ChassisControl = 0
	ControlPowerUp    ChassisControl = 1
	ControlPowerCycle ChassisControl = 2
	ControlHardReset  ChassisControl = 3
	ControlNMI        ChassisControl = 4
	ControlSoftOff    ChassisControl = 5
)

// RestorePolicyCode is the chassis power restore policy encoding.
type RestorePolicyCode uint8

const (
	RestoreAlwaysOff RestorePolicyCode = 0
	RestorePrevious  RestorePolicyCode = 1
	RestoreAlwaysOn  RestorePolicyCode = 2
	RestoreUnknown   RestorePolicyCode = 3
)

// ChassisStatus is the decoded Get Chassis Status response.
type ChassisStatus struct {
	PowerOn            bool
	RestorePolicy      RestorePolicyCode
	PowerFault         bool
	Interlock          bool
	Overload           bool
	IdentifySupported  bool
	IdentifyState      uint8 // 0 off, 1 temporary on, 2 indefinite on, 3 reserved
	IntrusionActive    bool
	FrontPanelLockout  bool
	DriveFault         bool
	CoolingFanFault    bool
	LastPowerByCommand bool
	ACFailed           bool
}

// BootDevice is the boot flags device selector.
type BootDevice uint8

const (
	BootNoOverride BootDevice = 0x00
	BootPXE        BootDevice = 0x01
	BootDisk       BootDevice = 0x02
	BootSafeDisk   BootDevice = 0x03
	BootDiag       BootDevice = 0x04
	BootCDROM      BootDevice = 0x05
	BootBIOS       BootDevice = 0x06
	BootRemoteFlop BootDevice = 0x07
	BootRemoteCD   BootDevice = 0x08
	BootRemoteDisk BootDevice = 0x0b
	BootFloppy     BootDevice = 0x0f
)

// BootFlags is the boot flags parameter (#5).
type BootFlags struct {
	Valid   bool
	Persist bool
	EFI     bool
	Device  BootDevice
}

// SensorRecord is one SDR + reading, pre-normalisation.
type SensorRecord struct {
	Number                       uint8
	Name                         string
	TypeName                     string // IPMI sensor type, e.g. "Temperature", "Voltage", "Fan"
	Unit                         string // e.g. "degrees C", "Volts", "RPM"
	Threshold                    bool   // threshold (analog) vs discrete
	Present                      bool   // reading available and sensor present/scanning
	Value                        float64
	Status                       string // "ok", "lnc", "lcr", "lnr", "unc", "ucr", "unr" or discrete text
	States                       []string
	LNR, LCR, LNC, UNC, UCR, UNR *float64
}

// SELRecord is one SEL record, pre-normalisation.
type SELRecord struct {
	ID           uint16
	RecordType   uint8
	Time         time.Time
	GeneratorID  uint16
	SensorType   string
	SensorNumber uint8
	EventType    string // event/reading type description
	Deassertion  bool
	Description  string
	Severity     string // "Info", "OK", "Warning", "Critical", "Degraded", "Non-fatal"
	EventData    [3]byte
	OEM          bool
}

// DeviceInfo is the Get Device ID response ("mc info").
type DeviceInfo struct {
	DeviceID         uint8
	DeviceRevision   uint8
	FirmwareMajor    uint8
	FirmwareMinor    uint8 // BCD-decoded
	IPMIVersion      string
	ManufacturerID   uint32
	ProductID        uint16
	DeviceAvailable  bool
	ProvidesSDRs     bool
	AuxFirmwareBytes []byte
}

// FirmwareVersion renders "major.minor" ("4.10").
func (d DeviceInfo) FirmwareVersion() string {
	return fmt.Sprintf("%d.%02d", d.FirmwareMajor, d.FirmwareMinor)
}

// LANConfig is the subset of LAN configuration parameters ("lan print").
type LANConfig struct {
	Channel  uint8
	IP       net.IP
	Netmask  net.IP
	Gateway  net.IP
	MAC      net.HardwareAddr
	IPSource string // "static", "dhcp", "bios", "other", "unspecified"
	VLANOn   bool
	VLANID   uint16
	RMCPPort uint16
}

// UserRecord is one IPMI user slot.
type UserRecord struct {
	ID        uint8
	Name      string
	Enabled   bool
	Privilege string // "ADMINISTRATOR", "OPERATOR", "USER", "CALLBACK", "NO ACCESS", "OEM"
	Callin    bool
	LinkAuth  bool
	IPMIMsg   bool
}
