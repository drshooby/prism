package orchestrator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/benkamin03/prism/internal/minio"
	"github.com/labstack/echo/v4"
)

type OrchestratorRoutesConfig struct {
	Echo        *echo.Echo
	MinioClient minio.MinioClient
}

type PlanRequest struct {
	RepoURL     string `json:"repo_url" validate:"required"`
	GitHubToken string `json:"github_token" validate:"required"`
	UserID      string `json:"user_id" validate:"required"`
}

func SetupRoutes(routesConfig *OrchestratorRoutesConfig) {
	e := routesConfig.Echo

	e.POST("/plan", func(c echo.Context) error {
		var planRequest PlanRequest
		bodyContent, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Error reading request body: %v", err))
		}
		if err := json.Unmarshal(bodyContent, &planRequest); err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Error parsing request body: %v", err))
		}

		orchestrator := NewOrchestrator(&NewOrchestratorInput{
			repoURL:     planRequest.RepoURL,
			gitHubToken: planRequest.GitHubToken,
			userID:      planRequest.UserID,
			minioClient: routesConfig.MinioClient,
			context:     c.Request().Context(),
		})

		if err := orchestrator.Plan(); err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("Error executing plan: %v", err))
		}

		return c.String(http.StatusOK, "Planned successfully")
	})
}
