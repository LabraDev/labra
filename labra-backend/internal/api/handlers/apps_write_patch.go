package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"labra-backend/internal/api/store"
)

// PatchAppHandler handles PATCH /v1/apps/:id
// merges partial update fields into the current app record
func PatchAppHandler(w http.ResponseWriter, r *http.Request) {
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

	// decode the partial update body
	var requestBody updateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// need to load current state before we can merge the patch
	currentApp, err := appStore.GetAppByIDForUser(r.Context(), appID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "app not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load app")
		return
	}

	// merge what was sent with what we already have
	updatedAppInput, err := mergeAppUpdate(currentApp, requestBody)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	// only record config history if meaningful fields changed - not just timestamps
	shouldRecordConfigHistory := shouldRecordConfigHistoryForPatch(currentApp, updatedAppInput)

	// write the merged state back to the database
	savedApp, err := appStore.UpdateAppForUser(r.Context(), appID, userID, updatedAppInput)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "app not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to update app")
		return
	}

	// refresh infra output placeholder after any update
	_ = ensureAppInfraOutput(r.Context(), savedApp)
	if shouldRecordConfigHistory {
		_ = recordAppConfigVersion(r.Context(), savedApp, "patch")
	}

	writeJSON(w, http.StatusOK, savedApp)
}