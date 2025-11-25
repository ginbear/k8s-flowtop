package tui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ginbear/k8s-flowtop/internal/k8s"
	"github.com/ginbear/k8s-flowtop/internal/types"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			MarginBottom(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	runningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))  // blue
	succeededStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // green
	failedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	pendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // yellow
	unknownStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // gray

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Bold(true)

	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Padding(0, 2)

	tabInactiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Background(lipgloss.Color("236")).
			Padding(0, 2)
)

// KeyMap defines the keybindings
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Refresh  key.Binding
	Quit     key.Binding
	Help     key.Binding
	Enter    key.Binding
	All      key.Binding
	Jobs     key.Binding
	Flows    key.Binding
	Events   key.Binding
}

var keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next view"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev view"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "details"),
	),
	All: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "all"),
	),
	Jobs: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "jobs"),
	),
	Flows: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "workflows"),
	),
	Events: key.NewBinding(
		key.WithKeys("4"),
		key.WithHelp("4", "events"),
	),
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tab, k.Refresh, k.Quit, k.Help}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Tab, k.ShiftTab},
		{k.All, k.Jobs, k.Flows, k.Events},
		{k.Refresh, k.Enter, k.Quit, k.Help},
	}
}

// Model is the main TUI model
type Model struct {
	k8sClient  *k8s.Client
	resources  []types.AsyncResource
	table      table.Model
	viewMode   types.ViewMode
	help       help.Model
	keys       KeyMap
	showHelp   bool
	err        error
	width      int
	height     int
	lastUpdate time.Time
}

// Messages
type tickMsg time.Time
type resourcesMsg []types.AsyncResource
type errMsg struct{ error }

// NewModel creates a new TUI model
func NewModel(client *k8s.Client) Model {
	columns := []table.Column{
		{Title: "KIND", Width: 12},
		{Title: "NAMESPACE", Width: 15},
		{Title: "NAME", Width: 30},
		{Title: "STATUS", Width: 12},
		{Title: "DURATION", Width: 12},
		{Title: "SCHEDULE", Width: 15},
		{Title: "MESSAGE", Width: 30},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = headerStyle
	s.Selected = selectedStyle
	t.SetStyles(s)

	return Model{
		k8sClient: client,
		table:     t,
		viewMode:  types.ViewAll,
		help:      help.New(),
		keys:      keys,
		showHelp:  false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.fetchResources(),
		m.tickCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, m.keys.Refresh):
			return m, m.fetchResources()

		case key.Matches(msg, m.keys.Tab):
			m.viewMode = (m.viewMode + 1) % 4
			m.updateTable()
			return m, nil

		case key.Matches(msg, m.keys.ShiftTab):
			if m.viewMode == 0 {
				m.viewMode = 3
			} else {
				m.viewMode--
			}
			m.updateTable()
			return m, nil

		case key.Matches(msg, m.keys.All):
			m.viewMode = types.ViewAll
			m.updateTable()
			return m, nil

		case key.Matches(msg, m.keys.Jobs):
			m.viewMode = types.ViewJobs
			m.updateTable()
			return m, nil

		case key.Matches(msg, m.keys.Flows):
			m.viewMode = types.ViewWorkflows
			m.updateTable()
			return m, nil

		case key.Matches(msg, m.keys.Events):
			m.viewMode = types.ViewEvents
			m.updateTable()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(msg.Height - 8)
		m.help.Width = msg.Width
		return m, nil

	case tickMsg:
		cmds = append(cmds, m.fetchResources(), m.tickCmd())

	case resourcesMsg:
		m.resources = msg
		m.lastUpdate = time.Now()
		m.updateTable()

	case errMsg:
		m.err = msg.error
	}

	// Update table
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateTable() {
	var rows []table.Row
	filtered := m.filterResources()

	// Sort by status priority then name
	sort.Slice(filtered, func(i, j int) bool {
		pi := statusPriority(filtered[i].Status)
		pj := statusPriority(filtered[j].Status)
		if pi != pj {
			return pi < pj
		}
		return filtered[i].Name < filtered[j].Name
	})

	for _, r := range filtered {
		rows = append(rows, resourceToRow(r))
	}

	m.table.SetRows(rows)
}

func (m Model) filterResources() []types.AsyncResource {
	if m.viewMode == types.ViewAll {
		return m.resources
	}

	var filtered []types.AsyncResource
	for _, r := range m.resources {
		switch m.viewMode {
		case types.ViewJobs:
			if r.Kind == types.KindJob || r.Kind == types.KindCronJob {
				filtered = append(filtered, r)
			}
		case types.ViewWorkflows:
			if r.Kind == types.KindWorkflow || r.Kind == types.KindCronWorkflow {
				filtered = append(filtered, r)
			}
		case types.ViewEvents:
			if r.Kind == types.KindSensor || r.Kind == types.KindEventSource {
				filtered = append(filtered, r)
			}
		}
	}
	return filtered
}

func statusPriority(s types.ResourceStatus) int {
	switch s {
	case types.StatusRunning:
		return 0
	case types.StatusFailed:
		return 1
	case types.StatusPending:
		return 2
	case types.StatusSucceeded:
		return 3
	default:
		return 4
	}
}

func resourceToRow(r types.AsyncResource) table.Row {
	duration := "-"
	if r.Duration > 0 {
		duration = formatDuration(r.Duration)
	}

	schedule := r.Schedule
	if schedule == "" {
		schedule = "-"
	}

	msg := r.Message
	if len(msg) > 30 {
		msg = msg[:27] + "..."
	}
	if msg == "" {
		msg = "-"
	}

	return table.Row{
		string(r.Kind),
		r.Namespace,
		r.Name,
		formatStatus(r.Status),
		duration,
		schedule,
		msg,
	}
}

func formatStatus(s types.ResourceStatus) string {
	switch s {
	case types.StatusRunning:
		return runningStyle.Render(string(s))
	case types.StatusSucceeded:
		return succeededStyle.Render(string(s))
	case types.StatusFailed:
		return failedStyle.Render(string(s))
	case types.StatusPending:
		return pendingStyle.Render(string(s))
	default:
		return unknownStyle.Render(string(s))
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	// Title
	title := titleStyle.Render("🔄 k8s-flowtop - Async Processing Monitor")

	// Tabs
	tabs := m.renderTabs()

	// Table
	tableView := m.table.View()

	// Status bar
	ns := m.k8sClient.GetNamespace()
	if ns == "" {
		ns = "all namespaces"
	}
	statusBar := statusBarStyle.Render(fmt.Sprintf(
		"Namespace: %s | Resources: %d | Updated: %s",
		ns,
		len(m.filterResources()),
		m.lastUpdate.Format("15:04:05"),
	))

	// Help
	var helpView string
	if m.showHelp {
		helpView = m.help.View(m.keys)
	} else {
		helpView = m.help.ShortHelpView(m.keys.ShortHelp())
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		tabs,
		tableView,
		statusBar,
		helpView,
	)
}

func (m Model) renderTabs() string {
	tabs := []string{"All", "Jobs", "Workflows", "Events"}
	var rendered []string

	for i, tab := range tabs {
		if types.ViewMode(i) == m.viewMode {
			rendered = append(rendered, tabActiveStyle.Render(fmt.Sprintf("%d:%s", i+1, tab)))
		} else {
			rendered = append(rendered, tabInactiveStyle.Render(fmt.Sprintf("%d:%s", i+1, tab)))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func (m Model) fetchResources() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resources, err := m.k8sClient.ListAll(ctx)
		if err != nil {
			return errMsg{err}
		}
		return resourcesMsg(resources)
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
