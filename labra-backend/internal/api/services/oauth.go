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

var oauthConfig = &oauth2.Config{}
var oauthEnabled bool

const (
	oauthStateCookieName    = "oauthstate"
	oauthVerifierCookieName = "oauthverifier"
)

type OAuthCallbackResult struct {
	UserBody    []byte
	AccessToken string
}

func InitOauth(gh_client, gh_secret, redirectURL string) {
	clientID := strings.TrimSpace(gh_client)
	clientSecret := strings.TrimSpace(gh_secret)
	if clientID == "" || clientSecret == "" {
		oauthConfig = &oauth2.Config{}
		oauthEnabled = false
		return
	}

	redirect := strings.TrimSpace(redirectURL)
	if redirect == "" {
		redirect = "http://localhost:8080/v1/callback"
	}

	oauthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"repo", "user"},
		Endpoint:     github.Endpoint,
		RedirectURL:  redirect,
	}

	oauthEnabled = true
}

func OAuthReady() bool {
	return oauthEnabled &&
		strings.TrimSpace(oauthConfig.ClientID) != "" &&
		strings.TrimSpace(oauthConfig.ClientSecret) != ""
}

func Authenticate(w http.ResponseWriter, r *http.Request) error {
	if !OAuthReady() {
		return fmt.Errorf("github oauth is not configured")
	}

	verifierStr := oauth2.GenerateVerifier()

	state, err := utils.GenerateState()
	if err != nil {
		return err
	}

	secureCookies := shouldUseSecureCookies(r)
	setOAuthCookie(w, oauthStateCookieName, state, secureCookies)
	setOAuthCookie(w, oauthVerifierCookieName, verifierStr, secureCookies)
	url := oauthConfig.AuthCodeURL(state, oauth2.S256ChallengeOption(verifierStr))

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)

	return nil
}

func CallbackWithToken(w http.ResponseWriter, r *http.Request) (OAuthCallbackResult, error) {
	if !OAuthReady() {
		return OAuthCallbackResult{}, fmt.Errorf("github oauth is not configured")
	}

	ctx := context.Background()
	secureCookies := shouldUseSecureCookies(r)

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		return OAuthCallbackResult{}, fmt.Errorf("error: code is unavaliable")
	}

	oauthState, err := r.Cookie(oauthStateCookieName)
	if err != nil {
		return OAuthCallbackResult{}, fmt.Errorf("error: oauth state cookie is missing")
	}
	if state != oauthState.Value {
		return OAuthCallbackResult{}, fmt.Errorf("error: state does not match")
	}

	verifierCookie, err := r.Cookie(oauthVerifierCookieName)
	if err != nil || strings.TrimSpace(verifierCookie.Value) == "" {
		return OAuthCallbackResult{}, fmt.Errorf("error: unable to get verifier")
	}
	defer clearOAuthCookie(w, oauthStateCookieName, secureCookies)
	defer clearOAuthCookie(w, oauthVerifierCookieName, secureCookies)

	tok, err := oauthConfig.Exchange(ctx, code, oauth2.VerifierOption(verifierCookie.Value))
	if err != nil {
		return OAuthCallbackResult{}, err
	}

	client := oauthConfig.Client(ctx, tok)

	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return OAuthCallbackResult{}, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return OAuthCallbackResult{}, fmt.Errorf("error: github user API returned status %d", resp.StatusCode)
	}

	return OAuthCallbackResult{
		UserBody:    body,
		AccessToken: strings.TrimSpace(tok.AccessToken),
	}, nil
}

func Callback(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	result, err := CallbackWithToken(w, r)
	if err != nil {
		return nil, err
	}
	return result.UserBody, nil
}

func requestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}

	return strings.EqualFold(strings.TrimSpace(r.Header.Get("CloudFront-Forwarded-Proto")), "https")
}

func shouldUseSecureCookies(r *http.Request) bool {
	if requestIsSecure(r) {
		return true
	}

	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(oauthConfig.RedirectURL)), "https://")
}

func setOAuthCookie(w http.ResponseWriter, name, value string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/v1/callback",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOAuthCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/v1/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
