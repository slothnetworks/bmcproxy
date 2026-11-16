// Package amimegarac is the client for the AMI MegaRAC SPX legacy "/api"
// web API. Every MegaRAC-based vendor (ASRock Rack, Gigabyte, ASUS, ...)
// shares it; vendor drivers build their capability methods on top.
//
// # Session model
//
// A Client owns at most one BMC web session at a time:
//
//   - Login is a form POST to /api/session (username, password, plus optional
//     profile fields such as certlogin=0 on X470-class firmware). The
//     response sets the QSESSIONID cookie and carries a CSRFToken in its
//     JSON body. Login takes 1.5–3 s on real hardware, so it happens lazily
//     on the first request and is then reused.
//   - Every request sends Cookie: QSESSIONID=... and X-CSRFTOKEN. GETs do
//     not enforce the token, but writes do.
//   - Logout is DELETE /api/session.
//
// The client is safe for concurrent use. Logins are singleflighted: if many
// goroutines see the session die at once, exactly one login runs and the
// rest wait for it.
//
// # Re-login and retry rule
//
// An HTTP 401 (normally {"cc":7,"error":"Invalid Authentication"}) means
// the session is gone: idle timeout, a backend restart (spx_restservice
// crash, which kills every session on the host), or a session ended
// elsewhere. The client drops the session, logs in again once, and retries
// the request once, if one of these holds:
//
//  1. The method is GET, HEAD or OPTIONS.
//  2. The request is marked idempotent (Request.Idempotent, or
//     transport.WithIdempotent on the context).
//  3. The 401 body is the AMI auth-gate rejection {"cc":7,...}. The gate
//     checks session and CSRF before it dispatches to any handler, so a
//     cc:7 proves the BMC did not act on the write.
//
// A write rejected with any other 401 body still renews the session, but it
// is not replayed: it fails with an error wrapping bmc.ErrAuth. If the retry
// also gets a 401, or the re-login fails, the call fails. A login rejection
// wraps bmc.ErrAuth. Transport failures during login keep their own class
// (timeout, busy).
//
// Re-logins are reported through Hooks.OnRelogin and counted in Stats. For
// Config.DegradedFor afterwards, Degraded reports true.
//
// # Backend restarts
//
// A 502 means lighttpd lost its FastCGI backend. An HTML (non-JSON) 500
// is the backend-crash page, as seen after logs/help-data. Either one
// probably invalidated every session on the host. The client reports it
// through Hooks.OnHostInvalidated so a session cache can drop all sessions
// for that host. The current call fails with bmc.ErrUpstream, and the next
// request logs in again.
//
// With Config.ReportSessionLost the signal is sticky instead: the current
// call and every later Do (Get, Clock, ...) fail with an error wrapping
// bmc.ErrSessionLost, without contacting the BMC, until an explicit Login.
// MarkHostLost enters the same state on request (e.g. after a BMC reset).
//
// # Denylist
//
// Some endpoints crash the BMC web backend or leak data. The client refuses
// them for every method, including the raw passthrough (Do), before
// anything is sent. Refusals wrap bmc.ErrForbidden. Paths are canonicalised
// (percent-decoded, cleaned, forced under /api/, lower-cased) before
// matching, so "..", "//" and encoding tricks cannot bypass the list. The
// defaults are DefaultDenyRules. Board profiles can add rules through
// Config.ExtraDeny or Client.Denylist().Add, but nothing can remove them.
//
// # Lenient decoding
//
// MegaRAC JSON is loosely typed: numbers arrive as strings, unset
// thresholds are "NA", absent values are "N/A", FRU strings carry a literal
// "\n", and disabled ports are negative ints with bit 31 set. NullableFloat,
// NullableInt, FlexString, FlexBool, CleanString and DecodePort deal with
// these, and vendor drivers can use them directly.
//
// # Observability
//
// Login, re-login and logout are logged at DEBUG under the "amimegarac"
// subsystem (obs.SubAMI). Host invalidation is logged at WARN. Wire dumps
// (TRACE) and HAR capture come from pkg/transport. Session cookies and
// CSRF tokens are never logged.
package amimegarac
