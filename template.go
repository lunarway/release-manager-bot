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

	template := template.New("test")
	template, err := template.Parse(data.Template)
	if err != nil {
		return "", errors.Wrapf(err, "invalid template: %s", data.Template)
		// or "parsing template"
	}

	err = template.Execute(&message, data)
	if err != nil {
		return "", errors.Wrap(err, "applying template to data")
	}

	return message.String(), nil
}
