package asrock

import (
	"context"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// The embedded *genericredfish.Session is nil when Redfish is not in use
// (X470D4U), so every promoted method is overridden here or in caps_*.go.

// Paths returns the Redfish resource paths (empty without Redfish).
func (s *Session) Paths() (system, manager, chassis, sel, audit string) {
	if s.Session == nil {
		return
	}
	return s.Session.Paths()
}

// VirtualMedia implements bmc.VirtualMedia (Redfish only; the /api media
// redirection is a KVM-applet feature and is not exposed).
func (s *Session) VirtualMedia(ctx context.Context) ([]bmc.VirtualMediaSlot, error) {
	if s.backend(bmc.CapVirtualMedia) == bmc.BackendRedfish {
		return s.Session.VirtualMedia(ctx)
	}
	return nil, s.unsupported("VirtualMedia")
}

// InsertMedia implements bmc.VirtualMedia (Redfish only).
func (s *Session) InsertMedia(ctx context.Context, r bmc.VirtualMediaInsert) error {
	if s.backend(bmc.CapVirtualMedia) == bmc.BackendRedfish {
		return s.Session.InsertMedia(ctx, r)
	}
	return s.unsupported("InsertMedia")
}

// EjectMedia implements bmc.VirtualMedia (Redfish only).
func (s *Session) EjectMedia(ctx context.Context, slotID string) error {
	if s.backend(bmc.CapVirtualMedia) == bmc.BackendRedfish {
		return s.Session.EjectMedia(ctx, slotID)
	}
	return s.unsupported("EjectMedia")
}

// ResetBMC implements bmc.BMCReset (disruptive). Redfish Manager.Reset when
// available; otherwise cold = AMI PUT asrr/bmccoldreset, warm = IPMI warm
// reset. Every session on the host is gone afterwards, so the session is
// marked lost and the next call fails with ErrSessionLost (the session
// manager then reopens).
func (s *Session) ResetBMC(ctx context.Context, kind bmc.BMCResetKind) error {
	const op = "ResetBMC"
	switch kind {
	case "", bmc.BMCResetCold, bmc.BMCResetWarm:
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "unknown reset kind %q (want cold|warm)", kind)
	}
	var err error
	switch s.backend(bmc.CapBMCReset) {
	case bmc.BackendRedfish:
		err = s.Session.ResetBMC(ctx, kind)
	case bmc.BackendAMI:
		if kind == bmc.BMCResetWarm {
			ic, ierr := s.ipmiClient(ctx)
			if ierr != nil {
				return bmc.Errorf(bmc.ErrUnsupported, Pkg, op, "warm reset needs IPMI (AMI /api only offers cold reset): %v", ierr)
			}
			err = ic.ResetBMC(ctx, bmc.BMCResetWarm)
		} else {
			err = s.oem.BMCColdReset(ctx)
		}
	default:
		return s.unsupported(op)
	}
	if err == nil {
		s.ami.MarkHostLost("BMC reset requested by this session")
	}
	return err
}
