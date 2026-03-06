package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// TrustedPeer represents a peer device that has been paired and trusted for synchronization.
type TrustedPeer struct {
	InstanceID string    `json:"instance_id"`
	Label      string    `json:"label"`
	PairedAt   time.Time `json:"paired_at"`
	LastSync   time.Time `json:"last_sync,omitempty"`
}

// trustStoreData is the on-disk JSON structure for the trust store.
type trustStoreData struct {
	Peers map[string]TrustedPeer `json:"peers"`
}

// TrustStore manages trusted peers for sync operations, persisting them to a JSON file.
type TrustStore struct {
	mu   sync.RWMutex
	path string
	data trustStoreData
}

// NewTrustStore creates or loads a TrustStore from the given file path.
// If the file does not exist, an empty store is created in memory; it will be
// written to disk on the first mutation.
func NewTrustStore(path string) (*TrustStore, error) {
	ts := &TrustStore{
		path: path,
		data: trustStoreData{Peers: make(map[string]TrustedPeer)},
	}

	raw, err := os.ReadFile(path) //nolint:gosec // path is derived from app config, not user input
	if err == nil {
		if err := json.Unmarshal(raw, &ts.data); err != nil {
			return nil, fmt.Errorf("could not parse trust store: %w", err)
		}
		if ts.data.Peers == nil {
			ts.data.Peers = make(map[string]TrustedPeer)
		}
	}

	return ts, nil
}

// Add registers a trusted peer and persists the change to disk.
func (ts *TrustStore) Add(peerID string, peer TrustedPeer) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.data.Peers[peerID] = peer
	return ts.persist()
}

// Remove deletes a trusted peer and persists the change to disk.
func (ts *TrustStore) Remove(peerID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.data.Peers, peerID)
	return ts.persist()
}

// IsTrusted reports whether the given peer ID exists in the trust store.
func (ts *TrustStore) IsTrusted(peerID string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	_, ok := ts.data.Peers[peerID]
	return ok
}

// Get returns the TrustedPeer for the given ID and a boolean indicating whether it was found.
func (ts *TrustStore) Get(peerID string) (TrustedPeer, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	p, ok := ts.data.Peers[peerID]
	return p, ok
}

// UpdateLastSync sets the LastSync timestamp for an existing peer and persists the change.
func (ts *TrustStore) UpdateLastSync(peerID string, t time.Time) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	p, ok := ts.data.Peers[peerID]
	if !ok {
		return fmt.Errorf("peer %q not found in trust store", peerID)
	}
	p.LastSync = t
	ts.data.Peers[peerID] = p
	return ts.persist()
}

// List returns a shallow copy of all trusted peers.
func (ts *TrustStore) List() map[string]TrustedPeer {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	out := make(map[string]TrustedPeer, len(ts.data.Peers))
	for k, v := range ts.data.Peers {
		out[k] = v
	}
	return out
}

// persist writes the current trust store data to disk as indented JSON.
// ATTN: must be called while holding ts.mu write lock.
func (ts *TrustStore) persist() error {
	raw, err := json.MarshalIndent(ts.data, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal trust store: %w", err)
	}
	if err := os.WriteFile(ts.path, raw, 0o600); err != nil {
		return fmt.Errorf("could not write trust store: %w", err)
	}
	return nil
}
