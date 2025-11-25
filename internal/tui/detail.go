package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ginbear/k8s-flowtop/internal/types"
)

var (
	detailBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("99")).
			Padding(1, 2).
			Width(60)

	detailTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("99")).
				MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(12)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))
)

// RenderDetail renders the detail view for a resource
func RenderDetail(r types.AsyncResource, width, height int) string {
	var b strings.Builder

	// Title
	b.WriteString(detailTitleStyle.Render(fmt.Sprintf("📋 %s: %s", r.Kind, r.Name)))
	b.WriteString("\n\n")

	// Basic info
	b.WriteString(renderField("Namespace", r.Namespace))
	b.WriteString(renderField("Status", formatDetailStatus(r.Status)))

	// Timing
	if r.StartTime != nil {
		b.WriteString(renderField("Started", r.StartTime.Format("2006-01-02 15:04:05")))
	}
	if r.EndTime != nil {
		b.WriteString(renderField("Ended", r.EndTime.Format("2006-01-02 15:04:05")))
	}
	if r.Duration > 0 {
		b.WriteString(renderField("Duration", formatDuration(r.Duration)))
	}

	// Schedule info
	if r.Schedule != "" {
		b.WriteString(renderField("Schedule", r.Schedule))
	}
	if r.LastRun != nil {
		b.WriteString(renderField("Last Run", r.LastRun.Format("2006-01-02 15:04:05")))
	}
	if r.NextRun != nil {
		b.WriteString(renderField("Next Run", r.NextRun.Format("2006-01-02 15:04:05")))
	}

	// Metrics
	if r.SuccessCount > 0 || r.FailureCount > 0 {
		b.WriteString("\n")
		b.WriteString(detailTitleStyle.Render("📊 Metrics"))
		b.WriteString("\n")
		b.WriteString(renderField("Success", fmt.Sprintf("%d", r.SuccessCount)))
		b.WriteString(renderField("Failures", fmt.Sprintf("%d", r.FailureCount)))
		if r.Retries > 0 {
			b.WriteString(renderField("Retries", fmt.Sprintf("%d / %d", r.Retries, r.MaxRetries)))
		}
		if r.Throughput > 0 {
			b.WriteString(renderField("Throughput", fmt.Sprintf("%.2f/min", r.Throughput)))
		}
		if r.QueueDepth > 0 {
			b.WriteString(renderField("Queue", fmt.Sprintf("%d", r.QueueDepth)))
		}
	}

	// Message
	if r.Message != "" {
		b.WriteString("\n")
		b.WriteString(detailTitleStyle.Render("💬 Message"))
		b.WriteString("\n")
		b.WriteString(wordWrap(r.Message, 50))
	}

	// Footer
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Press ESC or Enter to close"))

	content := detailBoxStyle.Render(b.String())

	// Center the box
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func renderField(label, value string) string {
	return labelStyle.Render(label+":") + " " + valueStyle.Render(value) + "\n"
}

func formatDetailStatus(s types.ResourceStatus) string {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Padding(0, 1)
	switch s {
	case types.StatusRunning:
		return base.Background(lipgloss.Color("24")).Render("● Running")
	case types.StatusSucceeded:
		return base.Background(lipgloss.Color("22")).Render("✓ Succeeded")
	case types.StatusFailed:
		return base.Background(lipgloss.Color("52")).Render("✗ Failed")
	case types.StatusPending:
		return base.Background(lipgloss.Color("58")).Render("○ Pending")
	default:
		return base.Background(lipgloss.Color("236")).Render("? Unknown")
	}
}

func wordWrap(text string, width int) string {
	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		if lineLen+len(word)+1 > width && lineLen > 0 {
			result.WriteString("\n")
			lineLen = 0
		}
		if i > 0 && lineLen > 0 {
			result.WriteString(" ")
			lineLen++
		}
		result.WriteString(word)
		lineLen += len(word)
	}

	return result.String()
}
