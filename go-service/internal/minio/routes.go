package minio

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

type MinioRoutesConfig struct {
	MinioClient MinioClient
	Echo        *echo.Echo
}

func SetupRoutes(routesConfig *MinioRoutesConfig) {
	e := routesConfig.Echo

	e.GET("/minio", func(c echo.Context) error {
		listBucketsResponse := routesConfig.MinioClient.ListBuckets(c.Request().Context())
		if listBucketsResponse.StatusCode != http.StatusOK {
			return c.String(500, fmt.Sprintf("Error listing buckets: %s", listBucketsResponse.Error))
		}
		return c.String(listBucketsResponse.StatusCode, fmt.Sprintf("Buckets: %v", listBucketsResponse.Buckets))
	})
}
