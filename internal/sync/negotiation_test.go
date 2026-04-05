package sync

import (
	"testing"
	"time"

	"apps.z7.ai/usm/internal/usm"
	"github.com/stretchr/testify/assert"
)

func TestNegotiate_BothEmpty(t *testing.T) {
	local := map[string]*usm.VaultEntry{}
	remote := map[string]*usm.VaultEntry{}
	plan := Negotiate(local, remote)
	assert.Empty(t, plan.Auto)
	assert.Empty(t, plan.Conflicts)
}

func TestNegotiate_LocalOnly_Push(t *testing.T) {
	now := time.Now().UTC()
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 1, Modified: now},
	}
	remote := map[string]*usm.VaultEntry{}
	plan := Negotiate(local, remote)

	assert.Len(t, plan.Auto, 1)
	assert.Equal(t, SyncDirectionPush, plan.Auto[0].Direction)
	assert.Equal(t, 1, plan.Auto[0].Tier)
}

func TestNegotiate_RemoteOnly_Pull(t *testing.T) {
	now := time.Now().UTC()
	local := map[string]*usm.VaultEntry{}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 1, Modified: now},
	}
	plan := Negotiate(local, remote)

	assert.Len(t, plan.Auto, 1)
	assert.Equal(t, SyncDirectionPull, plan.Auto[0].Direction)
	assert.Equal(t, 1, plan.Auto[0].Tier)
}

func TestNegotiate_SameVersion_Skip(t *testing.T) {
	now := time.Now().UTC()
	local := map[string]*usm.VaultEntry{"vault1": {Name: "vault1", Version: 3, Modified: now}}
	remote := map[string]*usm.VaultEntry{"vault1": {Name: "vault1", Version: 3, Modified: now}}
	plan := Negotiate(local, remote)

	assert.Empty(t, plan.Auto)
	assert.Empty(t, plan.Conflicts)
}

func TestNegotiate_LocalNewer_Push(t *testing.T) {
	now := time.Now().UTC()
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now},
	}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 2, Modified: now.Add(-time.Hour)},
	}
	plan := Negotiate(local, remote)

	assert.Len(t, plan.Auto, 1)
	assert.Equal(t, SyncDirectionPush, plan.Auto[0].Direction)
	assert.Equal(t, 1, plan.Auto[0].Tier)
}

func TestNegotiate_RemoteNewer_Pull(t *testing.T) {
	now := time.Now().UTC()
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 2, Modified: now.Add(-time.Hour)},
	}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now},
	}
	plan := Negotiate(local, remote)

	assert.Len(t, plan.Auto, 1)
	assert.Equal(t, SyncDirectionPull, plan.Auto[0].Direction)
	assert.Equal(t, 1, plan.Auto[0].Tier)
}

func TestNegotiate_BothModified_Conflict(t *testing.T) {
	now := time.Now().UTC()
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now},
	}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now.Add(time.Minute)},
	}
	plan := Negotiate(local, remote)

	assert.Empty(t, plan.Auto)
	assert.Len(t, plan.Conflicts, 1)
	assert.Equal(t, "vault1", plan.Conflicts[0].VaultName)
}

func TestNegotiate_Strict_FingerprintMismatch_Conflict(t *testing.T) {
	now := time.Now().UTC()
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now, KeyFingerprint: "sha256:aaa"},
	}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 2, Modified: now.Add(-time.Hour), KeyFingerprint: "sha256:bbb"},
	}

	// Relaxed: fingerprint mismatch is ignored, auto-push since local is newer
	relaxedPlan := Negotiate(local, remote)
	assert.Len(t, relaxedPlan.Auto, 1)
	assert.Equal(t, SyncDirectionPush, relaxedPlan.Auto[0].Direction)
	assert.Empty(t, relaxedPlan.Conflicts)

	// Strict: fingerprint mismatch becomes a conflict, no auto-transfer
	strictPlan := Negotiate(local, remote, WithStrictMode())
	assert.Empty(t, strictPlan.Auto)
	assert.Len(t, strictPlan.Conflicts, 1)
	assert.Equal(t, "vault1", strictPlan.Conflicts[0].VaultName)
}

func TestNegotiate_Strict_FingerprintMatch_AllowsTransfer(t *testing.T) {
	now := time.Now().UTC()
	fp := "sha256:same"
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now, KeyFingerprint: fp},
	}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 2, Modified: now.Add(-time.Hour), KeyFingerprint: fp},
	}

	plan := Negotiate(local, remote, WithStrictMode())
	assert.Len(t, plan.Auto, 1)
	assert.Equal(t, SyncDirectionPush, plan.Auto[0].Direction)
	assert.Empty(t, plan.Conflicts)
}

func TestNegotiate_Strict_EmptyFingerprint_AllowsTransfer(t *testing.T) {
	now := time.Now().UTC()
	// Migrated entries may have empty fingerprints — should not block sync
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now, KeyFingerprint: ""},
	}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 2, Modified: now.Add(-time.Hour), KeyFingerprint: "sha256:bbb"},
	}

	plan := Negotiate(local, remote, WithStrictMode())
	assert.Len(t, plan.Auto, 1)
	assert.Equal(t, SyncDirectionPush, plan.Auto[0].Direction)
	assert.Empty(t, plan.Conflicts)
}

