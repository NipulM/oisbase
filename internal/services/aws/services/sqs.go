package aws

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/sprig/v3"
	projectconfig "github.com/NipulM/oisbase/internal/config"
	"github.com/NipulM/oisbase/internal/services/aws/registry"
	"github.com/NipulM/oisbase/internal/services/aws/templates"
	"github.com/NipulM/oisbase/internal/utils"
)

type SQSService struct{}

func (s *SQSService) Name() string {
	return "sqs"
}

func (s *SQSService) GetConfig() (map[string]interface{}, error) {
	config := make(map[string]interface{})

	var queueName string
	survey.AskOne(&survey.Input{
		Message: "Queue name:",
	}, &queueName, survey.WithValidator(survey.Required))
	config["queue_name"] = queueName
	config["instance_name"] = queueName

	projectCfg, err := projectconfig.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load project config: %w", err)
	}

	err = projectCfg.AddServiceInstance(s.Name(), queueName)
	if err != nil {
		return nil, fmt.Errorf("failed to add sqs instance to config: %w", err)
	}

	err = projectconfig.SaveConfig(projectCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to save config after adding instance: %w", err)
	}

	affected, err := registry.PromptForConnections(s.Name(), queueName, projectCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to configure connections: %w", err)
	}

	if len(affected) > 0 {
		config["affected_instances"] = affected
	}

	return config, nil
}

func (s *SQSService) GenerateModule(config map[string]interface{}) (string, error) {
	environments := config["environments"].([]string)
	queueName := config["queue_name"].(string)
	projectName := config["project_name"].(string)
	region := config["region"].(string)

	var results []string

	for _, environment := range environments {
		group := utils.EnvironmentGroup(environment)

		// Create service directory structure: environments/{group}/{env}/sqs/
		serviceDir := filepath.Join("environments", group, environment, s.Name())
		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create sqs service directory: %w", err)
		}

		// Create backend.tf if it doesn't exist
		if err := s.createBackendTf(serviceDir, projectName, environment, region); err != nil {
			return "", err
		}

		// Create or update main.tf
		if err := s.createOrUpdateMainTf(serviceDir, region, queueName); err != nil {
			return "", err
		}

		// Create queue instance directory: environments/{group}/{env}/sqs/{queue-name}/
		instanceDir := filepath.Join(serviceDir, queueName)
		if err := os.MkdirAll(instanceDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create queue instance directory: %w", err)
		}

		// Pass environment into config for template rendering
		envConfig := make(map[string]interface{})
		for k, v := range config {
			envConfig[k] = v
		}
		envConfig["environment"] = environment

		// Generate template files in the instance directory
		if err := s.generateInstanceFiles(instanceDir, envConfig); err != nil {
			return "", err
		}

		_, err := projectconfig.LoadConfig()
		if err != nil {
			return "", fmt.Errorf("failed to load config for IAM generation: %w", err)
		}
	}

	return strings.Join(results, "\n"), nil
}

func (s *SQSService) createBackendTf(serviceDir, projectName, environment, region string) error {
	backendPath := filepath.Join(serviceDir, "backend.tf")

	// Don't overwrite if it exists
	if _, err := os.Stat(backendPath); err == nil {
		return nil
	}

	backendContent := fmt.Sprintf(`terraform {
  backend "s3" {
    bucket         = "%s-terraform-states-bucket-%s"
    key            = "%s/sqs/terraform.tfstate"
    region         = "%s"
    dynamodb_table = "%s-terraform-lock-table-%s"
    encrypt        = true
  }
}
`, projectName, environment, environment, region, projectName, environment)

	return os.WriteFile(backendPath, []byte(backendContent), 0644)
}

func (s *SQSService) createOrUpdateMainTf(serviceDir, region, functionName string) error {
	mainTfPath := filepath.Join(serviceDir, "main.tf")

	// Read existing content if file exists
	var existingContent string
	if content, err := os.ReadFile(mainTfPath); err == nil {
		existingContent = string(content)
	} else {
		// Create new main.tf with provider
		existingContent = fmt.Sprintf(`provider "aws" {
  region = "%s"
}

`, region)
	}

	// Check if module already exists (avoid duplicates)
	moduleName := strings.ReplaceAll(functionName, "-", "_")
	moduleBlock := fmt.Sprintf(`module "%s" {
  source = "./%s"
}

`, moduleName, functionName)

	if strings.Contains(existingContent, fmt.Sprintf(`module "%s"`, moduleName)) {
		// Module already exists, don't duplicate
		return nil
	}

	// Append new module
	newContent := existingContent + moduleBlock

	return os.WriteFile(mainTfPath, []byte(newContent), 0644)
}

func (s *SQSService) generateInstanceFiles(instanceDir string, config map[string]interface{}) error {
	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).ParseFS(templates.SQSFS, "sqs/*.tmpl")
	if err != nil {
		return err
	}

	// ParseFS names templates by base name, not full path
	templateFiles := map[string]string{
		"main.tf.tmpl":     "main.tf",
		"variables.tf.tmpl": "variables.tf",
	}

	for tmplName, fileName := range templateFiles {
		if tmpl.Lookup(tmplName) == nil {
			continue
		}

		var out bytes.Buffer
		if err := tmpl.ExecuteTemplate(&out, tmplName, config); err != nil {
			return fmt.Errorf("failed to execute template %s: %w", tmplName, err)
		}

		filePath := filepath.Join(instanceDir, fileName)
		if err := os.WriteFile(filePath, out.Bytes(), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", fileName, err)
		}
	}

	return nil
}