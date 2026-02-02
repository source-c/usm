package usm

import "time"

// UpdateVaultCatalogue adds or refreshes the catalogue metadata for the provided vault
func UpdateVaultCatalogue(catalogue map[string]*VaultEntry, vault *Vault, storage Storage) {
	if catalogue == nil || vault == nil || storage == nil {
		return
	}

	entry, ok := catalogue[vault.Name]
	if !ok {
		entry = &VaultEntry{
			Name:            vault.Name,
			Version:         1,
			StorageLocation: vaultRootPath(storage, vault.Name),
			Created:         vault.Created,
		}
		if entry.Created.IsZero() {
			entry.Created = time.Now().UTC()
		}
		catalogue[vault.Name] = entry
	}

	entry.KeyFingerprint = vault.Key().Fingerprint()
	entry.Modified = vault.Modified
	if entry.Modified.IsZero() {
		entry.Modified = time.Now().UTC()
	}
	entry.ItemCount = vault.Size()
}

// IncrementVaultVersion increases the stored version counter for a vault in the catalogue
func IncrementVaultVersion(catalogue map[string]*VaultEntry, vaultName string) {
	if catalogue == nil {
		return
	}

	entry, ok := catalogue[vaultName]
	if !ok {
		return
	}

	entry.Version++
	entry.Modified = time.Now().UTC()
}
