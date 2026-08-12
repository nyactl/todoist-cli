# todoist-cli

A fast, minimal Todoist CLI with a local SQLite cache. Designed for keyboard-driven workflows.

> **Unofficial tool** — not affiliated with or endorsed by [Doist](https://doist.com).

## Install

**Via Homebrew:**

```sh
brew install nyactl/tap/todoist-cli
```

**Via Linux package (`.deb` / `.rpm`):**

Download the package for your architecture from the [releases page](https://github.com/nyactl/todoist-cli/releases) and install:

```sh
# Debian / Ubuntu
sudo dpkg -i todoist-cli_1.3.0_linux_amd64.deb

# Fedora / RHEL
sudo rpm -i todoist-cli_1.3.0_linux_amd64.rpm
```

**Via `go install`:**

```sh
go install github.com/nyactl/todoist-cli/cmd/todoist-cli@latest
```

**From source:**

```sh
git clone https://github.com/nyactl/todoist-cli
cd todoist-cli
make install   # installs to ~/.local/bin/todoist-cli
```

Add `~/.local/bin` to your `$PATH` if needed.

**Verify a release (optional):**

All releases are signed with [cosign](https://github.com/sigstore/cosign) via keyless Sigstore signing. See [RELEASING.md](RELEASING.md#verifying-release-signatures) for the full verification workflow.

## Auth

```sh
td auth login    # prompts for API token, stores it in system keychain
td auth logout   # remove token
td auth status   # verify stored token
```

Get your token from **Todoist → Settings → Integrations → Developer → API token**.

Tokens are stored in the system keychain — never written to disk:

| Platform | Storage |
|----------|---------|
| macOS | Keychain |
| Linux | Secret Service (GNOME Keyring / KWallet) |
| Windows | Credential Manager |

**Headless / WSL / CI** — set `TODOIST_TOKEN` in your environment instead:

```sh
export TODOIST_TOKEN=your_token_here
```

## Commands

`<task>` accepts a task ID, ID prefix, or exact task name — all commands resolve the same way. Tab completion completes by name.

| Command | Description |
|---------|-------------|
| `sync` | Pull tasks, projects, labels, sections into local cache |
| `sync -p <project>` | Sync only one project (faster targeted sync) |
| `ls` | List today's and overdue tasks; or all tasks in active project grouped by section |
| `ls -b` | Board view — sections as side-by-side columns |
| `ls -d [period]` | List completed tasks (live API call) |
| `ls -l <label>` | Filter by label across all active tasks regardless of due date (repeatable, AND logic) |
| `ls -P <1-4>` | Filter by priority (1=normal, 4=urgent) |
| `add <content>` | Create a task in the active project |
| `add -D <due>` | Natural language due date — e.g. `"tomorrow"`, `"every monday"` |
| `add -p <project>` | Override project |
| `add -l <label>` | Attach label (repeatable) |
| `add -P <1-4>` | Set priority (1=normal, 4=urgent) |
| `add --parent <task>` | Create as subtask of the given task |
| `done <task>` | Mark a task complete |
| `edit <task>` | Edit content, due date, priority, description, labels or project |
| `show <task>` | Show full task details, subtasks, and comments |
| `mv <task> <section>` | Move task to a different section within the current project |
| `mv <task> -p <project>` | Move task to a different project |
| `mv <task> -p <project> -s <section>` | Move task to a specific section in a different project |
| `rm <task>` | Delete a task |
| `cp <task>` | Copy task URL to clipboard |
| `open <task>` | Reopen a completed task |
| `search <query>` | Search tasks by content or description across all projects |
| `comment <task> <text>` | Add a comment to a task |
| `overdue` | Triage overdue tasks interactively (done, reschedule, skip) |
| `pick` | Fuzzy-pick a task with fzf — prints ID for shell composition |
| `pick -l <label>` | Pick from label-filtered tasks |
| `cd <project>` | Set active project context |
| `cd` | Clear project context |
| `context` | Print active project, empty if none |
| `projects` | List all projects |
| `projects add <name>` | Create a project |
| `projects add <name> --parent <project>` | Create a sub-project |
| `projects mv <project> <new-name>` | Rename a project |
| `projects rm <project>` | Delete a project — refuses if it or any sub-project still has tasks |
| `projects rm <project> -f` | Force-delete a project even if it (or a sub-project) has tasks |
| `sections` | List sections in the active project |
| `sections rm <section>` | Delete a section from the active project |
| `labels` | List all labels |
| `stats` | Overdue, due today, due this week, open total (+ completed if token available) |

### Periods for `--done`

`today`, `week`, `month`, `year`, `Nd`, `Nw`, `Nm` — e.g. `7d`, `2w`, `3m`

## Shell integration

### Composing with `pick`

`td pick` fuzzy-finds a task and prints its ID — compose it with any command that accepts `<task>`:

```sh
td done $(td pick)              # complete a task
td show $(td pick)              # show details
td cp $(td pick)                # copy URL
td edit $(td pick) -D tomorrow  # reschedule
td pick -l urgent               # pick from urgent tasks only
```

### Alias

```sh
alias td='todoist-cli'
```

### Shell completion

**Zsh** (`~/.zshrc`):
```sh
source <(todoist-cli completion zsh)
```

**Bash** (`~/.bashrc`):
```sh
source <(todoist-cli completion bash)
```

**Fish** (`~/.config/fish/config.fish`):
```sh
todoist-cli completion fish | source
```

Or persist fish completions:
```sh
todoist-cli completion fish > ~/.config/fish/completions/todoist-cli.fish
```

### Prompt integration

`td context` outputs `id<TAB>name` when a project is active, nothing otherwise. Always exits 0.

**Starship** (`~/.config/starship.toml`):
```toml
[custom.todoist]
command = "todoist-cli context | cut -f2"
when = 'todoist-cli context | grep -q .'
format = "[✔ $output]($style) "
style = "fg:#8ec07c"
```

**Plain zsh** (`.zshrc`):
```zsh
_todoist_context() {
  local ctx=$(todoist-cli context 2>/dev/null | cut -f2)
  [[ -n "$ctx" ]] && print -n "%F{green}[✔ $ctx]%f "
}
RPROMPT='$(_todoist_context)'"$RPROMPT"
```

### Auto-sync on shell start (optional)

```sh
# ~/.zshrc
_todoist_bg_sync() {
    todoist-cli sync &>/dev/null &!
}
add-zsh-hook precmd _todoist_bg_sync
```

## Data

All data is stored in `~/.local/share/todoist-cli/` (XDG-compliant, override with `$XDG_DATA_HOME`):

- `todoist-cli.db` — SQLite cache (tasks, projects, labels, sections)
- `state.json` — active project context

The database is a read-through cache. Deleting the directory and running `td sync` is always safe — nothing is lost.

## Support

[![GitHub Sponsors](https://img.shields.io/github/sponsors/nyactl?style=flat&logo=github&label=Sponsor)](https://github.com/sponsors/nyactl)

If this tool saves you time, consider sponsoring — it helps keep the project maintained.
