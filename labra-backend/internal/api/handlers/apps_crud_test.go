package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestAppsCRUDAndInfraOutputsFlow(t *testing.T) {
	db := setupAppsCRUDTestDB(t)
	prevStore := appStore
	t.Cleanup(func() {
		appStore = prevStore
		_ = db.Close()
	})

	InitAppStore(db)

	createPayload := []byte(`{
		"name":"Marketing Site",
		"repo_full_name":"acme/web-portal",
		"branch":"main",
		"build_type":"static",
		"output_dir":"dist",
		"root_dir":"frontend"
	}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/apps", bytes.NewReader(createPayload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withTestPrincipal(createReq, 77)
	createRR := httptest.NewRecorder()
	CreateAppHandler(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected create status 201, got %d body=%s", createRR.Code, createRR.Body.String())
	}

	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	appPath := "/v1/apps/" + strconv.FormatInt(created.ID, 10)

	infraReq := httptest.NewRequest(http.MethodGet, appPath+"/infra-outputs", nil)
	infraReq = withTestPrincipal(infraReq, 77)
	infraRR := httptest.NewRecorder()
	GetAppInfraOutputsHandler(infraRR, infraReq)
	if infraRR.Code != http.StatusOK {
		t.Fatalf("expected infra outputs status 200, got %d body=%s", infraRR.Code, infraRR.Body.String())
	}

	var infraBody struct {
		Outputs struct {
			BucketName     string `json:"bucket_name"`
			DistributionID string `json:"distribution_id"`
			SiteURL        string `json:"site_url"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(infraRR.Body.Bytes(), &infraBody); err != nil {
		t.Fatalf("unmarshal infra outputs: %v", err)
	}
	if infraBody.Outputs.BucketName == "" {
		t.Fatalf("expected non-empty infra outputs, got %+v", infraBody.Outputs)
	}

	patchPayload := []byte(`{"branch":"release","output_dir":"public"}`)
	patchReq := httptest.NewRequest(http.MethodPatch, appPath, bytes.NewReader(patchPayload))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq = withTestPrincipal(patchReq, 77)
	patchRR := httptest.NewRecorder()
	PatchAppHandler(patchRR, patchReq)
	if patchRR.Code != http.StatusOK {
		t.Fatalf("expected patch status 200, got %d body=%s", patchRR.Code, patchRR.Body.String())
	}

	historyReq := httptest.NewRequest(http.MethodGet, appPath+"/config-history", nil)
	historyReq = withTestPrincipal(historyReq, 77)
	historyRR := httptest.NewRecorder()
	GetAppConfigHistoryHandler(historyRR, historyReq)
	if historyRR.Code != http.StatusOK {
		t.Fatalf("expected config history status 200, got %d body=%s", historyRR.Code, historyRR.Body.String())
	}

	var historyBody struct {
		Versions []struct {
			Source string `json:"source"`
		} `json:"config_versions"`
	}
	if err := json.Unmarshal(historyRR.Body.Bytes(), &historyBody); err != nil {
		t.Fatalf("unmarshal config history: %v", err)
	}
	if len(historyBody.Versions) < 2 {
		t.Fatalf("expected at least 2 config versions (create + patch), got %d", len(historyBody.Versions))
	}
	if historyBody.Versions[0].Source != "patch" {
		t.Fatalf("expected latest config source patch, got %q", historyBody.Versions[0].Source)
	}

	forbiddenReq := httptest.NewRequest(http.MethodGet, appPath+"/infra-outputs", nil)
	forbiddenReq = withTestPrincipal(forbiddenReq, 88)
	forbiddenRR := httptest.NewRecorder()
	GetAppInfraOutputsHandler(forbiddenRR, forbiddenReq)
	if forbiddenRR.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-owner, got %d body=%s", forbiddenRR.Code, forbiddenRR.Body.String())
	}
}

func TestPatchAutoDeployDoesNotCreateConfigHistoryPatchEntry(t *testing.T) {
	db := setupAppsCRUDTestDB(t)
	prevStore := appStore
	t.Cleanup(func() {
		appStore = prevStore
		_ = db.Close()
	})

	InitAppStore(db)

	createPayload := []byte(`{
		"name":"Toggle Test",
		"repo_full_name":"acme/toggle-test",
		"branch":"main",
		"build_type":"static",
		"output_dir":"dist"
	}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/apps", bytes.NewReader(createPayload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withTestPrincipal(createReq, 77)
	createRR := httptest.NewRecorder()
	CreateAppHandler(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected create status 201, got %d body=%s", createRR.Code, createRR.Body.String())
	}

	var created struct {
		ID                int64 `json:"id"`
		AutoDeployEnabled bool  `json:"auto_deploy_enabled"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if !created.AutoDeployEnabled {
		t.Fatalf("expected auto deploy enabled by default")
	}

	appPath := "/v1/apps/" + strconv.FormatInt(created.ID, 10)

	patchPayload := []byte(`{"auto_deploy_enabled":false}`)
	patchReq := httptest.NewRequest(http.MethodPatch, appPath, bytes.NewReader(patchPayload))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq = withTestPrincipal(patchReq, 77)
	patchRR := httptest.NewRecorder()
	PatchAppHandler(patchRR, patchReq)
	if patchRR.Code != http.StatusOK {
		t.Fatalf("expected patch status 200, got %d body=%s", patchRR.Code, patchRR.Body.String())
	}

	var patched struct {
		AutoDeployEnabled bool `json:"auto_deploy_enabled"`
	}
	if err := json.Unmarshal(patchRR.Body.Bytes(), &patched); err != nil {
		t.Fatalf("unmarshal patch response: %v", err)
	}
	if patched.AutoDeployEnabled {
		t.Fatalf("expected auto_deploy_enabled=false after patch")
	}

	historyReq := httptest.NewRequest(http.MethodGet, appPath+"/config-history", nil)
	historyReq = withTestPrincipal(historyReq, 77)
	historyRR := httptest.NewRecorder()
	GetAppConfigHistoryHandler(historyRR, historyReq)
	if historyRR.Code != http.StatusOK {
		t.Fatalf("expected config history status 200, got %d body=%s", historyRR.Code, historyRR.Body.String())
	}

	var historyBody struct {
		Versions []struct {
			Source string `json:"source"`
		} `json:"config_versions"`
	}
	if err := json.Unmarshal(historyRR.Body.Bytes(), &historyBody); err != nil {
		t.Fatalf("unmarshal config history: %v", err)
	}
	if len(historyBody.Versions) != 1 {
		t.Fatalf("expected only create config version after auto-deploy toggle, got %d", len(historyBody.Versions))
	}
	if historyBody.Versions[0].Source != "create" {
		t.Fatalf("expected config version source create, got %q", historyBody.Versions[0].Source)
	}
}

func setupAppsCRUDTestDB(t *testing.T) *sql.DB {
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

	CREATE TABLE IF NOT EXISTS app_config_versions (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  app_id INTEGER NOT NULL,
	  user_id INTEGER NOT NULL,
	  source TEXT NOT NULL,
	  config_json TEXT NOT NULL,
	  created_at INTEGER NOT NULL DEFAULT (unixepoch())
	);

	CREATE TABLE IF NOT EXISTS app_infra_outputs (
	  app_id INTEGER PRIMARY KEY,
	  user_id INTEGER NOT NULL,
	  bucket_name TEXT NOT NULL,
	  distribution_id TEXT NOT NULL,
	  site_url TEXT,
	  updated_at INTEGER NOT NULL DEFAULT (unixepoch())
	);
	`

	applySchema(t, db, schema)

	return db
}
