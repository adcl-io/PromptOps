// Package commands implements CLI command handlers.
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"nexus/internal/backend"
	"nexus/internal/config"
	"nexus/internal/proxy"
	"nexus/internal/session"
	"nexus/internal/ui"
	"nexus/internal/usage"
)

// HandlerConfig contains dependencies for the command handler.
type HandlerConfig struct {
	Version      string
	Config       *config.Config
	Registry     *backend.Registry
	StateReader  *backend.CurrentReader
	StateWriter  *backend.CurrentWriter
	SessionMgr   *session.Manager
	GetSessionID func() string
}

// Handler handles CLI commands.
type Handler struct {
	version      string
	cfg          *config.Config
	registry     *backend.Registry
	stateReader  *backend.CurrentReader
	stateWriter  *backend.CurrentWriter
	sessionMgr   *session.Manager
	getSessionID func() string
	auditLogger  *usage.AuditLogger
	tracker      *usage.Tracker
	ui           *ui.Renderer
}

// NewHandler creates a new command handler.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		version:      cfg.Version,
		cfg:          cfg.Config,
		registry:     cfg.Registry,
		stateReader:  cfg.StateReader,
		stateWriter:  cfg.StateWriter,
		sessionMgr:   cfg.SessionMgr,
		getSessionID: cfg.GetSessionID,
		ui:           ui.NewRenderer(),
	}
}

// SetAuditLogger sets the audit logger.
func (h *Handler) SetAuditLogger(logger *usage.AuditLogger) {
	h.auditLogger = logger
}

// SetUsageTracker sets the usage tracker.
func (h *Handler) SetUsageTracker(tracker *usage.Tracker) {
	h.tracker = tracker
}

// GetSessionID returns the current session ID for callbacks.
func (h *Handler) GetSessionID() string {
	return h.getSessionID()
}

// SwitchBackend switches to the specified backend and launches Claude Code.
func (h *Handler) SwitchBackend(name string, args []string) {
	be, ok := h.registry.Get(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: Unknown backend '%s'\n", name)
		os.Exit(1)
	}

	// Check for API key (not required for local backends like Ollama)
	apiKey := h.cfg.Keys[be.AuthVar]
	if apiKey == "" && be.Name != "ollama" {
		fmt.Fprintf(os.Stderr, "Error: %s not set in .env.local\n", be.AuthVar)
		os.Exit(1)
	}

	yolo := h.cfg.GetYoloMode(name)

	// Animations
	if !yolo {
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
			h.animateSwitch(msg)
		}
		fmt.Println()
		ui.PrintLogo(name)
		fmt.Println()

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
			h.showProgress(msg)
		}
	}

	// Save state
	if err := h.stateWriter.Set(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving state: %v\n", err)
		os.Exit(1)
	}

	// Audit log
	if h.auditLogger != nil {
		h.auditLogger.Log(fmt.Sprintf("SWITCH: %s", name))
	}

	if !yolo {
		fmt.Println()
		ui.DrawBox(fmt.Sprintf("%s BACKEND ACTIVE", strings.ToUpper(be.DisplayName)))
		fmt.Printf("  Provider: %s\n", be.Provider)
		fmt.Printf("  Models:   %s\n", be.Models)
		if be.BaseURL != "" {
			fmt.Printf("  Base URL: %s\n", be.BaseURL)
		}
		fmt.Printf("  API Key:  %s\n", ui.MaskKey(apiKey))
		fmt.Println("  Status:   [ONLINE]")
		fmt.Println()
		fmt.Println("-------------------------------------------------------")
		fmt.Println("[OK] Backend configured - launching Claude Code...")
		fmt.Println("-------------------------------------------------------")
		fmt.Println()
	}

	// Launch claude with proper env
	h.launchClaudeWithBackend(be, args)
}

