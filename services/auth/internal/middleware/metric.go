package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/peltastic/payment-microservices-reference/auth/internal/metrics"
)

func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath() // /payments/:id not /payments/pay_01HXYZ — prevents cardinality explosion

		if path == "" {
			path = "unknown"
		}

		metrics.HTTPRequestsTotal.WithLabelValues(
			c.Request.Method,
			path,
			status,
		).Inc()

		metrics.HTTPRequestDuration.WithLabelValues(
			c.Request.Method,
			path,
		).Observe(duration)
	}
}
