package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/networkengineer-cloud/build-app/internal/config"
	"github.com/networkengineer-cloud/build-app/internal/telemetry"
	"github.com/networkengineer-cloud/build-app/internal/webhook"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize telemetry
	telConfig := telemetry.Config{
		ServiceName:     getEnv("OTEL_SERVICE_NAME", "build-app"),
		ServiceVersion:  getEnv("SERVICE_VERSION", "1.0.0"),
		OTLPEndpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		TracesEnabled:   getEnvBool("OTEL_TRACES_ENABLED", true),
		MetricsEnabled:  getEnvBool("OTEL_METRICS_ENABLED", true),
	}

	tel, err := telemetry.Initialize(ctx, telConfig)
	if err != nil {
		log.Fatalf("Failed to initialize telemetry: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tel.Shutdown(shutdownCtx); err != nil {
			log.Printf("Failed to shutdown telemetry: %v", err)
		}
	}()

	// Create webhook handler with telemetry
	handler := webhook.NewHandler(cfg, tel)

	// Setup HTTP server with Otel instrumentation
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", handler.HandleWebhook)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap mux with OpenTelemetry instrumentation
	wrappedHandler := otelhttp.NewHandler(mux, "build-app")

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      wrappedHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting webhook server on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "1"
}