func (h *Handler) launchClaudeWithBackend(be backend.Backend, args []string) {
	cmdArgs := []string{}

	yolo := h.cfg.GetYoloMode(be.Name)
	if yolo {
		cmdArgs = append(cmdArgs, "--dangerously-skip-permissions")
	}

	// Sanitize user-provided arguments
	sanitizedArgs := ui.SanitizeArgs(args)
	cmdArgs = append(cmdArgs, sanitizedArgs...)

	cmd := exec.Command("claude", cmdArgs...)

	// Build environment with whitelist approach
	env := ui.FilterEnvironment(os.Environ(), ui.GetAllowedEnvVars())

	// Set auth token for Claude Code
	apiKey := h.cfg.Keys[be.AuthVar]
	if apiKey != "" {
		env = append(env, fmt.Sprintf("ANTHROPIC_AUTH_TOKEN=%s", apiKey))
	} else if be.Name == "ollama" {
		env = append(env, "ANTHROPIC_AUTH_TOKEN=ollama")
	}

	// Set backend-specific vars
	baseURL := be.BaseURL
	if be.BaseURL != "" {
		env = append(env, fmt.Sprintf("API_TIMEOUT_MS=%d", be.Timeout.Milliseconds()))

		haikuModel := be.HaikuModel
		sonnetModel := be.SonnetModel
		opusModel := be.OpusModel

		if be.Name == "ollama" {
			if m, ok := h.cfg.OllamaModels["haiku"]; ok && m != "" {
				haikuModel = strings.TrimSpace(m)
			}
			if m, ok := h.cfg.OllamaModels["sonnet"]; ok && m != "" {
				sonnetModel = strings.TrimSpace(m)
			}
			if m, ok := h.cfg.OllamaModels["opus"]; ok && m != "" {
				opusModel = strings.TrimSpace(m)
			}
		}

		if be.Name == "zai" {
			if m, ok := h.cfg.ZAIModels["haiku"]; ok && m != "" {
				haikuModel = strings.TrimSpace(m)
			}
			if m, ok := h.cfg.ZAIModels["sonnet"]; ok && m != "" {
				sonnetModel = strings.TrimSpace(m)
			}
			if m, ok := h.cfg.ZAIModels["opus"]; ok && m != "" {
				opusModel = strings.TrimSpace(m)
			}
		}

		if be.Name == "kimi" {
			if m, ok := h.cfg.KimiModels["haiku"]; ok && m != "" {
				haikuModel = strings.TrimSpace(m)
			}
			if m, ok := h.cfg.KimiModels["sonnet"]; ok && m != "" {
				sonnetModel = strings.TrimSpace(m)
			}
			if m, ok := h.cfg.KimiModels["opus"]; ok && m != "" {
				opusModel = strings.TrimSpace(m)
			}
		}

		env = append(env, fmt.Sprintf("ANTHROPIC_DEFAULT_HAIKU_MODEL=%s", haikuModel))
		env = append(env, fmt.Sprintf("ANTHROPIC_DEFAULT_SONNET_MODEL=%s", sonnetModel))
		env = append(env, fmt.Sprintf("ANTHROPIC_DEFAULT_OPUS_MODEL=%s", opusModel))
	}

	// For Ollama, start a proxy
	var prx *proxy.OllamaProxy
	if be.Name == "ollama" {
		prx = proxy.NewOllamaProxy(baseURL, proxy.BuildModelMap(h.cfg.OllamaModels))
		if err := prx.Start(18080); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting Ollama proxy: %v\n", err)
			os.Exit(1)
		}
		baseURL = "http://localhost:18080"
		if !yolo {
			fmt.Println("[OK] Started Anthropic-to-OpenAI proxy on port 18080")
		}
	}

	env = append(env, fmt.Sprintf("ANTHROPIC_BASE_URL=%s", baseURL))

	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	if prx != nil {
		prx.Stop()
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error launching claude: %v\n", err)
		os.Exit(1)
	}
}

