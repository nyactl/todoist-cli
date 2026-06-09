# ARCHITECTURE.md — todoist-cli

Developer guide for AI-assisted work on this codebase. Read this before implementing anything.

## Architecture overview

```
cmd/todoist-cli/     — one file per command (add.go, rm.go, mv.go, …)
internal/db/         — SQLite schema, migrations, db.Open()
internal/tasks/      — read-only query helpers against the local cache
internal/todoist/    — HTTP client wrapping the Todoist REST API v1
internal/config/     — token retrieval (keychain + TODOIST_TOKEN env)
internal/state/      — active project context (state.json)
```

The CLI is **cache-first**: all reads come from a local SQLite database populated by `sync`. Mutations (create, update, delete) call the API first, then update the cache optimistically. The cache can always be rebuilt with `sync`.

## Key invariants

- `db.Open()` runs migrations automatically — never touch the schema outside a migration file in `internal/db/migrations/`.
- `tasks.ByID` resolves a task by full ID, ID prefix, exact name, or substring — all commands that accept `<task>` use this.
- `tasks.ProjectByName` and `tasks.SectionByName` do exact name matching — no prefix resolution.
- `boolToInt` in `sync.go` is the canonical bool→SQLite int helper; reuse it, don't redefine.
- `shortID` in `ls.go` truncates an ID to 4 chars for compact display; reuse it.
- `truncate` in `ls.go` truncates a string with `…`; reuse it.
- `formatDue` in `ls.go` formats due dates as `today`, `tomorrow`, `overdue Nd`, or ISO date.

## Command conventions

Every command follows this structure:

1. Open DB (`db.Open()`), defer close.
2. Load context if needed (`loadContext`), error if project required but absent.
3. Resolve task/project/section from cache (fail fast with a helpful error if not found).
4. Get token (`config.GetToken()`), create client (`todoist.New(token)`).
5. Call API — if it fails, return the error **without** modifying the cache.
6. Update cache to reflect the mutation.
7. Print a short confirmation to `cmd.OutOrStdout()`.

**Never modify the cache before the API call succeeds.**

### Adding a new top-level command

- One file: `cmd/todoist-cli/<command>.go`
- Register in `init()` with `root.AddCommand(cmd)`
- Mirror the `Use`, `Short`, `Args`, `ValidArgsFunction` pattern from existing commands
- If the command has subcommands (like `projects add`/`projects rm`), the parent command keeps its `RunE` for the default action (list) and subcommands are added via `parent.AddCommand(sub)` in `init()`

### Flag conventions

| Flag | Short | Used for |
|------|-------|----------|
| `--project` | `-p` | project name |
| `--section` | `-s` | section name |
| `--label` | `-l` | label (repeatable, StringArrayVar) |
| `--due` | `-D` | due date string |
| `--priority` | `-P` | priority 1–4 |
| `--description` | `-d` | task description |
| `--content` | `-c` | task title replacement |
| `--parent` | none | parent task (for subtasks) or parent project |

`StringArrayVar` flags (`--label`) must be manually reset to `nil` in `harness_test.go:runCmd` — add them to the reset block there.

### Output conventions

- Mutations print `"deleted: <name>\n"`, `"updated  <shortID>  <content>\n"`, or similar short confirmation via `cmd.OutOrStdout()`.
- Creation prints the new resource ID via `fmt.Println(id)` (goes to os.Stdout).
- List commands use `fmt.Printf` directly with tab-separated `id\tname` format.
- Errors are returned, not printed — cobra handles printing.

## API client

`internal/todoist/` wraps the Todoist REST API v1 (`https://api.todoist.com/api/v1`).

- `doJSON(ctx, method, path, body, &out)` — sends JSON body, decodes JSON response.
- `do(ctx, method, path, body)` — raw request, caller closes body.
- `queryAll[T]` — handles cursor-based pagination for list endpoints.

Set `TODOIST_API_BASE` env var to redirect all API calls to a test server.

**Adding a new API method**: follow the pattern in `tasks.go` or `projects.go`. For DELETE endpoints that return 204, use `do()` and close the body. For POST/GET that return a resource, use `doJSON()`.

