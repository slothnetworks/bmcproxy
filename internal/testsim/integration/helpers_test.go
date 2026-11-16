package integration_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	v1 "github.com/slothnetworks/bmcproxy/internal/api/v1"
	"github.com/slothnetworks/bmcproxy/internal/testsim"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/ipmi"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	"github.com/slothnetworks/bmcproxy/vendors/asrock"
	genericredfish "github.com/slothnetworks/bmcproxy/vendors/generic/redfish"
	"github.com/slothnetworks/bmcproxy/vendors/gigabyte"
)

// fakeIPMI is an in-memory IPMI backend for the X470's IPMI-backed
// capabilities (boot override, power restore policy). It never dials.
type fakeIPMI struct {
	mu      sync.Mutex
	dials   int
	restore ipmi.RestorePolicyCode
	boot    ipmi.BootFlags
}

func (f *fakeIPMI) dial(context.Context, string, bmc.Creds, ipmi.Options) (ipmi.Conn, error) {
	f.mu.Lock()
	f.dials++
	f.mu.Unlock()
	return f, nil
}

func (f *fakeIPMI) ChassisStatus(context.Context) (ipmi.ChassisStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return ipmi.ChassisStatus{PowerOn: true, RestorePolicy: f.restore}, nil
}
func (f *fakeIPMI) ChassisControl(context.Context, ipmi.ChassisControl) error { return nil }
func (f *fakeIPMI) ChassisIdentify(context.Context, uint8, bool) error        { return nil }
func (f *fakeIPMI) SetBootDevice(_ context.Context, d ipmi.BootDevice, efi, persist bool) error {
	f.mu.Lock()
	f.boot = ipmi.BootFlags{Valid: true, Device: d, EFI: efi, Persist: persist}
	f.mu.Unlock()
	return nil
}
func (f *fakeIPMI) BootFlags(context.Context) (ipmi.BootFlags, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.boot, nil
}
func (f *fakeIPMI) SetPowerRestorePolicy(_ context.Context, p ipmi.RestorePolicyCode) error {
	f.mu.Lock()
	f.restore = p
	f.mu.Unlock()
	return nil
}
func (f *fakeIPMI) Sensors(context.Context) ([]ipmi.SensorRecord, error) { return nil, nil }
func (f *fakeIPMI) SEL(context.Context) ([]ipmi.SELRecord, error)        { return nil, nil }
func (f *fakeIPMI) ClearSEL(context.Context) error                       { return nil }
func (f *fakeIPMI) FRUs(context.Context) ([]bmc.FRU, error)              { return nil, nil }
func (f *fakeIPMI) DeviceID(context.Context) (ipmi.DeviceInfo, error)    { return ipmi.DeviceInfo{}, nil }
func (f *fakeIPMI) ColdReset(context.Context) error                      { return nil }
func (f *fakeIPMI) WarmReset(context.Context) error                      { return nil }
func (f *fakeIPMI) LANConfig(context.Context, uint8) (ipmi.LANConfig, error) {
	return ipmi.LANConfig{}, nil
}
func (f *fakeIPMI) Users(context.Context, uint8) ([]ipmi.UserRecord, error) { return nil, nil }
func (f *fakeIPMI) Close(context.Context) error                             { return nil }

var _ ipmi.Conn = (*fakeIPMI)(nil)

// drivers returns the production driver set (gigabyte, asrock and the
// generic Redfish fallback, as registered by cmd/bmcproxy/drivers.go),
// bound to the proxy's transport.
func drivers(ip *fakeIPMI) func(tm *transport.Manager) []bmc.Driver {
	return func(tm *transport.Manager) []bmc.Driver {
		return []bmc.Driver{
			gigabyte.New(gigabyte.Options{Transport: tm}),
			asrock.New(asrock.Options{Transport: tm, IPMIDialer: ip.dial}),
			genericredfish.New(tm),
		}
	}
}

// allBoards is the board matrix every board-generic test runs on.
var allBoards = []string{"x470", "b650", "x570"}

// gigabyteBoards is the Gigabyte board matrix (gigabyte_test.go).
var gigabyteBoards = []string{"mc13le0", "mz32ar0"}

// boardName is the live board name of each sim board (asrr/model-name on
// ASRock, Redfish ComputerSystem.Model on Gigabyte).
var boardName = map[string]string{"x470": "X470D4U", "b650": "B650D4U", "x570": "X570D4U",
	"mc13le0": "MC13-LE0-000", "mz32ar0": "MZ32-AR0-00"}

// boardVendor is the driver that must serve each sim board.
var boardVendor = map[string]string{"x470": "asrock", "b650": "asrock", "x570": "asrock", "mc13le0": "gigabyte", "mz32ar0": "gigabyte"}

// asrrDialect: the board uses the X470-era /api OEM dialect (asrr/settings
// fan endpoints, asrr/getvetobit) rather than settings/fan + ncsilinkup.
func asrrDialect(board string) bool { return board == "x470" || board == "x570" }

// amiSEL: the driver reads the SEL over /api logs/event (X470: no Redfish;
// X570: its Redfish SEL is near-empty) rather than Redfish.
func amiSEL(board string) bool { return board == "x470" || board == "x570" }

func startSim(t *testing.T, board string, smod ...func(*testsim.Options)) *testsim.Sim {
	t.Helper()
	switch board {
	case "x470":
		return testsim.StartX470(t, smod...)
	case "x570":
		return testsim.StartX570(t, smod...)
	case "mc13le0":
		return testsim.StartMC13LE0(t, smod...)
	case "mz32ar0":
		return testsim.StartMZ32AR0(t, smod...)
	}
	return testsim.StartB650(t, smod...)
}

// setupBoard starts a sim for board (see allBoards, gigabyteBoards) and a
// full proxy with the production driver set.
func setupBoard(t *testing.T, board string, popt func(*testsim.ProxyOptions), smod ...func(*testsim.Options)) (*testsim.Sim, *testsim.Proxy, *fakeIPMI) {
	t.Helper()
	sim := startSim(t, board, smod...)
	ip := &fakeIPMI{}
	o := testsim.ProxyOptions{Drivers: drivers(ip),
		V1: v1.Config{PowerPollInterval: 5 * time.Millisecond, PowerPollTimeout: time.Second}}
	if popt != nil {
		popt(&o)
	}
	return sim, testsim.StartProxy(t, o), ip
}

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
