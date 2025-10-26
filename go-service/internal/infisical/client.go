package infisical

import (
	"context"
	"fmt"
	"log"

	"github.com/benkamin03/prism/internal/infisical/jsondefs"
	infisical "github.com/infisical/go-sdk"
)

var PersistentConfig InfisicalClientConfig

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

	PersistentConfig.SiteUrl = config.SiteUrl
	PersistentConfig.InfisicalClientID = config.InfisicalClientID
	PersistentConfig.InfisicalClientSecret = config.InfisicalClientSecret

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

func (infisicalClient InfisicalClient) ListSecrets(options *InfisicalSecretOptions) *jsondefs.GetSecretResponse {
	secrets, err := infisicalClient.client.Secrets().List(NewListSecretOptions(*options))
	if err != nil {
		return &jsondefs.GetSecretResponse{
			StatusCode: 500,
			Error:      fmt.Sprintf("Error getting secrets: %v", err),
		}
	}

	secretsMap := make(map[string]string, len(secrets))
	for _, s := range secrets {
		secretsMap[s.SecretKey] = s.SecretValue
	}

	return &jsondefs.GetSecretResponse{
		Secrets:    secretValues,
		StatusCode: 200,
	}
}
