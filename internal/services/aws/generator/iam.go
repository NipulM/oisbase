package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/NipulM/oisbase/internal/config"
	"github.com/NipulM/oisbase/internal/services/aws/registry"
	"github.com/NipulM/oisbase/internal/services/aws/templates"
)

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

// UpdateAffectedInstances regenerates TF files for affected service instances.
// Uses the registry to decide: NeedsRegen() → re-render instance templates,
// otherwise → append-based IAM/data flow.
func (g *IAMPolicyGenerator) UpdateAffectedInstances(
	affected []registry.AffectedInstance,
	environments []string,
) error {
	for _, env := range environments {
		group := environmentGroup(env)
		for _, inst := range affected {
			instanceDir := filepath.Join("environments", group, env, inst.ServiceType, inst.InstanceName)

			if g.serviceNeedsRegen(inst.ServiceType) {
				if err := g.regenInstanceTemplates(inst.ServiceType, inst.InstanceName, env, instanceDir); err != nil {
					return fmt.Errorf("failed to regen %s/%s in %s: %w", inst.ServiceType, inst.InstanceName, env, err)
				}
			} else {
				if err := g.GenerateIAMPolicies(inst.ServiceType, inst.InstanceName, env, instanceDir); err != nil {
					return fmt.Errorf("failed to update IAM for %s/%s in %s: %w", inst.ServiceType, inst.InstanceName, env, err)
				}
			}
		}
	}
	return nil
}

// serviceNeedsRegen checks if any registry entry for this service type
// has InstanceRegenTemplates on its side.
func (g *IAMPolicyGenerator) serviceNeedsRegen(serviceType string) bool {
	service, exists := g.config.Services[serviceType]
	if !exists {
		return false
	}
	// Check any instance's access to find a registry entry
	for _, inst := range service.Instances {
		if inst.Access == nil {
			continue
		}
		for targetService := range inst.Access {
			myTemplates, _, ok := registry.GetTemplatesForService(serviceType, targetService)
			if ok && myTemplates.NeedsRegen() {
				return true
			}
		}
	}
	return false
}

// ---- Re-render flow ----

// ConnectedTarget represents a target instance generically.
// Templates iterate over these — the field values come from config + registry.
type ConnectedTarget struct {
	Name            string            // "marketplace-control-service"
	NameUnderscored string            // "marketplace_control_service"
	ServiceType     string            // "lambda"
	Category        string            // "compute"
	SSMParams       map[string]string // suffix → resource name, e.g. "arn" → "lambda_marketplace_control_service_arn"
}

// SSMParamResource returns the Terraform resource name for a given suffix.
// Used in templates: {{ .SSMParamResource "name" }}
func (ct ConnectedTarget) SSMParamResource(suffix string) string {
	return ct.SSMParams[suffix]
}

type RouteIntegration struct {
	Name   string // "MarketplaceControlServiceIntegration"
	URIVar string // "marketplace_control_service" (the connected target's underscored name)
}

type RouteMethodData struct {
	MethodLower     string
	Summary         string
	Description     string
	OperationID     string
	IntegrationName string
}

type RouteGroupData struct {
	Path    string
	Methods []RouteMethodData
}

func (g *IAMPolicyGenerator) regenInstanceTemplates(
	serviceType, instanceName, environment, instanceDir string,
) error {
	instance := g.config.Services[serviceType].Instances[instanceName]
	if instance == nil {
		return fmt.Errorf("instance %s/%s not found", serviceType, instanceName)
	}

	// Find registry config for this service
	var regenTemplates []registry.RegenTemplateConfig
	var templateFSGlob string

	for targetService := range instance.Access {
		myTemplates, _, ok := registry.GetTemplatesForService(serviceType, targetService)
		if !ok || !myTemplates.NeedsRegen() {
			continue
		}
		regenTemplates = myTemplates.InstanceRegenTemplates
		permTemplate, _ := registry.GetPermissionTemplate(serviceType, targetService)
		templateFSGlob = permTemplate.TemplateFS
		break
	}

	if len(regenTemplates) == 0 || templateFSGlob == "" {
		return nil
	}

	templateData := g.BuildRegenTemplateData(serviceType, instanceName, environment, instance)

	// Load templates from embedded FS
	fsToUse := templates.GetFS(templateFSGlob)
	if fsToUse == nil {
		return fmt.Errorf("no embedded FS found for glob %s", templateFSGlob)
	}

	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).ParseFS(fsToUse, templateFSGlob)
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	for _, rt := range regenTemplates {
		if tmpl.Lookup(rt.TemplateName) == nil {
			continue
		}
		var out bytes.Buffer
		if err := tmpl.ExecuteTemplate(&out, rt.TemplateName, templateData); err != nil {
			return fmt.Errorf("failed to execute %s: %w", rt.TemplateName, err)
		}
		if err := os.WriteFile(filepath.Join(instanceDir, rt.OutputFile), out.Bytes(), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", rt.OutputFile, err)
		}
	}

	fmt.Printf("  ✓ Updated %s/%s templates in %s\n", serviceType, instanceName, environment)
	return nil
}

