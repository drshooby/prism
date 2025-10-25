package main

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	e := echo.New()

	// Hardcoded connection info (fine for hackathon use)
	dsn := "host=database user=postgres password=test dbname=prism port=5432 sslmode=disable"

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

	// Routes
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	e.GET("/dbcheck", func(c echo.Context) error {
		if err := sqlDB.Ping(); err != nil {
			return c.String(http.StatusInternalServerError, "❌ DB not reachable")
		}
		return c.String(http.StatusOK, "✅ DB connection OK")
	})

	e.Logger.Fatal(e.Start(":1323"))
}
