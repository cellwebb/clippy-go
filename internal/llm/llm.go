package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Provider defines the interface for an LLM provider
type Provider interface {
	Generate(prompt string, systemPrompt string) (string, error)
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

func (p *OpenAIProvider) Generate(prompt string, systemPrompt string) (string, error) {
	url := p.Config.BaseURL + "/chat/completions"
	if p.Config.BaseURL == "" {
		url = "https://api.openai.com/v1/chat/completions"
	}

	reqBody := map[string]interface{}{
		"model": p.Config.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.Config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return result.Choices[0].Message.Content, nil
}

// AnthropicProvider implements Provider for Anthropic APIs
type AnthropicProvider struct {
	Config Config
}

func (p *AnthropicProvider) Generate(prompt string, systemPrompt string) (string, error) {
	url := p.Config.BaseURL + "/v1/messages"
	if p.Config.BaseURL == "" {
		url = "https://api.anthropic.com/v1/messages"
	}

	reqBody := map[string]interface{}{
		"model":      p.Config.Model,
		"max_tokens": 1024,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.Config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return result.Content[0].Text, nil
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
