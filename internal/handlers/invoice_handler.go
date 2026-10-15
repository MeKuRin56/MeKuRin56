package handlers

import (
	"database/sql"
	"fmt"
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

type InvoiceHandler struct {
	cfg *config.Config
}

func NewInvoiceHandler(cfg *config.Config) *InvoiceHandler {
	return &InvoiceHandler{cfg: cfg}
}

// ListInvoices shows all invoices (Admin sees all; Tenant sees own)
func (h *InvoiceHandler) ListInvoices(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	monthFilter := strings.TrimSpace(c.Query("month"))
	statusFilter := strings.TrimSpace(c.Query("status"))
	search := strings.TrimSpace(c.Query("search"))

	db := database.GetDB()

	// Base count query
	countSQL := `
		SELECT COUNT(*)
		FROM invoices i
		JOIN users u ON i.tenant_id = u.id
		LEFT JOIN rooms r ON i.room_id = r.id
		WHERE 1=1
	`
	var conditions []string
	var args []interface{}

	if user.Role == models.RoleTenant {
		conditions = append(conditions, "i.tenant_id = ?")
		args = append(args, user.ID)
	}

	if monthFilter != "" && monthFilter != "all" {
		conditions = append(conditions, "i.billing_month = ?")
		args = append(args, monthFilter)
	}

	if statusFilter != "" && statusFilter != "all" {
		conditions = append(conditions, "i.status = ?")
		args = append(args, statusFilter)
	}

	if search != "" {
		conditions = append(conditions, "(u.name LIKE ? OR r.room_number LIKE ?)")
		term := "%" + search + "%"
		args = append(args, term, term)
	}

	if len(conditions) > 0 {
		countSQL += " AND " + strings.Join(conditions, " AND ")
	}

	var totalItems int
	_ = db.QueryRow(db.Rebind(countSQL), args...).Scan(&totalItems)

	_, limit, offset, pageInfo := GetPagination(c, totalItems, 10)

	// Fetch invoices
	querySQL := `
		SELECT i.id, i.tenant_id, u.name, u.email, i.room_id, COALESCE(r.room_number, '-'),
		       i.billing_month, i.room_price, i.water_units, i.water_rate, i.water_total,
		       i.electric_units, i.electric_rate, i.electric_total, i.other_fees, i.total_amount,
		       i.due_date, i.status, COALESCE(i.notes, ''), i.created_at, i.updated_at
		FROM invoices i
		JOIN users u ON i.tenant_id = u.id
		LEFT JOIN rooms r ON i.room_id = r.id
		WHERE 1=1
	`
	if len(conditions) > 0 {
		querySQL += " AND " + strings.Join(conditions, " AND ")
	}
	querySQL += " ORDER BY i.created_at DESC LIMIT ? OFFSET ?"

	queryArgs := append(args, limit, offset)
	rows, err := db.Query(db.Rebind(querySQL), queryArgs...)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Title":   "Error",
			"Message": "ไม่สามารถโหลดข้อมูลใบแจ้งหนี้ได้",
			"User":    user,
		})
		return
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		var inv models.Invoice
		var rID sql.NullInt64
		err := rows.Scan(
			&inv.ID, &inv.TenantID, &inv.TenantName, &inv.TenantEmail, &rID, &inv.RoomNumber,
			&inv.BillingMonth, &inv.RoomPrice, &inv.WaterUnits, &inv.WaterRate, &inv.WaterTotal,
			&inv.ElectricUnits, &inv.ElectricRate, &inv.ElectricTotal, &inv.OtherFees, &inv.TotalAmount,
			&inv.DueDate, &inv.Status, &inv.Notes, &inv.CreatedAt, &inv.UpdatedAt,
		)
		if err == nil {
			if rID.Valid {
				idVal := rID.Int64
				inv.RoomID = &idVal
			}
			invoices = append(invoices, inv)
		}
	}

	// Fetch tenants with rooms for Admin create-invoice modal
	var tenantsWithRooms []models.User
	if user.Role == models.RoleAdmin {
		tRows, _ := db.Query(db.Rebind(`
			SELECT u.id, u.name, u.email, u.room_id, r.room_number, r.price
			FROM users u
			JOIN rooms r ON u.room_id = r.id
			WHERE u.role = 'tenant' AND u.room_id IS NOT NULL
			ORDER BY r.room_number ASC
		`))
		if tRows != nil {
			defer tRows.Close()
			for tRows.Next() {
				var tu models.User
				var price float64
				var rID int64
				if err := tRows.Scan(&tu.ID, &tu.Name, &tu.Email, &rID, &tu.RoomNo, &price); err == nil {
					tu.RoomID = &rID
					tenantsWithRooms = append(tenantsWithRooms, tu)
				}
			}
		}
	}

	templateName := "invoices_admin.html"
	if user.Role == models.RoleTenant {
		templateName = "invoices_tenant.html"
	}

	c.HTML(http.StatusOK, templateName, gin.H{
		"Title":            "ใบแจ้งค่าเช่ารายเดือน | Invoices",
		"User":             user,
		"Invoices":         invoices,
		"TenantsWithRooms": tenantsWithRooms,
		"PageInfo":         pageInfo,
		"MonthFilter":      monthFilter,
		"StatusFilter":     statusFilter,
		"Search":           search,
		"CSRFToken":        middleware.GetCSRFToken(c),
		"Success":          c.Query("success"),
		"Error":            c.Query("error"),
	})
}

