package asrock

import (
	"context"
	"strconv"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
)

// amiSensor is one element of GET /api/sensors. X470 and B650 differ:
//
//   - the threshold mask is settable_readable_threshMask (X470) or
//     settable_flag (B650/B550): low byte = readable thresholds, high byte =
//     settable, bits 0..5 = LNC, LC, LNR, UNC, UC, UNR (IPMI order);
//   - unset thresholds are 0.0 (X470) or the string "NA" (B650);
//   - temperatures are "°C" (X470, B650 4.10) or "deg_c" (B650 1.10).
//
// accessible != 0 (213 = 0xD5) means the reading is unavailable (host off,
// device absent).
type amiSensor struct {
	ID            amimegarac.NullableInt   `json:"id"`
	Number        amimegarac.NullableInt   `json:"sensor_number"`
	Name          amimegarac.FlexString    `json:"name"`
	Type          amimegarac.FlexString    `json:"type"`
	TypeNumber    amimegarac.NullableInt   `json:"type_number"`
	Reading       amimegarac.NullableFloat `json:"reading"`
	SensorState   amimegarac.NullableInt   `json:"sensor_state"`
	DiscreteState amimegarac.NullableInt   `json:"discrete_state"`
	Accessible    amimegarac.NullableInt   `json:"accessible"`
	MaskX470      amimegarac.NullableInt   `json:"settable_readable_threshMask"`
	MaskB650      amimegarac.NullableInt   `json:"settable_flag"`
	LNR           amimegarac.NullableFloat `json:"lower_non_recoverable_threshold"`
	LC            amimegarac.NullableFloat `json:"lower_critical_threshold"`
	LNC           amimegarac.NullableFloat `json:"lower_non_critical_threshold"`
	UNC           amimegarac.NullableFloat `json:"higher_non_critical_threshold"`
	UC            amimegarac.NullableFloat `json:"higher_critical_threshold"`
	UNR           amimegarac.NullableFloat `json:"higher_non_recoverable_threshold"`
	Unit          amimegarac.FlexString    `json:"unit"`
}

// Threshold mask bits (readable byte).
const (
	maskLNC = 1 << 0
	maskLC  = 1 << 1
	maskLNR = 1 << 2
	maskUNC = 1 << 3
	maskUC  = 1 << 4
	maskUNR = 1 << 5
)

func amiUnit(u string) string {
	switch strings.ToLower(strings.TrimSpace(u)) {
	case "°c", "deg_c", "degrees c", "c", "℃":
		return "C"
	case "v", "volts":
		return "V"
	case "rpm":
		return "RPM"
	case "a", "amps":
		return "A"
	case "w", "watts":
		return "W"
	case "%", "percent":
		return "%"
	case "", "unknown", "na":
		return ""
	}
	return u
}

func amiSensorType(typ string, typeNum int64, unit string) bmc.SensorType {
	switch typeNum {
	case 1:
		return bmc.SensorTemperature
	case 2:
		return bmc.SensorVoltage
	case 3:
		return bmc.SensorCurrent
	case 4:
		return bmc.SensorFan
	}
	switch strings.ToLower(typ) {
	case "temperature":
		return bmc.SensorTemperature
	case "voltage":
		return bmc.SensorVoltage
	case "current":
		return bmc.SensorCurrent
	case "fan":
		return bmc.SensorFan
	}
	if unit == "W" {
		return bmc.SensorPower
	}
	return bmc.SensorOther
}

// threshold returns the threshold value, honouring the readable mask (when
// the board reports one) and treating X470 0.0 placeholders as unset.
func threshold(v amimegarac.NullableFloat, mask int64, haveMask bool, bit int64, zeroUnset bool) *float64 {
	if !v.Valid {
		return nil // "NA"
	}
	if haveMask {
		if mask&bit == 0 {
			return nil
		}
		return bmc.F64(v.V)
	}
	if zeroUnset && v.V == 0 {
		return nil
	}
	return bmc.F64(v.V)
}

