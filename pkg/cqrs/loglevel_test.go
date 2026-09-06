package cqrs

import (
	"testing"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/errors"
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
	if !errors.IsInvalidInput(err) {
		t.Fatalf("expected ErrInvalidInput classification, got %v", err)
	}
	if field, ok := err.(interface{ ErrorContext() map[string]any }); ok {
		if field.ErrorContext()["field"] != "logLevel" {
			t.Fatalf("expected field=logLevel, got %v", field.ErrorContext())
		}
	} else {
		t.Fatal("expected structured error context")
	}

	if _, err := NewCQRSStack(t.Context(), cfg); err == nil {
		t.Fatal("expected NewCQRSStack to fail on invalid LogLevel before any store setup")
	}
}

func TestNewCQRSStack_LogLevel_AppliedToStackOwnedLogger(t *testing.T) {
	t.Cleanup(func() { log.Default().SetLevel(log.InfoLevel) })

	stack, err := NewCQRSStack(t.Context(), CQRSConfig{Backend: backendMemory, LogLevel: "warn"})
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

	stack, err := NewCQRSStack(t.Context(), CQRSConfig{Backend: backendMemory})
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

	stack, err := NewCQRSStack(t.Context(), CQRSConfig{
		Backend:     backendMemory,
		EventLogger: discardEventLogger(),
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
