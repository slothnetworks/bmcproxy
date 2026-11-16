package bmc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeDriver struct {
	name     string
	prio     int
	match    Match
	err      error
	calls    *[]string
	credSeen *Creds
}

func (d *fakeDriver) Name() string  { return d.name }
func (d *fakeDriver) Priority() int { return d.prio }
func (d *fakeDriver) Detect(ctx context.Context, t Target) (Match, error) {
	if d.calls != nil {
		*d.calls = append(*d.calls, d.name)
	}
	return d.match, d.err
}
func (d *fakeDriver) Open(ctx context.Context, t Target, c Creds) (Session, error) {
	return &fakeSession{}, nil
}

type credDriver struct{ fakeDriver }

func (d *credDriver) DetectWithCreds(ctx context.Context, t Target, c Creds) (Match, error) {
	d.credSeen = &c
	return Match{Score: ScoreCertain, Board: "X470D4U"}, nil
}

type fakeSession struct{ caps map[Capability]string }

func (s *fakeSession) Close(context.Context) error { return nil }
func (s *fakeSession) Info() SessionInfo           { return SessionInfo{Driver: "fake", Capabilities: s.caps} }
func (s *fakeSession) PowerState(context.Context) (PowerState, error) {
	return PowerOn, nil
}
func (s *fakeSession) Power(context.Context, PowerAction) error { return nil }
func (s *fakeSession) Sensors(context.Context) ([]Sensor, error) {
	return nil, Unsupported("fake", "Sensors")
}

func TestResolvePriorityAndCertain(t *testing.T) {
	var calls []string
	r := NewRegistry()
	r.Register(&fakeDriver{name: "generic", prio: -10, match: Match{Score: ScoreFallback}, calls: &calls})
	r.Register(&fakeDriver{name: "asrock", prio: 10, match: Match{Score: ScoreCertain, Board: "B650D4U"}, calls: &calls})
	r.Register(&fakeDriver{name: "other", match: Match{}, calls: &calls})

	names := []string{}
	for _, d := range r.Drivers() {
		names = append(names, d.Name())
	}
	if strings.Join(names, ",") != "asrock,other,generic" {
		t.Fatalf("order %v", names)
	}
	d, m, err := r.Resolve(context.Background(), Target{Host: "h"}, nil)
	if err != nil || d.Name() != "asrock" || m.Vendor != "asrock" || m.Board != "B650D4U" {
		t.Fatalf("got %v %+v %v", d, m, err)
	}
	if strings.Join(calls, ",") != "asrock" {
		t.Fatalf("certain match should short-circuit, calls=%v", calls)
	}
}

func TestResolveBestScoreAndFallback(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeDriver{name: "generic", prio: -10, match: Match{Score: ScoreFallback}})
	r.Register(&fakeDriver{name: "asrock", prio: 10, err: errors.New("conn refused")})
	r.Register(&fakeDriver{name: "likely", match: Match{Score: ScoreLikely}})
	d, _, err := r.Resolve(context.Background(), Target{Host: "h"}, nil)
	if err != nil || d.Name() != "likely" {
		t.Fatalf("got %v %v", d, err)
	}
}

func TestResolveVendorOverride(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeDriver{name: "asrock", err: errors.New("detect failed")})
	r.Register(&fakeDriver{name: "generic", match: Match{Score: ScoreCertain}})
	d, m, err := r.Resolve(context.Background(), Target{Host: "h", VendorHint: "ASRock"}, nil)
	if err != nil || d.Name() != "asrock" || m.Vendor != "asrock" {
		t.Fatalf("got %v %+v %v", d, m, err)
	}
	_, _, err = r.Resolve(context.Background(), Target{Host: "h", VendorHint: "nope"}, nil)
	if HTTPStatus(err) != 400 {
		t.Fatalf("unknown vendor: %v -> %d", err, HTTPStatus(err))
	}
}

