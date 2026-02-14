package registry

import "strings"

type PermissionTemplate struct {
	Type 				   string 
	SupportedAccessLevels  []string
	ActionMap              map[string][]string
	ResourceSuffix         string
}

var PermissionRegistry = map[string]PermissionTemplate{
	"lambda-to-dynamodb": {
		Type: "Identity",
		SupportedAccessLevels: []string{"Read", "Write", "Delete", "All"},
		ActionMap: map[string][]string{
			"Read": {"dynamodb:GetItem", "dynamodb:Scan", "dynamodb:Query"},
			"Write": {"dynamodb:PutItem", "dynamodb:UpdateItem"},
			"Delete": {"dynamodb:DeleteItem"},
			"All": {"dynamodb:*"},
		},
		ResourceSuffix: "table-arn",
	},
	"s3-to-lambda": {
        Type: "Identity",
        SupportedAccessLevels: []string{"Read", "Write", "Delete", "All"},
        ActionMap: map[string][]string{
            "Read": {"s3:GetObject", "s3:ListBucket"},
            "Write": {"s3:PutObject", "s3:PutObjectAcl"},
            "Delete": {"s3:DeleteObject"},
            "All": {"s3:*"},
        },
        ResourceSuffix: "bucket-arn",
    },
}

func GetAvailableConnections(currentService string) []string {
    var options []string
    for key := range PermissionRegistry {
        // key format is "source-to-destination"
        parts := strings.Split(key, "-to-")
        if parts[0] == currentService {
            options = append(options, parts[1]) // Service can ACT on these
        }
        if parts[1] == currentService {
            options = append(options, parts[0]) // These services can ACT on this
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