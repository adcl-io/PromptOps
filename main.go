// promptops - AI Model Backend Switcher
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

const version = "2.4.0"

// Lipgloss styles
var (
	// Base colors
	colorPrimary = lipgloss.Color("#00BCD4") // Cyan
	colorSuccess = lipgloss.Color("#4CAF50") // Green
	colorWarning = lipgloss.Color("#FFC107") // Yellow
	colorError   = lipgloss.Color("#F44336") // Red
	colorMuted   = lipgloss.Color("#757575") // Gray
	colorAccent  = lipgloss.Color("#E91E63") // Magenta
	colorText    = lipgloss.Color("#FFFFFF") // White
	colorSubtle  = lipgloss.Color("#9E9E9E") // Light gray
	colorDark    = lipgloss.Color("#212121") // Dark background

	// Styles
	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			Background(colorPrimary).
			Padding(0, 1).
			Width(78)

	styleSection = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginTop(1)

	styleLabel = lipgloss.NewStyle().
			Foreground(colorSubtle)

	styleValue = lipgloss.NewStyle().
			Foreground(colorText)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess)

	styleWarning = lipgloss.NewStyle().
			Foreground(colorWarning)

	styleError = lipgloss.NewStyle().
			Foreground(colorError)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleAccent = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleCurrent = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2).
			Width(80)

	styleProgressFilled = lipgloss.NewStyle().
				Background(colorSuccess).
				Foreground(colorText)

	styleProgressEmpty = lipgloss.NewStyle().
				Background(colorMuted).
				Foreground(colorText)
)

type Backend struct {
	Name        string
	DisplayName string
	Provider    string
	Models      string
	AuthVar     string
	BaseURL     string
	Timeout     string
	HaikuModel  string
	SonnetModel string
	OpusModel   string
	// Pricing per 1M tokens (USD)
	InputPrice  float64
	OutputPrice float64
}

var backends = map[string]Backend{
	"claude": {
		Name:        "claude",
		DisplayName: "Claude",
		Provider:    "Anthropic",
		Models:      "Claude Sonnet 4.5",
		AuthVar:     "ANTHROPIC_API_KEY",
		InputPrice:  3.00,
		OutputPrice: 15.00,
	},
	"zai": {
		Name:        "zai",
		DisplayName: "Z.AI",
		Provider:    "Z.AI (Zhipu AI)",
		Models:      "GLM-4.7 (Sonnet/Opus) / GLM-4.5-Air (Haiku)",
		AuthVar:     "ZAI_API_KEY",
		BaseURL:     "https://api.z.ai/api/anthropic",
		Timeout:     "3000000",
		HaikuModel:  "glm-4.5-air",
		SonnetModel: "glm-4.7",
		OpusModel:   "glm-4.7",
		InputPrice:  0.50,
		OutputPrice: 2.00,
	},
	"kimi": {
		Name:        "kimi",
		DisplayName: "Kimi",
		Provider:    "Kimi Code (Subscription)",
		Models:      "kimi-for-coding",
		AuthVar:     "KIMI_API_KEY",
		BaseURL:     "https://api.kimi.com/coding",
		Timeout:     "3000000",
		HaikuModel:  "kimi-for-coding",
		SonnetModel: "kimi-for-coding",
		OpusModel:   "kimi-for-coding",
		InputPrice:  2.00,
		OutputPrice: 8.00,
	},
	"deepseek": {
		Name:        "deepseek",
		DisplayName: "DeepSeek",
		Provider:    "DeepSeek AI",
		Models:      "DeepSeek-V3 / DeepSeek-R1",
		AuthVar:     "DEEPSEEK_API_KEY",
		BaseURL:     "https://api.deepseek.com/v1",
		Timeout:     "3000000",
		HaikuModel:  "deepseek-chat",
		SonnetModel: "deepseek-reasoner",
		OpusModel:   "deepseek-reasoner",
		InputPrice:  0.27,
		OutputPrice: 1.10,
	},
	"gemini": {
		Name:        "gemini",
		DisplayName: "Gemini",
		Provider:    "Google AI",
		Models:      "Gemini 2.5 Pro",
		AuthVar:     "GEMINI_API_KEY",
		BaseURL:     "https://generativelanguage.googleapis.com/v1beta/openai",
		Timeout:     "3000000",
		HaikuModel:  "gemini-2.5-flash",
		SonnetModel: "gemini-2.5-pro",
		OpusModel:   "gemini-2.5-pro",
		InputPrice:  1.25,
		OutputPrice: 10.00,
	},
	"mistral": {
		Name:        "mistral",
		DisplayName: "Mistral",
		Provider:    "Mistral AI",
		Models:      "Mistral Large / Codestral",
		AuthVar:     "MISTRAL_API_KEY",
		BaseURL:     "https://api.mistral.ai/v1",
		Timeout:     "3000000",
		HaikuModel:  "codestral-latest",
		SonnetModel: "mistral-large-latest",
		OpusModel:   "mistral-large-latest",
		InputPrice:  2.00,
		OutputPrice: 6.00,
	},
	"groq": {
		Name:        "groq",
		DisplayName: "Groq",
		Provider:    "Groq (Llama)",
		Models:      "Llama 3.3 70B / 405B",
		AuthVar:     "GROQ_API_KEY",
		BaseURL:     "https://api.groq.com/openai/v1",
		Timeout:     "3000000",
		HaikuModel:  "llama-3.3-70b-versatile",
		SonnetModel: "llama-3.3-70b-versatile",
		OpusModel:   "llama-3.1-405b-reasoning",
		InputPrice:  0.59,
		OutputPrice: 0.79,
	},
	"together": {
		Name:        "together",
		DisplayName: "Together AI",
		Provider:    "Together AI",
		Models:      "Llama / Qwen / DeepSeek",
		AuthVar:     "TOGETHER_API_KEY",
		BaseURL:     "https://api.together.xyz/v1",
		Timeout:     "3000000",
		HaikuModel:  "meta-llama/Llama-3.3-70B-Instruct-Turbo",
		SonnetModel: "deepseek-ai/DeepSeek-V3",
		OpusModel:   "meta-llama/Llama-3.1-405B-Instruct",
		InputPrice:  1.00,
		OutputPrice: 2.00,
	},
	"openrouter": {
		Name:        "openrouter",
		DisplayName: "OpenRouter",
		Provider:    "OpenRouter",
		Models:      "200+ models via meta-router",
		AuthVar:     "OPENROUTER_API_KEY",
		BaseURL:     "https://openrouter.ai/api/v1",
		Timeout:     "3000000",
		HaikuModel:  "google/gemini-flash-1.5",
		SonnetModel: "anthropic/claude-3.5-sonnet",
		OpusModel:   "anthropic/claude-3-opus",
		InputPrice:  3.00,
		OutputPrice: 15.00,
	},
	"openai": {
		Name:        "openai",
		DisplayName: "OpenAI",
		Provider:    "OpenAI",
		Models:      "GPT-4o / GPT-4o-mini / o1",
		AuthVar:     "OPENAI_API_KEY",
		BaseURL:     "https://api.openai.com/v1",
		Timeout:     "3000000",
		HaikuModel:  "gpt-4o-mini",
		SonnetModel: "gpt-4o",
		OpusModel:   "o1",
		InputPrice:  2.50,
		OutputPrice: 10.00,
	},
}

