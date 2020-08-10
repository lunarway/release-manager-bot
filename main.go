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

	releaseManagerAuthToken := pflag.String("release-manager-auth-token", "", "auth token for accessing release manager")
	releaseManagerURL := pflag.String("release-manager-url", "http://localhost:8080", "url to release manager")

	var httpServerConfig baseapp.HTTPConfig
	pflag.StringVar(&httpServerConfig.Address, "http-address", "localhost", "http listen address")
	pflag.IntVar(&httpServerConfig.Port, "http-port", 8080, "http listen port")
	pflag.StringVar(&httpServerConfig.PublicURL, "http-public_url", "https://localhost:8080", "http public url")

	var githubappConfig githubapp.Config
	pflag.StringVar(&githubappConfig.V3APIURL, "github-v3_api_url", "https://api.github.com/", "github v3 api url")
	pflag.Int64Var(&githubappConfig.App.IntegrationID, "github-integrationID", 0, "github App ID (App->General->About->App ID)")
	pflag.StringVar(&githubappConfig.App.WebhookSecret, "github-webhookSecret", "", "github webhook secret")
	pflag.StringVar(&githubappConfig.App.PrivateKey, "github-privateKey", "", "github app private key content")

	pflag.Parse()

	if *releaseManagerAuthToken == "" {
		logger.Error().Msgf("flag release-manager-auth-token is empty")
		os.Exit(1)
		return
	}

	server, err := baseapp.NewServer(
		httpServerConfig,
		baseapp.DefaultParams(logger, "exampleapp.")...,
	)
	if err != nil {
		logger.Error().Msgf("Failed to instatiate http server: %v", err)
		os.Exit(1)
		return
	}

	cc, err := githubapp.NewDefaultCachingClientCreator(
		githubappConfig,
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
		releaseManagerAuthToken: *releaseManagerAuthToken,
		releaseManagerURL:       *releaseManagerURL,
	}

	webhookHandler := githubapp.NewDefaultEventDispatcher(githubappConfig, pullRequestHandler)

	server.Mux().Handle(pat.Post("/webhook/github/bot"), webhookHandler)

	// Start is blocking
	err = server.Start()
	if err != nil {
		logger.Error().Msgf("Failed to serve: %v", err)
		os.Exit(1)
		return
	}

}
