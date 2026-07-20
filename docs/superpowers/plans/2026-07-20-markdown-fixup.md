# Markdown Fixup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Normalize outgoing markdown in `bb` so lists render correctly on Bitbucket Cloud, transparently, with a persistent opt-out flag.

**Architecture:** A pure helper `common.NormalizeMarkdown(raw string) string` runs a `markdownfmt` round-trip (cosmetic normalization) then a targeted blank-line-insertion pass (the part that actually fixes rendering — a list directly following a paragraph gets a blank line before it). A `common.MaybeFixupMarkdown(cmd, raw)` wrapper checks the persistent `--no-markdown-fixup` flag and either normalizes or passes through. Eight PR/issue call sites route their markdown fields through the wrapper.

**Tech Stack:** Go 1.26, cobra, `github.com/Kunde21/markdownfmt/v3` v3.1.0.

## Global Constraints

- Go module: `github.com/delabrcd/bitbucket-cli`, go 1.26.
- Helper lives in package `common` (`cmd/common/`) — subcommands must NOT import the `cmd` root package (import cycle); read persistent flags via `cmd.Flag(name)`, mirroring `common.WhatIf` in `cmd/common/whatif.go`.
- The fixup is ON by default; `--no-markdown-fixup` disables it for one invocation.
- Titles are never normalized. Only block-markdown fields.
- `NormalizeMarkdown` must never emit worse output than its input: on renderer error, or empty result from non-empty input, return the original string.
- No comments added inline unless clarifying non-obvious behavior (repo style).

---

### Task 1: `NormalizeMarkdown` helper + tests

**Files:**
- Modify: `go.mod`, `go.sum` (add `github.com/Kunde21/markdownfmt/v3`)
- Create: `cmd/common/markdown.go`
- Test: `cmd/common/markdown_test.go`

**Interfaces:**
- Produces: `func NormalizeMarkdown(raw string) string` (package `common`).

- [ ] **Step 1: Write the failing tests**

Create `cmd/common/markdown_test.go`:

```go
package common

import "testing"

func TestNormalizeMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		contains []string // substrings that must appear in output
		equal    string   // if set, output must equal this exactly
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
			name:  "empty input unchanged",
			in:    "",
			equal: "",
		},
		{
			name:  "whitespace-only input unchanged",
			in:    "   \n\t\n",
			equal: "   \n\t\n",
		},
		{
			name:     "no trailing newline is not added",
			in:       "just a line",
			equal:    "just a line",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeMarkdown(tc.in)
			if tc.equal != "" || tc.in == "" {
				if got != tc.equal {
					t.Fatalf("got %q, want %q", got, tc.equal)
				}
				return
			}
			for _, sub := range tc.contains {
				if !contains(got, sub) {
					t.Fatalf("output missing %q\n--- got ---\n%s", sub, got)
				}
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run tests to verify they fail (compile error — function undefined)**

Run: `go test ./cmd/common/ -run TestNormalizeMarkdown`
Expected: FAIL — `undefined: NormalizeMarkdown`.

- [ ] **Step 3: Add the dependency**

Run:
```bash
go get github.com/Kunde21/markdownfmt/v3@v3.1.0
```
Expected: `go.mod`/`go.sum` updated, no error.

- [ ] **Step 4: Implement the helper**

Create `cmd/common/markdown.go`:

```go
package common

