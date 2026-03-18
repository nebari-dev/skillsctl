# Write Operations Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add PublishSkill and GetSkillContent RPCs to the backend, with ownership enforcement, version immutability, and content storage in SQLite.

**Architecture:** Extend the existing RegistryService with two new RPCs. Content stored as nullable BLOB in skill_versions. Ownership keyed on OIDC subject. Input validation in the service layer, persistence invariants in the repository layer.

**Tech Stack:** ConnectRPC, SQLite (modernc.org/sqlite), goose migrations, buf for proto codegen

**Spec:** `docs/superpowers/specs/2026-03-14-write-operations-design.md`

---

## Chunk 1: Proto, Migration, Repository Interface

### Task 1: Update proto definitions and regenerate code

**Files:**
- Modify: `proto/skillsctl/v1/registry.proto`
- Regenerate: `gen/go/skillsctl/v1/registry.pb.go`, `gen/go/skillsctl/v1/skillsctlv1connect/registry.connect.go`

- [ ] **Step 1: Add PublishSkill and GetSkillContent to registry.proto**

Add after the existing `GetSkill` RPC in `proto/skillsctl/v1/registry.proto`:

```protobuf
  // Write - requires valid OIDC token
  rpc PublishSkill(PublishSkillRequest) returns (PublishSkillResponse);

  // Read - requires valid OIDC token
  rpc GetSkillContent(GetSkillContentRequest) returns (GetSkillContentResponse);
```

Add new message definitions after `GetSkillResponse`:

```protobuf
message PublishSkillRequest {
  string name = 1;
  string version = 2;
  string description = 3;
  repeated string tags = 4;
  string changelog = 5;
  bytes content = 6;
}

message PublishSkillResponse {
  Skill skill = 1;
  SkillVersion version = 2;
}

message GetSkillContentRequest {
  string name = 1;
  string version = 2;
  string digest = 3;
}

message GetSkillContentResponse {
  bytes content = 1;
  SkillVersion version = 2;
}
```

- [ ] **Step 2: Lint and regenerate**

Run:
```bash
cd proto && buf lint
cd proto && buf generate
```

Expected: No lint errors. Generated files updated in `gen/go/skillsctl/v1/`.

- [ ] **Step 3: Verify the generated interface has the new methods**

Run:
```bash
grep -n 'PublishSkill\|GetSkillContent' gen/go/skillsctl/v1/skillsctlv1connect/registry.connect.go
```

Expected: Both methods appear in `RegistryServiceHandler` and `RegistryServiceClient` interfaces.

- [ ] **Step 4: Verify build still compiles**

Run: `go build ./...`

Expected: Compile error because `registry.Service` doesn't implement the new interface methods yet. This is expected and will be fixed in Task 5.

- [ ] **Step 5: Commit proto and generated code**

```bash
git add proto/skillsctl/v1/registry.proto gen/
git commit -m "proto: add PublishSkill and GetSkillContent RPCs"
```

---

### Task 2: Add database migration for content column

**Files:**
- Create: `backend/internal/store/sqlite/migrations/002_skill_content.sql`

- [ ] **Step 1: Create migration file**

Create `backend/internal/store/sqlite/migrations/002_skill_content.sql`:

```sql
-- +goose Up
ALTER TABLE skill_versions ADD COLUMN content BLOB;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35.0.
-- For modernc.org/sqlite (which tracks recent SQLite versions), DROP COLUMN works.
ALTER TABLE skill_versions DROP COLUMN content;
```

- [ ] **Step 2: Verify migration applies on top of 001**

Run: `go test ./backend/internal/store/sqlite/... -run TestRepository_ListSkills -v -race -count=1`

Expected: Existing tests still pass (migration runs in `openTestDB` via `migrations.Run`).

- [ ] **Step 3: Commit**

```bash
git add backend/internal/store/sqlite/migrations/002_skill_content.sql
git commit -m "migrate: add content BLOB column to skill_versions"
```

---

### Task 3: Extend Repository interface with new methods and errors

**Files:**
- Modify: `backend/internal/store/repository.go`

- [ ] **Step 1: Add sentinel errors and new methods**

Add to `backend/internal/store/repository.go`:

```go
var ErrAlreadyExists = errors.New("already exists")
var ErrPermissionDenied = errors.New("permission denied")
var ErrDigestMismatch = errors.New("digest mismatch")
```

Add to the `Repository` interface:

```go
CreateSkillVersion(ctx context.Context, skill *skillsctlv1.Skill, version *skillsctlv1.SkillVersion, content []byte) error
GetSkillContent(ctx context.Context, name string, version string, digest string) ([]byte, *skillsctlv1.SkillVersion, error)
```

- [ ] **Step 2: Verify build fails for unimplemented methods**

Run: `go build ./backend/...`

Expected: Compile errors in `memory.go` and `sqlite/sqlite.go` because they don't implement the new methods.

- [ ] **Step 3: Add stub implementations to Memory store**

Add to `backend/internal/store/memory.go`:

```go
func (m *Memory) CreateSkillVersion(_ context.Context, _ *skillsctlv1.Skill, _ *skillsctlv1.SkillVersion, _ []byte) error {
	return errors.New("not implemented")
}

func (m *Memory) GetSkillContent(_ context.Context, _ string, _ string, _ string) ([]byte, *skillsctlv1.SkillVersion, error) {
	return nil, nil, errors.New("not implemented")
}
```

Add `"errors"` to the import block.

- [ ] **Step 4: Add stub implementations to SQLite store**

Add to `backend/internal/store/sqlite/sqlite.go`:

```go
func (r *Repository) CreateSkillVersion(_ context.Context, _ *skillsctlv1.Skill, _ *skillsctlv1.SkillVersion, _ []byte) error {
	return errors.New("not implemented")
}

func (r *Repository) GetSkillContent(_ context.Context, _ string, _ string, _ string) ([]byte, *skillsctlv1.SkillVersion, error) {
	return nil, nil, errors.New("not implemented")
}
```

- [ ] **Step 5: Add golang.org/x/mod dependency**

Run: `go get golang.org/x/mod@latest`

This is used by validation (semver), Memory store, and SQLite store for semver comparison.

- [ ] **Step 6: Add stub implementations to registry Service**

Add to `backend/internal/registry/service.go`:

```go
func (s *Service) PublishSkill(_ context.Context, _ *connect.Request[skillsctlv1.PublishSkillRequest]) (*connect.Response[skillsctlv1.PublishSkillResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *Service) GetSkillContent(_ context.Context, _ *connect.Request[skillsctlv1.GetSkillContentRequest]) (*connect.Response[skillsctlv1.GetSkillContentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}
```

