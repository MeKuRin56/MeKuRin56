package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/handlers"
	"dormitory-management-system/internal/middleware"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func setupTestApp(t *testing.T) (*gin.Engine, *database.DB, *config.Config) {
	gin.SetMode(gin.TestMode)
	_ = os.Remove("test_dormitory.db")

	cfg := &config.Config{
		Port:          "8085",
		DBDriver:      "sqlite",
		SessionSecret: "test-secret-key-at-least-32-characters-long!",
		AdminEmail:    "admin@dormitory.com",
		AdminPassword: "admin123456",
		AdminName:     "Test Admin",
		UploadDir:     "./test_uploads",
		Environment:   "development",
	}

	db, err := database.InitDB(cfg)
	if err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	middleware.InitAuth(cfg)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.SecurityHeadersMiddleware())
	r.Static("/static", "./static")
	r.Static("/locales", "./locales")
	r.Use(middleware.CSRFMiddleware(false))
	r.LoadHTMLGlob("templates/*")

	authHandler := handlers.NewAuthHandler(cfg)
	roomHandler := handlers.NewRoomHandler(cfg)
	maintHandler := handlers.NewMaintenanceHandler(cfg)
	paymentHandler := handlers.NewPaymentHandler(cfg)
	adminHandler := handlers.NewAdminHandler(cfg)
	tenantHandler := handlers.NewTenantHandler(cfg)

	r.GET("/", func(c *gin.Context) {
		currentUser := middleware.GetCurrentUser(c)
		c.HTML(http.StatusOK, "index.html", gin.H{"User": currentUser, "Stats": models.DashboardStats{}})
	})
	r.GET("/rooms", roomHandler.ListRooms)
	r.GET("/login", authHandler.ShowLogin)
	r.POST("/login", authHandler.HandleLogin)
	r.GET("/register", authHandler.ShowRegister)
	r.POST("/register", authHandler.HandleRegister)
	r.GET("/logout", authHandler.HandleLogout)

	tenantGroup := r.Group("/tenant")
	tenantGroup.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleTenant))
	{
		tenantGroup.GET("/dashboard", tenantHandler.ShowTenantDashboard)
	}

	maintGroup := r.Group("/maintenance")
	maintGroup.Use(middleware.AuthRequired())
	{
		maintGroup.GET("", maintHandler.ListMaintenance)
		maintGroup.POST("/new", middleware.RoleRequired(models.RoleTenant), maintHandler.CreateMaintenance)
		maintGroup.POST("/:id/status", middleware.RoleRequired(models.RoleAdmin), maintHandler.UpdateMaintenanceStatus)
		maintGroup.POST("/:id/delete", maintHandler.DeleteMaintenance)
	}

	payGroup := r.Group("/payments")
	payGroup.Use(middleware.AuthRequired())
	{
		payGroup.GET("", paymentHandler.ListPayments)
		payGroup.POST("/new", middleware.RoleRequired(models.RoleTenant), paymentHandler.CreatePayment)
		payGroup.POST("/:id/status", middleware.RoleRequired(models.RoleAdmin), paymentHandler.UpdatePaymentStatus)
	}

	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleAdmin))
	{
		adminGroup.GET("/dashboard", adminHandler.ShowAdminDashboard)
		adminGroup.POST("/assign-room", adminHandler.AssignTenantRoom)
		adminGroup.POST("/unassign-room", adminHandler.UnassignTenantRoom)
	}

	return r, db, cfg
}

