package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"nexus/internal/backend"
	"nexus/internal/ui"
	"nexus/internal/usage"
)

// ShowAPIUsage displays API usage from provider APIs.
func (h *Handler) ShowAPIUsage(args []string) {
	// If specific backend requested
	if len(args) > 0 {
		backendName := args[0]
		be, ok := h.registry.Get(backendName)
		if !ok {
			fmt.Printf("Error: Unknown backend '%s'\n", backendName)
			return
		}

		apiKey := h.cfg.Keys[be.AuthVar]
		if apiKey == "" && be.Name != "ollama" {
			fmt.Printf("Error: No API key configured for %s\n", be.DisplayName)
			return
		}

		fmt.Println()
		fmt.Printf("Fetching usage for %s...\n", be.DisplayName)
		info := fetchUsageForBackend(be, apiKey)
		displayUsage(info, h.registry)
		return
	}

	// Show usage for all configured backends
	fmt.Println()
	title := ui.StyleTitle.Render("API USAGE DASHBOARD")
	fmt.Println(lipgloss.PlaceHorizontal(80, lipgloss.Center, title))
	fmt.Println()

	var usages []usage.Info
	for _, name := range h.registry.GetOrdered() {
		be, ok := h.registry.Get(name)
		if !ok {
			continue
		}

		apiKey := h.cfg.Keys[be.AuthVar]
		if apiKey == "" {
			continue // Skip backends without keys
		}

		info := fetchUsageForBackend(be, apiKey)
		usages = append(usages, info)
	}

	if len(usages) == 0 {
		fmt.Println("No configured backends with API keys found.")
		fmt.Println("Add API keys to .env.local to see usage data.")
		return
	}

	// Display summary table
	rows := [][]string{}
	var totalCost float64
	var totalTokens int64

	for _, u := range usages {
		be, _ := h.registry.Get(u.Backend)
		status := ui.FormatCurrency(u.TotalCost)
		if u.Error != "" {
			status = ui.StyleMuted.Render(u.Error)
		}

		rows = append(rows, []string{
			be.DisplayName,
			ui.FormatNumber(u.TotalTokens),
			ui.FormatNumber(u.InputTokens),
			ui.FormatNumber(u.OutputTokens),
			ui.FormatNumberInt(u.RequestCount),
			status,
		})

		totalCost += u.TotalCost
		totalTokens += u.TotalTokens
	}

	t := table.New().
		Headers("Backend", "Total Tokens", "Input", "Output", "Requests", "Cost").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Width(90)

	fmt.Println(t.Render())
	fmt.Println()
	fmt.Printf("Total across all backends: %s  %s tokens\n",
		ui.StyleAccent.Render(ui.FormatCurrency(totalCost)),
		ui.FormatNumber(totalTokens))
	fmt.Println()

	// Show detailed breakdown for each backend
	for _, u := range usages {
		if u.Error != "" {
			displayUsageError(u, h.registry)
		} else if u.TotalTokens > 0 {
			displayUsageDetail(u, h.registry)
		}
	}
}

func fetchUsageForBackend(be backend.Backend, apiKey string) usage.Info {
	info := usage.Info{Backend: be.Name, Period: "current period"}

	switch be.Name {
	case "claude":
		return fetchAnthropicUsage()
	case "openai":
		return fetchOpenAIUsage()
	case "kimi":
		return fetchKimiUsage(apiKey)
	default:
		if be.BaseURL != "" {
			return fetchOpenAICompatibleUsage(be, apiKey)
		}
		info.Error = "Usage API not implemented for this provider"
	}

	return info
}

func fetchAnthropicUsage() usage.Info {
	return usage.Info{
		Backend: "claude",
		Period:  "last 24 hours",
		Error:   "N/A (see console)",
	}
}

func fetchOpenAIUsage() usage.Info {
	return usage.Info{
		Backend: "openai",
		Period:  "current billing period",
		Error:   "N/A (see dashboard)",
	}
}

