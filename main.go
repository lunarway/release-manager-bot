package main

import (
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/bluekeyes/hatpear"
	"github.com/gregjones/httpcache"
	"github.com/palantir/go-baseapp/baseapp"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/spf13/pflag"
	"goji.io/pat"
)

func main() {
	zerolog.TimestampFieldName = "@timestamp"
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("log_type", "app").
		Logger()
	httpLogger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("log_type", "reqresp").
		Logger()

	releaseManagerAuthToken := pflag.String("release-manager-auth-token", "", "auth token for accessing release manager")
	releaseManagerURL := pflag.String("release-manager-url", "http://localhost:8080", "url to release manager")

	var httpServerConfig baseapp.HTTPConfig
	pflag.StringVar(&httpServerConfig.Address, "http-address", "localhost", "http listen address")
	pflag.IntVar(&httpServerConfig.Port, "http-port", 8080, "http listen port")
	pflag.StringVar(&httpServerConfig.PublicURL, "http-public-url", "https://localhost:8080", "http public url")

	var githubappConfig githubapp.Config
	pflag.StringVar(&githubappConfig.V3APIURL, "github-v3-api-url", "https://api.github.com/", "github v3 api url")
	pflag.Int64Var(&githubappConfig.App.IntegrationID, "github-integration-id", 0, "github App ID (App->General->About->App ID)")
	pflag.StringVar(&githubappConfig.App.WebhookSecret, "github-webhook-secret", "", "github webhook secret")
	pflag.StringVar(&githubappConfig.App.PrivateKey, "github-private-key", "", "github app private key content")

	messageTemplate := pflag.String("message-template", "'{{.Branch}}' will auto-release to: {{range .AutoReleaseEnvironments}}\n {{.}}{{end}}", "Template string used when commenting on pull requests on Github. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].")
	repoFilter := pflag.StringSlice("ignored-repositories", []string{}, "Slice with names of repositories which the bot should not respond to.")

	pflag.Parse()

	// Flag validation
	if *releaseManagerAuthToken == "" {
		logger.Error().Msg("flag 'release-manager-auth-token' is empty")
		os.Exit(1)
		return
	}
	_, err := template.New("flagValidation").Parse(*messageTemplate)
	if err != nil {
		logger.Error().Msgf("flag 'message-template' parsing error recieved: %v", err)
		os.Exit(1)
		return
	}

	server, err := baseapp.NewServer(
		httpServerConfig,
		baseapp.WithLogger(logger),
		baseapp.WithMiddleware(
			hlog.NewHandler(logger),
			hlog.RequestIDHandler("rid", "X-Request-ID"),
			baseapp.AccessHandler(func(r *http.Request, status int, size int64, elapsed time.Duration) {
				httpLogger.Info().Int64("responseTime", elapsed.Milliseconds()).Msgf("[%d] %s %s", status, r.Method, r.URL.String())
			}),
			hatpear.Catch(baseapp.HandleRouteError),
			hatpear.Recover(),
		),
		baseapp.WithUTCNanoTime(),
		baseapp.WithErrorLogging(baseapp.RichErrorMarshalFunc),
		baseapp.WithMetrics(),
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
		messageTemplate:         *messageTemplate,
		repoFilters:             *repoFilter,
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
