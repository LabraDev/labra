package services

import (
	"context"
	"fmt"
	"io"
	"labra-backend/utils"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

var oauthConfig = &oauth2.Config{}

var verifiers map[string]verifier

type verifier struct {
	verifier string
	ttl      time.Time
}

func InitOauth(gh_client, gh_secret, redirectURL string) {
	redirect := strings.TrimSpace(redirectURL)
	if redirect == "" {
		redirect = "http://localhost:8080/v1/callback"
	}

	oauthConfig = &oauth2.Config{
		ClientID:     gh_client,
		ClientSecret: gh_secret,
		Scopes:       []string{"repo", "user"},
		Endpoint:     github.Endpoint,
		RedirectURL:  redirect,
	}

	verifiers = map[string]verifier{}

	go cleanupVerifiers()
}

func Authenticate(w http.ResponseWriter, r *http.Request) error {
	fmt.Println("CLIENT ID:", oauthConfig.ClientID)

	clientIP := utils.GetClientIP(r)
	verifierStr := oauth2.GenerateVerifier()

	state, err := utils.GenerateState()
	if err != nil {
		return err
	}

	cookieState := http.Cookie{
		Name:    "oauthstate",
		Value:   state,
		Expires: time.Now().Add(30 * time.Second),
	}

	http.SetCookie(w, &cookieState)
	url := oauthConfig.AuthCodeURL(state, oauth2.S256ChallengeOption(verifierStr))

	newVerifier := verifier{
		verifier: verifierStr,
		ttl:      time.Now().Add(24 * time.Hour),
	}
	verifiers[clientIP] = newVerifier

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)

	return nil
}

func Callback(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	ctx := context.Background()
	clientIP := utils.GetClientIP(r)

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		return nil, fmt.Errorf("error: code is unavaliable")
	}

	oauthState, _ := r.Cookie("oauthstate")
	fmt.Println("OAUTH ->>>", oauthState)
	if state != oauthState.Value {
		return nil, fmt.Errorf("error: state does not match")
	}

	verf, ok := verifiers[clientIP]
	if !ok {
		return nil, fmt.Errorf("error: unable to get verifier")
	}

	fmt.Println(clientIP, ":", verf)
	tok, err := oauthConfig.Exchange(ctx, code, oauth2.VerifierOption(verf.verifier))
	if err != nil {
		return nil, err
	}

	client := oauthConfig.Client(ctx, tok)

	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func cleanupVerifiers() {
	for {
		time.Sleep(5 * time.Minute)

		for k, elm := range verifiers {
			if elm.ttl.Before(time.Now()) {
				delete(verifiers, k)
			}
		}
	}
}
