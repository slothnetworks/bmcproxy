package testsim

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// X570D4U 1.20.00 Redfish, from docs/x570/live-probe.md and
// docs/x570/live-redfish-inventory.json. The resource tree is the B650 one
// (same AMI Redfish stack generation, same account handling) reshaped where
// the live X570 differs in ways the proxy cares about:
//
//   - ServiceRoot v1_5_2, RedfishVersion 1.8.0, no DeepOperations.
//   - Chassis/Self/Sensors: 3 discrete/PSU members with null readings
//     (/api/sensors has the real 48).
//   - Managers/Self/LogServices/SEL/Entries: 2 records (the /api SEL had
//     3639).
//   - FirmwareInventory: only BMC carries a Version (BIOS Version null).
//   - Systems/Self: Manufacturer "To Be Filled By O.E.M.", Model " ",
//     PowerRestorePolicy null.
//   - Managers/Self FirmwareVersion "1.20.0".
func x570Redfish(r map[string]map[string]any) {
	if root := r["/redfish/v1"]; root != nil {
		root["@odata.type"] = "#ServiceRoot.v1_5_2.ServiceRoot"
		root["RedfishVersion"] = "1.8.0"
		if pf, ok := root["ProtocolFeaturesSupported"].(map[string]any); ok {
			delete(pf, "DeepOperations")
		}
		if oem, ok := root["Oem"].(map[string]any); ok {
			if ami, ok := oem["Ami"].(map[string]any); ok {
				ami["RtpVersion"] = "1.8.0"
			}
		}
	}
	const sensors = "/redfish/v1/Chassis/Self/Sensors"
	for p := range r {
		if strings.HasPrefix(p, sensors+"/") {
			delete(r, p)
		}
	}
	var members []string
	for _, x := range []struct{ id, name, typ string }{{"112", "PSU1_Status", "Power"}, {"145", "PSU2_Status", "Power"}, {"160", "Chassis_Intrusion", "Physical"}} {
		p := sensors + "/" + x.id
		members = append(members, p)
		r[p] = map[string]any{"@odata.id": p, "@odata.type": "#Sensor.v1_0_2.Sensor", "Id": x.id, "Name": x.name,
			"ReadingType": x.typ, "ReadingUnits": nil, "Reading": nil, "Status": map[string]any{"State": "Enabled", "Health": "OK"}}
	}
	r[sensors] = collection(sensors, "#SensorCollection.SensorCollection", members)

	const sel = "/redfish/v1/Managers/Self/LogServices/SEL/Entries"
	if coll := r[sel]; coll != nil {
		var keep []string
		for p := range r {
			if strings.HasPrefix(p, sel+"/") {
				if id := strings.TrimPrefix(p, sel+"/"); id == "1" || id == "2" {
					keep = append(keep, p)
					r[p]["Created"] = "1969-12-31T19:00:00-05:00"
					continue
				}
				delete(r, p)
			}
		}
		sort.Strings(keep)
		r[sel] = collection(sel, "#LogEntryCollection.LogEntryCollection", keep)
	}
	if bios := r["/redfish/v1/UpdateService/FirmwareInventory/BIOS"]; bios != nil {
		bios["Version"] = nil
	}
	if bmcFW := r["/redfish/v1/UpdateService/FirmwareInventory/BMC"]; bmcFW != nil {
		bmcFW["Version"] = "1.20.00"
	}
	if sys := r["/redfish/v1/Systems/Self"]; sys != nil {
		sys["Manufacturer"] = "To Be Filled By O.E.M."
		sys["Model"] = " "
		sys["PowerRestorePolicy"] = nil
		sys["BiosVersion"] = "L1.57"
	}
	if mgr := r["/redfish/v1/Managers/Self"]; mgr != nil {
		mgr["FirmwareVersion"] = "1.20.0"
	}
}

func collection(path, typ string, members []string) map[string]any {
	ms := make([]any, 0, len(members))
	for _, m := range members {
		ms = append(ms, map[string]any{"@odata.id": m})
	}
	return map[string]any{"@odata.id": path, "@odata.type": typ, "Name": typ, "Members": ms, "Members@odata.count": len(members)}
}

// stripSelectExpand drops $select and $expand from r's query (X570 1.20
// ignores both and answers the full, unexpanded resource).
func stripSelectExpand(r *http.Request) {
	if r.URL.RawQuery == "" {
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return
	}
	if _, a := q["$select"]; !a {
		if _, b := q["$expand"]; !b {
			return
		}
	}
	q.Del("$select")
	q.Del("$expand")
	r.URL.RawQuery = q.Encode()
	r.RequestURI = r.URL.RequestURI()
}
