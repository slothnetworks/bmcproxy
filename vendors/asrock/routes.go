package asrock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/oem"
)

// VendorRoutes implements bmc.VendorRouter: everything under
// /api/v1/vendor/asrock/. Handlers type-assert the session to *Session.
func (d *Driver) VendorRoutes() []bmc.Route {
	return append(oemRoutes(), rawRoutes()...)
}

// withSession adapts a *Session handler to bmc.HandlerFunc.
func withSession(fn func(*Session, *http.Request) (any, error)) bmc.HandlerFunc {
	return func(s bmc.Session, r *http.Request) (any, error) {
		as, ok := s.(*Session)
		if !ok || as == nil {
			return nil, bmc.Errorf(bmc.ErrNotFound, Pkg, r.URL.Path, "target is not served by the asrock driver (%T)", s)
		}
		return fn(as, r)
	}
}

// decodeOptional is bmc.DecodeJSON that accepts an empty body.
func decodeOptional(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, r.URL.Path, "read body: %v", err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: decode request body: %v", bmc.ErrInvalidArgument, err)
	}
	return nil
}

// fanRefs parses ?type=open,closed&table=1,2 into table references.
func fanRefs(r *http.Request) ([]oem.TableRef, error) {
	q := r.URL.Query()
	types := splitList(q.Get("type"))
	if len(types) == 0 {
		types = []string{"open", "closed"}
	}
	tables := []int{1}
	if ts := splitList(q.Get("table")); len(ts) > 0 {
		tables = tables[:0]
		for _, t := range ts {
			n, err := strconv.Atoi(t)
			if err != nil || n < 1 || n > 16 {
				return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "fanTables", "bad table %q (want 1..16)", t)
			}
			tables = append(tables, n)
		}
	}
	var refs []oem.TableRef
	for _, typ := range types {
		for _, n := range tables {
			refs = append(refs, oem.TableRef{Type: typ, Table: n})
		}
	}
	return refs, nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
