package asrock

import (
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/boards"
)

// Raw passthroughs: ANY raw/api/{path...} and ANY raw/redfish/{path...}.
//
// Tier is decided per request before a session is opened (Route.TierFor):
//
//   - denylisted paths (amimegarac.DefaultDenyRules plus every board
//     profile's extras: logs/help-data, asrr/mca-file, GlobalLuaTables, ...)
//     and the session endpoints are refused with 403 forbidden whatever the
//     tier configuration;
//   - GET/HEAD/OPTIONS are read;
//   - DELETE is disruptive;
//   - other writes are disruptive when the path looks like flash, firmware,
//     restore, reset, factory default, network, user or log-clear
//     operations, else write.

// maxRawBody caps a passthrough request body (raw routes have no server
// body limit so firmware-sized uploads are possible later).
const maxRawBody = 64 << 20

// rawDeny is the static denylist used by TierFor (the session client adds
// the same defaults plus its board's extras at send time).
var rawDeny = amimegarac.NewDenylist(allExtraDeny()...)

// allExtraDeny unions the board profiles' extra deny rules.
func allExtraDeny() []amimegarac.DenyRule {
	var out []amimegarac.DenyRule
	for _, p := range []boards.Profile{boards.X470D4U(""), boards.B650D4U(""), boards.B550D4U(""), boards.Unknown("")} {
		out = append(out, p.ExtraDeny...)
	}
	return out
}

// amiDisruptive / redfishDisruptive are lower-case substrings that make a
// write disruptive.
var amiDisruptive = []string{
	"flash", "firmware", "upgrade", "upload", "hpm", "maintenance/", "restore", "reset", "factory",
	"reboot", "bmccoldreset", "settings/network", "network-bond", "ncsi", "settings/users", "user",
	"logs/", "clearmcalog", "certificate", "ssl", "ldap", "radius", "active_directory",
}

var redfishDisruptive = []string{
	"updateservice", "accountservice", "sessionservice", "ethernetinterfaces", "networkprotocol",
	"certificate", ".reset", "resettodefaults", "clearlog", "simpleupdate", "firmware",
}

func readMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func classify(method, p string, disruptive []string) bmc.Tier {
	switch {
	case readMethod(method):
		return bmc.TierRead
	case method == http.MethodDelete:
		return bmc.TierDisruptive
	}
	lp := strings.ToLower(p)
	for _, d := range disruptive {
		if strings.Contains(lp, d) {
			return bmc.TierDisruptive
		}
	}
	return bmc.TierWrite
}

func forbidden(op, format string, args ...any) error {
	return bmc.Errorf(bmc.ErrForbidden, Pkg, op, format, args...)
}

// rawAPIPath canonicalises the {path...} of raw/api.
func rawAPIPath(r *http.Request) (string, error) {
	cp, _, err := amimegarac.CanonicalPath(r.PathValue("path"))
	if err != nil {
		return "", bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "raw/api", "%v", err)
	}
	lp := strings.ToLower(cp)
	if rule, ok := rawDeny.Match(cp); ok {
		return "", forbidden("raw/api", "%s is denylisted: %s", cp, rule.Reason)
	}
	if lp == "/api/session" || strings.HasPrefix(lp, "/api/session/") {
		return "", forbidden("raw/api", "%s would log out or replace the proxy's shared session", cp)
	}
	return cp, nil
}

// rawRedfishPath canonicalises the {path...} of raw/redfish to
// /redfish/v1/....
func rawRedfishPath(r *http.Request) (string, error) {
	raw := r.PathValue("path")
	dec, err := url.PathUnescape(raw)
	if err != nil || strings.ContainsAny(dec, "%\\") || strings.ContainsFunc(dec, func(c rune) bool { return c < 0x20 || c == 0x7f }) {
		return "", bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "raw/redfish", "bad path %q", raw)
	}
	dec = strings.TrimPrefix(strings.TrimPrefix(dec, "/"), "redfish/v1")
	p := path.Clean("/redfish/v1/" + dec)
	if p != "/redfish/v1" && !strings.HasPrefix(p, "/redfish/v1/") {
		return "", bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "raw/redfish", "path escapes /redfish/v1: %q", raw)
	}
	if rule, ok := rawDeny.Match(p); ok {
		return "", forbidden("raw/redfish", "%s is denylisted: %s", p, rule.Reason)
	}
	lp := strings.ToLower(p)
	if strings.HasPrefix(lp, "/redfish/v1/sessionservice/sessions") && r.Method != http.MethodGet {
		return "", forbidden("raw/redfish", "session create/delete is reserved to the proxy")
	}
	return p, nil
}

