package infisical

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

type InfisicalRoutesConfig struct {
	InfisicalClient *InfisicalClient
	Echo            *echo.Echo
}

type ListSecretsRequest struct {
	Environment string `json:"environment" validate:"required"`
	ProjectID   string `json:"projectId" validate:"required"`
	SecretPath  string `json:"secretPath" validate:"required"`
}

// type User struct {
// 	Name  string `json:"name" form:"name" query:"name"`
// 	Email string `json:"email" form:"email" query:"email"`
// }

func SetupRoutes(routesConfig *InfisicalRoutesConfig) {
	e := routesConfig.Echo

	e.GET("/secrets", func(c echo.Context) error {
		return c.String(200, "Hi secrets!")
	})

	e.POST("/secrets", func(c echo.Context) error {
		var req ListSecretsRequest
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
