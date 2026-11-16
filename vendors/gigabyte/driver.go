// Package gigabyte is the Gigabyte / Giga Computing bmc.Driver
// ("gigabyte"). The evidence (docs/gigabyte/mc13-le0-live-probe.md,
// docs/firmware/mc13-le0.md) shows an AMI MegaRAC SP-X 13.x stack whose
// Redfish service accepts the web admin with Basic auth, so the driver is
// Redfish-first: it embeds the generic Redfish session
// (vendors/generic/redfish) and overrides only what Gigabyte does
// differently (SEL timestamps, the baseboard serial, collection paging via
// a board-profile quirk). The legacy AMI /api (pkg/protocol/amimegarac) is
// logged in lazily, only for the vendor routes that need it (fan-profile
// mode, raw/api).
//
// Detection is generic to Gigabyte boards: an unauthenticated ServiceRoot
// with Gigabyte/GBT/Gct markers, or a $metadata document declaring the
// GBT*/Gbt* OEM namespaces, scores above ASRock's MegaRAC guess; with
// credentials the ComputerSystem Manufacturer ("Giga Computing",
// "GIGABYTE") makes it certain and yields board + live firmware, which
// select a profile in vendors/gigabyte/boards.
//
// Register with a blank import of this package (cmd/bmcproxy/drivers.go,
// cmd/bmcprobe/drivers.go). The driver uses transport.Default() unless New
// is given a Manager.
package gigabyte

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	"github.com/slothnetworks/bmcproxy/vendors/gigabyte/boards"
)

// Name is the registry name, X-BMC-Vendor value and vendor route prefix.
const Name = "gigabyte"

// Pkg is the OpError.Pkg value for this driver.
const Pkg = "driver.gigabyte"

// SubDriver is the obs subsystem of this driver.
const SubDriver = "driver.gigabyte"

// Priority is above the vendor default (0): Detect is cheap and
// unauthenticated, and running before the ASRock driver keeps ASRock's
// credentialed /api login probe off Gigabyte BMCs (it would otherwise log in
// and claim any MegaRAC at score 30).
const Priority = 10

// Detection scores.
const (
	// scoreUnauth: Gigabyte markers in the unauthenticated ServiceRoot or
	// $metadata. Above ASRock's MegaRAC guesses (30/35) and ScoreLikely.
	scoreUnauth = bmc.ScoreLikely + 10
)

// Options configure a Driver. The zero value is production-ready.
type Options struct {
	// Transport is the southbound manager (nil = transport.Default()).
	Transport *transport.Manager
	// RedfishOptions are copied into every Redfish client (Creds, Auth and
	// Quirks are set by the driver; Quirks from the board profile are
	// merged over these).
	RedfishOptions rf.Options
	// AMIConfig is applied to the lazy /api client (Endpoint, Target, Creds,
	// ExtraDeny are set by the driver).
	AMIConfig amimegarac.Config
	// Now is the clock (tests). Default time.Now.
	Now func() time.Time
	// NoMetadataProbe skips the unauthenticated $metadata GET in Detect.
	NoMetadataProbe bool
}

// Driver is the Gigabyte driver. Safe for concurrent use.
type Driver struct {
	opts Options

	mu    sync.Mutex
	roots map[string]rootInfo // host -> last unauthenticated verdict (non-secret)
}

type rootInfo struct {
	ami bool
}

// Default is the registered instance.
var Default = New(Options{})

func init() { bmc.Register(Default) }

// New returns a driver with o.
func New(o Options) *Driver {
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Driver{opts: o, roots: map[string]rootInfo{}}
}

// Name implements bmc.Driver.
func (d *Driver) Name() string { return Name }

// Priority implements bmc.Prioritized.
func (d *Driver) Priority() int { return Priority }

// Transport returns the driver's transport manager.
func (d *Driver) Transport() *transport.Manager {
	if d.opts.Transport != nil {
		return d.opts.Transport
	}
	return transport.Default()
}

func (d *Driver) log(ctx context.Context) interface {
	DebugContext(context.Context, string, ...any)
	WarnContext(context.Context, string, ...any)
} {
	return obs.FromContext(ctx, SubDriver)
}

func (d *Driver) client(t bmc.Target, c bmc.Creds, auth rf.AuthMode, q rf.Quirks) (*rf.Client, error) {
	o := d.opts.RedfishOptions
	o.Creds, o.Auth = c, auth
	o.Quirks = mergeQuirks(o.Quirks, q)
	return rf.New(d.Transport().Endpoint(t.Addr(), transport.LaneRedfish), o)
}

