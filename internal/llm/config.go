package llm

import (
	"encoding/json"
	"os"
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
	return fc.LLM, nil
}
