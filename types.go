package main

import "time"

// Would be nice to be able to do, instead of this:
// httpinternal "github.com/lunarway/release-manager/internal/http"

// Policy
type ListPoliciesResponse struct {
	Service            string                    `json:"service,omitempty"`
	AutoReleases       []AutoReleasePolicy       `json:"autoReleases,omitempty"`
	BranchRestrictions []BranchRestrictionPolicy `json:"branchRestrictions,omitempty"`
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

// describeArtifact
type DescribeArtifactResponse struct {
	Service   string `json:"service,omitempty"`
	Artifacts []Spec `json:"artifacts,omitempty"`
}

type Spec struct {
	ID          string     `json:"id,omitempty"`
	Service     string     `json:"service,omitempty"`
	Namespace   string     `json:"namespace,omitempty"`
	Application Repository `json:"application,omitempty"`
	CI          CI         `json:"ci,omitempty"`
	Squad       string     `json:"squad,omitempty"`
	Shuttle     Shuttle    `json:"shuttle,omitempty"`
	Stages      []Stage    `json:"stages,omitempty"`
}

type Repository struct {
	Branch         string `json:"branch,omitempty"`
	SHA            string `json:"sha,omitempty"`
	AuthorName     string `json:"authorName,omitempty"`
	AuthorEmail    string `json:"authorEmail,omitempty"`
	CommitterName  string `json:"committerName,omitempty"`
	CommitterEmail string `json:"committerEmail,omitempty"`
	Message        string `json:"message,omitempty"`
	Name           string `json:"name,omitempty"`
	URL            string `json:"url,omitempty"`
	Provider       string `json:"provider,omitempty"`
}

type Shuttle struct {
	Plan           Repository `json:"plan,omitempty"`
	ShuttleVersion string     `json:"shuttleVersion,omitempty"`
}

type CI struct {
	JobURL string    `json:"jobUrl,omitempty"`
	Start  time.Time `json:"start,omitempty"`
	End    time.Time `json:"end,omitempty"`
}

type Stage struct {
	ID   string      `json:"id,omitempty"`
	Name string      `json:"name,omitempty"`
	Data interface{} `json:"data,omitempty"`
}
