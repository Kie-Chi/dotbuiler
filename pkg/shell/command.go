package shell

import (
	"bufio"
	"dotbuilder/pkg/logger"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Global lock to prevent interleaved output lines from concurrent tasks
var outputMu sync.Mutex

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

func (r *Runner) ExecStream(cmdStr string, id string) error {
	displayCmd := formatCmdForLog(cmdStr)

	if r.DryRun {
		outputMu.Lock()
		fmt.Printf("\033[36m[PLAN][%s]\033[0m %s\n", id, displayCmd)
		outputMu.Unlock()
		return nil
	}

	logger.Debug("[%s] Exec: %s", id, displayCmd)

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Env = os.Environ()
	for k, v := range r.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Create pipes
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	streamOutput := func(pipe io.Reader, isErr bool) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			text := scanner.Text()
			outputMu.Lock()
			prefixColor := "\033[34m" // Blue
			if isErr {
				prefixColor = "\033[31m" // Red
			}
			fmt.Printf("%s[%s]\033[0m %s\n", prefixColor, id, text)
			outputMu.Unlock()
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); streamOutput(stdoutPipe, false) }()
	go func() { defer wg.Done(); streamOutput(stderrPipe, true) }()

	wg.Wait()
	return cmd.Wait()
}

func (r *Runner) ExecSilent(cmdStr string) int {
	if r.DryRun {
		// In DryRun, we assume checks fail (so installation proceeds/simulates)
		// unless explicitly handled otherwise.
		return 1
	}

	logger.Debug("ExecSilent: %s", formatCmdForLog(cmdStr))
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Env = os.Environ()
	for k, v := range r.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		logger.Debug("  -> Check Output: %s", strings.TrimSpace(string(output)))
	}

	if err != nil {
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