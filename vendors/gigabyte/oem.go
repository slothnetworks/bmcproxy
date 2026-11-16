package gigabyte

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
)

// Gigabyte OEM fan-profile support.
//
// Evidence (docs/gigabyte/mc13-le0-live-probe.md §OEM, firmware 13.06.30
// web UI models/oem/fanprofile/*):
//   - Redfish GET Chassis/{id}/Thermal -> Oem.Gbt.FanprofileService ->
//     Fanprofile {strMode, strVersion, arrProfile[]} (Allow GET,PUT) and
//     SupportPCIEDevice {arrPCIEDevices[]} (Allow GET,PUT);
//   - AMI /api GET settings/fanprofile/mode -> {"strMode": "..."}; the UI
//     "plays" a profile with POST {"strMode": <profile strName>} and stops
//     it with POST {"strMode": "default"} (Backbone save of a new model).

// FanPolicy is one temperature-to-duty policy of a fan profile.
type FanPolicy struct {
	// PolicyType is the GBT iPolicyType (2 = linear between the two
	// reference points on the live board; 1 is the UI default).
	PolicyType int `json:"policyType" example:"2"`
	// Sensors are the IPMI sensor numbers the policy follows (arrSensor).
	Sensors []int `json:"sensors" example:"1"`
	// FanSensors are the IPMI sensor numbers of the fans it drives.
	FanSensors []int `json:"fanSensors" example:"160,162"`
	// ReferenceTemps are the temperature points in °C (arrRef).
	ReferenceTemps []int `json:"referenceTemps" example:"70,85"`
	// Duties are the fan duties in % at the reference points (arrDuty).
	Duties []int `json:"duties" example:"30,100"`
	// InitDuty is the starting duty in % (iInitDuty).
	InitDuty int `json:"initDuty" example:"30"`
	// Hysteresis in °C (iHysteresis).
	Hysteresis int `json:"hysteresis" example:"0"`
	// InSDR: the sensor is an SDR sensor (iInSDR).
	InSDR int `json:"inSdr" example:"1"`
	// SensorCode is the GBT iSensorCode.
	SensorCode int `json:"sensorCode" example:"1"`
	// CPUTDP is the iCpuTdp selector (0 = any).
	CPUTDP int `json:"cpuTdp" example:"0"`
	// AmbientSensor / AmbientSensorTemp are the ambient compensation inputs.
	AmbientSensor     int `json:"ambientSensor" example:"0"`
	AmbientSensorTemp int `json:"ambientSensorTemp" example:"0"`
	// PCIeDeviceEnable makes the policy apply only when a matching PCIe
	// device (PCIeVendorIDs/PCIeDeviceIDs) is present.
	PCIeDeviceEnable int      `json:"pcieDeviceEnable" example:"0"`
	PCIeVendorIDs    []string `json:"pcieVendorIds"`
	PCIeDeviceIDs    []string `json:"pcieDeviceIds"`
}

// FanProfile is one named fan profile.
type FanProfile struct {
	// Name is the profile name (strName); the mode selects one by name.
	Name string `json:"name" example:"default"`
	// Version is strVersion.
	Version string `json:"version" example:"1.00"`
	// Active is true when the BMC's current mode is this profile.
	Active   bool        `json:"active" example:"false"`
	Policies []FanPolicy `json:"policies"`
}

// FanProfiles is the Gigabyte fan-profile service state.
type FanProfiles struct {
	// Mode is the active profile name (strMode); "default" = factory curve.
	Mode string `json:"mode" example:"NEW_PROFILE"`
	// Version is the service strVersion.
	Version  string       `json:"version" example:"1.00"`
	Profiles []FanProfile `json:"profiles"`
	// Source is the backend that served the data.
	Source string `json:"source" example:"redfish"`
}

// FanProfileDevice is one PCIe device the fan profiles can key on, with its
// temperature limits.
type FanProfileDevice struct {
	Name     string `json:"name" example:"P100_16GB"`
	VendorID string `json:"vendorId" example:"0x10de"`
	DeviceID string `json:"deviceId" example:"0x15f8"`
	// UpperNonCritical / UpperCritical are °C limits (UNC/UC).
	UpperNonCritical int `json:"upperNonCritical" example:"80"`
	UpperCritical    int `json:"upperCritical" example:"85"`
}

// FanProfileDevices lists the supported PCIe devices.
type FanProfileDevices struct {
	Devices []FanProfileDevice `json:"devices"`
}

// FanMode is the active fan-profile mode.
type FanMode struct {
	// Mode is the active profile name; "default" = factory curve.
	Mode string `json:"mode" example:"default"`
	// Source is the backend that served it ("ami" or "redfish").
	Source string `json:"source" example:"ami"`
}

