package telemetry

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Telemetry holds the OpenTelemetry providers and meters
type Telemetry struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	tracer         trace.Tracer
	meter          metric.Meter
	
	// Metrics
	buildDuration    metric.Float64Histogram
	buildCounter     metric.Int64Counter
	webhookCounter   metric.Int64Counter
	httpDuration     metric.Float64Histogram
}

// Config holds telemetry configuration
type Config struct {
	ServiceName     string
	ServiceVersion  string
	OTLPEndpoint    string
	TracesEnabled   bool
	MetricsEnabled  bool
}

// Initialize sets up OpenTelemetry with the provided configuration
func Initialize(ctx context.Context, cfg Config) (*Telemetry, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tel := &Telemetry{}

	// Initialize tracer provider
	if cfg.TracesEnabled && cfg.OTLPEndpoint != "" {
		traceExporter, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
			otlptracehttp.WithInsecure(), // Use WithTLSClientConfig for production
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create trace exporter: %w", err)
		}

		tel.tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tel.tracerProvider)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		
		tel.tracer = tel.tracerProvider.Tracer("github.com/networkengineer-cloud/build-app")
		log.Printf("OpenTelemetry traces initialized with endpoint: %s", cfg.OTLPEndpoint)
	} else {
		// Use no-op tracer if disabled
		tel.tracer = otel.Tracer("github.com/networkengineer-cloud/build-app")
		log.Println("OpenTelemetry traces disabled")
	}

	// Initialize meter provider
	if cfg.MetricsEnabled && cfg.OTLPEndpoint != "" {
		metricExporter, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint),
			otlpmetrichttp.WithInsecure(), // Use WithTLSClientConfig for production
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create metric exporter: %w", err)
		}

		tel.meterProvider = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(tel.meterProvider)
		
		tel.meter = tel.meterProvider.Meter("github.com/networkengineer-cloud/build-app")
		
		// Create metrics
		if err := tel.createMetrics(); err != nil {
			return nil, fmt.Errorf("failed to create metrics: %w", err)
		}
		
		log.Printf("OpenTelemetry metrics initialized with endpoint: %s", cfg.OTLPEndpoint)
	} else {
		// Use no-op meter if disabled
		tel.meter = otel.Meter("github.com/networkengineer-cloud/build-app")
		log.Println("OpenTelemetry metrics disabled")
	}

	return tel, nil
}

// createMetrics initializes all application metrics
func (t *Telemetry) createMetrics() error {
	var err error
	
	// Build duration histogram
	t.buildDuration, err = t.meter.Float64Histogram(
		"build.duration",
		metric.WithDescription("Duration of container builds in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create build duration metric: %w", err)
	}

	// Build counter
	t.buildCounter, err = t.meter.Int64Counter(
		"build.count",
		metric.WithDescription("Number of builds executed"),
	)
	if err != nil {
		return fmt.Errorf("failed to create build counter metric: %w", err)
	}

	// Webhook counter
	t.webhookCounter, err = t.meter.Int64Counter(
		"webhook.count",
		metric.WithDescription("Number of webhooks processed"),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook counter metric: %w", err)
	}

	// HTTP duration histogram
	t.httpDuration, err = t.meter.Float64Histogram(
		"http.server.duration",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP duration metric: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the telemetry providers
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error

	if t.tracerProvider != nil {
		if err := t.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown tracer provider: %w", err))
		}
	}

	if t.meterProvider != nil {
		if err := t.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown meter provider: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// Tracer returns the tracer
func (t *Telemetry) Tracer() trace.Tracer {
	return t.tracer
}

// WebhookCounter returns the webhook counter
func (t *Telemetry) WebhookCounter() metric.Int64Counter {
	return t.webhookCounter
}

// BuildDuration returns the build duration histogram
func (t *Telemetry) BuildDuration() metric.Float64Histogram {
	return t.buildDuration
}

// BuildCounter returns the build counter
func (t *Telemetry) BuildCounter() metric.Int64Counter {
	return t.buildCounter
}

// RecordBuildDuration records the duration of a build operation
func (t *Telemetry) RecordBuildDuration(ctx context.Context, duration time.Duration, success bool, buildType string) {
	if t.buildDuration == nil {
		return
	}
	
	status := "success"
	if !success {
		status = "failure"
	}
	
	t.buildDuration.Record(ctx, duration.Seconds(),
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("build_type", buildType),
		),
	)
}

// IncrementBuildCounter increments the build counter
func (t *Telemetry) IncrementBuildCounter(ctx context.Context, success bool, buildType string) {
	if t.buildCounter == nil {
		return
	}
	
	status := "success"
	if !success {
		status = "failure"
	}
	
	t.buildCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
			attribute.String("build_type", buildType),
		),
	)
}

// IncrementWebhookCounter increments the webhook counter
func (t *Telemetry) IncrementWebhookCounter(ctx context.Context, event string, success bool) {
	if t.webhookCounter == nil {
		return
	}
	
	status := "success"
	if !success {
		status = "failure"
	}
	
	t.webhookCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("event", event),
			attribute.String("status", status),
		),
	)
}
