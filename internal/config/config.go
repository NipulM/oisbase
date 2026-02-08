package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const ConfigFileName = ".terraplate.json"

type ProjectConfig struct {
	ProjectName  string   `json:"project_name"`
	Environments []string `json:"environments"`
	Region       string   `json:"region"`
	Services     []string `json:"services"`
}

// SaveConfig saves the project configuration to .terraplate.json in current directory
func SaveConfig(config *ProjectConfig) error {
	config.ProjectName = strings.ToLower(config.ProjectName)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Save in current directory, not in project subdirectory
	if err := os.WriteFile(ConfigFileName, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadConfig loads the project configuration from .terraplate.json
func LoadConfig() (*ProjectConfig, error) {
	// Look for .terraplate.json in current directory
	data, err := os.ReadFile(ConfigFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file (are you in a terraplate project?): %w", err)
	}

	var config ProjectConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}