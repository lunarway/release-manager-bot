package main

import (
	"os"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/palantir/go-baseapp/baseapp"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"goji.io/pat"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	configPath := pflag.String("config-path", "config.yml", "path to configuration file")
	releaseManagerAuthToken := pflag.String("release-manager-auth-token", "", "auth token for accessing release manager")
	releaseManagerURL := pflag.String("release-manager-url", "http://localhost:8080", "url to release manager")
	pflag.Parse()

	if *releaseManagerAuthToken == "" {
		logger.Error().Msgf("flag release-manager-auth-token is empty")
		os.Exit(1)
		return
	}

	config, err := ReadConfig(*configPath)
	if err != nil {
		logger.Error().Msgf("Failed to parse config: %v", err)
		os.Exit(1)
		return
	}

	server, err := baseapp.NewServer(
		config.Server,
		baseapp.DefaultParams(logger, "exampleapp.")...,
	)
	if err != nil {
		logger.Error().Msgf("Failed to instatiate http server: %v", err)
		os.Exit(1)
		return
	}

	cc, err := githubapp.NewDefaultCachingClientCreator(
		config.Github,
		githubapp.WithClientUserAgent("release-managar-bot/1.0.0"),
		githubapp.WithClientTimeout(3*time.Second),
		githubapp.WithClientCaching(false, func() httpcache.Cache { return httpcache.NewMemoryCache() }),
		githubapp.WithClientMiddleware(
			githubapp.ClientMetrics(server.Registry()),
		),
	)
	if err != nil {
		logger.Error().Msgf("Failed to instatiate Github client: %v", err)
		os.Exit(1)
		return
	}

	pullRequestHandler := &PRCreateHandler{
		ClientCreator:           cc,
		preamble:                config.AppConfig.PullRequestPreamble,
		releaseManagerAuthToken: *releaseManagerAuthToken,
		releaseManagerURL:       *releaseManagerURL,
	}

	webhookHandler := githubapp.NewDefaultEventDispatcher(config.Github, pullRequestHandler)

	server.Mux().Handle(pat.Post("/webhook/github/bot"), webhookHandler)

	// Start is blocking
	err = server.Start()
	if err != nil {
		logger.Error().Msgf("Failed to serve: %v", err)
		os.Exit(1)
		return
	}

}
