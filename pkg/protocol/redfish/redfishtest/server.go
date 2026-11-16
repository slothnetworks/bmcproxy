// Package redfishtest is an in-memory fake Redfish service for tests. Its
// default fixture mirrors the live B650D4U (BMC 4.10.00) shapes recorded in
// docs/b650/live-probe.md and docs/b650/live-redfish-inventory.json:
// ServiceRoot v1_7_0 / RedfishVersion 1.15.1 / Vendor AMI, Systems/Self,
// Chassis/Self (IndicatorLED Lit/Blinking/Off, 60 Sensors paged at 50),
// Managers/Self (SEL with 11 entries, AuditLog, EventLog), per-group ETags,
// SessionService (SessionTimeout 30) and ResetActionInfo allowables.
//
// Behaviour knobs reproduce the X470 quirks too (501 on unknown query
// parameters, /redfish/v1 302, no ProtocolFeaturesSupported).
package redfishtest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Service-wide and per-group ETags as seen on the live B650.
const (
	ETagService = `"1793662187"`
	ETagAccount = `"1641013241"`
	ETagSEL     = `"1793662044"`
)

// Request is one recorded request.
type Request struct {
	Method   string
	Path     string
	RawQuery string
	Header   http.Header
	Body     string
}

// Server is a fake Redfish service. Exported fields may be changed before
// or between requests (guarded by Lock/Unlock when racing).
type Server struct {
	*httptest.Server

	mu sync.Mutex

	User, Pass string

	// Resources maps a path (no trailing slash) to its JSON body.
	Resources map[string]map[string]any
	// ETags per path (defaults to ETagService).
	ETags map[string]string

	// PageSize for collections (default 50, like the B650).
	PageSize int
	// StrictQuery makes unknown/unsupported query parameters answer 501
	// Base.1.0.QueryNotSupported (X470 behaviour).
	StrictQuery bool
	// RejectAllQuery 501s any query parameter (to test the adaptive retry).
	RejectAllQuery bool
	// RedirectNoSlash answers /redfish/v1 with 302 -> /redfish/v1/ (X470).
	RedirectNoSlash bool
	// RequireIfMatch answers 428 to PATCH without If-Match.
	RequireIfMatch bool
	// BrokenNextLink makes collections always return a nextLink to
	// themselves (loop guard test).
	BrokenNextLink bool
	// SessionIdle is the idle timeout of sessions (0 = never).
	SessionIdle time.Duration
	// ResetTask: when > 0, ComputerSystem.Reset answers 202 with a task
	// monitor that reports running for ResetTask polls.
	ResetTask int
	// ClockSkew is added to Managers/Self DateTime. A PATCH of
	// Managers/Self DateTime sets it (the BMC clock moves).
	ClockSkew time.Duration
	// OnClockSet, if set, is called (with the server lock held) after a
	// DateTime PATCH changed ClockSkew.
	OnClockSet func(skew time.Duration)

	Requests []Request
	sessions map[string]*session
	nextSess int
	tasks    map[string]*task
	Actions  []Request // POSTs to /Actions/
	Handlers map[string]http.HandlerFunc
	now      func() time.Time
}

type session struct {
	id       string
	token    string
	lastUsed time.Time
}

type task struct {
	remaining int
	final     int
}

// New starts a TLS fake with the B650 fixture and credentials user/pass.
func New(user, pass string) *Server {
	s := &Server{User: user, Pass: pass, PageSize: 50, sessions: map[string]*session{}, tasks: map[string]*task{},
		Handlers: map[string]http.HandlerFunc{}, ETags: map[string]string{}, now: time.Now}
	s.Resources = B650Fixture()
	s.ETags["/redfish/v1/AccountService"] = ETagAccount
	s.ETags["/redfish/v1/AccountService/Accounts"] = ETagAccount
	for _, id := range []string{"1", "2", "4"} {
		s.ETags["/redfish/v1/AccountService/Accounts/"+id] = ETagAccount
	}
	s.ETags["/redfish/v1/Managers/Self/LogServices/SEL"] = ETagSEL
	s.ETags["/redfish/v1/Managers/Self/LogServices/SEL/Entries"] = ETagSEL
	s.Server = httptest.NewTLSServer(http.HandlerFunc(s.serve))
	return s
}

// Host returns "127.0.0.1:port".
func (s *Server) Host() string { return s.Listener.Addr().String() }

// Lock / Unlock guard the exported fields.
func (s *Server) Lock()   { s.mu.Lock() }
func (s *Server) Unlock() { s.mu.Unlock() }

// Reqs returns a copy of the recorded requests.
func (s *Server) Reqs() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.Requests...)
}

// ResetLog clears recorded requests and actions.
func (s *Server) ResetLog() {
	s.mu.Lock()
	s.Requests, s.Actions = nil, nil
	s.mu.Unlock()
}

// Count returns how many recorded requests match method and path prefix.
func (s *Server) Count(method, pathPrefix string) int {
	n := 0
	for _, r := range s.Reqs() {
		if (method == "" || r.Method == method) && strings.HasPrefix(r.Path, pathPrefix) {
			n++
		}
	}
	return n
}

// LiveSessions returns the number of live sessions.
func (s *Server) LiveSessions() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

// ExpireSessions drops every session (simulating the 30 s timeout).
func (s *Server) ExpireSessions() {
	s.mu.Lock()
	s.sessions = map[string]*session{}
	s.mu.Unlock()
}

// Get returns a copy of the resource at path.
func (s *Server) Get(path string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.Resources[path])
}

// Set replaces a resource.
func (s *Server) Set(path string, v map[string]any) {
	s.mu.Lock()
	s.Resources[path] = v
	s.mu.Unlock()
}

