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

func (p *ProjectConfig) AddAccess(
	serviceName string,
	instanceName string,
	targetService string,
	resourceName string,
	permissions []string,
) error {

	service, ok := p.Services[serviceName]
	if !ok {
		return fmt.Errorf("service %s not found", serviceName)
	}

	instance, ok := service.Instances[instanceName]
	if !ok {
		return fmt.Errorf("instance %s not found", instanceName)
	}

	if instance.Access == nil {
		instance.Access = make(map[string]map[string][]string)
	}

	if instance.Access[targetService] == nil {
		instance.Access[targetService] = make(map[string][]string)
	}

	instance.Access[targetService][resourceName] = permissions

	return nil
}