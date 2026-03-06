package sync

import (
	"context"
	"path/filepath"
	"testing"
	"time"

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
	_ = svc.Start(ctx)
	_ = svc.Stop()
	err = svc.Stop() // second stop should not panic
	assert.NoError(t, err)
}