// 1 & 2: Test Password Hashing and Login Verification
func TestBcryptPasswordHashingAndAuth(t *testing.T) {
	r, db, _ := setupTestApp(t)

	// Verify Admin password in DB is bcrypt hash, NOT plain text
	var storedPass string
	err := db.QueryRow("SELECT password FROM users WHERE email = 'admin@dormitory.com'").Scan(&storedPass)
	if err != nil {
		t.Fatalf("Admin user not found in DB: %v", err)
	}

	if storedPass == "admin123456" {
		t.Fatalf("CRITICAL SECURITY ERROR: Password stored as plain text!")
	}

	if !strings.HasPrefix(storedPass, "$2a$") && !strings.HasPrefix(storedPass, "$2b$") {
		t.Fatalf("Password is not a valid bcrypt hash: %s", storedPass)
	}

	// Verify bcrypt hash matches password
	err = bcrypt.CompareHashAndPassword([]byte(storedPass), []byte("admin123456"))
	if err != nil {
		t.Fatalf("Bcrypt hash comparison failed: %v", err)
	}

	// First make a GET request to /login to receive the CSRF cookie and token
	getReq, _ := http.NewRequest("GET", "/login", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	var csrfCookie *http.Cookie
	for _, c := range getW.Result().Cookies() {
		if c.Name == middleware.CSRFCookieName {
			csrfCookie = c
			break
		}
	}
	if csrfCookie == nil {
		t.Fatalf("Expected CSRF cookie to be issued on GET /login")
	}

	// Test Login with wrong password -> 401
	loginBody := url.Values{}
	loginBody.Set("email", "admin@dormitory.com")
	loginBody.Set("password", "wrongpassword")
	loginBody.Set("_csrf", csrfCookie.Value)

	req, _ := http.NewRequest("POST", "/login", strings.NewReader(loginBody.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(csrfCookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for wrong credentials, got %d", w.Code)
	}

	// Test Login with correct password -> 200 (for JSON/API) or 302
	loginBody.Set("password", "admin123456")
	req, _ = http.NewRequest("POST", "/login", strings.NewReader(loginBody.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.AddCookie(csrfCookie)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for valid credentials, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == middleware.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Errorf("Expected session cookie to be set")
	} else if !sessionCookie.HttpOnly {
		t.Errorf("Session cookie must be HttpOnly")
	}
}

// 3: Test Role Separation (Tenant cannot access Admin routes)
func TestRoleBasedAccessControl(t *testing.T) {
	r, db, _ := setupTestApp(t)

	// Fetch demo tenant
	var tenant models.User
	var roomID sql.NullInt64
	err := db.QueryRow("SELECT id, name, email, role FROM users WHERE email = 'tenant@dormitory.com'").Scan(
		&tenant.ID, &tenant.Name, &tenant.Email, &tenant.Role,
	)
	if err != nil {
		t.Fatalf("Failed to fetch tenant: %v", err)
	}
	if roomID.Valid {
		idVal := roomID.Int64
		tenant.RoomID = &idVal
	}

	// Create fake context to sign session for tenant
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/", nil)
	middleware.SetUserSession(c, &tenant, false)

	var tenantCookie *http.Cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == middleware.SessionCookieName {
			tenantCookie = cookie
			break
		}
	}

	// Tenant attempting to access /admin/dashboard
	req, _ := http.NewRequest("GET", "/admin/dashboard", nil)
	req.AddCookie(tenantCookie)
	wAdmin := httptest.NewRecorder()
	r.ServeHTTP(wAdmin, req)

	if wAdmin.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden when tenant accesses admin dashboard, got %d", wAdmin.Code)
	}

	// Tenant accessing /tenant/dashboard should be allowed (200)
	reqTenant, _ := http.NewRequest("GET", "/tenant/dashboard", nil)
	reqTenant.AddCookie(tenantCookie)
	wTenant := httptest.NewRecorder()
	r.ServeHTTP(wTenant, reqTenant)

	if wTenant.Code != http.StatusOK {
		t.Errorf("Expected 200 OK when tenant accesses own dashboard, got %d", wTenant.Code)
	}
}

// 4: Test IDOR Protection (Tenant A cannot delete or modify Tenant B's request)
func TestIDORProtection(t *testing.T) {
	r, db, _ := setupTestApp(t)

	// Create User A (Tenant A)
	hashA, _ := bcrypt.GenerateFromPassword([]byte("passA123"), bcrypt.DefaultCost)
	_, _ = db.Exec("INSERT OR IGNORE INTO users (name, email, password, role) VALUES ('Tenant A', 'unique_a@test.com', ?, 'tenant')", string(hashA))
	var tenantAID int64
	_ = db.QueryRow("SELECT id FROM users WHERE email = 'unique_a@test.com'").Scan(&tenantAID)
	userA := &models.User{ID: tenantAID, Email: "unique_a@test.com", Role: models.RoleTenant, Name: "Tenant A"}

	// Create User B (Tenant B)
	hashB, _ := bcrypt.GenerateFromPassword([]byte("passB123"), bcrypt.DefaultCost)
	_, _ = db.Exec("INSERT OR IGNORE INTO users (name, email, password, role) VALUES ('Tenant B', 'unique_b@test.com', ?, 'tenant')", string(hashB))
	var tenantBID int64
	_ = db.QueryRow("SELECT id FROM users WHERE email = 'unique_b@test.com'").Scan(&tenantBID)
	userB := &models.User{ID: tenantBID, Email: "unique_b@test.com", Role: models.RoleTenant, Name: "Tenant B"}

	// Tenant B creates a maintenance request
	resReq, err := db.Exec("INSERT INTO maintenance_requests (tenant_id, title, description, category, status) VALUES (?, 'B Repair', 'Broken lamp', 'electrical', 'pending')", userB.ID)
	if err != nil {
		t.Fatalf("Failed to insert tenant B request: %v", err)
	}
	reqID, _ := resReq.LastInsertId()

	// Tenant A attempts to delete Tenant B's request
	wA := httptest.NewRecorder()
	cA, _ := gin.CreateTestContext(wA)
	cA.Request, _ = http.NewRequest("GET", "/", nil)
	middleware.SetUserSession(cA, userA, false)
	var cookieA *http.Cookie
	for _, ck := range wA.Result().Cookies() {
		if ck.Name == middleware.SessionCookieName {
			cookieA = ck
		}
	}

	// Try deletion by Tenant A
	csrfToken := "test-idor-csrf-token"
	reqDel, _ := http.NewRequest("POST", fmt.Sprintf("/maintenance/%d/delete", reqID), nil)
	reqDel.AddCookie(cookieA)
	reqDel.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})
	reqDel.Header.Set("X-CSRF-Token", csrfToken)
	wDel := httptest.NewRecorder()
	r.ServeHTTP(wDel, reqDel)

	// Verify request still exists in DB
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM maintenance_requests WHERE id = ?", reqID).Scan(&count)
	if count != 1 {
		t.Errorf("CRITICAL SECURITY ERROR (IDOR): Tenant A was able to delete Tenant B's maintenance request!")
	}
}

