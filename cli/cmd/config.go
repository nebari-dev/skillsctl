package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

// configPath is overridable via --config-path for testing.
var configPath string

// validConfigKeys lists the keys that config set/get accept.
var validConfigKeys = []string{"api_url", "skills_dir"}

func addConfigCmd(root *cobra.Command) {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage skillctl configuration",
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive first-time setup",
		RunE:  runConfigInit,
	}
	initCmd.Flags().Bool("force", false, "Overwrite existing config")

	setCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}

	getCmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigGet,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all config values",
		RunE:  runConfigList,
	}

	configCmd.PersistentFlags().StringVar(&configPath, "config-path", "", "Config file path (for testing)")
	configCmd.AddCommand(initCmd, setCmd, getCmd, listCmd)
	root.AddCommand(configCmd)
}

func resolveConfigPath() string {
	if configPath != "" {
		return configPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skillctl", "config.yaml")
}

func runConfigInit(cmd *cobra.Command, _ []string) error {
	path := resolveConfigPath()
	force, _ := cmd.Flags().GetBool("force")

	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config already exists at %s. Use --force to overwrite", path)
		}
	}

	home, _ := os.UserHomeDir()
	defaultAPI := "http://localhost:8080"
	defaultSkills := filepath.Join(home, ".claude", "skills")

	reader := bufio.NewReader(cmd.InOrStdin())

	fmt.Fprintf(cmd.ErrOrStderr(), "No configuration found. Let's set up skillctl.\n\n")
	fmt.Fprintf(cmd.ErrOrStderr(), "API URL [%s]: ", defaultAPI)
	apiInput, _ := reader.ReadString('\n')
	apiInput = strings.TrimSpace(apiInput)
	if apiInput == "" {
		apiInput = defaultAPI
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Skills directory [%s]: ", defaultSkills)
	skillsInput, _ := reader.ReadString('\n')
	skillsInput = strings.TrimSpace(skillsInput)
	if skillsInput == "" {
		skillsInput = defaultSkills
	}

	cfg := map[string]string{
		"api_url":    apiInput,
		"skills_dir": skillsInput,
	}

	if err := writeConfigFile(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "\nConfig saved to %s\n", path)
	return nil
}

func writeConfigFile(path string, data map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}

func readConfigFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := make(map[string]string)
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func isValidKey(key string) bool {
	for _, k := range validConfigKeys {
		if k == key {
			return true
		}
	}
	return false
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]
	if !isValidKey(key) {
		return fmt.Errorf("unknown config key %q. Valid keys: %s", key, strings.Join(validConfigKeys, ", "))
	}

	path := resolveConfigPath()
	cfg, err := readConfigFile(path)
	if err != nil {
		return err
	}
	cfg[key] = value
	return writeConfigFile(path, cfg)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	if !isValidKey(key) {
		return fmt.Errorf("unknown config key %q. Valid keys: %s", key, strings.Join(validConfigKeys, ", "))
	}

	loadConfigOverride()
	fmt.Fprintln(cmd.OutOrStdout(), viper.GetString(key))
	return nil
}

func runConfigList(cmd *cobra.Command, _ []string) error {
	loadConfigOverride()
	for _, key := range validConfigKeys {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", key, viper.GetString(key))
	}
	return nil
}

func loadConfigOverride() {
	if configPath != "" {
		viper.SetConfigFile(configPath)
		_ = viper.ReadInConfig()
	}
}
