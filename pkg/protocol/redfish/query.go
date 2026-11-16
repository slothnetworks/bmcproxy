package redfish

import (
	"context"
	"strconv"
	"strings"
)

// QueryCaps records which OData query options a service accepts. It is
// derived from ServiceRoot.ProtocolFeaturesSupported unless overridden by
// Quirks.QueryCaps. A service without ProtocolFeaturesSupported gets no
// query options at all: older AMI stacks (X470) 501 on unknown parameters.
type QueryCaps struct {
	Select     bool
	Expand     bool
	ExpandAll  bool // "*"
	ExpandDot  bool // "." (NoLinks)
	ExpandLink bool // "~" (Links)
	Levels     bool
	MaxLevels  int
	Filter     bool
	TopSkip    bool
	OnlyMember bool
	Excerpt    bool
}

// CapsFromFeatures derives QueryCaps from ProtocolFeaturesSupported (nil ->
// no capabilities).
func CapsFromFeatures(pf *ProtocolFeatures) QueryCaps {
	if pf == nil {
		return QueryCaps{}
	}
	qc := QueryCaps{
		Select:     pf.SelectQuery,
		Filter:     pf.FilterQuery,
		TopSkip:    pf.TopSkipQuery,
		OnlyMember: pf.OnlyMemberQuery,
		Excerpt:    pf.ExcerptQuery,
	}
	if e := pf.ExpandQuery; e != nil {
		qc.ExpandAll, qc.ExpandDot, qc.ExpandLink, qc.Levels = e.ExpandAll, e.NoLinks, e.Links, e.Levels
		qc.MaxLevels = e.MaxLevels
		qc.Expand = e.ExpandAll || e.NoLinks || e.Links
	}
	return qc
}

// QueryCaps returns the effective capabilities, fetching the ServiceRoot
// on first use (an unreachable root yields no capabilities).
func (c *Client) QueryCaps(ctx context.Context) QueryCaps {
	if c.opts.Quirks.StripQuery {
		return QueryCaps{}
	}
	c.mu.Lock()
	if c.capsSet {
		qc := c.caps
		c.mu.Unlock()
		return qc
	}
	c.mu.Unlock()
	if c.opts.Quirks.QueryCaps != nil {
		c.mu.Lock()
		c.caps, c.capsSet = *c.opts.Quirks.QueryCaps, true
		c.mu.Unlock()
		return *c.opts.Quirks.QueryCaps
	}
	root, err := c.ServiceRoot(ctx)
	qc := QueryCaps{}
	if err == nil {
		qc = CapsFromFeatures(root.ProtocolFeaturesSupported)
	}
	if err == nil {
		c.mu.Lock()
		c.caps, c.capsSet = qc, true
		c.mu.Unlock()
	}
	return qc
}

// SetQueryCaps replaces the effective capabilities at runtime.
func (c *Client) SetQueryCaps(qc QueryCaps) {
	c.mu.Lock()
	c.caps, c.capsSet = qc, true
	c.mu.Unlock()
}

func (c *Client) disableQueryCaps(q *Query) {
	if q == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.capsSet {
		c.capsSet = true
	}
	if len(q.Select) > 0 {
		c.caps.Select = false
	}
	if q.Expand != "" {
		c.caps.Expand, c.caps.ExpandAll, c.caps.ExpandDot, c.caps.ExpandLink, c.caps.Levels = false, false, false, false, false
	}
	if q.Filter != "" {
		c.caps.Filter = false
	}
	if q.Top != nil || q.Skip != nil {
		c.caps.TopSkip = false
	}
	if q.Only {
		c.caps.OnlyMember = false
	}
	if q.Excerpt {
		c.caps.Excerpt = false
	}
}

// Query is a set of OData query options. Options the service doesn't
// advertise are dropped at send time.
type Query struct {
	Select []string
	// Expand is "*", "." or "~" (default "." when Levels > 0 and Expand is
	// empty).
	Expand  string
	Levels  int
	Filter  string
	Top     *int
	Skip    *int
	Only    bool
	Excerpt bool
}

// Int is a helper for Query.Top/Skip.
func Int(n int) *int { return &n }

// Encode renders q against caps (all options allowed when caps is nil). The
// `$` of system query options is sent literally; values are
// percent-encoded except for the characters OData uses structurally.
func (q Query) Encode(caps *QueryCaps) string {
	all := QueryCaps{Select: true, Expand: true, ExpandAll: true, ExpandDot: true, ExpandLink: true, Levels: true,
		Filter: true, TopSkip: true, OnlyMember: true, Excerpt: true}
	if caps != nil {
		all = *caps
	}
	s, _ := q.encode(all)
	return s
}

func (q Query) encode(caps QueryCaps) (string, []string) {
	var parts, dropped []string
	add := func(ok bool, name, val string) {
		if ok {
			if val == "" {
				parts = append(parts, name)
			} else {
				parts = append(parts, name+"="+escapeQueryValue(val))
			}
		} else {
			dropped = append(dropped, name)
		}
	}
	if q.Only {
		add(caps.OnlyMember, "only", "")
	}
	if q.Excerpt {
		add(caps.Excerpt, "excerpt", "")
	}
	if len(q.Select) > 0 {
		add(caps.Select, "$select", strings.Join(q.Select, ","))
	}
	if q.Expand != "" || q.Levels > 0 {
		e := q.Expand
		if e == "" {
			e = "."
		}
		ok := caps.Expand
		switch e {
		case "*":
			ok = ok && caps.ExpandAll
		case ".":
			ok = ok && caps.ExpandDot
		case "~":
			ok = ok && caps.ExpandLink
		}
		v := e
		if q.Levels > 0 {
			lv := q.Levels
			if caps.MaxLevels > 0 && lv > caps.MaxLevels {
				lv = caps.MaxLevels
			}
			if caps.Levels {
				v = e + "($levels=" + strconv.Itoa(lv) + ")"
			} else if lv > 1 {
				ok = false
			}
		}
		add(ok, "$expand", v)
	}
	if q.Filter != "" {
		add(caps.Filter, "$filter", q.Filter)
	}
	if q.Top != nil {
		add(caps.TopSkip, "$top", strconv.Itoa(*q.Top))
	}
	if q.Skip != nil {
		add(caps.TopSkip, "$skip", strconv.Itoa(*q.Skip))
	}
	return strings.Join(parts, "&"), dropped
}

// escapeQueryValue percent-encodes a query value, leaving the OData
// structural characters ($ ( ) = , . * ~ ' : /) literal so `$expand=.($levels=1)`
// goes over the wire exactly as the Redfish spec writes it.
func escapeQueryValue(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9':
			b.WriteByte(ch)
		case strings.IndexByte("-_.~$()=,*':/!@", ch) >= 0:
			b.WriteByte(ch)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[ch>>4])
			b.WriteByte(hex[ch&15])
		}
	}
	return b.String()
}
