package boards

import (
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

func TestLookup(t *testing.T) {
	cases := []struct {
		board, fw, key string
		redfish        RedfishMode
		fan            FanDialect
		restore        RestoreBackend
	}{
		{"X470D4U", "2.32.00", "x470d4u", RedfishNo, FanASRR, RestoreIPMI},
		{"X470D4U2-2T", "3.02.00", "x470d4u", RedfishNo, FanASRR, RestoreIPMI},
		{"B650D4U", "4.10.00", "b650d4u", RedfishYes, FanSettings, RestoreASRR},
		{"B650D4U", "1.10.00", "b650d4u-1.x", RedfishProbe, FanASRR, RestoreASRR},
		{"B550D4U-2T", "1.10.00", "b550d4u", RedfishProbe, FanProbe, RestoreASRR},
		{"X570D4U", "1.20.00", "x570d4u", RedfishYes, FanASRR, RestoreASRR},
		{"X570D4U-2L2T", "1.20.00", "x570d4u", RedfishYes, FanASRR, RestoreASRR},
		{"X570D4U", "2.11.00", "x570d4u", RedfishYes, FanASRR, RestoreASRR},
		{"X570D4U", "beta", "x570d4u", RedfishProbe, FanProbe, RestoreASRR},
	}
	for _, c := range cases {
		p := Lookup(c.board, c.fw)
		if p.Key != c.key || p.Redfish != c.redfish || p.Fan != c.fan || p.Restore != c.restore {
			t.Errorf("%s %s: %+v", c.board, c.fw, p)
		}
	}
	u := Lookup("ROMED8-2T", "1.00")
	if u.Verified || u.Redfish != RedfishProbe || u.Fan != FanProbe {
		t.Errorf("unknown board must probe: %+v", u)
	}
	if !Lookup("X470D4U", "2.32.00").Verified || Lookup("X470D4U", "9.99").Verified {
		t.Error("verified flag")
	}
	if d, ok := Lookup("B650D4U", "4.10.00").TimeoutFor("/api/asrr/selsummary"); !ok || d <= 0 {
		t.Error("selsummary timeout")
	}
}

// TestX570Dialect pins the X570D4U 1.20 choices from docs/x570/live-probe.md.
func TestX570Dialect(t *testing.T) {
	p := Lookup(" x570d4u ", "1.20.00")
	if p.Board != "x570d4u" || !p.Verified {
		t.Fatalf("lookup: %+v", p)
	}
	if p.TLS != transport.ProfileModern || len(p.LoginExtra) != 0 {
		t.Errorf("transport: TLS %v login extra %v", p.TLS, p.LoginExtra)
	}
	if p.Fan != FanASRR || !p.FanDutyManual || p.LinkUp != LinkUpVetobit || p.Inventory != InventoryHost ||
		p.PostCode != PostCodeLogs || p.AutoDateField != "ntp_auto_date" || p.SensorMaskField != "settable_flag" ||
		p.ZeroThresholds || p.EventTrailer || p.BondIfc || p.Prochot || p.ClearMCA {
		t.Errorf("dialect: %+v", p)
	}
	for _, c := range []bmc.Capability{bmc.CapSensors, bmc.CapEventLog, bmc.CapEventLogClear, bmc.CapFirmwareInventory} {
		if !p.PrefersAMI(c) {
			t.Errorf("%s should prefer /api", c)
		}
	}
	if p.PrefersAMI(bmc.CapIdentity) || p.PrefersAMI(bmc.CapPower) {
		t.Error("identity/power stay Redfish-first")
	}
	if !p.HasComponent("bios") || !p.HasComponent("BP") || p.HasComponent("CPLD") {
		t.Errorf("components %v", p.Components)
	}
	if d, ok := p.TimeoutFor("/api/asrr/selsummary"); !ok || d <= 0 {
		t.Error("selsummary timeout")
	}
	if d, ok := p.TimeoutFor("/api/logs/event"); !ok || d < 60*time.Second {
		t.Errorf("logs/event timeout %v: the full SEL took 34 s live", d)
	}
	if Lookup("X570D4U", "2.11.00").Verified {
		t.Error("2.11 was never seen live")
	}
	if Lookup("B650D4U", "4.10.00").FanDutyManual || len(Lookup("B650D4U", "4.10.00").AMIFirst) != 0 {
		t.Error("X570-only fields leaked into the B650 profile")
	}
}
