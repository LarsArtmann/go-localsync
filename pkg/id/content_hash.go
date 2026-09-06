package id

// ContentHash is the SHA-256 hex digest of an item's raw provider payload.
// A named string type (not a struct): literal assignment and comparison keep
// compiling, while function signatures gain compile-time protection against
// mixing hashes with arbitrary strings.
type ContentHash string

// NewContentHash brands a SHA-256 hex string as a ContentHash.
func NewContentHash(hex string) ContentHash { return ContentHash(hex) }

// IsZero reports whether the hash is empty (provider set no raw payload).
func (h ContentHash) IsZero() bool { return h == "" }

// String returns the underlying hex digest.
func (h ContentHash) String() string { return string(h) }
