package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const ConfigFileName = ".oisbase.json"
type ProjectConfig struct {
	ProjectName  string             `json:"ProjectName"`
	Environments []string           `json:"Environments"`
	Region       string             `json:"Region"`
	Services     map[string]Service `json:"Services"`
}

type Service struct {
	Instances map[string]*Instance `json:"instances"`
}

type Instance struct {
	Access map[string]map[string][]string `json:"access,omitempty"`
}

func SaveConfig(config *ProjectConfig) error {
	config.ProjectName = strings.ToLower(config.ProjectName)

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(ConfigFileName, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func LoadConfig() (*ProjectConfig, error) {
	data, err := os.ReadFile(ConfigFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file (are you in a oisbase project?): %w", err)
	}

	var config ProjectConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Services == nil {
		config.Services = make(map[string]Service)
	}

	return &config, nil
}

func (p *ProjectConfig) AddService(serviceName string) {
	if p.Services == nil {
		p.Services = make(map[string]Service)
	}

	if _, exists := p.Services[serviceName]; !exists {
		p.Services[serviceName] = Service{
			Instances: make(map[string]*Instance),
		}
	}
}

func (p *ProjectConfig) AddServiceInstance(serviceName, instanceName string) error {
	p.AddService(serviceName)

	service := p.Services[serviceName]

	if service.Instances == nil {
		service.Instances = make(map[string]*Instance)
	}

	if _, exists := service.Instances[instanceName]; exists {
		return fmt.Errorf("instance %s already exists under %s", instanceName, serviceName)
	}

	service.Instances[instanceName] = &Instance{}
	p.Services[serviceName] = service

	return nil
}

func (p *ProjectConfig) AddInstanceAccess(
	serviceType string,
	instanceName string,
	targetServiceType string,
	targetInstanceName string,
	permissions []string,
) error {
	// Validate that the service and instance exist
	service, exists := p.Services[serviceType]
	if !exists {
		return fmt.Errorf("service type %s does not exist", serviceType)
	}

	instance, exists := service.Instances[instanceName]
	if !exists {
		return fmt.Errorf("instance %s does not exist under service %s", instanceName, serviceType)
	}

	// Initialize Access map if it doesn't exist
	if instance.Access == nil {
		instance.Access = make(map[string]map[string][]string)
	}

	// Initialize target service type map if it doesn't exist
	if instance.Access[targetServiceType] == nil {
		instance.Access[targetServiceType] = make(map[string][]string)
	}

	// Add or update the permissions for the target instance
	instance.Access[targetServiceType][targetInstanceName] = permissions

	// Re-assign the instance back to the map (in case it was a copy)
	service.Instances[instanceName] = instance
	p.Services[serviceType] = service

	return nil
}

func (p *ProjectConfig) GetAllExistingServiceTypes() []string {
    var types []string
    for svcType := range p.Services {
        types = append(types, svcType)
    }
    return types
}

func (p *ProjectConfig) GetServiceInstances(serviceType string) []string {
    var instanceNames []string

    service, exists := p.Services[serviceType]
    if !exists {
        return instanceNames
    }

    for name := range service.Instances {
        instanceNames = append(instanceNames, name)
    }

    return instanceNames
}