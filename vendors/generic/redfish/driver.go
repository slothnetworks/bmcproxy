// Package genericredfish is the fallback bmc.Driver ("redfish") for any
// standards-compliant Redfish BMC. It detects via the unauthenticated
// ServiceRoot with a low score (bmc.ScoreFallback) and a negative priority,
// so vendor drivers win whenever they recognise the board.
//
// The Session it returns is exported and constructible from any
// *redfish.Client (NewSession), so vendor drivers (e.g. ASRock on the B650,
// where Redfish works) can embed it and override or hide capabilities.
package genericredfish

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// Name is the registry name / X-BMC-Vendor value.
const Name = "redfish"

// Priority is below every vendor driver (default 0).
const Priority = -100

// Pkg is the OpError.Pkg value for this driver.
const Pkg = "driver.redfish"

// Driver is the generic Redfish driver.
type Driver struct {
	mu sync.Mutex
	tm *transport.Manager
	// Options are copied into every client the driver creates (Creds and
	// Auth are set by Open).
	Options rf.Options
	// Auth selects Basic (default) or SessionService sessions.
	Auth rf.AuthMode
}

// Default is the registered instance.
var Default = &Driver{}

func init() { bmc.Register(Default) }

// New returns a driver bound to tm (nil = a private default Manager
// created on first use).
func New(tm *transport.Manager) *Driver { return &Driver{tm: tm} }

// SetTransport binds the driver to the process transport.Manager. The
// server should call genericredfish.Default.SetTransport(tm) at startup;
// otherwise a private Manager with default settings is created lazily.
func (d *Driver) SetTransport(tm *transport.Manager) {
	d.mu.Lock()
	d.tm = tm
	d.mu.Unlock()
}

// Transport returns the driver's transport.Manager: the one set via
// SetTransport, else the process-wide transport.Default().
func (d *Driver) Transport() *transport.Manager {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.tm == nil {
		return transport.Default()
	}
	return d.tm
}

// Name implements bmc.Driver.
func (d *Driver) Name() string { return Name }

// Priority implements bmc.Prioritized.
func (d *Driver) Priority() int { return Priority }

// NewClient builds a Redfish client for t on the LaneRedfish endpoint.
func (d *Driver) NewClient(t bmc.Target, c bmc.Creds, auth rf.AuthMode) (*rf.Client, error) {
	ep := d.Transport().Endpoint(t.Addr(), transport.LaneRedfish)
	o := d.Options
	o.Creds, o.Auth = c, auth
	return rf.New(ep, o)
}

// Detect GETs the unauthenticated ServiceRoot. Any Redfish service scores
// bmc.ScoreFallback; non-Redfish answers return a zero Match. Transport
// failures (timeout, connection refused) are returned as errors.
func (d *Driver) Detect(ctx context.Context, t bmc.Target) (bmc.Match, error) {
	c, err := d.NewClient(t, bmc.Creds{}, rf.AuthNone)
	if err != nil {
		return bmc.Match{}, err
	}
	root, err := c.ServiceRoot(ctx)
	if err != nil {
		var oe *bmc.OpError
		if errors.As(err, &oe) && oe.UpstreamStatus != 0 {
			// The host answered HTTP but isn't a Redfish service.
			return bmc.Match{}, nil
		}
		if errors.Is(err, bmc.ErrUnsupported) {
			return bmc.Match{}, nil
		}
		return bmc.Match{}, err
	}
	return MatchFromRoot(root), nil
}

// MatchFromRoot builds the fallback Match from a ServiceRoot.
func MatchFromRoot(root *rf.ServiceRoot) bmc.Match {
	m := bmc.Match{Score: bmc.ScoreFallback, Vendor: Name, Manufacturer: root.Vendor,
		Backends: []string{bmc.BackendRedfish},
		Details: map[string]string{
			"redfishVersion":  root.RedfishVersion,
			"product":         root.Product,
			"vendor":          root.Vendor,
			"serviceRootType": root.ODataType,
		}}
	if root.ProtocolFeaturesSupported == nil {
		m.Details["queryOptions"] = "none"
	}
	for k, v := range m.Details {
		if v == "" {
			delete(m.Details, k)
		}
	}
	return m
}

// Open authenticates (Basic by default) and verifies the credentials with
// a GET of the Systems collection, then builds a Session.
func (d *Driver) Open(ctx context.Context, t bmc.Target, c bmc.Creds) (bmc.Session, error) {
	cl, err := d.NewClient(t, c, d.Auth)
	if err != nil {
		return nil, err
	}
	if err := cl.Login(ctx); err != nil {
		_ = cl.Close(ctx)
		return nil, err
	}
	s, err := NewSession(ctx, cl, SessionOptions{DriverName: Name, Target: t, CloseClient: true})
	if err != nil {
		_ = cl.Close(ctx)
		return nil, err
	}
	obs.FromContext(ctx, obs.SubDriverRedfish).DebugContext(ctx, "redfish session opened",
		"bmc", t.Addr(), "user", c.User, "auth", d.Auth.String(), "board", s.Info().Board)
	return s, nil
}

func lower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
