package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/benkamin03/prism/internal/infisical"
	"github.com/benkamin03/prism/internal/minio"
	"github.com/labstack/echo/v4"
)

type LLMRoutesConfig struct {
	Echo            *echo.Echo
	MinioClient     minio.MinioClient
	InfisicalClient infisical.InfisicalClient
}

type ChatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id"`
	GitHubToken    string `json:"github_token"`
	RepoURL        string `json:"repo_url"`
	UserID         string `json:"user_id"`
	ProjectID      string `json:"project_id"`
}

func SetupRoutes(routesConfig *LLMRoutesConfig) {
	e := routesConfig.Echo

	e.GET("/llm-plan", func(c echo.Context) error {
		return c.String(200, "Hello LLM!")
	})

	e.POST("/chat", func(c echo.Context) error {
		var chatReq ChatRequest
		if err := c.Bind(&chatReq); err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request payload"})
		}

		if chatReq.Message == "" || chatReq.ConversationID == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "message and conversation_id are required"})
		}

		if chatReq.RepoURL == "" || chatReq.GitHubToken == "" || chatReq.UserID == "" || chatReq.ProjectID == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "repo_url, github_token, user_id, and project_id are required"})
		}

		// Process chat using the extracted function
		result, err := ProcessChat(
			&ChatProcessRequest{
				Message:        chatReq.Message,
				ConversationID: chatReq.ConversationID,
				RepoURL:        chatReq.RepoURL,
				GitHubToken:    chatReq.GitHubToken,
				UserID:         chatReq.UserID,
				ProjectID:      chatReq.ProjectID,
			},
			&ChatProcessConfig{
				MinioClient:     routesConfig.MinioClient,
				InfisicalClient: routesConfig.InfisicalClient,
				Context:         c.Request().Context(),
			},
		)

		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		return c.JSON(http.StatusOK, result)
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

		if repoURL == "" || githubToken == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "repo_url, and github_token are required"})
		}

		files := form.File["files"]
		if len(files) == 0 {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "at least one file is required"})
		}

		// Create temp directory
		tmpDir, err := os.MkdirTemp("/var/tmp/", "cloned-repo-")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to create temp dir: %v", err)})
		}
		defer os.RemoveAll(tmpDir)

		// Clone with token in URL for authentication
		cloneURL := strings.Replace(repoURL, "https://", fmt.Sprintf("https://%s@", githubToken), 1)
		cmd := exec.Command("git", "clone", cloneURL, tmpDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{
				"error": fmt.Sprintf("failed to clone repo: %s, err: %v", string(output), err),
			})
		}

		// Change to cloned repo directory
		if err := os.Chdir(tmpDir); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to change to repo dir: %v", err)})
		}

		// Update remote URL to include token for push
		cmd = exec.Command("git", "remote", "set-url", "origin", cloneURL)
		if output, err := cmd.CombinedOutput(); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to set remote url: %s", string(output))})
		}

		// Check if branch exists remotely
		cmd = exec.Command("git", "ls-remote", "--heads", "origin", conversationID)
		output, _ = cmd.CombinedOutput()
		branchExists := len(output) > 0

		if branchExists {
			// Branch exists, checkout
			cmd = exec.Command("git", "checkout", conversationID)
			if output, err := cmd.CombinedOutput(); err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to checkout branch: %s", string(output))})
			}
		} else {
			// Branch doesn't exist, create new branch
			cmd = exec.Command("git", "checkout", "-b", conversationID)
			if output, err := cmd.CombinedOutput(); err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("failed to create branch: %s", string(output))})
			}
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

		// Configure git user
		cmd = exec.Command("git", "config", "user.email", "prism-bot@example.com")
		cmd.CombinedOutput()
		cmd = exec.Command("git", "config", "user.name", "Prism Bot")
		cmd.CombinedOutput()

		// Add all changes
		cmd = exec.Command("git", "add", ".")
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
