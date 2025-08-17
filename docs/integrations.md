# Desktop Integration Generation

This project automatically generates desktop integration files during the build process to make Linux and macOS experience more frictionless.

## Overview

The integration system consists of:

1. **Templates** (`templates/`): Go template files for desktop integrations
2. **Generator** (`scripts/gen-integrations.go`): Script that processes templates and generates integration files
3. **Makefile targets**: Build automation for different platforms

## Templates

### Linux Desktop Entry (`templates/linux.desktop.tmpl`)

Generates a `.desktop` file for Linux desktop environments following the [Desktop Entry Specification](https://specifications.freedesktop.org/desktop-entry-spec/desktop-entry-spec-latest.html).

**Template variables:**
- `{{.AppName}}`: Application display name
- `{{.BinaryName}}`: Executable name
- `{{.Description}}`: Application description
- `{{.InstallPath}}`: Installation directory path

### macOS App Bundle (`templates/macos.Info.plist.tmpl`)

Generates an `Info.plist` file for macOS app bundles following Apple's [Information Property List Key Reference](https://developer.apple.com/library/archive/documentation/General/Reference/InfoPlistKeyReference/Introduction/Introduction.html).

**Template variables:**
- `{{.AppName}}`: Application display name
- `{{.BinaryName}}`: Executable name
- `{{.Version}}`: Application version
- `{{.BundleID}}`: Unique bundle identifier
- `{{.Signature}}`: 4-character application signature
- `{{.IconName}}`: Icon file name
- `{{.MinOSVersion}}`: Minimum supported macOS version

## Build Targets

### `make generate-integrations`

Generates integration files from templates into `build/integrations/`:
- Creates Linux desktop file
- Creates macOS Info.plist
- Copies icon files to appropriate locations

### `make build-with-integrations`

Builds the application binary and generates integration files:
- Compiles the Go application
- Generates desktop integration files
- Copies binary to macOS app bundle

### `make package-linux`

Creates a Linux distribution package:
- Generates a proper directory structure for system installation
- Creates desktop file with correct system paths (`/usr/local/bin`, `/usr/share/pixmaps`)
- Packages everything into a `.tar.gz` archive

### `make package-macos`

Creates a macOS app bundle:
- Generates complete `.app` bundle structure
- Includes binary, icons, and metadata
- Creates a `.zip` archive for distribution

## Configuration

The generator script accepts the following command-line options:

- `-output`: Output directory (default: `build/integrations`)
- `-install`: Installation path (default: `/usr/local/bin`)
- `-version`: Application version (default: `dev`)

Environment variables:
- `VERSION`: Application version (auto-detected from git tags)

## Migration from Manual Files

The original manual integration files in `__integrations/` are preserved for reference but are no longer used in the build process. The new system provides:

1. **Consistency**: All integration files use the same configuration values
2. **Maintainability**: Single source of truth for application metadata
3. **Automation**: No manual updates required when version or paths change
4. **Flexibility**: Easy to customise for different distribution scenarios

## Example Usage

```bash
# Generate integrations for development
make generate-integrations

# Build application with integrations
make build-with-integrations

# Create distribution packages
make package-linux
make package-macos

# Custom installation path
VERSION=1.0.0 make package-linux

# Development build with custom version
make generate-integrations VERSION=dev-$(git rev-parse --short HEAD)
```

## File Structure

After running `make build-with-integrations`, the generated structure is:

```
build/integrations/
├── Linux/
│   ├── usm.desktop
│   └── usm.png
└── MacOS/
    └── USM.app/
        ├── Contents/
        │   ├── Info.plist
        │   └── MacOS/
        │       └── usm
        └── Resources/
            └── icon.icns
``` 