// BuildRegenTemplateData builds template data generically from config + registry.
// Exported so service files (api_gateway.go) can reuse the same builder.
func (g *IAMPolicyGenerator) BuildRegenTemplateData(
	serviceType, instanceName, environment string,
	instance *config.Instance,
) map[string]interface{} {
	var connectedTargets []ConnectedTarget
	targetSeen := make(map[string]bool)

	if instance.Access != nil {
		for targetService, targetInstances := range instance.Access {
			permTemplate, _ := registry.GetPermissionTemplate(serviceType, targetService)
			_, targetCategory, _ := registry.GetTemplatesForService(serviceType, targetService)

			for targetInst := range targetInstances {
				key := targetService + "/" + targetInst
				if targetSeen[key] {
					continue
				}
				targetSeen[key] = true

				nameUnderscored := strings.ReplaceAll(targetInst, "-", "_")
				ct := ConnectedTarget{
					Name:            targetInst,
					NameUnderscored: nameUnderscored,
					ServiceType:     targetService,
					Category:        targetCategory,
					SSMParams:       make(map[string]string),
				}

				// Build SSM param resource names from registry's ConnectedTargetSSM
				for _, ssmCfg := range permTemplate.ConnectedTargetSSM {
					ct.SSMParams[ssmCfg.Suffix] = targetService + "_" + nameUnderscored + "_" + ssmCfg.Suffix
				}

				connectedTargets = append(connectedTargets, ct)
			}
		}
	}

	integrations, routeGroups := buildRouteData(instance.Routes)

	return map[string]interface{}{
		"api_name":             instanceName,
		"api_name_underscored": strings.ReplaceAll(instanceName, "-", "_"),
		"project_name":         g.config.ProjectName,
		"region":               g.config.Region,
		"environment":          environment,
		"connected_targets":    connectedTargets,
		"integrations":         integrations,
		"route_groups":         routeGroups,
	}
}

func buildRouteData(routes []config.Route) ([]RouteIntegration, []RouteGroupData) {
	if len(routes) == 0 {
		return nil, nil
	}

	integrationSet := make(map[string]bool)
	var integrations []RouteIntegration
	pathRoutes := make(map[string][]RouteMethodData)
	var pathOrder []string

	for _, r := range routes {
		if !integrationSet[r.IntegrationName] {
			integrationSet[r.IntegrationName] = true
			integrations = append(integrations, RouteIntegration{
				Name:   r.IntegrationName,
				URIVar: strings.ReplaceAll(r.TargetInstance, "-", "_"),
			})
		}
		if _, exists := pathRoutes[r.Path]; !exists {
			pathOrder = append(pathOrder, r.Path)
		}
		pathRoutes[r.Path] = append(pathRoutes[r.Path], RouteMethodData{
			MethodLower:     strings.ToLower(r.Method),
			Summary:         r.Summary,
			Description:     r.Description,
			OperationID:     r.OperationID,
			IntegrationName: r.IntegrationName,
		})
	}

	sort.Strings(pathOrder)
	var routeGroups []RouteGroupData
	for _, path := range pathOrder {
		routeGroups = append(routeGroups, RouteGroupData{Path: path, Methods: pathRoutes[path]})
	}
	return integrations, routeGroups
}

// ---- Append-based flow (unchanged) ----

func (g *IAMPolicyGenerator) GenerateIAMPolicies(serviceType, instanceName, environment, instanceDir string) error {
	service, exists := g.config.Services[serviceType]
	if !exists {
		return fmt.Errorf("service %s not found", serviceType)
	}
	instance, exists := service.Instances[instanceName]
	if !exists {
		return fmt.Errorf("instance %s not found in service %s", instanceName, serviceType)
	}
	if instance.Access == nil {
		return nil
	}

	var groups []StatementGroup
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

	if err := g.generateDataTf(serviceType, groups, instanceDir); err != nil {
		return fmt.Errorf("failed to generate data.tf: %w", err)
	}
	if err := g.generateIAMPolicy(serviceType, instanceName, environment, groups, instanceDir); err != nil {
		return fmt.Errorf("failed to generate IAM policy: %w", err)
	}
	return nil
}

