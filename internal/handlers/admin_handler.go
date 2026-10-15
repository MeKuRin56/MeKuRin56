package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/middleware"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	cfg *config.Config
}

func NewAdminHandler(cfg *config.Config) *AdminHandler {
	return &AdminHandler{cfg: cfg}
}

// ShowAdminDashboard renders the admin overview with stats, recent lists, and management tools
func (h *AdminHandler) ShowAdminDashboard(c *gin.Context) {
	currentUser := middleware.GetCurrentUser(c)
	db := database.GetDB()

	var stats models.DashboardStats

	// Compute stats
	_ = db.QueryRow("SELECT COUNT(*) FROM rooms").Scan(&stats.TotalRooms)
	_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM rooms WHERE status = ?"), models.RoomStatusVacant).Scan(&stats.VacantRooms)
	_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM rooms WHERE status = ?"), models.RoomStatusOccupied).Scan(&stats.OccupiedRooms)
	_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM rooms WHERE status = ?"), models.RoomStatusMaintenance).Scan(&stats.MaintenanceRooms)
	_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM users WHERE role = ?"), models.RoleTenant).Scan(&stats.TotalTenants)
	_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM maintenance_requests WHERE status = ?"), models.MaintStatusPending).Scan(&stats.PendingRequests)
	_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM payments WHERE status = ?"), models.PaymentStatusPending).Scan(&stats.PendingPayments)
	_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM room_bookings WHERE status = ?"), models.BookingStatusPending).Scan(&stats.PendingBookings)
	_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM invoices WHERE status = ?"), models.InvoiceStatusUnpaid).Scan(&stats.UnpaidInvoices)

	currentMonth := time.Now().Format("2006-01")
	var monthlyRev sql.NullFloat64
	revQuery := db.Rebind("SELECT SUM(amount) FROM payments WHERE status = ? AND billing_month = ?")
	_ = db.QueryRow(revQuery, models.PaymentStatusApproved, currentMonth).Scan(&monthlyRev)
	if monthlyRev.Valid {
		stats.MonthlyRevenue = monthlyRev.Float64
	}

	// Fetch recent 5 maintenance requests
	maintQuery := db.Rebind(`
		SELECT m.id, m.tenant_id, u.name, u.email, m.room_id, COALESCE(r.room_number, '-'),
		       m.title, m.description, m.category, m.status, COALESCE(m.admin_notes, ''),
		       m.created_at, m.updated_at
		FROM maintenance_requests m
		JOIN users u ON m.tenant_id = u.id
		LEFT JOIN rooms r ON m.room_id = r.id
		ORDER BY m.created_at DESC
		LIMIT 5
	`)
	maintRows, _ := db.Query(maintQuery)
	var recentMaint []models.MaintenanceRequest
	if maintRows != nil {
		defer maintRows.Close()
		for maintRows.Next() {
			var m models.MaintenanceRequest
			var rID sql.NullInt64
			_ = maintRows.Scan(&m.ID, &m.TenantID, &m.TenantName, &m.TenantEmail, &rID, &m.RoomNumber,
				&m.Title, &m.Description, &m.Category, &m.Status, &m.AdminNotes, &m.CreatedAt, &m.UpdatedAt)
			if rID.Valid {
				idVal := rID.Int64
				m.RoomID = &idVal
			}
			recentMaint = append(recentMaint, m)
		}
	}

	// Fetch recent 5 payments
	payQuery := db.Rebind(`
		SELECT p.id, p.tenant_id, u.name, u.email, p.room_id, COALESCE(r.room_number, '-'),
		       p.billing_month, p.amount, p.payment_date, COALESCE(p.slip_url, ''),
		       p.status, COALESCE(p.rejection_reason, ''), p.created_at, p.updated_at
		FROM payments p
		JOIN users u ON p.tenant_id = u.id
		LEFT JOIN rooms r ON p.room_id = r.id
		ORDER BY p.created_at DESC
		LIMIT 5
	`)
	payRows, _ := db.Query(payQuery)
	var recentPayments []models.Payment
	if payRows != nil {
		defer payRows.Close()
		for payRows.Next() {
			var p models.Payment
			var rID sql.NullInt64
			_ = payRows.Scan(&p.ID, &p.TenantID, &p.TenantName, &p.TenantEmail, &rID, &p.RoomNumber,
				&p.BillingMonth, &p.Amount, &p.PaymentDate, &p.SlipURL,
				&p.Status, &p.RejectionReason, &p.CreatedAt, &p.UpdatedAt)
			if rID.Valid {
				idVal := rID.Int64
				p.RoomID = &idVal
			}
			recentPayments = append(recentPayments, p)
		}
	}

	// Fetch tenants list
	tenantsQuery := db.Rebind(`
		SELECT u.id, u.name, u.email, u.phone, u.room_id, COALESCE(r.room_number, '-'), u.created_at
		FROM users u
		LEFT JOIN rooms r ON u.room_id = r.id
		WHERE u.role = 'tenant'
		ORDER BY u.id DESC
	`)
	tenantRows, _ := db.Query(tenantsQuery)
	var tenants []models.User
	if tenantRows != nil {
		defer tenantRows.Close()
		for tenantRows.Next() {
			var t models.User
			var rID sql.NullInt64
			_ = tenantRows.Scan(&t.ID, &t.Name, &t.Email, &t.Phone, &rID, &t.RoomNo, &t.CreatedAt)
			if rID.Valid {
				idVal := rID.Int64
				t.RoomID = &idVal
			}
			tenants = append(tenants, t)
		}
	}

	// Fetch all rooms for assignment dropdown
	roomRows, _ := db.Query("SELECT id, room_number, room_type, price, floor, status FROM rooms ORDER BY room_number ASC")
	var allRooms []models.Room
	if roomRows != nil {
		defer roomRows.Close()
		for roomRows.Next() {
			var rm models.Room
			_ = roomRows.Scan(&rm.ID, &rm.RoomNumber, &rm.RoomType, &rm.Price, &rm.Floor, &rm.Status)
			allRooms = append(allRooms, rm)
		}
	}

	announcements := FetchActiveAnnouncements(5)

	c.HTML(http.StatusOK, "dashboard_admin.html", gin.H{
		"Title":          "ผู้ดูแลระบบ | Admin Dashboard",
		"Stats":          stats,
		"RecentMaint":    recentMaint,
		"RecentPayments": recentPayments,
		"Tenants":        tenants,
		"Rooms":          allRooms,
		"Announcements":    announcements,
		"User":             currentUser,
		"CSRFToken":        middleware.GetCSRFToken(c),
		"Success":          c.Query("success"),
		"Error":            c.Query("error"),
		"EmergencyWarning": h.cfg.EmergencyAdminPassword != "",
	})
}

