package config

import (
	"context"
	"fmt"
	"path/filepath"

	"apps.z7.ai/usm/internal/usm"
)

// StorageAdapter adapts Viracochan configuration manager to the existing AppStateStorage interface
type StorageAdapter struct {
	manager *Manager
}

// NewStorageAdapter creates a new storage adapter
func NewStorageAdapter(storagePath string) (*StorageAdapter, error) {
	// Create config directory for Viracochan
	configPath := filepath.Join(storagePath, "config")

	manager, err := NewManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	return &StorageAdapter{
		manager: manager,
	}, nil
}

// LoadAppState implements AppStateStorage interface
func (s *StorageAdapter) LoadAppState() (*usm.AppState, error) {
	ctx := context.Background()
	return s.manager.LoadAppState(ctx)
}

// StoreAppState implements AppStateStorage interface
func (s *StorageAdapter) StoreAppState(appState *usm.AppState) error {
	ctx := context.Background()
	return s.manager.StoreAppState(ctx, appState)
}

// GetHistory returns the version history of preferences
// ATTN: This is an additional method not in the original interface
func (s *StorageAdapter) GetHistory() ([]*usm.AppState, error) {
	ctx := context.Background()
	return s.manager.GetHistory(ctx)
}

// Rollback reverts to a specific version
// ATTN: This is an additional method not in the original interface
func (s *StorageAdapter) Rollback(version uint64) (*usm.AppState, error) {
	ctx := context.Background()
	return s.manager.Rollback(ctx, version)
}

// ValidateChain validates the integrity of the configuration chain
// ATTN: This is an additional method not in the original interface
func (s *StorageAdapter) ValidateChain() error {
	ctx := context.Background()
	return s.manager.ValidateChain(ctx)
}
