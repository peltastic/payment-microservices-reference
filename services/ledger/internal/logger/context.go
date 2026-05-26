package logger

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const loggerKey contextKey = "logger"

func WithContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

func FromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return WithTrace(ctx, log)
	}

	return WithTrace(ctx, slog.Default())
}

func WithTrace(ctx context.Context, log *slog.Logger) *slog.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return log
	}

	spanCtx := span.SpanContext()
	return log.With(
		"trace_id", spanCtx.TraceID().String(),
		"span_id", spanCtx.SpanID().String(),
	)
}