## Testing

**All tests use the stub-server pattern** — no real API calls, no network dependency.

### Test harness (`harness_test.go`)

- `newTestEnv(t, handler)` — sets up an isolated temp DB, sets `TODOIST_TOKEN=test-token`, and if `handler != nil` starts an `httptest.Server` and sets `TODOIST_API_BASE` to its URL.
- `runCmd(t, args...)` — executes the cobra command tree, captures stdout, resets flag state between calls.
- `writeJSON(w, v)` — writes a JSON response from the stub server.
- `pageResponse[T](items)` — wraps items in the paginated response envelope `{results: [...], next_cursor: ""}`.
- `emptyAPI()` — stub that returns empty lists for all sync endpoints.
- `noopHandler` — returns 204 No Content, used for close/delete/move.

### Seed helpers

| Helper | What it inserts |
|--------|----------------|
| `hSeedProject(t, conn, id, name)` | project with default ord=0 |
| `hSeedProjectOrd(t, conn, id, name, ord)` | project with explicit ord |
| `hSeedArchivedProject(t, conn, id, name)` | archived project |
| `hSeedSection(t, conn, id, name, projectID, ord)` | section |
| `hSeedTask(t, conn, id, content, projectID, sectionID)` | task |
| `hSeedSubtask(t, conn, id, content, projectID, parentID)` | subtask |
| `hSeedLabel(t, conn, id, name, ord)` | label |

### Required test coverage for every mutating command

Every command that calls the API and modifies the cache must have tests for:

1. **API payload** — correct fields/IDs sent to the stub (capture the request body or URL path).
2. **Cache update** — query the DB after the command and assert the expected state.
3. **Confirmation output** — assert the correct string appears in stdout.
4. **API error** — stub returns 4xx; assert error returned and cache **unchanged**.
5. **Unknown resource** — name/ID not in local cache; assert error before any API call.
6. **No-args / wrong-args** — cobra `Args` validation exercised explicitly.

For commands scoped to a project context, also test:
7. **No project context** — assert error when `cd` has not been run.
8. **Project scoping** — same name exists in another project; assert only the correct one is affected.

### Stub server patterns

```go
// Capture DELETE path
mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
    deletedID = strings.TrimPrefix(r.URL.Path, "/tasks/")
    w.WriteHeader(http.StatusNoContent)
})

// Capture POST body
mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
    var req todoist.CreateTaskRequest
    json.NewDecoder(r.Body).Decode(&req)
    writeJSON(w, todoist.Task{ID: "new-id", ...})
})

// Simulate API error
mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
    http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
})
```

Note: in Go's `http.ServeMux`, `/tasks/` (trailing slash) matches all sub-paths; `/tasks` (no slash) matches exactly. Use `/tasks` for create (POST to collection) and `/tasks/` for single-resource operations.

## Schema

Single migration: `internal/db/migrations/001_initial.sql`.

Tables: `projects`, `labels`, `sections`, `tasks`, `task_labels`, `sync_state`.

Key relationships:
- `sections.project_id` → `projects.id` ON DELETE CASCADE
- `tasks.project_id` → `projects.id` ON DELETE SET NULL
- `tasks.section_id` → `sections.id` ON DELETE SET NULL
- `tasks.parent_id` → `tasks.id` ON DELETE SET NULL
- `task_labels.task_id` → `tasks.id` ON DELETE CASCADE

Adding a new column always requires a new migration file (`002_*.sql`, etc.).

## Release process

See `RELEASING.md` for the full checklist. Summary:

1. `go test ./...` and `go vet ./...` must be clean.
2. Commit all changes to `main`.
3. `git tag vX.Y.Z && git push origin main && git push origin vX.Y.Z`
4. GoReleaser CI builds binaries, packages, and updates the Homebrew tap automatically.
5. After the release workflow completes, publish prose release notes via `gh release edit vX.Y.Z --notes "..."`.

**Semver**: new commands/flags → minor bump. Output/behaviour fixes → patch. Breaking changes → major.

GoReleaser commits `CHANGELOG.md` back to `main` after each release — always `git pull --rebase origin main` before tagging the next release.
