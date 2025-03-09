package instrumentation

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type ShutdownFunction func(context.Context) error

type ShutdownFunctions struct {
	Tracer  ShutdownFunction
	Metrics ShutdownFunction
}

func (s *ShutdownFunctions) Shutdown(ctx context.Context) error {
	var err error
	if s.Tracer != nil {
		err = errors.Join(err, s.Tracer(ctx))
	}
	if s.Metrics != nil {
		err = errors.Join(err, s.Metrics(ctx))
	}
	return err
}

func newExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	return otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
}

func InitTracer(ctx context.Context, res *resource.Resource) (ShutdownFunction, error) {
	exporter, err := newExporter(ctx)
	if err != nil {
		return nil, err
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	return tracerProvider.Shutdown, nil
}

func InitMetrics(ctx context.Context, res *resource.Resource) (ShutdownFunction, error) {
	metricExporter, err := stdoutmetric.New()
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			// Default is 1m. Set to 3s for demonstrative purposes.
			metric.WithInterval(3*time.Second))),
	)

	otel.SetMeterProvider(meterProvider)
	return meterProvider.Shutdown, nil
}

func Init(ctx context.Context) (*ShutdownFunctions, error) {
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("chainkills"),
		),
	)
	if err != nil {
		return nil, err
	}

	tracerShutdown, err := InitTracer(ctx, r)
	if err != nil {
		return nil, err
	}

	return &ShutdownFunctions{
		Tracer: tracerShutdown,
	}, nil

}
