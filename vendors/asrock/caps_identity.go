package asrock

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/oem"
)

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

// fwInfo returns asrr/fw-info (cached from Open, refreshed on demand).
func (s *Session) fwInfoMap(ctx context.Context, refresh bool) (map[string]any, error) {
	s.mu.Lock()
	m := s.fwInfo
	s.mu.Unlock()
	if m != nil && !refresh {
		return m, nil
	}
	var fw map[string]any
	if err := s.api.Get(ctx, "asrr/fw-info", &fw); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.fwInfo = fw
	s.mu.Unlock()
	return fw, nil
}

// SystemInfo implements bmc.Identity. Redfish: Systems/Self + Managers/Self
// with the board/firmware from /api; AMI: asrr/fw-info, asrr/model-name,
// fru (board serial), chassis-status, settings/network (BMC MAC).
func (s *Session) SystemInfo(ctx context.Context) (bmc.SystemInfo, error) {
	switch s.backend(bmc.CapIdentity) {
	case bmc.BackendRedfish:
		si, err := s.Session.SystemInfo(ctx)
		if err != nil {
			return si, err
		}
		si.Vendor = Name
		if s.prof.Board != "" {
			si.Board = s.prof.Board
		}
		if s.prof.Firmware != "" {
			si.BMCFirmware = s.prof.Firmware
		}
		// X570 1.20 Redfish reports the SMBIOS placeholders
		// "To Be Filled By O.E.M." and " " (its /api FRU board area says
		// ASRockRack).
		if isOEMPlaceholder(si.Manufacturer) {
			si.Manufacturer = "ASRockRack"
		}
		if isOEMPlaceholder(si.Model) {
			si.Model = ""
		}
		if s.amiOK {
			if fw, err := s.fwInfoMap(ctx, false); err == nil {
				addFWOem(&si, fw)
			}
		}
		return si, nil
	case bmc.BackendAMI:
		return s.amiSystemInfo(ctx)
	}
	return bmc.SystemInfo{}, s.unsupported("SystemInfo")
}

// isOEMPlaceholder reports whether v is empty or an unfilled SMBIOS/FRU
// placeholder string.
func isOEMPlaceholder(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "to be filled by o.e.m.", "to be filled by oem", "default string", "system manufacturer",
		"system product name", "not specified", "n/a", "na", "0123456789":
		return true
	}
	return false
}

func addFWOem(si *bmc.SystemInfo, fw map[string]any) {
	if si.Oem == nil {
		si.Oem = map[string]any{}
	}
	for _, k := range []string{"ME_fw_version", "Micro_Code_version", "CPLD_version", "PSP_fw_version", "BP_version"} {
		if v := oem.Str(fw, k); v != "" {
			si.Oem[k] = v
		}
	}
	if len(si.Oem) == 0 {
		si.Oem = nil
	}
}

func (s *Session) amiSystemInfo(ctx context.Context) (bmc.SystemInfo, error) {
	fw, err := s.fwInfoMap(ctx, true)
	if err != nil {
		return bmc.SystemInfo{}, err
	}
	si := bmc.SystemInfo{Vendor: Name, Manufacturer: "ASRockRack", Board: s.prof.Board,
		BMCFirmware: oem.Str(fw, "BMC_fw_version"), BIOSVersion: oem.Str(fw, "BIOS_fw_version")}
	if si.BMCFirmware == "" {
		si.BMCFirmware = s.prof.Firmware
	}
	if s.sysName != "" {
		si.Model = s.sysName
	}
	addFWOem(&si, fw)
	// Best-effort extras: a failure here does not fail the identity call,
	// except a lost session.
	if frus, err := s.amiFRU(ctx); err == nil && len(frus) > 0 {
		b := frus[0].Board
		if !isOEMPlaceholder(b.Manufacturer) {
			si.Manufacturer = b.Manufacturer
		}
		si.Serial = b.Serial
		if si.Model == "" && frus[0].Product.Name != "" {
			si.Model = frus[0].Product.Name
		}
	} else if isLost(err) {
		return bmc.SystemInfo{}, err
	}
	if st, err := s.amiChassis(ctx); err == nil {
		si.PowerState = st.power()
	} else if isLost(err) {
		return bmc.SystemInfo{}, err
	}
	if nics, err := s.amiNetwork(ctx); err == nil {
		for _, n := range nics {
			if n.MAC != "" {
				si.BMCMAC = n.MAC
				break
			}
		}
	} else if isLost(err) {
		return bmc.SystemInfo{}, err
	}
	si.Health = bmc.HealthUnknown
	return si, nil
}

func isLost(err error) bool { return err != nil && errorsIs(err, bmc.ErrSessionLost) }

// Firmware implements bmc.FirmwareInventory. AMI: asrr/fw-info ("N/A"
// components are skipped).
func (s *Session) Firmware(ctx context.Context) ([]bmc.FirmwareComponent, error) {
	switch s.backend(bmc.CapFirmwareInventory) {
	case bmc.BackendRedfish:
		return s.Session.Firmware(ctx)
	case bmc.BackendAMI:
		fw, err := s.fwInfoMap(ctx, true)
		if err != nil {
			return nil, err
		}
		names := []struct{ key, id, name string }{
			{"BMC_fw_version", "bmc", "BMC"}, {"BIOS_fw_version", "bios", "BIOS"}, {"ME_fw_version", "me", "ME"},
			{"PSP_fw_version", "psp", "PSP"}, {"Micro_Code_version", "microcode", "CPU microcode"},
			{"CPLD_version", "cpld", "CPLD"}, {"BP_version", "bp", "Backplane"}, {"BPCPLD_version", "bpcpld", "Backplane CPLD"},
		}
		out := []bmc.FirmwareComponent{}
		for _, n := range names {
			v := oem.Str(fw, n.key)
			if v == "" || amimegarac.IsNA(v) {
				continue
			}
			out = append(out, bmc.FirmwareComponent{ID: n.id, Name: n.name, Version: v, Updateable: false})
		}
		// Anything else the firmware reports.
		for k, raw := range fw {
			if !strings.HasSuffix(k, "_version") || knownFWKey(k, names) {
				continue
			}
			if v, ok := raw.(string); ok && !amimegarac.IsNA(v) {
				out = append(out, bmc.FirmwareComponent{ID: strings.ToLower(strings.TrimSuffix(k, "_version")), Name: k, Version: amimegarac.CleanString(v)})
			}
		}
		return out, nil
	}
	return nil, s.unsupported("Firmware")
}

func knownFWKey(k string, names []struct{ key, id, name string }) bool {
	for _, n := range names {
		if n.key == k {
			return true
		}
	}
	return false
}
