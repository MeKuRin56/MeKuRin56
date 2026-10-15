package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/middleware"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	cfg *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

// ShowLogin displays the login page
func (h *AuthHandler) ShowLogin(c *gin.Context) {
	// If already logged in, redirect based on role
	if user := middleware.GetCurrentUser(c); user != nil {
		if user.Role == models.RoleAdmin {
			c.Redirect(http.StatusFound, "/admin/dashboard")
			return
		}
		c.Redirect(http.StatusFound, "/tenant/dashboard")
		return
	}

	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title":     "เข้าสู่ระบบ | Login",
		"CSRFToken": middleware.GetCSRFToken(c),
		"Next":      c.Query("next"),
	})
}

// HandleLogin processes login request
func (h *AuthHandler) HandleLogin(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	password := c.PostForm("password")
	nextURL := c.PostForm("next")

	isAPI := c.ContentType() == "application/json" || c.GetHeader("Accept") == "application/json"

	if email == "" || password == "" {
		if isAPI {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Please enter both email and password"})
			return
		}
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"Title":     "เข้าสู่ระบบ | Login",
			"Error":     "กรุณากรอกอีเมลและรหัสผ่านให้ครบถ้วน (Please fill in all fields)",
			"Email":     email,
			"CSRFToken": middleware.GetCSRFToken(c),
			"Next":      nextURL,
		})
		return
	}

	db := database.GetDB()
	query := db.Rebind(`
		SELECT id, name, email, password, role, phone, room_id
		FROM users
		WHERE LOWER(email) = ?
	`)

	var user models.User
	var roomID sql.NullInt64
	err := db.QueryRow(query, email).Scan(
		&user.ID, &user.Name, &user.Email, &user.Password, &user.Role, &user.Phone, &roomID,
	)

	// Anti brute-force timing mitigation
	failLogin := func() {
		time.Sleep(300 * time.Millisecond)
	}

	if err != nil {
		failLogin()
		// Generic invalid credentials message to prevent user enumeration
		if isAPI {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"Title":     "เข้าสู่ระบบ | Login",
			"Error":     "อีเมลหรือรหัสผ่านไม่ถูกต้อง (Invalid email or password)",
			"Email":     email,
			"CSRFToken": middleware.GetCSRFToken(c),
			"Next":      nextURL,
		})
		return
	}

	// Verify bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		failLogin()
		if isAPI {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"Title":     "เข้าสู่ระบบ | Login",
			"Error":     "อีเมลหรือรหัสผ่านไม่ถูกต้อง (Invalid email or password)",
			"Email":     email,
			"CSRFToken": middleware.GetCSRFToken(c),
			"Next":      nextURL,
		})
		return
	}

	if roomID.Valid {
		idVal := roomID.Int64
		user.RoomID = &idVal
	}

	// Set secure session cookie
	middleware.SetUserSession(c, &user, h.cfg.IsProduction())

	if isAPI {
		c.JSON(http.StatusOK, gin.H{
			"message": "Login successful",
			"user": gin.H{
				"id":    user.ID,
				"name":  user.Name,
				"email": user.Email,
				"role":  user.Role,
			},
		})
		return
	}

	if nextURL != "" && !strings.HasPrefix(nextURL, "//") && strings.HasPrefix(nextURL, "/") {
		c.Redirect(http.StatusFound, nextURL)
		return
	}

	if user.Role == models.RoleAdmin {
		c.Redirect(http.StatusFound, "/admin/dashboard")
	} else {
		c.Redirect(http.StatusFound, "/tenant/dashboard")
	}
}

// ShowRegister displays the registration page
func (h *AuthHandler) ShowRegister(c *gin.Context) {
	if user := middleware.GetCurrentUser(c); user != nil {
		if user.Role == models.RoleAdmin {
			c.Redirect(http.StatusFound, "/admin/dashboard")
			return
		}
		c.Redirect(http.StatusFound, "/tenant/dashboard")
		return
	}

	c.HTML(http.StatusOK, "register.html", gin.H{
		"Title":     "สมัครสมาชิก | Register",
		"CSRFToken": middleware.GetCSRFToken(c),
	})
}

