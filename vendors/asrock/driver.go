// Package asrock is the ASRock Rack bmc.Driver ("asrock"). It speaks three
// southbound protocols and picks one per capability, per board profile:
//
//   - Redfish (pkg/protocol/redfish via vendors/generic/redfish) where the
//     BMC accepts the caller's credentials (B650D4U 4.10: Redfish-first);
//   - the legacy AMI MegaRAC /api (pkg/protocol/amimegarac) for everything
//     on the X470D4U 2.32 (whose Redfish rejects the admin account) and for
//     ASRock OEM features and Redfish gaps elsewhere;
//   - IPMI-over-LAN (pkg/protocol/ipmi), connected lazily, for boot override
//     and power restore policy on boards whose /api lacks them (X470).
//
// The board and live firmware come from /api/asrr/model-name and
// /api/asrr/fw-info and select a profile in vendors/asrock/boards. Unknown
// boards or versions fall back to capability probing. ASRock-only features
// are exposed as vendor routes under /api/v1/vendor/asrock/ (routes.go).
//
// Register with a blank import of this package (cmd/bmcproxy/drivers.go).
// The driver uses transport.Default() unless New is given a Manager.
package asrock

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/ipmi"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/boards"
)

// Name is the registry name, X-BMC-Vendor value and vendor route prefix.
const Name = "asrock"

// Pkg is the OpError.Pkg value for this driver.
const Pkg = "driver.asrock"

// Detection scores. Any ASRock signal scores above the generic Redfish
// fallback (bmc.ScoreFallback).
const (
	// scoreAMI: an AMI MegaRAC /api answered (unauthenticated 401 {cc:7}).
	// MegaRAC is shared by several vendors, so this is only "probably ours".
	scoreAMI = 30
	// scoreAMIRedfish: /api plus an AMI Redfish ServiceRoot.
	scoreAMIRedfish = 35
	// scoreNonASRock: a MegaRAC that is evidently not an ASRock board:
	// another vendor's markers in the unauthenticated ServiceRoot or
	// $metadata (e.g. the Gigabyte GBT* OEM namespaces), or a successful
	// /api login without the ASRock-only asrr namespace. It is below
	// bmc.ScoreFallback, so the generic Redfish driver (and any vendor
	// driver) wins; it stays above zero so a MegaRAC without a usable
	// Redfish service can still be served over /api as a last resort.
	scoreNonASRock = bmc.ScoreFallback / 2
)

// DefaultHandoffTTL is how long a session logged in by DetectWithCreds
// waits for Open to adopt it before it is logged out.
const DefaultHandoffTTL = 30 * time.Second

// Options configure a Driver. The zero value is production-ready.
type Options struct {
	// Transport is the southbound HTTP manager (nil = transport.Default(),
	// the process-wide Manager installed by cmd/bmcproxy).
	Transport *transport.Manager
	// IPMIDialer opens IPMI connections (nil = go-ipmi lanplus). Tests
	// inject a fake.
	IPMIDialer ipmi.Dialer
	// Now is the clock (tests). Default time.Now.
	Now func() time.Time
	// HandoffTTL overrides DefaultHandoffTTL (negative disables the
	// DetectWithCreds -> Open session handoff).
	HandoffTTL time.Duration
	// AMIConfig is applied to every amimegarac client (tests: short
	// LoginFailureBackoff etc.). Endpoint, Target, Creds, LoginExtra,
	// ExtraDeny and Hooks are set by the driver.
	AMIConfig amimegarac.Config
	// RedfishOptions are copied into every Redfish client (Creds and Auth
	// are set by the driver; Basic auth, since B650 SessionTimeout is 30 s).
	RedfishOptions rf.Options
}

// Driver is the ASRock Rack driver. Safe for concurrent use.
type Driver struct {
	opts Options

	mu       sync.Mutex
	families map[string]string // host -> boards.Family* (non-secret detection memory)
	handoffs map[string]*handoff
}

