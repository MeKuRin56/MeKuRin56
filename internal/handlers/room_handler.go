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

type RoomHandler struct {
	cfg *config.Config
}

func NewRoomHandler(cfg *config.Config) *RoomHandler {
	return &RoomHandler{cfg: cfg}
}

// ListRooms handles displaying and filtering rooms
func (h *RoomHandler) ListRooms(c *gin.Context) {
	statusFilter := strings.TrimSpace(c.Query("status"))
	floorFilter := strings.TrimSpace(c.Query("floor"))
	searchQuery := strings.TrimSpace(c.Query("search"))

	db := database.GetDB()

	// Base SQL with join to get tenant name if occupied
	baseSQL := `
		SELECT r.id, r.room_number, r.room_type, r.price, r.floor, r.status, r.description,
		       r.created_at, r.updated_at, u.id, u.name
		FROM rooms r
		LEFT JOIN users u ON u.room_id = r.id AND u.role = 'tenant'
		WHERE 1=1
	`
	var conditions []string
	var args []interface{}

	if statusFilter != "" && statusFilter != "all" {
		conditions = append(conditions, "r.status = ?")
		args = append(args, statusFilter)
	}

	if floorFilter != "" && floorFilter != "all" {
		floorNum, err := strconv.Atoi(floorFilter)
		if err == nil {
			conditions = append(conditions, "r.floor = ?")
			args = append(args, floorNum)
		}
	}

	if searchQuery != "" {
		conditions = append(conditions, "(r.room_number LIKE ? OR r.room_type LIKE ? OR r.description LIKE ?)")
		searchTerm := "%" + searchQuery + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	if len(conditions) > 0 {
		baseSQL += " AND " + strings.Join(conditions, " AND ")
	}

	baseSQL += " ORDER BY r.floor ASC, r.room_number ASC"

	reboundSQL := db.Rebind(baseSQL)
	rows, err := db.Query(reboundSQL, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query error"})
		return
	}
	defer rows.Close()

	var rooms []models.Room
	for rows.Next() {
		var r models.Room
		var tenantID sql.NullInt64
		var tenantName sql.NullString

		err := rows.Scan(
			&r.ID, &r.RoomNumber, &r.RoomType, &r.Price, &r.Floor, &r.Status, &r.Description,
			&r.CreatedAt, &r.UpdatedAt, &tenantID, &tenantName,
		)
		if err != nil {
			continue
		}

		if tenantID.Valid {
			idVal := tenantID.Int64
			r.TenantID = &idVal
		}
		if tenantName.Valid {
			r.TenantName = tenantName.String
		}

		rooms = append(rooms, r)
	}

	currentUser := middleware.GetCurrentUser(c)

	// API or JSON request
	if c.ContentType() == "application/json" || c.GetHeader("Accept") == "application/json" || c.Query("format") == "json" {
		c.JSON(http.StatusOK, gin.H{
			"rooms":  rooms,
			"total":  len(rooms),
			"filter": gin.H{"status": statusFilter, "floor": floorFilter, "search": searchQuery},
		})
		return
	}

	c.HTML(http.StatusOK, "rooms.html", gin.H{
		"Title":        "ห้องพัก | Rooms",
		"Rooms":        rooms,
		"StatusFilter": statusFilter,
		"FloorFilter":  floorFilter,
		"SearchQuery":  searchQuery,
		"User":         currentUser,
		"CSRFToken":    middleware.GetCSRFToken(c),
	})
}

// CreateRoom handles adding a new room (Admin only)
func (h *RoomHandler) CreateRoom(c *gin.Context) {
	roomNumber := strings.TrimSpace(c.PostForm("room_number"))
	roomType := strings.TrimSpace(c.PostForm("room_type"))
	priceStr := strings.TrimSpace(c.PostForm("price"))
	floorStr := strings.TrimSpace(c.PostForm("floor"))
	status := strings.TrimSpace(c.PostForm("status"))
	description := strings.TrimSpace(c.PostForm("description"))

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price < 0 {
		c.Redirect(http.StatusFound, "/rooms?error=invalid_price")
		return
	}

	floor, err := strconv.Atoi(floorStr)
	if err != nil || floor < 1 {
		floor = 1
	}

	if status == "" {
		status = models.RoomStatusVacant
	}

	db := database.GetDB()
	query := db.Rebind(`
		INSERT INTO rooms (room_number, room_type, price, floor, status, description)
		VALUES (?, ?, ?, ?, ?, ?)
	`)

	_, err = db.Exec(query, roomNumber, roomType, price, floor, status, description)
	if err != nil {
		c.Redirect(http.StatusFound, "/rooms?error=room_exists")
		return
	}

	c.Redirect(http.StatusFound, "/rooms?success=created")
}

// UpdateRoom handles editing a room (Admin only)
func (h *RoomHandler) UpdateRoom(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	roomNumber := strings.TrimSpace(c.PostForm("room_number"))
	roomType := strings.TrimSpace(c.PostForm("room_type"))
	priceStr := strings.TrimSpace(c.PostForm("price"))
	floorStr := strings.TrimSpace(c.PostForm("floor"))
	status := strings.TrimSpace(c.PostForm("status"))
	description := strings.TrimSpace(c.PostForm("description"))

	price, _ := strconv.ParseFloat(priceStr, 64)
	floor, _ := strconv.Atoi(floorStr)

	db := database.GetDB()
	query := db.Rebind(`
		UPDATE rooms
		SET room_number = ?, room_type = ?, price = ?, floor = ?, status = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`)

	_, err = db.Exec(query, roomNumber, roomType, price, floor, status, description, id)
	if err != nil {
		c.Redirect(http.StatusFound, "/rooms?error=update_failed")
		return
	}

	c.Redirect(http.StatusFound, "/rooms?success=updated")
}

// DeleteRoom handles deleting a room (Admin only)
func (h *RoomHandler) DeleteRoom(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	db := database.GetDB()

	// Check if any tenant is occupying this room
	var count int
	_ = db.QueryRow(db.Rebind("SELECT COUNT(*) FROM users WHERE room_id = ?"), id).Scan(&count)
	if count > 0 {
		c.Redirect(http.StatusFound, "/rooms?error=room_occupied")
		return
	}

	query := db.Rebind("DELETE FROM rooms WHERE id = ?")
	_, err = db.Exec(query, id)
	if err != nil {
		c.Redirect(http.StatusFound, "/rooms?error=delete_failed")
		return
	}

	c.Redirect(http.StatusFound, "/rooms?success=deleted")
}

// RentRoom allows an authenticated tenant to rent/book a vacant room
func (h *RoomHandler) RentRoom(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.Role != models.RoleTenant {
		c.Redirect(http.StatusFound, "/login?next=/rooms")
		return
	}

	idStr := c.Param("id")
	roomID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/rooms?error=invalid_room")
		return
	}

	db := database.GetDB()

	// Check that room is currently vacant
	var status string
	var roomNumber string
	err = db.QueryRow(db.Rebind("SELECT status, room_number FROM rooms WHERE id = ?"), roomID).Scan(&status, &roomNumber)
	if err != nil || status != models.RoomStatusVacant {
		c.Redirect(http.StatusFound, "/rooms?error=room_not_vacant")
		return
	}

	// If tenant already had an old room, mark the old room as vacant
	if user.RoomID != nil && *user.RoomID != roomID {
		_, _ = db.Exec(db.Rebind("UPDATE rooms SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?"), models.RoomStatusVacant, *user.RoomID)
	}

	// Update user's room_id
	_, err = db.Exec(db.Rebind("UPDATE users SET room_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?"), roomID, user.ID)
	if err != nil {
		c.Redirect(http.StatusFound, "/rooms?error=rent_failed")
		return
	}

	// Mark new room as occupied
	_, err = db.Exec(db.Rebind("UPDATE rooms SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?"), models.RoomStatusOccupied, roomID)
	if err != nil {
		c.Redirect(http.StatusFound, "/rooms?error=rent_failed")
		return
	}

	c.Redirect(http.StatusFound, "/tenant/dashboard?success=rented")
}
