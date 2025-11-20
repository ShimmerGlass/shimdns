package prov

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricSourceStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "shimdns_source_status",
		Help:      "1=Success 0=Failed",
	}, []string{"name"})

	metricSourceRecords = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "shimdns_source_records",
		Help:      "Number of records fetched by source",
	}, []string{"name"})

	metricSourceFetchTime = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "shimdns_source_fetch_time_seconds",
		Help:      "Time taken fetching the source",
	}, []string{"name"})

	metricModifierStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "shimdns_modifier_status",
		Help:      "1=Success 0=Failed",
	}, []string{"name"})

	metricModifierApplyTime = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "shimdns_modifier_apply_time_seconds",
		Help:      "Time taken applying the modifier",
	}, []string{"name"})

	metricSinkStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "shimdns_sink_status",
		Help:      "1=Success 0=Failed",
	}, []string{"name"})

	metricSinkWriteTime = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "shimdns_sink_write_time_seconds",
		Help:      "Time taken writing to the sink",
	}, []string{"name"})
)
