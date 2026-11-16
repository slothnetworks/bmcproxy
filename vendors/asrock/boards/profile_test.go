package boards

import "testing"

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