// AssignTenantRoom assigns a room to a tenant
func (h *AdminHandler) AssignTenantRoom(c *gin.Context) {
	tenantIDStr := strings.TrimSpace(c.PostForm("tenant_id"))
	roomIDStr := strings.TrimSpace(c.PostForm("room_id"))

	tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/dashboard?error=invalid_tenant")
		return
	}

	roomID, err := strconv.ParseInt(roomIDStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/dashboard?error=invalid_room")
		return
	}

	db := database.GetDB()

	// Update user room_id
	updateUser := db.Rebind("UPDATE users SET room_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	_, err = db.Exec(updateUser, roomID, tenantID)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/dashboard?error=assign_failed")
		return
	}

	// Update room status to occupied
	updateRoom := db.Rebind("UPDATE rooms SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	_, _ = db.Exec(updateRoom, models.RoomStatusOccupied, roomID)

	c.Redirect(http.StatusFound, "/admin/dashboard?success=assigned")
}

// UnassignTenantRoom frees a room from tenant
func (h *AdminHandler) UnassignTenantRoom(c *gin.Context) {
	tenantIDStr := strings.TrimSpace(c.PostForm("tenant_id"))
	tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/dashboard?error=invalid_tenant")
		return
	}

	db := database.GetDB()

	// Get current room_id
	var roomID sql.NullInt64
	_ = db.QueryRow(db.Rebind("SELECT room_id FROM users WHERE id = ?"), tenantID).Scan(&roomID)

	// Unlink user
	_, err = db.Exec(db.Rebind("UPDATE users SET room_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?"), tenantID)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/dashboard?error=unassign_failed")
		return
	}

	// Mark room vacant if it was occupied
	if roomID.Valid {
		_, _ = db.Exec(db.Rebind("UPDATE rooms SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?"), models.RoomStatusVacant, roomID.Int64)
	}

	c.Redirect(http.StatusFound, "/admin/dashboard?success=unassigned")
}
// ShowTenants renders the dedicated tenant management page for admins
func (h *AdminHandler) ShowTenants(c *gin.Context) {
	currentUser := middleware.GetCurrentUser(c)
	db := database.GetDB()

	search := strings.TrimSpace(c.Query("search"))

	query := `
		SELECT u.id, u.name, u.email, u.phone, u.room_id, COALESCE(r.room_number, '-'), u.created_at
		FROM users u
		LEFT JOIN rooms r ON u.room_id = r.id
		WHERE u.role = 'tenant'
	`
	args := []interface{}{}
	if search != "" {
		query += " AND (u.name LIKE ? OR u.email LIKE ? OR r.room_number LIKE ?)"
		term := "%" + search + "%"
		args = append(args, term, term, term)
	}
	query += " ORDER BY u.id DESC"

	rows, _ := db.Query(db.Rebind(query), args...)
	var tenants []models.User
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var t models.User
			var rID sql.NullInt64
			_ = rows.Scan(&t.ID, &t.Name, &t.Email, &t.Phone, &rID, &t.RoomNo, &t.CreatedAt)
			if rID.Valid {
				idVal := rID.Int64
				t.RoomID = &idVal
			}
			tenants = append(tenants, t)
		}
	}

	// All rooms for assignment dropdown
	roomRows, _ := db.Query("SELECT id, room_number, room_type, price, floor, status FROM rooms ORDER BY room_number ASC")
	var allRooms []models.Room
	if roomRows != nil {
		defer roomRows.Close()
		for roomRows.Next() {
			var rm models.Room
			_ = roomRows.Scan(&rm.ID, &rm.RoomNumber, &rm.RoomType, &rm.Price, &rm.Floor, &rm.Status)
			allRooms = append(allRooms, rm)
		}
	}

	c.HTML(http.StatusOK, "admin_tenants.html", gin.H{
		"Title":     "จัดการผู้เช่า | Tenant Management",
		"User":      currentUser,
		"Tenants":   tenants,
		"Rooms":     allRooms,
		"Search":    search,
		"CSRFToken": middleware.GetCSRFToken(c),
		"Success":   c.Query("success"),
		"Error":     c.Query("error"),
	})
}