func clone(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	b, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("OData-Version", "4.0")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ErrorBody builds a DMTF error envelope.
func ErrorBody(id, msg string, args ...string) map[string]any {
	if args == nil {
		args = []string{}
	}
	return map[string]any{"error": map[string]any{"code": id, "message": msg,
		"@Message.ExtendedInfo": []any{map[string]any{"MessageId": id, "Message": msg, "MessageArgs": args, "Severity": "Critical"}}}}
}

func (s *Server) authorized(r *http.Request) bool {
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Basic ") {
		b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(a, "Basic "))
		return err == nil && string(b) == s.User+":"+s.Pass
	}
	if tok := r.Header.Get("X-Auth-Token"); tok != "" {
		for _, se := range s.sessions {
			if se.token == tok {
				if s.SessionIdle > 0 && s.now().Sub(se.lastUsed) > s.SessionIdle {
					delete(s.sessions, se.id)
					return false
				}
				se.lastUsed = s.now()
				return true
			}
		}
	}
	return false
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body := ""
	if r.Body != nil {
		b := make([]byte, 0, 512)
		buf := make([]byte, 512)
		for {
			n, err := r.Body.Read(buf)
			b = append(b, buf[:n]...)
			if err != nil {
				break
			}
		}
		body = string(b)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Requests = append(s.Requests, Request{Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery, Header: r.Header.Clone(), Body: body})

	if h, ok := s.Handlers[r.URL.Path]; ok {
		s.mu.Unlock()
		h(w, r)
		s.mu.Lock()
		return
	}

	path := r.URL.Path
	if path == "/redfish/v1" && s.RedirectNoSlash {
		w.Header().Set("Location", "/redfish/v1/")
		w.WriteHeader(http.StatusFound)
		return
	}
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	unauth := r.Method == http.MethodGet && (path == "/redfish/v1" || path == "/redfish")
	if r.Method == http.MethodPost && path == "/redfish/v1/SessionService/Sessions" {
		s.login(w, body)
		return
	}
	if !unauth && !s.authorized(r) {
		w.Header().Add("WWW-Authenticate", `Basic realm="AMI_Redfish_Server"`)
		w.Header().Add("WWW-Authenticate", `X-Auth-Token realm="AMI_Redfish_Server"`)
		writeJSON(w, http.StatusUnauthorized, ErrorBody("Security.1.0.AccessDenied", "While attempting to establish a connection to /redfish/v1, the service denied access."))
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		s.get(w, r, path)
	case http.MethodPatch:
		s.patch(w, r, path, body)
	case http.MethodPost:
		s.post(w, r, path, body)
	case http.MethodDelete:
		s.del(w, r, path)
	default:
		writeJSON(w, http.StatusNotImplemented, ErrorBody("Base.1.12.ActionNotSupported", "not supported"))
	}
}

func (s *Server) login(w http.ResponseWriter, body string) {
	var cred struct{ UserName, Password string }
	_ = json.Unmarshal([]byte(body), &cred)
	if cred.UserName != s.User || cred.Password != s.Pass {
		writeJSON(w, http.StatusUnauthorized, ErrorBody("Security.1.0.AccessDenied", "access denied"))
		return
	}
	s.nextSess++
	id := strconv.Itoa(s.nextSess)
	tok := fmt.Sprintf("tok-%d-%d", s.nextSess, time.Now().UnixNano())
	s.sessions[id] = &session{id: id, token: tok, lastUsed: s.now()}
	loc := "/redfish/v1/SessionService/Sessions/" + id
	w.Header().Set("X-Auth-Token", tok)
	w.Header().Set("Location", loc)
	writeJSON(w, http.StatusCreated, map[string]any{"@odata.id": loc, "@odata.type": "#Session.v1_3_0.Session", "Id": id, "UserName": cred.UserName})
}

var knownParams = map[string]bool{"$select": true, "$expand": true, "$top": true, "$skip": true, "$filter": true, "only": true, "excerpt": true}

func (s *Server) get(w http.ResponseWriter, r *http.Request, path string) {
	q := r.URL.Query()
	if len(q) > 0 && s.RejectAllQuery {
		writeJSON(w, http.StatusNotImplemented, ErrorBody("Base.1.0.QueryNotSupported", "Querying is not supported by the implementation."))
		return
	}
	for k := range q {
		if !knownParams[k] {
			if s.StrictQuery {
				writeJSON(w, http.StatusNotImplemented, ErrorBody("Base.1.0.QueryNotSupported", "Querying is not supported by the implementation.", k))
				return
			}
		}
	}
	if path == "/redfish/v1/Managers/Self" {
		if m, ok := s.Resources[path]; ok {
			m["DateTime"] = s.now().Add(s.ClockSkew).In(time.FixedZone("", -5*3600)).Format("2006-01-02T15:04:05-07:00")
		}
	}
	if strings.HasPrefix(path, "/redfish/v1/TaskService/TaskMonitors/") {
		s.taskMonitor(w, path)
		return
	}
	res, ok := s.Resources[path]
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorBody("Base.1.12.ResourceMissingAtURI", "The resource at the URI "+path+" was not found.", path))
		return
	}
	out := clone(res)
	if members, ok := out["Members"].([]any); ok {
		skip, top := 0, len(members)
		if v := q.Get("$skip"); v != "" {
			skip, _ = strconv.Atoi(v)
		}
		if v := q.Get("$top"); v != "" {
			top, _ = strconv.Atoi(v)
		}
		total := len(members)
		if skip > total {
			skip = total
		}
		end := total
		limit := s.PageSize
		if top < limit {
			limit = top
		}
		if skip+limit < end {
			end = skip + limit
		}
		page := members[skip:end]
		if q.Get("$expand") != "" {
			exp := make([]any, 0, len(page))
			for _, m := range page {
				id, _ := m.(map[string]any)["@odata.id"].(string)
				if body, ok := s.Resources[id]; ok {
					exp = append(exp, clone(body))
				} else {
					exp = append(exp, m)
				}
			}
			page = exp
		}
		out["Members"] = page
		out["Members@odata.count"] = total
		if s.BrokenNextLink {
			out["Members@odata.nextLink"] = path
		} else if end < total && (q.Get("$top") == "" || end < skip+top) {
			out["Members@odata.nextLink"] = path + "?$skip=" + strconv.Itoa(end)
		} else {
			delete(out, "Members@odata.nextLink")
		}
	}
	if sel := q.Get("$select"); sel != "" {
		keep := map[string]bool{}
		for _, f := range strings.Split(sel, ",") {
			keep[strings.TrimSpace(f)] = true
		}
		for k := range out {
			if !keep[k] && !strings.HasPrefix(k, "@odata.") {
				delete(out, k)
			}
		}
	}
	et := s.etag(path)
	out["@odata.etag"] = et
	w.Header().Set("ETag", et)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) etag(path string) string {
	if e, ok := s.ETags[path]; ok {
		return e
	}
	return ETagService
}

