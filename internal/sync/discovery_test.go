package sync

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPeerList_AddAndGet(t *testing.T) {
	pl := NewPeerList()

	pl.Update(PeerInfo{
		ID:         "peer-1",
		Label:      "Test",
		InstanceID: "inst-1",
		Status:     PeerStatusDiscovered,
	})

	peers := pl.All()
	assert.Len(t, peers, 1)
	assert.Equal(t, "peer-1", peers[0].ID)
}

func TestPeerList_UpdateExisting(t *testing.T) {
	pl := NewPeerList()

	pl.Update(PeerInfo{ID: "peer-1", Label: "Old", Status: PeerStatusDiscovered})
	pl.Update(PeerInfo{ID: "peer-1", Label: "New", Status: PeerStatusConnected})

	peers := pl.All()
	assert.Len(t, peers, 1)
	assert.Equal(t, "New", peers[0].Label)
	assert.Equal(t, PeerStatusConnected, peers[0].Status)
}

func TestPeerList_Remove(t *testing.T) {
	pl := NewPeerList()
	pl.Update(PeerInfo{ID: "peer-1", Label: "Test"})
	pl.Remove("peer-1")
	assert.Empty(t, pl.All())
}

func TestPeerList_OnChange(t *testing.T) {
	pl := NewPeerList()

	var mu sync.Mutex
	var received []PeerInfo
	pl.OnChange(func(peers []PeerInfo) {
		mu.Lock()
		defer mu.Unlock()
		received = peers
	})

	pl.Update(PeerInfo{ID: "peer-1", Label: "Test"})

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, received, 1)
}

func TestPeerList_Sweep_RemovesStale(t *testing.T) {
	pl := NewPeerList()
	pl.Update(PeerInfo{ID: "fresh", Label: "Fresh", LastSeen: time.Now()})
	pl.Update(PeerInfo{ID: "stale", Label: "Stale", LastSeen: time.Now().Add(-5 * time.Minute)})

	pl.Sweep(3 * time.Minute)

	peers := pl.All()
	assert.Len(t, peers, 1)
	assert.Equal(t, "fresh", peers[0].ID)
}

func TestPeerList_Sweep_KeepsZeroLastSeen(t *testing.T) {
	pl := NewPeerList()
	// Peers with zero LastSeen (e.g. manually added) should not be swept
	pl.Update(PeerInfo{ID: "manual", Label: "Manual"})
	pl.Update(PeerInfo{ID: "stale", Label: "Stale", LastSeen: time.Now().Add(-5 * time.Minute)})

	pl.Sweep(3 * time.Minute)

	peers := pl.All()
	assert.Len(t, peers, 1)
	assert.Equal(t, "manual", peers[0].ID)
}

func TestPeerList_Sweep_NotifiesOnChange(t *testing.T) {
	pl := NewPeerList()
	pl.Update(PeerInfo{ID: "stale", Label: "Stale", LastSeen: time.Now().Add(-5 * time.Minute)})

	notified := false
	pl.OnChange(func(_ []PeerInfo) { notified = true })
	pl.Sweep(3 * time.Minute)

	assert.True(t, notified)
}

func TestPeerList_Sweep_NoNotifyWhenNothingRemoved(t *testing.T) {
	pl := NewPeerList()
	pl.Update(PeerInfo{ID: "fresh", Label: "Fresh", LastSeen: time.Now()})

	notified := false
	pl.OnChange(func(_ []PeerInfo) { notified = true })
	pl.Sweep(3 * time.Minute)

	assert.False(t, notified)
}

func TestPeerList_ConcurrentAccess(t *testing.T) {
	pl := NewPeerList()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			pl.Update(PeerInfo{ID: "peer-" + string(rune('A'+n%26))})
			_ = pl.All()
		}(i)
	}

	wg.Wait()
}
