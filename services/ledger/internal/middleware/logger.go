package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	appLogger "github.com/peltastic/payment-microservices-reference/ledger/internal/logger"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		baseLog := slog.Default().With(
			"request_id", c.GetHeader("x-request-id"),
			"merchant_id", c.GetHeader("x-merchant-id"),
			"method", c.Request.Method,
			"path", path,
			"client_ip", c.ClientIP(),
		)

		ctx := appLogger.WithContext(c.Request.Context(), baseLog)
		c.Request = c.Request.WithContext(ctx)
		log := appLogger.FromContext(ctx)
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
