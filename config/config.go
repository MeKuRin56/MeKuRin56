package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	DBDriver      string // "postgres" or "sqlite"
	DBURL         string // Full connection string if provided
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	DBSSLMode     string
	SessionSecret string
	AdminEmail             string
	AdminPassword          string
	AdminName              string
	UploadDir              string
	Environment            string // "development" or "production"
	EmergencyAdminPassword string // Server-level emergency password reset via environment variable
	EmergencyAdminEmail    string // Optional target email for emergency reset
}

func LoadConfig() *Config {
	// Load .env file if it exists, ignore error if missing (e.g. in container with env vars set)
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] No .env file found or error loading, using system environment variables")
	}

	port := getEnv("PORT", "8080")
	dbDriver := getEnv("DB_DRIVER", "sqlite")
	dbURL := getEnv("DATABASE_URL", "")

	// If DATABASE_URL starts with postgres://, auto set dbDriver = postgres
	if dbURL != "" && (len(dbURL) >= 11 && dbURL[:11] == "postgres://") {
		dbDriver = "postgres"
	}

	return &Config{
		Port:          port,
		DBDriver:      dbDriver,
		DBURL:         dbURL,
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        getEnv("DB_PORT", "5432"),
		DBUser:        getEnv("DB_USER", "postgres"),
		DBPassword:    getEnv("DB_PASSWORD", "postgres"),
		DBName:        getEnv("DB_NAME", "dormitory_db"),
		DBSSLMode:     getEnv("DB_SSLMODE", "disable"),
		SessionSecret: getOrGenerateSessionSecret(),
		// ADMIN_EMAIL and ADMIN_PASSWORD default to empty string intentionally.
		// When empty in production, the Setup Wizard (/setup) handles first-time admin creation.
		// In development, override via .env: ADMIN_EMAIL=admin@dormitory.com ADMIN_PASSWORD=admin123456
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		AdminName:              getEnv("ADMIN_NAME", "Dormitory Administrator"),
		UploadDir:              getEnv("UPLOAD_DIR", "./uploads"),
		Environment:            getEnv("ENV", "development"),
		EmergencyAdminPassword: os.Getenv("EMERGENCY_ADMIN_PASSWORD"),
		EmergencyAdminEmail:    os.Getenv("EMERGENCY_ADMIN_EMAIL"),
	}
}

func getOrGenerateSessionSecret() string {
	secret := os.Getenv("SESSION_SECRET")
	// If a custom non-default secret is provided via environment, use it
	if secret != "" && secret != "dormitory-super-secure-secret-key-2026-min-32-chars!" {
		return secret
	}

	// Try reading from persistent local key file
	keyFile := ".session_key"
	if data, err := os.ReadFile(keyFile); err == nil {
		trimmed := strings.TrimSpace(string(data))
		if len(trimmed) >= 32 {
			return trimmed
		}
	}

	// Generate 32 bytes (64 hex characters) of cryptographic random bytes
	b := make([]byte, 32)
	if _, err := rand.Read(b); err == nil {
		generated := hex.EncodeToString(b)
		_ = os.WriteFile(keyFile, []byte(generated), 0600)
		log.Println("[SECURITY] Generated persistent dynamic SESSION_SECRET in .session_key")
		return generated
	}

	return "dormitory-super-secure-secret-key-2026-min-32-chars!"
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultVal
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

func (c *Config) GetPortInt() int {
	p, err := strconv.Atoi(c.Port)
	if err != nil {
		return 8080
	}
	return p
}
