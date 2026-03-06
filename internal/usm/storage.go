package usm

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"time"
)

const (
	storageRootName      = "storage"
	keyFileName          = "key.age"
	vaultFileName        = "vault.age"
	appStateFileName     = "usm.json"
	lockFileName         = "usm.lock"
	logFileName          = "usm.log"
	socketFileName       = "agent.sock"
	namedPipe            = `\\.\pipe\usm`
	peerKeyFileName      = "peer.key"
	trustedPeersFileName = "trusted_peers.json"
)

type Storage interface {
	Root() string
	AppStateStorage
	VaultStorage
	ItemStorage
	LogStorage
	SocketAgentPath() string
	LockFilePath() string
	PeerKeyPath() string
	TrustedPeersPath() string
	MigrateVaultCatalogue() error
}
type AppStateStorage interface {
	LoadAppState() (*AppState, error)
	StoreAppState(s *AppState) error
}

type VaultStorage interface {
	// CreateVault encrypts and stores an empty vault into the underlying storage.
	CreateVault(name string, key *Key) (*Vault, error)
	// LoadVaultKey creates and stores a Key used to encrypt and decrypt the vault data
	// The file containing the key is encrypted using the provided password
	CreateVaultKey(name string, password string) (*Key, error)
	// DeleteVault delete the specified vault
	DeleteVault(name string) error
	// LoadVault returns a vault decrypting from the underlying storage
	LoadVault(name string, key *Key) (*Vault, error)
	// LoadVaultKey returns the Key used to encrypt and decrypt the vault data
	LoadVaultKey(name string, password string) (*Key, error)
	// StoreVault encrypts and stores the vault into the underlying storage
	StoreVault(vault *Vault) error
	// Vaults returns the list of vault names from the storage
	Vaults() ([]string, error)
	// ChangeVaultPassword re-encrypts the vault data using a newly generated key derived from the provided password
	ChangeVaultPassword(vault *Vault, oldPassword string, newPassword string) (*Vault, error)
}

type ItemStorage interface {
	// DeleteItem delete the item from the specified vaultName
	DeleteItem(vault *Vault, item Item) error
	// LoadItem returns a item from the vault decrypting from the underlying storage
	LoadItem(vault *Vault, itemMetadata *Metadata) (Item, error)
	// StoreItem encrypts and encrypts and stores the item into the specified vault
	StoreItem(vault *Vault, item Item) error
}

type LogStorage interface {
	LogFilePath() string
}

func storageRootPath(s Storage) string {
	return filepath.Join(s.Root(), storageRootName)
}

func appStateFilePath(s Storage) string {
	return filepath.Join(s.Root(), appStateFileName)
}

func vaultRootPath(s Storage, vaultName string) string {
	return filepath.Join(storageRootPath(s), vaultName)
}

func keyPath(s Storage, vaultName string) string {
	return filepath.Join(vaultRootPath(s, vaultName), keyFileName)
}

func vaultPath(s Storage, vaultName string) string {
	return filepath.Join(vaultRootPath(s, vaultName), vaultFileName)
}

func itemPath(s Storage, vaultName string, itemID string) string {
	itemFileName := fmt.Sprintf("%s.age", itemID)
	return filepath.Join(vaultRootPath(s, vaultName), itemFileName)
}

func socketAgentPath(s Storage) string {
	if runtime.GOOS == "windows" {
		return namedPipe
	}
	return filepath.Join(s.Root(), socketFileName)
}

func lockFilePath(s Storage) string {
	return filepath.Join(s.Root(), lockFileName)
}

func logFilePath(s Storage) string {
	return filepath.Join(s.Root(), logFileName)
}

func peerKeyPath(s Storage) string {
	return filepath.Join(s.Root(), peerKeyFileName)
}

func trustedPeersPath(s Storage) string {
	return filepath.Join(s.Root(), trustedPeersFileName)
}

// generateUUID returns a new random UUID v4 string
func generateUUID() (string, error) {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		return "", fmt.Errorf("could not generate instance ID: %w", err)
	}
	// Set version (4) and variant (RFC4122) bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}

// ensureInstanceID assigns a new instance ID if one is not yet set.
// The ID is set in memory and persisted on the next StoreAppState call.
func ensureInstanceID(appState *AppState) error {
	if appState.InstanceID != "" {
		return nil
	}
	id, err := generateUUID()
	if err != nil {
		return err
	}
	appState.InstanceID = id
	return nil
}

