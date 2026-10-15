package handlers

import (
	"net/http"
	"strconv"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/middleware"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
)

type BookingHandler struct {
	cfg *config.Config
}

func NewBookingHandler(cfg *config.Config) *BookingHandler {
	return &BookingHandler{cfg: cfg}
}

// ShowBookings shows all booking requests for admin
func (h *BookingHandler) ShowBookings(c *gin.Context) {
	currentUser := middleware.GetCurrentUser(c)
	db := database.GetDB()

	filterStatus := c.Query("status")

	query := `
		SELECT b.id, b.room_id, r.room_number, r.room_type, r.price,
		       b.tenant_id, u.name, u.email, COALESCE(u.phone,''),
		       b.status, COALESCE(b.message,''), COALESCE(b.admin_note,''),
		       b.created_at, b.updated_at
		FROM room_bookings b
		JOIN rooms r ON b.room_id = r.id
		JOIN users u ON b.tenant_id = u.id
	`
	args := []interface{}{}
	if filterStatus != "" {
		query += " WHERE b.status = ?"
		args = append(args, filterStatus)
	}
	query += " ORDER BY b.created_at DESC"

	rows, err := db.Query(db.Rebind(query), args...)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Title":   "Error",
			"Message": "ไม่สามารถดึงข้อมูลคำขอจองได้",
			"User":    currentUser,
		})
		return
	}
	defer rows.Close()

	var bookings []models.RoomBooking
	for rows.Next() {
		var b models.RoomBooking
		if err := rows.Scan(
			&b.ID, &b.RoomID, &b.RoomNumber, &b.RoomType, &b.Price,
			&b.TenantID, &b.TenantName, &b.TenantEmail, &b.TenantPhone,
			&b.Status, &b.Message, &b.AdminNote,
			&b.CreatedAt, &b.UpdatedAt,
		); err == nil {
			bookings = append(bookings, b)
		}
	}

	c.HTML(http.StatusOK, "admin_bookings.html", gin.H{
		"Title":        "จัดการคำขอจองห้อง",
		"User":         currentUser,
		"Bookings":     bookings,
		"FilterStatus": filterStatus,
		"CSRFToken":    c.GetString("csrf_token"),
		"Success":      c.Query("success"),
		"Error":        c.Query("error"),
	})
}

// ApproveBooking approves a pending booking: assigns tenant to room
func (h *BookingHandler) ApproveBooking(c *gin.Context) {
	idStr := c.Param("id")
	bookingID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/bookings?error=invalid_id")
		return
	}

	adminNote := c.PostForm("admin_note")
	db := database.GetDB()

	// Fetch booking details
	var b models.RoomBooking
	q := db.Rebind("SELECT id, room_id, tenant_id, status FROM room_bookings WHERE id = ?")
	err = db.QueryRow(q, bookingID).Scan(&b.ID, &b.RoomID, &b.TenantID, &b.Status)
	if err != nil || b.Status != models.BookingStatusPending {
		c.Redirect(http.StatusFound, "/admin/bookings?error=not_found_or_not_pending")
		return
	}

	// Verify room is still vacant
	var roomStatus string
	_ = db.QueryRow(db.Rebind("SELECT status FROM rooms WHERE id = ?"), b.RoomID).Scan(&roomStatus)
	if roomStatus != models.RoomStatusVacant {
		// Mark booking rejected — room taken
		_, _ = db.Exec(
			db.Rebind("UPDATE room_bookings SET status='rejected', admin_note=?, updated_at=CURRENT_TIMESTAMP WHERE id=?"),
			"ห้องไม่ว่างแล้ว ไม่สามารถอนุมัติได้", bookingID,
		)
		c.Redirect(http.StatusFound, "/admin/bookings?error=room_no_longer_vacant")
		return
	}

	// Reject any other pending bookings for the same room
	_, _ = db.Exec(
		db.Rebind("UPDATE room_bookings SET status='rejected', admin_note='ห้องถูกจองโดยผู้เช่าท่านอื่นแล้ว', updated_at=CURRENT_TIMESTAMP WHERE room_id=? AND status='pending' AND id != ?"),
		b.RoomID, bookingID,
	)

	// Mark the approved booking
	_, err = db.Exec(
		db.Rebind("UPDATE room_bookings SET status='approved', admin_note=?, updated_at=CURRENT_TIMESTAMP WHERE id=?"),
		adminNote, bookingID,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/bookings?error=update_failed")
		return
	}

	// Release tenant's old room if they had one
	var oldRoomID *int64
	var nullRoomID *int64
	row := db.QueryRow(db.Rebind("SELECT room_id FROM users WHERE id=?"), b.TenantID)
	_ = row.Scan(&oldRoomID)
	if oldRoomID != nil && *oldRoomID != b.RoomID {
		_, _ = db.Exec(
			db.Rebind("UPDATE rooms SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?"),
			models.RoomStatusVacant, *oldRoomID,
		)
	}
	_ = nullRoomID

	// Assign room to tenant
	_, err = db.Exec(
		db.Rebind("UPDATE users SET room_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?"),
		b.RoomID, b.TenantID,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/bookings?error=assign_failed")
		return
	}

	// Mark room as occupied
	_, _ = db.Exec(
		db.Rebind("UPDATE rooms SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?"),
		models.RoomStatusOccupied, b.RoomID,
	)

	c.Redirect(http.StatusFound, "/admin/bookings?success=approved")
}