func (h *Handler) animateSwitch(msg string) {
	frames := []string{"|", "/", "-", "\\"}
	for i := 0; i < 20; i++ {
		fmt.Printf("\r%s %s", frames[i%4], msg)
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("\r[OK] %s\n", msg)
}

func (h *Handler) showProgress(msg string) {
	fmt.Printf("%s [", msg)
	for i := 0; i < 30; i++ {
		fmt.Print(ui.StyleProgressFilled.Render("█"))
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println("] COMPLETE")
}

// RunClaude launches Claude Code with the current backend.
func (h *Handler) RunClaude(args []string) {
	current := h.stateReader.Get()

	if current == "" {
		fmt.Println("WARNING: No backend configured. Defaulting to Claude.")
		h.SwitchBackend("claude", args)
		return
	}

	be, ok := h.registry.Get(current)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: Unknown backend '%s' in state file\n", current)
		os.Exit(1)
	}

	fmt.Printf("INFO: Launching Claude Code with %s backend...\n\n", current)
	h.launchClaudeWithBackend(be, args)
}

// ShowStatus displays the current status.
func (h *Handler) ShowStatus() {
	current := h.stateReader.Get()
	session := h.sessionMgr.GetCurrent()
	costs := h.tracker.CalculateCosts()

	// Check for --check flag
	checkLatency := false
	for _, arg := range os.Args {
		if arg == "--check" || arg == "--latency" {
			checkLatency = true
			break
		}
	}

	fmt.Println()
	title := ui.StyleTitle.Render(fmt.Sprintf("PROMPTOPS v%s", h.version))
	fmt.Println(lipgloss.PlaceHorizontal(80, lipgloss.Center, title))
	fmt.Println()

	// Current Backend Section
	fmt.Println(ui.StyleSection.Render("CURRENT BACKEND"))
	if current != "" {
		be, ok := h.registry.Get(current)
		if !ok {
			fmt.Println(ui.StyleError.Render("Invalid backend in state: " + current))
			current = ""
		} else {
			status := ui.StyleCurrent.Render("> " + be.DisplayName)
			if h.cfg.GetYoloMode(current) {
				status += ui.StyleWarning.Render(" [YOLO]")
			}
			fmt.Println(status)
			fmt.Println(ui.StyleMuted.Render(be.Models))
			if custom := h.formatCustomModels(be.Name); custom != "" {
				fmt.Println(ui.StyleWarning.Render("Custom: " + custom))
			}
		}
	}
	if current == "" {
		fmt.Println(ui.StyleMuted.Render("No backend configured"))
	}

	// Session info
	if session != nil {
		fmt.Println()
		fmt.Println(ui.StyleSection.Render("SESSION"))
		fmt.Printf("%s %s (%s)\n", ui.StyleAccent.Render(">"), session.Name, ui.StyleSuccess.Render(session.Status))
	}

	// Backends Table
	fmt.Println()
	fmt.Println(ui.StyleSection.Render("AVAILABLE BACKENDS"))

	rows := [][]string{}
	for _, name := range h.registry.GetOrdered() {
		be, ok := h.registry.Get(name)
		if !ok {
			continue
		}
		hasKey := h.cfg.Keys[be.AuthVar] != ""

		marker := " "
		if name == current {
			marker = ui.StyleAccent.Render(">")
		}

		status := ui.StyleSuccess.Render("Ready")
		extraCol := "--"

		if !hasKey {
			if be.Name == "ollama" {
				status = ui.StyleSuccess.Render("Local")
			} else {
				status = ui.StyleMuted.Render("No Key")
			}
		} else if checkLatency {
			result := h.registry.CheckHealth(h.cfg, be)
			if result.Status == "ok" {
				extraCol = formatDuration(result.Latency)
			} else if result.Status == "error" {
				status = ui.StyleError.Render("Error")
			}
		}

		if !checkLatency {
			costStr := fmt.Sprintf("$%.2f/$%.2f", be.InputPrice, be.OutputPrice)
			if name == "kimi" || name == "zai" {
				extraCol = ui.StyleMuted.Render("Sub " + costStr)
			} else {
				extraCol = costStr
			}
		}

		tierStr := be.CodingTier
		switch tierStr {
		case "S":
			tierStr = ui.StyleSuccess.Render(tierStr)
		case "A":
			tierStr = lipgloss.NewStyle().Foreground(ui.ColorAccent).Render(tierStr)
		case "B":
			tierStr = lipgloss.NewStyle().Foreground(ui.ColorText).Render(tierStr)
		case "C":
			tierStr = ui.StyleMuted.Render(tierStr)
		}

		rows = append(rows, []string{
			marker,
			be.DisplayName,
			ui.Truncate(be.Models, 22),
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
		BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorSubtle)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary).Padding(0, 1)
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
	fmt.Println(ui.StyleSection.Render("COST SUMMARY"))
	fmt.Printf("This Month: %s / %s\n",
		ui.StyleValue.Render(ui.FormatCurrency(costs.Monthly)),
		ui.StyleValue.Render(ui.FormatCurrency(h.cfg.MonthlyBudget)))
	fmt.Println()

	// Budget progress bars
	fmt.Println(h.ui.RenderProgressBar("Daily  ", costs.Daily, h.cfg.DailyBudget))
	fmt.Println(h.ui.RenderProgressBar("Weekly ", costs.Weekly, h.cfg.WeeklyBudget))
	fmt.Println(h.ui.RenderProgressBar("Monthly", costs.Monthly, h.cfg.MonthlyBudget))

	// Top backends by usage
	if len(costs.ByBackend) > 0 {
		fmt.Println()
		fmt.Println(ui.StyleSection.Render("TOP BACKENDS BY USAGE"))

		type backendCost struct {
			name string
			cost float64
		}
		var bc []backendCost
		total := 0.0
		for name, cost := range costs.ByBackend {
			bc = append(bc, backendCost{name, cost})
			total += cost
		}
		sort.Slice(bc, func(i, j int) bool { return bc[i].cost > bc[j].cost })

		for _, b := range bc {
			percent := b.cost / total * 100
			be, _ := h.registry.Get(b.name)
			fmt.Printf("%-12s %8s  %s\n",
				be.DisplayName,
				ui.FormatCurrency(b.cost),
				h.ui.RenderMiniBar(percent),
			)
		}
	}

	fmt.Println()
}

func (h *Handler) formatCustomModels(backend string) string {
	var models map[string]string
	switch backend {
	case "ollama":
		models = h.cfg.OllamaModels
	case "zai":
		models = h.cfg.ZAIModels
	case "kimi":
		models = h.cfg.KimiModels
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

// ShowVersion displays version information.
func (h *Handler) ShowVersion() {
	fmt.Println("PromptOps Enterprise AI Model Backend Switcher")
	fmt.Printf("Version: %s\n", h.version)
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

// ShowHelp displays help information.
func (h *Handler) ShowHelp() {
	fmt.Println("+-------------------------------------------------------------------------------+")
	fmt.Println("|                    PROMPTOPS ENTERPRISE v" + h.version + "                       |")
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

// InitEnv initializes the .env.local file.
func (h *Handler) InitEnv() {
	dir, err := os.Getwd()
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

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dus", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
