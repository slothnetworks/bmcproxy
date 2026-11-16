package redfish_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/redfish/redfishtest"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

func newMgr(t *testing.T) *transport.Manager {
	t.Helper()
	m := transport.NewManager(transport.Config{Retry: transport.RetryPolicy{MaxAttempts: 1}, Breaker: transport.BreakerConfig{Disabled: true}})
	t.Cleanup(m.Close)
	return m
}

func newClient(t *testing.T, srv *redfishtest.Server, o rf.Options) *rf.Client {
	t.Helper()
	ep := newMgr(t).Endpoint(srv.Host(), transport.LaneRedfish)
	if o.Creds.User == "" {
		o.Creds = bmc.Creds{User: "admin", Pass: "secret"}
	}
	c, err := rf.New(ep, o)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func newServer(t *testing.T) *redfishtest.Server {
	t.Helper()
	s := redfishtest.New("admin", "secret")
	t.Cleanup(s.Close)
	return s
}

var ctx = context.Background()

func TestBasicAuthAndErrAuth(t *testing.T) {
	srv := newServer(t)
	c := newClient(t, srv, rf.Options{})
	if err := c.Login(ctx); err != nil {
		t.Fatalf("login: %v", err)
	}
	sys, err := c.GetSystem(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if sys.Manufacturer != "AsrockRack" || sys.PowerState != "Off" || sys.BiosVersion != "10.15" {
		t.Fatalf("system: %+v", sys)
	}
	for _, r := range srv.Reqs() {
		if strings.TrimRight(r.Path, "/") != "/redfish/v1" && !strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
			t.Fatalf("request %s without Basic auth", r.Path)
		}
	}

	bad := newClient(t, srv, rf.Options{Creds: bmc.Creds{User: "admin", Pass: "wrong"}})
	err = bad.Login(ctx)
	if !errors.Is(err, bmc.ErrAuth) || bmc.HTTPStatus(err) != 401 {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	oe, ok := bmc.AsOpError(err)
	if !ok || oe.UpstreamStatus != 401 || oe.Backend != bmc.BackendRedfish || !strings.Contains(oe.Detail, "AccessDenied") {
		t.Fatalf("op error: %+v", oe)
	}
	// ServiceRoot is unauthenticated and still works with bad creds.
	if root, err := bad.ServiceRoot(ctx); err != nil || root.RedfishVersion != "1.15.1" || root.Vendor != "AMI" {
		t.Fatalf("root: %+v %v", root, err)
	}
}

func TestSessionLoginLogoutRelogin(t *testing.T) {
	srv := newServer(t)
	c := newClient(t, srv, rf.Options{Auth: rf.AuthSession})
	if err := c.Login(ctx); err != nil {
		t.Fatal(err)
	}
	if srv.LiveSessions() != 1 || c.SessionLocation() != "/redfish/v1/SessionService/Sessions/1" {
		t.Fatalf("sessions=%d loc=%q", srv.LiveSessions(), c.SessionLocation())
	}
	if _, err := c.GetChassis(ctx, ""); err != nil {
		t.Fatal(err)
	}
	for _, r := range srv.Reqs() {
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("session mode sent Basic on %s", r.Path)
		}
	}
	// Server forgets the session (30 s idle timeout): next call re-logs in once.
	srv.ExpireSessions()
	if _, err := c.GetManager(ctx, ""); err != nil {
		t.Fatalf("after expiry: %v", err)
	}
	if n := srv.Count("POST", "/redfish/v1/SessionService/Sessions"); n != 2 {
		t.Fatalf("logins=%d, want 2", n)
	}
	if err := c.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if srv.LiveSessions() != 0 || srv.Count("DELETE", "/redfish/v1/SessionService/Sessions/2") != 1 {
		t.Fatalf("logout: live=%d", srv.LiveSessions())
	}
	if err := c.Close(ctx); err != nil { // idempotent
		t.Fatal(err)
	}

	bad := newClient(t, srv, rf.Options{Auth: rf.AuthSession, Creds: bmc.Creds{User: "admin", Pass: "nope"}})
	if _, err := bad.GetSystem(ctx, "/redfish/v1/Systems/Self"); !errors.Is(err, bmc.ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
}

func TestSessionProactiveRefresh(t *testing.T) {
	srv := newServer(t)
	now := time.Now()
	clock := func() time.Time { return now }
	c := newClient(t, srv, rf.Options{Auth: rf.AuthSession, Now: clock})
	if err := c.Login(ctx); err != nil {
		t.Fatal(err)
	}
	now = now.Add(28 * time.Second) // SessionTimeout 30 - margin 5 exceeded
	if _, err := c.GetSystem(ctx, "/redfish/v1/Systems/Self"); err != nil {
		t.Fatal(err)
	}
	if n := srv.Count("POST", "/redfish/v1/SessionService/Sessions"); n != 2 {
		t.Fatalf("logins=%d, want 2 (proactive refresh)", n)
	}
	if srv.Count("DELETE", "/redfish/v1/SessionService/Sessions/1") != 1 || srv.LiveSessions() != 1 {
		t.Fatalf("old session not deleted; live=%d", srv.LiveSessions())
	}
}

func TestNextLinkPagingTerminates(t *testing.T) {
	srv := newServer(t)
	c := newClient(t, srv, rf.Options{})
	ch, err := c.GetChassis(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	sensors, err := c.Sensors(ctx, ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(sensors) != 60 {
		t.Fatalf("sensors=%d, want 60", len(sensors))
	}
	seen := map[string]bool{}
	for _, s := range sensors {
		if seen[s.ID] {
			t.Fatalf("duplicate sensor %s", s.ID)
		}
		seen[s.ID] = true
	}
	// The expanded read keeps the nextLink's own $skip=50 and carries the
	// first page's $expand over (the B650 nextLink drops it).
	var follow bool
	for _, r := range srv.Reqs() {
		if r.Path == "/redfish/v1/Chassis/Self/Sensors" && r.RawQuery == "$skip=50&$expand=.($levels=1)" {
			follow = true
		}
	}
	if !follow {
		t.Fatalf("nextLink ?$skip=50 not followed with $expand carried over: %+v", srv.Reqs())
	}
	srv.ResetLog()
	links, err := c.MemberLinks(ctx, "/redfish/v1/Chassis/Self/Sensors")
	if err != nil || len(links) != 60 {
		t.Fatalf("links=%d err=%v", len(links), err)
	}
	// A plain walk (no client query) follows the nextLink verbatim.
	follow = false
	for _, r := range srv.Reqs() {
		if r.Path == "/redfish/v1/Chassis/Self/Sensors" && r.RawQuery == "$skip=50" {
			follow = true
		}
	}
	if !follow {
		t.Fatal("nextLink ?$skip=50 not followed verbatim")
	}
}

// TestExpandedSensorReadRequestCount: a 60-sensor read on the B650 (pages of
// 50, nextLink `?$skip=50` without $expand) costs exactly two collection
// GETs and no per-member GETs.
func TestExpandedSensorReadRequestCount(t *testing.T) {
	srv := newServer(t)
	c := newClient(t, srv, rf.Options{})
	ch, err := c.GetChassis(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	srv.ResetLog()
	sensors, err := c.Sensors(ctx, ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(sensors) != 60 {
		t.Fatalf("sensors=%d, want 60", len(sensors))
	}
	coll, member := 0, 0
	for _, r := range srv.Reqs() {
		switch {
		case r.Path == "/redfish/v1/Chassis/Self/Sensors":
			coll++
		case strings.HasPrefix(r.Path, "/redfish/v1/Chassis/Self/Sensors/"):
			member++
		}
	}
	if coll != 2 || member != 0 {
		t.Fatalf("sensor read: %d collection GETs, %d member GETs (want 2, 0); total %d requests",
			coll, member, len(srv.Reqs()))
	}
	if n := len(srv.Reqs()); n > 3 {
		t.Fatalf("sensor read made %d requests, want <= 3: %+v", n, srv.Reqs())
	}
}

func TestNextLinkLoopGuard(t *testing.T) {
	srv := newServer(t)
	srv.Lock()
	srv.BrokenNextLink = true
	srv.Unlock()
	c := newClient(t, srv, rf.Options{})
	links, err := c.MemberLinks(ctx, "/redfish/v1/Chassis/Self/Sensors")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 50 {
		t.Fatalf("links=%d, want first page only", len(links))
	}
	if n := srv.Count("GET", "/redfish/v1/Chassis/Self/Sensors"); n > 3 {
		t.Fatalf("loop guard failed: %d GETs", n)
	}

	// A server minting a fresh nextLink forever hits the page cap.
	srv.Lock()
	srv.Handlers["/redfish/v1/Endless"] = func(w http.ResponseWriter, r *http.Request) {
		skip := r.URL.Query().Get("$skip")
		n := 0
		fmt.Sscan(skip, &n)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"Members": []any{map[string]any{"@odata.id": fmt.Sprintf("/x/%d", n)}},
			"Members@odata.nextLink": fmt.Sprintf("/redfish/v1/Endless?$skip=%d", n+1)})
	}
	srv.Unlock()
	c2 := newClient(t, srv, rf.Options{MaxPages: 5})
	_, err = c2.MemberLinks(ctx, "/redfish/v1/Endless")
	if !errors.Is(err, bmc.ErrUpstream) || srv.Count("GET", "/redfish/v1/Endless") != 5 {
		t.Fatalf("page cap: err=%v gets=%d", err, srv.Count("GET", "/redfish/v1/Endless"))
	}
}

func TestQueryEscapingOnTheWire(t *testing.T) {
	srv := newServer(t)
	all := rf.QueryCaps{Select: true, Expand: true, ExpandDot: true, Levels: true, MaxLevels: 5, TopSkip: true, Filter: true}
	c := newClient(t, srv, rf.Options{Quirks: rf.Quirks{QueryCaps: &all}})
	var coll rf.Collection
	if _, err := c.GetQuery(ctx, "/redfish/v1/Chassis/Self/Sensors", rf.Query{Top: rf.Int(3)}, &coll); err != nil {
		t.Fatal(err)
	}
	if len(coll.Members) != 3 {
		t.Fatalf("$top=3 returned %d members", len(coll.Members))
	}
	var ch map[string]any
	if _, err := c.GetQuery(ctx, "/redfish/v1/Chassis/Self", rf.Query{Select: []string{"Id"}}, &ch); err != nil {
		t.Fatal(err)
	}
	if _, ok := ch["IndicatorLED"]; ok || ch["Id"] != "Self" {
		t.Fatalf("$select ignored: %v", ch)
	}
	if _, err := c.GetQuery(ctx, "/redfish/v1/AccountService/Accounts", rf.Query{Expand: ".", Levels: 1, Filter: "RoleId eq 'Administrator'"}, &coll); err != nil {
		t.Fatal(err)
	}
	reqs := srv.Reqs()
	raw := map[string]string{}
	for _, r := range reqs {
		raw[r.Path] = r.RawQuery
	}
	if raw["/redfish/v1/Chassis/Self/Sensors"] != "$top=3" {
		t.Fatalf("wire query %q", raw["/redfish/v1/Chassis/Self/Sensors"])
	}
	if raw["/redfish/v1/Chassis/Self"] != "$select=Id" {
		t.Fatalf("wire query %q", raw["/redfish/v1/Chassis/Self"])
	}
	acc := raw["/redfish/v1/AccountService/Accounts"]
	if !strings.Contains(acc, "$expand=.($levels=1)") || !strings.Contains(acc, "$filter=RoleId%20eq%20'Administrator'") {
		t.Fatalf("wire query %q", acc)
	}
	if v := redfishtest.UnescapedQuery(acc); v.Get("$filter") != "RoleId eq 'Administrator'" || v.Get("$expand") != ".($levels=1)" {
		t.Fatalf("server-side decode %v", v)
	}
	if s := (rf.Query{Select: []string{"Id", "Name"}, Top: rf.Int(2), Skip: rf.Int(4)}).Encode(nil); s != "$select=Id,Name&$top=2&$skip=4" {
		t.Fatalf("Encode=%q", s)
	}
}

func TestQueryOptionsDroppedWhenNotAdvertised(t *testing.T) {
	// B650 advertises Select/Expand/Filter but not TopSkipQuery.
	srv := newServer(t)
	c := newClient(t, srv, rf.Options{})
	var coll rf.Collection
	if _, err := c.GetQuery(ctx, "/redfish/v1/Chassis/Self/Sensors", rf.Query{Top: rf.Int(3), Select: []string{"Members"}}, &coll); err != nil {
		t.Fatal(err)
	}
	last := srv.Reqs()[len(srv.Reqs())-1]
	if last.RawQuery != "$select=Members" {
		t.Fatalf("B650 wire query %q (want $top dropped)", last.RawQuery)
	}

	// X470: no ProtocolFeaturesSupported and 501 on unknown params -> nothing sent.
	x := newServer(t)
	x.Lock()
	delete(x.Resources["/redfish/v1"], "ProtocolFeaturesSupported")
	x.StrictQuery = true
	x.Unlock()
	cx := newClient(t, x, rf.Options{})
	if _, err := cx.GetQuery(ctx, "/redfish/v1/Systems/Self", rf.Query{Select: []string{"Id"}, Top: rf.Int(1)}, nil); err != nil {
		t.Fatal(err)
	}
	for _, r := range x.Reqs() {
		if r.RawQuery != "" {
			t.Fatalf("X470 got query %q on %s", r.RawQuery, r.Path)
		}
	}

	// StripQuery quirk wins over advertisement.
	cs := newClient(t, srv, rf.Options{Quirks: rf.Quirks{StripQuery: true}})
	srv.ResetLog()
	if _, err := cs.GetQuery(ctx, "/redfish/v1/Chassis/Self", rf.Query{Select: []string{"Id"}}, nil); err != nil {
		t.Fatal(err)
	}
	if q := srv.Reqs()[0].RawQuery; q != "" {
		t.Fatalf("StripQuery sent %q", q)
	}

	// A service that 501s on an advertised option: one retry without it,
	// then the option is no longer sent.
	srv.Lock()
	srv.RejectAllQuery = true
	srv.Unlock()
	srv.ResetLog()
	if _, err := c.GetQuery(ctx, "/redfish/v1/Chassis/Self", rf.Query{Select: []string{"Id"}}, nil); err != nil {
		t.Fatalf("adaptive retry: %v", err)
	}
	if _, err := c.GetQuery(ctx, "/redfish/v1/Chassis/Self", rf.Query{Select: []string{"Id"}}, nil); err != nil {
		t.Fatal(err)
	}
	r := srv.Reqs()
	if len(r) != 3 || r[0].RawQuery != "$select=Id" || r[1].RawQuery != "" || r[2].RawQuery != "" {
		t.Fatalf("adaptive sequence: %+v", r)
	}
}

func TestETagIfMatchOnPatch(t *testing.T) {
	srv := newServer(t)
	c := newClient(t, srv, rf.Options{})
	ch, err := c.GetChassis(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.ETag(ch.ODataID) != redfishtest.ETagService {
		t.Fatalf("etag=%q", c.ETag(ch.ODataID))
	}
	if _, err := c.SetIndicatorLED(ctx, ch.ODataID, "Lit"); err != nil {
		t.Fatal(err)
	}
	last := srv.Reqs()[len(srv.Reqs())-1]
	if last.Method != "PATCH" || last.Header.Get("If-Match") != redfishtest.ETagService || !strings.Contains(last.Body, `"IndicatorLED":"Lit"`) {
		t.Fatalf("patch: %+v", last)
	}
	if srv.Get("/redfish/v1/Chassis/Self")["IndicatorLED"] != "Lit" {
		t.Fatal("not applied")
	}

	// Per-group ETag: the account subtree uses its own value.
	if _, err := c.Get(ctx, "/redfish/v1/AccountService", nil); err != nil {
		t.Fatal(err)
	}
	if c.ETag("/redfish/v1/AccountService") != redfishtest.ETagAccount {
		t.Fatalf("account etag %q", c.ETag("/redfish/v1/AccountService"))
	}

	// Stale ETag -> 412 -> re-read -> retry succeeds.
	srv.Lock()
	srv.ETags["/redfish/v1/Systems/Self"] = `"999"`
	srv.Unlock()
	if _, err := c.Patch(ctx, "/redfish/v1/Systems/Self", map[string]any{"Boot": map[string]string{"BootSourceOverrideTarget": "Pxe"}},
		rf.WithIfMatch("")); err != nil {
		t.Fatal(err)
	}
	// Server demands a precondition; client has none cached -> 428 -> GET -> retry.
	srv.Lock()
	srv.RequireIfMatch = true
	srv.Unlock()
	srv.ResetLog()
	if _, err := c.Patch(ctx, "/redfish/v1/Systems/Self", map[string]any{"PowerRestorePolicy": "AlwaysOn"}); err != nil {
		t.Fatal(err)
	}
	r := srv.Reqs()
	if len(r) != 3 || r[0].Header.Get("If-Match") != "" || r[1].Method != "GET" || r[2].Header.Get("If-Match") == "" {
		t.Fatalf("428 sequence: %v", r)
	}
	// Explicit stale If-Match is not retried.
	if _, err := c.Patch(ctx, "/redfish/v1/Systems/Self", map[string]any{"PowerRestorePolicy": "AlwaysOff"}, rf.WithIfMatch(`"1"`)); !errors.Is(err, bmc.ErrBusy) || !rf.IsStatus(err, 412) {
		t.Fatalf("explicit stale: %v", err)
	}
}

func TestTaskMonitor(t *testing.T) {
	srv := newServer(t)
	srv.Lock()
	srv.ResetTask = 2
	srv.Unlock()
	c := newClient(t, srv, rf.Options{TaskPollInterval: time.Millisecond})
	sys, err := c.GetSystem(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.ResetSystem(ctx, sys, "On")
	if err != nil {
		t.Fatal(err)
	}
	ref := rf.TaskRef(resp)
	if resp.Status != 202 || ref != "/redfish/v1/TaskService/TaskMonitors/1" {
		t.Fatalf("status=%d ref=%q", resp.Status, ref)
	}
	res, err := c.WaitTask(ctx, ref, rf.WaitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.State() != bmc.TaskCompleted || res.Polls != 3 || res.Task == nil || *res.Task.PercentComplete != 100 {
		t.Fatalf("result: %+v", res)
	}
	bt := res.ToBMCTask("1")
	if bt.State != bmc.TaskCompleted {
		t.Fatalf("bmc task %+v", bt)
	}

	// WaitActionTasks makes the action helper wait.
	c2 := newClient(t, srv, rf.Options{TaskPollInterval: time.Millisecond, WaitActionTasks: true})
	resp, err = c2.ResetSystem(ctx, sys, "ForceOff")
	if err != nil || resp.Status != 200 {
		t.Fatalf("waited action: %v %+v", err, resp)
	}

	// Task URI carried in MessageArgs.
	body := `{"@Message.ExtendedInfo":[{"MessageId":"Task.1.0.New","MessageArgs":["/redfish/v1/TaskService/Tasks/7"]}]}`
	if got := rf.TaskRef(&rf.Response{Status: 200, Body: []byte(body)}); got != "/redfish/v1/TaskService/Tasks/7" {
		t.Fatalf("MessageArgs ref %q", got)
	}

	// Timeout.
	srv.Lock()
	srv.ResetTask = 1000
	srv.Unlock()
	resp, _ = c.ResetSystem(ctx, sys, "On")
	_, err = c.WaitTask(ctx, rf.TaskRef(resp), rf.WaitOptions{Interval: 5 * time.Millisecond, Timeout: 50 * time.Millisecond})
	if !errors.Is(err, bmc.ErrTimeout) {
		t.Fatalf("want timeout, got %v", err)
	}
}

func TestRedirectAndRawDoAndErrors(t *testing.T) {
	srv := newServer(t)
	srv.Lock()
	srv.RedirectNoSlash = true
	srv.Unlock()
	c := newClient(t, srv, rf.Options{})
	var root rf.ServiceRoot
	if _, err := c.Get(ctx, "/redfish/v1", &root); err != nil || root.RedfishVersion != "1.15.1" {
		t.Fatalf("redirect: %v %+v", err, root)
	}
	resp, err := c.Do(ctx, "GET", "/redfish/v1/NoSuchThing12345", nil)
	if err != nil || resp.Status != 404 {
		t.Fatalf("raw: %v %+v", err, resp)
	}
	_, err = c.Get(ctx, "/redfish/v1/NoSuchThing12345", nil)
	if !errors.Is(err, bmc.ErrNotFound) {
		t.Fatalf("want 404 ErrNotFound: %v", err)
	}
	if oe, _ := bmc.AsOpError(err); !strings.Contains(oe.Detail, "ResourceMissingAtURI") {
		t.Fatalf("detail %q", oe.Detail)
	}
	if _, err := c.Get(ctx, "https://evil.example/redfish/v1", nil); !errors.Is(err, bmc.ErrForbidden) {
		t.Fatalf("cross-host: %v", err)
	}
}

func TestQuirksNAAndPathOverride(t *testing.T) {
	srv := newServer(t)
	srv.Set("/redfish/v1/Chassis/Self/Sensors/99", map[string]any{"@odata.id": "/redfish/v1/Chassis/Self/Sensors/99", "Id": "99",
		"Name": "TEMP_X", "ReadingType": "Temperature", "Reading": "NA", "Thresholds": map[string]any{"UpperCritical": map[string]any{"Reading": "NA"}}})
	plain := newClient(t, srv, rf.Options{})
	var s rf.Sensor
	if _, err := plain.Get(ctx, "/redfish/v1/Chassis/Self/Sensors/99", &s); err != nil {
		t.Fatalf("lenient decode failed: %v", err)
	}
	if s.Name != "TEMP_X" || s.Reading != nil {
		t.Fatalf("lenient: %+v", s)
	}
	na := newClient(t, srv, rf.Options{Quirks: rf.Quirks{NAAsNull: true}})
	var m map[string]any
	if _, err := na.Get(ctx, "/redfish/v1/Chassis/Self/Sensors/99", &m); err != nil {
		t.Fatal(err)
	}
	if v, ok := m["Reading"]; !ok || v != nil {
		t.Fatalf("NA not nulled: %v", m["Reading"])
	}

	ov := newClient(t, srv, rf.Options{Quirks: rf.Quirks{PathOverrides: map[string]string{
		rf.PathSystem: "/redfish/v1/Systems/Self", "/redfish/v1/Chassis/Old": "/redfish/v1/Chassis/Self"}}})
	srv.ResetLog()
	p, err := ov.SystemPath(ctx)
	if err != nil || p != "/redfish/v1/Systems/Self" || len(srv.Reqs()) != 0 {
		t.Fatalf("override: %q %v reqs=%d", p, err, len(srv.Reqs()))
	}
	ch, err := ov.GetChassis(ctx, "/redfish/v1/Chassis/Old")
	if err != nil || ch.ID != "Self" {
		t.Fatalf("path override: %v", err)
	}
}

func TestAllowableValuesAndActions(t *testing.T) {
	srv := newServer(t)
	c := newClient(t, srv, rf.Options{})
	sys, _ := c.GetSystem(ctx, "")
	vals := c.AllowableValues(ctx, sys.Actions, "#ComputerSystem.Reset", "ResetType")
	if strings.Join(vals, ",") != "ForceOff,ForceRestart,GracefulShutdown,On,PowerCycle" {
		t.Fatalf("allowable %v", vals)
	}
	mgr, _ := c.GetManager(ctx, "")
	if v := c.AllowableValues(ctx, mgr.Actions, "#Manager.Reset", "ResetType"); len(v) != 1 || v[0] != "ForceRestart" {
		t.Fatalf("manager allowable %v", v)
	}
	ch, _ := c.GetChassis(ctx, "")
	if strings.Join(ch.IndicatorLEDAllowable, ",") != "Lit,Blinking,Off" {
		t.Fatalf("led allowable %v", ch.IndicatorLEDAllowable)
	}
	if _, err := c.ResetSystem(ctx, sys, "Nmi"); !errors.Is(err, bmc.ErrInvalidArgument) {
		t.Fatalf("bad reset type: %v", err)
	}
	if _, err := c.SetNTP(ctx, mgr.NetworkProtocol.ID, rf.NTPUpdate{NTPServers: []string{"10.7.0.1"}}); err != nil {
		t.Fatal(err)
	}
	np, _ := c.GetNetworkProtocol(ctx, mgr)
	if np.NTP == nil || *np.NTP.NTPServers[0] != "10.7.0.1" || !*np.Services["IPMI"].ProtocolEnabled || *np.Services["SSH"].Port != 22 {
		t.Fatalf("network protocol: %+v", np)
	}
	if got := rf.FormatDateTime(time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC), "-05:00"); got != "2026-11-16T10:00:00-05:00" {
		t.Fatalf("FormatDateTime %q", got)
	}
}
