package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"apps.z7.ai/usm/internal/usm"
)

// MigrateExistingConfig migrates existing usm.json to Viracochan versioned config
func (s *StorageAdapter) MigrateExistingConfig(rootPath string) error {
	ctx := context.Background()

	// Check for existing usm.json file first
	oldConfigPath := filepath.Join(rootPath, "usm.json")
	data, err := os.ReadFile(oldConfigPath) //nolint:gosec // Path is constructed from trusted application rootPath
	if err != nil {
		if os.IsNotExist(err) {
			// No existing config to migrate
			return nil
		}
		return fmt.Errorf("failed to read existing config: %w", err)
	}

	// Check if we already have a versioned config (to avoid duplicate migration)
	if exists, _ := s.manager.configExists(ctx, "app-state"); exists {
		// Already have versioned config, just create backup
		backupPath := filepath.Join(rootPath, "usm.json.backup")
		_ = os.Rename(oldConfigPath, backupPath)
		return nil
	}

	// Parse old config
	var appState usm.AppState
	if err := json.Unmarshal(data, &appState); err != nil {
		return fmt.Errorf("failed to parse existing config: %w", err)
	}

	// Ensure preferences exist
	if appState.Preferences == nil {
		appState.Preferences = s.manager.newDefaultPreferences()
	}

	// Store as first version in Viracochan
	if err := s.manager.StoreAppState(ctx, &appState); err != nil {
		return fmt.Errorf("failed to store migrated config: %w", err)
	}

	// Create backup of old config
	backupPath := filepath.Join(rootPath, "usm.json.backup")
	if err := os.Rename(oldConfigPath, backupPath); err != nil {
		// Log error but don't fail migration
		fmt.Printf("Warning: Could not create backup of old config: %v\n", err)
	}

	return nil
}

// ExportToLegacy exports current config back to legacy usm.json format
// This is useful for backward compatibility during transition
func (s *StorageAdapter) ExportToLegacy(rootPath string) error {
	ctx := context.Background()

	// Get current state
	appState, err := s.manager.LoadAppState(ctx)
	if err != nil {
		return fmt.Errorf("failed to load current state: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(appState, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal app state: %w", err)
	}

	// Write to legacy location
	legacyPath := filepath.Join(rootPath, "usm.json")
	if err := os.WriteFile(legacyPath, data, 0o600); err != nil { //nolint:gosec // Path is constructed from trusted application rootPath
		return fmt.Errorf("failed to write legacy config: %w", err)
	}

	return nil
}
