package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/benkamin03/prism/internal/infisical"
	"github.com/benkamin03/prism/internal/minio"
)

type Orchestrator struct {
	repoURL         string
	gitHubToken     string
	userID          string
	projectID       string
	minioClient     minio.MinioClient
	infisicalClient infisical.InfisicalClient
	context         context.Context
}

type NewOrchestratorInput struct {
	repoURL         string
	gitHubToken     string
	userID          string
	projectID       string
	minioClient     minio.MinioClient
	infisicalClient infisical.InfisicalClient
	context         context.Context
}

func NewOrchestrator(config *NewOrchestratorInput) *Orchestrator {
	return &Orchestrator{
		repoURL:         config.repoURL,
		gitHubToken:     config.gitHubToken,
		userID:          config.userID,
		projectID:       config.projectID,
		minioClient:     config.minioClient,
		infisicalClient: config.infisicalClient,
		context:         config.context,
	}
}

func (o *Orchestrator) cloneAndNavigateToRepo() error {
	tmpDir, err := os.MkdirTemp("/var/tmp/", "cloned-repo-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Now go to this temporary directory
	log.Printf("Changing to temp directory: %s", tmpDir)
	if err := os.Chdir(tmpDir); err != nil {
		return fmt.Errorf("failed to change dir: %w", err)
	}

	// Export GitHub token for authentication
	log.Printf("Setting GH_TOKEN environment variable")
	if err := os.Setenv("GH_TOKEN", o.gitHubToken); err != nil {
		return fmt.Errorf("failed to set GH_TOKEN env: %w", err)
	}

	// Clone the repository
	log.Printf("Cloning repository into temp directory")
	cmd := exec.Command("git", "clone", o.repoURL, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone repo: %s, %w", string(output), err)
	}

	return nil
}

func (o *Orchestrator) downloadOrCreateTFStateFile(bucketName string) error {
	// Create the .terraform directory
	if err := os.MkdirAll(".terraform", os.ModePerm); err != nil {
		return fmt.Errorf("error creating .terraform directory: %w", err)
	}

	if err := o.minioClient.DownloadFileObject(o.context, bucketName, "terraform.tfstate", ".terraform/terraform.tfstate"); err != nil {
		// Create the file if it does not exist
		if err := os.NewFile(0, ".terraform/terraform.tfstate").Close(); err != nil {
			return fmt.Errorf("error creating empty terraform.tfstate: %w", err)
		}
		fmt.Println("terraform.tfstate not found in bucket, created empty file.")
	} else {
		fmt.Println("Downloaded terraform.tfstate from bucket.")
	}
	return nil
}

func (o *Orchestrator) manageTerraform() error {
	return nil
}

func (o *Orchestrator) Plan() (map[string]interface{}, error) {
	// Clone and navigate to the repository
	if err := o.cloneAndNavigateToRepo(); err != nil {
		return nil, fmt.Errorf("error in cloneAndNavigateToRepo: %w", err)
	}
	log.Printf("Successfully cloned and navigated to repo")

	// Check if the bucket exists, if not create it
	bucket, err := o.minioClient.GetOrCreateBucket(o.context, o.userID)
	if err != nil {
		return nil, fmt.Errorf("error in GetOrCreateBucket: %w", err)
	}

	// Download or create the terraform.tfstate file
	if err := o.downloadOrCreateTFStateFile(bucket.Name); err != nil {
		return nil, fmt.Errorf("error in downloadOrCreateTFStateFile: %w", err)
	}

	// Fetch and inject the secrets into the environment
	secretsResponse := o.infisicalClient.ListSecrets(&infisical.InfisicalSecretOptions{
		Environment: "dev",
		ProjectID:   o.projectID,
		SecretPath:  "/",
	})
	if secretsResponse.StatusCode != http.StatusOK || secretsResponse.Error != "" {
		return nil, fmt.Errorf("failed to fetch secrets (status code %d): %s", secretsResponse.StatusCode, secretsResponse.Error)
	}
	log.Printf("Fetched secrets from Infisical: %v", secretsResponse.Secrets)

	// Inject the secret key/value pairs into the environment
	for key, value := range secretsResponse.Secrets {
		log.Printf("Injecting secret into environment: %s", key)
		if err := os.Setenv(key, value); err != nil {
			return nil, fmt.Errorf("failed to set secret env %s: %w", key, err)
		}
	}

	// Run terraform plan
	log.Printf("Running terraform init")
	cmd := exec.Command("terraform", "init", "-upgrade")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform init failed: %s, %w", string(output), err)
	}
	log.Printf("Terraform initialized successfully")

	log.Printf("Running terraform plan")
	cmd = exec.Command("terraform", "plan", "-no-color", "-input=false", "-out=tfplan")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform plan failed: %s, %w", string(output), err)
	}
	log.Printf("Terraform plan executed successfully")

	// Ensure that we save this state file
	if err := o.minioClient.UploadFileObject(o.context, bucket.Name, "terraform.tfstate", ".terraform/terraform.tfstate"); err != nil {
		return nil, fmt.Errorf("error uploading terraform.tfstate: %w", err)
	}
	log.Printf("Uploaded updated terraform.tfstate to bucket %s", bucket.Name)

	// Convert the plan to json
	log.Printf("Converting terraform plan to JSON")
	cmd = exec.Command("sh", "-c", "terraform show -json tfplan > plan.json")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform show -json failed: %s, %w", string(output), err)
	} else {
		log.Printf("Terraform plan in JSON format:\n%s", string(output))
	}

	// Read the JSON file and log its content
	planFileContent, err := os.ReadFile("plan.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read plan.json: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(planFileContent, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan.json: %w", err)
	}
	log.Printf("Terraform Plan JSON Content: %v", response)

	// Remove the temporary directory
	tmpDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	if err := os.RemoveAll(tmpDir); err != nil {
		return nil, fmt.Errorf("failed to remove temp dir %s: %w", tmpDir, err)
	}
	log.Printf("Removed temporary directory: %s", tmpDir)

	return response, nil
}
