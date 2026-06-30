package pullrequest

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	"github.com/delabrcd/bitbucket-cli/cmd/repository"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

// commitStatus is a build/check status reported on a commit
// (GET /repositories/{ws}/{repo}/commit/{sha}/statuses).
type commitStatus struct {
	Key   string `json:"key"   mapstructure:"key"`
	Name  string `json:"name"  mapstructure:"name"`
	State string `json:"state" mapstructure:"state"`
	URL   string `json:"url"   mapstructure:"url"`
}

// label returns the most human-friendly identifier for the status.
func (status commitStatus) label() string {
	if len(status.Name) > 0 {
		return status.Name
	}
	return status.Key
}

// unmetStatuses returns the statuses that are not SUCCESSFUL (FAILED, STOPPED,
// INPROGRESS, …). These are the checks that should block a merge.
func unmetStatuses(statuses []commitStatus) []commitStatus {
	var unmet []commitStatus
	for _, status := range statuses {
		if !strings.EqualFold(status.State, "SUCCESSFUL") {
			unmet = append(unmet, status)
		}
	}
	return unmet
}

// verifyMergeChecks bails if the pull request's head commit carries any build
// status that is not SUCCESSFUL.
//
// The Bitbucket merge API evaluates the required-passing-builds check against
// the token owner's permissions, and silently honors an admin's override with
// no confirmation — so an admin token can merge a PR with a FAILED or
// in-progress build. This client-side guard restores the gate regardless of the
// token's privileges.
func verifyMergeChecks(ctx context.Context, cmd *cobra.Command, prof *profile.Profile, repo *repository.Repository, pullRequestID string) error {
	log := logger.Must(logger.FromContext(ctx)).Child("pullrequest", "mergechecks")

	var pullrequest PullRequest
	if err := prof.Get(ctx, cmd, repo.GetPath("pullrequests", pullRequestID), &pullrequest); err != nil {
		return errors.Join(errors.Errorf("failed to get pull request %s to verify merge checks", pullRequestID), err)
	}
	if pullrequest.Source.Commit == nil || len(pullrequest.Source.Commit.Hash) == 0 {
		log.Warnf("Pull request %s has no resolvable head commit; skipping build-status verification", pullRequestID)
		fmt.Fprintf(os.Stderr, "Warning: could not resolve the head commit of pull request %s; skipping build-status verification\n", pullRequestID)
		return nil
	}
	sha := pullrequest.Source.Commit.Hash

	statuses, err := profile.GetAll[commitStatus](ctx, cmd, repo.GetPath("commit", sha, "statuses"))
	if err != nil {
		return errors.Join(errors.Errorf("failed to get build statuses for commit %s", sha), err)
	}

	unmet := unmetStatuses(statuses)
	if len(unmet) == 0 {
		if len(statuses) == 0 {
			fmt.Fprintf(os.Stderr, "No build statuses on %s; nothing to verify.\n", pullrequest.Source.Commit.GetShortHash())
		} else {
			fmt.Fprintf(os.Stderr, "Pre-merge checks passed: %d build status(es) SUCCESSFUL on %s.\n", len(statuses), pullrequest.Source.Commit.GetShortHash())
		}
		return nil
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "refusing to merge pull request %s: %d of %d build check(s) on %s are not successful (use --skip-checks to override):",
		pullRequestID, len(unmet), len(statuses), pullrequest.Source.Commit.GetShortHash())
	for _, status := range unmet {
		fmt.Fprintf(&builder, "\n  - %s: %s", status.label(), strings.ToUpper(status.State))
	}
	return errors.New(builder.String())
}
