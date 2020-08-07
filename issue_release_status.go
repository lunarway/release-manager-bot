package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type PRCreateHandler struct {
	githubapp.ClientCreator
	preamble string

	releaseManagerAuthToken string
	releaseManagerURL       string
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
	// Recieve webhook
	var event github.PullRequestEvent

	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "parsing payload")
	}

	repository := event.GetRepo()
	prNum := event.GetNumber()
	installationID := githubapp.GetInstallationIDFromEvent(&event)

	ctx, logger := githubapp.PreparePRContext(ctx, installationID, repository, prNum)
	if event.GetAction() != "created" {
		return nil
	}

	prBase := event.GetPullRequest().GetBase().GetRef() // The branch which the pull request is ending.

	// Retrieve auto-release-policy

	serviceName := event.GetRepo().GetName()
	serviceName = strings.TrimSuffix(serviceName, "-service")
	serviceName = strings.TrimPrefix(serviceName, "lunar-war-")
	servicePath := handler.releaseManagerURL + "/policies?service="

	// Create client
	client := &http.Client{}

	req, err := http.NewRequest("GET", servicePath+serviceName, nil)
	if err != nil {
		return errors.Wrap(err, "create GET request for release-manager")
	}

	req.Header.Add("Authorization", "Bearer "+handler.releaseManagerAuthToken)

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "sending HTTP request")
	}

	var policyResponse ListPoliciesResponse

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "reading release-manager HTTP response body")
	}

	defer resp.Body.Close()

	err = json.Unmarshal(body, &policyResponse)
	if err != nil {
		return errors.Wrap(err, "parsing release-manager HTTP response body as json")
	}

	var autoReleaseEnvironments []string
	var botMessage string

	if len(policyResponse.AutoReleases) == 0 {
		log.Debug().Msg("No auto-release policies was detected for this service.")
		return nil
	}

	for i := 0; i < len(policyResponse.AutoReleases); i++ {
		if policyResponse.AutoReleases[i].Branch == prBase {
			autoReleaseEnvironments = append(autoReleaseEnvironments, policyResponse.AutoReleases[i].Environment)
		}
	}

	if len(autoReleaseEnvironments) == 0 {
		log.Debug().Msg("No auto-release policies was detected for this base branch.")
		return nil
	}

	botMessage += "Auto-Release Policy detected for service " + serviceName + " at (base)branch " + prBase + "\n\n"
	botMessage += "Merging this Pull Request will release the service to the following environments:\n"

	for i := 0; i < len(autoReleaseEnvironments); i++ {
		botMessage += autoReleaseEnvironments[i] + "\n"
	}

	logger.Debug().Msgf("%s", botMessage)

	/*
		var path string
		var resp httpinternal.ListPoliciesResponse
		var client *httpinternal.Client
		var serviceName string

		params := url.Values{}
		params.Add("service", serviceName)

		path, err := client.URLWithQuery(path, params)
		if err != nil {
			return err
		}

		err = client.Do(http.MethodGet, path, nil, &resp)
	*/

	// Send PR comment

	/*client, err := handler.NewInstallationClient(installationID)
	if err != nil {
		return errors.Wrap(err, "installing client")
	}

	repositoryOwner := repository.GetOwner().GetLogin()
	repositoryName := repository.GetName()

	msg := fmt.Sprintf("To the stars! :-)")
	newComment := github.IssueComment{
		Body: &msg,
	}

	if _, _, err := client.Issues.CreateComment(ctx, repositoryOwner, repositoryName, prNum, &newComment); err != nil {
		logger.Error().Err(err).Msg("Failed to comment on pull request")
	}*/

	return nil
}
