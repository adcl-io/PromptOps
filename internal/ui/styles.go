// Package ui provides Lipgloss styles and output helpers.
package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// Color definitions.
var (
	ColorPrimary = lipgloss.Color("#00BCD4") // Cyan
	ColorSuccess = lipgloss.Color("#4CAF50") // Green
	ColorWarning = lipgloss.Color("#FFC107") // Yellow
	ColorError   = lipgloss.Color("#F44336") // Red
	ColorMuted   = lipgloss.Color("#757575") // Gray
	ColorAccent  = lipgloss.Color("#E91E63") // Magenta
	ColorText    = lipgloss.Color("#FFFFFF") // White
	ColorSubtle  = lipgloss.Color("#9E9E9E") // Light gray
	ColorDark    = lipgloss.Color("#212121") // Dark background
)

// Styles.
var (
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(0, 1)

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			Background(ColorPrimary).
			Padding(0, 1).
			Width(78)

	StyleSection = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginTop(1)

	StyleLabel = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	StyleValue = lipgloss.NewStyle().
			Foreground(ColorText)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	StyleWarning = lipgloss.NewStyle().
			Foreground(ColorWarning)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorError)

	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleAccent = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	StyleCurrent = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	StyleBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			Width(80)

	StyleProgressFilled = lipgloss.NewStyle().
				Background(ColorSuccess).
				Foreground(ColorText)

	StyleProgressEmpty = lipgloss.NewStyle().
				Background(ColorMuted).
				Foreground(ColorText)
)

// Progress bar widths.
const (
	ProgressBarWidth = 40
	MiniBarWidth     = 20
)

// Renderer handles UI rendering.
type Renderer struct {
	progressBarWidth int
	miniBarWidth     int
}

// NewRenderer creates a new UI renderer.
func NewRenderer() *Renderer {
	return &Renderer{
		progressBarWidth: ProgressBarWidth,
		miniBarWidth:     MiniBarWidth,
	}
}

// RenderProgressBar renders a progress bar with label and values.
func (r *Renderer) RenderProgressBar(label string, current, limit float64) string {
	percent := current / limit * 100
	if percent > 100 {
		percent = 100
	}

	filled := int(percent * float64(r.progressBarWidth) / 100)
	if filled < 0 {
		filled = 0
	}

	barColor := ColorSuccess
	if percent >= 90 {
		barColor = ColorError
	} else if percent >= 70 {
		barColor = ColorWarning
	}

	filledBar := lipgloss.NewStyle().Background(barColor).Foreground(ColorText).Render(strings.Repeat(" ", filled))
	emptyBar := lipgloss.NewStyle().Background(ColorMuted).Render(strings.Repeat(" ", r.progressBarWidth-filled))

	return fmt.Sprintf("%s  %s / %s  %s%s  %.0f%%",
		StyleLabel.Render(label),
		StyleValue.Render(FormatCurrency(current)),
		StyleValue.Render(FormatCurrency(limit)),
		filledBar,
		emptyBar,
		percent,
	)
}

// RenderMiniBar renders a mini progress bar.
func (r *Renderer) RenderMiniBar(percent float64) string {
	filled := int(percent * float64(r.miniBarWidth) / 100)
	if filled < 0 {
		filled = 0
	}
	if filled > r.miniBarWidth {
		filled = r.miniBarWidth
	}

	barColor := ColorSuccess
	if percent >= 50 {
		barColor = ColorWarning
	}
	if percent >= 80 {
		barColor = ColorError
	}

	filledBar := lipgloss.NewStyle().Background(barColor).Render(strings.Repeat(" ", filled))
	emptyBar := lipgloss.NewStyle().Background(ColorMuted).Render(strings.Repeat(" ", r.miniBarWidth-filled))

	return filledBar + emptyBar + fmt.Sprintf(" %.0f%%", percent)
}