type Config struct {
	EnvFile            string
	StateFile          string
	AuditLog           string
	UsageFile          string
	SessionsFile       string
	SessionFile        string
	YoloMode           bool
	YoloModeClaude     bool
	YoloModeZai        bool
	YoloModeKimi       bool
	YoloModeDeepseek   bool
	YoloModeGemini     bool
	YoloModeMistral    bool
	YoloModeGroq       bool
	YoloModeTogether   bool
	YoloModeOpenrouter bool
	YoloModeOpenai     bool
	DefaultBackend     string
	VerifyOnSwitch     bool
	AuditEnabled       bool
	Keys               map[string]string
	// Budget settings
	DailyBudget   float64
	WeeklyBudget  float64
	MonthlyBudget float64
}

// UsageRecord represents a single API usage entry
type UsageRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"session_id"`
	Backend      string    `json:"backend"`
	Model        string    `json:"model"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
}

// Session represents a named working session
type Session struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Backend     string    `json:"backend"`
	StartTime   time.Time `json:"start_time"`
	LastActive  time.Time `json:"last_active"`
	WorkingDir  string    `json:"working_dir"`
	PromptCount int       `json:"prompt_count"`
	TotalCost   float64   `json:"total_cost"`
	Status      string    `json:"status"` // active, paused, closed
}

// HealthResult represents the result of a backend health check
type HealthResult struct {
	Backend string
	Status  string // ok, skip, error
	Latency time.Duration
	Message string
}

func main() {
	if len(os.Args) < 2 {
		showStatus()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "claude", "zai", "kimi", "deepseek", "gemini", "mistral", "groq", "together", "openrouter", "openai":
		switchBackend(cmd, args)
	case "status", "current":
		showStatus()
	case "run", "launch":
		runClaude(args)
	case "init", "setup":
		initEnv()
	case "version", "--version", "-v":
		showVersion()
	case "help", "--help", "-h":
		showHelp()
	// Cost tracking commands
	case "cost":
		if len(args) > 0 && args[0] == "log" {
			showCostLog()
		} else {
			showCostDashboard()
		}
	// Budget management commands
	case "budget":
		handleBudgetCommand(args)
	// Environment validation commands
	case "doctor":
		runDoctor()
	case "validate":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "Error: validate requires a backend name")
			os.Exit(1)
		}
		validateBackend(args[0])
	// Session management commands
	case "session":
		handleSessionCommand(args)
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command '%s'. Run 'promptops help' for usage.\n", cmd)
		os.Exit(1)
	}
}

func getScriptDir() string {
	ex, err := os.Executable()
	if err != nil {
		ex = os.Args[0]
	}
	return filepath.Dir(ex)
}

func loadConfig() *Config {
	dir := getScriptDir()
	envFile := os.Getenv("NEXUS_ENV_FILE")
	if envFile == "" {
		envFile = filepath.Join(dir, ".env.local")
	}

	cfg := &Config{
		EnvFile:        envFile,
		StateFile:      filepath.Join(dir, "state"),
		AuditLog:       filepath.Join(dir, ".promptops-audit.log"),
		UsageFile:      filepath.Join(dir, ".promptops-usage.jsonl"),
		SessionsFile:   filepath.Join(dir, ".promptops-sessions.json"),
		SessionFile:    filepath.Join(dir, "session"),
		Keys:           make(map[string]string),
		DefaultBackend: "claude",
		VerifyOnSwitch: true,
		AuditEnabled:   true,
		DailyBudget:    10.00,
		WeeklyBudget:   50.00,
		MonthlyBudget:  100.00,
	}

	// Parse .env.local
	data, err := os.ReadFile(envFile)
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, `"'`)

			switch key {
			case "NEXUS_YOLO_MODE":
				cfg.YoloMode = value == "true"
			case "NEXUS_YOLO_MODE_CLAUDE":
				cfg.YoloModeClaude = value == "true"
			case "NEXUS_YOLO_MODE_ZAI":
				cfg.YoloModeZai = value == "true"
			case "NEXUS_YOLO_MODE_KIMI":
				cfg.YoloModeKimi = value == "true"
			case "NEXUS_YOLO_MODE_DEEPSEEK":
				cfg.YoloModeDeepseek = value == "true"
			case "NEXUS_YOLO_MODE_GEMINI":
				cfg.YoloModeGemini = value == "true"
			case "NEXUS_YOLO_MODE_MISTRAL":
				cfg.YoloModeMistral = value == "true"
			case "NEXUS_YOLO_MODE_GROQ":
				cfg.YoloModeGroq = value == "true"
			case "NEXUS_YOLO_MODE_TOGETHER":
				cfg.YoloModeTogether = value == "true"
			case "NEXUS_YOLO_MODE_OPENROUTER":
				cfg.YoloModeOpenrouter = value == "true"
			case "NEXUS_YOLO_MODE_OPENAI":
				cfg.YoloModeOpenai = value == "true"
			case "NEXUS_DEFAULT_BACKEND":
				cfg.DefaultBackend = value
			case "NEXUS_VERIFY_ON_SWITCH":
				cfg.VerifyOnSwitch = value == "true"
			case "NEXUS_AUDIT_LOG":
				cfg.AuditEnabled = value == "true"
			case "NEXUS_DAILY_BUDGET":
				if v, err := strconv.ParseFloat(value, 64); err == nil {
					cfg.DailyBudget = v
				}
			case "NEXUS_WEEKLY_BUDGET":
				if v, err := strconv.ParseFloat(value, 64); err == nil {
					cfg.WeeklyBudget = v
				}
			case "NEXUS_MONTHLY_BUDGET":
				if v, err := strconv.ParseFloat(value, 64); err == nil {
					cfg.MonthlyBudget = v
				}
			case "ANTHROPIC_API_KEY", "ZAI_API_KEY", "KIMI_API_KEY", "DEEPSEEK_API_KEY", "GEMINI_API_KEY", "MISTRAL_API_KEY", "GROQ_API_KEY", "TOGETHER_API_KEY", "OPENROUTER_API_KEY", "OPENAI_API_KEY":
				cfg.Keys[key] = value
			}
		}
	}

	return cfg
}

