package estimator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/NipulM/oisbase/internal/config"
	"github.com/NipulM/oisbase/internal/utils"
)

func environmentGroup(env string) string {
	if env == "prod" {
		return "production"
	}
	return "pre-production"
}

func GenerateOpenInfraQuotePlanForTerraformPlan(projectConfig *config.ProjectConfig) (string, error) {
	projectConfig, err := config.LoadConfig()
	if err != nil {
		return "", err
	}

	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".oisbase", "bin")

	oiqPath, err := EnsureOIQInstalled(cacheDir)
	if err != nil {
		return "", fmt.Errorf("failed to install openinfraquote: %w", err)
	}

	priceSheet, err := DownloadPriceSheet()
	if err != nil {
		return "", fmt.Errorf("failed to download pricing data: %w", err)
	}
	defer os.Remove(priceSheet)

	environments := projectConfig.Environments
	services := projectConfig.Services
	region := projectConfig.Region

	environmentPrompt := &survey.MultiSelect{
		Message: "Which environment do you want to estimate the cost for?",
		Options: environments,
	}
	var userSelectedEnvironments []string
	survey.AskOne(environmentPrompt, &userSelectedEnvironments)

	var envResults []utils.EnvOIQResult

	for _, environment := range userSelectedEnvironments {
		var planPaths []string

		for serviceType := range services {
			group := environmentGroup(environment)
			workDir := filepath.Join("environments", group, environment, serviceType)

			cmd := exec.Command("terraform", "show", "-json", "tf.plan")
			cmd.Dir = workDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s/%s] terraform show failed:\n%s\n", environment, serviceType, string(out))
				return "", fmt.Errorf("terraform show in %s: %w", workDir, err)
			}

			tfplanPath := filepath.Join(workDir, "tfplan.json")
			if err := os.WriteFile(tfplanPath, out, 0644); err != nil {
				return "", fmt.Errorf("write tfplan.json in %s: %w", workDir, err)
			}

			planPaths = append(planPaths, tfplanPath)
			fmt.Printf("[%s/%s] plan saved to tfplan.json\n", environment, serviceType)
		}

		if len(planPaths) == 0 {
			continue
		}

		// Run OIQ for this environment's plan paths
		results, err := RunOIQCombined(oiqPath, priceSheet, planPaths, region)
		if err != nil {
			return "", fmt.Errorf("oiq failed for %s: %w", environment, err)
		}

		envResults = append(envResults, utils.EnvOIQResult{
			Environment: environment,
			Services:    results,
		})

		for _, planPath := range planPaths {
			os.Remove(planPath)
			os.Remove(filepath.Join(filepath.Dir(planPath), "tf.plan"))
		}
	}

	if len(envResults) == 0 {
		return "", nil
	}

	summary, err := utils.FormatOIQResultsMultiEnv(envResults)
	if err != nil {
		return "", fmt.Errorf("failed to format results: %w", err)
	}

	return summary, nil
}