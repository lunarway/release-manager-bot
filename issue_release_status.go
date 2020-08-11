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

type PRCreateHandler struct {
	githubapp.ClientCreator
	preamble string

	releaseManagerAuthToken string
	releaseManagerURL       string
	messageTemplate         string
}

func (handler *PRCreateHandler) Handles() []string {
	return []string{"pull_request"}
}

type AutoReleasePolicy struct {
	ID          string `json:"id,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Environment string `json:"environment,omitempty"`
}

type BranchRestrictionPolicy struct {
	ID          string `json:"id,omitempty"`
	Environment string `json:"environment,omitempty"`
	BranchRegex string `json:"branchRegex,omitempty"`
}

type ListPoliciesResponse struct {
	Service            string                    `json:"service,omitempty"`
	AutoReleases       []AutoReleasePolicy       `json:"autoReleases,omitempty"`
	BranchRestrictions []BranchRestrictionPolicy `json:"branchRestrictions,omitempty"`
}

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
	if event.GetAction() != "opened" {
		return nil
	}

	prBase := event.GetPullRequest().GetBase().GetRef() // The branch which the pull request is ending.

	// Retrieve auto-release-policy

	serviceName := event.GetRepo().GetName()
	serviceName = strings.TrimSuffix(serviceName, "-service")
	serviceName = strings.TrimPrefix(serviceName, "lunar-way-")
	servicePath := handler.releaseManagerURL + "/policies?service="

	// Create client
	httpClient := &http.Client{}

	req, err := http.NewRequest("GET", servicePath+serviceName, nil)
	if err != nil {
		return errors.Wrap(err, "create GET request for release-manager")
	}

	req.Header.Add("Authorization", "Bearer "+handler.releaseManagerAuthToken)

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

	var policyResponse ListPoliciesResponse

	err = json.Unmarshal(body, &policyResponse)
	if err != nil {
		return errors.Wrap(err, "parsing release-manager HTTP response body as json")
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
