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
	// Simple map wrapper for template data
	data := map[string]interface{}{
		"vars": vars,
	}
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

func ProcessFiles(files []config.File, vars map[string]string, dryRun bool) {
	logger.Info("=== Start processing file links ===")

	home, _ := os.UserHomeDir()

	for _, f := range files {
		// 1. Render variables in paths (Fix for {{.vars.home}})
		rawSrc := renderPathString(f.Src, vars)
		rawDest := renderPathString(f.Dest, vars)

		// 2. Path expansion (~)
		src := expandPath(rawSrc, home)
		dest := expandPath(rawDest, home)
		
		// Ensure absolute path for source if it exists relative to cwd
		if !filepath.IsAbs(src) && !strings.HasPrefix(src, "~") {
			wd, _ := os.Getwd()
			src = filepath.Join(wd, src)
		}

		logger.Info("File: %s -> %s", dest, src)

		// 1. DryRun / Mkdir
		dir := filepath.Dir(dest)
		if !dryRun {
			os.MkdirAll(dir, 0755)
		} else {
			logger.Debug("[DryRun] MkdirAll %s", dir)
		}

		// 2. Check destination status
		if info, err := os.Lstat(dest); err == nil {
			// 目标存在
			if f.Tpl {
				if !f.Force {
					logger.Warn("  Target exists, skipping (use force=true to overwrite)")
					continue
				}
			} else {
				if info.Mode()&os.ModeSymlink != 0 {
					target, _ := os.Readlink(dest)
					if target == src {
						logger.Success("  Already linked correctly.")
						continue
					}
				}
				
				if !f.Force {
					logger.Warn("  Target exists and is not correct link, skipping.")
					continue
				}
				
				if dryRun {
					logger.Info("  [DryRun] Remove %s", dest)
				} else {
					os.Remove(dest)
				}
			}
		}

		// 3. Execute Action
		if f.Tpl {
			if dryRun {
				logger.Info("  [DryRun] Render template to %s", dest)
			} else {
				if err := renderTemplateFile(src, dest, vars); err != nil {
					logger.Warn("  Template render failed: %v", err)
				} else {
					logger.Success("  Template rendered.")
				}
			}
		} else {
			if dryRun {
				logger.Info("  [DryRun] Ln -s %s %s", src, dest)
			} else {
				if err := os.Symlink(src, dest); err != nil {
					logger.Warn("  Link failed: %v", err)
				} else {
					logger.Success("  Linked.")
				}
			}
		}
	}
}

func expandPath(path, home string) string {
	if strings.HasPrefix(path, "~") {
		return filepath.Join(home, path[1:])
	}
	return path
}

func renderTemplateFile(src, dest string, data map[string]string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// Wrap data for consistency: {{.vars.email}}
	tplData := map[string]interface{}{
		"vars": data,
	}
	
	tmpl, err := template.New("file").Parse(string(b))
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(dest), 0755)
	
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, tplData)
}