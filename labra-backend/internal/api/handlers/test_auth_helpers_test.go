package handlers

import (
	"net/http"
	"strconv"

	"labra-backend/internal/api/auth"
)

func withTestPrincipal(req *http.Request, userID int64) *http.Request {
	principal := auth.Principal{
		UserID: userID,
		Sub:    "test-user-" + strconv.FormatInt(userID, 10),
		Roles:  []string{"owner"},
	}
	return req.WithContext(auth.WithPrincipal(req.Context(), principal))
}
