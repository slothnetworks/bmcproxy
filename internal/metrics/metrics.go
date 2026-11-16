// Package metrics exposes Prometheus metrics at /metrics from a private
// registry: northbound request counts/latency per host and error class,
// southbound per-host counters and latency histograms (from transport
// stats), circuit-breaker state, live sessions and cache effectiveness.
package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/slothnetworks/bmcproxy/internal/cache"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

const ns = "bmcproxy"

// Sources are the snapshot sources read at scrape time. Any may be nil.
type Sources struct {
	Sessions *sessions.Manager
	Cache    *cache.Cache
}

// Metrics owns the private registry.
type Metrics struct {
	reg       *prometheus.Registry
	src       Sources
	once      sync.Once
	h         http.Handler
	transport atomic.Pointer[statsHolder]

	requests  *prometheus.CounterVec
	duration  *prometheus.HistogramVec
	breakerTx *prometheus.CounterVec
	queueWait *prometheus.HistogramVec
}

type statsHolder struct{ s sessions.StatsSource }

// New creates the metrics registry. Call SetTransport once the transport
// manager exists (it needs Hooks() at construction).
func New(src Sources) *Metrics {
	m := &Metrics{reg: prometheus.NewRegistry(), src: src}
	m.requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "http_requests_total",
		Help: "Northbound requests by BMC host, route, status code and error class.",
	}, []string{"host", "route", "code", "class"})
	m.duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns, Name: "http_request_duration_seconds",
		Help:    "Northbound request latency by route.",
		Buckets: transport.LatencyBuckets,
	}, []string{"route"})
	m.breakerTx = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "breaker_transitions_total",
		Help: "Circuit-breaker state transitions by host and new state.",
	}, []string{"host", "to"})
	m.queueWait = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: ns, Name: "southbound_queue_wait_seconds",
		Help:    "Time southbound calls waited for a lane or global slot.",
		Buckets: transport.LatencyBuckets,
	}, []string{"lane"})
	m.reg.MustRegister(m.requests, m.duration, m.breakerTx, m.queueWait, &snapshotCollector{m: m},
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

// SetSources replaces the snapshot sources. Call before serving.
func (m *Metrics) SetSources(src Sources) { m.src = src }

// SetTransport sets the transport stats source (usually *transport.Manager).
func (m *Metrics) SetTransport(s sessions.StatsSource) { m.transport.Store(&statsHolder{s}) }

// Registry returns the private registry.
func (m *Metrics) Registry() *prometheus.Registry { return m.reg }

// Handler serves the registry in the Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	m.once.Do(func() { m.h = promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{Registry: m.reg}) })
	return m.h
}

// Mount registers GET /metrics on mux.
func (m *Metrics) Mount(mux *http.ServeMux) { mux.HandleFunc("GET /metrics", m.serveMetrics) }

// serveMetrics serves the Prometheus exposition.
//
//	@Summary		Prometheus metrics
//	@Description	Prometheus text exposition from a private registry. Series include `bmcproxy_http_requests_total{host,route,code,class}`, `bmcproxy_http_request_duration_seconds{route}`, `bmcproxy_southbound_requests_total{host,class}`, `bmcproxy_southbound_latency_seconds{host}`, `bmcproxy_breaker_state{host}` (0 closed, 1 half-open, 2 open), `bmcproxy_sessions_live`, `bmcproxy_session_relogins_total`, `bmcproxy_cache_hits_total`/`_misses_total`/`_coalesced_total`, `bmcproxy_cache_hit_ratio`, lane in-flight/queued gauges, plus Go/process collectors.
//	@Tags			health
//	@Produce		plain
//	@Success		200	{string}	string	"Prometheus text format"
//	@Router			/metrics [get]
func (m *Metrics) serveMetrics(w http.ResponseWriter, r *http.Request) { m.Handler().ServeHTTP(w, r) }

