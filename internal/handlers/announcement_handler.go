package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/middleware"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
)

type AnnouncementHandler struct {
	cfg *config.Config
}

func NewAnnouncementHandler(cfg *config.Config) *AnnouncementHandler {
	return &AnnouncementHandler{cfg: cfg}
}

// ListAnnouncements lists all announcements for admin management
func (h *AnnouncementHandler) ListAnnouncements(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	db := database.GetDB()

	rows, err := db.Query(`
		SELECT a.id, a.title, a.content, a.is_urgent, a.created_by, COALESCE(u.name, 'Admin'), a.created_at, a.updated_at
		FROM announcements a
		LEFT JOIN users u ON a.created_by = u.id
		ORDER BY a.is_urgent DESC, a.created_at DESC
	`)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Title":   "Error",
			"Message": "ไม่สามารถโหลดข้อมูลประกาศได้",
			"User":    user,
		})
		return
	}
	defer rows.Close()

	var announcements []models.Announcement
	for rows.Next() {
		var a models.Announcement
		var isUrgentVal interface{}
		if err := rows.Scan(&a.ID, &a.Title, &a.Content, &isUrgentVal, &a.CreatedBy, &a.AuthorName, &a.CreatedAt, &a.UpdatedAt); err == nil {
			switch v := isUrgentVal.(type) {
			case bool:
				a.IsUrgent = v
			case int64:
				a.IsUrgent = (v == 1)
			}
			announcements = append(announcements, a)
		}
	}

	c.HTML(http.StatusOK, "admin_announcements.html", gin.H{
		"Title":         "จัดการประกาศและข่าวสาร | Announcements",
		"User":          user,
		"Announcements": announcements,
		"CSRFToken":     middleware.GetCSRFToken(c),
		"Success":       c.Query("success"),
		"Error":         c.Query("error"),
	})
}

// CreateAnnouncement adds a new announcement (Admin only)
func (h *AnnouncementHandler) CreateAnnouncement(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.Role != models.RoleAdmin {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	content := strings.TrimSpace(c.PostForm("content"))
	isUrgent := c.PostForm("is_urgent") == "1" || c.PostForm("is_urgent") == "true"

	if title == "" || content == "" {
		c.Redirect(http.StatusFound, "/admin/announcements?error=fields_required")
		return
	}

	if len(title) > 200 {
		title = title[:200]
	}

	db := database.GetDB()
	var isUrgentParam interface{} = isUrgent
	if db.Driver == "sqlite" {
		if isUrgent {
			isUrgentParam = 1
		} else {
			isUrgentParam = 0
		}
	}

	_, err := db.Exec(
		db.Rebind("INSERT INTO announcements (title, content, is_urgent, created_by) VALUES (?, ?, ?, ?)"),
		title, content, isUrgentParam, user.ID,
	)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/announcements?error=create_failed")
		return
	}

	c.Redirect(http.StatusFound, "/admin/announcements?success=created")
}

// DeleteAnnouncement deletes an announcement (Admin only)
func (h *AnnouncementHandler) DeleteAnnouncement(c *gin.Context) {
	idStr := c.Param("id")
	announcementID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/announcements?error=invalid_id")
		return
	}

	db := database.GetDB()
	_, err = db.Exec(db.Rebind("DELETE FROM announcements WHERE id = ?"), announcementID)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/announcements?error=delete_failed")
		return
	}

	c.Redirect(http.StatusFound, "/admin/announcements?success=deleted")
}

// FetchActiveAnnouncements is a helper to fetch recent announcements for dashboards
func FetchActiveAnnouncements(limit int) []models.Announcement {
	db := database.GetDB()
	rows, err := db.Query(db.Rebind(`
		SELECT a.id, a.title, a.content, a.is_urgent, a.created_by, COALESCE(u.name, 'Admin'), a.created_at, a.updated_at
		FROM announcements a
		LEFT JOIN users u ON a.created_by = u.id
		ORDER BY a.is_urgent DESC, a.created_at DESC
		LIMIT ?
	`), limit)
	if err != nil || rows == nil {
		return nil
	}
	defer rows.Close()

	var list []models.Announcement
	for rows.Next() {
		var a models.Announcement
		var isUrgentVal interface{}
		if err := rows.Scan(&a.ID, &a.Title, &a.Content, &isUrgentVal, &a.CreatedBy, &a.AuthorName, &a.CreatedAt, &a.UpdatedAt); err == nil {
			switch v := isUrgentVal.(type) {
			case bool:
				a.IsUrgent = v
			case int64:
				a.IsUrgent = (v == 1)
			}
			list = append(list, a)
		}
	}
	return list
}
