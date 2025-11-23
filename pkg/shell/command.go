package shell

import (
	"dotbuilder/pkg/logger"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Runner executes shell commands
type Runner struct {
	Env    map[string]string
	DryRun bool
}

func NewRunner(dryRun bool) *Runner {
	return &Runner{
		Env:    make(map[string]string),
		DryRun: dryRun,
	}
}

// formatCmdForLog shortens multi-line commands for display
func formatCmdForLog(cmdStr string) string {
	lines := strings.Split(strings.TrimSpace(cmdStr), "\n")
	if len(lines) > 1 {
		return fmt.Sprintf("%s ... (%d lines)", strings.TrimSpace(lines[0]), len(lines))
	}
	return lines[0]
}

func (r *Runner) ExecStream(cmdStr string) error {
	displayCmd := formatCmdForLog(cmdStr)

	if r.DryRun {
		fmt.Printf("\033[36m[PLAN]\033[0m %s\n", displayCmd)
		return nil
	}

	logger.Debug("Exec: %s", displayCmd)

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Env = os.Environ()
	for k, v := range r.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	cmd.Stdin = os.Stdin
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	return cmd.Wait()
}

func (r *Runner) ExecSilent(cmdStr string) int {
	if r.DryRun {
		return 1 
	}

	logger.Debug("ExecSilent: %s", formatCmdForLog(cmdStr))
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Env = os.Environ()
	for k, v := range r.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode()
		}
		return 1
	}
	return 0
}

func CheckCommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}