package usm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateVaultCatalogueCreatesEntry(t *testing.T) {
	key, err := MakeOneTimeKey()
	require.NoError(t, err)

	vault := NewVault(key, "test-vault")
	storage := &StorageMock{}
	catalogue := map[string]*VaultEntry{}

	UpdateVaultCatalogue(catalogue, vault, storage)

	entry, ok := catalogue[vault.Name]
	require.True(t, ok)
	assert.Equal(t, vault.Name, entry.Name)
	assert.Equal(t, vaultRootPath(storage, vault.Name), entry.StorageLocation)
	assert.Equal(t, vault.Size(), entry.ItemCount)
	assert.Equal(t, key.Fingerprint(), entry.KeyFingerprint)
}
