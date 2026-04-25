package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestDeleteAppHandler_RemovesAppAndRelatedData(t *testing.T) {
	db := setupDeleteAppTestDB(t)
	prevStore := appStore
	t.Cleanup(func() {
		appStore = prevStore
		_ = db.Close()
	})
	InitAppStore(db)

	appID := seedDeleteAppFixture(t, db, 77, "delete-me", "acme/delete-me")
	otherAppID := seedDeleteAppFixture(t, db, 77, "keep-me", "acme/keep-me")

	req := httptest.NewRequest(http.MethodDelete, "/v1/apps/"+strconv.FormatInt(appID, 10), nil)
	req.Header.Set("X-User-ID", "77")
	rr := httptest.NewRecorder()
	DeleteAppHandler(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rr.Code, rr.Body.String())
	}

	assertCount(t, db, "SELECT COUNT(*) FROM apps WHERE id = ?", appID, 0)
	assertCount(t, db, "SELECT COUNT(*) FROM app_config_versions WHERE app_id = ?", appID, 0)
	assertCount(t, db, "SELECT COUNT(*) FROM app_infra_outputs WHERE app_id = ?", appID, 0)
	assertCount(t, db, "SELECT COUNT(*) FROM deployments WHERE app_id = ?", appID, 0)
	assertCount(t, db, "SELECT COUNT(*) FROM deployment_logs WHERE deployment_id IN (SELECT id FROM deployments WHERE app_id = ?)", appID, 0)
	assertCount(t, db, "SELECT COUNT(*) FROM ai_request_logs WHERE deployment_id IN (SELECT id FROM deployments WHERE app_id = ?)", appID, 0)
	assertCount(t, db, "SELECT COUNT(*) FROM webhook_deliveries WHERE app_id = ?", appID, 0)

	assertCount(t, db, "SELECT COUNT(*) FROM apps WHERE id = ?", otherAppID, 1)
}

func TestDeleteAppHandler_NotFoundForOtherUser(t *testing.T) {
	db := setupDeleteAppTestDB(t)
	prevStore := appStore
	t.Cleanup(func() {
		appStore = prevStore
		_ = db.Close()
	})
	InitAppStore(db)

	appID := seedDeleteAppFixture(t, db, 77, "owned-app", "acme/owned-app")

	req := httptest.NewRequest(http.MethodDelete, "/v1/apps/"+strconv.FormatInt(appID, 10), nil)
	req.Header.Set("X-User-ID", "88")
	rr := httptest.NewRecorder()
	DeleteAppHandler(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}

	assertCount(t, db, "SELECT COUNT(*) FROM apps WHERE id = ?", appID, 1)
}

func setupDeleteAppTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Keep all queries on the same in-memory database connection.
	db.SetMaxOpenConns(1)

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
	  created_at INTEGER NOT NULL DEFAULT (unixepoch())
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return db
}

func seedDeleteAppFixture(t *testing.T, db *sql.DB, userID int64, name, repo string) int64 {
	t.Helper()

	res, err := db.Exec(`
		INSERT INTO apps (user_id, name, repo_full_name, branch, build_type, output_dir, root_dir, site_url, auto_deploy_enabled, created_at, updated_at)
		VALUES (?, ?, ?, 'main', 'static', 'dist', '', 'https://example.com', 1, unixepoch(), unixepoch())
	`, userID, name, repo)
	if err != nil {
		t.Fatalf("seed app: %v", err)
	}
	appID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed app id: %v", err)
	}

	depRes, err := db.Exec(`
		INSERT INTO deployments (app_id, user_id, status, trigger_type, branch, created_at, updated_at)
		VALUES (?, ?, 'queued', 'manual', 'main', unixepoch(), unixepoch())
	`, appID, userID)
	if err != nil {
		t.Fatalf("seed deployment: %v", err)
	}
	deploymentID, err := depRes.LastInsertId()
	if err != nil {
		t.Fatalf("seed deployment id: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO deployment_logs (deployment_id, log_level, message, created_at)
		VALUES (?, 'info', 'queued', unixepoch())
	`, deploymentID); err != nil {
		t.Fatalf("seed deployment_logs: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO ai_request_logs (user_id, deployment_id, prompt_version, provider, model, input_redacted, fallback_used, status, created_at)
		VALUES (?, ?, 'phase7-v1', 'mock', 'mock-model', 1, 0, 'succeeded', unixepoch())
	`, userID, deploymentID); err != nil {
		t.Fatalf("seed ai_request_logs: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO webhook_deliveries (app_id, delivery_id, event_type, commit_sha, received_at)
		VALUES (?, ?, 'push', 'abc123', unixepoch())
	`, appID, "delivery-"+strconv.FormatInt(appID, 10)); err != nil {
		t.Fatalf("seed webhook_deliveries: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO app_config_versions (app_id, user_id, source, config_json, created_at)
		VALUES (?, ?, 'create', '{}', unixepoch())
	`, appID, userID); err != nil {
		t.Fatalf("seed app_config_versions: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO app_infra_outputs (app_id, user_id, bucket_name, distribution_id, site_url, updated_at)
		VALUES (?, ?, ?, ?, ?, unixepoch())
	`, appID, userID, "labra-bucket", "pending-dist", "https://preview.labra.local"); err != nil {
		t.Fatalf("seed app_infra_outputs: %v", err)
	}

	return appID
}

func assertCount(t *testing.T, db *sql.DB, query string, arg int64, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow(query, arg).Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != expected {
		t.Fatalf("expected count %d, got %d for query %q", expected, count, query)
	}
}
