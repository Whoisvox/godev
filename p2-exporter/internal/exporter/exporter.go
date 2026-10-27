package exporter

import (
	"github.com/Whoisvox/godev/probe-exporter/internal/probe"
	"github.com/prometheus/client_golang/prometheus"
)

var probeSuccess = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "probe_success",
		Help: "Whether the last probe succeeded(statuscode < 499 and not eq 429).",
	},
	[]string{"name"},
)

var probeTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "probe_requests_total",
		Help: "Total probes by result.",
	},
	[]string{"result"},
)

var probeDuration = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "probe_duration_seconds",
		Help: "probe Duration seconds",
	},
	[]string{"name"},
)

type Metrics struct {
	ProbeSuccess  *prometheus.GaugeVec
	ProbeTotal    *prometheus.CounterVec
	ProbeDuration *prometheus.GaugeVec
}

func init() {
	prometheus.MustRegister(probeSuccess, probeTotal, probeDuration)
}

func NewMetrics() *Metrics {
	m := &Metrics{
		ProbeSuccess:  probeSuccess,
		ProbeTotal:    probeTotal,
		ProbeDuration: probeDuration,
	}
	return m
}

func (m *Metrics) Update(results []probe.Result) {
	for _, r := range results {
		m.ProbeSuccess.WithLabelValues(r.Name).Set(boolFloat(r.Success))
		m.ProbeDuration.WithLabelValues(r.Name).Set(r.Duration.Seconds())
		m.ProbeTotal.WithLabelValues(resultLabel(r)).Inc()
	}
}

func boolFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func resultLabel(r probe.Result) string {
	if r.Success {
		return "ok"
	}
	return "error"
}
