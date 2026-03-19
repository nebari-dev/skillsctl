---
title: "Versioning and ownership"
weight: 40
---

# Versioning and ownership

## Versions

### Semver is required

Every skill version must follow semantic versioning: `MAJOR.MINOR.PATCH`. The server rejects versions that don't match this format.

Examples of valid versions: `1.0.0`, `2.3.1`, `0.1.0`

Examples of invalid versions: `v1.0.0`, `1.0`, `latest`, `2024-01-15`

### Versions are immutable

Once a version is published, its content cannot be changed. The primary key constraint on `(skill_name, version)` enforces this at the database level. If you publish `my-skill@1.0.0`, that content is fixed. To update the skill, publish `1.0.1` or `1.1.0`.

Immutability means installs are reproducible. If you pin a version and record its SHA-256 digest, you can verify the content has not changed:

```bash
SkillsCtl install my-skill@1.0.0 --digest sha256:abc123...
```

### latest_version tracking

The registry tracks a `latest_version` field for each skill. When a new version is published, `latest_version` is updated only if the new version is semver-greater than the current latest. Publishing an old version (e.g., a backport to `1.x` while `2.0.0` is current) does not change `latest_version`.

`skillsctl install my-skill` without a version specifier installs `latest_version`.

### Changelogs

Each version can include a changelog message. Pass it with `--changelog` when publishing:

```bash
SkillsCtl publish \
  --name my-skill \
  --version 1.1.0 \
  --description "..." \
  --file my-skill.md \
  --changelog "add output format instructions, fix typo in section 2"
```

Changelogs are stored with each version and help users decide whether to update.

### Digest verification

Every version has a SHA-256 digest computed from its content. The digest is displayed when you publish:

```
Published my-skill@1.0.0 (sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855)
```

Pin a version with its digest to detect if the content has been tampered with:

```bash
SkillsCtl install my-skill@1.0.0 --digest sha256:e3b0c44...
```

If the digest does not match, installation fails.

## Ownership

### Keyed on OIDC subject

Skill ownership is tied to the OIDC subject (`sub` claim) of the user who first published the skill. The subject is an opaque identifier assigned by the OIDC provider - it does not change when the user updates their email or display name.

Using subject instead of email means:
- A user who changes their email retains ownership of their skills
- Ownership is stable across provider-side profile updates
- Two accounts at the same provider always have different subjects

### What ownership enforces

Once a skill is owned by a subject, only that subject can publish new versions of it. Other users receive an error if they attempt to publish to a skill name they don't own.

Ownership is established on first publish. There is no separate ownership claim step.

### Admin override

Admin users (members of the group specified by `OIDC_ADMIN_GROUP`) can publish to any skill regardless of ownership. This allows platform admins to take over unmaintained skills or handle off-boarding.

### Transferring ownership

Ownership transfer is not currently supported through the CLI. An admin can update the owner field directly in the database if needed.

## Next steps

- [Auth model]({{< relref "/concepts/auth" >}}) - how OIDC subjects are established
- [Security]({{< relref "/concepts/security" >}}) - how immutability and digests fit into the security model
