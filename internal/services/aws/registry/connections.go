package registry

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	projectconfig "github.com/NipulM/oisbase/internal/config"
)

func PromptForConnections(serviceName string) (map[string]map[string][]string, error) {
    // 1. Get potential targets from registry (e.g., ["dynamodb", "s3"])
    potentialTargets := GetAvailableConnections(serviceName)

    projectCfg, err := projectconfig.LoadConfig()
    if err != nil {
        return nil, fmt.Errorf("failed to load project config: %w", err)
    }

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
        return nil, nil
    }

    // 3. Ask which Service Types to connect to (Single Question)
    var selectedTypes []string
    err = survey.AskOne(&survey.MultiSelect{
        Message: "Select service types this instance needs to access:",
        Options: validOptions,
        Help:    "Only services with existing instances in your project are shown.",
    }, &selectedTypes)
    if err != nil {
        return nil, err
    }

    accessControl := make(map[string]map[string][]string)

    // 4. Loop through selected types
    for _, svcType := range selectedTypes {
        instances := projectCfg.GetServiceInstances(svcType)
        
        // This won't be empty because of our filter in step 2
        var selectedInstances []string
        err = survey.AskOne(&survey.MultiSelect{
            Message: fmt.Sprintf("Which %s instances should be accessible?", svcType),
            Options: instances,
        }, &selectedInstances)
        if err != nil {
            return nil, err
        }

        permTemplate, ok := GetPermissionTemplate(serviceName, svcType)
        if !ok {
            return nil, fmt.Errorf("no permission template found for %s <-> %s", serviceName, svcType)
        }
        instanceMap := make(map[string][]string)

        for _, instName := range selectedInstances {
            var chosenPerms []string
            err = survey.AskOne(&survey.MultiSelect{
                Message: fmt.Sprintf("Access level for %s (%s):", instName, svcType),
                Options: permTemplate.SupportedAccessLevels,
            }, &chosenPerms)
            if err != nil {
                return nil, err
            }

            instanceMap[instName] = chosenPerms
        }
        
        accessControl[svcType] = instanceMap
    }

    return accessControl, nil
}