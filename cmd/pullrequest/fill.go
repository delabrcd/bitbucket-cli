package pullrequest

import (
	"context"
	"fmt"
	"strings"

	"github.com/delabrcd/bitbucket-cli/cmd/common"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
)

type commitInfo struct {
	Subject string
	Body    string
}

// defaultBaseBranch returns the local checkout's notion of the remote default
// branch (origin/HEAD), without the "origin/" prefix.
func defaultBaseBranch(ctx context.Context) (string, error) {
	out, err := common.GitOutput(ctx, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(strings.TrimSpace(out), "origin/"), nil
}

// resolveGitRef returns a ref git can resolve locally, trying the bare name
// first and then origin/<name>.
func resolveGitRef(ctx context.Context, name string) (string, error) {
	if _, err := common.GitOutput(ctx, "rev-parse", "--verify", "--quiet", name+"^{commit}"); err == nil {
		return name, nil
	}
	if _, err := common.GitOutput(ctx, "rev-parse", "--verify", "--quiet", "origin/"+name+"^{commit}"); err == nil {
		return "origin/" + name, nil
	}
	return "", errors.NotFound.With("branch", name)
}

// fillFromCommits builds a title and description from the commits in base..head.
func fillFromCommits(ctx context.Context, base, head string) (title, description string, err error) {
	log := logger.Must(logger.FromContext(ctx)).Child("pullrequest", "fill")

	baseRef, err := resolveGitRef(ctx, base)
	if err != nil {
		return "", "", errors.RuntimeError.Wrap(fmt.Errorf("cannot resolve base branch %q locally for --fill: %w", base, err))
	}
	headRef, err := resolveGitRef(ctx, head)
	if err != nil {
		return "", "", errors.RuntimeError.Wrap(fmt.Errorf("cannot resolve source branch %q locally for --fill: %w", head, err))
	}

	// One record per commit, oldest first: subject \x1f body \x1e
	out, err := common.GitOutput(ctx, "log", "--reverse", "--format=%s%x1f%b%x1e", baseRef+".."+headRef)
	if err != nil {
		return "", "", err
	}

	var commits []commitInfo
	for _, record := range strings.Split(out, "\x1e") {
		record = strings.Trim(record, "\n")
		if record == "" {
			continue
		}
		parts := strings.SplitN(record, "\x1f", 2)
		commit := commitInfo{Subject: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			commit.Body = strings.TrimSpace(parts[1])
		}
		commits = append(commits, commit)
	}

	if len(commits) == 0 {
		return "", "", errors.RuntimeError.Wrap(fmt.Errorf("no commits found in %s..%s for --fill", baseRef, headRef))
	}
	log.Debugf("Found %d commits for --fill", len(commits))

	title = commits[0].Subject
	if len(commits) == 1 {
		description = commits[0].Body
	} else {
		lines := make([]string, 0, len(commits))
		for _, commit := range commits {
			lines = append(lines, "* "+commit.Subject)
		}
		description = strings.Join(lines, "\n")
	}
	return title, description, nil
}