func TestResolveNoMatch(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeDriver{name: "a"})
	_, _, err := r.Resolve(context.Background(), Target{Host: "h"}, nil)
	if !errors.Is(err, ErrUnsupported) || HTTPStatus(err) != 501 {
		t.Fatalf("got %v", err)
	}
	// every driver errored with a timeout -> 504 not 501
	r = NewRegistry()
	r.Register(&fakeDriver{name: "a", err: fmt.Errorf("dial: %w", context.DeadlineExceeded)})
	r.Register(&fakeDriver{name: "b", err: errors.New("reset")})
	_, _, err = r.Resolve(context.Background(), Target{Host: "h"}, nil)
	if HTTPStatus(err) != 504 {
		t.Fatalf("got %v -> %d", err, HTTPStatus(err))
	}
	// mixed clean no + error -> 501
	r = NewRegistry()
	r.Register(&fakeDriver{name: "a"})
	r.Register(&fakeDriver{name: "b", err: errors.New("x")})
	_, _, err = r.Resolve(context.Background(), Target{Host: "h"}, nil)
	if HTTPStatus(err) != 501 {
		t.Fatalf("got %v -> %d", err, HTTPStatus(err))
	}
}

func TestResolveCredDetector(t *testing.T) {
	r := NewRegistry()
	cd := &credDriver{fakeDriver{name: "asrock"}}
	r.Register(cd)
	c := Creds{User: "admin", Pass: "pw"}
	d, m, err := r.Resolve(context.Background(), Target{Host: "h"}, &c)
	if err != nil || d.Name() != "asrock" || m.Board != "X470D4U" || cd.credSeen == nil || cd.credSeen.User != "admin" {
		t.Fatalf("got %v %+v %v", d, m, err)
	}
}

func TestRegisterPanics(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeDriver{name: "x"})
	for _, d := range []Driver{nil, &fakeDriver{name: ""}, &fakeDriver{name: "X"}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic for %v", d)
				}
			}()
			r.Register(d)
		}()
	}
}

func TestHTTPStatusTable(t *testing.T) {
	cases := []struct {
		err  error
		want int
		cls  string
	}{
		{nil, 200, ""},
		{ErrUnsupported, 501, "unsupported"},
		{ErrAuth, 401, "auth"},
		{ErrUpstream, 502, "upstream"},
		{ErrBusy, 503, "busy"},
		{ErrTimeout, 504, "timeout"},
		{ErrDisruptiveDenied, 403, "disruptive_denied"},
		{ErrForbidden, 403, "forbidden"},
		{ErrInvalidArgument, 400, "invalid_argument"},
		{ErrNotFound, 404, "not_found"},
		{context.DeadlineExceeded, 504, "timeout"},
		{context.Canceled, 499, "canceled"},
		{errors.New("boom"), 500, "internal"},
		{&OpError{Pkg: "amimegarac", Op: "Login", UpstreamStatus: 401, Detail: `{"cc":7}`, Err: ErrAuth}, 401, "auth"},
		{fmt.Errorf("outer: %w", &OpError{Pkg: "redfish", Op: "GET", Err: fmt.Errorf("%w: %w", ErrBusy, errors.New("evasive 403"))}), 503, "busy"},
		{Wrap("transport", "Do", fmt.Errorf("x: %w", context.DeadlineExceeded)), 504, "timeout"},
		{Wrap("transport", "Do", errors.New("connection reset")), 502, "upstream"},
		{Unsupported("driver.asrock", "BootOverride"), 501, "unsupported"},
	}
	for i, c := range cases {
		if got := HTTPStatus(c.err); got != c.want {
			t.Errorf("%d: HTTPStatus(%v)=%d want %d", i, c.err, got, c.want)
		}
		if got := Class(c.err); got != c.cls {
			t.Errorf("%d: Class(%v)=%q want %q", i, c.err, got, c.cls)
		}
	}
	if Wrap("p", "o", nil) != nil {
		t.Error("Wrap(nil) != nil")
	}
}

func TestOpErrorFormatting(t *testing.T) {
	e := &OpError{Pkg: "amimegarac", Op: "GET /api/sensors", Backend: BackendAMI, UpstreamStatus: 401, Detail: `{"cc":7}`, Err: ErrAuth}
	want := `amimegarac GET /api/sensors [ami]: BMC rejected credentials (upstream 401): {"cc":7}`
	if e.Error() != want {
		t.Fatalf("got %q", e.Error())
	}
	oe, ok := AsOpError(fmt.Errorf("wrap: %w", e))
	if !ok || oe.Backend != BackendAMI || !errors.Is(oe, ErrAuth) {
		t.Fatal("AsOpError")
	}
}

