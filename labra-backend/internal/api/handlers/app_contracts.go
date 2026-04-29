package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"labra-backend/internal/api/store"
)

func recordAppConfigVersion(ctx context.Context, app store.App, source string) error {
	payload := map[string]any{
		"id":                  app.ID,
		"name":                app.Name,
		"repo_full_name":      app.RepoFullName,
		"branch":              app.Branch,
		"build_type":          app.BuildType,
		"output_dir":          app.OutputDir,
		"root_dir":            app.RootDir,
		"site_url":            app.SiteURL,
		"auto_deploy_enabled": app.AutoDeployEnabled,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = appStore.CreateAppConfigVersion(ctx, store.CreateAppConfigVersionInput{
		AppID:      app.ID,
		UserID:     app.UserID,
		Source:     strings.TrimSpace(source),
		ConfigJSON: string(encoded),
	})
	return err
}

func ensureAppInfraOutput(ctx context.Context, app store.App) error {
	bucketName := buildBucketName(app)
	siteURL := appSiteURLOrDefault(app)
	distributionID := ""

	existing, err := appStore.GetAppInfraOutputByAppForUser(ctx, app.ID, app.UserID)
	if err == nil {
		distributionID = strings.TrimSpace(existing.DistributionID)
		if strings.TrimSpace(siteURL) == "" {
			siteURL = strings.TrimSpace(existing.SiteURL)
		}
	}

	_, err = appStore.UpsertAppInfraOutput(ctx, store.UpsertAppInfraOutputInput{
		AppID:          app.ID,
		UserID:         app.UserID,
		BucketName:     bucketName,
		DistributionID: distributionID,
		SiteURL:        siteURL,
	})
	return err
}

func buildBucketName(app store.App) string {
	prefix := fmt.Sprintf("labra-%d-%d-%s", app.UserID, app.ID, slugify(app.Name))
	prefix = strings.ToLower(prefix)
	prefix = strings.ReplaceAll(prefix, "_", "-")
	prefix = strings.ReplaceAll(prefix, ".", "-")
	if len(prefix) > 63 {
		prefix = prefix[:63]
	}
	prefix = strings.Trim(prefix, "-")
	if prefix == "" {
		return fmt.Sprintf("labra-%d-%d", app.UserID, app.ID)
	}
	return prefix
}

func GetAppConfigHistoryHandler(w http.ResponseWriter, r *http.Request) {
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

	_, err = appStore.GetAppByIDForUser(r.Context(), appID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "app not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load app")
		return
	}

	history, err := appStore.ListAppConfigVersionsByAppForUser(r.Context(), appID, userID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load app config history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"app_id":          appID,
		"config_versions": history,
	})
}

func GetAppInfraOutputsHandler(w http.ResponseWriter, r *http.Request) {
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
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "app not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load app")
		return
	}

	outputs, err := appStore.GetAppInfraOutputByAppForUser(r.Context(), appID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			if ensureErr := ensureAppInfraOutput(r.Context(), app); ensureErr != nil {
				writeJSONError(w, http.StatusInternalServerError, "failed to derive infra outputs")
				return
			}
			outputs, err = appStore.GetAppInfraOutputByAppForUser(r.Context(), appID, userID)
		}
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load app infra outputs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"app_id": appID,
		"outputs": map[string]any{
			"bucket_name":     outputs.BucketName,
			"distribution_id": outputs.DistributionID,
			"site_url":        outputs.SiteURL,
			"updated_at":      outputs.UpdatedAt,
		},
	})
}
