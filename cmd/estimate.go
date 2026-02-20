package cmd

import (
	"fmt"
	"os"

	"github.com/NipulM/oisbase/internal/config"
	"github.com/NipulM/oisbase/internal/estimator"
	"github.com/spf13/cobra"
)

var estimateCmd = &cobra.Command{
	Use:   "estimate",
	Short: "Estimate the cost of the project",
	Long:  "Estimate the cost of the project using openinfraquote",
	Run: func(cmd *cobra.Command, args []string) {
		projectConfig, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		_, err = estimator.Estimate(projectConfig)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},


}

func init() {
	rootCmd.AddCommand(estimateCmd)
}