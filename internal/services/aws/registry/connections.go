package registry

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	projectconfig "github.com/NipulM/oisbase/internal/config"
)

func PromptForConnections(serviceName, instanceName string, projectCfg *projectconfig.ProjectConfig) ([]AffectedInstance, error) {
	potentialTargets := GetAvailableConnections(serviceName)

	var validOptions []string
	for _, targetType := range potentialTargets {
		if len(projectCfg.GetServiceInstances(targetType)) > 0 {
			validOptions = append(validOptions, targetType)
		}
	}

	if len(validOptions) == 0 {
		fmt.Println("ℹ️ No existing instances found to connect to.")
		return nil, nil
	}

	var selectedTypes []string
	err := survey.AskOne(&survey.MultiSelect{
		Message: "Select service types this instance needs to access:",
		Options: validOptions,
		Help:    "Only services with existing instances in your project are shown.",
	}, &selectedTypes)
	if err != nil {
		return nil, err
	}

	var affected []AffectedInstance

	for _, selectedType := range selectedTypes {
		permTemplate, ok := GetPermissionTemplate(serviceName, selectedType)
		if !ok {
			return nil, fmt.Errorf("no permission template found for %s <-> %s", serviceName, selectedType)
		}

		switch permTemplate.GetMode() {
		case RouteMode:
			a, err := promptRouteConnection(serviceName, instanceName, selectedType, permTemplate, projectCfg)
			if err != nil {
				return nil, err
			}
			affected = append(affected, a...)
		default:
			a, err := promptPermissionConnection(serviceName, instanceName, selectedType, permTemplate, projectCfg)
			if err != nil {
				return nil, err
			}
			affected = append(affected, a...)
		}
	}

	if err := projectconfig.SaveConfig(projectCfg); err != nil {
		return nil, err
	}
	return affected, nil
}

// promptRouteConnection handles route-based connections.
// Routes and access are ALWAYS stored on the SOURCE side of the registry key.
func promptRouteConnection(
	serviceName, instanceName, selectedType string,
	permTemplate PermissionTemplate,
	projectCfg *projectconfig.ProjectConfig,
) ([]AffectedInstance, error) {
	var affected []AffectedInstance

	sourceService, targetService, currentIsSource, found := ResolveDirection(serviceName, selectedType)
	if !found {
		return nil, fmt.Errorf("could not resolve direction for %s <-> %s", serviceName, selectedType)
	}

	var sourceInstances, targetInstances []string
	var err error

	if currentIsSource {
		sourceInstances = []string{instanceName}
		targetInstances, err = promptInstances(targetService, projectCfg)
	} else {
		targetInstances = []string{instanceName}
		sourceInstances, err = promptInstances(sourceService, projectCfg)
	}
	if err != nil {
		return nil, err
	}

	for _, srcInst := range sourceInstances {
		for _, tgtInst := range targetInstances {
			integrationName := toIntegrationName(tgtInst)

			fmt.Printf("\n📡 Configuring routes on %s (%s) → %s (%s)\n", srcInst, sourceService, tgtInst, targetService)

			for {
				route, err := promptSingleRoute(tgtInst, integrationName, permTemplate.SupportedHTTPMethods)
				if err != nil {
					return nil, err
				}
				route.TargetService = targetService
				route.TargetInstance = tgtInst

				if err := projectCfg.AddInstanceRoute(sourceService, srcInst, route); err != nil {
					return nil, fmt.Errorf("failed to add route: %w", err)
				}

				var addMore bool
				survey.AskOne(&survey.Confirm{
					Message: "Add another route for " + tgtInst + "?",
					Default: false,
				}, &addMore)
				if !addMore {
					break
				}
			}

			// Expand all permissions from ActionMap
			var expandedPerms []string
			for _, actions := range permTemplate.ActionMap {
				expandedPerms = append(expandedPerms, actions...)
			}

			if err := projectCfg.AddInstanceAccess(sourceService, srcInst, targetService, tgtInst, expandedPerms); err != nil {
				return nil, fmt.Errorf("failed to add access: %w", err)
			}

			if sourceService != serviceName {
				affected = append(affected, AffectedInstance{ServiceType: sourceService, InstanceName: srcInst})
			}
		}
	}

	return affected, nil
}

func promptSingleRoute(targetInst, integrationName string, supportedMethods []string) (projectconfig.Route, error) {
	var path string
	err := survey.AskOne(&survey.Input{
		Message: "Route path (e.g., /users, /orders/{id}):",
	}, &path, survey.WithValidator(survey.Required))
	if err != nil {
		return projectconfig.Route{}, err
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	var method string
	err = survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("HTTP method for %s:", path),
		Options: supportedMethods,
		Default: "GET",
	}, &method)
	if err != nil {
		return projectconfig.Route{}, err
	}

	return projectconfig.Route{
		Path:            path,
		Method:          method,
		OperationID:     generateOperationID(method, path),
		Summary:         generateSummary(method, path),
		Description:     generateDescription(method, path),
		IntegrationName: integrationName,
	}, nil
}

