//go:build x470_pam

package asrock

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// TestEnablePAM mirrors scripts/x470d4u-redfish-pam-auth.sh -a enable
// against a fake: GET (ETag) -> PATCH with If-Match (first 428 forces one
// ETag refresh) -> verify.
func TestEnablePAM(t *testing.T) {
	var mu sync.Mutex
	enabled, etag, patches, sent428 := false, `"e1"`, 0, false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if u, p, _ := r.BasicAuth(); u != "Administrator" || p != "bootstrap" {
			w.WriteHeader(401)
			return
		}
		if r.URL.Path != pamConfigPath {
			w.WriteHeader(404)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("ETag", etag)
			_ = json.NewEncoder(w).Encode(map[string]any{"PAMEnabled": enabled, "PAMOrder": []string{"IPMI"}})
		case http.MethodPatch:
			patches++
			if !sent428 {
				sent428, etag = true, `"e2"`
				w.WriteHeader(http.StatusPreconditionRequired)
				return
			}
			if r.Header.Get("If-Match") != etag {
				w.WriteHeader(http.StatusPreconditionFailed)
				return
			}
			var b map[string]bool
			_ = json.NewDecoder(r.Body).Decode(&b)
			enabled = b["PAMEnabled"]
			w.WriteHeader(204)
		}
	}))
	defer srv.Close()
	m := newTransport(t)
	cl, err := rf.New(m.Endpoint(srv.Listener.Addr().String(), transport.LaneRedfish), rf.Options{
		Creds: bmc.Creds{User: "Administrator", Pass: "bootstrap"}, Auth: rf.AuthBasic,
	})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := enablePAM(context.Background(), cl)
	if err != nil || !changed || !enabled || patches != 2 {
		t.Fatalf("changed=%v err=%v enabled=%v patches=%d", changed, err, enabled, patches)
	}
	changed, err = enablePAM(context.Background(), cl)
	if err != nil || changed || patches != 2 {
		t.Fatalf("second run must be a no-op: changed=%v err=%v patches=%d", changed, err, patches)
	}
}
