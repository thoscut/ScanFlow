package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thoscut/scanflow/client/internal/client"
	"github.com/thoscut/scanflow/client/internal/config"
)

var (
	cfgFile   string
	cfg       *config.Config
	apiClient *client.Client
)

var rootCmd = &cobra.Command{
	Use:   "scanflow",
	Short: "ScanFlow - Network Scanner Client",
	Long:  "ScanFlow client for remote scanner control, scan management, and document delivery.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		if cfgFile != "" {
			cfg, err = config.LoadFrom(cfgFile)
		} else {
			cfg, err = config.Load()
		}
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		apiClient = client.New(cfg.Server.URL, cfg.Server.APIKey)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().String("server", "", "server URL (overrides config)")
	rootCmd.PersistentFlags().String("api-key", "", "API key (overrides config)")

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(devicesCmd)
	rootCmd.AddCommand(profilesCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getClient() *client.Client {
	return apiClient
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("scanflow client v0.1.0")
	},
}
