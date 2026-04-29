package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"labra-backend/internal/api/auth"
	"labra-backend/internal/api/store"
)

func TestAuthSessionAndUserProvisioningFlow(t *testing.T) {
	secret := "test-auth-secret"
	issuer := "https://issuer.example.com"
	audience := "labra-control-plane"

	db := setupAuthSessionTestDB(t)
	previousStore := appStore
	previousTokenIssuer := tokenIssuer
	previousAssumeRoleVerifier := assumeRoleVerifier
	t.Cleanup(func() {
		appStore = previousStore
		tokenIssuer = previousTokenIssuer
		assumeRoleVerifier = previousAssumeRoleVerifier
		_ = db.Close()
	})

	InitAppStore(db)
	validator := auth.HMACValidator{Issuer: issuer, Audience: audience, Secret: []byte(secret)}
	InitAuthRuntime(auth.TokenIssuer{
		Issuer:   issuer,
		Audience: audience,
		Secret:   []byte(secret),
		TTL:      2 * time.Hour,
	})

	t.Run("create auth session and provision user", func(t *testing.T) {
		sessionToken, isNew, err := issueSessionForTest(auth.Principal{
			Sub:   "external-user-001",
			Email: "casey@example.com",
			Roles: []string{"owner"},
		})
		if err != nil {
			t.Fatalf("issue session: %v", err)
		}
		if sessionToken == "" {
			t.Fatalf("expected session token")
		}
		if !isNew {
			t.Fatalf("expected first login to provision new user")
		}

		profileReq := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
		profileReq.Header.Set("Authorization", "Bearer "+sessionToken)
		profileRR := httptest.NewRecorder()
		auth.RequireAuth(validator)(http.HandlerFunc(GetProfileHandler)).ServeHTTP(profileRR, profileReq)
		if profileRR.Code != http.StatusOK {
			t.Fatalf("expected profile 200, got %d body=%s", profileRR.Code, profileRR.Body.String())
		}

		awsPayload := []byte(`{"role_arn":"arn:aws:iam::123456789012:role/labra-dev-access","external_id":"external-id-123","region":"us-west-2"}`)
		awsReq := httptest.NewRequest(http.MethodPost, "/v1/aws-connections", bytes.NewReader(awsPayload))
		awsReq.Header.Set("Content-Type", "application/json")
		awsReq.Header.Set("Authorization", "Bearer "+sessionToken)
		awsRR := httptest.NewRecorder()
		auth.RequireAuth(validator)(http.HandlerFunc(UpsertAWSConnectionHandler)).ServeHTTP(awsRR, awsReq)
		if awsRR.Code != http.StatusCreated {
			t.Fatalf("expected aws connection 201, got %d body=%s", awsRR.Code, awsRR.Body.String())
		}

		systemReq := httptest.NewRequest(http.MethodGet, "/v1/system/services", nil)
		systemReq.Header.Set("Authorization", "Bearer "+sessionToken)
		systemRR := httptest.NewRecorder()
		auth.RequireAuth(validator)(http.HandlerFunc(GetSystemServicesHandler)).ServeHTTP(systemRR, systemReq)
		if systemRR.Code != http.StatusOK {
			t.Fatalf("expected system services 200, got %d body=%s", systemRR.Code, systemRR.Body.String())
		}

		logoutReq := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
		logoutReq.Header.Set("Authorization", "Bearer "+sessionToken)
		logoutRR := httptest.NewRecorder()
		auth.RequireAuth(validator)(http.HandlerFunc(PostLogoutHandler)).ServeHTTP(logoutRR, logoutReq)
		if logoutRR.Code != http.StatusNoContent {
			t.Fatalf("expected logout 204, got %d body=%s", logoutRR.Code, logoutRR.Body.String())
		}
	})

	t.Run("second session reuses provisioned user", func(t *testing.T) {
		_, isNew, err := issueSessionForTest(auth.Principal{
			Sub:   "external-user-001",
			Email: "casey@example.com",
			Roles: []string{"owner"},
		})
		if err != nil {
			t.Fatalf("issue session: %v", err)
		}
		if isNew {
			t.Fatalf("expected existing provisioned user on second login")
		}
	})
}

func setupAuthSessionTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db := openInMemorySQLite(t)

	schema := `
	CREATE TABLE IF NOT EXISTS platform_users (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  email TEXT,
	  status TEXT NOT NULL DEFAULT 'active',
	  created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	  updated_at INTEGER NOT NULL DEFAULT (unixepoch())
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_platform_users_email
	  ON platform_users(email)
	  WHERE email IS NOT NULL;

	CREATE TABLE IF NOT EXISTS auth_identities (
	  id INTEGER PRIMARY KEY AUTOINCREMENT,
	  user_id INTEGER NOT NULL,
	  provider TEXT NOT NULL,
	  subject TEXT NOT NULL,
	  email TEXT,
	  access_token TEXT,
	  token_updated_at INTEGER,
	  created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	  updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
	  UNIQUE(provider, subject)
	);

	CREATE TABLE IF NOT EXISTS auth_sessions (
	  session_id TEXT PRIMARY KEY,
	  user_id INTEGER NOT NULL,
	  expires_at INTEGER NOT NULL,
	  created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	  revoked_at INTEGER
	);

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

func issueSessionForTest(principal auth.Principal) (string, bool, error) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	user, isNew, err := provisionPlatformUserWithProvider(req, principal, "external", "")
	if err != nil {
		return "", false, err
	}

	sessionID := auth.GenerateSessionID()
	signedToken, expiresAt, err := tokenIssuer.MintSessionToken(
		user.ID,
		principal.Sub,
		principal.Email,
		principal.Roles,
		sessionID,
	)
	if err != nil {
		return "", false, err
	}

	_, err = appStore.CreateAuthSession(req.Context(), store.CreateAuthSessionInput{
		SessionID: sessionID,
		UserID:    user.ID,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return "", false, err
	}

	return signedToken, isNew, nil
}

func TestAssumeRoleValidationRejectsAccountMismatch(t *testing.T) {
	db := setupAuthSessionTestDB(t)
	prevStore := appStore
	prevAssumeRoleVerifier := assumeRoleVerifier
	t.Cleanup(func() {
		appStore = prevStore
		assumeRoleVerifier = prevAssumeRoleVerifier
		_ = db.Close()
	})
	InitAppStore(db)

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: 11, Sub: "sub-11", Roles: []string{"owner"}})
	payload := []byte(`{"role_arn":"arn:aws:iam::123456789012:role/labra-dev-access","external_id":"external-id-123","region":"us-west-2","account_id":"999999999999"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/aws-connections", bytes.NewReader(payload)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	UpsertAWSConnectionHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}
