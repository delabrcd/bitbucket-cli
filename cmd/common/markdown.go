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

func insertBlankLinesBeforeLists(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines)+4)
	inFence := false
	fence := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inFence && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")):
			inFence, fence = true, trimmed[:3]
		case inFence && strings.HasPrefix(trimmed, fence):
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