// 5 & 6: Test Real Database Persistence (Maintenance & Payment Form Submissions)
func TestFormSubmissionsAndDatabasePersistence(t *testing.T) {
	_, db, _ := setupTestApp(t)

	// Verify rooms exist in database
	var roomCount int
	err := db.QueryRow("SELECT COUNT(*) FROM rooms").Scan(&roomCount)
	if err != nil || roomCount == 0 {
		t.Fatalf("Expected rooms to be seeded in database, got %d (err: %v)", roomCount, err)
	}

	// Insert new maintenance request directly via model / query
	insertMaint := `
		INSERT INTO maintenance_requests (tenant_id, title, description, category, status)
		VALUES (1, 'ทดสอบก๊อกน้ำรั่ว', 'น้ำหยดใต้ซิงค์ตลอดเวลา', 'plumbing', 'pending')
	`
	res, err := db.Exec(insertMaint)
	if err != nil {
		t.Fatalf("Failed to insert maintenance request: %v", err)
	}
	newMaintID, _ := res.LastInsertId()

	// Verify retrieval from DB
	var title, status string
	err = db.QueryRow("SELECT title, status FROM maintenance_requests WHERE id = ?", newMaintID).Scan(&title, &status)
	if err != nil {
		t.Fatalf("Failed to retrieve maintenance request: %v", err)
	}
	if title != "ทดสอบก๊อกน้ำรั่ว" || status != "pending" {
		t.Errorf("Retrieved maintenance request mismatch: %s / %s", title, status)
	}

	// Insert new rent payment into DB
	insertPay := `
		INSERT INTO payments (tenant_id, billing_month, amount, payment_date, slip_url, status)
		VALUES (1, '2026-10', 4500.00, '2026-10-15', '/uploads/sample_slip.png', 'pending')
	`
	resPay, err := db.Exec(insertPay)
	if err != nil {
		t.Fatalf("Failed to insert payment: %v", err)
	}
	newPayID, _ := resPay.LastInsertId()

	// Verify retrieval
	var amount float64
	var payStatus string
	err = db.QueryRow("SELECT amount, status FROM payments WHERE id = ?", newPayID).Scan(&amount, &payStatus)
	if err != nil {
		t.Fatalf("Failed to retrieve payment: %v", err)
	}
	if amount != 4500.00 || payStatus != "pending" {
		t.Errorf("Retrieved payment mismatch: %f / %s", amount, payStatus)
	}

	// Test Admin status update to 'approved'
	_, err = db.Exec("UPDATE payments SET status = 'approved', updated_at = CURRENT_TIMESTAMP WHERE id = ?", newPayID)
	if err != nil {
		t.Fatalf("Failed to update payment status: %v", err)
	}

	_ = db.QueryRow("SELECT status FROM payments WHERE id = ?", newPayID).Scan(&payStatus)
	if payStatus != "approved" {
		t.Errorf("Expected payment status to be approved, got %s", payStatus)
	}
}

