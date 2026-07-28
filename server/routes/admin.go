/* The /admin group contains routes for managing the invite-only allowlist and
   member roles. All routes require a signed-in user who can invite (member+);
   management actions further require an admin (see CanManageUsers). */
package routes

import (
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	"github.com/gin-gonic/gin"
	"sirtom/server/db"
	"sirtom/server/errs"
	"sirtom/server/logger"
	"sirtom/server/middleware"
	"sirtom/server/models"
	"sirtom/server/responses"
	"sirtom/server/utils"
)

// isValidEmail does a basic RFC 5322 address check and rejects '+' aliases
// (mirrors the frontend's sign-in validation).
func isValidEmail(email string) bool {
	if email == "" || strings.Contains(email, "+") {
		return false
	}
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

func InitAdmin(router *gin.RouterGroup) {
	adminRouter := router.Group("/admin")
	adminRouter.Use(middleware.AuthRequired(), middleware.CanInviteRequired())

	adminRouter.GET("/allowlist", getAllowlist)
	adminRouter.POST("/allowlist", addAllowlistEmail)
	adminRouter.DELETE("/allowlist", removeAllowlistEmail)
	adminRouter.POST("/member/role", setMemberRole)
}

// allowlistMember is an allowlist entry enriched with the registered-user
// status. The reported role is the account's role once they've signed in,
// otherwise the role recorded on the allowlist invitation.
type allowlistMember struct {
	models.AllowlistEntry
	HasAccount bool        `json:"hasAccount"`
	FirstName  string      `json:"firstName,omitempty"`
	LastName   string      `json:"lastName,omitempty"`
	Phone      string      `json:"phone,omitempty"`
	Role       models.Role `json:"role"`
}

func authUserFromContext(c *gin.Context) *models.User {
	userInterface, _ := c.Get("authUser")
	return userInterface.(*models.User)
}

// effectiveTargetRole returns the current role of the given email: the account
// role if they have one, otherwise the allowlist invitation role.
func effectiveTargetRole(email string) models.Role {
	if user, _ := db.GetUserByEmail(email); user != nil {
		return user.EffectiveRole()
	}
	return models.NormalizeRole(db.GetAllowlistRole(email))
}

// @Summary Lists the invite-only allowlist (with member status + role)
// @Tags admin
// @Produce json
// @Success 200 {array} allowlistMember
// @Router /admin/allowlist [get]
func getAllowlist(c *gin.Context) {
	// Any member+ may VIEW the full roll (the group's CanInviteRequired already
	// excludes guests). This is an intentional product choice — the club is
	// transparent about who's on the roll. Management actions (invite at an
	// elevated role, change roles, strike) stay admin-only in their handlers.
	entries, err := db.GetAllowlist()
	if err != nil {
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	// Batch-fetch the accounts for all listed emails in one query (avoids N+1).
	emails := make([]string, 0, len(entries))
	for _, entry := range entries {
		emails = append(emails, entry.Email)
	}
	users := db.GetUsersByEmails(emails)

	members := make([]allowlistMember, 0, len(entries))
	for _, entry := range entries {
		m := allowlistMember{AllowlistEntry: entry, Role: models.NormalizeRole(entry.Role)}
		if user, ok := users[utils.NormalizeEmail(entry.Email)]; ok {
			m.HasAccount = true
			m.FirstName = user.FirstName
			m.LastName = user.LastName
			m.Phone = user.Phone
			m.Role = user.EffectiveRole()
		}
		members = append(members, m)
	}
	c.JSON(http.StatusOK, members)
}

// @Summary Adds an email to the allowlist (invite a member)
// @Tags admin
// @Accept json
// @Produce json
// @Param payload body object{email=string,role=string} true "Email to invite and the role to grant"
// @Success 200
// @Router /admin/allowlist [post]
func addAllowlistEmail(c *gin.Context) {
	payload := struct {
		Email string      `json:"email" binding:"required"`
		Role  models.Role `json:"role"`
	}{}
	if err := c.BindJSON(&payload); err != nil {
		return
	}

	email := utils.NormalizeEmail(payload.Email)
	if !isValidEmail(email) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidEmail})
		return
	}

	// Reject unknown role values rather than silently coercing them. Empty is
	// allowed and defaults to guest.
	if payload.Role != "" && !models.IsKnownRole(payload.Role) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidRole})
		return
	}
	role := models.NormalizeRole(payload.Role)
	if payload.Role == "" {
		role = models.RoleGuest
	}
	actor := authUserFromContext(c)
	if !canGrantRole(actor.EffectiveRole(), role) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.InvalidRole})
		return
	}

	if err := db.AddToAllowlist(email, actor.Email, role); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	// Notify the invitee by email so they know to come sign in. Best-effort:
	// the invite (allowlist entry) already succeeded, so a mail failure must not
	// fail the request — we just report emailSent back to the admin. Skip people
	// who already have an account (they're already in — nothing to accept).
	existingUser, _ := db.GetUserByEmail(email)
	hasAccount := existingUser != nil
	emailSent := false
	if !hasAccount {
		signInURL := fmt.Sprintf("%s/sign-in", utils.GetOrigin(c))
		if err := utils.SendEmail(
			email,
			"You have been invited to The Fellowship",
			buildInvitationEmailBody(signInURL),
			"text/html",
		); err != nil {
			logger.StdErr.Println(err)
		} else {
			emailSent = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"email":      email,
		"role":       role,
		"hasAccount": hasAccount,
		"emailSent":  emailSent,
	})
}

