// Package ipmi is a small IPMI-over-LAN (RMCP+/lanplus) client over
// github.com/bougou/go-ipmi, normalising into pkg/bmc types.
//
// go-ipmi is hidden behind the Conn interface so the Client is testable with
// a fake. Every command to one BMC is serialised (IPMI session slots are
// finite and the BMCs are single-threaded); each command is logged under the
// "ipmi" obs subsystem with its NetFn/Cmd and completion code, and an
// optional Hook receives the same information for metrics.
package ipmi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// Pkg is the OpError.Pkg value.
const Pkg = "ipmi"

// Options configure Connect.
type Options struct {
	// Dialer opens the Conn (default: go-ipmi lanplus).
	Dialer Dialer
	// Timeout per UDP exchange (default 2 s) and Retries per exchange
	// (default 2).
	Timeout time.Duration
	Retries int
	// Channel is the LAN channel for lan print / user list (default 1).
	Channel uint8
	// Hook, if set, is called after every command (synchronously; keep it
	// fast).
	Hook func(CommandInfo)
	// ClockOffset (bmc - proxy) is used to correct SEL timestamps.
	ClockOffset time.Duration
}

// CommandInfo describes one IPMI command for the debug hook.
type CommandInfo struct {
	Host     string
	Op       string
	NetFn    uint8
	Cmd      uint8
	CC       uint8 // completion code (0 = OK or unknown)
	Duration time.Duration
	Err      error
}

// Client is an IPMI session to one BMC. It is safe for concurrent use;
// commands are serialised per host across all Clients in the process.
type Client struct {
	addr string
	conn Conn
	opts Options
	lock chan struct{}

	mu     sync.Mutex
	closed bool
}

var (
	hostLocksMu sync.Mutex
	hostLocks   = map[string]chan struct{}{}
)

func hostLock(addr string) chan struct{} {
	hostLocksMu.Lock()
	defer hostLocksMu.Unlock()
	l, ok := hostLocks[addr]
	if !ok {
		l = make(chan struct{}, 1)
		hostLocks[addr] = l
	}
	return l
}

func acquire(ctx context.Context, l chan struct{}) error {
	select {
	case l <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Connect opens a lanplus session to addr ("host:623", use
// bmc.Target.IPMIAddr()). Authentication failures wrap bmc.ErrAuth,
// unreachable/timeouts bmc.ErrTimeout.
func Connect(ctx context.Context, addr string, creds bmc.Creds, opts Options) (*Client, error) {
	if opts.Dialer == nil {
		opts.Dialer = DialLanplus
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Second
	}
	if opts.Retries <= 0 {
		opts.Retries = 2
	}
	if opts.Channel == 0 {
		opts.Channel = 1
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "623")
	}
	c := &Client{addr: addr, opts: opts, lock: hostLock(addr)}
	if err := acquire(ctx, c.lock); err != nil {
		return nil, bmc.Wrap(Pkg, "Connect", err)
	}
	defer func() { <-c.lock }()
	start := time.Now()
	conn, err := opts.Dialer(ctx, addr, creds, opts)
	c.report(ctx, "Connect", 0x06, 0x3c, start, err) // Activate Session family (RMCP+ open/RAKP)
	if err != nil {
		return nil, classify("Connect", err)
	}
	c.conn = conn
	return c, nil
}

// NewWithConn wraps an existing Conn (tests, or callers managing the
// transport themselves).
func NewWithConn(addr string, conn Conn, opts Options) *Client {
	if opts.Channel == 0 {
		opts.Channel = 1
	}
	return &Client{addr: addr, conn: conn, opts: opts, lock: hostLock(addr)}
}

// Addr returns the BMC address.
func (c *Client) Addr() string { return c.addr }

// SetClockOffset updates the SEL timestamp correction.
func (c *Client) SetClockOffset(d time.Duration) {
	c.mu.Lock()
	c.opts.ClockOffset = d
	c.mu.Unlock()
}

// Close closes the IPMI session. Idempotent.
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	return c.exec(ctx, "CloseSession", 0x06, 0x3c, func(ctx context.Context) error { return c.conn.Close(ctx) })
}

// exec runs fn under the per-host lock with logging and error mapping.
func (c *Client) exec(ctx context.Context, op string, netfn, cmd uint8, fn func(context.Context) error) error {
	c.mu.Lock()
	closed := c.closed && op != "CloseSession"
	c.mu.Unlock()
	if closed {
		return bmc.Errorf(bmc.ErrUpstream, Pkg, op, "session closed")
	}
	if err := acquire(ctx, c.lock); err != nil {
		return bmc.Wrap(Pkg, op, err)
	}
	defer func() { <-c.lock }()
	start := time.Now()
	err := fn(ctx)
	c.report(ctx, op, netfn, cmd, start, err)
	if err != nil {
		return classify(op, err)
	}
	return nil
}

func (c *Client) report(ctx context.Context, op string, netfn, cmd uint8, start time.Time, err error) {
	ci := CommandInfo{Host: c.addr, Op: op, NetFn: netfn, Cmd: cmd, Duration: time.Since(start), Err: err}
	var cc CompletionCoder
	if errors.As(err, &cc) {
		ci.CC = cc.IPMICompletionCode()
	}
	log := obs.FromContext(ctx, obs.SubIPMI)
	attrs := []any{"bmc", c.addr, "op", op, "netfn", fmt.Sprintf("0x%02x", netfn), "cmd", fmt.Sprintf("0x%02x", cmd),
		"cc", fmt.Sprintf("0x%02x", ci.CC), "dur_ms", ci.Duration.Milliseconds()}
	if err != nil {
		log.DebugContext(ctx, "ipmi command failed", append(attrs, "err", err)...)
	} else {
		log.DebugContext(ctx, "ipmi command", attrs...)
	}
	if c.opts.Hook != nil {
		c.opts.Hook(ci)
	}
}

// classify maps a Conn error to a *bmc.OpError.
func classify(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return bmc.Wrap(Pkg, op, err)
	}
	for _, s := range []error{bmc.ErrAuth, bmc.ErrUnsupported, bmc.ErrBusy, bmc.ErrTimeout, bmc.ErrNotFound, bmc.ErrInvalidArgument, bmc.ErrUpstream} {
		if errors.Is(err, s) {
			return &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendIPMI, Err: err}
		}
	}
	var cc CompletionCoder
	if errors.As(err, &cc) {
		code := cc.IPMICompletionCode()
		var sentinel error
		switch code {
		case CCNodeBusy:
			sentinel = bmc.ErrBusy
		case CCInvalidCommand, CCInvalidCommandForLUN, CCNotSupportedInState, CCSubfunctionDisabled:
			sentinel = bmc.ErrUnsupported
		case CCTimeout:
			sentinel = bmc.ErrTimeout
		case CCParameterOutOfRange, CCInvalidDataField:
			sentinel = bmc.ErrInvalidArgument
		case CCDataNotPresent:
			sentinel = bmc.ErrNotFound
		case CCInsufficientPriv:
			sentinel = bmc.ErrAuth
		default:
			sentinel = bmc.ErrUpstream
		}
		return &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendIPMI, Detail: fmt.Sprintf("cc=0x%02x", code),
			Err: fmt.Errorf("%w: %w", sentinel, err)}
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() || strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendIPMI, Err: fmt.Errorf("%w: %w", bmc.ErrTimeout, err)}
	}
	return &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendIPMI, Err: fmt.Errorf("%w: %w", bmc.ErrUpstream, err)}
}
