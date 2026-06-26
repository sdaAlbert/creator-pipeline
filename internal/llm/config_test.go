package llm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigSupportsTopLevelShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"provider":"minimax","base_url":"https://api.example.test/anthropic","model":"MiniMax-M2.7","api_key":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Provider != "minimax" || cfg.BaseURL == "" || cfg.Model != "MiniMax-M2.7" || cfg.APIKey != "secret" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadConfigSupportsNestedLLMShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"llm":{"provider":"minimax","base_url":"https://api.example.test/anthropic","model":"MiniMax-M2.7","api_key":"secret"}}`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Provider != "minimax" || cfg.BaseURL == "" || cfg.Model != "MiniMax-M2.7" || cfg.APIKey != "secret" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestValidateMiniMaxStrictRequiresAPIKey(t *testing.T) {
	cfg := Config{Provider: "minimax", BaseURL: "https://api.example.test/anthropic", Model: "MiniMax-M2.7"}
	if err := cfg.ValidateMiniMax(true); err == nil {
		t.Fatal("ValidateMiniMax(strict) succeeded, want missing api_key error")
	}
}

func TestValidateMiniMaxRejectsUnsupportedProvider(t *testing.T) {
	cfg := Config{Provider: "other", BaseURL: "https://api.example.test", Model: "model", APIKey: "secret"}
	if err := cfg.ValidateMiniMax(false); err == nil {
		t.Fatal("ValidateMiniMax() succeeded, want unsupported provider error")
	}
}
