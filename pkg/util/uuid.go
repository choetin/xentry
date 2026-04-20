// Package util provides shared helper functions used across the application.
package util

import (
	"crypto/rand"
	"encoding/hex"
)

// UUID generates a random 128-bit identifier encoded as 32 lowercase hex characters.
func UUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Token generates a random API token prefixed with "xentry_".
// The token consists of a 32-byte random hex string (64 characters).
func Token() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "xentry_" + hex.EncodeToString(b)
}
