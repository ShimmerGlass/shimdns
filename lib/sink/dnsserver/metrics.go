package dnsserver

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricResponseTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "shimdns_dnsserver_response_time_seconds",
		Help:    "Integrated DNS server response time",
		Buckets: prometheus.ExponentialBucketsRange(time.Microsecond.Seconds(), time.Second.Seconds(), 64),
	}, []string{"sink_id", "type"})
)