// promptPermissionConnection — unchanged from original.
func promptPermissionConnection(
	serviceName, instanceName, selectedType string,
	permTemplate PermissionTemplate,
	projectCfg *projectconfig.ProjectConfig,
) ([]AffectedInstance, error) {
	var affected []AffectedInstance

	sourceService, targetService, currentIsSource, found := ResolveDirection(serviceName, selectedType)
	if !found {
		return nil, fmt.Errorf("could not resolve direction for %s <-> %s", serviceName, selectedType)
	}

	var sourceInstances, targetInstances []string
	var err error

	if currentIsSource {
		sourceInstances = []string{instanceName}
		targetInstances, err = promptInstances(targetService, projectCfg)
	} else {
		targetInstances = []string{instanceName}
		sourceInstances, err = promptInstances(sourceService, projectCfg)
	}
	if err != nil {
		return nil, err
	}

	for _, srcInst := range sourceInstances {
		for _, tgtInst := range targetInstances {
			var chosenPerms []string
			prompt := &survey.MultiSelect{
				Message: fmt.Sprintf("Access level for %s (%s) → %s:", tgtInst, targetService, srcInst),
				Options: permTemplate.SupportedAccessLevels,
			}
			if permTemplate.DefaultAccessLevel != "" {
				prompt.Default = []string{permTemplate.DefaultAccessLevel}
			}
			if err := survey.AskOne(prompt, &chosenPerms, survey.WithValidator(survey.Required)); err != nil {
				return nil, err
			}

			var expandedPerms []string
			for _, level := range chosenPerms {
				if actions, ok := permTemplate.ActionMap[level]; ok {
					expandedPerms = append(expandedPerms, actions...)
				}
			}

			if permTemplate.Source.NeedsUpdate() {
				if err := projectCfg.AddInstanceAccess(sourceService, srcInst, targetService, tgtInst, expandedPerms); err != nil {
					return nil, fmt.Errorf("failed to add access on source: %w", err)
				}
				if sourceService != serviceName {
					affected = append(affected, AffectedInstance{ServiceType: sourceService, InstanceName: srcInst})
				}
			}

			if permTemplate.Target.NeedsUpdate() {
				if err := projectCfg.AddInstanceAccess(targetService, tgtInst, sourceService, srcInst, expandedPerms); err != nil {
					return nil, fmt.Errorf("failed to add access on target: %w", err)
				}
				if targetService != serviceName {
					affected = append(affected, AffectedInstance{ServiceType: targetService, InstanceName: tgtInst})
				}
			}
		}
	}

	return affected, nil
}

func promptInstances(serviceType string, projectCfg *projectconfig.ProjectConfig) ([]string, error) {
	instances := projectCfg.GetServiceInstances(serviceType)
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances found for %s", serviceType)
	}
	var selected []string
	err := survey.AskOne(&survey.MultiSelect{
		Message: fmt.Sprintf("Which %s instances?", serviceType),
		Options: instances,
	}, &selected)
	return selected, err
}

// --- Helpers ---

func toIntegrationName(instanceName string) string {
	parts := strings.Split(instanceName, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "") + "Integration"
}

func generateOperationID(method, path string) string {
	clean := strings.TrimPrefix(path, "/")
	segments := strings.Split(clean, "/")
	var parts []string
	for _, seg := range segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			param := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
			parts = append(parts, "By"+strings.ToUpper(param[:1])+param[1:])
		} else if seg != "" {
			parts = append(parts, strings.ToUpper(seg[:1])+seg[1:])
		}
	}
	methodLower := strings.ToLower(method)
	if len(parts) == 0 {
		return methodLower + "Root"
	}
	return methodLower + strings.Join(parts, "")
}

func generateSummary(method, path string) string {
	clean := strings.TrimPrefix(path, "/")
	clean = strings.ReplaceAll(clean, "/", " ")
	clean = strings.ReplaceAll(clean, "{", "")
	clean = strings.ReplaceAll(clean, "}", "")
	clean = strings.ReplaceAll(clean, "-", " ")
	methodTitle := strings.ToUpper(method[:1]) + strings.ToLower(method[1:])
	if clean == "" {
		return methodTitle + " root"
	}
	return methodTitle + " " + clean
}

func generateDescription(method, path string) string {
	return method + " " + path + " endpoint"
}