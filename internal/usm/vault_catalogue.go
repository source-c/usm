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
	} else if !vault.Modified.IsZero() && !entry.Modified.Equal(vault.Modified) {
		entry.Version++
	}

	entry.KeyFingerprint = vault.Key().Fingerprint()
	entry.Modified = vault.Modified
	if entry.Modified.IsZero() {
		entry.Modified = time.Now().UTC()
	}
	entry.ItemCount = vault.Size()
}
