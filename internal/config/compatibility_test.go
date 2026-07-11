package config

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"apps.z7.ai/usm/internal/usm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_LegacyViracochanV011DataRemainsReadableAndExtendable(t *testing.T) {
	root := copyFixtureTree(t, filepath.Join("testdata", "viracochan-v0.1.1-app"))

	manager, err := NewManager(root)
	require.NoError(t, err)

	ctx := context.Background()

	state, err := manager.LoadAppState(ctx)
	require.NoError(t, err)
	require.NotNil(t, state)
	require.NotNil(t, state.Preferences)
	assert.Equal(t, "#FF9800", state.Preferences.Theme.DefaultColour)
	assert.Equal(t, usm.SyncModeStrict, state.Preferences.Sync.Mode)

	err = manager.ValidateChain(ctx)
	require.NoError(t, err)

	history, err := manager.GetHistory(ctx)
	require.NoError(t, err)
	assert.Len(t, history, 2)

	state.Preferences.Theme.DefaultColour = "#009688"
	err = manager.StoreAppState(ctx, state)
	require.NoError(t, err)

	updated, err := manager.LoadAppState(ctx)
	require.NoError(t, err)
	assert.Equal(t, "#009688", updated.Preferences.Theme.DefaultColour)

	latest, err := manager.vcManager.GetLatest(ctx, "app-state")
	require.NoError(t, err)
	assert.Equal(t, uint64(3), latest.Meta.Version)

	history, err = manager.GetHistory(ctx)
	require.NoError(t, err)
	assert.Len(t, history, 3)
}

func TestCatalogueManager_LegacyViracochanV011DataRemainsReadableAndExtendable(t *testing.T) {
	root := copyFixtureTree(t, filepath.Join("testdata", "viracochan-v0.1.1-catalogue"))

	manager, err := NewCatalogueManager(root)
	require.NoError(t, err)

	ctx := context.Background()

	catalogue, err := manager.LoadCatalogue(ctx)
	require.NoError(t, err)
	require.Len(t, catalogue, 2)
	require.Contains(t, catalogue, "personal")
	require.Contains(t, catalogue, "work")

	assert.Equal(t, 2, catalogue["personal"].Version)
	assert.Equal(t, "a16275a70249d8914093bbbbe450090ab649ed006c18700b5397bb607a694ded", catalogue["personal"].ChainCS)
	assert.Equal(t, 1, catalogue["work"].Version)

	assert.Equal(t, uint64(4), manager.GetChainVersion(ctx))
	err = manager.vcManager.ValidateChain(ctx, catalogueConfigID)
	require.NoError(t, err)

	catalogue["work"].ItemCount = 6
	meta, err := manager.StoreCatalogue(ctx, catalogue)
	require.NoError(t, err)
	assert.Equal(t, uint64(5), meta.Version)
	assert.NotEmpty(t, meta.CS)

	reloaded, err := manager.LoadCatalogue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 6, reloaded["work"].ItemCount)

	err = manager.vcManager.ValidateChain(ctx, catalogueConfigID)
	require.NoError(t, err)
}

func copyFixtureTree(t *testing.T, src string) string {
	t.Helper()

	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		return copyFile(path, target)
	})
	require.NoError(t, err)

	return dst
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // test fixture path
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst) //nolint:gosec // test fixture path
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}

	return out.Close()
}
