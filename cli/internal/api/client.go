package api

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	skillsctlv1 "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1"
	"github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1/skillsctlv1connect"
)

// ClientOption configures the API client.
type ClientOption func(*Client)

// WithToken sets the Bearer token for all requests.
func WithToken(token string) ClientOption {
	return func(c *Client) {
		c.token = token
	}
}

// Client is the skillsctl API client.
type Client struct {
	registry skillsctlv1connect.RegistryServiceClient
	token    string
}

// NewClient creates an API client. Pass WithToken to attach auth.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}

	httpClient := http.DefaultClient
	if c.token != "" {
		httpClient = &http.Client{
			Transport: &tokenRoundTripper{
				base:  http.DefaultTransport,
				token: c.token,
			},
		}
	}

	c.registry = skillsctlv1connect.NewRegistryServiceClient(httpClient, baseURL)
	return c
}

type tokenRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (t *tokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

func (c *Client) ListSkills(ctx context.Context, tags []string, source skillsctlv1.SkillSource) ([]*skillsctlv1.Skill, error) {
	resp, err := c.registry.ListSkills(ctx, connect.NewRequest(&skillsctlv1.ListSkillsRequest{
		Tags:         tags,
		SourceFilter: source,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Skills, nil
}

func (c *Client) GetSkill(ctx context.Context, name string) (*skillsctlv1.Skill, []*skillsctlv1.SkillVersion, error) {
	resp, err := c.registry.GetSkill(ctx, connect.NewRequest(&skillsctlv1.GetSkillRequest{
		Name: name,
	}))
	if err != nil {
		return nil, nil, err
	}
	return resp.Msg.Skill, resp.Msg.Versions, nil
}

func (c *Client) GetSkillContent(ctx context.Context, name, version, digest string) ([]byte, *skillsctlv1.SkillVersion, error) {
	resp, err := c.registry.GetSkillContent(ctx, connect.NewRequest(&skillsctlv1.GetSkillContentRequest{
		Name:    name,
		Version: version,
		Digest:  digest,
	}))
	if err != nil {
		return nil, nil, err
	}
	return resp.Msg.Content, resp.Msg.Version, nil
}

func (c *Client) PublishSkill(ctx context.Context, name, version, description, changelog string, tags []string, content []byte) (*skillsctlv1.Skill, *skillsctlv1.SkillVersion, error) {
	resp, err := c.registry.PublishSkill(ctx, connect.NewRequest(&skillsctlv1.PublishSkillRequest{
		Name:        name,
		Version:     version,
		Description: description,
		Tags:        tags,
		Changelog:   changelog,
		Content:     content,
	}))
	if err != nil {
		return nil, nil, err
	}
	return resp.Msg.Skill, resp.Msg.Version, nil
}
