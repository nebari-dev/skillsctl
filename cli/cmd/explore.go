package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	skillsctlv1 "github.com/nebari-dev/skillsctl/gen/go/skillsctl/v1"
)

func addExploreCmd(root *cobra.Command) {
	var tags []string
	var source string

	exploreCmd := &cobra.Command{
		Use:   "explore",
		Short: "Browse available skills",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client := getClient()

			sourceFilter := skillsctlv1.SkillSource_SKILL_SOURCE_UNSPECIFIED
			switch source {
			case "internal":
				sourceFilter = skillsctlv1.SkillSource_SKILL_SOURCE_INTERNAL
			case "external":
				sourceFilter = skillsctlv1.SkillSource_SKILL_SOURCE_FEDERATED
			}

			skills, err := client.ListSkills(context.Background(), tags, sourceFilter)
			if err != nil {
				return fmt.Errorf("failed to list skills: %w", err)
			}

			printSkillTable(cmd.OutOrStdout(), skills)
			return nil
		},
	}

	exploreCmd.Flags().StringSliceVar(&tags, "tag", nil, "Filter by tag (repeatable)")
	exploreCmd.Flags().StringVar(&source, "source", "all", "Filter: internal, external, all")

	showCmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show skill details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getClient()
			skill, versions, err := client.GetSkill(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("skill not found: %w", err)
			}
			printSkillDetail(cmd.OutOrStdout(), skill, versions)

			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				content, _, err := client.GetSkillContent(cmd.Context(), args[0], "", "")
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not fetch content: %v\n", err)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "\n--- Content ---")
					fmt.Fprintln(cmd.OutOrStdout(), string(content))
				}
			}
			return nil
		},
	}
	showCmd.Flags().BoolP("verbose", "v", false, "include skill content in output")

	exploreCmd.AddCommand(showCmd)
	root.AddCommand(exploreCmd)
}

func printSkillTable(w io.Writer, skills []*skillsctlv1.Skill) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SOURCE\tNAME\tOWNER\tTAGS\tINSTALLS\tVERSION")
	for _, s := range skills {
		src := "internal"
		if s.Source == skillsctlv1.SkillSource_SKILL_SOURCE_FEDERATED {
			src = "external"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\n",
			src, s.Name, s.Owner, strings.Join(s.Tags, ","), s.InstallCount, s.LatestVersion)
	}
	_ = tw.Flush()
}

func printSkillDetail(w io.Writer, skill *skillsctlv1.Skill, versions []*skillsctlv1.SkillVersion) {
	fmt.Fprintf(w, "Name:        %s\n", skill.Name)
	fmt.Fprintf(w, "Description: %s\n", skill.Description)
	fmt.Fprintf(w, "Owner:       %s\n", skill.Owner)
	fmt.Fprintf(w, "Tags:        %s\n", strings.Join(skill.Tags, ", "))
	fmt.Fprintf(w, "Version:     %s\n", skill.LatestVersion)
	fmt.Fprintf(w, "Installs:    %d\n", skill.InstallCount)

	src := "internal"
	if skill.Source == skillsctlv1.SkillSource_SKILL_SOURCE_FEDERATED {
		src = fmt.Sprintf("external (%s)", skill.MarketplaceId)
	}
	fmt.Fprintf(w, "Source:      %s\n", src)

	if len(versions) > 0 {
		fmt.Fprintln(w, "\nVersions:")
		for _, v := range versions {
			fmt.Fprintf(w, "  %s\n", v.Version)
		}
	}
}
