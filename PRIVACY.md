# Privacy policy

tdc for Todoist (`todoist-cli`) is a command-line program that runs on your computer. It has no servers of its own and collects no data.

## What it accesses

- **Your Todoist data** — tasks, projects, sections, labels and comments, read and changed through the official Todoist API at `api.todoist.com`, only when you run a command.
- **Your Todoist API token** — needed to call the API on your behalf.

## Where it is stored

- **API token:** in your operating system's credential store (macOS Keychain, Secret Service on Linux, Windows Credential Manager), or in the `TODOIST_TOKEN` environment variable if you set one. It is never written to a file by the program.
- **Task data:** in a local SQLite cache at `~/.local/share/todoist-cli/` (or under `$XDG_DATA_HOME`). It never leaves your computer except as requests to the Todoist API. Deleting that directory removes it; `todoist-cli auth logout` removes the token.

## What it does not do

- No analytics, telemetry, crash reporting or tracking.
- No connections to any server other than `api.todoist.com`.
- No selling, sharing or transferring of your data to anyone.

Todoist's own handling of your data is covered by the [Todoist privacy policy](https://www.todoist.com/privacy).

## Contact

Questions about this policy: open an issue at https://github.com/nyactl/todoist-cli/issues.
