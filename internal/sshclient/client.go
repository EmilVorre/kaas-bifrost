package sshclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v4"
	"golang.org/x/crypto/ssh"
)

// RunResult holds captured output from a remote command.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// ErrCommandFailed is returned when a remote command exits non-zero.
type ErrCommandFailed struct {
	Result RunResult
}

func (e *ErrCommandFailed) Error() string {
	return fmt.Sprintf("remote command failed (exit %d): %s", e.Result.ExitCode, e.Result.Stderr)
}

// Client runs commands on a single SSH host.
type Client struct {
	cfg        Config
	sshConfig  *ssh.ClientConfig
	conn       *ssh.Client
	maxRetries time.Duration
}

// ClientOption configures optional Client behaviour.
type ClientOption func(*Client)

// WithMaxRetryElapsed sets how long Connect keeps retrying transient dial errors.
func WithMaxRetryElapsed(d time.Duration) ClientOption {
	return func(c *Client) {
		c.maxRetries = d
	}
}

// New validates config and prepares a client. Call Connect before Run.
func New(cfg Config, opts ...ClientOption) (*Client, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	sshCfg, err := cfg.clientConfig()
	if err != nil {
		return nil, err
	}

	c := &Client{
		cfg:        cfg,
		sshConfig:  sshCfg,
		maxRetries: 2 * time.Minute,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Connect dials the host, retrying transient network errors.
func (c *Client) Connect(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}

	addr := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))
	var conn *ssh.Client

	operation := func() error {
		if ctx.Err() != nil {
			return backoff.Permanent(ctx.Err())
		}

		dialer := &net.Dialer{Timeout: c.cfg.Timeout}
		netConn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			if !isRetryableDialError(err) {
				return backoff.Permanent(fmt.Errorf("dial %s: %w", addr, err))
			}
			return fmt.Errorf("dial %s: %w", addr, err)
		}

		sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, c.sshConfig)
		if err != nil {
			_ = netConn.Close()
			if !isRetryableDialError(err) {
				return backoff.Permanent(fmt.Errorf("ssh handshake %s: %w", addr, err))
			}
			return fmt.Errorf("ssh handshake %s: %w", addr, err)
		}

		conn = ssh.NewClient(sshConn, chans, reqs)
		return nil
	}

	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = c.maxRetries

	if err := backoff.Retry(operation, backoff.WithContext(bo, ctx)); err != nil {
		return err
	}

	c.conn = conn
	return nil
}

// Run executes command on the remote host. Connect must have succeeded.
// Non-zero exit codes return *ErrCommandFailed with stdout/stderr populated.
func (c *Client) Run(ctx context.Context, command string) (RunResult, error) {
	if c.conn == nil {
		return RunResult{}, errors.New("ssh: not connected")
	}

	sess, err := c.conn.NewSession()
	if err != nil {
		return RunResult{}, fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	done := make(chan error, 1)
	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	go func() {
		done <- sess.Run(command)
	}()

	select {
	case <-ctx.Done():
		_ = sess.Close()
		return RunResult{}, ctx.Err()
	case err := <-done:
		result := RunResult{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}
		if err == nil {
			result.ExitCode = 0
			return result, nil
		}

		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitStatus()
			return result, &ErrCommandFailed{Result: result}
		}
		return result, fmt.Errorf("run command: %w", err)
	}
}

// Close closes the SSH connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

// Addr returns host:port for logging and errors.
func (c *Client) Addr() string {
	return net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))
}
