package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thoscut/scanflow/client/internal/client"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Start a document scan",
	Long:  "Start a scan on the remote server with the specified profile and output target.",
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().StringP("profile", "p", "", "Scan profile (default: from config)")
	scanCmd.Flags().StringP("output", "o", "", "Output target (paperless, smb, filesystem)")
	scanCmd.Flags().StringP("title", "t", "", "Document title")
	scanCmd.Flags().BoolP("interactive", "i", false, "Interactive mode")
	scanCmd.Flags().IntSlice("tags", nil, "Paperless tag IDs")
	scanCmd.Flags().Int("correspondent", 0, "Paperless correspondent ID")
	scanCmd.Flags().Int("document-type", 0, "Paperless document type ID")
	scanCmd.Flags().String("filename", "", "Output filename")
	scanCmd.Flags().Bool("json", false, "Output as JSON")
}

func runScan(cmd *cobra.Command, args []string) error {
	c := getClient()

	interactive, _ := cmd.Flags().GetBool("interactive")
	if interactive {
		return runInteractiveScan(cmd, c)
	}

	profile, _ := cmd.Flags().GetString("profile")
	if profile == "" {
		profile = cfg.Defaults.Profile
	}

	req := &client.ScanRequest{
		Profile: profile,
	}

	// Output
	outputStr, _ := cmd.Flags().GetString("output")
	if outputStr == "" {
		outputStr = cfg.Defaults.Output
	}
	if outputStr != "" {
		req.Output = &client.OutputConfig{Target: outputStr}
	}
	if filename, _ := cmd.Flags().GetString("filename"); filename != "" {
		if req.Output == nil {
			req.Output = &client.OutputConfig{}
		}
		req.Output.Filename = filename
	}

	// Metadata
	title, _ := cmd.Flags().GetString("title")
	tags, _ := cmd.Flags().GetIntSlice("tags")
	correspondent, _ := cmd.Flags().GetInt("correspondent")
	documentType, _ := cmd.Flags().GetInt("document-type")

	if title != "" || len(tags) > 0 || correspondent > 0 || documentType > 0 {
		req.Metadata = &client.DocumentMetadata{
			Title:         title,
			Tags:          tags,
			Correspondent: correspondent,
			DocumentType:  documentType,
		}
	}

	fmt.Printf("Starting scan (profile: %s)...\n", profile)

	job, err := c.StartScan(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("start scan: %w", err)
	}

	fmt.Printf("Job ID: %s\n", job.ID)

	return waitForJob(cmd, c, job.ID)
}

func waitForJob(cmd *cobra.Command, c *client.Client, jobID string) error {
	lastStatus := ""
	spinner := []string{"|", "/", "-", "\\"}
	spinIdx := 0

	job, err := c.WaitForJob(cmd.Context(), jobID, func(job client.ScanJob) {
		if job.Status != lastStatus {
			if lastStatus != "" {
				fmt.Println()
			}
			lastStatus = job.Status
		}
		spinIdx = (spinIdx + 1) % len(spinner)
		fmt.Printf("\r  %s %s [%d pages]", spinner[spinIdx], job.Status, len(job.Pages))
	})

	fmt.Println()

	if err != nil {
		return fmt.Errorf("wait for job: %w", err)
	}

	switch job.Status {
	case "completed":
		fmt.Printf("Scan completed successfully (%d pages)\n", len(job.Pages))
	case "failed":
		return fmt.Errorf("scan failed: %s", job.Error)
	case "cancelled":
		fmt.Println("Scan was cancelled")
	}

	return nil
}

func runInteractiveScan(cmd *cobra.Command, c *client.Client) error {
	profile, _ := cmd.Flags().GetString("profile")
	if profile == "" {
		profile = cfg.Defaults.Profile
	}

	req := &client.ScanRequest{Profile: profile}
	job, err := c.StartScan(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("start scan: %w", err)
	}

	fmt.Printf("Job ID: %s\n", job.ID)
	fmt.Println("Scanning...")

	// Wait for initial scan to complete
	time.Sleep(2 * time.Second)

	for {
		job, err = c.GetJobStatus(cmd.Context(), job.ID)
		if err != nil {
			return err
		}

		fmt.Printf("\n%d pages scanned\n", len(job.Pages))
		fmt.Println()
		fmt.Println("[W] Scan more pages")
		fmt.Println("[F] Finish and create PDF")
		fmt.Println("[D] Delete last page")
		fmt.Println("[Q] Cancel and quit")
		fmt.Print("\nChoice: ")

		var choice string
		fmt.Scanln(&choice)
		choice = strings.ToLower(strings.TrimSpace(choice))

		switch choice {
		case "w":
			fmt.Println("Scanning more pages...")
			if err := c.ContinueScan(cmd.Context(), job.ID); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			time.Sleep(2 * time.Second)

		case "f":
			outputStr, _ := cmd.Flags().GetString("output")
			if outputStr == "" {
				outputStr = cfg.Defaults.Output
			}
			title, _ := cmd.Flags().GetString("title")

			var output *client.OutputConfig
			if outputStr != "" {
				output = &client.OutputConfig{Target: outputStr}
			}
			var metadata *client.DocumentMetadata
			if title != "" {
				metadata = &client.DocumentMetadata{Title: title}
			}

			if err := c.FinishScan(cmd.Context(), job.ID, output, metadata); err != nil {
				return fmt.Errorf("finish scan: %w", err)
			}

			return waitForJob(cmd, c, job.ID)

		case "d":
			if len(job.Pages) > 0 {
				lastPage := len(job.Pages)
				if err := c.DeletePage(cmd.Context(), job.ID, lastPage); err != nil {
					fmt.Printf("Error: %v\n", err)
				} else {
					fmt.Printf("Page %d deleted\n", lastPage)
				}
			} else {
				fmt.Println("No pages to delete")
			}

		case "q":
			if err := c.CancelJob(cmd.Context(), job.ID); err != nil {
				fmt.Printf("Error cancelling: %v\n", err)
			}
			fmt.Println("Scan cancelled")
			return nil

		default:
			fmt.Println("Unknown option")
		}
	}
}
