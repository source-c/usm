package config

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"apps.z7.ai/usm/internal/usm"
	"github.com/source-c/viracochan"
)

const catalogueConfigID = "vault-catalogue"

// CatalogueManager wraps Viracochan for versioned, integrity-checked vault catalogue storage.
// Each mutation creates a new immutable chain entry with a SHA-256 checksum, so sync
// negotiation can use the checksum to prove two peers have identical catalogue state.
type CatalogueManager struct {
	vcManager *viracochan.Manager
}

// NewCatalogueManager creates a CatalogueManager backed by the given storage root.
// The Viracochan files live under <storagePath>/ alongside the preferences chain.
func NewCatalogueManager(storagePath string) (*CatalogueManager, error) {
	storage, err := viracochan.NewFileStorage(storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create viracochan storage: %w", err)
	}

	vcManager, err := viracochan.NewManager(storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create viracochan manager: %w", err)
	}

	return &CatalogueManager{vcManager: vcManager}, nil
}

// LoadCatalogue returns the current vault catalogue from the Viracochan chain.
// Returns an empty map (not nil) if no chain exists yet.
func (cm *CatalogueManager) LoadCatalogue(ctx context.Context) (map[string]*usm.VaultEntry, error) {
	cfg, err := cm.vcManager.GetLatest(ctx, catalogueConfigID)
	if err != nil {
		return make(map[string]*usm.VaultEntry), nil
	}

	var catalogue map[string]*usm.VaultEntry
	if err := json.Unmarshal(cfg.Content, &catalogue); err != nil {
		return nil, fmt.Errorf("failed to unmarshal catalogue: %w", err)
	}
	if catalogue == nil {
		catalogue = make(map[string]*usm.VaultEntry)
	}
	return catalogue, nil
}

// StoreCatalogue persists the entire vault catalogue as a new chain version.
// Returns the chain metadata (version, checksum) for stamping entries.
func (cm *CatalogueManager) StoreCatalogue(ctx context.Context, catalogue map[string]*usm.VaultEntry) (*viracochan.Meta, error) {
	content, err := json.Marshal(catalogue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal catalogue: %w", err)
	}

	exists, err := cm.configExists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check config existence: %w", err)
	}

	var cfg *viracochan.Config
	if !exists {
		cfg, err = cm.vcManager.Create(ctx, catalogueConfigID, json.RawMessage(content))
	} else {
		cfg, err = cm.vcManager.Update(ctx, catalogueConfigID, json.RawMessage(content))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to store catalogue: %w", err)
	}

	meta := cfg.Meta
	return &meta, nil
}

// UpdateVaultEntry loads the catalogue, applies the existing UpdateVaultCatalogue logic,
// stores the result through Viracochan, and stamps the entry with chain metadata.
func (cm *CatalogueManager) UpdateVaultEntry(ctx context.Context, vault *usm.Vault, storage usm.Storage) (*viracochan.Meta, error) {
	catalogue, err := cm.LoadCatalogue(ctx)
	if err != nil {
		return nil, err
	}

	usm.UpdateVaultCatalogue(catalogue, vault, storage)

	meta, err := cm.StoreCatalogue(ctx, catalogue)
	if err != nil {
		return nil, err
	}

	// Stamp the updated entry with the chain checksum
	// ATTN: VaultEntry.Version tracks per-vault modifications and is managed by
	// usm.UpdateVaultCatalogue — do NOT overwrite it with the chain version.
	if entry, ok := catalogue[vault.Name]; ok {
		entry.ChainCS = meta.CS
		meta, err = cm.StoreCatalogue(ctx, catalogue)
		if err != nil {
			return nil, err
		}
	}

	return meta, nil
}

// GetChainVersion returns the current chain version, or 0 if no chain exists.
func (cm *CatalogueManager) GetChainVersion(ctx context.Context) uint64 {
	cfg, err := cm.vcManager.GetLatest(ctx, catalogueConfigID)
	if err != nil {
		return 0
	}
	return cfg.Meta.Version
}

// MigrateFromLegacy imports an existing catalogue from the legacy usm.json path
// into the Viracochan chain. Idempotent — does nothing if the chain already exists.
func (cm *CatalogueManager) MigrateFromLegacy(ctx context.Context, catalogue map[string]*usm.VaultEntry) error {
	exists, err := cm.configExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check config existence: %w", err)
	}
	if exists {
		return nil
	}
	if len(catalogue) == 0 {
		return nil
	}

	_, err = cm.vcManager.Create(ctx, catalogueConfigID, catalogue)
	if err != nil {
		return fmt.Errorf("failed to create catalogue chain: %w", err)
	}
	return nil
}

// configExists checks whether the vault-catalogue chain has been created.
func (cm *CatalogueManager) configExists(ctx context.Context) (bool, error) {
	_, err := cm.vcManager.GetLatest(ctx, catalogueConfigID)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// ConfigDir returns the Viracochan config directory path for the given storage root.
func ConfigDir(storageRoot string) string {
	return filepath.Join(storageRoot, "config")
}
