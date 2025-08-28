package infra

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

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

func InitTelemetry(configuration TelemetryConfiguration, apiVersion string) (TelemetryRessources, error) {
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
			semconv.ServiceVersion(apiVersion),
		),
	)
	if err != nil {
		return TelemetryRessources{}, fmt.Errorf("resource.New error: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(MarbleSampler{SamplingMap: configuration.SamplingMap}),
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

type SpanKind int

const DEFAULT_SAMPLING_RATE = 0.3

const (
	SpanOther SpanKind = iota
	SpanHttpIngress
	SpanDatabaseQuery
)

var (
	routePrefixSampling = map[string]float64{
		"/health":           0.0,
		"/liveness":         0.0,
		"/version":          0.0,
		"/config":           0.0,
		"/is-sso-available": 0.0,
		"/signup-status":    0.0,
		"/decisions":        0.05,
		"/v1/decisions":     0.05,
		"/ingestion":        0.05,
		"/v1/ingest":        0.05,
	}
)

type MarbleSampler struct {
	SamplingMap TelemetrySamplingMap
}

func (MarbleSampler) Description() string {
	return "marble-sampler"
}

func (ms MarbleSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	var (
		kind     SpanKind
		value    string
		prob     float64                   = DEFAULT_SAMPLING_RATE
		decision sdktrace.SamplingDecision = sdktrace.Drop
	)

	psc := trace.SpanContextFromContext(p.ParentContext)

	// This span should not be sampled if the parent is not. Except for the root
	// span ID (the one that does not have a trace ID).
	if psc.HasTraceID() && !psc.IsSampled() {
		return sdktrace.NeverSample().ShouldSample(p)
	}

	for _, attr := range p.Attributes {
		if attr.Key == semconv.HTTPRouteKey {
			kind = SpanHttpIngress
			value = attr.Value.AsString()
			break
		}

		if attr.Key == semconv.DBStatementKey {
			kind = SpanDatabaseQuery
			value = attr.Value.AsString()
			break
		}
	}

rates:
	switch kind {
	case SpanHttpIngress:
		for prefix, prefixProb := range routePrefixSampling {
			if strings.HasPrefix(value, prefix) {
				prob = prefixProb
				break rates
			}
		}

	case SpanDatabaseQuery:
		if strings.HasPrefix(p.Name, "prepare ") {
			prob = 0.0
			break rates
		}
		if strings.Contains(value, "SELECT SET_CONFIG") {
			prob = 0.0
			break rates
		}
		if psc.IsSampled() {
			prob = 1.0
		}

	default:
		if ratio, ok := ms.SamplingMap.SpanNames[p.Name]; ok {
			prob = ratio
			break rates
		}

		prob = 1.0
	}

	traceId := binary.BigEndian.Uint64(p.TraceID[:8])

	if traceId < uint64(prob*float64(math.MaxUint64)) {
		decision = sdktrace.RecordAndSample
	}

	return sdktrace.SamplingResult{
		Decision:   decision,
		Attributes: p.Attributes,
		Tracestate: trace.SpanContextFromContext(p.ParentContext).TraceState(),
	}
}
