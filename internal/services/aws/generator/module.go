package generator

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/NipulM/oisbase/internal/config"
	"github.com/NipulM/oisbase/templates"
)

func CopyModules(config *config.ProjectConfig) error {
	// Embedded FS paths are under services/aws/modules/<serviceName>/
	sourceBaseDir := "services/aws/modules"

	for serviceName := range config.Services {

		serviceDir := filepath.Join(sourceBaseDir, serviceName)

		// Read from embedded filesystem
		entries, err := fs.ReadDir(templates.ModulesFS, serviceDir)
		if err != nil {
			fmt.Printf("Warning: No module template found for %s, skipping\n", serviceName)
			continue
		}

		// Create destination directory
		destDir := filepath.Join("modules", serviceName)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create module directory: %w", err)
		}

		// Copy each file
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