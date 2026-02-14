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
	"github.com/NipulM/oisbase/internal/services/aws/templates"
)

type DynamoDBService struct{}

func (d *DynamoDBService) Name() string {
	return "dynamodb"
}

func (d *DynamoDBService) GetConfig() (map[string]interface{}, error) {
	config := make(map[string]interface{})

	var tableName string
	survey.AskOne(&survey.Input{
		Message: "DynamoDB table name:",
	}, &tableName, survey.WithValidator(survey.Required))
	config["table_name"] = tableName
	config["instance_name"] = tableName

	return config, nil
}

func (d *DynamoDBService) GenerateModule(config map[string]interface{}) (string, error) {
	environments := config["environments"].([]string)
	tableName := config["table_name"].(string)
	projectName := config["project_name"].(string)
	region := config["region"].(string)

	var results []string

	for _, environment := range environments {
		// Create service directory structure: environments/{group}/{env}/dynamodb/
		serviceDir := filepath.Join("environments", environment, "dynamodb")
		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create dynamodb service directory: %w", err)
		}

		// Create backend.tf if it doesn't exist
		if err := d.createBackendTf(serviceDir, projectName, environment, region); err != nil {
			return "", err
		}

		// Create or update main.tf
		if err := d.createOrUpdateMainTf(serviceDir, region, tableName); err != nil {
			return "", err
		}

		// Create table instance directory: environments/{group}/{env}/dynamodb/{table-name}/
		instanceDir := filepath.Join(serviceDir, tableName)
		if err := os.MkdirAll(instanceDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create table instance directory: %w", err)
		}

		// Pass environment into config for template rendering
		envConfig := make(map[string]interface{})
		for k, v := range config {
			envConfig[k] = v
		}
		envConfig["environment"] = environment

		// Generate template files in the instance directory
		if err := d.generateInstanceFiles(instanceDir, envConfig); err != nil {
			return "", err
		}

		results = append(results, fmt.Sprintf("  ✓ [%s] Created DynamoDB table: %s", environment, tableName))
	}

	return strings.Join(results, "\n"), nil	
}

func (d *DynamoDBService) createBackendTf(serviceDir, projectName, environment, region string) error {
	backendPath := filepath.Join(serviceDir, "backend.tf")

	// Don't overwrite if it exists
	if _, err := os.Stat(backendPath); err == nil {
		return nil
	}

	backendContent := fmt.Sprintf(`terraform {
  backend "s3" {
    bucket         = "%s-terraform-states-bucket-%s"
    key            = "%s/dynamodb/terraform.tfstate"
    region         = "%s"
    dynamodb_table = "%s-terraform-lock-table-%s"
    encrypt        = true
  }
}
`, projectName, environment, environment, region, projectName, environment)

	return os.WriteFile(backendPath, []byte(backendContent), 0644)
}

func (d *DynamoDBService) createOrUpdateMainTf(serviceDir, region, tableName string) error {
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
	moduleName := strings.ReplaceAll(tableName, "-", "_")
	moduleBlock := fmt.Sprintf(`module "%s" {
  source = "./%s"
}

`, moduleName, tableName)

	if strings.Contains(existingContent, fmt.Sprintf(`module "%s"`, moduleName)) {
		// Module already exists, don't duplicate
		return nil
	}

	// Append new module
	newContent := existingContent + moduleBlock

	return os.WriteFile(mainTfPath, []byte(newContent), 0644)
}

func (d *DynamoDBService) generateInstanceFiles(instanceDir string, config map[string]interface{}) error {
	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).ParseFS(templates.DynamoDBFS, "dynamodb/*.tmpl")
	if err != nil {
		return err
	}

	templateFiles := map[string]string{
		"dynamodb.tf.tmpl":  "dynamodb.tf",
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