func fetchKimiUsage(apiKey string) usage.Info {
	info := usage.Info{Backend: "kimi", Period: "current billing period"}

	req, err := http.NewRequest("GET", "https://api.kimi.com/coding/usage", nil)
	if err != nil {
		info.Error = "N/A"
		return info
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		info.Error = "N/A"
		return info
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		info.Error = "N/A (see console)"
		return info
	}

	if resp.StatusCode != http.StatusOK {
		info.Error = "N/A"
		return info
	}

	var result struct {
		Data struct {
			TotalTokens   int64   `json:"total_tokens"`
			InputTokens   int64   `json:"input_tokens"`
			OutputTokens  int64   `json:"output_tokens"`
			TotalRequests int64   `json:"total_requests"`
			TotalCost     float64 `json:"total_cost"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		info.Error = "N/A"
		return info
	}

	info.TotalTokens = result.Data.TotalTokens
	info.InputTokens = result.Data.InputTokens
	info.OutputTokens = result.Data.OutputTokens
	info.RequestCount = result.Data.TotalRequests
	info.TotalCost = result.Data.TotalCost

	return info
}

func fetchOpenAICompatibleUsage(be backend.Backend, apiKey string) usage.Info {
	info := usage.Info{Backend: be.Name, Period: "current period"}

	url := be.BaseURL + "/usage"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		info.Error = err.Error()
		return info
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		info.Error = fmt.Sprintf("Usage API not available (HTTP %d)", resp.StatusCode)
		return info
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		info.Error = err.Error()
		return info
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		if v, ok := data["total_tokens"].(float64); ok {
			info.TotalTokens = int64(v)
		}
		if v, ok := data["input_tokens"].(float64); ok {
			info.InputTokens = int64(v)
		}
		if v, ok := data["output_tokens"].(float64); ok {
			info.OutputTokens = int64(v)
		}
		if v, ok := data["total_cost"].(float64); ok {
			info.TotalCost = v
		}
	}

	return info
}

func displayUsage(info usage.Info, registry *backend.Registry) {
	be, _ := registry.Get(info.Backend)
	fmt.Println()
	fmt.Println(ui.StyleSection.Render(fmt.Sprintf("USAGE: %s", be.DisplayName)))

	if info.Error != "" {
		fmt.Println(ui.StyleWarning.Render(info.Error))
		return
	}

	fmt.Printf("  Period:        %s\n", info.Period)
	fmt.Printf("  Total Tokens:  %s\n", ui.FormatNumber(info.TotalTokens))
	fmt.Printf("  Input Tokens:  %s\n", ui.FormatNumber(info.InputTokens))
	fmt.Printf("  Output Tokens: %s\n", ui.FormatNumber(info.OutputTokens))
	fmt.Printf("  Requests:      %s\n", ui.FormatNumberInt(info.RequestCount))
	fmt.Printf("  Total Cost:    %s\n", ui.StyleAccent.Render(ui.FormatCurrency(info.TotalCost)))
	fmt.Println()
}

func displayUsageDetail(info usage.Info, registry *backend.Registry) {
	if info.Error != "" {
		return
	}

	be, _ := registry.Get(info.Backend)
	fmt.Printf("%s: ", be.DisplayName)
	fmt.Printf("%s tokens, ", ui.FormatNumber(info.TotalTokens))
	fmt.Printf("%s requests, ", ui.FormatNumberInt(info.RequestCount))
	fmt.Printf("cost: %s\n", ui.StyleAccent.Render(ui.FormatCurrency(info.TotalCost)))
}

func displayUsageError(info usage.Info, registry *backend.Registry) {
	if info.Error == "" || info.Error == "N/A" || info.Error == "N/A (see console)" || info.Error == "N/A (see dashboard)" {
		return
	}

	be, _ := registry.Get(info.Backend)
	fmt.Printf("%s: %s\n", be.DisplayName, ui.StyleWarning.Render(info.Error))
}