func (c *Config) getYoloMode(backend string) bool {
	if c.YoloMode {
		return true
	}
	switch backend {
	case "claude":
		return c.YoloModeClaude
	case "zai":
		return c.YoloModeZai
	case "kimi":
		return c.YoloModeKimi
	case "deepseek":
		return c.YoloModeDeepseek
	case "gemini":
		return c.YoloModeGemini
	case "mistral":
		return c.YoloModeMistral
	case "groq":
		return c.YoloModeGroq
	case "together":
		return c.YoloModeTogether
	case "openrouter":
		return c.YoloModeOpenrouter
	case "openai":
		return c.YoloModeOpenai
	}
	return false
}

func getCurrentBackend(cfg *Config) string {
	data, err := os.ReadFile(cfg.StateFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func setCurrentBackend(cfg *Config, backend string) error {
	return os.WriteFile(cfg.StateFile, []byte(backend), 0644)
}

func maskKey(key string) string {
	if len(key) <= 16 {
		if len(key) <= 8 {
			return "****"
		}
		return key[:4] + "****" + key[len(key)-4:]
	}
	return key[:8] + "..." + key[len(key)-4:]
}

func auditLog(cfg *Config, msg string) {
	if !cfg.AuditEnabled {
		return
	}
	f, err := os.OpenFile(cfg.AuditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	// Include session ID if available
	session := getCurrentSession(cfg)
	if session != nil {
		msg = fmt.Sprintf("[%s] %s", session.Name, msg)
	}

	fmt.Fprintf(f, "[%s] %s\n", time.Now().Format(time.RFC3339), msg)
}

func printLogo(backend string) {
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
	}
}

func drawBox(msg string) {
	width := len(msg) + 8
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

func animateSwitch(msg string) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i := 0; i < 20; i++ {
		fmt.Printf("\r%s %s", frames[i%10], msg)
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("\r[OK] %s\n", msg)
}

func showProgress(msg string) {
	fmt.Printf("%s [", msg)
	colors := []string{"\033[0;31m", "\033[1;33m", "\033[0;32m", "\033[0;36m", "\033[0;34m", "\033[0;35m"}
	reset := "\033[0m"
	for i := 0; i < 30; i++ {
		fmt.Printf("%s█%s", colors[i%6], reset)
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println("] COMPLETE")
}

func switchBackend(name string, args []string) {
	cfg := loadConfig()
	be, ok := backends[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: Unknown backend '%s'\n", name)
		os.Exit(1)
	}

	// Check for API key
	apiKey := cfg.Keys[be.AuthVar]
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "Error: %s not set in .env.local\n", be.AuthVar)
		os.Exit(1)
	}

	yolo := cfg.getYoloMode(name)

	if !yolo {
		fmt.Print("\033[H\033[2J") // clear screen
		fmt.Println()
	}

	// Animations
	if !yolo {
		switch name {
		case "claude":
			animateSwitch("Initializing neural pathways...")
		case "zai":
			animateSwitch("Reconfiguring quantum matrices...")
		case "kimi":
			animateSwitch("Engaging moonshot protocols...")
		}
		fmt.Println()
		printLogo(name)
		fmt.Println()

		switch name {
		case "claude":
			showProgress("Connecting to Anthropic")
		case "zai":
			showProgress("Connecting to Z.AI")
		case "kimi":
			showProgress("Connecting to Kimi Code")
		}
	}

	// Save state
	if err := setCurrentBackend(cfg, name); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
		os.Exit(1)
	}

	// Audit log
	auditLog(cfg, fmt.Sprintf("SWITCH: %s (key: %s)", name, maskKey(apiKey)))

	if !yolo {
		fmt.Println()
		drawBox(fmt.Sprintf("%s BACKEND ACTIVE", strings.ToUpper(be.DisplayName)))
		fmt.Printf("  Provider: %s\n", be.Provider)
		fmt.Printf("  Models:   %s\n", be.Models)
		if be.BaseURL != "" {
			fmt.Printf("  Base URL: %s\n", be.BaseURL)
		}
		fmt.Printf("  API Key:  %s\n", maskKey(apiKey))
		fmt.Println("  Status:   [ONLINE]")
		fmt.Println()
		fmt.Println("-------------------------------------------------------")
		fmt.Println("[OK] Backend configured - launching Claude Code...")
		fmt.Println("-------------------------------------------------------")
		fmt.Println()
	}

	// Launch claude with proper env
	launchClaudeWithBackend(cfg, be, args)
}

func launchClaudeWithBackend(cfg *Config, be Backend, args []string) {
	cmdArgs := []string{}

	yolo := cfg.getYoloMode(be.Name)
	if yolo {
		cmdArgs = append(cmdArgs, "--permission-mode", "acceptEdits")
	}

	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("claude", cmdArgs...)

	// Build environment
	env := os.Environ()

	// Remove any existing Claude-related vars
	filtered := env[:0]
	for _, e := range env {
		if !strings.HasPrefix(e, "ANTHROPIC_AUTH_TOKEN=") &&
			!strings.HasPrefix(e, "ANTHROPIC_BASE_URL=") &&
			!strings.HasPrefix(e, "CLAUDE_CODE_OAUTH_TOKEN=") {
			filtered = append(filtered, e)
		}
	}
	env = filtered

	// Set auth token
	env = append(env, fmt.Sprintf("ANTHROPIC_AUTH_TOKEN=%s", cfg.Keys[be.AuthVar]))

	// Set backend-specific vars
	if be.BaseURL != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_BASE_URL=%s", be.BaseURL))
		env = append(env, fmt.Sprintf("API_TIMEOUT_MS=%s", be.Timeout))
		env = append(env, fmt.Sprintf("ANTHROPIC_DEFAULT_HAIKU_MODEL=%s", be.HaikuModel))
		env = append(env, fmt.Sprintf("ANTHROPIC_DEFAULT_SONNET_MODEL=%s", be.SonnetModel))
		env = append(env, fmt.Sprintf("ANTHROPIC_DEFAULT_OPUS_MODEL=%s", be.OpusModel))
	}

	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error launching claude: %v\n", err)
		os.Exit(1)
	}
}

