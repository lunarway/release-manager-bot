package main

import (
	"flag"
	"os"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/palantir/go-baseapp/baseapp"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/rs/zerolog"
	"goji.io/pat"
)

func main() {

	configPath := flag.String("config-path", "config.yml", "path to configuration file")
	flag.Parse()

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

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
		githubapp.WithClientUserAgent("example-app/1.0.0"),
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
		ClientCreator: cc,
		preamble:      config.AppConfig.PullRequestPreamble,
	}

	webhookHandler := githubapp.NewDefaultEventDispatcher(config.Github, pullRequestHandler)

	server.Mux().Handle(pat.Post("/webhook/github"), webhookHandler)

	// Start is blocking
	err = server.Start()
	if err != nil {
		logger.Error().Msgf("Failed to serve: %v", err)
		os.Exit(1)
		return
	}

}
