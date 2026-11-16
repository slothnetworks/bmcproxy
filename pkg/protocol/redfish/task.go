package redfish

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// TaskRef extracts the task to poll from an action/write response:
//  1. the Location header (task monitor, e.g. /redfish/v1/TaskService/TaskMonitors/<id>);
//  2. a Task body's @odata.id or TaskMonitor;
//  3. a task URI in @Message.ExtendedInfo[].MessageArgs (AMI "TaskStarted"-style
//     messages carry the task path as an argument).
//
// It returns "" when the response doesn't reference a task.
func TaskRef(resp *Response) string {
	if resp == nil {
		return ""
	}
	if resp.Location != "" && resp.Status == http.StatusAccepted {
		return resp.Location
	}
	var body struct {
		ODataID      string        `json:"@odata.id"`
		ODataType    string        `json:"@odata.type"`
		TaskMonitor  string        `json:"TaskMonitor"`
		TaskState    string        `json:"TaskState"`
		ExtendedInfo []MessageInfo `json:"@Message.ExtendedInfo"`
		Error        *ErrorInfo    `json:"error"`
	}
	_ = json.Unmarshal(resp.Body, &body)
	if body.TaskState != "" || strings.Contains(body.ODataType, "#Task.") {
		if body.TaskMonitor != "" {
			return body.TaskMonitor
		}
		if body.ODataID != "" {
			return body.ODataID
		}
	}
	infos := body.ExtendedInfo
	if body.Error != nil {
		infos = append(infos, body.Error.ExtendedInfo...)
	}
	for _, m := range infos {
		for _, a := range m.MessageArgs {
			if strings.Contains(a, "/TaskService/Tasks/") || strings.Contains(a, "/TaskService/TaskMonitors/") {
				return a
			}
		}
	}
	if resp.Location != "" && (strings.Contains(resp.Location, "/TaskService/") || strings.Contains(resp.Location, "/Tasks/")) {
		return resp.Location
	}
	return ""
}

// TaskResult is the outcome of WaitTask.
type TaskResult struct {
	// Task is the last Task body seen (nil when polling a task monitor that
	// returned the operation's own final response).
	Task *Task
	// Final is the last response (the task monitor's final response or the
	// terminal Task GET).
	Final *Response
	Polls int
}

// State maps the result to a bmc.TaskState.
func (r *TaskResult) State() bmc.TaskState {
	if r.Task != nil {
		switch {
		case r.Task.Succeeded():
			return bmc.TaskCompleted
		case r.Task.Terminal():
			return bmc.TaskFailed
		case r.Task.TaskState == "New" || r.Task.TaskState == "Pending" || r.Task.TaskState == "Starting":
			return bmc.TaskPending
		default:
			return bmc.TaskRunning
		}
	}
	if r.Final != nil && r.Final.IsSuccess() {
		return bmc.TaskCompleted
	}
	return bmc.TaskFailed
}

// WaitOptions tune WaitTask (zero values use the client defaults).
type WaitOptions struct {
	Interval time.Duration
	Timeout  time.Duration
}

// WaitTask polls ref (a task monitor or Task URI) until completion, error
// or timeout. A task monitor answering 202 is still running (Retry-After is
// honoured, capped at 30 s); any other 2xx from a monitor is the final
// response. A Task resource is polled until TaskState is terminal. A failed
// task returns its result together with an ErrUpstream OpError. Timeout
// returns ErrTimeout.
func (c *Client) WaitTask(ctx context.Context, ref string, wo WaitOptions) (*TaskResult, error) {
	if ref == "" {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "WaitTask", "empty task reference")
	}
	interval, timeout := wo.Interval, wo.Timeout
	if interval <= 0 {
		interval = c.opts.TaskPollInterval
	}
	if timeout <= 0 {
		timeout = c.opts.TaskTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	log := obs.FromContext(ctx, obs.SubRedfish)
	res := &TaskResult{}
	for {
		res.Polls++
		resp, err := c.Do(ctx, http.MethodGet, ref, nil, WithOp("redfish.TaskMonitor"))
		if err != nil {
			if ctx.Err() != nil {
				return res, c.taskTimeout(ref, res)
			}
			return res, err
		}
		res.Final = resp
		wait := interval
		switch {
		case resp.Status == http.StatusAccepted:
			var t Task
			if decodeLenient(resp.Body, &t, c.opts.Quirks) == nil && t.TaskState != "" {
				res.Task = &t
			}
			if ra := retryAfter(resp.Header.Get("Retry-After")); ra > 0 {
				wait = ra
			}
			if resp.Location != "" && resp.Location != ref && strings.Contains(resp.Location, "/Task") {
				ref = resp.Location
			}
		case resp.IsSuccess():
			var t Task
			if decodeLenient(resp.Body, &t, c.opts.Quirks) == nil && t.TaskState != "" {
				res.Task = &t
				if !t.Terminal() {
					break
				}
				if !t.Succeeded() {
					return res, &bmc.OpError{Pkg: Pkg, Op: "WaitTask " + ref, Backend: bmc.BackendRedfish,
						UpstreamStatus: resp.Status, Detail: taskDetail(&t), Err: bmc.ErrUpstream}
				}
				return res, nil
			}
			// A task monitor's final response is the operation's own result.
			return res, nil
		default:
			return res, StatusError(resp)
		}
		log.DebugContext(ctx, "redfish task running", "ref", ref, "polls", res.Polls, "wait", wait)
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return res, c.taskTimeout(ref, res)
		case <-t.C:
		}
	}
}

func (c *Client) taskTimeout(ref string, res *TaskResult) error {
	d := "task did not finish"
	if res.Task != nil {
		d += " (state " + res.Task.TaskState + ")"
	}
	return &bmc.OpError{Pkg: Pkg, Op: "WaitTask " + ref, Backend: bmc.BackendRedfish, Detail: d, Err: bmc.ErrTimeout}
}

func taskDetail(t *Task) string {
	s := t.TaskState
	if t.TaskStatus != "" {
		s += "/" + t.TaskStatus
	}
	for _, m := range t.Messages {
		if m.Message != "" {
			return s + ": " + m.Message
		}
	}
	return s
}

func retryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
		d := time.Duration(n) * time.Second
		if d > 30*time.Second {
			d = 30 * time.Second
		}
		return d
	}
	return 0
}

// ToBMCTask converts a TaskResult to the normalized bmc.Task.
func (r *TaskResult) ToBMCTask(id string) bmc.Task {
	t := bmc.Task{ID: id, State: r.State()}
	if r.Task != nil {
		t.PercentComplete = r.Task.PercentComplete
		t.Message = taskDetail(r.Task)
		t.StartTime, _ = ParseTime(r.Task.StartTime)
		t.EndTime, _ = ParseTime(r.Task.EndTime)
		if r.Task.ID != "" && id == "" {
			t.ID = r.Task.ID
		}
	}
	return t
}

// ParseTime parses a Redfish DateTime (RFC 3339, with or without
// fractional seconds). Empty or unparseable values return the zero time.
func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04:05Z0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, bmc.Errorf(bmc.ErrUpstream, Pkg, "ParseTime", "bad DateTime %q", s)
}