func runClaude(args []string) {
	cfg := loadConfig()
	current := getCurrentBackend(cfg)

	if current == "" {
		fmt.Println("WARNING: No backend configured. Defaulting to Claude.")
		switchBackend("claude", args)
		return
	}

	be, ok := backends[current]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: Unknown backend '%s' in state file\n", current)
		os.Exit(1)
	}

	fmt.Printf("INFO: Launching Claude Code with %s backend...\n\n", current)
	launchClaudeWithBackend(cfg, be, args)
}

func showStatus() {
	cfg := loadConfig()
	current := getCurrentBackend(cfg)
	session := getCurrentSession(cfg)
	dailyCost, weeklyCost, monthlyCost, byBackend := calculateCosts(cfg)

	// Title
	fmt.Println()
	title := styleTitle.Render(fmt.Sprintf("PROMPTOPS v%s", version))
	fmt.Println(lipgloss.PlaceHorizontal(80, lipgloss.Center, title))
	fmt.Println()

	// Current Backend Section
	fmt.Println(styleSection.Render("CURRENT BACKEND"))
	if current != "" {
		be := backends[current]
		status := styleCurrent.Render("> " + be.DisplayName)
		if cfg.getYoloMode(current) {
			status += styleWarning.Render(" [YOLO]")
		}
		fmt.Println(status)
		fmt.Println(styleMuted.Render(be.Models))
	} else {
		fmt.Println(styleMuted.Render("No backend configured"))
	}

	// Session info
	if session != nil {
		fmt.Println()
		fmt.Println(styleSection.Render("SESSION"))
		fmt.Printf("%s %s (%s)\n", styleAccent.Render(">"), session.Name, styleSuccess.Render(session.Status))
	}

	// Backends Table
	fmt.Println()
	fmt.Println(styleSection.Render("AVAILABLE BACKENDS"))

	backendOrder := []string{"claude", "openai", "deepseek", "gemini", "mistral", "zai", "kimi", "groq", "together", "openrouter"}

	rows := [][]string{}
	for _, name := range backendOrder {
		be := backends[name]
		hasKey := cfg.Keys[be.AuthVar] != ""

		marker := " "
		if name == current {
			marker = styleAccent.Render(">")
		}

		status := styleSuccess.Render("Ready")
		if !hasKey {
			status = styleMuted.Render("No Key")
		}

		rows = append(rows, []string{
			marker,
			be.DisplayName,
			truncate(be.Models, 25),
			status,
			"--",
		})
	}

	t := table.New().
		Headers("", "Provider", "Models", "Status", "Latency").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(colorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
			}
			if col == 0 {
				return lipgloss.NewStyle().Width(2)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Width(80)

	fmt.Println(t.Render())

	// Cost Summary
	fmt.Println()
	fmt.Println(styleSection.Render("COST SUMMARY"))
	fmt.Printf("This Month: %s / %s\n",
		styleValue.Render(formatCurrency(monthlyCost)),
		styleValue.Render(formatCurrency(cfg.MonthlyBudget)))
	fmt.Println()

	// Budget progress bars
	renderProgressBar("Daily  ", dailyCost, cfg.DailyBudget)
	renderProgressBar("Weekly ", weeklyCost, cfg.WeeklyBudget)
	renderProgressBar("Monthly", monthlyCost, cfg.MonthlyBudget)

	// Top backends by usage
	if len(byBackend) > 0 {
		fmt.Println()
		fmt.Println(styleSection.Render("TOP BACKENDS BY USAGE"))

		type backendCost struct {
			name string
			cost float64
		}
		var bc []backendCost
		total := 0.0
		for name, cost := range byBackend {
			bc = append(bc, backendCost{name, cost})
			total += cost
		}
		sort.Slice(bc, func(i, j int) bool { return bc[i].cost > bc[j].cost })

		for _, b := range bc {
			percent := b.cost / total * 100
			fmt.Printf("%-12s %8s  %s\n",
				backends[b.name].DisplayName,
				formatCurrency(b.cost),
				renderMiniBar(percent),
			)
		}
	}

	fmt.Println()
}

func renderProgressBar(label string, current, limit float64) {
	percent := current / limit * 100
	if percent > 100 {
		percent = 100
	}

	width := 40
	filled := int(percent * float64(width) / 100)
	if filled < 0 {
		filled = 0
	}

	barColor := colorSuccess
	if percent >= 90 {
		barColor = colorError
	} else if percent >= 70 {
		barColor = colorWarning
	}

	filledBar := lipgloss.NewStyle().Background(barColor).Foreground(colorText).Render(strings.Repeat(" ", filled))
	emptyBar := lipgloss.NewStyle().Background(colorMuted).Render(strings.Repeat(" ", width-filled))

	fmt.Printf("%s  %s / %s  %s%s  %.0f%%\n",
		styleLabel.Render(label),
		styleValue.Render(formatCurrency(current)),
		styleValue.Render(formatCurrency(limit)),
		filledBar,
		emptyBar,
		percent,
	)
}

func renderMiniBar(percent float64) string {
	width := 20
	filled := int(percent * float64(width) / 100)
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	barColor := colorSuccess
	if percent >= 50 {
		barColor = colorWarning
	}
	if percent >= 80 {
		barColor = colorError
	}

	filledBar := lipgloss.NewStyle().Background(barColor).Render(strings.Repeat(" ", filled))
	emptyBar := lipgloss.NewStyle().Background(colorMuted).Render(strings.Repeat(" ", width-filled))

	return filledBar + emptyBar + fmt.Sprintf(" %.0f%%", percent)
}

func initEnv() {
	dir := getScriptDir()
	envFile := filepath.Join(dir, ".env.local")

	if _, err := os.Stat(envFile); err == nil {
		fmt.Println("[OK] .env.local already exists")
		return
	}

	content := `# -------------------------------------------------------------------------------
# PROMPTOPS - AI Model Backend Switcher Configuration
# -------------------------------------------------------------------------------

# -------------------------------------------------------------------------------
# YOLO MODE - Auto-confirm settings for each backend
# Set to "true" to skip confirmations and auto-launch for that backend
# -------------------------------------------------------------------------------
NEXUS_YOLO_MODE_CLAUDE=false
NEXUS_YOLO_MODE_ZAI=false
NEXUS_YOLO_MODE_KIMI=false
NEXUS_YOLO_MODE_DEEPSEEK=false
NEXUS_YOLO_MODE_GEMINI=false
NEXUS_YOLO_MODE_MISTRAL=false
NEXUS_YOLO_MODE_GROQ=false
NEXUS_YOLO_MODE_TOGETHER=false
NEXUS_YOLO_MODE_OPENROUTER=false
NEXUS_YOLO_MODE_OPENAI=false

# Global YOLO mode - overrides all backends when true
NEXUS_YOLO_MODE=false

# -------------------------------------------------------------------------------
# Enterprise Settings
# -------------------------------------------------------------------------------
# Enable audit logging (logs all backend switches to .promptops-audit.log)
NEXUS_AUDIT_LOG=true

# Default backend when none specified (claude|zai|kimi|deepseek|gemini|mistral|groq|together|openrouter)
NEXUS_DEFAULT_BACKEND=claude

# Verify API keys on switch (true|false)
NEXUS_VERIFY_ON_SWITCH=true

# -------------------------------------------------------------------------------
# Budget Settings (USD)
# -------------------------------------------------------------------------------
NEXUS_DAILY_BUDGET=10.00
NEXUS_WEEKLY_BUDGET=50.00
NEXUS_MONTHLY_BUDGET=100.00

# -------------------------------------------------------------------------------
# LLM API Keys (add your keys here)
# -------------------------------------------------------------------------------

# Anthropic Claude API Key
# Get your API key from: https://console.anthropic.com/
ANTHROPIC_API_KEY=

# OpenAI API Key
# Get your API key from: https://platform.openai.com/
OPENAI_API_KEY=

# Z.AI (GLM/Zhipu AI) API Key
# Get your API key from: https://open.bigmodel.cn/
ZAI_API_KEY=

# Kimi (Moonshot AI) API Key
# Get your API key from: https://platform.moonshot.cn/
KIMI_API_KEY=

# DeepSeek API Key
# Get your API key from: https://platform.deepseek.com/
DEEPSEEK_API_KEY=

# Google Gemini API Key
# Get your API key from: https://ai.google.dev/
GEMINI_API_KEY=

# Mistral API Key
# Get your API key from: https://console.mistral.ai/
MISTRAL_API_KEY=

# Groq API Key
# Get your API key from: https://console.groq.com/
GROQ_API_KEY=

# Together AI API Key
# Get your API key from: https://api.together.xyz/
TOGETHER_API_KEY=

# OpenRouter API Key
# Get your API key from: https://openrouter.ai/
OPENROUTER_API_KEY=
`
	if err := os.WriteFile(envFile, []byte(content), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .env.local: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[OK] Created .env.local")
	fmt.Println("INFO: Please add your API keys to .env.local")
}

func showVersion() {
	fmt.Println("PromptOps Enterprise AI Model Backend Switcher")
	fmt.Printf("Version: %s\n", version)
	fmt.Println()
	fmt.Println("Supported backends:")
	fmt.Println("  Tier 1 (Recommended for code/security):")
	fmt.Println("    - deepseek: DeepSeek V3/R1 - https://api.deepseek.com")
	fmt.Println("    - gemini: Google Gemini 2.5 Pro - https://ai.google.dev")
	fmt.Println("    - mistral: Mistral Large / Codestral - https://console.mistral.ai")
	fmt.Println("    - claude: Anthropic Claude Sonnet 4.5")
	fmt.Println("    - openai: OpenAI GPT-4o / GPT-4o-mini / o1 - https://openai.com")
	fmt.Println("    - zai: Z.AI GLM-4.7 / GLM-4.5-Air")
	fmt.Println("    - kimi: Kimi K2 Thinking / K2 Thinking Turbo")
	fmt.Println()
	fmt.Println("  Tier 2 (Alternative providers):")
	fmt.Println("    - groq: Groq Llama 3.3 70B/405B - https://console.groq.com")
	fmt.Println("    - together: Together AI (Llama/Qwen/DeepSeek) - https://api.together.xyz")
	fmt.Println("    - openrouter: OpenRouter (200+ models) - https://openrouter.ai")
}

func showHelp() {
	fmt.Println("+-------------------------------------------------------------------------------+")
	fmt.Println("|                    PROMPTOPS ENTERPRISE v" + version + "                       |")
	fmt.Println("+-------------------------------------------------------------------------------+")
	fmt.Println()
	fmt.Println("Usage: promptops <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  Tier 1 Backends:")
	fmt.Println("    claude                  Switch to Claude (Anthropic) and launch")
	fmt.Println("    openai                  Switch to OpenAI (GPT-4o/o1) and launch")
	fmt.Println("    zai                     Switch to Z.AI (GLM) and launch")
	fmt.Println("    kimi                    Switch to Kimi (Moonshot) and launch")
	fmt.Println("    deepseek                Switch to DeepSeek (V3/R1) and launch")
	fmt.Println("    gemini                  Switch to Gemini (Google) and launch")
	fmt.Println("    mistral                 Switch to Mistral (Large/Codestral) and launch")
	fmt.Println()
	fmt.Println("  Tier 2 Backends:")
	fmt.Println("    groq                    Switch to Groq (Llama) and launch")
	fmt.Println("    together                Switch to Together AI and launch")
	fmt.Println("    openrouter              Switch to OpenRouter (200+ models) and launch")
	fmt.Println()
	fmt.Println("  Cost Tracking:")
	fmt.Println("    cost                    Show cost dashboard with budgets")
	fmt.Println("    cost log                Show detailed usage log")
	fmt.Println()
	fmt.Println("  Budget Management:")
	fmt.Println("    budget status           Show budget progress")
	fmt.Println("    budget set <period> <amount>  Set budget (daily/weekly/monthly)")
	fmt.Println()
	fmt.Println("  Environment Validation:")
	fmt.Println("    doctor                  Full health check of all backends")
	fmt.Println("    validate <backend>      Validate specific backend connectivity")
	fmt.Println()
	fmt.Println("  Session Management:")
	fmt.Println("    session start <name>    Start a new named session")
	fmt.Println("    session list            List all sessions")
	fmt.Println("    session resume <name>   Resume a previous session")
	fmt.Println("    session info [name]     Show session details")
	fmt.Println("    session close <name>    Close a session")
	fmt.Println("    session cleanup         Remove old closed sessions")
	fmt.Println()
	fmt.Println("  General Commands:")
	fmt.Println("    status                  Show current backend and configuration")
	fmt.Println("    run [args]              Launch Claude Code with current backend")
	fmt.Println("    init                    Initialize .env.local with API key templates")
	fmt.Println("    version                 Show version information")
	fmt.Println("    help                    Show this help message")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  NEXUS_ENV_FILE            Path to env file (default: ./.env.local)")
	fmt.Println("  NEXUS_YOLO_MODE           Global YOLO mode (true|false)")
	fmt.Println("  NEXUS_YOLO_MODE_<BACKEND> YOLO mode for specific backend")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  promptops deepseek        # Switch to DeepSeek and launch Claude Code")
	fmt.Println("  promptops gemini          # Switch to Gemini and launch")
	fmt.Println("  promptops openrouter      # Switch to OpenRouter and launch")
	fmt.Println("  promptops status          # Check current configuration")
	fmt.Println("  promptops run             # Launch with current backend")
	fmt.Println("  promptops doctor          # Run health checks")
	fmt.Println("  promptops session start bugfix-123")
	fmt.Println()
}

// For testing - allows running with mocked input
func readLine(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// ============================================================================
// NEW FEATURES: Session Management, Cost Tracking, Health Checks
// ============================================================================

// Session management functions
func getCurrentSession(cfg *Config) *Session {
	data, err := os.ReadFile(cfg.SessionFile)
	if err != nil {
		return nil
	}
	sessionID := strings.TrimSpace(string(data))
	if sessionID == "" {
		return nil
	}

	sessions := loadSessions(cfg)
	for _, s := range sessions {
		if s.ID == sessionID {
			return &s
		}
	}
	return nil
}

func setCurrentSession(cfg *Config, sessionID string) error {
	return os.WriteFile(cfg.SessionFile, []byte(sessionID), 0644)
}

func loadSessions(cfg *Config) []Session {
	data, err := os.ReadFile(cfg.SessionsFile)
	if err != nil {
		return []Session{}
	}

	var sessions []Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return []Session{}
	}
	return sessions
}

func saveSessions(cfg *Config, sessions []Session) error {
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.SessionsFile, data, 0600)
}

func createSession(cfg *Config, name string) *Session {
	sessions := loadSessions(cfg)

	// Generate unique ID
	sessionID := fmt.Sprintf("%s-%d", name, time.Now().Unix())

	session := Session{
		ID:          sessionID,
		Name:        name,
		Backend:     getCurrentBackend(cfg),
		StartTime:   time.Now(),
		LastActive:  time.Now(),
		WorkingDir:  getWorkingDir(),
		PromptCount: 0,
		TotalCost:   0,
		Status:      "active",
	}

	sessions = append(sessions, session)
	saveSessions(cfg, sessions)
	setCurrentSession(cfg, sessionID)

	return &session
}

func getWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

// Usage tracking functions
func logUsage(cfg *Config, backend string, inputTokens, outputTokens int64) {
	be, ok := backends[backend]
	if !ok {
		return
	}

	// Calculate cost
	inputCost := float64(inputTokens) * be.InputPrice / 1000000
	outputCost := float64(outputTokens) * be.OutputPrice / 1000000
	totalCost := inputCost + outputCost

	record := UsageRecord{
		Timestamp:    time.Now(),
		SessionID:    "",
		Backend:      backend,
		Model:        be.SonnetModel,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      totalCost,
	}

	// Include session ID if available
	session := getCurrentSession(cfg)
	if session != nil {
		record.SessionID = session.ID
	}

	// Append to usage file
	data, _ := json.Marshal(record)
	f, err := os.OpenFile(cfg.UsageFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, string(data))
}

func loadUsageRecords(cfg *Config) []UsageRecord {
	data, err := os.ReadFile(cfg.UsageFile)
	if err != nil {
		return []UsageRecord{}
	}

	var records []UsageRecord
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record UsageRecord
		if err := json.Unmarshal([]byte(line), &record); err == nil {
			records = append(records, record)
		}
	}
	return records
}

