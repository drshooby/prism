package orchestrator

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/benkamin03/prism/internal/infisical"
	"github.com/benkamin03/prism/internal/minio"
	"github.com/labstack/echo/v4"
)

type OrchestratorRoutesConfig struct {
	Echo            *echo.Echo
	MinioClient     minio.MinioClient
	InfisicalClient infisical.InfisicalClient
}

type PlanRequest struct {
	RepoURL     string `json:"repo_url" validate:"required"`
	GitHubToken string `json:"github_token" validate:"required"`
	UserID      string `json:"user_id" validate:"required"`
	ProjectID   string `json:"project_id" validate:"required"`
}

func SetupRoutes(routesConfig *OrchestratorRoutesConfig) {
	e := routesConfig.Echo
	log.Printf("infisicalClient (from SetupRoutes): %+v", routesConfig.InfisicalClient)

	e.POST("/plan", func(c echo.Context) error {
		var planRequest PlanRequest
		bodyContent, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Error reading request body: %v", err))
		}
		if err := json.Unmarshal(bodyContent, &planRequest); err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Error parsing request body: %v", err))
		}

		log.Printf("infisicalClient (from routes): %+v", routesConfig.InfisicalClient)

		orchestrator := NewOrchestrator(&NewOrchestratorInput{
			RepoURL:         planRequest.RepoURL,
			GitHubToken:     planRequest.GitHubToken,
			UserID:          planRequest.UserID,
			MinioClient:     routesConfig.MinioClient,
			InfisicalClient: routesConfig.InfisicalClient,
			ProjectID:       planRequest.ProjectID,
			context:         c.Request().Context(),
		})

		response, err := orchestrator.Plan()
		if err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("Error executing plan: %v", err))
		}

		return c.JSON(http.StatusOK, response)
	})

	e.GET("/conversations/:conversationID", func(c echo.Context) error {
		conversationID := c.Param("conversationID")
		repoURL := c.QueryParam("repo_url")
		githubToken := c.Request().Header.Get("Authorization")

		orchestrator := NewOrchestrator(&NewOrchestratorInput{
			MinioClient:     routesConfig.MinioClient,
			InfisicalClient: routesConfig.InfisicalClient,
			context:         c.Request().Context(),
			GitHubToken:     githubToken,
			RepoURL:         repoURL,
		})

		response, err := orchestrator.GetConversation(conversationID)
		if err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("Error retrieving conversation: %v", err))
		}

		return c.JSON(http.StatusOK, response)
	})

	e.DELETE("/conversations/:conversationID/messages/:commitHash", func(c echo.Context) error {
		conversationID := c.Param("conversationID")
		commitHash := c.Param("commitHash")
		repoURL := c.QueryParam("repo_url")
		githubToken := c.Request().Header.Get("Authorization")

		orchestrator := NewOrchestrator(&NewOrchestratorInput{
			MinioClient:     routesConfig.MinioClient,
			InfisicalClient: routesConfig.InfisicalClient,
			context:         c.Request().Context(),
			GitHubToken:     githubToken,
			RepoURL:         repoURL,
		})

		if err := orchestrator.DeleteCommit(conversationID, commitHash); err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("Error deleting message: %v", err))
		}

		return c.NoContent(http.StatusNoContent)
	})
}
