package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	infisical "github.com/infisical/go-sdk"
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

	client := infisical.NewInfisicalClient(context.Background(), infisical.Config{
		SiteUrl: "http://infisical-backend:8080",
	})

	// For machine identity (what go sdk uses)
	// 1. Org -> Access Control -> Identities -> Create Identity w/ Member Role
	// 2. Secrets Manager -> Access Management -> Machine Identities -> Add Identity -> Select w/ Developer Role
	// 3. Org -> Access Control -> Identities -> Click Identity -> Universal Auth
	// -> Copy Client ID -> Create Client Secret -> Copy Client Secret

	_, err = client.Auth().UniversalAuthLogin("0149ab3c-ddc4-4487-aef4-b052ddf809f6", "8920d6201a6ca6d6ad21bba940f825f1f6d2aafb8ebd5af8b97c05b26dde1098")
	if err != nil {
		panic(fmt.Sprintf("Authentication failed: %v", err))
	}

	e.GET("/secrets", func(c echo.Context) error {
		testSecret, err := client.Secrets().Retrieve(infisical.RetrieveSecretOptions{
			SecretKey:   "testSecret",
			Environment: "dev",
			ProjectID:   "dec7cfaf-8b50-48b4-8577-12035f9dd954", // project settings -> copy project id
			SecretPath:  "/",
		})
		if err != nil {
			return c.String(200, fmt.Sprintf("Error: %v", err))
		}
		return c.String(200, fmt.Sprintf("Your secret: %s", testSecret.SecretValue))
	})

	e.Logger.Fatal(e.Start(":1323"))
}
