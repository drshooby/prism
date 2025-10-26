package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/benkamin03/prism/internal/infisical"
	"github.com/benkamin03/prism/internal/minio"
	"github.com/benkamin03/prism/internal/orchestrator"
)

type LLMClient struct {
	apiKey string
	model  string
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenRouterRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type OpenRouterResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

type FileModification struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type LLMModificationResponse struct {
	Files []FileModification `json:"files"`
}

func NewLLMClient() *LLMClient {
	apiKey := os.Getenv("OPENROUTER_API_KEY")

	return &LLMClient{
		apiKey: apiKey,
		model:  "anthropic/claude-3.5-sonnet", // Fast and high quality model
	}
}

func (c *LLMClient) GenerateFileModifications(userMessage string, currentFiles []FileModification) (*LLMModificationResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY not set")
	}

	// Build context with current files
	filesContext := "Current Terraform files:\n"
	for _, file := range currentFiles {
		filesContext += fmt.Sprintf("\n--- %s ---\n%s\n", file.Path, file.Content)
	}

	systemPrompt := `You are a Terraform infrastructure expert. Based on the user's request, you need to modify the Terraform files to implement their changes.

IMPORTANT: You must PRESERVE all existing code and only make the minimal changes necessary. Think of this like adding to or modifying existing code, not replacing it entirely.

When modifying a file:
- Keep all existing resources, variables, and outputs that are not being changed
- Only add new resources or modify specific attributes that the user requested
- Preserve comments, formatting, and organization of the existing code
- If adding a new resource, append it to the existing file content
- If modifying an existing resource, only change the specific attributes needed

CRITICAL: Respond with ONLY a JSON object. No explanations, no markdown code blocks, no text before or after. Just raw JSON.

Format:
{
  "files": [
    {
      "path": "main.tf",
      "content": "resource \"aws_instance\" \"example\" {\n  ami = \"ami-123\"\n}"
    }
  ]
}

Only include files that need to be modified or created.`

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: filesContext},
		{Role: "user", Content: userMessage},
	}

	reqBody := OpenRouterRequest{
		Model:    c.model,
		Messages: messages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenRouter API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, string(body))
	}

	var openRouterResp OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&openRouterResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openRouterResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	// Parse the LLM's response
	var modResponse LLMModificationResponse
	if err := json.Unmarshal([]byte(openRouterResp.Choices[0].Message.Content), &modResponse); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return &modResponse, nil
}

type ChatProcessRequest struct {
	Message        string
	ConversationID string
	RepoURL        string
	GitHubToken    string
	UserID         string
	ProjectID      string
}

type ChatProcessConfig struct {
	MinioClient     minio.MinioClient
	InfisicalClient infisical.InfisicalClient
	Context         context.Context
}

func ProcessChat(req *ChatProcessRequest, config *ChatProcessConfig) (*orchestrator.ChatWorkflowResult, error) {
	// Create orchestrator
	orch := orchestrator.NewOrchestrator(&orchestrator.NewOrchestratorInput{
		RepoURL:         req.RepoURL,
		GitHubToken:     req.GitHubToken,
		UserID:          req.UserID,
		ProjectID:       req.ProjectID,
		MinioClient:     config.MinioClient,
		InfisicalClient: config.InfisicalClient,
		Context:         config.Context,
	})

	// Get current files from the conversation
	filesResp, err := orch.GetConversation(req.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// Convert to LLM format
	currentFiles := make([]FileModification, len(filesResp.Files))
	for i, f := range filesResp.Files {
		currentFiles[i] = FileModification{
			Path:    f.Path,
			Content: f.Content,
		}
	}

	// Use LLM to generate file modifications
	llmClient := NewLLMClient()
	modResponse, err := llmClient.GenerateFileModifications(req.Message, currentFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to generate modifications: %w", err)
	}

	// Convert to orchestrator format
	modifiedFiles := make([]orchestrator.ModifiedFile, len(modResponse.Files))
	for i, f := range modResponse.Files {
		modifiedFiles[i] = orchestrator.ModifiedFile{
			Path:    f.Path,
			Content: f.Content,
		}
	}

	// Process the complete workflow: commit, push, inject secrets, terraform plan
	commitMsg := fmt.Sprintf("AI: %s", req.Message)
	result, err := orch.ProcessChatWorkflow(req.ConversationID, modifiedFiles, commitMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to process workflow: %w", err)
	}

	return result, nil
}
