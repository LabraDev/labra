package handlers

import (
	"context"
	"net/http"
	"strings"

	"labra-backend/internal/api/store"
)

type repoAccessErr struct {
	Status  int
	Message string
}

func (e repoAccessErr) Error() string {
	return e.Message
}

func ensureUserCanTrackRepo(ctx context.Context, userID int64, repoFullName string) error {
	repo := strings.TrimSpace(repoFullName)
	if repo == "" {
		return repoAccessErr{Status: http.StatusBadRequest, Message: "repo_full_name is required"}
	}

	// If GitHub App credentials are not configured, keep existing fallback behavior.
	if !githubAppConfigured() {
		return nil
	}

	installation, err := appStore.GetGitHubInstallationByUserID(ctx, userID)
	if err != nil {
		if err == store.ErrNotFound {
			return repoAccessErr{
				Status:  http.StatusBadRequest,
				Message: "GitHub App installation missing. Install GitHub App and select repositories.",
			}
		}
		return repoAccessErr{Status: http.StatusInternalServerError, Message: "failed to load GitHub installation"}
	}

	installationToken, err := createGitHubInstallationAccessToken(installation.InstallationID)
	if err != nil {
		return repoAccessErr{Status: http.StatusBadGateway, Message: err.Error()}
	}

	repos, err := fetchGitHubInstallationRepositories(installationToken)
	if err != nil {
		return repoAccessErr{Status: http.StatusBadGateway, Message: err.Error()}
	}

	repoLower := strings.ToLower(repo)
	for _, candidate := range repos {
		if strings.ToLower(strings.TrimSpace(candidate.FullName)) == repoLower {
			return nil
		}
	}

	return repoAccessErr{
		Status:  http.StatusForbidden,
		Message: "Repository is not granted to your GitHub App installation. Update installation access and retry.",
	}
}
