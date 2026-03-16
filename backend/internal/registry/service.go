package registry

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
	"github.com/nebari-dev/skillctl/gen/go/skillctl/v1/skillctlv1connect"

	"github.com/nebari-dev/skillctl/backend/internal/auth"
	"github.com/nebari-dev/skillctl/backend/internal/store"
)

// Service implements the RegistryService ConnectRPC handler.
type Service struct {
	store store.Repository
}

var _ skillctlv1connect.RegistryServiceHandler = (*Service)(nil)

// NewService creates a RegistryService backed by the given store.
func NewService(s store.Repository) *Service {
	return &Service{store: s}
}

func (s *Service) ListSkills(ctx context.Context, req *connect.Request[skillctlv1.ListSkillsRequest]) (*connect.Response[skillctlv1.ListSkillsResponse], error) {
	skills, nextToken, err := s.store.ListSkills(ctx, req.Msg.Tags, req.Msg.SourceFilter, req.Msg.PageSize, req.Msg.PageToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&skillctlv1.ListSkillsResponse{
		Skills:        skills,
		NextPageToken: nextToken,
	}), nil
}

func (s *Service) GetSkill(ctx context.Context, req *connect.Request[skillctlv1.GetSkillRequest]) (*connect.Response[skillctlv1.GetSkillResponse], error) {
	skill, versions, err := s.store.GetSkill(ctx, req.Msg.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&skillctlv1.GetSkillResponse{
		Skill:    skill,
		Versions: versions,
	}), nil
}

func (s *Service) PublishSkill(ctx context.Context, req *connect.Request[skillctlv1.PublishSkillRequest]) (*connect.Response[skillctlv1.PublishSkillResponse], error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required to publish"))
	}

	msg := req.Msg
	if err := validatePublishRequest(msg.Name, msg.Version, msg.Description, msg.Tags, msg.Content); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	digest := computeDigest(msg.Content)

	skill := &skillctlv1.Skill{
		Name:        msg.Name,
		Description: msg.Description,
		Owner:       claims.Subject,
		Tags:        msg.Tags,
		Source:      skillctlv1.SkillSource_SKILL_SOURCE_INTERNAL,
	}

	ver := &skillctlv1.SkillVersion{
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

	// Re-fetch to get authoritative state (timestamps, install_count, etc.)
	updatedSkill, versions, err := s.store.GetSkill(ctx, msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Find the just-published version from the re-fetched list so the response
	// includes any server-set fields (e.g. published_at).
	var publishedVer *skillctlv1.SkillVersion
	for _, v := range versions {
		if v.Version == msg.Version {
			publishedVer = v
			break
		}
	}
	if publishedVer == nil {
		// Fallback to the locally-built version if re-fetch somehow missed it.
		publishedVer = ver
	}

	return connect.NewResponse(&skillctlv1.PublishSkillResponse{
		Skill:   updatedSkill,
		Version: publishedVer,
	}), nil
}

func (s *Service) GetSkillContent(ctx context.Context, req *connect.Request[skillctlv1.GetSkillContentRequest]) (*connect.Response[skillctlv1.GetSkillContentResponse], error) {
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

	return connect.NewResponse(&skillctlv1.GetSkillContentResponse{
		Content: content,
		Version: ver,
	}), nil
}

// computeDigest returns a sha256 hex digest of the given content, prefixed with "sha256:".
func computeDigest(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("sha256:%x", h)
}
