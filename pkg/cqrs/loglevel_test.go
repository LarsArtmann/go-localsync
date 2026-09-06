package cqrs

import (
	stderrors "errors"
	"log/slog"
	"testing"

	"charm.land/log/v2"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

func TestCQRSConfig_LogLevel_ValidNames(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		cfg := CQRSConfig{Backend: backendMemory, LogLevel: level}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected LogLevel %q to validate, got %v", level, err)
		}
	}
}

func TestCQRSConfig_LogLevel_InvalidRejectedAtConstruction(t *testing.T) {
	cfg := CQRSConfig{Backend: backendMemory, LogLevel: "shout"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected invalid LogLevel to be rejected")
	}
	if !stderrors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput classification, got %v", err)
	}

	if _, cerr := NewCQRSStack(cfg); cerr == nil {
		t.Fatal("expected NewCQRSStack to fail on invalid LogLevel before any store setup")
	}
}

func TestNewCQRSStack_LogLevel_AppliedToStackOwnedLogger(t *testing.T) {
	t.Cleanup(func() { log.Default().SetLevel(log.InfoLevel) })

	stack, err := NewCQRSStack(CQRSConfig{Backend: backendMemory, LogLevel: "warn"})
	if err != nil {
		t.Fatalf("NewCQRSStack: %v", err)
	}
	defer func() { _ = stack.Close() }()

	if got := log.Default().GetLevel(); got != log.WarnLevel {
		t.Fatalf("expected stack-owned logger level warn, got %v", got)
	}
}

func TestNewCQRSStack_LogLevel_EmptyKeepsDefault(t *testing.T) {
	t.Cleanup(func() { log.Default().SetLevel(log.InfoLevel) })
	log.Default().SetLevel(log.InfoLevel)

	stack, err := NewCQRSStack(CQRSConfig{Backend: backendMemory})
	if err != nil {
		t.Fatalf("NewCQRSStack: %v", err)
	}
	defer func() { _ = stack.Close() }()

	if got := log.Default().GetLevel(); got != log.InfoLevel {
		t.Fatalf("expected default level info untouched, got %v", got)
	}
}

func TestNewCQRSStack_LogLevel_IgnoredWhenEventLoggerProvided(t *testing.T) {
	t.Cleanup(func() { log.Default().SetLevel(log.InfoLevel) })
	log.Default().SetLevel(log.InfoLevel)

	stack, err := NewCQRSStack(CQRSConfig{
		Backend:     backendMemory,
		EventLogger: slog.New(slog.DiscardHandler),
		LogLevel:    "warn",
	})
	if err != nil {
		t.Fatalf("NewCQRSStack: %v", err)
	}
	defer func() { _ = stack.Close() }()

	if got := log.Default().GetLevel(); got != log.InfoLevel {
		t.Fatalf("expected consumer-provided EventLogger to own level control (info untouched), got %v", got)
	}
}