// CreateInvoice generates a monthly invoice for a tenant (Admin only)
func (h *InvoiceHandler) CreateInvoice(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.Role != models.RoleAdmin {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	tenantIDStr := strings.TrimSpace(c.PostForm("tenant_id"))
	billingMonth := strings.TrimSpace(c.PostForm("billing_month"))
	roomPriceStr := strings.TrimSpace(c.PostForm("room_price"))
	waterUnitsStr := strings.TrimSpace(c.PostForm("water_units"))
	waterRateStr := strings.TrimSpace(c.PostForm("water_rate"))
	electricUnitsStr := strings.TrimSpace(c.PostForm("electric_units"))
	electricRateStr := strings.TrimSpace(c.PostForm("electric_rate"))
	otherFeesStr := strings.TrimSpace(c.PostForm("other_fees"))
	dueDate := strings.TrimSpace(c.PostForm("due_date"))
	notes := strings.TrimSpace(c.PostForm("notes"))

	tenantID, err := strconv.ParseInt(tenantIDStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/invoices?error=invalid_tenant")
		return
	}

	db := database.GetDB()

	// Verify tenant and fetch their room_id
	var roomID sql.NullInt64
	var roomPriceDB float64
	err = db.QueryRow(db.Rebind(`
		SELECT u.room_id, COALESCE(r.price, 0)
		FROM users u
		LEFT JOIN rooms r ON u.room_id = r.id
		WHERE u.id = ? AND u.role = 'tenant'
	`), tenantID).Scan(&roomID, &roomPriceDB)

	if err != nil || !roomID.Valid {
		c.Redirect(http.StatusFound, "/admin/invoices?error=tenant_has_no_room")
		return
	}

	if billingMonth == "" {
		billingMonth = time.Now().Format("2006-01")
	}

	// Check if invoice for this tenant & month already exists
	var existingID int64
	checkQ := db.Rebind("SELECT id FROM invoices WHERE tenant_id = ? AND billing_month = ?")
	if err := db.QueryRow(checkQ, tenantID, billingMonth).Scan(&existingID); err == nil {
		c.Redirect(http.StatusFound, "/admin/invoices?error=invoice_already_exists")
		return
	}

	roomPrice, _ := strconv.ParseFloat(roomPriceStr, 64)
	if roomPrice <= 0 {
		roomPrice = roomPriceDB
	}

	waterUnits, _ := strconv.ParseFloat(waterUnitsStr, 64)
	waterRate, _ := strconv.ParseFloat(waterRateStr, 64)
	if waterRate <= 0 {
		waterRate = 18.00
	}
	waterTotal := waterUnits * waterRate

	electricUnits, _ := strconv.ParseFloat(electricUnitsStr, 64)
	electricRate, _ := strconv.ParseFloat(electricRateStr, 64)
	if electricRate <= 0 {
		electricRate = 8.00
	}
	electricTotal := electricUnits * electricRate

	otherFees, _ := strconv.ParseFloat(otherFeesStr, 64)

	totalAmount := roomPrice + waterTotal + electricTotal + otherFees

	if dueDate == "" {
		dueDate = time.Now().AddDate(0, 0, 15).Format("2006-01-02")
	}

	insertSQL := db.Rebind(`
		INSERT INTO invoices (
			tenant_id, room_id, billing_month, room_price,
			water_units, water_rate, water_total,
			electric_units, electric_rate, electric_total,
			other_fees, total_amount, due_date, status, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'unpaid', ?)
	`)

	_, err = db.Exec(insertSQL,
		tenantID, roomID.Int64, billingMonth, roomPrice,
		waterUnits, waterRate, waterTotal,
		electricUnits, electricRate, electricTotal,
		otherFees, totalAmount, dueDate, notes,
	)

	if err != nil {
		c.Redirect(http.StatusFound, "/admin/invoices?error=create_failed")
		return
	}

	c.Redirect(http.StatusFound, "/admin/invoices?success=created")
}

// UpdateInvoiceStatus changes invoice status (Admin only)
func (h *InvoiceHandler) UpdateInvoiceStatus(c *gin.Context) {
	idStr := c.Param("id")
	invoiceID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/invoices?error=invalid_id")
		return
	}

	status := strings.TrimSpace(c.PostForm("status"))
	if status != models.InvoiceStatusUnpaid && status != models.InvoiceStatusPaid && status != models.InvoiceStatusOverdue {
		c.Redirect(http.StatusFound, "/admin/invoices?error=invalid_status")
		return
	}

	db := database.GetDB()
	_, err = db.Exec(db.Rebind("UPDATE invoices SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?"), status, invoiceID)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/invoices?error=update_failed")
		return
	}

	c.Redirect(http.StatusFound, "/admin/invoices?success=updated")
}

