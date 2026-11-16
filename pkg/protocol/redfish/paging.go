package redfish

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// WalkCollection GETs a collection and follows Members@odata.nextLink,
// calling fn with each page's raw members. The walk is bounded by
// Options.MaxPages and refuses to revisit a URL (loop guard). The first
// page may carry client query options q (filtered by capability).
// Server-generated nextLinks are followed as given, except that $expand and
// $select from q are carried over when the nextLink lacks them: the live
// B650 4.10 answers `?$skip=50` alone (docs/b650/live-probe.md §4), and
// following it verbatim would degrade an expanded read into one GET per
// member on every page after the first.
func (c *Client) WalkCollection(ctx context.Context, path string, q *Query, fn func(members []json.RawMessage) error) error {
	seen := map[string]bool{}
	next := path
	first := true
	log := obs.FromContext(ctx, obs.SubRedfish)
	for page := 0; next != ""; page++ {
		if page >= c.opts.MaxPages {
			log.WarnContext(ctx, "redfish nextLink walk hit page cap", "path", path, "pages", page)
			return &bmc.OpError{Pkg: Pkg, Op: "GET " + path, Backend: bmc.BackendRedfish,
				Detail: "nextLink walk exceeded page cap", Err: bmc.ErrUpstream}
		}
		var opts []RequestOption
		if first && q != nil {
			opts = append(opts, WithQuery(*q))
		} else if q != nil {
			if cq, ok := carryQuery(*q, next); ok {
				opts = append(opts, WithQuery(cq))
			}
		}
		var coll Collection
		resp, err := c.Get(ctx, next, &coll, opts...)
		if err != nil {
			return err
		}
		key := resp.Path
		if first {
			key = next
		}
		seen[key] = true
		seen[next] = true
		if err := fn(coll.Members); err != nil {
			return err
		}
		first = false
		nl := coll.NextLink
		if nl == "" {
			return nil
		}
		nl = c.pathOf(nl)
		if seen[nl] {
			log.WarnContext(ctx, "redfish nextLink loop detected; stopping", "path", path, "next", nl)
			return nil
		}
		if len(coll.Members) == 0 {
			log.WarnContext(ctx, "redfish nextLink on empty page; stopping", "path", path, "next", nl)
			return nil
		}
		next = nl
	}
	return nil
}

// carryQuery returns the shape-preserving options of q ($expand/$levels and
// $select) that the nextLink does not already carry.
func carryQuery(q Query, next string) (Query, bool) {
	has := map[string]bool{}
	if i := strings.IndexByte(next, '?'); i >= 0 {
		for _, kv := range strings.Split(next[i+1:], "&") {
			k, _, _ := strings.Cut(kv, "=")
			if uk, err := url.QueryUnescape(k); err == nil {
				k = uk
			}
			has[strings.ToLower(k)] = true
		}
	}
	var out Query
	if (q.Expand != "" || q.Levels > 0) && !has["$expand"] {
		out.Expand, out.Levels = q.Expand, q.Levels
	}
	if len(q.Select) > 0 && !has["$select"] {
		out.Select = q.Select
	}
	return out, out.Expand != "" || out.Levels > 0 || len(out.Select) > 0
}

// MemberLinks returns every member @odata.id of a (paged) collection.
func (c *Client) MemberLinks(ctx context.Context, path string) ([]string, error) {
	var out []string
	err := c.WalkCollection(ctx, path, nil, func(ms []json.RawMessage) error {
		for _, m := range ms {
			var l Link
			if json.Unmarshal(m, &l) == nil && l.ID != "" {
				out = append(out, l.ID)
			}
		}
		return nil
	})
	return out, err
}

// isInlined reports whether a member carries more than its @odata.id.
func isInlined(m json.RawMessage) bool {
	var probe map[string]json.RawMessage
	if json.Unmarshal(m, &probe) != nil {
		return false
	}
	for k := range probe {
		if k != "@odata.id" {
			return true
		}
	}
	return false
}

// Members fetches every member of a collection as T. When the service
// advertises $expand (and Quirks.NoExpand is unset) one expanded walk is
// tried first; members that come back as bare links are fetched one by
// one. Members that 404 are skipped.
func Members[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var out []T
	var pending []string
	var q *Query
	if caps := c.QueryCaps(ctx); caps.Expand && caps.ExpandDot && !c.opts.Quirks.NoExpand {
		q = &Query{Expand: ".", Levels: 1}
	}
	err := c.WalkCollection(ctx, path, q, func(ms []json.RawMessage) error {
		for _, m := range ms {
			if q != nil && isInlined(m) {
				var v T
				if err := decodeLenient(m, &v, c.opts.Quirks); err == nil {
					out = append(out, v)
					continue
				}
			}
			var l Link
			if json.Unmarshal(m, &l) == nil && l.ID != "" {
				pending = append(pending, l.ID)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, p := range pending {
		var v T
		if _, err := c.Get(ctx, p, &v); err != nil {
			if IsStatus(err, 404) {
				continue
			}
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// GetAll fetches each path as T, skipping 404s.
func GetAll[T any](ctx context.Context, c *Client, paths []string) ([]T, error) {
	out := make([]T, 0, len(paths))
	for _, p := range paths {
		var v T
		if _, err := c.Get(ctx, p, &v); err != nil {
			if IsStatus(err, 404) {
				continue
			}
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
