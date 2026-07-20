package common

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNormalizeMarkdown(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		contains  []string // substrings that must appear in output
		equal     string   // exact expected output, used when wantEqual is true
		wantEqual bool
	}{
		{
			name:     "list interrupting paragraph gains blank line",
			in:       "Test plan:\n- [x] one\n- [x] two\n",
			contains: []string{"Test plan:\n\n- ", "one", "two"},
		},
		{
			name:     "already correct list stays a list",
			in:       "Intro.\n\n- a\n- b\n",
			contains: []string{"Intro.\n\n- a\n- b"},
		},
		{
			name:     "ordered list interrupting paragraph gains blank line",
			in:       "Steps:\n1. first\n2. second\n",
			contains: []string{"Steps:\n\n1. first"},
		},
		{
			name:     "bullets inside code fence are untouched",
			in:       "```\nintro\n- not a bullet\n```\n",
			contains: []string{"intro\n- not a bullet"},
		},
		{
			name:     "gfm table survives",
			in:       "text\n\n| A | B |\n|---|---|\n| 1 | 2 |\n",
			contains: []string{"| A | B |", "| 1 | 2 |"},
		},
		{
			name:     "mentions and emoji survive",
			in:       "cc @someone :tada:\n",
			contains: []string{"@someone", ":tada:"},
		},
		{
			name:      "empty input unchanged",
			in:        "",
			equal:     "",
			wantEqual: true,
		},
		{
			name:      "whitespace-only input unchanged",
			in:        "   \n\t\n",
			equal:     "   \n\t\n",
			wantEqual: true,
		},
		{
			name:      "no trailing newline is not added",
			in:        "just a line",
			equal:     "just a line",
			wantEqual: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeMarkdown(tc.in)
			if tc.wantEqual {
				if got != tc.equal {
					t.Fatalf("got %q, want %q", got, tc.equal)
				}
				return
			}
			for _, sub := range tc.contains {
				if !strings.Contains(got, sub) {
					t.Fatalf("output missing %q\n--- got ---\n%s", sub, got)
				}
			}
		})
	}
}

func TestMaybeFixupMarkdown(t *testing.T) {
	in := "Test plan:\n- [x] one\n"

	on := &cobra.Command{}
	on.Flags().Bool("no-markdown-fixup", false, "")
	if got := MaybeFixupMarkdown(on, in); !strings.Contains(got, "Test plan:\n\n- ") {
		t.Fatalf("fixup should run when flag is false; got %q", got)
	}

	off := &cobra.Command{}
	off.Flags().Bool("no-markdown-fixup", true, "")
	if got := MaybeFixupMarkdown(off, in); got != in {
		t.Fatalf("fixup should be skipped when flag is true; got %q", got)
	}

	none := &cobra.Command{} // flag not registered -> default to enabled
	if got := MaybeFixupMarkdown(none, in); !strings.Contains(got, "Test plan:\n\n- ") {
		t.Fatalf("fixup should run when flag absent; got %q", got)
	}
}

func TestInsertBlankLinesBeforeLists_LongFenceNotClosedEarly(t *testing.T) {
	in := "Intro.\n````\ninner\n```\n- bullet\n````\nafter\n"
	got := insertBlankLinesBeforeLists(in)
	if strings.Contains(got, "```\n\n- bullet") {
		t.Fatalf("blank line inserted inside fenced code block\n--- got ---\n%s", got)
	}
	if got != in {
		t.Fatalf("expected input unchanged since the bullet is inside the fence\ngot:  %q\nwant: %q", got, in)
	}
}
