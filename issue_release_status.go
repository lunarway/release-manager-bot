package main

import (
	"context"
	"encoding/json"
	"fmt"

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

func (handler *PRCreateHandler) Handle(ctx context.Context, eventType, deliveryID string, payload []byte) error {
	var event github.PullRequestEvent

	if err := json.Unmarshal(payload, &event); err != nil {
		return errors.Wrap(err, "failed to parse pull request event payload")
	}

	// Måske lave flere checks?

	repository := event.GetRepo()
	prNum := event.GetNumber()
	installationID := githubapp.GetInstallationIDFromEvent(&event)

	ctx, logger := githubapp.PreparePRContext(ctx, installationID, repository, prNum)

	client, err := handler.NewInstallationClient(installationID)
	if err != nil {
		return err
	}

	repositoryOwner := repository.GetOwner().GetLogin()
	repositoryName := repository.GetName()

	msg := fmt.Sprintf("To the stars! :-)")
	newComment := github.IssueComment{
		Body: &msg,
	}

	if _, _, err := client.Issues.CreateComment(ctx, repositoryOwner, repositoryName, prNum, &newComment); err != nil {
		logger.Error().Err(err).Msg("Failed to comment on pull request")
	}

	return nil
}
