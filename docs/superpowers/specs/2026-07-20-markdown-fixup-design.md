# Markdown normalization for outgoing content (`bb`)

## Problem

Bitbucket Cloud's markdown renderer does not allow a list to interrupt a
paragraph without a blank line before it. Content like:

```
Test plan:
- [x] item one
- [x] item two
```

renders as a single `<p>Test plan:\n- [x] item one\n- [x] item two</p>` — the
bullets are absorbed into the paragraph as literal text instead of a `<ul>`.
This is common in agent-authored PR descriptions and comments (observed on
`inficon-global/cdt-haps` PR #157, where the "Test plan:" list failed to render
while an earlier list — which happened to have a blank line before it — rendered
fine).

## Goal

`bb` should normalize outgoing markdown so lists (and other blocks) render
correctly on Bitbucket, transparently, without the author needing to remember
formatting rules.

## Approach

A two-stage pipeline:

1. **Round-trip normalization.** Re-serialize the markdown through a CommonMark
   parser+renderer (`markdownfmt`, goldmark under the hood, GFM enabled). This
   normalizes presentation (unified list markers, consistent block spacing) and
   preserves content.
2. **Blank-line insertion pass.** A targeted pass that inserts a blank line
   before any list block that directly follows a non-blank, non-list line.

**Why stage 2 is required (proven empirically):** the round-trip alone does
*not* fix the bug. CommonMark treats a list that immediately follows a paragraph
(`Test plan:\n- item`) as a valid list — no blank line needed — so a faithful
renderer *preserves* exactly the tight form Bitbucket chokes on. markdownfmt was
verified to emit `Test plan:\n- [X] item` unchanged. Empirically, markdownfmt
separates every *other* block with a blank line; the paragraph→list interrupt is
the only tight case left, so stage 2 only has to handle that one pattern.

### Library choice

- `github.com/Kunde21/markdownfmt/v3` (v3.1.x) — goldmark-based markdown→markdown
  renderer, GFM enabled by default (tables, task lists, strikethrough, autolinks
  all survive). Entry point: `markdownfmt.Process("", []byte(raw))`.
- No separate direct goldmark dependency is required for the helper (markdownfmt
  vendors it); the insertion pass is plain string/regex work.

### The blank-line insertion pass

Operate line-by-line on the round-tripped output:

- Track fenced code blocks (``` ``` ``` and `~~~`); never insert inside a fence.
- A line is a **list item** if it matches `^ {0,3}([-*+]|\d{1,9}[.)])\s`
  (bullet, or ordered — this deliberately excludes `---` thematic breaks / setext
  underlines, which have no trailing space+content).
- For each list-item line at index `i > 0`, if the previous line is non-blank and
  is **not** itself a list item, insert one blank line before it.

### The helper

New package/function: `markdown.NormalizeMarkdown(raw string) string`.

- Stage 1: `markdownfmt.Process`. Stage 2: the insertion pass on the result.
- **Safety fallback:** if parsing or rendering returns an error, or produces an
  empty result while the input was non-empty, return the original `raw`
  unchanged. The function must never emit worse output than it received.
- Preserve the input's trailing-newline shape so we don't introduce spurious
  diffs (e.g. a description that had no trailing newline should not gain one).
- Empty / whitespace-only input returns unchanged.

### Accepted trade-off: whole-document cosmetic reflow

A full round-trip re-serializes the entire document, not just the broken spot.
Already-correct text will be cosmetically normalized: list markers unified to
`-`, consistent bullet/indent spacing, normalized heading and emphasis style,
reordered link reference definitions. This is accepted (confirmed with the user).
Content is preserved — GFM tables, task lists, fenced code blocks, `@mentions`,
and emoji all pass through unchanged; only presentation bytes shift.

## Integration points

The helper is applied to every outgoing markdown-bearing field, immediately
before the value is placed into the request payload:

| Command | Field |
|---|---|
| `pr create` | `PullRequestCreator.Description` (`cmd/pullrequest/create.go`) |
| `pr update` | `pullrequest.Description` + `pullrequest.Summary.Raw` (`cmd/pullrequest/update.go`) |
| `pr comment create` | `ContentCreator.Raw` (`cmd/pullrequest/comment/create.go`) |
| `pr comment update` | comment `Raw` (`cmd/pullrequest/comment/update.go`) |
| `issue create` | `RenderedText.Raw` (`cmd/issue/create.go`) |
| `issue update` | `RenderedText.Raw` (`cmd/issue/update.go`) |
| `issue comment create` | `RenderedText.Raw` (`cmd/issue/comment/create.go`) |
| `issue comment update` | `RenderedText.Raw` (`cmd/issue/comment/update.go`) |

Titles are **not** normalized (single-line, no block markdown).

## Opt-out

A persistent root flag turns the fixup off for a single invocation:

- `--no-markdown-fixup` (bool, default `false`), registered on `RootCmd`
  alongside the existing persistent flags in `cmd/root.go` (mirrors the
  `--dry-run` pattern). Default behavior is fixup **on**.
- Call sites skip `NormalizeMarkdown` when the flag is set. The flag is read the
  same way other persistent options are surfaced to subcommands (via the shared
  `CmdOptions` / `cmd.Flags().GetBool`).

Config-key support (persisting the opt-out globally) is out of scope for this
change; the flag is sufficient. Revisit only if requested.

## Testing

Unit tests on `NormalizeMarkdown` (table-driven), covering:

1. The reported bug: paragraph immediately followed by a `- [x]` task list gains
   a blank line and renders as a list.
2. Already-correct list (blank line present) stays a valid list.
3. Ordered list that starts at `1` interrupting a paragraph.
4. Fenced code block containing `-` bullets is left untouched (no spurious
   blank-line insertion inside the fence).
5. GFM table round-trips without corruption.
6. `@mention` and emoji shortcodes survive.
7. Empty and whitespace-only input returned unchanged.
8. Malformed input that trips the renderer falls back to the original string.
9. Trailing-newline shape preserved.

No live API calls in tests — the helper is pure string→string.

## Out of scope

- Config-file key for the opt-out (flag only for now).
- Normalizing titles or any non-markdown field.
- Making the round-trip optional/separable from the insertion pass. The full
  round-trip (cosmetic reflow) was chosen deliberately and runs before the
  insertion pass that actually fixes rendering.
