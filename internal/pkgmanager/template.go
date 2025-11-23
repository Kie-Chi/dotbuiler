package pkgmanager

import (
	"bytes"
	"text/template"
)

// RenderCmd renders a command template using structured data
func RenderCmd(tplStr string, data interface{}) string {
	tmpl, err := template.New("cmd").Parse(tplStr)
	if err != nil {
		return tplStr // Fallback
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return tplStr
	}
	return buf.String()
}