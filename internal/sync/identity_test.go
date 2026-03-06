package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOrCreateIdentity_CreatesNewKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "peer.key")

	priv, err := LoadOrCreateIdentity(keyPath)
	require.NoError(t, err)
	assert.NotNil(t, priv)

	_, err = os.Stat(keyPath)
	assert.NoError(t, err)
}

func TestLoadOrCreateIdentity_LoadsExistingKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "peer.key")

	priv1, err := LoadOrCreateIdentity(keyPath)
	require.NoError(t, err)

	priv2, err := LoadOrCreateIdentity(keyPath)
	require.NoError(t, err)

	raw1, _ := priv1.Raw()
	raw2, _ := priv2.Raw()
	assert.Equal(t, raw1, raw2)
}

func TestLoadOrCreateIdentity_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "peer.key")

	_, err := LoadOrCreateIdentity(keyPath)
	require.NoError(t, err)

	info, err := os.Stat(keyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