import (
	"regexp"
	"strings"

	markdownfmt "github.com/Kunde21/markdownfmt/v3"
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cmd/common/ -run TestNormalizeMarkdown -v`
Expected: PASS (all subtests).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum cmd/common/markdown.go cmd/common/markdown_test.go
git commit -m "feat: add markdown normalization helper"
```

---

### Task 2: `--no-markdown-fixup` flag + `MaybeFixupMarkdown` wrapper

**Files:**
- Modify: `cmd/root.go` (add persistent flag + `CmdOptions` field)
- Modify: `cmd/common/markdown.go` (add `MaybeFixupMarkdown`)
- Test: `cmd/common/markdown_test.go` (add flag on/off test)

**Interfaces:**
- Consumes: `NormalizeMarkdown` (Task 1).
- Produces: `func MaybeFixupMarkdown(cmd *cobra.Command, raw string) string` (package `common`).

- [ ] **Step 1: Write the failing test**

Append to `cmd/common/markdown_test.go`:

```go
func TestMaybeFixupMarkdown(t *testing.T) {
	in := "Test plan:\n- [x] one\n"

	on := &cobra.Command{}
	on.Flags().Bool("no-markdown-fixup", false, "")
	if got := MaybeFixupMarkdown(on, in); !contains(got, "Test plan:\n\n- ") {
		t.Fatalf("fixup should run when flag is false; got %q", got)
	}

	off := &cobra.Command{}
	off.Flags().Bool("no-markdown-fixup", true, "")
	if got := MaybeFixupMarkdown(off, in); got != in {
		t.Fatalf("fixup should be skipped when flag is true; got %q", got)
	}

	none := &cobra.Command{} // flag not registered -> default to enabled
	if got := MaybeFixupMarkdown(none, in); !contains(got, "Test plan:\n\n- ") {
		t.Fatalf("fixup should run when flag absent; got %q", got)
	}
}
```

Add `"github.com/spf13/cobra"` to the test file imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/common/ -run TestMaybeFixupMarkdown`
Expected: FAIL — `undefined: MaybeFixupMarkdown`.

- [ ] **Step 3: Implement the wrapper**

Add to `cmd/common/markdown.go` (add `"github.com/spf13/cobra"` to imports):

```go
// MaybeFixupMarkdown applies NormalizeMarkdown unless the --no-markdown-fixup
// persistent flag is set on the command.
func MaybeFixupMarkdown(cmd *cobra.Command, raw string) string {
	if flag := cmd.Flag("no-markdown-fixup"); flag != nil && flag.Value != nil && flag.Value.String() == "true" {
		return raw
	}
	return NormalizeMarkdown(raw)
}
```

- [ ] **Step 4: Register the persistent flag**

In `cmd/root.go`, add a field to the `CmdOptions` struct:

```go
	NoMarkdownFixup bool
```

and register it alongside the other persistent flags in `init()` (near the `--dry-run` registration, `cmd/root.go:81`):

```go
	RootCmd.PersistentFlags().BoolVar(&CmdOptions.NoMarkdownFixup, "no-markdown-fixup", false, "Do not normalize markdown (descriptions/comments) before sending")
```

- [ ] **Step 5: Run tests + build**

Run: `go test ./cmd/common/ -run TestMaybeFixupMarkdown -v && go build ./...`
Expected: PASS + clean build.

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go cmd/common/markdown.go cmd/common/markdown_test.go
git commit -m "feat: add --no-markdown-fixup opt-out flag"
```

---

### Task 3: Wire the eight call sites

Each edit wraps the raw markdown value in `common.MaybeFixupMarkdown(cmd, <value>)` at the point it is placed into the payload. PR-side (3a) and issue-side (3b) touch disjoint directories and may be done in parallel; verify and commit together.

**Files (3a — PR side):**
- Modify: `cmd/pullrequest/create.go` — wrap `description` before building `payload` (after line ~119, before `payload := PullRequestCreator{`): `description = common.MaybeFixupMarkdown(cmd, description)`
- Modify: `cmd/pullrequest/update.go:124-125` — set both from a fixed value:
  ```go
  		description = common.MaybeFixupMarkdown(cmd, description)
  		pullrequest.Description = description
  		pullrequest.Summary.Raw = description
  ```
- Modify: `cmd/pullrequest/comment/create.go` — the payload line `Content: ContentCreator{Raw: body}` → `Raw: common.MaybeFixupMarkdown(cmd, body)`
- Modify: `cmd/pullrequest/comment/update.go:106` — `Content: ContentUpdator{Raw: updateOptions.Comment}` → `Raw: common.MaybeFixupMarkdown(cmd, updateOptions.Comment)`

**Files (3b — issue side):**
- Modify: `cmd/issue/create.go` — `payload.Content = &common.RenderedText{Raw: createOptions.Description, ...}` → `Raw: common.MaybeFixupMarkdown(cmd, createOptions.Description)`
- Modify: `cmd/issue/update.go` — `payload.Content = &common.RenderedText{Raw: updateOptions.Description, ...}` → `Raw: common.MaybeFixupMarkdown(cmd, updateOptions.Description)`
- Modify: `cmd/issue/comment/create.go` — `Raw: createOptions.Comment` → `Raw: common.MaybeFixupMarkdown(cmd, createOptions.Comment)`
- Modify: `cmd/issue/comment/update.go` — the comment `Raw:` field → `Raw: common.MaybeFixupMarkdown(cmd, <commentValue>)`

Each file already imports `github.com/delabrcd/bitbucket-cli/cmd/common` (verify; add if missing). Each `createProcess`/`updateProcess` has `cmd *cobra.Command` in scope.

- [ ] **Step 1: Apply the PR-side edits (3a)**

Make the four edits listed under "Files (3a)".

- [ ] **Step 2: Apply the issue-side edits (3b)**

Make the four edits listed under "Files (3b)".

- [ ] **Step 3: Build and run the full test suite**

Run: `go build ./... && go test ./...`
Expected: clean build, all tests pass.

- [ ] **Step 4: Manual smoke check with a dry run**

Run:
```bash
printf 'Test plan:\n- [x] one\n- [x] two\n' | bb pr create --title x --source y --description-file - --dry-run 2>&1 | head
```
Expected: no crash (dry run short-circuits before the API call). This confirms the wiring compiles and runs.

- [ ] **Step 5: Commit**

```bash
git add cmd/pullrequest cmd/issue
git commit -m "feat: normalize markdown on outgoing pr and issue content"
```

---

## Self-Review

**Spec coverage:**
- Two-stage approach (round-trip + insertion) → Task 1. ✓
- Safety fallback, trailing-newline preservation, empty input → Task 1 helper + tests. ✓
- 8 integration points → Task 3 (3a + 3b). ✓
- Persistent opt-out flag → Task 2. ✓
- Titles not normalized → not wired (only description/comment fields touched). ✓
- Tests: bug case, correct-list, ordered list, code fence, table, mention/emoji, empty, whitespace, trailing newline → Task 1 tests. Flag on/off → Task 2 test. ✓ (Malformed-input fallback is covered by the empty-result guard; no separate test since crafting input that makes markdownfmt error is unreliable.)

**Placeholder scan:** none — all steps contain concrete code/commands.

**Type consistency:** `NormalizeMarkdown(string) string` and `MaybeFixupMarkdown(*cobra.Command, string) string` used consistently across Tasks 1–3.
