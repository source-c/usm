package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"apps.z7.ai/usm/internal/usm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "usm-config-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create manager
	manager, err := NewManager(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, manager)

	ctx := context.Background()

	t.Run("LoadDefaultState", func(t *testing.T) {
		// Load state when no config exists
		state, err := manager.LoadAppState(ctx)
		require.NoError(t, err)
		require.NotNil(t, state)
		require.NotNil(t, state.Preferences)

		// Check some default values
		assert.False(t, state.Preferences.FaviconDownloader.Disabled)
		assert.Equal(t, usm.PassphrasePasswordDefaultLength, state.Preferences.Password.Passphrase.DefaultLength)
	})

	t.Run("StoreAndLoadState", func(t *testing.T) {
		// Create custom state
		customState := &usm.AppState{
			Preferences: &usm.Preferences{
				FaviconDownloader: usm.FaviconDownloaderPreferences{
					Disabled: true,
				},
				Theme: usm.ThemePreferences{
					DefaultColour: "#2196F3",
				},
			},
		}

		// Store state
		err := manager.StoreAppState(ctx, customState)
		require.NoError(t, err)

		// Load state back
		loadedState, err := manager.LoadAppState(ctx)
		require.NoError(t, err)
		require.NotNil(t, loadedState)

		// Verify preferences were preserved
		assert.True(t, loadedState.Preferences.FaviconDownloader.Disabled)
		assert.Equal(t, "#2196F3", loadedState.Preferences.Theme.DefaultColour)
	})

	t.Run("VersionHistory", func(t *testing.T) {
		// Create multiple versions
		for i := 0; i < 3; i++ {
			state := &usm.AppState{
				Preferences: &usm.Preferences{
					Theme: usm.ThemePreferences{
						DefaultColour: usm.PredefinedColours[i].Value,
					},
				},
			}
			err := manager.StoreAppState(ctx, state)
			require.NoError(t, err)
		}

		// Get history
		history, err := manager.GetHistory(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(history), 3)
	})

	t.Run("ChainValidation", func(t *testing.T) {
		// Validate chain integrity
		err := manager.ValidateChain(ctx)
		assert.NoError(t, err)
	})
}

func TestStorageAdapter(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "usm-adapter-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create adapter
	adapter, err := NewStorageAdapter(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, adapter)

	t.Run("ImplementsInterface", func(t *testing.T) {
		// Verify adapter implements AppStateStorage interface
		var _ usm.AppStateStorage = adapter
	})

	t.Run("LoadStore", func(t *testing.T) {
		// Test through interface
		state := &usm.AppState{
			Preferences: &usm.Preferences{
				TOTP: usm.TOTPPreferences{
					Digits:   6,
					Interval: 30,
				},
			},
		}

		err := adapter.StoreAppState(state)
		require.NoError(t, err)

		loaded, err := adapter.LoadAppState()
		require.NoError(t, err)
		assert.Equal(t, 6, loaded.Preferences.TOTP.Digits)
		assert.Equal(t, 30, loaded.Preferences.TOTP.Interval)
	})
}

func TestMigration(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "usm-migration-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("MigrateExistingConfig", func(t *testing.T) {
		// Create old-style config file
		oldConfig := `{
			"modified": "2024-01-01T00:00:00Z",
			"preferences": {
				"theme": {
					"default_colour": "#4CAF50"
				}
			}
		}`

		oldConfigPath := filepath.Join(tmpDir, "usm.json")
		err := os.WriteFile(oldConfigPath, []byte(oldConfig), 0o600) //nolint:gosec // Test file path in temp directory
		require.NoError(t, err)

		// Create adapter and migrate
		adapter, err := NewStorageAdapter(tmpDir)
		require.NoError(t, err)

		err = adapter.MigrateExistingConfig(tmpDir)
		require.NoError(t, err)

		// Verify migration
		state, err := adapter.LoadAppState()
		require.NoError(t, err)
		assert.Equal(t, "#4CAF50", state.Preferences.Theme.DefaultColour)

		// Verify backup was created
		_, err = os.Stat(filepath.Join(tmpDir, "usm.json.backup"))
		assert.NoError(t, err)
	})

	t.Run("ExportToLegacy", func(t *testing.T) {
		adapter, err := NewStorageAdapter(tmpDir)
		require.NoError(t, err)

		// Store a state
		state := &usm.AppState{
			Preferences: &usm.Preferences{
				Theme: usm.ThemePreferences{
					DefaultColour: "#FF9800",
				},
			},
		}
		err = adapter.StoreAppState(state)
		require.NoError(t, err)

		// Export to legacy format
		err = adapter.ExportToLegacy(tmpDir)
		require.NoError(t, err)

		// Verify legacy file exists
		_, err = os.Stat(filepath.Join(tmpDir, "usm.json"))
		assert.NoError(t, err)
	})
}
