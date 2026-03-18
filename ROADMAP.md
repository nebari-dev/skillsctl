# skillsctl Roadmap

## Current - MVP

Core backend and CLI for discovering, installing, and publishing Claude Code skills.

- [x] Proto definitions (skill.proto, registry.proto)
- [x] Backend: SQLite store with read operations (ListSkills, GetSkill)
- [x] Backend: OIDC auth middleware (token validation, admin group check)
- [x] Backend: Write operations (PublishSkill, GetSkillContent)
- [x] Backend: /auth/config endpoint for zero-config CLI auth
- [x] CLI: Config management (init, set, get, list)
- [x] CLI: OIDC device flow login (discovers settings from server)
- [x] CLI: Install and publish commands
- [x] CI/CD pipelines (GitHub Actions, GoReleaser, Docker)
- [x] Homebrew tap (brew tap nebari-dev/tap && brew install skillsctl)
- [x] Dogfood skill (skillsctl-usage.md)
- [ ] Helm chart for Kubernetes deployment
- [ ] Federation and marketplace management
- [ ] Documentation site (Hugo, GitHub Pages)

## Next - Post-MVP

Prioritized features after the core is stable.

### OIDC scopes for publish authorization
Fine-grained write access control via OIDC scopes or custom claims (e.g., `skillsctl:publish`) rather than allowing any authenticated user to publish.

### LLM-based skill scanning (opt-in)
Optional defense-in-depth scan of skill content on publish using an LLM to detect potentially malicious instructions. A dedicated system prompt can be tuned to catch prompt injection patterns, data exfiltration attempts, and social engineering in skill content. Opt-in only - requires LLM API credentials and adds an external dependency. Tradeoffs: adds a useful signal beyond what static analysis can catch for natural language payloads, but no security measure is perfect, and adding complexity always adds risk (the LLM API credentials themselves become an attack surface). Not a replacement for ownership, immutability, and audit logging.

### Admin approval queue (opt-in)
Optional workflow where new skills or first-time publishers require admin approval before the skill is visible. Off by default. Adds friction and relies on admins actually reviewing content, so likely less effective than automated scanning for most orgs. May be useful for compliance-driven environments.

### Signed skills
Publisher signs skill content cryptographically, users verify on install. Similar to cosign for container images. Provides tamper detection without requiring trust in the registry.

## Later

### Skill manifest file
A `.claude/skills.yaml` file that declares which skills a project uses, with pinned versions. Run `skillsctl install` to sync all skills from the manifest - like pixi.toml or package.json for skills. Includes `skillsctl add <name>` to install and add to the manifest, plus a lock file with content digests for reproducible installs.

### OCI artifact storage
Optional OCI registry backend (via oras-go) for skill content storage. Useful if skills grow beyond small text files to include bundled assets or large payloads. The backend is designed so an OCI-backed storage implementation can be swapped in without changing the API contract.

### Web UI
A browser-based interface for exploring, searching, and managing skills. Built on a TypeScript client library auto-generated from the proto definitions via buf (ConnectRPC has first-class browser support). The backend already serves JSON over HTTP, so the web UI connects directly with no API gateway needed.

## Ideas - Not Yet Planned

Features that have been discussed but not committed to.

(None yet - add ideas here as they come up)
