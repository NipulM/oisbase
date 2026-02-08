package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "ois",
    Short: "Generate AWS Terraform infrastructure templates",
    Long:  `An interactive CLI tool to scaffold production-ready Terraform configurations for AWS infrastructure.`,
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}