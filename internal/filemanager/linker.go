package filemanager

import (
	"bytes"
	"dotbuilder/internal/config"
	"dotbuilder/pkg/logger"
	"os"
	"path/filepath"
	"strings"
	"text/template"
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

func ProcessFiles(files []config.File, vars map[string]string, dryRun bool, baseDir string) {
	logger.Info("=== Start processing file links ===")
	
	var fs FileSystem
	if dryRun {
		fs = DryRunFS{}
	} else {
		fs = RealFS{}
	}

	for _, f := range files {
		ProcessSingleFile(f, vars, fs, baseDir)
	}
}

func ProcessSingleFile(f config.File, vars map[string]string, fs FileSystem, baseDir string) {
	home, _ := os.UserHomeDir()
	rawSrc := renderPathString(f.Src, vars)
	rawDest := renderPathString(f.Dest, vars)
	
	src := expandPath(rawSrc, home)
	dest := expandPath(rawDest, home)

	// Fix: Resolve relative path based on config location (BaseDir)
	if !filepath.IsAbs(src) && !strings.HasPrefix(src, "~") {
		src = filepath.Join(baseDir, src)
	}

	logger.Info("File: %s -> %s", dest, src)

	dir := filepath.Dir(dest)
	fs.MkdirAll(dir, 0755)

	if info, err := fs.Lstat(dest); err == nil {
		if f.Tpl {
			if !f.Force {
				logger.Warn("  Target exists, skipping (use force=true)")
				return
			} else {
				fs.Remove(dest)
			}
		} else {
			if info.Mode()&os.ModeSymlink != 0 {
				target, _ := fs.Readlink(dest)
				if target == src {
					logger.Success("  Already linked correctly.")
					return
				}
			}
			if !f.Force {
				logger.Warn("  Target exists/incorrect, skipping.")
				return
			}
			fs.Remove(dest)
		}
	}

	if f.Tpl {
		if err := renderTemplateFile(src, dest, vars, fs); err != nil {
			logger.Warn("  Template render failed: %v", err)
		} else {
			logger.Success("  Template rendered.")
		}
	} else {
		if err := fs.Symlink(src, dest); err != nil {
			logger.Warn("  Link failed: %v", err)
		} else {
			logger.Success("  Linked.")
		}
	}
}

func expandPath(path, home string) string {
	path = os.ExpandEnv(path)
	if strings.HasPrefix(path, "~") {
		return filepath.Join(home, path[1:])
	}
	return path
}

func renderTemplateFile(src, dest string, data map[string]string, fs FileSystem) error {
	b, err := fs.ReadFile(src)
	if err != nil {
		return err
	}
	tplData := map[string]interface{}{"vars": data}
	tmpl, err := template.New("file").Parse(string(b))
	if err != nil {
		return err
	}
	
	// Prepare buffer to write
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tplData); err != nil {
		return err
	}
	
	// Check source perms
	var mode os.FileMode = 0644
	if info, err := fs.Stat(src); err == nil {
		mode = info.Mode().Perm()
	}

	return fs.WriteFile(dest, buf.Bytes(), mode)
}