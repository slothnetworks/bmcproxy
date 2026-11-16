package integration_test

import (
	"context"
	"testing"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// TestRegistryResolvesEveryBoard: the full production registry (gigabyte,
// asrock, generic Redfish) resolves each ASRock sim to asrock and each
// Gigabyte sim to gigabyte, with and without credentials, and never logs
// in to a Gigabyte /api while doing so.
func TestRegistryResolvesEveryBoard(t *testing.T) {
	ctx := context.Background()
	for _, board := range append(append([]string(nil), allBoards...), gigabyteBoards...) {
		t.Run(board, func(t *testing.T) {
			sim := startSim(t, board)
			tm := transport.NewManager(transport.Config{Retry: transport.RetryPolicy{MaxAttempts: 1}, Breaker: transport.BreakerConfig{Disabled: true}})
			t.Cleanup(tm.Close)
			reg := bmc.NewRegistry()
			for _, d := range drivers(&fakeIPMI{})(tm) {
				reg.Register(d)
			}
			c := sim.Creds()
			for _, creds := range []*bmc.Creds{nil, &c} {
				d, m, err := reg.Resolve(ctx, sim.Target(), creds)
				if err != nil {
					t.Fatalf("creds=%v: %v", creds != nil, err)
				}
				if d.Name() != boardVendor[board] {
					t.Fatalf("creds=%v: resolved %s (%+v), want %s", creds != nil, d.Name(), m, boardVendor[board])
				}
				if creds != nil && (m.Score < bmc.ScoreCertain || m.Board != boardName[board]) {
					t.Fatalf("with creds: %+v, want certain match on %s", m, boardName[board])
				}
			}
			if boardVendor[board] == "gigabyte" {
				if n := sim.Count("POST", "/api/session"); n != 0 {
					t.Fatalf("%d /api logins during resolve", n)
				}
			}
		})
	}
}
