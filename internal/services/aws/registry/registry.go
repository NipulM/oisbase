package registry

import "strings"

type AffectedInstance struct {
	ServiceType  string
	InstanceName string
}

type ConnectionMode string

const (
	PermissionMode ConnectionMode = "permission"
	RouteMode      ConnectionMode = "route"
)

type ServiceTemplateConfig struct {
	Category string
	NameVar  string

	// Append-based flow (PermissionMode)
	DataTemplate *DataTemplateConfig
	IAMTemplate  *IAMTemplateConfig

	// Re-render flow — when set, the generator re-renders these instance
	// template files instead of using the append-based flow.
	InstanceRegenTemplates []RegenTemplateConfig
}

func (s ServiceTemplateConfig) NeedsUpdate() bool {
	return s.DataTemplate != nil || s.IAMTemplate != nil || len(s.InstanceRegenTemplates) > 0
}

func (s ServiceTemplateConfig) NeedsRegen() bool {
	return len(s.InstanceRegenTemplates) > 0
}

type DataTemplateConfig struct {
	TemplatePath  string
	ParameterName string
	SSMPath       string
}

type IAMTemplateConfig struct {
	TemplatePath string
}

type RegenTemplateConfig struct {
	TemplateName string // e.g., "data.tf.tmpl"
	OutputFile   string // e.g., "data.tf"
}

// ConnectedTargetSSMConfig declares what SSM params each connected target needs.
type ConnectedTargetSSMConfig struct {
	Suffix string // "arn", "name", etc.
}

type PermissionTemplate struct {
	Mode ConnectionMode

	SupportedAccessLevels []string
	DefaultAccessLevel    string
	ActionMap             map[string][]string

	SupportedHTTPMethods []string

	// Declares what SSM lookups each connected target needs (for re-render flow).
	ConnectedTargetSSM []ConnectedTargetSSMConfig

	// Embedded FS glob for the service's instance templates (for re-render flow).
	TemplateFS string

	Source ServiceTemplateConfig
	Target ServiceTemplateConfig
}

func (p PermissionTemplate) GetMode() ConnectionMode {
	if p.Mode == "" {
		return PermissionMode
	}
	return p.Mode
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
			NameVar:  "lambda_name",
			DataTemplate: &DataTemplateConfig{
				TemplatePath:  "common/data-ssm-parameter.tf.tmpl",
				ParameterName: "{{ .target_instance }}_table_arn",
				SSMPath:       "/databases/{{ .target_instance }}/arn",
			},
			IAMTemplate: &IAMTemplateConfig{
				TemplatePath: "common/iam-policy.tf.tmpl",
			},
		},
		Target: ServiceTemplateConfig{
			Category: "databases",
			NameVar:  "table_name",
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
			NameVar:  "lambda_name",
			DataTemplate: &DataTemplateConfig{
				TemplatePath:  "common/data-ssm-parameter.tf.tmpl",
				ParameterName: "{{ .target_instance }}_bucket_arn",
				SSMPath:       "/storage/{{ .target_instance }}/arn",
			},
			IAMTemplate: &IAMTemplateConfig{
				TemplatePath: "common/iam-policy.tf.tmpl",
			},
		},
		Target: ServiceTemplateConfig{
			Category: "storage",
			NameVar:  "bucket_name",
		},
	},
	"api-gateway-to-lambda": {
		Mode:                 RouteMode,
		SupportedHTTPMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		ActionMap: map[string][]string{
			"invoke": {"lambda:InvokeFunction"},
		},
		ConnectedTargetSSM: []ConnectedTargetSSMConfig{
			{Suffix: "arn"},
			{Suffix: "name"},
		},
		TemplateFS: "api-gw/http/*.tmpl",
		Source: ServiceTemplateConfig{
			Category: "network",
			NameVar:  "api_name",
			InstanceRegenTemplates: []RegenTemplateConfig{
				{TemplateName: "main.tf.tmpl", OutputFile: "main.tf"},
				{TemplateName: "data.tf.tmpl", OutputFile: "data.tf"},
				{TemplateName: "iam.tf.tmpl", OutputFile: "iam.tf"},
				{TemplateName: "api.yaml.tmpl", OutputFile: "api.yaml"},
			},
		},
		Target: ServiceTemplateConfig{
			Category: "compute",
			NameVar:  "lambda_name",
		},
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

func ResolveDirection(currentService, otherService string) (sourceService, targetService string, currentIsSource bool, found bool) {
	if _, ok := PermissionRegistry[currentService+"-to-"+otherService]; ok {
		return currentService, otherService, true, true
	}
	if _, ok := PermissionRegistry[otherService+"-to-"+currentService]; ok {
		return otherService, currentService, false, true
	}
	return "", "", false, false
}

func GetTemplatesForService(serviceType, otherService string) (mine ServiceTemplateConfig, otherCategory string, ok bool) {
	if tmpl, found := PermissionRegistry[serviceType+"-to-"+otherService]; found {
		return tmpl.Source, tmpl.Target.Category, true
	}
	if tmpl, found := PermissionRegistry[otherService+"-to-"+serviceType]; found {
		return tmpl.Target, tmpl.Source.Category, true
	}
	return ServiceTemplateConfig{}, "", false
}