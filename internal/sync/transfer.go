package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	stagingDirName = ".sync-staging"
	backupDirName  = ".sync-backup"
)

// maxTransferFileSize is the maximum allowed size of a single transferred file (50 MB).
// ATTN: this guards against memory exhaustion from malicious or corrupted payloads.
const maxTransferFileSize = 50 * 1024 * 1024

// validateSyncName checks that a name (vault name or file name) received from a
// remote peer is safe to use in a file path. Rejects empty names, path traversal
// attempts, absolute paths, and names containing path separators.
func validateSyncName(name string) error {
	if name == "" {
		return fmt.Errorf("empty name")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("path traversal in name: %q", name)
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("absolute path in name: %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("path separator in name: %q", name)
	}
	// Reject hidden files/dirs that could collide with staging/backup
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("hidden name not allowed: %q", name)
	}
	return nil
}

// StagingDir returns the staging directory path for a vault
func StagingDir(vaultDir string) string {
	return filepath.Join(vaultDir, stagingDirName)
}

// BackupDir returns the backup directory path for a vault
func BackupDir(vaultDir string) string {
	return filepath.Join(vaultDir, backupDirName)
}

// PrepareStaging creates a clean staging directory for receiving vault files
func PrepareStaging(vaultDir string) (string, error) {
	staging := StagingDir(vaultDir)
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return "", fmt.Errorf("could not create staging dir: %w", err)
	}
	return staging, nil
}

// CommitStaging atomically replaces the live vault directory with the staging
// directory contents. The existing vault is backed up first and removed on success.
func CommitStaging(vaultDir string) error {
	staging := StagingDir(vaultDir)
	backup := BackupDir(vaultDir)

	if _, err := os.Stat(staging); os.IsNotExist(err) {
		return fmt.Errorf("staging directory does not exist: %s", staging)
	}

	_ = os.RemoveAll(backup)

	entries, err := os.ReadDir(vaultDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not read vault dir: %w", err)
	}

	if err := os.MkdirAll(backup, 0o700); err != nil {
		return fmt.Errorf("could not create backup dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == stagingDirName || name == backupDirName {
			continue
		}
		src := filepath.Join(vaultDir, name)
		dst := filepath.Join(backup, name)
		if err := os.Rename(src, dst); err != nil {
			rollbackFromBackup(vaultDir, backup)
			return fmt.Errorf("could not backup %s: %w", name, err)
		}
	}

	stagingEntries, err := os.ReadDir(staging)
	if err != nil {
		rollbackFromBackup(vaultDir, backup)
		return fmt.Errorf("could not read staging dir: %w", err)
	}

	for _, entry := range stagingEntries {
		src := filepath.Join(staging, entry.Name())
		dst := filepath.Join(vaultDir, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			rollbackFromBackup(vaultDir, backup)
			return fmt.Errorf("could not commit %s: %w", entry.Name(), err)
		}
	}

	_ = os.RemoveAll(staging)
	_ = os.RemoveAll(backup)

	return nil
}

// CleanupOrphanedSync detects and cleans up incomplete sync state.
// Called on application startup to recover from crashes during sync.
func CleanupOrphanedSync(vaultDir string) {
	staging := StagingDir(vaultDir)
	backup := BackupDir(vaultDir)

	if _, err := os.Stat(staging); err == nil {
		_ = os.RemoveAll(staging)
	}

	if _, err := os.Stat(backup); err == nil {
		if _, err := os.Stat(filepath.Join(vaultDir, "vault.age")); err == nil {
			_ = os.RemoveAll(backup)
		} else {
			rollbackFromBackup(vaultDir, backup)
		}
	}
}

func rollbackFromBackup(vaultDir, backup string) {
	entries, err := os.ReadDir(backup)
	if err != nil {
		return
	}
	for _, entry := range entries {
		src := filepath.Join(backup, entry.Name())
		dst := filepath.Join(vaultDir, entry.Name())
		_ = os.Rename(src, dst)
	}
	_ = os.RemoveAll(backup)
}
