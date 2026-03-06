# LAN Synchronisation

USM supports vault synchronisation between instances running on the same local network. Sync is peer-to-peer with no cloud relay — data never leaves the LAN.

## Overview

When enabled, USM uses **mDNS** to discover other USM instances on the local network and **libp2p** (Noise-encrypted TCP streams) to transport vault data between them. Synchronisation is always user-initiated: discovery runs in the background, but no data moves until the user explicitly triggers a sync.

## Sync Modes

Configured in **Preferences > LAN Sync > Sync Mode**. Changes take effect immediately (no restart required).

| Mode | Behaviour |
|------|-----------|
| **Disabled** | No network activity. mDNS responder and libp2p host are shut down. Default. |
| **Relaxed** | Peers must be paired (6-character code exchange) before syncing. Once paired, sync proceeds without further verification. |
| **Strict** | Same as Relaxed, plus vault key-fingerprint verification during negotiation. Vaults whose fingerprints diverge (e.g. one side re-keyed) are treated as conflicts instead of auto-transferring, preventing sync of data encrypted with a key the other side cannot decrypt. |

## Architecture

```
internal/sync/
  identity.go      Ed25519 peer identity (generate / persist / load)
  truststore.go    Paired-peer trust store (JSON on disk)
  pairing.go       Pairing code generation and HMAC verification
  protocol.go      Wire types, message actions, peer color palette
  discovery.go     mDNS notifee and thread-safe PeerList
  negotiation.go   Catalogue comparison and sync plan generation
  transfer.go      Staging directory, atomic commit, crash recovery
  service.go       Lifecycle management (Start / Stop / Peers)
```

### Peer Identity

Each USM instance has a persistent **Ed25519 keypair** stored at `<storage_root>/peer.key` (base64-encoded libp2p marshalled key). The corresponding libp2p peer ID is the instance's network identity. Generated on first sync-service start.

### Discovery

An **mDNS responder** advertises the service name `usm-sync/1.0`. When another USM instance is detected, it appears in the discovery screen with a deterministic **colour dot** derived from `SHA-256(peerID) mod 16` over a 16-colour palette.

Peers are classified as:

- **Trusted** — previously paired; shown with their stored label.
- **Discovered** — visible on the network but not yet paired; shown with a truncated peer ID.

The peer list updates in real time via a callback subscription (`OnPeerChange`).

### Trust Store

Paired peers are persisted in `<storage_root>/trusted_peers.json`:

```json
{
  "peers": {
    "<peer_id>": {
      "instance_id": "...",
      "label": "MacBook Pro",
      "paired_at": "2026-03-05T10:00:00Z",
      "last_sync": "2026-03-05T12:30:00Z"
    }
  }
}
```

The trust store is thread-safe (RWMutex) and writes to disk on every mutation.

### Pairing Protocol

Pairing establishes mutual trust between two instances:

1. **Initiator** generates a 32-byte random secret and derives a 6-character code via HKDF-SHA256 with rejection sampling (alphabet: `23456789ABCDEFGHJKMNPQRSTUVWXYZ`, no ambiguous characters). The code is displayed on screen.
2. **Responder** enters the code on their device.
3. Both sides compute `HMAC-SHA256(secret, nonce)` and exchange MACs over a Noise-encrypted libp2p stream (`/usm/pair/1.0`).
4. If HMAC verification succeeds on both sides, each adds the other to its trust store.

Pairing codes have a 60-second TTL and are single-use.

### Negotiation

Before transferring data, both peers exchange vault catalogues and run the **Negotiate** algorithm:

| Condition | Result | Tier |
|-----------|--------|------|
| Vault exists only on one side | Push or Pull | 1 (auto) |
| One side has a higher version | Push or Pull toward the newer | 1 (auto) |
| Same version, same timestamp | Skip (already in sync) | — |
| Same version, different timestamps | Conflict | 2 (manual) |

- **Tier 1** transfers execute automatically.
- **Tier 2** conflicts require user resolution (keep local, accept remote, or skip).

### Transfer and Crash Recovery

Vault data is received into a **staging directory** (`.sync-staging/` inside the vault folder). Once all files arrive, an atomic commit sequence runs:

1. Back up the live vault into `.sync-backup/`.
2. Move staged files into the vault directory.
3. Remove staging and backup directories.

If the application crashes mid-sync, **orphan cleanup** runs on the next startup:

- If `.sync-staging/` exists, it is removed (incomplete receive).
- If `.sync-backup/` exists and `vault.age` is present in the vault directory, the backup is removed (commit already succeeded). Otherwise, the backup is rolled back into the vault directory.

Both `.sync-staging/` and `.sync-backup/` are listed in `.gitignore`.

## Wire Protocol

All messages are JSON-encoded over libp2p streams using the Noise security transport.

**Protocol IDs:**
- `/usm/pair/1.0` — pairing handshake
- `/usm/sync/1.0` — sync session

**Message format:**

```json
{"action": "<action>", "payload": { ... }}
```

**Sync session actions (in order):**

| Action | Direction | Description |
|--------|-----------|-------------|
| `sync_request` | Initiator -> Responder | Handshake with instance ID, USM version, chain version |
| `sync_accept` | Responder -> Initiator | Acknowledges the request |
| `catalogue` | Both | Exchange vault catalogues (includes `key_fingerprint` per vault) |
| `sync_plan` | Initiator -> Responder | Proposed transfer plan from negotiation |
| `sync_plan_resolved` | Responder -> Initiator | Resolved plan (conflicts decided) |
| `transfer_begin` | Sender | Start of vault file stream |
| `transfer_file` | Sender | Individual file payload |
| `transfer_end` | Sender | End of vault file stream |
| `sync_complete` | Both | Session complete |
| `sync_abort` | Either | Abort at any point |

## User Interface

### Preferences

The **LAN Sync** card in Preferences contains:

- **Sync Mode** selector (Disabled / Relaxed / Strict) — toggling to an enabled mode starts the sync service immediately; toggling to Disabled stops it.
- **Open Discovery** button — visible when the sync service is running. Navigates to the discovery screen.

### Discovery Screen

Accessible from:
- The **Open Discovery** button in Preferences.
- The **computer icon** in the toolbar (when toolbar is visible and sync is running).
- **File > Discovery** in the main menu (when sync is running).

The screen shows:
- A scanning indicator (animated spinner).
- **Trusted Peers** section — peers previously paired, each with a Sync action button.
- **Discovered Peers** section — unpaired peers, each with a Pair action button.
- Each peer card displays a colour dot, label (or truncated peer ID), and connection status.

### Peer Colours

Each peer gets a deterministic colour derived from `SHA-256(peerID)`, mapped to a 16-colour palette. This helps users visually distinguish peers across sessions and devices.

## Files

| Path | Purpose |
|------|---------|
| `<storage_root>/peer.key` | Ed25519 identity key (base64) |
| `<storage_root>/trusted_peers.json` | Paired peer registry |
| `<vault_dir>/.sync-staging/` | Temporary receive buffer (transient) |
| `<vault_dir>/.sync-backup/` | Pre-commit backup (transient) |

## Current Status

The sync infrastructure is fully implemented end-to-end: identity, trust, discovery, pairing (6-character code with HMAC verification), negotiation, vault transfer (with atomic staging and crash recovery), Relaxed/Strict modes, and UI integration (discovery screen, incoming sync lock, state reload after sync).
