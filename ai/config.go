package ai

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ModelConfig defines the configuration for a single LLM.
type ModelConfig struct {
	Name        string  `json:"name" yaml:"name"`                             // e.g., "gemini-pro", "gpt-4"
	Provider    string  `json:"provider" yaml:"provider"`                     // e.g., "openai", "google", "ollama"
	APIKey      string  `json:"api_key" yaml:"api_key"`                       // Environment variable reference or direct key
	BaseURL     string  `json:"base_url,omitempty" yaml:"base_url,omitempty"` // Optional: for custom endpoints
	ModelName   string  `json:"model_name" yaml:"model_name"`                 // The specific model ID (e.g., "gemini-1.5-pro")
	MaxTokens   int     `json:"max_tokens" yaml:"max_tokens"`                 // Max output tokens
	Temperature float64 `json:"temperature" yaml:"temperature"`               // Creativity
}

// Config holds the global AI configuration.
type Config struct {
	DefaultModel string        `json:"default_model" yaml:"default_model"`
	Models       []ModelConfig `json:"models" yaml:"models"`
}

// LoadConfig reads and parses the configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
