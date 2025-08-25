package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"apps.z7.ai/usm/internal/usm"
)

// WatchAppState watches for changes to the application state
// Returns a channel that emits new AppState whenever it changes
func (m *Manager) WatchAppState(ctx context.Context, interval time.Duration) (<-chan *usm.AppState, error) {
	// Create channel for AppState updates
	appStateChan := make(chan *usm.AppState)

	// Get the underlying config watch channel
	configChan, err := m.vcManager.Watch(ctx, "app-state", interval)
	if err != nil {
		close(appStateChan)
		return appStateChan, err
	}

	// Transform viracochan.Config to usm.AppState
	go func() {
		defer close(appStateChan)

		for {
			select {
			case <-ctx.Done():
				return
			case cfg, ok := <-configChan:
				if !ok {
					return
				}

				// Convert to AppState
				appState := &usm.AppState{}
				var content map[string]interface{}
				if err := json.Unmarshal(cfg.Content, &content); err != nil {
					// Skip invalid configs
					continue
				}
				if err := m.extractAppState(content, appState); err != nil {
					// Skip invalid configs
					continue
				}
				appState.Modified = cfg.Meta.Time

				// Send to channel
				select {
				case appStateChan <- appState:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return appStateChan, nil
}

// Reconstruct attempts to reconstruct the latest state from journal entries
// This is useful for disaster recovery scenarios
func (m *Manager) Reconstruct(ctx context.Context) (*usm.AppState, error) {
	cfg, err := m.vcManager.Reconstruct(ctx, "app-state")
	if err != nil {
		return nil, err
	}

	appState := &usm.AppState{}
	var content map[string]interface{}
	if err := json.Unmarshal(cfg.Content, &content); err != nil {
		return nil, fmt.Errorf("failed to unmarshal content: %w", err)
	}
	if err := m.extractAppState(content, appState); err != nil {
		return nil, err
	}
	appState.Modified = cfg.Meta.Time

	return appState, nil
}

// Export exports the current configuration for backup
func (m *Manager) Export(ctx context.Context) ([]byte, error) {
	return m.vcManager.Export(ctx, "app-state")
}

// Import imports a previously exported configuration
func (m *Manager) Import(ctx context.Context, data []byte) error {
	return m.vcManager.Import(ctx, "app-state", data)
}
