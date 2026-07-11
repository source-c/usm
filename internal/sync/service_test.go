package sync

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"apps.z7.ai/usm/internal/config"
	"apps.z7.ai/usm/internal/usm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_StartStop(t *testing.T) {
	dir := t.TempDir()

	svc, err := NewService(ServiceConfig{
		PeerKeyPath:      filepath.Join(dir, "peer.key"),
		TrustedPeersPath: filepath.Join(dir, "trusted_peers.json"),
		SyncMode:         "relaxed",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = svc.Start(ctx)
	if shouldSkipStart(err) {
		t.Skip(err)
	}
	require.NoError(t, err)

	// Should have a host running
	peers := svc.Peers()
	assert.NotNil(t, peers)

	err = svc.Stop()
	require.NoError(t, err)
}

func TestService_DisabledMode_Noop(t *testing.T) {
	dir := t.TempDir()

	svc, err := NewService(ServiceConfig{
		PeerKeyPath:      filepath.Join(dir, "peer.key"),
		TrustedPeersPath: filepath.Join(dir, "trusted_peers.json"),
		SyncMode:         "disabled",
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = svc.Start(ctx)
	require.NoError(t, err)

	// Peers should return empty, no crash
	assert.Empty(t, svc.Peers())

	err = svc.Stop()
	require.NoError(t, err)
}

func TestService_DoubleStop_NoPanic(t *testing.T) {
	dir := t.TempDir()

	svc, err := NewService(ServiceConfig{
		PeerKeyPath:      filepath.Join(dir, "peer.key"),
		TrustedPeersPath: filepath.Join(dir, "trusted_peers.json"),
		SyncMode:         "relaxed",
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = svc.Start(ctx)
	if shouldSkipStart(err) {
		t.Skip(err)
	}
	require.NoError(t, err)
	_ = svc.Stop()
	err = svc.Stop() // second stop should not panic
	assert.NoError(t, err)
}

func TestUpdateCatalogueAfterReceive_UpdatesExistingEntry(t *testing.T) {
	now := time.Now().UTC()
	remoteModified := now.Add(time.Hour)

	var stored *usm.AppState
	storage := usm.StorageMock{}
	storage.OnLoadAppState = func() (*usm.AppState, error) {
		return &usm.AppState{
			VaultCatalogue: map[string]*usm.VaultEntry{
				"vault1": {
					Name:            "vault1",
					Version:         3,
					Modified:        now,
					KeyFingerprint:  "sha256:old",
					ItemCount:       5,
					StorageLocation: filepath.Join(storage.Root(), "storage", "vault1"),
				},
			},
		}, nil
	}
	storage.OnStoreAppState = func(s *usm.AppState) error {
		stored = s
		return nil
	}

	remoteCat := map[string]*usm.VaultEntry{
		"vault1": {
			Name:           "vault1",
			Version:        7,
			Modified:       remoteModified,
			KeyFingerprint: "sha256:new",
			ItemCount:      10,
		},
	}

	err := updateCatalogueAfterReceive(&storage, nil, remoteCat, "vault1")
	require.NoError(t, err)
	require.NotNil(t, stored)

	entry := stored.VaultCatalogue["vault1"]
	assert.Equal(t, 7, entry.Version)
	assert.Equal(t, "sha256:new", entry.KeyFingerprint)
	assert.True(t, remoteModified.Equal(entry.Modified))
	assert.Equal(t, 10, entry.ItemCount)
	assert.False(t, stored.Modified.IsZero(), "app state Modified should be updated")
}

func TestUpdateCatalogueAfterReceive_CreatesNewEntry(t *testing.T) {
	createdTime := time.Now().UTC().Add(-24 * time.Hour)

	var stored *usm.AppState
	storage := usm.StorageMock{}
	storage.OnLoadAppState = func() (*usm.AppState, error) {
		return &usm.AppState{
			VaultCatalogue: map[string]*usm.VaultEntry{},
		}, nil
	}
	storage.OnStoreAppState = func(s *usm.AppState) error {
		stored = s
		return nil
	}

	remoteCat := map[string]*usm.VaultEntry{
		"new-vault": {
			Name:           "new-vault",
			Version:        2,
			Modified:       time.Now().UTC(),
			KeyFingerprint: "sha256:abc",
			ItemCount:      3,
			Created:        createdTime,
		},
	}

	err := updateCatalogueAfterReceive(&storage, nil, remoteCat, "new-vault")
	require.NoError(t, err)
	require.NotNil(t, stored)

	entry := stored.VaultCatalogue["new-vault"]
	assert.Equal(t, "new-vault", entry.Name)
	assert.Equal(t, 2, entry.Version)
	assert.Equal(t, "sha256:abc", entry.KeyFingerprint)
	assert.Equal(t, 3, entry.ItemCount)
	assert.Contains(t, entry.StorageLocation, "storage/new-vault")
	assert.True(t, createdTime.Equal(entry.Created), "Created should be copied from remote")
}

func TestUpdateCatalogueAfterReceive_MissingRemoteEntry(t *testing.T) {
	storage := &usm.StorageMock{
		AppStateStorageMock: usm.AppStateStorageMock{
			OnLoadAppState: func() (*usm.AppState, error) {
				return &usm.AppState{}, nil
			},
		},
	}

	remoteCat := map[string]*usm.VaultEntry{}

	err := updateCatalogueAfterReceive(storage, nil, remoteCat, "missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in remote catalogue")
}

func TestUpdateCatalogueAfterReceive_LoadError(t *testing.T) {
	storage := &usm.StorageMock{
		AppStateStorageMock: usm.AppStateStorageMock{
			OnLoadAppState: func() (*usm.AppState, error) {
				return nil, fmt.Errorf("disk read error")
			},
		},
	}

	remoteCat := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 2},
	}

	err := updateCatalogueAfterReceive(storage, nil, remoteCat, "vault1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not load app state")
}

func TestUpdateCatalogueAfterReceive_StoreError(t *testing.T) {
	storage := &usm.StorageMock{
		AppStateStorageMock: usm.AppStateStorageMock{
			OnLoadAppState: func() (*usm.AppState, error) {
				return &usm.AppState{
					VaultCatalogue: map[string]*usm.VaultEntry{
						"vault1": {Name: "vault1", Version: 1},
					},
				}, nil
			},
			OnStoreAppState: func(s *usm.AppState) error {
				return fmt.Errorf("disk full")
			},
		},
	}

	remoteCat := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 5},
	}

	err := updateCatalogueAfterReceive(storage, nil, remoteCat, "vault1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not store app state")
}

func TestUpdateCatalogueAfterReceive_WithLegacyViracochanCatalogue(t *testing.T) {
	root := copyConfigFixtureTree(t, filepath.Join("..", "config", "testdata", "viracochan-v0.1.1-catalogue"))

	catMgr, err := config.NewCatalogueManager(root)
	require.NoError(t, err)

	remoteModified := time.Date(2025, 9, 2, 10, 0, 0, 0, time.UTC)
	remoteCat := map[string]*usm.VaultEntry{
		"personal": {
			Name:           "personal",
			Version:        3,
			Modified:       remoteModified,
			KeyFingerprint: "fp-personal-v3",
			ItemCount:      4,
			ChainCS:        "remote-chain-cs-v3",
		},
	}

	err = updateCatalogueAfterReceive(&usm.StorageMock{}, catMgr, remoteCat, "personal")
	require.NoError(t, err)

	catalogue, err := catMgr.LoadCatalogue(context.Background())
	require.NoError(t, err)
	require.Contains(t, catalogue, "personal")

	entry := catalogue["personal"]
	assert.Equal(t, 3, entry.Version)
	assert.Equal(t, "fp-personal-v3", entry.KeyFingerprint)
	assert.Equal(t, 4, entry.ItemCount)
	assert.Equal(t, "remote-chain-cs-v3", entry.ChainCS)
	assert.True(t, remoteModified.Equal(entry.Modified))

	assert.Equal(t, uint64(5), catMgr.GetChainVersion(context.Background()))
}

func shouldSkipStart(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "operation not permitted")
}

func copyConfigFixtureTree(t *testing.T, src string) string {
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

		return copyConfigFixtureFile(path, target)
	})
	require.NoError(t, err)

	return dst
}

func copyConfigFixtureFile(src, dst string) error {
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