func rawAPITier(r *http.Request) (bmc.Tier, error) {
	cp, err := rawAPIPath(r)
	if err != nil {
		return 0, err
	}
	return classify(r.Method, cp, amiDisruptive), nil
}

func rawRedfishTier(r *http.Request) (bmc.Tier, error) {
	p, err := rawRedfishPath(r)
	if err != nil {
		return 0, err
	}
	return classify(r.Method, p, redfishDisruptive), nil
}

func rawRoutes() []bmc.Route {
	return []bmc.Route{
		{Path: "raw/api/{path...}", Tier: bmc.TierDisruptive, TierFor: rawAPITier, Handler: withSession(handleRawAPI),
			Summary: "Authenticated passthrough to the AMI /api (tier by method/path; denylist enforced)"},
		{Path: "raw/redfish/{path...}", Tier: bmc.TierDisruptive, TierFor: rawRedfishTier, Handler: withSession(handleRawRedfish),
			Summary: "Authenticated passthrough to the BMC Redfish service (tier by method/path; denylist enforced)"},
	}
}

func readRawBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, maxRawBody+1))
	if err != nil {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "raw", "read body: %v", err)
	}
	if len(b) > maxRawBody {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "raw", "body larger than %d bytes", maxRawBody)
	}
	return b, nil
}

// passHeaders are copied from the BMC response.
var passHeaders = []string{"Content-Type", "Content-Disposition", "ETag", "Location", "Allow", "OData-Version"}

func rawResponse(status int, h http.Header, body []byte) *bmc.RawResponse {
	out := http.Header{}
	for _, k := range passHeaders {
		if v := h.Values(k); len(v) > 0 {
			out[k] = append([]string(nil), v...)
		}
	}
	return &bmc.RawResponse{Status: status, Header: out, Body: body}
}

// handleRawAPI serves ANY /api/v1/vendor/asrock/raw/api/{path...}.
//
//	@Summary		Raw AMI /api passthrough
//	@Description	Tier: **dynamic** — GET/HEAD/OPTIONS read; DELETE disruptive; POST/PUT/PATCH disruptive when the path matches flash/firmware/upgrade/upload/hpm/maintenance/restore/reset/factory/network/ncsi/users/logs, else write. Sends the request through the proxy's authenticated AMI session (CSRF token, cookie, cc:7 re-login) to `/api/{path}` with the query string and body as given; the BMC status, body and content type are returned verbatim. Always refused (403 forbidden): `logs/help-data`, `asrr/mca-file`, `settings/download_image`, `maintenance/download_config`, `hpm/*status`, GlobalLuaTables, X470 `asrr/java-console`, and `session` (would log out the shared session). Method-override headers are stripped. Both boards (B650 `/api` works alongside Redfish).
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			X-BMC-Confirm	header		string	false	"true, required for disruptive requests (with --allow-disruptive)"
//	@Param			path			path		string	true	"path under /api, e.g. chassis-status or asrr/fw-info"
//	@Param			body			body		object	false	"forwarded verbatim"
//	@Success		200				{object}	object	"BMC response, verbatim (any status is passed through)"
//	@Failure		400				{object}	server.ErrorBody	"bad path"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"denylisted path / tier gate"
//	@Failure		502				{object}	server.ErrorBody	"BMC error"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/raw/api/{path} [get]
//	@Router			/api/v1/vendor/asrock/raw/api/{path} [post]
//	@Router			/api/v1/vendor/asrock/raw/api/{path} [put]
//	@Router			/api/v1/vendor/asrock/raw/api/{path} [patch]
//	@Router			/api/v1/vendor/asrock/raw/api/{path} [delete]
func handleRawAPI(s *Session, r *http.Request) (any, error) {
	cp, err := rawAPIPath(r)
	if err != nil {
		return nil, err
	}
	body, err := readRawBody(r)
	if err != nil {
		return nil, err
	}
	p := cp
	if r.URL.RawQuery != "" {
		p += "?" + r.URL.RawQuery
	}
	req := &amimegarac.Request{
		Method: r.Method, Path: p, Body: body, ContentType: r.Header.Get("Content-Type"),
		Idempotent: readMethod(r.Method), Op: "raw " + r.Method + " " + cp,
	}
	if len(body) == 0 {
		req.ContentType = ""
	}
	resp, err := s.api.Do(r.Context(), req)
	if err != nil {
		return nil, err
	}
	return rawResponse(resp.Status, resp.Header, resp.Body), nil
}

