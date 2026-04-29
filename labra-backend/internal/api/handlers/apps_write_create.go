package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

// CreateAppHandler handles POST /v1/apps
// validates github access, creates the app record, sets up infra output placeholder, and records config history
func CreateAppHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	userID, ok := readUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	// decode the request body first before doing any db work
	var requestBody createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// normalize and validate the input fields
	normalizedInput, err := normalizeCreateApp(requestBody)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	normalizedInput.UserID = userID

	// make sure the user actually has access to this github repo before we create the app
	if err := ensureUserCanTrackRepo(r.Context(), normalizedInput.UserID, normalizedInput.RepoFullName); err != nil {
		if accessErr, ok := err.(repoAccessErr); ok {
			writeJSONError(w, accessErr.Status, accessErr.Message)
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to validate GitHub repository access")
		return
	}

	// create the app record in the database
	createdApp, err := appStore.CreateApp(r.Context(), normalizedInput)
	if err != nil {
		// conflict means an app already exists for this repo+branch combo
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeJSONError(w, http.StatusConflict, "app already exists for repo+branch")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to create app")
		return
	}

	// kick off infra output setup and record config version in the background
	// we ignore errors here - these are best-effort and don't affect the response
	_ = ensureAppInfraOutput(r.Context(), createdApp)
	_ = recordAppConfigVersion(r.Context(), createdApp, "create")

	writeJSON(w, http.StatusCreated, createdApp)
}