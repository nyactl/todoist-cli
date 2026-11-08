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
	"golang.org/x/term"
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
		token, err := readToken()
		if err != nil {
			return fmt.Errorf("read token: %w", err)
		}
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

// stdinIsTerminal and readPassword are variables so tests can stand in for
// a terminal.
var (
	stdinIsTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
	readPassword    = func() ([]byte, error) { return term.ReadPassword(int(os.Stdin.Fd())) }
)

// readToken reads the API token without echoing it on a terminal, and as a
// plain line from piped input.
func readToken() (string, error) {
	if stdinIsTerminal() {
		b, err := readPassword()
		fmt.Fprintln(os.Stderr)
		return strings.TrimSpace(string(b)), err
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text()), scanner.Err()
}

func init() {
	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd)
	root.AddCommand(authCmd)
}
