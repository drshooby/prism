package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	dir, err := os.MkdirTemp("/var/tmp/repo", "cloned-repo-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Now go to this dir
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("failed to change dir: %w", err)
	}

	// Export GitHub token for authentication
	if err := os.Setenv("GH_TOKEN", o.gitHubToken); err != nil {
		return fmt.Errorf("failed to set GH_TOKEN env: %w", err)
	}

	// Clone the repository
	cmd := exec.Command("git", "clone", o.repoURL, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone repo: %s, %w", string(output), err)
	}

	segments := strings.Split(o.repoURL, "/")
	repoName := strings.TrimSuffix(segments[len(segments)-1], ".git")

	// Go to the cloned repo directory
	os.Chdir(repoName)

	return nil
}

func (o *Orchestrator) downloadOrCreateTFStateFile(bucketName string) error {
	if err := o.minioClient.DownloadFileObject(o.context, bucketName, "terraform.tfstate", "terraform.tfstate"); err != nil {
		if err := os.NewFile(0, "terraform.tfstate").Close(); err != nil {
			return fmt.Errorf("error creating empty terraform.tfstate: %w", err)
		}
		fmt.Println("terraform.tfstate not found in bucket, created empty file.")
	} else {
		fmt.Println("Downloaded terraform.tfstate from bucket.")
	}
	return nil
}

func (o *Orchestrator) Run() error {
	// Clone and navigate to the repository
	if err := o.cloneAndNavigateToRepo(); err != nil {
		return fmt.Errorf("error in cloneAndNavigateToRepo: %w", err)
	}

	// Check if the bucket exists, if not create it
	bucket, err := o.minioClient.GetOrCreateBucket(o.context, o.userID)
	if err != nil {
		return fmt.Errorf("error in GetOrCreateBucket: %w", err)
	}

	if err := o.downloadOrCreateTFStateFile(bucket.Name); err != nil {
		return fmt.Errorf("error in downloadOrCreateTFStateFile: %w", err)
	}

	return nil
}
