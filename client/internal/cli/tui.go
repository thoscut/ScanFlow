package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thoscut/scanflow/client/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Start interactive terminal UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := getClient()
		app := tui.New(c, cfg)
		if err := app.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	},
}