// ObserveRequest implements server.RequestObserver.
func (m *Metrics) ObserveRequest(host, route, method string, status int, class string, d time.Duration) {
	if host == "" {
		host = "none"
	}
	if class == "" {
		class = "ok"
	}
	m.requests.WithLabelValues(host, route, strconv.Itoa(status), class).Inc()
	m.duration.WithLabelValues(route).Observe(d.Seconds())
}

// Hooks returns transport hooks feeding event-style metrics. Compose with
// other hooks if needed.
func (m *Metrics) Hooks() transport.Hooks {
	return transport.Hooks{
		OnBreakerChange: func(host string, _, to transport.BreakerState) {
			m.breakerTx.WithLabelValues(host, to.String()).Inc()
		},
		OnQueueWait: func(_, lane string, wait time.Duration) {
			m.queueWait.WithLabelValues(lane).Observe(wait.Seconds())
		},
	}
}

var (
	dSouthReq = prometheus.NewDesc(ns+"_southbound_requests_total", "Southbound logical requests by host and final outcome class.", []string{"host", "class"}, nil)
	dSouthAtt = prometheus.NewDesc(ns+"_southbound_attempts_total", "Southbound HTTP attempts including retries.", []string{"host"}, nil)
	dSouthRet = prometheus.NewDesc(ns+"_southbound_retries_total", "Southbound retries.", []string{"host"}, nil)
	dSouthLat = prometheus.NewDesc(ns+"_southbound_latency_seconds", "Southbound request latency by host.", []string{"host"}, nil)
	dBreaker  = prometheus.NewDesc(ns+"_breaker_state", "Circuit-breaker state per host (0 closed, 1 half-open, 2 open).", []string{"host"}, nil)
	dTrips    = prometheus.NewDesc(ns+"_breaker_trips_total", "Circuit-breaker trips per host.", []string{"host"}, nil)
	dInFlight = prometheus.NewDesc(ns+"_lane_in_flight", "Southbound requests in flight per host lane.", []string{"host", "lane"}, nil)
	dQueued   = prometheus.NewDesc(ns+"_lane_queued", "Southbound requests queued per host lane.", []string{"host", "lane"}, nil)

	dSessLive  = prometheus.NewDesc(ns+"_sessions_live", "Live cached BMC sessions.", nil, nil)
	dSessHost  = prometheus.NewDesc(ns+"_sessions_live_by_host", "Live cached BMC sessions per host.", []string{"host"}, nil)
	dSessOpen  = prometheus.NewDesc(ns+"_session_opens_total", "BMC sessions opened (logins).", nil, nil)
	dSessFail  = prometheus.NewDesc(ns+"_session_open_failures_total", "Failed session opens.", nil, nil)
	dSessHit   = prometheus.NewDesc(ns+"_session_reuses_total", "Requests served by a cached session.", nil, nil)
	dSessEvict = prometheus.NewDesc(ns+"_session_evictions_total", "Sessions evicted by idle TTL.", nil, nil)
	dSessRelog = prometheus.NewDesc(ns+"_session_relogins_total", "Relogins after an auth failure on a cached session.", nil, nil)
	dSessClose = prometheus.NewDesc(ns+"_session_logouts_total", "Session logouts (Close) by result.", []string{"result"}, nil)

	dCacheHit   = prometheus.NewDesc(ns+"_cache_hits_total", "Reads served from the TTL cache.", nil, nil)
	dCacheMiss  = prometheus.NewDesc(ns+"_cache_misses_total", "Reads that executed a BMC call.", nil, nil)
	dCacheCoal  = prometheus.NewDesc(ns+"_cache_coalesced_total", "Reads that joined an identical in-flight call (singleflight).", nil, nil)
	dCacheEnt   = prometheus.NewDesc(ns+"_cache_entries", "Stored cache entries.", nil, nil)
	dCacheRatio = prometheus.NewDesc(ns+"_cache_hit_ratio", "(hits+coalesced)/(hits+coalesced+misses) since start.", nil, nil)
)

type snapshotCollector struct{ m *Metrics }