// DeleteTenant deletes a tenant account (admin only, cannot delete self)
func (h *AdminHandler) DeleteTenant(c *gin.Context) {
	currentUser := middleware.GetCurrentUser(c)
	idStr := c.Param("id")
	tenantID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/tenants?error=invalid_id")
		return
	}

	// Safety: cannot delete yourself
	if tenantID == currentUser.ID {
		c.Redirect(http.StatusFound, "/admin/tenants?error=cannot_delete_self")
		return
	}

	db := database.GetDB()

	// Verify target is actually a tenant (not an admin)
	var role string
	err = db.QueryRow(db.Rebind("SELECT role FROM users WHERE id=?"), tenantID).Scan(&role)
	if err != nil || role != models.RoleTenant {
		c.Redirect(http.StatusFound, "/admin/tenants?error=not_found_or_not_tenant")
		return
	}

	// Free the room before deleting
	var roomID sql.NullInt64
	_ = db.QueryRow(db.Rebind("SELECT room_id FROM users WHERE id=?"), tenantID).Scan(&roomID)
	if roomID.Valid {
		_, _ = db.Exec(db.Rebind("UPDATE rooms SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?"),
			models.RoomStatusVacant, roomID.Int64)
	}

	// Cancel pending bookings
	_, _ = db.Exec(db.Rebind("UPDATE room_bookings SET status='cancelled' WHERE tenant_id=? AND status='pending'"), tenantID)

	// Delete user
	_, err = db.Exec(db.Rebind("DELETE FROM users WHERE id=? AND role='tenant'"), tenantID)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/tenants?error=delete_failed")
		return
	}

	c.Redirect(http.StatusFound, "/admin/tenants?success=deleted")
}