// DeleteInvoice deletes an invoice (Admin only)
func (h *InvoiceHandler) DeleteInvoice(c *gin.Context) {
	idStr := c.Param("id")
	invoiceID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/invoices?error=invalid_id")
		return
	}

	db := database.GetDB()
	_, err = db.Exec(db.Rebind("DELETE FROM invoices WHERE id = ?"), invoiceID)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/invoices?error=delete_failed")
		return
	}

	c.Redirect(http.StatusFound, "/admin/invoices?success=deleted")
}

// ExportInvoicesCSV exports all filtered invoices as CSV with UTF-8 BOM
func (h *InvoiceHandler) ExportInvoicesCSV(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.Role != models.RoleAdmin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	db := database.GetDB()
	rows, err := db.Query(`
		SELECT i.id, u.name, COALESCE(r.room_number, '-'), i.billing_month,
		       i.room_price, i.water_total, i.electric_total, i.other_fees, i.total_amount,
		       i.due_date, i.status, i.created_at
		FROM invoices i
		JOIN users u ON i.tenant_id = u.id
		LEFT JOIN rooms r ON i.room_id = r.id
		ORDER BY i.billing_month DESC, i.id DESC
	`)
	if err != nil {
		c.String(http.StatusInternalServerError, "Export failed")
		return
	}
	defer rows.Close()

	filename := fmt.Sprintf("invoices_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")

	// UTF-8 BOM for Microsoft Excel Thai compatibility
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	// CSV Header
	c.Writer.WriteString("Invoice ID,ผู้เช่า,หมายเลขห้อง,ประจำเดือน,ค่าห้อง,ค่าน้ำ,ค่าไฟ,ค่าอื่นๆ,ยอดรวมสุทธิ,วันครบกำหนด,สถานะ,วันที่ออกใบแจ้ง\n")

	for rows.Next() {
		var id int64
		var name, roomNo, month, dueDate, status string
		var roomPrice, waterTotal, elecTotal, otherFees, totalAmount float64
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &roomNo, &month, &roomPrice, &waterTotal, &elecTotal, &otherFees, &totalAmount, &dueDate, &status, &createdAt); err == nil {
			line := fmt.Sprintf("%d,\"%s\",\"%s\",\"%s\",%.2f,%.2f,%.2f,%.2f,%.2f,\"%s\",\"%s\",\"%s\"\n",
				id, name, roomNo, month, roomPrice, waterTotal, elecTotal, otherFees, totalAmount, dueDate, status, createdAt.Format("2006-01-02 15:04"),
			)
			c.Writer.WriteString(line)
		}
	}
}