func (c *snapshotCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{dSouthReq, dSouthAtt, dSouthRet, dSouthLat, dBreaker, dTrips, dInFlight, dQueued,
		dSessLive, dSessHost, dSessOpen, dSessFail, dSessHit, dSessEvict, dSessRelog, dSessClose,
		dCacheHit, dCacheMiss, dCacheCoal, dCacheEnt, dCacheRatio} {
		ch <- d
	}
}

func (c *snapshotCollector) Collect(ch chan<- prometheus.Metric) {
	m := c.m
	if h := m.transport.Load(); h != nil && h.s != nil {
		for _, hs := range h.s.Stats() {
			for class, n := range hs.Classes {
				ch <- prometheus.MustNewConstMetric(dSouthReq, prometheus.CounterValue, float64(n), hs.Host, class)
			}
			ch <- prometheus.MustNewConstMetric(dSouthAtt, prometheus.CounterValue, float64(hs.Attempts), hs.Host)
			ch <- prometheus.MustNewConstMetric(dSouthRet, prometheus.CounterValue, float64(hs.Retries), hs.Host)
			if hs.Latency.Counts != nil {
				ch <- prometheus.MustNewConstHistogram(dSouthLat, hs.Latency.Count, hs.Latency.Sum, hs.Latency.Counts, hs.Host)
			}
			ch <- prometheus.MustNewConstMetric(dBreaker, prometheus.GaugeValue, hs.Breaker.State.Gauge(), hs.Host)
			ch <- prometheus.MustNewConstMetric(dTrips, prometheus.CounterValue, float64(hs.Breaker.Trips), hs.Host)
			for _, l := range hs.Lanes {
				ch <- prometheus.MustNewConstMetric(dInFlight, prometheus.GaugeValue, float64(l.InFlight), hs.Host, l.Name)
				ch <- prometheus.MustNewConstMetric(dQueued, prometheus.GaugeValue, float64(l.Queued), hs.Host, l.Name)
			}
		}
	}
	if s := m.src.Sessions; s != nil {
		ctr := s.Counters()
		ch <- prometheus.MustNewConstMetric(dSessLive, prometheus.GaugeValue, float64(ctr.Live))
		ch <- prometheus.MustNewConstMetric(dSessOpen, prometheus.CounterValue, float64(ctr.Opens))
		ch <- prometheus.MustNewConstMetric(dSessFail, prometheus.CounterValue, float64(ctr.OpenFails))
		ch <- prometheus.MustNewConstMetric(dSessHit, prometheus.CounterValue, float64(ctr.Hits))
		ch <- prometheus.MustNewConstMetric(dSessEvict, prometheus.CounterValue, float64(ctr.Evictions))
		ch <- prometheus.MustNewConstMetric(dSessRelog, prometheus.CounterValue, float64(ctr.Relogins))
		ch <- prometheus.MustNewConstMetric(dSessClose, prometheus.CounterValue, float64(ctr.Closes-ctr.CloseErrs), "ok")
		ch <- prometheus.MustNewConstMetric(dSessClose, prometheus.CounterValue, float64(ctr.CloseErrs), "error")
		byHost := map[string]int{}
		for _, ss := range s.Snapshot() {
			byHost[ss.Host]++
		}
		for h, n := range byHost {
			ch <- prometheus.MustNewConstMetric(dSessHost, prometheus.GaugeValue, float64(n), h)
		}
	}
	if cc := m.src.Cache; cc != nil {
		st := cc.Stats()
		ch <- prometheus.MustNewConstMetric(dCacheHit, prometheus.CounterValue, float64(st.Hits))
		ch <- prometheus.MustNewConstMetric(dCacheMiss, prometheus.CounterValue, float64(st.Misses))
		ch <- prometheus.MustNewConstMetric(dCacheCoal, prometheus.CounterValue, float64(st.Coalesced))
		ch <- prometheus.MustNewConstMetric(dCacheEnt, prometheus.GaugeValue, float64(st.Entries))
		ch <- prometheus.MustNewConstMetric(dCacheRatio, prometheus.GaugeValue, st.HitRate)
	}
}
