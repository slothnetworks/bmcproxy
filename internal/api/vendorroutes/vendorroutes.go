// Package vendorroutes mounts every registered driver's VendorRoutes() under
// /api/v1/vendor/{driver}/..., applying the tier gate (Route.Tier or
// Route.TierFor) and the HandlerFunc result contract (nil -> 204, Responder
// -> as-is, otherwise JSON).
package vendorroutes

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/slothnetworks/bmcproxy/internal/server"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// Mount registers the vendor routes of every driver in reg (default: the
// server's registry). Routes that fail to mount (bad or conflicting
// patterns) are logged and skipped; the count mounted is returned.
func Mount(s *server.Server, reg *bmc.Registry) int {
	if reg == nil {
		reg = s.Registry()
	}
	log := s.Obs().Logger(obs.SubNorth)
	n := 0
	for _, d := range reg.Drivers() {
		vr, ok := d.(bmc.VendorRouter)
		if !ok {
			continue
		}
		vendor := strings.ToLower(d.Name())
		prefix := server.VendorPrefix(vendor)
		for _, rt := range vr.VendorRoutes() {
			if rt.Handler == nil {
				log.Error("vendor route without handler; skipped", "driver", vendor, "path", rt.Path)
				continue
			}
			rt.Path = strings.TrimPrefix(rt.Path, "/")
			err := s.Handle(server.Route{
				Method:      rt.Method,
				Path:        prefix + "/" + rt.Path,
				Tier:        rt.Tier,
				TierFor:     rt.TierFor,
				NeedsTarget: true,
				Group:       "vendor:" + vendor,
				Summary:     rt.Summary,
				MaxBody:     maxBody(rt),
				Handler:     handler(s, vendor, rt),
			})
			if err != nil {
				log.Error("vendor route not mounted", "driver", vendor, "pattern", rt.Pattern(prefix), "err", err)
				continue
			}
			log.Debug("vendor route mounted", "driver", vendor, "pattern", rt.Pattern(prefix), "tier", rt.Tier)
			n++
		}
	}
	return n
}

// maxBody: raw passthroughs may carry large bodies (uploads); others use the
// server default.
func maxBody(rt bmc.Route) int64 {
	if strings.HasPrefix(rt.Path, "raw/") {
		return -1
	}
	return 0
}

func handler(s *server.Server, vendor string, rt bmc.Route) func(*http.Request) (any, error) {
	return func(r *http.Request) (any, error) {
		t, _ := server.TargetFrom(r.Context())
		if t.VendorHint != "" && t.VendorHint != vendor {
			return nil, bmc.Errorf(bmc.ErrInvalidArgument, "vendorroutes", rt.Path,
				"%s: %s conflicts with the /api/v1/vendor/%s/ route", server.HeaderVendor, t.VendorHint, vendor)
		}
		return s.Call(r, func(ctx context.Context, h *sessions.Handle) (any, error) {
			if got := strings.ToLower(h.Driver.Name()); got != vendor {
				return nil, bmc.Errorf(bmc.ErrNotFound, "vendorroutes", rt.Path,
					"target %s is handled by driver %q, not %q; use /api/v1/vendor/%s/ or set %s: %s",
					h.Key.Host, got, vendor, got, server.HeaderVendor, vendor)
			}
			obs.FromContext(ctx, obs.SubNorth).Log(ctx, slog.LevelDebug, "vendor route", "driver", vendor, "path", rt.Path)
			return rt.Handler(h.Session, r.WithContext(ctx))
		})
	}
}
