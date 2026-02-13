package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/networkengineer-cloud/build-app/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

// mockTelemetry provides a no-op telemetry for testing
type mockTelemetry struct{}

func (m *mockTelemetry) Tracer() trace.Tracer {
	return otel.Tracer("test")
}

func (m *mockTelemetry) WebhookCounter() metric.Int64Counter {
	return noop.Int64Counter{}
}

func (m *mockTelemetry) BuildDuration() metric.Float64Histogram {
	return noop.Float64Histogram{}
}

func (m *mockTelemetry) BuildCounter() metric.Int64Counter {
	return noop.Int64Counter{}
}

func TestValidateSignature(t *testing.T) {
	secret := "test-secret"
	cfg := &config.Config{
		WebhookSecret: secret,
		GitHubToken:   "test-token",
	}
	handler := NewHandler(cfg, &mockTelemetry{})

	tests := []struct {
		name      string
		body      string
		secret    string
		wantValid bool
	}{
		{
			name:      "valid signature",
			body:      `{"test":"data"}`,
			secret:    secret,
			wantValid: true,
		},
		{
			name:      "invalid signature",
			body:      `{"test":"data"}`,
			secret:    "wrong-secret",
			wantValid: false,
		},
		{
			name:      "empty signature",
			body:      `{"test":"data"}`,
			secret:    "",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(tt.body)

			// Generate signature
			var signature string
			if tt.secret != "" {
				mac := hmac.New(sha256.New, []byte(tt.secret))
				mac.Write(body)
				signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
			}

			valid := handler.validateSignature(body, signature)
			if valid != tt.wantValid {
				t.Errorf("validateSignature() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

func TestHandleWebhook_Method(t *testing.T) {
	cfg := &config.Config{
		WebhookSecret: "test-secret",
		GitHubToken:   "test-token",
	}
	handler := NewHandler(cfg, &mockTelemetry{})

	tests := []struct {
		name       string
		method     string
		wantStatus int
	}{
		{
			name:       "POST method",
			method:     http.MethodPost,
			wantStatus: http.StatusUnauthorized, // Will fail signature check but pass method check
		},
		{
			name:       "GET method not allowed",
			method:     http.MethodGet,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "PUT method not allowed",
			method:     http.MethodPut,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/webhook", nil)
			w := httptest.NewRecorder()

			handler.HandleWebhook(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("HandleWebhook() status = %v, want %v", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestHandleWebhook_UnsupportedEvent(t *testing.T) {
	cfg := &config.Config{
		WebhookSecret: "test-secret",
		GitHubToken:   "test-token",
	}
	handler := NewHandler(cfg, &mockTelemetry{})

	body := []byte(`{}`)
	mac := hmac.New(sha256.New, []byte(cfg.WebhookSecret))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "unsupported")

	w := httptest.NewRecorder()
	handler.HandleWebhook(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("HandleWebhook() status = %v, want %v", resp.StatusCode, http.StatusOK)
	}
}
