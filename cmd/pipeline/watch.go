package pipeline

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	plcommon "github.com/delabrcd/bitbucket-cli/cmd/pipeline/common"
	"github.com/delabrcd/bitbucket-cli/cmd/pipeline/step"
	"github.com/delabrcd/bitbucket-cli/cmd/profile"
	"github.com/delabrcd/bitbucket-cli/cmd/repository"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

const minWatchInterval = 2 * time.Second

var watchCmd = &cobra.Command{
	Use:   "watch [flags] [pipeline-build-number-or-uuid]",
	Short: "watch a pipeline until it completes, refreshing its step states",
	Long: `watch a pipeline until it completes, refreshing its step states on an interval.

With no build number the latest pipeline is watched. The view redraws in place
on a terminal; when piped, each refresh is appended. Use --exit-status to make
the command exit non-zero when the pipeline does not succeed.`,
	Args:              cobra.RangeArgs(0, 1),
	ValidArgsFunction: logsValidArgs,
	RunE:              watchProcess,
}

var watchOptions struct {
	Interval   time.Duration
	ExitStatus bool
}

func init() {
	Command.AddCommand(watchCmd)

	watchCmd.Flags().DurationVar(&watchOptions.Interval, "interval", 5*time.Second, "Refresh interval (minimum 2s)")
	watchCmd.Flags().BoolVar(&watchOptions.ExitStatus, "exit-status", false, "Exit with a non-zero status if the pipeline did not succeed")
}

func watchProcess(cmd *cobra.Command, args []string) error {
	log := logger.Must(logger.FromContext(cmd.Context())).Child(cmd.Parent().Name(), "watch")

	prof, err := profile.GetProfileFromCommand(cmd.Context(), cmd)
	if err != nil {
		return err
	}
	repo, err := repository.GetRepository(cmd.Context(), cmd)
	if err != nil {
		return err
	}

	var pipelineID string
	if len(args) == 1 {
		pipelineID = args[0]
	} else {
		latest, err := plcommon.GetLatestPipelineID(log.ToContext(cmd.Context()), cmd)
		if err != nil {
			return errors.Join(errors.New("failed to resolve the latest pipeline"), err)
		}
		pipelineID = latest
		fmt.Fprintf(os.Stderr, "Watching latest pipeline #%s\n", pipelineID)
	}

	interval := watchOptions.Interval
	if interval < minWatchInterval {
		interval = minWatchInterval
	}

	out := cmd.OutOrStdout()
	tty := false
	if f, ok := out.(*os.File); ok {
		tty = isatty.IsTerminal(f.Fd())
	}

	prevLines := 0
	for {
		var pl Pipeline
		if err := prof.Get(log.ToContext(cmd.Context()), cmd, repo.GetPath("pipelines", pipelineID), &pl); err != nil {
			return errors.Join(errors.Errorf("failed to get pipeline %s", pipelineID), err)
		}
		steps, err := profile.GetAll[step.Step](log.ToContext(cmd.Context()), cmd, repo.GetPath("pipelines", pipelineID, "steps"))
		if err != nil {
			return errors.Join(errors.Errorf("failed to get steps for pipeline %s", pipelineID), err)
		}

		lines := renderWatch(pl, steps, interval, tty)
		if tty && prevLines > 0 {
			fmt.Fprintf(out, "\033[%dA\033[J", prevLines)
		}
		for _, line := range lines {
			fmt.Fprintln(out, line)
		}
		prevLines = len(lines)

		if isPipelineDone(pl) {
			return finishWatch(out, pl)
		}
		time.Sleep(interval)
	}
}

// isPipelineDone reports whether the pipeline has reached a terminal state.
func isPipelineDone(pl Pipeline) bool {
	return pl.State.Result != nil || strings.EqualFold(pl.State.Name, "COMPLETED")
}

// finishWatch prints the closing summary and applies --exit-status.
func finishWatch(out io.Writer, pl Pipeline) error {
	result := ""
	if pl.State.Result != nil {
		result = strings.ToUpper(pl.State.Result.Name)
	}
	if result != "SUCCESSFUL" && result != "" {
		fmt.Fprintf(out, "\nPipeline #%d finished: %s. See logs with: bb pipeline logs %d --failed\n", pl.BuildNumber, pl.State.String(), pl.BuildNumber)
	} else {
		fmt.Fprintf(out, "\nPipeline #%d finished: %s\n", pl.BuildNumber, pl.State.String())
	}
	if watchOptions.ExitStatus && result != "SUCCESSFUL" {
		os.Exit(1)
	}
	return nil
}

func renderWatch(pl Pipeline, steps []step.Step, interval time.Duration, tty bool) []string {
	pipelineResult := ""
	if pl.State.Result != nil {
		pipelineResult = pl.State.Result.Name
	}
	lines := []string{
		colorize(fmt.Sprintf("Pipeline #%d  %s", pl.BuildNumber, pl.State.String()), stateColor(pl.State.Name, pipelineResult), tty),
		fmt.Sprintf("Refreshed %s  ·  every %s  ·  Ctrl-C to stop", time.Now().Format("15:04:05"), interval),
		"",
	}

	nameWidth := len("STEP")
	for _, s := range steps {
		if len(s.Name) > nameWidth {
			nameWidth = len(s.Name)
		}
	}
	if nameWidth > 40 {
		nameWidth = 40
	}

	lines = append(lines, fmt.Sprintf("  %-*s  %-22s  %s", nameWidth, "STEP", "STATE", "DURATION"))
	for _, s := range steps {
		stepResult := ""
		if s.State.Result != nil {
			stepResult = s.State.Result.Name
		}
		stateField := colorize(fmt.Sprintf("%-22s", s.State.String()), stateColor(s.State.Name, stepResult), tty)
		lines = append(lines, fmt.Sprintf("  %-*s  %s  %s", nameWidth, truncate(s.Name, nameWidth), stateField, stepDuration(s)))
	}
	return lines
}

// stepDuration shows a finished step's duration, a running step's elapsed time,
// or "-" for a step that has not started.
func stepDuration(s step.Step) string {
	if s.Duration > 0 {
		return s.Duration.Round(time.Second).String()
	}
	if strings.EqualFold(s.State.Name, "IN_PROGRESS") && !s.StartedOn.IsZero() {
		return time.Since(s.StartedOn).Round(time.Second).String()
	}
	return "-"
}

func truncate(value string, width int) string {
	if len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	return value[:width-1] + "…"
}

// stateColor returns the ANSI color for a state/result, or "" for the default.
func stateColor(name, result string) string {
	switch strings.ToUpper(result) {
	case "SUCCESSFUL":
		return "\033[32m" // green
	case "FAILED", "ERROR":
		return "\033[31m" // red
	case "STOPPED":
		return "\033[33m" // yellow
	}
	if strings.EqualFold(name, "IN_PROGRESS") {
		return "\033[36m" // cyan
	}
	return ""
}

func colorize(value, color string, tty bool) string {
	if !tty || len(color) == 0 {
		return value
	}
	return color + value + "\033[0m"
}
