package filemanager

import (
	"bytes"
	"dotbuilder/internal/config"
    "dotbuilder/pkg/shell"
	"dotbuilder/pkg/logger"
	"os"
    "os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"dotbuilder/internal/errors"
)

// Helper to render path strings
func renderPathString(tplStr string, vars map[string]string) string {
	data := map[string]interface{}{"vars": vars}
	tmpl, err := template.New("path").Parse(tplStr)
	if err != nil {
		return tplStr
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return tplStr
	}
	return buf.String()
}

func runCheckCommand(cmdStr string, vars map[string]string) bool {
    finalCmd := renderPathString(cmdStr, vars)
	cmd := exec.Command("sh", "-c", finalCmd)
	return cmd.Run() == nil // Exit Code 0 means true
}

func ProcessFiles(files []config.File, vars map[string]string, runner *shell.Runner, baseDir string) {
	logger.Info("=== Start processing file links ===")

	var fs FileSystem
	if runner.DryRun {
		fs = DryRunFS{}
	} else {
		fs = RealFS{}
	}

	for _, f := range files {
		ProcessSingleFile(f, vars, fs, baseDir, runner)
	}
}


func ProcessSingleFile(f config.File, vars map[string]string, fs FileSystem, baseDir string, runner *shell.Runner) error {
	if f.Check != "" {
		renderedCheck := renderPathString(f.Check, vars)
		if runner.ExecSilent(renderedCheck) == 0 {
			logger.Success("  File Check passed for dest '%s' (Skipped).", f.Dest)
			return errors.NewSkipError("Check passed")
		}
		logger.Debug("  File Check failed for dest '%s', proceeding.", f.Dest)
	}

	if f.Override && f.Append {
		logger.Error("File config error: 'override' and 'append' cannot be both true for dest: %s", f.Dest)
		return nil
	}

	home, _ := os.UserHomeDir()
	rawSrc := renderPathString(f.Src, vars)
	rawDest := renderPathString(f.Dest, vars)

	src := expandPath(rawSrc, home)
	dest := expandPath(rawDest, home)

	if !filepath.IsAbs(src) && !strings.HasPrefix(src, "~") {
		src = filepath.Join(baseDir, src)
	}

	logger.InfoFile("%s -> %s", dest, src)

	var srcContent []byte
	var err error

	if f.Tpl {
		srcContent, err = renderContent(src, vars, fs)
	} else {
		srcContent, err = fs.ReadFile(src)
	}

	if err != nil {
		logger.Error("  Failed to read/render source: %v", err)
		return err
	}

	dir := filepath.Dir(dest)
	fs.MkdirAll(dir, 0755)

	destInfo, err := fs.Lstat(dest)
	destExists := err == nil

	if destExists {
		if !f.Tpl && destInfo.Mode()&os.ModeSymlink != 0 {
			target, _ := fs.Readlink(dest)
			if target == src {
				logger.Success("  Already linked correctly.")
				return errors.NewSkipError("Already linked")
			}
		}

		if f.Tpl || f.Override {
			destContent, errRead := fs.ReadFile(dest)
			if errRead == nil && bytes.Equal(destContent, srcContent) {
				logger.Success("  Content identical (Skipped).")
				return errors.NewSkipError("Content identical")
			}
		}
	}

	if f.Append {
		if !destExists {
			logger.InfoFile("Creating new file (Append): %s", dest)
			if err := fs.WriteFile(dest, srcContent, 0644); err != nil {
				logger.Error("  Write failed: %v", err)
				return err
			}
			logger.Success("  Created.")
			return nil
		}

		destContent, err := fs.ReadFile(dest)
		if err != nil {
			logger.Error("  Failed to read target for append check: %v", err)
			return err
		}

		if bytes.Contains(destContent, srcContent) {
			logger.Success("  Content already exists in target (Skipped).")
			return errors.NewSkipError("Content exists")
		}

		logger.InfoFile("Appending content to: %s", dest)
		if len(destContent) > 0 && destContent[len(destContent)-1] != '\n' {
			destContent = append(destContent, '\n')
		}
		newContent := append(destContent, srcContent...)
		if err := fs.WriteFile(dest, newContent, destInfo.Mode()); err != nil {
			logger.Error("  Append failed: %v", err)
			return err
		}
		logger.Success("  Appended.")
		return nil
	}

	if destExists {
		shouldOverride := f.Override
		dryRun := runner.DryRun

		if f.Override && f.OverrideIf != "" {
			if dryRun {
				logger.Info("  [DryRun] Check command: %s -> assume true", f.OverrideIf)
				shouldOverride = true
			} else {
				if runCheckCommand(f.OverrideIf, vars) {
					logger.Info("  Check passed, proceeding to override.")
					shouldOverride = true
				} else {
					logger.Warn("  Check failed (exit code != 0), skipping override.")
					shouldOverride = false
				}
			}
		}
		if !shouldOverride {
			logger.Warn("  Target exists, skipping (override=false or check failed).")
			return errors.NewSkipError("Target exists")
		}

		logger.InfoFile("Removing existing target: %s", dest)
		fs.Remove(dest)
	}

	if f.Tpl {
		if err := fs.WriteFile(dest, srcContent, 0644); err != nil {
			logger.Error("  Write failed: %v", err)
			return err
		}
		logger.Success("  Template rendered and written.")
	} else {
		if err := fs.Symlink(src, dest); err != nil {
			logger.Warn("  Link failed: %v", err)
			return err
		}
		logger.Success("  Linked.")
	}

	return nil
}

func renderContent(src string, data map[string]string, fs FileSystem) ([]byte, error) {
	b, err := fs.ReadFile(src)
	if err != nil {
		return nil, err
	}
	tplData := map[string]interface{}{"vars": data}
	tmpl, err := template.New("file").Parse(string(b))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tplData); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func expandPath(path, home string) string {
	path = os.ExpandEnv(path)
	if strings.HasPrefix(path, "~") {
		return filepath.Join(home, path[1:])
	}
	return path
}