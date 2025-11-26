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
	"github.com/robfig/cron/v3"
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

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// Column definitions per view mode
// All view: simple overview
var colWidthsAll = []int{14, 15, 45, 12, 10, 30}
var colHeadersAll = []string{"KIND", "NAMESPACE", "NAME", "STATUS", "DURATION", "MESSAGE"}

// Jobs/Workflows view: schedule-focused
var colWidthsJobs = []int{14, 15, 38, 12, 10, 5, 5, 5, 5, 5, 12, 13, 13, 20}
var colHeadersJobs = []string{"KIND", "NAMESPACE", "NAME", "STATUS", "DURATION", "MIN", "HRS", "DAY", "MON", "DOW", "TZ", "LAST", "NEXT", "MESSAGE"}

// Events view: event-focused
var colWidthsEvents = []int{13, 15, 32, 10, 22, 40, 40}
var colHeadersEvents = []string{"KIND", "NAMESPACE", "NAME", "STATUS", "EVENT_SOURCE", "EVENT_NAME", "TRIGGER"}

// SortMode represents the current sort mode
type SortMode int

const (
	SortByStatus SortMode = iota
	SortByNextRun
)

func (s SortMode) String() string {
	switch s {
	case SortByNextRun:
		return "next"
	default:
		return "status"
	}
}

// KeyMap defines the keybindings
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Tab        key.Binding
	ShiftTab   key.Binding
	Refresh    key.Binding
	Quit       key.Binding
	Help       key.Binding
	Enter      key.Binding
	All        key.Binding
	Jobs       key.Binding
	Flows      key.Binding
	Events     key.Binding
	ToggleJST  key.Binding
	ToggleSort key.Binding
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
	ToggleJST: key.NewBinding(
		key.WithKeys("J"),
		key.WithHelp("J", "toggle JST/UTC"),
	),
	ToggleSort: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "sort by next run"),
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
	treePrefixes     []string // tree prefix for each item in filteredCache
	cursor           int
	viewMode         types.ViewMode
	sortMode         SortMode
	help             help.Model
	keys             KeyMap
	showHelp         bool
	showDetail       bool
	selectedResource *types.AsyncResource
	err              error
	width            int
	height           int
	lastUpdate       time.Time
	useJST           bool
	jstLocation      *time.Location
}

// Messages
type tickMsg time.Time
type resourcesMsg []types.AsyncResource
type errMsg struct{ error }

