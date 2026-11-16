package v1

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/server"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// PowerActionResult is the body of POST /api/v1/power. The proxy never
// assumes an action took effect (a graceful shutdown may be silently ignored
// mid-POST): it polls PowerState and reports what it observed.
type PowerActionResult struct {
	// The action performed.
	Action bmc.PowerAction `json:"action" example:"shutdown"`
	// The state the action should reach (empty for nmi).
	Expected bmc.PowerState `json:"expected,omitempty" example:"off"`
	// The last observed power state ("unknown" if never polled successfully).
	State bmc.PowerState `json:"state" example:"off"`
	// The observed state reached Expected within the timeout.
	Converged bool `json:"converged" example:"true"`
	// Polling was performed (false for wait=false or nmi).
	Polled bool `json:"polled" example:"true"`
	// Number of PowerState polls.
	Polls int `json:"polls" example:"3"`
	// Seconds spent polling.
	WaitedSeconds float64 `json:"waitedSeconds" example:"6.1"`
	// The polling timeout applied.
	TimeoutSeconds float64 `json:"timeoutSeconds" example:"60"`
	// Set when the state did not converge.
	Warning string `json:"warning,omitempty"`
	// The last polling error, if any.
	LastPollError string `json:"lastPollError,omitempty"`
}

// expectedState is the state an action should reach ("" = don't poll).
func expectedState(a bmc.PowerAction) bmc.PowerState {
	switch a {
	case bmc.ActionOn, bmc.ActionCycle, bmc.ActionReset:
		return bmc.PowerOn
	case bmc.ActionOff, bmc.ActionShutdown:
		return bmc.PowerOff
	}
	return ""
}

func (a *api) powerAction(ctx context.Context, c bmc.PowerController, h *sessions.Handle, r *http.Request) (any, error) {
	var req PowerRequest
	if err := bmc.DecodeJSON(r, &req); err != nil {
		return nil, err
	}
	act, err := bmc.ParsePowerAction(req.Action)
	if err != nil {
		return nil, err
	}
	wait := true
	if req.Wait != nil {
		wait = *req.Wait
	}
	if qs := r.URL.Query().Get("wait"); qs != "" {
		if b, err := strconv.ParseBool(qs); err == nil {
			wait = b
		}
	}
	timeout := a.cfg.PowerPollTimeout
	if req.TimeoutSeconds != nil {
		if *req.TimeoutSeconds < 0 {
			return nil, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "Power", "timeoutSeconds must be >= 0")
		}
		timeout = time.Duration(*req.TimeoutSeconds * float64(time.Second))
	}
	timeout = min(timeout, a.cfg.PowerPollMax)

	if err := c.Power(ctx, act); err != nil {
		return nil, err
	}
	// Reads served after this point must see the new state.
	a.s.Cache().InvalidateHost(h.Key.Host)

	res := PowerActionResult{Action: act, Expected: expectedState(act), State: bmc.PowerUnknown, TimeoutSeconds: timeout.Seconds()}
	if !wait || res.Expected == "" || timeout <= 0 {
		return server.WithStatus(http.StatusAccepted, res), nil
	}
	res.Polled = true
	log := obs.FromContext(ctx, obs.SubNorth)
	start := time.Now()
	deadline := start.Add(timeout)
	tick := time.NewTimer(min(a.cfg.PowerPollInterval, timeout))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			res.WaitedSeconds = time.Since(start).Seconds()
			return nil, ctx.Err()
		case <-tick.C:
		}
		st, err := c.PowerState(ctx)
		res.Polls++
		if err != nil {
			res.LastPollError = server.ErrorMessage(err, false)
		} else {
			res.State = st
			if st == res.Expected {
				res.Converged = true
			}
		}
		log.DebugContext(ctx, "power poll", "action", act, "state", res.State, "poll", res.Polls)
		now := time.Now()
		if res.Converged || !now.Before(deadline) {
			break
		}
		tick.Reset(min(a.cfg.PowerPollInterval, deadline.Sub(now)))
	}
	res.WaitedSeconds = time.Since(start).Seconds()
	if !res.Converged {
		res.Warning = "power state did not reach " + string(res.Expected) + " within the timeout; the BMC may have ignored the action (e.g. a graceful shutdown during POST)"
		return server.WithStatus(http.StatusAccepted, res), nil
	}
	return res, nil
}
