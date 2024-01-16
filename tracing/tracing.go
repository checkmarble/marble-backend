package tracing

import (
	"context"
	"fmt"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	gcppropagator "github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type Configuration struct {
	Enabled         bool
	ApplicationName string
	ProjectID       string
}

func Init(configuration Configuration) (trace.Tracer, error) {
	if !configuration.Enabled {
		return &noop.Tracer{}, nil
	}

	exporter, err := texporter.New(texporter.WithProjectID(configuration.ProjectID))
	if err != nil {
		return nil, fmt.Errorf("texporter.New error: %v", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(configuration.ApplicationName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("resource.New error: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			gcppropagator.CloudTraceFormatPropagator{},
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	otel.SetTracerProvider(tp)

	tracer := tp.Tracer(configuration.ApplicationName)
	return tracer, nil

}
