// Package token generates and hashes opaque bearer tokens.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// prefix identifies snagbox-issued tokens in plaintext form.
const prefix = "snb_"

// New returns a fresh plaintext token and its hash. The plaintext is
// "snb_" followed by 32 random bytes in raw URL-safe base64; the hash is
// the value returned by Hash.
func New() (plaintext, hash string) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("token: read random: " + err.Error())
	}
	plaintext = prefix + base64.RawURLEncoding.EncodeToString(b[:])
	return plaintext, Hash(plaintext)
}

// Hash returns the hex-encoded SHA-256 of plaintext.
func Hash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
