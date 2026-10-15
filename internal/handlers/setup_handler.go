package handlers

import (
	"net/http"
	"strings"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/middleware"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type SetupHandler struct {
	cfg *config.Config
}

func NewSetupHandler(cfg *config.Config) *SetupHandler {
	return &SetupHandler{cfg: cfg}
}

// HasAdminUser checks if an administrator account already exists in the database
func HasAdminUser() bool {
	db := database.GetDB()
	if db == nil {
		return false
	}
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE role = ?", models.RoleAdmin).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// ShowSetup displays the first-time setup wizard
func (h *SetupHandler) ShowSetup(c *gin.Context) {
	if HasAdminUser() {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "setup.html", gin.H{
		"Title":     "ตั้งค่าระบบครั้งแรก | First-Time Setup",
		"CSRFToken": middleware.GetCSRFToken(c),
	})
}

// HandleSetup processes the first-time setup submission
func (h *SetupHandler) HandleSetup(c *gin.Context) {
	if HasAdminUser() {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	phone := strings.TrimSpace(c.PostForm("phone"))
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")

	// Input Validation
	if name == "" || email == "" || password == "" {
		c.HTML(http.StatusBadRequest, "setup.html", gin.H{
			"Title":     "ตั้งค่าระบบครั้งแรก | First-Time Setup",
			"Error":     "กรุณากรอกชื่อ อีเมล และรหัสผ่านให้ครบถ้วน (Please fill in all required fields)",
			"Name":      name,
			"Email":     email,
			"Phone":     phone,
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	if len(password) < 8 {
		c.HTML(http.StatusBadRequest, "setup.html", gin.H{
			"Title":     "ตั้งค่าระบบครั้งแรก | First-Time Setup",
			"Error":     "รหัสผ่านต้องมีความยาวอย่างน้อย 8 ตัวอักษร (Password must be at least 8 characters)",
			"Name":      name,
			"Email":     email,
			"Phone":     phone,
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	if password != confirmPassword {
		c.HTML(http.StatusBadRequest, "setup.html", gin.H{
			"Title":     "ตั้งค่าระบบครั้งแรก | First-Time Setup",
			"Error":     "รหัสผ่านและการยืนยันรหัสผ่านไม่ตรงกัน (Passwords do not match)",
			"Name":      name,
			"Email":     email,
			"Phone":     phone,
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	db := database.GetDB()

	// Hash password with bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "setup.html", gin.H{
			"Title":     "ตั้งค่าระบบครั้งแรก | First-Time Setup",
			"Error":     "เกิดข้อผิดพลาดในการเข้ารหัสรหัสผ่าน กรุณาลองใหม่อีกครั้ง",
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	// Insert Admin account
	insertQuery := db.Rebind(`
		INSERT INTO users (name, email, password, role, phone)
		VALUES (?, ?, ?, ?, ?)
	`)
	res, err := db.Exec(insertQuery, name, email, string(hashedPassword), models.RoleAdmin, phone)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "setup.html", gin.H{
			"Title":     "ตั้งค่าระบบครั้งแรก | First-Time Setup",
			"Error":     "ไม่สามารถสร้างบัญชีผู้ดูแลได้ (" + err.Error() + ")",
			"Name":      name,
			"Email":     email,
			"Phone":     phone,
			"CSRFToken": middleware.GetCSRFToken(c),
		})
		return
	}

	newID, _ := res.LastInsertId()
	adminUser := &models.User{
		ID:    newID,
		Name:  name,
		Email: email,
		Role:  models.RoleAdmin,
		Phone: phone,
	}

	// Seed default initial rooms if none exist so the system is ready to use
	var roomCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM rooms").Scan(&roomCount)
	if roomCount == 0 {
		rooms := []struct {
			number   string
			roomType string
			price    float64
			floor    int
			status   string
			desc     string
		}{
			{"101", "Standard", 4500.00, 1, models.RoomStatusVacant, "ห้องพักมาตรฐาน ชั้น 1 เตียงเดี่ยว 5 ฟุต เฟอร์นิเจอร์ครบ"},
			{"102", "Standard", 4500.00, 1, models.RoomStatusVacant, "ห้องพักมาตรฐาน ชั้น 1 วิวสวน เงียบสงบ"},
			{"103", "Deluxe", 5500.00, 1, models.RoomStatusVacant, "ห้องดีลักซ์ ชั้น 1 ระเบียงกว้าง ตู้เย็น ไมโครเวฟ"},
			{"201", "Deluxe", 5500.00, 2, models.RoomStatusVacant, "ห้องดีลักซ์ ชั้น 2 แอร์อินเวอร์เตอร์ เครื่องทำน้ำอุ่น"},
			{"202", "Studio", 6500.00, 2, models.RoomStatusVacant, "ห้องสตูดิโอ ชั้น 2 พร้อมมุมทำอาหารและเครื่องซักผ้า"},
			{"301", "Suite", 8000.00, 3, models.RoomStatusVacant, "ห้องสูท ชั้น 3 พื้นที่ 45 ตร.ม. ห้องนั่งเล่นแยกส่วน"},
		}
		insRoom := db.Rebind(`
			INSERT INTO rooms (room_number, room_type, price, floor, status, description)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
		for _, r := range rooms {
			_, _ = db.Exec(insRoom, r.number, r.roomType, r.price, r.floor, r.status, r.desc)
		}
	}

	// Auto-login the freshly created administrator
	middleware.SetUserSession(c, adminUser, h.cfg.IsProduction())

	c.Redirect(http.StatusFound, "/admin/dashboard?success=setup_complete")
}
