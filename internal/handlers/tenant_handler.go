package handlers

import (
	"database/sql"
	"net/http"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/middleware"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
)

type TenantHandler struct {
	cfg *config.Config
}

func NewTenantHandler(cfg *config.Config) *TenantHandler {
	return &TenantHandler{cfg: cfg}
}

// ShowTenantDashboard displays personal dashboard for tenant
func (h *TenantHandler) ShowTenantDashboard(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	db := database.GetDB()

	// 1. Get room details if assigned
	var room *models.Room
	if user.RoomID != nil {
		var r models.Room
		roomQuery := db.Rebind(`
			SELECT id, room_number, room_type, price, floor, status, description
			FROM rooms
			WHERE id = ?
		`)
		err := db.QueryRow(roomQuery, *user.RoomID).Scan(
			&r.ID, &r.RoomNumber, &r.RoomType, &r.Price, &r.Floor, &r.Status, &r.Description,
		)
		if err == nil {
			room = &r
		}
	}

	// 2. Fetch tenant's maintenance requests (IDOR safe)
	maintQuery := db.Rebind(`
		SELECT m.id, m.tenant_id, u.name, u.email, m.room_id, COALESCE(r.room_number, '-'),
		       m.title, m.description, m.category, m.status, COALESCE(m.admin_notes, ''),
		       m.created_at, m.updated_at
		FROM maintenance_requests m
		JOIN users u ON m.tenant_id = u.id
		LEFT JOIN rooms r ON m.room_id = r.id
		WHERE m.tenant_id = ?
		ORDER BY m.created_at DESC
		LIMIT 10
	`)
	maintRows, _ := db.Query(maintQuery, user.ID)
	var requests []models.MaintenanceRequest
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
			requests = append(requests, m)
		}
	}

	// 3. Fetch tenant's payment records (IDOR safe)
	payQuery := db.Rebind(`
		SELECT p.id, p.tenant_id, u.name, u.email, p.room_id, COALESCE(r.room_number, '-'),
		       p.billing_month, p.amount, p.payment_date, COALESCE(p.slip_url, ''),
		       p.status, COALESCE(p.rejection_reason, ''), p.created_at, p.updated_at
		FROM payments p
		JOIN users u ON p.tenant_id = u.id
		LEFT JOIN rooms r ON p.room_id = r.id
		WHERE p.tenant_id = ?
		ORDER BY p.created_at DESC
		LIMIT 10
	`)
	payRows, _ := db.Query(payQuery, user.ID)
	var payments []models.Payment
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
			payments = append(payments, p)
		}
	}

	c.HTML(http.StatusOK, "dashboard_tenant.html", gin.H{
		"Title":     "แดชบอร์ดผู้เช่า | Tenant Dashboard",
		"User":      user,
		"Room":      room,
		"Requests":  requests,
		"Payments":  payments,
		"CSRFToken": middleware.GetCSRFToken(c),
		"Success":   c.Query("success"),
		"Error":     c.Query("error"),
	})
}

// ReleaseRoom allows a tenant to vacate/cancel their current room assignment
func (h *TenantHandler) ReleaseRoom(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.RoomID == nil {
		c.Redirect(http.StatusFound, "/tenant/dashboard")
		return
	}

	db := database.GetDB()
	// Mark room vacant
	_, _ = db.Exec(db.Rebind("UPDATE rooms SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?"), models.RoomStatusVacant, *user.RoomID)
	// Clear user's room_id
	_, _ = db.Exec(db.Rebind("UPDATE users SET room_id = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?"), user.ID)

	c.Redirect(http.StatusFound, "/tenant/dashboard?success=released")
}
