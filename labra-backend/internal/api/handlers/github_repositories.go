package handlers

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"labra-backend/internal/api/auth"
	"labra-backend/internal/api/store"
)

type githubRepository struct {
	FullName      string `json:"full_name"`
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url"`
}

type githubBranch struct {
	Name      string `json:"name"`
	Protected bool   `json:"protected"`
}

type upsertGitHubInstallationRequest struct {
	InstallationID int64  `json:"installation_id"`
	AccountLogin   string `json:"account_login"`
	TargetType     string `json:"target_type"`
}

type githubInstallationTokenResponse struct {
	Token string `json:"token"`
}

type githubInstallationReposResponse struct {
	Repositories []githubRepository `json:"repositories"`
}

type githubRepoBranchesResponse struct {
	Branches []githubBranch `json:"branches"`
}

type githubAppRuntime struct {
	appID      int64
	appSlug    string
	privateKey *rsa.PrivateKey
}

var appGitHubRuntime githubAppRuntime

func InitGitHubAppRuntime(appIDRaw, privateKeyPEM, appSlug string) {
	runtime := githubAppRuntime{
		appSlug: strings.TrimSpace(appSlug),
	}

	parsedID, err := strconv.ParseInt(strings.TrimSpace(appIDRaw), 10, 64)
	if err == nil && parsedID > 0 {
		runtime.appID = parsedID
	}
	runtime.privateKey = parseGitHubPrivateKey(privateKeyPEM)
	appGitHubRuntime = runtime
}

func parseGitHubPrivateKey(privateKeyPEM string) *rsa.PrivateKey {
	normalized := strings.TrimSpace(privateKeyPEM)
	if strings.Contains(normalized, `\n`) {
		normalized = strings.ReplaceAll(normalized, `\n`, "\n")
	}
	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil
	}
	return key
}

func githubAppConfigured() bool {
	return appGitHubRuntime.appID > 0 && appGitHubRuntime.privateKey != nil
}

func GetGitHubAppInstallURLHandler(w http.ResponseWriter, _ *http.Request) {
	if strings.TrimSpace(appGitHubRuntime.appSlug) == "" {
		writeJSONError(w, http.StatusServiceUnavailable, "GitHub App slug is not configured.")
		return
	}
	installURL := fmt.Sprintf("https://github.com/apps/%s/installations/new", url.PathEscape(strings.TrimSpace(appGitHubRuntime.appSlug)))
	writeJSON(w, http.StatusOK, map[string]any{
		"install_url": installURL,
	})
}

func UpsertGitHubInstallationHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || principal.UserID <= 0 {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	var in upsertGitHubInstallationRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if in.InstallationID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "installation_id must be a positive integer")
		return
	}

	installation, err := appStore.UpsertGitHubInstallation(r.Context(), store.UpsertGitHubInstallationInput{
		UserID:         principal.UserID,
		InstallationID: in.InstallationID,
		AccountLogin:   strings.TrimSpace(in.AccountLogin),
		TargetType:     strings.TrimSpace(in.TargetType),
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save GitHub installation")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"installation": installation,
	})
}

func ListGitHubRepositoriesHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || principal.UserID <= 0 {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	if !githubAppConfigured() {
		writeJSONError(w, http.StatusServiceUnavailable, "GitHub App credentials are not configured.")
		return
	}

	installation, err := appStore.GetGitHubInstallationByUserID(r.Context(), principal.UserID)
	if err != nil {
		if err == store.ErrNotFound {
			writeJSONError(w, http.StatusBadRequest, "GitHub App installation missing. Install GitHub App and select repositories.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load GitHub installation")
		return
	}

	installationToken, err := createGitHubInstallationAccessToken(installation.InstallationID)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	repos, err := fetchGitHubInstallationRepositories(installationToken)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"repositories": repos,
	})
}

func ListGitHubBranchesHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || principal.UserID <= 0 {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	repoFullName := strings.TrimSpace(r.URL.Query().Get("repo_full_name"))
	if repoFullName == "" {
		writeJSONError(w, http.StatusBadRequest, "repo_full_name query param is required")
		return
	}
	if !repoPattern.MatchString(repoFullName) {
		writeJSONError(w, http.StatusBadRequest, "repo_full_name must look like owner/repo")
		return
	}

	if !githubAppConfigured() {
		writeJSONError(w, http.StatusServiceUnavailable, "GitHub App credentials are not configured.")
		return
	}

	installation, err := appStore.GetGitHubInstallationByUserID(r.Context(), principal.UserID)
	if err != nil {
		if err == store.ErrNotFound {
			writeJSONError(w, http.StatusBadRequest, "GitHub App installation missing. Install GitHub App and select repositories.")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load GitHub installation")
		return
	}

	installationToken, err := createGitHubInstallationAccessToken(installation.InstallationID)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	branches, err := fetchGitHubRepoBranches(installationToken, repoFullName)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"branches": branches,
	})
}

func signGitHubAppJWT() (string, error) {
	now := time.Now().Unix()
	claims := jwt.MapClaims{
		"iat": now - 60,
		"exp": now + 540,
		"iss": strconv.FormatInt(appGitHubRuntime.appID, 10),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(appGitHubRuntime.privateKey)
}

func createGitHubInstallationAccessToken(installationID int64) (string, error) {
	appJWT, err := signGitHubAppJWT()
	if err != nil {
		return "", fmt.Errorf("failed to create GitHub App token")
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID), nil)
	if err != nil {
		return "", fmt.Errorf("failed to build GitHub API request")
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(appJWT))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "labra-control-api")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to reach GitHub API")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read GitHub API response")
	}

	if resp.StatusCode >= http.StatusBadRequest {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("GitHub App credentials are invalid.")
		}
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("GitHub App installation not found. Reinstall GitHub App and retry.")
		}
		return "", fmt.Errorf("GitHub API error (%d)", resp.StatusCode)
	}

	var tokenResp githubInstallationTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse GitHub installation token")
	}
	token := strings.TrimSpace(tokenResp.Token)
	if token == "" {
		return "", fmt.Errorf("GitHub installation token was empty")
	}
	return token, nil
}

func fetchGitHubInstallationRepositories(installationToken string) ([]githubRepository, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/installation/repositories?per_page=100", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build GitHub API request")
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(installationToken))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "labra-control-api")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach GitHub API")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read GitHub API response")
	}

	if resp.StatusCode >= http.StatusBadRequest {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("GitHub App installation access denied. Update installation permissions and retry.")
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("GitHub App installation not found. Reinstall GitHub App and retry.")
		}
		return nil, fmt.Errorf("GitHub API error (%d)", resp.StatusCode)
	}

	var raw githubInstallationReposResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub repositories")
	}

	out := make([]githubRepository, 0, len(raw.Repositories))
	for _, repo := range raw.Repositories {
		repo.FullName = strings.TrimSpace(repo.FullName)
		repo.Name = strings.TrimSpace(repo.Name)
		repo.DefaultBranch = strings.TrimSpace(repo.DefaultBranch)
		repo.HTMLURL = strings.TrimSpace(repo.HTMLURL)
		if repo.FullName == "" {
			continue
		}
		if repo.DefaultBranch == "" {
			repo.DefaultBranch = "main"
		}
		out = append(out, repo)
	}
	return out, nil
}

func fetchGitHubRepoBranches(installationToken, repoFullName string) ([]githubBranch, error) {
	parts := strings.SplitN(strings.TrimSpace(repoFullName), "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("repo_full_name must look like owner/repo")
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/branches?per_page=100",
		url.PathEscape(owner),
		url.PathEscape(repo),
	)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build GitHub API request")
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(installationToken))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "labra-control-api")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach GitHub API")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read GitHub API response")
	}

	if resp.StatusCode >= http.StatusBadRequest {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("GitHub App installation access denied. Update installation permissions and retry.")
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("Repository is not granted to your GitHub App installation. Update installation access and retry.")
		}
		return nil, fmt.Errorf("GitHub API error (%d)", resp.StatusCode)
	}

	var raw []githubBranch
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub branches")
	}

	out := make([]githubBranch, 0, len(raw))
	for _, b := range raw {
		b.Name = strings.TrimSpace(b.Name)
		if b.Name == "" {
			continue
		}
		out = append(out, b)
	}
	return out, nil
}