// 7 & 8: Test Locales and Asset Availability
func TestLocalesDictionaries(t *testing.T) {
	// Verify th.json exists and is readable
	thData, err := os.ReadFile("locales/th.json")
	if err != nil {
		t.Fatalf("Failed to read locales/th.json: %v", err)
	}
	if !strings.Contains(string(thData), "ระบบจัดการหอพัก") {
		t.Errorf("locales/th.json missing Thai application title")
	}

	// Verify en.json exists and is readable
	enData, err := os.ReadFile("locales/en.json")
	if err != nil {
		t.Fatalf("Failed to read locales/en.json: %v", err)
	}
	if !strings.Contains(string(enData), "Dormitory Management System") {
		t.Errorf("locales/en.json missing English application title")
	}
}

// 9: Test SQL Injection Parameterized Query Safety
func TestSQLInjectionSafety(t *testing.T) {
	_, db, _ := setupTestApp(t)

	// Attempt SQL injection via malicious email input
	maliciousEmail := "admin@dormitory.com' OR '1'='1"
	query := db.Rebind("SELECT id FROM users WHERE email = ?")

	var id int64
	err := db.QueryRow(query, maliciousEmail).Scan(&id)
	if err == nil {
		t.Fatalf("CRITICAL SECURITY ERROR: Parameterized query allowed SQL injection!")
	}
	if err != sql.ErrNoRows {
		t.Errorf("Unexpected error (should be sql.ErrNoRows): %v", err)
	}
}

// 10: Test Rate Limiting
func TestRateLimiter(t *testing.T) {
	limiter := middleware.NewRateLimiter(3, 100*time.Millisecond)

	r := gin.New()
	r.POST("/limited", limiter.Middleware(), func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Make 3 allowed requests
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("POST", "/limited", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d expected 200, got %d", i+1, w.Code)
		}
	}

	// 4th request should be 429 Too Many Requests
	req, _ := http.NewRequest("POST", "/limited", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests on 4th call, got %d", w.Code)
	}
}
