package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List available scanner devices",
	RunE:  runDevices,
}

func init() {
	devicesCmd.Flags().Bool("json", false, "Output as JSON")
}

func runDevices(cmd *cobra.Command, args []string) error {
	c := getClient()

	devices, err := c.ListDevices(cmd.Context())
	if err != nil {
		return fmt.Errorf("list devices: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(devices)
	}

	if len(devices) == 0 {
		fmt.Println("No scanner devices found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVENDOR\tMODEL\tTYPE")
	for _, d := range devices {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", d.Name, d.Vendor, d.Model, d.Type)
	}
	return w.Flush()
}
