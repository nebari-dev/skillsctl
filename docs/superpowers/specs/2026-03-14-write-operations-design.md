# Write Operations Design

**Date:** 2026-03-14
**Status:** Approved
**Scope:** PublishSkill and GetSkillContent RPCs, ownership model, version immutability, content storage in SQLite

## Context

The backend currently supports read-only operations (ListSkills, GetSkill). This design adds write operations so authenticated users can publish skills and retrieve skill content for installation.

Key decisions made during design:
- **No push tokens** - any authenticated OIDC user can publish. Fine-grained write authorization (OIDC scopes) deferred to the roadmap.
- **No OCI registry** - skill content stored directly in SQLite as a BLOB. Skills are small text-based artifacts where OCI adds complexity without clear value. The Repository interface is kept clean so OCI can be added later if needed.
- **Ownership model** - the first publisher of a skill becomes the owner. Only the owner can publish new versions.
- **Version immutability** - once a version is published, it cannot be overwritten or changed.

## Proto Changes

Two new RPCs added to `RegistryService` in `registry.proto`:

```protobuf
service RegistryService {
  // Existing read RPCs
  rpc ListSkills(ListSkillsRequest) returns (ListSkillsResponse);
  rpc GetSkill(GetSkillRequest) returns (GetSkillResponse);

  // New write RPCs
  rpc PublishSkill(PublishSkillRequest) returns (PublishSkillResponse);
  rpc GetSkillContent(GetSkillContentRequest) returns (GetSkillContentResponse);
}

message PublishSkillRequest {
  string name = 1;         // skill name (lowercase alphanumeric + hyphens, 2-64 chars)
  string version = 2;      // semver (e.g. "1.2.0")
  string description = 3;  // required, max 2000 chars
  repeated string tags = 4;
  string changelog = 5;    // optional, describes what changed in this version
  bytes content = 6;       // skill archive content (max 1MB)
}

message PublishSkillResponse {
  Skill skill = 1;                // updated skill metadata
  SkillVersion version = 2;       // the newly created version
}

message GetSkillContentRequest {
  string name = 1;       // skill name
  string version = 2;    // optional - defaults to latest version
  string digest = 3;     // optional - if provided, server verifies content matches
}

message GetSkillContentResponse {
  bytes content = 1;              // skill content bytes
  SkillVersion version = 2;      // version metadata (includes digest for lock files)
}
```

**ConnectRPC max message size**: The server must configure ConnectRPC's max receive message size to at least 1MB + overhead to accommodate the content field. Default may be smaller.

**Note on `oci_ref` field**: The `SkillVersion.oci_ref` field (field 3) in `skill.proto` is retained but left empty by this implementation. It is reserved for future use if OCI storage is added.

## Database Migration

New migration `002_skill_content.sql`:

```sql
-- +goose Up
ALTER TABLE skill_versions ADD COLUMN content BLOB;

-- +goose Down
ALTER TABLE skill_versions DROP COLUMN content;
```

Content is stored on `skill_versions` because each version has its own content. The column is nullable: NULL means "no content stored" (e.g., federated skills or legacy rows), while an empty blob would mean "content is explicitly empty." `GetSkillContent` returns `ErrNotFound` when content is NULL. The existing `digest` column is populated with sha256 of the content on publish.

## Repository Interface

Two new methods added to `store.Repository`:

```go
type Repository interface {
    // Existing
    ListSkills(ctx context.Context, tags []string, sourceFilter skillsctlv1.SkillSource, pageSize int32, pageToken string) ([]*skillsctlv1.Skill, string, error)
    GetSkill(ctx context.Context, name string) (*skillsctlv1.Skill, []*skillsctlv1.SkillVersion, error)

    // New
    CreateSkillVersion(ctx context.Context, skill *skillsctlv1.Skill, version *skillsctlv1.SkillVersion, content []byte) error
    GetSkillContent(ctx context.Context, name string, version string, digest string) ([]byte, *skillsctlv1.SkillVersion, error)
}
```

New sentinel errors:

```go
var ErrAlreadyExists = errors.New("already exists")
var ErrPermissionDenied = errors.New("permission denied")
```

### CreateSkillVersion behavior

