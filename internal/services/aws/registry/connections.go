package registry

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	projectconfig "github.com/NipulM/oisbase/internal/config"
)

func PromptForConnections(serviceName, instanceName string, projectCfg *projectconfig.ProjectConfig) error {
	// 1. Get potential targets from registry (e.g., ["dynamodb", "s3"])
	potentialTargets := GetAvailableConnections(serviceName)

	// REMOVED: Don't load config here, use the one passed in
	// projectCfg, err := projectconfig.LoadConfig()

	// 2. Filter options: Must be a potential target AND have at least 1 instance
	var validOptions []string
	for _, targetType := range potentialTargets {
		instances := projectCfg.GetServiceInstances(targetType)
		if len(instances) > 0 {
			validOptions = append(validOptions, targetType)
		}
	}

	// If no instances exist anywhere, exit early
	if len(validOptions) == 0 {
		fmt.Println("ℹ️ No existing instances (DynamoDB, S3, etc.) found to connect to.")
		return nil
	}

	// 3. Ask which Service Types to connect to (Single Question)
	var selectedTypes []string
	err := survey.AskOne(&survey.MultiSelect{
		Message: "Select service types this instance needs to access:",
		Options: validOptions,
		Help:    "Only services with existing instances in your project are shown.",
	}, &selectedTypes)
	if err != nil {
		return err
	}

	// 4. Loop through selected types
	for _, targetType := range selectedTypes {
		permTemplate, ok := GetPermissionTemplate(serviceName, targetType)
		if !ok {
			return fmt.Errorf("no permission template found for %s <-> %s", serviceName, targetType)
		}

		// Determine which service gets updated
		serviceTypeToUpdate, targetServiceType, found := GetUpdateDirection(serviceName, targetType)
		if !found {
			return fmt.Errorf("could not determine update direction for %s <-> %s", serviceName, targetType)
		}

		// Determine which instance gets updated
		var instanceToUpdate string
		if serviceTypeToUpdate == serviceName {
			// Current service gets updated, use the instance we're creating
			instanceToUpdate = instanceName
		} else {
			// The OTHER service gets updated, need to ask which instance
			otherInstances := projectCfg.GetServiceInstances(serviceTypeToUpdate)
			if len(otherInstances) == 0 {
				return fmt.Errorf("no instances found for %s to update", serviceTypeToUpdate)
			}

			err = survey.AskOne(&survey.Select{
				Message: fmt.Sprintf("Which %s instance should get access?", serviceTypeToUpdate),
				Options: otherInstances,
			}, &instanceToUpdate)
			if err != nil {
				return err
			}
		}

		// Get instances of the target service type
		targetInstances := projectCfg.GetServiceInstances(targetServiceType)

		var selectedInstances []string
		err = survey.AskOne(&survey.MultiSelect{
			Message: fmt.Sprintf("Which %s instances should be accessible?", targetServiceType),
			Options: targetInstances,
		}, &selectedInstances)
		if err != nil {
			return err
		}

		// For each selected instance, ask for permissions
		for _, targetInstanceName := range selectedInstances {
			var chosenPerms []string
			err = survey.AskOne(&survey.MultiSelect{
				Message: fmt.Sprintf("Access level for %s (%s):", targetInstanceName, targetServiceType),
				Options: permTemplate.SupportedAccessLevels,
			}, &chosenPerms)
			if err != nil {
				return err
			}

			// Expand permissions based on ActionMap
			var expandedPerms []string
			for _, level := range chosenPerms {
				if actions, ok := permTemplate.ActionMap[level]; ok {
					expandedPerms = append(expandedPerms, actions...)
				}
			}

			// Save to config
			err = projectCfg.AddInstanceAccess(
				serviceTypeToUpdate,
				instanceToUpdate,   
				targetServiceType,  
				targetInstanceName, 
				expandedPerms,      
			)

			if err != nil {
				return fmt.Errorf("failed to add access: %w", err)
			}
		}
	}

	// Save the updated config ONCE at the end
	return projectconfig.SaveConfig(projectCfg)
}