package testsim

import "time"

// powerState is the host power state machine shared by /api and Redfish.
// Transitions take Options.PowerDelay; soft-off may be silently ignored
// (Options.PowerIgnoresSoftOff), as seen mid-POST on real hardware.
type powerState struct {
	on      bool
	pending bool
	target  bool
	at      time.Time
	led     int // 0 off, 1 blink, 2 on (uid_command codes)
}

func (p *powerState) settle(now time.Time) {
	if p.pending && !now.Before(p.at) {
		p.on, p.pending = p.target, false
	}
}

func (p *powerState) isOn(now time.Time) bool {
	p.settle(now)
	return p.on
}

// AMI actions/power codes.
const (
	PowerCmdOff     = 0
	PowerCmdOn      = 1
	PowerCmdCycle   = 2
	PowerCmdReset   = 3
	PowerCmdSoftOff = 5
)

// powerAction applies an AMI power_command; false if the code is unknown.
func (s *Sim) powerAction(cmd int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.power.settle(now)
	set := func(on bool) {
		if s.opts.PowerDelay <= 0 {
			s.power.on, s.power.pending = on, false
			return
		}
		s.power.pending, s.power.target, s.power.at = true, on, now.Add(s.opts.PowerDelay)
	}
	switch cmd {
	case PowerCmdOff:
		set(false)
	case PowerCmdOn:
		set(true)
	case PowerCmdCycle:
		s.power.on = false
		set(true)
	case PowerCmdReset:
		if s.power.on {
			set(true)
		}
	case PowerCmdSoftOff:
		if s.opts.PowerIgnoresSoftOff {
			s.stats.IgnoredSoftOffs++
		} else if s.power.on {
			set(false)
		}
	default:
		return false
	}
	s.stats.PowerActions++
	return true
}

// redfishResetToCmd maps a ComputerSystem.Reset ResetType to an AMI code.
func redfishResetToCmd(rt string) (int, bool) {
	switch rt {
	case "On", "ForceOn":
		return PowerCmdOn, true
	case "ForceOff":
		return PowerCmdOff, true
	case "GracefulShutdown":
		return PowerCmdSoftOff, true
	case "ForceRestart", "GracefulRestart":
		return PowerCmdReset, true
	case "PowerCycle":
		return PowerCmdCycle, true
	}
	return 0, false
}