// PrintLogo prints the ASCII logo for a backend.
func PrintLogo(backend string) {
	switch backend {
	case "claude":
		fmt.Println("   ▄████▄   ██▓    ▄▄▄       █    ██ ▓█████▄ ▓█████ ")
		fmt.Println("  ▒██▀ ▀█  ▓██▒   ▒████▄     ██  ▓██▒▒██▀ ██▌▓█   ▀ ")
		fmt.Println("  ▒▓█    ▄ ▒██░   ▒██  ▀█▄  ▓██  ▒██░░██   █▌▒███   ")
		fmt.Println("  ▒▓▓▄ ▄██▒▒██░   ░██▄▄▄▄██ ▓▓█  ░██░░▓█▄   ▌▒▓█  ▄ ")
		fmt.Println("  ▒ ▓███▀ ░░██████▒▓█   ▓██▒▒▒█████▓ ░▒████▓ ░▒████▒")
		fmt.Println("  ░ ░▒ ▒  ░░ ▒░▓  ░▒▒   ▓▒█░░▒▓▒ ▒ ▒  ▒▒▓  ▒ ░░ ▒░ ░")
	case "zai":
		fmt.Println("  ▒███████▒    ▄▄▄       ██▓    ")
		fmt.Println("  ▒ ▒ ▒ ▄▀░   ▒████▄    ▓██▒    ")
		fmt.Println("  ░ ▒ ▄▀▒░    ▒██  ▀█▄  ▒██▒    ")
		fmt.Println("    ▄▀▒   ░   ░██▄▄▄▄██ ░██░    ")
		fmt.Println("  ▒███████▒    ▓█   ▓██▒░██░    ")
		fmt.Println("  ░▒▒ ▓░▒░▒    ▒▒   ▓▒█░░▓      ")
		fmt.Println("  ░░▒ ▒ ░ ▒     ▒   ▒▒ ░ ▒ ░    ")
		fmt.Println("     GLM-4.7 POWERED")
	case "kimi":
		fmt.Println("  ██ ▄█▀ ██▓ ███▄ ▄███▓ ██▓")
		fmt.Println("  ██▄█▒ ▓██▒▓██▒▀█▀ ██▒▓██▒")
		fmt.Println(" ▓███▄░ ▒██▒▓██    ▓██░▒██▒")
		fmt.Println(" ▓██ █▄ ░██░▒██    ▒██ ░██░")
		fmt.Println(" ▒██▒ █▄░██░▒██▒   ░██▒░██░")
		fmt.Println(" ▒ ▒▒ ▓▒░▓  ░ ▒░   ░  ░░▓  ")
		fmt.Println("  MOONSHOT AI K2.5")
	case "deepseek":
		fmt.Println("  ██████   ███████  ███████  ███████  ██   ██ ██   ██")
		fmt.Println("  ██   ██  ██       ██       ██       ██   ██ ██   ██")
		fmt.Println("  ██   ██  █████    █████    █████    ███████ ███████")
		fmt.Println("  ██   ██  ██       ██       ██       ██   ██ ██   ██")
		fmt.Println("  ██████   ███████  ███████  ███████  ██   ██ ██   ██")
		fmt.Println("  DEEPSEEK V3/R1")
	case "gemini":
		fmt.Println("   ██████   ███████  ███    ███  ██   ██ ██   ██")
		fmt.Println("  ██        ██       ████  ████  ██   ██ ██   ██")
		fmt.Println("  ██   ███  █████    ██ ████ ██  ███████ ███████")
		fmt.Println("  ██    ██  ██       ██  ██  ██  ██   ██ ██   ██")
		fmt.Println("   ██████   ███████  ██      ██  ██   ██ ██   ██")
		fmt.Println("  GOOGLE GEMINI 2.5 PRO")
	case "mistral":
		fmt.Println("  ███    ███ ██ ███████ ████████ ██████   █████  ██")
		fmt.Println("  ████  ████ ██ ██         ██    ██   ██ ██   ██ ██")
		fmt.Println("  ██ ████ ██ ██ █████      ██    ██████  ███████ ██")
		fmt.Println("  ██  ██  ██ ██ ██         ██    ██   ██ ██   ██ ██")
		fmt.Println("  ██      ██ ██ ██         ██    ██   ██ ██   ██ ███████")
		fmt.Println("  MISTRAL LARGE/CODESTRAL")
	case "groq":
		fmt.Println("   ██████   ██████   ██████   ███████")
		fmt.Println("  ██       ██    ██  ██   ██  ██")
		fmt.Println("  ██   ███ ██    ██  ██████   █████")
		fmt.Println("  ██    ██ ██    ██  ██   ██  ██")
		fmt.Println("   ██████   ██████   ██   ██  ███████")
		fmt.Println("  GROQ - LLAMA 3.3 70B/405B")
	case "together":
		fmt.Println("  ████████  ██████   ███████  ████████ ██████   ███████  ██████")
		fmt.Println("     ██    ██    ██  ██          ██    ██   ██  ██       ██   ██")
		fmt.Println("     ██    ██    ██  █████       ██    ██████   █████    ██████")
		fmt.Println("     ██    ██    ██  ██          ██    ██   ██  ██       ██   ██")
		fmt.Println("     ██     ██████   ███████     ██    ██   ██  ███████  ██   ██")
		fmt.Println("  TOGETHER AI - LLAMA/QWEN/DEEPSEEK")
	case "openrouter":
		fmt.Println("   ██████   ██████  ██   ██ ███████ ████████ ████████ ██████   ███████  ██████")
		fmt.Println("  ██    ██ ██    ██ ██   ██ ██         ██       ██    ██   ██  ██       ██   ██")
		fmt.Println("  ██    ██ ██    ██ ███████ █████      ██       ██    ██████   █████    ██████")
		fmt.Println("  ██    ██ ██    ██ ██   ██ ██         ██       ██    ██   ██  ██       ██   ██")
		fmt.Println("   ██████   ██████  ██   ██ ███████    ██       ██    ██   ██  ███████  ██   ██")
		fmt.Println("  OPENROUTER - 200+ MODELS")
	case "openai":
		fmt.Println("   ██████   ██████  ███████  ███████  ███████  ██")
		fmt.Println("  ██    ██ ██    ██ ██       ██       ██       ██")
		fmt.Println("  ██    ██ ██    ██ █████    █████    █████    ██")
		fmt.Println("  ██    ██ ██    ██ ██       ██       ██       ██")
		fmt.Println("   ██████   ██████  ███████  ███████  ███████  ██")
		fmt.Println("  OPENAI - GPT-4o / GPT-4o-mini / o1")
	case "ollama":
		fmt.Println("   ██████  ██      ██       █████  ███    ███  █████")
		fmt.Println("  ██    ██ ██      ██      ██   ██ ████  ████ ██   ██")
		fmt.Println("  ██    ██ ██      ██      ███████ ██ ████ ██ ███████")
		fmt.Println("  ██    ██ ██      ██      ██   ██ ██  ██  ██ ██   ██")
		fmt.Println("   ██████  ███████ ███████ ██   ██ ██      ██ ██   ██")
		fmt.Println("  OLLAMA - LOCAL LLM INFERENCE")
	}
}

