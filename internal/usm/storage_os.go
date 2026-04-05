package usm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Declare conformity to Item interface
var _ Storage = (*OSStorage)(nil)

type OSStorage struct {
	root string
}

// NewOSStorage returns an OS Storage implementation rooted at os.UserConfigDir()
func NewOSStorage() (Storage, error) {
	var urd string
	var err error
	urd = os.Getenv(ENV_HOME)
	if urd == "" {
		urd, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not get the default root directory to use for user-specific configuration data: %w", err)
		}
	}
	return NewOSStorageRooted(urd)
}

// NewOSStorageRooted returns an OS Storage implementation rooted at root
func NewOSStorageRooted(root string) (Storage, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("storage root must be an absolute path, got %s", root)
	}

	// Fyne does not allow to customize the root for a storage
	// so we'll use the same
	storageRoot := filepath.Join(root, ".usm")

	s := &OSStorage{root: storageRoot}

	migrated, err := s.migrateDeprecatedRootStorage()
	if migrated {
		if err != nil {
			return nil, fmt.Errorf("found deprecated storage but was unable to move to new location: %w", err)
		}
		return s, nil
	}

	err = s.mkdirIfNotExists(storageRootPath(s))
	return s, err
}

func (s *OSStorage) Root() string {
	return s.root
}

// CreateVault encrypts and stores an empty vault into the underlying storage.
func (s *OSStorage) CreateVaultKey(name string, password string) (*Key, error) {
	err := s.mkdirIfNotExists(vaultRootPath(s, name))
	if err != nil {
		return nil, fmt.Errorf("could not create vault root dir: %w", err)
	}

	keyFile := keyPath(s, name)
	if s.isExist(keyFile) {
		return nil, errors.New("key with the same name already exists")
	}

	w, err := s.createFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("could not create writer for the key file: %w", err)
	}
	defer w.Close()

	key, err := MakeKey(password, w)
	if err != nil {
		return nil, fmt.Errorf("could not create the vault key file: %w", err)
	}

	return key, nil
}

// CreateVault encrypts and stores an empty vault into the underlying storage.
func (s *OSStorage) CreateVault(name string, key *Key) (*Vault, error) {
	err := s.mkdirIfNotExists(vaultRootPath(s, name))
	if err != nil {
		return nil, fmt.Errorf("could not create vault root dir: %w", err)
	}

	vault := NewVault(key, name)
	err = s.StoreVault(vault)
	if err != nil {
		return nil, err
	}
	return vault, nil
}

// DeleteVault delete the specified vault
func (s *OSStorage) DeleteVault(name string) error {
	// ATTN: delete the entire vault directory (key, vault file, and all item files)
	if err := os.RemoveAll(vaultRootPath(s, name)); err != nil {
		return fmt.Errorf("could not delete the vault: %w", err)
	}
	return nil
}

// LoadVaultIdentity returns a vault decrypting from the underlying storage
func (s *OSStorage) LoadVaultKey(name string, password string) (*Key, error) {
	keyFile := keyPath(s, name)
	r, err := os.Open(keyFile) //nolint:gosec // keyFile is application-controlled path
	if err != nil {
		return nil, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	return LoadKey(password, r)
}

// LoadVault returns a vault decrypting from the underlying storage
func (s *OSStorage) LoadVault(name string, key *Key) (*Vault, error) {
	vault := NewVault(key, name)
	vaultFile := vaultPath(s, name)

	r, err := os.Open(vaultFile) //nolint:gosec // vaultFile is application-controlled path
	if err != nil {
		return nil, fmt.Errorf("could not create reader: %w", err)
	}
	defer r.Close()

	err = decrypt(key, r, vault)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the vault: %w", err)
	}

	if err := persistVaultCatalogue(s, vault, func(entry *VaultEntry) {
		entry.LastUnlocked = time.Now().UTC()
	}); err != nil {
		return nil, err
	}

	return vault, nil
}

// StoreVault encrypts and stores the vault into the underlying storage
func (s *OSStorage) StoreVault(vault *Vault) error {
	vaultFile := vaultPath(s, vault.Name)
	w, err := s.createFile(vaultFile)
	if err != nil {
		return fmt.Errorf("could not create writer: %w", err)
	}
	defer w.Close()

	err = encrypt(vault.key, w, vault)
	if err != nil {
		return fmt.Errorf("could not encrypt and store the vault: %w", err)
	}

	if err := persistVaultCatalogue(s, vault, nil); err != nil {
		return err
	}

	return nil
}

