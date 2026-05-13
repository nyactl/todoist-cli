package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nyactl/todoist-cli/internal/config"
	"github.com/nyactl/todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Todoist API authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Save your Todoist API token to the system keychain",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprint(os.Stderr, "Todoist API token: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		token := strings.TrimSpace(scanner.Text())
		if token == "" {
			return fmt.Errorf("token cannot be empty")
		}

		client := todoist.New(token)
		projects, err := client.GetProjects(context.Background())
		if err != nil {
			return fmt.Errorf("token verification failed: %w", err)
		}

		if err := config.SetToken(token); err != nil {
			return fmt.Errorf("save token: %w", err)
		}
		fmt.Fprintf(os.Stderr, "logged in — %d projects found\n", len(projects))
		fmt.Fprintln(os.Stderr, "syncing...")
		return syncCmd.RunE(cmd, nil)
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the stored API token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.DeleteToken(); err != nil {
			return fmt.Errorf("remove token: %w", err)
		}
		fmt.Fprintln(os.Stderr, "logged out")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Verify the stored token and show account info",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)
		projects, err := client.GetProjects(context.Background())
		if err != nil {
			return fmt.Errorf("token invalid: %w", err)
		}
		fmt.Fprintf(os.Stderr, "authenticated — %d projects\n", len(projects))
		return nil
	},
}

func init() {
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd)
	root.AddCommand(authCmd)
}
