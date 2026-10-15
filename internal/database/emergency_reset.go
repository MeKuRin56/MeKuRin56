package database

import (
	"database/sql"
	"log"

	"dormitory-management-system/config"
	"dormitory-management-system/internal/models"

	"golang.org/x/crypto/bcrypt"
)

// ProcessEmergencyReset checks if EMERGENCY_ADMIN_PASSWORD is set in environment variables.
// If set and >= 8 characters, it securely resets the admin account password using bcrypt.
// This executes ONLY at the server-environment level on boot, exposing ZERO web endpoints.
func (db *DB) ProcessEmergencyReset(cfg *config.Config) error {
	newPass := cfg.EmergencyAdminPassword
	if newPass == "" {
		return nil
	}

	if len(newPass) < 8 {
		log.Println("[EMERGENCY RESET WARNING] EMERGENCY_ADMIN_PASSWORD must be at least 8 characters. Reset skipped.")
		return nil
	}

	log.Println("[EMERGENCY RESET] Processing emergency admin password reset requested via server environment...")

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[EMERGENCY RESET ERROR] Failed to hash new password: %v", err)
		return err
	}

	targetEmail := cfg.EmergencyAdminEmail

	if targetEmail != "" {
		// Target specific admin by email
		query := db.Rebind(`
			UPDATE users 
			SET password = ?, updated_at = CURRENT_TIMESTAMP 
			WHERE LOWER(email) = LOWER(?) AND role = ?
		`)
		res, err := db.Exec(query, string(hashedPassword), targetEmail, models.RoleAdmin)
		if err != nil {
			log.Printf("[EMERGENCY RESET ERROR] Failed to update admin: %v", err)
			return err
		}
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("==================================================================================")
			log.Printf(" [EMERGENCY RESET SUCCESS] Admin password for %s has been successfully reset!", targetEmail)
			log.Printf(" [SECURITY NOTICE] Log in and delete EMERGENCY_ADMIN_PASSWORD from Render now!")
			log.Printf("==================================================================================")
			return nil
		}
	}

	// Target the first existing admin
	var adminID int64
	var adminEmail string
	checkQuery := db.Rebind("SELECT id, email FROM users WHERE role = ? ORDER BY id ASC LIMIT 1")
	err = db.QueryRow(checkQuery, models.RoleAdmin).Scan(&adminID, &adminEmail)
	if err == sql.ErrNoRows {
		if targetEmail != "" {
			insertQuery := db.Rebind(`
				INSERT INTO users (name, email, password, role, phone)
				VALUES (?, ?, ?, ?, ?)
			`)
			_, err = db.Exec(insertQuery, "Administrator", targetEmail, string(hashedPassword), models.RoleAdmin, "081-000-0000")
			if err == nil {
				log.Printf("[EMERGENCY RESET SUCCESS] Created new admin account %s with emergency password!", targetEmail)
			}
		} else {
			log.Println("[EMERGENCY RESET WARNING] No admin account exists to reset. Complete /setup first or set EMERGENCY_ADMIN_EMAIL.")
		}
		return nil
	} else if err != nil {
		return err
	}

	// Update existing admin
	updateQuery := db.Rebind("UPDATE users SET password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	_, err = db.Exec(updateQuery, string(hashedPassword), adminID)
	if err != nil {
		log.Printf("[EMERGENCY RESET ERROR] Failed to update admin: %v", err)
		return err
	}

	log.Printf("==================================================================================")
	log.Printf(" [EMERGENCY RESET SUCCESS] Admin password for %s has been successfully reset!", adminEmail)
	log.Printf(" [SECURITY NOTICE] Log in and delete EMERGENCY_ADMIN_PASSWORD from Render now!")
	log.Printf("==================================================================================")
	return nil
}
