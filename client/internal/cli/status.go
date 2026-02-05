package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show server status",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().Bool("json", false, "Output as JSON")
}

func runStatus(cmd *cobra.Command, args []string) error {
	c := getClient()

	status, err := c.Status(cmd.Context())
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	fmt.Printf("Server:      %s\n", cfg.Server.URL)
	fmt.Printf("Status:      %s\n", status.Status)
	fmt.Printf("Version:     %s\n", status.Version)
	fmt.Printf("Scanner:     %v\n", status.Scanner)
	fmt.Printf("Devices:     %d\n", status.Devices)
	fmt.Printf("Active Jobs: %d\n", status.ActiveJobs)
	fmt.Printf("Total Jobs:  %d\n", status.TotalJobs)

	return nil
}
