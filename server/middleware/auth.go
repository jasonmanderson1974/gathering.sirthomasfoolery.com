package middleware

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/responses"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if userId is set
		session := sessions.Default(c)
		if session.Get("userId") == nil {
			// User id is not set, user is not signed in!
			c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotSignedIn})
			c.Abort()
			return
		}

		// Check if user with user id exists
		user, userErr := db.GetUserById(session.Get("userId").(string))
		if userErr != nil {
			c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
			c.Abort()
			return
		}

		if user == nil {
			c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.UserDoesNotExist})
			c.Abort()
			return
		}

		// Enforce the invite-only allowlist on every request, not just at
		// sign-in. If a member is struck from the roll, their existing session
		// stops working on the next request (cookie sessions can't be revoked
		// server-side otherwise). Fail-open while the allowlist is empty.
		if !db.IsAccessAllowed(user.Email) {
			session.Delete("userId")
			// If this Save fails the revocation is not persisted and the
			// struck-off member's cookie stays live for the next request. We
			// still deny THIS request, but the failure must be visible.
			if err := session.Save(); err != nil {
				logger.StdErr.Printf("failed to persist session revocation for %s: %v", user.Email, err)
			}
			c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotInvited})
			c.Abort()
			return
		}

		c.Set("authUser", user)

		c.Next()
	}
}

// AuthRequiredIfInviteOnly enforces AuthRequired only when the instance is a
// strictly-enforced invite-only instance (INVITE_ONLY_ENFORCED). It gates
// unauthenticated writes that should be member-only on a locked-down instance
// (e.g. createEvent) while leaving guest flows open on dev/open instances.
func AuthRequiredIfInviteOnly() gin.HandlerFunc {
	authRequired := AuthRequired()
	return func(c *gin.Context) {
		if db.AccessControlEnforced() {
			// Delegates to AuthRequired, which either aborts (401/403) or sets
			// authUser and advances the chain via c.Next().
			authRequired(c)
			return
		}
		c.Next()
	}
}

// CanInviteRequired must be chained AFTER AuthRequired. It rejects signed-in
// users who cannot invite (guests), gating the /admin group. Individual
// handlers further restrict management actions to admins (CanManageUsers).
func CanInviteRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userInterface, exists := c.Get("authUser")
		if !exists {
			c.JSON(http.StatusUnauthorized, responses.Error{Error: errs.NotSignedIn})
			c.Abort()
			return
		}
		user := userInterface.(*models.User)

		if !user.EffectiveRole().CanInvite() {
			c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
			c.Abort()
			return
		}

		c.Next()
	}
}
