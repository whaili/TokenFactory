package lab01

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// seal encrypts plaintext with XChaCha20-Poly1305.
//
// XChaCha20 (the "X" meaning extended nonce, 192-bit) is used rather than
// standard ChaCha20-Poly1305 (96-bit nonce) because the nonce here is
// random per message with no counter, so a larger nonce space removes
// the birthday-bound concern entirely. At 32 random bytes per message
// the collision probability is negligible for any realistic volume.
//
// The nonce is passed separately (not prepended) so the caller controls
// the wire format; see pasetoToken.Validate for the layout.
func seal(key, nonce, plaintext []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("key must be %d bytes, got %d", chacha20poly1305.KeySize, len(key))
	}
	if len(nonce) != chacha20poly1305.NonceSizeX {
		return nil, fmt.Errorf("nonce must be %d bytes, got %d", chacha20poly1305.NonceSizeX, len(nonce))
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("new aead: %w", err)
	}
	return aead.Seal(nil, nonce, plaintext, nil), nil
}

// open decrypts and authenticates ciphertext produced by seal.
//
// A failed authentication returns an error rather than partial plaintext —
// this is the authenticated-encryption contract and the reason a forged
// or tampered token can never yield usable claims.
func open(key, nonce, ciphertext []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("key must be %d bytes, got %d", chacha20poly1305.KeySize, len(key))
	}
	if len(nonce) != chacha20poly1305.NonceSizeX {
		return nil, fmt.Errorf("nonce must be %d bytes, got %d", chacha20poly1305.NonceSizeX, len(nonce))
	}
	if len(ciphertext) < chacha20poly1305.Overhead {
		return nil, errors.New("ciphertext shorter than tag")
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("new aead: %w", err)
	}
	return aead.Open(nil, nonce, ciphertext, nil)
}
