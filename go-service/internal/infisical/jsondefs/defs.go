package jsondefs

type ListSecretsRequest struct {
	Environment string `json:"environment" validate:"required"`
	ProjectID   string `json:"projectId" validate:"required"`
	SecretPath  string `json:"secretPath" validate:"required"`
}

type GetSecretResponse struct {
	Secrets    []string `json:"secret"`
	StatusCode int      `json:"statusCode"`
	Error      string   `json:"error,omitempty"`
}

type CreateProjectRequest struct {
	ProjectName        string `json:"projectName"`
	ProjectDescription string `json:"projectDescription,omitempty"`
	Slug               string `json:"slug,omitempty"`
	TypeField          string `json:"type,omitempty"`
}

type CreateProjectResponse struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}
