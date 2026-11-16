package ipmi_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/ipmi"
)

// fakeConn is an in-memory BMC shaped after the X470D4U IPMI surface.
type fakeConn struct {
	mu       sync.Mutex
	status   ipmi.ChassisStatus
	controls []ipmi.ChassisControl
	identify []string
	boot     ipmi.BootFlags
	sensors  []ipmi.SensorRecord
	sel      []ipmi.SELRecord
	users    []ipmi.UserRecord
	failNext error
	closed   int
	cold     int
	inflight atomic.Int32
	peak     atomic.Int32
	sleep    time.Duration
}

func f(v float64) *float64 { return &v }

func newFake() *fakeConn {
	return &fakeConn{
		status: ipmi.ChassisStatus{PowerOn: false, RestorePolicy: ipmi.RestoreAlwaysOff, IdentifySupported: true},
		sensors: []ipmi.SensorRecord{
			{Number: 0x01, Name: "3VSB", TypeName: "Voltage", Unit: "Volts", Threshold: true, Present: true, Value: 3.36, Status: "ok",
				LCR: f(2.88), UCR: f(3.72)},
			{Number: 0x0c, Name: "CPU Temp", TypeName: "Temperature", Unit: "degrees C", Threshold: true, Present: false, Status: "N/A",
				UCR: f(95)},
			{Number: 0x10, Name: "MB Temp", TypeName: "Temperature", Unit: "degrees C", Threshold: true, Present: true, Value: 86, Status: "ucr",
				UNC: f(80), UCR: f(85)},
			{Number: 0x40, Name: "FAN1", TypeName: "Fan", Unit: "RPM", Threshold: true, Present: true, Value: 300, Status: "lnc", LNC: f(400)},
			{Number: 0x60, Name: "ChassisIntr", TypeName: "Physical Security", Unit: "discrete", Threshold: false, Present: true,
				Status: "0x0100", States: []string{"General Chassis intrusion"}},
		},
		sel: []ipmi.SELRecord{
			{ID: 3, Time: time.Date(2026, 11, 14, 20, 6, 0, 0, time.UTC), SensorType: "Temperature", SensorNumber: 0x10,
				EventType: "Threshold", Description: "Upper Critical going high", Severity: "Critical", EventData: [3]byte{0x59, 0x56, 0x55}},
			{ID: 1, Time: time.Date(2026, 11, 14, 20, 5, 0, 0, time.UTC), SensorType: "Power Supply", SensorNumber: 0x70,
				EventType: "Sensor-specific", Description: "Power Supply AC lost", Severity: "Warning"},
			{ID: 2, Time: time.Date(2026, 11, 14, 20, 5, 30, 0, time.UTC), SensorType: "Power Supply", SensorNumber: 0x70,
				EventType: "Sensor-specific", Deassertion: true, Description: "Power Supply AC lost", Severity: "OK"},
			{ID: 7, OEM: true, RecordType: 0xe0, Description: "OEM record 0102"},
		},
		users: []ipmi.UserRecord{{ID: 1, Name: ""}, {ID: 2, Name: "admin", Enabled: true, Privilege: "ADMINISTRATOR", IPMIMsg: true},
			{ID: 3, Name: "ops", Enabled: false, Privilege: "OPERATOR"}},
	}
}

func (c *fakeConn) enter() func() {
	n := c.inflight.Add(1)
	for {
		p := c.peak.Load()
		if n <= p || c.peak.CompareAndSwap(p, n) {
			break
		}
	}
	if c.sleep > 0 {
		time.Sleep(c.sleep)
	}
	return func() { c.inflight.Add(-1) }
}

func (c *fakeConn) fail() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.failNext
	c.failNext = nil
	return err
}

