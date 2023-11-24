package main

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/palantir/go-baseapp/baseapp"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
)

func main() {
	// Logging
	zerolog.TimestampFieldName = "@timestamp"
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("log_type", "app").
		Logger()
	httpLogger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("log_type", "reqresp").
		Logger()

	// Flags
	releaseManagerAuthToken := pflag.String("release-manager-auth-token", "", "auth token for accessing release manager")
	releaseManagerURL := pflag.String("release-manager-url", "http://localhost:8080", "url to release manager; without '/' at the end")

	var httpServerConfig baseapp.HTTPConfig
	pflag.StringVar(&httpServerConfig.Address, "http-address", "localhost", "http listen address")
	pflag.IntVar(&httpServerConfig.Port, "http-port", 8080, "http listen port")
	pflag.StringVar(&httpServerConfig.PublicURL, "http-public-url", "https://localhost:8080", "http public url")

	var githubappConfig githubapp.Config
	pflag.StringVar(&githubappConfig.V3APIURL, "github-v3-api-url", "https://api.github.com/", "github v3 api url")
	pflag.Int64Var(&githubappConfig.App.IntegrationID, "github-integration-id", 0, "github App ID (App->General->About->App ID)")
	pflag.StringVar(&githubappConfig.App.WebhookSecret, "github-webhook-secret", "", "github webhook secret")
	pflag.StringVar(&githubappConfig.App.PrivateKey, "github-private-key", "", "github app private key content")
	githubWebhookRoute := pflag.String("github-webhook-route", "/webhook/github/bot", "route to listen for webhooks from Github")

	messageTemplate := pflag.String("message-template", "'{{.Branch}}' will auto-release to: {{range .AutoReleaseEnvironments}}\n {{.}}{{end}}", "Template string used when commenting on pull requests on Github. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].")
	repoFilter := pflag.StringSlice("ignored-repositories", []string{}, "Slice with names of repositories which the bot should not respond to")
	repoToServiceMap := pflag.StringToString("map-repo-to-service", map[string]string{}, "Map where key is repo name and value is assigned/interpreted service name. Ex. usage: '--map-repo-to-service=repo1=service1,repo2=service2'")
	metricsRoute := pflag.String("metrics-route", "/metrics", "route to expect prometheus requests from")

	pflag.Parse()

	// Flag validation
	if *releaseManagerAuthToken == "" {
		logger.Error().Msg("flag 'release-manager-auth-token' is empty")
		os.Exit(1)
		return
	}

	// Template validation, fail fast
	_, err := BotMessage(BotMessageData{
		Template:                *messageTemplate,
		Branch:                  "master",
		AutoReleaseEnvironments: []string{"dev", "prod"},
	})
	if err != nil {
		logger.Error().Msgf("flag 'message-template' parsing error recieved: %v", err)
		os.Exit(1)
		return
	}

	// Metrics
	prometheusRegistry := prometheus.DefaultRegisterer

	// Create Github client
	cc, err := githubapp.NewDefaultCachingClientCreator(
		githubappConfig,
		githubapp.WithClientUserAgent("release-managar-bot/1.0.0"),
		githubapp.WithClientTimeout(3*time.Second),
		githubapp.WithClientCaching(false, func() httpcache.Cache { return httpcache.NewMemoryCache() }),
		githubapp.WithClientMiddleware(
			githubapp.ClientLogging(zerolog.DebugLevel),
			clientMetricsMiddleware(prometheusRegistry, "github"),
			githubMetricsMiddleware(prometheusRegistry),
		),
	)
	if err != nil {
		logger.Error().Msgf("Failed to instatiate Github client: %v", err)
		os.Exit(1)
		return
	}

	pullRequestHandler := &PRCreateHandler{
		ClientCreator:                   cc,
		releaseManagerMetricsMiddleware: clientMetricsMiddleware(prometheusRegistry, "release-manager")(http.DefaultTransport),
		releaseManagerAuthToken:         *releaseManagerAuthToken,
		releaseManagerURL:               *releaseManagerURL,
		messageTemplate:                 *messageTemplate,
		repoFilters:                     *repoFilter,
		repoToServiceMap:                *repoToServiceMap,
	}

	webhookHandler := githubapp.NewDefaultEventDispatcher(githubappConfig, pullRequestHandler)

	// Create http server
	mux := http.NewServeMux()
	mux.Handle(*githubWebhookRoute, inboundMetricsMiddleware(prometheusRegistry, webhookHandler))
	mux.Handle(*metricsRoute, promhttp.Handler())

	// Middleware
	httpHandler := loggerMiddleware(func(msg string, m map[string]interface{}) {
		httpLogger.Info().Fields(m).Msg(msg)
	}, mux)
	httpHandler = loggingContextMiddleware(logger, httpHandler)

	s := http.Server{
		Addr:    net.JoinHostPort("", strconv.Itoa(httpServerConfig.Port)),
		Handler: httpHandler,
	}

	// Serve
	err = s.ListenAndServe()
	if err != nil {
		logger.Error().Msgf("Failed to serve: %v", err)
		os.Exit(1)
	}
}
