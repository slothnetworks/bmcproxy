package asrock

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/boards"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/oem"
)

// ---- FRU ----

type amiFRUEntry struct {
	Device struct {
		ID   amimegarac.NullableInt `json:"id"`
		Name amimegarac.FlexString  `json:"name"`
	} `json:"device"`
	Chassis struct {
		Type       amimegarac.FlexString `json:"type"`
		PartNumber amimegarac.FlexString `json:"part_number"`
		Serial     amimegarac.FlexString `json:"serial_number"`
		Custom     amimegarac.FlexString `json:"custom_fields"`
	} `json:"chassis"`
	Board struct {
		Date         amimegarac.FlexString `json:"date"`
		Manufacturer amimegarac.FlexString `json:"manufacturer"`
		Product      amimegarac.FlexString `json:"product_name"`
		Serial       amimegarac.FlexString `json:"serial_number"`
		PartNumber   amimegarac.FlexString `json:"part_number"`
		FRUFileID    amimegarac.FlexString `json:"fru_file_id"`
		Custom       amimegarac.FlexString `json:"custom_fields"`
	} `json:"board"`
	Product struct {
		Manufacturer amimegarac.FlexString `json:"manufacturer"`
		Name         amimegarac.FlexString `json:"product_name"`
		PartNumber   amimegarac.FlexString `json:"part_number"`
		Version      amimegarac.FlexString `json:"product_version"`
		Serial       amimegarac.FlexString `json:"serial_number"`
		AssetTag     amimegarac.FlexString `json:"asset_tag"`
		FRUFileID    amimegarac.FlexString `json:"fru_file_id"`
		Custom       amimegarac.FlexString `json:"custom_fields"`
	} `json:"product"`
}

func extra(s amimegarac.FlexString) []string {
	if v := s.Clean(); v != "" {
		return []string{v}
	}
	return nil
}