// handleRawRedfish serves ANY /api/v1/vendor/asrock/raw/redfish/{path...}.
//
//	@Summary		Raw Redfish passthrough
//	@Description	Tier: **dynamic** — GET/HEAD/OPTIONS read; DELETE disruptive; POST/PATCH/PUT disruptive for UpdateService, AccountService, SessionService, EthernetInterfaces, NetworkProtocol, certificates, *.Reset, ResetToDefaults, ClearLog, SimpleUpdate, firmware, else write. Forwards to `/redfish/v1/{path}` with the proxy's authenticated Redfish client (If-Match / Content-Type forwarded); the BMC status and body are returned verbatim. B650: available. X470: 501 — Redfish keeps a separate account store and the driver does not use it (unless built with `-tags x470_pam` and PAM was enabled). Refused (403): GlobalLuaTables (unauthenticated, DELETE clears the Lua cache) and session create/delete.
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			X-BMC-Confirm	header		string	false	"true, required for disruptive requests (with --allow-disruptive)"
//	@Param			path			path		string	true	"path under /redfish/v1, e.g. Systems/Self"
//	@Param			body			body		object	false	"forwarded verbatim"
//	@Success		200				{object}	object	"BMC response, verbatim (any status is passed through)"
//	@Failure		400				{object}	server.ErrorBody	"bad path"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"denylisted path / tier gate"
//	@Failure		501				{object}	server.ErrorBody	"Redfish not in use on this board (X470)"
//	@Failure		502				{object}	server.ErrorBody	"BMC error"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/raw/redfish/{path} [get]
//	@Router			/api/v1/vendor/asrock/raw/redfish/{path} [post]
//	@Router			/api/v1/vendor/asrock/raw/redfish/{path} [put]
//	@Router			/api/v1/vendor/asrock/raw/redfish/{path} [patch]
//	@Router			/api/v1/vendor/asrock/raw/redfish/{path} [delete]
func handleRawRedfish(s *Session, r *http.Request) (any, error) {
	p, err := rawRedfishPath(r)
	if err != nil {
		return nil, err
	}
	if s.rfClient == nil {
		return nil, &bmc.OpError{Pkg: Pkg, Op: "raw/redfish", Backend: bmc.BackendRedfish,
			Detail: "Redfish is not in use on " + s.prof.Key + " (separate Redfish account store)", Err: bmc.ErrUnsupported}
	}
	if err := s.ami.SessionLostError("raw/redfish"); err != nil {
		return nil, err
	}
	body, err := readRawBody(r)
	if err != nil {
		return nil, err
	}
	if r.URL.RawQuery != "" {
		p += "?" + r.URL.RawQuery
	}
	var opts []rf.RequestOption
	opts = append(opts, rf.WithOp("raw "+r.Method))
	if ct := r.Header.Get("Content-Type"); ct != "" && len(body) > 0 {
		opts = append(opts, rf.WithHeader("Content-Type", ct))
	}
	if im := r.Header.Get("If-Match"); im != "" {
		opts = append(opts, rf.WithIfMatch(im))
	}
	var b any
	if len(body) > 0 {
		b = body
	}
	resp, err := s.rfClient.Do(r.Context(), r.Method, p, b, opts...)
	if err != nil {
		return nil, err
	}
	return rawResponse(resp.Status, resp.Header, resp.Body), nil
}
