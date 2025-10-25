package infisical

import (
	"context"
	"fmt"
	"log"

	infisical "github.com/infisical/go-sdk"
)

type InfisicalClientConfig struct {
	SiteUrl               string
	InfisicalClientID     string
	InfisicalClientSecret string
}

type InfisicalSecretOptions struct {
	Environment string
	ProjectID   string
	SecretPath  string
}

type InfisicalClient struct {
	client infisical.InfisicalClientInterface
}

type GetSecretResponse struct {
	Secrets    map[string]string `json:"secret"`
	StatusCode int               `json:"statusCode"`
	Error      string            `json:"error,omitempty"`
}

func NewInfisicalClient(config *InfisicalClientConfig) (*InfisicalClient, error) {
	infisicalClientInterface := infisical.NewInfisicalClient(context.Background(), infisical.Config{
		SiteUrl:          config.SiteUrl,
		AutoTokenRefresh: true,
	})

	if infisicalClientInterface == nil {
		log.Fatal("infisicalClient is nil")
	}

	// For machine identity (what go sdk uses)
	// 1. Org -> Access Control -> Identities -> Create Identity w/ Member Role
	// 2. Secrets Manager -> Access Management -> Machine Identities -> Add Identity -> Select w/ Developer Role
	// 3. Org -> Access Control -> Identities -> Click Identity -> Universal Auth
	// -> Copy Client ID -> Create Client Secret -> Copy Client Secret
	_, err := infisicalClientInterface.Auth().UniversalAuthLogin(config.InfisicalClientID, config.InfisicalClientSecret)
	if err != nil {
		panic(fmt.Sprintf("Authentication failed: %v", err))
	}

	return &InfisicalClient{
		client: infisicalClientInterface,
	}, nil
}

func NewListSecretOptions(base InfisicalSecretOptions) infisical.ListSecretsOptions {
	return infisical.ListSecretsOptions{
		Environment:        base.Environment,
		ProjectID:          base.ProjectID,
		SecretPath:         base.SecretPath,
		AttachToProcessEnv: true, // access secrets with getEnv
	}
}

func (infisicalClient InfisicalClient) ListSecrets(options *InfisicalSecretOptions) *GetSecretResponse {
	secrets, err := infisicalClient.client.Secrets().List(NewListSecretOptions(*options))
	if err != nil {
		return &GetSecretResponse{
			StatusCode: 500,
			Error:      fmt.Sprintf("Error getting secrets: %v", err),
		}
	}

	secretsMap := make(map[string]string, len(secrets))
	for _, s := range secrets {
		secretsMap[s.SecretKey] = s.SecretValue
	}

	return &GetSecretResponse{
		Secrets:    secretsMap,
		StatusCode: 200,
	}
}
