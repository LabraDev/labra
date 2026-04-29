package handlers

import (
	"net/http"
)

// GetAppDeploysHandler handles GET /v1/apps/:id/deploys
// returns the full deployment history for an app
func GetAppDeploysHandler(w http.ResponseWriter, r *http.Request) {
	if !ensureAppStore(w) {
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	appID, ok := readAppIDFromRequest(w, r)
	if !ok {
		return
	}

	// verify the app belongs to this user before showing deploys
	app, ok := loadAppForUser(w, r, appID, userID)
	if !ok {
		return
	}

	// pull all deployments for this app
	deploymentsList, err := appStore.ListDeploymentsByAppForUser(r.Context(), app.ID, userID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load app deployments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"app_id":      app.ID,
		"app_name":    app.Name,
		"repo":        app.RepoFullName,
		"branch":      app.Branch,
		"deployments": deploymentsList,
	})
}
