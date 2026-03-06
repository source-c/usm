package sync

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrustStore_AddAndCheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trusted_peers.json")

	ts, err := NewTrustStore(path)
	require.NoError(t, err)

	now := time.Now().UTC()
	err = ts.Add("peer-123", TrustedPeer{
		InstanceID: "inst-abc",
		Label:      "Test Peer",
		PairedAt:   now,
	})
	require.NoError(t, err)

	assert.True(t, ts.IsTrusted("peer-123"))
	assert.False(t, ts.IsTrusted("peer-unknown"))
}

func TestTrustStore_PersistAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trusted_peers.json")

	ts1, err := NewTrustStore(path)
	require.NoError(t, err)

	err = ts1.Add("peer-123", TrustedPeer{
		InstanceID: "inst-abc",
		Label:      "Test Peer",
		PairedAt:   time.Now().UTC(),
	})
	require.NoError(t, err)

	ts2, err := NewTrustStore(path)
	require.NoError(t, err)
	assert.True(t, ts2.IsTrusted("peer-123"))
}

func TestTrustStore_Remove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trusted_peers.json")

	ts, err := NewTrustStore(path)
	require.NoError(t, err)

	_ = ts.Add("peer-123", TrustedPeer{InstanceID: "inst-abc", Label: "Test", PairedAt: time.Now().UTC()})
	assert.True(t, ts.IsTrusted("peer-123"))

	err = ts.Remove("peer-123")
	require.NoError(t, err)
	assert.False(t, ts.IsTrusted("peer-123"))
}

func TestTrustStore_UpdateLastSync(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trusted_peers.json")

	ts, err := NewTrustStore(path)
	require.NoError(t, err)

	_ = ts.Add("peer-123", TrustedPeer{InstanceID: "inst-abc", Label: "Test", PairedAt: time.Now().UTC()})

	syncTime := time.Now().UTC()
	err = ts.UpdateLastSync("peer-123", syncTime)
	require.NoError(t, err)

	peer, ok := ts.Get("peer-123")
	require.True(t, ok)
	assert.Equal(t, syncTime.Unix(), peer.LastSync.Unix())
}

func TestTrustStore_List(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trusted_peers.json")

	ts, err := NewTrustStore(path)
	require.NoError(t, err)

	_ = ts.Add("peer-1", TrustedPeer{InstanceID: "inst-1", Label: "Peer 1", PairedAt: time.Now().UTC()})
	_ = ts.Add("peer-2", TrustedPeer{InstanceID: "inst-2", Label: "Peer 2", PairedAt: time.Now().UTC()})

	peers := ts.List()
	assert.Len(t, peers, 2)
}