// DrawBox draws a box around a message.
func DrawBox(msg string) {
	width := utf8.RuneCountInString(msg) + 8
	fmt.Print("+")
	for i := 0; i < width; i++ {
		fmt.Print("-")
	}
	fmt.Println("+")
	fmt.Print("|    ")
	fmt.Print(msg)
	fmt.Print("    |\n")
	fmt.Print("+")
	for i := 0; i < width; i++ {
		fmt.Print("-")
	}
	fmt.Println("+")
}

// FormatCurrency formats a float as currency.
func FormatCurrency(amount float64) string {
	return fmt.Sprintf("$%.2f", amount)
}

// Truncate truncates a string to maxLen, adding ellipsis if needed.
func Truncate(s string, maxLen int) string {
	if maxLen <= 3 {
		return "..."
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// FormatNumber formats a number with K/M suffixes.
func FormatNumber(n int64) string {
	if n == 0 {
		return "-"
	}
	if n >= 1000000 {
		return fmt.Sprintf("%.2fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// FormatNumberInt formats an int64 as a string.
func FormatNumberInt(n int64) string {
	if n == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", n)
}

// MaskKey masks an API key for display.
func MaskKey(key string) string {
	const (
		minLength     = 8
		visiblePrefix = 4
		visibleSuffix = 4
		replacement   = "****"
	)

	if len(key) <= minLength {
		return replacement
	}
	return key[:visiblePrefix] + replacement + key[len(key)-visibleSuffix:]
}

// SanitizeArgs removes potentially dangerous characters from command arguments.
func SanitizeArgs(args []string) []string {
	var sanitized []string
	for _, arg := range args {
		// Remove null bytes and control characters
		arg = strings.ReplaceAll(arg, "\x00", "")
		arg = strings.ReplaceAll(arg, "\n", "")
		arg = strings.ReplaceAll(arg, "\r", "")
		// Limit argument length to prevent DoS
		if len(arg) > 4096 {
			arg = arg[:4096]
		}
		sanitized = append(sanitized, arg)
	}
	return sanitized
}

// FilterEnvironment returns only whitelisted environment variables.
func FilterEnvironment(env []string, allowed map[string]bool) []string {
	var filtered []string
	for _, e := range env {
		key := strings.SplitN(e, "=", 2)[0]
		if allowed[key] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// GetAllowedEnvVars returns the set of allowed environment variables.
func GetAllowedEnvVars() map[string]bool {
	return map[string]bool{
		"PATH":               true,
		"HOME":               true,
		"USER":               true,
		"SHELL":              true,
		"TERM":               true,
		"TERMINFO":           true,
		"LANG":               true,
		"LANGUAGE":           true,
		"LC_ALL":             true,
		"LC_CTYPE":           true,
		"EDITOR":             true,
		"PAGER":              true,
		"LESS":               true,
		"MORE":               true,
		"TMPDIR":             true,
		"TEMP":               true,
		"TMP":                true,
		"SSH_AUTH_SOCK":      true,
		"SSH_AGENT_LAUNCHER": true,
		// Anthropic/Claude specific variables
		"ANTHROPIC_AUTH_TOKEN":           true,
		"ANTHROPIC_BASE_URL":             true,
		"API_TIMEOUT_MS":                 true,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  true,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": true,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   true,
		// Ollama specific variables
		"OLLAMA_API_KEY":      true,
		"OLLAMA_HAIKU_MODEL":  true,
		"OLLAMA_SONNET_MODEL": true,
		"OLLAMA_OPUS_MODEL":   true,
		"ZAI_HAIKU_MODEL":     true,
		"ZAI_SONNET_MODEL":    true,
		"ZAI_OPUS_MODEL":      true,
		"KIMI_HAIKU_MODEL":    true,
		"KIMI_SONNET_MODEL":   true,
		"KIMI_OPUS_MODEL":     true,
	}
}
