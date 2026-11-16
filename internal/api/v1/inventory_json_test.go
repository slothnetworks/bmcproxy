package v1

import (
	"encoding/json"
	"testing"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

func TestInventoryResponseEmptyCollectionEmitted(t *testing.T) {
	b, err := json.Marshal(InventoryResponse{Kind: bmc.InventoryCPUs})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `{"kind":"cpus","cpus":[]}`; got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	b, _ = json.Marshal(InventoryResponse{Kind: bmc.InventoryMemory, Memory: []bmc.MemoryModule{{}}})
	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)
	if _, ok := m["memory"]; !ok || len(m) != 2 {
		t.Fatalf("unexpected keys: %s", b)
	}
}
