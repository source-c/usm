package sync

import (
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerList is a thread-safe container for discovered peers
type PeerList struct {
	mu       sync.RWMutex
	peers    map[string]PeerInfo
	onChange func([]PeerInfo)
}

// NewPeerList creates a new empty peer list
func NewPeerList() *PeerList {
	return &PeerList{
		peers: make(map[string]PeerInfo),
	}
}

// Update adds or updates a peer in the list
func (pl *PeerList) Update(info PeerInfo) {
	pl.mu.Lock()
	pl.peers[info.ID] = info
	pl.mu.Unlock()
	pl.notify()
}

// Remove removes a peer from the list
func (pl *PeerList) Remove(id string) {
	pl.mu.Lock()
	delete(pl.peers, id)
	pl.mu.Unlock()
	pl.notify()
}

// All returns a snapshot of all peers
func (pl *PeerList) All() []PeerInfo {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	result := make([]PeerInfo, 0, len(pl.peers))
	for _, p := range pl.peers {
		result = append(result, p)
	}
	return result
}

// SetTrusted updates the Trusted flag for a peer and triggers notification
func (pl *PeerList) SetTrusted(id string, trusted bool, label string) {
	pl.mu.Lock()
	if p, ok := pl.peers[id]; ok {
		p.Trusted = trusted
		if label != "" {
			p.Label = label
		}
		pl.peers[id] = p
	}
	pl.mu.Unlock()
	pl.notify()
}

// OnChange sets a callback invoked whenever the peer list changes
func (pl *PeerList) OnChange(fn func([]PeerInfo)) {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	pl.onChange = fn
}

// Sweep removes peers not seen within maxAge and notifies on change
func (pl *PeerList) Sweep(maxAge time.Duration) {
	pl.mu.Lock()
	cutoff := time.Now().Add(-maxAge)
	changed := false
	for id, p := range pl.peers {
		if !p.LastSeen.IsZero() && p.LastSeen.Before(cutoff) {
			delete(pl.peers, id)
			changed = true
		}
	}
	pl.mu.Unlock()
	if changed {
		pl.notify()
	}
}

func (pl *PeerList) notify() {
	pl.mu.RLock()
	fn := pl.onChange
	pl.mu.RUnlock()
	if fn != nil {
		fn(pl.All())
	}
}

// discoveryNotifee implements mdns.Notifee for mDNS peer discovery
type discoveryNotifee struct {
	selfID     peer.ID
	host       host.Host
	peerList   *PeerList
	trustStore *TrustStore
}

// HandlePeerFound is called by mDNS when a peer is discovered on the LAN
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if pi.ID == n.selfID {
		return
	}

	// ATTN: store the peer's addresses so libp2p can dial them later.
	// 2-minute TTL — mDNS re-discovers active peers every ~10s, so this is
	// long enough to survive gaps but short enough to expire dead peers.
	n.host.Peerstore().AddAddrs(pi.ID, pi.Addrs, 2*time.Minute)
	log.Printf("mDNS: discovered peer %s at %v", pi.ID.ShortString(), pi.Addrs)

	peerID := pi.ID.String()
	trusted := n.trustStore.IsTrusted(peerID)
	label := peerID
	if len(peerID) > 12 {
		label = peerID[:12] + "..."
	}
	if trusted {
		if p, ok := n.trustStore.Get(peerID); ok {
			label = p.Label
		}
	}

	n.peerList.Update(PeerInfo{
		ID:       peerID,
		Label:    label,
		Trusted:  trusted,
		Status:   PeerStatusDiscovered,
		LastSeen: time.Now(),
	})
}
