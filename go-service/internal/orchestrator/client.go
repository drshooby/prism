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
	"strings"

	"github.com/benkamin03/prism/internal/infisical"
	"github.com/benkamin03/prism/internal/minio"
	"github.com/labstack/echo/v4"
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
	RepoURL         string
	GitHubToken     string
	UserID          string
	ProjectID       string
	MinioClient     minio.MinioClient
	InfisicalClient infisical.InfisicalClient
	Context         context.Context
}

func NewOrchestrator(config *NewOrchestratorInput) *Orchestrator {
	return &Orchestrator{
		repoURL:         config.RepoURL,
		gitHubToken:     config.GitHubToken,
		userID:          config.UserID,
		projectID:       config.ProjectID,
		minioClient:     config.MinioClient,
		infisicalClient: config.InfisicalClient,
		context:         config.Context,
	}
}

func (o *Orchestrator) CloneAndNavigateToRepo() error {
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

func (o *Orchestrator) DownloadOrCreateTFStateFile(bucketName string) error {
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

func (o *Orchestrator) GetOrCreateBranch(branchName string) error {
	// Clone and navigate to the repository
	if err := o.CloneAndNavigateToRepo(); err != nil {
		return fmt.Errorf("error in CloneAndNavigateToRepo: %w", err)
	}

	// Check if branch exists locally
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	if err := cmd.Run(); err != nil {
		// Branch does not exist, create it
		cmd = exec.Command("git", "checkout", "-b", branchName)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create branch %s: %s, %w", branchName, string(output), err)
		}
	} else {
		// Branch exists, checkout to it
		cmd = exec.Command("git", "checkout", branchName)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to checkout to branch %s: %s, %w", branchName, string(output), err)
		}
	}

	// Push the branch to remote
	cmd = exec.Command("git", "push", "-u", "origin", branchName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push branch %s to remote: %s, %w", branchName, string(output), err)
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

func (o *Orchestrator) GetConversation(conversationID string) (*FilesResponse, error) {
	// Get or create the branch for the conversation
	if err := o.GetOrCreateBranch(conversationID); err != nil {
		return nil, fmt.Errorf("error in GetOrCreateBranch: %w", err)
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

func (o *Orchestrator) Plan() (map[string]interface{}, error) {
	// Clone and navigate to the repository
	if err := o.CloneAndNavigateToRepo(); err != nil {
		return nil, fmt.Errorf("error in CloneAndNavigateToRepo: %w", err)
	}
	log.Printf("Successfully cloned and navigated to repo")

	// Check if the bucket exists, if not create it
	bucket, err := o.minioClient.GetOrCreateBucket(o.context, o.userID)
	if err != nil {
		return nil, fmt.Errorf("error in GetOrCreateBucket: %w", err)
	}

	// Download or create the terraform.tfstate file
	if err := o.DownloadOrCreateTFStateFile(bucket.Name); err != nil {
		return nil, fmt.Errorf("error in DownloadOrCreateTFStateFile: %w", err)
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

type ModifiedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ChatWorkflowResult struct {
	Plan          map[string]interface{} `json:"plan"`
	ModifiedFiles []ModifiedFile         `json:"modified_files"`
	Branch        string                 `json:"branch"`
	CommitHash    string                 `json:"commit_hash"`
}

func (o *Orchestrator) ProcessChatWorkflow(branchName string, modifiedFiles []ModifiedFile, commitMessage string) (*ChatWorkflowResult, error) {
	// Step 1: Clone and checkout/create branch
	if err := o.GetOrCreateBranch(branchName); err != nil {
		return nil, fmt.Errorf("error in GetOrCreateBranch: %w", err)
	}
	log.Printf("Successfully checked out branch: %s", branchName)

	// Step 2: Write modified files
	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	for _, file := range modifiedFiles {
		filePath := filepath.Join(workDir, file.Path)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for %s: %w", file.Path, err)
		}
		if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", file.Path, err)
		}
		log.Printf("Wrote file: %s", file.Path)
	}

	// Step 3: Commit changes
	cmd := exec.Command("git", "add", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to add files: %s, %w", string(output), err)
	}

	cmd = exec.Command("git", "commit", "-m", commitMessage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "nothing to commit") {
			return nil, fmt.Errorf("failed to commit: %s, %w", string(output), err)
		}
		log.Printf("No changes to commit")
	}

	// Get commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	commitHashBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit hash: %s, %w", string(commitHashBytes), err)
	}
	commitHash := strings.TrimSpace(string(commitHashBytes))

	// Step 4: Push changes
	cmd = exec.Command("git", "push", "-u", "origin", branchName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to push: %s, %w", string(output), err)
	}
	log.Printf("Pushed changes to branch: %s", branchName)

	// Step 5: Inject secrets and run terraform plan
	bucket, err := o.minioClient.GetOrCreateBucket(o.context, o.userID)
	if err != nil {
		return nil, fmt.Errorf("error in GetOrCreateBucket: %w", err)
	}

	if err := o.DownloadOrCreateTFStateFile(bucket.Name); err != nil {
		return nil, fmt.Errorf("error in DownloadOrCreateTFStateFile: %w", err)
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
	log.Printf("Fetched secrets from Infisical")

	// Inject the secret key/value pairs into the environment
	for key, value := range secretsResponse.Secrets {
		log.Printf("Injecting secret into environment: %s", key)
		if err := os.Setenv(key, value); err != nil {
			return nil, fmt.Errorf("failed to set secret env %s: %w", key, err)
		}
	}

	// Run terraform init
	log.Printf("Running terraform init")
	cmd = exec.Command("terraform", "init", "-upgrade")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform init failed: %s, %w", string(output), err)
	}

	// Run terraform plan
	log.Printf("Running terraform plan")
	cmd = exec.Command("terraform", "plan", "-no-color", "-input=false", "-out=tfplan")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform plan failed: %s, %w", string(output), err)
	}

	// Upload state file
	if err := o.minioClient.UploadFileObject(o.context, bucket.Name, "terraform.tfstate", ".terraform/terraform.tfstate"); err != nil {
		return nil, fmt.Errorf("error uploading terraform.tfstate: %w", err)
	}

	// Convert the plan to JSON
	log.Printf("Converting terraform plan to JSON")
	cmd = exec.Command("sh", "-c", "terraform show -json tfplan > plan.json")
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("terraform show -json failed: %s, %w", string(output), err)
	}

	// Read the JSON file
	planFileContent, err := os.ReadFile("plan.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read plan.json: %w", err)
	}

	var planJSON map[string]interface{}
	if err := json.Unmarshal(planFileContent, &planJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan.json: %w", err)
	}

	// Clean up temp directory
	tmpDir := workDir
	defer os.RemoveAll(tmpDir)
	log.Printf("Will clean up temporary directory: %s", tmpDir)

	return &ChatWorkflowResult{
		Plan:          planJSON,
		ModifiedFiles: modifiedFiles,
		Branch:        branchName,
		CommitHash:    commitHash,
	}, nil
}
