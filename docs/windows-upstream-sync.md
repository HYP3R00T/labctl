# Updating the Windows branch from upstream

The `windows-support` branch adds Windows compatibility on top of the official
labctl releases. Upstream updates should first be tested on a temporary branch
so the known-working Windows version remains available.

## Current update

- Previous base: `v0.1.91`
- Backup: `backup/windows-support-v0.1.91`
- Update target: `v0.1.100`
- Temporary branch: `codex/update-windows-v0.1.100`
- Merge status: Windows changes applied without conflicts
- Test status: Automated checks completed with the Windows limitation below

The temporary branch starts from the official `v0.1.100` tag. The existing
Windows-support commit is then applied on top of it.

## Workflow

1. Confirm that the working tree is clean.
2. Create a backup branch named after the current labctl version.
3. Fetch upstream branches and tags.
4. Choose the latest stable release tag, not the moving `upstream/main` branch.
5. Create a temporary update branch from that release.
6. Apply the Windows changes to the temporary branch.
7. Test the result on Windows.
8. Update `windows-support` only after the tests pass.

On this Windows machine, GitHub fetching may require the OpenSSL backend:

```powershell
git -c http.sslBackend=openssl fetch --prune upstream --tags
```

## Checks before accepting an update

- Run the Go tests and build the Windows executable.
- Check login, SSH, SCP, IDE connection, kube-proxy, and port forwarding.
- Check Windows paths, OpenSSH discovery, Ctrl+C handling, and process cleanup.
- Keep the backup branch until the updated Windows version is confirmed working.

If the update fails, return to `windows-support` or the versioned backup branch.

## Automated results for v0.1.100

- `go build ./...`: Passed on Windows/AMD64.
- `go vet ./...`: Passed.
- `go mod verify`: Passed.
- `labctl version` and `labctl --help`: Passed.
- `go test ./...`: All reported packages passed except one upstream content
  test that creates a symbolic link. Windows rejected the symlink because the
  current session does not have the required privilege.
- Race-enabled tests were not run because CGO is disabled.
- `gofmt -l .` reported one pre-existing extra blank line in
  `internal/config/config.go`.

Manual testing is still required before replacing `windows-support`.

## Windows content download fix

Content download URLs now use `path.Join`, which keeps forward slashes on every
operating system. Local destination paths continue to use `path/filepath`.
The audit found no other HTTP endpoint built with `filepath.Join`.

`content create` now distinguishes remote creation failures, local directory
preparation failures, and initial download failures. If a download fails after
remote creation, the error reports the created content name and URL and prints
a `labctl content pull` recovery command. It does not delete the remote content.

Automated Windows verification:

- Nested `__static__/WARNING.txt` downloads passed for both content creation and
  content pull using a local test server.
- The full Go test suite and `go vet ./...` passed.
- `labctl.exe` was rebuilt and its version and content-pull help commands ran
  successfully.
- Manual Windows testing of the rebuilt executable passed.

## Windows content upload paths

Locally discovered content files are converted from native filesystem paths to
slash-separated remote keys before reconciliation. Remote keys returned by the
API are not normalized, so malformed backslash keys remain visible and are
deleted after the canonical slash key is uploaded.

Canonical remote keys are converted back with `filepath.FromSlash` only when
opening the corresponding local file. This applies to one-time pushes and every
watch-mode reconciliation.

Automated Windows verification covers:

- Root and nested local files producing only slash-separated remote keys.
- Binary files under `__static__` and Markdown files in nested directories.
- Uploading the canonical key while deleting an existing malformed backslash
  key.
- Watch mode retaining the canonical slash key after a nested file update.
- The complete Go test suite, `go vet ./...`, and `go build ./...`.
