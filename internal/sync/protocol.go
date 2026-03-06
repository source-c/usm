package sync

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"time"

	"apps.z7.ai/usm/internal/usm"
)

const (
	PairProtocol = "/usm/pair/1.0"
	SyncProtocol = "/usm/sync/1.0"
	MDNSService  = "usm-sync/1.0"
)

const (
	ActionPairRequest  = "pair_request"
	ActionPairResponse = "pair_response"
	ActionPairResult   = "pair_result"
)

// PairRequestPayload is sent by the initiator to start a pairing session
type PairRequestPayload struct {
	Label string `json:"label"`
	Nonce string `json:"nonce"` // base64-encoded 32-byte random nonce for HMAC challenge
}

// PairResponsePayload is sent by the responder with HMAC proof of the entered code.
// ATTN: the code is never sent in plaintext — only HMAC(code, nonce) is transmitted,
// providing defense-in-depth even over the Noise-encrypted stream.
type PairResponsePayload struct {
	MAC   string `json:"mac"` // base64 HMAC-SHA256(code_bytes, nonce)
	Label string `json:"label"`
}

// PairResultPayload is sent by the initiator to confirm or reject the pairing
type PairResultPayload struct {
	Accepted bool `json:"accepted"`
}

const (
	ActionSyncRequest       = "sync_request"
	ActionSyncAccept        = "sync_accept"
	ActionSyncAbort         = "sync_abort"
	ActionChallenge         = "challenge"
	ActionChallengeResponse = "challenge_response"
	ActionCatalogue         = "catalogue"
	ActionSyncPlan          = "sync_plan"
	ActionSyncPlanResolved  = "sync_plan_resolved"
	ActionTransferBegin     = "transfer_begin"
	ActionTransferFile      = "transfer_file"
	ActionTransferEnd       = "transfer_end"
	ActionSyncComplete      = "sync_complete"
)

// Message is the wire format for sync protocol communication
type Message struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// PeerStatus represents the connection state of a discovered peer
type PeerStatus int

const (
	PeerStatusDiscovered PeerStatus = iota
	PeerStatusConnected
	PeerStatusSyncing
	PeerStatusError
)

func (s PeerStatus) String() string {
	switch s {
	case PeerStatusDiscovered:
		return "discovered"
	case PeerStatusConnected:
		return "connected"
	case PeerStatusSyncing:
		return "syncing"
	case PeerStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// PeerInfo holds the information displayed for a discovered peer
type PeerInfo struct {
	ID           string
	Label        string
	ColorHash    uint32
	InstanceID   string
	USMVersion   string
	ChainVersion uint64
	Vaults       []VaultSummary
	Trusted      bool
	Status       PeerStatus
	LastSeen     time.Time // updated on every mDNS re-discovery
}

// VaultSummary is the per-vault metadata exchanged during negotiation
type VaultSummary struct {
	Name           string `json:"name"`
	Version        int    `json:"version"`
	KeyFingerprint string `json:"key_fingerprint"`
	ItemCount      int    `json:"item_count"`
	Modified       int64  `json:"modified"`
}

// SyncDirection indicates which way vault data flows
type SyncDirection int

const (
	SyncDirectionSkip SyncDirection = iota
	SyncDirectionPush
	SyncDirectionPull
)

func (d SyncDirection) String() string {
	switch d {
	case SyncDirectionPush:
		return "push"
	case SyncDirectionPull:
		return "pull"
	default:
		return "skip"
	}
}

// VaultTransfer describes a planned vault transfer
type VaultTransfer struct {
	VaultName string        `json:"vault_name"`
	Direction SyncDirection `json:"direction"`
	Tier      int           `json:"tier"`
}

// SyncResult summarises the outcome of a sync operation
type SyncResult struct {
	Transfers []VaultTransfer `json:"transfers"`
	Skipped   []string        `json:"skipped,omitempty"`
	Errors    []string        `json:"errors,omitempty"`
}

// SyncRequestPayload is sent at handshake
type SyncRequestPayload struct {
	InstanceID   string `json:"instance_id"`
	USMVersion   string `json:"usm_version"`
	ChainVersion uint64 `json:"chain_version"`
}

// CataloguePayload carries the vault catalogue for negotiation
type CataloguePayload struct {
	VaultCatalogue map[string]*usm.VaultEntry `json:"vault_catalogue"`
}

// SyncPlanPayload describes the negotiated sync plan
type SyncPlanPayload struct {
	Auto      []VaultTransfer `json:"auto"`
	Conflicts []VaultConflict `json:"conflicts,omitempty"`
}

// VaultConflict describes a vault that diverged on both sides
type VaultConflict struct {
	VaultName      string `json:"vault_name"`
	LocalVersion   int    `json:"local_version"`
	RemoteVersion  int    `json:"remote_version"`
	LocalModified  int64  `json:"local_modified"`
	RemoteModified int64  `json:"remote_modified"`
}

// TransferFilePayload carries a single file during vault transfer
type TransferFilePayload struct {
	VaultName string `json:"vault_name"`
	FileName  string `json:"file_name"`
	Data      string `json:"data"` // base64-encoded
}

// ColorFromInstanceID derives a deterministic color index from an instance ID
func ColorFromInstanceID(instanceID string) uint32 {
	h := sha256.Sum256([]byte(instanceID))
	return binary.BigEndian.Uint32(h[:4])
}

// PeerColorPalette is a set of accessible, distinct colors for peer dots
var PeerColorPalette = []string{
	"#E53935", "#D81B60", "#8E24AA", "#5E35B1",
	"#3949AB", "#1E88E5", "#039BE5", "#00ACC1",
	"#00897B", "#43A047", "#7CB342", "#C0CA33",
	"#FDD835", "#FFB300", "#FB8C00", "#F4511E",
}

// PeerColor returns the hex color string for a given instance ID
func PeerColor(instanceID string) string {
	idx := ColorFromInstanceID(instanceID) % uint32(len(PeerColorPalette))
	return PeerColorPalette[idx]
}
