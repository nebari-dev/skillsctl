package cmd

import (
	"errors"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/nebari-dev/skillctl/cli/internal/api"
)

const maxContentBytes = 1024 * 1024 // 1MB

func addPublishCmd(root *cobra.Command) {
	var (
		name        string
		version     string
		description string
		filePath    string
		tags        []string
		changelog   string
	)

	publishCmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a skill to the registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file %s: %w", filePath, err)
			}
			if len(content) > maxContentBytes {
				return fmt.Errorf("file exceeds maximum size of %d bytes", maxContentBytes)
			}

			client := api.NewClient(getAPIURL())
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
	publishCmd.Flags().StringVar(&filePath, "file", "", "Path to skill content file")
	publishCmd.Flags().StringSliceVar(&tags, "tag", nil, "Tags (repeatable)")
	publishCmd.Flags().StringVar(&changelog, "changelog", "", "Version changelog")

	publishCmd.MarkFlagRequired("name")
	publishCmd.MarkFlagRequired("version")
	publishCmd.MarkFlagRequired("description")
	publishCmd.MarkFlagRequired("file")

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
		return fmt.Errorf("not authenticated. Run 'skillctl auth login' first")
	case connect.CodePermissionDenied:
		return fmt.Errorf("permission denied. You are not the owner of this skill")
	case connect.CodeInvalidArgument:
		return fmt.Errorf("%s", connectErr.Message())
	default:
		return fmt.Errorf("error: %s", connectErr.Message())
	}
}