- [ ] **Step 7: Verify everything compiles and existing tests pass**

Run: `go build ./... && go test ./... -race -count=1`

Expected: All compile. All existing tests pass.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/store/repository.go backend/internal/store/memory.go backend/internal/store/sqlite/sqlite.go backend/internal/registry/service.go go.mod go.sum
git commit -m "feat: extend Repository interface with write operation stubs"
```

---

## Chunk 2: Input Validation

### Task 4: Add input validation package

**Files:**
- Create: `backend/internal/registry/validate.go`
- Create: `backend/internal/registry/validate_test.go`

- [ ] **Step 1: Write failing tests for name validation**

Create `backend/internal/registry/validate_test.go`:

```go
package registry

import (
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid short", input: "go", wantErr: false},
		{name: "valid with hyphens", input: "my-skill", wantErr: false},
		{name: "valid 64 chars", input: strings.Repeat("a", 64), wantErr: false},
		{name: "too short", input: "a", wantErr: true},
		{name: "too long", input: strings.Repeat("a", 65), wantErr: true},
		{name: "uppercase", input: "MySkill", wantErr: true},
		{name: "starts with hyphen", input: "-skill", wantErr: true},
		{name: "ends with hyphen", input: "skill-", wantErr: true},
		{name: "contains underscore", input: "my_skill", wantErr: true},
		{name: "contains space", input: "my skill", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "1.2.3", wantErr: false},
		{name: "valid with v prefix", input: "v1.2.3", wantErr: false},
		{name: "valid prerelease", input: "1.0.0-beta.1", wantErr: false},
		{name: "valid with build", input: "1.0.0+build.123", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "not semver", input: "1.2", wantErr: true},
		{name: "garbage", input: "not-a-version", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTags(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		wantErr bool
	}{
		{name: "valid", tags: []string{"go", "testing"}, wantErr: false},
		{name: "empty list", tags: nil, wantErr: false},
		{name: "too many", tags: make([]string, 21), wantErr: true},
		{name: "tag too long", tags: []string{strings.Repeat("a", 65)}, wantErr: true},
		{name: "uppercase tag", tags: []string{"Go"}, wantErr: true},
		{name: "tag with spaces", tags: []string{"my tag"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill the "too many" test case with valid tag values
			if tt.name == "too many" {
				for i := range tt.tags {
					tt.tags[i] = "tag"
				}
			}
			err := validateTags(tt.tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTags(%v) error = %v, wantErr %v", tt.tags, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePublishRequest(t *testing.T) {
	validContent := []byte("# My Skill\nDoes stuff")

	tests := []struct {
		name        string
		skillName   string
		version     string
		description string
		tags        []string
		content     []byte
		wantErr     bool
	}{
		{
			name:        "valid",
			skillName:   "my-skill",
			version:     "1.0.0",
			description: "A useful skill",
			tags:        []string{"go"},
			content:     validContent,
			wantErr:     false,
		},
		{
			name:        "empty content",
			skillName:   "my-skill",
			version:     "1.0.0",
			description: "A useful skill",
			content:     nil,
			wantErr:     true,
		},
		{
			name:        "content too large",
			skillName:   "my-skill",
			version:     "1.0.0",
			description: "A useful skill",
			content:     make([]byte, 1024*1024+1),
			wantErr:     true,
		},
		{
			name:        "empty description",
			skillName:   "my-skill",
			version:     "1.0.0",
			description: "",
			content:     validContent,
			wantErr:     true,
		},
		{
			name:        "description too long",
			skillName:   "my-skill",
			version:     "1.0.0",
			description: strings.Repeat("a", 2001),
			content:     validContent,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePublishRequest(tt.skillName, tt.version, tt.description, tt.tags, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePublishRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backend/internal/registry/... -run TestValidate -v -race -count=1`

Expected: FAIL - functions not defined.

- [ ] **Step 3: Implement validation functions**

Create `backend/internal/registry/validate.go`:

```go
package registry

import (
	"fmt"
	"regexp"

	"golang.org/x/mod/semver"
)

const (
	maxNameLen        = 64
	minNameLen        = 2
	maxDescriptionLen = 2000
	maxTagLen         = 64
	maxTags           = 20
	maxContentBytes   = 1024 * 1024 // 1MB
)

var nameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func validateName(name string) error {
	if len(name) < minNameLen || len(name) > maxNameLen {
		return fmt.Errorf("name must be between %d and %d characters", minNameLen, maxNameLen)
	}
	if !nameRegexp.MatchString(name) {
		return fmt.Errorf("name must be lowercase alphanumeric with hyphens, cannot start or end with a hyphen")
	}
	return nil
}

func validateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version is required")
	}
	// semver.IsValid requires a "v" prefix
	v := version
	if v[0] != 'v' {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return fmt.Errorf("version %q is not valid semver", version)
	}
	return nil
}

var tagRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func validateTags(tags []string) error {
	if len(tags) > maxTags {
		return fmt.Errorf("too many tags (max %d)", maxTags)
	}
	for _, tag := range tags {
		if len(tag) > maxTagLen {
			return fmt.Errorf("tag %q exceeds max length of %d", tag, maxTagLen)
		}
		if len(tag) < 2 || !tagRegexp.MatchString(tag) {
			return fmt.Errorf("tag %q must be 2+ lowercase alphanumeric characters with hyphens", tag)
		}
	}
	return nil
}

func validatePublishRequest(name, version, description string, tags []string, content []byte) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := validateVersion(version); err != nil {
		return err
	}
	if description == "" {
		return fmt.Errorf("description is required")
	}
	if len(description) > maxDescriptionLen {
		return fmt.Errorf("description exceeds max length of %d", maxDescriptionLen)
	}
	if err := validateTags(tags); err != nil {
		return err
	}
	if len(content) == 0 {
		return fmt.Errorf("content is required")
	}
	if len(content) > maxContentBytes {
		return fmt.Errorf("content exceeds max size of %d bytes", maxContentBytes)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backend/internal/registry/... -run TestValidate -v -race -count=1`

Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/registry/validate.go backend/internal/registry/validate_test.go
git commit -m "feat: add input validation for publish requests"
```

---

## Chunk 3: Memory Store Write Implementation

### Task 5: Implement CreateSkillVersion and GetSkillContent in Memory store

**Files:**
- Modify: `backend/internal/store/memory.go`
- Modify: `backend/internal/store/memory_test.go`

- [ ] **Step 1: Write failing tests for CreateSkillVersion**

Add to `backend/internal/store/memory_test.go`:

```go
func TestMemoryStore_CreateSkillVersion(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*testing.T, *store.Memory)
		skill     *skillsctlv1.Skill
		version   *skillsctlv1.SkillVersion
		content   []byte
		wantErr   error
	}{
		{
			name:  "first publish creates skill",
			setup: func(t *testing.T, m *store.Memory) {},
			skill: &skillsctlv1.Skill{
				Name:        "new-skill",
				Description: "A new skill",
				Owner:       "user-123",
				Tags:        []string{"go"},
			},
			version: &skillsctlv1.SkillVersion{
				Version:     "1.0.0",
				PublishedBy: "user@example.com",
				Digest:      "sha256:abc",
				SizeBytes:   100,
			},
			content: []byte("skill content"),
		},
		{
			name: "second version by same owner",
			setup: func(t *testing.T, m *store.Memory) {
				t.Helper()
				err := m.CreateSkillVersion(context.Background(),
					&skillsctlv1.Skill{Name: "existing", Description: "test", Owner: "user-123", Tags: []string{}},
					&skillsctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 5},
					[]byte("v1"),
				)
				if err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			skill: &skillsctlv1.Skill{
				Name:  "existing",
				Owner: "user-123",
			},
			version: &skillsctlv1.SkillVersion{
				Version:     "2.0.0",
				PublishedBy: "u@ex.com",
				Digest:      "sha256:b",
				SizeBytes:   5,
			},
			content: []byte("v2"),
		},
		{
			name: "different owner rejected",
			setup: func(t *testing.T, m *store.Memory) {
				t.Helper()
				err := m.CreateSkillVersion(context.Background(),
					&skillsctlv1.Skill{Name: "owned", Description: "test", Owner: "user-123", Tags: []string{}},
					&skillsctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 5},
					[]byte("v1"),
				)
				if err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			skill:   &skillsctlv1.Skill{Name: "owned", Owner: "other-user"},
			version: &skillsctlv1.SkillVersion{Version: "2.0.0", Digest: "sha256:b", SizeBytes: 5},
			content: []byte("v2"),
			wantErr: store.ErrPermissionDenied,
		},
		{
			name: "duplicate version rejected",
			setup: func(t *testing.T, m *store.Memory) {
				t.Helper()
				err := m.CreateSkillVersion(context.Background(),
					&skillsctlv1.Skill{Name: "dup", Description: "test", Owner: "user-123", Tags: []string{}},
					&skillsctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 5},
					[]byte("v1"),
				)
				if err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			skill:   &skillsctlv1.Skill{Name: "dup", Owner: "user-123"},
			version: &skillsctlv1.SkillVersion{Version: "1.0.0", Digest: "sha256:b", SizeBytes: 5},
			content: []byte("v1-again"),
			wantErr: store.ErrAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := store.NewMemory(nil)
			tt.setup(t, m)
			err := m.CreateSkillVersion(context.Background(), tt.skill, tt.version, tt.content)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestMemoryStore_GetSkillContent(t *testing.T) {
	setup := func() *store.Memory {
		m := store.NewMemory(nil)
		m.CreateSkillVersion(context.Background(),
			&skillsctlv1.Skill{Name: "my-skill", Description: "test", Owner: "user-123", Tags: []string{}, LatestVersion: "1.0.0"},
			&skillsctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:abc", SizeBytes: 13},
			[]byte("skill content"),
		)
		return m
	}

	tests := []struct {
		name        string
		skillName   string
		version     string
		digest      string
		wantContent []byte
		wantErr     error
	}{
		{
			name:        "get by version",
			skillName:   "my-skill",
			version:     "1.0.0",
			wantContent: []byte("skill content"),
		},
		{
			name:        "get latest when version empty",
			skillName:   "my-skill",
			version:     "",
			wantContent: []byte("skill content"),
		},
		{
			name:      "nonexistent skill",
			skillName: "nope",
			version:   "1.0.0",
			wantErr:   store.ErrNotFound,
		},
		{
			name:      "nonexistent version",
			skillName: "my-skill",
			version:   "9.9.9",
			wantErr:   store.ErrNotFound,
		},
		{
			name:        "matching digest",
			skillName:   "my-skill",
			version:     "1.0.0",
			digest:      "sha256:abc",
			wantContent: []byte("skill content"),
		},
		{
			name:      "mismatched digest",
			skillName: "my-skill",
			version:   "1.0.0",
			digest:    "sha256:wrong",
			wantErr:   store.ErrDigestMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := setup()
			content, ver, err := m.GetSkillContent(context.Background(), tt.skillName, tt.version, tt.digest)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(content) != string(tt.wantContent) {
				t.Errorf("expected content %q, got %q", tt.wantContent, content)
			}
			if ver == nil {
				t.Fatal("expected version metadata, got nil")
			}
		})
	}
}

func TestMemoryStore_CreateSkillVersion_SemverLatest(t *testing.T) {
	m := store.NewMemory(nil)

	// Publish 2.0.0 first
	m.CreateSkillVersion(context.Background(),
		&skillsctlv1.Skill{Name: "sv", Description: "test", Owner: "u", Tags: []string{}},
		&skillsctlv1.SkillVersion{Version: "2.0.0", Digest: "sha256:a", SizeBytes: 2},
		[]byte("v2"),
	)

	// Publish 1.0.0 after - latest should stay at 2.0.0
	m.CreateSkillVersion(context.Background(),
		&skillsctlv1.Skill{Name: "sv", Owner: "u"},
		&skillsctlv1.SkillVersion{Version: "1.0.0", Digest: "sha256:b", SizeBytes: 2},
		[]byte("v1"),
	)

	skill, _, err := m.GetSkill(context.Background(), "sv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.LatestVersion != "2.0.0" {
		t.Errorf("expected latest_version 2.0.0, got %s", skill.LatestVersion)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backend/internal/store/... -run "TestMemoryStore_Create|TestMemoryStore_GetSkillContent|TestMemoryStore_CreateSkillVersion_Semver" -v -race -count=1`

Expected: FAIL - stubs return "not implemented".

- [ ] **Step 3: Implement Memory store write methods**

Replace the stub methods in `backend/internal/store/memory.go` with full implementations. The Memory store needs internal storage for versions and content. Update the struct:

```go
package store

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/mod/semver"

	skillsctlv1 "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1"
)

type memoryVersion struct {
	meta    *skillsctlv1.SkillVersion
	content []byte
}

// Memory is an in-memory Repository for local development and testing.
type Memory struct {
	mu       sync.Mutex
	skills   []*skillsctlv1.Skill
	versions map[string][]memoryVersion // keyed by skill name
}

var _ Repository = (*Memory)(nil)

// NewMemory creates an in-memory store pre-populated with the given skills.
func NewMemory(skills []*skillsctlv1.Skill) *Memory {
	if skills == nil {
		skills = []*skillsctlv1.Skill{}
	}
	return &Memory{
		skills:   skills,
		versions: make(map[string][]memoryVersion),
	}
}
```

Keep the existing `ListSkills`, `GetSkill`, and `hasAnyTag` unchanged.

Add `CreateSkillVersion`:

```go
func (m *Memory) CreateSkillVersion(_ context.Context, skill *skillsctlv1.Skill, version *skillsctlv1.SkillVersion, content []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if skill exists
	var existing *skillsctlv1.Skill
	for _, s := range m.skills {
		if s.Name == skill.Name {
			existing = s
			break
		}
	}

	if existing != nil {
		// Ownership check
		if existing.Owner != skill.Owner {
			return fmt.Errorf("%w: skill %q is owned by %q", ErrPermissionDenied, skill.Name, existing.Owner)
		}
		// Immutability check
		for _, v := range m.versions[skill.Name] {
			if v.meta.Version == version.Version {
				return fmt.Errorf("%w: version %s of %s", ErrAlreadyExists, version.Version, skill.Name)
			}
		}
		// Update latest_version only if new version is semver-greater
		if compareSemver(version.Version, existing.LatestVersion) > 0 {
			existing.LatestVersion = version.Version
		}
		existing.Description = skill.Description
		if len(skill.Tags) > 0 {
			existing.Tags = skill.Tags
		}
	} else {
		// First publish
		newSkill := &skillsctlv1.Skill{
			Name:          skill.Name,
			Description:   skill.Description,
			Owner:         skill.Owner,
			Tags:          skill.Tags,
			LatestVersion: version.Version,
			Source:        skillsctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
		}
		m.skills = append(m.skills, newSkill)
	}

	m.versions[skill.Name] = append(m.versions[skill.Name], memoryVersion{
		meta:    version,
		content: content,
	})

	return nil
}
```

Add `GetSkillContent`:

```go
func (m *Memory) GetSkillContent(_ context.Context, name string, version string, digest string) ([]byte, *skillsctlv1.SkillVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Resolve version
	if version == "" {
		for _, s := range m.skills {
			if s.Name == name {
				version = s.LatestVersion
				break
			}
		}
		if version == "" {
			return nil, nil, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
	}

	versions, ok := m.versions[name]
	if !ok {
		return nil, nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	for _, v := range versions {
		if v.meta.Version == version {
			if v.content == nil {
				return nil, nil, fmt.Errorf("%w: no content for %s@%s", ErrNotFound, name, version)
			}
			if digest != "" && v.meta.Digest != digest {
				return nil, nil, fmt.Errorf("%w: expected %s, got %s", ErrDigestMismatch, digest, v.meta.Digest)
			}
			return v.content, v.meta, nil
		}
	}

	return nil, nil, fmt.Errorf("%w: %s@%s", ErrNotFound, name, version)
}
```

Add the semver comparison helper (package-level, unexported):

```go
// compareSemver compares two version strings as semver.
// Returns >0 if a > b, <0 if a < b, 0 if equal.
// Adds "v" prefix if missing (semver package requires it).
func compareSemver(a, b string) int {
	if a != "" && a[0] != 'v' {
		a = "v" + a
	}
	if b != "" && b[0] != 'v' {
		b = "v" + b
	}
	return semver.Compare(a, b)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./backend/internal/store/... -v -race -count=1`

Expected: All tests PASS including new ones.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/store/memory.go backend/internal/store/memory_test.go
git commit -m "feat: implement write operations in Memory store"
```

---

## Chunk 4: SQLite Store Write Implementation

### Task 6: Implement CreateSkillVersion and GetSkillContent in SQLite store

**Files:**
- Modify: `backend/internal/store/sqlite/sqlite.go`
- Modify: `backend/internal/store/sqlite/sqlite_test.go`

- [ ] **Step 1: Write failing tests for SQLite CreateSkillVersion**

Add to `backend/internal/store/sqlite/sqlite_test.go`:

```go
func TestRepository_CreateSkillVersion(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T, *sql.DB)
		skill   *skillsctlv1.Skill
		version *skillsctlv1.SkillVersion
		content []byte
		wantErr error
	}{
		{
			name:  "first publish creates skill and version",
			setup: func(t *testing.T, db *sql.DB) {},
			skill: &skillsctlv1.Skill{
				Name:        "new-skill",
				Description: "A new skill",
				Owner:       "user-123",
				Tags:        []string{"go", "testing"},
			},
			version: &skillsctlv1.SkillVersion{
				Version:     "1.0.0",
				PublishedBy: "user@example.com",
				Digest:      "sha256:abc123",
				SizeBytes:   100,
				Changelog:   "Initial release",
			},
			content: []byte("# My Skill\nDoes stuff"),
		},
		{
			name: "second version by same owner succeeds",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				repo := sqlitestore.New(db)
				err := repo.CreateSkillVersion(context.Background(),
					&skillsctlv1.Skill{Name: "existing", Description: "test", Owner: "user-123", Tags: []string{}},
					&skillsctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 5},
					[]byte("v1"),
				)
				if err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			skill: &skillsctlv1.Skill{
				Name:        "existing",
				Description: "updated desc",
				Owner:       "user-123",
				Tags:        []string{"updated"},
			},
			version: &skillsctlv1.SkillVersion{
				Version:     "2.0.0",
				PublishedBy: "u@ex.com",
				Digest:      "sha256:b",
				SizeBytes:   5,
			},
			content: []byte("v2"),
		},
		{
			name: "different owner rejected",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				repo := sqlitestore.New(db)
				repo.CreateSkillVersion(context.Background(),
					&skillsctlv1.Skill{Name: "owned", Description: "test", Owner: "user-123", Tags: []string{}},
					&skillsctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 5},
					[]byte("v1"),
				)
			},
			skill:   &skillsctlv1.Skill{Name: "owned", Owner: "other-user"},
			version: &skillsctlv1.SkillVersion{Version: "2.0.0", Digest: "sha256:b", SizeBytes: 5},
			content: []byte("v2"),
			wantErr: store.ErrPermissionDenied,
		},
		{
			name: "duplicate version rejected",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				repo := sqlitestore.New(db)
				repo.CreateSkillVersion(context.Background(),
					&skillsctlv1.Skill{Name: "dup", Description: "test", Owner: "user-123", Tags: []string{}},
					&skillsctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 5},
					[]byte("v1"),
				)
			},
			skill:   &skillsctlv1.Skill{Name: "dup", Owner: "user-123"},
			version: &skillsctlv1.SkillVersion{Version: "1.0.0", Digest: "sha256:b", SizeBytes: 5},
			content: []byte("v1-again"),
			wantErr: store.ErrAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDB(t)
			tt.setup(t, db)
			repo := sqlitestore.New(db)
			err := repo.CreateSkillVersion(context.Background(), tt.skill, tt.version, tt.content)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the skill was created/updated
			skill, versions, err := repo.GetSkill(context.Background(), tt.skill.Name)
			if err != nil {
				t.Fatalf("GetSkill after create: %v", err)
			}
			if skill.Name != tt.skill.Name {
				t.Errorf("expected name %q, got %q", tt.skill.Name, skill.Name)
			}
			// Verify at least one version exists
			found := false
			for _, v := range versions {
				if v.Version == tt.version.Version {
					found = true
					if v.Digest != tt.version.Digest {
						t.Errorf("expected digest %q, got %q", tt.version.Digest, v.Digest)
					}
				}
			}
			if !found {
				t.Errorf("version %s not found after create", tt.version.Version)
			}
		})
	}
}

