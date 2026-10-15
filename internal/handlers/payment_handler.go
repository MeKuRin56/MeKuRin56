package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/database"
	"dormitory-management-system/internal/middleware"
	"dormitory-management-system/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	cfg *config.Config
}

func NewPaymentHandler(cfg *config.Config) *PaymentHandler {
	return &PaymentHandler{cfg: cfg}
}

// ListPayments shows payments history
// Tenant sees only their own; Admin sees all payments with filters
func (h *PaymentHandler) ListPayments(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	statusFilter := strings.TrimSpace(c.Query("status"))
	monthFilter := strings.TrimSpace(c.Query("month"))
	searchQuery := strings.TrimSpace(c.Query("search"))

	db := database.GetDB()

	baseSQL := `
		SELECT p.id, p.tenant_id, u.name, u.email, p.room_id, COALESCE(r.room_number, '-'),
		       p.billing_month, p.amount, p.payment_date, COALESCE(p.slip_url, ''),
		       p.status, COALESCE(p.rejection_reason, ''), p.created_at, p.updated_at
		FROM payments p
		JOIN users u ON p.tenant_id = u.id
		LEFT JOIN rooms r ON p.room_id = r.id
		WHERE 1=1
	`
	var conditions []string
	var args []interface{}

	// IDOR protection: tenant can only see own payments
	if user.Role == models.RoleTenant {
		conditions = append(conditions, "p.tenant_id = ?")
		args = append(args, user.ID)
	}

	if statusFilter != "" && statusFilter != "all" {
		conditions = append(conditions, "p.status = ?")
		args = append(args, statusFilter)
	}

	if monthFilter != "" && monthFilter != "all" {
		conditions = append(conditions, "p.billing_month = ?")
		args = append(args, monthFilter)
	}

	if searchQuery != "" {
		conditions = append(conditions, "(u.name LIKE ? OR r.room_number LIKE ?)")
		term := "%" + searchQuery + "%"
		args = append(args, term, term)
	}

	if len(conditions) > 0 {
		baseSQL += " AND " + strings.Join(conditions, " AND ")
	}

	countSQL := `
		SELECT COUNT(*)
		FROM payments p
		JOIN users u ON p.tenant_id = u.id
		LEFT JOIN rooms r ON p.room_id = r.id
		WHERE 1=1
	`
	if len(conditions) > 0 {
		countSQL += " AND " + strings.Join(conditions, " AND ")
	}

	var totalItems int
	_ = db.QueryRow(db.Rebind(countSQL), args...).Scan(&totalItems)

	_, limit, offset, pageInfo := GetPagination(c, totalItems, 10)

	baseSQL += " ORDER BY p.created_at DESC LIMIT ? OFFSET ?"
	queryArgs := append(args, limit, offset)

	reboundSQL := db.Rebind(baseSQL)
	rows, err := db.Query(reboundSQL, queryArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load payments"})
		return
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var p models.Payment
		var roomID sql.NullInt64

		err := rows.Scan(
			&p.ID, &p.TenantID, &p.TenantName, &p.TenantEmail, &roomID, &p.RoomNumber,
			&p.BillingMonth, &p.Amount, &p.PaymentDate, &p.SlipURL,
			&p.Status, &p.RejectionReason, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			continue
		}

		if roomID.Valid {
			idVal := roomID.Int64
			p.RoomID = &idVal
		}

		payments = append(payments, p)
	}

	if c.Query("format") == "json" {
		c.JSON(http.StatusOK, gin.H{"payments": payments, "total": len(payments)})
		return
	}

	c.HTML(http.StatusOK, "payments.html", gin.H{
		"Title":        "ระบบชำระค่าเช่า | Payments",
		"Payments":     payments,
		"PageInfo":     pageInfo,
		"StatusFilter": statusFilter,
		"MonthFilter":  monthFilter,
		"SearchQuery":  searchQuery,
		"User":         user,
		"CSRFToken":    middleware.GetCSRFToken(c),
		"Success":      c.Query("success"),
		"Error":        c.Query("error"),
	})
}

