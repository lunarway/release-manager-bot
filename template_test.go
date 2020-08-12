package main

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestBotMessage(t *testing.T) {
	tt := []struct {
		name            string
		input           BotMessageData
		expectedMessage string
		expectedError   error
	}{
		{
			name: "valid template",
			input: BotMessageData{
				Template: "'{{.Branch}}' will auto-release to: {{range .AutoReleaseEnvironments}}\n {{.}}{{end}}",
				Branch:   "master",
				AutoReleaseEnvironments: []string{
					"dev",
				},
			},
			expectedMessage: "'master' will auto-release to: \n dev",
			expectedError:   nil,
		},
		{
			name: "invalid template",
			input: BotMessageData{
				Template: "'{{.Branch}}' will auto-release to: {{range .AutoReleaseEnvironments}}\n {{.}}",
				Branch:   "master",
				AutoReleaseEnvironments: []string{
					"dev",
				},
			},
			expectedMessage: "",
			expectedError:   errors.New(""),
		},
		{
			name: "no template",
			input: BotMessageData{
				Template: "",
				Branch:   "master",
				AutoReleaseEnvironments: []string{
					"dev",
				},
			},
			expectedMessage: "",
			expectedError:   errors.New(""),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			actualMessage, actualError := BotMessage(tc.input)

			// Assert
			assert.Equal(t, tc.expectedMessage, actualMessage)

			if tc.expectedError != nil {
				assert.Error(t, actualError)
			} else {
				assert.NoError(t, actualError)
			}
		})
	}
}
