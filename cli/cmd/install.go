package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/nebari-dev/skillsctl/internal/skillpkg"
)

var skillNameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func validateSkillName(name string) error {
	if len(name) < 2 || len(name) > 64 {
		return fmt.Errorf("skill name must be between 2 and 64 characters")
	}
	if !skillNameRegexp.MatchString(name) {
		return fmt.Errorf("skill name must be lowercase alphanumeric with hyphens, cannot start or end with a hyphen")
	}
	return nil
}

func addInstallCmd(root *cobra.Command) {
	var (
		digest     string
		skillsDir  string
		projectDir string
		force      bool
	)

	installCmd := &cobra.Command{
		Use:   "install <name[@version]>",
		Short: "Install a skill from the registry",
		Long: `Download a skill from the registry and write it to the local skills directory.

By default skills install to the directory configured as skills_dir
(typically ~/.claude/skills). Use --project to install into a specific
project's .claude/skills/ instead, or --skills-dir to override the
target for a single invocation.

--project takes an optional value, so a path must be attached with an
equals sign (--project=/path/to/repo). Passing it bare (--project)
installs into the current directory.`,
		Example: `  # Install the latest version
  skillsctl install git-commit

  # Pin a version and verify the digest
  skillsctl install git-commit@1.2.0 --digest sha256:abc123...

  # Install into the current project (writes to ./.claude/skills/)
  skillsctl install git-commit --project

  # Install into a specific project directory (note the '=')
  skillsctl install git-commit --project=/path/to/repo`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, version := parseNameVersion(args[0])
			if err := validateSkillName(name); err != nil {
				return err
			}

			var dir string
			switch {
			case projectDir != "":
				dir = filepath.Join(projectDir, ".claude", "skills")
			case skillsDir != "":
				dir = skillsDir
			default:
				dir = viper.GetString("skills_dir")
			}

			client := getClientCtx(cmd.Context())
			content, ver, err := client.GetSkillContent(cmd.Context(), name, version, digest)
			if err != nil {
				return mapInstallError(err, name, version)
			}

			destDir := filepath.Join(dir, name)
			absSkillsDir, _ := filepath.Abs(dir)
			absDest, _ := filepath.Abs(destDir)
			if !strings.HasPrefix(absDest, absSkillsDir+string(filepath.Separator)) {
				return fmt.Errorf("invalid skill name: resolved path escapes skills directory")
			}

			var installedPath string
			if skillpkg.IsTarball(content) {
				limits, err := client.GetLimits(cmd.Context())
				if err != nil {
					limits = skillpkg.DefaultLimits()
				}
				if err := installTarball(content, destDir, force, limits); err != nil {
					return err
				}
				installedPath = destDir
			} else {
				if err := installSingleFile(content, destDir); err != nil {
					return err
				}
				installedPath = filepath.Join(destDir, "SKILL.md")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed %s@%s to %s\n", name, ver.Version, installedPath)
			return nil
		},
	}

	installCmd.Flags().StringVar(&digest, "digest", "", "Expected content digest in sha256:... form; install aborts if the downloaded content does not match")
	installCmd.Flags().StringVar(&skillsDir, "skills-dir", "", "Override the configured skills directory for this install")
	installCmd.Flags().StringVar(&projectDir, "project", "", "Install to <path>/.claude/skills; when used without a value, installs into the current directory")
	// Passing --project without a value resolves to the current directory.
	installCmd.Flags().Lookup("project").NoOptDefVal = "."
	installCmd.MarkFlagsMutuallyExclusive("project", "skills-dir")
	installCmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing skill directory")

	root.AddCommand(installCmd)
}

func installTarball(content []byte, destDir string, force bool, limits skillpkg.Limits) error {
	if force {
		if err := os.RemoveAll(destDir); err != nil {
			return fmt.Errorf("remove existing %s: %w", destDir, err)
		}
	}
	if err := skillpkg.Extract(content, destDir, limits); err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	return nil
}

func installSingleFile(content []byte, destDir string) error {
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("create directory %s: %w", destDir, err)
	}
	return atomicWrite(filepath.Join(destDir, "SKILL.md"), content)
}

func parseNameVersion(arg string) (string, string) {
	if idx := strings.LastIndex(arg, "@"); idx > 0 {
		return arg[:idx], arg[idx+1:]
	}
	return arg, ""
}

func atomicWrite(destPath string, data []byte) error {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".skillsctl-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename to %s: %w", destPath, err)
	}
	return nil
}

func mapInstallError(err error, name, version string) error {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return err
	}
	switch connectErr.Code() {
	case connect.CodeNotFound:
		if version != "" {
			return fmt.Errorf("version %s of skill %q not found", version, name)
		}
		return fmt.Errorf("skill %q not found", name)
	case connect.CodeFailedPrecondition:
		return fmt.Errorf("digest mismatch for %s@%s. Content may have been tampered with", name, version)
	default:
		return fmt.Errorf("error: %s", connectErr.Message())
	}
}
