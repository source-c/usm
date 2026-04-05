package sync

import (
	"fmt"

	"apps.z7.ai/usm/internal/usm"
)

// Negotiate compares local and remote vault catalogues and produces a sync plan.
// Tier 1 (auto): one side is strictly newer or vault exists only on one side.
// Tier 2 (conflict): both sides modified the same vault (same version, different timestamps).
//
// In Strict mode, vaults that exist on both sides with different key fingerprints
// are treated as conflicts rather than auto-transfers, preventing sync of data
// encrypted with a key the other side does not possess.
func Negotiate(local, remote map[string]*usm.VaultEntry, opts ...NegotiateOption) SyncPlanPayload {
	cfg := negotiateConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	plan := SyncPlanPayload{}

	seen := make(map[string]bool)

	for name, le := range local {
		seen[name] = true
		re, exists := remote[name]

		if !exists {
			plan.Auto = append(plan.Auto, VaultTransfer{
				VaultName: name,
				Direction: SyncDirectionPush,
				Tier:      1,
			})
			continue
		}

		// ATTN: in Strict mode, reject vaults whose key fingerprints diverged.
		// This guards against syncing data encrypted with a re-keyed vault that
		// the other side cannot decrypt.
		if cfg.strict && fingerprintMismatch(le, re) {
			plan.Conflicts = append(plan.Conflicts, VaultConflict{
				VaultName:      name,
				LocalVersion:   le.Version,
				RemoteVersion:  re.Version,
				LocalModified:  le.Modified.Unix(),
				RemoteModified: re.Modified.Unix(),
			})
			continue
		}

		transfer := compareEntries(name, le, re)
		if transfer != nil {
			plan.Auto = append(plan.Auto, *transfer)
		}

		conflict := detectConflict(name, le, re)
		if conflict != nil {
			plan.Conflicts = append(plan.Conflicts, *conflict)
		}
	}

	for name := range remote {
		if seen[name] {
			continue
		}
		plan.Auto = append(plan.Auto, VaultTransfer{
			VaultName: name,
			Direction: SyncDirectionPull,
			Tier:      1,
		})
	}

	return plan
}

// negotiateConfig holds options for the Negotiate function.
type negotiateConfig struct {
	strict bool
}

// NegotiateOption configures the Negotiate function.
type NegotiateOption func(*negotiateConfig)

// WithStrictMode enables key-fingerprint verification during negotiation.
func WithStrictMode() NegotiateOption {
	return func(c *negotiateConfig) { c.strict = true }
}

// fingerprintMismatch returns true when both entries carry a fingerprint but they differ.
// Vaults with an empty fingerprint (e.g. migrated entries) are not rejected.
func fingerprintMismatch(local, remote *usm.VaultEntry) bool {
	return local.KeyFingerprint != "" && remote.KeyFingerprint != "" &&
		local.KeyFingerprint != remote.KeyFingerprint
}

// VerifyPlanConsistency checks that an initiator's sync plan is consistent with
// the responder's independently computed plan. This prevents a malicious initiator
// from requesting transfers the responder's own negotiation would not allow.
//
// ATTN: the initiator's directions are from the initiator's perspective, so
// Push (initiator→responder) must correspond to Pull in the responder's plan.
func VerifyPlanConsistency(initiatorPlan, responderPlan SyncPlanPayload) error {
	responderAuto := make(map[string]SyncDirection, len(responderPlan.Auto))
	for _, t := range responderPlan.Auto {
		responderAuto[t.VaultName] = t.Direction
	}

	for _, t := range initiatorPlan.Auto {
		respDir, ok := responderAuto[t.VaultName]
		if !ok {
			return fmt.Errorf("initiator plan contains vault %q not in responder plan", t.VaultName)
		}
		expected := flipDirection(t.Direction)
		if respDir != expected {
			return fmt.Errorf("direction mismatch for vault %q: initiator=%s, responder=%s (expected %s)",
				t.VaultName, t.Direction, respDir, expected)
		}
		delete(responderAuto, t.VaultName)
	}

	for name := range responderAuto {
		return fmt.Errorf("responder plan contains vault %q not in initiator plan", name)
	}

	initiatorConflicts := make(map[string]bool, len(initiatorPlan.Conflicts))
	for _, c := range initiatorPlan.Conflicts {
		initiatorConflicts[c.VaultName] = true
	}
	responderConflicts := make(map[string]bool, len(responderPlan.Conflicts))
	for _, c := range responderPlan.Conflicts {
		responderConflicts[c.VaultName] = true
	}
	for name := range initiatorConflicts {
		if !responderConflicts[name] {
			return fmt.Errorf("initiator conflict %q not in responder conflicts", name)
		}
	}
	for name := range responderConflicts {
		if !initiatorConflicts[name] {
			return fmt.Errorf("responder conflict %q not in initiator conflicts", name)
		}
	}

	return nil
}

// flipDirection returns the opposite sync direction
func flipDirection(d SyncDirection) SyncDirection {
	switch d {
	case SyncDirectionPush:
		return SyncDirectionPull
	case SyncDirectionPull:
		return SyncDirectionPush
	default:
		return d
	}
}

// compareEntries determines whether a vault needs to be pushed or pulled based on version comparison.
// When both sides carry a ChainCS checksum, matching checksums prove identical catalogue state and
// the vault is skipped regardless of version/timestamp values.
func compareEntries(name string, local, remote *usm.VaultEntry) *VaultTransfer {
	if local.ChainCS != "" && remote.ChainCS != "" && local.ChainCS == remote.ChainCS {
		return nil
	}

	if local.Version == remote.Version && local.Modified.Equal(remote.Modified) {
		return nil
	}

	if local.Version > remote.Version {
		return &VaultTransfer{VaultName: name, Direction: SyncDirectionPush, Tier: 1}
	}
	if remote.Version > local.Version {
		return &VaultTransfer{VaultName: name, Direction: SyncDirectionPull, Tier: 1}
	}

	return nil
}

// detectConflict identifies cases where both sides have the same version but different modification timestamps.
// Matching ChainCS checksums override the conflict — the catalogue state is provably identical.
func detectConflict(name string, local, remote *usm.VaultEntry) *VaultConflict {
	if local.Version != remote.Version {
		return nil
	}
	if local.ChainCS != "" && remote.ChainCS != "" && local.ChainCS == remote.ChainCS {
		return nil
	}
	if local.ChainCS != "" && remote.ChainCS != "" && local.ChainCS != remote.ChainCS {
		return &VaultConflict{
			VaultName:      name,
			LocalVersion:   local.Version,
			RemoteVersion:  remote.Version,
			LocalModified:  local.Modified.Unix(),
			RemoteModified: remote.Modified.Unix(),
		}
	}
	if local.ItemCount != remote.ItemCount {
		return &VaultConflict{
			VaultName:      name,
			LocalVersion:   local.Version,
			RemoteVersion:  remote.Version,
			LocalModified:  local.Modified.Unix(),
			RemoteModified: remote.Modified.Unix(),
		}
	}
	if local.Modified.Equal(remote.Modified) {
		return nil
	}

	return &VaultConflict{
		VaultName:      name,
		LocalVersion:   local.Version,
		RemoteVersion:  remote.Version,
		LocalModified:  local.Modified.Unix(),
		RemoteModified: remote.Modified.Unix(),
	}
}
