package handlers

import (
	"context"
	"fmt"
	"strings"

	"labra-backend/internal/api/store"
)

// runManualDeployment is the core deploy function - runs in a goroutine for async deploys
// marks running, executes the pipeline, then marks succeeded/failed/awaiting_site_url
func runManualDeployment(deploymentID int64, app store.App) {
	ctx := context.Background()
	startedAtTimestamp := store.UnixNow()

	// mark as running right away so the frontend can show progress
	_, _ = appStore.UpdateDeploymentStatus(ctx, deploymentID, "running", "", app.SiteURL, startedAtTimestamp, 0)

	// try to load the deployment to pick up the branch that was stored at queue time
	storedDeployment, loadDeployErr := appStore.GetDeploymentByIDForUser(ctx, deploymentID, app.UserID)
	branchToCheckout := strings.TrimSpace(app.Branch)
	if loadDeployErr == nil {
		// use branch from stored deployment if it has one - it may differ from app.Branch for retries
		if candidateBranch := strings.TrimSpace(storedDeployment.Branch); candidateBranch != "" {
			branchToCheckout = candidateBranch
		}
	}

	// we only support static builds right now
	if app.BuildType != "static" {
		finishedAtTimestamp := store.UnixNow()
		_, _ = appStore.UpdateDeploymentStatus(ctx, deploymentID, "failed", "unsupported build type", "", startedAtTimestamp, finishedAtTimestamp)
		_ = appStore.CreateDeploymentLog(ctx, deploymentID, "error", "deployment failed: unsupported build type")
		return
	}

	// run the actual build + upload pipeline
	pipelineResult, err := executeDeploymentPipelineFn(ctx, deploymentID, app, branchToCheckout)
	if err != nil {
		finishedAtTimestamp := store.UnixNow()
		_, _ = appStore.UpdateDeploymentStatus(ctx, deploymentID, "failed", err.Error(), "", startedAtTimestamp, finishedAtTimestamp)
		_ = appStore.CreateDeploymentLog(ctx, deploymentID, "error", fmt.Sprintf("deployment failed: %v", err))
		return
	}

	finishedAtTimestamp := store.UnixNow()
	resultSiteURL := strings.TrimSpace(pipelineResult.SiteURL)

	// awaiting_site_url means we uploaded everything but cloudfront isn't reachable yet
	if pipelineResult.Status == "awaiting_site_url" {
		_, _ = appStore.UpdateDeploymentStatus(ctx, deploymentID, "awaiting_site_url", "", resultSiteURL, startedAtTimestamp, finishedAtTimestamp)
		if resultSiteURL != "" {
			// start a background monitor that polls until cloudfront is reachable
			go monitorAwaitingSiteURL(deploymentID, app.UserID, resultSiteURL)
		}
		return
	}

	// fully successful deploy
	_ = appStore.CreateDeploymentLog(ctx, deploymentID, "info", "deployment completed successfully")
	_, _ = appStore.UpdateDeploymentStatus(ctx, deploymentID, "succeeded", "", resultSiteURL, startedAtTimestamp, finishedAtTimestamp)
}

// triggerDeployment fires the deploy - async by default, sync in tests
func triggerDeployment(deploymentID int64, app store.App) {
	if runDeploymentAsync {
		// run in a goroutine so the http handler returns immediately
		go runManualDeployment(deploymentID, app)
		return
	}
	// synchronous path used in tests so we don't need to sleep
	runManualDeployment(deploymentID, app)
}