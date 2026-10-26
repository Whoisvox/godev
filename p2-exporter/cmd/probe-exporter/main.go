package main

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var probeSuccess = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "probe_success",
		Help: "Whether the last probe succeeded.",
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

func init() {
	prometheus.MustRegister(probeSuccess, probeTotal)
}

func main() {
	probeSuccess.WithLabelValues("abab").Set(1)
	probeTotal.WithLabelValues("ok").Inc()

	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(":9115", nil)
	if err != nil {
		log.Fatal(err)
	}
}
