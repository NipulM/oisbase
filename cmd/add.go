package cmd

import (
	"fmt"
	"os"

	"github.com/NipulM/terraplate/internal/config"
	"github.com/NipulM/terraplate/internal/services"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [service]",
	Short: "Add a service instance to your project",
	Long:  `Interactively configure and add a service instance (e.g., lambda, rds, vpc) to your Terraform project.`,
	Args:  cobra.ExactArgs(1), // Requires exactly one argument (service name)
	Run: func(cmd *cobra.Command, args []string) {
		serviceName := args[0]

		fmt.Printf("🔧 Adding %s to your project...\n\n", serviceName)

		// Load project-level config from .terraplate.json
		projectCfg, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			fmt.Println("Run 'terraplate init' first to initialize your project.")
			os.Exit(1)
		}

		// Get the service implementation
		service, err := services.GetService(serviceName)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			fmt.Printf("\nAvailable services: %v\n", services.ListAvailableServices())
			os.Exit(1)
		}

		// Get service-specific configuration from user
		svcConfig, err := service.GetConfig()
		if err != nil {
			fmt.Printf("❌ Error getting configuration: %v\n", err)
			os.Exit(1)
		}

		// Merge project-level config into service config
		svcConfig["project_name"] = projectCfg.ProjectName
		svcConfig["region"] = projectCfg.Region
		svcConfig["environments"] = projectCfg.Environments

		// Generate the module block
		module, err := service.GenerateModule(svcConfig)
		if err != nil {
			fmt.Printf("❌ Error generating module: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(module)

		fmt.Printf("\n✅ Successfully added %s!\n", serviceName)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}