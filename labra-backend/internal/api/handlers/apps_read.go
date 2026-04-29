package handlers

import (
	"errors"
	"net/http"

	"labra-backend/internal/api/store"
)

// ListAppsHandler returns all apps belonging to the current user
func ListAppsHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	userID, ok := readUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	// just pull everything for this user and return it
	apps, err := appStore.ListAppsByUser(r.Context(), userID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list apps")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"apps": apps})
}

// GetAppHandler returns a single app by id - only returns it if it belongs to the current user
func GetAppHandler(w http.ResponseWriter, r *http.Request) {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return
	}

	userID, ok := readUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "missing auth principal")
		return
	}

	appID, err := readIDFromPathOrQuery(r, "apps")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	app, err := appStore.GetAppByIDForUser(r.Context(), appID, userID)
	if err != nil {
		// 404 if the app doesn't exist or doesn't belong to this user
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "app not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load app")
		return
	}

	writeJSON(w, http.StatusOK, app)
}