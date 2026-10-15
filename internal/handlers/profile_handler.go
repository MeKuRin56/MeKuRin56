package handlers

import (
	"net/http"
	"strings"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/middleware"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type ProfileHandler struct {
	cfg *config.Config
}

func NewProfileHandler(cfg *config.Config) *ProfileHandler {
	return &ProfileHandler{cfg: cfg}
}

// ShowProfile shows the profile edit page
func (h *ProfileHandler) ShowProfile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	c.HTML(http.StatusOK, "profile.html", gin.H{
		"Title":     "แก้ไขโปรไฟล์ | My Profile",
		"User":      user,
		"CSRFToken": middleware.GetCSRFToken(c),
		"Success":   c.Query("success"),
		"Error":     c.Query("error"),
	})
}

// UpdateProfile handles name and phone update
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	name := strings.TrimSpace(c.PostForm("name"))
	phone := strings.TrimSpace(c.PostForm("phone"))

	if name == "" {
		c.Redirect(http.StatusFound, "/profile?error=name_required")
		return
	}
	// Enforce length limits
	if len(name) > 100 {
		name = name[:100]
	}
	if len(phone) > 20 {
		phone = phone[:20]
	}

	db := database.GetDB()
	_, err := db.Exec(
		db.Rebind("UPDATE users SET name=?, phone=?, updated_at=CURRENT_TIMESTAMP WHERE id=?"),
		name, phone, user.ID,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/profile?error=update_failed")
		return
	}

	c.Redirect(http.StatusFound, "/profile?success=updated")
}

// ChangePassword handles password change
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	currentPwd := c.PostForm("current_password")
	newPwd := c.PostForm("new_password")
	confirmPwd := c.PostForm("confirm_password")

	if currentPwd == "" || newPwd == "" || confirmPwd == "" {
		c.Redirect(http.StatusFound, "/profile?error=fields_required")
		return
	}

	if len(newPwd) < 8 {
		c.Redirect(http.StatusFound, "/profile?error=password_too_short")
		return
	}

	if newPwd != confirmPwd {
		c.Redirect(http.StatusFound, "/profile?error=password_mismatch")
		return
	}

	// Fetch current hashed password from DB
	db := database.GetDB()
	var hashedPwd string
	err := db.QueryRow(db.Rebind("SELECT password FROM users WHERE id=?"), user.ID).Scan(&hashedPwd)
	if err != nil {
		c.Redirect(http.StatusFound, "/profile?error=not_found")
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPwd), []byte(currentPwd)); err != nil {
		c.Redirect(http.StatusFound, "/profile?error=wrong_current_password")
		return
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		c.Redirect(http.StatusFound, "/profile?error=hash_failed")
		return
	}

	_, err = db.Exec(
		db.Rebind("UPDATE users SET password=?, updated_at=CURRENT_TIMESTAMP WHERE id=?"),
		string(newHash), user.ID,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/profile?error=update_failed")
		return
	}

	// Force re-login after password change for security
	middleware.ClearUserSession(c, h.cfg.IsProduction())
	c.Redirect(http.StatusFound, "/login?success=password_changed")
}
