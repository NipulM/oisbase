package generator

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/NipulM/oisbase/internal/prompts"
	"github.com/NipulM/oisbase/internal/utils"
	"github.com/NipulM/oisbase/templates"
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

func CopyModules(config *prompts.ProjectConfig) error {
    modulesBaseDir := "services/aws/modules"
    
    for _, service := range config.Services {
        serviceDir := filepath.Join(modulesBaseDir, service)
        
        // Read from embedded filesystem
        entries, err := fs.ReadDir(templates.ModulesFS, serviceDir)
        if err != nil {
            fmt.Printf("Warning: No module template found for %s, skipping\n", service)
            continue
        }
        
        // Create destination directory
        destDir := filepath.Join("modules", service)
        if err := os.MkdirAll(destDir, 0755); err != nil {
            return fmt.Errorf("failed to create module directory: %w", err)
        }
        
        // Copy each file from embedded FS to destination
        for _, entry := range entries {
            if entry.IsDir() {
                continue
            }
            
            srcPath := filepath.Join(serviceDir, entry.Name())
            content, err := templates.ModulesFS.ReadFile(srcPath)
            if err != nil {
                return fmt.Errorf("failed to read embedded file %s: %w", srcPath, err)
            }
            
            destPath := filepath.Join(destDir, entry.Name())
            if err := os.WriteFile(destPath, content, 0644); err != nil {
                return fmt.Errorf("failed to write file %s: %w", destPath, err)
            }
        }
    }
    
    return nil
}