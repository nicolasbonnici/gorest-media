package media

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	auth "github.com/nicolasbonnici/gorest/auth"
)

// currentUserID returns the authenticated uploader, or the zero UUID when the
// request is anonymous. Ownership is recorded when available but not required,
// so the plugin works whether or not the host app mounts auth middleware.
func currentUserID(c fiber.Ctx) uuid.UUID {
	user := auth.GetAuthenticatedUser(c)
	if user == nil {
		return uuid.Nil
	}
	id, err := uuid.Parse(user.UserID)
	if err != nil {
		return uuid.Nil
	}
	return id
}
