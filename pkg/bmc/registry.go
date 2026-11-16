package bmc

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry holds drivers. Most code uses the package-level functions, which
// operate on DefaultRegistry; tests can build their own with NewRegistry.
type Registry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
	order   []string // registration order (tie-break)
}

// DefaultRegistry is the process-wide registry used by Register/Resolve.
var DefaultRegistry = NewRegistry()

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{drivers: map[string]Driver{}} }

// Register adds d. It panics if d is nil, its Name is empty, or a driver of
// the same name (case-insensitive) is already registered — like
// database/sql.Register, misregistration is a programming error.
func (r *Registry) Register(d Driver) {
	if d == nil {
		panic("bmc: Register driver is nil")
	}
	name := strings.ToLower(d.Name())
	if name == "" {
		panic("bmc: Register driver with empty name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.drivers[name]; dup {
		panic("bmc: Register called twice for driver " + name)
	}
	r.drivers[name] = d
	r.order = append(r.order, name)
}

// Lookup returns the driver named name (case-insensitive).
func (r *Registry) Lookup(name string) (Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.drivers[strings.ToLower(strings.TrimSpace(name))]
	return d, ok
}

// Drivers returns all drivers in detection order: descending Priority, then
// registration order.
func (r *Registry) Drivers() []Driver {
	r.mu.RLock()
	ds := make([]Driver, 0, len(r.order))
	for _, n := range r.order {
		ds = append(ds, r.drivers[n])
	}
	r.mu.RUnlock()
	sort.SliceStable(ds, func(i, j int) bool { return priority(ds[i]) > priority(ds[j]) })
	return ds
}

func priority(d Driver) int {
	if p, ok := d.(Prioritized); ok {
		return p.Priority()
	}
	return 0
}

// Resolve picks the driver for t.
//
//   - If t.VendorHint is set, that driver is used (unknown name ->
//     ErrInvalidArgument). Its Detect is still called to fill Match
//     (board/firmware), but a detection failure or zero score is ignored;
//     the returned Match then has at least Vendor set.
//   - Otherwise every driver is asked to Detect in Drivers() order (using
//     DetectWithCreds when the driver implements CredDetector and c is
//     non-nil). The first Match with Score >= ScoreCertain wins immediately;
//     otherwise the highest score wins (ties: earlier in order).
//   - If no driver matches, the error wraps ErrUnsupported, joined with any
//     detection errors. If every driver failed with an error (none returned
//     a clean no-match), the first detection error's class is kept so a
//     timeout surfaces as ErrTimeout (504) rather than 501.
func (r *Registry) Resolve(ctx context.Context, t Target, c *Creds) (Driver, Match, error) {
	detect := func(d Driver) (Match, error) {
		if cd, ok := d.(CredDetector); ok && c != nil {
			return cd.DetectWithCreds(ctx, t, *c)
		}
		return d.Detect(ctx, t)
	}
	if t.VendorHint != "" {
		d, ok := r.Lookup(t.VendorHint)
		if !ok {
			return nil, Match{}, &OpError{Pkg: "bmc", Op: "Resolve", Err: ErrInvalidArgument,
				Detail: fmt.Sprintf("unknown vendor %q (X-BMC-Vendor)", t.VendorHint)}
		}
		m, err := detect(d)
		if err != nil || m.Vendor == "" {
			m.Vendor = d.Name()
		}
		return d, m, nil
	}

	var (
		best     Driver
		bestM    Match
		errs     []error
		cleanNos int
	)
	for _, d := range r.Drivers() {
		if err := ctx.Err(); err != nil {
			return nil, Match{}, Wrap("bmc", "Resolve", err)
		}
		m, err := detect(d)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.Name(), err))
			continue
		}
		if m.Score <= ScoreNone {
			cleanNos++
			continue
		}
		if m.Vendor == "" {
			m.Vendor = d.Name()
		}
		if m.Score >= ScoreCertain {
			return d, m, nil
		}
		if best == nil || m.Score > bestM.Score {
			best, bestM = d, m
		}
	}
	if best != nil {
		return best, bestM, nil
	}
	op := &OpError{Pkg: "bmc", Op: "Resolve", Detail: "no driver recognised " + t.Addr()}
	switch {
	case len(errs) > 0 && cleanNos == 0:
		first := errs[0]
		if !hasSentinel(first) {
			first = fmt.Errorf("%w: %w", classify(first), first)
		}
		op.Err = errors.Join(append([]error{first}, errs[1:]...)...)
	case len(errs) > 0:
		op.Err = errors.Join(append([]error{ErrUnsupported}, errs...)...)
	default:
		op.Err = ErrUnsupported
	}
	return nil, Match{}, op
}

// Register adds d to DefaultRegistry. Call it from a driver's init.
func Register(d Driver) { DefaultRegistry.Register(d) }

// Lookup finds a driver by name in DefaultRegistry.
func Lookup(name string) (Driver, bool) { return DefaultRegistry.Lookup(name) }

// Drivers lists DefaultRegistry's drivers in detection order.
func Drivers() []Driver { return DefaultRegistry.Drivers() }

// Resolve resolves t against DefaultRegistry. c may be nil.
func Resolve(ctx context.Context, t Target, c *Creds) (Driver, Match, error) {
	return DefaultRegistry.Resolve(ctx, t, c)
}
