package commands

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"nexus/internal/config"
	"nexus/internal/ui"
)

// ShowCostDashboard displays the cost dashboard.
func (h *Handler) ShowCostDashboard() {
	costs := h.tracker.CalculateCosts()

	fmt.Println()
	fmt.Println(ui.StyleSection.Render("COST DASHBOARD"))
	fmt.Println()

	fmt.Println(ui.StyleSection.Render("SPENDING SUMMARY"))
	fmt.Println(h.ui.RenderProgressBar("Today    ", costs.Daily, h.cfg.DailyBudget))
	fmt.Println(h.ui.RenderProgressBar("This Week", costs.Weekly, h.cfg.WeeklyBudget))
	fmt.Println(h.ui.RenderProgressBar("This Month", costs.Monthly, h.cfg.MonthlyBudget))

	if len(costs.ByBackend) > 0 {
		fmt.Println()
		fmt.Println(ui.StyleSection.Render("BACKEND BREAKDOWN"))

		records := h.tracker.LoadAll()
		now := time.Now()
		today := now.Truncate(24 * time.Hour)
		weekStart := today.AddDate(0, 0, -int(today.Weekday()))
		monthStart := today.AddDate(0, 0, -today.Day()+1)

		backendDaily := make(map[string]float64)
		backendWeekly := make(map[string]float64)
		backendMonthly := make(map[string]float64)

		for _, r := range records {
			if r.Timestamp.After(monthStart) {
				backendMonthly[r.Backend] += r.CostUSD
			}
			if r.Timestamp.After(weekStart) {
				backendWeekly[r.Backend] += r.CostUSD
			}
			if r.Timestamp.Truncate(24 * time.Hour).Equal(today) {
				backendDaily[r.Backend] += r.CostUSD
			}
		}

		total := 0.0
		for _, cost := range costs.ByBackend {
			total += cost
		}

		rows := [][]string{}
		for name := range h.registry.GetAll() {
			if costs.ByBackend[name] == 0 {
				continue
			}
			be, _ := h.registry.Get(name)
			percent := costs.ByBackend[name] / total * 100
			rows = append(rows, []string{
				be.DisplayName,
				ui.FormatCurrency(backendDaily[name]),
				ui.FormatCurrency(backendWeekly[name]),
				ui.FormatCurrency(backendMonthly[name]),
				fmt.Sprintf("%.0f%%", percent),
			})
		}

		t := table.New().
			Headers("Backend", "Today", "This Week", "This Month", "%").
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
	}

	fmt.Println()
}

// ShowCostLog displays the detailed usage log.
func (h *Handler) ShowCostLog() {
	records := h.tracker.LoadAll()

	if len(records) == 0 {
		fmt.Println("No usage records found.")
		return
	}

	// Show last 20 records
	start := 0
	if len(records) > 20 {
		start = len(records) - 20
	}

	fmt.Println()
	fmt.Println(ui.StyleSection.Render("Recent Usage Records"))

	rows := [][]string{}
	for i := len(records) - 1; i >= start; i-- {
		r := records[i]
		sessionID := ui.Truncate(r.SessionID, 18)
		if sessionID == "" {
			sessionID = "-"
		}
		rows = append(rows, []string{
			r.Timestamp.Format("2006-01-02 15:04"),
			r.Backend,
			sessionID,
			fmt.Sprintf("%d", r.InputTokens),
			fmt.Sprintf("%d", r.OutputTokens),
			ui.FormatCurrency(r.CostUSD),
		})
	}

	t := table.New().
		Headers("Timestamp", "Backend", "Session", "Input", "Output", "Cost").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Width(100)

	fmt.Println(t.Render())
	fmt.Println()
}

// HandleBudgetCommand handles budget-related commands.
func (h *Handler) HandleBudgetCommand(args []string) {
	if len(args) == 0 {
		h.showBudgetStatus()
		return
	}

	subcmd := args[0]
	switch subcmd {
	case "status":
		h.showBudgetStatus()
	case "set":
		if len(args) < 3 {
			fmt.Println("Usage: promptops budget set <daily|weekly|monthly> <amount>")
			return
		}
		h.setBudget(args[1], args[2])
	default:
		fmt.Printf("Unknown budget command: %s\n", subcmd)
	}
}

func (h *Handler) showBudgetStatus() {
	costs := h.tracker.CalculateCosts()

	fmt.Println()
	fmt.Println(ui.StyleSection.Render("BUDGET STATUS"))
	fmt.Println()

	fmt.Println(h.ui.RenderProgressBar("Daily  ", costs.Daily, h.cfg.DailyBudget))
	fmt.Println(h.ui.RenderProgressBar("Weekly ", costs.Weekly, h.cfg.WeeklyBudget))
	fmt.Println(h.ui.RenderProgressBar("Monthly", costs.Monthly, h.cfg.MonthlyBudget))

	fmt.Println()
}

func (h *Handler) setBudget(period, amountStr string) {
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		fmt.Printf("Error: Invalid amount: %s\n", amountStr)
		return
	}

	varKey := ""
	switch period {
	case "daily":
		varKey = "NEXUS_DAILY_BUDGET"
	case "weekly":
		varKey = "NEXUS_WEEKLY_BUDGET"
	case "monthly":
		varKey = "NEXUS_MONTHLY_BUDGET"
	default:
		fmt.Printf("Error: Invalid period '%s'. Use daily, weekly, or monthly.\n", period)
		return
	}

	// Read current content
	content, err := os.ReadFile(h.cfg.EnvFile)
	if err != nil {
		fmt.Printf("Error reading .env.local: %v\n", err)
		return
	}

	lines := splitLines(string(content))
	found := false
	newLine := fmt.Sprintf("%s=%.2f", varKey, amount)

	for i, line := range lines {
		if len(line) >= len(varKey)+1 && line[:len(varKey)+1] == varKey+"=" {
			lines[i] = newLine
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, newLine)
	}

	newContent := joinLines(lines)
	if err := config.WriteFileAtomic(h.cfg.EnvFile, []byte(newContent), 0600); err != nil {
		fmt.Printf("Error: failed to update configuration: %v\n", err)
		return
	}

	fmt.Printf("[OK] Set %s budget to %s\n", period, ui.FormatCurrency(amount))
}

func splitLines(s string) []string {
	var lines []string
	var current []rune
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, string(current))
			current = nil
		} else {
			current = append(current, r)
		}
	}
	if len(current) > 0 {
		lines = append(lines, string(current))
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
