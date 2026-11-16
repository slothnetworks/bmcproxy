package redfish

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// ---- ServiceRoot and discovery ----

// ServiceRoot GETs (unauthenticated) and caches the ServiceRoot.
func (c *Client) ServiceRoot(ctx context.Context) (*ServiceRoot, error) {
	c.mu.Lock()
	if c.root != nil {
		r := c.root
		c.mu.Unlock()
		return r, nil
	}
	c.mu.Unlock()
	var root ServiceRoot
	p := c.opts.Quirks.LogicalPath(PathServiceRoot)
	if p == "" {
		p = DefaultServiceRoot
	}
	resp, err := c.Get(ctx, p, &root, WithoutAuth(), WithOp("redfish.ServiceRoot"))
	if err != nil {
		return nil, err
	}
	if root.RedfishVersion == "" && root.ODataType == "" {
		return nil, &bmc.OpError{Pkg: Pkg, Op: "GET " + p, Backend: bmc.BackendRedfish, UpstreamStatus: resp.Status,
			Detail: "not a Redfish ServiceRoot", Err: bmc.ErrUnsupported}
	}
	c.mu.Lock()
	c.root, c.rootRaw = &root, append(json.RawMessage(nil), resp.Body...)
	c.mu.Unlock()
	return &root, nil
}

// ServiceRootRaw returns the cached raw ServiceRoot body (nil before the
// first ServiceRoot call).
func (c *Client) ServiceRootRaw() json.RawMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rootRaw
}

func (c *Client) rootLink(ctx context.Context, pick func(*ServiceRoot) string, def string) string {
	if root, err := c.ServiceRoot(ctx); err == nil {
		if p := pick(root); p != "" {
			return p
		}
	}
	return DefaultServiceRoot + def
}

// firstMember discovers (and caches under key) the first member of a
// collection, honouring a PathOverrides logical key.
func (c *Client) firstMember(ctx context.Context, key string, collFn func(context.Context) string) (string, error) {
	if p := c.opts.Quirks.LogicalPath(key); p != "" {
		return p, nil
	}
	c.mu.Lock()
	if p, ok := c.paths[key]; ok {
		c.mu.Unlock()
		return p, nil
	}
	c.mu.Unlock()
	coll := collFn(ctx)
	links, err := c.MemberLinks(ctx, coll)
	if err != nil {
		return "", err
	}
	if len(links) == 0 {
		return "", &bmc.OpError{Pkg: Pkg, Op: "discover " + key, Backend: bmc.BackendRedfish,
			Detail: coll + " is empty", Err: bmc.ErrNotFound}
	}
	c.SetPath(key, links[0])
	return links[0], nil
}

// SetPath pins a discovered logical path (system, manager, chassis, ...).
func (c *Client) SetPath(key, path string) {
	c.mu.Lock()
	c.paths[key] = path
	c.mu.Unlock()
}

// SystemsPath / ManagersPath / ChassisCollectionPath return the collection
// paths from the ServiceRoot.
func (c *Client) SystemsPath(ctx context.Context) string {
	return c.rootLink(ctx, func(r *ServiceRoot) string { return r.Systems.ID }, "Systems")
}

func (c *Client) ManagersPath(ctx context.Context) string {
	return c.rootLink(ctx, func(r *ServiceRoot) string { return r.Managers.ID }, "Managers")
}

func (c *Client) ChassisCollectionPath(ctx context.Context) string {
	return c.rootLink(ctx, func(r *ServiceRoot) string { return r.Chassis.ID }, "Chassis")
}

// SystemPath returns the first ComputerSystem (e.g. /redfish/v1/Systems/Self).
func (c *Client) SystemPath(ctx context.Context) (string, error) {
	return c.firstMember(ctx, PathSystem, c.SystemsPath)
}

// ManagerPath returns the first Manager.
func (c *Client) ManagerPath(ctx context.Context) (string, error) {
	return c.firstMember(ctx, PathManager, c.ManagersPath)
}

