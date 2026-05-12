# todoist-cli

A fast, minimal Todoist CLI with a local SQLite cache. Designed for keyboard-driven workflows.

> **Unofficial tool** — not affiliated with or endorsed by [Doist](https://doist.com).

## Install

**Via `go install`:**

```sh
go install github.com/kenjikokubo/todoist-cli/cmd/todoist-cli@latest
```

**From source:**

```sh
git clone https://github.com/kenjikokubo/todoist-cli
cd todoist-cli
make install   # installs to ~/.local/bin/todoist-cli
```

Add `~/.local/bin` to your `$PATH` if needed.

## Auth

```sh
td auth login    # prompts for API token, stores it in system keychain
td auth logout   # remove token
td auth status   # verify stored token
```

Get your token from **Todoist → Settings → Integrations → Developer → API token**.

Your token is stored in the macOS Keychain and never written to disk.

## Commands

| Command | Description |
|---------|-------------|
| `sync` | Pull tasks, projects, labels, sections into local cache |
| `ls` | List today's and overdue tasks; or all tasks in active project grouped by section |
| `ls -b` | Board view — sections as side-by-side columns |
| `ls --done [period]` | List completed tasks (live API call) |
| `ls -l <label>` | Filter by label (repeatable, AND logic) |
| `add <content>` | Create a task in the active project |
| `add -D <due>` | Natural language due date — e.g. `"tomorrow"`, `"every monday"` |
| `add -p <project>` | Override project |
| `add -l <label>` | Attach label (repeatable) |
| `done <task>` | Mark a task complete |
| `show <task>` | Show full task details, subtasks, and comments |
| `mv <task> <section>` | Move task to a different section (kanban column) |
| `rm <task>` | Delete a task |
| `cd <project>` | Set active project context |
| `cd` | Clear project context |
| `context` | Print active project, empty if none |
| `projects` | List all projects |
| `labels` | List all labels |

All commands that take a `<task>` argument support tab completion by task name.

### Periods for `--done`

`today`, `week`, `month`, `year`, `Nd`, `Nw`, `Nm` — e.g. `7d`, `2w`, `3m`

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

If this tool saves you time, you can [buy me a coffee](https://buymeacoffee.com) ☕