func (s *Server) patch(w http.ResponseWriter, r *http.Request, path, body string) {
	res, ok := s.Resources[path]
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorBody("Base.1.12.ResourceMissingAtURI", "missing", path))
		return
	}
	if len(r.URL.RawQuery) > 0 {
		writeJSON(w, http.StatusBadRequest, ErrorBody("Base.1.12.QueryNotSupportedOnOperation", "query on PATCH"))
		return
	}
	im := r.Header.Get("If-Match")
	if im == "" && s.RequireIfMatch {
		writeJSON(w, http.StatusPreconditionRequired, ErrorBody("Ami.1.0.PreconditionHeaderMissing", "the server requires the request to be conditional"))
		return
	}
	if im != "" && im != "*" && im != s.etag(path) {
		writeJSON(w, http.StatusPreconditionFailed, ErrorBody("Base.1.12.PreconditionFailed", "ETag mismatch"))
		return
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(body), &patch); err != nil || len(patch) == 0 {
		writeJSON(w, http.StatusBadRequest, ErrorBody("Base.1.12.MalformedJSON", "bad body"))
		return
	}
	if dt, ok := patch["DateTime"].(string); ok && path == "/redfish/v1/Managers/Self" {
		t, err := time.Parse(time.RFC3339, dt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorBody("Base.1.12.PropertyValueFormatError", "bad DateTime", dt, "DateTime"))
			return
		}
		s.ClockSkew = t.Sub(s.now()).Truncate(time.Second)
		if s.OnClockSet != nil {
			s.OnClockSet(s.ClockSkew)
		}
	}
	merge(res, patch)
	// bump the resource's ETag
	old := strings.Trim(s.etag(path), `"`)
	n, _ := strconv.Atoi(old)
	s.ETags[path] = `"` + strconv.Itoa(n+1) + `"`
	w.WriteHeader(http.StatusNoContent)
}

func merge(dst, src map[string]any) {
	for k, v := range src {
		if sv, ok := v.(map[string]any); ok {
			if dv, ok := dst[k].(map[string]any); ok {
				merge(dv, sv)
				continue
			}
		}
		dst[k] = v
	}
}

