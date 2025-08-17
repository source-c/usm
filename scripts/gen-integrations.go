package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"
)

// Config holds the template variables for generating integration files
type Config struct {
	AppName      string
	BinaryName   string
	Description  string
	Version      string
	InstallPath  string
	IconName     string
	BundleID     string
	Signature    string
	MinOSVersion string
}

func main() {
	var (
		outputDir   = flag.String("output", "build/integrations", "Output directory for generated integration files")
		installPath = flag.String("install", "/usr/local/bin", "Installation path for the application")
		version     = flag.String("version", "dev", "Application version")
	)
	flag.Parse()

	config := Config{
		AppName:      "USM",
		BinaryName:   "usm",
		Description:  "Simple, modern and privacy-focused secrets manager",
		Version:      *version,
		InstallPath:  *installPath,
		IconName:     "usm",
		BundleID:     "ai.z7.apps.usm",
		Signature:    "USM!",
		MinOSVersion: "10.15",
	}

	if err := generateIntegrations(config, *outputDir); err != nil {
		log.Fatalf("Failed to generate integrations: %v", err)
	}

	fmt.Printf("Generated integration files in: %s\n", *outputDir)
}

func generateIntegrations(config Config, outputDir string) error {
	// Create output directories
	linuxDir := filepath.Join(outputDir, "Linux")
	macosDir := filepath.Join(outputDir, "MacOS", config.AppName+".app", "Contents")
	macosResourcesDir := filepath.Join(outputDir, "MacOS", config.AppName+".app", "Resources")
	macosBinDir := filepath.Join(outputDir, "MacOS", config.AppName+".app", "Contents", "MacOS")

	dirs := []string{linuxDir, macosDir, macosResourcesDir, macosBinDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Generate Linux desktop file
	if err := generateFromTemplate(
		"templates/linux.desktop.tmpl",
		filepath.Join(linuxDir, config.BinaryName+".desktop"),
		config,
	); err != nil {
		return fmt.Errorf("failed to generate Linux desktop file: %w", err)
	}

	// Generate macOS Info.plist
	if err := generateFromTemplate(
		"templates/macos.Info.plist.tmpl",
		filepath.Join(macosDir, "Info.plist"),
		config,
	); err != nil {
		return fmt.Errorf("failed to generate macOS Info.plist: %w", err)
	}

	// Copy icon files (these will be copied by the Makefile)
	fmt.Println("Note: Icon files need to be copied separately by the build process")

	return nil
}

func generateFromTemplate(templatePath, outputPath string, config Config) error {
	// Read template file
	tmplContent, err := os.ReadFile(templatePath) //nolint:gosec // templatePath is application-controlled
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	// Parse template
	tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	// Create output file
	outputFile, err := os.Create(outputPath) //nolint:gosec // outputPath is application-controlled
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outputFile.Close()

	// Execute template
	if err := tmpl.Execute(outputFile, config); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templatePath, err)
	}

	fmt.Printf("Generated: %s\n", outputPath)
	return nil
}
