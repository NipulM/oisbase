package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/NipulM/oisbase/internal/config"
	"github.com/NipulM/oisbase/internal/services/aws/registry"
	"github.com/NipulM/oisbase/internal/services/aws/templates"
)

// environmentGroup maps an environment name to its directory group.
func environmentGroup(env string) string {
	if env == "prod" {
		return "production"
	}
	return "pre-production"
}

type IAMPolicyGenerator struct {
	config *config.ProjectConfig
}

type StatementGroup struct {
	TargetService  string
	TargetInstance string
	Actions        []string
	ParamName      string
}

func NewIAMPolicyGenerator(cfg *config.ProjectConfig) *IAMPolicyGenerator {
	return &IAMPolicyGenerator{config: cfg}
}

// UpdateAffectedInstances regenerates IAM/data.tf files for service instances
// that were modified as a side-effect of creating a different service.
func (g *IAMPolicyGenerator) UpdateAffectedInstances(
	affected []registry.AffectedInstance,
	environments []string,
) error {
	for _, env := range environments {
		group := environmentGroup(env)
		for _, inst := range affected {
			instanceDir := filepath.Join("environments", group, env, inst.ServiceType, inst.InstanceName)
			if err := g.GenerateIAMPolicies(inst.ServiceType, inst.InstanceName, env, instanceDir); err != nil {
				return fmt.Errorf("failed to update IAM for %s/%s in %s: %w",
					inst.ServiceType, inst.InstanceName, env, err)
			}
		}
	}
	return nil
}

// GenerateIAMPolicies generates IAM policies for a specific instance
func (g *IAMPolicyGenerator) GenerateIAMPolicies(
	serviceType string,
	instanceName string,
	environment string,
	instanceDir string, // e.g., "environments/pre-production/dev/lambda/auth-service"
) error {
	service, exists := g.config.Services[serviceType]
	if !exists {
		return fmt.Errorf("service %s not found", serviceType)
	}

	instance, exists := service.Instances[instanceName]
	if !exists {
		return fmt.Errorf("instance %s not found in service %s", instanceName, serviceType)
	}

	// Skip if no access defined
	if instance.Access == nil {
		return nil
	}

	var groups []StatementGroup

	// Iterate through all access definitions
	for targetService, instances := range instance.Access {
		for targetInstance, permissions := range instances {
			groups = append(groups, StatementGroup{
				TargetService:  targetService,
				TargetInstance: targetInstance,
				Actions:        permissions,
				ParamName:      fmt.Sprintf("%s_%s_arn", targetInstance, targetService),
			})
		}
	}

	// Generate data.tf entries
	if err := g.generateDataTf(serviceType, groups, instanceDir); err != nil {
		return fmt.Errorf("failed to generate data.tf: %w", err)
	}

	// Generate IAM policy
	if err := g.generateIAMPolicy(serviceType, instanceName, environment, groups, instanceDir); err != nil {
		return fmt.Errorf("failed to generate IAM policy: %w", err)
	}

	return nil
}

func (g *IAMPolicyGenerator) generateDataTf(
	serviceType string,
	groups []StatementGroup,
	instanceDir string,
) error {
	dataPath := filepath.Join(instanceDir, "data.tf")

	// Read existing data.tf if it exists
	existingContent := ""
	if content, err := os.ReadFile(dataPath); err == nil {
		existingContent = string(content)
	}

	// Load template
	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).ParseFS(
		templates.CommonFS,
		"common/data-ssm-parameter.tf.tmpl",
	)
	if err != nil {
		return fmt.Errorf("failed to parse data template: %w", err)
	}

	var newEntries strings.Builder

	for _, group := range groups {
		// Use GetTemplatesForService to get the templates for this service
		// and the other side's category (for SSM path)
		myTemplates, otherCategory, ok := registry.GetTemplatesForService(serviceType, group.TargetService)
		if !ok || myTemplates.DataTemplate == nil {
			continue
		}

		paramName := fmt.Sprintf("%s_%s_arn", group.TargetInstance, group.TargetService)

		// Check if this parameter already exists
		if strings.Contains(existingContent, fmt.Sprintf(`"%s"`, paramName)) {
			continue
		}

		// Generate SSM path using the other side's category
		ssmPath := fmt.Sprintf("/%s/%s/arn", otherCategory, group.TargetInstance)

		var buf bytes.Buffer
		err = tmpl.ExecuteTemplate(&buf, "data-ssm-parameter.tf.tmpl", map[string]string{
			"parameter_name": paramName,
			"ssm_path":       ssmPath,
		})
		if err != nil {
			return fmt.Errorf("failed to execute data template: %w", err)
		}

		newEntries.WriteString(buf.String())
		newEntries.WriteString("\n")
	}

	// Append new entries to existing content
	if newEntries.Len() > 0 {
		finalContent := existingContent + "\n" + newEntries.String()
		if err := os.WriteFile(dataPath, []byte(finalContent), 0644); err != nil {
			return fmt.Errorf("failed to write data.tf: %w", err)
		}
	}

	return nil
}

