package generator

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"

	"github.com/NipulM/oisbase/internal/prompts"
	"github.com/NipulM/oisbase/internal/utils"
)

func GenerateReadme(config *prompts.ProjectConfig) {
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

func CopyModules(config *prompts.ProjectConfig) {
	modulesDir := "templates/services/aws/modules"
	modules, err := os.ReadDir(modulesDir)
	if err != nil {
		log.Fatalf("Failed to read modules directory: %v", err)
	}

	selectedSet := make(map[string]bool)
	for _, s := range config.Services {
		selectedSet[s] = true
	}

	for _, module := range modules {
		if !module.IsDir() {
			continue
		}

		if !selectedSet[module.Name()] {
			continue
		}

		moduleDir := filepath.Join(modulesDir, module.Name())
		moduleFiles, err := os.ReadDir(moduleDir)
		if err != nil {
			log.Fatalf("Failed to read module directory: %v", err)
		}

		outDir := filepath.Join("modules", module.Name())
		if err := os.MkdirAll(outDir, 0755); err != nil {
			log.Fatalf("Failed to create output directory %s: %v", outDir, err)
		}

		for _, moduleFile := range moduleFiles {
			if moduleFile.IsDir() {
				continue
			}
			moduleFilePath := filepath.Join(moduleDir, moduleFile.Name())
			moduleFileContent, err := os.ReadFile(moduleFilePath)
			if err != nil {
				log.Fatalf("Failed to read module file: %v", err)
			}

			outPath := filepath.Join(outDir, moduleFile.Name())
			if err := os.WriteFile(outPath, moduleFileContent, 0644); err != nil {
				log.Fatalf("Failed to write %s: %v", outPath, err)
			}
		}
	}
}