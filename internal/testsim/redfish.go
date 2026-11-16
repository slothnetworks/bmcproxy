package testsim

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/protocol/redfish/redfishtest"
)

// Redfish emulation.
//
// B650 (4.10.00): delegated to redfishtest (the live B650 resource tree),
// with the sim's power state machine, clock offset and session drops layered
// on top. X470 (2.32.00): only the unauthenticated ServiceRoot
// (v1_3_1 / RedfishVersion 1.5.0) and GlobalLuaTables answer; everything
// else, including SessionService logins, is 401 Security.1.0.AccessDenied,
// as on the live board.

const x470ETag = `W/"1794669113"`

var globalLuaTables = map[string]any{"@odata.id": "/redfish/v1/GlobalLuaTables",
	"Members": []any{map[string]any{"@odata.id": "/redfish/v1/"}}, "Members@odata.count": 1}

func (s *Sim) serveRedfish(w http.ResponseWriter, r *http.Request, body []byte) {
	p := strings.TrimRight(r.URL.Path, "/")
	// Unauthenticated on both boards (the proxy must block it).
	if r.Method == http.MethodGet && p == "/redfish/v1/GlobalLuaTables" {
		w.Header().Set("OData-Version", "4.0")
		writeJSON(w, http.StatusOK, globalLuaTables)
		return
	}
	if s.profile.RedfishAuth && s.rf != nil {
		s.serveB650Redfish(w, r, body, p)
		return
	}
	s.serveX470Redfish(w, r, p)
}

func (s *Sim) serveB650Redfish(w http.ResponseWriter, r *http.Request, body []byte, p string) {
	// Mirror the power state machine into the resource tree (this also
	// undoes redfishtest's own instant PowerState flip after a Reset).
	s.mu.Lock()
	on := s.power.isOn(s.now())
	s.mu.Unlock()
	ps := "Off"
	if on {
		ps = "On"
	}
	s.rf.Lock()
	for _, path := range []string{"/redfish/v1/Systems/Self", "/redfish/v1/Chassis/Self"} {
		if m := s.rf.Resources[path]; m != nil {
			m["PowerState"] = ps
		}
	}
	s.rf.Unlock()

	sw := &statusWriter{ResponseWriter: w}
	s.rf.Config.Handler.ServeHTTP(sw, r)

	// redfishtest authenticated and validated the action; apply it to the
	// sim's state machine (delay, ignored soft-off).
	if r.Method == http.MethodPost && strings.HasSuffix(p, "/Systems/Self/Actions/ComputerSystem.Reset") && sw.status/100 == 2 {
		var args struct{ ResetType string }
		_ = json.Unmarshal(body, &args)
		if cmd, ok := redfishResetToCmd(args.ResetType); ok {
			s.powerAction(cmd)
		}
	}
	s.mu.Lock()
	switch {
	case r.Method == http.MethodPost && p == "/redfish/v1/SessionService/Sessions":
		if sw.status == http.StatusCreated {
			s.stats.Logins++
		} else {
			s.stats.LoginFailures++
		}
	case r.Method == http.MethodDelete && strings.HasPrefix(p, "/redfish/v1/SessionService/Sessions/"):
		if sw.status/100 == 2 {
			s.stats.Logouts++
		}
	case sw.status == http.StatusUnauthorized:
		s.stats.AuthRejects++
	}
	s.mu.Unlock()
}

var x470KnownQuery = map[string]bool{"$select": true, "$expand": true, "$filter": true, "$top": true, "$skip": true}

func (s *Sim) serveX470Redfish(w http.ResponseWriter, r *http.Request, p string) {
	h := w.Header()
	h.Set("Server", "AMI MegaRAC Redfish Service")
	h.Set("OData-Version", "4.0")
	switch {
	case r.Method == http.MethodOptions:
		writeHTML(w, http.StatusMethodNotAllowed, "<html><body><h1>405 - Method Not Allowed</h1></body></html>\n")
		return
	case r.Method == http.MethodGet && r.URL.Path == "/redfish/v1":
		h.Set("Location", "/redfish/v1/")
		w.WriteHeader(http.StatusFound)
		return
	case r.Method == http.MethodGet && (p == "/redfish/v1" || p == "/redfish"):
		for k := range r.URL.Query() {
			if !x470KnownQuery[k] {
				writeJSON(w, http.StatusNotImplemented, redfishtest.ErrorBody("Base.1.0.QueryNotSupported",
					"Querying is not supported by the implementation.", k))
				return
			}
		}
		h.Set("ETag", x470ETag)
		writeRaw(w, http.StatusOK, s.x470Root)
		return
	}
	if r.Method == http.MethodPost && p == "/redfish/v1/SessionService/Sessions" {
		s.mu.Lock()
		s.stats.LoginFailures++
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		s.stats.AuthRejects++
		s.mu.Unlock()
	}
	// Every resource below the root, with or without (valid /api)
	// credentials: the 2.32 Redfish account store does not know admin.
	h.Add("WWW-Authenticate", `Basic realm="AMI_Redfish_Server"`)
	h.Add("WWW-Authenticate", `X-Auth-Token realm="AMI_Redfish_Server"`)
	writeJSON(w, http.StatusUnauthorized, redfishtest.ErrorBody("Security.1.0.AccessDenied",
		"While attempting to establish a connection to "+r.URL.Path+", the service was denied access.", r.URL.Path))
}
