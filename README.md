# Bitbucket Command Line Interface

[bb](https://github.com/delabrcd/bitbucket-cli) is the missing command line interface for Bitbucket. It brings the power of the Bitbucket platform to your command line — creating and merging pull requests, cloning repositories, driving pipelines, and more are just a few keystrokes away.

This is a hard fork of [gildas/bitbucket-cli](https://github.com/gildas/bitbucket-cli), maintained independently at `github.com/delabrcd/bitbucket-cli`.

## Installation

On Linux, add the package repository so `bb` stays up to date with your normal `apt upgrade` / `dnf upgrade` / `pacman -Syu`:

```bash
# Debian / Ubuntu
curl -fsSL https://delabrcd.github.io/bitbucket-cli/bitbucket-cli.gpg | sudo tee /usr/share/keyrings/bitbucket-cli.gpg > /dev/null
echo "deb [signed-by=/usr/share/keyrings/bitbucket-cli.gpg] https://delabrcd.github.io/bitbucket-cli/deb ./" | sudo tee /etc/apt/sources.list.d/bitbucket-cli.list
sudo apt update && sudo apt install bitbucket-cli

# Fedora / RHEL
sudo curl -fsSL -o /etc/yum.repos.d/bitbucket-cli.repo https://delabrcd.github.io/bitbucket-cli/rpm/bitbucket-cli.repo
sudo dnf install bitbucket-cli
```

The Windows installer, Arch (pacman) repo, `go install`, and standalone binaries are on the [Releases page](https://github.com/delabrcd/bitbucket-cli/releases). See the **[Installation guide](https://github.com/delabrcd/bitbucket-cli/wiki/Installation)** for all methods and details.

## Quick start

```bash
# 1. Create and authorize a profile (OAuth 2.0 shown; API tokens & access tokens also supported)
bb profile create --name myprofile --default-workspace myworkspace \
  --client-id <id> --client-secret <secret> --callback-port 8080
bb profile authorize myprofile

# 2. Go
bb repo list --workspace myworkspace
bb pullrequest create --title "My PR" --source my-branch --destination main
bb pullrequest merge
```

`bb` uses a clean `bb <resource> <action>` grammar. Get help on anything with `bb --help` or `bb <subcommand> --help`. By default it works in the current git repository; most commands accept `--repository`, `--workspace`, `--output <format>`, and `--dry-run`.

For endpoints without a dedicated command, `bb api` is a `gh api`-style authenticated passthrough to the [Bitbucket Cloud REST API](https://developer.atlassian.com/cloud/bitbucket/rest/intro/).

## Documentation

Full usage documentation lives in the **[wiki](https://github.com/delabrcd/bitbucket-cli/wiki)**:

- [Installation](https://github.com/delabrcd/bitbucket-cli/wiki/Installation)
- [Getting Started](https://github.com/delabrcd/bitbucket-cli/wiki/Getting-Started) — grammar, global flags, output formats, filtering & sorting
- [Profiles & Authentication](https://github.com/delabrcd/bitbucket-cli/wiki/Profiles-and-Authentication)
- [REST API Passthrough](https://github.com/delabrcd/bitbucket-cli/wiki/REST-API) (`bb api`)
- [Users, Workspaces & Projects](https://github.com/delabrcd/bitbucket-cli/wiki/Users-Workspaces-and-Projects)
- [Repositories, Branches, Commits & Tags](https://github.com/delabrcd/bitbucket-cli/wiki/Repositories-Branches-Commits-and-Tags)
- [Pull Requests](https://github.com/delabrcd/bitbucket-cli/wiki/Pull-Requests)
- [Issues](https://github.com/delabrcd/bitbucket-cli/wiki/Issues)
- [Pipelines, Runners & Artifacts](https://github.com/delabrcd/bitbucket-cli/wiki/Pipelines-Runners-and-Artifacts)
- [SSH & GPG Keys](https://github.com/delabrcd/bitbucket-cli/wiki/SSH-and-GPG-Keys)
- [Shell Completion](https://github.com/delabrcd/bitbucket-cli/wiki/Shell-Completion) · [Agent Skill](https://github.com/delabrcd/bitbucket-cli/wiki/Agent-Skill) · [Cache](https://github.com/delabrcd/bitbucket-cli/wiki/Cache) · [Debugging & Logs](https://github.com/delabrcd/bitbucket-cli/wiki/Debugging-and-Logs)

## Roadmap

We will add more commands in the future. If you have any suggestions, please [open an issue](https://github.com/delabrcd/bitbucket-cli/issues).

We are in the process of adding support for Bitbucket Server/Data Center ([issue #65](https://github.com/delabrcd/bitbucket-cli/issues/65)).

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) and our [Code of Conduct](CODE_OF_CONDUCT.md). This project is licensed under the terms of the [LICENSE](LICENSE).

## Stargazers over time

[![Stargazers over time](https://starchart.cc/delabrcd/bitbucket-cli.svg?variant=adaptive)](https://starchart.cc/delabrcd/bitbucket-cli)
