package cmd

import (
	"fmt"

	"os"
	"path/filepath"

	"github.com/NipulM/oisbase/internal/config"
	"github.com/NipulM/oisbase/internal/generator"
	"github.com/NipulM/oisbase/internal/prompts"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Terraform project",
	Long:  `Interactively configure and generate a new Terraform project with AWS modules.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 Let's create your Terraform project!\n")

		promptConfig, err := prompts.GetProjectConfig()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("\n✅ Configuration complete!\n")
		fmt.Printf("   Project: %s\n", promptConfig.ProjectName)
		fmt.Printf("   Environments: %v\n", promptConfig.Environments)
		fmt.Printf("   Services: %v\n", promptConfig.Services)
		fmt.Printf("   Region: %s\n", promptConfig.Region)
		fmt.Println("\n📦 Generating your Terraform configuration...")

		// Create project structure in current directory
		// Create environments directories

		for _, env := range promptConfig.Environments {
			envPath := filepath.Join("environments", env)
			if err := os.MkdirAll(envPath, 0755); err != nil {
				fmt.Printf("Error creating environment directory: %v\n", err)
				return
			}
		}

		if err := os.MkdirAll("modules", 0755); err != nil {
			fmt.Printf("Error creating modules directory: %v\n", err)
			return
		}

		// Generate README
		generator.GenerateReadme(promptConfig)
		if err != nil {
			fmt.Printf("Error generating README: %v\n", err)
			return
		}

		// Copy modules
		generator.CopyModules(promptConfig)
		if err != nil {
			fmt.Printf("Error copying modules: %v\n", err)
			return
		}

		// Save project configuration in current directory
		projectConfig := &config.ProjectConfig{
			ProjectName:  promptConfig.ProjectName,
			Environments: promptConfig.Environments,
			Region:       promptConfig.Region,
			Services:     promptConfig.Services,
		}

		if err := config.SaveConfig(projectConfig); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}

		fmt.Println("\n🎉 Project created successfully!")
		fmt.Printf("📂 Next steps:\n")
		fmt.Printf("   oisbase add lambda  # Add a Lambda function\n")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}