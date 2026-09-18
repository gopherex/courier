package app

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
)

func telemetry(ctx context.Context) (func(context.Context) error, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("metrics exporter: %w", err)
	}

	res := resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("courier"))
	metrics := metric.NewMeterProvider(metric.WithReader(exporter), metric.WithResource(res))
	otel.SetMeterProvider(metrics)

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		return metrics.Shutdown, nil
	}

	traces, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("trace exporter: %w", err)
	}

	provider := trace.NewTracerProvider(trace.WithBatcher(traces), trace.WithResource(res))
	otel.SetTracerProvider(provider)

	return func(ctx context.Context) error {
		if shutdownErr := provider.Shutdown(ctx); shutdownErr != nil {
			return fmt.Errorf("trace shutdown: %w", shutdownErr)
		}

		if shutdownErr := metrics.Shutdown(ctx); shutdownErr != nil {
			return fmt.Errorf("metric shutdown: %w", shutdownErr)
		}

		return nil
	}, nil
}