func calculateCosts(cfg *Config) (daily, weekly, monthly float64, byBackend map[string]float64) {
	records := loadUsageRecords(cfg)
	byBackend = make(map[string]float64)

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	monthStart := today.AddDate(0, 0, -today.Day()+1)

	for _, r := range records {
		byBackend[r.Backend] += r.CostUSD

		recordDay := r.Timestamp.Truncate(24 * time.Hour)
		if recordDay.Equal(today) {
			daily += r.CostUSD
		}
		if r.Timestamp.After(weekStart) {
			weekly += r.CostUSD
		}
		if r.Timestamp.After(monthStart) {
			monthly += r.CostUSD
		}
	}

	return daily, weekly, monthly, byBackend
}

func formatCurrency(amount float64) string {
	return fmt.Sprintf("$%.2f", amount)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dus", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// Command handlers for new features

func showCostDashboard() {
	cfg := loadConfig()
	dailyCost, weeklyCost, monthlyCost, byBackend := calculateCosts(cfg)

	fmt.Println()
	fmt.Println(styleSection.Render("COST DASHBOARD"))
	fmt.Println()

	fmt.Println(styleSection.Render("SPENDING SUMMARY"))
	renderProgressBar("Today    ", dailyCost, cfg.DailyBudget)
	renderProgressBar("This Week", weeklyCost, cfg.WeeklyBudget)
	renderProgressBar("This Month", monthlyCost, cfg.MonthlyBudget)

	if len(byBackend) > 0 {
		fmt.Println()
		fmt.Println(styleSection.Render("BACKEND BREAKDOWN"))

		// Calculate totals by period per backend
		now := time.Now()
		today := now.Truncate(24 * time.Hour)
		weekStart := today.AddDate(0, 0, -int(today.Weekday()))
		monthStart := today.AddDate(0, 0, -today.Day()+1)

		records := loadUsageRecords(cfg)
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
		for _, cost := range byBackend {
			total += cost
		}

		rows := [][]string{}
		for name, be := range backends {
			if byBackend[name] == 0 {
				continue
			}
			percent := byBackend[name] / total * 100
			rows = append(rows, []string{
				be.DisplayName,
				formatCurrency(backendDaily[name]),
				formatCurrency(backendWeekly[name]),
				formatCurrency(backendMonthly[name]),
				fmt.Sprintf("%.0f%%", percent),
			})
		}

		t := table.New().
			Headers("Backend", "Today", "This Week", "This Month", "%").
			Rows(rows...).
			BorderStyle(lipgloss.NewStyle().Foreground(colorSubtle)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == 0 {
					return lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
				}
				return lipgloss.NewStyle().Padding(0, 1)
			}).
			Width(80)

		fmt.Println(t.Render())
	}

	fmt.Println()
}

