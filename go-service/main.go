package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/benkamin03/prism/internal/infisical"
	"github.com/benkamin03/prism/internal/minio"
	"github.com/labstack/echo/v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Environment holds all configuration loaded from environment variables
type Environment struct {
	// Infisical (required in production)
	InfisicalClientID     string
	InfisicalClientSecret string
	InfisicalSiteURL      string

	// MinIO (with defaults for development)
	MinioEndpoint        string
	MinioAccessKeyID     string
	MinioSecretAccessKey string
	MinioUseSSL          bool

	// Database (with defaults for development)
	DBHost     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPort     string
}

// Global environment configuration accessible throughout the package
var env *Environment

type Clients struct {
	DatabaseClient *gorm.DB
	MinioClient    minio.MinioClient
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// loadEnvironment loads and validates all environment variables
func loadEnvironment() *Environment {
	// Required fields (no defaults)
	infisicalClientID := os.Getenv("INFISICAL_CLIENT_ID")
	if infisicalClientID == "" {
		log.Fatal("❌ INFISICAL_CLIENT_ID environment variable is required")
	}

	infisicalClientSecret := os.Getenv("INFISICAL_CLIENT_SECRET")
	if infisicalClientSecret == "" {
		log.Fatal("❌ INFISICAL_CLIENT_SECRET environment variable is required")
	}

	return &Environment{
		// Infisical
		InfisicalClientID:     infisicalClientID,
		InfisicalClientSecret: infisicalClientSecret,
		InfisicalSiteURL:      getEnv("INFISICAL_SITE_URL", "http://infisical-backend:8080"),

		// MinIO
		MinioEndpoint:        getEnv("MINIO_ENDPOINT", "minio:9000"),
		MinioAccessKeyID:     getEnv("MINIO_ACCESS_KEY_ID", "minio-admin"),
		MinioSecretAccessKey: getEnv("MINIO_SECRET_ACCESS_KEY", "minio-admin-password"),
		MinioUseSSL:          false,

		// Database
		DBHost:     getEnv("DB_HOST", "database"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "test"),
		DBName:     getEnv("DB_NAME", "prism"),
		DBPort:     getEnv("DB_PORT", "5432"),
	}
}

func setupDatabaseClient() *sql.DB {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		env.DBHost, env.DBUser, env.DBPassword, env.DBName, env.DBPort)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Failed to get DB instance: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("❌ Database not responding: %v", err)
	}

	log.Println("✅ Connected to PostgreSQL successfully")
	return sqlDB
}

func setupInfisicalClient() *infisical.InfisicalClient {
	client, err := infisical.NewInfisicalClient(&infisical.InfisicalClientConfig{
		SiteUrl:               env.InfisicalSiteURL,
		InfisicalClientID:     env.InfisicalClientID,
		InfisicalClientSecret: env.InfisicalClientSecret,
	})

	if err != nil {
		panic(fmt.Sprintf("Authentication failed: %v", err))
	}

	log.Println("✅ Infisical client authenticated successfully")
	return client
}

func setupMinioClient() *minio.MinioClient {
	minioClient, err := minio.NewMinioClient(&minio.MinioClientConfig{
		Endpoint:        env.MinioEndpoint,
		AccessKeyID:     env.MinioAccessKeyID,
		SecretAccessKey: env.MinioSecretAccessKey,
		UseSSL:          env.MinioUseSSL,
	})

	if err != nil {
		log.Fatalf("❌ Failed to initialize Minio client: %v", err)
	}

	log.Println("✅ Minio client initialized successfully")
	return minioClient
}

func main() {
	// Load environment configuration first
	env = loadEnvironment()
	log.Println("✅ Environment configuration loaded")

	e := echo.New()
	dbClient := setupDatabaseClient()
	minioClient := setupMinioClient()
	infisicalClient := setupInfisicalClient()

	// Routes
	log.Printf("infisicalClient (from main): %+v", infisicalClient)
	SetupRoutes(&RoutesConfig{
		Echo:            e,
		DatabaseClient:  dbClient,
		InfisicalClient: infisicalClient,
		MinioClient:     *minioClient,
	})

	e.Logger.Fatal(e.Start(":1323"))
}
