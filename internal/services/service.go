package services

import (
	"fmt"

	aws "github.com/NipulM/oisbase/internal/services/aws/services"
)

type Service interface {
	Name() string

	GetConfig() (map[string]interface{}, error)

    GenerateModule(config map[string]interface{}) (string, error)
}

func GetService(serviceName string) (Service, error) {
	switch serviceName {
	case "lambda":
		return &aws.LambdaService{}, nil
	case "dynamodb":
		return &aws.DynamoDBService{}, nil
	default:
		return nil, fmt.Errorf("service '%s' not supported yet", serviceName)
	}
}

func ListAvailableServices() []string {
	return []string{"lambda", "dynamodb"}
}