// Default is the registered instance.
var Default = New(Options{})

func init() { bmc.Register(Default) }

// New returns a driver with o.
func New(o Options) *Driver {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.HandoffTTL == 0 {
		o.HandoffTTL = DefaultHandoffTTL
	}
	return &Driver{opts: o, families: map[string]string{}, handoffs: map[string]*handoff{}}
}

// Name implements bmc.Driver.
func (d *Driver) Name() string { return Name }

// Priority implements bmc.Prioritized: the default (0), above the generic
// Redfish fallback (-100).
func (d *Driver) Priority() int { return 0 }

// Transport returns the driver's transport manager.
func (d *Driver) Transport() *transport.Manager {
	if d.opts.Transport != nil {
		return d.opts.Transport
	}
	return transport.Default()
}

func (d *Driver) log(ctx context.Context) interface {
	DebugContext(context.Context, string, ...any)
	InfoContext(context.Context, string, ...any)
	WarnContext(context.Context, string, ...any)
} {
	return obs.FromContext(ctx, obs.SubDriverASRock)
}

// ---- detection ----

// rootProbe is the result of an unauthenticated ServiceRoot GET.
type rootProbe struct {
	root *rf.ServiceRoot
	err  error
}

func (d *Driver) probeRoot(ctx context.Context, t bmc.Target) rootProbe {
	o := d.opts.RedfishOptions
	o.Creds, o.Auth = bmc.Creds{}, rf.AuthNone
	c, err := rf.New(d.Transport().Endpoint(t.Addr(), transport.LaneRedfish), o)
	if err != nil {
		return rootProbe{err: err}
	}
	root, err := c.ServiceRoot(transport.WithOp(ctx, "asrock.Detect.ServiceRoot"))
	return rootProbe{root: root, err: err}
}

// familyFromRoot classifies the BMC generation from its ServiceRoot: the
// X470 class serves ServiceRoot v1_3_x / RedfishVersion 1.5.x without a
// Vendor property; B650/B550 serve v1_7+ with Vendor "AMI".
func familyFromRoot(root *rf.ServiceRoot) string {
	if root == nil {
		return ""
	}
	t := strings.ToLower(root.ODataType)
	if strings.Contains(t, "v1_3_") || strings.Contains(t, "v1_2_") || strings.Contains(t, "v1_1_") ||
		strings.HasPrefix(root.RedfishVersion, "1.5.") || strings.HasPrefix(root.RedfishVersion, "1.4.") ||
		strings.HasPrefix(root.RedfishVersion, "1.0.") {
		return boards.FamilyX470
	}
	return boards.FamilyModern
}

func isAMIRoot(root *rf.ServiceRoot) bool {
	if root == nil {
		return false
	}
	if _, ok := root.Oem["Ami"]; ok {
		return true
	}
	s := strings.ToLower(root.Vendor + " " + root.Product)
	return strings.Contains(s, "ami")
}

func isASRockRoot(root *rf.ServiceRoot) bool {
	if root == nil {
		return false
	}
	s := strings.ToLower(root.Vendor + " " + root.Product)
	return strings.Contains(s, "asrock")
}

// foreignRootOem are ServiceRoot Oem keys of other MegaRAC OEMs (lower
// case). ASRock firmware only carries the stock AMI ones (Ami, Dre, Vendor).
var foreignRootOem = map[string]bool{"gbt": true, "gigabyte": true, "gct": true, "supermicro": true, "dell": true,
	"hpe": true, "hp": true, "lenovo": true, "inspur": true, "fujitsu": true, "quanta": true, "tyan": true}

