package telemetry

import (
	"context"
	"fmt"
	"net/url"

	"github.com/peltastic/payment-microservices-reference/auth/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func InitTracer(ctx context.Context, cfg config.TelemetryConfig) (func(context.Context) error, error) {
	options := traceExporterOptions(cfg.OTLPEndpoint)
	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

func traceExporterOptions(endpoint string) []otlptracehttp.Option {
	options := []otlptracehttp.Option{otlptracehttp.WithInsecure()}
	if endpoint == "" {
		return options
	}

	parsed, err := url.Parse(endpoint)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		if parsed.Path == "" || parsed.Path == "/" {
			parsed.Path = "/v1/traces"
		}

		options = []otlptracehttp.Option{otlptracehttp.WithEndpointURL(parsed.String())}
		if parsed.Scheme == "http" {
			options = append(options, otlptracehttp.WithInsecure())
		}
		return options
	}

	return append(options, otlptracehttp.WithEndpoint(endpoint))
}
