package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"labra-backend/utils"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// oauthConfig holds the github oauth app credentials - set via InitOauth at startup
var oauthConfig = &oauth2.Config{}

// oauthEnabled tracks whether oauth was successfully configured
var oauthEnabled bool

const (
	// cookie names for the state and verifier we set during the oauth flow
	oauthStateCookieName    = "oauthstate"
	oauthVerifierCookieName = "oauthverifier"
)

// OAuthCallbackResult is what we return from the callback - the user's profile body and their access token
type OAuthCallbackResult struct {
	UserBody    []byte
	AccessToken string
}

// InitOauth configures the github oauth app credentials - called at startup
// if either client id or secret is missing we disable oauth entirely
func InitOauth(ghClientID, ghClientSecret, redirectURL string) {
	trimmedClientID := strings.TrimSpace(ghClientID)
	trimmedClientSecret := strings.TrimSpace(ghClientSecret)
	if trimmedClientID == "" || trimmedClientSecret == "" {
		// missing credentials means oauth can't work - disable it
		oauthConfig = &oauth2.Config{}
		oauthEnabled = false
		return
	}

	trimmedRedirectURL := strings.TrimSpace(redirectURL)
	if trimmedRedirectURL == "" {
		// fall back to localhost for local dev
		trimmedRedirectURL = "http://localhost:8080/v1/callback"
	}

	oauthConfig = &oauth2.Config{
		ClientID:     trimmedClientID,
		ClientSecret: trimmedClientSecret,
		// repo scope for cloning, user scope for profile info
		Scopes:      []string{"repo", "user"},
		Endpoint:    github.Endpoint,
		RedirectURL: trimmedRedirectURL,
	}

	oauthEnabled = true
}

// OAuthReady returns true if oauth is configured and usable
func OAuthReady() bool {
	return oauthEnabled &&
		strings.TrimSpace(oauthConfig.ClientID) != "" &&
		strings.TrimSpace(oauthConfig.ClientSecret) != ""
}

// Authenticate redirects the user to github to start the oauth flow
// sets cookies for state and pkce verifier to protect against csrf
func Authenticate(w http.ResponseWriter, r *http.Request) error {
	if !OAuthReady() {
		return fmt.Errorf("github oauth is not configured")
	}

	// generate pkce verifier for extra security
	pkceVerifierString := oauth2.GenerateVerifier()

	// generate random state for csrf protection
	csrfStateValue, err := utils.GenerateState()
	if err != nil {
		return err
	}

	// store both in short-lived cookies so we can verify them in the callback
	secureCookiesNeeded := shouldUseSecureCookies(r)
	setOAuthCookie(w, oauthStateCookieName, csrfStateValue, secureCookiesNeeded)
	setOAuthCookie(w, oauthVerifierCookieName, pkceVerifierString, secureCookiesNeeded)

	// redirect to github with the state and pkce challenge
	githubAuthURL := oauthConfig.AuthCodeURL(csrfStateValue, oauth2.S256ChallengeOption(pkceVerifierString))
	http.Redirect(w, r, githubAuthURL, http.StatusTemporaryRedirect)

	return nil
}

// CallbackWithToken completes the oauth exchange and returns the user's profile + access token
// validates state and verifier against the cookies we set in Authenticate
func CallbackWithToken(w http.ResponseWriter, r *http.Request) (OAuthCallbackResult, error) {
	if !OAuthReady() {
		return OAuthCallbackResult{}, fmt.Errorf("github oauth is not configured")
	}

	ctx := context.Background()
	secureCookiesNeeded := shouldUseSecureCookies(r)

	// github sends back a code and our original state
	oauthCodeParam := r.URL.Query().Get("code")
	returnedStateParam := r.URL.Query().Get("state")

	if oauthCodeParam == "" {
		return OAuthCallbackResult{}, fmt.Errorf("error: code is unavaliable")
	}

	// verify the state cookie matches what github sent back
	savedStateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil {
		return OAuthCallbackResult{}, fmt.Errorf("error: oauth state cookie is missing")
	}
	if returnedStateParam != savedStateCookie.Value {
		return OAuthCallbackResult{}, fmt.Errorf("error: state does not match")
	}

	// get the pkce verifier we stored in the cookie
	savedVerifierCookie, err := r.Cookie(oauthVerifierCookieName)
	if err != nil || strings.TrimSpace(savedVerifierCookie.Value) == "" {
		return OAuthCallbackResult{}, fmt.Errorf("error: unable to get verifier")
	}
	// clear the cookies now that we've used them
	defer clearOAuthCookie(w, oauthStateCookieName, secureCookiesNeeded)
	defer clearOAuthCookie(w, oauthVerifierCookieName, secureCookiesNeeded)

	// exchange the code for an access token
	githubAccessToken, err := oauthConfig.Exchange(ctx, oauthCodeParam, oauth2.VerifierOption(savedVerifierCookie.Value))
	if err != nil {
		return OAuthCallbackResult{}, err
	}

	// use the access token to fetch the user's github profile
	githubAPIClient := oauthConfig.Client(ctx, githubAccessToken)
	githubUserAPIResponse, err := githubAPIClient.Get("https://api.github.com/user")
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	defer githubUserAPIResponse.Body.Close()

	rawUserProfileBody, err := io.ReadAll(githubUserAPIResponse.Body)
	if err != nil {
		return OAuthCallbackResult{}, err
	}

	if githubUserAPIResponse.StatusCode >= http.StatusBadRequest {
		return OAuthCallbackResult{}, fmt.Errorf("error: github user API returned status %d", githubUserAPIResponse.StatusCode)
	}

	return OAuthCallbackResult{
		UserBody:    rawUserProfileBody,
		AccessToken: strings.TrimSpace(githubAccessToken.AccessToken),
	}, nil
}

// Callback is the simpler version that just returns the user body without the access token
func Callback(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	callbackResult, err := CallbackWithToken(w, r)
	if err != nil {
		return nil, err
	}
	return callbackResult.UserBody, nil
}

// requestIsSecure returns true if the request came in over https
// checks tls state, x-forwarded-proto, and cloudfront-forwarded-proto headers
func requestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}

	return strings.EqualFold(strings.TrimSpace(r.Header.Get("CloudFront-Forwarded-Proto")), "https")
}

// shouldUseSecureCookies returns true if we should set the Secure flag on cookies
// secure cookies only work on https so we check if the request is secure
func shouldUseSecureCookies(r *http.Request) bool {
	if requestIsSecure(r) {
		return true
	}
	// also use secure cookies if the redirect url is https even if the current request isn't
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(oauthConfig.RedirectURL)), "https://")
}

// setOAuthCookie sets a short-lived httponly cookie for oauth flow state
// 5 minute ttl is plenty since the oauth flow should complete quickly
func setOAuthCookie(w http.ResponseWriter, cookieName, cookieValue string, useSecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    cookieValue,
		Path:     "/v1/callback",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   useSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearOAuthCookie expires an oauth cookie immediately after the flow completes
func clearOAuthCookie(w http.ResponseWriter, cookieName string, useSecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/v1/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   useSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
