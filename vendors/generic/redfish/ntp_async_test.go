package genericredfish_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/redfish/redfishtest"
	genericredfish "github.com/slothnetworks/bmcproxy/vendors/generic/redfish"
)

const (
	npPath   = "/redfish/v1/Managers/Self/NetworkProtocol"
	taskPath = "/redfish/v1/TaskService/Tasks/9"
)

// amiAsyncNTP makes the fake behave like AMI MegaRAC (B650D4U): a
// NetworkProtocol PATCH answers 202 with a task and changes nothing; apply
// (if non-nil) runs when the task reaches its terminal state.
func amiAsyncNTP(t *testing.T, srv *redfishtest.Server, taskStates []string, taskMsg string, apply func()) *atomic.Int32 {
	t.Helper()
	var polls atomic.Int32
	srv.Handlers[npPath] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch {
			w.Header().Set("Location", taskPath)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"@odata.id":"` + taskPath + `","@odata.type":"#Task.v1_4_2.Task","TaskState":"New"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(srv.Get(npPath))
	}
	srv.Handlers[taskPath] = func(w http.ResponseWriter, r *http.Request) {
		n := int(polls.Add(1)) - 1
		if n >= len(taskStates) {
			n = len(taskStates) - 1
		}
		state := taskStates[n]
		if (state == "Completed" || state == "Exception") && apply != nil && n == len(taskStates)-1 {
			apply()
			apply = nil
		}
		status := "OK"
		if state == "Exception" {
			status = "Critical"
		}
		body := map[string]any{"@odata.id": taskPath, "@odata.type": "#Task.v1_4_2.Task", "Id": "9",
			"Name": "NetworkProtocolTask", "TaskState": state, "TaskStatus": status}
		if taskMsg != "" {
			body["Messages"] = []any{map[string]any{"MessageId": "Base.1.8.GeneralError", "Message": taskMsg}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}
	return &polls
}

func setNTPEnabled(srv *redfishtest.Server, servers ...string) {
	np := srv.Get(npPath)
	ntp := np["NTP"].(map[string]any)
	ntp["ProtocolEnabled"] = true
	if len(servers) > 0 {
		s := make([]any, 0, len(servers))
		for _, v := range servers {
			s = append(s, v)
		}
		ntp["NTPServers"] = s
	}
	srv.Set(npPath, np)
}

func TestSetTimeNTPWaitsForAcceptedTask(t *testing.T) {
	defer genericredfish.SetNTPVerify(time.Second, 10*time.Millisecond)()
	srv, d, tg := setup(t)
	polls := amiAsyncNTP(t, srv, []string{"Running", "Running", "Completed"}, "", func() { setNTPEnabled(srv, "10.7.0.1", "10.7.0.2") })
	s := open(t, d, tg)
	en := true
	if err := s.SetTime(ctx, bmc.TimeUpdate{NTPEnabled: &en, NTPServers: []string{"10.7.0.1", "10.7.0.2"}}); err != nil {
		t.Fatalf("SetTime: %v", err)
	}
	if polls.Load() < 3 {
		t.Fatalf("task polled %d times, want the 202 task awaited to completion", polls.Load())
	}
	tc, err := s.Time(ctx)
	if err != nil || !tc.NTPEnabled {
		t.Fatalf("time after set: %+v %v", tc, err)
	}
}

func TestSetTimeNTPTaskFailureSurfaced(t *testing.T) {
	defer genericredfish.SetNTPVerify(time.Second, 10*time.Millisecond)()
	srv, d, tg := setup(t)
	amiAsyncNTP(t, srv, []string{"Running", "Exception"}, "The NTP server could not be reached.", nil)
	s := open(t, d, tg)
	en := true
	err := s.SetTime(ctx, bmc.TimeUpdate{NTPEnabled: &en})
	if !errors.Is(err, bmc.ErrUpstream) || !strings.Contains(err.Error(), "could not be reached") {
		t.Fatalf("want ErrUpstream carrying the task message, got %v", err)
	}
}

func TestSetTimeNTPNotAppliedIsAnError(t *testing.T) {
	defer genericredfish.SetNTPVerify(80*time.Millisecond, 10*time.Millisecond)()
	srv, d, tg := setup(t)
	// The task "completes" but the BMC never actually enables NTP.
	amiAsyncNTP(t, srv, []string{"Completed"}, "", nil)
	s := open(t, d, tg)
	en := true
	err := s.SetTime(ctx, bmc.TimeUpdate{NTPEnabled: &en})
	if !errors.Is(err, bmc.ErrUpstream) || !strings.Contains(err.Error(), "did not apply") || !strings.Contains(err.Error(), "ProtocolEnabled=false") {
		t.Fatalf("want a did-not-apply ErrUpstream, got %v", err)
	}
}
