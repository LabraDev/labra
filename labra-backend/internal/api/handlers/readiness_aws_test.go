package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadinessAndAWSConnectionsFlow(t *testing.T) {
	db := setupReadinessAWSTestDB(t)
	previousStore := appStore
	previousProbe := readinessProbe
	t.Cleanup(func() {
		appStore = previousStore
		readinessProbe = previousProbe
		_ = db.Close()
	})

	InitAppStore(db)
	InitReadiness(func(ctx context.Context) error { return db.PingContext(ctx) })

	t.Run("ready endpoint returns success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rr := httptest.NewRecorder()
		HandleReadiness(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("upsert and list aws connections", func(t *testing.T) {
		payload := []byte(`{"role_arn":"arn:aws:iam::123456789012:role/labra-dev-access","external_id":"ext-id-12345","region":"us-west-2"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/aws-connections", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req = withTestPrincipal(req, 7)
		rr := httptest.NewRecorder()

		UpsertAWSConnectionHandler(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d, body=%s", rr.Code, rr.Body.String())
		}

		listReq := httptest.NewRequest(http.MethodGet, "/v1/aws-connections", nil)
		listReq = withTestPrincipal(listReq, 7)
		listRR := httptest.NewRecorder()
		ListAWSConnectionsHandler(listRR, listReq)
		if listRR.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d, body=%s", listRR.Code, listRR.Body.String())
		}

		var body struct {
			Connections []struct {
				RoleARN   string `json:"role_arn"`
				AccountID string `json:"account_id"`
				Region    string `json:"region"`
			} `json:"aws_connections"`
		}
		if err := json.Unmarshal(listRR.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal list response: %v", err)
		}
		if len(body.Connections) != 1 {
			t.Fatalf("expected 1 connection, got %d", len(body.Connections))
		}
		if body.Connections[0].AccountID != "123456789012" {
			t.Fatalf("expected derived account ID, got %q", body.Connections[0].AccountID)
		}

		deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/aws-connections/1", nil)
		deleteReq = withTestPrincipal(deleteReq, 7)
		deleteRR := httptest.NewRecorder()
		DeleteAWSConnectionHandler(deleteRR, deleteReq)
		if deleteRR.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d, body=%s", deleteRR.Code, deleteRR.Body.String())
		}

		listAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/v1/aws-connections", nil)
		listAfterDeleteReq = withTestPrincipal(listAfterDeleteReq, 7)
		listAfterDeleteRR := httptest.NewRecorder()
		ListAWSConnectionsHandler(listAfterDeleteRR, listAfterDeleteReq)
		if listAfterDeleteRR.Code != http.StatusOK {
			t.Fatalf("expected 200 after delete, got %d, body=%s", listAfterDeleteRR.Code, listAfterDeleteRR.Body.String())
		}

		var afterDelete struct {
			Connections []struct{} `json:"aws_connections"`
		}
		if err := json.Unmarshal(listAfterDeleteRR.Body.Bytes(), &afterDelete); err != nil {
			t.Fatalf("unmarshal list after delete response: %v", err)
		}
		if len(afterDelete.Connections) != 0 {
			t.Fatalf("expected 0 connections after delete, got %d", len(afterDelete.Connections))
		}
	})

	t.Run("invalid role arn returns bad request", func(t *testing.T) {
		payload := []byte(`{"role_arn":"bad-arn","external_id":"ext-id-12345","region":"us-west-2"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/aws-connections", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req = withTestPrincipal(req, 7)
		rr := httptest.NewRecorder()

		UpsertAWSConnectionHandler(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
		}
	})
}

func setupReadinessAWSTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db := openInMemorySQLite(t)

	schema := `
	CREATE TABLE IF NOT EXISTS aws_connections (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  user_id INTEGER NOT NULL,
	  role_arn TEXT NOT NULL,
	  external_id TEXT NOT NULL,
	  region TEXT NOT NULL,
	  account_id TEXT NOT NULL,
	  status TEXT NOT NULL,
	  last_validated_at INTEGER,
	  created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	  updated_at INTEGER NOT NULL DEFAULT (unixepoch())
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_aws_connections_user_role_region
	  ON aws_connections(user_id, role_arn, region);

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

func TestReadinessFailureReturnsServiceUnavailable(t *testing.T) {
	previousProbe := readinessProbe
	t.Cleanup(func() {
		readinessProbe = previousProbe
	})

	InitReadiness(func(context.Context) error { return fmt.Errorf("db unavailable") })

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()
	HandleReadiness(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d, body=%s", rr.Code, rr.Body.String())
	}
}
