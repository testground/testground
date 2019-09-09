package api

import "time"

type TriggerSource int

const (
	TriggerSourceManual TriggerSource = iota
	TriggerSourceGithubMention
	TriggerSourceGithubCommit
	TriggerSourceGithubRelease
)

type RepoCommand struct {
	TriggerContext TriggerContext
}

type TriggerContext struct {
	Timestamp time.Time
	// Source is an enum indicating the method by which this run was triggered.
	Source TriggerSource
	// User carries the username that triggered this run.
	User string
	// RepoURL carries the URL of the upstream repo subject of test.
	RepoURL string
	// CommitSHA indicates the commit hash to be subjected to testing.
	CommitSHA string
	// Release carries the release name, if this run was triggered by a release.
	Release string
	// Branch carries the branch name, if this run was triggered by a commit or a mention.
	Branch string
	// PullRequestURL carries the URL of the pull request, if this release was triggered by a mention.
	PullRequestURL string
}
