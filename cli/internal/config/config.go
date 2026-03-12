package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	APIURL    string `mapstructure:"api_url"`
	SkillsDir string `mapstructure:"skills_dir"`
	Auth      Auth   `mapstructure:"auth"`
}

type Auth struct {
	OIDCIssuer string `mapstructure:"oidc_issuer"`
	ClientID   string `mapstructure:"client_id"`
}

func Load() *Config {
	home, _ := os.UserHomeDir()

	viper.SetDefault("api_url", "http://localhost:8080")
	viper.SetDefault("skills_dir", filepath.Join(home, ".claude", "skills"))

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(filepath.Join(home, ".config", "skillctl"))
	viper.SetEnvPrefix("SKILLCTL")
	viper.AutomaticEnv()

	_ = viper.ReadInConfig() // ok if config file not found

	cfg := &Config{}
	_ = viper.Unmarshal(cfg)
	return cfg
}
