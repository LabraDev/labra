package handlers

import (
	"context"
	"errors"
	"fmt"

	"labra-backend/internal/api/store"
)

// buildDeploymentRuntime assembles the credentials and config needed for a single deployment
// fetches github install token and aws assumed-role config in one place
func buildDeploymentRuntime(ctx context.Context, userID int64) (deploymentRuntime, error) {
	// make sure we have github app credentials configured at all
	if !githubAppConfigured() {
		return deploymentRuntime{}, fmt.Errorf("GitHub App credentials are not configured")
	}

	// look up the github app installation for this user
	githubInstallation, err := appStore.GetGitHubInstallationByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// user hasn't installed the github app yet - give them a helpful message
			return deploymentRuntime{}, fmt.Errorf("GitHub App installation missing. Install GitHub App and select repositories")
		}
		return deploymentRuntime{}, fmt.Errorf("failed to load GitHub installation: %w", err)
	}

	// generate a short-lived installation access token for cloning
	githubInstallToken, err := createGitHubInstallationAccessToken(githubInstallation.InstallationID)
	if err != nil {
		return deploymentRuntime{}, fmt.Errorf("failed to create GitHub installation access token: %w", err)
	}

	// find a validated aws connection to deploy into
	validatedAWSConn, err := selectValidatedAWSConnection(ctx, userID)
	if err != nil {
		if errors.Is(err, errNoValidatedAWSConnection) {
			return deploymentRuntime{}, fmt.Errorf("no validated AWS connection was found")
		}
		return deploymentRuntime{}, fmt.Errorf("failed to load AWS connection: %w", err)
	}

	// assume the role specified in the aws connection - gives us scoped credentials
	assumedRoleAWSConfig, err := loadAssumedRoleConfig(ctx, validatedAWSConn)
	if err != nil {
		return deploymentRuntime{}, fmt.Errorf("failed to load assumed-role AWS config: %w", err)
	}

	return deploymentRuntime{
		InstallationToken: githubInstallToken,
		AWSConnection:     validatedAWSConn,
		AWSConfig:         assumedRoleAWSConfig,
	}, nil
}