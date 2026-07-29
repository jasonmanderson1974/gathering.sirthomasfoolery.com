package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/models"
	"sirtom/server/responses"
)

func InitUsers(router *gin.RouterGroup) {
	usersRouter := router.Group("/users")

	// Unauthenticated, like the public profile below — an avatar is exactly as
	// public as the Google `picture` URL that profile already hands out, and
	// making it authed would mean every <img> carried a session cookie.
	usersRouter.GET("/:userId/avatar", getUserAvatar)

	// Public profile for invite screens, ads, etc. (no auth). Must be registered
	// after more specific /:userId/... routes.
	usersRouter.GET("/:userId", getPublicUserProfile)
}

// @Summary Serves a user's uploaded profile photo
// @Description Cached immutably: the URL is expected to carry the account's
// @Description avatarUpdatedAt as a `?v=` parameter, so a new upload is a new URL.
// @Description An ETag is sent as well, for clients that revalidate anyway.
// @Tags users
// @Produce image/jpeg
// @Param userId path string true "User ID"
// @Success 200 {file} binary "The user's avatar"
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
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
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