func TestDescribe(t *testing.T) {
	rep := Describe(&fakeSession{caps: map[Capability]string{CapPower: BackendAMI, CapSensors: ""}})
	got := map[Capability]CapabilityReport{}
	for _, r := range rep {
		got[r.Capability] = r
	}
	if r := got[CapPower]; !r.Supported || r.Backend != BackendAMI {
		t.Errorf("power %+v", r)
	}
	if got[CapSensors].Supported || got[CapFRU].Supported {
		t.Error("sensors/fru should be unsupported")
	}
	rep = Describe(&fakeSession{})
	for _, r := range rep {
		if r.Capability == CapPower && !r.Supported {
			t.Error("nil caps map: implemented => supported")
		}
	}
}

func TestParseTarget(t *testing.T) {
	cases := map[string]Target{
		"192.0.2.207":      {Host: "192.0.2.207"},
		"192.0.2.207:8443": {Host: "192.0.2.207", Port: 8443},
		"bmc.example":      {Host: "bmc.example"},
		"[fe80::1]:444":    {Host: "fe80::1", Port: 444},
		"fe80::1":          {Host: "fe80::1"},
	}
	for in, want := range cases {
		got, err := ParseTarget(in)
		if err != nil || got != want {
			t.Errorf("%s: %+v %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "http://x", "u@h", "h:0", "h:x"} {
		if _, err := ParseTarget(bad); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%q: expected invalid, got %v", bad, err)
		}
	}
	tg := Target{Host: "fe80::1"}
	if tg.BaseURL() != "https://[fe80::1]" || tg.Addr() != "[fe80::1]:443" || tg.IPMIAddr() != "[fe80::1]:623" {
		t.Errorf("%s %s", tg.BaseURL(), tg.Addr())
	}
	if (Target{Host: "h", Port: 8443}).BaseURL() != "https://h:8443" {
		t.Error("baseurl port")
	}
}

func TestCredsNeverPrintPassword(t *testing.T) {
	c := Creds{User: "admin", Pass: "hunter2"}
	for _, s := range []string{fmt.Sprint(c), fmt.Sprintf("%v %+v %#v %s", c, c, c, c), c.LogValue().String()} {
		if strings.Contains(s, "hunter2") {
			t.Fatalf("password leaked: %s", s)
		}
	}
}

func TestRouteHelpers(t *testing.T) {
	r := Route{Method: "POST", Path: "disableBonding", Tier: TierDisruptive}
	if r.Pattern("/api/v1/vendor/asrock/") != "POST /api/v1/vendor/asrock/disableBonding" {
		t.Error(r.Pattern("/api/v1/vendor/asrock/"))
	}
	if (Route{Path: "raw/api/{path...}"}).Pattern("/v") != "/v/raw/api/{path...}" {
		t.Error("any method pattern")
	}
	var tier Tier
	if err := tier.UnmarshalText([]byte("disruptive")); err != nil || tier != TierDisruptive {
		t.Error("tier parse")
	}
	rr := httptest.NewRecorder()
	_ = (&RawResponse{Status: 202, Header: http.Header{"X-A": {"1"}}, Body: []byte("ok")}).WriteResponse(rr)
	if rr.Code != 202 || rr.Body.String() != "ok" || rr.Header().Get("X-A") != "1" {
		t.Error("raw response")
	}
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"interface":"eth0","bogus":1}`))
	var body struct{ Interface string }
	if err := DecodeJSON(req, &body); !errors.Is(err, ErrInvalidArgument) {
		t.Error("unknown field should be rejected")
	}
}

func TestTimestamp(t *testing.T) {
	raw := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := NewTimestamp(raw, 2*time.Hour) // BMC two hours fast
	if !ts.Corrected.Equal(raw.Add(-2*time.Hour)) || !ts.Raw.Equal(raw) {
		t.Fatalf("%+v", ts)
	}
	if !NewTimestamp(time.Time{}, time.Hour).Raw.IsZero() {
		t.Fatal("zero raw")
	}
	if _, err := ParsePowerAction("Cycle"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePowerAction("explode"); HTTPStatus(err) != 400 {
		t.Fatal("bad action should be 400")
	}
}
