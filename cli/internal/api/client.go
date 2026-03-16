package api

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	skillctlv1 "github.com/nebari-dev/skillctl/gen/go/skillctl/v1"
	"github.com/nebari-dev/skillctl/gen/go/skillctl/v1/skillctlv1connect"
)

type Client struct {
	registry skillctlv1connect.RegistryServiceClient
}

func NewClient(baseURL string) *Client {
	return &Client{
		registry: skillctlv1connect.NewRegistryServiceClient(http.DefaultClient, baseURL),
	}
}

func (c *Client) ListSkills(ctx context.Context, tags []string, source skillctlv1.SkillSource) ([]*skillctlv1.Skill, error) {
	resp, err := c.registry.ListSkills(ctx, connect.NewRequest(&skillctlv1.ListSkillsRequest{
		Tags:         tags,
		SourceFilter: source,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Skills, nil
}

func (c *Client) GetSkill(ctx context.Context, name string) (*skillctlv1.Skill, []*skillctlv1.SkillVersion, error) {
	resp, err := c.registry.GetSkill(ctx, connect.NewRequest(&skillctlv1.GetSkillRequest{
		Name: name,
	}))
	if err != nil {
		return nil, nil, err
	}
	return resp.Msg.Skill, resp.Msg.Versions, nil
}

func (c *Client) GetSkillContent(ctx context.Context, name, version string) ([]byte, *skillctlv1.SkillVersion, error) {
	resp, err := c.registry.GetSkillContent(ctx, connect.NewRequest(&skillctlv1.GetSkillContentRequest{
		Name:    name,
		Version: version,
	}))
	if err != nil {
		return nil, nil, err
	}
	return resp.Msg.Content, resp.Msg.Version, nil
}
