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

type DashboardStats struct {
	TotalRooms        int     `json:"total_rooms"`
	VacantRooms       int     `json:"vacant_rooms"`
	OccupiedRooms     int     `json:"occupied_rooms"`
	MaintenanceRooms  int     `json:"maintenance_rooms"`
	TotalTenants      int     `json:"total_tenants"`
	PendingRequests   int     `json:"pending_requests"`
	PendingPayments   int     `json:"pending_payments"`
	MonthlyRevenue    float64 `json:"monthly_revenue"`
}