func showCostLog() {
	cfg := loadConfig()
	records := loadUsageRecords(cfg)

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
	fmt.Println(styleSection.Render("Recent Usage Records"))

	rows := [][]string{}
	for i := len(records) - 1; i >= start; i-- {
		r := records[i]
		sessionID := r.SessionID
		if len(sessionID) > 18 {
			sessionID = sessionID[:15] + "..."
		}
		if sessionID == "" {
			sessionID = "-"
		}
		rows = append(rows, []string{
			r.Timestamp.Format("2006-01-02 15:04"),
			r.Backend,
			sessionID,
			fmt.Sprintf("%d", r.InputTokens),
			fmt.Sprintf("%d", r.OutputTokens),
			formatCurrency(r.CostUSD),
		})
	}

	t := table.New().
		Headers("Timestamp", "Backend", "Session", "Input", "Output", "Cost").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(colorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Width(100)

	fmt.Println(t.Render())
	fmt.Println()
}

func handleBudgetCommand(args []string) {
	if len(args) == 0 {
		showBudgetStatus()
		return
	}

	subcmd := args[0]
	switch subcmd {
	case "status":
		showBudgetStatus()
	case "set":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: promptops budget set <daily|weekly|monthly> <amount>")
			os.Exit(1)
		}
		setBudget(args[1], args[2])
	default:
		fmt.Fprintf(os.Stderr, "Unknown budget command: %s\n", subcmd)
		os.Exit(1)
	}
}

