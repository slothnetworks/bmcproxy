package obs

import (
	"encoding/json"
	"expvar"
	"net/http"
	"net/http/pprof"
	"strings"
)

// RegisterDebugHandlers mounts net/http/pprof under /debug/pprof/, expvar at
// /debug/vars and, if o is non-nil, the runtime level control at
// /debug/levels on mux. The caller decides whether to call it
// (--debug-endpoints) and must bind the listener to localhost.
func RegisterDebugHandlers(mux *http.ServeMux, o *Obs) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/vars", expvar.Handler())
	if o != nil {
		mux.Handle("/debug/levels", LevelsHandler(o.Levels()))
	}
}

// LevelsHandler serves the level registry:
//
//	GET  /debug/levels                     -> {"*":"INFO","transport":"TRACE"}
//	POST /debug/levels?set=transport=trace,redfish   (same syntax as --debug)
//	POST /debug/levels?default=debug
//	POST /debug/levels?reset=transport
func LevelsHandler(l *Levels) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
		case http.MethodPost, http.MethodPut:
			q := r.URL.Query()
			if d := q.Get("default"); d != "" {
				lv, err := ParseLevel(d)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				l.SetDefault(lv)
			}
			for _, s := range strings.Split(q.Get("reset"), ",") {
				if s = strings.TrimSpace(s); s != "" {
					l.Reset(s)
				}
			}
			if s := q.Get("set"); s != "" {
				// explicit set may also lower verbosity, unlike ApplyDebug
				for _, item := range strings.Split(s, ",") {
					name, lvs, ok := strings.Cut(strings.TrimSpace(item), "=")
					if name == "" {
						continue
					}
					lv, err := ParseLevel(map[bool]string{true: lvs, false: "debug"}[ok])
					if err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					if name == "all" || name == "*" {
						l.SetDefault(lv)
					} else {
						l.Set(name, lv)
					}
				}
			}
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(l.Snapshot())
	})
}