// NewModel creates a new TUI model
func NewModel(client *k8s.Client) Model {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	return Model{
		k8sClient:   client,
		viewMode:    types.ViewAll,
		help:        help.New(),
		keys:        keys,
		showHelp:    false,
		cursor:      0,
		useJST:      false,
		jstLocation: jst,
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

		case key.Matches(msg, m.keys.ToggleJST):
			m.useJST = !m.useJST
			return m, nil

		case key.Matches(msg, m.keys.ToggleSort):
			if m.sortMode == SortByStatus {
				m.sortMode = SortByNextRun
			} else {
				m.sortMode = SortByStatus
			}
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

	// Separate parents and children
	var parents []types.AsyncResource
	childrenMap := make(map[string][]types.AsyncResource) // key: "namespace/parentName"

	for _, r := range filtered {
		if r.ParentName != "" {
			key := r.Namespace + "/" + r.ParentName
			childrenMap[key] = append(childrenMap[key], r)
		} else {
			parents = append(parents, r)
		}
	}

	// Sort parents based on sort mode
	switch m.sortMode {
	case SortByNextRun:
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		sort.Slice(parents, func(i, j int) bool {
			nextI := m.getNextRunTimeValue(parents[i].Schedule, parents[i].Timezone, parser)
			nextJ := m.getNextRunTimeValue(parents[j].Schedule, parents[j].Timezone, parser)
			if nextI.IsZero() && nextJ.IsZero() {
				return parents[i].Name < parents[j].Name
			}
			if nextI.IsZero() {
				return false
			}
			if nextJ.IsZero() {
				return true
			}
			return nextI.Before(nextJ)
		})
	default:
		sort.Slice(parents, func(i, j int) bool {
			pi := statusPriority(parents[i].Status)
			pj := statusPriority(parents[j].Status)
			if pi != pj {
				return pi < pj
			}
			return parents[i].Name < parents[j].Name
		})
	}

	// Sort children by start time (newest first) or name
	for key := range childrenMap {
		children := childrenMap[key]
		sort.Slice(children, func(i, j int) bool {
			// Sort by start time descending (newest first)
			if children[i].StartTime != nil && children[j].StartTime != nil {
				return children[i].StartTime.After(*children[j].StartTime)
			}
			if children[i].StartTime != nil {
				return true
			}
			if children[j].StartTime != nil {
				return false
			}
			return children[i].Name > children[j].Name
		})
		childrenMap[key] = children
	}

	// Build final list with tree structure
	var result []types.AsyncResource
	var prefixes []string

	for _, parent := range parents {
		result = append(result, parent)
		prefixes = append(prefixes, "")

		key := parent.Namespace + "/" + parent.Name
		children := childrenMap[key]
		for i, child := range children {
			result = append(result, child)
			if i == len(children)-1 {
				prefixes = append(prefixes, "┗ ")
			} else {
				prefixes = append(prefixes, "┣ ")
			}
		}
		// Remove used children
		delete(childrenMap, key)
	}

	// Add orphan children (whose parent is not in filtered list)
	for _, children := range childrenMap {
		for _, child := range children {
			result = append(result, child)
			prefixes = append(prefixes, "")
		}
	}

	m.filteredCache = result
	m.treePrefixes = prefixes

	// Adjust cursor if needed
	if m.cursor >= len(m.filteredCache) {
		m.cursor = len(m.filteredCache) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// getNextRunTimeValue returns the next run time as time.Time for sorting
// If timezone is specified, schedule is interpreted in that timezone
func (m *Model) getNextRunTimeValue(schedule, timezone string, parser cron.Parser) time.Time {
	if schedule == "" {
		return time.Time{}
	}
	sched, err := parser.Parse(schedule)
	if err != nil {
		return time.Time{}
	}

	// Determine the timezone for schedule interpretation
	var now time.Time
	if timezone != "" {
		loc, err := time.LoadLocation(timezone)
		if err == nil {
			now = time.Now().In(loc)
		} else {
			now = time.Now().UTC()
		}
	} else {
		now = time.Now().UTC()
	}

	return sched.Next(now)
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

	// Context, Cluster, Namespace, and status info
	infoLine := m.renderInfoLine()

	// Separator line
	width := m.width
	if width <= 0 {
		width = 80
	}
	separator := separatorStyle.Render(strings.Repeat("─", width))

	// Tabs
	tabs := m.renderTabs()

	// Table
	tableView := m.renderTable()

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
		infoLine,
		separator,
		tabs,
		tableView,
		helpView,
	)
}

func (m Model) renderInfoLine() string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ctxStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	clusterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	nsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("159")).Bold(true)
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("156"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	tzStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	ctx := m.k8sClient.GetContext()
	cluster := m.k8sClient.GetCluster()
	ns := m.k8sClient.GetNamespace()
	if ns == "" {
		ns = "all"
	}

	tz := "UTC"
	if m.useJST {
		tz = "JST"
	}

	sortStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("171")).Bold(true)

	return fmt.Sprintf("%s %s  %s %s  %s %s  %s %s  %s %s  %s %s  %s %s",
		labelStyle.Render("ctx:"),
		ctxStyle.Render(ctx),
		labelStyle.Render("cluster:"),
		clusterStyle.Render(cluster),
		labelStyle.Render("ns:"),
		nsStyle.Render(ns),
		labelStyle.Render("resources:"),
		countStyle.Render(fmt.Sprintf("%d", len(m.filteredCache))),
		labelStyle.Render("tz:"),
		tzStyle.Render(tz),
		labelStyle.Render("sort:"),
		sortStyle.Render(m.sortMode.String()),
		labelStyle.Render("updated:"),
		timeStyle.Render(m.lastUpdate.Format("15:04:05")),
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
		prefix := ""
		if i < len(m.treePrefixes) {
			prefix = m.treePrefixes[i]
		}

		row := m.renderRow(r, isSelected, prefix)
		b.WriteString(clipToWidth(row, width))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) getColumnConfig() ([]int, []string) {
	switch m.viewMode {
	case types.ViewEvents:
		return colWidthsEvents, colHeadersEvents
	case types.ViewAll:
		return colWidthsAll, colHeadersAll
	default: // ViewJobs, ViewWorkflows
		return colWidthsJobs, colHeadersJobs
	}
}

func (m Model) renderHeader() string {
	tz := "UTC"
	if m.useJST {
		tz = "JST"
	}

	colWidths, colHeaders := m.getColumnConfig()

	var result strings.Builder
	for i, h := range colHeaders {
		header := h
		// Add timezone to LAST and NEXT columns (Jobs/Workflows view)
		if m.viewMode != types.ViewEvents && m.viewMode != types.ViewAll {
			if header == "LAST" {
				header = fmt.Sprintf("LAST(%s)", tz)
			} else if header == "NEXT" {
				header = fmt.Sprintf("NEXT(%s)", tz)
			}
		}
		result.WriteString(headerStyle.Render(padRight(header, colWidths[i])))
	}
	return result.String()
}

// clipToWidth clips a string to the given width, accounting for ANSI codes
func clipToWidth(s string, width int) string {
	// Use lipgloss to handle ANSI-aware width
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func (m Model) renderRow(r types.AsyncResource, isSelected bool, treePrefix string) string {
	colWidths, _ := m.getColumnConfig()

	duration := "-"
	if r.Duration > 0 {
		duration = formatDuration(r.Duration)
	}

	// Build KIND column with tree prefix
	kindStr := treePrefix + string(r.Kind)

	var cells []string
	statusColIdx := 3 // Status column index for coloring

	switch m.viewMode {
	case types.ViewAll:
		// All view: KIND, NAMESPACE, NAME, STATUS, DURATION, MESSAGE
		msg := truncateMsg(r.Message, colWidths[5]-2)
		cells = []string{
			padRight(kindStr, colWidths[0]),
			padRight(truncate(r.Namespace, colWidths[1]-2), colWidths[1]),
			padRight(truncate(r.Name, colWidths[2]-2), colWidths[2]),
			padRight(formatStatusText(r.Status), colWidths[3]),
			padRight(duration, colWidths[4]),
			padRight(msg, colWidths[5]),
		}

	case types.ViewEvents:
		// Events view: KIND, NAMESPACE, NAME, STATUS, EVENT_SOURCE, EVENT_NAME, TRIGGER
		eventSource := r.EventSourceName
		if eventSource == "" {
			eventSource = "-"
		}
		eventName := "-"
		if len(r.EventNames) > 0 {
			eventName = r.EventNames[0]
			if len(r.EventNames) > 1 {
				eventName += fmt.Sprintf(" (+%d)", len(r.EventNames)-1)
			}
		}
		trigger := "-"
		if len(r.TriggerNames) > 0 {
			trigger = r.TriggerNames[0]
			if len(r.TriggerNames) > 1 {
				trigger += fmt.Sprintf(" (+%d)", len(r.TriggerNames)-1)
			}
		}
		// For EventSource, show event type instead
		if r.Kind == types.KindEventSource {
			eventName = r.EventType
			if eventName == "" {
				eventName = "-"
			}
			trigger = "-"
		}
		cells = []string{
			padRight(kindStr, colWidths[0]),
			padRight(truncate(r.Namespace, colWidths[1]-2), colWidths[1]),
			padRight(truncate(r.Name, colWidths[2]-2), colWidths[2]),
			padRight(formatStatusText(r.Status), colWidths[3]),
			padRight(truncate(eventSource, colWidths[4]-2), colWidths[4]),
			padRight(truncate(eventName, colWidths[5]-2), colWidths[5]),
			padRight(truncate(trigger, colWidths[6]-2), colWidths[6]),
		}

	default: // ViewJobs, ViewWorkflows
		// Jobs/Workflows view: full schedule columns
		cronFields := parseCronFields(r.Schedule)
		lastRun := m.formatTime(r.LastRun)
		nextRun := m.getNextRunTime(r.Schedule, r.Timezone)

		tz := r.Timezone
		if tz == "" {
			tz = "-"
		} else {
			tz = strings.TrimPrefix(tz, "Asia/")
			tz = strings.TrimPrefix(tz, "America/")
			tz = strings.TrimPrefix(tz, "Europe/")
		}

		msg := truncateMsg(r.Message, colWidths[13]-2)
		cells = []string{
			padRight(kindStr, colWidths[0]),
			padRight(truncate(r.Namespace, colWidths[1]-2), colWidths[1]),
			padRight(truncate(r.Name, colWidths[2]-2), colWidths[2]),
			padRight(formatStatusText(r.Status), colWidths[3]),
			padRight(duration, colWidths[4]),
			padCenter(cronFields[0], colWidths[5]),  // MIN
			padCenter(cronFields[1], colWidths[6]),  // HRS
			padCenter(cronFields[2], colWidths[7]),  // DAY
			padCenter(cronFields[3], colWidths[8]),  // MON
			padCenter(cronFields[4], colWidths[9]),  // DOW
			padRight(tz, colWidths[10]),             // TZ
			padRight(lastRun, colWidths[11]),        // LAST
			padRight(nextRun, colWidths[12]),        // NEXT
			padRight(msg, colWidths[13]),
		}
	}

	var result strings.Builder
	for i, cell := range cells {
		if isSelected {
			result.WriteString(selectedRowStyle.Render(cell))
		} else if i == statusColIdx {
			// Status column - apply background color
			result.WriteString(getStatusStyle(r.Status).Render(cell))
		} else {
			result.WriteString(cellStyle.Render(cell))
		}
	}

	return result.String()
}

func truncateMsg(msg string, maxLen int) string {
	if msg == "" {
		return "-"
	}
	if len(msg) > maxLen {
		return msg[:maxLen-3] + "..."
	}
	return msg
}

// parseCronFields splits a cron expression into 5 fields
func parseCronFields(schedule string) []string {
	empty := []string{"-", "-", "-", "-", "-"}
	if schedule == "" {
		return empty
	}

	fields := strings.Fields(schedule)
	if len(fields) < 5 {
		return empty
	}

	// Return first 5 fields (min, hrs, day, mon, dow)
	return fields[:5]
}

// formatTime formats a time pointer with timezone consideration
func (m Model) formatTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	tt := *t
	if m.useJST && m.jstLocation != nil {
		tt = tt.In(m.jstLocation)
	} else {
		tt = tt.UTC()
	}
	return tt.Format("01/02 15:04")
}

// getNextRunTime calculates the next run time from a cron expression
// If timezone is specified (e.g., "Asia/Tokyo"), schedule is interpreted in that timezone
// Otherwise, schedule is interpreted in UTC (Kubernetes default)
func (m Model) getNextRunTime(schedule, timezone string) string {
	if schedule == "" {
		return "-"
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return "-"
	}

	// Determine the timezone for schedule interpretation
	var now time.Time
	if timezone != "" {
		loc, err := time.LoadLocation(timezone)
		if err == nil {
			now = time.Now().In(loc)
		} else {
			now = time.Now().UTC()
		}
	} else {
		now = time.Now().UTC()
	}

	// Calculate next run in the schedule's timezone
	next := sched.Next(now)

	// Convert to display timezone
	if m.useJST && m.jstLocation != nil {
		next = next.In(m.jstLocation)
	} else {
		next = next.UTC()
	}
	return next.Format("01/02 15:04")
}

// padCenter pads a string to center it within the given width
func padCenter(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	leftPad := (width - w) / 2
	rightPad := width - w - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
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
