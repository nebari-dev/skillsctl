package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nebari-dev/skillctl/cli/internal/auth"
)

var credentialsPath string

func addAuthCmd(root *cobra.Command) {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the registry",
		RunE:  runAuthLogin,
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE:  runAuthStatus,
	}

	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove cached credentials",
		RunE:  runAuthLogout,
	}

	authCmd.PersistentFlags().StringVar(&credentialsPath, "credentials-path", "", "Credentials file path (for testing)")
	authCmd.AddCommand(loginCmd, statusCmd, logoutCmd)
	root.AddCommand(authCmd)
}

func resolveCredentialsPath() string {
	if credentialsPath != "" {
		return credentialsPath
	}
	return auth.DefaultCredentialsPath()
}

func runAuthLogin(cmd *cobra.Command, _ []string) error {
	serverURL := getAPIURL()
	fmt.Fprintf(cmd.ErrOrStderr(), "Fetching auth config from %s...\n", serverURL)

	pending, err := auth.StartDeviceFlow(cmd.Context(), serverURL)
	if err != nil {
		if err == auth.ErrAuthDisabled {
			fmt.Fprintln(cmd.OutOrStdout(), "Server does not require authentication.")
			return nil
		}
		return err
	}

	// HTTPS warning
	if !strings.HasPrefix(pending.VerificationURI, "https://") &&
		!strings.HasPrefix(pending.VerificationURI, "http://localhost") &&
		!strings.HasPrefix(pending.VerificationURI, "http://127.0.0.1") {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: OIDC issuer is not using HTTPS.\n")
	}

	// Display verification info
	if pending.VerificationURIComplete != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "\nGo to: %s\n", pending.VerificationURIComplete)
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "\nGo to: %s\nEnter code: %s\n", pending.VerificationURI, pending.UserCode)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "\nWaiting for authentication...\n")

	// Poll for token
	result, err := auth.PollForToken(cmd.Context(), pending, 0)
	if err != nil {
		return err
	}

	// Save credentials
	path := resolveCredentialsPath()
	tok := &auth.CachedToken{
		IDToken: result.IDToken,
		Expiry:  result.Expiry,
	}
	if err := auth.SaveToken(path, tok); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s\n", result.Email)
	return nil
}

func runAuthStatus(cmd *cobra.Command, _ []string) error {
	path := resolveCredentialsPath()

	// Use LoadTokenRaw to distinguish expired from missing
	tok, _ := auth.LoadTokenRaw(path)
	if tok == nil {
		return fmt.Errorf("not logged in")
	}

	if time.Now().After(tok.Expiry) {
		return fmt.Errorf("session expired. Run 'skillctl auth login' to re-authenticate")
	}

	email, _ := auth.DecodeJWTClaims(tok.IDToken)
	fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s\n", email)
	fmt.Fprintf(cmd.OutOrStdout(), "Token expires at %s\n", tok.Expiry.UTC().Format("2006-01-02 15:04:05 UTC"))
	return nil
}

func runAuthLogout(cmd *cobra.Command, _ []string) error {
	path := resolveCredentialsPath()
	auth.DeleteToken(path)
	fmt.Fprintln(cmd.OutOrStdout(), "Logged out")
	return nil
}
