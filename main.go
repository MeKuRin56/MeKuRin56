package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/handlers"
	"dormitory-management-system/internal/middleware"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Load application configuration
	cfg := config.LoadConfig()

	// 2. Initialize database, run migrations, and seed initial data
	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Database initialization failed: %v", err)
	}
	defer db.Close()

	// 3. Initialize authentication secrets
	middleware.InitAuth(cfg)

	// 4. Set Gin Mode
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// 5. Create Gin engine
	r := gin.Default()

	// 6. Global Middlewares
	r.Use(middleware.SecurityHeadersMiddleware())

	// Static Assets & Uploads
	r.Static("/static", "./static")
	r.Static("/locales", "./locales")

	// Secure uploaded slips serving: Authenticated & Authorized (Admin or Slip Owner)
	r.GET("/uploads/:filename", middleware.AuthRequired(), func(c *gin.Context) {
		filename := c.Param("filename")
		cleanFilename := filepath.Base(filename)
		if cleanFilename != filename || strings.Contains(filename, "..") || cleanFilename == "." {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		user := middleware.GetCurrentUser(c)
		if user == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// Authorization check: tenants can only access slips belonging to their own payments
		if user.Role != models.RoleAdmin {
			slipURL := "/uploads/" + cleanFilename
			var ownerID int64
			err := db.QueryRow(db.Rebind("SELECT tenant_id FROM payments WHERE slip_url = ?"), slipURL).Scan(&ownerID)
			if err != nil || ownerID != user.ID {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		filePath := filepath.Join(cfg.UploadDir, cleanFilename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		c.Header("X-Content-Type-Options", "nosniff")
		c.File(filePath)
	})

	// CSRF Protection Middleware
	r.Use(middleware.CSRFMiddleware(cfg.IsProduction()))

	// HTML Templates with custom functions
	r.SetFuncMap(template.FuncMap{
		// derefInt64 safely dereferences a *int64 pointer, returns 0 if nil
		"derefInt64": func(p *int64) int64 {
			if p == nil {
				return 0
			}
			return *p
		},
	})
	r.LoadHTMLGlob("templates/*")

	// Rate limiter for login & registration (15 attempts per minute per IP)
	authLimiter := middleware.NewRateLimiter(15, time.Minute)

	// Handlers initialization
	authHandler := handlers.NewAuthHandler(cfg)
	roomHandler := handlers.NewRoomHandler(cfg)
	maintHandler := handlers.NewMaintenanceHandler(cfg)
	paymentHandler := handlers.NewPaymentHandler(cfg)
	adminHandler := handlers.NewAdminHandler(cfg)
	tenantHandler := handlers.NewTenantHandler(cfg)
	bookingHandler := handlers.NewBookingHandler(cfg)
	profileHandler := handlers.NewProfileHandler(cfg)
	invoiceHandler := handlers.NewInvoiceHandler(cfg)
	announcementHandler := handlers.NewAnnouncementHandler(cfg)

	// ==================== Public & Home Routes ====================
	// Root route lands directly on the Login page as requested by user
	r.GET("/", authHandler.ShowLogin)

	// Home overview page
	r.GET("/home", func(c *gin.Context) {
		currentUser := middleware.GetCurrentUser(c)
		var stats models.DashboardStats

		_ = db.QueryRow("SELECT COUNT(*) FROM rooms").Scan(&stats.TotalRooms)
		_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM rooms WHERE status = ?"), models.RoomStatusVacant).Scan(&stats.VacantRooms)
		_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM rooms WHERE status = ?"), models.RoomStatusOccupied).Scan(&stats.OccupiedRooms)
		_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM rooms WHERE status = ?"), models.RoomStatusMaintenance).Scan(&stats.MaintenanceRooms)

		announcements := handlers.FetchActiveAnnouncements(3)

		c.HTML(http.StatusOK, "index.html", gin.H{
			"Title":         "ระบบจัดการหอพัก | Dormitory MS",
			"User":          currentUser,
			"Stats":         stats,
			"Announcements": announcements,
		})
	})

	// Rooms browsing (Public & Logged-in)
	r.GET("/rooms", roomHandler.ListRooms)

	// ==================== Authentication Routes ====================
	r.GET("/login", authHandler.ShowLogin)
	r.POST("/login", authLimiter.Middleware(), authHandler.HandleLogin)
	r.GET("/register", authHandler.ShowRegister)
	r.POST("/register", authLimiter.Middleware(), authHandler.HandleRegister)
	r.GET("/logout", authHandler.HandleLogout)
	r.GET("/api/me", authHandler.GetCurrentUserAPI)
	r.GET("/demo-login", authHandler.DemoLogin)

	// ==================== Printable Receipt (Authenticated) ====================
	r.GET("/invoices/:id/receipt", middleware.AuthRequired(), invoiceHandler.PrintReceipt)

	// ==================== Tenant Protected Routes ====================
	tenantGroup := r.Group("/tenant")
	tenantGroup.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleTenant))
	{
		tenantGroup.GET("/dashboard", tenantHandler.ShowTenantDashboard)
		tenantGroup.POST("/release-room", tenantHandler.ReleaseRoom)
		tenantGroup.GET("/bookings", bookingHandler.ShowMyBookings)
		tenantGroup.POST("/bookings/:id/cancel", bookingHandler.CancelBooking)
		tenantGroup.GET("/invoices", invoiceHandler.ListInvoices)
	}

	// Profile (all logged-in users)
	profileGroup := r.Group("/profile")
	profileGroup.Use(middleware.AuthRequired())
	{
		profileGroup.GET("", profileHandler.ShowProfile)
		profileGroup.POST("", profileHandler.UpdateProfile)
		profileGroup.POST("/password", authLimiter.Middleware(), profileHandler.ChangePassword)
	}

	// Room booking action (Tenant submits request — admin must approve)
	r.POST("/rooms/:id/book", middleware.AuthRequired(), middleware.RoleRequired(models.RoleTenant), roomHandler.BookRoom)

	// ==================== Maintenance Routes (Authenticated) ====================
	maintGroup := r.Group("/maintenance")
	maintGroup.Use(middleware.AuthRequired())
	{
		maintGroup.GET("", maintHandler.ListMaintenance)
		maintGroup.POST("/new", middleware.RoleRequired(models.RoleTenant), maintHandler.CreateMaintenance)
		maintGroup.POST("/:id/status", middleware.RoleRequired(models.RoleAdmin), maintHandler.UpdateMaintenanceStatus)
		maintGroup.POST("/:id/delete", maintHandler.DeleteMaintenance)
	}

	// ==================== Payment Routes (Authenticated) ====================
	payGroup := r.Group("/payments")
	payGroup.Use(middleware.AuthRequired())
	{
		payGroup.GET("", paymentHandler.ListPayments)
		payGroup.POST("/new", middleware.RoleRequired(models.RoleTenant), paymentHandler.CreatePayment)
		payGroup.POST("/:id/status", middleware.RoleRequired(models.RoleAdmin), paymentHandler.UpdatePaymentStatus)
	}

	// ==================== Admin Protected Routes ====================
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleAdmin))
	{
		adminGroup.GET("/dashboard", adminHandler.ShowAdminDashboard)
		adminGroup.POST("/assign-room", adminHandler.AssignTenantRoom)
		adminGroup.POST("/unassign-room", adminHandler.UnassignTenantRoom)
		adminGroup.GET("/bookings", bookingHandler.ShowBookings)
		adminGroup.POST("/bookings/:id/approve", bookingHandler.ApproveBooking)
		adminGroup.POST("/bookings/:id/reject", bookingHandler.RejectBooking)
		adminGroup.GET("/tenants", adminHandler.ShowTenants)
		adminGroup.POST("/tenants/:id/delete", adminHandler.DeleteTenant)
		adminGroup.GET("/invoices", invoiceHandler.ListInvoices)
		adminGroup.POST("/invoices/new", invoiceHandler.CreateInvoice)
		adminGroup.POST("/invoices/:id/status", invoiceHandler.UpdateInvoiceStatus)
		adminGroup.POST("/invoices/:id/delete", invoiceHandler.DeleteInvoice)
		adminGroup.GET("/invoices/export", invoiceHandler.ExportInvoicesCSV)
		adminGroup.GET("/payments/export", paymentHandler.ExportPaymentsCSV)
		adminGroup.GET("/announcements", announcementHandler.ListAnnouncements)
		adminGroup.POST("/announcements/new", announcementHandler.CreateAnnouncement)
		adminGroup.POST("/announcements/:id/delete", announcementHandler.DeleteAnnouncement)
	}

	// Admin Room Management
	adminRooms := r.Group("/rooms")
	adminRooms.Use(middleware.AuthRequired(), middleware.RoleRequired(models.RoleAdmin))
	{
		adminRooms.POST("/new", roomHandler.CreateRoom)
		adminRooms.POST("/:id/edit", roomHandler.UpdateRoom)
		adminRooms.POST("/:id/delete", roomHandler.DeleteRoom)
	}

	// ==================== 404 Not Found Handler ====================
	r.NoRoute(func(c *gin.Context) {
		currentUser := middleware.GetCurrentUser(c)
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"Title":   "404 Not Found",
			"Message": "ขออภัย ไม่พบหน้าที่คุณร้องขอ (The requested page could not be found)",
			"User":    currentUser,
		})
	})

	// 7. Start HTTP Server
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("=================================================================")
	log.Printf("  Dormitory Management System is running at http://localhost:%s", cfg.Port)
	log.Printf("  Database Driver: %s", db.Driver)
	log.Printf("  Environment:     %s", cfg.Environment)
	log.Printf("=================================================================")

	if err := r.Run(addr); err != nil {
		log.Fatalf("[FATAL] Server failed to start: %v", err)
	}
}
