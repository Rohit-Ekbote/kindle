package terraform

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type RunnerConfig struct {
	TerraformBin string
}

type Runner struct {
	bin string
}

func NewRunner(cfg RunnerConfig) *Runner {
	bin := cfg.TerraformBin
	if bin == "" {
		bin = "terraform"
	}
	return &Runner{bin: bin}
}

func (r *Runner) Apply(workDir, logPath string) error {
	return r.run(workDir, logPath, "apply", "-auto-approve", "-input=false")
}

func (r *Runner) Destroy(workDir, logPath string) error {
	return r.run(workDir, logPath, "destroy", "-auto-approve", "-input=false")
}

func (r *Runner) Init(workDir, logPath string) error {
	return r.run(workDir, logPath, "init", "-input=false")
}

func (r *Runner) run(workDir, logPath string, args ...string) error {
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(r.bin, args...)
	cmd.Dir = workDir
	cmd.Stdout = io.MultiWriter(logFile, os.Stdout)
	cmd.Stderr = io.MultiWriter(logFile, os.Stderr)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform %s: %w", args[0], err)
	}
	return nil
}
