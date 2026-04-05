package usm

import "time"

// AppState represents the application state
type AppState struct {
	// InstanceID uniquely identifies this USM installation for backup and sync purposes
	InstanceID string `json:"instance_id,omitempty"`
	// Modified is the last time the state was modified, example: preferences changed or vaults modified
	Modified time.Time `json:"modified,omitempty"`
	// Preferences contains the application preferences
	Preferences *Preferences `json:"preferences,omitempty"`
	// VaultCatalogue stores metadata entries for each managed vault
	VaultCatalogue map[string]*VaultEntry `json:"vault_catalogue,omitempty"`
}

// VaultEntry represents the catalogue metadata tracked for a vault
type VaultEntry struct {
	// Name is the vault identifier and directory name
	Name string `json:"name"`
	// Version is a monotonic counter incremented on every vault modification (item add/edit/delete,
	// password rotation, colour change). Used by sync negotiation to determine which side is newer.
	Version int `json:"version"`
	// StorageLocation is the absolute path holding the vault payload
	StorageLocation string `json:"storage_location"`
	// KeyFingerprint is the SHA256 hash of the vault's public key
	KeyFingerprint string `json:"key_fingerprint"`
	// Created holds the initial creation timestamp for the vault
	Created time.Time `json:"created"`
	// Modified stores the last modification timestamp for the vault
	Modified time.Time `json:"modified"`
	// LastUnlocked stores the last successful unlock timestamp, empty if never unlocked
	LastUnlocked time.Time `json:"last_unlocked,omitempty"`
	// ItemCount caches how many items were present the last time the catalogue was refreshed
	ItemCount int `json:"item_count"`
	// ChainCS is the SHA-256 checksum of the Viracochan catalogue chain at the time this entry was
	// last modified. Empty for entries created before Viracochan integration. When present on both
	// sides of a sync negotiation, matching checksums prove identical catalogue state.
	ChainCS string `json:"chain_cs,omitempty"`
}
