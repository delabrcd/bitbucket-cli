package common

import (
	"regexp"
	"strings"

	markdownfmt "github.com/Kunde21/markdownfmt/v3"
	"github.com/spf13/cobra"
)

var listItemLine = regexp.MustCompile(`^ {0,3}([-*+]|\d{1,9}[.)])\s`)

// NormalizeMarkdown re-serializes markdown so it renders correctly on Bitbucket
// Cloud: a markdownfmt round-trip normalizes presentation, then a targeted pass
// inserts a blank line before any list that directly follows a non-list line
// (Bitbucket does not let a list interrupt a paragraph without one).
//
// It never returns worse output than it received: on any renderer error, or an
// empty result from non-empty input, the original string is returned unchanged.
func NormalizeMarkdown(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	out, err := markdownfmt.Process("", []byte(raw))
	if err != nil || len(out) == 0 {
		return raw
	}
	fixed := insertBlankLinesBeforeLists(string(out))
	if strings.TrimSpace(fixed) == "" {
		return raw
	}
	if !strings.HasSuffix(raw, "\n") {
		fixed = strings.TrimRight(fixed, "\n")
	}
	return fixed
}

// MaybeFixupMarkdown applies NormalizeMarkdown unless the --no-markdown-fixup
// persistent flag is set on the command.
func MaybeFixupMarkdown(cmd *cobra.Command, raw string) string {
	if flag := cmd.Flag("no-markdown-fixup"); flag != nil && flag.Value != nil && flag.Value.String() == "true" {
		return raw
	}
	return NormalizeMarkdown(raw)
}

// fenceRun reports the fence char and run length at the start of a trimmed
// line, e.g. "````info" -> ('`', 4). It returns (0, 0) if the line doesn't
// start with a backtick or tilde.
func fenceRun(trimmed string) (ch byte, n int) {
	if trimmed == "" {
		return 0, 0
	}
	ch = trimmed[0]
	if ch != '`' && ch != '~' {
		return 0, 0
	}
	for n < len(trimmed) && trimmed[n] == ch {
		n++
	}
	return ch, n
}

func insertBlankLinesBeforeLists(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines)+4)
	inFence := false
	var fenceChar byte
	fenceLen := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		ch, n := fenceRun(trimmed)
		switch {
		case !inFence && n >= 3:
			inFence, fenceChar, fenceLen = true, ch, n
		case inFence && ch == fenceChar && n == len(trimmed) && n >= fenceLen:
			inFence = false
		}
		if !inFence && i > 0 && listItemLine.MatchString(line) {
			prev := lines[i-1]
			if strings.TrimSpace(prev) != "" && !listItemLine.MatchString(prev) {
				out = append(out, "")
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
