package main

import (
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

type BotMessageData struct {
	Template                string
	Branch                  string
	AutoReleaseEnvironments []string
}

func BotMessage(data BotMessageData) (string, error) {
	var message strings.Builder

	if data.Template == "" {
		return "", errors.New("template is empty")
	}

	templateFuncs := template.FuncMap{
		"contains":   strings.Contains,
		"replaceAll": strings.ReplaceAll,
	}

	template := template.New("test")
	template, err := template.Funcs(templateFuncs).Parse(data.Template)
	if err != nil {
		return "", errors.Wrapf(err, "parsing template: '%s'", data.Template)
	}

	err = template.Execute(&message, data)
	if err != nil {
		return "", errors.Wrap(err, "applying template to data")
	}

	return message.String(), nil
}
