# Releasing

## Semver guidelines

| Bump | When |
|------|------|
| **Major** `v2.0.0` | Breaking changes to the CLI interface — renamed or removed commands, changed flag semantics, output format changes that scripts may depend on |
| **Minor** `v1.x.0` | New commands, new flags, new features — fully backward-compatible |
| **Patch** `v1.1.x` | Bug fixes, performance improvements, dependency updates, documentation |

When in doubt between minor and patch: if a user has to change how they invoke the tool, it's minor. If not, it's patch.

## Pre-release checklist

Run through this before tagging any release:

- [ ] All tests pass — `go test ./...`
- [ ] Vet clean — `go vet ./...`
- [ ] Coverage gate passes — `go test ./internal/... -coverprofile=coverage.out && go tool cover -func=coverage.out`
- [ ] GoReleaser dry run succeeds — `goreleaser release --snapshot --clean --skip=publish`
- [ ] README reflects any new commands or flags
- [ ] Version bump follows the semver guidelines above

## Stable release

```sh
git tag v1.x.0
git push origin v1.x.0
```

> Release tags (`v*`) are protected against deletion and force-pushes.

GitHub Actions takes it from there:
1. Tests run — release is blocked if they fail
2. GoReleaser builds binaries, `.deb`, `.rpm`, and tarballs
3. GitHub Release is published with all artifacts
4. Homebrew formula in `nyactl/homebrew-tap` is updated automatically
5. `CHANGELOG.md` is committed back to `main`

After the release workflow completes, draft and publish prose release notes. GoReleaser's auto-generated changelog is replaced with human-written notes covering only user-facing changes, with usage examples:

```sh
gh release edit vX.Y.Z --notes "..."
```

## Pre-release / RC

Use an RC tag when shipping significant changes (new commands, major refactors) to get early feedback before promoting to stable.

```sh
git tag v1.3.0-rc.1
git push origin v1.3.0-rc.1
```

GoReleaser marks it as a pre-release automatically — it will not be set as "Latest" on GitHub. RC releases go through the same CI pipeline as stable releases but the Homebrew tap is **not** updated for pre-releases. RC entries are also excluded from `CHANGELOG.md`.

To promote to stable once the RC is validated:

```sh
git tag v1.3.0
git push origin v1.3.0
```

Multiple RCs are fine — increment the suffix: `rc.1`, `rc.2`, etc.