// DeleteItem delete the item from the specified vaultName
func (s *OSStorage) DeleteItem(vault *Vault, item Item) error {
	itemFile := itemPath(s, vault.Name, item.ID())
	err := os.Remove(itemFile)
	if err != nil {
		return fmt.Errorf("could not delete the item: %w", err)
	}
	return nil
}

// LoadItem returns a item from the vault decrypting from the underlying storage
func (s *OSStorage) LoadItem(vault *Vault, itemMetadata *Metadata) (Item, error) {
	var item Item
	switch itemMetadata.Type {
	case NoteItemType:
		item = &Note{}
	case PasswordItemType:
		item = &Password{}
	case LoginItemType:
		item = &Login{}
	case SSHKeyItemType:
		item = &SSHKey{}
	}

	itemFile := itemPath(s, vault.Name, itemMetadata.ID())
	r, err := os.Open(itemFile) //nolint:gosec // itemFile is application-controlled path
	if err != nil {
		return nil, fmt.Errorf("could not create reader: %w", err)
	}
	defer r.Close()

	err = decrypt(vault.key, r, item)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the item: %w", err)
	}
	return item, nil
}

// StoreItem encrypts and encrypts and stores the item into the specified vault
func (s *OSStorage) StoreItem(vault *Vault, item Item) error {
	itemFile := itemPath(s, vault.Name, item.ID())
	w, err := s.createFile(itemFile)
	if err != nil {
		return fmt.Errorf("could not create writer: %w", err)
	}
	defer w.Close()

	err = encrypt(vault.key, w, item)
	if err != nil {
		return fmt.Errorf("could not encrypt and store the item: %w", err)
	}
	return nil
}

// Vaults returns the list of vault names from the storage
func (s *OSStorage) Vaults() ([]string, error) {
	root := storageRootPath(s)
	dirEntries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	vaults := []string{}
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		vaults = append(vaults, dirEntry.Name())
	}

	return vaults, nil
}

// ChangeVaultPassword re-encrypts all vault contents using a newly generated key protected by newPassword
func (s *OSStorage) ChangeVaultPassword(vault *Vault, oldPassword string, newPassword string) (*Vault, error) {
	if vault == nil {
		return nil, errors.New("vault cannot be nil")
	}
	if vault.Name == "" {
		return nil, errors.New("vault name cannot be empty")
	}
	if newPassword == "" {
		return nil, errors.New("new password cannot be empty")
	}
	if oldPassword == newPassword {
		return nil, errors.New("new password must differ from the old password")
	}

	// Ensure supplied old password is valid for the stored key
	if _, err := s.LoadVaultKey(vault.Name, oldPassword); err != nil {
		return nil, fmt.Errorf("invalid old password: %w", err)
	}

	items, err := loadVaultItems(s, vault)
	if err != nil {
		return nil, err
	}

	// ATTN: Moving the active vault directory to a backup must be atomic to avoid data loss
	activeDir := vaultRootPath(s, vault.Name)
	backupVersion := catalogueVaultVersion(s, vault.Name)
	if backupVersion <= 0 {
		backupVersion = 1
	}
	backupDir := fmt.Sprintf("%s.v%d", activeDir, backupVersion)

	if err := os.Rename(activeDir, backupDir); err != nil {
		return nil, fmt.Errorf("could not move vault to backup: %w", err)
	}

	restoreBackup := func() error {
		_ = os.RemoveAll(activeDir)
		return os.Rename(backupDir, activeDir)
	}

	newKey, err := s.CreateVaultKey(vault.Name, newPassword)
	if err != nil {
		_ = restoreBackup()
		return nil, fmt.Errorf("could not create vault key with new password: %w", err)
	}

	newVault := NewVault(newKey, vault.Name)
	newVault.Colour = vault.Colour
	newVault.Version = vault.Version
	newVault.Created = vault.Created
	newVault.Modified = time.Now().UTC()

	for _, item := range items {
		if err := newVault.AddItem(item); err != nil {
			_ = restoreBackup()
			return nil, fmt.Errorf("could not add item to new vault: %w", err)
		}
		if err := s.StoreItem(newVault, item); err != nil {
			_ = restoreBackup()
			return nil, fmt.Errorf("could not store item into new vault: %w", err)
		}
	}

	if err := s.StoreVault(newVault); err != nil {
		_ = restoreBackup()
		return nil, fmt.Errorf("could not store new vault: %w", err)
	}

	// Remove older backups if we only preserve the previous generation
	if backupVersion > 1 {
		oldBackupDir := fmt.Sprintf("%s.v%d", activeDir, backupVersion-1)
		_ = os.RemoveAll(oldBackupDir)
	}

	return newVault, nil
}

