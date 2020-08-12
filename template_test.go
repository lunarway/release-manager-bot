package main

import "testing"

func TestBotMessage(t *testing.T) {
	tt := []struct {
		name  	string
		input 	BotMessageData
		output  string
	}{
		{
			name:	"normal template",
			input:	BotMessageData{
				Template: "",
				Branch:   "master",
				AutoReleaseEnvironments: []string{
					"dev",
				},
			},
			output:	
		},
	}

}
