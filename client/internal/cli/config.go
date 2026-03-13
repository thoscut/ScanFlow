package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage client configuration",
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Current configuration:")
		fmt.Println()
		fmt.Printf("  server.url        = %s\n", cfg.Server.URL)
		fmt.Printf("  server.api_key    = %s\n", maskKey(cfg.Server.APIKey))
		fmt.Printf("  defaults.profile  = %s\n", cfg.Defaults.Profile)
		fmt.Printf("  defaults.output   = %s\n", cfg.Defaults.Output)
		fmt.Printf("  tui.theme         = %s\n", cfg.TUI.Theme)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Long: `Set a configuration value. Valid keys:
  server.url        Server URL (e.g. http://scanserver.local:8080)
  server.api_key    API key for authentication
  defaults.profile  Default scan profile name
  defaults.output   Default output target (paperless, smb, filesystem)
  tui.theme         TUI theme (dark, light)`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		if err := cfg.Set(key, value); err != nil {
			return err
		}

		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		fmt.Printf("%s = %s\n", key, value)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Long: `Get a configuration value. Valid keys:
  server.url        Server URL
  server.api_key    API key
  defaults.profile  Default scan profile
  defaults.output   Default output target
  tui.theme         TUI theme`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		value, err := cfg.Get(args[0])
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	},
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
