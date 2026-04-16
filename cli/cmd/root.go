package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/nebari-dev/skillsctl/cli/internal/api"
	"github.com/nebari-dev/skillsctl/cli/internal/auth"
)

var (
	apiURL  string
	version = "dev"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "skillsctl",
		Short: "Discover, install, and publish Claude Code skills",
		Long: `skillsctl is the CLI for the SkillsCtl skill registry. Use it to browse
and install skills published by your team or organization, and to publish
your own. Skills install into ~/.claude/skills/ by default and are picked
up automatically by Claude Code at session start.`,
	}
	rootCmd.Version = version

	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "Registry URL; overrides the api_url setting from config and SKILLCTL_API_URL")
	rootCmd.PersistentFlags().StringVar(&credentialsPath, "credentials-path", "", "Credentials file path (for testing)")

	cobra.OnInitialize(func() {
		home, _ := os.UserHomeDir()
		viper.SetDefault("api_url", "http://localhost:8080")
		viper.SetDefault("skills_dir", filepath.Join(home, ".claude", "skills"))
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(home + "/.config/skillsctl")
		viper.SetEnvPrefix("SKILLCTL")
		viper.AutomaticEnv()
		_ = viper.ReadInConfig()
	})

	addExploreCmd(rootCmd)
	addConfigCmd(rootCmd)
	addPublishCmd(rootCmd)
	addInstallCmd(rootCmd)
	addAuthCmd(rootCmd)
	return rootCmd
}

func getAPIURL() string {
	if apiURL != "" {
		return apiURL
	}
	return viper.GetString("api_url")
}

// getClientCtx builds an API client using cached credentials, attempting a
// silent refresh_token grant if the cached ID token is near expiry.
func getClientCtx(ctx context.Context) *api.Client {
	token := ""
	if tok, _ := auth.LoadAndRefresh(ctx, resolveCredentialsPath()); tok != nil {
		token = tok.IDToken
	}
	return api.NewClient(getAPIURL(), api.WithToken(token))
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
