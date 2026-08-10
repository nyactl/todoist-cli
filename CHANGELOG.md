# Changelog

## [1.11.1] - 2026-08-10

### Bug Fixes

- Error on ambiguous project name instead of silently picking the first

### Documentation

- Document projects rm cascade and sync FK known limitations


## [1.11.0] - 2026-07-21

### Features

- Add projects mv to rename a project


## [1.10.0] - 2026-07-21

### Documentation

- Add architecture and contribution guide
- Fix ARCHITECTURE.md title and description

### Features

- Add sections rm subcommand
- Guard projects rm against deleting non-empty projects without --force


## [1.9.1] - 2026-06-09

### Features

- Print resolved date after rescheduling in overdue triage
- Print due date cleared when rescheduling with no date


## [1.9.0] - 2026-06-08

### Features

- Show project/labels in triage header, fetch comments in v


## [1.8.0] - 2026-06-08

### Features

- Add -p flag to mv for cross-project task moves
- Add v = details action to overdue triage


## [1.6.0] - 2026-06-07

### Documentation

- Document projects add and rm in README

### Features

- Add projects add and rm subcommands


## [1.5.0] - 2026-06-05

### Features

- Add overdue triage command
- Add --parent flag to add for subtask creation


## [1.4.0] - 2026-06-04

### Bug Fixes

- Remove --tag flag from git-cliff, fix duplicate CHANGELOG sections

### Documentation

- Clarify tag protection scope in RELEASING.md
- Add .deb and .rpm install instructions

### Features

- Add cosign keyless signing for release artifacts
- Add comment command to post task comments


## [1.3.0] - 2026-06-04

### Bug Fixes

- Remove Unreleased section from CHANGELOG, only emit tagged releases
- Align release/CI test gates, skip Homebrew for RC, exclude RC from changelog

### CI

- Gate release on test job passing

### Documentation

- Add RELEASING.md with semver guidelines, checklist, and RC workflow

### Features

- Add CHANGELOG.md with git-cliff, auto-update on release
- Add .deb/.rpm packages, drop Windows support
- Add search command for full-text task search


## [1.2.0] - 2026-06-04

### Bug Fixes

- Set Formula/ directory for homebrew tap output
- Add -d shorthand to ls --done flag

### Documentation

- Fix open command description

### Features

- Add -P priority filter to ls command


## [1.1.0] - 2026-06-03

### Bug Fixes

- Use cd context in add, purge deleted tasks on sync, add --due flag
- Wrap sync steps in transactions, case-insensitive task content lookup
- Parallelize sync fetches, substring task lookup, URL fallback in cp
- Construct task URL from ID instead of relying on API field
- Install binary as todoist-cli, no alias

### CI

- Add GitHub Actions CI and GoReleaser release workflow
- Opt into Node.js 24 for GitHub Actions runners

### Documentation

- Clarify task resolution in README, remove duplicate row
- Add td open to command table in README
- Add Homebrew install instructions
- Document --priority flag on add

### Features

- Board view, drop task IDs from ls, fix section order
- Mv task between sections, rm task
- Add --section flag to td add for direct kanban column placement
- Add td stats command with overdue, due today, this week and open counts
- Add td sections command to list sections in active project
- Add td edit command for partial task updates
- Add td cp command to copy task URL to clipboard
- Td sync -p to sync a single project
- Add td pick for interactive task selection via fzf
- Add --version flag injected via GoReleaser ldflags
- Add Homebrew tap via GoReleaser
- Add --priority flag to add command

### Refactoring

- Clean up command structure, fix completions, add description flag



