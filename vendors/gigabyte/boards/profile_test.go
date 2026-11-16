package boards

import "testing"

func TestLookup(t *testing.T) {
	for _, tc := range []struct {
		board, fw, key string
		verified       bool
	}{
		{"MC13-LE0-000", "13.04.15", "mc13le0", true},
		{"mc13-le0", "13.04.15", "mc13le0", true},
		{"MC13-LE0-000", "13.06.30", "mc13le0", false},
		{"MZ99-XX0", "1.0", "unknown", false},
		{"", "", "unknown", false},
	} {
		p := Lookup(tc.board, tc.fw)
		if p.Key != tc.key || p.Verified != tc.verified {
			t.Errorf("Lookup(%q,%q) = %s verified=%v, want %s %v", tc.board, tc.fw, p.Key, p.Verified, tc.key, tc.verified)
		}
	}
	p := Lookup("MC13-LE0-000", "13.04.15")
	if p.Redfish.SkipPageSize != 50 || !p.Redfish.NoExpand || p.SELTime != SELTimeEvent || !p.AMI || !p.FanProfile {
		t.Fatalf("mc13le0 profile: %+v", p)
	}
	if u := Unknown("X", "1", false); u.AMI || u.Redfish.SkipPageSize != 0 || len(u.ExtraDeny) != 0 {
		t.Fatalf("non-AMI unknown should be plain Redfish: %+v", u)
	}
	if u := Unknown("X", "1", true); !u.AMI || u.Redfish.SkipPageSize != 50 {
		t.Fatalf("AMI unknown: %+v", u)
	}
}

func TestLookupMZ32AndGenerations(t *testing.T) {
	p := Lookup("MZ32-AR0-00", "12.61.39")
	if p.Key != "mz32ar0" || !p.Verified || p.SELSource != SELAMI || p.Redfish.QueryCaps == nil || p.Redfish.QueryCaps.Filter ||
		!p.Redfish.QueryCaps.TopSkip || p.Redfish.SkipPageSize != 0 {
		t.Fatalf("mz32: %+v", p)
	}
	if p := Lookup("MZ32-AR0-00", "12.62.00"); p.Verified || p.Key != "mz32ar0" {
		t.Fatalf("mz32 unverified: %+v", p)
	}
	if u := Unknown("MZ72-HB0", "12.60.1", true); u.Redfish.QueryCaps == nil || u.Redfish.SkipPageSize != 0 {
		t.Fatalf("unknown 12.x should get SP-X 12 quirks: %+v", u)
	}
	if u := Unknown("MC62-G40", "13.07.01", true); u.Redfish.SkipPageSize != 50 {
		t.Fatalf("unknown 13.x should get SP-X 13 quirks: %+v", u)
	}
	for in, want := range map[string]int{"12.61.39": 12, "13.04.15": 13, "": -1, "v1": -1} {
		if fwMajor(in) != want {
			t.Errorf("fwMajor(%q)", in)
		}
	}
	deny := map[string]bool{}
	for _, r := range AllExtraDeny() {
		deny[r.Pattern] = true
	}
	for _, p := range []string{"ipmi_response", "hostinterface/diag", "kvm/token"} {
		if !deny[p] {
			t.Errorf("AllExtraDeny missing %s", p)
		}
	}
}
