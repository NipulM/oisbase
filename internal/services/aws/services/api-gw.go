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
	awsgenerator "github.com/NipulM/oisbase/internal/services/aws/generator"
	"github.com/NipulM/oisbase/internal/services/aws/registry"
	"github.com/NipulM/oisbase/internal/services/aws/templates"
	"github.com/NipulM/oisbase/internal/utils"
)

type APIGatewayService struct{}

func (a *APIGatewayService) Name() string {
	return "api-gateway"
}

func (a *APIGatewayService) GetConfig() (map[string]interface{}, error) {
	config := make(map[string]interface{})

	var apiType string
	survey.AskOne(&survey.Select{
		Message: "API Gateway type:",
		Options: []string{"HTTP"},
		Default: "HTTP",
	}, &apiType)
	config["api_type"] = apiType

	var apiName string
	survey.AskOne(&survey.Input{
		Message: "API Gateway name:",
	}, &apiName, survey.WithValidator(survey.Required))
	config["api_name"] = apiName
	config["instance_name"] = apiName

	projectCfg, err := projectconfig.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load project config: %w", err)
	}

	err = projectCfg.AddServiceInstance(a.Name(), apiName)
	if err != nil {
		return nil, fmt.Errorf("failed to add api gateway instance to config: %w", err)
	}

	err = projectconfig.SaveConfig(projectCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to save config after adding instance: %w", err)
	}

	affected, err := registry.PromptForConnections(a.Name(), apiName, projectCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to configure connections: %w", err)
	}

	if len(affected) > 0 {
		config["affected_instances"] = affected
	}

	return config, nil
}

func (a *APIGatewayService) GenerateModule(config map[string]interface{}) (string, error) {
	environments := config["environments"].([]string)
	apiName := config["api_name"].(string)
	projectName := config["project_name"].(string)
	region := config["region"].(string)

	var results []string

	// Reload config to get routes/access saved during PromptForConnections
	projectCfg, err := projectconfig.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	iamGen := awsgenerator.NewIAMPolicyGenerator(projectCfg)

	for _, environment := range environments {
		group := utils.EnvironmentGroup(environment)
		serviceDir := filepath.Join("environments", group, environment, a.Name())
		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create service directory: %w", err)
		}

		if err := a.createBackendTf(serviceDir, projectName, environment, region); err != nil {
			return "", err
		}
		if err := a.createOrUpdateMainTf(serviceDir, region, apiName); err != nil {
			return "", err
		}

		instanceDir := filepath.Join(serviceDir, apiName)
		if err := os.MkdirAll(instanceDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create instance directory: %w", err)
		}

		// Build template data generically from config + registry
		instance := projectCfg.Services[a.Name()].Instances[apiName]
		templateData := iamGen.BuildRegenTemplateData(a.Name(), apiName, environment, instance)
		templateData["region"] = region

		if err := a.generateInstanceFiles(instanceDir, templateData); err != nil {
			return "", err
		}

		results = append(results, fmt.Sprintf("  ✓ [%s/%s] Created API Gateway: %s", group, environment, apiName))
	}

	return strings.Join(results, "\n"), nil
}

func (a *APIGatewayService) createBackendTf(serviceDir, projectName, environment, region string) error {
	backendPath := filepath.Join(serviceDir, "backend.tf")
	if _, err := os.Stat(backendPath); err == nil {
		return nil
	}
	content := fmt.Sprintf(`terraform {
  backend "s3" {
    bucket         = "%s-terraform-states-bucket-%s"
    key            = "%s/api-gateway/terraform.tfstate"
    region         = "%s"
    dynamodb_table = "%s-terraform-lock-table-%s"
    encrypt        = true
  }
}
`, projectName, environment, environment, region, projectName, environment)
	return os.WriteFile(backendPath, []byte(content), 0644)
}

func (a *APIGatewayService) createOrUpdateMainTf(serviceDir, region, apiName string) error {
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

	moduleName := strings.ReplaceAll(apiName, "-", "_")
	if strings.Contains(existingContent, fmt.Sprintf(`module "%s"`, moduleName)) {
		return nil
	}

	moduleBlock := fmt.Sprintf(`module "%s" {
  source = "./%s"
}

`, moduleName, apiName)
	return os.WriteFile(mainTfPath, []byte(existingContent+moduleBlock), 0644)
}

func (a *APIGatewayService) generateInstanceFiles(instanceDir string, config map[string]interface{}) error {
	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).ParseFS(templates.APIGatewayFS, "api-gw/http/*.tmpl")
	if err != nil {
		return err
	}

	templateFiles := map[string]string{
		"main.tf.tmpl":      "main.tf",
		"variables.tf.tmpl": "variables.tf",
		"outputs.tf.tmpl":   "outputs.tf",
		"iam.tf.tmpl":       "iam.tf",
		"data.tf.tmpl":      "data.tf",
		"api.yaml.tmpl":     "api.yaml",
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