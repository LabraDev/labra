package handlers

import (
	"errors"
	"net/http"
	"strings"

	"labra-backend/internal/api/auth"
	"labra-backend/internal/api/store"
)

// tokenIssuer is the global token issuer - set at startup with InitAuthRuntime
var (
	tokenIssuer auth.TokenIssuer
)

// InitAuthRuntime wires up the token issuer with the jwt config from the environment
func InitAuthRuntime(issuer auth.TokenIssuer) {
	tokenIssuer = issuer
}

// GetProfileHandler handles GET /v1/auth/profile
// returns the current user's info and their principal from the jwt
func GetProfileHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	userID, ok := readUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	// load the platform user record from the database
	platformUserRecord, err := appStore.GetPlatformUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "user profile not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load user profile")
		return
	}

	// also grab the jwt principal for session info
	jwtPrincipal, _ := auth.PrincipalFromContext(r.Context())
	// include aws connection count so the frontend knows if aws is configured
	awsConnections, _ := appStore.ListAWSConnectionsByUser(r.Context(), userID)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":         platformUserRecord.ID,
			"email":      platformUserRecord.Email,
			"status":     platformUserRecord.Status,
			"created_at": platformUserRecord.CreatedAt,
			"updated_at": platformUserRecord.UpdatedAt,
		},
		"principal": map[string]any{
			"sub":        jwtPrincipal.Sub,
			"email":      jwtPrincipal.Email,
			"roles":      jwtPrincipal.Roles,
			"session_id": jwtPrincipal.SessionID,
			"expires_at": jwtPrincipal.ExpiresAt,
		},
		"aws_connection_count": len(awsConnections),
	})
}

// PostLogoutHandler handles POST /v1/auth/logout
// revokes the current session in the database
func PostLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	// need the session id from the jwt to revoke it
	currentPrincipal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || strings.TrimSpace(currentPrincipal.SessionID) == "" {
		writeJSONError(w, http.StatusBadRequest, "session_id missing from auth token")
		return
	}

	// revoke the session - not-found is ok since it might already be expired
	revokeErr := appStore.RevokeAuthSession(r.Context(), currentPrincipal.SessionID)
	if revokeErr != nil && !errors.Is(revokeErr, store.ErrNotFound) {
		writeJSONError(w, http.StatusInternalServerError, "failed to revoke session")
		return
	}

	// audit log the logout event
	_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
		ActorUserID: currentPrincipal.UserID,
		EventType:   "auth.session.logout",
		TargetType:  "auth_session",
		TargetID:    currentPrincipal.SessionID,
		Status:      "success",
		Message:     "session revoked",
	})

	// 204 no content on successful logout
	w.WriteHeader(http.StatusNoContent)
}

// provisionPlatformUserWithProvider finds or creates a platform user for a given oauth identity
// first login creates a new user, subsequent logins just refresh the access token
func provisionPlatformUserWithProvider(r *http.Request, principal auth.Principal, providerName, oauthAccessToken string) (store.PlatformUser, bool, error) {
	// clean up the provider and token values
	sanitizedProviderName := strings.TrimSpace(providerName)
	if sanitizedProviderName == "" {
		sanitizedProviderName = "external"
	}
	sanitizedAccessToken := strings.TrimSpace(oauthAccessToken)

	// try to find an existing user by their provider + subject (e.g. github user id)
	existingUser, lookupErr := appStore.GetPlatformUserByIdentity(r.Context(), sanitizedProviderName, principal.Sub)
	if lookupErr == nil {
		// user exists - just refresh their access token and return
		_, _ = appStore.UpsertAuthIdentity(r.Context(), store.UpsertAuthIdentityInput{
			UserID:      existingUser.ID,
			Provider:    sanitizedProviderName,
			Subject:     principal.Sub,
			Email:       principal.Email,
			AccessToken: sanitizedAccessToken,
		})
		return existingUser, false, nil
	}
	if !errors.Is(lookupErr, store.ErrNotFound) {
		// real database error
		return store.PlatformUser{}, false, lookupErr
	}

	// new user - create the platform user record
	newUser, createErr := appStore.CreatePlatformUser(r.Context(), store.CreatePlatformUserInput{
		Email:  principal.Email,
		Status: "active",
	})
	if createErr != nil {
		return store.PlatformUser{}, false, createErr
	}

	// link their oauth identity to the new user
	_, identityErr := appStore.UpsertAuthIdentity(r.Context(), store.UpsertAuthIdentityInput{
		UserID:      newUser.ID,
		Provider:    sanitizedProviderName,
		Subject:     principal.Sub,
		Email:       principal.Email,
		AccessToken: sanitizedAccessToken,
	})
	if identityErr != nil {
		return store.PlatformUser{}, false, identityErr
	}

	// audit log the new user provisioning event
	_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
		ActorUserID: newUser.ID,
		EventType:   "auth.user.provisioned",
		TargetType:  "platform_user",
		TargetID:    strings.TrimSpace(principal.Sub),
		Status:      "success",
		Message:     "new platform user provisioned",
		Metadata:    "{\"provider\":\"" + sanitizedProviderName + "\"}",
	})

	return newUser, true, nil
}
