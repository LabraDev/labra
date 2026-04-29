package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"labra-backend/internal/api/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

// invalidateCloudFrontDistribution tells cloudfront to clear its cache for all paths
// returns the invalidation id for logging
func invalidateCloudFrontDistribution(ctx context.Context, cfg aws.Config, distributionID string) (string, error) {
	cfClient := cloudfront.NewFromConfig(cfg)
	// caller reference needs to be unique per invalidation request
	callerReferenceValue := fmt.Sprintf("labra-%d", time.Now().UnixNano())
	allPathsPattern := "/*"

	invalidationResponse, err := cfClient.CreateInvalidation(ctx, &cloudfront.CreateInvalidationInput{
		DistributionId: &distributionID,
		InvalidationBatch: &cftypes.InvalidationBatch{
			CallerReference: &callerReferenceValue,
			Paths: &cftypes.Paths{
				Quantity: int32Ptr(1),
				Items:    []string{allPathsPattern},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("create cloudfront invalidation: %w", err)
	}
	invalidationIDValue := strings.TrimSpace(strVal(invalidationResponse.Invalidation.Id))
	if invalidationIDValue == "" {
		invalidationIDValue = "unknown"
	}
	return invalidationIDValue, nil
}

// monitorAwaitingSiteURL polls cloudfront until the site becomes reachable
// runs in a goroutine after a deployment that finished uploading but cloudfront wasn't ready yet
func monitorAwaitingSiteURL(deploymentID, userID int64, siteURL string) {
	trimmedSiteURL := strings.TrimSpace(siteURL)
	if trimmedSiteURL == "" {
		return
	}

	// give it up to 12 minutes - cloudfront can be slow to propagate sometimes
	monitorCtx, cancelMonitor := context.WithTimeout(context.Background(), awaitingSiteMonitorTimeout)
	defer cancelMonitor()
	pollTicker := time.NewTicker(awaitingSiteMonitorInterval)
	defer pollTicker.Stop()

	_ = appStore.CreateDeploymentLog(monitorCtx, deploymentID, "info", "monitoring cloudfront URL until it becomes reachable")
	for {
		if isSiteReachable(trimmedSiteURL) {
			// site is up - mark the deployment as succeeded
			currentDeployment, err := appStore.GetDeploymentByIDForUser(monitorCtx, deploymentID, userID)
			if err != nil {
				return
			}
			currentStatus := strings.TrimSpace(strings.ToLower(currentDeployment.Status))
			// only update if still in a waiting state - user might have canceled it
			if currentStatus != "awaiting_site_url" && currentStatus != "running" {
				return
			}
			_, _ = appStore.UpdateDeploymentStatus(monitorCtx, deploymentID, "succeeded", "", trimmedSiteURL, currentDeployment.StartedAt, store.UnixNow())
			_ = appStore.CreateDeploymentLog(monitorCtx, deploymentID, "info", "cloudfront URL reachable; deployment marked succeeded")
			return
		}

		select {
		case <-monitorCtx.Done():
			// timed out waiting for cloudfront - leave it in awaiting_site_url for user to check
			_ = appStore.CreateDeploymentLog(
				context.Background(),
				deploymentID,
				"warn",
				"cloudfront URL is still unreachable; deployment remains in awaiting_site_url",
			)
			return
		case <-pollTicker.C:
			// tick - try again
		}
	}
}

// isSiteReachable makes a quick http get to see if the site responds with anything useful
// anything below 5xx means the site is up and serving something
func isSiteReachable(siteURL string) bool {
	trimmedURL := strings.TrimSpace(siteURL)
	if trimmedURL == "" {
		return false
	}
	// short timeout so we don't block the poller too long
	probeCtx, cancelProbe := context.WithTimeout(context.Background(), siteProbeTimeout)
	defer cancelProbe()

	probeRequest, err := http.NewRequestWithContext(probeCtx, http.MethodGet, trimmedURL, nil)
	if err != nil {
		return false
	}
	// tell cloudfront not to serve from its own cache for this check
	probeRequest.Header.Set("Cache-Control", "no-cache")
	probeRequest.Header.Set("Pragma", "no-cache")

	probeResponse, err := http.DefaultClient.Do(probeRequest)
	if err != nil {
		return false
	}
	defer probeResponse.Body.Close()
	// treat anything below 500 as reachable - 4xx is fine, means the site is there
	return probeResponse.StatusCode >= http.StatusOK && probeResponse.StatusCode < http.StatusInternalServerError
}