// RejectBooking rejects a pending booking
func (h *BookingHandler) RejectBooking(c *gin.Context) {
	idStr := c.Param("id")
	bookingID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/bookings?error=invalid_id")
		return
	}

	adminNote := c.PostForm("admin_note")
	db := database.GetDB()

	res, err := db.Exec(
		db.Rebind("UPDATE room_bookings SET status='rejected', admin_note=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND status='pending'"),
		adminNote, bookingID,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/bookings?error=update_failed")
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.Redirect(http.StatusFound, "/admin/bookings?error=not_found_or_not_pending")
		return
	}

	c.Redirect(http.StatusFound, "/admin/bookings?success=rejected")
}

// ShowMyBookings shows the current tenant's own booking requests
func (h *BookingHandler) ShowMyBookings(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	db := database.GetDB()

	q := db.Rebind(`
		SELECT b.id, b.room_id, r.room_number, r.room_type, r.price,
		       b.tenant_id, u.name, u.email, COALESCE(u.phone,''),
		       b.status, COALESCE(b.message,''), COALESCE(b.admin_note,''),
		       b.created_at, b.updated_at
		FROM room_bookings b
		JOIN rooms r ON b.room_id = r.id
		JOIN users u ON b.tenant_id = u.id
		WHERE b.tenant_id = ?
		ORDER BY b.created_at DESC
	`)
	rows, err := db.Query(q, user.ID)
	if err != nil {
		c.Redirect(http.StatusFound, "/tenant/dashboard?error=load_failed")
		return
	}
	defer rows.Close()

	var bookings []models.RoomBooking
	for rows.Next() {
		var b models.RoomBooking
		if err := rows.Scan(
			&b.ID, &b.RoomID, &b.RoomNumber, &b.RoomType, &b.Price,
			&b.TenantID, &b.TenantName, &b.TenantEmail, &b.TenantPhone,
			&b.Status, &b.Message, &b.AdminNote,
			&b.CreatedAt, &b.UpdatedAt,
		); err == nil {
			bookings = append(bookings, b)
		}
	}

	c.HTML(http.StatusOK, "my_bookings.html", gin.H{
		"Title":     "คำขอจองของฉัน",
		"User":      user,
		"Bookings":  bookings,
		"CSRFToken": middleware.GetCSRFToken(c),
		"Success":   c.Query("success"),
		"Error":     c.Query("error"),
	})
}

// CancelBooking allows tenant to cancel their own pending booking
func (h *BookingHandler) CancelBooking(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	idStr := c.Param("id")
	bookingID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/tenant/bookings?error=invalid_id")
		return
	}

	db := database.GetDB()
	// IDOR check: tenant can only cancel their own pending bookings
	res, err := db.Exec(
		db.Rebind("UPDATE room_bookings SET status='cancelled', updated_at=CURRENT_TIMESTAMP WHERE id=? AND tenant_id=? AND status='pending'"),
		bookingID, user.ID,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/tenant/bookings?error=cancel_failed")
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.Redirect(http.StatusFound, "/tenant/bookings?error=not_found")
		return
	}

	c.Redirect(http.StatusFound, "/tenant/bookings?success=cancelled")
}
