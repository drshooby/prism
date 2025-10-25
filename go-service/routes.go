package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/benkamin03/prism/internal/minio"
	infisical "github.com/infisical/go-sdk"
	"github.com/labstack/echo/v4"
)

type RoutesConfig struct {
	Echo           *echo.Echo
	DatabaseClient *sql.DB
	InfiClient     infisical.InfisicalClientInterface
	MinioClient    minio.MinioClient
}

func SetupRoutes(routesConfig *RoutesConfig) {
	e := routesConfig.Echo
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	e.GET("/dbcheck", func(c echo.Context) error {
		if err := routesConfig.DatabaseClient.Ping(); err != nil {
			return c.String(http.StatusInternalServerError, "❌ DB not reachable")
		}
		return c.String(http.StatusOK, "✅ DB connection OK")
	})

	e.GET("/secrets", func(c echo.Context) error {
		testSecret, err := routesConfig.InfiClient.Secrets().Retrieve(infisical.RetrieveSecretOptions{
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

	minio.SetupRoutes(&minio.MinioRoutesConfig{
		MinioClient: routesConfig.MinioClient,
		Echo:        e,
	})
}
