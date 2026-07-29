package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/middleware"
	"sirtom/server/models"
	"sirtom/server/responses"
)

func InitUsers(router *gin.RouterGroup) {
	usersRouter := router.Group("/users")

	// Signed-in only. A member's face is club membership data: this is an
	// invite-only Fellowship, so a photo should not be fetchable by anyone who
	// happens to hold a user id. Authed per-route rather than on the group,
	// because the public profile below must stay reachable while signed out.
	//
	// Carrying the session cookie costs nothing in practice — production serves
	// the SPA and the API from one origin, so every <img> sends it already. The
	// cross-origin `npm run serve` case is handled by the crossorigin attribute
	// UserAvatarContent sets on its own avatar URLs.
	usersRouter.GET("/:userId/avatar", middleware.AuthRequired(), getUserAvatar)

	// Public profile for invite screens, ads, etc. (no auth). Must be registered
	// after more specific /:userId/... routes.
	usersRouter.GET("/:userId", getPublicUserProfile)
}

// @Summary Serves a user's uploaded profile photo
// @Description Requires a signed-in session — member photos are club data, not public.
// @Description Cached immutably: the URL is expected to carry the account's
// @Description avatarUpdatedAt as a `?v=` parameter, so a new upload is a new URL.
// @Description An ETag is sent as well, for clients that revalidate anyway.
// @Tags users
// @Produce image/jpeg
// @Param userId path string true "User ID"
// @Success 200 {file} binary "The user's avatar"
// @Failure 401 {object} responses.Error "Not signed in"
// @Failure 404 {object} responses.Error "The user has no uploaded avatar"
// @Router /users/{userId}/avatar [get]
func getUserAvatar(c *gin.Context) {
	// A malformed id can't have an avatar, which is the same answer as an id
	// that simply doesn't — no reason to distinguish them to an anonymous
	// caller.
	userId, err := primitive.ObjectIDFromHex(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.UserDoesNotExist})
		return
	}

	avatar, avatarErr := db.GetAvatar(userId)
	if avatarErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if avatar == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.UserDoesNotExist})
		return
	}

	etag := avatarETag(avatar)
	// `private`, not `public`: this response is now behind a session, and the
	// site sits behind Cloudflare. `public` invites a shared cache to store one
	// member's photo and hand it to whoever asks for the same URL next —
	// including someone with no session at all, which is exactly what gating
	// the route was meant to prevent. `private` keeps the browser cache, which
	// is where the benefit actually is: the URL is per-user and changes on
	// every upload, so a client refetches only when the photo really changed.
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		// AbortWithStatus rather than Status: the latter only records the code
		// on gin's writer, leaving it to be flushed by a later write that a
		// bodyless 304 never makes.
		c.AbortWithStatus(http.StatusNotModified)
		return
	}

	contentType := avatar.ContentType
	if contentType == "" {
		contentType = avatarContentType
	}
	c.Data(http.StatusOK, contentType, avatar.Data.Data)
}

// @Summary Returns a minimal public user profile (safe for unauthenticated clients)
// @Tags users
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} models.User
// @Router /users/{userId} [get]
func getPublicUserProfile(c *gin.Context) {
	userId := c.Param("userId")
	user, userErr := db.GetUserById(userId)
	if userErr != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if user == nil {
		c.JSON(http.StatusNotFound, responses.Error{Error: errs.UserDoesNotExist})
		return
	}

	public := models.User{
		Id:              user.Id,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		Nickname:        user.Nickname,
		Picture:         user.Picture,
		AvatarUpdatedAt: user.AvatarUpdatedAt,
	}
	c.JSON(http.StatusOK, public)
}