func TestRepository_CreateSkillVersion_SemverLatest(t *testing.T) {
	db := openTestDB(t)
	repo := sqlitestore.New(db)

	// Publish 2.0.0 first
	err := repo.CreateSkillVersion(context.Background(),
		&skillsctlv1.Skill{Name: "sv", Description: "test", Owner: "u", Tags: []string{}},
		&skillsctlv1.SkillVersion{Version: "2.0.0", PublishedBy: "u@ex.com", Digest: "sha256:a", SizeBytes: 2},
		[]byte("v2"),
	)
	if err != nil {
		t.Fatalf("publish 2.0.0: %v", err)
	}

	// Publish 1.0.0 after - latest should stay at 2.0.0
	err = repo.CreateSkillVersion(context.Background(),
		&skillsctlv1.Skill{Name: "sv", Owner: "u"},
		&skillsctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:b", SizeBytes: 2},
		[]byte("v1"),
	)
	if err != nil {
		t.Fatalf("publish 1.0.0: %v", err)
	}

	skill, _, err := repo.GetSkill(context.Background(), "sv")
	if err != nil {
		t.Fatalf("get skill: %v", err)
	}
	if skill.LatestVersion != "2.0.0" {
		t.Errorf("expected latest_version 2.0.0, got %s", skill.LatestVersion)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backend/internal/store/sqlite/... -run "TestRepository_CreateSkillVersion" -v -race -count=1`

Expected: FAIL - stubs return "not implemented".

- [ ] **Step 3: Implement SQLite CreateSkillVersion**

Replace the stub `CreateSkillVersion` in `backend/internal/store/sqlite/sqlite.go`:

```go
func (r *Repository) CreateSkillVersion(ctx context.Context, skill *skillsctlv1.Skill, version *skillsctlv1.SkillVersion, content []byte) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Check if skill exists
	var existingOwner string
	var existingLatest string
	err = tx.QueryRowContext(ctx, "SELECT owner, latest_version FROM skills WHERE name = ?", skill.Name).Scan(&existingOwner, &existingLatest)

	if errors.Is(err, sql.ErrNoRows) {
		// First publish - create skill
		tagsJSON, jsonErr := json.Marshal(skill.Tags)
		if jsonErr != nil {
			return fmt.Errorf("marshal tags: %w", jsonErr)
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO skills (name, description, owner, tags, latest_version, source) VALUES (?, ?, ?, ?, ?, ?)`,
			skill.Name, skill.Description, skill.Owner, string(tagsJSON), version.Version,
			int(skillsctlv1.SkillSource_SKILL_SOURCE_INTERNAL),
		)
		if err != nil {
			return fmt.Errorf("insert skill: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check skill: %w", err)
	} else {
		// Existing skill - check ownership
		if existingOwner != skill.Owner {
			return fmt.Errorf("%w: skill %q is owned by %q", store.ErrPermissionDenied, skill.Name, existingOwner)
		}
		// Update latest_version only if semver-greater
		if compareSemver(version.Version, existingLatest) > 0 {
			_, err = tx.ExecContext(ctx,
				`UPDATE skills SET latest_version = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE name = ?`,
				version.Version, skill.Name,
			)
			if err != nil {
				return fmt.Errorf("update latest_version: %w", err)
			}
		}
		// Update description and tags if provided
		if skill.Description != "" {
			_, err = tx.ExecContext(ctx, `UPDATE skills SET description = ? WHERE name = ?`, skill.Description, skill.Name)
			if err != nil {
				return fmt.Errorf("update description: %w", err)
			}
		}
		if len(skill.Tags) > 0 {
			tagsJSON, jsonErr := json.Marshal(skill.Tags)
			if jsonErr != nil {
				return fmt.Errorf("marshal tags: %w", jsonErr)
			}
			_, err = tx.ExecContext(ctx, `UPDATE skills SET tags = ? WHERE name = ?`, string(tagsJSON), skill.Name)
			if err != nil {
				return fmt.Errorf("update tags: %w", err)
			}
		}
	}

	// Insert version - PRIMARY KEY constraint enforces immutability
	_, err = tx.ExecContext(ctx,
		`INSERT INTO skill_versions (skill_name, version, published_by, changelog, digest, size_bytes, content) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		skill.Name, version.Version, version.PublishedBy, version.Changelog, version.Digest, version.SizeBytes, content,
	)
	if err != nil {
		// Check for unique constraint violation (version already exists)
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: version %s of %s", store.ErrAlreadyExists, version.Version, skill.Name)
		}
		return fmt.Errorf("insert version: %w", err)
	}

	return tx.Commit()
}

// isUniqueViolation checks if a SQLite error is a UNIQUE constraint violation.
// modernc.org/sqlite returns UNIQUE violations as plain errors with this message;
// no typed error code is exposed via database/sql.
func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
```

Add the `compareSemver` helper (same as Memory store version):

```go
func compareSemver(a, b string) int {
	if a != "" && a[0] != 'v' {
		a = "v" + a
	}
	if b != "" && b[0] != 'v' {
		b = "v" + b
	}
	return semver.Compare(a, b)
}
```

Add `"golang.org/x/mod/semver"` to imports.

- [ ] **Step 4: Run CreateSkillVersion tests**

Run: `go test ./backend/internal/store/sqlite/... -run "TestRepository_CreateSkillVersion" -v -race -count=1`

Expected: All PASS.

- [ ] **Step 5: Write failing tests for SQLite GetSkillContent**

Add to `backend/internal/store/sqlite/sqlite_test.go`:

```go
func TestRepository_GetSkillContent(t *testing.T) {
	setup := func(t *testing.T) (*sql.DB, *sqlitestore.Repository) {
		t.Helper()
		db := openTestDB(t)
		repo := sqlitestore.New(db)
		err := repo.CreateSkillVersion(context.Background(),
			&skillsctlv1.Skill{Name: "my-skill", Description: "test", Owner: "user-123", Tags: []string{"go"}},
			&skillsctlv1.SkillVersion{Version: "1.0.0", PublishedBy: "u@ex.com", Digest: "sha256:abc", SizeBytes: 13, Changelog: "Initial"},
			[]byte("skill content"),
		)
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		return db, repo
	}

	tests := []struct {
		name        string
		skillName   string
		version     string
		digest      string
		wantContent string
		wantErr     error
	}{
		{
			name:        "get by version",
			skillName:   "my-skill",
			version:     "1.0.0",
			wantContent: "skill content",
		},
		{
			name:        "get latest when version empty",
			skillName:   "my-skill",
			version:     "",
			wantContent: "skill content",
		},
		{
			name:      "nonexistent skill",
			skillName: "nope",
			version:   "1.0.0",
			wantErr:   store.ErrNotFound,
		},
		{
			name:      "nonexistent version",
			skillName: "my-skill",
			version:   "9.9.9",
			wantErr:   store.ErrNotFound,
		},
		{
			name:        "matching digest passes",
			skillName:   "my-skill",
			version:     "1.0.0",
			digest:      "sha256:abc",
			wantContent: "skill content",
		},
		{
			name:      "mismatched digest rejected",
			skillName: "my-skill",
			version:   "1.0.0",
			digest:    "sha256:wrong",
			wantErr:   store.ErrDigestMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, repo := setup(t)
			content, ver, err := repo.GetSkillContent(context.Background(), tt.skillName, tt.version, tt.digest)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(content) != tt.wantContent {
				t.Errorf("expected content %q, got %q", tt.wantContent, content)
			}
			if ver == nil {
				t.Fatal("expected version metadata, got nil")
			}
			if ver.Version != "1.0.0" {
				t.Errorf("expected version 1.0.0, got %s", ver.Version)
			}
		})
	}
}

