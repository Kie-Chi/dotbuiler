package pkgmanager

import (
	"bytes"
	"dotbuilder/pkg/logger"
	"text/template"
)

func RenderCmd(tplStr string, data interface{}) string {
	tmpl, err := template.New("cmd").Parse(tplStr)
	if err != nil {
		logger.Warn("Template Parse Error: %v (Content: %s)", err, tplStr)
		return tplStr // Fallback
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		logger.Warn("Template Execute Error: %v (Content: %s)", err, tplStr)
		return tplStr
	}
	return buf.String()
}