// parseFRUDate parses "Thu Jan 13 06:21:00 2022" (the literal "\n" suffix
// is removed by CleanString).
func parseFRUDate(s string) *time.Time {
	s = strings.Join(strings.Fields(s), " ")
	for _, layout := range []string{"Mon Jan 2 15:04:05 2006", time.ANSIC} {
		if t, err := time.Parse(layout, s); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

func (s *Session) amiFRU(ctx context.Context) ([]bmc.FRU, error) {
	var raw []amiFRUEntry
	if err := s.api.Get(ctx, "fru", &raw); err != nil {
		return nil, err
	}
	out := make([]bmc.FRU, 0, len(raw))
	for _, f := range raw {
		out = append(out, bmc.FRU{
			ID: int(f.Device.ID.V), Name: f.Device.Name.Clean(),
			Chassis: bmc.FRUChassis{Type: f.Chassis.Type.Clean(), PartNumber: f.Chassis.PartNumber.Clean(),
				Serial: f.Chassis.Serial.Clean(), Extra: extra(f.Chassis.Custom)},
			Board: bmc.FRUBoard{MfgDate: parseFRUDate(f.Board.Date.Clean()), Manufacturer: f.Board.Manufacturer.Clean(),
				Product: f.Board.Product.Clean(), Serial: f.Board.Serial.Clean(), PartNumber: f.Board.PartNumber.Clean(),
				FRUFileID: f.Board.FRUFileID.Clean(), Extra: extra(f.Board.Custom)},
			Product: bmc.FRUProduct{Manufacturer: f.Product.Manufacturer.Clean(), Name: f.Product.Name.Clean(),
				PartNumber: f.Product.PartNumber.Clean(), Version: f.Product.Version.Clean(), Serial: f.Product.Serial.Clean(),
				AssetTag: f.Product.AssetTag.Clean(), FRUFileID: f.Product.FRUFileID.Clean(), Extra: extra(f.Product.Custom)},
		})
	}
	return out, nil
}

// FRU implements bmc.FRUReader. AMI /api/fru is preferred on every board
// (it carries the real FRU areas; the Redfish chassis does not).
func (s *Session) FRU(ctx context.Context) ([]bmc.FRU, error) {
	switch s.backend(bmc.CapFRU) {
	case bmc.BackendAMI:
		return s.amiFRU(ctx)
	case bmc.BackendRedfish:
		return s.Session.FRU(ctx)
	}
	return nil, s.unsupported("FRU")
}

// ---- inventory ----

// CodeNoHostInventory is the /api error code of system_inventory/* while
// the host is off or the BIOS has not pushed inventory ("Error in Getting
// the Device Count Information", HTTP 500).
const CodeNoHostInventory = 11272

func isNoInventory(err error) bool {
	var ae *amimegarac.APIError
	return errors.As(err, &ae) && ae.Code == CodeNoHostInventory
}

// inventoryUnavailable is the clean error for "the BMC has no host
// inventory right now" (404 not_found, with the reason in the detail).
func inventoryUnavailable(op string, cause error) error {
	d := "host inventory unavailable (host off or not yet pushed by the BIOS)"
	if cause != nil {
		d += ": " + cause.Error()
	}
	return &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, Detail: d, Err: bmc.ErrNotFound}
}

// hostInv GETs a host_inventory endpoint as a list of objects. 11272 maps
// to inventoryUnavailable.
func (s *Session) hostInv(ctx context.Context, op, path string) ([]map[string]any, error) {
	var raw []map[string]any
	if err := s.api.Get(ctx, path, &raw); err != nil {
		if isNoInventory(err) {
			return nil, inventoryUnavailable(op, err)
		}
		return nil, err
	}
	return raw, nil
}

// sysInv tries the X470 system_inventory endpoint first and falls back to
// host_inventory when it answers 11272 (or does not exist). unavailable is
// non-nil when system_inventory reported 11272: callers return it instead
// of an empty list when the fallback has nothing either, so "host off" is a
// clean 404 rather than a misleading empty inventory.
func (s *Session) sysInv(ctx context.Context, op, sysPath, hostPath string) (rows []map[string]any, fromSystem bool, unavailable, err error) {
	if s.prof.Inventory == boards.InventorySystem && sysPath != "" {
		var raw []map[string]any
		err := s.api.Get(ctx, sysPath, &raw)
		switch {
		case err == nil:
			return raw, true, nil, nil
		case isNoInventory(err):
			unavailable = inventoryUnavailable(op, err)
			s.d.log(ctx).DebugContext(ctx, "system_inventory: no host inventory (11272); trying host_inventory", "path", sysPath)
		case errors.Is(err, bmc.ErrUnsupported), errors.Is(err, bmc.ErrNotFound):
			s.d.log(ctx).DebugContext(ctx, "system_inventory absent; trying host_inventory", "path", sysPath, "err", err)
		default:
			return nil, false, nil, err
		}
	}
	rows, err = s.hostInv(ctx, op, hostPath)
	return rows, false, unavailable, err
}

func intOf(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := oem.Int(m, k); ok {
			return v
		}
	}
	return 0
}

func strOf(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := oem.Str(m, k); v != "" && !strings.EqualFold(v, "Not Available") && !strings.EqualFold(v, "Unknown") {
			return v
		}
	}
	return ""
}

func healthFromState(m map[string]any) bmc.Health {
	switch strings.ToLower(oem.Str(m, "State")) {
	case "enabled":
		return bmc.HealthOK
	case "":
		return ""
	}
	return bmc.HealthUnknown
}

