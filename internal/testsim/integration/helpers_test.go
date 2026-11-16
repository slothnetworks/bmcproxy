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

// drivers returns the production driver set (asrock + generic Redfish
// fallback), bound to the proxy's transport.
func drivers(ip *fakeIPMI) func(tm *transport.Manager) []bmc.Driver {
	return func(tm *transport.Manager) []bmc.Driver {
		return []bmc.Driver{
			asrock.New(asrock.Options{Transport: tm, IPMIDialer: ip.dial}),
			genericredfish.New(tm),
		}
	}
}

func startSim(t *testing.T, board string, smod ...func(*testsim.Options)) *testsim.Sim {
	t.Helper()
	if board == "x470" {
		return testsim.StartX470(t, smod...)
	}
	return testsim.StartB650(t, smod...)
}

// setupBoard starts a sim for board ("x470" | "b650") and a full proxy with
// the asrock + generic Redfish drivers.
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
