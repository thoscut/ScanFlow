package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thoscut/scanflow/client/internal/client"
	"github.com/thoscut/scanflow/client/internal/config"
)

// scanModel handles the scan view in the TUI.
type scanModel struct {
	client   *client.Client
	config   *config.Config
	job      *client.ScanJob
	status   string
	pages    int
	progress int
	err      error
	quitting bool
	done     bool
}

type scanStartedMsg struct {
	job *client.ScanJob
}

type scanUpdateMsg struct {
	job *client.ScanJob
}

type scanErrorMsg struct {
	err error
}

type scanCompleteMsg struct{}

func newScanModel(c *client.Client, cfg *config.Config) scanModel {
	return scanModel{
		client: c,
		config: cfg,
		status: "Preparing...",
	}
}

func (m scanModel) init() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		req := &client.ScanRequest{
			Profile: m.config.Defaults.Profile,
			Output: &client.OutputConfig{
				Target: m.config.Defaults.Output,
			},
		}

		job, err := m.client.StartScan(ctx, req)
		if err != nil {
			return scanErrorMsg{err: err}
		}
		return scanStartedMsg{job: job}
	}
}

func (m scanModel) update(msg tea.Msg) (scanModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "w":
			if m.job != nil && !m.done {
				m.status = "Scanning more pages..."
				return m, m.continueScan()
			}
		case "f":
			if m.job != nil && !m.done {
				m.status = "Finishing..."
				return m, m.finishScan()
			}
		case "d":
			if m.job != nil && m.pages > 0 && !m.done {
				return m, m.deletePage()
			}
		case "q", "esc":
			if m.job != nil && !m.done {
				m.client.CancelJob(context.Background(), m.job.ID)
			}
			m.quitting = true
			return m, nil
		}

	case scanStartedMsg:
		m.job = msg.job
		m.status = "Scanning..."
		return m, m.pollJob()

	case scanUpdateMsg:
		m.job = msg.job
		m.status = msg.job.Status
		m.pages = len(msg.job.Pages)
		m.progress = msg.job.Progress

		switch msg.job.Status {
		case "completed":
			m.done = true
			m.status = "Completed"
		case "failed":
			m.done = true
			m.err = fmt.Errorf("%s", msg.job.Error)
		case "cancelled":
			m.done = true
			m.status = "Cancelled"
		default:
			return m, m.pollJob()
		}

	case scanErrorMsg:
		m.err = msg.err
		m.done = true
	}

	return m, nil
}

func (m scanModel) view() string {
	s := titleStyle.Render("Scan") + "\n\n"

	if m.err != nil {
		s += errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n"
		s += helpStyle.Render("Press q to go back")
		return s
	}

	if m.job != nil {
		s += fmt.Sprintf("Job:    %s\n", m.job.ID[:8])
	}
	s += fmt.Sprintf("Status: %s\n", m.status)
	s += fmt.Sprintf("Pages:  %d\n", m.pages)

	if m.progress > 0 && m.progress < 100 {
		barWidth := 30
		filled := barWidth * m.progress / 100
		bar := ""
		for i := 0; i < barWidth; i++ {
			if i < filled {
				bar += "█"
			} else {
				bar += "░"
			}
		}
		s += fmt.Sprintf("\n%s %d%%\n", bar, m.progress)
	}

	if m.done {
		if m.err == nil {
			s += "\n" + successStyle.Render("Document processed successfully!") + "\n"
		}
		s += helpStyle.Render("\nPress q to go back")
	} else {
		s += helpStyle.Render("\n[W] More pages  [F] Finish  [D] Delete  [Q] Cancel")
	}

	return s
}

func (m scanModel) continueScan() tea.Cmd {
	return func() tea.Msg {
		if err := m.client.ContinueScan(context.Background(), m.job.ID); err != nil {
			return scanErrorMsg{err: err}
		}
		return nil
	}
}

func (m scanModel) finishScan() tea.Cmd {
	return func() tea.Msg {
		output := &client.OutputConfig{Target: m.config.Defaults.Output}
		if err := m.client.FinishScan(context.Background(), m.job.ID, output, nil); err != nil {
			return scanErrorMsg{err: err}
		}
		return nil
	}
}

func (m scanModel) deletePage() tea.Cmd {
	return func() tea.Msg {
		if err := m.client.DeletePage(context.Background(), m.job.ID, m.pages); err != nil {
			return scanErrorMsg{err: err}
		}
		job, err := m.client.GetJobStatus(context.Background(), m.job.ID)
		if err != nil {
			return scanErrorMsg{err: err}
		}
		return scanUpdateMsg{job: job}
	}
}

func (m scanModel) pollJob() tea.Cmd {
	return func() tea.Msg {
		if m.job == nil {
			return nil
		}
		job, err := m.client.GetJobStatus(context.Background(), m.job.ID)
		if err != nil {
			return scanErrorMsg{err: err}
		}
		return scanUpdateMsg{job: job}
	}
}
