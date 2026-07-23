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
- Test status: Pending because Go is not currently available in `PATH`

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