// CreatePayment handles submitting a payment slip
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	billingMonth := strings.TrimSpace(c.PostForm("billing_month"))
	amountStr := strings.TrimSpace(c.PostForm("amount"))
	paymentDate := strings.TrimSpace(c.PostForm("payment_date"))

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		c.Redirect(http.StatusFound, "/payments?error=invalid_amount")
		return
	}

	if billingMonth == "" {
		billingMonth = time.Now().Format("2006-01")
	}
	if paymentDate == "" {
		paymentDate = time.Now().Format("2006-01-02")
	}

	var slipURL string

	// Handle optional slip upload with security validation
	file, fileHeader, err := c.Request.FormFile("slip_file")
	if err == nil && file != nil {
		defer file.Close()

		// Max size check (5 MB)
		if fileHeader.Size > 5*1024*1024 {
			c.Redirect(http.StatusFound, "/payments?error=file_too_large")
			return
		}

		// Extension validation
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".pdf": true}
		if !allowedExts[ext] {
			c.Redirect(http.StatusFound, "/payments?error=invalid_file_type")
			return
		}

		// Read magic bytes to verify REAL file type (prevent extension spoofing)
		magic := make([]byte, 8)
		n, _ := file.Read(magic)
		magic = magic[:n]

		validMime := false
		switch ext {
		case ".jpg", ".jpeg":
			// JPEG magic: FF D8 FF
			validMime = len(magic) >= 3 && magic[0] == 0xFF && magic[1] == 0xD8 && magic[2] == 0xFF
		case ".png":
			// PNG magic: 89 50 4E 47 0D 0A 1A 0A
			validMime = len(magic) >= 8 &&
				magic[0] == 0x89 && magic[1] == 0x50 && magic[2] == 0x4E && magic[3] == 0x47 &&
				magic[4] == 0x0D && magic[5] == 0x0A && magic[6] == 0x1A && magic[7] == 0x0A
		case ".pdf":
			// PDF magic: %PDF
			validMime = len(magic) >= 4 && magic[0] == 0x25 && magic[1] == 0x50 && magic[2] == 0x44 && magic[3] == 0x46
		}

		if !validMime {
			c.Redirect(http.StatusFound, "/payments?error=invalid_file_content")
			return
		}

		// Seek back to start after reading magic bytes
		if seeker, ok := file.(interface{ Seek(int64, int) (int64, error) }); ok {
			_, _ = seeker.Seek(0, 0)
		}

		// Ensure upload dir exists
		if err := os.MkdirAll(h.cfg.UploadDir, 0755); err != nil {
			c.Redirect(http.StatusFound, "/payments?error=upload_failed")
			return
		}

		// Generate cryptographically unique filename (no original filename in path)
		uniqueFilename := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102"), uuid.New().String(), ext)
		targetPath := filepath.Join(h.cfg.UploadDir, uniqueFilename)

		if err := c.SaveUploadedFile(fileHeader, targetPath); err != nil {
			c.Redirect(http.StatusFound, "/payments?error=save_failed")
			return
		}

		slipURL = "/uploads/" + uniqueFilename
	}

	db := database.GetDB()
	query := db.Rebind(`
		INSERT INTO payments (tenant_id, room_id, billing_month, amount, payment_date, slip_url, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)

	_, err = db.Exec(query, user.ID, user.RoomID, billingMonth, amount, paymentDate, slipURL, models.PaymentStatusPending)
	if err != nil {
		c.Redirect(http.StatusFound, "/payments?error=db_error")
		return
	}

	c.Redirect(http.StatusFound, "/payments?success=submitted")
}

// UpdatePaymentStatus handles approval / rejection (Admin only)
func (h *PaymentHandler) UpdatePaymentStatus(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.Role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Admin only"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment ID"})
		return
	}

	status := strings.TrimSpace(c.PostForm("status"))
	rejectionReason := strings.TrimSpace(c.PostForm("rejection_reason"))

	if status != models.PaymentStatusPending && status != models.PaymentStatusApproved && status != models.PaymentStatusRejected {
		c.Redirect(http.StatusFound, "/payments?error=invalid_status")
		return
	}

	db := database.GetDB()
	query := db.Rebind(`
		UPDATE payments
		SET status = ?, rejection_reason = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`)

	_, err = db.Exec(query, status, rejectionReason, id)
	if err != nil {
		c.Redirect(http.StatusFound, "/payments?error=update_failed")
		return
	}

	c.Redirect(http.StatusFound, "/payments?success=updated")
}

// ExportPaymentsCSV exports all payments as CSV with UTF-8 BOM for Excel
func (h *PaymentHandler) ExportPaymentsCSV(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.Role != models.RoleAdmin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	db := database.GetDB()
	rows, err := db.Query(`
		SELECT p.id, u.name, COALESCE(r.room_number, '-'), p.billing_month,
		       p.amount, p.payment_date, p.status, COALESCE(p.rejection_reason, ''), p.created_at
		FROM payments p
		JOIN users u ON p.tenant_id = u.id
		LEFT JOIN rooms r ON p.room_id = r.id
		ORDER BY p.created_at DESC
	`)
	if err != nil {
		c.String(http.StatusInternalServerError, "Export failed")
		return
	}
	defer rows.Close()

	filename := fmt.Sprintf("payments_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")

	// UTF-8 BOM
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	c.Writer.WriteString("Payment ID,ผู้เช่า,หมายเลขห้อง,ประจำเดือน,จำนวนเงิน (บาท),วันที่ชำระ,สถานะ,เหตุผลปฏิเสธ(ถ้ามี),บันทึกเมื่อ\n")

	for rows.Next() {
		var id int64
		var name, roomNo, month, payDate, status, reason string
		var amount float64
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &roomNo, &month, &amount, &payDate, &status, &reason, &createdAt); err == nil {
			line := fmt.Sprintf("%d,\"%s\",\"%s\",\"%s\",%.2f,\"%s\",\"%s\",\"%s\",\"%s\"\n",
				id, name, roomNo, month, amount, payDate, status, reason, createdAt.Format("2006-01-02 15:04"),
			)
			c.Writer.WriteString(line)
		}
	}
}

