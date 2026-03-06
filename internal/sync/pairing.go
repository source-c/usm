package sync

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/hkdf"
)

const pairingAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

const pairingCodeLength = 6

// GeneratePairingCode creates a 6-character pairing code and the underlying secret.
// The code is derived from HKDF so it is not simply random bytes mod alphabet.
func GeneratePairingCode() (code string, secret []byte, err error) {
	secret = make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, fmt.Errorf("could not generate pairing secret: %w", err)
	}

	reader := hkdf.New(sha256.New, secret, nil, []byte("usm-pair"))
	alphabetLen := byte(len(pairingAlphabet))
	// ATTN: largest multiple of alphabetLen that fits in a byte, used for rejection sampling
	maxUnbiased := 256 - (256 % int(alphabetLen))
	codeBytes := make([]byte, pairingCodeLength)
	for i := range codeBytes {
		for {
			var b [1]byte
			if _, err := reader.Read(b[:]); err != nil {
				return "", nil, fmt.Errorf("could not derive pairing code: %w", err)
			}
			if int(b[0]) < maxUnbiased {
				codeBytes[i] = pairingAlphabet[b[0]%alphabetLen]
				break
			}
		}
	}

	return string(codeBytes), secret, nil
}

// ComputePairingHMAC computes HMAC-SHA256(secret, nonce) for the pairing protocol
func ComputePairingHMAC(secret, nonce []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(nonce)
	return mac.Sum(nil)
}

// VerifyPairingHMAC checks that the provided MAC matches the expected HMAC
func VerifyPairingHMAC(secret, nonce, receivedMAC []byte) bool {
	expected := ComputePairingHMAC(secret, nonce)
	return hmac.Equal(expected, receivedMAC)
}
