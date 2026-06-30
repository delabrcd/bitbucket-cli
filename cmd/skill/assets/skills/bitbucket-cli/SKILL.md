---
name: bitbucket-cli
description: Use whenever doing Bitbucket Cloud work via the `bb` CLI (gildas/bitbucket-cli) — pull requests, comments/reviews, repos, branches, commits, pipelines, runners, merges. Covers the command grammar, the `bb api` raw-REST escape hatch, the PR-review (pending/inline) workflow, the merge-gate verify-first rule, and @-mention/default-reviewer gotchas.
---

# Bitbucket `bb` CLI reference

`bb` (gildas/bitbucket-cli) is the daily driver for Bitbucket Cloud work: clean `bb <resource> <action>` grammar, `-o json` for scripting, and a `bb api` raw-REST escape hatch for anything the typed subcommands don't cover.

Upstream: https://github.com/gildas/bitbucket-cli · REST docs: https://developer.atlassian.com/cloud/bitbucket/rest/

## Conventions

- **Grammar:** `bb <resource> <action> [args] [flags]`, e.g. `bb pullrequest list`, `bb pipeline step logs`. `pr` is an alias for `pullrequest`.
- **Auth/workspace:** profile and workspace come from your `bb` config (set via `bb config` or `~/.config/bb/config.yaml`), the cwd's git remote, or explicit `--profile` / `--workspace` flags.
- **Repository:** auto-detected from the cwd's git remote; otherwise pass `--repository <slug>` (case-sensitive — use the URL slug, not the display name).
- **Output:** pass `-o json` whenever parsing programmatically (`-o yaml`/`-o table` also available). Pipe to `jq` for filtering.
- **Mutations** (`create`, `update`, `merge`, `decline`, `approve`, `delete`, comment posts, pipeline `trigger`/`stop`) are remote-mutating — confirm intent before invoking. Use `--dry-run` to preview without changing anything.
- `bb repo list` defaults to `--role owner` and returns **empty** for most workspace members — pass `--role member` to see all workspace repos (documented Bitbucket API default, not a bug).

## `bb api` — raw REST escape hatch

For anything the typed subcommands don't expose, hit the REST API directly:

```
bb api <endpoint> [-X METHOD] [-f key=val] [-F key=val] [--input body.json] [--paginate] [-i]
```

- The endpoint is joined to the API root with the **`/2.0` prefix added automatically** — do NOT include `/2.0`. A full `https://…` URL is used verbatim (handy for following `next` links).
- Method defaults to GET, or POST when any field/body is given.
- `-f key=val` adds a **string** field; `-F key=val` types values (`true`/`false`/`null`/numbers typed; `@file` reads a file; `@-` reads stdin). Nested via dotted keys: `-f source.branch.name=feature/x`.
- `--input body.json` (or `-` for stdin) sends a raw JSON body; `--content-type` overrides the default `application/json`.
- `--paginate` follows `next` links and merges every page's `values` into one result.
- `-i` includes the response status line + headers.

Common endpoints worth knowing (all relative, no `/2.0`):

- `repositories/{workspace}/{repo}/commit/{sha}/statuses` — commit build statuses (the merge gate; no typed subcommand for this)
- `repositories/{workspace}/{repo}/branch-restrictions` — merge-check config
- `repositories/{workspace}/{repo}/default-reviewers` — repo default reviewers
- `repositories/{workspace}/{repo}/pullrequests/{id}/activity` — activity feed (used to verify pending comment state)

## Common commands

**Repos / branches / commits:**
- `bb repo list --role member -o json` — workspace repos (`--role` values: `all|owner|admin|contributor|member`; default `owner` returns empty for most users)
- `bb repo get <slug>` — repo details (slug is positional)
- `bb branch list` — **only `list` exists** under `bb branch`. To get/delete a branch use `bb api`: `bb api repositories/{workspace}/{repo}/refs/branches/<name>` (GET) or `bb api -X DELETE repositories/{workspace}/{repo}/refs/branches/<name>`.
- `bb commit list` · `bb commit get <sha>` · `bb commit diff <sha>`

