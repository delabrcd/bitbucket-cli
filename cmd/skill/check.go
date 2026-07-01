package skill

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gildas/go-logger"
)

// notifyThrottle is the minimum interval between "notify" mode stderr notices.
const notifyThrottle = 24 * time.Hour

// MaybeCheck runs the configured skill update-mode behavior at startup. It must
// NEVER fail the command or write to stdout — only stderr, and only for the
// notify/auto modes. version is cmd.Root().Version.
func MaybeCheck(ctx context.Context, version string) {
	defer func() {
		_ = recover() // never let a panic here escape to the caller
	}()

	debugf := func(format string, args ...any) {
		if log, err := logger.FromContext(ctx); err == nil && log != nil {
			log.Debugf(format, args...)
		}
	}

	if shouldSkipCheck() {
		return
	}

	if len(os.Getenv("BB_NO_SKILL_CHECK")) > 0 {
		return
	}

	state, err := LoadState()
	if err != nil {
		debugf("skill: failed to load registry: %s", err)
		return
	}

	if state.UpdateMode == ModeOff {
		return
	}
	if len(state.Installations) == 0 {
		return
	}

	type drifted struct {
		index int
		path  string
	}
	var driftedInstalls []drifted
	for i, inst := range state.Installations {
		isDrifted, exists, err := IsDrifted(inst.Path)
		if err != nil {
			debugf("skill: failed to check %s: %s", inst.Path, err)
			continue
		}
		if exists && isDrifted {
			driftedInstalls = append(driftedInstalls, drifted{index: i, path: inst.Path})
		}
	}
	if len(driftedInstalls) == 0 {
		return
	}

	switch state.UpdateMode {
	case ModeAuto:
		data, err := assets.ReadFile(skillAsset)
		if err != nil {
			debugf("skill: failed to read embedded skill: %s", err)
			return
		}

		updated := 0
		for _, d := range driftedInstalls {
			if err := os.WriteFile(d.path, data, 0644); err != nil {
				debugf("skill: failed to update %s: %s", d.path, err)
				continue
			}
			state.Installations[d.index].Version = version
			updated++
		}

		if updated > 0 {
			if err := state.Save(); err != nil {
				debugf("skill: failed to save registry: %s", err)
			}
			fmt.Fprintf(os.Stderr, "bb: auto-updated %d out-of-date skill(s).\n", updated)
		}

	case ModeNotify:
		if !state.LastNotified.IsZero() && time.Since(state.LastNotified) < notifyThrottle {
			return
		}

		fmt.Fprintf(os.Stderr, "bb: %d installed skill(s) are out of date:\n", len(driftedInstalls))
		for _, d := range driftedInstalls {
			fmt.Fprintf(os.Stderr, "  %s\n", d.path)
		}
		fmt.Fprintln(os.Stderr, "Run \"bb skill update\" to refresh them, or \"bb skill mode auto\"/\"off\" to change this behavior.")

		state.LastNotified = time.Now()
		if err := state.Save(); err != nil {
			debugf("skill: failed to save registry: %s", err)
		}
	}
}

// shouldSkipCheck inspects os.Args for signs that this invocation is shell
// completion or skill-management itself, in which case the update check
// should not run.
func shouldSkipCheck() bool {
	for _, arg := range os.Args {
		switch {
		case strings.HasPrefix(arg, "__complete"):
			return true
		case arg == "completion":
			return true
		case arg == "skill":
			return true
		}
	}
	return false
}
