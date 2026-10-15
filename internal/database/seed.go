package database

import (
	"database/sql"
	"log"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/models"

	"golang.org/x/crypto/bcrypt"
)

// SeedData inserts initial administrative user, sample rooms, and demo data if not already present
func (db *DB) SeedData(cfg *config.Config) error {
	log.Println("[DATABASE] Checking and seeding initial data...")

	// 1. Seed Rooms if table is empty
	var roomCount int
	err := db.QueryRow("SELECT COUNT(*) FROM rooms").Scan(&roomCount)
	if err != nil {
		return err
	}

	if roomCount == 0 {
		log.Println("[DATABASE] Seeding default rooms...")
		rooms := []struct {
			number   string
			roomType string
			price    float64
			floor    int
			status   string
			desc     string
		}{
			{"101", "Standard", 4500.00, 1, models.RoomStatusOccupied, "ห้องพักมาตรฐาน ชั้น 1 เตียงเดี่ยว 5 ฟุต เฟอร์นิเจอร์ครบ"},
			{"102", "Standard", 4500.00, 1, models.RoomStatusVacant, "ห้องพักมาตรฐาน ชั้น 1 วิวสวน เงียบสงบ"},
			{"103", "Deluxe", 5500.00, 1, models.RoomStatusVacant, "ห้องดีลักซ์ ชั้น 1 ระเบียงกว้าง ตู้เย็น ไมโครเวฟ"},
			{"201", "Deluxe", 5500.00, 2, models.RoomStatusOccupied, "ห้องดีลักซ์ ชั้น 2 แอร์อินเวอร์เตอร์ เครื่องทำน้ำอุ่น"},
			{"202", "Studio", 6500.00, 2, models.RoomStatusVacant, "ห้องสตูดิโอ ชั้น 2 พร้อมมุมทำอาหารและเครื่องซักผ้า"},
			{"301", "Suite", 8000.00, 3, models.RoomStatusVacant, "ห้องสูท ชั้น 3 พื้นที่ 45 ตร.ม. ห้องนั่งเล่นแยกส่วน"},
			{"302", "Standard", 4800.00, 3, models.RoomStatusMaintenance, "ห้องพักมาตรฐาน ชั้น 3 กำลังซ่อมระบบประปาและทาสีใหม่"},
		}

		insertRoomQuery := db.Rebind(`
			INSERT INTO rooms (room_number, room_type, price, floor, status, description)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
		for _, r := range rooms {
			_, err := db.Exec(insertRoomQuery, r.number, r.roomType, r.price, r.floor, r.status, r.desc)
			if err != nil {
				log.Printf("[DATABASE ERROR] Failed to seed room %s: %v", r.number, err)
			}
		}
	}

	// 2. Seed Admin User
	var adminID int64
	checkAdminQuery := db.Rebind("SELECT id FROM users WHERE email = ?")
	err = db.QueryRow(checkAdminQuery, cfg.AdminEmail).Scan(&adminID)
	if err == sql.ErrNoRows {
		log.Printf("[DATABASE] Creating default admin account: %s", cfg.AdminEmail)
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		insertAdminQuery := db.Rebind(`
			INSERT INTO users (name, email, password, role, phone)
			VALUES (?, ?, ?, ?, ?)
		`)
		_, err = db.Exec(insertAdminQuery, cfg.AdminName, cfg.AdminEmail, string(hashedPassword), models.RoleAdmin, "081-234-5678")
		if err != nil {
			log.Printf("[DATABASE ERROR] Failed to create admin: %v", err)
		} else {
			log.Printf("[DATABASE] Default admin account created successfully!")
		}
	}

	// 3. Seed Demo Tenant
	demoTenantEmail := "tenant@dormitory.com"
	var tenantID int64
	checkTenantQuery := db.Rebind("SELECT id FROM users WHERE email = ?")
	err = db.QueryRow(checkTenantQuery, demoTenantEmail).Scan(&tenantID)
	if err == sql.ErrNoRows {
		log.Printf("[DATABASE] Creating demo tenant account: %s", demoTenantEmail)
		hashedPass, _ := bcrypt.GenerateFromPassword([]byte("tenant123456"), bcrypt.DefaultCost)

		// Find Room 101 ID
		var room101ID int64
		_ = db.QueryRow(db.Rebind("SELECT id FROM rooms WHERE room_number = ?"), "101").Scan(&room101ID)

		insertTenantQuery := db.Rebind(`
			INSERT INTO users (name, email, password, role, phone, room_id)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
		var res sql.Result
		if room101ID > 0 {
			res, err = db.Exec(insertTenantQuery, "สมชาย ใจดี (Somchai Jaidee)", demoTenantEmail, string(hashedPass), models.RoleTenant, "089-987-6543", room101ID)
		} else {
			res, err = db.Exec(insertTenantQuery, "สมชาย ใจดี (Somchai Jaidee)", demoTenantEmail, string(hashedPass), models.RoleTenant, "089-987-6543", nil)
		}

		if err == nil {
			tenantID, _ = res.LastInsertId()
			log.Printf("[DATABASE] Demo tenant account created successfully!")

			// Seed sample maintenance requests for demo tenant
			if room101ID > 0 {
				insertMaintQuery := db.Rebind(`
					INSERT INTO maintenance_requests (tenant_id, room_id, title, description, category, status, admin_notes)
					VALUES (?, ?, ?, ?, ?, ?, ?)
				`)
				_, _ = db.Exec(insertMaintQuery, tenantID, room101ID, "ก๊อกน้ำอ่างล้างหน้ารั่วซึม", "มีน้ำหยดตลอดเวลาบริเวณข้อต่อใต้อ่างล้างหน้าในห้องน้ำ", models.CategoryPlumbing, models.MaintStatusInProgress, "ช่างนัดเข้าตรวจสอบวันพรุ่งนี้ 10:00 น.")
				_, _ = db.Exec(insertMaintQuery, tenantID, room101ID, "แอร์ไม่ค่อยเย็น มีเสียงดัง", "เปิด 24 องศาแล้วรู้สึกไม่เย็น และมีเสียงลมดังผิดปกติ", models.CategoryAirCon, models.MaintStatusCompleted, "ล้างแผ่นกรองและเติมน้ำยาแอร์เรียบร้อยแล้ว")

				// Seed sample rent payments
				insertPayQuery := db.Rebind(`
					INSERT INTO payments (tenant_id, room_id, billing_month, amount, payment_date, slip_url, status, rejection_reason)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				`)
				_, _ = db.Exec(insertPayQuery, tenantID, room101ID, "2026-09", 4500.00, "2026-09-02", "", models.PaymentStatusApproved, "")
				_, _ = db.Exec(insertPayQuery, tenantID, room101ID, "2026-10", 4910.00, "2026-10-01", "", models.PaymentStatusPending, "")

				// Seed sample invoice
				insertInvQuery := db.Rebind(`
					INSERT INTO invoices (tenant_id, room_id, billing_month, room_price, water_units, water_rate, water_total, electric_units, electric_rate, electric_total, other_fees, total_amount, due_date, status, notes)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				`)
				_, _ = db.Exec(insertInvQuery, tenantID, room101ID, "2026-10", 4500.00, 5.0, 18.0, 90.0, 40.0, 8.0, 320.0, 0.0, 4910.00, "2026-11-05", "unpaid", "กรุณาชำระเงินก่อนวันที่ 5 ของเดือน")
			}
		}
	}

	// 4. Seed sample announcements if none exist
	var announcementCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM announcements").Scan(&announcementCount)
	if announcementCount == 0 {
		var isUrgentTrue, isUrgentFalse interface{} = true, false
		if db.Driver == "sqlite" {
			isUrgentTrue, isUrgentFalse = 1, 0
		}
		insAnnounce := db.Rebind(`
			INSERT INTO announcements (title, content, is_urgent, created_by)
			VALUES (?, ?, ?, ?)
		`)
		_, _ = db.Exec(insAnnounce, "มาตรการความปลอดภัย: ปิดประตูคีย์การ์ดทุกครั้ง", "เพื่อความปลอดภัยของผู้พักอาศัยทุกคน กรุณาอย่าเปิดประตูค้างไว้ และไม่อนุญาตให้บุคคลภายนอกเข้าโดยไม่มีการแลกบัตร", isUrgentTrue, 1)
		_, _ = db.Exec(insAnnounce, "แจ้งกำหนดการล้างถังพักน้ำประจำเดือน", "จะมีการล้างถังพักน้ำในวันอาทิตย์ที่ 25 นี้ เวลา 09:00 - 12:00 น. น้ำประปาจะหยุดไหลชั่วคราว กรุณาสำรองน้ำล่วงหน้า", isUrgentFalse, 1)
	}

	return nil
}
