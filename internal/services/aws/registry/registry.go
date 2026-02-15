package registry

import "strings"

// AffectedInstance represents a service instance whose TF files need
// regenerating because a cross-service connection modified its config.
type AffectedInstance struct {
	ServiceType  string
	InstanceName string
}

// ServiceTemplateConfig defines what templates a particular side of a
// connection needs. If a pointer is nil, that template type is not needed.
// The system uses NeedsUpdate() to decide whether TF files must be
// (re)generated for that side.
type ServiceTemplateConfig struct {
	Category     string           // SSM path category, e.g. "databases", "compute"
	DataTemplate *DataTemplateConfig
	IAMTemplate  *IAMTemplateConfig
}

// NeedsUpdate returns true if this side has any templates that require generation.
func (s ServiceTemplateConfig) NeedsUpdate() bool {
	return s.DataTemplate != nil || s.IAMTemplate != nil
}

type DataTemplateConfig struct {
	TemplatePath  string // Path to the data.tf template
	ParameterName string // e.g., "{{ .target_instance }}_table_arn"
	SSMPath       string // e.g., "/databases/{{ .target_instance }}/arn"
}

type IAMTemplateConfig struct {
	TemplatePath    string   // Path to the IAM policy template
	PolicyName      string   // e.g., "{{ .source_instance }}_{{ .target_service }}_policy"
	ResourcePattern []string // e.g., ["data.aws_ssm_parameter.{{ .param_name }}.value"]
}

type PermissionTemplate struct {
	SupportedAccessLevels []string
	DefaultAccessLevel    string // Pre-selected option in the access level prompt
	ActionMap             map[string][]string

	// Source is the left side of "X-to-Y", Target is the right side.
	// Each side independently declares what templates it needs.
	Source ServiceTemplateConfig
	Target ServiceTemplateConfig
}

var PermissionRegistry = map[string]PermissionTemplate{
	"lambda-to-dynamodb": {
		SupportedAccessLevels: []string{"Read", "Write", "Delete", "All"},
		DefaultAccessLevel:    "Read",
		ActionMap: map[string][]string{
			"Read":   {"dynamodb:GetItem", "dynamodb:Scan", "dynamodb:Query"},
			"Write":  {"dynamodb:PutItem", "dynamodb:UpdateItem"},
			"Delete": {"dynamodb:DeleteItem"},
			"All":    {"dynamodb:*"},
		},
		Source: ServiceTemplateConfig{
			Category: "compute",
			DataTemplate: &DataTemplateConfig{
				TemplatePath:  "common/data-ssm-parameter.tf.tmpl",
				ParameterName: "{{ .target_instance }}_table_arn",
				SSMPath:       "/databases/{{ .target_instance }}/arn",
			},
			IAMTemplate: &IAMTemplateConfig{
				TemplatePath: "common/iam-policy.tf.tmpl",
				PolicyName:   "{{ .source_instance }}_dynamodb_policy",
				ResourcePattern: []string{
					"data.aws_ssm_parameter.{{ .param_name }}.value",
				},
			},
		},
		Target: ServiceTemplateConfig{
			Category: "databases",
		},
	},
	"lambda-to-s3": {
		SupportedAccessLevels: []string{"Read", "Write", "Delete", "All"},
		DefaultAccessLevel:    "Read",
		ActionMap: map[string][]string{
			"Read":   {"s3:GetObject", "s3:ListBucket"},
			"Write":  {"s3:PutObject", "s3:PutObjectAcl"},
			"Delete": {"s3:DeleteObject"},
			"All":    {"s3:*"},
		},
		Source: ServiceTemplateConfig{
			Category: "compute",
			DataTemplate: &DataTemplateConfig{
				TemplatePath:  "common/data-ssm-parameter.tf.tmpl",
				ParameterName: "{{ .target_instance }}_bucket_arn",
				SSMPath:       "/storage/{{ .target_instance }}/arn",
			},
			IAMTemplate: &IAMTemplateConfig{
				TemplatePath: "common/iam-policy.tf.tmpl",
				PolicyName:   "{{ .source_instance }}_s3_policy",
				ResourcePattern: []string{
					"data.aws_ssm_parameter.{{ .param_name }}.value",
				},
			},
		},
		Target: ServiceTemplateConfig{
			Category: "storage",
			// S3 doesn't need any TF files generated on its side
		},
	},
}

// GetAvailableConnections returns service types that the given service can
// connect to, based on registry entries in either direction.
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

// GetPermissionTemplate finds the registry entry for a pair of services,
// trying both key directions.
func GetPermissionTemplate(serviceA, serviceB string) (PermissionTemplate, bool) {
	if tmpl, ok := PermissionRegistry[serviceA+"-to-"+serviceB]; ok {
		return tmpl, true
	}
	if tmpl, ok := PermissionRegistry[serviceB+"-to-"+serviceA]; ok {
		return tmpl, true
	}
	return PermissionTemplate{}, false
}

// ResolveDirection determines which side of the registry key the given
// service is on. Returns (sourceService, targetService, currentIsSource).
func ResolveDirection(currentService, otherService string) (sourceService, targetService string, currentIsSource bool, found bool) {
	if _, ok := PermissionRegistry[currentService+"-to-"+otherService]; ok {
		return currentService, otherService, true, true
	}
	if _, ok := PermissionRegistry[otherService+"-to-"+currentService]; ok {
		return otherService, currentService, false, true
	}
	return "", "", false, false
}

// GetTemplatesForService returns the templates that apply to serviceType
// when it is connected to otherService, plus the other side's category.
// This lets the generator know which templates to use without caring about
// key direction.
func GetTemplatesForService(serviceType, otherService string) (mine ServiceTemplateConfig, otherCategory string, ok bool) {
	if tmpl, found := PermissionRegistry[serviceType+"-to-"+otherService]; found {
		// serviceType is Source
		return tmpl.Source, tmpl.Target.Category, true
	}
	if tmpl, found := PermissionRegistry[otherService+"-to-"+serviceType]; found {
		// serviceType is Target
		return tmpl.Target, tmpl.Source.Category, true
	}
	return ServiceTemplateConfig{}, "", false
}
