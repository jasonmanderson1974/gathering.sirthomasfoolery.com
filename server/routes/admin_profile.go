// Admin-on-behalf profile editing: an admin fixing up another member's name,
// nickname or photo from the roll.
//
// WHY THIS IS NOT IN user.go: the self-serve routes address the signed-in
// account and need no authorization beyond "is signed in". These address
// *someone else's* account, so every one of them repeats the same three
// guards. That repetition is deliberate — the /admin group is gated by
// CanInviteRequired, which admits plain members, so the per-handler
// CanManageUsers() check is the only thing standing between a member and
// editing the roll. Do not "simplify" it into the group middleware without
// moving the read-only routes (getAllowlist) out first.
package routes

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/responses"
	"sirtom/server/utils"
)

// nameMaxRunes bounds an admin-supplied name. The self-serve /user/name route
// takes names unbounded (you can only hurt yourself there); editing on behalf
// of someone else deserves the same cap the nickname gets.
const nameMaxRunes = 60

// resolveAdminTarget applies the guards shared by every admin-on-behalf edit
// and returns the target account. It writes the error response itself; a false
// second return means the caller must return immediately.
//
// Order matters: authorization is checked before anything reveals whether the
// email exists, so a non-admin cannot use these routes to probe the roll.
func resolveAdminTarget(c *gin.Context, rawEmail string) (*models.User, bool) {
	email := utils.NormalizeEmail(rawEmail)
	actor := authUserFromContext(c)

	// Only admins may edit someone else's profile.
	if !actor.EffectiveRole().CanManageUsers() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return nil, false
	}
	// Super admins are immutable via the app, same as for roles and striking.
	if effectiveTargetRole(email).IsSuperAdmin() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.SuperAdminImmutable})
		return nil, false
	}
	// Unlike setMemberRole there is no self-edit block: an admin editing their
	// own name here is harmless, and Settings lets them do it anyway.

	// An allowlist entry with hasAccount=false is an unclaimed invitation —
	// there is no document to write these fields to. The roll hides the control
	// in that case; this is the server-side half of that rule.
	user, err := db.GetUserByEmail(email)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return nil, false
	}
	if user == nil {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.UserDoesNotExist})
		return nil, false
	}
	return user, true
}

// @Summary Updates another member's name and/or nickname (admin only)
// @Description Admin-only. Fields are optional: omitting one leaves it unchanged.
// @Description Sending an empty nickname clears it. Super admins cannot be edited,
// @Description and the target must already have an account (an unclaimed invitation
// @Description has nothing to edit).
// @Tags admin
// @Accept json
// @Produce json
// @Param payload body object{email=string,firstName=string,lastName=string,nickname=string} true "Target email plus the fields to change"
// @Success 200 {object} object{email=string,firstName=string,lastName=string,nickname=string}
// @Failure 400 {object} responses.Error "user-does-not-exist"
// @Failure 403 {object} responses.Error "not-authorized / super-admin-immutable"
// @Router /admin/member/profile [patch]
func updateMemberProfile(c *gin.Context) {
	// Pointers, so "not sent" is distinguishable from "sent empty" — a PATCH
	// naming only a nickname must not blank the name.
	payload := struct {
		Email     string  `json:"email" binding:"required"`
		FirstName *string `json:"firstName"`
		LastName  *string `json:"lastName"`
		Nickname  *string `json:"nickname"`
	}{}
	if err := c.BindJSON(&payload); err != nil {
		return
	}

	target, ok := resolveAdminTarget(c, payload.Email)
	if !ok {
		return
	}

	fields := db.UserProfileUpdate{}
	// A name may be edited but not erased: DisplayName falls back to the name
	// when there is no nickname, so an empty pair would leave the person
	// nameless everywhere. A blank-but-sent name is rejected rather than
	// silently ignored.
	if payload.FirstName != nil {
		firstName := trimAndTruncate(*payload.FirstName, nameMaxRunes)
		if firstName == "" {
			c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidName})
			return
		}
		fields.FirstName = &firstName
	}
	if payload.LastName != nil {
		lastName := trimAndTruncate(*payload.LastName, nameMaxRunes)
		if lastName == "" {
			c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidName})
			return
		}
		fields.LastName = &lastName
	}
	if payload.Nickname != nil {
		nickname := trimAndTruncate(*payload.Nickname, nicknameMaxRunes)
		fields.Nickname = &nickname
	}

	matched, err := db.UpdateUserProfileByEmail(target.Email, fields)
	if err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if matched == 0 {
		// resolveAdminTarget just found this account, so a miss here means it
		// was struck in between. Report it the same way.
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.UserDoesNotExist})
		return
	}

	// Echo the resulting values so the roll can update the row without
	// re-fetching the whole allowlist.
	result := gin.H{"email": target.Email, "firstName": target.FirstName, "lastName": target.LastName, "nickname": target.Nickname}
	if fields.FirstName != nil {
		result["firstName"] = *fields.FirstName
	}
	if fields.LastName != nil {
		result["lastName"] = *fields.LastName
	}
	if fields.Nickname != nil {
		result["nickname"] = *fields.Nickname
	}
	c.JSON(http.StatusOK, result)
}

// @Summary Sets another member's profile photo (admin only)
// @Description Admin-only. Runs the identical pipeline as the self-serve route:
// @Description centre-cropped, downscaled to 256x256, re-encoded as JPEG.
// @Tags admin
// @Accept json
// @Produce json
// @Param payload body object{email=string,image=string} true "Target email and a data URL of the image"
// @Success 200 {object} object{avatarUpdatedAt=int}
// @Failure 400 {object} responses.Error "invalid-image / user-does-not-exist"
// @Failure 403 {object} responses.Error "not-authorized / super-admin-immutable"
// @Failure 413 {object} responses.Error "image-too-large"
// @Router /admin/member/avatar [put]
func updateMemberAvatar(c *gin.Context) {
	// Cap the body before binding reads it, exactly as the self-serve route
	// does — the byte caps inside the pipeline only run once the whole payload
	// is already in memory.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarRequestBytes)

	payload := struct {
		Email string `json:"email" binding:"required"`
		Image string `json:"image" binding:"required"`
	}{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, responses.Error{Error: errs.ImageTooLarge})
			return
		}
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidImage})
		return
	}

	target, ok := resolveAdminTarget(c, payload.Email)
	if !ok {
		return
	}

	updatedAt, err := saveAvatarForUser(target.Id, payload.Image)
	if err != nil {
		respondToAvatarError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"avatarUpdatedAt": updatedAt})
}

// @Summary Removes another member's profile photo (admin only)
// @Description Admin-only. Idempotent — removing a photo that isn't there succeeds.
// @Tags admin
// @Accept json
// @Produce json
// @Param payload body object{email=string} true "Target email"
// @Success 200
// @Failure 400 {object} responses.Error "user-does-not-exist"
// @Failure 403 {object} responses.Error "not-authorized / super-admin-immutable"
// @Router /admin/member/avatar [delete]
func deleteMemberAvatar(c *gin.Context) {
	payload := struct {
		Email string `json:"email" binding:"required"`
	}{}
	if err := c.BindJSON(&payload); err != nil {
		return
	}

	target, ok := resolveAdminTarget(c, payload.Email)
	if !ok {
		return
	}

	if err := db.DeleteAvatar(target.Id); err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}
