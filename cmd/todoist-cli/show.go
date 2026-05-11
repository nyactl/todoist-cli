package main

import (
	"fmt"
	"strings"

	"todoist-cli/internal/config"
	"todoist-cli/internal/todoist"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show full task details (live API call)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := config.GetToken()
		if err != nil {
			return err
		}
		client := todoist.New(token)
		t, err := client.GetTask(cmd.Context(), args[0])
		if err != nil {
			return err
		}

		fmt.Printf("%s  %s\n", t.ID[:min(4, len(t.ID))], t.Content)
		if t.Description != "" {
			fmt.Printf("\n%s\n", t.Description)
		}
		if t.Due != nil {
			due := t.Due.Date
			if t.Due.Datetime != "" {
				due = t.Due.Datetime
			}
			fmt.Printf("\ndue  %s", due)
			if t.Due.String != "" {
				fmt.Printf("  (%s)", t.Due.String)
			}
			fmt.Println()
		}
		if len(t.Labels) > 0 {
			fmt.Printf("labels  %s\n", strings.Join(t.Labels, ", "))
		}
		if t.URL != "" {
			fmt.Printf("url  %s\n", t.URL)
		}
		return nil
	},
}

func init() {
	root.AddCommand(showCmd)
}
