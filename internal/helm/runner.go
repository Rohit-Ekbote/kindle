package helm

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type RunnerConfig struct {
	HelmBin string
}

type Runner struct {
	bin string
}

func NewRunner(cfg RunnerConfig) *Runner {
	bin := cfg.HelmBin
	if bin == "" {
		bin = "helm"
	}
	return &Runner{bin: bin}
}

type UpgradeParams struct {
	ReleaseName    string
	Namespace      string
	ChartPath      string
	ValuesFile     string
	KubeconfigPath string
	LogPath        string
}

func (r *Runner) Upgrade(p UpgradeParams) error {
	args := []string{
		"upgrade", "--install", p.ReleaseName, p.ChartPath,
		"--namespace", p.Namespace,
		"--create-namespace",
		"--values", p.ValuesFile,
		"--kubeconfig", p.KubeconfigPath,
		"--wait", "--timeout", "10m",
	}
	return r.run(p.LogPath, args...)
}

type StatusParams struct {
	ReleaseName    string
	Namespace      string
	KubeconfigPath string
}

func (r *Runner) Status(p StatusParams) (string, error) {
	cmd := exec.Command(r.bin, "status", p.ReleaseName,
		"--namespace", p.Namespace,
		"--kubeconfig", p.KubeconfigPath,
		"--output", "json",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("helm status: %w", err)
	}
	return string(out), nil
}

func (r *Runner) run(logPath string, args ...string) error {
	dir := filepath.Dir(logPath)
	os.MkdirAll(dir, 0755)

	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(r.bin, args...)
	cmd.Stdout = io.MultiWriter(logFile, os.Stdout)
	cmd.Stderr = io.MultiWriter(logFile, os.Stderr)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm %s: %w", args[0], err)
	}
	return nil
}