// SetFanModeRequest selects the active fan profile.
type SetFanModeRequest struct {
	// Mode is a profile name from GET fanProfiles, or "default" to stop the
	// custom profile and return to the factory curve.
	Mode string `json:"mode" example:"default"`
}

// wire shapes

type gbtPolicy struct {
	ArrDuty            []int    `json:"arrDuty"`
	ArrFanSensor       []int    `json:"arrFanSensor"`
	ArrHexDeviceID     []string `json:"arrHexDeviceID"`
	ArrHexVendorID     []string `json:"arrHexVendorID"`
	ArrRef             []int    `json:"arrRef"`
	ArrSensor          []int    `json:"arrSensor"`
	IAmbientSensor     int      `json:"iAmbientSensor"`
	IAmbientSensorTemp int      `json:"iAmbientSensorTemp"`
	ICPUTDP            int      `json:"iCpuTdp"`
	IHysteresis        int      `json:"iHysteresis"`
	IInSDR             int      `json:"iInSDR"`
	IInitDuty          int      `json:"iInitDuty"`
	IPCIEDeviceEnable  int      `json:"iPCIEDeviceEnable"`
	IPolicyType        int      `json:"iPolicyType"`
	ISensorCode        int      `json:"iSensorCode"`
}

type gbtProfile struct {
	StrName    string      `json:"strName"`
	StrVersion string      `json:"strVersion"`
	ArrPolicy  []gbtPolicy `json:"arrPolicy"`
}

type gbtFanprofile struct {
	StrMode    string       `json:"strMode"`
	StrVersion string       `json:"strVersion"`
	ArrProfile []gbtProfile `json:"arrProfile"`
}

type gbtDevice struct {
	StrName     string `json:"strName"`
	HexVendorID string `json:"hexVendorID"`
	HexDeviceID string `json:"hexDeviceID"`
	UNC         int    `json:"UNC"`
	UC          int    `json:"UC"`
}

func (p gbtPolicy) norm() FanPolicy {
	return FanPolicy{PolicyType: p.IPolicyType, Sensors: nz(p.ArrSensor), FanSensors: nz(p.ArrFanSensor),
		ReferenceTemps: nz(p.ArrRef), Duties: nz(p.ArrDuty), InitDuty: p.IInitDuty, Hysteresis: p.IHysteresis,
		InSDR: p.IInSDR, SensorCode: p.ISensorCode, CPUTDP: p.ICPUTDP, AmbientSensor: p.IAmbientSensor,
		AmbientSensorTemp: p.IAmbientSensorTemp, PCIeDeviceEnable: p.IPCIEDeviceEnable,
		PCIeVendorIDs: nzs(p.ArrHexVendorID), PCIeDeviceIDs: nzs(p.ArrHexDeviceID)}
}

func nz(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}