func showBudgetStatus() {
	cfg := loadConfig()
	dailyCost, weeklyCost, monthlyCost, _ := calculateCosts(cfg)

	fmt.Println()
	fmt.Println(styleSection.Render("BUDGET STATUS"))
	fmt.Println()

	renderProgressBar("Daily  ", dailyCost, cfg.DailyBudget)
	renderProgressBar("Weekly ", weeklyCost, cfg.WeeklyBudget)
	renderProgressBar("Monthly", monthlyCost, cfg.MonthlyBudget)

	fmt.Println()
}

func setBudget(period, amountStr string) {
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid amount: %s\n", amountStr)
		os.Exit(1)
	}

	cfg := loadConfig()
	envFile := cfg.EnvFile

	data, err := os.ReadFile(envFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading .env.local: %v\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "Error: Invalid period '%s'. Use daily, weekly, or monthly.\n", period)
		os.Exit(1)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	found := false
	newLine := fmt.Sprintf("%s=%.2f", varKey, amount)

	for i, line := range lines {
		if strings.HasPrefix(line, varKey+"=") {
			lines[i] = newLine
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, newLine)
	}

	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(envFile, []byte(newContent), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing .env.local: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[OK] Set %s budget to %s\n", period, formatCurrency(amount))
}

func runDoctor() {
	cfg := loadConfig()

	fmt.Println()
	fmt.Println(styleSection.Render("ENVIRONMENT HEALTH CHECK"))
	fmt.Println()

	rows := [][]string{}
	for _, name := range []string{"claude", "openai", "deepseek", "gemini", "mistral", "zai", "kimi", "groq", "together", "openrouter"} {
		be := backends[name]
		result := checkBackendHealth(cfg, be)

		statusStr := ""
		switch result.Status {
		case "ok":
			statusStr = styleSuccess.Render("OK")
		case "skip":
			statusStr = styleMuted.Render("SKIP")
		case "error":
			statusStr = styleError.Render("FAIL")
		}

		latencyStr := "--"
		if result.Latency > 0 {
			latencyStr = formatDuration(result.Latency)
		}

		rows = append(rows, []string{
			be.DisplayName,
			statusStr,
			latencyStr,
			truncate(result.Message, 35),
		})
	}

	t := table.New().
		Headers("Backend", "Status", "Latency", "Message").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(colorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Width(80)

	fmt.Println(t.Render())
	fmt.Println()
}

func validateBackend(name string) {
	cfg := loadConfig()
	be, ok := backends[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: Unknown backend '%s'\n", name)
		os.Exit(1)
	}

	fmt.Printf("Validating %s...\n", be.DisplayName)
	result := checkBackendHealth(cfg, be)

	switch result.Status {
	case "ok":
		fmt.Printf("%s %s is healthy (latency: %s)\n", styleSuccess.Render("✓"), be.DisplayName, formatDuration(result.Latency))
	case "skip":
		fmt.Printf("%s %s - %s\n", styleMuted.Render("-"), be.DisplayName, result.Message)
	case "error":
		fmt.Printf("%s %s - %s\n", styleError.Render("✗"), be.DisplayName, result.Message)
		os.Exit(1)
	}
}

func checkBackendHealth(cfg *Config, be Backend) HealthResult {
	apiKey := cfg.Keys[be.AuthVar]
	if apiKey == "" {
		return HealthResult{Backend: be.Name, Status: "skip", Message: "No API key configured"}
	}

	// Make a lightweight API call to check health
	start := time.Now()

	var url string
	var req *http.Request
	var err error

	switch be.Name {
	case "claude":
		url = "https://api.anthropic.com/v1/models"
		req, err = http.NewRequest("GET", url, nil)
		if err == nil {
			req.Header.Set("x-api-key", apiKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		}
	case "openai":
		url = "https://api.openai.com/v1/models"
		req, err = http.NewRequest("GET", url, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	default:
		// For other backends, just check if we can resolve the base URL
		if be.BaseURL != "" {
			url = be.BaseURL + "/models"
			req, err = http.NewRequest("GET", url, nil)
			if err == nil {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			}
		} else {
			return HealthResult{Backend: be.Name, Status: "skip", Message: "Health check not implemented"}
		}
	}

	if err != nil {
		return HealthResult{Backend: be.Name, Status: "error", Message: err.Error()}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return HealthResult{Backend: be.Name, Status: "error", Latency: latency, Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return HealthResult{Backend: be.Name, Status: "ok", Latency: latency, Message: "Connection verified"}
	}

	body, _ := io.ReadAll(resp.Body)
	return HealthResult{Backend: be.Name, Status: "error", Latency: latency, Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 50))}
}

func handleSessionCommand(args []string) {
	if len(args) == 0 {
		listSessions()
		return
	}

	subcmd := args[0]
	switch subcmd {
	case "start":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: promptops session start <name>")
			os.Exit(1)
		}
		startSession(args[1])
	case "list":
		listSessions()
	case "resume":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: promptops session resume <name>")
			os.Exit(1)
		}
		resumeSession(args[1])
	case "info":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		showSessionInfo(name)
	case "close":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: promptops session close <name>")
			os.Exit(1)
		}
		closeSession(args[1])
	case "cleanup":
		cleanupSessions()
	default:
		fmt.Fprintf(os.Stderr, "Unknown session command: %s\n", subcmd)
		os.Exit(1)
	}
}

