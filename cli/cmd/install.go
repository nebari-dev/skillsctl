package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func addInstallCmd(root *cobra.Command) {
	var (
		digest    string
		skillsDir string
	)

	installCmd := &cobra.Command{
		Use:   "install <name[@version]>",
		Short: "Install a skill from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, version := parseNameVersion(args[0])

			dir := skillsDir
			if dir == "" {
				dir = viper.GetString("skills_dir")
			}

			client := getClient()
			content, ver, err := client.GetSkillContent(cmd.Context(), name, version, digest)
			if err != nil {
				return mapInstallError(err, name, version)
			}

			destPath := filepath.Join(dir, name+".md")
			if err := atomicWrite(destPath, content); err != nil {
				return fmt.Errorf("write skill file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed %s@%s to %s\n", name, ver.Version, destPath)
			return nil
		},
	}

	installCmd.Flags().StringVar(&digest, "digest", "", "Expected content digest for verification")
	installCmd.Flags().StringVar(&skillsDir, "skills-dir", "", "Override skills directory")

	root.AddCommand(installCmd)
}

func parseNameVersion(arg string) (string, string) {
	if idx := strings.LastIndex(arg, "@"); idx > 0 {
		return arg[:idx], arg[idx+1:]
	}
	return arg, ""
}

func atomicWrite(destPath string, data []byte) error {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".skillsctl-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
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
