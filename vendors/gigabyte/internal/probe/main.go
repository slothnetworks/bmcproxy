//go:build gigabytelive

// Command probe is a READ-ONLY live check of the gigabyte driver against a
// real BMC. It is excluded from default builds (tag gigabytelive):
//
//	set -a; . ./mc13-le0-000.env; set +a
//	go run -tags gigabytelive ./vendors/gigabyte/internal/probe
//
// Credentials come only from the environment (BMC_HOST, BMC_USER,
// BMC_PASS) and are never printed. It resolves the driver, opens one
// session, calls only getters (no Set*, no actions, no KVM launch) one at a
// time, prints a sanitized summary (no serials, MACs, UUIDs, addresses or
// user names) and closes the session (Redfish has nothing to close with
// Basic auth; the lazy AMI /api session is logged out).
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	_ "github.com/slothnetworks/bmcproxy/vendors/generic/redfish"
	"github.com/slothnetworks/bmcproxy/vendors/gigabyte"
)

func main() {
	host, user := os.Getenv("BMC_HOST"), os.Getenv("BMC_USER")
	pass := os.Getenv("BMC_PASS")
	if host == "" || user == "" || pass == "" {
		fmt.Fprintln(os.Stderr, "BMC_HOST, BMC_USER and BMC_PASS must be set")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	tg, err := bmc.ParseTarget(host)
	if err != nil {
		fail("target", err)
	}
	creds := bmc.Creds{User: user, Pass: pass}

	t0 := time.Now()
	m, err := gigabyte.Default.Detect(ctx, tg)
	step("Detect (unauth)", t0, err, fmt.Sprintf("score=%d signal=%q stack=%q", m.Score, m.Details["signal"], m.Details["stack"]))

	t0 = time.Now()
	d, rm, err := bmc.Resolve(ctx, tg, &creds)
	name := ""
	if d != nil {
		name = d.Name()
	}
	step("Resolve (creds)", t0, err, fmt.Sprintf("driver=%s score=%d board=%s firmware=%s profile=%s verified=%s",
		name, rm.Score, rm.Board, rm.Firmware, rm.Details["profile"], rm.Details["verified"]))
	if err != nil || name != gigabyte.Name {
		os.Exit(1)
	}

	t0 = time.Now()
	bs, err := d.Open(ctx, tg, creds)
	step("Open", t0, err, "")
	if err != nil {
		os.Exit(1)
	}
	s := bs.(*gigabyte.Session)
	defer func() {
		t0 := time.Now()
		err := s.Close(context.Background())
		step("Close", t0, err, "")
	}()
	in := s.Info()
	caps := make([]string, 0, len(in.Capabilities))
	for c := range in.Capabilities {
		caps = append(caps, string(c))
	}
	sort.Strings(caps)
	fmt.Printf("info: driver=%s profile=%s degraded=%q caps=%s\n", in.Driver, s.Profile().Key, in.Degraded, strings.Join(caps, ","))

	run := func(n string, f func() (string, error)) {
		t0 := time.Now()
		out, err := f()
		step(n, t0, err, out)
	}
	run("SystemInfo", func() (string, error) {
		v, err := s.SystemInfo(ctx)
		return fmt.Sprintf("manufacturer=%q board=%q bmc=%s bios=%s power=%s health=%s", v.Manufacturer, v.Board, v.BMCFirmware, v.BIOSVersion, v.PowerState, v.Health), err
	})
	run("PowerState", func() (string, error) { v, err := s.PowerState(ctx); return string(v), err })
	run("RestorePolicy", func() (string, error) { v, err := s.GetPowerRestorePolicy(ctx); return string(v), err })
	run("IndicatorLED", func() (string, error) { v, err := s.GetIndicatorLED(ctx); return fmt.Sprint(v), err })
	run("BootOverride", func() (string, error) { v, err := s.GetBootOverride(ctx); return fmt.Sprintf("%+v", v), err })
	run("Sensors", func() (string, error) {
		v, err := s.Sensors(ctx)
		by := map[bmc.SensorType]int{}
		units := map[string]int{}
		for _, x := range v {
			by[x.Type]++
			units[x.Unit]++
		}
		return fmt.Sprintf("n=%d types=%v units=%v", len(v), by, units), err
	})
	run("Events", func() (string, error) {
		v, err := s.Events(ctx, bmc.EventQuery{})
		if len(v.Entries) == 0 {
			return "n=0", err
		}
		first, last := v.Entries[0], v.Entries[len(v.Entries)-1]
		distinct := map[time.Time]bool{}
		for _, e := range v.Entries {
			distinct[e.Time.Raw] = true
		}
		return fmt.Sprintf("n=%d ids=%s..%s first=%s last=%s distinctTimes=%d", len(v.Entries), first.ID, last.ID,
			first.Time.Raw.Format(time.RFC3339), last.Time.Raw.Format(time.RFC3339), len(distinct)), err
	})
	run("AuditLog", func() (string, error) {
		v, err := s.AuditLog(ctx, bmc.AuditQuery{})
		return fmt.Sprintf("n=%d", len(v)), err
	})
	run("FRU", func() (string, error) {
		v, err := s.FRU(ctx)
		if len(v) == 0 {
			return "n=0", err
		}
		return fmt.Sprintf("n=%d boardSerialSet=%v boardSerialPlaceholder=%v product=%q", len(v), v[0].Board.Serial != "",
			strings.HasPrefix(v[0].Board.Serial, "0123456789"), v[0].Product.Name), err
	})
	run("CPUs", func() (string, error) { v, err := s.CPUs(ctx); return fmt.Sprintf("n=%d", len(v)), err })
	run("Memory", func() (string, error) { v, err := s.Memory(ctx); return fmt.Sprintf("n=%d", len(v)), err })
	run("PCIe", func() (string, error) { v, err := s.PCIeDevices(ctx); return fmt.Sprintf("n=%d", len(v)), err })
	run("Storage", func() (string, error) { v, err := s.Storage(ctx); return fmt.Sprintf("n=%d", len(v)), err })
	run("Baseboard", func() (string, error) {
		v, err := s.Baseboard(ctx)
		return fmt.Sprintf("manufacturer=%q product=%q serialSet=%v", v.Manufacturer, v.Product, v.Serial != ""), err
	})
	run("Network", func() (string, error) {
		v, err := s.NetworkInterfaces(ctx)
		var names []string
		for _, n := range v {
			names = append(names, n.Name)
		}
		return fmt.Sprintf("n=%d names=%v", len(v), names), err
	})
	run("Users", func() (string, error) { v, err := s.Users(ctx); return fmt.Sprintf("n=%d", len(v)), err })
	run("Services", func() (string, error) { v, err := s.Services(ctx); return fmt.Sprintf("n=%d", len(v)), err })
	run("Time", func() (string, error) {
		v, err := s.Time(ctx)
		return fmt.Sprintf("offset=%.0fs ntp=%v tz=%q", v.OffsetSeconds, v.NTPEnabled, v.Timezone), err
	})
	run("Firmware", func() (string, error) {
		v, err := s.Firmware(ctx)
		var parts []string
		for _, c := range v {
			parts = append(parts, c.Name+"="+c.Version)
		}
		return strings.Join(parts, " "), err
	})
	run("VirtualMedia", func() (string, error) { v, err := s.VirtualMedia(ctx); return fmt.Sprintf("n=%d", len(v)), err })
	run("vendor fanProfiles", func() (string, error) {
		v, err := s.FanProfiles(ctx)
		var names []string
		for _, p := range v.Profiles {
			names = append(names, fmt.Sprintf("%s(%d)", p.Name, len(p.Policies)))
		}
		return fmt.Sprintf("mode=%s profiles=%v", v.Mode, names), err
	})
	run("vendor fanProfileDevices", func() (string, error) {
		v, err := s.FanProfileDevices(ctx)
		return fmt.Sprintf("n=%d", len(v.Devices)), err
	})
	run("vendor fanMode", func() (string, error) {
		v, err := s.FanMode(ctx)
		return fmt.Sprintf("mode=%s source=%s", v.Mode, v.Source), err
	})
	in = s.Info()
	for _, b := range in.Backends {
		fmt.Printf("backend: %s available=%v authenticated=%v detail=%q\n", b.Name, b.Available, b.Authenticated, b.Detail)
	}
}

func step(n string, t0 time.Time, err error, out string) {
	status := "ok"
	if err != nil {
		status = "ERR " + err.Error()
	}
	fmt.Printf("%-26s %6dms %s %s\n", n, time.Since(t0).Milliseconds(), status, out)
}

func fail(n string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", n, err)
	os.Exit(1)
}
