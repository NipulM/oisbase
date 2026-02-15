package registry

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	projectconfig "github.com/NipulM/oisbase/internal/config"
)

// PromptForConnections asks the user to configure connections between the
// service being created and other existing services. It returns a list of
// AffectedInstance entries — instances of OTHER services whose TF files
// need regenerating because their config was updated.
func PromptForConnections(serviceName, instanceName string, projectCfg *projectconfig.ProjectConfig) ([]AffectedInstance, error) {
	// 1. Get potential targets from registry (e.g., ["dynamodb", "s3"])
	potentialTargets := GetAvailableConnections(serviceName)

	// 2. Filter: must have at least 1 existing instance
	var validOptions []string
	for _, targetType := range potentialTargets {
		instances := projectCfg.GetServiceInstances(targetType)
		if len(instances) > 0 {
			validOptions = append(validOptions, targetType)
		}
	}

	if len(validOptions) == 0 {
		fmt.Println("ℹ️ No existing instances (DynamoDB, S3, etc.) found to connect to.")
		return nil, nil
	}

	// 3. Ask which service types to connect to
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

	// 4. For each selected service type, configure the connection
	for _, selectedType := range selectedTypes {
		permTemplate, ok := GetPermissionTemplate(serviceName, selectedType)
		if !ok {
			return nil, fmt.Errorf("no permission template found for %s <-> %s", serviceName, selectedType)
		}

		// Determine which side of the key the current service is on
		sourceService, targetService, currentIsSource, found := ResolveDirection(serviceName, selectedType)
		if !found {
			return nil, fmt.Errorf("could not resolve direction for %s <-> %s", serviceName, selectedType)
		}

		// Determine source and target instances
		var sourceInstances, targetInstances []string

		if currentIsSource {
			// Current service is Source → we know the source instance
			sourceInstances = []string{instanceName}
			// Ask which target instances to connect to
			targetInstances, err = promptInstances(targetService, projectCfg)
			if err != nil {
				return nil, err
			}
		} else {
			// Current service is Target → we know the target instance
			targetInstances = []string{instanceName}
			// Ask which source instances to connect to
			sourceInstances, err = promptInstances(sourceService, projectCfg)
			if err != nil {
				return nil, err
			}
		}

		// For each source-target pair, ask for access levels and store access
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
				err = survey.AskOne(prompt, &chosenPerms, survey.WithValidator(survey.Required))
				if err != nil {
					return nil, err
				}

				// Expand permissions based on ActionMap
				var expandedPerms []string
				for _, level := range chosenPerms {
					if actions, ok := permTemplate.ActionMap[level]; ok {
						expandedPerms = append(expandedPerms, actions...)
					}
				}

				// Store access on each side that has templates defined
				if permTemplate.Source.NeedsUpdate() {
					err = projectCfg.AddInstanceAccess(
						sourceService, srcInst,
						targetService, tgtInst,
						expandedPerms,
					)
					if err != nil {
						return nil, fmt.Errorf("failed to add access on source: %w", err)
					}

					// If the source is NOT the current service, record as affected
					if sourceService != serviceName {
						affected = append(affected, AffectedInstance{
							ServiceType:  sourceService,
							InstanceName: srcInst,
						})
					}
				}

				if permTemplate.Target.NeedsUpdate() {
					err = projectCfg.AddInstanceAccess(
						targetService, tgtInst,
						sourceService, srcInst,
						expandedPerms,
					)
					if err != nil {
						return nil, fmt.Errorf("failed to add access on target: %w", err)
					}

					// If the target is NOT the current service, record as affected
					if targetService != serviceName {
						affected = append(affected, AffectedInstance{
							ServiceType:  targetService,
							InstanceName: tgtInst,
						})
					}
				}
			}
		}
	}

	// Save the updated config ONCE at the end
	if err := projectconfig.SaveConfig(projectCfg); err != nil {
		return nil, err
	}
	return affected, nil
}

// promptInstances asks the user to select instances of a given service type.
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
	if err != nil {
		return nil, err
	}
	return selected, nil
}
