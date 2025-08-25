package config

import (
	"fmt"

	"apps.z7.ai/usm/internal/usm"
)

// VersionedAppStateStorage extends the basic AppStateStorage interface
// with versioning capabilities from Viracochan
type VersionedAppStateStorage interface {
	usm.AppStateStorage

	// Version history operations
	GetHistory() ([]*usm.AppState, error)
	Rollback(version uint64) (*usm.AppState, error)
	ValidateChain() error

	// Migration operations
	MigrateExistingConfig(rootPath string) error
	ExportToLegacy(rootPath string) error
}

// CreateVersionedStorage creates a versioned storage adapter
// that can be used as a drop-in replacement for AppStateStorage
func CreateVersionedStorage(rootPath string) (VersionedAppStateStorage, error) {
	adapter, err := NewStorageAdapter(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create versioned storage: %w", err)
	}

	// Automatically migrate existing config if present
	if err := adapter.MigrateExistingConfig(rootPath); err != nil {
		return nil, fmt.Errorf("failed to migrate existing config: %w", err)
	}

	return adapter, nil
}

// Example of how to integrate with existing storage implementations:
//
// In storage_os.go or storage_fyne.go, replace the LoadAppState and StoreAppState
// methods to use the versioned storage:
//
// type OSStorage struct {
//     root string
//     versionedConfig VersionedAppStateStorage  // Add this field
// }
//
// func NewOSStorage(root string) (*OSStorage, error) {
//     versionedConfig, err := CreateVersionedStorage(root)
//     if err != nil {
//         return nil, err
//     }
//     return &OSStorage{
//         root: root,
//         versionedConfig: versionedConfig,
//     }, nil
// }
//
// func (s *OSStorage) LoadAppState() (*usm.AppState, error) {
//     return s.versionedConfig.LoadAppState()
// }
//
// func (s *OSStorage) StoreAppState(appState *usm.AppState) error {
//     return s.versionedConfig.StoreAppState(appState)
// }