// mergeQuirks overlays the profile quirks p on base (non-zero fields win).
func mergeQuirks(base, p rf.Quirks) rf.Quirks {
	if p.SkipPageSize > 0 {
		base.SkipPageSize = p.SkipPageSize
	}
	base.NoExpand = base.NoExpand || p.NoExpand
	base.StripQuery = base.StripQuery || p.StripQuery
	base.NAAsNull = base.NAAsNull || p.NAAsNull
	base.NullStrings = append(base.NullStrings, p.NullStrings...)
	base.RequireIfMatch = base.RequireIfMatch || p.RequireIfMatch
	base.NoIfMatch = base.NoIfMatch || p.NoIfMatch
	if p.QueryCaps != nil {
		base.QueryCaps = p.QueryCaps
	}
	if len(p.PathOverrides) > 0 {
		m := map[string]string{}
		for k, v := range base.PathOverrides {
			m[k] = v
		}
		for k, v := range p.PathOverrides {
			m[k] = v
		}
		base.PathOverrides = m
	}
	if len(p.ResetTypes) > 0 {
		base.ResetTypes = p.ResetTypes
	}
	return base
}

// ---- detection ----

// gigabyteWords are lower-case markers of Gigabyte firmware.
var gigabyteWords = []string{"gigabyte", "giga computing", "giga-byte", "gigacomputing"}

