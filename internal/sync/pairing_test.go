package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePairingCode(t *testing.T) {
	code, secret, err := GeneratePairingCode()
	require.NoError(t, err)
	assert.Len(t, code, 6)
	assert.NotEmpty(t, secret)

	for _, c := range code {
		assert.Contains(t, pairingAlphabet, string(c))
	}
}

func TestGeneratePairingCode_NoDuplicates(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code, _, err := GeneratePairingCode()
		require.NoError(t, err)
		seen[code] = true
	}
	assert.Greater(t, len(seen), 90)
}

func TestVerifyPairingHMAC(t *testing.T) {
	_, secret, err := GeneratePairingCode()
	require.NoError(t, err)

	nonce := make([]byte, 32)
	mac := ComputePairingHMAC(secret, nonce)

	assert.True(t, VerifyPairingHMAC(secret, nonce, mac))
	assert.False(t, VerifyPairingHMAC(secret, nonce, []byte("wrong")))
}

func TestVerifyPairingHMAC_WithCodeAsKey(t *testing.T) {
	// This mirrors the actual pairing flow: code is used as HMAC key
	code, _, err := GeneratePairingCode()
	require.NoError(t, err)

	nonce := make([]byte, 32)
	mac := ComputePairingHMAC([]byte(code), nonce)

	assert.True(t, VerifyPairingHMAC([]byte(code), nonce, mac))
	assert.False(t, VerifyPairingHMAC([]byte("WRONG1"), nonce, mac), "wrong code should fail")
	assert.False(t, VerifyPairingHMAC([]byte(code), []byte("different-nonce-value!!!!!!!!!!!"), mac), "wrong nonce should fail")
}
