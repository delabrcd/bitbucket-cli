package step

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	"github.com/delabrcd/bitbucket-cli/cmd/repository"
	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/spf13/cobra"
)

// LogOptions controls which step logs SelectAndDumpLogs writes.
type LogOptions struct {
	Step   string // explicit step uuid-or-name; takes precedence over Failed/All
	Failed bool   // only the steps that finished with a failing result
	All    bool   // every step of the pipeline
}

// IsFailed reports whether the step finished with a failing result
// (FAILED or ERROR). A step still running or successful returns false.
func (step Step) IsFailed() bool {
	if step.State.Result == nil {
		return false
	}
	switch strings.ToUpper(step.State.Result.Name) {
	case "FAILED", "ERROR":
		return true
	}
	return false
}

// SelectAndDumpLogs resolves which steps of the given pipeline to show
// according to opts, then writes their logs to w. When more than one step is
// written, each is preceded by a banner so the output stays navigable.
//
// Selection precedence:
//   - opts.Step set    → that single step (by uuid or name), no banner.
//   - opts.All set     → every step.
//   - opts.Failed set  → only failed steps (none → friendly notice, no error).
//   - otherwise        → the sole step if there is one; else the failed
//     step(s) if any failed; else an error listing the steps to choose from.
func SelectAndDumpLogs(ctx context.Context, cmd *cobra.Command, pipelineID string, opts LogOptions, w io.Writer) error {
	log := logger.Must(logger.FromContext(ctx)).Child("pipeline", "dumplogs")

	prof, err := profile.GetProfileFromCommand(ctx, cmd)
	if err != nil {
		return err
	}
	repo, err := repository.GetRepository(ctx, cmd)
	if err != nil {
		return err
	}

	if len(opts.Step) > 0 {
		stepID, err := GetPipelineStepID(ctx, cmd, pipelineID, opts.Step)
		if err != nil {
			return errors.Join(errors.Errorf("failed to find step %s in pipeline %s", opts.Step, pipelineID), err)
		}
		return dumpStepLog(ctx, cmd, prof, repo, pipelineID, stepID, "", w)
	}

	steps, err := profile.GetAll[Step](ctx, cmd, repo.GetPath("pipelines", pipelineID, "steps"))
	if err != nil {
		return errors.Join(errors.Errorf("failed to get steps for pipeline %s", pipelineID), err)
	}
	if len(steps) == 0 {
		return errors.NotFound.With("pipeline steps", pipelineID)
	}

	var selected []Step
	switch {
	case opts.All:
		selected = steps
	case opts.Failed:
		selected = core.Filter(steps, func(s Step) bool { return s.IsFailed() })
		if len(selected) == 0 {
			fmt.Fprintf(os.Stderr, "Pipeline %s has no failed steps.\n", pipelineID)
			return nil
		}
	default:
		if len(steps) == 1 {
			selected = steps
			break
		}
		failed := core.Filter(steps, func(s Step) bool { return s.IsFailed() })
		if len(failed) > 0 {
			fmt.Fprintf(os.Stderr, "Pipeline %s has %d steps; showing the %d failed step(s). Use --all for every step or --step <name> to pick one.\n", pipelineID, len(steps), len(failed))
			selected = failed
			break
		}
		var b strings.Builder
		fmt.Fprintf(&b, "pipeline %s has %d steps; pick one with --step <name>, or use --all / --failed. Steps:", pipelineID, len(steps))
		for _, s := range steps {
			fmt.Fprintf(&b, "\n  - %s [%s]", s.Name, s.State.String())
		}
		return errors.New(b.String())
	}

	log.Debugf("Selected %d of %d steps for pipeline %s", len(selected), len(steps), pipelineID)
	withBanner := len(selected) > 1
	var merr errors.MultiError
	for _, s := range selected {
		banner := ""
		if withBanner {
			banner = fmt.Sprintf("%s\n Step: %s   State: %s\n%s", strings.Repeat("=", 80), s.Name, s.State.String(), strings.Repeat("=", 80))
		}
		if err := dumpStepLog(ctx, cmd, prof, repo, pipelineID, s.ID.String(), banner, w); err != nil {
			merr.Append(err)
		}
	}
	return merr.AsError()
}

// dumpStepLog writes one step's raw log to w, optionally preceded by banner.
// A step with no log yet (e.g. never ran) is reported to stderr, not failed.
func dumpStepLog(ctx context.Context, cmd *cobra.Command, prof *profile.Profile, repo *repository.Repository, pipelineID, stepID, banner string, w io.Writer) error {
	if len(banner) > 0 {
		fmt.Fprintf(w, "\n%s\n", banner)
	}
	reader, err := prof.GetRaw(ctx, cmd, repo.GetPath("pipelines", pipelineID, "steps", stepID, "log"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "No log available for step %s: %s\n", stepID, err)
		return nil
	}
	_, err = io.Copy(w, reader)
	return err
}
