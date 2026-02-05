package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thoscut/scanflow/client/internal/client"
	"github.com/thoscut/scanflow/client/internal/config"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginBottom(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)

// App is the main TUI application.
type App struct {
	client *client.Client
	config *config.Config
}

// New creates a new TUI application.
func New(c *client.Client, cfg *config.Config) *App {
	return &App{
		client: c,
		config: cfg,
	}
}

// Run starts the TUI application.
func (a *App) Run() error {
	model := newMainModel(a.client, a.config)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// View represents the current TUI view.
type view int

const (
	viewMain view = iota
	viewScan
	viewDevices
	viewProfiles
)

// mainModel is the top-level TUI model.
type mainModel struct {
	client    *client.Client
	config    *config.Config
	view      view
	scanModel scanModel
	cursor    int
	width     int
	height    int
	err       error
}

func newMainModel(c *client.Client, cfg *config.Config) mainModel {
	return mainModel{
		client: c,
		config: cfg,
		view:   viewMain,
	}
}

func (m mainModel) Init() tea.Cmd {
	return nil
}

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.view == viewScan {
			var cmd tea.Cmd
			m.scanModel, cmd = m.scanModel.update(msg)
			if m.scanModel.quitting {
				m.view = viewMain
				return m, nil
			}
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < 3 {
				m.cursor++
			}

		case "enter":
			switch m.cursor {
			case 0: // Scan
				m.view = viewScan
				m.scanModel = newScanModel(m.client, m.config)
				return m, m.scanModel.init()
			case 1: // Devices
				// Show devices (simplified)
			case 2: // Profiles
				// Show profiles (simplified)
			case 3: // Quit
				return m, tea.Quit
			}
		}

	case errMsg:
		m.err = msg.err
	}

	return m, nil
}

func (m mainModel) View() string {
	if m.view == viewScan {
		return m.scanModel.view()
	}

	s := titleStyle.Render("ScanFlow") + "\n\n"

	if m.err != nil {
		s += errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n"
	}

	menuItems := []string{
		"Scan starten",
		"Scanner anzeigen",
		"Profile anzeigen",
		"Beenden",
	}

	for i, item := range menuItems {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
			item = lipgloss.NewStyle().Bold(true).Render(item)
		}
		s += fmt.Sprintf("%s%s\n", cursor, item)
	}

	s += helpStyle.Render("\n↑/↓ Navigate  Enter Select  q Quit")

	return s
}

type errMsg struct {
	err error
}