func nzs(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// NormalizeFanProfiles converts the GBT wire shape.
func normalizeFanProfiles(f gbtFanprofile, source string) FanProfiles {
	out := FanProfiles{Mode: f.StrMode, Version: f.StrVersion, Source: source, Profiles: []FanProfile{}}
	for _, p := range f.ArrProfile {
		fp := FanProfile{Name: p.StrName, Version: p.StrVersion, Active: p.StrName == f.StrMode, Policies: []FanPolicy{}}
		for _, pol := range p.ArrPolicy {
			fp.Policies = append(fp.Policies, pol.norm())
		}
		out.Profiles = append(out.Profiles, fp)
	}
	return out
}

// fanService returns the GBT FanprofileService path from the chassis
// Thermal resource (Oem.Gbt.FanprofileService).
func (s *Session) fanService(ctx context.Context) (string, error) {
	if !s.prof.FanProfile {
		return "", bmc.Unsupported(Pkg, "FanprofileService")
	}
	_, _, chassis, _, _ := s.Paths()
	if chassis == "" {
		return "", bmc.Unsupported(Pkg, "FanprofileService (no chassis)")
	}
	var ch struct {
		Thermal rf.Link `json:"Thermal"`
	}
	if _, err := s.C.Get(ctx, chassis, &ch); err != nil {
		return "", err
	}
	if ch.Thermal.ID == "" {
		return "", bmc.Unsupported(Pkg, "FanprofileService (no Thermal)")
	}
	var th struct {
		Oem map[string]struct {
			FanprofileService rf.Link `json:"FanprofileService"`
		} `json:"Oem"`
	}
	if _, err := s.C.Get(ctx, ch.Thermal.ID, &th); err != nil {
		return "", err
	}
	for k, v := range th.Oem {
		if strings.EqualFold(k, "Gbt") && v.FanprofileService.ID != "" {
			return v.FanprofileService.ID, nil
		}
	}
	return "", bmc.Unsupported(Pkg, "FanprofileService (Thermal has no Oem.Gbt.FanprofileService)")
}

func (s *Session) fanServiceMember(ctx context.Context, member string) (string, error) {
	svc, err := s.fanService(ctx)
	if err != nil {
		return "", err
	}
	var fs map[string]any
	if _, err := s.C.Get(ctx, svc, &fs); err != nil {
		return "", err
	}
	if l, ok := fs[member].(map[string]any); ok {
		if id, ok := l["@odata.id"].(string); ok && id != "" {
			return id, nil
		}
	}
	return "", bmc.Unsupported(Pkg, "FanprofileService."+member)
}

// FanProfiles reads the fan profiles over Redfish (GBT OEM).
func (s *Session) FanProfiles(ctx context.Context) (FanProfiles, error) {
	p, err := s.fanServiceMember(ctx, "Fanprofile")
	if err != nil {
		return FanProfiles{}, err
	}
	var f gbtFanprofile
	if _, err := s.C.Get(ctx, p, &f); err != nil {
		return FanProfiles{}, err
	}
	return normalizeFanProfiles(f, bmc.BackendRedfish), nil
}

// FanProfileDevices reads the PCIe device table of the fan profiles.
func (s *Session) FanProfileDevices(ctx context.Context) (FanProfileDevices, error) {
	p, err := s.fanServiceMember(ctx, "SupportPCIEDevice")
	if err != nil {
		return FanProfileDevices{}, err
	}
	var w struct {
		ArrPCIEDevices []gbtDevice `json:"arrPCIEDevices"`
	}
	if _, err := s.C.Get(ctx, p, &w); err != nil {
		return FanProfileDevices{}, err
	}
	out := FanProfileDevices{Devices: make([]FanProfileDevice, 0, len(w.ArrPCIEDevices))}
	for _, d := range w.ArrPCIEDevices {
		out.Devices = append(out.Devices, FanProfileDevice{Name: d.StrName, VendorID: d.HexVendorID, DeviceID: d.HexDeviceID,
			UpperNonCritical: d.UNC, UpperCritical: d.UC})
	}
	return out, nil
}

const apiFanMode = "settings/fanprofile/mode"

// FanMode reads the active fan-profile mode: AMI /api when the board has
// it, else the strMode of the Redfish Fanprofile.
func (s *Session) FanMode(ctx context.Context) (FanMode, error) {
	if s.ami != nil {
		var m struct {
			StrMode string `json:"strMode"`
		}
		if err := s.ami.Get(ctx, apiFanMode, &m); err == nil {
			return FanMode{Mode: m.StrMode, Source: bmc.BackendAMI}, nil
		} else if !errors.Is(err, bmc.ErrUnsupported) && !errors.Is(err, bmc.ErrNotFound) {
			return FanMode{}, err
		}
	}
	fp, err := s.FanProfiles(ctx)
	if err != nil {
		return FanMode{}, err
	}
	return FanMode{Mode: fp.Mode, Source: bmc.BackendRedfish}, nil
}

// SetFanMode activates a fan profile by name ("default" stops the custom
// profile), then reads the mode back.
//
// UNVERIFIED: the POST body comes from the 13.06.30 web UI
// (FanProfileModeModel.save); never exercised on hardware.
func (s *Session) SetFanMode(ctx context.Context, mode string) (FanMode, error) {
	mode = strings.TrimSpace(mode)
	if mode == "" || len(mode) > 64 {
		return FanMode{}, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetFanMode", "mode must be a profile name (1..64 chars)")
	}
	if s.ami == nil {
		return FanMode{}, bmc.Unsupported(Pkg, "SetFanMode (no AMI /api on "+s.prof.Key+")")
	}
	if mode != "default" {
		fp, err := s.FanProfiles(ctx)
		if err != nil {
			return FanMode{}, err
		}
		found := false
		var names []string
		for _, p := range fp.Profiles {
			names = append(names, p.Name)
			if p.Name == mode {
				found = true
			}
		}
		if !found {
			return FanMode{}, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetFanMode", "no fan profile %q (have: default, %s)", mode, strings.Join(names, ", "))
		}
	}
	if err := s.ami.DoJSON(ctx, http.MethodPost, apiFanMode, map[string]string{"strMode": mode}, nil); err != nil {
		return FanMode{}, err
	}
	got, err := s.FanMode(ctx)
	if err != nil {
		return FanMode{}, err
	}
	if got.Mode != mode {
		return got, &bmc.OpError{Pkg: Pkg, Op: "SetFanMode", Backend: bmc.BackendAMI, Err: bmc.ErrUpstream,
			Detail: "BMC accepted the fan mode but still reports " + got.Mode}
	}
	return got, nil
}
