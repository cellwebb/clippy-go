package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cellwebb/clippy-go/internal/tools"
)

// ToolCall represents a request from the LLM to execute a tool
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// Message represents a chat message
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // For tool responses
	Usage      *Usage     `json:"usage,omitempty"`        // Token usage stats
}

// Usage represents token usage statistics
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Provider defines the interface for an LLM provider
type Provider interface {
	Generate(messages []Message, tools []tools.Tool) (*Message, error)
	UpdateConfig(cfg Config)
	GetConfig() Config
}

// Config holds configuration for LLM providers
type Config struct {
	APIKey   string
	BaseURL  string
	Model    string
	Provider string // "openai" or "anthropic"
}

// NewProvider creates a new LLM provider based on config
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "openai":
		return &OpenAIProvider{Config: cfg}, nil
	case "anthropic":
		return &AnthropicProvider{Config: cfg}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// OpenAIProvider implements Provider for OpenAI compatible APIs
type OpenAIProvider struct {
	Config Config
}

func (p *OpenAIProvider) UpdateConfig(cfg Config) {
	p.Config = cfg
}

func (p *OpenAIProvider) GetConfig() Config {
	return p.Config
}

func (p *OpenAIProvider) Generate(messages []Message, availableTools []tools.Tool) (*Message, error) {
	url := p.Config.BaseURL + "/chat/completions"
	if p.Config.BaseURL == "" {
		url = "https://api.openai.com/v1/chat/completions"
	}

	// Convert internal messages to OpenAI format
	apiMessages := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		m := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]map[string]interface{}, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Arguments)
				toolCalls[j] = map[string]interface{}{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]interface{}{
						"name":      tc.Name,
						"arguments": string(argsJSON),
					},
				}
			}
			m["tool_calls"] = toolCalls
		}
		if msg.ToolCallID != "" {
			m["tool_call_id"] = msg.ToolCallID
		}
		apiMessages[i] = m
	}

	// Convert tools to OpenAI format
	var apiTools []map[string]interface{}
	if len(availableTools) > 0 {
		apiTools = make([]map[string]interface{}, len(availableTools))
		for i, t := range availableTools {
			def := t.Definition()
			apiTools[i] = map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        def.Name,
					"description": def.Description,
					"parameters":  def.Parameters,
				},
			}
		}
	}

	reqBody := map[string]interface{}{
		"model":    p.Config.Model,
		"messages": apiMessages,
	}
	if len(apiTools) > 0 {
		reqBody["tools"] = apiTools
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.Config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no response from API")
	}

	choice := result.Choices[0].Message
	responseMsg := &Message{
		Role:    "assistant",
		Content: choice.Content,
		Usage: &Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}

	if len(choice.ToolCalls) > 0 {
		for _, tc := range choice.ToolCalls {
			var args map[string]interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			responseMsg.ToolCalls = append(responseMsg.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}

	return responseMsg, nil
}

// AnthropicProvider implements Provider for Anthropic APIs
type AnthropicProvider struct {
	Config Config
}

func (p *AnthropicProvider) UpdateConfig(cfg Config) {
	p.Config = cfg
}

func (p *AnthropicProvider) GetConfig() Config {
	return p.Config
}

func (p *AnthropicProvider) Generate(messages []Message, availableTools []tools.Tool) (*Message, error) {
	url := p.Config.BaseURL + "/v1/messages"
	if p.Config.BaseURL == "" {
		url = "https://api.anthropic.com/v1/messages"
	}

	// Convert internal messages to Anthropic format
	var systemPrompt string
	var apiMessages []map[string]interface{}

	for i := 0; i < len(messages); i++ {
		msg := messages[i]

		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}

		// Handle tool results (Role: "tool")
		// Anthropic expects tool results to be in a "user" message.
		// If there are multiple consecutive tool results, they must be in a single user message.
		if msg.Role == "tool" {
			content := []map[string]interface{}{}

			// Collect this and subsequent tool messages
			for i < len(messages) && messages[i].Role == "tool" {
				toolMsg := messages[i]
				content = append(content, map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": toolMsg.ToolCallID,
					"content":     toolMsg.Content,
				})
				i++
			}
			i-- // Decrement because the outer loop will increment

			apiMessages = append(apiMessages, map[string]interface{}{
				"role":    "user",
				"content": content,
			})
			continue
		}

		m := map[string]interface{}{
			"role": msg.Role,
		}

		if len(msg.ToolCalls) > 0 {
			content := []map[string]interface{}{}
			if msg.Content != "" {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": tc.Arguments,
				})
			}
			m["content"] = content
		} else {
			m["content"] = msg.Content
		}

		apiMessages = append(apiMessages, m)
	}

	// Convert tools to Anthropic format
	var apiTools []map[string]interface{}
	if len(availableTools) > 0 {
		apiTools = make([]map[string]interface{}, len(availableTools))
		for i, t := range availableTools {
			def := t.Definition()
			apiTools[i] = map[string]interface{}{
				"name":         def.Name,
				"description":  def.Description,
				"input_schema": def.Parameters,
			}
		}
	}

	reqBody := map[string]interface{}{
		"model":      p.Config.Model,
		"max_tokens": 1024,
		"messages":   apiMessages,
	}
	if systemPrompt != "" {
		reqBody["system"] = systemPrompt
	}
	if len(apiTools) > 0 {
		reqBody["tools"] = apiTools
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.Config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Content []struct {
			Type  string                 `json:"type"`
			Text  string                 `json:"text"`
			ID    string                 `json:"id"`
			Name  string                 `json:"name"`
			Input map[string]interface{} `json:"input"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("no response from API")
	}

	responseMsg := &Message{
		Role: "assistant",
		Usage: &Usage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		},
	}

	for _, c := range result.Content {
		if c.Type == "text" {
			responseMsg.Content += c.Text
		} else if c.Type == "tool_use" {
			responseMsg.ToolCalls = append(responseMsg.ToolCalls, ToolCall{
				ID:        c.ID,
				Name:      c.Name,
				Arguments: c.Input,
			})
		}
	}

	return responseMsg, nil
}

// LoadConfigFromEnv loads config from environment variables
func LoadConfigFromEnv() Config {
	return Config{
		APIKey:   os.Getenv("CLIPPY_API_KEY"),
		BaseURL:  os.Getenv("CLIPPY_BASE_URL"),
		Model:    os.Getenv("CLIPPY_MODEL"),
		Provider: os.Getenv("CLIPPY_PROVIDER"),
	}
}

// ModelsDevResponse represents the response from models.dev
type ModelsDevResponse []struct {
	Created     int    `json:"created"`
	Description string `json:"description"`
	ID          string `json:"id"`
	Object      string `json:"object"`
	OwnedBy     string `json:"owned_by"`
}

// FetchModels retrieves the list of available models from models.dev
func FetchModels() ([]string, error) {
	resp, err := http.Get("https://models.dev/api/models")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch models: %s", resp.Status)
	}

	var modelsResp ModelsDevResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range modelsResp {
		models = append(models, m.ID)
	}

	return models, nil
}
