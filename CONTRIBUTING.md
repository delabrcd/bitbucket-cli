# Contributing

Thanks for your interest in this project!

`bb` is a **personal hard fork** of [gildas/bitbucket-cli](https://github.com/gildas/bitbucket-cli), maintained at [delabrcd/bitbucket-cli](https://github.com/delabrcd/bitbucket-cli). It exists to add functionality that makes Bitbucket workflows driven by AI agents more useful and productive, and is **not** merged back upstream.

- If you're looking for the original, actively community-maintained project, head to [gildas/bitbucket-cli](https://github.com/gildas/bitbucket-cli).
- Issues and pull requests against **this** fork are welcome, but this is a spare-time project — I may be slow to respond, and I may decline changes that don't fit how I use the tool. No hard feelings either way.

---

## Reporting issues

Found a bug or have a suggestion? Please [open an issue](https://github.com/delabrcd/bitbucket-cli/issues) with a clear description. Check the existing issues first to avoid duplicates.

## Pull requests

1. **Fork** this repository and **clone** your fork.
2. Create a **feature branch** off the latest `main`.
3. Open your PR against **`main`** (this fork's default branch).

A few things that make a PR easy to accept:

- **Follow the CLI shape.** `bb` uses `bb <resource> <subresource...> <command>` — resources are nouns (`repository`, `pullrequest`), commands are verbs (`list`, `get`, `create`, `update`, `delete`). Support the standard CRUD verbs where they apply.
- **`--dry-run`.** Commands that modify data on Bitbucket should support `--dry-run` to preview changes.
- **Output formats.** Keep `list`/`get` output compatible with the supported formats (JSON, YAML, table, etc.).
- **Format & test.** Run `task fmt` and `task test` (or `go test ./...`) before opening the PR. Add tests where applicable; put JSON payloads in `testdata/` and anonymize them.
- **Docs.** Update the CLI help text for any behavior change. User-facing usage docs live in the [wiki](https://github.com/delabrcd/bitbucket-cli/wiki) rather than the README.

## License

By contributing, you agree that your contributions are licensed under the project's [LICENSE](LICENSE) (MIT).