// LoadAppState load the configuration from the underlying storage
func (s *OSStorage) LoadAppState() (*AppState, error) {
	defaultAppState := &AppState{
		Modified:    time.Now().UTC(),
		Preferences: newDefaultPreferences(),
	}
	appStateFile := appStateFilePath(s)
	r, err := os.Open(appStateFile) //nolint:gosec // appStateFile is application-controlled path
	if os.IsNotExist(err) {
		if err := ensureInstanceID(defaultAppState); err != nil {
			return defaultAppState, fmt.Errorf("could not ensure instance ID: %w", err)
		}
		return defaultAppState, nil
	}
	if err != nil {
		return defaultAppState, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	appState := &AppState{}
	err = json.NewDecoder(r).Decode(appState)
	if err != nil {
		return defaultAppState, err
	}
	if err := ensureInstanceID(appState); err != nil {
		return appState, fmt.Errorf("could not ensure instance ID: %w", err)
	}
	return appState, nil
}

// StoreAppState store the configuration into the underlying storage
func (s *OSStorage) StoreAppState(appState *AppState) error {
	if err := mergeExistingCatalogue(s, appState); err != nil {
		return err
	}

	appStateFile := appStateFilePath(s)
	w, err := s.createFile(appStateFile)
	if err != nil {
		return err
	}
	defer w.Close()
	return json.NewEncoder(w).Encode(appState)
}

// SocketAgentPath return the socket agent path
func (s *OSStorage) SocketAgentPath() string {
	return socketAgentPath(s)
}

// LockFilePath return the lock file path
func (s *OSStorage) LockFilePath() string {
	return lockFilePath(s)
}

// LogFilePath return the log file path
func (s *OSStorage) LogFilePath() string {
	return logFilePath(s)
}

// PeerKeyPath return the peer key file path
func (s *OSStorage) PeerKeyPath() string {
	return peerKeyPath(s)
}

// TrustedPeersPath return the trusted peers file path
func (s *OSStorage) TrustedPeersPath() string {
	return trustedPeersPath(s)
}

func (s *OSStorage) isExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (s *OSStorage) mkdirIfNotExists(path string) error {
	if s.isExist(path) {
		return nil
	}
	return os.MkdirAll(path, 0o700)
}

func (s *OSStorage) createFile(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // name is application-controlled path
}

// migrateDeprecatedRootStorage migrates the deprecated 'vaults' storage folder to new one
func (s *OSStorage) migrateDeprecatedRootStorage() (bool, error) {
	defaultRoot, err := defaultOSStorageRoot()
	if err != nil {
		return false, nil
	}
	if filepath.Clean(s.Root()) != filepath.Clean(defaultRoot) {
		return false, nil
	}

	oldRoot, err := os.UserConfigDir()
	if err != nil {
		return false, nil
	}

	src := filepath.Join(oldRoot, "fyne", ID, "vaults")
	_, err = os.Stat(src)
	if os.IsNotExist(err) {
		return false, nil
	}
	dest := storageRootPath(s)
	err = os.Rename(src, dest)
	return true, err
}

func defaultOSStorageRoot() (string, error) {
	root := os.Getenv(ENV_HOME)
	if root == "" {
		var err error
		root, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(root, ".usm"), nil
}

// MigrateVaultCatalogue creates catalogue entries for vaults missing metadata
func (s *OSStorage) MigrateVaultCatalogue() error {
	return migrateVaultCatalogueEntries(s)
}
