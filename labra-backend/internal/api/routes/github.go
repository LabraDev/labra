package routes

import (
	"labra-backend/internal/api/handlers"

	"github.com/go-fuego/fuego"
)

func GitHubRoutes(s *fuego.Server) {
	fuego.GetStd(s, "/v1/github/install-url", withAuth(handlers.GetGitHubAppInstallURLHandler))
	fuego.PostStd(s, "/v1/github/installation", withAuth(handlers.UpsertGitHubInstallationHandler))
	fuego.GetStd(s, "/v1/github/repositories", withAuth(handlers.ListGitHubRepositoriesHandler))
	fuego.GetStd(s, "/v1/github/branches", withAuth(handlers.ListGitHubBranchesHandler))
}
