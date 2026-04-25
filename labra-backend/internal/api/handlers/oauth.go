package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"labra-backend/internal/api/auth"
	"labra-backend/internal/api/services"
	"labra-backend/internal/api/store"
)

type githubOAuthUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if err := services.Authenticate(w, r); err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "GitHub sign-in is unavailable right now. Use Session Sign-In.")
		return
	}
}

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	result, err := services.CallbackWithToken(w, r)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "not configured") {
			status = http.StatusServiceUnavailable
		}
		if isOAuthRetryableCookieError(err) {
			http.Redirect(w, r, "/login?notice="+url.QueryEscape("GitHub sign-in expired. Click Login with GitHub again."), http.StatusTemporaryRedirect)
			return
		}
		writeJSONError(w, status, "GitHub sign-in failed. "+err.Error())
		return
	}

	var ghUser githubOAuthUser
	if err := json.Unmarshal(result.UserBody, &ghUser); err != nil {
		writeJSONError(w, http.StatusBadGateway, "failed to parse GitHub user profile")
		return
	}
	if ghUser.ID <= 0 {
		writeJSONError(w, http.StatusBadGateway, "GitHub user profile did not include a valid id")
		return
	}
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}
	if len(tokenIssuer.Secret) == 0 {
		writeJSONError(w, http.StatusServiceUnavailable, "auth issuer is not configured")
		return
	}

	email := strings.TrimSpace(ghUser.Email)
	if email == "" {
		login := strings.TrimSpace(ghUser.Login)
		if login == "" {
			login = "github-user-" + strconv.FormatInt(ghUser.ID, 10)
		}
		email = login + "@users.noreply.github.com"
	}

	principal := auth.Principal{
		Sub:   fmt.Sprintf("github:%d", ghUser.ID),
		Email: email,
		Roles: []string{"owner"},
	}

	user, _, err := provisionPlatformUserWithProvider(r, principal, "github", result.AccessToken)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to provision GitHub user identity")
		return
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
		writeJSONError(w, http.StatusServiceUnavailable, "failed to create auth session from GitHub sign-in")
		return
	}

	_, err = appStore.CreateAuthSession(r.Context(), store.CreateAuthSessionInput{
		SessionID: sessionID,
		UserID:    user.ID,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to persist auth session")
		return
	}

	_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
		ActorUserID: user.ID,
		EventType:   "auth.session.create",
		TargetType:  "auth_session",
		TargetID:    sessionID,
		Status:      "success",
		Message:     "session issued via GitHub OAuth",
	})

	redirectParams := url.Values{}
	redirectParams.Set("session_token", signedToken)
	redirectParams.Set("provider", "github")
	http.Redirect(w, r, "/login#"+redirectParams.Encode(), http.StatusTemporaryRedirect)
}

func isOAuthRetryableCookieError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "oauth state cookie is missing") ||
		strings.Contains(msg, "state does not match") ||
		strings.Contains(msg, "unable to get verifier")
}
