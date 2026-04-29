package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"labra-backend/internal/api/store"
)

// runDeploymentAsync controls whether deploys fire in a goroutine or block - tests set this to false
var runDeploymentAsync = true

// executeDeploymentPipelineFn lets tests swap in a fake pipeline without real aws calls
var executeDeploymentPipelineFn = executeDeploymentPipeline

// missingUserIDError is the message we send when the auth principal is not in context
const missingUserIDError = "missing auth principal"

// ensureAppStore checks the global store is ready - saves repeating the nil check everywhere
func ensureAppStore(w http.ResponseWriter) bool {
	if appStore == nil {
		writeJSONError(w, http.StatusInternalServerError, "store not initialized")
		return false
	}
	return true
}

// readAppIDFromRequest grabs the app id from the request path or query string
func readAppIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	appID, err := readIDFromPathOrQuery(r, "apps")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return 0, false
	}
	return appID, true
}

// readDeploymentIDFromRequest grabs the deployment id from the request path or query string
func readDeploymentIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	deploymentID, err := readIDFromPathOrQuery(r, "deploys")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return 0, false
	}
	return deploymentID, true
}

// requireUserID is a helper that writes a 401 if the user id is missing from context
func requireUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, ok := readUserID(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, missingUserIDError)
		return 0, false
	}
	return userID, true
}

// loadAppForUser fetches an app and checks it belongs to the given user
// writes 404 or 500 and returns false if anything goes wrong
func loadAppForUser(w http.ResponseWriter, r *http.Request, appID, userID int64) (store.App, bool) {
	app, err := appStore.GetAppByIDForUser(r.Context(), appID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "app not found")
			return store.App{}, false
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load app")
		return store.App{}, false
	}
	return app, true
}

// loadDeploymentForUser fetches a deployment and verifies it belongs to the given user
func loadDeploymentForUser(w http.ResponseWriter, r *http.Request, deploymentID, userID int64) (store.Deployment, bool) {
	deployment, err := appStore.GetDeploymentByIDForUser(r.Context(), deploymentID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "deployment not found")
			return store.Deployment{}, false
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to load deployment")
		return store.Deployment{}, false
	}
	return deployment, true
}

// validateDeployEligibility checks if an app is in a deployable state
// right now we only support static builds and require an output dir
func validateDeployEligibility(app store.App) error {
	if strings.TrimSpace(app.BuildType) != "static" {
		return fmt.Errorf("app is not eligible for deployment: unsupported build_type")
	}
	if strings.TrimSpace(app.OutputDir) == "" {
		return fmt.Errorf("app is not eligible for deployment: output_dir is required")
	}
	return nil
}

// queueDeployment creates a deployment record and fires the pipeline
// fills in defaults from the app if the input fields are empty
func queueDeployment(ctx context.Context, app store.App, deployInput store.CreateDeploymentInput, queueLogMessage string) (store.Deployment, error) {
	if appStore == nil {
		return store.Deployment{}, fmt.Errorf("store not initialized")
	}

	// fill in defaults from the app record if not set in the input
	if deployInput.AppID <= 0 {
		deployInput.AppID = app.ID
	}
	if deployInput.UserID <= 0 {
		deployInput.UserID = app.UserID
	}
	if strings.TrimSpace(deployInput.Status) == "" {
		deployInput.Status = "queued"
	}
	if strings.TrimSpace(deployInput.Branch) == "" {
		deployInput.Branch = app.Branch
	}
	if strings.TrimSpace(deployInput.SiteURL) == "" {
		deployInput.SiteURL = app.SiteURL
	}

	// write the deployment record to the database
	createdDeployment, err := appStore.CreateDeployment(ctx, deployInput)
	if err != nil {
		return store.Deployment{}, err
	}

	// log a message to the deployment log if one was provided
	if strings.TrimSpace(queueLogMessage) != "" {
		_ = appStore.CreateDeploymentLog(ctx, createdDeployment.ID, "info", queueLogMessage)
	}
	// kick off the actual pipeline - this may run async
	triggerDeployment(createdDeployment.ID, app)
	return createdDeployment, nil
}

// appSiteURLOrDefault returns the app's site url, trimmed
func appSiteURLOrDefault(app store.App) string {
	siteURLValue := strings.TrimSpace(app.SiteURL)
	return siteURLValue
}

