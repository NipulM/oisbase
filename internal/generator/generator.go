package generator

import (
	"fmt"
	"html/template"
	"log"
	"os"

	"github.com/NipulM/oisbase/internal/config"
	"github.com/NipulM/oisbase/internal/utils"
)

func GenerateReadme(config *config.ProjectConfig) {
	file, err := os.Create("README.md")
	if err != nil {
		log.Fatalf("Failed to create README.md: %v", err)
	}
	defer file.Close()

	config.ProjectName = utils.CapitalizeWords(config.ProjectName)

	readmeTemplate := `
	#{{.ProjectName}}
	`

	tmpl, err := template.New("readme").Parse(readmeTemplate)
	if err != nil {
		log.Fatalf("Failed to parse README template: %v", err)
	}

	err = tmpl.Execute(file, config)
	if err != nil {
		log.Fatalf("Failed to execute README template: %v", err)
	}

	fmt.Println("README.md generated successfully")
}