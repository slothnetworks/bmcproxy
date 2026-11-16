package ipmi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/bougou/go-ipmi/pkg/client"
	"github.com/bougou/go-ipmi/pkg/command/chassis"
	"github.com/bougou/go-ipmi/pkg/types"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// goipmiConn adapts *client.Client (go-ipmi v0.9.2, lanplus) to Conn.
type goipmiConn struct {
	c *client.Client
}

// DialLanplus is the default Dialer: an RMCP+ (IPMI 2.0 lanplus) session
// via github.com/bougou/go-ipmi.
func DialLanplus(ctx context.Context, addr string, creds bmc.Creds, o Options) (Conn, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "Connect", "bad address %q", addr)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "Connect", "bad port %q", portStr)
	}
	c, err := client.NewClient(host, port, creds.User, creds.Pass)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", bmc.ErrAuth, err) // user name too long
	}
	c.WithInterface(client.InterfaceLanplus).WithTimeout(o.Timeout).WithRetry(o.Retries)
	if err := c.Connect(ctx); err != nil {
		return nil, wrapConnectErr(err)
	}
	return &goipmiConn{c: c}, nil
}

func wrapConnectErr(err error) error {
	if errors.Is(err, client.ErrRAKPAuthentication) {
		return fmt.Errorf("%w: %w", bmc.ErrAuth, err)
	}
	var rs *types.RmcpStatusError
	if errors.As(err, &rs) {
		switch rs.StatusCode() {
		case types.RmcpStatusCodeInvalidRole, types.RmcpStatusCodeUnauthorizedRoleOfPriLevel,
			types.RmcpStatusCodeInvalidNameLength, types.RmcpStatusCodeUnauthorizedName,
			types.RmcpStatusCodeInvalidIntegrityCheckValue:
			return fmt.Errorf("%w: %w", bmc.ErrAuth, err)
		case types.RmcpStatusCodeNoResToCreateSess, types.RmcpStatusCodeNoResToCreateSessAtRole:
			return fmt.Errorf("%w: %w", bmc.ErrBusy, err) // session slots exhausted
		}
	}
	return wrapErr(err)
}

// wrapErr converts go-ipmi completion-code errors to *CCError.
func wrapErr(err error) error {
	if err == nil {
		return nil
	}
	if re, ok := types.IsResponseError(err); ok {
		return fmt.Errorf("%w (%v)", &CCError{Code: uint8(re.CompletionCode()), Desc: re.CompletionCode().String()}, err)
	}
	var cc types.CompletionCode
	if errors.As(err, &cc) {
		return &CCError{Code: uint8(cc), Desc: cc.String()}
	}
	return err
}

func (g *goipmiConn) ChassisStatus(ctx context.Context) (ChassisStatus, error) {
	r, err := g.c.GetChassisStatus(ctx)
	if err != nil {
		return ChassisStatus{}, wrapErr(err)
	}
	return ChassisStatus{PowerOn: r.PowerIsOn, RestorePolicy: RestorePolicyCode(r.PowerRestorePolicy), PowerFault: r.PowerFault,
		Interlock: r.InterLock, Overload: r.PowerOverload, IdentifySupported: r.ChassisIdentifySupported,
		IdentifyState: uint8(r.ChassisIdentifyState), IntrusionActive: r.ChassisIntrusionActive,
		FrontPanelLockout: r.FrontPanelLockoutActive, DriveFault: r.DriveFault, CoolingFanFault: r.CollingFanFault,
		LastPowerByCommand: r.LastPowerOnByCommand, ACFailed: r.ACFailed}, nil
}

func (g *goipmiConn) ChassisControl(ctx context.Context, ctl ChassisControl) error {
	_, err := g.c.ChassisControl(ctx, chassis.ChassisControl(ctl))
	return wrapErr(err)
}

func (g *goipmiConn) ChassisIdentify(ctx context.Context, interval uint8, force bool) error {
	_, err := g.c.ChassisIdentify(ctx, interval, force)
	return wrapErr(err)
}

func (g *goipmiConn) SetBootDevice(ctx context.Context, dev BootDevice, efi, persist bool) error {
	return wrapErr(g.c.SetBootDevice(ctx, types.BootDeviceSelector(dev), types.BIOSBootType(efi), persist))
}

