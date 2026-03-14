package registry

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
	"github.com/nebari-dev/skillctl/gen/go/skillctl/v1/skillctlv1connect"

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

func (s *Service) PublishSkill(_ context.Context, _ *connect.Request[skillctlv1.PublishSkillRequest]) (*connect.Response[skillctlv1.PublishSkillResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *Service) GetSkillContent(_ context.Context, _ *connect.Request[skillctlv1.GetSkillContentRequest]) (*connect.Response[skillctlv1.GetSkillContentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}