// CPUs implements bmc.Inventory. AMI: host_inventory/host_interface_processor_info
// (X470: system_inventory/cpu_info first).
func (s *Session) CPUs(ctx context.Context) ([]bmc.CPU, error) {
	switch s.backend(bmc.CapInventory) {
	case bmc.BackendRedfish:
		return s.Session.CPUs(ctx)
	case bmc.BackendAMI:
		rows, sys, unavail, err := s.sysInv(ctx, "CPUs", "system_inventory/cpu_info", "host_inventory/host_interface_processor_info")
		if err != nil {
			return nil, err
		}
		out := []bmc.CPU{}
		for i, m := range rows {
			c := bmc.CPU{ID: strOf(m, "Id", "id", "Name"), Socket: strOf(m, "Socket", "socket_num"),
				Manufacturer: strOf(m, "Manufacturer"), Cores: intOf(m, "TotalCores"), Threads: intOf(m, "TotalThreads"),
				MaxSpeedMHz: intOf(m, "MaxSpeedMHz"), Present: true, Health: healthFromState(m)}
			model := strOf(m, "Model", "EffectiveFamily")
			if j := strings.Index(model, ","); j > 0 {
				model = strings.TrimSpace(model[:j])
			}
			c.Model = strings.Join(strings.Fields(model), " ")
			if c.ID == "" {
				c.ID = strconv.Itoa(i)
			}
			if sys {
				c.Oem = m
			}
			out = append(out, c)
		}
		if len(out) == 0 && unavail != nil {
			return nil, unavail
		}
		return out, nil
	}
	return nil, s.unsupported("CPUs")
}

// Memory implements bmc.Inventory (host_interface_memory_info; X470
// system_inventory/dimm_info first).
func (s *Session) Memory(ctx context.Context) ([]bmc.MemoryModule, error) {
	switch s.backend(bmc.CapInventory) {
	case bmc.BackendRedfish:
		return s.Session.Memory(ctx)
	case bmc.BackendAMI:
		rows, sys, unavail, err := s.sysInv(ctx, "Memory", "system_inventory/dimm_info", "host_inventory/host_interface_memory_info")
		if err != nil {
			return nil, err
		}
		out := []bmc.MemoryModule{}
		for i, m := range rows {
			mm := bmc.MemoryModule{ID: strOf(m, "Id", "id", "Name"), Slot: strOf(m, "DeviceLocator"),
				Manufacturer: strOf(m, "Manufacturer"), PartNumber: strings.TrimSpace(strOf(m, "PartNumber")),
				Serial: strOf(m, "SerialNumber"), SizeMiB: intOf(m, "CapacityMiB"), SpeedMTs: intOf(m, "OperatingSpeedMhz"),
				Type: strOf(m, "MemoryDeviceType"), Health: healthFromState(m)}
			mm.Present = mm.SizeMiB > 0 || strings.EqualFold(oem.Str(m, "State"), "Enabled")
			if mm.ID == "" {
				mm.ID = strconv.Itoa(i)
			}
			if sys {
				mm.Oem = m
			}
			out = append(out, mm)
		}
		if len(out) == 0 && unavail != nil {
			return nil, unavail
		}
		return out, nil
	}
	return nil, s.unsupported("Memory")
}

// PCIeDevices implements bmc.Inventory (host_interface_pcie_device_function_info
// [{PCIE_Devices:[...], PCIE_Function:[...]}]; X470 system_inventory/pci_info
// first).
func (s *Session) PCIeDevices(ctx context.Context) ([]bmc.PCIeDevice, error) {
	switch s.backend(bmc.CapInventory) {
	case bmc.BackendRedfish:
		return s.Session.PCIeDevices(ctx)
	case bmc.BackendAMI:
		rows, sys, unavail, err := s.sysInv(ctx, "PCIeDevices", "system_inventory/pci_info", "host_inventory/host_interface_pcie_device_function_info")
		if err != nil {
			return nil, err
		}
		var devs []map[string]any
		if sys {
			devs = rows
		} else {
			for _, r := range rows {
				if l, ok := r["PCIE_Devices"].([]any); ok {
					for _, x := range l {
						if m, ok := x.(map[string]any); ok {
							devs = append(devs, m)
						}
					}
				}
			}
		}
		out := []bmc.PCIeDevice{}
		for i, m := range devs {
			d := bmc.PCIeDevice{ID: strOf(m, "Id", "id", "Name"), Name: strOf(m, "Name"), Manufacturer: strOf(m, "Manufacturer"),
				VendorID: strOf(m, "VendorId"), DeviceID: strOf(m, "DeviceId"), SubsystemVendorID: strOf(m, "SubsystemVendorId"),
				SubsystemID: strOf(m, "SubsystemId"), ClassCode: strOf(m, "ClassCode"), Slot: strOf(m, "Slot"),
				LinkWidth: strOf(m, "LanesInUse", "LinkWidth"), LinkSpeed: strOf(m, "PCIeType", "LinkSpeed"),
				Health: healthFromState(m), Oem: m}
			if d.ID == "" {
				d.ID = strconv.Itoa(i)
			}
			out = append(out, d)
		}
		if len(out) == 0 && unavail != nil {
			return nil, unavail
		}
		return out, nil
	}
	return nil, s.unsupported("PCIeDevices")
}