- **First publish** (skill doesn't exist): INSERT into `skills` with caller's OIDC subject as owner, INSERT into `skill_versions` with content and computed digest. Source is set to `SKILL_SOURCE_INTERNAL`.
- **New version** (skill exists): Check caller's OIDC subject matches `skills.owner`. If not, return `ErrPermissionDenied`. INSERT version - rely on the `PRIMARY KEY (skill_name, version)` constraint for immutability. If the INSERT fails with a unique constraint violation, map to `ErrAlreadyExists`. UPDATE `skills.latest_version` only if the new version is semver-greater than the current latest (prevents downgrade when publishing a patch for an older line). UPDATE `skills.updated_at`.
- Both operations run in a transaction.
- The `owner` field stores the OIDC subject (immutable identity). The `published_by` field on versions stores the email (human-readable). Ownership checks always compare against subject, never email.

### GetSkillContent behavior

- If version is empty, resolve to `skills.latest_version`.
- Fetch the version row including `content` from `skill_versions`.
- If digest is provided and doesn't match the stored digest, return an error (content has changed or was corrupted).
- Return content bytes and version metadata.

## Service Layer

`registry.Service` gets two new handler methods.

### PublishSkill handler

1. Extract caller identity via `auth.ClaimsFromContext(ctx)`. Check the `ok` return value - if false (nil validator / dev mode), return `CodeUnauthenticated`. Publishing requires a verified identity for ownership.
2. Validate inputs:
   - `name`: lowercase alphanumeric + hyphens, 2-64 chars, regex `^[a-z0-9][a-z0-9-]*[a-z0-9]$` (min 2 chars to allow names like `go`)
   - `version`: valid semver (use Go `semver` package or simple regex)
   - `description`: required, max 2000 chars
   - `content`: non-empty, max 1MB
   - `tags`: each tag lowercase alphanumeric + hyphens, max 64 chars per tag, max 20 tags
3. Compute sha256 digest of content.
4. Build `Skill` and `SkillVersion` proto messages with caller's subject as owner and email as published_by.
5. Call `repo.CreateSkillVersion()`.
6. Map `ErrAlreadyExists` to Connect `CodeAlreadyExists`, `ErrPermissionDenied` to Connect `CodePermissionDenied`.
7. Return the skill and version in the response.

### GetSkillContent handler

1. Extract caller identity via `auth.ClaimsFromContext(ctx)`. Check the `ok` return value - if false and nil validator (dev mode), allow the request (read operation). If validator is configured but token is invalid, the interceptor already rejects it.
2. Call `repo.GetSkillContent(name, version, digest)`.
3. Map `ErrNotFound` to Connect `CodeNotFound`.
4. If digest mismatch, return Connect `CodeFailedPrecondition` with descriptive message.
5. Return content and version metadata.

## Security

- **Ownership**: Enforced at the repository layer. The owner is the OIDC subject (immutable identity) from the token's claims on first publish. Only the owner can push subsequent versions. Subject is used instead of email because emails can be reassigned in some OIDC providers.
- **Version immutability**: Enforced at the repository layer. Attempting to publish an existing version returns `ErrAlreadyExists`.
- **Content size limit**: 1MB max, enforced at the service layer before touching the database.
- **Audit trail**: The `published_by` field on each version records who published it. A full audit log table is planned in a future migration.
- **Input validation**: Name format, semver version, description length, tag count - all validated before write.

## CLI Changes

No new CLI commands in this PR. The `explore` commands remain metadata-only:
- `skillsctl explore` - list skills (metadata only)
- `skillsctl explore show <name>` - show skill detail (metadata only)
- `skillsctl explore show <name> --verbose` - include content in output (new flag)

CLI `install` and `push` commands will be added in Phase 4.

## Testing

- **Repository tests**: Table-driven tests for both SQLite and Memory implementations covering first publish, second version, ownership rejection, version immutability, content retrieval, digest verification.
- **Service tests**: Handler tests with mock repository covering input validation, auth integration, error mapping.
- **Migration test**: Verify 002 migration applies cleanly on top of 001.

## Future Extensibility

- The `Repository` interface accepts/returns `[]byte` content, so an OCI-backed implementation could store content externally and return it transparently.
- The `digest` field enables lock file support for the planned skill manifest feature.
- The `GetSkillContentRequest.digest` field enables verified downloads for reproducible installs.
- No push token infrastructure is baked in, so OIDC scopes can be layered in cleanly when needed.