func startSession(name string) {
	cfg := loadConfig()

	// Check if session with this name already exists
	sessions := loadSessions(cfg)
	for _, s := range sessions {
		if s.Name == name && s.Status != "closed" {
			fmt.Fprintf(os.Stderr, "Error: Session '%s' already exists (status: %s)\n", name, s.Status)
			os.Exit(1)
		}
	}

	session := createSession(cfg, name)
	fmt.Printf("[OK] Started session '%s' with %s backend\n", session.Name, backends[session.Backend].DisplayName)
}

func listSessions() {
	cfg := loadConfig()
	sessions := loadSessions(cfg)
	current := getCurrentSession(cfg)

	if len(sessions) == 0 {
		fmt.Println("No sessions found. Use 'promptops session start <name>' to create one.")
		return
	}

	fmt.Println()
	fmt.Println(styleSection.Render("SESSIONS"))

	// Sort by last active (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActive.After(sessions[j].LastActive)
	})

	rows := [][]string{}
	for _, s := range sessions {
		marker := " "
		if current != nil && s.ID == current.ID {
			marker = styleAccent.Render(">")
		}

		statusStr := s.Status
		switch s.Status {
		case "active":
			statusStr = styleSuccess.Render(s.Status)
		case "paused":
			statusStr = styleWarning.Render(s.Status)
		case "closed":
			statusStr = styleMuted.Render(s.Status)
		}

		started := s.StartTime.Format("01-02 15:04")

		rows = append(rows, []string{
			marker,
			truncate(s.Name, 14),
			backends[s.Backend].DisplayName,
			started,
			fmt.Sprintf("%d", s.PromptCount),
			formatCurrency(s.TotalCost),
			statusStr,
		})
	}

	t := table.New().
		Headers("", "Name", "Backend", "Started", "Prompts", "Cost", "Status").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(colorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
			}
			if col == 0 {
				return lipgloss.NewStyle().Width(2)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Width(90)

	fmt.Println(t.Render())
	fmt.Println()
}

func resumeSession(name string) {
	cfg := loadConfig()
	sessions := loadSessions(cfg)

	for i, s := range sessions {
		if s.Name == name {
			if s.Status == "closed" {
				fmt.Fprintf(os.Stderr, "Error: Session '%s' is closed\n", name)
				os.Exit(1)
			}

			sessions[i].Status = "active"
			sessions[i].LastActive = time.Now()
			saveSessions(cfg, sessions)
			setCurrentSession(cfg, s.ID)

			// Also switch to the session's backend
			setCurrentBackend(cfg, s.Backend)

			fmt.Printf("[OK] Resumed session '%s' (%s backend)\n", s.Name, backends[s.Backend].DisplayName)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Error: Session '%s' not found\n", name)
	os.Exit(1)
}

func showSessionInfo(name string) {
	cfg := loadConfig()
	sessions := loadSessions(cfg)

	var session *Session
	if name == "" {
		session = getCurrentSession(cfg)
		if session == nil {
			fmt.Println("No active session. Use 'promptops session info <name>' to show a specific session.")
			os.Exit(1)
		}
	} else {
		for i, s := range sessions {
			if s.Name == name {
				session = &sessions[i]
				break
			}
		}
	}

	if session == nil {
		fmt.Fprintf(os.Stderr, "Error: Session '%s' not found\n", name)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(styleSection.Render(fmt.Sprintf("SESSION: %s", session.Name)))
	fmt.Println()

	infoStyle := lipgloss.NewStyle().Width(20).Foreground(colorSubtle)
	valueStyle := lipgloss.NewStyle()

	fmt.Printf("%s %s\n", infoStyle.Render("ID:"), valueStyle.Render(truncate(session.ID, 50)))
	fmt.Printf("%s %s\n", infoStyle.Render("Backend:"), valueStyle.Render(backends[session.Backend].DisplayName))

	statusStr := session.Status
	switch session.Status {
	case "active":
		statusStr = styleSuccess.Render(session.Status)
	case "paused":
		statusStr = styleWarning.Render(session.Status)
	case "closed":
		statusStr = styleMuted.Render(session.Status)
	}
	fmt.Printf("%s %s\n", infoStyle.Render("Status:"), statusStr)

	fmt.Printf("%s %s\n", infoStyle.Render("Started:"), valueStyle.Render(session.StartTime.Format("2006-01-02 15:04:05")))
	fmt.Printf("%s %s\n", infoStyle.Render("Last Active:"), valueStyle.Render(session.LastActive.Format("2006-01-02 15:04:05")))
	fmt.Printf("%s %s\n", infoStyle.Render("Working Dir:"), valueStyle.Render(truncate(session.WorkingDir, 50)))
	fmt.Printf("%s %s\n", infoStyle.Render("Prompts:"), valueStyle.Render(fmt.Sprintf("%d", session.PromptCount)))
	fmt.Printf("%s %s\n", infoStyle.Render("Total Cost:"), valueStyle.Render(formatCurrency(session.TotalCost)))

	fmt.Println()
}

func closeSession(name string) {
	cfg := loadConfig()
	sessions := loadSessions(cfg)
	current := getCurrentSession(cfg)

	for i, s := range sessions {
		if s.Name == name {
			sessions[i].Status = "closed"
			sessions[i].LastActive = time.Now()
			saveSessions(cfg, sessions)

			// If this was the current session, clear it
			if current != nil && s.ID == current.ID {
				os.Remove(cfg.SessionFile)
			}

			fmt.Printf("[OK] Closed session '%s'\n", s.Name)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Error: Session '%s' not found\n", name)
	os.Exit(1)
}

func cleanupSessions() {
	cfg := loadConfig()
	sessions := loadSessions(cfg)

	// Remove sessions closed for more than 30 days
	cutoff := time.Now().AddDate(0, 0, -30)
	var kept []Session
	removed := 0

	for _, s := range sessions {
		if s.Status == "closed" && s.LastActive.Before(cutoff) {
			removed++
		} else {
			kept = append(kept, s)
		}
	}

	if removed > 0 {
		saveSessions(cfg, kept)
		fmt.Printf("[OK] Removed %d old closed sessions\n", removed)
	} else {
		fmt.Println("No old sessions to cleanup")
	}
}