// ChassisPath returns the first Chassis.
func (c *Client) ChassisPath(ctx context.Context) (string, error) {
	return c.firstMember(ctx, PathChassis, c.ChassisCollectionPath)
}

// Systems lists the Systems collection member paths.
func (c *Client) Systems(ctx context.Context) ([]string, error) {
	return c.MemberLinks(ctx, c.SystemsPath(ctx))
}

// Managers lists the Managers collection member paths.
func (c *Client) Managers(ctx context.Context) ([]string, error) {
	return c.MemberLinks(ctx, c.ManagersPath(ctx))
}

// ChassisMembers lists the Chassis collection member paths.
func (c *Client) ChassisMembers(ctx context.Context) ([]string, error) {
	return c.MemberLinks(ctx, c.ChassisCollectionPath(ctx))
}

// ---- typed getters ----

func getAt[T any](ctx context.Context, c *Client, path string) (*T, error) {
	var v T
	if _, err := c.Get(ctx, path, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// GetSystem GETs a ComputerSystem ("" = first system).
func (c *Client) GetSystem(ctx context.Context, path string) (*ComputerSystem, error) {
	if path == "" {
		p, err := c.SystemPath(ctx)
		if err != nil {
			return nil, err
		}
		path = p
	}
	return getAt[ComputerSystem](ctx, c, path)
}

// GetManager GETs a Manager ("" = first manager).
func (c *Client) GetManager(ctx context.Context, path string) (*Manager, error) {
	if path == "" {
		p, err := c.ManagerPath(ctx)
		if err != nil {
			return nil, err
		}
		path = p
	}
	return getAt[Manager](ctx, c, path)
}

// GetChassis GETs a Chassis ("" = first chassis).
func (c *Client) GetChassis(ctx context.Context, path string) (*Chassis, error) {
	if path == "" {
		p, err := c.ChassisPath(ctx)
		if err != nil {
			return nil, err
		}
		path = p
	}
	return getAt[Chassis](ctx, c, path)
}

// Sensors returns every Sensor of a chassis' Sensors collection (paged).
func (c *Client) Sensors(ctx context.Context, chassis *Chassis) ([]Sensor, error) {
	p := chassis.Sensors.ID
	if p == "" {
		return nil, bmc.Unsupported(Pkg, "Sensors")
	}
	return Members[Sensor](ctx, c, p)
}

// Thermal GETs a chassis' legacy Thermal resource.
func (c *Client) Thermal(ctx context.Context, chassis *Chassis) (*Thermal, error) {
	if chassis.Thermal.ID == "" {
		return nil, bmc.Unsupported(Pkg, "Thermal")
	}
	return getAt[Thermal](ctx, c, chassis.Thermal.ID)
}

// Power GETs a chassis' legacy Power resource.
func (c *Client) Power(ctx context.Context, chassis *Chassis) (*Power, error) {
	if chassis.Power.ID == "" {
		return nil, bmc.Unsupported(Pkg, "Power")
	}
	return getAt[Power](ctx, c, chassis.Power.ID)
}

// LogServices lists the LogServices of a collection link.
func (c *Client) LogServices(ctx context.Context, coll string) ([]LogService, error) {
	if coll == "" {
		return nil, bmc.Unsupported(Pkg, "LogServices")
	}
	return Members[LogService](ctx, c, coll)
}

// LogEntries returns every entry of a log service (paged). Entries are
// fetched from the collection with $expand when advertised; plain AMI
// collections inline full LogEntry members already.
func (c *Client) LogEntries(ctx context.Context, ls *LogService) ([]LogEntry, error) {
	p := ls.Entries.ID
	if p == "" {
		p = strings.TrimRight(ls.ODataID, "/") + "/Entries"
	}
	return Members[LogEntry](ctx, c, p)
}

// Processors lists a system's processors.
func (c *Client) Processors(ctx context.Context, sys *ComputerSystem) ([]Processor, error) {
	if sys.Processors.ID == "" {
		return nil, bmc.Unsupported(Pkg, "Processors")
	}
	return Members[Processor](ctx, c, sys.Processors.ID)
}

// MemoryModules lists a system's DIMMs.
func (c *Client) MemoryModules(ctx context.Context, sys *ComputerSystem) ([]Memory, error) {
	if sys.Memory.ID == "" {
		return nil, bmc.Unsupported(Pkg, "Memory")
	}
	return Members[Memory](ctx, c, sys.Memory.ID)
}

// PCIeDevices returns a system's PCIe devices (ComputerSystem.PCIeDevices
// links), falling back to the chassis PCIeDevices collection.
func (c *Client) PCIeDevices(ctx context.Context, sys *ComputerSystem, chassis *Chassis) ([]PCIeDevice, error) {
	if sys != nil && len(sys.PCIeDevices) > 0 {
		paths := make([]string, 0, len(sys.PCIeDevices))
		for _, l := range sys.PCIeDevices {
			paths = append(paths, l.ID)
		}
		return GetAll[PCIeDevice](ctx, c, paths)
	}
	if chassis != nil && chassis.PCIeDevices.ID != "" {
		return Members[PCIeDevice](ctx, c, chassis.PCIeDevices.ID)
	}
	return nil, bmc.Unsupported(Pkg, "PCIeDevices")
}

// PCIeFunctions lists a device's functions (empty if none linked).
func (c *Client) PCIeFunctions(ctx context.Context, dev *PCIeDevice) ([]PCIeFunction, error) {
	if dev.PCIeFunctions.ID == "" {
		return nil, nil
	}
	return Members[PCIeFunction](ctx, c, dev.PCIeFunctions.ID)
}

// StorageDrives returns every Drive under a system's Storage subsystems.
func (c *Client) StorageDrives(ctx context.Context, sys *ComputerSystem) ([]Drive, error) {
	if sys.Storage.ID == "" {
		return nil, bmc.Unsupported(Pkg, "Storage")
	}
	storages, err := Members[Storage](ctx, c, sys.Storage.ID)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, s := range storages {
		for _, d := range s.Drives {
			paths = append(paths, d.ID)
		}
	}
	return GetAll[Drive](ctx, c, paths)
}

// SimpleStorages lists a system's SimpleStorage controllers.
func (c *Client) SimpleStorages(ctx context.Context, sys *ComputerSystem) ([]SimpleStorage, error) {
	if sys.SimpleStorage.ID == "" {
		return nil, bmc.Unsupported(Pkg, "SimpleStorage")
	}
	return Members[SimpleStorage](ctx, c, sys.SimpleStorage.ID)
}

// EthernetInterfaces lists the interfaces of a collection link (manager or
// system EthernetInterfaces).
func (c *Client) EthernetInterfaces(ctx context.Context, coll string) ([]EthernetInterface, error) {
	if coll == "" {
		return nil, bmc.Unsupported(Pkg, "EthernetInterfaces")
	}
	return Members[EthernetInterface](ctx, c, coll)
}

// AccountServicePath returns the AccountService path.
func (c *Client) AccountServicePath(ctx context.Context) string {
	if p := c.opts.Quirks.LogicalPath(PathAccountService); p != "" {
		return p
	}
	return c.rootLink(ctx, func(r *ServiceRoot) string { return r.AccountService.ID }, "AccountService")
}

// Accounts lists the manager accounts.
func (c *Client) Accounts(ctx context.Context) ([]ManagerAccount, error) {
	as, err := getAt[AccountService](ctx, c, c.AccountServicePath(ctx))
	if err != nil {
		return nil, err
	}
	p := as.Accounts.ID
	if p == "" {
		p = strings.TrimRight(as.ODataID, "/") + "/Accounts"
	}
	return Members[ManagerAccount](ctx, c, p)
}

// GetNetworkProtocol GETs a manager's NetworkProtocol.
func (c *Client) GetNetworkProtocol(ctx context.Context, mgr *Manager) (*NetworkProtocol, error) {
	if mgr.NetworkProtocol.ID == "" {
		return nil, bmc.Unsupported(Pkg, "NetworkProtocol")
	}
	return getAt[NetworkProtocol](ctx, c, mgr.NetworkProtocol.ID)
}

// UpdateServicePath returns the UpdateService path ("" if not linked).
func (c *Client) UpdateServicePath(ctx context.Context) string {
	if p := c.opts.Quirks.LogicalPath(PathUpdateService); p != "" {
		return p
	}
	if root, err := c.ServiceRoot(ctx); err == nil {
		return root.UpdateService.ID
	}
	return ""
}

// FirmwareInventory lists UpdateService/FirmwareInventory members.
func (c *Client) FirmwareInventory(ctx context.Context) ([]SoftwareInventory, error) {
	p := c.UpdateServicePath(ctx)
	if p == "" {
		return nil, bmc.Unsupported(Pkg, "FirmwareInventory")
	}
	us, err := getAt[UpdateService](ctx, c, p)
	if err != nil {
		return nil, err
	}
	if us.FirmwareInventory.ID == "" {
		return nil, bmc.Unsupported(Pkg, "FirmwareInventory")
	}
	return Members[SoftwareInventory](ctx, c, us.FirmwareInventory.ID)
}

// VirtualMediaSlots lists a VirtualMedia collection (manager or system).
func (c *Client) VirtualMediaSlots(ctx context.Context, coll string) ([]VirtualMedia, error) {
	if coll == "" {
		return nil, bmc.Unsupported(Pkg, "VirtualMedia")
	}
	return Members[VirtualMedia](ctx, c, coll)
}

// ---- actions ----

// ActionTarget returns the target of action name ("#ComputerSystem.Reset")
// from acts, or the conventional "<resource>/Actions/<Type.Action>".
func ActionTarget(resource string, acts Actions, name string) string {
	if a, ok := acts[name]; ok && a.Target != "" {
		return a.Target
	}
	return strings.TrimRight(resource, "/") + "/Actions/" + strings.TrimPrefix(name, "#")
}

// AllowableValues returns the allowable values of param for action name:
// inline @Redfish.AllowableValues, then the @Redfish.ActionInfo resource
// (cached), then Quirks.ResetTypes for ResetType. nil means unknown.
func (c *Client) AllowableValues(ctx context.Context, acts Actions, name, param string) []string {
	a, ok := acts[name]
	if ok {
		if v := a.Allowable[param]; len(v) > 0 {
			return v
		}
		if a.ActionInfo != "" {
			c.mu.Lock()
			ai, cached := c.actInfos[a.ActionInfo]
			c.mu.Unlock()
			if !cached {
				var info ActionInfo
				if _, err := c.Get(ctx, a.ActionInfo, &info); err == nil {
					ai = &info
				}
				c.mu.Lock()
				c.actInfos[a.ActionInfo] = ai
				c.mu.Unlock()
			}
			if v := ai.Allowable(param); len(v) > 0 {
				return v
			}
		}
	}
	if param == "ResetType" {
		if v := c.opts.Quirks.ResetTypes[name]; len(v) > 0 {
			return v
		}
	}
	return nil
}

// PostAction POSTs an action. If the service answers 202 with a task and
// Options.WaitActionTasks is set, the task is awaited.
func (c *Client) PostAction(ctx context.Context, target string, body any) (*Response, error) {
	resp, err := c.Post(ctx, target, body, WithOp("redfish.Action"))
	if err != nil {
		return resp, err
	}
	if c.opts.WaitActionTasks && resp.Status == http.StatusAccepted {
		if ref := TaskRef(resp); ref != "" {
			res, werr := c.WaitTask(ctx, ref, WaitOptions{})
			if werr != nil {
				return resp, werr
			}
			if res.Final != nil {
				return res.Final, nil
			}
		}
	}
	return resp, nil
}

// ResetSystem invokes ComputerSystem.Reset with resetType.
func (c *Client) ResetSystem(ctx context.Context, sys *ComputerSystem, resetType string) (*Response, error) {
	return c.PostAction(ctx, ActionTarget(sys.ODataID, sys.Actions, "#ComputerSystem.Reset"), map[string]string{"ResetType": resetType})
}

// ResetManager invokes Manager.Reset with resetType.
func (c *Client) ResetManager(ctx context.Context, mgr *Manager, resetType string) (*Response, error) {
	return c.PostAction(ctx, ActionTarget(mgr.ODataID, mgr.Actions, "#Manager.Reset"), map[string]string{"ResetType": resetType})
}

// ClearLog invokes LogService.ClearLog.
func (c *Client) ClearLog(ctx context.Context, ls *LogService) (*Response, error) {
	return c.PostAction(ctx, ActionTarget(ls.ODataID, ls.Actions, "#LogService.ClearLog"), map[string]any{})
}

// SetIndicatorLED PATCHes IndicatorLED ("Lit", "Blinking", "Off") on path.
func (c *Client) SetIndicatorLED(ctx context.Context, path, value string) (*Response, error) {
	return c.Patch(ctx, path, map[string]string{"IndicatorLED": value})
}

// SetLocationIndicator PATCHes LocationIndicatorActive on path.
func (c *Client) SetLocationIndicator(ctx context.Context, path string, active bool) (*Response, error) {
	return c.Patch(ctx, path, map[string]bool{"LocationIndicatorActive": active})
}

// BootOverride is the PATCH body for ComputerSystem.Boot (empty fields are
// omitted).
type BootOverride struct {
	Target  string `json:"BootSourceOverrideTarget,omitempty"`
	Enabled string `json:"BootSourceOverrideEnabled,omitempty"`
	Mode    string `json:"BootSourceOverrideMode,omitempty"`
}

// SetBootOverride PATCHes {"Boot": {...}} on a system.
func (c *Client) SetBootOverride(ctx context.Context, sysPath string, b BootOverride) (*Response, error) {
	return c.Patch(ctx, sysPath, map[string]any{"Boot": b})
}

// SetPowerRestorePolicy PATCHes PowerRestorePolicy ("AlwaysOn",
// "AlwaysOff", "LastState") on a system.
func (c *Client) SetPowerRestorePolicy(ctx context.Context, sysPath, policy string) (*Response, error) {
	return c.Patch(ctx, sysPath, map[string]string{"PowerRestorePolicy": policy})
}

// FormatDateTime renders t in the BMC's DateTimeLocalOffset ("-05:00").
func FormatDateTime(t time.Time, localOffset string) string {
	if loc := ParseOffset(localOffset); loc != nil {
		t = t.In(loc)
	} else {
		t = t.UTC()
	}
	return t.Format("2006-01-02T15:04:05-07:00")
}

// ParseOffset parses "+HH:MM"/"-HH:MM"/"Z" into a fixed zone (nil if bad).
func ParseOffset(s string) *time.Location {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if s == "Z" || s == "+00:00" || s == "-00:00" {
		return time.UTC
	}
	t, err := time.Parse("-07:00", s)
	if err != nil {
		return nil
	}
	_, off := t.Zone()
	return time.FixedZone(s, off)
}

// OffsetMinutes parses a DateTimeLocalOffset into minutes east of UTC.
func OffsetMinutes(s string) int {
	loc := ParseOffset(s)
	if loc == nil {
		return 0
	}
	_, off := time.Now().In(loc).Zone()
	return off / 60
}

// SetDateTime PATCHes Manager.DateTime (keeping the BMC's local offset).
func (c *Client) SetDateTime(ctx context.Context, mgr *Manager, t time.Time) (*Response, error) {
	resp, err := c.Patch(ctx, mgr.ODataID, map[string]string{"DateTime": FormatDateTime(t, mgr.DateTimeLocalOffset)})
	if err != nil {
		return resp, err
	}
	return c.WaitAccepted(ctx, resp, DefaultWriteTaskTimeout)
}

// DefaultWriteTaskTimeout bounds how long a synchronous northbound write
// waits for the BMC task behind a 202 PATCH (the client's TaskTimeout is
// sized for firmware updates and is far too long for an HTTP request).
const DefaultWriteTaskTimeout = 60 * time.Second

// WaitAccepted completes a write that the service answered with 202
// Accepted. AMI MegaRAC firmware (B650D4U and others) applies PATCHes such
// as NetworkProtocol asynchronously: the PATCH only queues the change and
// starts a task (e.g. NetworkProtocolTask), so a 202 means "queued", not
// "applied". When the response references a task it is polled until it
// finishes (bounded by timeout, and by the client's TaskTimeout if that is
// shorter); a failed task returns an ErrUpstream OpError carrying the task's
// messages. Any other response, or a 202 without a task reference, is
// returned unchanged; callers should then verify the result by reading it
// back.
func (c *Client) WaitAccepted(ctx context.Context, resp *Response, timeout time.Duration) (*Response, error) {
	if resp == nil || resp.Status != http.StatusAccepted {
		return resp, nil
	}
	ref := TaskRef(resp)
	if ref == "" {
		return resp, nil
	}
	if timeout <= 0 || (c.opts.TaskTimeout > 0 && c.opts.TaskTimeout < timeout) {
		timeout = c.opts.TaskTimeout
	}
	res, err := c.WaitTask(ctx, ref, WaitOptions{Timeout: timeout})
	if err != nil {
		return resp, err
	}
	if res.Final != nil {
		return res.Final, nil
	}
	return resp, nil
}

// NTPUpdate is the PATCH body for NetworkProtocol.NTP.
type NTPUpdate struct {
	ProtocolEnabled *bool    `json:"ProtocolEnabled,omitempty"`
	NTPServers      []string `json:"NTPServers,omitempty"`
}

// SetNTP PATCHes {"NTP": {...}} on a NetworkProtocol resource and, when the
// service answers 202 with a task (AMI MegaRAC queues NetworkProtocol
// changes into a NetworkProtocolTask), waits for that task (see
// WaitAccepted). A 2xx return still does not prove the change took effect;
// read NetworkProtocol back to confirm.
func (c *Client) SetNTP(ctx context.Context, netProtoPath string, u NTPUpdate) (*Response, error) {
	resp, err := c.Patch(ctx, netProtoPath, map[string]any{"NTP": u})
	if err != nil {
		return resp, err
	}
	return c.WaitAccepted(ctx, resp, DefaultWriteTaskTimeout)
}

// InsertMediaRequest is the VirtualMedia.InsertMedia body.
type InsertMediaRequest struct {
	Image                string `json:"Image"`
	Inserted             *bool  `json:"Inserted,omitempty"`
	WriteProtected       *bool  `json:"WriteProtected,omitempty"`
	TransferProtocolType string `json:"TransferProtocolType,omitempty"`
	UserName             string `json:"UserName,omitempty"`
	Password             string `json:"Password,omitempty"`
}

// InsertMedia invokes VirtualMedia.InsertMedia, falling back to a PATCH of
// Image/Inserted when the slot has no action.
func (c *Client) InsertMedia(ctx context.Context, vm *VirtualMedia, r InsertMediaRequest) (*Response, error) {
	if _, ok := vm.Actions["#VirtualMedia.InsertMedia"]; ok || len(vm.Actions) > 0 {
		return c.PostAction(ctx, ActionTarget(vm.ODataID, vm.Actions, "#VirtualMedia.InsertMedia"), r)
	}
	t := true
	body := map[string]any{"Image": r.Image, "Inserted": t}
	if r.WriteProtected != nil {
		body["WriteProtected"] = *r.WriteProtected
	}
	return c.Patch(ctx, vm.ODataID, body)
}

// EjectMedia invokes VirtualMedia.EjectMedia, falling back to a PATCH.
func (c *Client) EjectMedia(ctx context.Context, vm *VirtualMedia) (*Response, error) {
	if len(vm.Actions) > 0 {
		return c.PostAction(ctx, ActionTarget(vm.ODataID, vm.Actions, "#VirtualMedia.EjectMedia"), map[string]any{})
	}
	return c.Patch(ctx, vm.ODataID, map[string]any{"Image": nil, "Inserted": false})
}
