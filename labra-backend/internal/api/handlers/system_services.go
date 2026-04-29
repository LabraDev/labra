package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ServiceStatus is what we return per service in the services endpoint
type ServiceStatus struct {
	Name        string `json:"name"`
	Tier        string `json:"tier"`
	Mode        string `json:"mode"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// serviceDefinition describes a logical service and how to check if it's healthy
type serviceDefinition struct {
	Name        string
	Tier        string
	Mode        string
	Description string
	Probe       func(context.Context) (status string, details string)
}

// serviceCatalog is the list of all services we track - add new ones here
var serviceCatalog = []serviceDefinition{
	{
		Name:        "control-api",
		Tier:        "api",
		Mode:        "in-process",
		Description: "User-facing API gateway and metadata endpoints",
		Probe:       probeControlAPI,
	},
	{
		Name:        "deploy-orchestrator",
		Tier:        "worker",
		Mode:        "in-process",
		Description: "Deployment queueing and execution orchestration",
		Probe:       probeDeployOrchestrator,
	},
	{
		Name:        "webhook-ingestor",
		Tier:        "ingestion",
		Mode:        "in-process",
		Description: "GitHub webhook normalization and routing",
		Probe:       probeWebhookIngestor,
	},
	{
		Name:        "ai-assistant",
		Tier:        "ai",
		Mode:        "in-process",
		Description: "AI deployment insight generation",
		Probe:       probeAIAssistant,
	},
}

// GetSystemServicesHandler handles GET /v1/system/services
// returns status of all internal services
func GetSystemServicesHandler(w http.ResponseWriter, r *http.Request) {
	allServiceStatuses := collectServiceStatuses(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"services": allServiceStatuses,
		"architecture": map[string]any{
			"pattern": "3-tier + microservice control-plane",
			"tiers":   []string{"frontend", "api", "metadata"},
		},
	})
}

// GetReadinessChecklistHandler handles GET /v1/system/readiness
// returns a checklist of things that need to be configured for the system to work properly
func GetReadinessChecklistHandler(w http.ResponseWriter, _ *http.Request) {
	// each check has a control name, a bool status, and a description
	readinessChecks := []map[string]any{
		{
			"control": "webhook_replay_window_configured",
			"status":  webhookMaxSkewSeconds > 0,
			"details": "Webhook timestamp replay window is enabled",
		},
		{
			"control": "webhook_secret_configured",
			"status":  strings.TrimSpace(githubWebhookSecret) != "",
			"details": "GitHub webhook secret must be configured",
		},
		{
			"control": "ai_prompt_version_present",
			"status":  strings.TrimSpace(aiPromptVersion) != "",
			"details": "AI prompt versioning is configured",
		},
		{
			"control": "ai_provider_timeout_configured",
			"status":  aiProviderTimeout > 0,
			"details": "AI provider timeout protects against hanging requests",
		},
		{
			"control": "ai_fallback_response_enabled",
			"status":  true,
			"details": "Fallback insight path is available when AI provider calls fail",
		},
		{
			"control": "service_inventory_includes_ai",
			"status":  serviceExists("ai-assistant"),
			"details": "Service status includes AI assistant component",
		},
	}

	// overall ready only if all individual checks pass
	overallReady := true
	for _, checkItem := range readinessChecks {
		if statusValue, ok := checkItem["status"].(bool); !ok || !statusValue {
			overallReady = false
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ready":  overallReady,
		"checks": readinessChecks,
	})
}

// serviceExists checks if a service with the given name is in the catalog
func serviceExists(serviceName string) bool {
	normalizedSearchName := strings.TrimSpace(strings.ToLower(serviceName))
	if normalizedSearchName == "" {
		return false
	}
	for _, catalogEntry := range serviceCatalog {
		if strings.TrimSpace(strings.ToLower(catalogEntry.Name)) == normalizedSearchName {
			return true
		}
	}
	return false
}

// collectServiceStatuses runs all the probes and returns the results
// each probe gets 750ms before we consider it timed out
func collectServiceStatuses(parentCtx context.Context) []ServiceStatus {
	statusResults := make([]ServiceStatus, 0, len(serviceCatalog))
	for _, serviceEntry := range serviceCatalog {
		probeStatusResult := "degraded"
		probeDetailsResult := "probe unavailable"
		if serviceEntry.Probe != nil {
			// short timeout per probe so we don't block the whole endpoint
			probeCtx, cancelProbe := context.WithTimeout(parentCtx, 750*time.Millisecond)
			probeStatusResult, probeDetailsResult = serviceEntry.Probe(probeCtx)
			cancelProbe()
		}
		if probeStatusResult == "" {
			probeStatusResult = "degraded"
		}
		// combine the base description with probe details if we got any
		combinedDescription := serviceEntry.Description
		if trimmedDetails := strings.TrimSpace(probeDetailsResult); trimmedDetails != "" {
			combinedDescription = fmt.Sprintf("%s (%s)", serviceEntry.Description, trimmedDetails)
		}
		statusResults = append(statusResults, ServiceStatus{
			Name:        serviceEntry.Name,
			Tier:        serviceEntry.Tier,
			Mode:        serviceEntry.Mode,
			Status:      probeStatusResult,
			Description: combinedDescription,
		})
	}
	return statusResults
}

// probeControlAPI checks if the api itself is healthy by running the readiness probe
func probeControlAPI(ctx context.Context) (string, string) {
	if readinessProbe == nil {
		return "degraded", "readiness probe not configured"
	}
	if err := readinessProbe(ctx); err != nil {
		return "down", "readiness probe failed"
	}
	return "healthy", "request handling ready"
}

// probeDeployOrchestrator checks if deployments can be persisted
func probeDeployOrchestrator(ctx context.Context) (string, string) {
	if appStore == nil {
		return "down", "store not initialized"
	}
	if readinessProbe == nil {
		return "degraded", "readiness probe not configured"
	}
	if err := readinessProbe(ctx); err != nil {
		return "down", "queue persistence unavailable"
	}
	return "healthy", "queue persistence available"
}

// probeWebhookIngestor checks if webhooks can be received and processed
func probeWebhookIngestor(ctx context.Context) (string, string) {
	if appStore == nil {
		return "down", "store not initialized"
	}
	// webhook secret must be set or we can't verify incoming payloads
	if strings.TrimSpace(githubWebhookSecret) == "" {
		return "down", "webhook secret missing"
	}
	if readinessProbe != nil {
		if err := readinessProbe(ctx); err != nil {
			return "down", "webhook persistence unavailable"
		}
	}
	// replay window being 0 is a security concern but not a hard failure
	if webhookMaxSkewSeconds <= 0 {
		return "degraded", "replay window disabled"
	}
	return "healthy", "signature and replay checks active"
}

// probeAIAssistant checks if the ai provider is configured and ready
func probeAIAssistant(_ context.Context) (string, string) {
	if aiProvider == nil {
		return "down", "provider not configured"
	}
	if aiProviderTimeout <= 0 {
		return "down", "provider timeout not configured"
	}
	if strings.TrimSpace(aiProviderModel) == "" {
		return "degraded", "provider model missing"
	}
	if !aiEnabled {
		return "degraded", "feature disabled"
	}
	if aiDisabled {
		// ai disabled flag is set - consciously turned off, not a crash
		return "degraded", "ai disabled"
	}
	return "healthy", "provider configured"
}
