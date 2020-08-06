package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/google/go-github/v32/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
)

type PRCreateHandler struct {
	githubapp.ClientCreator

	preamble string
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
	authToken := "XXXXXX" //Has to use env flags or other things to be more secure
	host := "http://localhost:8081"
	servicePath := host + "/policies?service="

	// Create client
	client := &http.Client{}

	// Set route
	resp, err := client.Get(servicePath + serviceName)
	if err != nil {
		return errors.Wrap(err, "issueing a GET to specified URL")
	}

	req, err := http.NewRequest("GET", servicePath+serviceName, nil) // dublicate work??
	if err != nil {
		return errors.Wrap(err, "wrapping new request")
	}

	req.Header.Add("Authorization", "Bearer "+authToken)

	resp, err = client.Do(req)

	var policyResponse ListPoliciesResponse

	body, err := ioutil.ReadAll(resp.Body)

	defer resp.Body.Close()

	json.Unmarshal(body, &policyResponse)

	var autoReleaseEnvironments []string
	var botMessage string

	// Not that easy to read, sry
	if len(policyResponse.AutoReleases) > 0 {
		for i := 0; i < len(policyResponse.AutoReleases); i++ {
			if policyResponse.AutoReleases[i].Branch == prBase {
				autoReleaseEnvironments = append(autoReleaseEnvironments, policyResponse.AutoReleases[i].Environment)
			}
		}

		botMessage += "Auto-Release Policy detected for service " + serviceName + " at branch " + prBase + "\n\n"
		botMessage += "Merging this PR will release the service to the following environments:\n"

		for i := 0; i < len(autoReleaseEnvironments); i++ {
			botMessage += autoReleaseEnvironments[i] + "\n"
		}
	} else {
		botMessage += "No auto-release policies was detected for the base branch"
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
