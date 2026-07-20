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

Round-trip the markdown through a CommonMark parser and re-serialize it. goldmark
parses `Test plan:` and the following list as separate blocks (CommonMark allows
bullet lists to interrupt paragraphs), and the markdown renderer emits the
required blank line between them. This fixes the reported bug as a side effect of
faithful re-serialization, and handles the general class of "missing blank line
before a block" problems rather than one special case.

### Library choice

- Parser: `github.com/yuin/goldmark` (v1.8.x) with the GFM extension enabled
  (tables, task lists, strikethrough, autolinks) so those constructs survive.
- Renderer: `github.com/Kunde21/markdownfmt/v3` (v3.1.x) — a goldmark-based
  markdown→markdown renderer.

### The helper

New package/function: `markdown.NormalizeMarkdown(raw string) string`.

- Parse `raw` with goldmark (GFM), render back to markdown with markdownfmt.
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
- A "narrow, insert-only-missing-blank-lines" renderer variant (the full
  round-trip with cosmetic reflow was chosen deliberately).
