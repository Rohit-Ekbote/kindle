package iapssh

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	Project string
	Zone    string
}

type Client struct {
	cfg Config
}

func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg}
}

// Dial opens an SSH connection to the named VM via IAP tunnel.
// The portal's service account must have roles/iap.tunnelResourceAccessor.
// The VM must have OS Login enabled (enable-oslogin=TRUE metadata).
func (c *Client) Dial(ctx context.Context, vmName string) (*ssh.Client, error) {
	localPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("find free port: %w", err)
	}

	// Start IAP tunnel in background
	tunnel := exec.CommandContext(ctx,
		"gcloud", "compute", "start-iap-tunnel", vmName, "22",
		"--local-host-port", fmt.Sprintf("localhost:%d", localPort),
		"--zone", c.cfg.Zone,
		"--project", c.cfg.Project,
	)
	if err := tunnel.Start(); err != nil {
		return nil, fmt.Errorf("start IAP tunnel: %w", err)
	}

	// Wait for tunnel to be ready (up to 15 seconds)
	addr := fmt.Sprintf("localhost:%d", localPort)
	if err := waitForPort(addr, 15*time.Second); err != nil {
		tunnel.Process.Kill()
		return nil, fmt.Errorf("IAP tunnel not ready: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            osLoginUser(),
		Auth:            []ssh.AuthMethod{ssh.PublicKeysCallback(loadOSLoginKey)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		tunnel.Process.Kill()
		return nil, fmt.Errorf("SSH dial: %w", err)
	}
	return client, nil
}

// RunCommand opens a session on the SSH client, runs cmd, and returns stdout.
func RunCommand(client *ssh.Client, cmd string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.Output(cmd)
}

// ParseZoneAndRegion splits a GCP zone (e.g. us-central1-a) into zone and region.
func ParseZoneAndRegion(zone string) (string, string) {
	parts := strings.Split(zone, "-")
	if len(parts) < 3 {
		return zone, zone
	}
	region := strings.Join(parts[:len(parts)-1], "-")
	return zone, region
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s", addr)
}

func osLoginUser() string {
	return ""
}

func loadOSLoginKey() ([]ssh.Signer, error) {
	return nil, nil
}
