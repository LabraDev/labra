package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type systemServicesTestAIProvider struct{}

func (systemServicesTestAIProvider) Generate(_ context.Context, _ AIProviderInput) (AIProviderOutput, error) {
	return AIProviderOutput{Text: "ok", Provider: "test", Model: "test-model", Confidence: "medium"}, nil
}

func TestGetSystemServicesHandler_UsesLiveProbes(t *testing.T) {
	prevReadiness := readinessProbe
	prevStore := appStore
	prevWebhookSecret := githubWebhookSecret
	prevWebhookSkew := webhookMaxSkewSeconds
	prevAIEnabled := aiEnabled
	prevAIDisabled := aiDisabled
	prevAIProvider := aiProvider
	prevAIProviderModel := aiProviderModel
	prevAIProviderTimeout := aiProviderTimeout

	t.Cleanup(func() {
		readinessProbe = prevReadiness
		appStore = prevStore
		githubWebhookSecret = prevWebhookSecret
		webhookMaxSkewSeconds = prevWebhookSkew
		aiEnabled = prevAIEnabled
		aiDisabled = prevAIDisabled
		aiProvider = prevAIProvider
		aiProviderModel = prevAIProviderModel
		aiProviderTimeout = prevAIProviderTimeout
	})

	readinessProbe = func(context.Context) error { return nil }
	appStore = nil
	githubWebhookSecret = "test-secret"
	webhookMaxSkewSeconds = 300
	aiEnabled = true
	aiDisabled = false
	aiProvider = systemServicesTestAIProvider{}
	aiProviderModel = "us.amazon.nova-lite-v1:0"
	aiProviderTimeout = time.Second

	req := httptest.NewRequest(http.MethodGet, "/v1/system/services", nil)
	rr := httptest.NewRecorder()
	GetSystemServicesHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var body struct {
		Services []ServiceStatus `json:"services"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal services response: %v", err)
	}

	statusByName := make(map[string]string, len(body.Services))
	for _, svc := range body.Services {
		statusByName[svc.Name] = svc.Status
	}

	if got := statusByName["control-api"]; got != "healthy" {
		t.Fatalf("expected control-api healthy, got %q", got)
	}
	if got := statusByName["deploy-orchestrator"]; got != "down" {
		t.Fatalf("expected deploy-orchestrator down when store missing, got %q", got)
	}
	if got := statusByName["webhook-ingestor"]; got != "down" {
		t.Fatalf("expected webhook-ingestor down when store missing, got %q", got)
	}
	if got := statusByName["ai-assistant"]; got != "healthy" {
		t.Fatalf("expected ai-assistant healthy, got %q", got)
	}
}

func TestGetSystemServicesHandler_ControlAPIProbeFailureMarksDown(t *testing.T) {
	prevReadiness := readinessProbe
	t.Cleanup(func() {
		readinessProbe = prevReadiness
	})

	readinessProbe = func(context.Context) error { return fmt.Errorf("db unavailable") }

	req := httptest.NewRequest(http.MethodGet, "/v1/system/services", nil)
	rr := httptest.NewRecorder()
	GetSystemServicesHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var body struct {
		Services []ServiceStatus `json:"services"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal services response: %v", err)
	}

	for _, svc := range body.Services {
		if svc.Name == "control-api" {
			if svc.Status != "down" {
				t.Fatalf("expected control-api down when readiness probe fails, got %q", svc.Status)
			}
			return
		}
	}
	t.Fatalf("control-api service not found in response")
}
