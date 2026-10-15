package models

import (
	"time"
)

// User role constants
const (
	RoleAdmin  = "admin"
	RoleTenant = "tenant"
)

// Room status constants
const (
	RoomStatusVacant      = "vacant"
	RoomStatusOccupied    = "occupied"
	RoomStatusMaintenance = "maintenance"
)

// Maintenance request status constants
const (
	MaintStatusPending    = "pending"
	MaintStatusInProgress = "in_progress"
	MaintStatusCompleted  = "completed"
)

// Maintenance category constants
const (
	CategoryPlumbing    = "plumbing"
	CategoryElectrical  = "electrical"
	CategoryAirCon      = "air_con"
	CategoryFurniture   = "furniture"
	CategoryOther       = "other"
)

// Payment status constants
const (
	PaymentStatusPending  = "pending"
	PaymentStatusApproved = "approved"
	PaymentStatusRejected = "rejected"
)

// Booking status constants
const (
	BookingStatusPending  = "pending"
	BookingStatusApproved = "approved"
	BookingStatusRejected = "rejected"
	BookingStatusCancelled = "cancelled"
)

// Invoice status constants
const (
	InvoiceStatusUnpaid  = "unpaid"
	InvoiceStatusPending = "pending"
	InvoiceStatusPaid    = "paid"
	InvoiceStatusOverdue = "overdue"
)

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Role      string    `json:"role"` // admin, tenant
	Phone     string    `json:"phone"`
	RoomID    *int64    `json:"room_id"`
	RoomNo    string    `json:"room_number,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Room struct {
	ID          int64     `json:"id"`
	RoomNumber  string    `json:"room_number"`
	RoomType    string    `json:"room_type"` // Standard, Deluxe, Studio, Suite
	Price       float64   `json:"price"`
	Floor       int       `json:"floor"`
	Status      string    `json:"status"` // vacant, occupied, maintenance
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	TenantID    *int64    `json:"tenant_id,omitempty"`
	TenantName  string    `json:"tenant_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MaintenanceRequest struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenant_id"`
	TenantName  string    `json:"tenant_name"`
	TenantEmail string    `json:"tenant_email"`
	RoomID      *int64    `json:"room_id"`
	RoomNumber  string    `json:"room_number"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Status      string    `json:"status"` // pending, in_progress, completed
	AdminNotes  string    `json:"admin_notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Payment struct {
	ID              int64     `json:"id"`
	TenantID        int64     `json:"tenant_id"`
	TenantName      string    `json:"tenant_name"`
	TenantEmail     string    `json:"tenant_email"`
	RoomID          *int64    `json:"room_id"`
	RoomNumber      string    `json:"room_number"`
	BillingMonth    string    `json:"billing_month"` // e.g., "2026-10"
	Amount          float64   `json:"amount"`
	PaymentDate     string    `json:"payment_date"` // YYYY-MM-DD
	SlipURL         string    `json:"slip_url"`
	Status          string    `json:"status"` // pending, approved, rejected
	RejectionReason string    `json:"rejection_reason"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type RoomBooking struct {
	ID         int64     `json:"id"`
	RoomID     int64     `json:"room_id"`
	RoomNumber string    `json:"room_number"`
	RoomType   string    `json:"room_type"`
	Price      float64   `json:"price"`
	TenantID   int64     `json:"tenant_id"`
	TenantName string    `json:"tenant_name"`
	TenantEmail string   `json:"tenant_email"`
	TenantPhone string   `json:"tenant_phone"`
	Status     string    `json:"status"` // pending, approved, rejected, cancelled
	Message    string    `json:"message"`
	AdminNote  string    `json:"admin_note"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Invoice struct {
	ID            int64     `json:"id"`
	TenantID      int64     `json:"tenant_id"`
	TenantName    string    `json:"tenant_name"`
	TenantEmail   string    `json:"tenant_email"`
	TenantPhone   string    `json:"tenant_phone"`
	RoomID        *int64    `json:"room_id"`
	RoomNumber    string    `json:"room_number"`
	BillingMonth  string    `json:"billing_month"` // e.g. "2026-10"
	RoomPrice     float64   `json:"room_price"`
	WaterUnits    float64   `json:"water_units"`
	WaterRate     float64   `json:"water_rate"`
	WaterTotal    float64   `json:"water_total"`
	ElectricUnits float64   `json:"electric_units"`
	ElectricRate  float64   `json:"electric_rate"`
	ElectricTotal float64   `json:"electric_total"`
	OtherFees     float64   `json:"other_fees"`
	TotalAmount   float64   `json:"total_amount"`
	DueDate       string    `json:"due_date"` // YYYY-MM-DD
	Status        string    `json:"status"`   // unpaid, pending, paid, overdue
	Notes         string    `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Announcement struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	IsUrgent   bool      `json:"is_urgent"`
	CreatedBy  *int64    `json:"created_by"`
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type PageInfo struct {
	CurrentPage int   `json:"current_page"`
	TotalPages  int   `json:"total_pages"`
	TotalItems  int   `json:"total_items"`
	Limit       int   `json:"limit"`
	HasPrev     bool  `json:"has_prev"`
	HasNext     bool  `json:"has_next"`
	PrevPage    int   `json:"prev_page"`
	NextPage    int   `json:"next_page"`
	Pages       []int `json:"pages"`
}

type DashboardStats struct {
	TotalRooms      int     `json:"total_rooms"`
	VacantRooms     int     `json:"vacant_rooms"`
	OccupiedRooms   int     `json:"occupied_rooms"`
	MaintenanceRooms int    `json:"maintenance_rooms"`
	TotalTenants    int     `json:"total_tenants"`
	PendingRequests int     `json:"pending_requests"`
	PendingPayments int     `json:"pending_payments"`
	PendingBookings int     `json:"pending_bookings"`
	UnpaidInvoices  int     `json:"unpaid_invoices"`
	MonthlyRevenue  float64 `json:"monthly_revenue"`
}