func TestVerifyPlanConsistency_MatchingPlans(t *testing.T) {
	initiator := SyncPlanPayload{
		Auto: []VaultTransfer{
			{VaultName: "vault1", Direction: SyncDirectionPush},
			{VaultName: "vault2", Direction: SyncDirectionPull},
		},
	}
	responder := SyncPlanPayload{
		Auto: []VaultTransfer{
			{VaultName: "vault1", Direction: SyncDirectionPull},
			{VaultName: "vault2", Direction: SyncDirectionPush},
		},
	}
	assert.NoError(t, VerifyPlanConsistency(initiator, responder))
}

func TestVerifyPlanConsistency_EmptyPlans(t *testing.T) {
	assert.NoError(t, VerifyPlanConsistency(SyncPlanPayload{}, SyncPlanPayload{}))
}

func TestVerifyPlanConsistency_InitiatorExtraVault(t *testing.T) {
	initiator := SyncPlanPayload{
		Auto: []VaultTransfer{
			{VaultName: "vault1", Direction: SyncDirectionPush},
			{VaultName: "extra", Direction: SyncDirectionPush},
		},
	}
	responder := SyncPlanPayload{
		Auto: []VaultTransfer{
			{VaultName: "vault1", Direction: SyncDirectionPull},
		},
	}
	err := VerifyPlanConsistency(initiator, responder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extra")
}

func TestVerifyPlanConsistency_ResponderExtraVault(t *testing.T) {
	initiator := SyncPlanPayload{
		Auto: []VaultTransfer{
			{VaultName: "vault1", Direction: SyncDirectionPush},
		},
	}
	responder := SyncPlanPayload{
		Auto: []VaultTransfer{
			{VaultName: "vault1", Direction: SyncDirectionPull},
			{VaultName: "extra", Direction: SyncDirectionPush},
		},
	}
	err := VerifyPlanConsistency(initiator, responder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extra")
}

func TestVerifyPlanConsistency_DirectionMismatch(t *testing.T) {
	initiator := SyncPlanPayload{
		Auto: []VaultTransfer{
			{VaultName: "vault1", Direction: SyncDirectionPush},
		},
	}
	responder := SyncPlanPayload{
		Auto: []VaultTransfer{
			{VaultName: "vault1", Direction: SyncDirectionPush}, // should be Pull
		},
	}
	err := VerifyPlanConsistency(initiator, responder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "direction mismatch")
}

func TestVerifyPlanConsistency_ConflictMismatch(t *testing.T) {
	initiator := SyncPlanPayload{
		Conflicts: []VaultConflict{{VaultName: "vault1"}},
	}
	responder := SyncPlanPayload{}
	err := VerifyPlanConsistency(initiator, responder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vault1")
}

func TestVerifyPlanConsistency_MatchingConflicts(t *testing.T) {
	initiator := SyncPlanPayload{
		Conflicts: []VaultConflict{{VaultName: "vault1"}},
	}
	responder := SyncPlanPayload{
		Conflicts: []VaultConflict{{VaultName: "vault1"}},
	}
	assert.NoError(t, VerifyPlanConsistency(initiator, responder))
}

func TestNegotiate_ChainCS_MatchingChecksum_Skips(t *testing.T) {
	now := time.Now().UTC()
	// Different versions and timestamps, but matching ChainCS → provably in sync
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now, ChainCS: "sha256:abc123"},
	}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 5, Modified: now.Add(time.Hour), ChainCS: "sha256:abc123"},
	}
	plan := Negotiate(local, remote)

	assert.Empty(t, plan.Auto, "matching ChainCS should skip regardless of version/timestamp")
	assert.Empty(t, plan.Conflicts, "matching ChainCS should not conflict")
}

func TestNegotiate_ChainCS_DifferentChecksum_VersionWins(t *testing.T) {
	now := time.Now().UTC()
	// Both have ChainCS but they differ → fall back to version comparison
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now, ChainCS: "sha256:aaa"},
	}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 5, Modified: now, ChainCS: "sha256:bbb"},
	}
	plan := Negotiate(local, remote)

	assert.Len(t, plan.Auto, 1)
	assert.Equal(t, SyncDirectionPull, plan.Auto[0].Direction)
}

func TestNegotiate_ChainCS_EmptyOnOneSide_Fallback(t *testing.T) {
	now := time.Now().UTC()
	// Only one side has ChainCS → fall back to version/timestamp comparison
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now, ChainCS: "sha256:aaa"},
	}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now}, // no ChainCS
	}
	plan := Negotiate(local, remote)

	assert.Empty(t, plan.Auto)
	assert.Empty(t, plan.Conflicts)
}

func TestNegotiate_ChainCS_PreventsConflict(t *testing.T) {
	now := time.Now().UTC()
	// Same version but different timestamps → would be a conflict, but matching ChainCS prevents it
	local := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now, ChainCS: "sha256:same"},
	}
	remote := map[string]*usm.VaultEntry{
		"vault1": {Name: "vault1", Version: 3, Modified: now.Add(time.Minute), ChainCS: "sha256:same"},
	}
	plan := Negotiate(local, remote)

	assert.Empty(t, plan.Auto)
	assert.Empty(t, plan.Conflicts, "matching ChainCS should prevent conflict despite different timestamps")
}
