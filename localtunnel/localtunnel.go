package localtunnel

import (
	"bufio"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/rs/zerolog"
)

// Config holds the settings needed to start a localtunnel process.
type Config struct {
	Subdomain string // localtunnel subdomain (--subdomain flag)
	AppHost   string // local host:port string (e.g. ":8000")
}

// Tunnel wraps a running localtunnel (lt) subprocess.
type Tunnel struct {
	cmd    *exec.Cmd
	done   chan error
	logger zerolog.Logger
}

// New starts a localtunnel process and returns a Tunnel.
// Returns an error if the lt binary is not found or the process fails to start.
func New(cfg Config, logger zerolog.Logger) (*Tunnel, error) {
	if _, err := exec.LookPath("lt"); err != nil {
		return nil, fmt.Errorf("localtunnel: lt binary not found in PATH: %w", err)
	}

	port, err := extractPort(cfg.AppHost)
	if err != nil {
		return nil, fmt.Errorf("localtunnel: %w", err)
	}
	if cfg.Subdomain == "" {
		return nil, fmt.Errorf("localtunnel: subdomain required")
	}

	args := []string{"--port", port, "--subdomain", cfg.Subdomain}
	cmd := exec.Command("lt", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("localtunnel: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("localtunnel: start: %w", err)
	}

	t := &Tunnel{cmd: cmd, done: make(chan error, 1), logger: logger}

	go func() {
		in := bufio.NewScanner(stdout)
		for in.Scan() {
			logger.Info().Msg(in.Text())
		}
		if err := in.Err(); err != nil {
			logger.Error().Err(err).Msg("localtunnel stdout")
		}
	}()

	go func() {
		t.done <- cmd.Wait()
	}()

	return t, nil
}

// Close kills the localtunnel process and waits for it to exit.
func (t *Tunnel) Close() error {
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}
	_ = t.cmd.Process.Kill()
	<-t.done
	return nil
}

// extractPort parses the port from a host string like ":8000" or "http://host:8000".
func extractPort(h string) (string, error) {
	if h == "" {
		return "", fmt.Errorf("host required")
	}
	if strings.HasPrefix(h, ":") {
		return strings.TrimPrefix(h, ":"), nil
	}
	u, err := url.Parse(h)
	if err != nil {
		return "", fmt.Errorf("parse host %q: %w", h, err)
	}
	port := u.Port()
	if port == "" {
		return "", fmt.Errorf("could not determine port from host %q", h)
	}
	return port, nil
}