func TestRepository_GetSkillContent_NullContent(t *testing.T) {
	db := openTestDB(t)
	// Insert a skill with no content (federated skill scenario)
	seedSkills(t, db)
	_, err := db.Exec(
		`INSERT INTO skill_versions (skill_name, version, digest, published_by) VALUES (?, ?, ?, ?)`,
		"data-pipeline", "1.3.0", "sha256:abc", "user@example.com",
	)
	if err != nil {
		t.Fatalf("insert version: %v", err)
	}

	repo := sqlitestore.New(db)
	_, _, err = repo.GetSkillContent(context.Background(), "data-pipeline", "1.3.0", "")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for null content, got %v", err)
	}
}
```

- [ ] **Step 6: Run GetSkillContent tests to verify they fail**

Run: `go test ./backend/internal/store/sqlite/... -run "TestRepository_GetSkillContent" -v -race -count=1`

Expected: FAIL - stub returns "not implemented".

- [ ] **Step 7: Implement SQLite GetSkillContent**

Replace the stub `GetSkillContent` in `backend/internal/store/sqlite/sqlite.go`:

```go
func (r *Repository) GetSkillContent(ctx context.Context, name string, version string, digest string) ([]byte, *skillsctlv1.SkillVersion, error) {
	// Resolve version if empty
	if version == "" {
		var latest string
		err := r.db.QueryRowContext(ctx, "SELECT latest_version FROM skills WHERE name = ?", name).Scan(&latest)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("%w: %s", store.ErrNotFound, name)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("resolve latest version: %w", err)
		}
		version = latest
	}

	var (
		v           skillsctlv1.SkillVersion
		content     []byte
		publishedAt string
		draft       int
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT version, changelog, oci_ref, digest, size_bytes, published_by, published_at, draft, content
		 FROM skill_versions WHERE skill_name = ? AND version = ?`,
		name, version,
	).Scan(&v.Version, &v.Changelog, &v.OciRef, &v.Digest, &v.SizeBytes, &v.PublishedBy, &publishedAt, &draft, &content)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("%w: %s@%s", store.ErrNotFound, name, version)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get skill content: %w", err)
	}

	v.Draft = draft != 0
	if publishedAt != "" {
		t, err := parseTimestamp(publishedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("parse published_at: %w", err)
		}
		v.PublishedAt = t
	}

	if content == nil {
		return nil, nil, fmt.Errorf("%w: no content for %s@%s", store.ErrNotFound, name, version)
	}

	if digest != "" && v.Digest != digest {
		return nil, nil, fmt.Errorf("%w: expected %s, got %s", store.ErrDigestMismatch, digest, v.Digest)
	}

	return content, &v, nil
}
```

- [ ] **Step 8: Run all SQLite tests**

Run: `go test ./backend/internal/store/sqlite/... -v -race -count=1`

Expected: All PASS.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/store/sqlite/sqlite.go backend/internal/store/sqlite/sqlite_test.go
git commit -m "feat: implement write operations in SQLite store"
```

---

## Chunk 5: Service Layer and Server Wiring

### Task 7: Implement PublishSkill and GetSkillContent handlers

**Files:**
- Modify: `backend/internal/registry/service.go`
- Modify: `backend/internal/registry/service_test.go`

- [ ] **Step 1: Write failing tests for PublishSkill handler**

Add to `backend/internal/registry/service_test.go`. Note: these tests need to inject auth claims into the context. Import `auth` package and create a helper:

```go
import (
	"github.com/nebari-dev/skillsctl/backend/internal/auth"
)

func newTestClientWithAuth(t *testing.T) skillsctlv1connect.RegistryServiceClient {
	t.Helper()
	svc := registry.NewService(store.NewMemory(nil))
	interceptor := auth.NewInterceptor(nil) // nil validator = dev mode
	mux := http.NewServeMux()
	path, handler := skillsctlv1connect.NewRegistryServiceHandler(svc, connect.WithInterceptors(interceptor))
	mux.Handle(path, handler)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return skillsctlv1connect.NewRegistryServiceClient(http.DefaultClient, ts.URL)
}

func TestRegistryService_PublishSkill(t *testing.T) {
	tests := []struct {
		name     string
		req      *skillsctlv1.PublishSkillRequest
		wantCode connect.Code
	}{
		{
			name: "valid publish",
			req: &skillsctlv1.PublishSkillRequest{
				Name:        "my-skill",
				Version:     "1.0.0",
				Description: "A useful skill",
				Tags:        []string{"go", "testing"},
				Content:     []byte("# My Skill"),
			},
		},
		{
			name: "invalid name",
			req: &skillsctlv1.PublishSkillRequest{
				Name:        "A",
				Version:     "1.0.0",
				Description: "desc",
				Content:     []byte("content"),
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name: "invalid version",
			req: &skillsctlv1.PublishSkillRequest{
				Name:        "my-skill",
				Version:     "not-semver",
				Description: "desc",
				Content:     []byte("content"),
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name: "empty content",
			req: &skillsctlv1.PublishSkillRequest{
				Name:        "my-skill",
				Version:     "1.0.0",
				Description: "desc",
				Content:     nil,
			},
			wantCode: connect.CodeInvalidArgument,
		},
		{
			name: "empty description",
			req: &skillsctlv1.PublishSkillRequest{
				Name:        "my-skill",
				Version:     "1.0.0",
				Description: "",
				Content:     []byte("content"),
			},
			wantCode: connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// PublishSkill requires auth claims in context.
			// With nil validator, the interceptor passes through without claims.
			// The handler must return CodeUnauthenticated.
			// For the "valid" case, we need to set up a test with claims.
			// This is tested via direct service call with injected context.
			svc := registry.NewService(store.NewMemory(nil))
			ctx := auth.WithClaims(context.Background(), &auth.Claims{
				Subject: "user-123",
				Email:   "user@example.com",
			})
			resp, err := svc.PublishSkill(ctx, connect.NewRequest(tt.req))
			if tt.wantCode != 0 {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if connectErr := new(connect.Error); errors.As(err, &connectErr) {
					if connectErr.Code() != tt.wantCode {
						t.Errorf("expected code %v, got %v: %s", tt.wantCode, connectErr.Code(), connectErr.Message())
					}
				} else {
					t.Errorf("expected connect.Error, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Msg.Skill.Name != tt.req.Name {
				t.Errorf("expected skill name %q, got %q", tt.req.Name, resp.Msg.Skill.Name)
			}
			if resp.Msg.Version.Version != tt.req.Version {
				t.Errorf("expected version %q, got %q", tt.req.Version, resp.Msg.Version.Version)
			}
			if resp.Msg.Version.Digest == "" {
				t.Error("expected non-empty digest")
			}
		})
	}
}

func TestRegistryService_PublishSkill_Unauthenticated(t *testing.T) {
	svc := registry.NewService(store.NewMemory(nil))
	// No claims in context
	_, err := svc.PublishSkill(context.Background(), connect.NewRequest(&skillsctlv1.PublishSkillRequest{
		Name:        "my-skill",
		Version:     "1.0.0",
		Description: "desc",
		Content:     []byte("content"),
	}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		if connectErr.Code() != connect.CodeUnauthenticated {
			t.Errorf("expected CodeUnauthenticated, got %v", connectErr.Code())
		}
	}
}
```

Add `"errors"` to imports if not already present.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./backend/internal/registry/... -run "TestRegistryService_Publish" -v -race -count=1`

Expected: FAIL - stub returns "not implemented".

- [ ] **Step 3: Implement PublishSkill handler**

Replace the stub `PublishSkill` in `backend/internal/registry/service.go`:

```go
func (s *Service) PublishSkill(ctx context.Context, req *connect.Request[skillsctlv1.PublishSkillRequest]) (*connect.Response[skillsctlv1.PublishSkillResponse], error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required to publish"))
	}

	msg := req.Msg
	if err := validatePublishRequest(msg.Name, msg.Version, msg.Description, msg.Tags, msg.Content); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	digest := computeDigest(msg.Content)

	skill := &skillsctlv1.Skill{
		Name:        msg.Name,
		Description: msg.Description,
		Owner:       claims.Subject,
		Tags:        msg.Tags,
		Source:      skillsctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
	}

	ver := &skillsctlv1.SkillVersion{
		Version:     msg.Version,
		PublishedBy: claims.Email,
		Changelog:   msg.Changelog,
		Digest:      digest,
		SizeBytes:   int64(len(msg.Content)),
	}

	if err := s.store.CreateSkillVersion(ctx, skill, ver, msg.Content); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		if errors.Is(err, store.ErrPermissionDenied) {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Re-fetch the skill to get the full state (timestamps, install_count, etc.)
	updatedSkill, _, err := s.store.GetSkill(ctx, msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&skillsctlv1.PublishSkillResponse{
		Skill:   updatedSkill,
		Version: ver,
	}), nil
}
```

Add the `computeDigest` helper and import:

```go
import (
	"crypto/sha256"
	"fmt"

	"github.com/nebari-dev/skillsctl/backend/internal/auth"
)

func computeDigest(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("sha256:%x", h)
}
```

- [ ] **Step 4: Run PublishSkill tests**

Run: `go test ./backend/internal/registry/... -run "TestRegistryService_Publish" -v -race -count=1`

Expected: All PASS.

- [ ] **Step 5: Write failing tests for GetSkillContent handler**

Add to `backend/internal/registry/service_test.go`:

```go
func TestRegistryService_GetSkillContent(t *testing.T) {
	// Setup: publish a skill first
	svc := registry.NewService(store.NewMemory(nil))
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		Subject: "user-123",
		Email:   "user@example.com",
	})
	_, err := svc.PublishSkill(ctx, connect.NewRequest(&skillsctlv1.PublishSkillRequest{
		Name:        "my-skill",
		Version:     "1.0.0",
		Description: "A useful skill",
		Content:     []byte("# My Skill\nContent here"),
	}))
	if err != nil {
		t.Fatalf("setup publish: %v", err)
	}

	tests := []struct {
		name     string
		req      *skillsctlv1.GetSkillContentRequest
		wantCode connect.Code
	}{
		{
			name: "get by name and version",
			req:  &skillsctlv1.GetSkillContentRequest{Name: "my-skill", Version: "1.0.0"},
		},
		{
			name: "get latest",
			req:  &skillsctlv1.GetSkillContentRequest{Name: "my-skill"},
		},
		{
			name:     "nonexistent skill",
			req:      &skillsctlv1.GetSkillContentRequest{Name: "nope", Version: "1.0.0"},
			wantCode: connect.CodeNotFound,
		},
		{
			name:     "digest mismatch",
			req:      &skillsctlv1.GetSkillContentRequest{Name: "my-skill", Version: "1.0.0", Digest: "sha256:wrong"},
			wantCode: connect.CodeFailedPrecondition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.GetSkillContent(ctx, connect.NewRequest(tt.req))
			if tt.wantCode != 0 {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if connectErr := new(connect.Error); errors.As(err, &connectErr) {
					if connectErr.Code() != tt.wantCode {
						t.Errorf("expected code %v, got %v", tt.wantCode, connectErr.Code())
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(resp.Msg.Content) == 0 {
				t.Error("expected non-empty content")
			}
			if resp.Msg.Version == nil {
				t.Error("expected version metadata")
			}
		})
	}
}
```

- [ ] **Step 6: Run GetSkillContent tests to verify they fail**

Run: `go test ./backend/internal/registry/... -run "TestRegistryService_GetSkillContent" -v -race -count=1`

Expected: FAIL - stub returns "not implemented".

- [ ] **Step 7: Implement GetSkillContent handler**

Replace the stub `GetSkillContent` in `backend/internal/registry/service.go`:

```go
func (s *Service) GetSkillContent(ctx context.Context, req *connect.Request[skillsctlv1.GetSkillContentRequest]) (*connect.Response[skillsctlv1.GetSkillContentResponse], error) {
	content, ver, err := s.store.GetSkillContent(ctx, req.Msg.Name, req.Msg.Version, req.Msg.Digest)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, store.ErrDigestMismatch) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&skillsctlv1.GetSkillContentResponse{
		Content: content,
		Version: ver,
	}), nil
}
```

- [ ] **Step 8: Write test for unauthenticated GetSkillContent access**

Add to `backend/internal/registry/service_test.go`:

```go
func TestRegistryService_GetSkillContent_Unauthenticated(t *testing.T) {
	// Setup: publish a skill with auth
	svc := registry.NewService(store.NewMemory(nil))
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		Subject: "user-123",
		Email:   "user@example.com",
	})
	_, err := svc.PublishSkill(ctx, connect.NewRequest(&skillsctlv1.PublishSkillRequest{
		Name:        "my-skill",
		Version:     "1.0.0",
		Description: "A useful skill",
		Content:     []byte("# My Skill"),
	}))
	if err != nil {
		t.Fatalf("setup publish: %v", err)
	}

	// GetSkillContent without claims should succeed (read operation)
	resp, err := svc.GetSkillContent(context.Background(), connect.NewRequest(&skillsctlv1.GetSkillContentRequest{
		Name:    "my-skill",
		Version: "1.0.0",
	}))
	if err != nil {
		t.Fatalf("expected unauthenticated GetSkillContent to succeed, got: %v", err)
	}
	if len(resp.Msg.Content) == 0 {
		t.Error("expected non-empty content")
	}
}
```

- [ ] **Step 9: Run all registry tests**

Run: `go test ./backend/internal/registry/... -v -race -count=1`

Expected: All PASS.

- [ ] **Step 9: Run all project tests**

Run: `go test ./... -race -count=1`

Expected: All PASS.

- [ ] **Step 10: Commit**

```bash
git add backend/internal/registry/service.go backend/internal/registry/service_test.go
git commit -m "feat: implement PublishSkill and GetSkillContent handlers"
```

---

### Task 8: Run linting, full test suite, verify proto consistency

**Files:** None modified, verification only.

- [ ] **Step 1: Run linter**

Run: `golangci-lint run ./...`

Expected: No errors.

- [ ] **Step 2: Run full test suite with race detection**

Run: `go test ./... -race -count=1`

Expected: All PASS.

- [ ] **Step 3: Verify proto generated code is up to date**

Run: `cd proto && buf generate && git diff --exit-code gen/`

Expected: No diff - generated code matches proto definitions.

- [ ] **Step 4: Verify build**

Run: `CGO_ENABLED=0 go build ./backend/cmd/server && CGO_ENABLED=0 go build ./cli`

Expected: Both compile successfully.

- [ ] **Step 5: Fix any issues found, commit if needed**

If linting or tests surfaced issues, fix and commit with an appropriate message.

---

### Task 9: Add --verbose flag to explore show command

**Files:**
- Modify: `cli/cmd/explore.go`
- Modify: `cli/cmd/explore_test.go`
- Modify: `cli/internal/api/client.go`

- [ ] **Step 1: Add GetSkillContent to CLI API client**

Add to `cli/internal/api/client.go`:

```go
func (c *Client) GetSkillContent(ctx context.Context, name, version string) ([]byte, *skillsctlv1.SkillVersion, error) {
	resp, err := c.registry.GetSkillContent(ctx, connect.NewRequest(&skillsctlv1.GetSkillContentRequest{
		Name:    name,
		Version: version,
	}))
	if err != nil {
		return nil, nil, err
	}
	return resp.Msg.Content, resp.Msg.Version, nil
}
```

- [ ] **Step 2: Add --verbose flag to explore show**

In `cli/cmd/explore.go`, add the `--verbose` flag to the show subcommand and conditionally fetch/display content when set. Add the flag in the init or command setup:

```go
showCmd.Flags().BoolP("verbose", "v", false, "include skill content in output")
```

In the show command's RunE, after printing the existing metadata, add:

```go
verbose, _ := cmd.Flags().GetBool("verbose")
if verbose {
	content, _, err := apiClient.GetSkillContent(cmd.Context(), name, "")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not fetch content: %v\n", err)
	} else {
		fmt.Println("\n--- Content ---")
		fmt.Println(string(content))
	}
}
```

- [ ] **Step 3: Write test for --verbose flag**

Add to `cli/cmd/explore_test.go` a test that verifies the verbose flag fetches content. The test should use the existing stub server pattern from `cli/internal/testutil/stub_server.go`, ensuring `GetSkillContent` is handled by the stub. If the stub doesn't implement the new RPC yet, extend `StubRegistryService` with `PublishSkill` and `GetSkillContent` stubs.

- [ ] **Step 4: Run CLI tests**

Run: `go test ./cli/... -v -race -count=1`

Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add cli/cmd/explore.go cli/cmd/explore_test.go cli/internal/api/client.go
git commit -m "feat: add --verbose flag to explore show for content display"
```

---

### Task 10: Final verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -race -count=1`

Expected: All PASS.

- [ ] **Step 2: Run linter**

Run: `golangci-lint run ./...`

Expected: Clean.

- [ ] **Step 3: Verify proto generated code is committed**

Run: `cd proto && buf generate && git diff --exit-code gen/`

Expected: No diff.

- [ ] **Step 4: Verify the untracked proto files are committed**

The `proto/skillsctl/` directory was untracked at conversation start. Ensure it's included:

Run: `git status`

If `proto/skillsctl/` is still untracked, add it:
```bash
git add proto/skillsctl/
git commit -m "chore: track proto source files"
```