func (g *goipmiConn) BootFlags(ctx context.Context) (BootFlags, error) {
	p := &types.BootOptionParam_BootFlags{}
	if err := g.c.GetSystemBootOptionsParamFor(ctx, p); err != nil {
		return BootFlags{}, wrapErr(err)
	}
	return BootFlags{Valid: p.BootFlagsValid, Persist: p.Persist, EFI: bool(p.BIOSBootType), Device: BootDevice(p.BootDeviceSelector)}, nil
}

func (g *goipmiConn) SetPowerRestorePolicy(ctx context.Context, p RestorePolicyCode) error {
	_, err := g.c.SetPowerRestorePolicy(ctx, chassis.PowerRestorePolicy(p))
	return wrapErr(err)
}

func readable(s *types.Sensor, t types.SensorThresholdType, v float64) *float64 {
	if !s.IsThresholdReadable(t) {
		return nil
	}
	return &v
}

func (g *goipmiConn) Sensors(ctx context.Context) ([]SensorRecord, error) {
	list, err := g.c.GetSensors(ctx)
	if err != nil {
		return nil, wrapErr(err)
	}
	out := make([]SensorRecord, 0, len(list))
	for _, s := range list {
		if s == nil {
			continue
		}
		r := SensorRecord{Number: s.Number, Name: s.Name, TypeName: s.SensorType.String(), Unit: s.SensorUnit.String(),
			Threshold: s.IsThreshold(), Present: !s.NotPresent && !s.ScanningDisabled && s.IsReadingValid(),
			Value: s.Value, Status: s.Status()}
		if r.Threshold {
			th := s.Threshold
			r.LNR = readable(s, types.SensorThresholdType_LNR, th.LNR)
			r.LCR = readable(s, types.SensorThresholdType_LCR, th.LCR)
			r.LNC = readable(s, types.SensorThresholdType_LNC, th.LNC)
			r.UNC = readable(s, types.SensorThresholdType_UNC, th.UNC)
			r.UCR = readable(s, types.SensorThresholdType_UCR, th.UCR)
			r.UNR = readable(s, types.SensorThresholdType_UNR, th.UNR)
		} else if r.Present {
			r.States = s.DiscreteActiveEventsString()
		}
		out = append(out, r)
	}
	return out, nil
}

func (g *goipmiConn) SEL(ctx context.Context) ([]SELRecord, error) {
	info, err := g.c.GetSELInfo(ctx)
	if err != nil {
		return nil, wrapErr(err)
	}
	if info.Entries == 0 {
		return nil, nil
	}
	list, err := g.c.GetSELEntries(ctx, 0)
	if err != nil {
		return nil, wrapErr(err)
	}
	out := make([]SELRecord, 0, len(list))
	for _, e := range list {
		if e == nil {
			continue
		}
		r := SELRecord{ID: e.RecordID, RecordType: uint8(e.RecordType)}
		switch {
		case e.Standard != nil:
			s := e.Standard
			r.Time, r.GeneratorID, r.SensorType, r.SensorNumber = s.Timestamp, uint16(s.GeneratorID), s.SensorType.String(), uint8(s.SensorNumber)
			r.EventType = s.EventReadingType.String()
			r.Deassertion = s.EventDir != types.EventDirAssertion
			r.Description = s.EventString()
			r.Severity = string(s.EventSeverity())
			r.EventData = [3]byte{s.EventData.EventData1, s.EventData.EventData2, s.EventData.EventData3}
		case e.OEMTimestamped != nil:
			r.OEM, r.Time = true, e.OEMTimestamped.Timestamp
			r.Description = fmt.Sprintf("OEM record (manufacturer 0x%06x) %x", e.OEMTimestamped.ManufacturerID, e.OEMTimestamped.OEMDefined[:])
		case e.OEMNonTimestamped != nil:
			r.OEM = true
			r.Description = fmt.Sprintf("OEM record %x", e.OEMNonTimestamped.OEM[:])
		}
		out = append(out, r)
	}
	return out, nil
}

func (g *goipmiConn) ClearSEL(ctx context.Context) error {
	res, err := g.c.ReserveSEL(ctx)
	if err != nil {
		return wrapErr(err)
	}
	_, err = g.c.ClearSEL(ctx, res.ReservationID)
	return wrapErr(err)
}

func fruStr(b []byte) string { return strings.TrimSpace(strings.TrimRight(string(b), "\x00")) }

