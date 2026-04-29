package routes

import (
	"labra-backend/internal/api/handlers"

	"github.com/go-fuego/fuego"
)

func AuthRoutes(s *fuego.Server) {
	fuego.GetStd(s, "/v1/profile", withAuth(handlers.GetProfileHandler))
	fuego.PostStd(s, "/v1/auth/logout", withAuth(handlers.PostLogoutHandler))
}
