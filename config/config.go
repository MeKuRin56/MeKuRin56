package config

import (
	"log"
	"os"
	"strconv"

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
	AdminEmail    string
	AdminPassword string
	AdminName     string
	UploadDir     string
	Environment   string // "development" or "production"
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
		SessionSecret: getEnv("SESSION_SECRET", "dormitory-super-secure-secret-key-2026-min-32-chars!"),
		AdminEmail:    getEnv("ADMIN_EMAIL", "admin@dormitory.com"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin123456"),
		AdminName:     getEnv("ADMIN_NAME", "Dormitory Administrator"),
		UploadDir:     getEnv("UPLOAD_DIR", "./uploads"),
		Environment:   getEnv("ENV", "development"),
	}
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
