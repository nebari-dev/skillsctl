# Skill Resources (Multi-File Skills) - Design

Issue: [#38 - Support Publish and Install of Skill Resources](https://github.com/nebari-dev/skillsctl/issues/38)

## Problem

`skillsctl` today treats a skill as a single `SKILL.md` file: `publish --file
SKILL.md` uploads one file, and `install` writes one file to
`~/.claude/skills/<name>/SKILL.md`. Real Claude Code skills are directories -
`SKILL.md` plus optional `scripts/`, `references/`, `assets/`, and any other
supporting files. The registry needs to publish, store, and install whole
skill directories.

## Goals

- Publish a directory containing `SKILL.md` plus arbitrary supporting files.
- Install that directory atomically into `~/.claude/skills/<name>/`.
- Preserve content addressing: one digest per skill version.
- Keep existing single-file skill records working without migration.
- No proto changes, no DB schema changes.

## Non-Goals

- Per-file queries or partial downloads. The unit of distribution is the
  whole skill version.
- OCI / external blob storage. The tarball lives in the existing
  `skill_versions.content` BLOB.
- A fixed subdirectory layout (no enforced `scripts/`, `references/`).
  Claude Code does not require one and constraining it now is premature.

## Approach

Pack the skill directory into a deterministic `tar.gz`, send it as the
existing `PublishSkillRequest.content` bytes, store it as-is in
`skill_versions.content`, and extract it on install. The tarball's sha256 is
the version digest.

Determinism matters: the same source directory must produce the same bytes
on any host, so the digest is reproducible and the publish flow has no
hidden state.

### Publish flow

1. CLI `publish --dir <path>` walks the directory and validates client-side:
   `SKILL.md` exists at the root, no symlinks, no paths containing `..`.
2. CLI builds a deterministic `tar.gz`:
   - Entries sorted by path.
   - `uid`/`gid` = 0; `uname`/`gname` empty.
   - `mtime` = 0.
   - Mode normalized: files `0644`, dirs `0755`.
   - No xattrs or pax headers beyond what's needed for long names.
3. CLI fetches `GET /limits` and fails fast if the packed tarball exceeds
   the server's `max_packed_bytes`. Fetch failure is non-fatal: fall back
   to a built-in conservative default (5 MB).
4. CLI sends the tarball bytes via the existing
   `PublishSkillRequest.content` field. The proto is unchanged.
5. Server re-validates the tarball (the client is not trusted), computes
   sha256, inserts into `skill_versions.content` with `digest` = sha256,
   `size_bytes` = `len(tarball)`.

### Install flow

1. CLI `install <name>[@version]` calls `GetSkillContent` (unchanged).
2. CLI verifies digest against `SkillVersion.digest`, and against
   `--digest` if provided.
3. Detect format: bytes starting with the gzip magic `1f 8b` are a
   tarball; anything else is treated as a legacy raw `SKILL.md`.
4. Tarball path:
   - If `<skillsDir>/<name>/` exists and `--force` was not passed, fail
     before touching the filesystem.
   - Extract into a sibling temp dir `<skillsDir>/.skillsctl-tmp-<rand>/`,
     re-validating each entry as it streams.
   - On success: with `--force`, remove the existing `<name>/` first;
     then `os.Rename` temp -> `<name>/`.
   - On failure: remove temp; existing `<name>/` is untouched.
5. Legacy path: write the bytes to `<skillsDir>/<name>/SKILL.md` exactly
   as today.

### Validation rules

The server enforces, and the CLI re-enforces on install:

- `SKILL.md` at the tarball root.
- No symlinks, no hardlinks.
- No absolute paths, no paths containing `..` segments.
- File count <= `max_files` (default 100).
- Per-file uncompressed size <= `max_file_bytes` (default 1 MB).
- Total uncompressed size <= `max_total_bytes` (default 20 MB).
- Compressed size <= `max_packed_bytes` (default 5 MB).

Any violation -> `connect.CodeInvalidArgument` from the server, naming the
specific rule.

## Components

### `internal/skillpkg` (shared by CLI and server)

One package, one purpose: pack, validate, and extract skill tarballs.

- `Pack(dir string) ([]byte, error)` - deterministic tar.gz from a
  directory.
- `Validate(tarball []byte, limits Limits) error` - streams the gzip/tar
  and enforces the rules above.
- `Extract(tarball []byte, destDir string, limits Limits) error` -
  re-validates as it extracts; writes via temp files inside `destDir`.
- `IsTarball(b []byte) bool` - gzip magic check for legacy detection.
- `Limits` struct: `MaxPackedBytes`, `MaxTotalBytes`, `MaxFiles`,
  `MaxFileBytes`.

### Server (`backend/internal/registry`, `backend/internal/server`)

- Publish handler calls `skillpkg.Validate` before insert.
- New unauthenticated `GET /limits` endpoint returns the `Limits` as
  JSON.
- Viper config keys: `limits.max_packed_bytes`,
  `limits.max_total_bytes`, `limits.max_files`, `limits.max_file_bytes`.

### CLI (`cli/cmd`, `cli/internal/client`)

- `publish`: replace `--file` with `--dir`. This is a breaking flag
  change, called out in the changelog. Fetch `/limits`, call
  `skillpkg.Pack`, send bytes.
- `install`: add `--force`. Branch on `skillpkg.IsTarball`: tarball ->
  `skillpkg.Extract` with temp-dir + rename; else -> existing single-file
  write.
- New client method for `GET /limits`.

### Proto and DB

No changes. `PublishSkillRequest.content` and
`GetSkillContentResponse.content` already carry `bytes`. The tarball fits
in the existing nullable `skill_versions.content` BLOB. The `digest`
column holds the tarball sha256.

## Error Handling

- Client pre-flight (`Pack`) returns typed errors for missing
  `SKILL.md`, symlinks, and traversal paths so the CLI prints actionable
  messages.
- `/limits` fetch failure: CLI falls back to built-in defaults, lets the
  server be authoritative.
- Server validation failure: `connect.CodeInvalidArgument` with the rule
  named.
- Install: if extraction fails partway, the temp dir is removed and
  `<name>/` is untouched. Digest mismatch:
  `connect.CodeFailedPrecondition` (already wired). Existing dir without
  `--force`: CLI-side error before the network call.
- Legacy single-file rows install exactly as today.

## Testing

- `skillpkg` table-driven unit tests:
  - Pack determinism: same dir -> byte-identical tarball across runs.
  - Validate rejects each bad case: symlink, abs path, `..`, oversize,
    missing `SKILL.md`, too many files.
  - Extract respects limits and cleans up on partial failure.
- Round-trip: `Pack(dir) -> Validate -> Extract` matches the source
  tree.
- Server: publish a multi-file skill via ConnectRPC, assert digest and
  `size_bytes`; publish an invalid tarball, assert `InvalidArgument`.
  Existing single-file publish tests stay green.
- CLI: `publish --dir` happy path and validation failures; `install`
  with and without `--force` against an existing dir; legacy single-file
  install path still works (fixture with raw `SKILL.md` BLOB).
- E2E: extend the existing flow to publish a directory skill end-to-end
  and verify the extracted layout.

## Documentation

- Update `publish` and `install` command reference.
- Add a short "skill layout" section explaining `SKILL.md` plus
  supporting files.
- Changelog entry: `--file` -> `--dir` flag change on `publish`, new
  `--force` flag on `install`, new `GET /limits` endpoint.
