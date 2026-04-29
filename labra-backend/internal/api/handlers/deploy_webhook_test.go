package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"labra-backend/internal/api/store"
)

func TestDeployAndWebhookFlow(t *testing.T) {
	db := setupDeployWebhookTestDB(t)

	prevStore := appStore
	prevAsync := runDeploymentAsync
	prevExecutor := executeDeploymentPipelineFn
	prevSecret := githubWebhookSecret
	prevNow := webhookNowUnix
	t.Cleanup(func() {
		appStore = prevStore
		runDeploymentAsync = prevAsync
		executeDeploymentPipelineFn = prevExecutor
		githubWebhookSecret = prevSecret
		webhookNowUnix = prevNow
		_ = db.Close()
	})

	InitAppStore(db)
	InitWebhook("test-webhook-secret")
	runDeploymentAsync = false
	executeDeploymentPipelineFn = func(_ context.Context, _ int64, _ store.App, _ string) (deploymentPipelineResult, error) {
		return deploymentPipelineResult{
			SiteURL: "https://example.cloudfront.net",
			Status:  "succeeded",
		}, nil
	}
	webhookNowUnix = func() int64 { return time.Now().Unix() }

	appID := createDeployTestApp(t, 42, "Deploy Test Site", "owner/repo", "main")
	appPath := "/v1/apps/" + strconv.FormatInt(appID, 10)

	t.Run("manual deploy succeeds and logs are queryable", func(t *testing.T) {
		createReq := httptest.NewRequest(http.MethodPost, appPath+"/deploy", nil)
		createReq = withTestPrincipal(createReq, 42)
		createRR := httptest.NewRecorder()
		CreateDeployHandler(createRR, createReq)
		if createRR.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d body=%s", createRR.Code, createRR.Body.String())
		}

		var createBody struct {
			Deployment struct {
				ID int64 `json:"id"`
			} `json:"deployment"`
		}
		if err := json.Unmarshal(createRR.Body.Bytes(), &createBody); err != nil {
			t.Fatalf("unmarshal deploy create response: %v", err)
		}
		if createBody.Deployment.ID <= 0 {
			t.Fatalf("expected deployment id > 0, got %d", createBody.Deployment.ID)
		}

		deployReq := httptest.NewRequest(http.MethodGet, "/v1/deploys/"+strconv.FormatInt(createBody.Deployment.ID, 10), nil)
		deployReq = withTestPrincipal(deployReq, 42)
		deployRR := httptest.NewRecorder()
		GetDeployHandler(deployRR, deployReq)
		if deployRR.Code != http.StatusOK {
			t.Fatalf("expected deploy details 200, got %d body=%s", deployRR.Code, deployRR.Body.String())
		}

		var dep struct {
			Status      string `json:"status"`
			TriggerType string `json:"trigger_type"`
		}
		if err := json.Unmarshal(deployRR.Body.Bytes(), &dep); err != nil {
			t.Fatalf("unmarshal deploy details: %v", err)
		}
		if dep.Status != "succeeded" {
			t.Fatalf("expected succeeded status, got %q", dep.Status)
		}
		if dep.TriggerType != "manual" {
			t.Fatalf("expected manual trigger_type, got %q", dep.TriggerType)
		}

		logReq := httptest.NewRequest(http.MethodGet, "/v1/deploys/"+strconv.FormatInt(createBody.Deployment.ID, 10)+"/logs", nil)
		logReq = withTestPrincipal(logReq, 42)
		logRR := httptest.NewRecorder()
		GetDeployLogsHandler(logRR, logReq)
		if logRR.Code != http.StatusOK {
			t.Fatalf("expected logs 200, got %d body=%s", logRR.Code, logRR.Body.String())
		}

		var logsBody struct {
			Logs []struct {
				Message string `json:"message"`
			} `json:"logs"`
		}
		if err := json.Unmarshal(logRR.Body.Bytes(), &logsBody); err != nil {
			t.Fatalf("unmarshal deploy logs: %v", err)
		}
		if len(logsBody.Logs) == 0 {
			t.Fatalf("expected deploy logs to be present")
		}
		found := false
		for _, l := range logsBody.Logs {
			if strings.Contains(strings.ToLower(l.Message), "completed successfully") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected success completion log, got %+v", logsBody.Logs)
		}
	})

	t.Run("cancel and retry lifecycle", func(t *testing.T) {
		queued, err := appStore.CreateDeployment(context.Background(), store.CreateDeploymentInput{
			AppID:       appID,
			UserID:      42,
			Status:      "queued",
			TriggerType: "manual",
			Branch:      "main",
		})
		if err != nil {
			t.Fatalf("create queued deployment: %v", err)
		}

		cancelReq := httptest.NewRequest(http.MethodPost, "/v1/deploys/"+strconv.FormatInt(queued.ID, 10)+"/cancel", bytes.NewReader([]byte(`{}`)))
		cancelReq = withTestPrincipal(cancelReq, 42)
		cancelRR := httptest.NewRecorder()
		CancelDeployHandler(cancelRR, cancelReq)
		if cancelRR.Code != http.StatusOK {
			t.Fatalf("expected cancel 200, got %d body=%s", cancelRR.Code, cancelRR.Body.String())
		}

		var canceled struct {
			Deployment struct {
				Status string `json:"status"`
			} `json:"deployment"`
		}
		if err := json.Unmarshal(cancelRR.Body.Bytes(), &canceled); err != nil {
			t.Fatalf("unmarshal cancel response: %v", err)
		}
		if canceled.Deployment.Status != "canceled" {
			t.Fatalf("expected canceled status, got %q", canceled.Deployment.Status)
		}

		retryReq := httptest.NewRequest(http.MethodPost, "/v1/deploys/"+strconv.FormatInt(queued.ID, 10)+"/retry", bytes.NewReader([]byte(`{}`)))
		retryReq = withTestPrincipal(retryReq, 42)
		retryRR := httptest.NewRecorder()
		RetryDeployHandler(retryRR, retryReq)
		if retryRR.Code != http.StatusAccepted {
			t.Fatalf("expected retry 202, got %d body=%s", retryRR.Code, retryRR.Body.String())
		}

		var retryBody struct {
			Deployment struct {
				ID          int64  `json:"id"`
				TriggerType string `json:"trigger_type"`
			} `json:"deployment"`
		}
		if err := json.Unmarshal(retryRR.Body.Bytes(), &retryBody); err != nil {
			t.Fatalf("unmarshal retry response: %v", err)
		}
		if retryBody.Deployment.ID <= 0 {
			t.Fatalf("expected retried deployment id > 0")
		}
		if retryBody.Deployment.TriggerType != "manual_retry" {
			t.Fatalf("expected manual_retry trigger, got %q", retryBody.Deployment.TriggerType)
		}

		retriedReq := httptest.NewRequest(http.MethodGet, "/v1/deploys/"+strconv.FormatInt(retryBody.Deployment.ID, 10), nil)
		retriedReq = withTestPrincipal(retriedReq, 42)
		retriedRR := httptest.NewRecorder()
		GetDeployHandler(retriedRR, retriedReq)
		if retriedRR.Code != http.StatusOK {
			t.Fatalf("expected retried deployment details 200, got %d body=%s", retriedRR.Code, retriedRR.Body.String())
		}
		var retried struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(retriedRR.Body.Bytes(), &retried); err != nil {
			t.Fatalf("unmarshal retried deployment: %v", err)
		}
		if retried.Status != "succeeded" {
			t.Fatalf("expected retried deployment to succeed, got %q", retried.Status)
		}
	})

	t.Run("webhook auto deploy uses dedupe and freshness checks", func(t *testing.T) {
		payload := webhookPayload("refs/heads/main", "owner/repo", "abc123def456", "feat: new feature", "Casey")
		deliveryID := "deploy-webhook-delivery-1"
		now := strconv.FormatInt(time.Now().Unix(), 10)

		req1 := signedWebhookRequest(payload, deliveryID, "test-webhook-secret")
		req1.Header.Set("X-Labra-Webhook-Timestamp", now)
		rr1 := httptest.NewRecorder()
		GitHubWebhookHandler(rr1, req1)
		if rr1.Code != http.StatusAccepted {
			t.Fatalf("expected webhook 202, got %d body=%s", rr1.Code, rr1.Body.String())
		}

		var body1 map[string]any
		if err := json.Unmarshal(rr1.Body.Bytes(), &body1); err != nil {
			t.Fatalf("unmarshal webhook response: %v", err)
		}
		if got := numberAsInt(body1["triggered_count"]); got != 1 {
			t.Fatalf("expected triggered_count=1, got %d body=%v", got, body1)
		}
		if got := numberAsInt(body1["duplicate_count"]); got != 0 {
			t.Fatalf("expected duplicate_count=0, got %d body=%v", got, body1)
		}

		req2 := signedWebhookRequest(payload, deliveryID, "test-webhook-secret")
		req2.Header.Set("X-Labra-Webhook-Timestamp", now)
		rr2 := httptest.NewRecorder()
		GitHubWebhookHandler(rr2, req2)
		if rr2.Code != http.StatusAccepted {
			t.Fatalf("expected duplicate webhook 202, got %d body=%s", rr2.Code, rr2.Body.String())
		}

		var body2 map[string]any
		if err := json.Unmarshal(rr2.Body.Bytes(), &body2); err != nil {
			t.Fatalf("unmarshal duplicate webhook response: %v", err)
		}
		if got := numberAsInt(body2["triggered_count"]); got != 0 {
			t.Fatalf("expected duplicate triggered_count=0, got %d body=%v", got, body2)
		}
		if got := numberAsInt(body2["duplicate_count"]); got != 1 {
			t.Fatalf("expected duplicate_count=1, got %d body=%v", got, body2)
		}

		staleReq := signedWebhookRequest(payload, "deploy-webhook-stale-delivery", "test-webhook-secret")
		staleReq.Header.Set("X-Labra-Webhook-Timestamp", strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10))
		staleRR := httptest.NewRecorder()
		GitHubWebhookHandler(staleRR, staleReq)
		if staleRR.Code != http.StatusBadRequest {
			t.Fatalf("expected stale webhook 400, got %d body=%s", staleRR.Code, staleRR.Body.String())
		}

		historyReq := httptest.NewRequest(http.MethodGet, appPath+"/deploys", nil)
		historyReq = withTestPrincipal(historyReq, 42)
		historyRR := httptest.NewRecorder()
		GetAppDeploysHandler(historyRR, historyReq)
		if historyRR.Code != http.StatusOK {
			t.Fatalf("expected deploy history 200, got %d body=%s", historyRR.Code, historyRR.Body.String())
		}

		var history struct {
			Deployments []struct {
				TriggerType   string `json:"trigger_type"`
				CommitSHA     string `json:"commit_sha"`
				CommitMessage string `json:"commit_message"`
			} `json:"deployments"`
		}
		if err := json.Unmarshal(historyRR.Body.Bytes(), &history); err != nil {
			t.Fatalf("unmarshal history response: %v", err)
		}
		webhookFound := false
		for _, dep := range history.Deployments {
			if dep.TriggerType == "webhook" {
				webhookFound = dep.CommitSHA != "" && dep.CommitMessage != ""
				break
			}
		}
		if !webhookFound {
			t.Fatalf("expected history to include webhook deployment with commit metadata")
		}
	})
}

func setupDeployWebhookTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db := openInMemorySQLite(t)

	schema := `
	CREATE TABLE IF NOT EXISTS apps (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  user_id INTEGER NOT NULL,
	  name TEXT NOT NULL,
	  repo_full_name TEXT NOT NULL,
	  branch TEXT NOT NULL DEFAULT 'main',
	  build_type TEXT NOT NULL DEFAULT 'static',
	  output_dir TEXT NOT NULL DEFAULT 'dist',
	  root_dir TEXT,
	  site_url TEXT,
	  auto_deploy_enabled INTEGER NOT NULL DEFAULT 1,
	  created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	  updated_at INTEGER NOT NULL DEFAULT (unixepoch())
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_apps_user_repo_branch
	  ON apps(user_id, repo_full_name, branch);

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

	CREATE TABLE IF NOT EXISTS webhook_deliveries (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  app_id INTEGER NOT NULL,
	  delivery_id TEXT NOT NULL,
	  event_type TEXT NOT NULL,
	  commit_sha TEXT,
	  received_at INTEGER NOT NULL DEFAULT (unixepoch()),
	  UNIQUE(app_id, delivery_id)
	);
	`

	applySchema(t, db, schema)

	return db
}

func createDeployTestApp(t *testing.T, userID int64, name, repo, branch string) int64 {
	t.Helper()

	payload := []byte(`{
		"name":"` + name + `",
		"repo_full_name":"` + repo + `",
		"branch":"` + branch + `",
		"build_type":"static",
		"output_dir":"dist",
		"root_dir":"",
		"site_url":""
	}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/apps", bytes.NewReader(payload))
	req = withTestPrincipal(req, userID)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	CreateAppHandler(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected create app 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	var body struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal create app response: %v", err)
	}
	if body.ID <= 0 {
		t.Fatalf("expected app id > 0")
	}
	return body.ID
}