// Replace these two functions in iam_generator.go.
// Everything else in the file stays the same.

func (g *IAMPolicyGenerator) generateDataTf(serviceType string, groups []StatementGroup, instanceDir string) error {
	dataPath := filepath.Join(instanceDir, "data.tf")
	existingContent := ""
	if content, err := os.ReadFile(dataPath); err == nil {
		existingContent = string(content)
	}

	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).ParseFS(templates.CommonFS, "common/data-ssm-parameter.tf.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse data template: %w", err)
	}

	var newEntries strings.Builder
	for _, group := range groups {
		myTemplates, otherCategory, ok := registry.GetTemplatesForService(serviceType, group.TargetService)
		if !ok || myTemplates.DataTemplate == nil {
			continue
		}

		// FIX: normalize hyphens to underscores so it matches what the template renders
		paramName := strings.ReplaceAll(
			fmt.Sprintf("%s_%s_arn", group.TargetInstance, group.TargetService),
			"-", "_",
		)

		if strings.Contains(existingContent, fmt.Sprintf(`"%s"`, paramName)) {
			continue
		}
		ssmPath := fmt.Sprintf("/%s/%s/arn", otherCategory, group.TargetInstance)

		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "data-ssm-parameter.tf.tmpl", map[string]string{
			"parameter_name": paramName,
			"ssm_path":       ssmPath,
		}); err != nil {
			return fmt.Errorf("failed to execute data template: %w", err)
		}
		newEntries.WriteString(buf.String())
		newEntries.WriteString("\n")
	}

	if newEntries.Len() > 0 {
		return os.WriteFile(dataPath, []byte(existingContent+"\n"+newEntries.String()), 0644)
	}
	return nil
}

func (g *IAMPolicyGenerator) generateIAMPolicy(serviceType, instanceName, environment string, groups []StatementGroup, instanceDir string) error {
	iamPath := filepath.Join(instanceDir, "iam.tf")
	existingContent := ""
	if content, err := os.ReadFile(iamPath); err == nil {
		existingContent = string(content)
	}

	type PolicyStatement struct {
		Actions   []string
		Resources []string
		Last      bool
	}

	tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).Funcs(template.FuncMap{
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

	// FIX: Group by target service and generate one policy per target service.
	// This prevents the "lambda_access_policy" catch-all that combines everything,
	// and ensures each target service gets its own clean policy.
	groupsByTarget := make(map[string][]StatementGroup)
	for _, group := range groups {
		groupsByTarget[group.TargetService] = append(groupsByTarget[group.TargetService], group)
	}

	for targetService, targetGroups := range groupsByTarget {
		policyName := fmt.Sprintf("%s_%s_policy", serviceType, targetService)

		// Skip if this policy already exists in the file
		if strings.Contains(existingContent, fmt.Sprintf(`"%s"`, policyName)) {
			continue
		}

		var statements []PolicyStatement
		for i, group := range targetGroups {
			paramName := strings.ReplaceAll(
				fmt.Sprintf("%s_%s_arn", group.TargetInstance, group.TargetService),
				"-", "_",
			)
			statements = append(statements, PolicyStatement{
				Actions:   group.Actions,
				Resources: []string{fmt.Sprintf("data.aws_ssm_parameter.%s.value", paramName)},
				Last:      i == len(targetGroups)-1,
			})
		}

		nameVar := serviceType + "_name"
		if myTemplates, _, ok := registry.GetTemplatesForService(serviceType, targetService); ok && myTemplates.NameVar != "" {
			nameVar = myTemplates.NameVar
		}

		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "iam-policy.tf.tmpl", map[string]interface{}{
			"policy_name":     policyName,
			"source_instance": instanceName,
			"target_label":    targetService,
			"name_var":        nameVar,
			"role_name":       fmt.Sprintf("%s_role", serviceType),
			"statements":      statements,
		}); err != nil {
			return fmt.Errorf("failed to execute IAM template: %w", err)
		}

		existingContent = existingContent + "\n" + buf.String()
	}

	return os.WriteFile(iamPath, []byte(existingContent), 0644)
}