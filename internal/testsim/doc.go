// Package testsim is a HAR-replay BMC simulator plus an in-process proxy
// harness, so the whole of bmcproxy can be tested in CI with no hardware.
//
// # Simulator
//
// A Sim is an httptest TLS server emulating one board+firmware profile:
//
//   - X470() — X470D4U, BMC 2.32.00: legacy /api only, RSA-key-exchange TLS
//     1.0–1.2, Redfish ServiceRoot v1_3_1 reachable but 401
//     Security.1.0.AccessDenied for the admin account (and for session
//     logins), sensors with 0.0 thresholds, no SEL trailer.
//   - B650() — B650D4U, BMC 4.10.00: /api bodies from the 1.10 HAR with the
//     firmware overridden, "NA" thresholds, {"event_log_count":N} SEL
//     trailer, Secure/HttpOnly cookie, and a working Redfish service backed
//     by pkg/protocol/redfish/redfishtest (the live 4.10 resource tree),
//     with mod_evasive's 10-per-IP /redfish cap answered by an HTML 403.
//
// Both profiles emulate the documented /api quirks: form login at
// /api/session issuing QSESSIONID + an 8-char CSRFToken; the auth gate's
// 401 {"cc":7}; CSRF enforced on writes but not on GETs (any non-empty
// token passes, a missing one fails); X-HTTP-Method-Override honoured;
// logs/help-data and the hpm status endpoints crash the backend (HTML 500,
// or 502 with Options.HelpDataStatus) and the 502 handlers do the same,
// each invalidating EVERY session; a session slot cap (login code 15000);
// GlobalLuaTables served unauthenticated; logs/event?LASTEVENTID=n
// incremental SEL; the X470 two-step fan table cursor (select via
// setopenlooptablenumber, read via getfanopenloopcontroltable) with
// interleaving detection; settings/date-time GET/PUT where a partial PUT
// body answers 500; a clock offset reflected in the Date header,
// date-time, Redfish Manager.DateTime and new SEL records; and a power
// state machine (delay, soft-off optionally ignored) shared by /api
// actions/power and Redfish ComputerSystem.Reset. A Redfish PATCH of
// Managers/Self DateTime moves the BMC clock. Stateful ASRock OEM objects
// (oem.go: X470 fan control mode and vetobit; B650 settings/fan/mode,
// ncsilinkup and asrr power restore; network-bond on both) merge writes
// and are readable with OEMObject. /api requests are serialised like the
// single FastCGI worker.
//
// Fault injection: InjectEvasive403(n), SetEvasiveMaxConcurrent(n),
// Restart() (drops all /api and Redfish sessions), SetLatency, Wedge /
// Unwedge (requests hang: timeouts and circuit-breaker tests),
// SetPowerIgnoresSoftOff, SetPowerDelay, SetPower, SetClockOffset,
// AddEvent, Handle (override one route).
//
// Introspection: Stats() (requests, logins, logouts, restarts, CSRF and
// auth rejects, method overrides, evasive 403s, max in-flight overall,
// /api and /redfish — assert MaxInFlightAPI <= 1 to prove per-host /api
// serialisation —, fan interleavings, power actions, ignored soft-offs,
// date-time 500s), Requests(), Count(), LiveSessions(), APISessions().
//
// # Fixtures
//
// fixtures/{x470,b650}/endpoints.json and fixtures/x470/redfish-serviceroot.json
// are generated from docs/*/har-endpoints.json and
// docs/x470/live-redfish-inventory.json by `go generate ./internal/testsim`
// (cmd/extract), which re-sanitizes everything: passwords, CSRF tokens,
// session ids, serials, MACs, UUIDs, host names and e-mails become
// <REDACTED_*> placeholders and private IPv4 addresses map to 192.0.2.0/24.
// cmd/extract's TestFixturesUpToDate fails when they are stale.
//
// # Proxy harness
//
// StartProxy builds the full daemon in-process exactly like cmd/bmcproxy
// (internal/server + sessions + cache + /api/v1 + vendor routes + health +
// metrics, one transport.Manager with test-friendly timeouts) on a loopback
// listener, and shuts it down gracefully at cleanup. It is driver-agnostic:
//
//	sim := testsim.StartB650(t)
//	p := testsim.StartProxy(t, testsim.ProxyOptions{
//		Drivers: func(tm *transport.Manager) []bmc.Driver {
//			return []bmc.Driver{genericredfish.New(tm)}
//		},
//	})
//	r := p.Get(sim, "/api/v1/sensors")
//
// To add ASRock integration tests, return the ASRock driver bound to tm
// from Drivers (e.g. asrock.New(tm), or the registered instance after
// SetTransport — StartProxy also binds any Registry driver implementing
// TransportBinder), and drive StartX470/StartB650 sims through p. The
// /api-specific quirks (cc:7 re-login, help-data restart → re-login, fan
// lock, "NA" thresholds, date-time full-object PUT, LASTEVENTID) then
// become assertions on sim.Stats() and the northbound responses.
package testsim
