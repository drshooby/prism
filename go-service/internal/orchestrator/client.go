package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/benkamin03/prism/internal/minio"
)

type Orchestrator struct {
	repoURL     string
	gitHubToken string
	userID      string
	minioClient minio.MinioClient
	context     context.Context
}

type NewOrchestratorInput struct {
	repoURL     string
	gitHubToken string
	userID      string
	minioClient minio.MinioClient
	context     context.Context
}

func NewOrchestrator(config *NewOrchestratorInput) *Orchestrator {
	return &Orchestrator{
		repoURL:     config.repoURL,
		gitHubToken: config.gitHubToken,
		userID:      config.userID,
		minioClient: config.minioClient,
		context:     config.context,
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
	if err := o.minioClient.DownloadFileObject(o.context, bucketName, "terraform.tfstate", "terraform.tfstate"); err != nil {
		// Create the file if it does not exist
		if err := os.NewFile(0, "terraform.tfstate").Close(); err != nil {
			return fmt.Errorf("error creating empty terraform.tfstate: %w", err)
		}
		fmt.Println("terraform.tfstate not found in bucket, created empty file.")
	} else {
		fmt.Println("Downloaded terraform.tfstate from bucket.")
	}
	return nil
}

func (o *Orchestrator) Plan() error {
	// Clone and navigate to the repository
	if err := o.cloneAndNavigateToRepo(); err != nil {
		return fmt.Errorf("error in cloneAndNavigateToRepo: %w", err)
	}
	log.Printf("Successfully cloned and navigated to repo")

	// Check if the bucket exists, if not create it
	log.Printf("Bucket name: %s", o.userID)
	log.Printf("Minio client: %v", o.minioClient)
	bucket, err := o.minioClient.GetOrCreateBucket(o.context, o.userID)
	if err != nil {
		return fmt.Errorf("error in GetOrCreateBucket: %w", err)
	}

	// Download or create the terraform.tfstate file
	if err := o.downloadOrCreateTFStateFile(bucket.Name); err != nil {
		return fmt.Errorf("error in downloadOrCreateTFStateFile: %w", err)
	}

	return nil
}
