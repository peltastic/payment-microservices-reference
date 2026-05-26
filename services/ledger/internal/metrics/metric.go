package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics — applies to all services
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "path"},
	)
)

// Ledger-specific metrics
var (
	JournalEntriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_reference_journal_entries_total",
			Help: "Total journal entries created",
		},
		[]string{"entry_type"}, // debit, credit
	)

	KafkaConsumeTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_reference_kafka_consume_total",
			Help: "Total Kafka messages consumed",
		},
		[]string{"topic", "status"}, // status: success, failed
	)

	KafkaConsumerLag = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payment_reference_kafka_consumer_lag",
			Help: "Current Kafka consumer lag",
		},
		[]string{"topic"},
	)
)
