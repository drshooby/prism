package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/benkamin03/prism/internal/infisical"
	"github.com/benkamin03/prism/internal/orchestrator"
	"github.com/labstack/echo/v4"
)

type LLMRoutesConfig struct {
	Echo            *echo.Echo
	InfisicalClient infisical.InfisicalClient
}

func SetupRoutes(routesConfig *LLMRoutesConfig) {
	e := routesConfig.Echo

	e.GET("/llm-plan", func(c echo.Context) error {
		return c.String(200, "Hello LLM!")
	})

	// POST /llm-plan
	// Expected payload (multipart/form-data):
	// - repo_url: string (required) - GitHub repository URL to clone
	// - github_token: string (required) - GitHub personal access token for authentication
	// - files: file[] (required) - One or more .tf files to replace/add in the cloned repo
	//
	// Returns JSON:
	// - On success: { "plan": <terraform_plan_json>, "output": <terraform_plan_text> }
	// - On error: { "error": <error_message> }

	e.POST("/conversations/:id", func(c echo.Context) error {
		conversationID := c.Param("id")

		if conversationID == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "conversation id is required"})
		}

		// Parse multipart form
		form, err := c.MultipartForm()
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "failed to parse form"})
		}

		repoURL := c.FormValue("repo_url")
		githubToken := c.FormValue("github_token")
		projectID := c.FormValue("project_id")

		if repoURL == "" || githubToken == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "repo_url, and github_token are required"})
		}

		files := form.File["files"]
		if len(files) == 0 {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "at least one file is required"})
		}

		orchestrator := orchestrator.NewOrchestrator(&orchestrator.NewOrchestratorInput{
			RepoURL:     repoURL,
			GitHubToken: githubToken,
			ProjectID:   projectID,
		})
		log.Printf("Orchestrator initialized: %v", orchestrator)

		tmpDir, err := orchestrator.CloneAndNavigateToRepo()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to clone and navigate to repo: %v", err)})
		}

		if err := orchestrator.GetOrCreateBranch(conversationID); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to get or create branch: %v", err)})
		}

		// Replace/add files from upload
		for _, fileHeader := range files {
			src, err := fileHeader.Open()
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to open uploaded file %s: %v", fileHeader.Filename, err)})
			}
			defer src.Close()

			dstPath := filepath.Join(tmpDir, fileHeader.Filename)

			// Create directories if needed
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to create directory for %s: %v", fileHeader.Filename, err)})
			}

			dst, err := os.Create(dstPath)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to create file %s: %v", fileHeader.Filename, err)})
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to write file %s: %v", fileHeader.Filename, err)})
			}
		}

		// Add all changes
		cmd := exec.Command("git", "add", ".")
		if output, err := cmd.CombinedOutput(); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to add files: %s", string(output))})
		}

		// Commit changes
		commitMsg := fmt.Sprintf("Update terraform config for conversation %s", conversationID)
		cmd = exec.Command("git", "commit", "-m", commitMsg)
		if output, err := cmd.CombinedOutput(); err != nil {
			// Check if it's "nothing to commit" error
			if !strings.Contains(string(output), "nothing to commit") {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to commit: %s", string(output))})
			}
		}

		// Get commit hash
		cmd = exec.Command("git", "rev-parse", "HEAD")
		commitHashBytes, err := cmd.CombinedOutput()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to get commit hash: %s", string(commitHashBytes))})
		}
		commitHash := strings.TrimSpace(string(commitHashBytes))

		// Push to remote
		cmd = exec.Command("git", "push", "origin", conversationID)
		if output, err := cmd.CombinedOutput(); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to push: %s", string(output))})
		}

		// Inject the variables into the environment for terraform
		secretsResponse := routesConfig.InfisicalClient.ListSecrets(&infisical.InfisicalSecretOptions{
			Environment: "dev",
			ProjectID:   projectID,
			SecretPath:  "/",
		})
		if secretsResponse.StatusCode != http.StatusOK || secretsResponse.Error != "" {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to fetch secrets: %s", secretsResponse.Error)})
		}
		log.Printf("Fetched secrets from Infisical: %v", secretsResponse.Secrets)

		// Inject the secret key/value pairs into the environment
		for key, value := range secretsResponse.Secrets {
			log.Printf("Injecting secret into environment: %s", key)
			if err := os.Setenv(key, value); err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to set env var %s: %v", key, err)})
			}
		}

		// Run terraform init
		cmd = exec.Command("terraform", "init")
		if output, err := cmd.CombinedOutput(); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("terraform init failed: %s", string(output))})
		}

		// Run terraform plan
		cmd = exec.Command("terraform", "plan", "-no-color", "-input=false", "-out=tfplan")
		planOutput, err := cmd.CombinedOutput()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("terraform plan failed: %s", string(planOutput))})
		}

		// Convert plan to JSON
		cmd = exec.Command("terraform", "show", "-json", "tfplan")
		jsonOutput, err := cmd.CombinedOutput()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("terraform show failed: %s", string(jsonOutput))})
		}

		var planJSON map[string]interface{}
		if err := json.Unmarshal(jsonOutput, &planJSON); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to parse plan JSON: %v", err)})
		}

		return c.JSON(http.StatusOK, echo.Map{
			"plan":        planJSON,
			"commit_hash": commitHash,
			"branch":      conversationID,
		})
	})
}
