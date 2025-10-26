package infisical

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/benkamin03/prism/internal/infisical/jsondefs"
	"github.com/benkamin03/prism/internal/infisical/utils"
	"github.com/labstack/echo/v4"
)

type InfisicalRoutesConfig struct {
	InfisicalClient *InfisicalClient
	Echo            *echo.Echo
}

func SetupRoutes(routesConfig *InfisicalRoutesConfig) {
	e := routesConfig.Echo

	e.GET("/secrets", func(c echo.Context) error {
		return c.String(200, "Hi secrets!")
	})

	e.POST("/secrets/project/create", func(c echo.Context) error {
		var req jsondefs.CreateProjectRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
		}

		accessToken, err := utils.GetInfisicalAccessToken(
			PersistentConfig.InfisicalClientID,
			PersistentConfig.InfisicalClientSecret,
			PersistentConfig.SiteUrl,
		)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("auth failed: %v", err)})
		}

		payload := map[string]interface{}{
			"projectName":        req.ProjectName,
			"projectDescription": req.ProjectDescription,
			"slug":               req.Slug,
			"type":               req.TypeField,
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to marshal payload"})
		}

		apiURL := fmt.Sprintf("%s/api/v2/workspace", PersistentConfig.SiteUrl)
		httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to create request"})
		}

		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
		httpReq.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(httpReq)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to send request"})
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": "api returned non-OK status and failed to read response body"})
			}

			return c.JSON(http.StatusInternalServerError, echo.Map{
				"error": fmt.Sprintf("api returned %d: %s", resp.StatusCode, string(bodyBytes)),
			})
		}

		var result struct {
			Project jsondefs.CreateProjectResponse `json:"project"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to decode response"})
		}

		return c.JSON(http.StatusOK, echo.Map{
			"id":   result.Project.ID,
			"name": result.Project.Name,
			"slug": result.Project.Slug,
		})
	})

	e.POST("/secrets/list", func(c echo.Context) error {
		var req jsondefs.ListSecretsRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
		}

		if req.Environment == "" || req.ProjectID == "" || req.SecretPath == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "missing required fields"})
		}

		getSecretResponse := routesConfig.InfisicalClient.ListSecrets(&InfisicalSecretOptions{
			Environment: req.Environment,
			ProjectID:   req.ProjectID,
			SecretPath:  req.SecretPath,
		})

		if getSecretResponse.StatusCode != http.StatusOK {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("error retrieving secrest: %v", getSecretResponse.Error)})
		}

		return c.JSON(http.StatusOK, getSecretResponse)
	})

	// create, fetch
}