// Storage implements bmc.Inventory (host_interface_storage_info
// [{Storage_Drives:[...], Storage_Contoller:[...]}]; "Contoller" sic).
func (s *Session) Storage(ctx context.Context) ([]bmc.StorageDevice, error) {
	switch s.backend(bmc.CapInventory) {
	case bmc.BackendRedfish:
		return s.Session.Storage(ctx)
	case bmc.BackendAMI:
		rows, err := s.hostInv(ctx, "Storage", "host_inventory/host_interface_storage_info")
		if err != nil {
			return nil, err
		}
		out := []bmc.StorageDevice{}
		for _, r := range rows {
			l, _ := r["Storage_Drives"].([]any)
			for i, x := range l {
				m, ok := x.(map[string]any)
				if !ok {
					continue
				}
				d := bmc.StorageDevice{ID: strOf(m, "Id", "Name"), Name: strOf(m, "Name"), Manufacturer: strOf(m, "Manufacturer"),
					Model: strOf(m, "Model"), Serial: strOf(m, "SerialNumber"), Firmware: strOf(m, "FirmwareVersion"),
					MediaType: strings.ToLower(strOf(m, "MediaType")), Protocol: strings.ToLower(strOf(m, "Protocol")),
					Health: healthFromState(m)}
				if v, ok := oem.Int(m, "CapacityBytes"); ok {
					d.CapacityBytes = int64(v)
				}
				if d.ID == "" {
					d.ID = strconv.Itoa(i)
				}
				out = append(out, d)
			}
		}
		return out, nil
	}
	return nil, s.unsupported("Storage")
}

// Baseboard implements bmc.Inventory. AMI: FRU board area + asrr/fw-info
// BIOS version + asrr/model-name.
func (s *Session) Baseboard(ctx context.Context) (bmc.Baseboard, error) {
	switch s.backend(bmc.CapInventory) {
	case bmc.BackendRedfish:
		bb, err := s.Session.Baseboard(ctx)
		if err == nil && bb.Product == "" {
			bb.Product = s.prof.Board
		}
		return bb, err
	case bmc.BackendAMI:
		bb := bmc.Baseboard{Product: s.prof.Board}
		frus, err := s.amiFRU(ctx)
		if err != nil {
			return bmc.Baseboard{}, err
		}
		if len(frus) > 0 {
			b := frus[0].Board
			bb.Manufacturer, bb.Serial, bb.PartNumber = b.Manufacturer, b.Serial, b.PartNumber
			if b.Product != "" {
				bb.Product = b.Product
			}
		}
		if fw, err := s.fwInfoMap(ctx, false); err == nil {
			bb.BIOSVersion = oem.Str(fw, "BIOS_fw_version")
		} else if isLost(err) {
			return bmc.Baseboard{}, err
		}
		return bb, nil
	}
	return bmc.Baseboard{}, s.unsupported("Baseboard")
}
