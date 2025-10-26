package main

import (
	"database/sql"
	"net/http"

	"github.com/benkamin03/prism/internal/infisical"
	"github.com/benkamin03/prism/internal/llm"
	"github.com/benkamin03/prism/internal/minio"
	"github.com/benkamin03/prism/internal/orchestrator"
	"github.com/labstack/echo/v4"
)

type RoutesConfig struct {
	Echo            *echo.Echo
	DatabaseClient  *sql.DB
	InfisicalClient infisical.InfisicalClient
	MinioClient     minio.MinioClient
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

	minio.SetupRoutes(&minio.MinioRoutesConfig{
		MinioClient: routesConfig.MinioClient,
		Echo:        e,
	})

	orchestrator.SetupRoutes(&orchestrator.OrchestratorRoutesConfig{
		Echo:            e,
		MinioClient:     routesConfig.MinioClient,
		InfisicalClient: routesConfig.InfisicalClient,
	})

	infisical.SetupRoutes(&infisical.InfisicalRoutesConfig{
		InfisicalClient: routesConfig.InfisicalClient,
		Echo:            e,
	})

	llm.SetupRoutes(&llm.LLMRoutesConfig{
		Echo:            e,
		MinioClient:     routesConfig.MinioClient,
		InfisicalClient: routesConfig.InfisicalClient,
	})
}