// HandleRegister processes user registration
func (h *AuthHandler) HandleRegister(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")
	phone := strings.TrimSpace(c.PostForm("phone"))

	isAPI := c.ContentType() == "application/json" || c.GetHeader("Accept") == "application/json"

	// Validation
	if name == "" || email == "" || password == "" {
		msg := "กรุณากรอกข้อมูลที่จำเป็นให้ครบถ้วน (Please fill in all required fields)"
		if isAPI {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"Title":     "สมัครสมาชิก | Register",
			"Error":     msg,
			"Name":      name,
			"Email":     email,
			"Phone":     phone,
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	// Input length limits (prevent DB overflow attacks)
	if len(name) > 100 {
		name = name[:100]
	}
	if len(email) > 150 {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"Title": "สมัครสมาชิก | Register", "Error": "อีเมลยาวเกินไป",
			"Name": name, "Phone": phone, "CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}
	if len(phone) > 20 {
		phone = phone[:20]
	}

	if len(password) < 8 {
		msg := "รหัสผ่านต้องมีความยาวอย่างน้อย 8 ตัวอักษร (Password must be at least 8 characters)"
		if isAPI {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"Title":     "สมัครสมาชิก | Register",
			"Error":     msg,
			"Name":      name,
			"Email":     email,
			"Phone":     phone,
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	if password != confirmPassword {
		msg := "รหัสผ่านและการยืนยันรหัสผ่านไม่ตรงกัน (Passwords do not match)"
		if isAPI {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"Title":     "สมัครสมาชิก | Register",
			"Error":     msg,
			"Name":      name,
			"Email":     email,
			"Phone":     phone,
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	db := database.GetDB()

	// Check if email already registered
	var existingID int64
	checkQuery := db.Rebind("SELECT id FROM users WHERE LOWER(email) = ?")
	err := db.QueryRow(checkQuery, email).Scan(&existingID)
	if err == nil {
		msg := "อีเมลนี้มีผู้ใช้งานในระบบแล้ว (Email already registered)"
		if isAPI {
			c.JSON(http.StatusConflict, gin.H{"error": msg})
			return
		}
		c.HTML(http.StatusConflict, "register.html", gin.H{
			"Title":     "สมัครสมาชิก | Register",
			"Error":     msg,
			"Name":      name,
			"Email":     email,
			"Phone":     phone,
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	// Hash password with bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "register.html", gin.H{
			"Title":     "สมัครสมาชิก | Register",
			"Error":     "เกิดข้อผิดพลาดในการประมวลผล (Server error)",
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	// Insert new tenant user
	insertQuery := db.Rebind(`
		INSERT INTO users (name, email, password, role, phone)
		VALUES (?, ?, ?, ?, ?)
	`)
	res, err := db.Exec(insertQuery, name, email, string(hashedPassword), models.RoleTenant, phone)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "register.html", gin.H{
			"Title":     "สมัครสมาชิก | Register",
			"Error":     "ไม่สามารถบันทึกข้อมูลได้ กรุณาลองใหม่อีกครั้ง",
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	newID, _ := res.LastInsertId()
	newUser := &models.User{
		ID:    newID,
		Name:  name,
		Email: email,
		Role:  models.RoleTenant,
		Phone: phone,
	}

	// Automatically log in
	middleware.SetUserSession(c, newUser, h.cfg.IsProduction())

	if isAPI {
		c.JSON(http.StatusCreated, gin.H{
			"message": "Registration successful",
			"user": gin.H{
				"id":    newID,
				"name":  name,
				"email": email,
				"role":  models.RoleTenant,
			},
		})
		return
	}

	c.Redirect(http.StatusFound, "/tenant/dashboard")
}

// HandleLogout logs the user out
func (h *AuthHandler) HandleLogout(c *gin.Context) {
	middleware.ClearUserSession(c, h.cfg.IsProduction())
	c.Redirect(http.StatusFound, "/login?logged_out=1")
}

// GetCurrentUserAPI returns current user info in JSON
func (h *AuthHandler) GetCurrentUserAPI(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"user": gin.H{
			"id":          user.ID,
			"name":        user.Name,
			"email":       user.Email,
			"role":        user.Role,
			"phone":       user.Phone,
			"room_id":     user.RoomID,
			"room_number": user.RoomNo,
		},
	})
}

// DemoLogin allows one-click demo login for evaluators and testers
func (h *AuthHandler) DemoLogin(c *gin.Context) {
	role := c.Query("role")
	db := database.GetDB()

	var targetEmail string
	if role == "admin" {
		targetEmail = h.cfg.AdminEmail
		if targetEmail == "" {
			targetEmail = "admin56@gmail.com"
		}
	} else {
		targetEmail = "tenant@dormitory.com"
	}

	var user models.User
	var roomID sql.NullInt64
	var roomNo sql.NullString
	query := db.Rebind(`
		SELECT u.id, u.name, u.email, u.role, COALESCE(u.phone, ''), u.room_id, r.room_number, u.created_at, u.updated_at
		FROM users u
		LEFT JOIN rooms r ON u.room_id = r.id
		WHERE LOWER(u.email) = LOWER(?)
		LIMIT 1
	`)
	err := db.QueryRow(query, targetEmail).Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.Phone, &roomID, &roomNo, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?error=demo_not_found")
		return
	}

	if roomID.Valid {
		idVal := roomID.Int64
		user.RoomID = &idVal
	}
	if roomNo.Valid {
		user.RoomNo = roomNo.String
	}

	middleware.SetUserSession(c, &user, false)

	if user.Role == models.RoleAdmin {
		c.Redirect(http.StatusFound, "/admin/dashboard?success=demo_admin")
	} else {
		c.Redirect(http.StatusFound, "/tenant/dashboard?success=demo_tenant")
	}
}
