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

type LambdaService struct{}

func (l *LambdaService) Name() string {
	return "lambda"
}

func (l *LambdaService) GetConfig() (map[string]interface{}, error) {
	config := make(map[string]interface{})

	var functionName string
	survey.AskOne(&survey.Input{
		Message: "Lambda function name:",
	}, &functionName, survey.WithValidator(survey.Required))
	config["function_name"] = functionName
	config["instance_name"] = functionName

	var runtime string
	survey.AskOne(&survey.Select{
		Message: "Runtime:",
		Default: "nodejs20.x",
		Options: []string{"python3.9", "python3.11", "nodejs20.x"},
	}, &runtime)
	config["runtime"] = runtime

	var handler string
	survey.AskOne(&survey.Input{
		Message: "Handler (e.g., index.handler):",
		Default: "index.handler",
	}, &handler)
	config["handler"] = handler

	return config, nil
}

// environmentGroup returns "production" for prod, "pre-production" for everything else.
func environmentGroup(env string) string {
	if env == "prod" {
		return "production"
	}
	return "pre-production"
}

func (l *LambdaService) GenerateModule(config map[string]interface{}) (string, error) {
	environments := config["environments"].([]string)
	functionName := config["function_name"].(string)
	projectName := config["project_name"].(string)
	region := config["region"].(string)

	var results []string

	for _, environment := range environments {
		group := environmentGroup(environment)

		// Create service directory structure: environments/{group}/{env}/lambda/
		serviceDir := filepath.Join("environments", group, environment, "lambda")
		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create lambda service directory: %w", err)
		}

		// Create backend.tf if it doesn't exist
		if err := l.createBackendTf(serviceDir, projectName, environment, region); err != nil {
			return "", err
		}

		// Create or update main.tf
		if err := l.createOrUpdateMainTf(serviceDir, region, functionName); err != nil {
			return "", err
		}

		// Create function instance directory: environments/{group}/{env}/lambda/{function-name}/
		instanceDir := filepath.Join(serviceDir, functionName)
		if err := os.MkdirAll(instanceDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create function instance directory: %w", err)
		}

		// Pass environment into config for template rendering
		envConfig := make(map[string]interface{})
		for k, v := range config {
			envConfig[k] = v
		}
		envConfig["environment"] = environment

		// Generate template files in the instance directory
		if err := l.generateInstanceFiles(instanceDir, envConfig); err != nil {
			return "", err
		}

		results = append(results, fmt.Sprintf("  ✓ [%s/%s] Created Lambda function: %s", group, environment, functionName))
	}

	return strings.Join(results, "\n"), nil
}

func (l *LambdaService) createBackendTf(serviceDir, projectName, environment, region string) error {
	backendPath := filepath.Join(serviceDir, "backend.tf")

	// Don't overwrite if it exists
	if _, err := os.Stat(backendPath); err == nil {
		return nil
	}

	backendContent := fmt.Sprintf(`terraform {
  backend "s3" {
    bucket         = "%s-terraform-states-bucket-%s"
    key            = "%s/lambda/terraform.tfstate"
    region         = "%s"
    dynamodb_table = "%s-terraform-lock-table-%s"
    encrypt        = true
  }
}
`, projectName, environment, environment, region, projectName, environment)

	return os.WriteFile(backendPath, []byte(backendContent), 0644)
}

func (l *LambdaService) createOrUpdateMainTf(serviceDir, region, functionName string) error {
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

func (l *LambdaService) generateInstanceFiles(instanceDir string, config map[string]interface{}) error {
	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).ParseFS(templates.LambdaFS, "lambda/*.tmpl")
	if err != nil {
		return err
	}

	templateFiles := map[string]string{
		"lambda.tf.tmpl":    "lambda.tf",
		"variables.tf.tmpl": "variables.tf",
		"outputs.tf.tmpl":   "outputs.tf",
		"iam.tf.tmpl":       "iam.tf",
		"data.tf.tmpl":      "data.tf",
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