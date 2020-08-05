package main

import (
	"io/ioutil"

	"github.com/palantir/go-baseapp/baseapp"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Server baseapp.HTTPConfig `yaml:"server"`
	Github githubapp.Config   `yaml:"github"`

	AppConfig MyApplicationConfig `yaml:"app_configuration"`
}

type MyApplicationConfig struct {
	PullRequestPreamble string `yaml:"pull_request_preamble"`
}

func ReadConfig(path string) (*Config, error) {
	var c Config

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading server config file")
	}

	if err := yaml.UnmarshalStrict(bytes, &c); err != nil {
		return nil, errors.Wrap(err, "parsing configuration file as yaml")
	}

	return &c, nil
}
