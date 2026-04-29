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

// githubOAuthUser is the shape of what github sends back when we ask for /user
type githubOAuthUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

// LoginHandler handles GET /v1/login
// redirects the user to github to kick off the oauth flow
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if err := services.Authenticate(w, r); err != nil {
		// if oauth isn't configured this will fail - show a friendly error
		writeJSONError(w, http.StatusServiceUnavailable, "GitHub sign-in is unavailable right now.")
		return
	}
}

// CallbackHandler handles GET /v1/callback
// github redirects here after the user authorizes our app
// exchanges the code for a token, creates or finds the user, then issues a session token
func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	// complete the oauth exchange and get the github user's access token
	oauthCallbackResult, err := services.CallbackWithToken(w, r)
	if err != nil {
		// figure out the right status code for the error
		errorStatusCode := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "not configured") {
			errorStatusCode = http.StatusServiceUnavailable
		}
		// state/cookie errors mean the user should just try clicking login again
		if isOAuthRetryableCookieError(err) {
			http.Redirect(w, r, "/login?notice="+url.QueryEscape("GitHub sign-in expired. Click Login with GitHub again."), http.StatusTemporaryRedirect)
			return
		}
		writeJSONError(w, errorStatusCode, "GitHub sign-in failed. "+err.Error())
		return
	}

	// parse the github user profile from the oauth response
	var githubUserProfile githubOAuthUser
	if err := json.Unmarshal(oauthCallbackResult.UserBody, &githubUserProfile); err != nil {
		writeJSONError(w, http.StatusBadGateway, "failed to parse GitHub user profile")
		return
	}
	if githubUserProfile.ID <= 0 {
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

	// github doesn't always give us an email address - synthesize one from login if needed
	userEmailAddress := strings.TrimSpace(githubUserProfile.Email)
	if userEmailAddress == "" {
		loginName := strings.TrimSpace(githubUserProfile.Login)
		if loginName == "" {
			loginName = "github-user-" + strconv.FormatInt(githubUserProfile.ID, 10)
		}
		// use github's noreply address format
		userEmailAddress = loginName + "@users.noreply.github.com"
	}

	// build the principal for this github user
	githubPrincipal := auth.Principal{
		Sub:   fmt.Sprintf("github:%d", githubUserProfile.ID),
		Email: userEmailAddress,
		Roles: []string{"owner"},
	}

	// find or create the platform user record
	platformUser, _, err := provisionPlatformUserWithProvider(r, githubPrincipal, "github", oauthCallbackResult.AccessToken)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to provision GitHub user identity")
		return
	}

	// create a new session and mint a jwt for it
	newSessionID := auth.GenerateSessionID()
	signedJWTToken, tokenExpiresAt, err := tokenIssuer.MintSessionToken(
		platformUser.ID,
		githubPrincipal.Sub,
		githubPrincipal.Email,
		githubPrincipal.Roles,
		newSessionID,
	)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "failed to create auth session from GitHub sign-in")
		return
	}

	// persist the session to the database so we can revoke it on logout
	_, err = appStore.CreateAuthSession(r.Context(), store.CreateAuthSessionInput{
		SessionID: newSessionID,
		UserID:    platformUser.ID,
		ExpiresAt: tokenExpiresAt,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to persist auth session")
		return
	}

	// audit log the session creation
	_ = appStore.CreateAuditEvent(r.Context(), store.AuditEventInput{
		ActorUserID: platformUser.ID,
		EventType:   "auth.session.create",
		TargetType:  "auth_session",
		TargetID:    newSessionID,
		Status:      "success",
		Message:     "session issued via GitHub OAuth",
	})

	// redirect to the login page with the token in the hash fragment
	// frontend reads the hash and stores it in localStorage
	redirectQueryParams := url.Values{}
	redirectQueryParams.Set("session_token", signedJWTToken)
	http.Redirect(w, r, "/login#"+redirectQueryParams.Encode(), http.StatusTemporaryRedirect)
}

// isOAuthRetryableCookieError returns true for errors where the fix is "just click login again"
// these happen when the oauth state cookie expires or the browser blocks it
func isOAuthRetryableCookieError(err error) bool {
	if err == nil {
		return false
	}
	errMessageLower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(errMessageLower, "oauth state cookie is missing") ||
		strings.Contains(errMessageLower, "state does not match") ||
		strings.Contains(errMessageLower, "unable to get verifier")
}
