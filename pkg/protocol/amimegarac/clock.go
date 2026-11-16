package amimegarac

import (
	"context"
	"net/http"
	"time"
)

// DateTime is GET /api/settings/date-time. X470 2.32 example
// (docs/x470/har-endpoints.json):
//
//	{"id":1,"primary_ntp":"pool.ntp.org","secondary_ntp":"time.nist.gov","auto_date":1,
//	 "localized_timestamp":1794808815,"utc_minutes":0,"daylight":0,"timestamp":1794808815,"timezone":"GMT+0"}
//
// B650 renames auto_date to ntp_auto_date, drops daylight and adds ptp_*
// fields. "timestamp" is BMC UTC epoch seconds. localized_timestamp equals
// timestamp + utc_minutes*60.
type DateTime struct {
	ID                 NullableInt `json:"id"`
	PrimaryNTP         FlexString  `json:"primary_ntp"`
	SecondaryNTP       FlexString  `json:"secondary_ntp"`
	AutoDate           NullableInt `json:"auto_date"`     // X470
	NTPAutoDate        NullableInt `json:"ntp_auto_date"` // B650
	LocalizedTimestamp NullableInt `json:"localized_timestamp"`
	UTCMinutes         NullableInt `json:"utc_minutes"`
	Daylight           NullableInt `json:"daylight"`
	Timestamp          NullableInt `json:"timestamp"`
	Timezone           FlexString  `json:"timezone"`
}

// ClockSample is one measurement of the BMC clock.
type ClockSample struct {
	BMCTime time.Time     // BMC UTC time (1 s resolution)
	Offset  time.Duration // bmc − proxy, against the midpoint of the round trip
	RTT     time.Duration
	At      time.Time // proxy time at the midpoint
	Config  DateTime
}

// Clock samples the BMC clock with GET /api/settings/date-time. Use it to
// measure clock skew (bmc.NewTimestamp takes the offset). If the endpoint
// gives no timestamp, Clock falls back to the response's Date header.
func (c *Client) Clock(ctx context.Context) (ClockSample, error) {
	start := c.now()
	resp, err := c.Do(ctx, &Request{Method: http.MethodGet, Path: "/api/settings/date-time", Op: "amimegarac.Clock"})
	if err != nil {
		return ClockSample{}, err
	}
	end := c.now()
	var dt DateTime
	if err := resp.Decode(&dt); err != nil {
		return ClockSample{}, err
	}
	s := ClockSample{RTT: end.Sub(start), At: start.Add(end.Sub(start) / 2), Config: dt}
	switch {
	case dt.Timestamp.Valid && dt.Timestamp.V > 0:
		s.BMCTime = time.Unix(dt.Timestamp.V, 0).UTC()
	default:
		if d, err := http.ParseTime(resp.Header.Get("Date")); err == nil {
			s.BMCTime = d.UTC()
		}
	}
	if !s.BMCTime.IsZero() {
		s.Offset = s.BMCTime.Sub(s.At).Truncate(time.Second)
	}
	return s, nil
}