**Pull requests:**
- `bb pr list --repository <slug> -o json` (server-side filter with `--state`; default `open`, also `merged`/`declined`/`superseded`)
- `bb pr get <id> --repository <slug>` · `bb pr diff <id>` · `bb pr commits <id>` · `bb pr activities <id>`
- `bb pr create --title "…" --source <branch> --destination <branch> --reviewer default --repository <slug>`
  - `--reviewer default` as the **first** reviewer auto-resolves the repo/project default reviewers. Add more with repeated `--reviewer <id|uuid|name|nickname>`.
  - `--fill` fills title+description from the commit messages between destination and source.
  - `--description "…"` / `--description-file <f|->` · `--close-source-branch`
- `bb pr update <id> --title "…"` · `bb pr approve <id>` / `bb pr unapprove <id>` · `bb pr request-changes <id>` / `bb pr remove-request-changes <id>` · `bb pr decline <id>`

**Pipelines:**
- `bb pipeline list` · `bb pipeline get <build#-or-uuid>` (accepts build number directly) · `bb pipeline trigger` · `bb pipeline stop <uuid>`
- `bb pipeline step list --pipeline <build#>` · **step logs:** `bb pipeline step logs --pipeline <build#> [<step-uuid-or-name>] [--step <uuid-or-name>|--failed|--all]` — positional step is optional; `--step`/`--failed`/`--all` are mutually exclusive.
- `bb pipeline logs [<build#>] [--step <name>|--failed|--all]` — quick step-log access; no build# = latest pipeline (prints `Using latest pipeline #N` to stderr), no selector = failed step(s) or the sole step. Alias: `log`.
- `bb pipeline watch [<build#>] [--interval <dur>] [--exit-status]` — follow a pipeline until it completes (analogous to `gh run watch`); no build# = latest; redraws in place on a TTY, appended when piped; `--exit-status` exits non-zero on a non-SUCCESSFUL result.

**Runners** (self-hosted Pipelines runners):
- `bb runner list` (alias `ls`) · `bb runner get <uuid>` (`show`/`info`/`display`) · `bb runner create --name <n> [--label …]` · `bb runner delete <uuid…>` (`remove`/`rm`)
- **Scope:** repository by default; pass `-W` / `--workspace-level` for the workspace's shared runners. Workspace-level requires workspace-admin on the token owner's account.
- `get`/`delete` accept the UUID **with or without** `{braces}` (auto-wrapped).
- `create`: `--label` is repeatable; include one OS label (`linux`/`linux.arm64`/`linux.shell`/`windows`/`macos`), `self.hosted` is auto-added. The response's `oauth_client.secret` is printed **only once** — capture it immediately with `-o json` (never retrievable again). The agent version appears under `state.version.current`; `state.cordoned` flags a paused runner.

**Skill and shell completions:**
- `bb skill install` — install this skill into a Claude skills directory. Auto-detects project `.claude/skills` and personal `~/.claude/skills`; `--dir` to specify a target, `--force` to overwrite.
- `bb completion install` — install shell completions (bash/zsh/fish). Auto-detects shell and install directory; `--shell` and `--dir` to override.

## Merging a PR — the API does NOT enforce merge checks for admins

`bb pr merge <id>` (→ `POST .../merge`) evaluates branch merge checks (e.g. `require_passing_builds_to_merge` = "≥1 successful, no failed, no in-progress builds") against the **token owner's permissions**. For a non-admin a failing check refuses the merge. But if the token owner has **repo admin**, their override is applied **automatically and silently** — there is no "Merge anyway" confirmation like the web UI. An admin token will push a PR with a failed or in-progress pipeline straight onto the target branch, no questions asked.

