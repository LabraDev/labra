package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"labra-backend/internal/api/auth"
	"labra-backend/internal/api/store"
)

// appStore is the global store instance - gets set once at startup via InitAppStore
var appStore *store.Store

// teardownAppInfraFn lets tests swap out the real infra teardown with a fake
var teardownAppInfraFn = teardownDeploymentInfra

// repoPattern validates that repo names look like "owner/repo" before we do anything with them
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// createAppRequest is what the frontend sends when creating a new app
type createAppRequest struct {
	Name              string `json:"name"`
	RepoFullName      string `json:"repo_full_name"`
	Branch            string `json:"branch"`
	BuildType         string `json:"build_type"`
	OutputDir         string `json:"output_dir"`
	RootDir           string `json:"root_dir"`
	SiteURL           string `json:"site_url"`
	AutoDeployEnabled *bool  `json:"auto_deploy_enabled"`
}

// updateAppRequest is for patch requests - all fields optional so we can do partial updates
type updateAppRequest struct {
	Name              *string `json:"name"`
	Branch            *string `json:"branch"`
	BuildType         *string `json:"build_type"`
	OutputDir         *string `json:"output_dir"`
	RootDir           *string `json:"root_dir"`
	SiteURL           *string `json:"site_url"`
	AutoDeployEnabled *bool   `json:"auto_deploy_enabled"`
}

// InitAppStore wires up the global store - called once from main
func InitAppStore(db *sql.DB) {
	appStore = store.New(db)
}

// readUserID grabs the authenticated user's id from request context
// returns false if the user isn't logged in
func readUserID(r *http.Request) (int64, bool) {
	if principal, ok := auth.PrincipalFromContext(r.Context()); ok && principal.UserID > 0 {
		return principal.UserID, true
	}
	return 0, false
}

// readIDFromPathOrQuery tries to get a numeric id from the url path or query string
// path format is /v1/<base>/<id> or query format is ?id=<id>
func readIDFromPathOrQuery(r *http.Request, basePath string) (int64, error) {
	// check query string first - ?id=123
	if rawIDString := strings.TrimSpace(r.URL.Query().Get("id")); rawIDString != "" {
		parsedID, err := strconv.ParseInt(rawIDString, 10, 64)
		if err != nil || parsedID <= 0 {
			return 0, fmt.Errorf("id must be a positive integer")
		}
		return parsedID, nil
	}

	// fall back to path parsing - /v1/apps/123
	urlPrefix := "/v1/" + strings.Trim(basePath, "/") + "/"
	urlPath := strings.TrimSpace(r.URL.Path)
	if !strings.HasPrefix(urlPath, urlPrefix) {
		return 0, fmt.Errorf("id not found in path")
	}

	rawPathSegment := strings.Trim(strings.TrimPrefix(urlPath, urlPrefix), "/")
	if rawPathSegment == "" {
		return 0, fmt.Errorf("id is required")
	}
	// take only the first path segment in case there are more segments after the id
	pathSegments := strings.Split(rawPathSegment, "/")
	firstSegment := strings.TrimSpace(pathSegments[0])
	if firstSegment == "" {
		return 0, fmt.Errorf("id is required")
	}

	parsedID, err := strconv.ParseInt(firstSegment, 10, 64)
	if err != nil || parsedID <= 0 {
		return 0, fmt.Errorf("id must be a positive integer")
	}
	return parsedID, nil
}

// writeJSONError sends a structured error response - all errors go through here for consistency
func writeJSONError(w http.ResponseWriter, statusCode int, errorMessage string) {
	writeJSON(w, statusCode, map[string]any{
		"error": map[string]any{
			"status":  statusCode,
			"message": errorMessage,
		},
	})
}

// writeJSON serializes body to json and sends it - sets cache headers to prevent stale responses
func writeJSON(w http.ResponseWriter, statusCode int, responseBody any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(responseBody)
}