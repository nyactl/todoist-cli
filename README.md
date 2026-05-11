# todoist-cli

A fast, minimal Todoist CLI with a local SQLite cache. Designed for keyboard-driven workflows.

## Install

```sh
git clone <repo-url>
cd todoist-cli
go build -o ~/.local/bin/todoist-cli ./cmd/todoist-cli
```

Or with Make:

```sh
make install
```

## Auth

```sh
todoist-cli auth login    # save API token to system keychain, then syncs automatically
todoist-cli auth logout   # remove token
todoist-cli auth status   # verify token
```

Get your token from **Todoist → Settings → Integrations → Developer → API token**.

## Commands

| Command | Description |
|---------|-------------|
| `sync` | Pull all tasks, projects, labels, sections into local cache |
| `ls` | List today's and overdue tasks (or all tasks in active project) |
| `ls --done [period]` | List completed tasks (live API call) |
| `ls -l <label>` | Filter by label (repeatable, AND logic) |
| `add <content>` | Create a task |
| `done <id>` | Mark a task complete |
| `open <id>` | Reopen a completed task |
| `show <id>` | Show full task details, subtasks, and comments |
| `cd <project>` | Set active project context |
| `cd` | Clear project context |
| `context` | Print active project (`id<TAB>name`), empty if none |
| `projects` | List all projects (`id<TAB>name`) |
| `labels` | List all labels (`id<TAB>name`) |

### Periods for `--done`

`today`, `week`, `month`, `year`, `Nd`, `Nw`, `Nm` — e.g. `7d`, `2w`, `3m`

### ID prefix resolution

All commands that take an `<id>` accept a 4-character prefix. The local cache is queried first; a `sync` is needed if the task is not cached yet.

## Shell integration

### Alias

```sh
alias td='todoist-cli'
```

### Zsh completion

```sh
# ~/.zshrc
source <(todoist-cli completion zsh)
```

### Fish completion

```sh
todoist-cli completion fish | source
```

### Prompt integration

`todoist-cli context` outputs `id<TAB>name` when a project is active, nothing otherwise. Always exits 0. Reads only `~/.todoist-cli/state.json` — no DB, sub-millisecond.

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

Keeps the cache fresh without blocking your shell:

```sh
# ~/.zshrc
_todoist_bg_sync() {
    todoist-cli sync &>/dev/null &!
}
add-zsh-hook precmd _todoist_bg_sync
```

## Data

All data is stored in `~/.todoist-cli/`:

- `todoist-cli.db` — SQLite cache (projects, tasks, labels, sections)
- `state.json` — active project context

The database is a read-through cache; `sync` rebuilds it from the API. Deleting the directory and re-running `sync` is always safe.
