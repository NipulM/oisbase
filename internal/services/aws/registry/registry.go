package registry

import "strings"

type PermissionTemplate struct {
	Type                  string
	SupportedAccessLevels []string
	ActionMap             map[string][]string
	ResourceSuffix        string
	UpdateSide            string // "source" or "target" - which side of the relationship gets updated
}

var PermissionRegistry = map[string]PermissionTemplate{
	"lambda-to-dynamodb": {
		Type:                  "Identity",
		UpdateSide:            "source", // source (lambda) gets updated
		SupportedAccessLevels: []string{"Read", "Write", "Delete", "All"},
		ActionMap: map[string][]string{
			"Read":   {"dynamodb:GetItem", "dynamodb:Scan", "dynamodb:Query"},
			"Write":  {"dynamodb:PutItem", "dynamodb:UpdateItem"},
			"Delete": {"dynamodb:DeleteItem"},
			"All":    {"dynamodb:*"},
		},
		ResourceSuffix: "table-arn",
	},
	"lambda-to-s3": {
		Type:                  "Identity",
		UpdateSide:            "source", // source (lambda) gets updated
		SupportedAccessLevels: []string{"Read", "Write", "Delete", "All"},
		ActionMap: map[string][]string{
			"Read":   {"s3:GetObject", "s3:ListBucket"},
			"Write":  {"s3:PutObject", "s3:PutObjectAcl"},
			"Delete": {"s3:DeleteObject"},
			"All":    {"s3:*"},
		},
		ResourceSuffix: "bucket-arn",
	},
}

func GetAvailableConnections(currentService string) []string {
	var options []string
	seen := make(map[string]bool)

	for key := range PermissionRegistry {
		parts := strings.Split(key, "-to-")
		if parts[0] == currentService && !seen[parts[1]] {
			options = append(options, parts[1])
			seen[parts[1]] = true
		}
		if parts[1] == currentService && !seen[parts[0]] {
			options = append(options, parts[0])
			seen[parts[0]] = true
		}
	}
	return options
}

func GetPermissionTemplate(serviceA, serviceB string) (PermissionTemplate, bool) {
	if tmpl, ok := PermissionRegistry[serviceA+"-to-"+serviceB]; ok {
		return tmpl, true
	}
	if tmpl, ok := PermissionRegistry[serviceB+"-to-"+serviceA]; ok {
		return tmpl, true
	}
	return PermissionTemplate{}, false
}

// Returns which service type should be updated and what the target service type is
func GetUpdateDirection(currentService, targetService string) (serviceTypeToUpdate, targetServiceType string, found bool) {
	// Try direct key first
	key := currentService + "-to-" + targetService
	tmpl, ok := PermissionRegistry[key]
	
	if ok {
		// currentService is source, targetService is target in the key
		if tmpl.UpdateSide == "source" {
			return currentService, targetService, true
		} else { // "target"
			return targetService, currentService, true
		}
	}

	// Try reverse key
	reverseKey := targetService + "-to-" + currentService
	tmpl, ok = PermissionRegistry[reverseKey]
	
	if ok {
		// targetService is source, currentService is target in the key
		if tmpl.UpdateSide == "source" {
			return targetService, currentService, true
		} else { // "target"
			return currentService, targetService, true
		}
	}

	return "", "", false
}