// IsGigabyteManufacturer reports whether a Manufacturer/Vendor string names
// Gigabyte or its server arm Giga Computing.
func IsGigabyteManufacturer(s string) bool {
	l := strings.ToLower(strings.TrimSpace(s))
	for _, w := range gigabyteWords {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

// rootMarkers returns why root looks like Gigabyte firmware ("" = no).
func rootMarkers(root *rf.ServiceRoot) string {
	if root == nil {
		return ""
	}
	if IsGigabyteManufacturer(root.Vendor) || IsGigabyteManufacturer(root.Product) {
		return "ServiceRoot Vendor/Product"
	}
	for k := range root.Oem {
		switch strings.ToLower(k) {
		case "gbt", "gigabyte", "gct":
			return "ServiceRoot Oem." + k
		}
	}
	return ""
}

func isAMIRoot(root *rf.ServiceRoot) bool {
	if root == nil {
		return false
	}
	if _, ok := root.Oem["Ami"]; ok {
		return true
	}
	return strings.Contains(strings.ToLower(root.Vendor+" "+root.Product), "ami")
}

// metadataMarkers are the Gigabyte OEM CSDL namespaces (MC13-LE0 13.04.15
// declares GBTChassisOemProperty, GBTFanprofile*, GBTSystemsOemProperty,
// GBTThermal, GbtGPUManager*, GbtMemoryOemProperty, GbtPowerLimit, ...).
var metadataMarkers = [][]byte{[]byte(`Namespace="GBT`), []byte(`Namespace="Gbt`), []byte(`Namespace="Gct`), []byte(`Namespace="GCT`)}

// metadataHasGBT GETs the unauthenticated $metadata (about 115 KB on the
// MC13-LE0) and looks for the Gigabyte OEM namespaces.
func metadataHasGBT(ctx context.Context, c *rf.Client) bool {
	resp, err := c.Do(transport.WithOp(ctx, "gigabyte.Detect.metadata"), http.MethodGet, "/redfish/v1/$metadata", nil, rf.WithoutAuth())
	if err != nil || resp.Status != http.StatusOK {
		return false
	}
	for _, m := range metadataMarkers {
		if bytes.Contains(resp.Body, m) {
			return true
		}
	}
	return false
}

// Detect implements bmc.Driver: an unauthenticated ServiceRoot GET, then
// (when the root carries no Gigabyte marker and the probe is enabled) the
// $metadata. It never sends credentials and returns a zero Match for
// non-Gigabyte services (ASRock, generic AMI, other vendors).
func (d *Driver) Detect(ctx context.Context, t bmc.Target) (bmc.Match, error) {
	c, err := d.client(t, bmc.Creds{}, rf.AuthNone, rf.Quirks{})
	if err != nil {
		return bmc.Match{}, err
	}
	root, err := c.ServiceRoot(transport.WithOp(ctx, "gigabyte.Detect.ServiceRoot"))
	if err != nil {
		var oe *bmc.OpError
		if errors.As(err, &oe) && oe.UpstreamStatus != 0 || errors.Is(err, bmc.ErrUnsupported) {
			return bmc.Match{}, nil // answered HTTP, not Redfish
		}
		return bmc.Match{}, err
	}
	ami := isAMIRoot(root)
	d.mu.Lock()
	d.roots[t.Addr()] = rootInfo{ami: ami}
	d.mu.Unlock()
	why := rootMarkers(root)
	if why == "" && !d.opts.NoMetadataProbe && metadataHasGBT(ctx, c) {
		why = "$metadata GBT OEM namespaces"
	}
	if why == "" {
		return bmc.Match{}, nil
	}
	m := bmc.Match{Score: scoreUnauth, Vendor: Name, Manufacturer: "Gigabyte", Backends: []string{bmc.BackendRedfish},
		Details: map[string]string{"signal": why, "redfishVersion": root.RedfishVersion, "serviceRootType": root.ODataType}}
	if root.Product != "" {
		m.Details["product"] = root.Product
	}
	if ami {
		m.Details["stack"] = "AMI MegaRAC"
		m.Backends = append(m.Backends, bmc.BackendAMI)
	}
	return m, nil
}

// identity is what Redfish says about the board.
type identity struct {
	Manufacturer, Board, Firmware, BIOS string
	AMI                                 bool
}

// identify reads the ComputerSystem and Manager with an authenticated
// client.
func identify(ctx context.Context, c *rf.Client) (identity, error) {
	var id identity
	root, err := c.ServiceRoot(ctx)
	if err != nil {
		return id, err
	}
	id.AMI = isAMIRoot(root)
	sp, err := c.SystemPath(ctx)
	if err != nil {
		return id, err
	}
	sys, err := c.GetSystem(ctx, sp)
	if err != nil {
		return id, err
	}
	id.Manufacturer, id.Board, id.BIOS = strings.TrimSpace(sys.Manufacturer), strings.TrimSpace(sys.Model), sys.BiosVersion
	mp := ""
	if len(sys.Links.ManagedBy) > 0 {
		mp = sys.Links.ManagedBy[0].ID
	} else if p, err := c.ManagerPath(ctx); err == nil {
		mp = p
	}
	if mp != "" {
		if m, err := c.GetManager(ctx, mp); err == nil {
			id.Firmware = strings.TrimSpace(m.FirmwareVersion)
		}
	}
	return id, nil
}

// DetectWithCreds implements bmc.CredDetector. Credentials are only sent
// when the unauthenticated Detect already saw Gigabyte markers, so ASRock
// and other BMCs never get a Basic-auth attempt from this driver (no
// failed-login counters). A Gigabyte Manufacturer makes the match certain,
// with board, live firmware and the selected profile.
func (d *Driver) DetectWithCreds(ctx context.Context, t bmc.Target, cr bmc.Creds) (bmc.Match, error) {
	m, err := d.Detect(ctx, t)
	if err != nil || m.Score == 0 {
		return m, err
	}
	c, err := d.client(t, cr, rf.AuthBasic, rf.Quirks{})
	if err != nil {
		return m, nil
	}
	id, err := identify(transport.WithOp(ctx, "gigabyte.DetectWithCreds"), c)
	if err != nil {
		if errors.Is(err, bmc.ErrAuth) {
			m.Details["redfishAuth"] = "rejected"
			return m, nil // Open will surface the 401
		}
		return m, nil
	}
	if !IsGigabyteManufacturer(id.Manufacturer) {
		m.Details["manufacturer"] = id.Manufacturer
		return m, nil
	}
	p := boards.Lookup(id.Board, id.Firmware)
	m.Score, m.Manufacturer, m.Board, m.Firmware = bmc.ScoreCertain, id.Manufacturer, id.Board, id.Firmware
	m.Details["profile"] = p.Key
	m.Details["verified"] = boolStr(p.Verified)
	return m, nil
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ---- Open ----

// Open implements bmc.Driver: authenticates over Redfish (Basic: the
// MC13-LE0 SessionTimeout is 30 s), identifies the board and live firmware,
// selects the profile and builds the Session with the profile's quirks. It
// works without a previous Detect.
func (d *Driver) Open(ctx context.Context, t bmc.Target, c bmc.Creds) (bmc.Session, error) {
	boot, err := d.client(t, c, rf.AuthBasic, rf.Quirks{})
	if err != nil {
		return nil, err
	}
	if err := boot.Login(ctx); err != nil {
		return nil, err
	}
	id, err := identify(ctx, boot)
	if err != nil {
		return nil, err
	}
	p := boards.Lookup(id.Board, id.Firmware)
	if p.Key == "unknown" {
		p = boards.Unknown(id.Board, id.Firmware, id.AMI)
	}
	return d.openSession(ctx, t, c, p, id)
}
