package testsim

import (
	"net/http"
	"strconv"
	"sync"
)

// Stateful ASRock OEM /api objects the HAR captures do not cover (or only
// cover as a single read): fan control mode, keep-NIC-link-up and the
// network bond. Shapes come from the live probes and the firmware/UI
// catalog; each GET returns the current object and each write merges the
// request body into it (like the BMC handlers, which store what the UI
// sends):
//
//   - X470 GET asrr/settings/getfancontrolmode: a one-element array of
//     {mode1..mode16}, all 0 live on 2.32 (docs/x470/endpoint-reference.json);
//     POST asrr/settings/setfancontrolmode {mode1..mode16} (a write_same
//     round trip answered 200 live).
//   - X470 GET asrr/getvetobit {vetobit} (live 2.32: {"vetobit":0});
//     PUT asrr/setvetobit {vetobit} (UI JS).
//   - B650 GET/PUT settings/fan/mode: {platform "AMD_platform", num_of_fan 16,
//     min_duty 20, max_duty 100} live on 4.10 (docs/b650/live-probe.md §5).
//   - B650 GET/PUT settings/ncsilinkup {sharednic, sharednic2} (7.02/9.05 UI
//     JS; values not captured, 0/0 assumed).
//   - B650 GET asrr/GetPowerRestore {powerRestore}, POST asrr/SetPowerRestore
//     {powerRestore} (7.02/9.05 UI JS; values not captured, 1 assumed).
//   - Both: PUT settings/network-bond with the full object
//     {id, bond_enable, bond_mode, auto_configuration_enable[, bond_ifc]}
//     (B650 HAR); GET then reflects it (bond_ifc is not returned by GET).
type oemState struct {
	mu   sync.Mutex
	docs map[string]map[string]any // GET path -> current object
}

// oemDoc describes one stateful object: its GET path, write method+path,
// whether GET wraps it in a one-element array, and the keys GET hides.
type oemDoc struct {
	get, writeMethod, write string
	array                   bool
	hidden                  []string
	initial                 func(s *Sim) map[string]any
}

func oemDocs(b Board) []oemDoc {
	bond := oemDoc{get: "/api/settings/network-bond", writeMethod: http.MethodPut, write: "/api/settings/network-bond",
		hidden: []string{"bond_ifc"}, initial: func(s *Sim) map[string]any {
			m, _ := s.fixtureObject(http.MethodGet, "/api/settings/network-bond").(map[string]any)
			if m == nil {
				m = map[string]any{"id": 1, "bond_enable": 0, "bond_mode": "active-backup", "auto_configuration_enable": 1}
			}
			return m
		}}
	if b == BoardX470 {
		return []oemDoc{bond,
			{get: "/api/asrr/settings/getfancontrolmode", writeMethod: http.MethodPost, write: "/api/asrr/settings/setfancontrolmode",
				array: true, initial: func(*Sim) map[string]any {
					m := map[string]any{}
					for i := 1; i <= 16; i++ {
						m["mode"+strconv.Itoa(i)] = 0
					}
					return m
				}},
			{get: "/api/asrr/getvetobit", writeMethod: http.MethodPut, write: "/api/asrr/setvetobit",
				initial: func(*Sim) map[string]any { return map[string]any{"vetobit": 0} }},
		}
	}
	return []oemDoc{bond,
		{get: "/api/settings/fan/mode", writeMethod: http.MethodPut, write: "/api/settings/fan/mode",
			initial: func(*Sim) map[string]any {
				return map[string]any{"platform": "AMD_platform", "num_of_fan": 16, "min_duty": 20, "max_duty": 100}
			}},
		{get: "/api/settings/ncsilinkup", writeMethod: http.MethodPut, write: "/api/settings/ncsilinkup",
			initial: func(*Sim) map[string]any { return map[string]any{"sharednic": 0, "sharednic2": 0} }},
		{get: "/api/asrr/GetPowerRestore", writeMethod: http.MethodPost, write: "/api/asrr/SetPowerRestore",
			initial: func(*Sim) map[string]any { return map[string]any{"powerRestore": 1} }},
	}
}

// oemAPI serves the stateful OEM objects; false falls through.
func (s *Sim) oemAPI(w http.ResponseWriter, method, p string, body []byte) bool {
	for _, d := range oemDocs(s.profile.Board) {
		switch {
		case method == http.MethodGet && p == d.get:
			cur := s.oemDoc(d)
			for _, k := range d.hidden {
				delete(cur, k)
			}
			if d.array {
				writeJSON(w, http.StatusOK, []any{cur})
			} else {
				writeJSON(w, http.StatusOK, cur)
			}
			return true
		case method == d.writeMethod && p == d.write:
			v, err := decodeAny(body)
			m, ok := v.(map[string]any)
			if err != nil || !ok {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid data", "code": 1001})
				return true
			}
			s.oem.mu.Lock()
			cur := s.oemDocLocked(d)
			for k, v := range m {
				cur[k] = v
			}
			out := cloneJSON(cur)
			s.oem.mu.Unlock()
			writeJSON(w, http.StatusOK, out)
			return true
		}
	}
	return false
}

func (s *Sim) oemDoc(d oemDoc) map[string]any {
	s.oem.mu.Lock()
	defer s.oem.mu.Unlock()
	return cloneJSON(s.oemDocLocked(d)).(map[string]any)
}

func (s *Sim) oemDocLocked(d oemDoc) map[string]any {
	if s.oem.docs == nil {
		s.oem.docs = map[string]map[string]any{}
	}
	cur, ok := s.oem.docs[d.get]
	if !ok {
		cur = d.initial(s)
		s.oem.docs[d.get] = cur
	}
	return cur
}

// OEMObject returns a copy of a stateful OEM object by its GET path (for
// example "/api/settings/network-bond"), or nil if the board has none.
func (s *Sim) OEMObject(getPath string) map[string]any {
	for _, d := range oemDocs(s.profile.Board) {
		if d.get == getPath {
			return s.oemDoc(d)
		}
	}
	return nil
}
