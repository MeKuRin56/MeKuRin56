package database

import (
	"database/sql"
	"log"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/models"

	"golang.org/x/crypto/bcrypt"
)

// SeedData inserts initial rooms and demo data if not already present
func (db *DB) SeedData(cfg *config.Config) error {
	log.Println("[DATABASE] Checking and seeding initial data...")

	// 1. Seed Rooms if table is empty — always needed regardless of mode
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
			imageURL string
		}{
			{"101", "Standard", 4500.00, 1, models.RoomStatusOccupied, "ห้องพักมาตรฐาน ชั้น 1 เตียงเดี่ยว 5 ฟุต เฟอร์นิเจอร์ครบ", "https://images.unsplash.com/photo-1598928506311-c55ded91a20c?auto=format&fit=crop&w=800&q=80"},
			{"102", "Standard", 4500.00, 1, models.RoomStatusVacant, "ห้องพักมาตรฐาน ชั้น 1 วิวสวน เงียบสงบ", "https://images.unsplash.com/photo-1505693416388-ac5ce068fe85?auto=format&fit=crop&w=800&q=80"},
			{"103", "Deluxe", 5500.00, 1, models.RoomStatusVacant, "ห้องดีลักซ์ ชั้น 1 ระเบียงกว้าง ตู้เย็น ไมโครเวฟ", "https://images.unsplash.com/photo-1590490360182-c33d57733427?auto=format&fit=crop&w=800&q=80"},
			{"201", "Deluxe", 5500.00, 2, models.RoomStatusOccupied, "ห้องดีลักซ์ ชั้น 2 แอร์อินเวอร์เตอร์ เครื่องทำน้ำอุ่น", "https://images.unsplash.com/photo-1566665797739-1674de7a421a?auto=format&fit=crop&w=800&q=80"},
			{"202", "Studio", 6500.00, 2, models.RoomStatusVacant, "ห้องสตูดิโอ ชั้น 2 พร้อมมุมทำอาหารและเครื่องซักผ้า", "https://images.unsplash.com/photo-1522771739844-6a9f6d5f14af?auto=format&fit=crop&w=800&q=80"},
			{"301", "Suite", 8000.00, 3, models.RoomStatusVacant, "ห้องสูท ชั้น 3 พื้นที่ 45 ตร.ม. ห้องนั่งเล่นแยกส่วน", "https://images.unsplash.com/photo-1582719478250-c89cae4dc85b?auto=format&fit=crop&w=800&q=80"},
			{"302", "Standard", 4800.00, 3, models.RoomStatusMaintenance, "ห้องพักมาตรฐาน ชั้น 3 กำลังซ่อมระบบประปาและทาสีใหม่", "https://images.unsplash.com/photo-1540518614846-7eded433c457?auto=format&fit=crop&w=800&q=80"},
		}

		insertRoomQuery := db.Rebind(`
			INSERT INTO rooms (room_number, room_type, price, floor, status, description, image_url)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`)
		for _, r := range rooms {
			if _, err := db.Exec(insertRoomQuery, r.number, r.roomType, r.price, r.floor, r.status, r.desc, r.imageURL); err != nil {
				log.Printf("[DATABASE ERROR] Failed to seed room %s: %v", r.number, err)
			}
		}
	}

	// Backfill existing rooms that don't have images
	_, _ = db.Exec(db.Rebind("UPDATE rooms SET image_url = 'https://images.unsplash.com/photo-1598928506311-c55ded91a20c?auto=format&fit=crop&w=800&q=80' WHERE (image_url IS NULL OR image_url = '') AND room_type = 'Standard'"))
	_, _ = db.Exec(db.Rebind("UPDATE rooms SET image_url = 'https://images.unsplash.com/photo-1590490360182-c33d57733427?auto=format&fit=crop&w=800&q=80' WHERE (image_url IS NULL OR image_url = '') AND room_type = 'Deluxe'"))
	_, _ = db.Exec(db.Rebind("UPDATE rooms SET image_url = 'https://images.unsplash.com/photo-1522771739844-6a9f6d5f14af?auto=format&fit=crop&w=800&q=80' WHERE (image_url IS NULL OR image_url = '') AND room_type = 'Studio'"))
	_, _ = db.Exec(db.Rebind("UPDATE rooms SET image_url = 'https://images.unsplash.com/photo-1582719478250-c89cae4dc85b?auto=format&fit=crop&w=800&q=80' WHERE (image_url IS NULL OR image_url = '') AND room_type = 'Suite'"))

	// 2. Seed Admin User
	// Automatically initializes the secure administrator account configured in Config
	if cfg.AdminEmail != "" {
		var adminID int64
		checkAdminQuery := db.Rebind("SELECT id FROM users WHERE email = ?")
		err = db.QueryRow(checkAdminQuery, cfg.AdminEmail).Scan(&adminID)
		if err == sql.ErrNoRows {
			log.Printf("[DATABASE] Initializing admin account: %s", cfg.AdminEmail)
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			insertAdminQuery := db.Rebind(`
				INSERT INTO users (name, email, password, role, phone)
				VALUES (?, ?, ?, ?, ?)
			`)
			if _, err = db.Exec(insertAdminQuery, cfg.AdminName, cfg.AdminEmail, string(hashedPassword), models.RoleAdmin, "081-234-5678"); err != nil {
				log.Printf("[DATABASE ERROR] Failed to create admin: %v", err)
			} else {
				log.Printf("[DATABASE] Default admin account created successfully!")
			}
		}
	}

	// 3. Seed Demo Tenant (Development only)
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

			if room101ID > 0 {
				insertMaintQuery := db.Rebind(`
					INSERT INTO maintenance_requests (tenant_id, room_id, title, description, category, status, admin_notes)
					VALUES (?, ?, ?, ?, ?, ?, ?)
				`)
				_, _ = db.Exec(insertMaintQuery, tenantID, room101ID, "ก๊อกน้ำอ่างล้างหน้ารั่วซึม", "มีน้ำหยดตลอดเวลาบริเวณข้อต่อใต้อ่างล้างหน้าในห้องน้ำ", models.CategoryPlumbing, models.MaintStatusInProgress, "ช่างนัดเข้าตรวจสอบวันพรุ่งนี้ 10:00 น.")
				_, _ = db.Exec(insertMaintQuery, tenantID, room101ID, "แอร์ไม่ค่อยเย็น มีเสียงดัง", "เปิด 24 องศาแล้วรู้สึกไม่เย็น และมีเสียงลมดังผิดปกติ", models.CategoryAirCon, models.MaintStatusCompleted, "ล้างแผ่นกรองและเติมน้ำยาแอร์เรียบร้อยแล้ว")

				insertPayQuery := db.Rebind(`
					INSERT INTO payments (tenant_id, room_id, billing_month, amount, payment_date, slip_url, status, rejection_reason)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				`)
				_, _ = db.Exec(insertPayQuery, tenantID, room101ID, "2026-09", 4500.00, "2026-09-02", "", models.PaymentStatusApproved, "")
				_, _ = db.Exec(insertPayQuery, tenantID, room101ID, "2026-10", 4910.00, "2026-10-01", "", models.PaymentStatusPending, "")

				insertInvQuery := db.Rebind(`
					INSERT INTO invoices (tenant_id, room_id, billing_month, room_price, water_units, water_rate, water_total, electric_units, electric_rate, electric_total, other_fees, total_amount, due_date, status, notes)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				`)
				// Paid invoice for 2026-09 (ready for testing A4 printable receipt)
				_, _ = db.Exec(insertInvQuery, tenantID, room101ID, "2026-09", 4500.00, 4.0, 18.0, 72.0, 35.0, 8.0, 280.0, 0.0, 4852.00, "2026-10-05", "paid", "ชำระเงินเรียบร้อยแล้ว - พิมพ์ใบเสร็จได้")
				// Unpaid invoice for 2026-10
				_, _ = db.Exec(insertInvQuery, tenantID, room101ID, "2026-10", 4500.00, 5.0, 18.0, 90.0, 40.0, 8.0, 320.0, 0.0, 4910.00, "2026-11-05", "unpaid", "กรุณาชำระเงินก่อนวันที่ 5 ของเดือน")
				// Overdue invoice for 2026-08 (ready for testing overdue alert banner)
				_, _ = db.Exec(insertInvQuery, tenantID, room101ID, "2026-08", 4500.00, 6.0, 18.0, 108.0, 42.0, 8.0, 336.0, 50.0, 4994.00, "2026-09-05", "overdue", "บิลเกินกำหนดชำระ กรุณาติดต่อชำระโดยด่วน")
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
