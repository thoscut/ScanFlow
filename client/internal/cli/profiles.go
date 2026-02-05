package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage scan profiles",
}

func init() {
	profilesCmd.AddCommand(profilesListCmd)
	profilesCmd.AddCommand(profilesShowCmd)
}

var profilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available profiles",
	RunE:  runProfilesList,
}

var profilesShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show profile details",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfilesShow,
}

func init() {
	profilesListCmd.Flags().Bool("json", false, "Output as JSON")
}

func runProfilesList(cmd *cobra.Command, args []string) error {
	c := getClient()

	profiles, err := c.ListProfiles(cmd.Context())
	if err != nil {
		return fmt.Errorf("list profiles: %w", err)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(profiles)
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tRESOLUTION\tMODE")
	for _, p := range profiles {
		fmt.Fprintf(w, "%s\t%s\t%d DPI\t%s\n",
			p.Profile.Name,
			p.Profile.Description,
			p.Scanner.Resolution,
			p.Scanner.Mode)
	}
	return w.Flush()
}

func runProfilesShow(cmd *cobra.Command, args []string) error {
	c := getClient()

	profile, err := c.GetProfile(cmd.Context(), args[0])
	if err != nil {
		return fmt.Errorf("get profile: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(profile)
}