// buildInvitationEmailBody returns the Fellowship-themed HTML invitation email.
// It does NOT contain a code — the invitee requests their own sign-in code when
// they visit the link and enter this address.
func buildInvitationEmailBody(signInURL string) string {
	return utils.RenderEmailWithFooter(
		"You are invited",
		utils.EmailParagraphHTML(
			"You have been invited to take your place among The Fellowship. To accept, present "+
				"yourself at the gate and sign in with "+utils.EmailEmphasis("this email address")+". "+
				"A one-time code will be sent to confirm your entry."),
		utils.EmailFooterURL(signInURL),
		utils.EmailAction{Label: "Enter the Gathering", URL: signInURL},
	)
}

// @Summary Removes an email from the allowlist
// @Tags admin
// @Accept json
// @Produce json
// @Param payload body object{email=string} true "Email to remove"
// @Success 200
// @Router /admin/allowlist [delete]
func removeAllowlistEmail(c *gin.Context) {
	payload := struct {
		Email string `json:"email" binding:"required"`
	}{}
	if err := c.BindJSON(&payload); err != nil {
		return
	}

	email := utils.NormalizeEmail(payload.Email)
	actor := authUserFromContext(c)

	// Only admins may remove members from the roll.
	if !actor.EffectiveRole().CanManageUsers() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}
	// Can't remove yourself (avoid self-lockout).
	if email == utils.NormalizeEmail(actor.Email) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.CannotRemoveSelf})
		return
	}
	// Super admins are immutable via the app.
	if effectiveTargetRole(email).IsSuperAdmin() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.SuperAdminImmutable})
		return
	}

	if err := db.RemoveFromAllowlist(email); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.JSON(http.StatusOK, gin.H{"email": email})
}

// @Summary Sets a member's role (guest/member/admin)
// @Tags admin
// @Accept json
// @Produce json
// @Param payload body object{email=string,role=string} true "Member email and new role"
// @Success 200
// @Router /admin/member/role [post]
func setMemberRole(c *gin.Context) {
	payload := struct {
		Email string      `json:"email" binding:"required"`
		Role  models.Role `json:"role" binding:"required"`
	}{}
	if err := c.BindJSON(&payload); err != nil {
		return
	}

	email := utils.NormalizeEmail(payload.Email)
	actor := authUserFromContext(c)

	// Reject unknown role values rather than silently coercing them to member.
	if !models.IsKnownRole(payload.Role) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.InvalidRole})
		return
	}

	// Only admins may change roles.
	if !actor.EffectiveRole().CanManageUsers() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.NotAuthorized})
		return
	}
	// Can't change your own role (avoid self-lockout / accidental demotion).
	if email == utils.NormalizeEmail(actor.Email) {
		c.JSON(http.StatusBadRequest, responses.Error{Error: errs.CannotRemoveSelf})
		return
	}
	// Super admin can't be granted, and existing super admins are immutable. The
	// requested-role check is first so an attempt to grant super admin is rejected
	// without a DB lookup.
	if payload.Role.IsSuperAdmin() || effectiveTargetRole(email).IsSuperAdmin() {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.SuperAdminImmutable})
		return
	}
	// The new role must be one the actor is allowed to grant.
	newRole := models.NormalizeRole(payload.Role)
	if !canGrantRole(actor.EffectiveRole(), newRole) {
		c.JSON(http.StatusForbidden, responses.Error{Error: errs.InvalidRole})
		return
	}

	// Keep the account role and the allowlist invitation role in sync. Write the
	// account role first — it is the authoritative source for permissions (and
	// getAllowlist reports it over the allowlist role for anyone with an account),
	// so if the second write fails the live permission change has still landed.
	// Both writes are idempotent; standalone Mongo has no multi-doc transactions.
	if _, err := db.SetUserRole(email, newRole); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}
	if err := db.SetAllowlistRole(email, newRole); err != nil {
		logger.StdErr.Println(err)
		c.JSON(http.StatusInternalServerError, responses.Error{Error: errs.Internal})
		return
	}

	c.JSON(http.StatusOK, gin.H{"email": email, "role": newRole})
}

// canGrantRole reports whether an actor with actorRole may grant targetRole.
// Members may only grant guest; admins/super admins may grant guest/member/admin.
// Super admin is never grantable through the app.
func canGrantRole(actorRole, targetRole models.Role) bool {
	if targetRole.IsSuperAdmin() {
		return false
	}
	if actorRole.CanManageUsers() {
		// admin & super admin: guest/member/admin
		return true
	}
	if actorRole.CanInvite() {
		// member: guests only
		return targetRole == models.RoleGuest
	}
	return false
}