**Before any `bb pr merge`, verify the build state yourself — the API won't:**

1. Get the PR head commit: `bb pr get <id> -o json | jq -r '.source.commit.hash'`
2. Check every status on it: `bb api repositories/{workspace}/{repo}/commit/<sha>/statuses | jq '.values[] | {key, state}'`
3. Merge only if **all** are `SUCCESSFUL` (no `FAILED`, no `INPROGRESS`). Multiple producers can report here (the pipeline build AND external checks such as code-quality tools) — all must be green.

Merge-check config lives at `…/branch-restrictions` (kinds: `require_passing_builds_to_merge`, `require_no_changes_requested`, `require_tasks_to_be_completed`). Bitbucket Cloud has no per-check "disallow admin override" toggle, so this verify-first habit is the only guard.

Strategy/flags: `--merge-strategy merge_commit|squash|fast_forward`, `--message`, `--close-source-branch`.

## PR review workflow — batched, inline-first (preferred)

When leaving review feedback, follow this workflow (actionable comments + **one** notification instead of a flood):

1. **Prefer inline comments over general ones.** Anchor each comment to a specific file/line. Reserve general (non-inline) comments only for an actionable item that genuinely spans the whole PR with no single line to anchor to.

   Inline comment:
   ```
   bb pr comment create --pullrequest <id> --repository <slug> \
     --comment "suggestion: rename this for clarity" --file src/foo.c --to 42
   ```
   - `--to <line>` = NEW (post-change) side — use for **added** ("+") lines (also valid for context). `--line` is an alias for `--to`.
   - `--from <line>` = OLD (pre-change) side — use for **removed** ("-") lines.
   - File line numbers from `grep -n` on the head revision are NEW-side → `--to`.
   - `--parent <comment-id>` replies to an existing comment.
   - `--comment-file <f|->` reads the body from a file or stdin.
   - Only inline (diff) comments can be **resolved** (`bb pr comment resolve <comment-id>`); a general comment returns `403 "You can only resolve comments on the diff."` — another reason to make everything inline where possible.

2. **Batch into one notification with `--pending`.** Stage every comment as a draft by adding `--pending`. A pending comment is a draft: visible only to the review author, sends **no** notification. Do not fire non-pending comments hoping the notifier debounces — it doesn't; that's one email per comment.

   **Verify that `--pending` actually held before bulk-posting.** After the first pending comment, confirm it is genuinely in draft state before continuing: `bb pr comment get <comment-id> --pullrequest <pr>` should show pending, and the comment must **not** appear in `bb pr activities <pr>` (pending drafts never show in the activity feed; published ones do). If it published instead, stop and inform the user — you cannot unsend notifications.

3. **The user submits the review themselves.** There is no API endpoint to publish a batch of pending comments (a known Atlassian gap). Stage all pending comments, hand the user a summary + PR link in chat, and tell them to click "Finish review" in the Bitbucket web UI (publishes all at once, single notification). Do not auto-publish, and do not `approve`/`request-changes` on the user's behalf unless explicitly asked (`approve` is a separate participant action that does not publish pending comments).

## @-mentions in comments

The comment body must use the token form `@{<account_id>}` — the Atlassian account ID wrapped in `@{…}`. A plaintext `@Display Name` is stored verbatim and notifies **nobody** (the web editor hides this by auto-inserting the token; the API/CLI has no autocomplete).

Fetch the account ID first, then write e.g. `--comment "@{557058:…} please re-review"`.

- From the CLI (fields are **flat** — no `.user` nesting in `bb pr get` output): `bb pr get <id> -o json | jq '.author, .reviewers[] | {display_name, account_id}'`
- For the full participant list (commenters etc., not just author/reviewers), use the raw API where users **are** nested under `.user`: `bb api repositories/{workspace}/{repo}/pullrequests/<id> | jq '.participants[] | {name: .user.display_name, id: .user.account_id}'`
