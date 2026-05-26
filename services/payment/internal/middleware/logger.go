package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/peltastic/payment-microservices-reference/payment/internal/logger"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		requestID := c.GetHeader("x-request-id")
		merchantID := c.GetHeader("x-merchant-id")

		baseLog := slog.Default().With(
			"request_id", requestID,
			"merchant_id", merchantID,
			"method", c.Request.Method,
			"path", c.FullPath(),
		)

		ctx := logger.WithContext(c.Request.Context(), baseLog)
		c.Request = c.Request.WithContext(ctx)
		log := logger.FromContext(ctx)
		c.Set("logger", log)

		log.Info("request started")

		c.Next()

		log.Info("request completed",
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes_out", c.Writer.Size(),
		)
	}
}
