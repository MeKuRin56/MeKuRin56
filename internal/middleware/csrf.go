package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CSRFCookieName = "dorm_csrf"
	CSRFHeaderName = "X-CSRF-Token"
	CSRFFormField  = "_csrf"
	CSRFContextKey = "csrf_token"
)

func generateRandomToken() string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// CSRFMiddleware handles CSRF protection for all state-changing requests
func CSRFMiddleware(isProduction bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(CSRFCookieName)
		if err != nil || token == "" {
			token = generateRandomToken()
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie(
				CSRFCookieName,
				token,
				86400, // 1 day
				"/",
				"",
				isProduction,
				false, // JavaScript accessible for AJAX
			)
		}

		c.Set(CSRFContextKey, token)

		// State-changing methods must be checked
		method := c.Request.Method
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete || method == http.MethodPatch {
			// Check header first
			submittedToken := c.GetHeader(CSRFHeaderName)
			// Check form field if not in header
			if submittedToken == "" {
				submittedToken = c.PostForm(CSRFFormField)
			}

			if submittedToken == "" || submittedToken != token {
				if c.ContentType() == "application/json" || c.GetHeader("Accept") == "application/json" {
					c.JSON(http.StatusForbidden, gin.H{
						"error": "Invalid or missing CSRF token",
					})
				} else {
					c.HTML(http.StatusForbidden, "error.html", gin.H{
						"Title":   "Security Error",
						"Message": "คำขอไม่ถูกต้องหรือหมดอายุ (Invalid CSRF Token). กรุณารีเฟรชหน้าเว็บและลองใหม่อีกครั้ง",
					})
				}
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// GetCSRFToken retrieves the token from context for templates
func GetCSRFToken(c *gin.Context) string {
	if val, exists := c.Get(CSRFContextKey); exists {
		if token, ok := val.(string); ok {
			return token
		}
	}
	token, _ := c.Cookie(CSRFCookieName)
	return token
}
