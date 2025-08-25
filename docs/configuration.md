# Configuration Management

This document describes the versioned configuration management system for USM using [Viracochan](https://github.com/source-c/viracochan).

## Features

- **Versioned Configurations**: Every preference change creates a new immutable version
- **Cryptographic Integrity**: SHA-256 checksums ensure data integrity
- **Chain Validation**: Each version links to its predecessor via checksums
- **Journaling**: All changes are recorded in an append-only journal
- **State Reconstruction**: Rebuild state from journal entries
- **Migration Support**: Seamless migration from legacy `usm.json` format

## Architecture

The integration follows a simple adapter pattern:

```
USM Application
      ↓
StorageAdapter (implements AppStateStorage)
      ↓
Manager (Viracochan wrapper)
      ↓
Viracochan Library
      ↓
File Storage
```

## Key Components

The configuration system is implemented in `internal/config/` with the following components:

### Manager (`internal/config/manager.go`)
- Wraps Viracochan for USM-specific operations
- Handles conversion between USM's AppState and Viracochan's content format
- Provides versioning, history, and rollback capabilities

### StorageAdapter (`internal/config/storage_adapter.go`)
- Implements the existing `AppStateStorage` interface
- Drop-in replacement for existing storage implementations
- Adds versioning capabilities transparently

### Migration (`internal/config/migration.go`)
- Migrates existing `usm.json` files to versioned format
- Creates backups of legacy configuration
- Supports export back to legacy format for compatibility

### Watch (`internal/config/watch.go`)
- Real-time configuration change monitoring
- Disaster recovery through state reconstruction
- Import/export for backup and restore

### Preferences (`internal/config/preferences.go`)
- Handles conversion between USM preferences and Viracochan content
- Maintains all existing preference structures
- Ensures backward compatibility

## Usage

### Basic Integration

```go
// Create versioned storage
storage, err := config.CreateVersionedStorage("/path/to/storage")
if err != nil {
    log.Fatal(err)
}

// Use as normal AppStateStorage
appState, err := storage.LoadAppState()
err = storage.StoreAppState(appState)
```

### Advanced Features

```go
// Get configuration history
history, err := storage.GetHistory()

// Rollback to specific version
state, err := storage.Rollback(3)

// Validate chain integrity
err = storage.ValidateChain()

// Watch for changes
ctx := context.Background()
changes, err := storage.(*StorageAdapter).manager.WatchAppState(ctx, 1*time.Second)
for state := range changes {
    log.Printf("Config updated: %+v", state)
}
```

## Migration

The system automatically migrates existing `usm.json` files:

1. On first run, detects existing `usm.json`
2. Imports configuration as version 1
3. Creates `usm.json.backup` for safety
4. All future changes create new versions

## Testing

Run the comprehensive test suite:

```bash
go test ./internal/config/...
```
