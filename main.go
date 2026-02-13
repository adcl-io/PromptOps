// Package main implements PromptOps - an AI Model Backend Switcher
// that provides consistent CLI access to multiple LLM providers.
package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var version = "dev"

// Build-time version (set by Makefile with -ldflags)
var buildVersion string

// getVersion returns the effective version, preferring buildVersion if set
func getVersion() string {
	if buildVersion != "" {
		return buildVersion
	}
	return version
}

// Default timeout for API calls (50 minutes)
const defaultTimeout = 50 * time.Minute

// Health check HTTP timeout
const healthCheckTimeout = 5 * time.Second

// Progress bar widths
const (
	progressBarWidth = 40
	miniBarWidth     = 20
)

// HTTP client and request timeouts
const (
	httpClientTimeout  = 10 * time.Second
	maxResponseSize    = 10 * 1024 * 1024 // 10MB
	maxArgLength       = 4096
	maxModelNameLength = 128
	sessionCleanupDays = 30
)

// allowedEnvVars defines which environment variables are safe to pass to child processes
var allowedEnvVars = map[string]bool{
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
	// Ollama specific variables (for local LLM configuration)
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
	// Additional sensitive variables to filter out (never pass to child processes)
	"AWS_SECRET_ACCESS_KEY": true,
	"AWS_ACCESS_KEY_ID":     true,
	"AWS_SESSION_TOKEN":     true,
	"GITHUB_TOKEN":          true,
	"GITHUB_API_TOKEN":      true,
	"GITLAB_TOKEN":          true,
	"KUBECONFIG":            true,
	"DOCKER_CONFIG":         true,
	"NPM_TOKEN":             true,
	"PYPI_TOKEN":            true,
	"GEM_API_KEY":           true,
	"SLACK_TOKEN":           true,
	"SLACK_WEBHOOK_URL":     true,
	"TWILIO_AUTH_TOKEN":     true,
	"SENDGRID_API_KEY":      true,
	"STRIPE_SECRET_KEY":     true,
	"STRIPE_API_KEY":        true,
	"PRIVATE_KEY":           true,
	"SSH_PRIVATE_KEY":       true,
	"PGPASSWORD":            true,
	"MYSQL_PWD":             true,
	"REDIS_PASSWORD":        true,
	"MONGODB_URI":           true,
	"DATABASE_URL":          true,
}

// sanitizeArgs removes potentially dangerous characters from command arguments
func sanitizeArgs(args []string) []string {
	var sanitized []string
	for _, arg := range args {
		// Remove null bytes and control characters
		arg = strings.ReplaceAll(arg, "\x00", "")
		arg = strings.ReplaceAll(arg, "\n", "")
		arg = strings.ReplaceAll(arg, "\r", "")
		// Limit argument length to prevent DoS
		if len(arg) > maxArgLength {
			arg = arg[:maxArgLength]
		}
		sanitized = append(sanitized, arg)
	}
	return sanitized
}

// filterEnvironment returns only whitelisted environment variables
func filterEnvironment(env []string) []string {
	var filtered []string
	for _, e := range env {
		// Handle malformed env vars (no = sign)
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 0 {
			continue
		}
		key := parts[0]
		if key == "" {
			continue
		}
		// Only include if explicitly allowed AND not in the sensitive blocklist
		if allowedEnvVars[key] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// Shared HTTP client with connection pooling and secure TLS
var httpClient = &http.Client{
	Timeout: healthCheckTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		},
	},
}

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
	// Pricing per 1M tokens (USD) - grouped first for alignment
	InputPrice  float64
	OutputPrice float64
	// String fields
	Name        string
	DisplayName string
	Provider    string
	Models      string
	AuthVar     string
	BaseURL     string
	Timeout     time.Duration
	HaikuModel  string
	SonnetModel string
	OpusModel   string
	// Coding capability tier (S/A/B/C)
	CodingTier string
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
		CodingTier:  "S",
	},
	"zai": {
		Name:        "zai",
		DisplayName: "Z.AI",
		Provider:    "Z.AI (Zhipu AI)",
		Models:      "GLM-5 (Sonnet/Opus) / GLM-4.5-Air (Haiku)",
		AuthVar:     "ZAI_API_KEY",
		BaseURL:     "https://api.z.ai/api/anthropic",
		Timeout:     defaultTimeout,
		HaikuModel:  "glm-4.5-air",
		SonnetModel: "glm-5",
		OpusModel:   "glm-5",
		InputPrice:  0.50,
		OutputPrice: 2.00,
		CodingTier:  "A",
	},
	"kimi": {
		Name:        "kimi",
		DisplayName: "Kimi",
		Provider:    "Kimi Code (Subscription)",
		Models:      "kimi-for-coding",
		AuthVar:     "KIMI_API_KEY",
		BaseURL:     "https://api.kimi.com/coding",
		Timeout:     defaultTimeout,
		HaikuModel:  "kimi-for-coding",
		SonnetModel: "kimi-for-coding",
		OpusModel:   "kimi-for-coding",
		InputPrice:  2.00,
		OutputPrice: 8.00,
		CodingTier:  "S",
	},
	"deepseek": {
		Name:        "deepseek",
		DisplayName: "DeepSeek",
		Provider:    "DeepSeek AI",
		Models:      "DeepSeek-V3 / DeepSeek-R1",
		AuthVar:     "DEEPSEEK_API_KEY",
		BaseURL:     "https://api.deepseek.com/v1",
		Timeout:     defaultTimeout,
		HaikuModel:  "deepseek-chat",
		SonnetModel: "deepseek-reasoner",
		OpusModel:   "deepseek-reasoner",
		InputPrice:  0.27,
		OutputPrice: 1.10,
		CodingTier:  "S",
	},
	"gemini": {
		Name:        "gemini",
		DisplayName: "Gemini",
		Provider:    "Google AI",
		Models:      "Gemini 2.5 Pro",
		AuthVar:     "GEMINI_API_KEY",
		BaseURL:     "https://generativelanguage.googleapis.com/v1beta/openai",
		Timeout:     defaultTimeout,
		HaikuModel:  "gemini-2.5-flash",
		SonnetModel: "gemini-2.5-pro",
		OpusModel:   "gemini-2.5-pro",
		InputPrice:  1.25,
		OutputPrice: 10.00,
		CodingTier:  "A",
	},
	"mistral": {
		Name:        "mistral",
		DisplayName: "Mistral",
		Provider:    "Mistral AI",
		Models:      "Mistral Large / Codestral",
		AuthVar:     "MISTRAL_API_KEY",
		BaseURL:     "https://api.mistral.ai/v1",
		Timeout:     defaultTimeout,
		HaikuModel:  "codestral-latest",
		SonnetModel: "mistral-large-latest",
		OpusModel:   "mistral-large-latest",
		InputPrice:  2.00,
		OutputPrice: 6.00,
		CodingTier:  "B",
	},
	"groq": {
		Name:        "groq",
		DisplayName: "Groq",
		Provider:    "Groq (Llama)",
		Models:      "Llama 3.3 70B / 405B",
		AuthVar:     "GROQ_API_KEY",
		BaseURL:     "https://api.groq.com/openai/v1",
		Timeout:     defaultTimeout,
		HaikuModel:  "llama-3.3-70b-versatile",
		SonnetModel: "llama-3.3-70b-versatile",
		OpusModel:   "llama-3.1-405b-reasoning",
		InputPrice:  0.59,
		OutputPrice: 0.79,
		CodingTier:  "B",
	},
	"together": {
		Name:        "together",
		DisplayName: "Together AI",
		Provider:    "Together AI",
		Models:      "Llama / Qwen / DeepSeek",
		AuthVar:     "TOGETHER_API_KEY",
		BaseURL:     "https://api.together.xyz/v1",
		Timeout:     defaultTimeout,
		HaikuModel:  "meta-llama/Llama-3.3-70B-Instruct-Turbo",
		SonnetModel: "deepseek-ai/DeepSeek-V3",
		OpusModel:   "meta-llama/Llama-3.1-405B-Instruct",
		InputPrice:  1.00,
		OutputPrice: 2.00,
		CodingTier:  "B",
	},
	"openrouter": {
		Name:        "openrouter",
		DisplayName: "OpenRouter",
		Provider:    "OpenRouter",
		Models:      "200+ models via meta-router",
		AuthVar:     "OPENROUTER_API_KEY",
		BaseURL:     "https://openrouter.ai/api/v1",
		Timeout:     defaultTimeout,
		HaikuModel:  "google/gemini-flash-1.5",
		SonnetModel: "anthropic/claude-3.5-sonnet",
		OpusModel:   "anthropic/claude-3-opus",
		InputPrice:  3.00,
		OutputPrice: 15.00,
		CodingTier:  "A",
	},
	"openai": {
		Name:        "openai",
		DisplayName: "OpenAI",
		Provider:    "OpenAI",
		Models:      "GPT-4o / GPT-4o-mini / o1",
		AuthVar:     "OPENAI_API_KEY",
		BaseURL:     "https://api.openai.com/v1",
		Timeout:     defaultTimeout,
		HaikuModel:  "gpt-4o-mini",
		SonnetModel: "gpt-4o",
		OpusModel:   "o1",
		InputPrice:  2.50,
		OutputPrice: 10.00,
		CodingTier:  "A",
	},
	"ollama": {
		Name:        "ollama",
		DisplayName: "Ollama",
		Provider:    "Ollama (Local)",
		Models:      "llama3.2 / codellama / mistral",
		AuthVar:     "OLLAMA_API_KEY",
		BaseURL:     "http://localhost:11434/v1",
		Timeout:     defaultTimeout,
		HaikuModel:  "llama3.2",
		SonnetModel: "codellama",
		OpusModel:   "llama3.3",
		InputPrice:  0.00,
		OutputPrice: 0.00,
		CodingTier:  "B",
	},
}

