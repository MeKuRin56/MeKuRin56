package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/middleware"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
)

type MaintenanceHandler struct {
	cfg *config.Config
}

func NewMaintenanceHandler(cfg *config.Config) *MaintenanceHandler {
	return &MaintenanceHandler{cfg: cfg}
}

// ListMaintenance shows maintenance requests
// Tenant sees only their own requests; Admin sees all requests with status/search filter
func (h *MaintenanceHandler) ListMaintenance(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	statusFilter := strings.TrimSpace(c.Query("status"))
	categoryFilter := strings.TrimSpace(c.Query("category"))
	searchQuery := strings.TrimSpace(c.Query("search"))

	db := database.GetDB()

	baseSQL := `
		SELECT m.id, m.tenant_id, u.name, u.email, m.room_id, COALESCE(r.room_number, '-'),
		       m.title, m.description, m.category, m.status, COALESCE(m.admin_notes, ''),
		       m.created_at, m.updated_at
		FROM maintenance_requests m
		JOIN users u ON m.tenant_id = u.id
		LEFT JOIN rooms r ON m.room_id = r.id
		WHERE 1=1
	`
	var conditions []string
	var args []interface{}

	// If tenant, IDOR protection: strictly enforce tenant_id
	if user.Role == models.RoleTenant {
		conditions = append(conditions, "m.tenant_id = ?")
		args = append(args, user.ID)
	}

	if statusFilter != "" && statusFilter != "all" {
		conditions = append(conditions, "m.status = ?")
		args = append(args, statusFilter)
	}

	if categoryFilter != "" && categoryFilter != "all" {
		conditions = append(conditions, "m.category = ?")
		args = append(args, categoryFilter)
	}

	if searchQuery != "" {
		conditions = append(conditions, "(m.title LIKE ? OR m.description LIKE ? OR u.name LIKE ?)")
		term := "%" + searchQuery + "%"
		args = append(args, term, term, term)
	}

	if len(conditions) > 0 {
		baseSQL += " AND " + strings.Join(conditions, " AND ")
	}

	baseSQL += " ORDER BY m.created_at DESC"

	reboundSQL := db.Rebind(baseSQL)
	rows, err := db.Query(reboundSQL, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load requests"})
		return
	}
	defer rows.Close()

	var requests []models.MaintenanceRequest
	for rows.Next() {
		var req models.MaintenanceRequest
		var roomID sql.NullInt64

		err := rows.Scan(
			&req.ID, &req.TenantID, &req.TenantName, &req.TenantEmail, &roomID, &req.RoomNumber,
			&req.Title, &req.Description, &req.Category, &req.Status, &req.AdminNotes,
			&req.CreatedAt, &req.UpdatedAt,
		)
		if err != nil {
			continue
		}

		if roomID.Valid {
			idVal := roomID.Int64
			req.RoomID = &idVal
		}

		requests = append(requests, req)
	}

	if c.Query("format") == "json" {
		c.JSON(http.StatusOK, gin.H{"requests": requests, "total": len(requests)})
		return
	}

	c.HTML(http.StatusOK, "maintenance.html", gin.H{
		"Title":          "ระบบแจ้งซ่อม | Maintenance",
		"Requests":       requests,
		"StatusFilter":   statusFilter,
		"CategoryFilter": categoryFilter,
		"SearchQuery":    searchQuery,
		"User":           user,
		"CSRFToken":      middleware.GetCSRFToken(c),
		"Success":        c.Query("success"),
		"Error":          c.Query("error"),
	})
}

// CreateMaintenance handles submitting a new repair ticket
func (h *MaintenanceHandler) CreateMaintenance(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	description := strings.TrimSpace(c.PostForm("description"))
	category := strings.TrimSpace(c.PostForm("category"))

	if title == "" || description == "" {
		c.Redirect(http.StatusFound, "/maintenance?error=missing_fields")
		return
	}

	if category == "" {
		category = models.CategoryOther
	}

	db := database.GetDB()

	// If tenant has an assigned room, attach room_id
	var roomID *int64 = user.RoomID

	query := db.Rebind(`
		INSERT INTO maintenance_requests (tenant_id, room_id, title, description, category, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`)

	_, err := db.Exec(query, user.ID, roomID, title, description, category, models.MaintStatusPending)
	if err != nil {
		c.Redirect(http.StatusFound, "/maintenance?error=create_failed")
		return
	}

	c.Redirect(http.StatusFound, "/maintenance?success=created")
}

// UpdateMaintenanceStatus handles status changes by admin
func (h *MaintenanceHandler) UpdateMaintenanceStatus(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.Role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Admin only"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	status := strings.TrimSpace(c.PostForm("status"))
	adminNotes := strings.TrimSpace(c.PostForm("admin_notes"))

	if status != models.MaintStatusPending && status != models.MaintStatusInProgress && status != models.MaintStatusCompleted {
		c.Redirect(http.StatusFound, "/maintenance?error=invalid_status")
		return
	}

	db := database.GetDB()
	query := db.Rebind(`
		UPDATE maintenance_requests
		SET status = ?, admin_notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`)

	_, err = db.Exec(query, status, adminNotes, id)
	if err != nil {
		c.Redirect(http.StatusFound, "/maintenance?error=update_failed")
		return
	}

	c.Redirect(http.StatusFound, "/maintenance?success=updated")
}

// DeleteMaintenance handles deletion with IDOR check
func (h *MaintenanceHandler) DeleteMaintenance(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	db := database.GetDB()

	// Server-side ownership / permission verification (IDOR protection)
	var reqTenantID int64
	var reqStatus string
	checkQuery := db.Rebind("SELECT tenant_id, status FROM maintenance_requests WHERE id = ?")
	err = db.QueryRow(checkQuery, id).Scan(&reqTenantID, &reqStatus)
	if err != nil {
		c.Redirect(http.StatusFound, "/maintenance?error=not_found")
		return
	}

	// If tenant, they can only delete their own request and only if status is pending
	if user.Role == models.RoleTenant {
		if reqTenantID != user.ID {
			c.Redirect(http.StatusFound, "/maintenance?error=forbidden")
			return
		}
		if reqStatus != models.MaintStatusPending {
			c.Redirect(http.StatusFound, "/maintenance?error=cannot_delete_active")
			return
		}
	}

	deleteQuery := db.Rebind("DELETE FROM maintenance_requests WHERE id = ?")
	_, err = db.Exec(deleteQuery, id)
	if err != nil {
		c.Redirect(http.StatusFound, "/maintenance?error=delete_failed")
		return
	}

	c.Redirect(http.StatusFound, "/maintenance?success=deleted")
}
