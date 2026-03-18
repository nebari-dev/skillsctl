package cmd

import (
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
	}
	rootCmd.Version = version

	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "Backend API URL")
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

func getClient() *api.Client {
	token := ""
	if tok, _ := auth.LoadToken(resolveCredentialsPath()); tok != nil {
		token = tok.IDToken
	}
	return api.NewClient(getAPIURL(), api.WithToken(token))
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