type Config struct {
	EnvFile        string
	StateFile      string
	AuditLog       string
	UsageFile      string
	SessionsFile   string
	SessionFile    string
	YoloMode       bool
	YoloModes      map[string]bool // Per-backend YOLO mode settings
	DefaultBackend string
	VerifyOnSwitch bool
	AuditEnabled   bool
	Keys           map[string]string
	// Budget settings
	DailyBudget   float64
	WeeklyBudget  float64
	MonthlyBudget float64
	// Ollama model configuration (allows user to specify local models)
	OllamaModels map[string]string // haiku/sonnet/opus -> model name
	// Z.AI model configuration (allows user to specify GLM model versions)
	ZAIModels map[string]string // haiku/sonnet/opus -> model name
	// Kimi model configuration (allows user to specify Kimi model versions)
	KimiModels map[string]string // haiku/sonnet/opus -> model name
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
	case "claude", "zai", "kimi", "deepseek", "gemini", "mistral", "groq", "together", "openrouter", "openai", "ollama":
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
	// Usage command - fetch real API usage from providers
	case "usage":
		showAPIUsage(args)
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command '%s'. Run 'promptops help' for usage.\n", cmd)
		os.Exit(1)
	}
}

func getScriptDir() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		// Fallback to working directory if executable path unavailable
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			return "", fmt.Errorf("cannot determine script directory: executable error: %w; wd error: %w", err, wdErr)
		}
		return wd, nil
	}
	return filepath.Dir(ex), nil
}

