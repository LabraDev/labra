package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"labra-backend/internal/api/store"
)

// DeleteAppHandler handles DELETE /v1/apps/:id
// tears down aws infrastructure first then deletes the app record
// this can be slow since cloudfront distributions take a while to disable
func DeleteAppHandler(w http.ResponseWriter, r *http.Request) {
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

	// load the app to make sure it belongs to this user before we do anything destructive
	appToDelete, err := appStore.GetAppByIDForUser(r.Context(), appID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "app not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load app")
		return
	}

	// use a long timeout here - cloudfront teardown can take up to 15+ minutes in the worst case
	deleteContext, cancelDelete := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancelDelete()

	// tear down s3 and cloudfront before we remove the db record
	if err := teardownAppInfraFn(deleteContext, appToDelete); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete cloud infrastructure: %v", err))
		return
	}

	// infrastructure is gone, now remove the db record
	if err := appStore.DeleteAppForUser(deleteContext, appID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "app not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to delete app")
		return
	}

	// 204 means success with no body
	w.WriteHeader(http.StatusNoContent)
}