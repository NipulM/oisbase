package estimator

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/NipulM/oisbase/internal/config"
)

func Estimate(config *config.ProjectConfig) (float64, error) {
	const (
		colourYellow = "\033[33m"
		colourReset = "\033[0m"
	)

	fmt.Println(colourYellow + "Warning: Cost estimation requires a saved Terraform plan file generated via 'terraform plan -out=tf.plan'." + colourReset)

	userApprovalPrompt := &survey.Confirm{
		Message: "Do you want to proceed with cost estimation using OpenInfraQuote?",
	}
	var userApproval bool
	survey.AskOne(userApprovalPrompt, &userApproval)
	if !userApproval {
		return 0, nil
	}

	terraformPlan, err := GenerateOpenInfraQuotePlanForTerraformPlan(config)
	if err != nil {
		return 0, fmt.Errorf("failed to generate terraform plan: %w", err)
	}

	fmt.Println(terraformPlan)

	return 0, nil
}