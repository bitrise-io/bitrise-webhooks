package templateutil

import (
	"bytes"
	"text/template"
)

// EvaluateTemplateStringToString ...
func EvaluateTemplateStringToString(templateContent string, inventory interface{}, funcs template.FuncMap) (string, error) {
	tmpl := template.New("").Funcs(funcs)
	tmpl, err := tmpl.Parse(templateContent)
	if err != nil {
		return "", err
	}

	var resBuffer bytes.Buffer
	if err := tmpl.Execute(&resBuffer, inventory); err != nil {
		return "", err
	}

	return resBuffer.String(), nil
}
