package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type FileConfig struct {
	LLM Config `json:"llm"`
}

type Config struct {
	Provider       string `json:"provider"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	APIKey         string `json:"api_key"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var fc FileConfig
	if err := json.Unmarshal(b, &fc); err != nil {
		return Config{}, err
	}
	if !fc.LLM.empty() {
		return fc.LLM.normalized(), nil
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg.normalized(), nil
}

func (c Config) ValidateMiniMax(strict bool) error {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" && !strict {
		provider = "minimax"
	}
	if provider != "minimax" {
		return fmt.Errorf("unsupported llm provider %q, want minimax", c.Provider)
	}

	var missing []string
	if strings.TrimSpace(c.BaseURL) == "" {
		missing = append(missing, "base_url")
	}
	if strings.TrimSpace(c.Model) == "" {
		missing = append(missing, "model")
	}
	if strict && strings.TrimSpace(c.APIKey) == "" {
		missing = append(missing, "api_key")
	}
	if len(missing) > 0 {
		return fmt.Errorf("invalid minimax config: missing %s", strings.Join(missing, ", "))
	}
	return nil
}

func (c Config) empty() bool {
	return c.Provider == "" && c.BaseURL == "" && c.Model == "" && c.APIKey == "" && c.TimeoutSeconds == 0
}

func (c Config) normalized() Config {
	c.Provider = strings.ToLower(strings.TrimSpace(c.Provider))
	c.BaseURL = strings.TrimSpace(c.BaseURL)
	c.Model = strings.TrimSpace(c.Model)
	c.APIKey = strings.TrimSpace(c.APIKey)
	return c
}
