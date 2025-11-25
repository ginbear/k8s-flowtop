package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
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

	// Status background colors
	runningBg   = lipgloss.Color("24")  // dark blue
	succeededBg = lipgloss.Color("22")  // dark green
	failedBg    = lipgloss.Color("52")  // dark red
	pendingBg   = lipgloss.Color("58")  // dark yellow/olive
	unknownBg   = lipgloss.Color("236") // dark gray

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Padding(0, 1)

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57")).
				Bold(true).
				Padding(0, 1)

	cellStyle = lipgloss.NewStyle().
			Padding(0, 1)

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

// Column widths
var colWidths = []int{12, 15, 30, 12, 10, 15, 25}
var colHeaders = []string{"KIND", "NAMESPACE", "NAME", "STATUS", "DURATION", "SCHEDULE", "MESSAGE"}

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
	k8sClient        *k8s.Client
	resources        []types.AsyncResource
	filteredCache    []types.AsyncResource
	cursor           int
	viewMode         types.ViewMode
	help             help.Model
	keys             KeyMap
	showHelp         bool
	showDetail       bool
	selectedResource *types.AsyncResource
	err              error
	width            int
	height           int
	lastUpdate       time.Time
}

// Messages
type tickMsg time.Time
type resourcesMsg []types.AsyncResource
type errMsg struct{ error }

// NewModel creates a new TUI model
func NewModel(client *k8s.Client) Model {
	return Model{
		k8sClient: client,
		viewMode:  types.ViewAll,
		help:      help.New(),
		keys:      keys,
		showHelp:  false,
		cursor:    0,
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
		// Handle detail view escape
		if m.showDetail {
			switch msg.String() {
			case "esc", "enter", "q":
				m.showDetail = false
				m.selectedResource = nil
				return m, nil
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil

		case key.Matches(msg, m.keys.Refresh):
			return m, m.fetchResources()

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.filteredCache)-1 {
				m.cursor++
			}
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			// Show detail view
			if m.cursor >= 0 && m.cursor < len(m.filteredCache) {
				r := m.filteredCache[m.cursor]
				m.selectedResource = &r
				m.showDetail = true
			}
			return m, nil

		case key.Matches(msg, m.keys.Tab):
			m.viewMode = (m.viewMode + 1) % 4
			m.updateFiltered()
			return m, nil

		case key.Matches(msg, m.keys.ShiftTab):
			if m.viewMode == 0 {
				m.viewMode = 3
			} else {
				m.viewMode--
			}
			m.updateFiltered()
			return m, nil

		case key.Matches(msg, m.keys.All):
			m.viewMode = types.ViewAll
			m.updateFiltered()
			return m, nil

		case key.Matches(msg, m.keys.Jobs):
			m.viewMode = types.ViewJobs
			m.updateFiltered()
			return m, nil

		case key.Matches(msg, m.keys.Flows):
			m.viewMode = types.ViewWorkflows
			m.updateFiltered()
			return m, nil

		case key.Matches(msg, m.keys.Events):
			m.viewMode = types.ViewEvents
			m.updateFiltered()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil

	case tickMsg:
		cmds = append(cmds, m.fetchResources(), m.tickCmd())

	case resourcesMsg:
		m.resources = msg
		m.lastUpdate = time.Now()
		m.updateFiltered()

	case errMsg:
		m.err = msg.error
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateFiltered() {
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

	m.filteredCache = filtered

	// Adjust cursor if needed
	if m.cursor >= len(m.filteredCache) {
		m.cursor = len(m.filteredCache) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
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

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	// Show detail view if active
	if m.showDetail && m.selectedResource != nil {
		return RenderDetail(*m.selectedResource, m.width, m.height)
	}

	// Title
	title := titleStyle.Render("🔄 k8s-flowtop - Async Processing Monitor")

	// Tabs
	tabs := m.renderTabs()

	// Table
	tableView := m.renderTable()

	// Status bar
	ns := m.k8sClient.GetNamespace()
	if ns == "" {
		ns = "all namespaces"
	}
	statusBar := statusBarStyle.Render(fmt.Sprintf(
		"Namespace: %s | Resources: %d | Updated: %s",
		ns,
		len(m.filteredCache),
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

func (m Model) renderTable() string {
	var b strings.Builder

	width := m.width
	if width <= 0 {
		width = 120
	}

	// Header - clip to screen width
	header := m.renderHeader()
	b.WriteString(clipToWidth(header, width))
	b.WriteString("\n")

	// Calculate visible rows
	maxRows := m.height - 10
	if maxRows < 5 {
		maxRows = 5
	}

	startIdx := 0
	if m.cursor >= maxRows {
		startIdx = m.cursor - maxRows + 1
	}

	endIdx := startIdx + maxRows
	if endIdx > len(m.filteredCache) {
		endIdx = len(m.filteredCache)
	}

	// Rows
	for i := startIdx; i < endIdx; i++ {
		r := m.filteredCache[i]
		isSelected := i == m.cursor

		row := m.renderRow(r, isSelected)
		b.WriteString(clipToWidth(row, width))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderHeader() string {
	var result strings.Builder
	for i, h := range colHeaders {
		result.WriteString(headerStyle.Render(padRight(h, colWidths[i])))
	}
	return result.String()
}

// clipToWidth clips a string to the given width, accounting for ANSI codes
func clipToWidth(s string, width int) string {
	// Use lipgloss to handle ANSI-aware width
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func (m Model) renderRow(r types.AsyncResource, isSelected bool) string {
	duration := "-"
	if r.Duration > 0 {
		duration = formatDuration(r.Duration)
	}

	schedule := r.Schedule
	if schedule == "" {
		schedule = "-"
	}

	msg := r.Message
	if len(msg) > colWidths[6]-2 {
		msg = msg[:colWidths[6]-5] + "..."
	}
	if msg == "" {
		msg = "-"
	}

	cells := []string{
		padRight(string(r.Kind), colWidths[0]),
		padRight(truncate(r.Namespace, colWidths[1]-2), colWidths[1]),
		padRight(truncate(r.Name, colWidths[2]-2), colWidths[2]),
		padRight(formatStatusText(r.Status), colWidths[3]),
		padRight(duration, colWidths[4]),
		padRight(truncate(schedule, colWidths[5]-2), colWidths[5]),
		padRight(msg, colWidths[6]),
	}

	var result strings.Builder
	for i, cell := range cells {
		if isSelected {
			result.WriteString(selectedRowStyle.Render(cell))
		} else if i == 3 {
			// Status column - apply background color
			result.WriteString(getStatusStyle(r.Status).Render(cell))
		} else {
			result.WriteString(cellStyle.Render(cell))
		}
	}

	return result.String()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func getStatusStyle(s types.ResourceStatus) lipgloss.Style {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Padding(0, 1)
	switch s {
	case types.StatusRunning:
		return base.Background(runningBg)
	case types.StatusSucceeded:
		return base.Background(succeededBg)
	case types.StatusFailed:
		return base.Background(failedBg)
	case types.StatusPending:
		return base.Background(pendingBg)
	default:
		return base.Background(unknownBg)
	}
}

func formatStatusText(s types.ResourceStatus) string {
	switch s {
	case types.StatusRunning:
		return "● Running"
	case types.StatusSucceeded:
		return "✓ Succeeded"
	case types.StatusFailed:
		return "✗ Failed"
	case types.StatusPending:
		return "○ Pending"
	default:
		return "? Unknown"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
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
