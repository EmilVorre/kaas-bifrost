package sshclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got, err := expandPath("~/.ssh/id_ed25519")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".ssh", "id_ed25519")
	if got != want {
		t.Fatalf("expandPath() = %q, want %q", got, want)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := Config{}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected validation error for empty config")
	}
}
