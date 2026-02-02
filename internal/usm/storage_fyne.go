package usm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

// Declare conformity to Item interface
var _ Storage = (*FyneStorage)(nil)

type FyneStorage struct {
	fyne.Storage
}

// NewFyneStorage returns an Fyne Storage implementation
func NewFyneStorage(s fyne.Storage) (Storage, error) {
	fs := &FyneStorage{Storage: s}
	err := fs.mkdirIfNotExists(storageRootPath(fs))
	if err != nil {
		return nil, fmt.Errorf("could not create storage dir: %w", err)
	}
	return fs, nil
}

func (s *FyneStorage) Root() string {
	return s.Storage.RootURI().Path()
}

// CreateVault encrypts and stores an empty vault into the underlying storage.
func (s *FyneStorage) CreateVaultKey(name string, password string) (*Key, error) {
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
func (s *FyneStorage) CreateVault(name string, key *Key) (*Vault, error) {
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
func (s *FyneStorage) DeleteVault(name string) error {
	// ATTN: delete the entire vault directory (key, vault file, and all item files)
	root := storage.NewFileURI(vaultRootPath(s, name))
	// Best-effort delete contents first
	if entries, err := storage.List(root); err == nil {
		for _, e := range entries {
			_ = storage.Delete(e)
		}
	}
	if err := storage.Delete(root); err != nil {
		return fmt.Errorf("could not delete the vault: %w", err)
	}
	return nil
}

// LoadVaultIdentity returns a vault decrypting from the underlying storage
func (s *FyneStorage) LoadVaultKey(name string, password string) (*Key, error) {
	keyFile := keyPath(s, name)
	r, err := storage.Reader(storage.NewFileURI(keyFile))
	if err != nil {
		return nil, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	return LoadKey(password, r)
}

// LoadVault returns a vault decrypting from the underlying storage
func (s *FyneStorage) LoadVault(name string, key *Key) (*Vault, error) {
	vault := NewVault(key, name)
	vaultFile := vaultPath(s, name)

	r, err := storage.Reader(storage.NewFileURI(vaultFile))
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
func (s *FyneStorage) StoreVault(vault *Vault) error {
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
func (s *FyneStorage) DeleteItem(vault *Vault, item Item) error {
	itemFile := itemPath(s, vault.Name, item.ID())
	err := storage.Delete(storage.NewFileURI(itemFile))
	if err != nil {
		return fmt.Errorf("could not delete the item: %w", err)
	}
	return nil
}

// LoadItem returns a item from the vault decrypting from the underlying storage
func (s *FyneStorage) LoadItem(vault *Vault, itemMetadata *Metadata) (Item, error) {
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
	r, err := storage.Reader(storage.NewFileURI(itemFile))
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
func (s *FyneStorage) StoreItem(vault *Vault, item Item) error {
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
func (s *FyneStorage) Vaults() ([]string, error) {
	root := storage.NewFileURI(storageRootPath(s))
	vaults := []string{}

	dirEntries, err := storage.List(root)
	if err != nil {
		return nil, err
	}

	for _, dirEntry := range dirEntries {
		if ok, err := storage.CanList(dirEntry); !ok {
			if err != nil {
				fyne.LogError("could not list dir entry", err)
			}
			continue
		}
		vaults = append(vaults, dirEntry.Name())
	}

	return vaults, nil
}

// ChangeVaultPassword rekeys the vault contents using a new password-protected key
func (s *FyneStorage) ChangeVaultPassword(vault *Vault, oldPassword string, newPassword string) (*Vault, error) {
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

	if _, err := s.LoadVaultKey(vault.Name, oldPassword); err != nil {
		return nil, fmt.Errorf("invalid old password: %w", err)
	}

	items, err := loadVaultItems(s, vault)
	if err != nil {
		return nil, err
	}

	activeDir := vaultRootPath(s, vault.Name)
	backupVersion := catalogueVaultVersion(s, vault.Name)
	if backupVersion <= 0 {
		backupVersion = 1
	}
	backupDir := fmt.Sprintf("%s.v%d", activeDir, backupVersion)

	// ATTN: Ensure backup is created before mutating the active vault directory to prevent data loss
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

	if backupVersion > 1 {
		oldBackupDir := fmt.Sprintf("%s.v%d", activeDir, backupVersion-1)
		_ = os.RemoveAll(oldBackupDir)
	}

	return newVault, nil
}

// LoadAppState load the configuration from the underlying storage
func (s *FyneStorage) LoadAppState() (*AppState, error) {
	defaultAppState := &AppState{
		Modified:    time.Now().UTC(),
		Preferences: newDefaultPreferences(),
	}
	appStateFile := appStateFilePath(s)
	uri := storage.NewFileURI(appStateFile)
	if ok, _ := storage.Exists(uri); !ok {
		return defaultAppState, nil
	}
	r, err := storage.Reader(uri)
	if err != nil {
		return defaultAppState, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	appState := &AppState{}
	err = json.NewDecoder(r).Decode(appState)
	if err != nil {
		return defaultAppState, err
	}
	return appState, nil
}

// StoreAppState store the configuration into the underlying storage
func (s *FyneStorage) StoreAppState(appState *AppState) error {
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
func (s *FyneStorage) SocketAgentPath() string {
	return socketAgentPath(s)
}

// LockFilePath return the lock file path
func (s *FyneStorage) LockFilePath() string {
	return lockFilePath(s)
}

// LogFilePath return the log file path
func (s *FyneStorage) LogFilePath() string {
	return logFilePath(s)
}

func (s *FyneStorage) isExist(path string) bool {
	ok, _ := storage.Exists(storage.NewFileURI(path))
	return ok
}

func (s *FyneStorage) mkdirIfNotExists(path string) error {
	if s.isExist(path) {
		return nil
	}
	return storage.CreateListable(storage.NewFileURI(path))
}

func (s *FyneStorage) createFile(name string) (fyne.URIWriteCloser, error) {
	return storage.Writer(storage.NewFileURI(name))
}

// MigrateVaultCatalogue creates catalogue entries for vaults missing metadata
func (s *FyneStorage) MigrateVaultCatalogue() error {
	return migrateVaultCatalogueEntries(s)
}
