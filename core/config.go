package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	LLM    LLMConfig    `toml:"llm"`
	Agent  AgentConfig  `toml:"agent"`
	Prompt PromptConfig `toml:"prompt"`
	Shell  ShellConfig  `toml:"shell"`
}

type LLMConfig struct {
	APIKey   string `toml:"api_key"`
	BaseURL  string `toml:"base_url"`
	Model    string `toml:"model"`
	MaxSteps int    `toml:"max_steps"`
}

type AgentConfig struct {
	SystemDir  string `toml:"system_dir"`
	HistoryDir string `toml:"history_dir"`
	WorkDir    string `toml:"work_dir"`
}

type PromptConfig struct {
	Common         string `toml:"common"`
	AgentAPI       string `toml:"agent_api"`
	AgentImplement string `toml:"agent_implement"`
}

type ShellConfig struct {
	Commands     []string                `toml:"commands"`
	PathLocation map[string]PathLocation `toml:"path_location"`
}

type PathLocation struct {
	Position []uint   `toml:"position"`
	After    []string `toml:"after"`
	Prefix   []string `toml:"prefix"`
}

func LoadConfig(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	// Validate required fields
	if cfg.LLM.APIKey == "" || cfg.LLM.BaseURL == "" || cfg.LLM.Model == "" {
		return nil, fmt.Errorf("api_key, base_url, and model must be set in config [llm]")
	}

	baseDir := filepath.Dir(path)
	// Resolve relative paths to absolute
	if !filepath.IsAbs(cfg.Agent.SystemDir) {
		absSystemDir, err := filepath.Abs(filepath.Join(baseDir, cfg.Agent.SystemDir))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve system_dir: %w", err)
		}
		cfg.Agent.SystemDir = absSystemDir
	}
	if fileInfo, err := os.Stat(cfg.Agent.SystemDir); err != nil || !fileInfo.IsDir() {
		return nil, fmt.Errorf("system_dir must be a valid directory: %w", err)
	}
	if !filepath.IsAbs(cfg.Agent.HistoryDir) {
		absHistoryDir, err := filepath.Abs(filepath.Join(baseDir, cfg.Agent.HistoryDir))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve history_dir: %w", err)
		}
		cfg.Agent.HistoryDir = absHistoryDir
	}
	if fileInfo, err := os.Stat(cfg.Agent.HistoryDir); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(cfg.Agent.HistoryDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create history_dir: %w", err)
		}
	} else if err != nil || !fileInfo.IsDir() {
		return nil, fmt.Errorf("history_dir must be a valid directory: %w", err)
	}
	if !filepath.IsAbs(cfg.Agent.WorkDir) {
		absWorkDir, err := filepath.Abs(filepath.Join(baseDir, cfg.Agent.WorkDir))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve work_dir: %w", err)
		}
		cfg.Agent.WorkDir = absWorkDir
	}
	if fileInfo, err := os.Stat(cfg.Agent.WorkDir); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(cfg.Agent.WorkDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create work_dir: %w", err)
		}
	} else if err != nil || !fileInfo.IsDir() {
		return nil, fmt.Errorf("work_dir must be a valid directory: %w", err)
	}
	return &cfg, nil
}
