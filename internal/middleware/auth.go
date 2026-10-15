package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
)

const (
	SessionCookieName = "dorm_session"
	UserContextKey    = "currentUser"
)

type SessionData struct {
	UserID    int64     `json:"uid"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Name      string    `json:"name"`
	ExpiresAt time.Time `json:"exp"`
}

var sessionSecret []byte

func InitAuth(cfg *config.Config) {
	sessionSecret = []byte(cfg.SessionSecret)
}

// sign generates HMAC-SHA256 signature for data
func sign(data []byte) string {
	h := hmac.New(sha256.New, sessionSecret)
	h.Write(data)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// verify validates HMAC signature
func verify(data []byte, signature string) bool {
	expected := sign(data)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// SetUserSession encrypts and signs session data into an HttpOnly cookie
func SetUserSession(c *gin.Context, user *models.User, isProduction bool) {
	session := SessionData{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		Name:      user.Name,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	rawJSON, err := json.Marshal(session)
	if err != nil {
		return
	}

	payload := base64.RawURLEncoding.EncodeToString(rawJSON)
	sig := sign([]byte(payload))
	cookieVal := fmt.Sprintf("%s.%s", payload, sig)

	// Set secure HttpOnly cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		SessionCookieName,
		cookieVal,
		int(24*time.Hour.Seconds()),
		"/",
		"",
		isProduction,
		true, // HttpOnly
	)
}

// ClearUserSession removes the session cookie
func ClearUserSession(c *gin.Context, isProduction bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		SessionCookieName,
		"",
		-1,
		"/",
		"",
		isProduction,
		true,
	)
}

// GetCurrentUser reads and validates session from cookie
func GetCurrentUser(c *gin.Context) *models.User {
	if val, exists := c.Get(UserContextKey); exists {
		if user, ok := val.(*models.User); ok {
			return user
		}
	}

	cookieVal, err := c.Cookie(SessionCookieName)
	if err != nil || cookieVal == "" {
		return nil
	}

	parts := strings.Split(cookieVal, ".")
	if len(parts) != 2 {
		return nil
	}

	payload := parts[0]
	sig := parts[1]

	if !verify([]byte(payload), sig) {
		return nil
	}

	rawJSON, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil
	}

	var sess SessionData
	if err := json.Unmarshal(rawJSON, &sess); err != nil {
		return nil
	}

	if time.Now().After(sess.ExpiresAt) {
		return nil
	}

	// Fetch fresh user record from DB to verify status and room_id
	db := database.GetDB()
	var user models.User
	var roomID sqlNullInt64
	var roomNo sqlNullString

	query := db.Rebind(`
		SELECT u.id, u.name, u.email, u.role, COALESCE(u.phone, ''), u.room_id, r.room_number, u.created_at, u.updated_at
		FROM users u
		LEFT JOIN rooms r ON u.room_id = r.id
		WHERE u.id = ?
	`)

	row := db.QueryRow(query, sess.UserID)
	err = row.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.Phone, &roomID, &roomNo, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil
	}

	if roomID.Valid {
		idVal := roomID.Int64
		user.RoomID = &idVal
	}
	if roomNo.Valid {
		user.RoomNo = roomNo.String
	}

	c.Set(UserContextKey, &user)
	return &user
}

// AuthRequired ensures user is logged in
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)
		if user == nil {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized. Please log in."})
				c.Abort()
				return
			}
			c.Redirect(http.StatusFound, "/login?next="+c.Request.URL.Path)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RoleRequired ensures logged in user has one of the allowed roles
func RoleRequired(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)
		if user == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		hasRole := false
		for _, role := range allowedRoles {
			if user.Role == role {
				hasRole = true
				break
			}
		}

		if !hasRole {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Insufficient privileges"})
				c.Abort()
				return
			}
			c.HTML(http.StatusForbidden, "error.html", gin.H{
				"Title":   "Forbidden",
				"Message": "คุณไม่มีสิทธิ์เข้าถึงหน้านี้ (Access Denied)",
				"User":    user,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Helpers for sql null types
type sqlNullInt64 struct {
	Int64 int64
	Valid bool
}

func (n *sqlNullInt64) Scan(value interface{}) error {
	if value == nil {
		n.Int64, n.Valid = 0, false
		return nil
	}
	n.Valid = true
	switch v := value.(type) {
	case int64:
		n.Int64 = v
	case int:
		n.Int64 = int64(v)
	case []byte:
		i, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return err
		}
		n.Int64 = i
	default:
		n.Int64 = 0
		n.Valid = false
	}
	return nil
}

type sqlNullString struct {
	String string
	Valid  bool
}

func (n *sqlNullString) Scan(value interface{}) error {
	if value == nil {
		n.String, n.Valid = "", false
		return nil
	}
	n.Valid = true
	switch v := value.(type) {
	case string:
		n.String = v
	case []byte:
		n.String = string(v)
	default:
		n.String = fmt.Sprintf("%v", v)
	}
	return nil
}
