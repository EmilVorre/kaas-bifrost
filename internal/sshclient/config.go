package sshclient

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	defaultPort    = 22
	defaultTimeout = 30 * time.Second
)

// Config holds SSH connection parameters for a single host.
type Config struct {
	Host       string
	Port       int
	User       string
	PrivateKey string
	Timeout    time.Duration
}

// withDefaults returns a copy of cfg with unset fields filled in.
func (cfg Config) withDefaults() Config {
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	return cfg
}

func (cfg Config) validate() error {
	if cfg.Host == "" {
		return fmt.Errorf("ssh host is required")
	}
	if cfg.User == "" {
		return fmt.Errorf("ssh user is required")
	}
	if cfg.PrivateKey == "" {
		return fmt.Errorf("ssh private key path is required")
	}
	return nil
}

func (cfg Config) clientConfig() (*ssh.ClientConfig, error) {
	keyPath, err := expandPath(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("expand private key path: %w", err)
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key %q: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // bootstrap tooling; tighten when known_hosts is wired
		Timeout:         cfg.Timeout,
	}, nil
}

func expandPath(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
