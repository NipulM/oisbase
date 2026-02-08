package prompts

import (
	"github.com/AlecAivazis/survey/v2"
)

type ProjectConfig struct {
    ProjectName  string
    Environments []string
    Services     []string
    Region       string
}

func GetProjectConfig() (*ProjectConfig, error) {
    config := &ProjectConfig{}
    
    // Project name
    projectNamePrompt := &survey.Input{
        Message: "What's your project name?",
        Help:    "This will be used as a prefix for your resources",
    }
    survey.AskOne(projectNamePrompt, &config.ProjectName, survey.WithValidator(survey.Required))
    
    // Number of environments
    var envCount string
    envCountPrompt := &survey.Select{
        Message: "How many environments do you need?",
        Options: []string{"Just Production", "Development and Production", "Staging, Development and Production", "UAT, Staging, Development and Production"},
        Default: "Staging, Development and Production",
    }
    survey.AskOne(envCountPrompt, &envCount)
    
    // Environment names
    defaultEnvs := map[string][]string{
        "Just Production": {"prod"},
        "Development and Production": {"dev", "prod"},
        "Staging, Development and Production": {"stg", "dev", "prod"},
        "UAT, Staging, Development and Production": {"uat", "stg", "dev", "prod"},
    }
    config.Environments = defaultEnvs[envCount]
    
    // AWS Services selection
    servicesPrompt := &survey.MultiSelect{
        Message: "Which AWS services do you want to configure?",
        Options: []string{
            // "VPC (Virtual Private Cloud)",
            // "RDS (Relational Database)",
            // "ECS (Container Service)",
            "Lambda (Serverless Functions)",
            // "Lambda ECR (Serverless Functions with ECR)",
            // "S3 (Object Storage)",
            // "ALB (Application Load Balancer)",
            // "CloudFront (CDN)",
            // "DynamoDB (NoSQL Database)",
        },
    }
    var selectedServices []string
    survey.AskOne(servicesPrompt, &selectedServices, survey.WithValidator(survey.Required))
    
    // Clean up service names (remove descriptions)
    for _, service := range selectedServices {
        switch {
        case contains(service, "VPC"):
            config.Services = append(config.Services, "vpc")
        case contains(service, "RDS"):
            config.Services = append(config.Services, "rds")
        case contains(service, "ECS"):
            config.Services = append(config.Services, "ecs")
        case contains(service, "Lambda ECR"):
            config.Services = append(config.Services, "lambda-ecr")
        case contains(service, "Lambda"):
            config.Services = append(config.Services, "lambda")
        case contains(service, "S3"):
            config.Services = append(config.Services, "s3")
        case contains(service, "ALB"):
            config.Services = append(config.Services, "alb")
        case contains(service, "CloudFront"):
            config.Services = append(config.Services, "cloudfront")
        case contains(service, "DynamoDB"):
            config.Services = append(config.Services, "dynamodb")
        }
    }
    
    // AWS Region
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
    survey.AskOne(regionPrompt, &config.Region)
    
    return config, nil
}

func contains(s, substr string) bool {
    return len(s) >= len(substr) && s[:len(substr)] == substr
}