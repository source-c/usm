package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"apps.z7.ai/usm/internal/usm"
	"github.com/source-c/viracochan"
)

// Manager handles versioned configuration management for USM
// using Viracochan for integrity, versioning, and journaling
type Manager struct {
	vcManager *viracochan.Manager
	storage   viracochan.Storage
}

// NewManager creates a new configuration manager
func NewManager(storagePath string) (*Manager, error) {
	// Create file-based storage for Viracochan
	storage, err := viracochan.NewFileStorage(storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create viracochan storage: %w", err)
	}

	// Create Viracochan manager without signing for now
	// ATTN: Signing can be enabled later for authentication
	vcManager, err := viracochan.NewManager(storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create viracochan manager: %w", err)
	}

	return &Manager{
		vcManager: vcManager,
		storage:   storage,
	}, nil
}

// LoadAppState loads the current application state with preferences
func (m *Manager) LoadAppState(ctx context.Context) (*usm.AppState, error) {
	// Try to get the latest version of app-state config
	cfg, err := m.vcManager.GetLatest(ctx, "app-state")
	if err != nil {
		// If no config exists, return default state
		return &usm.AppState{
			Modified:    time.Now().UTC(),
			Preferences: m.newDefaultPreferences(),
		}, nil
	}

	// Extract AppState from configuration
	appState := &usm.AppState{}
	var content map[string]interface{}
	if err := json.Unmarshal(cfg.Content, &content); err != nil {
		return nil, fmt.Errorf("failed to unmarshal content: %w", err)
	}
	if err := m.extractAppState(content, appState); err != nil {
		return nil, fmt.Errorf("failed to extract app state: %w", err)
	}

	// Update modified time from metadata
	appState.Modified = cfg.Meta.Time

	return appState, nil
}

// StoreAppState stores the application state as a new version
func (m *Manager) StoreAppState(ctx context.Context, appState *usm.AppState) error {
	// Convert AppState to configuration content
	content := m.appStateToContent(appState)

	// Check if configuration exists
	exists, err := m.configExists(ctx, "app-state")
	if err != nil {
		return fmt.Errorf("failed to check config existence: %w", err)
	}

	if !exists {
		// Create initial configuration
		_, err = m.vcManager.Create(ctx, "app-state", content)
	} else {
		// Update existing configuration (creates new version)
		_, err = m.vcManager.Update(ctx, "app-state", content)
	}

	if err != nil {
		return fmt.Errorf("failed to store app state: %w", err)
	}

	return nil
}

// GetHistory returns all versions of the application state
func (m *Manager) GetHistory(ctx context.Context) ([]*usm.AppState, error) {
	configs, err := m.vcManager.GetHistory(ctx, "app-state")
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	history := make([]*usm.AppState, 0, len(configs))
	for _, cfg := range configs {
		appState := &usm.AppState{}
		var content map[string]interface{}
		if err := json.Unmarshal(cfg.Content, &content); err != nil {
			continue // Skip invalid entries
		}
		if err := m.extractAppState(content, appState); err != nil {
			continue // Skip invalid entries
		}
		appState.Modified = cfg.Meta.Time
		history = append(history, appState)
	}

	return history, nil
}

// Rollback reverts to a specific version of preferences
func (m *Manager) Rollback(ctx context.Context, version uint64) (*usm.AppState, error) {
	cfg, err := m.vcManager.Rollback(ctx, "app-state", version)
	if err != nil {
		return nil, fmt.Errorf("failed to rollback: %w", err)
	}

	appState := &usm.AppState{}
	var content map[string]interface{}
	if err := json.Unmarshal(cfg.Content, &content); err != nil {
		return nil, fmt.Errorf("failed to unmarshal content: %w", err)
	}
	if err := m.extractAppState(content, appState); err != nil {
		return nil, fmt.Errorf("failed to extract app state: %w", err)
	}
	appState.Modified = cfg.Meta.Time

	return appState, nil
}

// ValidateChain validates the integrity of the configuration chain
func (m *Manager) ValidateChain(ctx context.Context) error {
	return m.vcManager.ValidateChain(ctx, "app-state")
}

// extractAppState converts configuration content to AppState
func (m *Manager) extractAppState(content map[string]interface{}, appState *usm.AppState) error {
	// Extract preferences if present
	if prefs, ok := content["preferences"].(map[string]interface{}); ok {
		appState.Preferences = m.extractPreferences(prefs)
	} else {
		appState.Preferences = m.newDefaultPreferences()
	}

	return nil
}

// appStateToContent converts AppState to configuration content
func (m *Manager) appStateToContent(appState *usm.AppState) map[string]interface{} {
	content := make(map[string]interface{})

	if appState.Preferences != nil {
		content["preferences"] = m.preferencesToMap(appState.Preferences)
	}

	return content
}

// configExists checks if a configuration exists
func (m *Manager) configExists(ctx context.Context, id string) (bool, error) {
	_, err := m.vcManager.GetLatest(ctx, id)
	// ATTN: Following the demo pattern - any error means config doesn't exist
	return err == nil, nil
}