func (s *Server) post(w http.ResponseWriter, r *http.Request, path, body string) {
	if strings.Contains(path, "/Actions/") {
		s.Actions = append(s.Actions, Request{Method: r.Method, Path: path, Body: body})
		var args map[string]any
		_ = json.Unmarshal([]byte(body), &args)
		switch {
		case strings.HasSuffix(path, "ComputerSystem.Reset"):
			rt, _ := args["ResetType"].(string)
			if !contains(s.allowable("/redfish/v1/Systems/Self/ResetActionInfo"), rt) {
				writeJSON(w, http.StatusBadRequest, ErrorBody("Base.1.12.ActionParameterValueNotInList", "bad ResetType", rt, "ResetType"))
				return
			}
			sys := s.Resources["/redfish/v1/Systems/Self"]
			switch rt {
			case "On", "ForceRestart", "PowerCycle":
				sys["PowerState"] = "On"
			case "ForceOff", "GracefulShutdown":
				sys["PowerState"] = "Off"
			}
			if s.ResetTask > 0 {
				id := strconv.Itoa(len(s.tasks) + 1)
				s.tasks[id] = &task{remaining: s.ResetTask}
				s.Resources["/redfish/v1/TaskService/Tasks/"+id] = map[string]any{"@odata.id": "/redfish/v1/TaskService/Tasks/" + id,
					"@odata.type": "#Task.v1_4_3.Task", "Id": id, "TaskState": "Running", "TaskStatus": "OK", "PercentComplete": 0}
				w.Header().Set("Location", "/redfish/v1/TaskService/TaskMonitors/"+id)
				w.Header().Set("Retry-After", "0")
				writeJSON(w, http.StatusAccepted, s.Resources["/redfish/v1/TaskService/Tasks/"+id])
				return
			}
		case strings.HasSuffix(path, "Manager.Reset"):
			rt, _ := args["ResetType"].(string)
			if !contains(s.allowable("/redfish/v1/Managers/Self/ResetActionInfo"), rt) {
				writeJSON(w, http.StatusBadRequest, ErrorBody("Base.1.12.ActionParameterValueNotInList", "bad ResetType", rt, "ResetType"))
				return
			}
		case strings.HasSuffix(path, "LogService.ClearLog"):
			svc := strings.TrimSuffix(path, "/Actions/LogService.ClearLog")
			if e, ok := s.Resources[svc+"/Entries"]; ok {
				for _, m := range e["Members"].([]any) {
					delete(s.Resources, m.(map[string]any)["@odata.id"].(string))
				}
				e["Members"] = []any{}
			}
		case strings.HasSuffix(path, "VirtualMedia.InsertMedia"):
			slot := strings.TrimSuffix(path, "/Actions/VirtualMedia.InsertMedia")
			if vm, ok := s.Resources[slot]; ok {
				vm["Image"] = args["Image"]
				vm["Inserted"] = true
				if wp, ok := args["WriteProtected"]; ok {
					vm["WriteProtected"] = wp
				}
			}
		case strings.HasSuffix(path, "VirtualMedia.EjectMedia"):
			slot := strings.TrimSuffix(path, "/Actions/VirtualMedia.EjectMedia")
			if vm, ok := s.Resources[slot]; ok {
				vm["Image"] = nil
				vm["Inserted"] = false
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, ErrorBody("Base.1.12.OperationNotAllowed", "POST not allowed"))
}

func (s *Server) allowable(path string) []string {
	ai := s.Resources[path]
	if ai == nil {
		return nil
	}
	for _, p := range ai["Parameters"].([]any) {
		pm := p.(map[string]any)
		if pm["Name"] == "ResetType" {
			var out []string
			for _, v := range pm["AllowableValues"].([]any) {
				out = append(out, v.(string))
			}
			return out
		}
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func (s *Server) taskMonitor(w http.ResponseWriter, path string) {
	id := strings.TrimPrefix(path, "/redfish/v1/TaskService/TaskMonitors/")
	t, ok := s.tasks[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorBody("Base.1.12.ResourceMissingAtURI", "missing", path))
		return
	}
	tr := s.Resources["/redfish/v1/TaskService/Tasks/"+id]
	if t.remaining > 0 {
		t.remaining--
		tr["PercentComplete"] = 50
		w.Header().Set("Location", path)
		w.Header().Set("Retry-After", "0")
		writeJSON(w, http.StatusAccepted, tr)
		return
	}
	tr["TaskState"] = "Completed"
	tr["PercentComplete"] = 100
	writeJSON(w, http.StatusOK, tr)
}

func (s *Server) del(w http.ResponseWriter, r *http.Request, path string) {
	if strings.HasPrefix(path, "/redfish/v1/SessionService/Sessions/") {
		id := strings.TrimPrefix(path, "/redfish/v1/SessionService/Sessions/")
		if _, ok := s.sessions[id]; !ok {
			writeJSON(w, http.StatusNotFound, ErrorBody("Base.1.12.ResourceMissingAtURI", "missing", path))
			return
		}
		delete(s.sessions, id)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, ErrorBody("Base.1.12.OperationNotAllowed", "DELETE not allowed"))
}

// AddSELEntry appends a SEL record (id auto-assigned) and returns its id.
func (s *Server) AddSELEntry(msg, severity string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	coll := s.Resources["/redfish/v1/Managers/Self/LogServices/SEL/Entries"]
	members := coll["Members"].([]any)
	maxID := 0
	for _, m := range members {
		id := m.(map[string]any)["@odata.id"].(string)
		n, _ := strconv.Atoi(id[strings.LastIndexByte(id, '/')+1:])
		if n > maxID {
			maxID = n
		}
	}
	id := strconv.Itoa(maxID + 1)
	p := "/redfish/v1/Managers/Self/LogServices/SEL/Entries/" + id
	s.Resources[p] = selEntry(id, msg, severity, "2026-11-16T09:00:00-05:00")
	coll["Members"] = append(members, map[string]any{"@odata.id": p})
	return id
}

// ---- fixture ----

func link(p string) map[string]any { return map[string]any{"@odata.id": p} }

func links(ps ...string) []any {
	out := make([]any, 0, len(ps))
	for _, p := range ps {
		out = append(out, link(p))
	}
	return out
}

func collection(path, typ string, members []string) map[string]any {
	return map[string]any{"@odata.id": path, "@odata.type": typ, "Name": typ, "Members": links(members...), "Members@odata.count": len(members)}
}

func actionInfo(path string, vals ...string) map[string]any {
	return map[string]any{"@odata.id": path, "@odata.type": "#ActionInfo.v1_1_2.ActionInfo", "Id": "ResetActionInfo",
		"Parameters": []any{map[string]any{"Name": "ResetType", "Required": true, "DataType": "String", "AllowableValues": toAny(vals)}}}
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func selEntry(id, msg, severity, created string) map[string]any {
	return map[string]any{"@odata.id": "/redfish/v1/Managers/Self/LogServices/SEL/Entries/" + id,
		"@odata.type": "#LogEntry.v1_4_3.LogEntry", "Id": id, "Name": "SEL Entry " + id,
		"EntryType": "SEL", "EntryCode": "Assert", "Severity": severity, "Created": created, "EventTimestamp": created,
		"Message": msg, "MessageId": "IPMI.0.1.0.SensorEvent", "SensorType": "Temperature", "SensorNumber": 12,
		"OriginOfCondition": link("/redfish/v1/Chassis/Self/Sensors/12")}
}

// B650Fixture returns a fresh copy of the B650D4U 4.10.00 resource tree.
func B650Fixture() map[string]map[string]any {
	r := map[string]map[string]any{}
	r["/redfish/v1"] = map[string]any{
		"@odata.id": "/redfish/v1/", "@odata.type": "#ServiceRoot.v1_7_0.ServiceRoot", "RedfishVersion": "1.15.1",
		"Product": "AMI Redfish Server", "Vendor": "AMI", "Id": "RootService", "Name": "Root Service",
		"ProtocolFeaturesSupported": map[string]any{
			"DeepOperations": map[string]any{"DeepPATCH": true, "DeepPOST": false, "MaxLevels": 6},
			"ExcerptQuery":   true,
			"ExpandQuery":    map[string]any{"ExpandAll": true, "Levels": true, "Links": true, "MaxLevels": 5, "NoLinks": true},
			"FilterQuery":    true, "OnlyMemberQuery": true, "SelectQuery": true},
		"Oem":            map[string]any{"Ami": map[string]any{"@odata.type": "#AMIServiceRoot.v1_0_0.AMIServiceRoot", "RtpVersion": "13.05"}},
		"AccountService": link("/redfish/v1/AccountService"), "Chassis": link("/redfish/v1/Chassis"),
		"Managers": link("/redfish/v1/Managers"), "SessionService": link("/redfish/v1/SessionService"),
		"Systems": link("/redfish/v1/Systems"), "Tasks": link("/redfish/v1/TaskService"),
		"UpdateService": link("/redfish/v1/UpdateService"), "EventService": link("/redfish/v1/EventService"),
		"Links": map[string]any{"Sessions": link("/redfish/v1/SessionService/Sessions")},
	}
	r["/redfish"] = map[string]any{"v1": "/redfish/v1/"}
	r["/redfish/v1/SessionService"] = map[string]any{"@odata.id": "/redfish/v1/SessionService",
		"@odata.type": "#SessionService.v1_1_6.SessionService", "ServiceEnabled": true, "SessionTimeout": 30,
		"Sessions": link("/redfish/v1/SessionService/Sessions")}
	r["/redfish/v1/SessionService/Sessions"] = collection("/redfish/v1/SessionService/Sessions", "#SessionCollection.SessionCollection", nil)

	// Systems
	r["/redfish/v1/Systems"] = collection("/redfish/v1/Systems", "#ComputerSystemCollection.ComputerSystemCollection", []string{"/redfish/v1/Systems/Self"})
	r["/redfish/v1/Systems/Self"] = map[string]any{
		"@odata.id": "/redfish/v1/Systems/Self", "@odata.type": "#ComputerSystem.v1_16_0.ComputerSystem",
		"Id": "Self", "Name": "System", "SystemType": "Physical", "Manufacturer": "AsrockRack", "Model": nil,
		"SerialNumber": "To be filled by O.E.M.", "UUID": "00000000-0000-0000-0000-000000000000",
		"BiosVersion": "10.15", "PowerState": "Off", "PowerRestorePolicy": "AlwaysOff",
		"Status": map[string]any{"State": "Enabled", "Health": "OK"},
		"Boot": map[string]any{"BootSourceOverrideEnabled": "Disabled", "BootSourceOverrideMode": "Legacy",
			"BootSourceOverrideTarget": "None", "BootOrderPropertySelection": "BootOrder",
			"BootOrder":   toAny([]string{"Boot0003", "Boot0004", "Boot0005", "Boot0006", "Boot0002", "Boot0001"}),
			"BootOptions": link("/redfish/v1/Systems/Self/BootOptions"), "BootNext": nil, "HttpBootUri": nil},
		"Bios": link("/redfish/v1/Systems/Self/Bios"), "Processors": link("/redfish/v1/Systems/Self/Processors"),
		"Memory": link("/redfish/v1/Systems/Self/Memory"), "Storage": link("/redfish/v1/Systems/Self/Storage"),
		"SimpleStorage": link("/redfish/v1/Systems/Self/SimpleStorage"), "LogServices": link("/redfish/v1/Systems/Self/LogServices"),
		"EthernetInterfaces": link("/redfish/v1/Systems/Self/EthernetInterfaces"),
		"Actions": map[string]any{"#ComputerSystem.Reset": map[string]any{
			"target":              "/redfish/v1/Systems/Self/Actions/ComputerSystem.Reset",
			"@Redfish.ActionInfo": "/redfish/v1/Systems/Self/ResetActionInfo"}},
		"Links": map[string]any{"Chassis": links("/redfish/v1/Chassis/Self"), "ManagedBy": links("/redfish/v1/Managers/Self")},
	}
	r["/redfish/v1/Systems/Self/ResetActionInfo"] = actionInfo("/redfish/v1/Systems/Self/ResetActionInfo", "ForceOff", "ForceRestart", "GracefulShutdown", "On", "PowerCycle")
	r["/redfish/v1/Systems/Self/LogServices"] = collection("/redfish/v1/Systems/Self/LogServices", "#LogServiceCollection.LogServiceCollection", nil)
	procs := []string{"/redfish/v1/Systems/Self/Processors/CPU0"}
	r["/redfish/v1/Systems/Self/Processors"] = collection("/redfish/v1/Systems/Self/Processors", "#ProcessorCollection.ProcessorCollection", procs)
	r[procs[0]] = map[string]any{"@odata.id": procs[0], "@odata.type": "#Processor.v1_13_0.Processor", "Id": "CPU0", "Name": "Processor",
		"Socket": "CPU0", "ProcessorType": "CPU", "Manufacturer": "Advanced Micro Devices, Inc.", "Model": "AMD Ryzen 9 7950X 16-Core Processor",
		"TotalCores": 16, "TotalThreads": 32, "MaxSpeedMHz": 5850, "Status": map[string]any{"State": "Enabled", "Health": "OK"}}
	var dimms []string
	for i := 0; i < 4; i++ {
		p := fmt.Sprintf("/redfish/v1/Systems/Self/Memory/DevType2_DIMM%d", i)
		dimms = append(dimms, p)
		m := map[string]any{"@odata.id": p, "@odata.type": "#Memory.v1_14_0.Memory", "Id": fmt.Sprintf("DevType2_DIMM%d", i),
			"Name": fmt.Sprintf("DIMM%d", i), "DeviceLocator": fmt.Sprintf("DIMM%d", i), "Status": map[string]any{"State": "Absent"},
			"CapacityMiB": nil, "Manufacturer": nil}
		if i == 0 {
			m["Status"] = map[string]any{"State": "Enabled", "Health": "OK"}
			m["CapacityMiB"] = 32768
			m["OperatingSpeedMhz"] = 4800
			m["Manufacturer"] = "Samsung"
			m["PartNumber"] = "M324R4GA3BB0-CQK"
			m["MemoryDeviceType"] = "DDR5"
		}
		r[p] = m
	}
	r["/redfish/v1/Systems/Self/Memory"] = collection("/redfish/v1/Systems/Self/Memory", "#MemoryCollection.MemoryCollection", dimms)
	r["/redfish/v1/Systems/Self/Storage"] = collection("/redfish/v1/Systems/Self/Storage", "#StorageCollection.StorageCollection", []string{"/redfish/v1/Systems/Self/Storage/StorageUnit_0"})
	r["/redfish/v1/Systems/Self/Storage/StorageUnit_0"] = map[string]any{"@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0",
		"@odata.type": "#Storage.v1_13_0.Storage", "Id": "StorageUnit_0", "Name": "Storage",
		"Drives": links("/redfish/v1/Systems/Self/Storage/StorageUnit_0/Drives/0")}
	r["/redfish/v1/Systems/Self/Storage/StorageUnit_0/Drives/0"] = map[string]any{"@odata.id": "/redfish/v1/Systems/Self/Storage/StorageUnit_0/Drives/0",
		"@odata.type": "#Drive.v1_15_0.Drive", "Id": "0", "Name": "NVMe0", "Model": "Samsung SSD 990 PRO 2TB", "Manufacturer": "Samsung",
		"SerialNumber": "S6Z0NX0W000000", "Revision": "4B2QJXD7", "CapacityBytes": 2000398934016, "MediaType": "SSD", "Protocol": "NVMe",
		"Status": map[string]any{"State": "Enabled", "Health": "OK"}}

	// Chassis
	r["/redfish/v1/Chassis"] = collection("/redfish/v1/Chassis", "#ChassisCollection.ChassisCollection", []string{"/redfish/v1/Chassis/Self"})
	r["/redfish/v1/Chassis/Self"] = map[string]any{
		"@odata.id": "/redfish/v1/Chassis/Self", "@odata.type": "#Chassis.v1_21_0.Chassis",
		"Id": "Self", "ChassisType": "Other", "Name": "Computer System Chassis", "Manufacturer": "To be filled by O.E.M.",
		"Model": "B650D4U", "SerialNumber": "To be filled by O.E.M.", "PartNumber": "To be filled by O.E.M.",
		"PowerState": "Off", "IndicatorLED": "Off", "IndicatorLED@Redfish.AllowableValues": toAny([]string{"Lit", "Blinking", "Off"}),
		"Status":  map[string]any{"State": "Enabled", "Health": "OK"},
		"Sensors": link("/redfish/v1/Chassis/Self/Sensors"), "Thermal": link("/redfish/v1/Chassis/Self/Thermal"),
		"Power": link("/redfish/v1/Chassis/Self/Power"), "PowerSubsystem": link("/redfish/v1/Chassis/Self/PowerSubsystem"),
		"ThermalSubsystem": link("/redfish/v1/Chassis/Self/ThermalSubsystem"),
		"Actions": map[string]any{"#Chassis.Reset": map[string]any{"target": "/redfish/v1/Chassis/Self/Actions/Chassis.Reset",
			"@Redfish.ActionInfo": "/redfish/v1/Chassis/Self/ResetActionInfo"}},
	}
	var sensors []string
	for i := 1; i <= 60; i++ {
		p := fmt.Sprintf("/redfish/v1/Chassis/Self/Sensors/%d", i)
		sensors = append(sensors, p)
		r[p] = genSensor(i)
	}
	r["/redfish/v1/Chassis/Self/Sensors"] = collection("/redfish/v1/Chassis/Self/Sensors", "#SensorCollection.SensorCollection", sensors)
	r["/redfish/v1/Chassis/Self/Thermal"] = map[string]any{"@odata.id": "/redfish/v1/Chassis/Self/Thermal", "@odata.type": "#Thermal.v1_5_3.Thermal",
		"Id": "Thermal", "Name": "Thermal",
		"Temperatures": []any{
			map[string]any{"MemberId": "0", "Name": "TEMP_CPU", "SensorNumber": 12, "ReadingCelsius": nil, "UpperThresholdCritical": 95,
				"Status": map[string]any{"State": "Absent"}},
			map[string]any{"MemberId": "1", "Name": "TEMP_MB", "SensorNumber": 13, "ReadingCelsius": 31, "UpperThresholdCritical": 80, "UpperThresholdFatal": 85,
				"Status": map[string]any{"State": "Enabled", "Health": "OK"}}},
		"Fans": []any{map[string]any{"MemberId": "0", "Name": "FAN1_1", "SensorNumber": 40, "Reading": nil, "ReadingUnits": "RPM",
			"LowerThresholdCritical": 300, "Status": map[string]any{"State": "Absent"}}}}
	r["/redfish/v1/Chassis/Self/Power"] = map[string]any{"@odata.id": "/redfish/v1/Chassis/Self/Power", "@odata.type": "#Power.v1_5_4.Power",
		"Id": "Power", "Name": "Power",
		"Voltages": []any{map[string]any{"MemberId": "0", "Name": "VOLT_3VSB", "ReadingVolts": 3.32, "UpperThresholdCritical": 3.72,
			"LowerThresholdCritical": 2.88, "SensorNumber": 1, "Status": map[string]any{"State": "Enabled", "Health": "OK"}}},
		"PowerControl": []any{map[string]any{"MemberId": "0", "Name": "Chassis Power Control", "PowerConsumedWatts": nil}}}

	// Managers
	r["/redfish/v1/Managers"] = collection("/redfish/v1/Managers", "#ManagerCollection.ManagerCollection", []string{"/redfish/v1/Managers/Self"})
	r["/redfish/v1/Managers/Self"] = map[string]any{
		"@odata.id": "/redfish/v1/Managers/Self", "@odata.type": "#Manager.v1_13_0.Manager",
		"Id": "Self", "Name": "Manager", "ManagerType": "BMC", "FirmwareVersion": "4.10.0", "Model": "821749622", "PowerState": "On",
		"DateTime": "2026-11-16T10:00:00-05:00", "DateTimeLocalOffset": "-05:00",
		"UUID":            "00000000-0000-0000-0000-000000000000",
		"Status":          map[string]any{"State": "Enabled", "Health": "OK"},
		"NetworkProtocol": link("/redfish/v1/Managers/Self/NetworkProtocol"), "EthernetInterfaces": link("/redfish/v1/Managers/Self/EthernetInterfaces"),
		"LogServices": link("/redfish/v1/Managers/Self/LogServices"), "VirtualMedia": link("/redfish/v1/Managers/Self/VirtualMedia"),
		"Links": map[string]any{"ManagerInChassis": link("/redfish/v1/Chassis/Self"), "ManagerForServers": links("/redfish/v1/Systems/Self")},
		"Oem":   map[string]any{"Ami": map[string]any{"PowerSaveMode": "Enable", "VirtualMedia": map[string]any{"CDInstances": 1, "RMediaStatus": "Disabled"}}},
		"Actions": map[string]any{"#Manager.Reset": map[string]any{"target": "/redfish/v1/Managers/Self/Actions/Manager.Reset",
			"@Redfish.ActionInfo": "/redfish/v1/Managers/Self/ResetActionInfo",
			"@Redfish.OperationApplyTimeSupport": map[string]any{"SupportedValues": toAny([]string{"Immediate", "AtMaintenanceWindowStart"}),
				"MaintenanceWindowDurationInSeconds": 600}}},
	}
	r["/redfish/v1/Managers/Self/ResetActionInfo"] = actionInfo("/redfish/v1/Managers/Self/ResetActionInfo", "ForceRestart")
	r["/redfish/v1/Managers/Self/NetworkProtocol"] = map[string]any{"@odata.id": "/redfish/v1/Managers/Self/NetworkProtocol",
		"@odata.type": "#ManagerNetworkProtocol.v1_9_0.ManagerNetworkProtocol", "Id": "NetworkProtocol", "Name": "Manager Network Protocol",
		"HTTPS":        map[string]any{"ProtocolEnabled": true, "Port": 443},
		"KVMIP":        map[string]any{"ProtocolEnabled": true, "Port": 443},
		"VirtualMedia": map[string]any{"ProtocolEnabled": true, "Port": 443},
		"IPMI":         map[string]any{"ProtocolEnabled": true, "Port": 623},
		"SSH":          map[string]any{"ProtocolEnabled": true, "Port": 22},
		"SNMP":         map[string]any{"ProtocolEnabled": true, "Port": 161},
		"NTP":          map[string]any{"ProtocolEnabled": false, "Port": 123, "NTPServers": toAny([]string{"pool.ntp.org", "time.nist.gov", ""})},
	}
	r["/redfish/v1/Managers/Self/EthernetInterfaces"] = collection("/redfish/v1/Managers/Self/EthernetInterfaces", "#EthernetInterfaceCollection.EthernetInterfaceCollection",
		[]string{"/redfish/v1/Managers/Self/EthernetInterfaces/eth0"})
	r["/redfish/v1/Managers/Self/EthernetInterfaces/eth0"] = map[string]any{"@odata.id": "/redfish/v1/Managers/Self/EthernetInterfaces/eth0",
		"@odata.type": "#EthernetInterface.v1_6_2.EthernetInterface", "Id": "eth0", "Name": "eth0",
		"InterfaceEnabled": true, "LinkStatus": "LinkUp", "SpeedMbps": 1000, "MTUSize": 1500,
		"MACAddress": "00:00:5e:00:53:01", "PermanentMACAddress": "00:00:5e:00:53:01", "HostName": "AMI00005E005301", "FQDN": nil,
		"DHCPv4":            map[string]any{"DHCPEnabled": false},
		"IPv4Addresses":     []any{map[string]any{"Address": "192.0.2.55", "AddressOrigin": "Static", "Gateway": "192.0.2.1", "SubnetMask": "255.255.255.0"}},
		"IPv6Addresses":     []any{map[string]any{"Address": "fe80::200:5eff:fe00:5301", "AddressOrigin": "LinkLocal", "PrefixLength": 64}},
		"StaticNameServers": []any{"192.168.0.53", nil, nil},
		"VLAN":              map[string]any{"VLANEnable": false, "VLANId": 0},
		"Oem":               map[string]any{"Ami": map[string]any{"@odata.type": "#AMIDNSConfiguration.v1_0_0.AMIDNSConfiguration", "DNSEnabled": true, "HostNameSetting": "Auto"}}}
	r["/redfish/v1/Managers/Self/VirtualMedia"] = collection("/redfish/v1/Managers/Self/VirtualMedia", "#VirtualMediaCollection.VirtualMediaCollection",
		[]string{"/redfish/v1/Managers/Self/VirtualMedia/CD1"})
	// Synthetic slot (the live unit's collection is empty with RMedia disabled).
	r["/redfish/v1/Managers/Self/VirtualMedia/CD1"] = map[string]any{"@odata.id": "/redfish/v1/Managers/Self/VirtualMedia/CD1",
		"@odata.type": "#VirtualMedia.v1_5_0.VirtualMedia", "Id": "CD1", "Name": "Virtual CD",
		"MediaTypes": toAny([]string{"CD", "DVD"}), "Image": nil, "Inserted": false, "WriteProtected": true, "ConnectedVia": "NotConnected",
		"Actions": map[string]any{
			"#VirtualMedia.InsertMedia": map[string]any{"target": "/redfish/v1/Managers/Self/VirtualMedia/CD1/Actions/VirtualMedia.InsertMedia"},
			"#VirtualMedia.EjectMedia":  map[string]any{"target": "/redfish/v1/Managers/Self/VirtualMedia/CD1/Actions/VirtualMedia.EjectMedia"}}}

	ls := "/redfish/v1/Managers/Self/LogServices"
	r[ls] = collection(ls, "#LogServiceCollection.LogServiceCollection", []string{ls + "/AuditLog", ls + "/EventLog", ls + "/SEL"})
	r[ls+"/SEL"] = map[string]any{"@odata.id": ls + "/SEL", "@odata.type": "#LogService.v1_1_3.LogService", "Id": "SEL", "Name": "SEL Log",
		"MaxNumberOfRecords": 150, "OverWritePolicy": "WrapsWhenFull", "LogEntryType": "SEL", "ServiceEnabled": true,
		"Entries": link(ls + "/SEL/Entries"),
		"Actions": map[string]any{"#LogService.ClearLog": map[string]any{"target": ls + "/SEL/Actions/LogService.ClearLog"}}}
	var sel []string
	created := []string{"2022-01-01T00:00:12-05:00", "2025-04-16T11:08:55-05:00", "2026-11-14T15:06:01-05:00"}
	for i := 1; i <= 11; i++ {
		id := strconv.Itoa(i)
		p := ls + "/SEL/Entries/" + id
		sel = append(sel, p)
		sev := "OK"
		if i%4 == 0 {
			sev = "Warning"
		}
		r[p] = selEntry(id, fmt.Sprintf("Temperature TEMP_CPU event %d", i), sev, created[i%len(created)])
	}
	r[ls+"/SEL/Entries"] = collection(ls+"/SEL/Entries", "#LogEntryCollection.LogEntryCollection", sel)
	r[ls+"/AuditLog"] = map[string]any{"@odata.id": ls + "/AuditLog", "@odata.type": "#LogService.v1_1_3.LogService", "Id": "AuditLog",
		"Name": "Audit Log", "LogEntryType": "Multiple", "Entries": link(ls + "/AuditLog/Entries")}
	r[ls+"/AuditLog/Entries"] = collection(ls+"/AuditLog/Entries", "#LogEntryCollection.LogEntryCollection", []string{ls + "/AuditLog/Entries/1", ls + "/AuditLog/Entries/2"})
	r[ls+"/AuditLog/Entries/1"] = map[string]any{"@odata.id": ls + "/AuditLog/Entries/1", "@odata.type": "#LogEntry.v1_4_3.LogEntry", "Id": "1",
		"EntryType": "Oem", "Severity": "OK", "Created": "2026-11-14T15:06:00-05:00", "Message": "BMC started"}
	r[ls+"/AuditLog/Entries/2"] = map[string]any{"@odata.id": ls + "/AuditLog/Entries/2", "@odata.type": "#LogEntry.v1_4_3.LogEntry", "Id": "2",
		"EntryType": "Oem", "Severity": "OK", "Created": "2026-11-16T09:30:00-05:00", "Message": "User admin logged in from 192.0.2.5"}
	r[ls+"/EventLog"] = map[string]any{"@odata.id": ls + "/EventLog", "@odata.type": "#LogService.v1_1_3.LogService", "Id": "EventLog",
		"Name": "Event Log", "LogEntryType": "Event", "Entries": link(ls + "/EventLog/Entries")}
	r[ls+"/EventLog/Entries"] = collection(ls+"/EventLog/Entries", "#LogEntryCollection.LogEntryCollection", nil)

	// AccountService
	r["/redfish/v1/AccountService"] = map[string]any{"@odata.id": "/redfish/v1/AccountService", "@odata.type": "#AccountService.v1_8_0.AccountService",
		"Id": "AccountService", "AccountLockoutThreshold": 5, "AccountLockoutDuration": 30, "MinPasswordLength": 8, "MaxPasswordLength": 20,
		"LocalAccountAuth": "LocalFirst", "Accounts": link("/redfish/v1/AccountService/Accounts"), "Roles": link("/redfish/v1/AccountService/Roles")}
	r["/redfish/v1/AccountService/Accounts"] = collection("/redfish/v1/AccountService/Accounts", "#ManagerAccountCollection.ManagerAccountCollection",
		[]string{"/redfish/v1/AccountService/Accounts/1", "/redfish/v1/AccountService/Accounts/2", "/redfish/v1/AccountService/Accounts/4"})
	for _, a := range []struct{ id, user, role string }{{"1", "Administrator", "Administrator"}, {"2", "HostAutoXXXXXXXXXX", "HostInterfaceAdministrator"}, {"4", "admin", "Administrator"}} {
		p := "/redfish/v1/AccountService/Accounts/" + a.id
		r[p] = map[string]any{"@odata.id": p, "@odata.type": "#ManagerAccount.v1_8_0.ManagerAccount", "Id": a.id, "Name": "User Account",
			"UserName": a.user, "RoleId": a.role, "Enabled": true, "Locked": false, "Password": nil}
	}

	// UpdateService
	r["/redfish/v1/UpdateService"] = map[string]any{"@odata.id": "/redfish/v1/UpdateService", "@odata.type": "#UpdateService.v1_6_0.UpdateService",
		"Id": "UpdateService", "ServiceEnabled": true, "HttpPushUri": nil, "MultipartHttpPushUri": "/redfish/v1/UpdateService/upload",
		"FirmwareInventory": link("/redfish/v1/UpdateService/FirmwareInventory"),
		"Actions":           map[string]any{"#UpdateService.SimpleUpdate": map[string]any{"target": "/redfish/v1/UpdateService/Actions/SimpleUpdate"}}}
	r["/redfish/v1/UpdateService/FirmwareInventory"] = collection("/redfish/v1/UpdateService/FirmwareInventory", "#SoftwareInventoryCollection.SoftwareInventoryCollection",
		[]string{"/redfish/v1/UpdateService/FirmwareInventory/BIOS", "/redfish/v1/UpdateService/FirmwareInventory/BMC"})
	r["/redfish/v1/UpdateService/FirmwareInventory/BIOS"] = map[string]any{"@odata.id": "/redfish/v1/UpdateService/FirmwareInventory/BIOS",
		"@odata.type": "#SoftwareInventory.v1_2_3.SoftwareInventory", "Id": "BIOS", "Name": "BIOS", "Version": "10.15", "Updateable": true,
		"Status": map[string]any{"State": "Enabled", "Health": "OK"}}
	r["/redfish/v1/UpdateService/FirmwareInventory/BMC"] = map[string]any{"@odata.id": "/redfish/v1/UpdateService/FirmwareInventory/BMC",
		"@odata.type": "#SoftwareInventory.v1_2_3.SoftwareInventory", "Id": "BMC", "Name": "BMC", "Version": "4.10.00", "Updateable": true,
		"Status": map[string]any{"State": "Enabled", "Health": "OK"}}
	r["/redfish/v1/TaskService"] = map[string]any{"@odata.id": "/redfish/v1/TaskService", "@odata.type": "#TaskService.v1_1_4.TaskService",
		"ServiceEnabled": true, "Tasks": link("/redfish/v1/TaskService/Tasks")}
	return r
}

var sensorNames = []struct {
	name, typ string
	reading   float64
}{
	{"VOLT_3VSB", "Voltage", 3.32}, {"VOLT_5VSB", "Voltage", 5.1}, {"VOLT_12V", "Voltage", 12.1}, {"VOLT_BAT", "Voltage", 3.08},
	{"TEMP_MB", "Temperature", 31}, {"TEMP_CPU", "Temperature", 0}, {"FAN1_1", "Rotational", 0}, {"PWR_PSU", "Power", 42},
}

func genSensor(i int) map[string]any {
	p := fmt.Sprintf("/redfish/v1/Chassis/Self/Sensors/%d", i)
	base := sensorNames[(i-1)%len(sensorNames)]
	name := fmt.Sprintf("%s_%d", base.name, i)
	m := map[string]any{"@odata.id": p, "@odata.type": "#Sensor.v1_0_2.Sensor", "Id": strconv.Itoa(i), "Name": name,
		"ReadingType": base.typ, "ReadingUnits": nil, "Reading": base.reading,
		"Status": map[string]any{"State": "Enabled", "Health": "OK"}}
	if i == 13 { // the documented VOLT_BAT sample
		m["Name"] = "VOLT_BAT"
		m["ReadingType"] = "Voltage"
		m["Reading"] = 3.08
		m["PhysicalContext"] = "VoltageRegulator"
		m["Thresholds"] = map[string]any{"LowerCritical": map[string]any{"Reading": 2.4}, "LowerFatal": map[string]any{"Reading": 2.22},
			"UpperCritical": map[string]any{"Reading": 3.3}, "UpperFatal": map[string]any{"Reading": 3.46}}
	}
	if base.name == "TEMP_CPU" || base.name == "FAN1_1" { // host off
		m["Reading"] = nil
		m["Status"] = map[string]any{"State": "Absent"}
	}
	return m
}

// Paths returns the sorted resource paths (debug helper).
func (s *Server) Paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ks := make([]string, 0, len(s.Resources))
	for k := range maps.Keys(s.Resources) {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// UnescapedQuery decodes a recorded RawQuery.
func UnescapedQuery(raw string) url.Values {
	v, _ := url.ParseQuery(raw)
	return v
}
