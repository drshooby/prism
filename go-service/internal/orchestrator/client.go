package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/benkamin03/prism/internal/infisical"
	"github.com/benkamin03/prism/internal/minio"
	"github.com/labstack/echo/v4"
)

type Orchestrator struct {
	RepoURL         string
	GitHubToken     string
	UserID          string
	ProjectID       string
	MinioClient     minio.MinioClient
	InfisicalClient infisical.InfisicalClient
	context         context.Context
}

type NewOrchestratorInput struct {
	RepoURL         string
	GitHubToken     string
	UserID          string
	ProjectID       string
	MinioClient     minio.MinioClient
	InfisicalClient infisical.InfisicalClient
	context         context.Context
}

func NewOrchestrator(config *NewOrchestratorInput) *Orchestrator {
	return &Orchestrator{
		RepoURL:         config.RepoURL,
		GitHubToken:     config.GitHubToken,
		UserID:          config.UserID,
		ProjectID:       config.ProjectID,
		MinioClient:     config.MinioClient,
		InfisicalClient: config.InfisicalClient,
		context:         config.context,
	}
}

func (o *Orchestrator) CloneAndNavigateToRepo() (string, error) {
	tmpDir, err := os.MkdirTemp("/var/tmp/", "cloned-repo-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Now go to this temporary directory
	log.Printf("Changing to temp directory: %s", tmpDir)
	if err := os.Chdir(tmpDir); err != nil {
		return "", fmt.Errorf("failed to change dir: %w", err)
	}

	// Export GitHub token for authentication
	log.Printf("Setting GH_TOKEN environment variable")
	if err := os.Setenv("GH_TOKEN", o.GitHubToken); err != nil {
		return "", fmt.Errorf("failed to set GH_TOKEN env: %w", err)
	}

	// Clone the repository
	log.Printf("Cloning repository into temp directory")
	cmd := exec.Command("git", "clone", o.RepoURL, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to clone repo: %s, %w", string(output), err)
	}

	return tmpDir, nil
}

func (o *Orchestrator) downloadOrCreateTFStateFile(bucketName string) error {
	// Create the .terraform directory
	if err := os.MkdirAll(".terraform", os.ModePerm); err != nil {
		return fmt.Errorf("error creating .terraform directory: %w", err)
	}

	if err := o.MinioClient.DownloadFileObject(o.context, bucketName, "terraform.tfstate", ".terraform/terraform.tfstate"); err != nil {
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

func (o *Orchestrator) remoteBranchExists(branchName string) bool {
	// Check if branch exists on remote
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func (o *Orchestrator) pushToRemote(branchName string) error {
	// Push the branch to remote
	cmd := exec.Command("git", "push", "--force", "-u", "origin", branchName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push branch %s to remote: %s, %w", branchName, string(output), err)
	}
	return nil
}

func (o *Orchestrator) GetOrCreateBranch(branchName string) error {
	// Check if branch exists
	if !o.remoteBranchExists(branchName) {
		// Branch does not exist, create it
		cmd := exec.Command("git", "checkout", "-b", branchName)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create branch %s: %s, %w", branchName, string(output), err)
		}
	} else {
		// Branch exists, checkout to it
		if err := o.checkoutLocalBranch(branchName); err != nil {
			return fmt.Errorf("error in checkoutLocalBranch: %w", err)
		}

		// Pull the latest changes
		cmd := exec.Command("git", "pull", "origin", branchName)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to pull latest changes for branch %s: %s, %w", branchName, string(output), err)
		}
	}

	// Push the branch to remote
	if err := o.pushToRemote(branchName); err != nil {
		return fmt.Errorf("error in pushToRemote: %w", err)
	}

	return nil
}

type FileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type FilesResponse struct {
	Files []FileContent `json:"files"`
	Count int           `json:"count"`
}

func getTerraformFiles() ([]FileContent, error) {
	var files []FileContent
	rootPath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file has .tf extension
		if filepath.Ext(path) == ".tf" {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Get relative path from root
			relPath, err := filepath.Rel(rootPath, path)
			if err != nil {
				relPath = path
			}

			files = append(files, FileContent{
				Path:    relPath,
				Content: string(content),
			})
		}

		return nil
	})

	return files, err
}

