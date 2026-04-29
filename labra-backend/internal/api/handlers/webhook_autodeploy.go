package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"labra-backend/internal/api/store"
)

// enqueueWebhookDeployments kicks off deployments for all eligible apps from a push event
// returns a list of app_id/deployment_id pairs that were triggered
func enqueueWebhookDeployments(
	r *http.Request,
	pushPayload githubPushEvent,
	eligibleApps []map[string]any,
) ([]map[string]any, error) {
	triggeredDeployments := make([]map[string]any, 0, len(eligibleApps))

	// extract commit info from the push payload to store on each deployment
	pushedBranch, _ := extractBranch(pushPayload.Ref)
	commitSHAValue := strings.TrimSpace(pushPayload.After)
	if commitSHAValue == "" {
		// fall back to head_commit.id if after is somehow empty
		commitSHAValue = strings.TrimSpace(pushPayload.HeadCommit.ID)
	}
	commitMessageValue := strings.TrimSpace(pushPayload.HeadCommit.Message)
	commitAuthorValue := strings.TrimSpace(pushPayload.HeadCommit.Author.Name)
	githubDeliveryID := strings.TrimSpace(r.Header.Get("X-GitHub-Delivery"))

	for _, eligibleApp := range eligibleApps {
		// pull out the app id and user id from the eligible app map
		appIDValue, err := mustInt64(eligibleApp["id"])
		if err != nil {
			return nil, fmt.Errorf("invalid eligible app id: %w", err)
		}
		appUserIDValue, err := mustInt64(eligibleApp["user_id"])
		if err != nil {
			return nil, fmt.Errorf("invalid eligible app user_id: %w", err)
		}

		// reload the app from the database to get the latest version
		appRecord, err := appStore.GetAppByIDForUser(r.Context(), appIDValue, appUserIDValue)
		if err != nil {
			return nil, fmt.Errorf("failed to load app for webhook deploy: %w", err)
		}

		// queue the deployment with a correlation id that ties it back to this delivery
		queuedDeployment, err := queueDeployment(r.Context(), appRecord, store.CreateDeploymentInput{
			TriggerType:   "webhook",
			CommitSHA:     commitSHAValue,
			CommitMessage: commitMessageValue,
			CommitAuthor:  commitAuthorValue,
			Branch:        pushedBranch,
			// include delivery id and timestamp to make the correlation id unique
			CorrelationID: fmt.Sprintf("webhook-%s-%d-%d", githubDeliveryID, appRecord.ID, time.Now().UnixNano()),
		}, "deployment queued by webhook trigger")
		if err != nil {
			return nil, fmt.Errorf("failed to create webhook deployment: %w", err)
		}

		triggeredDeployments = append(triggeredDeployments, map[string]any{
			"app_id":        appRecord.ID,
			"deployment_id": queuedDeployment.ID,
		})
	}

	return triggeredDeployments, nil
}
