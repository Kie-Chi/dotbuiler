package pkgmanager

import (
	"bytes"
	"dotbuilder/pkg/logger"
	"os"
	"path/filepath"
	"text/template"
)

func Prepare(scripts map[string]string, vars map[string]string) (string, error) {
	if len(scripts) == 0 {
		return "", nil
	}

	tmpDir := filepath.Join(os.TempDir(), "dotbuilder_scripts")
	os.RemoveAll(tmpDir) 
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}

	logger.Info("Preparing %d helper scripts in %s...", len(scripts), tmpDir)

	data := map[string]interface{}{"vars": vars}

	for name, content := range scripts {
		tmpl, err := template.New(name).Parse(content)
		if err != nil {
			logger.Warn("Failed to parse script [%s]: %v", name, err)
			continue
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			logger.Warn("Failed to render script [%s]: %v", name, err)
			continue
		}

		scriptPath := filepath.Join(tmpDir, name)
		if err := os.WriteFile(scriptPath, buf.Bytes(), 0755); err != nil {
			logger.Error("Failed to write script [%s]: %v", name, err)
		}
	}

	return tmpDir, nil
}