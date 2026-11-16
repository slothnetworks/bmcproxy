package transport

import (
	"sort"
	"sync"
	"time"
)

// Outcome classes used in stats, hooks and logs.
const (
	ClassOK          = "ok"
	ClassClientError = "client_error" // 4xx other than 401 / evasive 403
	ClassAuth        = "auth"         // 401
	ClassBusy        = "busy"         // 503, evasive 403
	ClassServerError = "server_error" // other 5xx
	ClassTimeout     = "timeout"
	ClassConn        = "conn" // connection/TLS errors
	ClassBreakerOpen = "breaker_open"
	ClassCanceled    = "canceled"
)

// LatencyBuckets are the histogram upper bounds in seconds (Prometheus
// style; +Inf implied).
var LatencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}

// Histogram is a cumulative-bucket latency histogram snapshot, directly
// convertible to prometheus.MustNewConstHistogram.
type Histogram struct {
	Buckets []float64          `json:"buckets"` // upper bounds (s)
	Counts  map[float64]uint64 `json:"-"`       // cumulative count per bound
	// Cumulative is Counts in bucket order (for JSON).
	Cumulative []uint64 `json:"cumulative"`
	Count      uint64   `json:"count"`
	Sum        float64  `json:"sum"` // seconds
}

type histogram struct {
	counts []uint64 // non-cumulative, len(LatencyBuckets)+1
	count  uint64
	sum    float64
}

func newHistogram() histogram { return histogram{counts: make([]uint64, len(LatencyBuckets)+1)} }

func (h *histogram) observe(d time.Duration) {
	s := d.Seconds()
	i := sort.SearchFloat64s(LatencyBuckets, s)
	h.counts[i]++
	h.count++
	h.sum += s
}

func (h *histogram) snapshot() Histogram {
	out := Histogram{Buckets: LatencyBuckets, Counts: make(map[float64]uint64, len(LatencyBuckets)), Count: h.count, Sum: h.sum}
	var cum uint64
	for i, b := range LatencyBuckets {
		cum += h.counts[i]
		out.Counts[b] = cum
		out.Cumulative = append(out.Cumulative, cum)
	}
	return out
}

// LaneStats is the limiter state of one (host, lane).
type LaneStats struct {
	Name     string `json:"name"`
	Limit    int    `json:"limit"`
	InFlight int    `json:"inFlight"`
	Queued   int    `json:"queued"`
}

// HostStats is a snapshot of one BMC host's southbound activity. It backs
// /debug/stats, /metrics and /bmcs/{host}/status.
type HostStats struct {
	Host     string            `json:"host"`
	Requests uint64            `json:"requests"` // logical calls (Do)
	Attempts uint64            `json:"attempts"` // HTTP attempts incl. retries
	Retries  uint64            `json:"retries"`
	Classes  map[string]uint64 `json:"classes"` // final outcome class counts of logical calls
	Lanes    []LaneStats       `json:"lanes"`
	Breaker  BreakerSnapshot   `json:"breaker"`
	// Latency of logical calls (queue wait + all attempts).
	Latency Histogram `json:"latency"`
	// QueueWait totals time spent waiting for the per-host limiter.
	QueueWaitSeconds float64   `json:"queueWaitSeconds"`
	QueueWaits       uint64    `json:"queueWaits"`
	LastError        string    `json:"lastError,omitempty"`
	LastErrorAt      time.Time `json:"lastErrorAt,omitzero"`
	LastSuccessAt    time.Time `json:"lastSuccessAt,omitzero"`
	// Profile is the configured TLS profile; TLS is the last negotiated
	// connection (nil before the first TLS connection).
	Profile TLSProfile `json:"-"`
	TLS     *TLSInfo   `json:"tls,omitempty"`
}

// AttemptInfo is passed to Hooks.OnAttempt after every HTTP attempt.
type AttemptInfo struct {
	Host, Lane, Method, Path, Op string
	Attempt                      int
	Status                       int // 0 on transport error
	Class                        string
	Duration                     time.Duration
	Err                          error
	WillRetry                    bool
}

// RequestInfo is passed to Hooks.OnRequest when a logical call finishes.
type RequestInfo struct {
	Host, Lane, Method, Path, Op string
	Attempts                     int
	Status                       int
	Class                        string
	Duration                     time.Duration // total incl. queueing and backoff
	Err                          error
}

// Hooks are optional push-style callbacks for metrics. They are called
// synchronously on the request goroutine and must be fast and
// concurrency-safe. Pull-style data is available from Manager.Stats.
type Hooks struct {
	OnAttempt       func(AttemptInfo)
	OnRequest       func(RequestInfo)
	OnBreakerChange func(host string, from, to BreakerState)
	OnQueueWait     func(host, lane string, wait time.Duration)
}

// hostCounters is guarded by hostState.mu.
type hostCounters struct {
	requests, attempts, retries uint64
	classes                     map[string]uint64
	latency                     histogram
	queueWait                   time.Duration
	queueWaits                  uint64
	lastErr                     string
	lastErrAt, lastOKAt         time.Time
	tls                         *TLSInfo
}

type statsMu struct {
	sync.Mutex
	c hostCounters
}