func (g *IAMPolicyGenerator) generateIAMPolicy(
	serviceType string,
	instanceName string,
	environment string,
	groups []StatementGroup,
	instanceDir string,
) error {
	iamPath := filepath.Join(instanceDir, "iam.tf")

	// Read existing iam.tf
	existingContent := ""
	if content, err := os.ReadFile(iamPath); err == nil {
		existingContent = string(content)
	}

	// Build statements for the policy
	type PolicyStatement struct {
		Actions   []string
		Resources []string
		Last      bool
	}

	var statements []PolicyStatement
	for i, group := range groups {
		paramName := fmt.Sprintf("%s_%s_arn", group.TargetInstance, group.TargetService)

		stmt := PolicyStatement{
			Actions: group.Actions,
			Resources: []string{
				fmt.Sprintf("data.aws_ssm_parameter.%s.value", paramName),
			},
			Last: i == len(groups)-1,
		}

		statements = append(statements, stmt)
	}

	// Load template
	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).Funcs(template.FuncMap{
		// toJson quotes values — use for Action strings like "dynamodb:GetItem"
		"toJson": func(v interface{}) string {
			switch val := v.(type) {
			case []string:
				if len(val) == 1 {
					return fmt.Sprintf(`"%s"`, val[0])
				}
				quoted := make([]string, len(val))
				for i, s := range val {
					quoted[i] = fmt.Sprintf(`"%s"`, s)
				}
				return "[" + strings.Join(quoted, ", ") + "]"
			default:
				return ""
			}
		},
		// toRef outputs bare references — use for Terraform expressions like data.aws_ssm_parameter.x.value
		"toRef": func(v interface{}) string {
			switch val := v.(type) {
			case []string:
				if len(val) == 1 {
					return val[0]
				}
				return "[" + strings.Join(val, ", ") + "]"
			default:
				return ""
			}
		},
	}).ParseFS(templates.CommonFS, "common/iam-policy.tf.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse IAM template: %w", err)
	}

	targetLabel := "access"
	nameVar := serviceType + "_name" // fallback convention
	if len(groups) > 0 {
		first := groups[0].TargetService
		allSame := true
		for _, g := range groups[1:] {
			if g.TargetService != first {
				allSame = false
				break
			}
		}
		if allSame {
			targetLabel = first
			if myTemplates, _, ok := registry.GetTemplatesForService(serviceType, first); ok && myTemplates.NameVar != "" {
				nameVar = myTemplates.NameVar
			}
		}
	}
	policyName := fmt.Sprintf("%s_%s_policy", serviceType, targetLabel)

	roleName := fmt.Sprintf("%s_role", serviceType)

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "iam-policy.tf.tmpl", map[string]interface{}{
		"policy_name":     policyName,
		"source_instance": instanceName,
		"target_label":    targetLabel,
		"name_var":        nameVar,
		"role_name":       roleName,
		"statements":      statements,
	})
	if err != nil {
		return fmt.Errorf("failed to execute IAM template: %w", err)
	}

	// Check if policy already exists
	if strings.Contains(existingContent, fmt.Sprintf(`"%s"`, policyName)) {
		return nil // Policy already exists
	}

	// Append to iam.tf
	finalContent := existingContent + "\n" + buf.String()
	if err := os.WriteFile(iamPath, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write iam.tf: %w", err)
	}

	return nil
}
