package prompts

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/NipulM/oisbase/internal/config"
)

func GetProjectConfig() (*config.ProjectConfig, error) {
	cfg := &config.ProjectConfig{
		Services: make(map[string]config.Service),
	}

	// Project name
	projectNamePrompt := &survey.Input{
		Message: "What's your project name?",
		Help:    "This will be used as a prefix for your resources",
	}
	survey.AskOne(projectNamePrompt, &cfg.ProjectName, survey.WithValidator(survey.Required))

	// Environment selection
	var envCount string
	envCountPrompt := &survey.Select{
		Message: "How many environments do you need?",
		Options: []string{
			"Just Production",
			"Development and Production",
			"Staging, Development and Production",
			"UAT, Staging, Development and Production",
		},
		Default: "Staging, Development and Production",
	}
	survey.AskOne(envCountPrompt, &envCount)

	defaultEnvs := map[string][]string{
		"Just Production":                          {"prod"},
		"Development and Production":               {"dev", "prod"},
		"Staging, Development and Production":      {"stg", "dev", "prod"},
		"UAT, Staging, Development and Production": {"uat", "stg", "dev", "prod"},
	}
	cfg.Environments = defaultEnvs[envCount]

	// Services selection
	servicesPrompt := &survey.MultiSelect{
		Message: "Which AWS services do you want to configure?",
		Options: []string{
			"Lambda (Serverless Functions)",
			"DynamoDB (NoSQL Database)",
		},
	}
	var selectedServices []string
	survey.AskOne(servicesPrompt, &selectedServices, survey.WithValidator(survey.Required))

	for _, service := range selectedServices {
		switch {
		case contains(service, "Lambda"):
			cfg.Services["lambda"] = config.Service{
				Instances: make(map[string]*config.Instance),
			}
		case contains(service, "DynamoDB"):
			cfg.Services["dynamodb"] = config.Service{
				Instances: make(map[string]*config.Instance),
			}
		}
	}

	// Region
	regionPrompt := &survey.Select{
		Message: "Which AWS region?",
		Options: []string{
			"us-east-1",
			"us-west-2",
			"eu-west-1",
			"ap-southeast-1",
			"ap-south-1",
		},
		Default: "us-east-1",
	}
	survey.AskOne(regionPrompt, &cfg.Region)

	return cfg, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}