// sensorFromAMI normalises one /api sensor. zeroUnset is the X470 quirk.
func sensorFromAMI(a amiSensor, zeroUnset bool) bmc.Sensor {
	unit := amiUnit(a.Unit.Clean())
	discrete := a.SensorState.Valid && a.SensorState.V == 0 && a.DiscreteState.V != 0 ||
		unit == "" && strings.ToLower(a.Unit.Clean()) == "unknown"
	s := bmc.Sensor{
		ID:      strconv.FormatInt(a.ID.V, 10),
		Name:    a.Name.Clean(),
		Unit:    unit,
		Type:    amiSensorType(a.Type.Clean(), a.TypeNumber.V, unit),
		Backend: bmc.BackendAMI,
		Health:  bmc.HealthUnknown,
		Oem:     map[string]any{"amiType": a.Type.Clean()},
	}
	if a.Number.Valid {
		n := int(a.Number.V)
		s.Number = &n
	}
	mask, haveMask := a.MaskB650, a.MaskB650.Valid
	if a.MaskX470.Valid {
		mask, haveMask = a.MaskX470, true
	}
	readable := mask.V & 0xff
	if haveMask {
		s.Settable = (mask.V>>8)&0x3f != 0
		s.Oem["thresholdMask"] = mask.V
	}
	available := !a.Accessible.Valid || a.Accessible.V == 0
	if a.Accessible.Valid && a.Accessible.V != 0 {
		s.Oem["accessible"] = a.Accessible.V
	}
	if discrete {
		s.Type = bmc.SensorDiscrete
		s.State = "0x" + strconv.FormatInt(a.DiscreteState.V, 16)
		if available {
			s.Health = bmc.HealthOK
		}
		return s
	}
	s.Thresholds = bmc.Thresholds{
		LowerNonRecoverable: threshold(a.LNR, readable, haveMask, maskLNR, zeroUnset),
		LowerCritical:       threshold(a.LC, readable, haveMask, maskLC, zeroUnset),
		LowerNonCritical:    threshold(a.LNC, readable, haveMask, maskLNC, zeroUnset),
		UpperNonCritical:    threshold(a.UNC, readable, haveMask, maskUNC, zeroUnset),
		UpperCritical:       threshold(a.UC, readable, haveMask, maskUC, zeroUnset),
		UpperNonRecoverable: threshold(a.UNR, readable, haveMask, maskUNR, zeroUnset),
	}
	if available && a.Reading.Valid {
		s.Reading = bmc.F64(a.Reading.V)
		s.Health = healthFromThresholds(a.Reading.V, s.Thresholds)
	}
	return s
}

func healthFromThresholds(v float64, t bmc.Thresholds) bmc.Health {
	below := func(p *float64) bool { return p != nil && v <= *p }
	above := func(p *float64) bool { return p != nil && v >= *p }
	switch {
	case below(t.LowerNonRecoverable), below(t.LowerCritical), above(t.UpperCritical), above(t.UpperNonRecoverable):
		return bmc.HealthCritical
	case below(t.LowerNonCritical), above(t.UpperNonCritical):
		return bmc.HealthWarning
	}
	return bmc.HealthOK
}

// Sensors implements bmc.SensorReader. Redfish: Chassis Sensors (B650);
// AMI: GET /api/sensors with "NA" / X470 0.0-threshold handling.
func (s *Session) Sensors(ctx context.Context) ([]bmc.Sensor, error) {
	switch s.backend(bmc.CapSensors) {
	case bmc.BackendRedfish:
		return s.Session.Sensors(ctx)
	case bmc.BackendAMI:
		var raw []amiSensor
		if err := s.api.Get(ctx, "sensors", &raw); err != nil {
			return nil, err
		}
		out := make([]bmc.Sensor, 0, len(raw))
		for _, a := range raw {
			if !a.ID.Valid && a.Name.Clean() == "" {
				continue
			}
			out = append(out, sensorFromAMI(a, s.prof.ZeroThresholds))
		}
		return out, nil
	}
	return nil, s.unsupported("Sensors")
}
