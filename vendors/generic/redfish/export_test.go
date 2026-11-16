package genericredfish

import "time"

// SetNTPVerify shortens the NTP read-back verification for tests and
// returns a func restoring the previous values.
func SetNTPVerify(timeout, interval time.Duration) func() {
	t, i := ntpVerifyTimeout, ntpVerifyInterval
	ntpVerifyTimeout, ntpVerifyInterval = timeout, interval
	return func() { ntpVerifyTimeout, ntpVerifyInterval = t, i }
}
