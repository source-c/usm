package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStagingCommit_Success(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "storage", "testvault")

	require.NoError(t, os.MkdirAll(vaultDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(vaultDir, "vault.age"), []byte("old"), 0o600))

	stagingDir := StagingDir(vaultDir)
	require.NoError(t, os.MkdirAll(stagingDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(stagingDir, "vault.age"), []byte("new"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(stagingDir, "item1.age"), []byte("item"), 0o600))

	err := CommitStaging(vaultDir)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(vaultDir, "vault.age"))
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))

	data, err = os.ReadFile(filepath.Join(vaultDir, "item1.age"))
	require.NoError(t, err)
	assert.Equal(t, "item", string(data))

	assert.NoDirExists(t, StagingDir(vaultDir))
	assert.NoDirExists(t, BackupDir(vaultDir))
}

func TestStagingCommit_RollbackOnMissing(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "storage", "testvault")

	err := CommitStaging(vaultDir)
	assert.Error(t, err)
}

func TestCleanupOrphanedStaging(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "storage", "testvault")

	stagingDir := StagingDir(vaultDir)
	require.NoError(t, os.MkdirAll(stagingDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(stagingDir, "junk.age"), []byte("x"), 0o600))

	CleanupOrphanedSync(vaultDir)
	assert.NoDirExists(t, stagingDir)
}

func TestCleanupOrphanedBackup_RestoresIfLiveMissing(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "storage", "testvault")

	require.NoError(t, os.MkdirAll(vaultDir, 0o700))

	backupDir := BackupDir(vaultDir)
	require.NoError(t, os.MkdirAll(backupDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "vault.age"), []byte("backup"), 0o600))

	CleanupOrphanedSync(vaultDir)

	data, err := os.ReadFile(filepath.Join(vaultDir, "vault.age"))
	require.NoError(t, err)
	assert.Equal(t, "backup", string(data))
	assert.NoDirExists(t, backupDir)
}

func TestPrepareStaging(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "storage", "testvault")
	require.NoError(t, os.MkdirAll(vaultDir, 0o700))

	staging, err := PrepareStaging(vaultDir)
	require.NoError(t, err)
	assert.DirExists(t, staging)
	assert.Equal(t, StagingDir(vaultDir), staging)
}

func TestValidateSyncName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid vault name", "my-vault", false},
		{"valid file name", "vault.age", false},
		{"empty name", "", true},
		{"path traversal", "../etc/passwd", true},
		{"path traversal mid", "foo/../bar", true},
		{"absolute path", "/etc/passwd", true},
		{"forward slash", "foo/bar", true},
		{"backslash", `foo\bar`, true},
		{"hidden file", ".ssh", true},
		{"hidden staging", ".sync-staging", true},
		{"double dot only", "..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSyncName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