// foreignRoot reports why an unauthenticated ServiceRoot is evidently not
// an ASRock board ("" = no evidence): a Vendor/Product naming neither AMI
// nor ASRock (e.g. "Giga Computing"), or another OEM's ServiceRoot Oem
// block. ASRock roots say Vendor "AMI" (B650/X570) or nothing (X470).
func foreignRoot(root *rf.ServiceRoot) string {
	if root == nil || isASRockRoot(root) {
		return ""
	}
	for _, v := range []string{root.Vendor, root.Product} {
		l := strings.ToLower(strings.TrimSpace(v))
		if l != "" && !strings.Contains(l, "ami") && !strings.Contains(l, "megarac") && !strings.Contains(l, "american megatrends") {
			return "ServiceRoot Vendor/Product " + strings.TrimSpace(v)
		}
	}
	for k := range root.Oem {
		if foreignRootOem[strings.ToLower(k)] {
			return "ServiceRoot Oem." + k
		}
	}
	return ""
}

// foreignMetadataMarkers are CSDL namespaces of other MegaRAC OEMs' Redfish
// extensions (Gigabyte / Giga Computing GBT*, Gbt*, Gct*: MC13-LE0 13.04.15
// and MZ32-AR0 12.61.39 declare them in the unauthenticated $metadata).
var foreignMetadataMarkers = [][]byte{[]byte(`Namespace="GBT`), []byte(`Namespace="Gbt`), []byte(`Namespace="Gct`), []byte(`Namespace="GCT`)}

// foreignMetadata GETs the unauthenticated /redfish/v1/$metadata (DMTF
// requires it to be readable without credentials) and reports another
// OEM's namespaces. Any failure (404, 401, timeout) is "no evidence".
func (d *Driver) foreignMetadata(ctx context.Context, t bmc.Target) string {
	o := d.opts.RedfishOptions
	o.Creds, o.Auth = bmc.Creds{}, rf.AuthNone
	c, err := rf.New(d.Transport().Endpoint(t.Addr(), transport.LaneRedfish), o)
	if err != nil {
		return ""
	}
	resp, err := c.Do(transport.WithOp(ctx, "asrock.Detect.metadata"), http.MethodGet, "/redfish/v1/$metadata", nil, rf.WithoutAuth())
	if err != nil || resp.Status != http.StatusOK {
		return ""
	}
	for _, m := range foreignMetadataMarkers {
		if bytes.Contains(resp.Body, m) {
			return "$metadata " + strings.TrimPrefix(string(m), "Namespace=") + "* OEM namespaces"
		}
	}
	return ""
}

// nonASRockMatch is the capped match for a MegaRAC that is not ASRock.
func nonASRockMatch(root *rf.ServiceRoot, ami bool, why string) bmc.Match {
	m := bmc.Match{Score: scoreNonASRock, Vendor: Name, Manufacturer: "AMI", Details: map[string]string{"nonASRock": why}}
	if root != nil {
		m.Backends = append(m.Backends, bmc.BackendRedfish)
		m.Details["redfishVersion"] = root.RedfishVersion
		m.Details["serviceRootType"] = root.ODataType
	}
	if ami {
		m.Backends = append(m.Backends, bmc.BackendAMI)
	}
	return m
}

