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
	"github.com/NipulM/oisbase/internal/services/aws/templates"
	"github.com/NipulM/oisbase/internal/utils"
)

type CognitoService struct{}

func (c *CognitoService) Name() string {
	return "cognito"
}

func (c *CognitoService) GetConfig() (map[string]interface{}, error) {
	config := make(map[string]interface{})

	var poolName string
	survey.AskOne(&survey.Input{
		Message: "Cognito pool name:",
	}, &poolName, survey.WithValidator(survey.Required))
	config["pool_name"] = poolName
	config["instance_name"] = poolName

	projectCfg, err := projectconfig.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load project config: %w", err)
	}

	err = projectCfg.AddServiceInstance(c.Name(), poolName)
	if err != nil {
		return nil, fmt.Errorf("failed to add cognito instance to config: %w", err)
	}

	err = projectconfig.SaveConfig(projectCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to save config after adding instance: %w", err)
	}

	return config, nil
}

func (c *CognitoService) GenerateModule(config map[string]interface{}) (string, error) {
	environments := config["environments"].([]string)
	poolName := config["pool_name"].(string)
	projectName := config["project_name"].(string)
	region := config["region"].(string)

	var results []string

	for _, environment := range environments {
		group := utils.EnvironmentGroup(environment)

		serviceDir := filepath.Join("environments", group, environment, c.Name())
		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create cognito service directory: %w", err)
		}

		if err := c.createBackendTf(serviceDir, projectName, environment, region); err != nil {
			return "", err
		}

		if err := c.createOrUpdateMainTf(serviceDir, region, poolName); err != nil {
			return "", err
		}

		instanceDir := filepath.Join(serviceDir, poolName)
		if err := os.MkdirAll(instanceDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create cognito instance directory: %w", err)
		}

		// Pass environment into config for template rendering
		envConfig := make(map[string]interface{})
		for k, v := range config {
			envConfig[k] = v
		}
		envConfig["environment"] = environment

		if err := c.generateInstanceFiles(instanceDir, envConfig); err != nil {
			return "", err
		}

		results = append(results, fmt.Sprintf("  ✓ [%s/%s] Created Cognito pool: %s", group, environment, poolName))
	}

	return strings.Join(results, "\n"), nil
}

func (c *CognitoService) createBackendTf(serviceDir, projectName, environment, region string) error {
	backendPath := filepath.Join(serviceDir, "backend.tf")
	if _, err := os.Stat(backendPath); err == nil {
		return nil
	}
	content := fmt.Sprintf(`terraform {
  backend "s3" {
    bucket         = "%s-terraform-states-bucket-%s"
    key            = "%s/cognito/terraform.tfstate"
    region         = "%s"
    dynamodb_table = "%s-terraform-lock-table-%s"
    encrypt        = true
  }
}
`, projectName, environment, environment, region, projectName, environment)
	return os.WriteFile(backendPath, []byte(content), 0644)
}

func (c *CognitoService) createOrUpdateMainTf(serviceDir, region, poolName string) error {
	mainTfPath := filepath.Join(serviceDir, "main.tf")
	var existingContent string
	if content, err := os.ReadFile(mainTfPath); err == nil {
		existingContent = string(content)
	} else {
		existingContent = fmt.Sprintf(`provider "aws" {
  region = "%s"
}

`, region)
	}

	moduleName := strings.ReplaceAll(poolName, "-", "_")
	if strings.Contains(existingContent, fmt.Sprintf(`module "%s"`, moduleName)) {
		return nil
	}

	moduleBlock := fmt.Sprintf(`module "%s" {
  source = "./%s"
}

`, moduleName, poolName)
	return os.WriteFile(mainTfPath, []byte(existingContent+moduleBlock), 0644)
}

func (c *CognitoService) generateInstanceFiles(instanceDir string, config map[string]interface{}) error {
	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).ParseFS(templates.CognitoFS, "cognito/*.tmpl")
	if err != nil {
		return err
	}

	templateFiles := map[string]string{
		"main.tf.tmpl":      "main.tf",
		"variables.tf.tmpl": "variables.tf",
		"outputs.tf.tmpl":   "outputs.tf",
	}

	for tmplName, fileName := range templateFiles {
		if tmpl.Lookup(tmplName) == nil {
			continue
		}
		var out bytes.Buffer
		if err := tmpl.ExecuteTemplate(&out, tmplName, config); err != nil {
			return fmt.Errorf("failed to execute template %s: %w", tmplName, err)
		}
		if err := os.WriteFile(filepath.Join(instanceDir, fileName), out.Bytes(), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", fileName, err)
		}
	}

	return nil
}