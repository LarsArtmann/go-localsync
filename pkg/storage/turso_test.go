package storage

import "testing"

func TestIsRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"libsql scheme", "libsql://my-db.turso.io", true},
		{"https scheme", "https://my-db.turso.io", true},
		{"http scheme", "http://my-db.turso.io", true},
		{"file scheme", "file:/tmp/test.db", false},
		{"file with double slash", "file:///tmp/test.db", false},
		{"empty string", "", false},
		{"local path", "/tmp/test.db", false},
		{"relative path", "./test.db", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRemoteURL(tt.url)
			if got != tt.expected {
				t.Errorf("isRemoteURL(%q) = %v, want %v", tt.url, got, tt.expected)
			}
		})
	}
}