// CreateDeployHandler handles POST /v1/apps/:id/deploy
// queues a manual deployment for the given app
func CreateDeployHandler(w http.ResponseWriter, r *http.Request) {
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

	// make sure the app exists and belongs to this user
	app, ok := loadAppForUser(w, r, appID, userID)
	if !ok {
		return
	}
	if err := validateDeployEligibility(app); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// create the deployment record and fire the pipeline
	createdDeployment, err := queueDeployment(r.Context(), app, store.CreateDeploymentInput{
		TriggerType:   "manual",
		CorrelationID: fmt.Sprintf("manual-%d", time.Now().UnixNano()),
	}, "deployment queued by manual trigger")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	// 202 because the deploy runs async - it's not done yet
	writeJSON(w, http.StatusAccepted, map[string]any{
		"deployment": createdDeployment,
	})
}

// CancelDeployHandler handles POST /v1/deploys/:id/cancel
// only works on queued or running deployments
func CancelDeployHandler(w http.ResponseWriter, r *http.Request) {
	if !ensureAppStore(w) {
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deploymentID, ok := readDeploymentIDFromRequest(w, r)
	if !ok {
		return
	}

	currentDeployment, ok := loadDeploymentForUser(w, r, deploymentID, userID)
	if !ok {
		return
	}

	// check what state the deployment is in before trying to cancel
	switch strings.TrimSpace(strings.ToLower(currentDeployment.Status)) {
	case "queued", "running":
		// these are the only states where cancel makes sense
	case "canceled":
		// already canceled - just return it as-is
		writeJSON(w, http.StatusOK, map[string]any{
			"deployment": currentDeployment,
		})
		return
	default:
		writeJSONError(w, http.StatusConflict, "deployment cannot be canceled in current status")
		return
	}

	// stamp the finish time and update status
	finishedAtTimestamp := store.UnixNow()
	updatedDeployment, err := appStore.UpdateDeploymentStatus(r.Context(), deploymentID, "canceled", "canceled by user", currentDeployment.SiteURL, currentDeployment.StartedAt, finishedAtTimestamp)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to cancel deployment")
		return
	}
	_ = appStore.CreateDeploymentLog(r.Context(), deploymentID, "warn", "deployment canceled by user")

	writeJSON(w, http.StatusOK, map[string]any{
		"deployment": updatedDeployment,
	})
}

// RetryDeployHandler handles POST /v1/deploys/:id/retry
// creates a new deployment copying commit info from the original
func RetryDeployHandler(w http.ResponseWriter, r *http.Request) {
	if !ensureAppStore(w) {
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deploymentID, ok := readDeploymentIDFromRequest(w, r)
	if !ok {
		return
	}

	// load the previous deployment to copy commit info from it
	previousDeployment, ok := loadDeploymentForUser(w, r, deploymentID, userID)
	if !ok {
		return
	}

	// can only retry failed or canceled deploys
	previousStatus := strings.TrimSpace(strings.ToLower(previousDeployment.Status))
	if previousStatus != "failed" && previousStatus != "canceled" {
		writeJSONError(w, http.StatusConflict, "deployment can only be retried from failed or canceled status")
		return
	}

	// make sure the app still exists
	app, ok := loadAppForUser(w, r, previousDeployment.AppID, userID)
	if !ok {
		return
	}

	// create a new deployment carrying over the commit info from the previous one
	retryDeployment, err := queueDeployment(r.Context(), app, store.CreateDeploymentInput{
		TriggerType:   "manual_retry",
		CommitSHA:     previousDeployment.CommitSHA,
		CommitMessage: previousDeployment.CommitMessage,
		CommitAuthor:  previousDeployment.CommitAuthor,
		CorrelationID: fmt.Sprintf("retry-%d-%d", previousDeployment.ID, time.Now().UnixNano()),
	}, fmt.Sprintf("deployment queued by retry (from deployment %d)", previousDeployment.ID))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create retry deployment")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"retried_from": previousDeployment.ID,
		"deployment":   retryDeployment,
	})
}

// GetDeployHandler handles GET /v1/deploys/:id
// returns a single deployment by id
func GetDeployHandler(w http.ResponseWriter, r *http.Request) {
	if !ensureAppStore(w) {
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deploymentID, ok := readDeploymentIDFromRequest(w, r)
	if !ok {
		return
	}

	deployment, ok := loadDeploymentForUser(w, r, deploymentID, userID)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, deployment)
}

// GetDeployLogsHandler handles GET /v1/deploys/:id/logs
// returns all log lines for a deployment
func GetDeployLogsHandler(w http.ResponseWriter, r *http.Request) {
	if !ensureAppStore(w) {
		return
	}

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	deploymentID, ok := readDeploymentIDFromRequest(w, r)
	if !ok {
		return
	}

	// verify ownership before showing logs
	if _, ok := loadDeploymentForUser(w, r, deploymentID, userID); !ok {
		return
	}

	// pull all log lines for this deployment
	deploymentLogs, err := appStore.ListDeploymentLogs(r.Context(), deploymentID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load deployment logs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deployment_id": deploymentID,
		"logs":          deploymentLogs,
	})
}