func encrypt(key *Key, w io.Writer, v interface{}) error {
	encWriter, err := key.Encrypt(w)
	if err != nil {
		return fmt.Errorf("could not create encrypted writer: %w", err)
	}

	err = json.NewEncoder(encWriter).Encode(v)
	if err != nil {
		encWriter.Close()
		return fmt.Errorf("could not encode data: %w", err)
	}

	// ATTN: Close finalizes the age ciphertext; its error must be checked
	if err := encWriter.Close(); err != nil {
		return fmt.Errorf("could not finalize encryption: %w", err)
	}

	return nil
}

func decrypt(key *Key, r io.Reader, v interface{}) error {
	encReader, err := key.Decrypt(r)
	if err != nil {
		return fmt.Errorf("could not decrypt content: %w", err)
	}

	err = json.NewDecoder(encReader).Decode(v)
	if err != nil {
		return fmt.Errorf("could not decode content: %w", err)
	}
	return nil
}

func loadVaultItems(storage ItemStorage, vault *Vault) ([]Item, error) {
	items := make([]Item, 0, vault.Size())
	var loadErr error

	vault.Range(func(id string, meta *Metadata) bool {
		item, err := storage.LoadItem(vault, meta)
		if err != nil {
			loadErr = fmt.Errorf("could not load item %s: %w", id, err)
			return false
		}
		items = append(items, item)
		return true
	})

	return items, loadErr
}

func catalogueVaultVersion(storage Storage, vaultName string) int {
	appState, err := storage.LoadAppState()
	if err != nil || appState == nil {
		return 1
	}

	if appState.VaultCatalogue == nil {
		return 1
	}

	if entry, ok := appState.VaultCatalogue[vaultName]; ok && entry != nil && entry.Version > 0 {
		return entry.Version
	}

	return 1
}

func persistVaultCatalogue(storage Storage, vault *Vault, mutate func(entry *VaultEntry)) error {
	if vault == nil || vault.Name == "" {
		return fmt.Errorf("vault is required to update catalogue")
	}

	appState, err := storage.LoadAppState()
	if err != nil {
		return fmt.Errorf("could not load app state: %w", err)
	}

	if appState.VaultCatalogue == nil {
		appState.VaultCatalogue = make(map[string]*VaultEntry)
	}

	UpdateVaultCatalogue(appState.VaultCatalogue, vault, storage)

	if mutate != nil {
		if entry := appState.VaultCatalogue[vault.Name]; entry != nil {
			mutate(entry)
		}
	}

	appState.Modified = time.Now().UTC()
	if err := storage.StoreAppState(appState); err != nil {
		return fmt.Errorf("could not store app state: %w", err)
	}

	return nil
}

func migrateVaultCatalogueEntries(storage Storage) error {
	appState, err := storage.LoadAppState()
	if err != nil {
		return fmt.Errorf("could not load app state: %w", err)
	}

	if appState.VaultCatalogue == nil {
		appState.VaultCatalogue = make(map[string]*VaultEntry)
	}

	vaults, err := storage.Vaults()
	if err != nil {
		return fmt.Errorf("could not list vaults: %w", err)
	}

	now := time.Now().UTC()
	updated := false

	for _, name := range vaults {
		if _, exists := appState.VaultCatalogue[name]; exists {
			continue
		}

		appState.VaultCatalogue[name] = &VaultEntry{
			Name:            name,
			Version:         1,
			StorageLocation: vaultRootPath(storage, name),
			Created:         now,
			Modified:        now,
			ItemCount:       0,
		}
		updated = true
	}

	if !updated {
		return nil
	}

	appState.Modified = now
	if err := storage.StoreAppState(appState); err != nil {
		return fmt.Errorf("could not store app state: %w", err)
	}

	return nil
}

func mergeExistingCatalogue(storage Storage, appState *AppState) error {
	if appState == nil {
		return fmt.Errorf("app state is required")
	}

	existing, err := storage.LoadAppState()
	if err != nil {
		return fmt.Errorf("could not load current app state: %w", err)
	}

	appState.VaultCatalogue = mergeCatalogueEntries(existing.VaultCatalogue, appState.VaultCatalogue)
	return nil
}

func mergeCatalogueEntries(current, incoming map[string]*VaultEntry) map[string]*VaultEntry {
	if current == nil {
		return incoming
	}

	if incoming == nil {
		incoming = make(map[string]*VaultEntry)
	}

	for name, entry := range current {
		if entry == nil {
			continue
		}

		if existing, ok := incoming[name]; !ok || catalogueEntryRecency(entry).After(catalogueEntryRecency(existing)) {
			incoming[name] = entry
		}
	}

	return incoming
}

func catalogueEntryRecency(entry *VaultEntry) time.Time {
	if entry == nil {
		return time.Time{}
	}

	ts := entry.Modified
	if entry.Created.After(ts) {
		ts = entry.Created
	}
	if entry.LastUnlocked.After(ts) {
		ts = entry.LastUnlocked
	}

	return ts
}
