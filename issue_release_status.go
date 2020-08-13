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

func retrieveFromReleaseManager(endpoint string, authToken string, output interface{}, logger zerolog.Logger) error {
	httpClient := &http.Client{}

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

	releaseManagerAuthToken string
	releaseManagerURL       string
	messageTemplate         string
	repoFilters             []string
}

func (handler *PRCreateHandler) Handles() []string {
	return []string{"pull_request"}
}

// Handler
func (handler *PRCreateHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	zerolog.Ctx(ctx).Info().Msgf("handling delivery ID: '%s', eventtype '%s'", deliveryID, eventType)

	// Recieve webhook
	var event github.PullRequestEvent

	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "parsing payload")
	}

	repository := event.GetRepo()
	prNum := event.GetNumber()
	installationID := githubapp.GetInstallationIDFromEvent(&event)

	ctx, logger := githubapp.PreparePRContext(ctx, installationID, repository, prNum)

	prBase := event.GetPullRequest().GetBase().GetRef() // The branch which the pull request is ending.
	serviceName := event.GetRepo().GetName()
	serviceName = strings.TrimSuffix(serviceName, "-service")
	serviceName = strings.TrimPrefix(serviceName, "lunar-way-")
	policyPath := handler.releaseManagerURL + "/policies?service="
	describeArtifactPath := handler.releaseManagerURL + "/describe/artifact/"

	// Filters
	// - Action type
	if event.GetAction() != "opened" {
		return nil
	}
	// - Services not managed by release-manager
	var describeArtifactResponse DescribeArtifactResponse
	err := retrieveFromReleaseManager(describeArtifactPath+serviceName, handler.releaseManagerAuthToken, &describeArtifactResponse, logger)
	if err != nil {
		return errors.Wrap(err, "requesting describeArtifact from release manager")
	}
	if len(describeArtifactResponse.Artifacts) == 0 {
		return nil
	}
	// - Ignored repositories
	if any(handler.repoFilters, func(filterRepo string) bool {
		return filterRepo == repository.GetName()
	}) {
		return nil
	}

	// Get policies
	var policyResponse ListPoliciesResponse
	err = retrieveFromReleaseManager(policyPath+serviceName, handler.releaseManagerAuthToken, &policyResponse, logger)
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

	logger.Debug().Msgf("%s", botMessage)

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

	return nil
}
