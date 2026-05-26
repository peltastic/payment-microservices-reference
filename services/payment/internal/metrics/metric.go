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

// Payment-specific metrics — only register in payment service
var (
	PaymentsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_reference_payments_total",
			Help: "Total number of payments processed",
		},
		[]string{"status", "currency"},
	)

	PaymentDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "payment_reference_payment_duration_seconds",
			Help:    "Time taken to process a payment end to end",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10},
		},
	)

	BankCallDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "payment_reference_bank_call_duration_seconds",
			Help:    "Time taken for the bank authorization call",
			Buckets: []float64{.05, .1, .25, .5, 1, 2, 5},
		},
	)

	BankCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_reference_bank_calls_total",
			Help: "Total bank authorization calls",
		},
		[]string{"result"}, // success, declined, unreachable
	)

	IdempotencyCacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "payment_reference_idempotency_cache_hits_total",
			Help: "Number of idempotency cache hits (duplicate requests caught)",
		},
	)

	KafkaPublishTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_reference_kafka_publish_total",
			Help: "Total Kafka messages published",
		},
		[]string{"topic", "status"}, // status: success, failed
	)

	RecoveryJobRuns = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "payment_reference_recovery_job_runs_total",
			Help: "Number of times the recovery job has run",
		},
	)

	RecoveryJobEvents = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "payment_reference_recovery_job_events_total",
			Help: "Number of events recovered by the recovery job",
		},
	)
)
