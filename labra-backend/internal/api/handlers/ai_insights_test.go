package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"labra-backend/internal/api/store"
)

type mockAIProvider struct {
	lastPrompt string
	fail       bool
}

func (p *mockAIProvider) Generate(_ context.Context, in AIProviderInput) (AIProviderOutput, error) {
	p.lastPrompt = in.Prompt
	if p.fail {
		return AIProviderOutput{}, fmt.Errorf("provider failed")
	}
	return AIProviderOutput{
		Text:       "AI summary: deployment looks healthy.",
		Provider:   "test-provider",
		Model:      "test-model",
		Confidence: "medium",
	}, nil
}

func TestAIInsightsFlow(t *testing.T) {
	db := setupAIInsightsTestDB(t)

	prevStore := appStore
	prevEnabled := aiEnabled
	prevDisabled := aiDisabled
	prevPromptVersion := aiPromptVersion
	prevProviderModel := aiProviderModel
	prevTimeout := aiProviderTimeout
	prevRetries := aiProviderRetries
	prevProvider := aiProvider
	prevSecret := githubWebhookSecret
	t.Cleanup(func() {
		appStore = prevStore
		aiEnabled = prevEnabled
		aiDisabled = prevDisabled
		aiPromptVersion = prevPromptVersion
		aiProviderModel = prevProviderModel
		aiProviderTimeout = prevTimeout
		aiProviderRetries = prevRetries
		aiProvider = prevProvider
		githubWebhookSecret = prevSecret
		_ = db.Close()
	})

	InitAppStore(db)
	InitWebhook("ai-test-secret")

	provider := &mockAIProvider{}
	InitAIRuntime(AIRuntimeConfig{
		AIEnabled:       true,
		DisableAI:       false,
		PromptVersion:   "v2",
		ProviderModel:   "test-model",
		ProviderTimeout: 1500 * time.Millisecond,
		ProviderRetries: 1,
		Provider:        provider,
	})

	dep := seedAITestDeployment(t, 55)

	t.Run("ai insight succeeds with redaction and logs", func(t *testing.T) {
		payload := []byte(`{"deployment_id":` + strconv.FormatInt(dep.ID, 10) + `,"prompt":"review failure for casey@example.com token=abc123XYZ"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/deploy-insights", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req = withTestPrincipal(req, 55)
		rr := httptest.NewRecorder()
		PostAIDeployInsightsHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
		}

		if strings.Contains(provider.lastPrompt, "casey@example.com") {
			t.Fatalf("expected email redacted, got %q", provider.lastPrompt)
		}
		if strings.Contains(strings.ToLower(provider.lastPrompt), "token=abc123xyz") {
			t.Fatalf("expected token redacted, got %q", provider.lastPrompt)
		}

		var body struct {
			Source        string `json:"source"`
			FallbackUsed  bool   `json:"fallback_used"`
			PromptVersion string `json:"prompt_version"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal ai insight response: %v", err)
		}
		if body.Source != "test-provider" {
			t.Fatalf("expected provider source, got %q", body.Source)
		}
		if body.FallbackUsed {
			t.Fatalf("expected fallback_used=false")
		}
		if body.PromptVersion != "v2" {
			t.Fatalf("expected prompt_version v2, got %q", body.PromptVersion)
		}

		logsReq := httptest.NewRequest(http.MethodGet, "/v1/ai/requests?limit=10", nil)
		logsReq = withTestPrincipal(logsReq, 55)
		logsRR := httptest.NewRecorder()
		GetAIRequestLogsHandler(logsRR, logsReq)
		if logsRR.Code != http.StatusOK {
			t.Fatalf("expected ai logs 200, got %d body=%s", logsRR.Code, logsRR.Body.String())
		}
		var logsBody struct {
			Count int `json:"count"`
			Logs  []struct {
				InputRedacted bool `json:"input_redacted"`
			} `json:"logs"`
		}
		if err := json.Unmarshal(logsRR.Body.Bytes(), &logsBody); err != nil {
			t.Fatalf("unmarshal ai logs response: %v", err)
		}
		if logsBody.Count == 0 || len(logsBody.Logs) == 0 {
			t.Fatalf("expected at least one ai log entry")
		}
		if !logsBody.Logs[0].InputRedacted {
			t.Fatalf("expected input_redacted=true for sensitive prompt")
		}
	})

	t.Run("provider failure falls back safely", func(t *testing.T) {
		provider.fail = true
		payload := []byte(`{"deployment_id":` + strconv.FormatInt(dep.ID, 10) + `,"prompt":"why failed"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/ai/deploy-insights", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req = withTestPrincipal(req, 55)
		rr := httptest.NewRecorder()
		PostAIDeployInsightsHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 fallback, got %d body=%s", rr.Code, rr.Body.String())
		}
		var body struct {
			Source       string `json:"source"`
			FallbackUsed bool   `json:"fallback_used"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal fallback response: %v", err)
		}
		if body.Source != "fallback" || !body.FallbackUsed {
			t.Fatalf("expected fallback response, got %+v", body)
		}
		provider.fail = false
	})

	t.Run("readiness checklist reports hardening controls", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/system/readiness-checklist", nil)
		rr := httptest.NewRecorder()
		GetReadinessChecklistHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected readiness 200, got %d body=%s", rr.Code, rr.Body.String())
		}
		var body struct {
			Ready  bool `json:"ready"`
			Checks []struct {
				Control string `json:"control"`
				Status  bool   `json:"status"`
			} `json:"checks"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal readiness response: %v", err)
		}
		if !body.Ready {
			t.Fatalf("expected readiness checks to be ready=true")
		}
		if len(body.Checks) < 4 {
			t.Fatalf("expected readiness checks to include hardening controls")
		}
	})
}

func setupAIInsightsTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db := openInMemorySQLite(t)

	schema := `
	CREATE TABLE IF NOT EXISTS deployments (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  app_id INTEGER NOT NULL,
	  user_id INTEGER NOT NULL,
	  status TEXT NOT NULL,
	  trigger_type TEXT NOT NULL,
	  commit_sha TEXT,
	  commit_message TEXT,
	  commit_author TEXT,
	  branch TEXT,
	  site_url TEXT,
	  failure_reason TEXT,
	  correlation_id TEXT,
	  created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	  updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
	  started_at INTEGER,
	  finished_at INTEGER
	);
	CREATE TABLE IF NOT EXISTS deployment_logs (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  deployment_id INTEGER NOT NULL,
	  log_level TEXT NOT NULL,
	  message TEXT NOT NULL,
	  created_at INTEGER NOT NULL DEFAULT (unixepoch())
	);
	CREATE TABLE IF NOT EXISTS ai_request_logs (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  user_id INTEGER NOT NULL,
	  deployment_id INTEGER NOT NULL,
	  prompt_version TEXT NOT NULL,
	  provider TEXT NOT NULL,
	  model TEXT NOT NULL,
	  input_redacted INTEGER NOT NULL DEFAULT 0,
	  fallback_used INTEGER NOT NULL DEFAULT 0,
	  status TEXT NOT NULL,
	  input_excerpt TEXT,
	  output_excerpt TEXT,
	  output_text TEXT,
	  created_at INTEGER NOT NULL DEFAULT (unixepoch())
	);
	CREATE TABLE IF NOT EXISTS audit_events (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  actor_user_id INTEGER NOT NULL,
	  event_type TEXT NOT NULL,
	  target_type TEXT NOT NULL,
	  target_id TEXT,
	  status TEXT NOT NULL,
	  message TEXT,
	  metadata_json TEXT,
	  created_at INTEGER NOT NULL DEFAULT (unixepoch())
	);
	`

	applySchema(t, db, schema)

	return db
}

func seedAITestDeployment(t *testing.T, userID int64) store.Deployment {
	t.Helper()

	dep, err := appStore.CreateDeployment(context.Background(), store.CreateDeploymentInput{
		AppID:         101,
		UserID:        userID,
		Status:        "failed",
		TriggerType:   "manual",
		CommitSHA:     "abc123def456",
		CommitMessage: "feat: ai insights test",
		CommitAuthor:  "Casey",
		Branch:        "main",
		FailureReason: "build failed",
		CorrelationID: "corr-ai-test",
	})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	if err := appStore.CreateDeploymentLog(context.Background(), dep.ID, "error", "token=abc123XYZ build step failed"); err != nil {
		t.Fatalf("create deployment log: %v", err)
	}
	if err := appStore.CreateDeploymentLog(context.Background(), dep.ID, "info", "retry suggested"); err != nil {
		t.Fatalf("create deployment log: %v", err)
	}

	return dep
}