func loadConfig() *Config {
	dir, err := getScriptDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	envFile := os.Getenv("NEXUS_ENV_FILE")
	if envFile != "" {
		// Validate to prevent path traversal using EvalSymlinks
		cleanPath := filepath.Clean(envFile)
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: NEXUS_ENV_FILE invalid path: %s\n", envFile)
			os.Exit(1)
		}
		// Resolve symlinks to prevent bypass
		resolvedPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			// File may not exist yet, check parent directory
			parentDir := filepath.Dir(absPath)
			resolvedParent, err := filepath.EvalSymlinks(parentDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: NEXUS_ENV_FILE parent directory invalid: %s\n", envFile)
				os.Exit(1)
			}
			// Reconstruct path with resolved parent
			resolvedPath = filepath.Join(resolvedParent, filepath.Base(absPath))
		}
		// Ensure resolved path is within allowed directories (home or script dir)
		homeDir, _ := os.UserHomeDir()
		scriptDir, _ := getScriptDir()
		inHome := homeDir != "" && strings.HasPrefix(resolvedPath, homeDir+string(filepath.Separator))
		inScript := scriptDir != "" && strings.HasPrefix(resolvedPath, scriptDir+string(filepath.Separator))
		isHomeFile := homeDir != "" && resolvedPath == homeDir
		isScriptFile := scriptDir != "" && resolvedPath == scriptDir
		if !inHome && !inScript && !isHomeFile && !isScriptFile {
			fmt.Fprintf(os.Stderr, "Error: NEXUS_ENV_FILE must be within home or script directory: %s\n", envFile)
			os.Exit(1)
		}
		envFile = resolvedPath
	} else {
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
		YoloModes:      make(map[string]bool),
		OllamaModels:   make(map[string]string),
		ZAIModels:      make(map[string]string),
		KimiModels:     make(map[string]string),
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
				cfg.YoloModes["claude"] = value == "true"
			case "NEXUS_YOLO_MODE_ZAI":
				cfg.YoloModes["zai"] = value == "true"
			case "NEXUS_YOLO_MODE_KIMI":
				cfg.YoloModes["kimi"] = value == "true"
			case "NEXUS_YOLO_MODE_DEEPSEEK":
				cfg.YoloModes["deepseek"] = value == "true"
			case "NEXUS_YOLO_MODE_GEMINI":
				cfg.YoloModes["gemini"] = value == "true"
			case "NEXUS_YOLO_MODE_MISTRAL":
				cfg.YoloModes["mistral"] = value == "true"
			case "NEXUS_YOLO_MODE_GROQ":
				cfg.YoloModes["groq"] = value == "true"
			case "NEXUS_YOLO_MODE_TOGETHER":
				cfg.YoloModes["together"] = value == "true"
			case "NEXUS_YOLO_MODE_OPENROUTER":
				cfg.YoloModes["openrouter"] = value == "true"
			case "NEXUS_YOLO_MODE_OPENAI":
				cfg.YoloModes["openai"] = value == "true"
			case "NEXUS_YOLO_MODE_OLLAMA":
				cfg.YoloModes["ollama"] = value == "true"
			case "NEXUS_DEFAULT_BACKEND":
				cfg.DefaultBackend = value
			case "NEXUS_VERIFY_ON_SWITCH":
				cfg.VerifyOnSwitch = value == "true"
			case "NEXUS_AUDIT_LOG":
				cfg.AuditEnabled = value == "true"
			case "NEXUS_DAILY_BUDGET":
				if v, err := strconv.ParseFloat(value, 64); err == nil {
					cfg.DailyBudget = v
				} else {
					fmt.Fprintf(os.Stderr, "Warning: invalid NEXUS_DAILY_BUDGET value '%s': %v\n", value, err)
				}
			case "NEXUS_WEEKLY_BUDGET":
				if v, err := strconv.ParseFloat(value, 64); err == nil {
					cfg.WeeklyBudget = v
				} else {
					fmt.Fprintf(os.Stderr, "Warning: invalid NEXUS_WEEKLY_BUDGET value '%s': %v\n", value, err)
				}
			case "NEXUS_MONTHLY_BUDGET":
				if v, err := strconv.ParseFloat(value, 64); err == nil {
					cfg.MonthlyBudget = v
				} else {
					fmt.Fprintf(os.Stderr, "Warning: invalid NEXUS_MONTHLY_BUDGET value '%s': %v\n", value, err)
				}
			case "ANTHROPIC_API_KEY", "ZAI_API_KEY", "KIMI_API_KEY", "DEEPSEEK_API_KEY", "GEMINI_API_KEY", "MISTRAL_API_KEY", "GROQ_API_KEY", "TOGETHER_API_KEY", "OPENROUTER_API_KEY", "OPENAI_API_KEY", "OLLAMA_API_KEY":
				cfg.Keys[key] = value
			// Ollama model configuration - allow custom local models
			case "OLLAMA_HAIKU_MODEL":
				cfg.OllamaModels["haiku"] = value
			case "OLLAMA_SONNET_MODEL":
				cfg.OllamaModels["sonnet"] = value
			case "OLLAMA_OPUS_MODEL":
				cfg.OllamaModels["opus"] = value
			// Z.AI model configuration - allow custom GLM model versions
			case "ZAI_HAIKU_MODEL":
				cfg.ZAIModels["haiku"] = value
			case "ZAI_SONNET_MODEL":
				cfg.ZAIModels["sonnet"] = value
			case "ZAI_OPUS_MODEL":
				cfg.ZAIModels["opus"] = value
			// Kimi model configuration - allow custom model versions
			case "KIMI_HAIKU_MODEL":
				cfg.KimiModels["haiku"] = value
			case "KIMI_SONNET_MODEL":
				cfg.KimiModels["sonnet"] = value
			case "KIMI_OPUS_MODEL":
				cfg.KimiModels["opus"] = value
			}
		}
	}

	return cfg
}

func (c *Config) getYoloMode(backend string) bool {
	if c.YoloMode {
		return true
	}
	if c.YoloModes != nil {
		if v, ok := c.YoloModes[backend]; ok {
			return v
		}
	}
	return true
}

