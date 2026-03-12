package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var apiURL string

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "skillctl",
		Short: "Discover, install, and publish Claude Code skills",
	}

	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "Backend API URL")

	cobra.OnInitialize(func() {
		home, _ := os.UserHomeDir()
		viper.SetDefault("api_url", "http://localhost:8080")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(home + "/.config/skillctl")
		viper.SetEnvPrefix("SKILLCTL")
		viper.AutomaticEnv()
		_ = viper.ReadInConfig()
	})

	addExploreCmd(rootCmd)
	return rootCmd
}

func getAPIURL() string {
	if apiURL != "" {
		return apiURL
	}
	return viper.GetString("api_url")
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
