package commands

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"nexus/internal/ui"
)

// RunDoctor runs health checks on all backends.
func (h *Handler) RunDoctor() {
	fmt.Println()
	fmt.Println(ui.StyleSection.Render("ENVIRONMENT HEALTH CHECK"))
	fmt.Println()

	rows := [][]string{}
	for _, name := range h.registry.GetOrdered() {
		be, ok := h.registry.Get(name)
		if !ok {
			continue
		}
		result := h.registry.CheckHealth(h.cfg, be)

		statusStr := ""
		switch result.Status {
		case "ok":
			statusStr = ui.StyleSuccess.Render("OK")
		case "skip":
			statusStr = ui.StyleMuted.Render("SKIP")
		case "error":
			statusStr = ui.StyleError.Render("FAIL")
		}

		latencyStr := "--"
		if result.Latency > 0 {
			latencyStr = formatDuration(result.Latency)
		}

		rows = append(rows, []string{
			be.DisplayName,
			statusStr,
			latencyStr,
			ui.Truncate(result.Message, 35),
		})
	}

	t := table.New().
		Headers("Backend", "Status", "Latency", "Message").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Width(80)

	fmt.Println(t.Render())
	fmt.Println()
}

// ValidateBackend validates a specific backend.
func (h *Handler) ValidateBackend(name string) {
	be, ok := h.registry.Get(name)
	if !ok {
		fmt.Printf("Error: Unknown backend '%s'\n", name)
		return
	}

	fmt.Printf("Validating %s...\n", be.DisplayName)
	result := h.registry.CheckHealth(h.cfg, be)

	switch result.Status {
	case "ok":
		fmt.Printf("[OK] %s is healthy (latency: %s)\n", be.DisplayName, formatDuration(result.Latency))
	case "skip":
		fmt.Printf("[--] %s - %s\n", be.DisplayName, result.Message)
	case "error":
		fmt.Printf("[FAIL] %s - %s\n", be.DisplayName, result.Message)
	}
}
