package sync

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessage_RoundTrip(t *testing.T) {
	msg := Message{
		Action:  ActionSyncRequest,
		Payload: json.RawMessage(`{"instance_id":"abc"}`),
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, ActionSyncRequest, decoded.Action)
}

func TestPeerStatus_String(t *testing.T) {
	assert.Equal(t, "discovered", PeerStatusDiscovered.String())
	assert.Equal(t, "connected", PeerStatusConnected.String())
	assert.Equal(t, "syncing", PeerStatusSyncing.String())
	assert.Equal(t, "error", PeerStatusError.String())
}

func TestColorFromInstanceID(t *testing.T) {
	c1 := ColorFromInstanceID("instance-aaa")
	c2 := ColorFromInstanceID("instance-bbb")
	c3 := ColorFromInstanceID("instance-aaa")

	assert.Equal(t, c1, c3, "same ID should produce same color")
	_ = c2
}

func TestVaultSyncDirection(t *testing.T) {
	assert.Equal(t, "push", SyncDirectionPush.String())
	assert.Equal(t, "pull", SyncDirectionPull.String())
	assert.Equal(t, "skip", SyncDirectionSkip.String())
}

func TestPeerColor(t *testing.T) {
	color := PeerColor("test-instance-id")
	assert.NotEmpty(t, color)
	assert.Equal(t, "#", color[:1])
}
