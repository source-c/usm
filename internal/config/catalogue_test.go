package config

import (
	"context"
	"os"
	"testing"
	"time"

	"apps.z7.ai/usm/internal/usm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogueManager_StoreAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "catalogue_manager_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cm, err := NewCatalogueManager(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Initial load should return empty map
	cat, err := cm.LoadCatalogue(ctx)
	require.NoError(t, err)
	assert.Empty(t, cat)

	now := time.Now().UTC()

	// Store new catalogue
	cat["vault1"] = &usm.VaultEntry{
		Name:            "vault1",
		Version:         1,
		StorageLocation: "/path/to/vault1",
		Created:         now,
		Modified:        now,
		ItemCount:       42,
	}

	meta, err := cm.StoreCatalogue(ctx, cat)
	require.NoError(t, err)
	assert.NotNil(t, meta)
	assert.NotEmpty(t, meta.CS)
	assert.Equal(t, uint64(1), meta.Version)

	// Load back
	loaded, err := cm.LoadCatalogue(ctx)
	require.NoError(t, err)
	assert.Len(t, loaded, 1)

	entry := loaded["vault1"]
	assert.Equal(t, "vault1", entry.Name)
	assert.Equal(t, 1, entry.Version)
	assert.Equal(t, 42, entry.ItemCount)
	// Time equality across JSON marshal/unmarshal might lose precision
	assert.WithinDuration(t, now, entry.Created, time.Millisecond)
}

func TestCatalogueManager_MigrateFromLegacy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "catalogue_migrate_test_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cm, err := NewCatalogueManager(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	now := time.Now().UTC()
	legacyCat := map[string]*usm.VaultEntry{
		"legacy1": {Name: "legacy1", Version: 5, Modified: now},
		"legacy2": {Name: "legacy2", Version: 2, Modified: now},
	}

	// First migration should work
	err = cm.MigrateFromLegacy(ctx, legacyCat)
	require.NoError(t, err)

	// Check it was stored
	loaded, err := cm.LoadCatalogue(ctx)
	require.NoError(t, err)
	assert.Len(t, loaded, 2)
	assert.Equal(t, 5, loaded["legacy1"].Version)

	// Second migration should be a no-op (idempotent)
	err = cm.MigrateFromLegacy(ctx, map[string]*usm.VaultEntry{"new_legacy": {Name: "new_legacy"}})
	require.NoError(t, err)

	loadedAgain, err := cm.LoadCatalogue(ctx)
	require.NoError(t, err)
	assert.Len(t, loadedAgain, 2) // "new_legacy" should not be added
}
