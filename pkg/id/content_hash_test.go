package id

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestContentHash_ConstructorAndRoundTrip covers NewContentHash/String/IsZero
// and the literal-compatibility contract that makes the named string type
// ergonomical: an untyped string constant assigns directly, while typed
// mismatches stay compile errors.
func TestContentHash_ConstructorAndRoundTrip(t *testing.T) {
	t.Parallel()

	raw := "deadbeef"
	hash := NewContentHash(raw)

	if hash.String() != raw {
		t.Errorf("String() = %q, want %q", hash.String(), raw)
	}

	if hash.IsZero() {
		t.Error("non-empty hash must not be zero")
	}

	if !ContentHash("").IsZero() {
		t.Error("empty hash must be zero")
	}

	// sha256 compatibility: the documented construction path (hex of a
	// SHA-256 over the raw payload) produces a valid ContentHash that
	// round-trips through String.
	sum := sha256.Sum256([]byte("payload"))
	fromHex := NewContentHash(hex.EncodeToString(sum[:]))

	if fromHex.String() != hex.EncodeToString(sum[:]) {
		t.Errorf("sha256 hex round trip failed: %q", fromHex.String())
	}

	if fromHex.IsZero() {
		t.Error("derived hash must not be zero")
	}
}

// TestContentHash_LiteralCompat pins the deliberate type decision (ADR-0002
// branded-type style): a named string, NOT a struct. Untyped constants
// assign and compare without conversion; identity is the string value.
func TestContentHash_LiteralCompat(t *testing.T) {
	t.Parallel()

	var hash ContentHash = "abc123" // untyped constant literal assignment

	if hash != "abc123" {
		t.Errorf("literal comparison failed: %q", hash)
	}

	if NewContentHash("abc123") != hash {
		t.Error("constructor and literal with same value must be equal")
	}

	// Equality is value-based, not pointer/identity-based.
	if NewContentHash("x") == NewContentHash("y") {
		t.Error("distinct hashes must not be equal")
	}
}
