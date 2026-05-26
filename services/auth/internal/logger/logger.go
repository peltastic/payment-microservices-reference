package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/peltastic/payment-microservices-reference/auth/internal/config"
)

type defaultFieldsHandler struct {
	next     slog.Handler
	defaults []slog.Attr
	attrs    []slog.Attr
}

func Init(cfg config.LoggerConfig) {
	level := slog.LevelInfo
	if strings.EqualFold(cfg.Level, "debug") {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.MessageKey:
				attr.Key = "message"
			case slog.LevelKey:
				attr.Value = slog.StringValue(strings.ToLower(attr.Value.String()))
			}
			return attr
		},
	})

	slog.SetDefault(slog.New(&defaultFieldsHandler{
		next: handler,
		defaults: []slog.Attr{
			slog.String("service", cfg.ServiceName),
			slog.String("env", cfg.Environment),
			slog.String("request_id", ""),
			slog.String("merchant_id", ""),
			slog.Int64("duration_ms", 0),
			slog.String("component", ""),
			slog.String("trace_id", ""),
			slog.String("span_id", ""),
		},
	}))
}

func (h *defaultFieldsHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *defaultFieldsHandler) Handle(ctx context.Context, record slog.Record) error {
	seen := map[string]bool{}
	for _, attr := range h.attrs {
		seen[attr.Key] = true
	}
	record.Attrs(func(attr slog.Attr) bool {
		seen[attr.Key] = true
		return true
	})

	for _, attr := range h.defaults {
		if !seen[attr.Key] {
			record.AddAttrs(attr)
		}
	}

	return h.next.Handle(ctx, record)
}

func (h *defaultFieldsHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := h.next.WithAttrs(attrs)
	allAttrs := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	allAttrs = append(allAttrs, h.attrs...)
	allAttrs = append(allAttrs, attrs...)

	return &defaultFieldsHandler{
		next:     next,
		defaults: h.defaults,
		attrs:    allAttrs,
	}
}

func (h *defaultFieldsHandler) WithGroup(name string) slog.Handler {
	return &defaultFieldsHandler{
		next:     h.next.WithGroup(name),
		defaults: h.defaults,
		attrs:    h.attrs,
	}
}
