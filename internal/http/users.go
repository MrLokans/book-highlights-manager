package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/auth"
)

// ProfileController handles user profile operations.
type ProfileController struct {
	authService *auth.Service
}

// NewProfileController creates a new ProfileController.
func NewProfileController(authService *auth.Service) *ProfileController {
	return &ProfileController{
		authService: authService,
	}
}

// ProfilePage renders the user profile page.
func (pc *ProfileController) ProfilePage(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.Redirect(http.StatusFound, "/login?next=/profile")
		return
	}

	user, err := pc.authService.GetUserByID(userID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error loading user: %s", err.Error())
		return
	}

	// Check if user has a token set
	hasToken := user.TokenHash != ""

	c.HTML(http.StatusOK, "profile", gin.H{
		"User":      user,
		"HasToken":  hasToken,
		"Auth":      GetAuthTemplateData(c),
		"Analytics": GetAnalyticsTemplateData(c),
	})
}

// ChangePassword handles password change requests.
func (pc *ProfileController) ChangePassword(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.HTML(http.StatusUnauthorized, "password-result", gin.H{
			"Success": false,
			"Error":   "Not authenticated",
		})
		return
	}

	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	if newPassword != confirmPassword {
		c.HTML(http.StatusBadRequest, "password-result", gin.H{
			"Success": false,
			"Error":   "New passwords do not match",
		})
		return
	}

	if len(newPassword) < 8 {
		c.HTML(http.StatusBadRequest, "password-result", gin.H{
			"Success": false,
			"Error":   "Password must be at least 8 characters",
		})
		return
	}

	err := pc.authService.ChangePassword(userID, currentPassword, newPassword)
	if err != nil {
		errMsg := "Failed to change password"
		if err == auth.ErrInvalidPassword {
			errMsg = "Current password is incorrect"
		}
		c.HTML(http.StatusBadRequest, "password-result", gin.H{
			"Success": false,
			"Error":   errMsg,
		})
		return
	}

	c.HTML(http.StatusOK, "password-result", gin.H{
		"Success": true,
	})
}

// GenerateToken creates a new API token for the user.
func (pc *ProfileController) GenerateToken(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.HTML(http.StatusUnauthorized, "token-result", gin.H{
			"Error": "Not authenticated",
		})
		return
	}

	token, err := pc.authService.GenerateToken(userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "token-result", gin.H{
			"Error": "Failed to generate token: " + err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "token-result", gin.H{
		"Token": token,
	})
}

// RegenerateToken creates a new API token, replacing the existing one.
func (pc *ProfileController) RegenerateToken(c *gin.Context) {
	// Same as GenerateToken - it replaces the existing token
	pc.GenerateToken(c)
}

// RevokeToken removes the user's API token.
func (pc *ProfileController) RevokeToken(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.HTML(http.StatusUnauthorized, "token-result", gin.H{
			"Error": "Not authenticated",
		})
		return
	}

	err := pc.authService.RevokeToken(userID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "token-result", gin.H{
			"Error": "Failed to revoke token: " + err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "token-result", gin.H{
		"Revoked": true,
	})
}
