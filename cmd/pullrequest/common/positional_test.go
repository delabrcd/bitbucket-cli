package prcommon

import (
	"testing"

	"github.com/gildas/go-flags"
	"github.com/spf13/cobra"
)

func newPullRequestCmd() (*cobra.Command, *flags.EnumFlag) {
	cmd := &cobra.Command{Use: "test"}
	id := flags.NewEnumFlag("26")
	cmd.Flags().Var(id, "pullrequest", "pullrequest")
	return cmd, id
}

func TestPullRequestArgsCountDependsOnTheFlag(t *testing.T) {
	cases := []struct {
		name    string
		own     int
		flagSet bool
		args    []string
		wantErr bool
	}{
		{"one own arg, flag set", 1, true, []string{"833"}, false},
		{"one own arg, flag set, extra arg", 1, true, []string{"26", "833"}, true},
		{"one own arg, no flag, needs two", 1, false, []string{"26", "833"}, false},
		{"one own arg, no flag, only one", 1, false, []string{"833"}, true},
		{"no own args, flag set", 0, true, nil, false},
		{"no own args, no flag, needs one", 0, false, []string{"26"}, false},
		{"no own args, no flag, none given", 0, false, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, _ := newPullRequestCmd()
			if tc.flagSet {
				if err := cmd.Flags().Set("pullrequest", "26"); err != nil {
					t.Fatalf("setting the flag: %v", err)
				}
			}
			err := PullRequestArgs(tc.own)(cmd, tc.args)
			if tc.wantErr && err == nil {
				t.Errorf("PullRequestArgs(%d)(%v) = nil, want an error", tc.own, tc.args)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("PullRequestArgs(%d)(%v) = %v, want nil", tc.own, tc.args, err)
			}
		})
	}
}

func TestPullRequestArgsMinimumCountDependsOnTheFlag(t *testing.T) {
	cmd, _ := newPullRequestCmd()
	if err := PullRequestArgsMinimum(1)(cmd, []string{"26", "833", "834"}); err != nil {
		t.Errorf("three args without the flag: %v", err)
	}
	if err := PullRequestArgsMinimum(1)(cmd, []string{"833"}); err == nil {
		t.Error("one arg without the flag = nil, want an error")
	}

	cmd, _ = newPullRequestCmd()
	if err := cmd.Flags().Set("pullrequest", "26"); err != nil {
		t.Fatalf("setting the flag: %v", err)
	}
	if err := PullRequestArgsMinimum(1)(cmd, []string{"833"}); err != nil {
		t.Errorf("one arg with the flag: %v", err)
	}
}

func TestTakePullRequestIDPrefersTheFlag(t *testing.T) {
	cmd, id := newPullRequestCmd()
	if err := cmd.Flags().Set("pullrequest", "26"); err != nil {
		t.Fatalf("setting the flag: %v", err)
	}

	rest, err := TakePullRequestID(cmd, id, []string{"833", "834"})
	if err != nil {
		t.Fatalf("TakePullRequestID: %v", err)
	}
	if id.Value != "26" {
		t.Errorf("pullrequest = %q, want 26", id.Value)
	}
	if len(rest) != 2 || rest[0] != "833" || rest[1] != "834" {
		t.Errorf("rest = %v, want [833 834]", rest)
	}
}

func TestTakePullRequestIDConsumesTheFirstPositional(t *testing.T) {
	cmd, id := newPullRequestCmd()

	rest, err := TakePullRequestID(cmd, id, []string{"26", "833"})
	if err != nil {
		t.Fatalf("TakePullRequestID: %v", err)
	}
	if id.Value != "26" {
		t.Errorf("pullrequest = %q, want 26", id.Value)
	}
	if len(rest) != 1 || rest[0] != "833" {
		t.Errorf("rest = %v, want [833]", rest)
	}
}

func TestTakePullRequestIDWithoutFlagOrArgs(t *testing.T) {
	cmd, id := newPullRequestCmd()

	if _, err := TakePullRequestID(cmd, id, nil); err == nil {
		t.Error("TakePullRequestID with neither a flag nor an argument = nil, want an error")
	}
}
