package sync

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// LoadOrCreateIdentity loads an Ed25519 private key from path, or generates
// and persists a new one if the file does not exist.
func LoadOrCreateIdentity(path string) (crypto.PrivKey, error) {
	if data, err := os.ReadFile(path); err == nil { //nolint:gosec // path is derived from app config, not user input
		raw, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			return nil, fmt.Errorf("could not decode peer key: %w", err)
		}
		return crypto.UnmarshalPrivateKey(raw)
	}

	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("could not generate peer key: %w", err)
	}

	raw, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("could not marshal peer key: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		return nil, fmt.Errorf("could not persist peer key: %w", err)
	}

	return priv, nil
}
