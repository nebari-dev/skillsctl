package cmd

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

func addPublishCmd(root *cobra.Command) {
	var (
		name        string
		version     string
		description string
		dirPath     string
		tags        []string
		changelog   string
	)

	publishCmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a skill to the registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			content, err := skillpkg.Pack(dirPath)
			if err != nil {
				return fmt.Errorf("pack %s: %w", dirPath, err)
			}

			client := getClientCtx(cmd.Context())

			limits, err := client.GetLimits(cmd.Context())
			if err != nil {
				limits = skillpkg.DefaultLimits()
			}
			if limits.MaxPackedBytes > 0 && int64(len(content)) > limits.MaxPackedBytes {
				return fmt.Errorf("packed skill is %d bytes, server limit is %d", len(content), limits.MaxPackedBytes)
			}

			_, ver, err := client.PublishSkill(cmd.Context(), name, version, description, changelog, tags, content)
			if err != nil {
				return mapPublishError(err, name, version)
			}

			if ver.Digest != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Published %s@%s (%s)\n", name, version, ver.Digest)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Published %s@%s\n", name, version)
			}
			return nil
		},
	}

	publishCmd.Flags().StringVar(&name, "name", "", "Skill name")
	publishCmd.Flags().StringVar(&version, "version", "", "Skill version (semver)")
	publishCmd.Flags().StringVar(&description, "description", "", "Skill description")
	publishCmd.Flags().StringVar(&dirPath, "dir", "", "Path to skill directory containing SKILL.md")
	publishCmd.Flags().StringSliceVar(&tags, "tag", nil, "Tags (repeatable)")
	publishCmd.Flags().StringVar(&changelog, "changelog", "", "Version changelog")

	_ = publishCmd.MarkFlagRequired("name")
	_ = publishCmd.MarkFlagRequired("version")
	_ = publishCmd.MarkFlagRequired("description")
	_ = publishCmd.MarkFlagRequired("dir")

	root.AddCommand(publishCmd)
}

func mapPublishError(err error, name, version string) error {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return err
	}
	switch connectErr.Code() {
	case connect.CodeAlreadyExists:
		return fmt.Errorf("version %s of %s already exists", version, name)
	case connect.CodeUnauthenticated:
		return fmt.Errorf("not authenticated. Run 'skillsctl auth login' first")
	case connect.CodePermissionDenied:
		return fmt.Errorf("permission denied. You are not the owner of this skill")
	case connect.CodeInvalidArgument:
		return fmt.Errorf("%s", connectErr.Message())
	default:
		return fmt.Errorf("error: %s", connectErr.Message())
	}
}
