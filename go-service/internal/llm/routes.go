package llm

import (
	"bytes"
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

		log.Printf("Files to process: %v", files)
		// Replace/add files from upload
		for _, fileHeader := range files {
			src, err := fileHeader.Open()
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to open uploaded file %s: %v", fileHeader.Filename, err)})
			}
			defer src.Close()

			dstPath := filepath.Join(tmpDir, fileHeader.Filename)
			log.Printf("Writing uploaded file to: %s", dstPath)

			// Create directories if needed
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to create directory for %s: %v", fileHeader.Filename, err)})
			}

			dst, err := os.Create(dstPath)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to create file %s: %v", fileHeader.Filename, err)})
			}
			defer dst.Close()

			log.Printf("Copying contents '%s' to '%s'", fileHeader.Filename, dstPath)

			if _, err := io.Copy(dst, src); err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to write file %s: %v", fileHeader.Filename, err)})
			}
		}

		cmd := exec.Command("git", "status")
		statusOutput, err := cmd.CombinedOutput()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to get git status: %s", string(statusOutput))})
		}
		log.Printf("Git status after adding files:\n%s", string(statusOutput))

		// Add all changes
		cmd = exec.Command("git", "add", ".")
		if output, err := cmd.CombinedOutput(); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to add files: %s", string(output))})
		}
		log.Printf("Staged changes for commit")

		// Commit changes
		commitMsg := fmt.Sprintf("Update terraform config for conversation %s", conversationID)
		cmd = exec.Command("git", "commit", "-m", commitMsg)
		if output, err := cmd.CombinedOutput(); err != nil {
			// Check if it's "nothing to commit" error
			if !strings.Contains(string(output), "nothing to commit") {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to commit: %s", string(output))})
			}
		}
		log.Printf("Committed changes with message: %s", commitMsg)

		// Get commit hash
		cmd = exec.Command("git", "rev-parse", "HEAD")
		commitHashBytes, err := cmd.CombinedOutput()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to get commit hash: %s", string(commitHashBytes))})
		}
		commitHash := strings.TrimSpace(string(commitHashBytes))
		log.Printf("Commit hash: %s", commitHash)

		// Push to remote
		cmd = exec.Command("git", "push", "--force", "origin", conversationID)
		if output, err := cmd.CombinedOutput(); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to push: %s", string(output))})
		}
		log.Printf("Pushed changes to remote branch %s", conversationID)

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

	e.POST("/conversations/:id/pr", func(c echo.Context) error {
		conversationID := c.Param("id")

		if conversationID == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "conversation id is required"})
		}

		// Parse JSON request body
		var req CreatePRRequestBody
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid JSON body"})
		}

		if req.RepoURL == "" || req.GithubToken == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "repo_url and github_token are required"})
		}

		// Set defaults
		baseBranch := req.BaseBranch
		if baseBranch == "" {
			baseBranch = "main"
		}

		prTitle := req.PRTitle
		if prTitle == "" {
			prTitle = fmt.Sprintf("Terraform updates for conversation %s", conversationID)
		}

		prBody := req.PRBody
		if prBody == "" {
			prBody = fmt.Sprintf("Automated terraform configuration updates for conversation %s", conversationID)
		}

		// Extract owner and repo from repoURL
		// Example: https://github.com/drshooby/test-terraform-repo.git -> drshooby, test-terraform-repo
		repoPath := strings.TrimPrefix(req.RepoURL, "https://github.com/")
		repoPath = strings.TrimSuffix(repoPath, ".git")
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid repo URL format"})
		}
		owner := parts[0]
		repoName := parts[1]

		// Check if branch exists remotely using GitHub API
		exists, err := branchExistsOnGitHub(conversationID, owner, repoName, req.GithubToken)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to check branch: %v", err)})
		}
		if !exists {
			return c.JSON(http.StatusNotFound, echo.Map{"error": fmt.Sprintf("branch %s does not exist", conversationID)})
		}

		// Create pull request
		pr, err := createPullRequest(req.GithubToken, owner, repoName, conversationID, baseBranch, prTitle, prBody)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to create PR: %v", err)})
		}

		return c.JSON(http.StatusOK, echo.Map{
			"pr_number": pr.Number,
			"pr_url":    pr.HTMLURL,
			"branch":    conversationID,
			"base":      baseBranch,
		})
	})
}

type CreatePRRequestBody struct {
	RepoURL     string `json:"repo_url"`
	GithubToken string `json:"github_token"`
	BaseBranch  string `json:"base_branch,omitempty"` // defaults to "main"
	PRTitle     string `json:"pr_title,omitempty"`
	PRBody      string `json:"pr_body,omitempty"`
}

type CreatePRRequest struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body"`
}

type CreatePRResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Title   string `json:"title"`
}

// Helper functions
func branchExistsOnGitHub(branchName, owner, repo, token string) (bool, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches/%s", owner, repo, branchName)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
}

func createPullRequest(token, owner, repo, head, base, title, body string) (*CreatePRResponse, error) {
	prRequest := CreatePRRequest{
		Title: title,
		Head:  head,
		Base:  base,
		Body:  body,
	}

	jsonData, err := json.Marshal(prRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal PR request: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create PR (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var prResponse CreatePRResponse
	if err := json.NewDecoder(resp.Body).Decode(&prResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &prResponse, nil
}