// probeAMI sends one unauthenticated GET /api/asrr/model-name. A MegaRAC
// answers 401 {"cc":7,...} (or 200 with mb_name if it does not enforce
// auth). It never mutates the BMC.
func (d *Driver) probeAMI(ctx context.Context, t bmc.Target) (present bool, board string, err error) {
	ep := d.Transport().Endpoint(t.Addr(), transport.LaneAMI)
	req, err := ep.NewRequest(transport.WithOp(ctx, "asrock.Detect.AMI"), http.MethodGet, "/api/asrr/model-name", nil)
	if err != nil {
		return false, "", err
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json")
	resp, err := ep.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	var body struct {
		CC     *amimegarac.NullableInt `json:"cc"`
		MBName amimegarac.FlexString   `json:"mb_name"`
	}
	buf := make([]byte, 0, 512)
	tmp := make([]byte, 512)
	for {
		n, rerr := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if rerr != nil || len(buf) > 64<<10 {
			break
		}
	}
	_ = amimegarac.Decode(buf, &body)
	switch {
	case resp.StatusCode == http.StatusUnauthorized && body.CC != nil && body.CC.Valid:
		return true, "", nil
	case resp.StatusCode == http.StatusOK && body.MBName.Clean() != "":
		return true, body.MBName.Clean(), nil
	}
	return false, "", nil
}

// Detect implements bmc.Driver with two cheap unauthenticated probes: the
// Redfish ServiceRoot (Vendor/Product/Oem.Ami and version, which also tells
// the X470 generation from the B650 one) and an AMI /api auth-gate probe.
// It scores above the generic Redfish fallback whenever an AMI MegaRAC is
// found; board and firmware need credentials (DetectWithCreds).
func (d *Driver) Detect(ctx context.Context, t bmc.Target) (bmc.Match, error) {
	rp := d.probeRoot(ctx, t)
	ami, board, aerr := d.probeAMI(ctx, t)
	if fam := familyFromRoot(rp.root); fam != "" {
		d.setFamily(t.Addr(), fam)
	}
	m := bmc.Match{Vendor: Name, Details: map[string]string{}}
	if rp.root != nil {
		m.Backends = append(m.Backends, bmc.BackendRedfish)
		m.Manufacturer = rp.root.Vendor
		m.Details["redfishVersion"] = rp.root.RedfishVersion
		m.Details["serviceRootType"] = rp.root.ODataType
		if rp.root.Product != "" {
			m.Details["product"] = rp.root.Product
		}
		m.Details["family"] = familyFromRoot(rp.root)
	}
	if ami {
		m.Backends = append(m.Backends, bmc.BackendAMI)
		m.Details["ami"] = "auth-gate 401 cc:7"
	}
	switch {
	case board != "":
		m.Score, m.Board = bmc.ScoreCertain, board
	case isASRockRoot(rp.root):
		m.Score = bmc.ScoreLikely
	case ami && foreignRoot(rp.root) != "":
		// Another OEM's MegaRAC (free evidence: no extra request).
		m.Score = scoreNonASRock
		m.Details["nonASRock"] = foreignRoot(rp.root)
	case ami && isAMIRoot(rp.root):
		m.Score = scoreAMIRedfish
	case ami:
		m.Score = scoreAMI
	default:
		// Not a MegaRAC /api. Surface a transport failure only when both
		// probes failed to reach the host at all.
		if rp.root == nil && aerr != nil && !answeredHTTP(rp.err) {
			return bmc.Match{}, aerr
		}
		return bmc.Match{}, nil
	}
	if m.Manufacturer == "" {
		m.Manufacturer = "AMI"
	}
	return m, nil
}

func answeredHTTP(err error) bool {
	var oe *bmc.OpError
	return errors.As(err, &oe) && oe.UpstreamStatus != 0
}

func (d *Driver) setFamily(host, fam string) {
	d.mu.Lock()
	d.families[host] = fam
	d.mu.Unlock()
}

func (d *Driver) family(ctx context.Context, t bmc.Target) string {
	d.mu.Lock()
	fam := d.families[t.Addr()]
	d.mu.Unlock()
	if fam != "" {
		return fam
	}
	rp := d.probeRoot(ctx, t)
	fam = familyFromRoot(rp.root)
	if fam == "" {
		// No Redfish answer: assume the modern generation (TLS auto
		// negotiation still falls back to legacy).
		fam = boards.FamilyModern
	} else {
		d.setFamily(t.Addr(), fam)
	}
	return fam
}

// DetectWithCreds implements bmc.CredDetector: it logs in to /api and reads
// asrr/model-name and asrr/fw-info (the "asrr" namespace is ASRock-only),
// so a successful answer is bmc.ScoreCertain with board and live firmware.
// The logged-in client is handed to the following Open (within
// Options.HandoffTTL) to avoid a second 1.5 s login.
//
// Before logging in it looks at the unauthenticated evidence: a ServiceRoot
// naming another vendor, or (for an AMI root that does not say ASRock) a
// $metadata declaring another OEM's namespaces, means "not ASRock" and
// returns the capped scoreNonASRock without sending credentials (no
// session slot, no failed-login counter). A login that finds no asrr
// namespace is capped the same way. Either way the generic Redfish driver
// outranks ASRock on another vendor's MegaRAC.
func (d *Driver) DetectWithCreds(ctx context.Context, t bmc.Target, c bmc.Creds) (bmc.Match, error) {
	rp := d.probeRoot(ctx, t)
	fam := familyFromRoot(rp.root)
	if fam != "" {
		d.setFamily(t.Addr(), fam)
	} else {
		fam = d.family(ctx, t)
	}
	if rp.root != nil {
		why := foreignRoot(rp.root)
		if why == "" && isAMIRoot(rp.root) && !isASRockRoot(rp.root) {
			why = d.foreignMetadata(ctx, t)
		}
		if why != "" {
			d.log(ctx).DebugContext(ctx, "asrock: not an ASRock BMC, skipping /api login", "bmc", t.Addr(), "evidence", why)
			return nonASRockMatch(rp.root, false, why), nil
		}
	}
	cl, err := d.newAMI(t, c, boards.ForFamily(fam))
	if err != nil {
		return bmc.Match{}, err
	}
	if err := cl.Login(ctx); err != nil {
		if errors.Is(err, bmc.ErrAuth) {
			// Wrong /api creds: fall back to the unauthenticated verdict so
			// Resolve can still pick a driver (Open then reports the 401).
			return d.Detect(ctx, t)
		}
		return bmc.Match{}, err
	}
	id, err := identify(ctx, cl)
	if err != nil {
		_ = cl.Logout(ctx)
		if errors.Is(err, bmc.ErrUnsupported) || errors.Is(err, bmc.ErrNotFound) {
			// A MegaRAC, but not ASRock firmware: never outrank the generic
			// Redfish fallback (or another vendor's driver) with it.
			m := nonASRockMatch(rp.root, true, "logged in; no asrr namespace")
			m.Details["ami"] = "logged in; no asrr namespace"
			return m, nil
		}
		return bmc.Match{}, err
	}
	p := boards.Lookup(id.Board, id.Firmware)
	d.setFamily(t.Addr(), familyForProfile(p))
	d.putHandoff(ctx, t, c, &handoff{client: cl, id: id})
	return bmc.Match{
		Score: bmc.ScoreCertain, Vendor: Name, Manufacturer: "ASRockRack", Board: id.Board, Firmware: id.Firmware,
		Backends: []string{bmc.BackendAMI},
		Details:  map[string]string{"profile": p.Key, "verified": boolStr(p.Verified), "redfish": p.Redfish.String()},
	}, nil
}

func familyForProfile(p boards.Profile) string {
	if p.Key == "x470d4u" {
		return boards.FamilyX470
	}
	return boards.FamilyModern
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// identity is what /api tells us about the board.
type identity struct {
	Board    string
	SysName  string
	Firmware string // live BMC firmware
	FWInfo   map[string]any
}

// identify reads asrr/model-name and asrr/fw-info (falling back to
// firmware-info for the version).
func identify(ctx context.Context, cl *amimegarac.Client) (identity, error) {
	var mn struct {
		MBName  amimegarac.FlexString `json:"mb_name"`
		SysName amimegarac.FlexString `json:"sys_name"`
	}
	if err := cl.Get(ctx, "asrr/model-name", &mn); err != nil {
		return identity{}, err
	}
	id := identity{Board: mn.MBName.Clean(), SysName: mn.SysName.Clean()}
	var fw map[string]any
	if err := cl.Get(ctx, "asrr/fw-info", &fw); err == nil {
		id.FWInfo = fw
		if v, ok := fw["BMC_fw_version"].(string); ok {
			id.Firmware = amimegarac.CleanString(v)
		}
	}
	if id.Firmware == "" {
		var fi struct {
			FWVer amimegarac.FlexString `json:"fw_ver"`
		}
		if err := cl.Get(ctx, "firmware-info", &fi); err == nil {
			id.Firmware = fi.FWVer.Clean()
		}
	}
	if id.Board == "" {
		return id, bmc.Errorf(bmc.ErrUnsupported, Pkg, "identify", "asrr/model-name has no mb_name")
	}
	return id, nil
}

// newAMI builds an amimegarac client for t with the login fields and deny
// rules of profile p. ReportSessionLost makes a backend restart surface as
// bmc.ErrSessionLost from every later call on the client.
func (d *Driver) newAMI(t bmc.Target, c bmc.Creds, p boards.Profile) (*amimegarac.Client, error) {
	tm := d.Transport()
	if p.TLS != transport.ProfileAuto {
		tm.SetProfile(t.Addr(), p.TLS)
	}
	cfg := d.opts.AMIConfig
	cfg.Endpoint = tm.Endpoint(t.Addr(), transport.LaneAMI)
	cfg.Target, cfg.Creds = t, c
	cfg.LoginExtra = p.LoginExtra
	cfg.ExtraDeny = append(append([]amimegarac.DenyRule(nil), cfg.ExtraDeny...), p.ExtraDeny...)
	cfg.ReportSessionLost = true
	if cfg.Now == nil {
		cfg.Now = d.opts.Now
	}
	return amimegarac.New(cfg)
}

// ---- DetectWithCreds -> Open handoff ----

type handoff struct {
	client *amimegarac.Client
	id     identity
	timer  *time.Timer
}

func handoffKey(t bmc.Target, c bmc.Creds) string {
	h := sha256.Sum256([]byte(c.Pass))
	return t.Addr() + "\x00" + c.User + "\x00" + hex.EncodeToString(h[:])
}

func (d *Driver) putHandoff(ctx context.Context, t bmc.Target, c bmc.Creds, h *handoff) {
	if d.opts.HandoffTTL < 0 {
		_ = h.client.Logout(ctx)
		return
	}
	k := handoffKey(t, c)
	bg := context.WithoutCancel(ctx)
	h.timer = time.AfterFunc(d.opts.HandoffTTL, func() {
		d.mu.Lock()
		cur, ok := d.handoffs[k]
		if ok && cur == h {
			delete(d.handoffs, k)
		}
		d.mu.Unlock()
		if ok && cur == h {
			lctx, cancel := context.WithTimeout(bg, 15*time.Second)
			defer cancel()
			_ = h.client.Logout(lctx)
		}
	})
	d.mu.Lock()
	old := d.handoffs[k]
	d.handoffs[k] = h
	d.mu.Unlock()
	if old != nil {
		old.timer.Stop()
		_ = old.client.Logout(ctx)
	}
}

func (d *Driver) takeHandoff(t bmc.Target, c bmc.Creds) *handoff {
	k := handoffKey(t, c)
	d.mu.Lock()
	h := d.handoffs[k]
	delete(d.handoffs, k)
	d.mu.Unlock()
	if h != nil {
		h.timer.Stop()
	}
	return h
}

// ---- Open ----

// Open implements bmc.Driver: logs in to /api, identifies the board and
// live firmware, selects the board profile and builds a Session that picks
// a backend per capability (Redfish -> AMI /api -> IPMI). It works without
// a previous Detect.
func (d *Driver) Open(ctx context.Context, t bmc.Target, c bmc.Creds) (bmc.Session, error) {
	var (
		cl *amimegarac.Client
		id identity
	)
	if h := d.takeHandoff(t, c); h != nil {
		cl, id = h.client, h.id
	} else {
		var err error
		cl, err = d.newAMI(t, c, boards.ForFamily(d.family(ctx, t)))
		if err != nil {
			return nil, err
		}
	}
	return d.openSession(ctx, t, c, cl, id)
}
