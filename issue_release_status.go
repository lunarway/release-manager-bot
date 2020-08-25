package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Util
func any(vs []string, f func(string) bool) bool {
	for _, v := range vs {
		if f(v) {
			return true
		}
	}
	return false
}

func retrieveFromReleaseManager(endpoint string, authToken string, output interface{}, logger zerolog.Logger, metricMiddleware http.RoundTripper) error {
	httpClient := &http.Client{Transport: metricMiddleware}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return errors.Wrapf(err, "create GET request for release-manager endpoint '%s'", endpoint)
	}

	req.Header.Add("Authorization", "Bearer "+authToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "sending HTTP request")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "reading release-manager HTTP response body")
	}

	if resp.StatusCode != 200 {
		logger.Info().Msgf("Request body: %v", body)
		return errors.Errorf("expected status code 200, but recieved " + fmt.Sprintf("%v", resp.StatusCode))
	}

	err = json.Unmarshal(body, output)
	if err != nil {
		return errors.Wrap(err, "parsing release-manager HTTP response body as json")
	}

	return nil
}

// Structs
type PRCreateHandler struct {
	githubapp.ClientCreator
	preamble string

	releaseManagerMetricsMiddleware http.RoundTripper
	releaseManagerAuthToken         string
	releaseManagerURL               string
	messageTemplate                 string
	repoFilters                     []string
	logger                          zerolog.Logger
}

func (handler *PRCreateHandler) Handles() []string {
	return []string{"pull_request"}
}

// Handler
func (handler *PRCreateHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	// Recieve webhook
	var event github.PullRequestEvent

	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "parsing payload")
	}

	repository := event.GetRepo()
	prNum := event.GetNumber()
	installationID := githubapp.GetInstallationIDFromEvent(&event)

	logctx := zerolog.Ctx(ctx).With().
		Int64("github_installation_id", installationID).
		Str("github_repository_owner", repository.GetOwner().GetLogin()).
		Str("github_repository_name", repository.GetName()).
		Int("github_pr_num", prNum)

	logger := logctx.Logger()
	ctx = logger.WithContext(ctx)

	logger.Info().Msgf("Handling deliveryID: '%s', eventType '%s'", deliveryID, eventType)

	prBase := event.GetPullRequest().GetBase().GetRef() // The branch which the pull request is ending.
	serviceName := event.GetRepo().GetName()
	serviceName = strings.TrimSuffix(serviceName, "-service")
	serviceName = strings.TrimPrefix(serviceName, "lunar-way-")
	policyPath := handler.releaseManagerURL + "/policies?service="
	describeArtifactPath := handler.releaseManagerURL + "/describe/artifact/"

	// Filters - Consider using Chain of Responsibility for this if it gets bloated.
	// - Action type
	if event.GetAction() != "opened" && event.GetAction() != "edited" {
		logger.Info().Msgf("Filter ActionType triggered. Action: '%s'", event.GetAction())
		return nil
	}
	// - Edited; but no change in base branch
	if event.GetAction() == "edited" {
		if event.Changes == nil {
			logger.Info().Msg("Filter NoChanges triggered") // Check in some weeks if this state has every been triggered 25/08/2020
			return nil
		} else if event.Changes.Base == nil { // to prevent nil dereference
			logger.Info().Msg("Filter NoBaseChanges triggered")
			return nil
		}
	}
	// - Services not managed by release-manager
	var describeArtifactResponse DescribeArtifactResponse
	err := retrieveFromReleaseManager(describeArtifactPath+serviceName, handler.releaseManagerAuthToken, &describeArtifactResponse, logger, handler.releaseManagerMetricsMiddleware)
	if err != nil {
		return errors.Wrap(err, "requesting describeArtifact from release manager")
	}
	if len(describeArtifactResponse.Artifacts) == 0 {
		logger.Info().Msgf("Filter UnmanagedService triggered. Service: '%s'", serviceName)
		return nil
	}
	// - Ignored repositories
	if any(handler.repoFilters, func(filterRepo string) bool {
		return filterRepo == repository.GetName()
	}) {
		logger.Info().Msgf("Filter IgnoredRepo triggered. Repo: '%s'", repository.GetName())
		return nil
	}

	// Get policies
	var policyResponse ListPoliciesResponse
	err = retrieveFromReleaseManager(policyPath+serviceName, handler.releaseManagerAuthToken, &policyResponse, logger, handler.releaseManagerMetricsMiddleware)
	if err != nil {
		return errors.Wrap(err, "requesting policy from release manager")
	}

	var autoReleaseEnvironments []string
	for i := 0; i < len(policyResponse.AutoReleases); i++ {
		if policyResponse.AutoReleases[i].Branch == prBase {
			autoReleaseEnvironments = append(autoReleaseEnvironments, policyResponse.AutoReleases[i].Environment)
		}
	}

	messageData := BotMessageData{
		Branch:                  prBase,
		AutoReleaseEnvironments: autoReleaseEnvironments,
		Template:                handler.messageTemplate,
	}
	botMessage, err := BotMessage(messageData)
	if err != nil {
		return errors.Wrapf(err, "creating bot message")
	}

	// Send PR comment
	client, err := handler.NewInstallationClient(installationID)
	if err != nil {
		return errors.Wrapf(err, "creating new github.Client from installation id '%d'", installationID)
	}

	repositoryOwner := repository.GetOwner().GetLogin()
	repositoryName := repository.GetName()

	// It's intentional that it's an IssueComment. The alternative PullRequestComment is a review comment
	newComment := github.IssueComment{
		Body: &botMessage,
	}

	if _, _, err := client.Issues.CreateComment(ctx, repositoryOwner, repositoryName, prNum, &newComment); err != nil {
		return errors.Wrapf(err, "commenting on pull request, with DeliveryID '%v'", deliveryID)
	}

	logger.Info().Msgf("Comment created on %s PR %d", repositoryName, *event.PullRequest.Number)

	return nil
}