func handleGetTerraformFiles(c echo.Context) error {
	//
	files, err := getTerraformFiles()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	response := FilesResponse{
		Files: files,
		Count: len(files),
	}

	return c.JSON(http.StatusOK, response)
}

func (o *Orchestrator) checkoutLocalBranch(branchName string) error {
	// Checkout to the branch
	cmd := exec.Command("git", "checkout", branchName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout to branch %s: %s, %w", branchName, string(output), err)
	}
	return nil
}

func (o *Orchestrator) DeleteCommit(conversationID, commitHash string) (map[string]interface{}, error) {
	// Clone and navigate to the repository
	if _, err := o.CloneAndNavigateToRepo(); err != nil {
		return nil, fmt.Errorf("error in cloneAndNavigateToRepo: %w", err)
	}
	log.Printf("Successfully cloned and navigated to repo")

	// Checkout to the conversation branch
	if err := o.checkoutLocalBranch(conversationID); err != nil {
		return nil, fmt.Errorf("error in checkoutLocalBranch: %w", err)
	}
	log.Printf("Checked out to branch: %s", conversationID)

	// Delete the commit by resetting to the previous commit
	cmd := exec.Command("git", "reset", "--hard", commitHash+"^")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to reset commit %s: %s, %w", commitHash, string(output), err)
	}
	log.Printf("Reset to previous commit before: %s", commitHash)

	cmd = exec.Command("git", "push", "--force", "origin", conversationID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to force push after deleting commit %s: %s, %w", commitHash, string(output), err)
	}
	log.Printf("Force pushed changes to branch: %s", conversationID)

	return o.generateJSONPlan()
}

func (o *Orchestrator) GetConversation(conversationID string) (*FilesResponse, error) {
	// Clone and navigate to the repository
	if _, err := o.CloneAndNavigateToRepo(); err != nil {
		return nil, fmt.Errorf("error in cloneAndNavigateToRepo: %w", err)
	}

	// Get or create the branch for the conversation
	if err := o.GetOrCreateBranch(conversationID); err != nil {
		return nil, fmt.Errorf("error in getOrCreateBranch: %w", err)
	}
	log.Printf("Successfully got or created branch for conversation ID: %s", conversationID)

	// Fetch all the
	files, err := getTerraformFiles()
	response := FilesResponse{
		Files: files,
		Count: len(files),
	}
	if err != nil {
		return nil, fmt.Errorf("error in getTerraformFiles: %w", err)
	}

	return &response, nil
}

func (o *Orchestrator) generateJSONPlan() (map[string]interface{}, error) {
	// Check if the bucket exists, if not create it
	bucket, err := o.MinioClient.GetOrCreateBucket(o.context, o.UserID)
	if err != nil {
		return nil, fmt.Errorf("error in GetOrCreateBucket: %w", err)
	}

	// Download or create the terraform.tfstate file
	if err := o.downloadOrCreateTFStateFile(bucket.Name); err != nil {
		return nil, fmt.Errorf("error in downloadOrCreateTFStateFile: %w", err)
	}

	// Fetch and inject the secrets into the environment
	secretsResponse := o.InfisicalClient.ListSecrets(&infisical.InfisicalSecretOptions{
		Environment: "dev",
		ProjectID:   o.ProjectID,
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
	if err := o.MinioClient.UploadFileObject(o.context, bucket.Name, "terraform.tfstate", ".terraform/terraform.tfstate"); err != nil {
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

	return response, nil
}

func (o *Orchestrator) Plan() (map[string]interface{}, error) {
	// Clone and navigate to the repository
	tmpDir, err := o.CloneAndNavigateToRepo()
	if err != nil {
		return nil, fmt.Errorf("error in cloneAndNavigateToRepo: %w", err)
	}
	defer os.RemoveAll(tmpDir) // Clean up temp dir after execution
	log.Printf("Successfully cloned and navigated to repo")

	response, err := o.generateJSONPlan()
	if err != nil {
		return nil, fmt.Errorf("error in generateJSONPlan: %w", err)
	}
	log.Printf("Removed temporary directory: %s", tmpDir)

	return response, nil
}
