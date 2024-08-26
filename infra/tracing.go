package infra

import (
	"context"
	"fmt"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	gcppropagator "github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator"
	"google.golang.org/api/option"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type TelemetryRessources struct {
	TracerProvider    trace.TracerProvider
	Tracer            trace.Tracer
	TextMapPropagator propagation.TextMapPropagator
}

func NoopTelemetry() TelemetryRessources {
	return TelemetryRessources{
		TracerProvider:    noop.NewTracerProvider(),
		Tracer:            &noop.Tracer{},
		TextMapPropagator: nil,
	}
}

func InitTelemetry(configuration TelemetryConfiguration) (TelemetryRessources, error) {
	if !configuration.Enabled {
		return NoopTelemetry(), nil
	}

	exporter, err := texporter.New(
		texporter.WithProjectID(configuration.ProjectID), // If empty (env variable GOOGLE_CLOUD_PROJECT not set), it will try to determine the project id from the GCP metadata server
		texporter.WithTraceClientOptions([]option.ClientOption{option.WithTelemetryDisabled()}),
	)
	if err != nil {
		return TelemetryRessources{}, fmt.Errorf("texporter.New error: %w", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(configuration.ApplicationName),
		),
	)
	if err != nil {
		return TelemetryRessources{}, fmt.Errorf("resource.New error: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	tracer := tp.Tracer(configuration.ApplicationName)
	return TelemetryRessources{
		TracerProvider: tp,
		Tracer:         tracer,
		TextMapPropagator: propagation.NewCompositeTextMapPropagator(
			gcppropagator.CloudTraceFormatPropagator{},
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	}, nil
}