func fruCustom(fs []types.FRUField) []string {
	var out []string
	for _, f := range fs {
		if s := fruStr(f.Data); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (g *goipmiConn) FRUs(ctx context.Context) ([]bmc.FRU, error) {
	list, err := g.c.GetFRUs(ctx)
	if err != nil {
		return nil, wrapErr(err)
	}
	out := make([]bmc.FRU, 0, len(list))
	for _, f := range list {
		if f == nil || !f.Present() {
			continue
		}
		x := bmc.FRU{ID: int(f.DeviceID), Name: f.DeviceName}
		if a := f.ChassisInfoArea; a != nil {
			x.Chassis = bmc.FRUChassis{Type: a.ChassisType.String(), PartNumber: fruStr(a.PartNumber), Serial: fruStr(a.SerialNumber),
				Extra: fruCustom(a.Custom)}
		}
		if a := f.BoardInfoArea; a != nil {
			x.Board = bmc.FRUBoard{Manufacturer: fruStr(a.Manufacturer), Product: fruStr(a.ProductName), Serial: fruStr(a.SerialNumber),
				PartNumber: fruStr(a.PartNumber), FRUFileID: fruStr(a.FRUFileID), Extra: fruCustom(a.Custom)}
			if !a.MfgDateTime.IsZero() {
				t := a.MfgDateTime
				x.Board.MfgDate = &t
			}
		}
		if a := f.ProductInfoArea; a != nil {
			x.Product = bmc.FRUProduct{Manufacturer: fruStr(a.Manufacturer), Name: fruStr(a.Name), PartNumber: fruStr(a.PartModel),
				Version: fruStr(a.Version), Serial: fruStr(a.SerialNumber), AssetTag: fruStr(a.AssetTag), FRUFileID: fruStr(a.FRUFileID),
				Extra: fruCustom(a.Custom)}
		}
		out = append(out, x)
	}
	return out, nil
}

func bcd(b uint8) uint8 { return (b>>4)*10 + b&0x0f }

func (g *goipmiConn) DeviceID(ctx context.Context) (DeviceInfo, error) {
	r, err := g.c.GetDeviceID(ctx)
	if err != nil {
		return DeviceInfo{}, wrapErr(err)
	}
	return DeviceInfo{DeviceID: r.DeviceID, DeviceRevision: r.DeviceRevision, FirmwareMajor: r.MajorFirmwareRevision,
		FirmwareMinor: bcd(r.MinorFirmwareRevision), IPMIVersion: fmt.Sprintf("%d.%d", r.MajorIPMIVersion, r.MinorIPMIVersion),
		ManufacturerID: r.ManufacturerID, ProductID: r.ProductID, DeviceAvailable: !r.DeviceAvailable, ProvidesSDRs: r.ProvideDeviceSDRs,
		AuxFirmwareBytes: r.AuxiliaryFirmwareRevision}, nil
}

func (g *goipmiConn) ColdReset(ctx context.Context) error { return wrapErr(g.c.ColdReset(ctx)) }
func (g *goipmiConn) WarmReset(ctx context.Context) error { return wrapErr(g.c.WarmReset(ctx)) }

func (g *goipmiConn) LANConfig(ctx context.Context, channel uint8) (LANConfig, error) {
	lc, err := g.c.GetLanConfig(ctx, channel)
	if err != nil {
		return LANConfig{}, wrapErr(err)
	}
	src := map[types.LanIPAddressSource]string{types.IPAddressSourceStatic: "static", types.IPAddressSourceDHCP: "dhcp",
		types.IPAddressSourceBIOS: "bios", types.IPAddressSourceOther: "other"}[lc.IPSource]
	if src == "" {
		src = "unspecified"
	}
	return LANConfig{Channel: channel, IP: lc.IP, Netmask: lc.SubnetMask, Gateway: lc.DefaultGatewayIP, MAC: lc.MAC, IPSource: src,
		VLANOn: lc.VLANEnabled, VLANID: lc.VLANID, RMCPPort: lc.PrimaryRMCPPort}, nil
}

func (g *goipmiConn) Users(ctx context.Context, channel uint8) ([]UserRecord, error) {
	us, err := g.c.GetUsers(ctx, channel)
	if err != nil {
		return nil, wrapErr(err)
	}
	out := make([]UserRecord, 0, len(us))
	for _, u := range us {
		if u == nil {
			continue
		}
		out = append(out, UserRecord{ID: u.ID, Name: u.Name, Enabled: u.IPMIMessagingEnabled, Privilege: strings.ToUpper(u.MaxPrivLevel.String()),
			Callin: u.Callin, LinkAuth: u.LinkAuthEnabled, IPMIMsg: u.IPMIMessagingEnabled})
	}
	return out, nil
}

func (g *goipmiConn) Close(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return wrapErr(g.c.Close(cctx))
}