func (c *fakeConn) ChassisStatus(ctx context.Context) (ipmi.ChassisStatus, error) {
	defer c.enter()()
	if err := c.fail(); err != nil {
		return ipmi.ChassisStatus{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status, nil
}

func (c *fakeConn) ChassisControl(ctx context.Context, ctl ipmi.ChassisControl) error {
	defer c.enter()()
	if err := c.fail(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.controls = append(c.controls, ctl)
	switch ctl {
	case ipmi.ControlPowerUp, ipmi.ControlPowerCycle, ipmi.ControlHardReset:
		c.status.PowerOn = true
	case ipmi.ControlPowerDown, ipmi.ControlSoftOff:
		c.status.PowerOn = false
	}
	return nil
}

func (c *fakeConn) ChassisIdentify(ctx context.Context, interval uint8, force bool) error {
	defer c.enter()()
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case force:
		c.status.IdentifyState = 2
	case interval > 0:
		c.status.IdentifyState = 1
	default:
		c.status.IdentifyState = 0
	}
	return nil
}

func (c *fakeConn) SetBootDevice(ctx context.Context, dev ipmi.BootDevice, efi, persist bool) error {
	defer c.enter()()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.boot = ipmi.BootFlags{Valid: dev != ipmi.BootNoOverride, Persist: persist, EFI: efi, Device: dev}
	return nil
}

func (c *fakeConn) BootFlags(ctx context.Context) (ipmi.BootFlags, error) {
	defer c.enter()()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.boot, nil
}

func (c *fakeConn) SetPowerRestorePolicy(ctx context.Context, p ipmi.RestorePolicyCode) error {
	defer c.enter()()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.RestorePolicy = p
	return nil
}

func (c *fakeConn) Sensors(ctx context.Context) ([]ipmi.SensorRecord, error) {
	defer c.enter()()
	return c.sensors, c.fail()
}

func (c *fakeConn) SEL(ctx context.Context) ([]ipmi.SELRecord, error) {
	defer c.enter()()
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ipmi.SELRecord(nil), c.sel...), nil
}

func (c *fakeConn) ClearSEL(ctx context.Context) error {
	defer c.enter()()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sel = nil
	return nil
}

func (c *fakeConn) FRUs(ctx context.Context) ([]bmc.FRU, error) {
	defer c.enter()()
	return []bmc.FRU{{ID: 0, Name: "Builtin FRU Device", Board: bmc.FRUBoard{Manufacturer: "ASRockRack", Product: "X470D4U"}}}, nil
}

func (c *fakeConn) DeviceID(ctx context.Context) (ipmi.DeviceInfo, error) {
	defer c.enter()()
	return ipmi.DeviceInfo{DeviceID: 0x20, FirmwareMajor: 2, FirmwareMinor: 32, IPMIVersion: "2.0", ManufacturerID: 0xC1D6, ProductID: 0x0B02}, nil
}

func (c *fakeConn) ColdReset(ctx context.Context) error {
	defer c.enter()()
	c.mu.Lock()
	c.cold++
	c.mu.Unlock()
	return nil
}

func (c *fakeConn) WarmReset(ctx context.Context) error {
	return &ipmi.CCError{Code: ipmi.CCInvalidCommand, Desc: "Invalid Command"}
}

func (c *fakeConn) LANConfig(ctx context.Context, ch uint8) (ipmi.LANConfig, error) {
	defer c.enter()()
	mac, _ := net.ParseMAC("00:00:5E:00:53:02")
	return ipmi.LANConfig{Channel: ch, IP: net.ParseIP("192.0.2.207").To4(), Netmask: net.ParseIP("255.255.255.0").To4(),
		Gateway: net.ParseIP("192.0.2.1").To4(), MAC: mac, IPSource: "static", RMCPPort: 623}, nil
}

func (c *fakeConn) Users(ctx context.Context, ch uint8) ([]ipmi.UserRecord, error) {
	defer c.enter()()
	return c.users, nil
}

func (c *fakeConn) Close(ctx context.Context) error {
	c.mu.Lock()
	c.closed++
	c.mu.Unlock()
	return nil
}

var ctx = context.Background()

func connect(t *testing.T, fc *fakeConn, addr string, hook func(ipmi.CommandInfo)) *ipmi.Client {
	t.Helper()
	dial := func(ctx context.Context, a string, cr bmc.Creds, o ipmi.Options) (ipmi.Conn, error) {
		if cr.Pass != "secret" {
			return nil, errors.Join(bmc.ErrAuth, errors.New("RAKP authentication failed"))
		}
		return fc, nil
	}
	c, err := ipmi.Connect(ctx, addr, bmc.Creds{User: "admin", Pass: "secret"}, ipmi.Options{Dialer: dial, Hook: hook})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestConnectAuthAndAddr(t *testing.T) {
	dial := func(ctx context.Context, a string, cr bmc.Creds, o ipmi.Options) (ipmi.Conn, error) {
		if a != "192.0.2.10:623" {
			t.Errorf("addr %q", a)
		}
		return nil, errors.Join(bmc.ErrAuth, errors.New("RAKP authentication failed"))
	}
	_, err := ipmi.Connect(ctx, "192.0.2.10", bmc.Creds{User: "admin", Pass: "x"}, ipmi.Options{Dialer: dial})
	if !errors.Is(err, bmc.ErrAuth) || bmc.HTTPStatus(err) != 401 {
		t.Fatalf("want ErrAuth: %v", err)
	}
	timeout := func(ctx context.Context, a string, cr bmc.Creds, o ipmi.Options) (ipmi.Conn, error) {
		return nil, errors.New("read udp: i/o timeout")
	}
	if _, err := ipmi.Connect(ctx, "192.0.2.10:623", bmc.Creds{}, ipmi.Options{Dialer: timeout}); !errors.Is(err, bmc.ErrTimeout) {
		t.Fatalf("want ErrTimeout: %v", err)
	}
}

func TestPowerChassisBootPolicy(t *testing.T) {
	fc := newFake()
	c := connect(t, fc, "192.0.2.11:623", nil)
	if st, err := c.PowerState(ctx); err != nil || st != bmc.PowerOff {
		t.Fatalf("state %v %v", st, err)
	}
	for _, a := range []bmc.PowerAction{bmc.ActionOn, bmc.ActionShutdown, bmc.ActionOff, bmc.ActionCycle, bmc.ActionReset} {
		if err := c.Power(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	want := []ipmi.ChassisControl{1, 5, 0, 2, 3}
	for i, w := range want {
		if fc.controls[i] != w {
			t.Fatalf("controls %v, want %v", fc.controls, want)
		}
	}
	if st, _ := c.PowerState(ctx); st != bmc.PowerOn {
		t.Fatal("not on after reset")
	}

	if err := c.SetIndicatorLED(ctx, bmc.LEDOn); err != nil {
		t.Fatal(err)
	}
	if s, _ := c.GetIndicatorLED(ctx); s != bmc.LEDOn {
		t.Fatalf("led %v", s)
	}
	_ = c.SetIndicatorLED(ctx, bmc.LEDBlinking)
	if s, _ := c.GetIndicatorLED(ctx); s != bmc.LEDBlinking {
		t.Fatalf("led %v", s)
	}
	_ = c.SetIndicatorLED(ctx, bmc.LEDOff)
	if s, _ := c.GetIndicatorLED(ctx); s != bmc.LEDOff {
		t.Fatalf("led %v", s)
	}

	if err := c.SetBootOverride(ctx, bmc.BootOverrideConfig{Target: bmc.BootPXE, Mode: bmc.BootOnce, BootMode: "uefi"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := c.GetBootOverride(ctx)
	if err != nil || cfg.Target != bmc.BootPXE || cfg.Mode != bmc.BootOnce || cfg.BootMode != "uefi" {
		t.Fatalf("boot %+v %v", cfg, err)
	}
	_ = c.SetBootOverride(ctx, bmc.BootOverrideConfig{Target: bmc.BootDisk, Mode: bmc.BootContinuous})
	cfg, _ = c.GetBootOverride(ctx)
	if cfg.Target != bmc.BootDisk || cfg.Mode != bmc.BootContinuous || cfg.BootMode != "legacy" {
		t.Fatalf("boot %+v", cfg)
	}
	_ = c.SetBootOverride(ctx, bmc.BootOverrideConfig{Target: bmc.BootDisk, Mode: bmc.BootDisabled})
	if cfg, _ = c.GetBootOverride(ctx); cfg.Mode != bmc.BootDisabled || cfg.Target != bmc.BootNone {
		t.Fatalf("boot disabled %+v", cfg)
	}

	if p, _ := c.GetPowerRestorePolicy(ctx); p != bmc.RestoreAlwaysOff {
		t.Fatalf("policy %v", p)
	}
	if err := c.SetPowerRestorePolicy(ctx, bmc.RestoreLastState); err != nil {
		t.Fatal(err)
	}
	if p, _ := c.GetPowerRestorePolicy(ctx); p != bmc.RestoreLastState {
		t.Fatalf("policy %v", p)
	}
	if err := c.SetPowerRestorePolicy(ctx, "bogus"); !errors.Is(err, bmc.ErrInvalidArgument) {
		t.Fatalf("bad policy: %v", err)
	}
}

func TestSensorsNormalized(t *testing.T) {
	c := connect(t, newFake(), "192.0.2.12:623", nil)
	ss, err := c.Sensors(ctx)
	if err != nil || len(ss) != 5 {
		t.Fatalf("sensors %d %v", len(ss), err)
	}
	v := ss[0]
	if v.ID != "1" || *v.Number != 1 || v.Type != bmc.SensorVoltage || v.Unit != "V" || *v.Reading != 3.36 || v.Health != bmc.HealthOK ||
		*v.Thresholds.LowerCritical != 2.88 || v.Thresholds.LowerNonCritical != nil || v.Backend != "ipmi" {
		t.Fatalf("3VSB %+v", v)
	}
	if cpu := ss[1]; cpu.Reading != nil || cpu.Health != bmc.HealthUnknown || cpu.Unit != "C" || *cpu.Thresholds.UpperCritical != 95 {
		t.Fatalf("CPU Temp %+v", cpu)
	}
	if mb := ss[2]; mb.Health != bmc.HealthCritical || mb.Type != bmc.SensorTemperature {
		t.Fatalf("MB Temp %+v", mb)
	}
	if fan := ss[3]; fan.Health != bmc.HealthWarning || fan.Type != bmc.SensorFan || fan.Unit != "RPM" {
		t.Fatalf("FAN1 %+v", fan)
	}
	if d := ss[4]; d.Type != bmc.SensorDiscrete || d.Reading != nil || d.State != "General Chassis intrusion" || d.Unit != "" {
		t.Fatalf("discrete %+v", d)
	}
}

func TestSELIncrementalAndClear(t *testing.T) {
	fc := newFake()
	c := connect(t, fc, "192.0.2.13:623", nil)
	c.SetClockOffset(time.Hour)
	page, err := c.Events(ctx, bmc.EventQuery{})
	if err != nil || len(page.Entries) != 4 || page.LastID != "7" {
		t.Fatalf("all %+v %v", page, err)
	}
	e := page.Entries[2]
	if e.ID != "3" || e.Severity != bmc.HealthCritical || e.Direction != "assertion" || e.EventData != "595655" || *e.SensorNumber != 16 ||
		!e.Time.Corrected.Equal(e.Time.Raw.Add(-time.Hour)) {
		t.Fatalf("entry %+v", e)
	}
	if d := page.Entries[1]; d.Direction != "deassertion" || d.Severity != bmc.HealthOK {
		t.Fatalf("deassert %+v", d)
	}
	if o := page.Entries[3]; o.SensorNumber != nil || o.Oem["recordType"] != "0xe0" {
		t.Fatalf("oem %+v", o)
	}
	page, _ = c.Events(ctx, bmc.EventQuery{SinceID: "1", Limit: 1})
	if len(page.Entries) != 1 || page.Entries[0].ID != "2" || !page.More || page.LastID != "2" {
		t.Fatalf("since/limit %+v", page)
	}
	page, _ = c.Events(ctx, bmc.EventQuery{SinceID: "7"})
	if len(page.Entries) != 0 || page.LastID != "7" {
		t.Fatalf("none %+v", page)
	}
	if _, err := c.Events(ctx, bmc.EventQuery{SinceID: "abc"}); !errors.Is(err, bmc.ErrInvalidArgument) {
		t.Fatalf("bad since %v", err)
	}
	if err := c.ClearEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if page, _ = c.Events(ctx, bmc.EventQuery{}); len(page.Entries) != 0 {
		t.Fatal("not cleared")
	}
}

func TestFRUMCInfoLANUsersReset(t *testing.T) {
	fc := newFake()
	c := connect(t, fc, "192.0.2.14:623", nil)
	fru, err := c.FRU(ctx)
	if err != nil || fru[0].Board.Product != "X470D4U" {
		t.Fatalf("fru %+v %v", fru, err)
	}
	mc, err := c.MCInfo(ctx)
	if err != nil || mc.FirmwareVersion() != "2.32" || mc.ManufacturerID != 0xC1D6 {
		t.Fatalf("mc %+v %v", mc, err)
	}
	n, err := c.NetworkInterface(ctx)
	if err != nil || n.IPv4.Address != "192.0.2.207" || n.IPv4.Gateway != "192.0.2.1" || n.MAC != "00:00:5e:00:53:02" || n.IPv4.DHCP || n.Name != "lan1" {
		t.Fatalf("lan %+v %v", n, err)
	}
	users, err := c.Users(ctx)
	if err != nil || len(users) != 2 || users[0].Name != "admin" || users[0].Privilege != bmc.PrivAdministrator || users[1].Enabled {
		t.Fatalf("users %+v %v", users, err)
	}
	if err := c.ResetBMC(ctx, bmc.BMCResetCold); err != nil || fc.cold != 1 {
		t.Fatalf("cold reset %v", err)
	}
	if err := c.ResetBMC(ctx, bmc.BMCResetWarm); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("warm reset cc 0xC1 -> unsupported: %v", err)
	}
	if err := c.Close(ctx); err != nil {
		t.Fatal(err)
	}
	_ = c.Close(ctx)
	if fc.closed != 1 {
		t.Fatalf("closed %d", fc.closed)
	}
	if _, err := c.PowerState(ctx); err == nil {
		t.Fatal("command after close succeeded")
	}
}

func TestDebugHookAndCompletionCodes(t *testing.T) {
	fc := newFake()
	var infos []ipmi.CommandInfo
	var hmu sync.Mutex
	c := connect(t, fc, "192.0.2.15:623", func(ci ipmi.CommandInfo) {
		hmu.Lock()
		infos = append(infos, ci)
		hmu.Unlock()
	})
	var buf bytes.Buffer
	lctx := obs.WithLogger(ctx, slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	fc.failNext = &ipmi.CCError{Code: ipmi.CCNodeBusy}
	if _, err := c.PowerState(lctx); !errors.Is(err, bmc.ErrBusy) || bmc.HTTPStatus(err) != 503 {
		t.Fatalf("busy: %v", err)
	}
	fc.failNext = &ipmi.CCError{Code: ipmi.CCInsufficientPriv}
	if err := c.Power(lctx, bmc.ActionOn); !errors.Is(err, bmc.ErrAuth) {
		t.Fatalf("priv: %v", err)
	}
	fc.failNext = &ipmi.CCError{Code: ipmi.CCInvalidCommand}
	if _, err := c.Sensors(lctx); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("unsupported: %v", err)
	}
	if _, err := c.PowerState(lctx); err != nil {
		t.Fatal(err)
	}
	hmu.Lock()
	defer hmu.Unlock()
	// Connect + 4 commands
	if len(infos) != 5 {
		t.Fatalf("hook calls %d", len(infos))
	}
	if i := infos[1]; i.Op != "GetChassisStatus" || i.NetFn != 0x00 || i.Cmd != 0x01 || i.CC != 0xC0 || i.Host != "192.0.2.15:623" {
		t.Fatalf("hook info %+v", i)
	}
	if i := infos[2]; i.Cmd != 0x02 || i.CC != 0xD4 {
		t.Fatalf("hook info %+v", i)
	}
	if i := infos[4]; i.CC != 0 || i.Err != nil {
		t.Fatalf("hook ok info %+v", i)
	}
	log := buf.String()
	if !strings.Contains(log, "netfn=0x00") || !strings.Contains(log, "cmd=0x01") || !strings.Contains(log, "cc=0xc0") ||
		!strings.Contains(log, "netfn=0x0a cmd=0x23") {
		t.Fatalf("debug log:\n%s", log)
	}
	if strings.Contains(log, "secret") {
		t.Fatal("password leaked into log")
	}
}

func TestSerialisedPerHost(t *testing.T) {
	fc := newFake()
	fc.sleep = 5 * time.Millisecond
	a := connect(t, fc, "192.0.2.16:623", nil)
	b := connect(t, fc, "192.0.2.16:623", nil) // second session, same host
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := a
			if i%2 == 1 {
				c = b
			}
			if _, err := c.PowerState(ctx); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if p := fc.peak.Load(); p != 1 {
		t.Fatalf("peak in-flight %d, want 1", p)
	}
	// A canceled context waiting for the host lock returns promptly.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := a.PowerState(cctx); err == nil || !errors.Is(err, context.Canceled) {
		// the lock may be free; only assert the error class when it failed
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled: %v", err)
		}
	}
}