func getCurrentBackend(cfg *Config) string {
	data, err := os.ReadFile(cfg.StateFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func setCurrentBackend(cfg *Config, backend string) error {
	return writeFileAtomic(cfg.StateFile, []byte(backend), 0600)
}

// writeFileAtomic writes data to a file atomically using temp file + rename
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	// Create temp file with restricted permissions from the start (0600)
	// This prevents race condition between CreateTemp and Chmod
	tmpFile, err := os.OpenFile(filepath.Join(dir, ".tmp-"+strconv.FormatInt(time.Now().UnixNano(), 10)),
		os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Cleanup on error
	defer func() {
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	if _, err = tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Apply requested permissions (for sensitive files this should be 0600)
	if perm != 0600 {
		if err = os.Chmod(tmpPath, perm); err != nil {
			return fmt.Errorf("chmod temp file: %w", err)
		}
	}

	if err = os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// Constants for key masking
const (
	maskKeyMinLength     = 8
	maskKeyVisiblePrefix = 4
	maskKeyVisibleSuffix = 4
	maskKeyReplacement   = "****"
)

// modelNameRegex validates model names against whitelist pattern
// Allows: alphanumeric, underscore, hyphen, colon, forward slash, period
// Max length: 128 characters
var modelNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-:/.]+$`)

// validateModelName validates a model name against security constraints
func validateModelName(model string) error {
	if model == "" {
		return errors.New("model name cannot be empty")
	}
	if len(model) > maxModelNameLength {
		return fmt.Errorf("model name exceeds maximum length of %d characters", maxModelNameLength)
	}
	if !modelNameRegex.MatchString(model) {
		return fmt.Errorf("model name contains invalid characters: must match pattern %s", modelNameRegex.String())
	}
	return nil
}

// sanitizeError removes potentially sensitive information from error messages
func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()

	// Remove common API key patterns
	sensitivePatterns := []string{
		`sk-[a-zA-Z0-9]{20,}`,
		`sk-(?:ant-|kimi-|proj-)[a-zA-Z0-9_-]{10,}`,
		`[a-zA-Z0-9]{32,}`,
		`Bearer\s+[a-zA-Z0-9_-]+`,
		`api[_-]?key[=:]\s*[a-zA-Z0-9_-]+`,
	}

	for _, pattern := range sensitivePatterns {
		re := regexp.MustCompile(pattern)
		errStr = re.ReplaceAllString(errStr, "[REDACTED]")
	}

	return errors.New(errStr)
}

func maskKey(key string) string {
	if len(key) <= maskKeyMinLength {
		return maskKeyReplacement
	}
	// Consistent masking: always show first 4 and last 4
	return key[:maskKeyVisiblePrefix] + maskKeyReplacement + key[len(key)-maskKeyVisibleSuffix:]
}

func auditLog(cfg *Config, msg string) {
	if !cfg.AuditEnabled {
		return
	}
	f, err := os.OpenFile(cfg.AuditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to open audit log: %v\n", err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close audit log: %v\n", err)
		}
	}()

	// Include session ID if available
	session := getCurrentSession(cfg)
	if session != nil {
		msg = fmt.Sprintf("[%s] %s", session.Name, msg)
	}

	if _, err := fmt.Fprintf(f, "[%s] %s\n", time.Now().Format(time.RFC3339), msg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write audit log: %v\n", err)
	}
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
	case "ollama":
		fmt.Println("   ██████  ██      ██       █████  ███    ███  █████")
		fmt.Println("  ██    ██ ██      ██      ██   ██ ████  ████ ██   ██")
		fmt.Println("  ██    ██ ██      ██      ███████ ██ ████ ██ ███████")
		fmt.Println("  ██    ██ ██      ██      ██   ██ ██  ██  ██ ██   ██")
		fmt.Println("   ██████  ███████ ███████ ██   ██ ██      ██ ██   ██")
		fmt.Println("  OLLAMA - LOCAL LLM INFERENCE")
	}
}

func drawBox(msg string) {
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

func animateSwitch(msg string) {
	// ASCII spinner frames - pure ASCII as per CLAUDE.md
	frames := []string{"|", "/", "-", "\\"}
	for i := 0; i < 20; i++ {
		fmt.Printf("\r%s %s", frames[i%4], msg)
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("\r[OK] %s\n", msg)
}

func showProgress(msg string) {
	fmt.Printf("%s [", msg)
	for i := 0; i < 30; i++ {
		fmt.Print(styleProgressFilled.Render("█"))
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

	// Check for API key (not required for local backends like Ollama)
	apiKey := cfg.Keys[be.AuthVar]
	if apiKey == "" && be.Name != "ollama" {
		fmt.Fprintf(os.Stderr, "Error: %s not set in .env.local\n", be.AuthVar)
		os.Exit(1)
	}

	yolo := cfg.getYoloMode(name)

	// Animations
	if !yolo {
		// Animation messages for all backends
		animMsgs := map[string]string{
			"claude":     "Initializing neural pathways...",
			"zai":        "Reconfiguring quantum matrices...",
			"kimi":       "Engaging moonshot protocols...",
			"deepseek":   "Activating deep reasoning...",
			"gemini":     "Initializing Gemini core...",
			"mistral":    "Loading Mistral engines...",
			"groq":       "Establishing Groq connection...",
			"together":   "Connecting to Together AI...",
			"openrouter": "Routing through OpenRouter...",
			"openai":     "Connecting to OpenAI...",
			"ollama":     "Starting local inference engine...",
		}
		if msg, ok := animMsgs[name]; ok {
			animateSwitch(msg)
		}
		fmt.Println()
		printLogo(name)
		fmt.Println()

		// Progress messages for all backends
		progressMsgs := map[string]string{
			"claude":     "Connecting to Anthropic",
			"zai":        "Connecting to Z.AI",
			"kimi":       "Connecting to Kimi Code",
			"deepseek":   "Connecting to DeepSeek",
			"gemini":     "Connecting to Google AI",
			"mistral":    "Connecting to Mistral AI",
			"groq":       "Connecting to Groq",
			"together":   "Connecting to Together AI",
			"openrouter": "Connecting to OpenRouter",
			"openai":     "Connecting to OpenAI",
			"ollama":     "Connecting to local Ollama",
		}
		if msg, ok := progressMsgs[name]; ok {
			showProgress(msg)
		}
	}

	// Save state
	if err := setCurrentBackend(cfg, name); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
		os.Exit(1)
	}

	// Audit log - never log API keys even masked
	auditLog(cfg, fmt.Sprintf("SWITCH: %s", name))

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
		cmdArgs = append(cmdArgs, "--dangerously-skip-permissions")
	}

	// Sanitize user-provided arguments
	sanitizedArgs := sanitizeArgs(args)
	cmdArgs = append(cmdArgs, sanitizedArgs...)

	cmd := exec.Command("claude", cmdArgs...)

	// Build environment with whitelist approach
	env := filterEnvironment(os.Environ())

	// Set auth token for Claude Code
	// Note: For backends like Ollama that don't require API keys, we still need
	// to set ANTHROPIC_AUTH_TOKEN for Claude Code itself
	apiKey := cfg.Keys[be.AuthVar]
	if apiKey != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_AUTH_TOKEN=%s", apiKey))
	} else if be.Name == "ollama" {
		// Ollama doesn't require an API key, but Claude Code still needs
		// ANTHROPIC_AUTH_TOKEN to be set when using a custom base URL
		env = append(env, "ANTHROPIC_AUTH_TOKEN=ollama")
	}

	// Set backend-specific vars
	baseURL := be.BaseURL
	if be.BaseURL != "" {
		env = append(env, fmt.Sprintf("API_TIMEOUT_MS=%d", be.Timeout.Milliseconds()))

		// Use custom Ollama models if configured, otherwise use defaults
		haikuModel := be.HaikuModel
		sonnetModel := be.SonnetModel
		opusModel := be.OpusModel

		if be.Name == "ollama" {
			if m, ok := cfg.OllamaModels["haiku"]; ok && m != "" {
				haikuModel = strings.TrimSpace(m)
			}
			if m, ok := cfg.OllamaModels["sonnet"]; ok && m != "" {
				sonnetModel = strings.TrimSpace(m)
			}
			if m, ok := cfg.OllamaModels["opus"]; ok && m != "" {
				opusModel = strings.TrimSpace(m)
			}
		}

		if be.Name == "zai" {
			if m, ok := cfg.ZAIModels["haiku"]; ok && m != "" {
				haikuModel = strings.TrimSpace(m)
			}
			if m, ok := cfg.ZAIModels["sonnet"]; ok && m != "" {
				sonnetModel = strings.TrimSpace(m)
			}
			if m, ok := cfg.ZAIModels["opus"]; ok && m != "" {
				opusModel = strings.TrimSpace(m)
			}
		}

		if be.Name == "kimi" {
			if m, ok := cfg.KimiModels["haiku"]; ok && m != "" {
				haikuModel = strings.TrimSpace(m)
			}
			if m, ok := cfg.KimiModels["sonnet"]; ok && m != "" {
				sonnetModel = strings.TrimSpace(m)
			}
			if m, ok := cfg.KimiModels["opus"]; ok && m != "" {
				opusModel = strings.TrimSpace(m)
			}
		}

		// Validate model names before setting environment variables
		if err := validateModelName(haikuModel); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid haiku model name: %v\n", err)
			os.Exit(1)
		}
		if err := validateModelName(sonnetModel); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid sonnet model name: %v\n", err)
			os.Exit(1)
		}
		if err := validateModelName(opusModel); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid opus model name: %v\n", err)
			os.Exit(1)
		}

		env = append(env, fmt.Sprintf("ANTHROPIC_DEFAULT_HAIKU_MODEL=%s", haikuModel))
		env = append(env, fmt.Sprintf("ANTHROPIC_DEFAULT_SONNET_MODEL=%s", sonnetModel))
		env = append(env, fmt.Sprintf("ANTHROPIC_DEFAULT_OPUS_MODEL=%s", opusModel))
	}

	// For Ollama, start a proxy to translate Anthropic API to OpenAI format
	var proxy *OllamaProxy
	if be.Name == "ollama" {
		proxy = NewOllamaProxy(baseURL, buildModelMap(cfg))
		if err := proxy.Start(18080); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting Ollama proxy: %v\n", err)
			os.Exit(1)
		}
		// Point Claude Code to our proxy instead of directly to Ollama
		baseURL = "http://localhost:18080"
		if !yolo {
			fmt.Println("[OK] Started Anthropic-to-OpenAI proxy on port 18080")
		}
	}

	// Set the base URL (may have been changed to proxy for Ollama)
	env = append(env, fmt.Sprintf("ANTHROPIC_BASE_URL=%s", baseURL))

	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	// Stop the proxy if it was started
	if proxy != nil {
		proxy.Stop()
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error launching claude: %v\n", err)
		os.Exit(1)
	}
}

// buildModelMap creates a mapping from Anthropic model names to Ollama model names
func buildModelMap(cfg *Config) map[string]string {
	modelMap := map[string]string{
		"llama3.2":         "llama3.2:latest",
		"llama3.2:latest":  "llama3.2:latest",
		"llama3.2:3b":      "llama3.2:3b",
		"codellama":        "codellama:latest",
		"codellama:latest": "codellama:latest",
		"phi3":             "phi3:latest",
		"phi3:latest":      "phi3:latest",
		"mistral":          "mistral:latest",
		"mistral:latest":   "mistral:latest",
		"llama3.3":         "llama3.3:latest",
		"llama3.3:latest":  "llama3.3:latest",
	}

	// Add custom models from config
	if cfg.OllamaModels != nil {
		if m, ok := cfg.OllamaModels["haiku"]; ok && m != "" {
			validated := strings.TrimSpace(m)
			if err := validateModelName(validated); err == nil {
				modelMap[m] = validated
				modelMap["haiku"] = validated
			}
		}
		if m, ok := cfg.OllamaModels["sonnet"]; ok && m != "" {
			validated := strings.TrimSpace(m)
			if err := validateModelName(validated); err == nil {
				modelMap[m] = validated
				modelMap["sonnet"] = validated
			}
		}
		if m, ok := cfg.OllamaModels["opus"]; ok && m != "" {
			validated := strings.TrimSpace(m)
			if err := validateModelName(validated); err == nil {
				modelMap[m] = validated
				modelMap["opus"] = validated
			}
		}
	}

	return modelMap
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

// formatCustomModels returns a formatted string of custom models for the given backend
func formatCustomModels(backend string, cfg *Config) string {
	var models map[string]string
	switch backend {
	case "ollama":
		models = cfg.OllamaModels
	case "zai":
		models = cfg.ZAIModels
	case "kimi":
		models = cfg.KimiModels
	default:
		return ""
	}

	if len(models) == 0 {
		return ""
	}

	var customModels []string
	if m, ok := models["haiku"]; ok && m != "" {
		customModels = append(customModels, fmt.Sprintf("haiku=%s", m))
	}
	if m, ok := models["sonnet"]; ok && m != "" {
		customModels = append(customModels, fmt.Sprintf("sonnet=%s", m))
	}
	if m, ok := models["opus"]; ok && m != "" {
		customModels = append(customModels, fmt.Sprintf("opus=%s", m))
	}

	return strings.Join(customModels, ", ")
}

func showStatus() {
	cfg := loadConfig()
	current := getCurrentBackend(cfg)
	session := getCurrentSession(cfg)
	dailyCost, weeklyCost, monthlyCost, byBackend := calculateCosts(cfg)

	// Check for --check flag to enable health check/latency
	checkLatency := false
	for _, arg := range os.Args {
		if arg == "--check" || arg == "--latency" {
			checkLatency = true
			break
		}
	}

	// Title
	fmt.Println()
	title := styleTitle.Render(fmt.Sprintf("PROMPTOPS v%s", getVersion()))
	fmt.Println(lipgloss.PlaceHorizontal(80, lipgloss.Center, title))
	fmt.Println()

	// Current Backend Section
	fmt.Println(styleSection.Render("CURRENT BACKEND"))
	if current != "" {
		be, ok := backends[current]
		if !ok {
			fmt.Println(styleError.Render("Invalid backend in state: " + current))
			current = ""
		} else {
			status := styleCurrent.Render("> " + be.DisplayName)
			if cfg.getYoloMode(current) {
				status += styleWarning.Render(" [YOLO]")
			}
			fmt.Println(status)
			fmt.Println(styleMuted.Render(be.Models))
			// Show custom models if configured
			if custom := formatCustomModels(be.Name, cfg); custom != "" {
				fmt.Println(styleWarning.Render("Custom: " + custom))
			}
		}
	}
	if current == "" {
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

	backendOrder := []string{"claude", "openai", "deepseek", "gemini", "mistral", "zai", "kimi", "groq", "together", "openrouter", "ollama"}

	rows := [][]string{}
	for _, name := range backendOrder {
		be, ok := backends[name]
		if !ok {
			continue // Skip unknown backends
		}
		hasKey := cfg.Keys[be.AuthVar] != ""

		marker := " "
		if name == current {
			marker = styleAccent.Render(">")
		}

		status := styleSuccess.Render("Ready")
		extraCol := "--"

		if !hasKey {
			if be.Name == "ollama" {
				status = styleSuccess.Render("Local")
			} else {
				status = styleMuted.Render("No Key")
			}
		} else if checkLatency {
			result := checkBackendHealth(cfg, be)
			if result.Status == "ok" {
				extraCol = formatDuration(result.Latency)
			} else if result.Status == "error" {
				status = styleError.Render("Error")
			}
		}

		// Show cost - subscription models highlighted differently
		if !checkLatency {
			costStr := fmt.Sprintf("$%.2f/$%.2f", be.InputPrice, be.OutputPrice)
			if name == "kimi" || name == "zai" {
				// Subscription models - show cost with "Sub" indicator
				extraCol = styleMuted.Render("Sub " + costStr)
			} else {
				// Token-based models
				extraCol = costStr
			}
		}

		// Style the coding tier
		tierStr := be.CodingTier
		switch tierStr {
		case "S":
			tierStr = styleSuccess.Render(tierStr)
		case "A":
			tierStr = lipgloss.NewStyle().Foreground(colorAccent).Render(tierStr)
		case "B":
			tierStr = lipgloss.NewStyle().Foreground(colorText).Render(tierStr)
		case "C":
			tierStr = styleMuted.Render(tierStr)
		}

		rows = append(rows, []string{
			marker,
			be.DisplayName,
			truncate(be.Models, 22),
			status,
			tierStr,
			extraCol,
		})
	}

	header := "Cost ($in/out per 1M)"
	if checkLatency {
		header = "Latency"
	}

	t := table.New().
		Headers("", "Provider", "Models", "Status", "Tier", header).
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(colorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Padding(0, 1)
			}
			if col == 0 {
				return lipgloss.NewStyle().Width(2)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Width(90)

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

	filled := int(percent * float64(progressBarWidth) / 100)
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
	emptyBar := lipgloss.NewStyle().Background(colorMuted).Render(strings.Repeat(" ", progressBarWidth-filled))

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
	filled := int(percent * float64(miniBarWidth) / 100)
	if filled < 0 {
		filled = 0
	}
	if filled > miniBarWidth {
		filled = miniBarWidth
	}

	barColor := colorSuccess
	if percent >= 50 {
		barColor = colorWarning
	}
	if percent >= 80 {
		barColor = colorError
	}

	filledBar := lipgloss.NewStyle().Background(barColor).Render(strings.Repeat(" ", filled))
	emptyBar := lipgloss.NewStyle().Background(colorMuted).Render(strings.Repeat(" ", miniBarWidth-filled))

	return filledBar + emptyBar + fmt.Sprintf(" %.0f%%", percent)
}

func initEnv() {
	dir, err := getScriptDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
# DEFAULT: true (skip confirmations and auto-launch)
# Set to "false" if you want Claude Code to prompt for permissions
# -------------------------------------------------------------------------------
# NEXUS_YOLO_MODE_CLAUDE=false
# NEXUS_YOLO_MODE_ZAI=false
# NEXUS_YOLO_MODE_KIMI=false
# NEXUS_YOLO_MODE_DEEPSEEK=false
# NEXUS_YOLO_MODE_GEMINI=false
# NEXUS_YOLO_MODE_MISTRAL=false
# NEXUS_YOLO_MODE_GROQ=false
# NEXUS_YOLO_MODE_TOGETHER=false
# NEXUS_YOLO_MODE_OPENROUTER=false
# NEXUS_YOLO_MODE_OPENAI=false
# NEXUS_YOLO_MODE_OLLAMA=false

# Global YOLO mode - overrides all backends when set
# NEXUS_YOLO_MODE=false

# -------------------------------------------------------------------------------
# Enterprise Settings
# -------------------------------------------------------------------------------
# Enable audit logging (logs all backend switches to .promptops-audit.log)
NEXUS_AUDIT_LOG=true

# Default backend when none specified (claude|zai|kimi|deepseek|gemini|mistral|groq|together|openrouter|ollama)
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

# Ollama (optional - local backend, no key required by default)
# Ollama runs locally at http://localhost:11434
# Only set this if you've configured Ollama with authentication
OLLAMA_API_KEY=

# Ollama Model Configuration (optional - defaults shown below)
# Set these to use specific local models instead of the defaults
# Defaults: llama3.2 (haiku), codellama (sonnet), llama3.3 (opus)
# Examples: qwen2.5-coder, phi4, mistral, gemma2, deepseek-coder
# OLLAMA_HAIKU_MODEL=llama3.2
# OLLAMA_SONNET_MODEL=codellama
# OLLAMA_OPUS_MODEL=llama3.3

# Z.AI Model Configuration (optional - defaults shown below)
# Set these to use specific GLM model versions instead of the defaults
# Defaults: glm-4.5-air (haiku), glm-5 (sonnet), glm-5 (opus)
# ZAI_HAIKU_MODEL=glm-4.5-air
# ZAI_SONNET_MODEL=glm-5
# ZAI_OPUS_MODEL=glm-5

# Kimi Model Configuration (optional - defaults shown below)
# Set these to use specific Kimi model versions instead of the defaults
# Defaults: kimi-for-coding (all tiers)
# KIMI_HAIKU_MODEL=kimi-for-coding
# KIMI_SONNET_MODEL=kimi-for-coding
# KIMI_OPUS_MODEL=kimi-for-coding
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
	fmt.Printf("Version: %s\n", getVersion())
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
	fmt.Println()
	fmt.Println("  Local (Self-hosted):")
	fmt.Println("    - ollama: Ollama Local LLM - http://localhost:11434")
}

func showHelp() {
	fmt.Println("+-------------------------------------------------------------------------------+")
	fmt.Println("|                    PROMPTOPS ENTERPRISE v" + getVersion() + "                       |")
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
	fmt.Println("  Local Backends:")
	fmt.Println("    ollama                  Switch to Ollama (local) and launch")
	fmt.Println()
	fmt.Println("  Cost Tracking:")
	fmt.Println("    cost                    Show cost dashboard with budgets")
	fmt.Println("    cost log                Show detailed usage log")
	fmt.Println()
	fmt.Println("  API Usage:")
	fmt.Println("    usage                   Show usage data from all provider APIs")
	fmt.Println("    usage <backend>         Show usage for specific backend")
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
	fmt.Println("    usage [backend]         Check API usage from provider APIs")
	fmt.Println("    init                    Initialize .env.local with API key templates")
	fmt.Println("    version                 Show version information")
	fmt.Println("    help                    Show this help message")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  NEXUS_ENV_FILE            Path to env file (default: ./.env.local)")
	fmt.Println("  NEXUS_YOLO_MODE           Global YOLO mode (default: true)")
	fmt.Println("  NEXUS_YOLO_MODE_<BACKEND> YOLO mode for specific backend (default: true)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  promptops deepseek        # Switch to DeepSeek and launch Claude Code")
	fmt.Println("  promptops gemini          # Switch to Gemini and launch")
	fmt.Println("  promptops openrouter      # Switch to OpenRouter and launch")
	fmt.Println("  promptops status          # Check current configuration")
	fmt.Println("  promptops run             # Launch with current backend")
	fmt.Println("  promptops doctor          # Run health checks")
	fmt.Println("  promptops usage           # Check API usage from all providers")
	fmt.Println("  promptops usage claude    # Check Claude API usage")
	fmt.Println("  promptops session start bugfix-123")
	fmt.Println()
}

// For testing - allows running with mocked input
func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
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
		if s == nil {
			continue
		}
		if s.ID == sessionID {
			return s
		}
	}
	return nil
}

func setCurrentSession(cfg *Config, sessionID string) error {
	return writeFileAtomic(cfg.SessionFile, []byte(sessionID), 0600)
}

// withFileLock executes the given function with an exclusive file lock
func withFileLock(lockPath string, fn func() error) error {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer os.Remove(lockPath)
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}

func loadSessions(cfg *Config) []*Session {
	lockPath := cfg.SessionsFile + ".lock"

	var sessions []*Session
	err := withFileLock(lockPath, func() error {
		data, err := os.ReadFile(cfg.SessionsFile)
		if err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: failed to read sessions file: %v\n", err)
			}
			return nil
		}

		if err := json.Unmarshal(data, &sessions); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: sessions file corrupted: %v\n", err)

			// Backup corrupted file
			backupPath := cfg.SessionsFile + ".corrupted." + time.Now().Format("20060102-150405")
			if err := os.WriteFile(backupPath, data, 0600); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to backup corrupted sessions: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "Info: backed up corrupted sessions to %s\n", backupPath)
			}

			return nil
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to lock sessions file: %v\n", err)
		return []*Session{}
	}

	if sessions == nil {
		return []*Session{}
	}
	return sessions
}

func saveSessions(cfg *Config, sessions []*Session) error {
	lockPath := cfg.SessionsFile + ".lock"

	return withFileLock(lockPath, func() error {
		data, err := json.MarshalIndent(sessions, "", "  ")
		if err != nil {
			return err
		}
		return writeFileAtomic(cfg.SessionsFile, data, 0600)
	})
}

// generateSessionID creates a unique session ID with random component
// generateSessionID creates a unique session ID with random component
// Fails hard if crypto/rand fails, as this is a security-critical operation
func generateSessionID(name string) (string, error) {
	b := make([]byte, 16) // Increased to 16 bytes for more entropy
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate secure random session ID: %w", err)
	}
	return fmt.Sprintf("%s-%d-%s", name, time.Now().Unix(), hex.EncodeToString(b)), nil
}

func createSession(cfg *Config, name string) (*Session, error) {
	sessions := loadSessions(cfg)

	// Generate unique ID with random component to prevent collisions
	sessionID, err := generateSessionID(name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

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

	sessions = append(sessions, &session)
	if err := saveSessions(cfg, sessions); err != nil {
		return nil, fmt.Errorf("failed to save sessions: %w", err)
	}
	if err := setCurrentSession(cfg, sessionID); err != nil {
		return nil, fmt.Errorf("failed to set current session: %w", err)
	}

	return &session, nil
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
	data, err := json.Marshal(record)
	if err != nil {
		// Log to stderr but don't fail - usage tracking is best-effort
		fmt.Fprintf(os.Stderr, "Warning: failed to marshal usage record: %v\n", err)
		return
	}
	f, err := os.OpenFile(cfg.UsageFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to open usage file: %v\n", err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close usage file: %v\n", err)
		}
	}()
	if _, err := fmt.Fprintln(f, string(data)); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write usage record: %v\n", err)
	}
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
	// Week starts on Sunday (Weekday() returns 0 for Sunday)
	// Note: This is US-centric; some regions start week on Monday
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
	if maxLen <= 3 {
		return "..."
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
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
		sessionID = truncate(sessionID, 18)
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
	if err := writeFileAtomic(envFile, []byte(newContent), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to update configuration\n")
		auditLog(cfg, fmt.Sprintf("CONFIG_WRITE_ERROR: %v", err))
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
	for _, name := range []string{"claude", "openai", "deepseek", "gemini", "mistral", "zai", "kimi", "groq", "together", "openrouter", "ollama"} {
		be, ok := backends[name]
		if !ok {
			continue // Skip unknown backends (defensive)
		}
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
		fmt.Printf("[OK] %s is healthy (latency: %s)\n", be.DisplayName, formatDuration(result.Latency))
	case "skip":
		fmt.Printf("[--] %s - %s\n", be.DisplayName, result.Message)
	case "error":
		fmt.Printf("[FAIL] %s - %s\n", be.DisplayName, result.Message)
		os.Exit(1)
	}
}

func checkBackendHealth(cfg *Config, be Backend) HealthResult {
	apiKey := cfg.Keys[be.AuthVar]
	if apiKey == "" && be.Name != "ollama" {
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
	case "kimi":
		// Kimi API - try the BaseURL first
		if be.BaseURL != "" {
			url = be.BaseURL + "/v1/models"
			req, err = http.NewRequest("GET", url, nil)
			if err == nil {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			}
		} else {
			return HealthResult{Backend: be.Name, Status: "skip", Message: "No BaseURL configured"}
		}
	case "ollama":
		// Ollama is local, no auth required
		if be.BaseURL != "" {
			url = be.BaseURL + "/models"
			req, err = http.NewRequest("GET", url, nil)
			if err == nil && apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			}
		} else {
			return HealthResult{Backend: be.Name, Status: "skip", Message: "No BaseURL configured"}
		}
	default:
		// For other backends, just check if we can resolve the base URL
		if be.BaseURL != "" {
			url = be.BaseURL + "/models"
			req, err = http.NewRequest("GET", url, nil)
			if err != nil {
				return HealthResult{Backend: be.Name, Status: "error", Message: err.Error()}
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
		} else {
			return HealthResult{Backend: be.Name, Status: "skip", Message: "Health check not implemented"}
		}
	}

	if err != nil || req == nil {
		return HealthResult{Backend: be.Name, Status: "error", Message: err.Error()}
	}

	client := httpClient
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return HealthResult{Backend: be.Name, Status: "error", Latency: latency, Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return HealthResult{Backend: be.Name, Status: "ok", Latency: latency, Message: "Connection verified"}
	}

	// Read body for error details but sanitize to prevent API key exposure
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	errMsg := sanitizeError(fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))).Error()
	return HealthResult{Backend: be.Name, Status: "error", Latency: latency, Message: truncate(errMsg, 100)}
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

	session, err := createSession(cfg, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	be, ok := backends[session.Backend]
	if !ok {
		be = Backend{DisplayName: session.Backend}
	}
	fmt.Printf("[OK] Started session '%s' with %s backend\n", session.Name, be.DisplayName)
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

		// Safe backend name lookup
		backendName := s.Backend
		if be, ok := backends[s.Backend]; ok {
			backendName = be.DisplayName
		}

		rows = append(rows, []string{
			marker,
			truncate(s.Name, 14),
			backendName,
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

			// Safe backend name lookup
			backendName := s.Backend
			if be, ok := backends[s.Backend]; ok {
				backendName = be.DisplayName
			}
			fmt.Printf("[OK] Resumed session '%s' (%s backend)\n", s.Name, backendName)
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
		for _, s := range sessions {
			if s.Name == name {
				session = s
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
	backendName := "Unknown"
	if be, ok := backends[session.Backend]; ok {
		backendName = be.DisplayName
	}
	fmt.Printf("%s %s\n", infoStyle.Render("Backend:"), valueStyle.Render(backendName))

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
				if err := os.Remove(cfg.SessionFile); err != nil && !os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "Warning: failed to remove session file: %v\n", err)
				}
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
	cutoff := time.Now().AddDate(0, 0, -sessionCleanupDays)
	var kept []*Session
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

// ============================================================================
// API USAGE COMMAND - Fetch real usage from provider APIs
// ============================================================================

// UsageInfo represents usage data from a provider
type UsageInfo struct {
	Backend      string
	TotalTokens  int64
	InputTokens  int64
	OutputTokens int64
	TotalCost    float64
	RequestCount int64
	Period       string
	Error        string
}

func showAPIUsage(args []string) {
	cfg := loadConfig()

	// If specific backend requested
	if len(args) > 0 {
		backend := args[0]
		be, ok := backends[backend]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: Unknown backend '%s'\n", backend)
			os.Exit(1)
		}

		apiKey := cfg.Keys[be.AuthVar]
		if apiKey == "" && be.Name != "ollama" {
			fmt.Fprintf(os.Stderr, "Error: No API key configured for %s\n", be.DisplayName)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Printf("Fetching usage for %s...\n", be.DisplayName)
		usage := fetchUsageForBackend(be, apiKey)
		displayUsage(usage)
		return
	}

	// Show usage for all configured backends
	fmt.Println()
	title := styleTitle.Render("API USAGE DASHBOARD")
	fmt.Println(lipgloss.PlaceHorizontal(80, lipgloss.Center, title))
	fmt.Println()

	var usages []UsageInfo
	for _, name := range []string{"claude", "openai", "zai", "kimi", "deepseek", "gemini", "mistral", "groq", "together", "openrouter"} {
		be, ok := backends[name]
		if !ok {
			continue
		}

		apiKey := cfg.Keys[be.AuthVar]
		if apiKey == "" {
			continue // Skip backends without keys
		}

		usage := fetchUsageForBackend(be, apiKey)
		usages = append(usages, usage)
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
		be := backends[u.Backend]
		status := formatCurrency(u.TotalCost)
		if u.Error != "" {
			status = styleMuted.Render(u.Error)
		}

		rows = append(rows, []string{
			be.DisplayName,
			formatNumber(u.TotalTokens),
			formatNumber(u.InputTokens),
			formatNumber(u.OutputTokens),
			formatNumberInt(u.RequestCount),
			status,
		})

		totalCost += u.TotalCost
		totalTokens += u.TotalTokens
	}

	t := table.New().
		Headers("Backend", "Total Tokens", "Input", "Output", "Requests", "Cost").
		Rows(rows...).
		BorderStyle(lipgloss.NewStyle().Foreground(colorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Width(90)

	fmt.Println(t.Render())
	fmt.Println()
	fmt.Printf("Total across all backends: %s  %s tokens\n",
		styleAccent.Render(formatCurrency(totalCost)),
		formatNumber(totalTokens))
	fmt.Println()

	// Show detailed breakdown for each backend
	for _, u := range usages {
		if u.Error != "" {
			displayUsageError(u)
		} else if u.TotalTokens > 0 {
			displayUsageDetail(u)
		}
	}
}

func fetchUsageForBackend(be Backend, apiKey string) UsageInfo {
	usage := UsageInfo{Backend: be.Name, Period: "current period"}

	switch be.Name {
	case "claude":
		return fetchAnthropicUsage(apiKey)
	case "openai":
		return fetchOpenAIUsage(apiKey)
	case "kimi":
		return fetchKimiUsage(apiKey)
	default:
		// For other backends, try generic OpenAI-compatible endpoint or return N/A
		if be.BaseURL != "" {
			return fetchOpenAICompatibleUsage(be, apiKey)
		}
		usage.Error = "Usage API not implemented for this provider"
	}

	return usage
}

func fetchAnthropicUsage(apiKey string) UsageInfo {
	usage := UsageInfo{Backend: "claude", Period: "last 24 hours"}

	// Anthropic doesn't have a public usage API endpoint
	// Return N/A instead of error for cleaner display
	usage.Error = "N/A (see console)"
	return usage
}

func fetchOpenAIUsage(apiKey string) UsageInfo {
	usage := UsageInfo{Backend: "openai", Period: "current billing period"}

	// OpenAI's usage API requires admin access and a specific 'date' parameter
	// Most users don't have access to this endpoint
	// Return N/A instead of error for cleaner display
	usage.Error = "N/A (see dashboard)"
	return usage
}

func fetchKimiUsage(apiKey string) UsageInfo {
	usage := UsageInfo{Backend: "kimi", Period: "current billing period"}

	// Kimi API usage endpoint
	ctx, cancel := context.WithTimeout(context.Background(), httpClientTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.kimi.com/coding/usage", nil)
	if err != nil {
		usage.Error = "N/A"
		return usage
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: httpClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		usage.Error = "N/A"
		return usage
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		// Kimi may not expose this endpoint
		usage.Error = "N/A (see console)"
		return usage
	}

	if resp.StatusCode != http.StatusOK {
		usage.Error = "N/A"
		return usage
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

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&result); err != nil {
		usage.Error = "N/A"
		return usage
	}

	usage.TotalTokens = result.Data.TotalTokens
	usage.InputTokens = result.Data.InputTokens
	usage.OutputTokens = result.Data.OutputTokens
	usage.RequestCount = result.Data.TotalRequests
	usage.TotalCost = result.Data.TotalCost

	return usage
}

func fetchOpenAICompatibleUsage(be Backend, apiKey string) UsageInfo {
	usage := UsageInfo{Backend: be.Name, Period: "current period"}

	// Generic handler for OpenAI-compatible APIs
	url := be.BaseURL + "/usage"
	ctx, cancel := context.WithTimeout(context.Background(), httpClientTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		usage.Error = err.Error()
		return usage
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: httpClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		usage.Error = err.Error()
		return usage
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		usage.Error = fmt.Sprintf("Usage API not available (HTTP %d)", resp.StatusCode)
		return usage
	}

	// Generic parsing - try common field names
	var result map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&result); err != nil {
		usage.Error = err.Error()
		return usage
	}

	// Try to extract common fields
	if data, ok := result["data"].(map[string]interface{}); ok {
		if v, ok := data["total_tokens"].(float64); ok {
			usage.TotalTokens = int64(v)
		}
		if v, ok := data["input_tokens"].(float64); ok {
			usage.InputTokens = int64(v)
		}
		if v, ok := data["output_tokens"].(float64); ok {
			usage.OutputTokens = int64(v)
		}
		if v, ok := data["total_cost"].(float64); ok {
			usage.TotalCost = v
		}
	}

	return usage
}

func displayUsage(u UsageInfo) {
	be := backends[u.Backend]
	fmt.Println()
	fmt.Println(styleSection.Render(fmt.Sprintf("USAGE: %s", be.DisplayName)))

	if u.Error != "" {
		fmt.Println(styleWarning.Render(u.Error))
		return
	}

	fmt.Printf("  Period:      %s\n", u.Period)
	fmt.Printf("  Total Tokens: %s\n", formatNumber(u.TotalTokens))
	fmt.Printf("  Input Tokens: %s\n", formatNumber(u.InputTokens))
	fmt.Printf("  Output Tokens: %s\n", formatNumber(u.OutputTokens))
	fmt.Printf("  Requests:    %s\n", formatNumberInt(u.RequestCount))
	fmt.Printf("  Total Cost:  %s\n", styleAccent.Render(formatCurrency(u.TotalCost)))
	fmt.Println()
}

func displayUsageDetail(u UsageInfo) {
	if u.Error != "" {
		return
	}

	be := backends[u.Backend]

	fmt.Printf("%s: ", be.DisplayName)
	fmt.Printf("%s tokens, ", formatNumber(u.TotalTokens))
	fmt.Printf("%s requests, ", formatNumberInt(u.RequestCount))
	fmt.Printf("cost: %s\n", styleAccent.Render(formatCurrency(u.TotalCost)))

}

func displayUsageError(u UsageInfo) {
	if u.Error == "" || u.Error == "N/A" || u.Error == "N/A (see console)" || u.Error == "N/A (see dashboard)" {
		return
	}

	be := backends[u.Backend]
	fmt.Printf("%s: %s\n", be.DisplayName, styleWarning.Render(u.Error))
}

func formatNumber(n int64) string {
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

func formatNumberInt(n int64) string {
	if n == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", n)
}
