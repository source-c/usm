<div align="center">
    <img alt="usm" src="logo/usm.png" height="128" />
</div>

# USM - Simple, modern and privacy-focused Open Source secrets manager

USM is a secrets manager designed to offer a secure and user-friendly solution for managing your digital data across multiple platforms, featuring modern encryption, making it an ideal tool for both personal and professional use.

It is written in Go and uses [Fyne](https://github.com/fyne-io/fyne) as UI toolkit and [age](https://github.com/FiloSottile/age) as encryption library.

## Warning

**This software has not undergone a formal third-party security audit.**

That said, USM is built from the ground up with strong security principles: every cryptographic choice follows established best practices, the build pipeline enforces static analysis and vulnerability scanning on every change, and the codebase is fully open for review. See [Security posture](#security-posture) for details.

## Main features

* Cross-platform application (Linux, macOS, Windows, BSD) with a single codebase
* Desktop, Mobile and CLI application with a single binary
* Minimal direct dependencies
* LAN synchronisation between instances (peer-to-peer, no cloud)
* Versioned configuration with cryptographic integrity via [Viracochan](https://github.com/source-c/viracochan)
* Agent to handle SSH keys and CLI sessions
* Browser extension integration via native messaging
* Open source: code can be audited
* Audit passwords against data breaches
* TOTP support
* Password import/export

### Later goals

* Web application (however, this is highly debatable)
* Passwords management and generation enhancements

## Installation

### Development version

To try the development version or help with testing:

```
git clone https://github.com/source-c/usm.git usm-go
cd usm-go
make help
make clean && make generate-mocks && make check && make build
make generate-integrations
```

## Documentation

- [Configuration Management](docs/configuration.md) - Versioned configuration system using Viracochan
- [LAN Synchronisation](docs/synchronisation.md) - Peer-to-peer vault sync over local network
- [Desktop Integrations](docs/integrations.md) - Desktop integration file generation for Linux and macOS

## Build identification

Each binary built via `make build` embeds version, build ID (git commit) and build time via linker flags.

```
$ usm cli version
usm version v1.0.0 (abc1234 2026-03-05T12:00:00Z)
instance: 550e8400-e29b-41d4-a716-446655440000
```

The **instance ID** is a UUID v4 generated on first run and persisted in the application state. It uniquely identifies a USM installation for synchronisation and backup purposes.

When building outside of `make`, the version falls back to Go module metadata or `(unknown)`.

## How it works

### Cryptography

#### Vault initialisation

One or more vaults can be initialised to store secrets and identities.

When a vault is initialised the user is prompted for a vault name and password.
An [age](https://github.com/FiloSottile/age) key is generated and encrypted using an age Scrypt recipient with the provided password, then saved on disk (`key.age`).
The X25519 identity and its recipient from the key file are used to decrypt and encrypt the vault data.
Each item is stored separately on disk so that the content can be decrypted manually using the age tool, if needed.
All items' metadata are encrypted and stored in the `vault.age` file so that no information is in clear text.

#### Random password

Random passwords are derived by reading byte-by-byte from an [HKDF](https://pkg.go.dev/golang.org/x/crypto/hkdf) cryptographic key derivation function that uses the age key as secret. Printable characters that match the desired password rule (uppercase, lowercase, symbols and digits) are included in the generated password. Rejection sampling eliminates modular bias in character selection.

#### Custom password

Where a generated password is not applicable a custom password can be specified.

### Versioned configuration (Viracochan)

Application state (preferences, vault catalogue, instance identity) is managed by [Viracochan](https://github.com/source-c/viracochan), a library that provides immutable versioned configuration with cryptographic integrity guarantees:

* **Append-only chain** - every preference change creates a new immutable version; existing versions are never modified.
* **SHA-256 integrity** - each version is checksummed over its canonical JSON content and linked to its predecessor's checksum, forming a tamper-evident chain.
* **Journaling** - all operations are recorded in an append-only log, enabling audit trails and disaster recovery.
* **State reconstruction** - configuration can be rebuilt from scattered files or out-of-order journal entries.
* **Rollback** - any previous version can be restored without losing the history.

The [Merkle-Forest](https://canny.substack.com/p/merkleforest-privacy) extension of this model adds privacy-preserving properties through hiding commitments and zero-knowledge proofs, allowing integrity verification without exposing configuration content to external observers.

See [docs/configuration.md](docs/configuration.md) for integration details.

### LAN synchronisation

USM instances on the same local network can synchronise vaults peer-to-peer. No data leaves the LAN.

* **Discovery** via mDNS - instances advertise and find each other automatically.
* **Transport** via [libp2p](https://github.com/libp2p/go-libp2p) - Noise-encrypted TCP streams; no relay, no DHT, LAN only.
* **Pairing** - a 6-character HKDF-derived code (with HMAC verification) establishes mutual trust between devices.
* **Three sync modes**: Disabled (zero network activity), Relaxed (pairing required), Strict (pairing + vault key-fingerprint challenge on every sync).
* **Atomic transfer** - received data lands in a staging directory; an atomic commit-with-backup ensures the vault is never left in a partial state, with automatic rollback on crash.

See [docs/synchronisation.md](docs/synchronisation.md) for the full protocol specification.

### Vault structure

Vault internally is organized hierarchically like:
```
- vault
    ├── login
    |    └── www.example.com
    |    └── my.site.com
    ├── password
    |    └── mypassword
    ├── ssh_key
    |    └── id_ed25519
    └── note
         └── mysecretnote
```

### Items

Items are typed templates for identity and secret management.

Currently available:

- **login** - website credentials (username, password, URI, TOTP, favicon)
- **password** - standalone passwords or passphrases
- **ssh_key** - SSH keypairs with optional agent integration
- **note** - encrypted free-form text

## Security posture

USM has not undergone a formal external audit. However, the project enforces a layered defence through its build and review pipeline:

| Layer | Tool / Practice | Purpose |
|-------|----------------|---------|
| Static analysis | [golangci-lint](https://golangci-lint.run/) (gosec, gocritic, revive, gocyclo, ...) | Catches security anti-patterns, complexity, and code smells |
| Vulnerability scanning | [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) | Detects known CVEs in dependencies |
| Race detection | `go test -race` | Identifies data races in concurrent code |
| Formatting | gofumpt + gci | Enforces consistent, reviewable code |
| Dependency hygiene | Minimal direct dependencies; `go.sum` integrity | Reduces supply-chain surface |
| Cryptographic choices | age (X25519 + Scrypt), HKDF-SHA256, HMAC-SHA256, Ed25519, libp2p Noise | Proven primitives; no custom cryptography |
| Configuration integrity | Viracochan SHA-256 chain with journaling | Tamper-evident, auditable state |

Every `make check` run executes the full suite: tests with race detection, linting, and vulnerability scanning. The CI gate rejects any commit that introduces a linter violation or a known vulnerability.

## Threat model

The threat model of USM assumes _there are no attackers on your local machine_.

When LAN sync is enabled, the additional assumption is that paired peers are trusted. The Noise transport encrypts all traffic, and Strict mode adds per-sync vault key-fingerprint verification to guard against compromised or re-keyed peers.

## Contribute

Contributions are welcome! Please feel free to submit a Pull Request.

## Credits

### Libraries and Tools
 - [age](https://github.com/FiloSottile/age) for the encryption library
 - [Fyne](https://github.com/fyne-io/fyne) for the UI toolkit
 - [libp2p](https://github.com/libp2p/go-libp2p) for peer-to-peer networking
 - [Viracochan](https://github.com/source-c/viracochan) for versioned configuration with cryptographic integrity
 - [Tabler icons](https://tabler.io/icons) for the icons

### Inspiration
Thanks to these Open Source password managers that inspired the USM project:

- [gopass](https://github.com/gopasspw/gopass)
- [lesspass](https://github.com/lesspass/lesspass)
- [pass](https://www.passwordstore.org/)
- [passage](https://github.com/FiloSottile/passage)
- [passgo](https://github.com/ejcx/passgo)

This project was originally heavily inspired by [Paw](https://github.com/lucor/paw) by Luca Corbo, but has since evolved into USM with a distinct architecture, UX and branding.


## License

MIT License only - meaning you are only the one who decides what to do next. There is no intention to make this project anyhow commercial.