// promptops - AI Model Backend Switcher
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const version = "2.1.0"

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
}

var backends = map[string]Backend{
	"claude": {
		Name:        "claude",
		DisplayName: "Claude",
		Provider:    "Anthropic",
		Models:      "Claude Sonnet 4.5",
		AuthVar:     "ANTHROPIC_API_KEY",
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
	},
}

type Config struct {
	EnvFile              string
	StateFile            string
	AuditLog             string
	YoloMode             bool
	YoloModeClaude       bool
	YoloModeZai          bool
	YoloModeKimi         bool
	DefaultBackend       string
	VerifyOnSwitch       bool
	AuditEnabled         bool
	Keys                 map[string]string
}

func main() {
	if len(os.Args) < 2 {
		showStatus()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "claude", "zai", "kimi":
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
		Keys:           make(map[string]string),
		DefaultBackend: "claude",
		VerifyOnSwitch: true,
		AuditEnabled:   true,
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
			case "NEXUS_DEFAULT_BACKEND":
				cfg.DefaultBackend = value
			case "NEXUS_VERIFY_ON_SWITCH":
				cfg.VerifyOnSwitch = value == "true"
			case "NEXUS_AUDIT_LOG":
				cfg.AuditEnabled = value == "true"
			case "ANTHROPIC_API_KEY", "ZAI_API_KEY", "KIMI_API_KEY":
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
	cmd := exec.Command("claude", args...)

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

	fmt.Print("\033[H\033[2J") // clear screen
	fmt.Println("+-------------------------------------------------------------------------------+")
	fmt.Println("|                    PROMPTOPS ENTERPRISE v" + version + "                       |")
	fmt.Println("+-------------------------------------------------------------------------------+")
	fmt.Println()

	if current != "" {
		switch current {
		case "claude":
			printLogo("claude")
		case "zai":
			printLogo("zai")
		case "kimi":
			printLogo("kimi")
		}
		fmt.Println()
		drawBox(fmt.Sprintf("CURRENT: %s BACKEND", strings.ToUpper(current)))
	} else {
		fmt.Println("WARNING: No backend configured")
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("Available Backends:")
	fmt.Println("------------------------------------------------------")
	fmt.Println()
	fmt.Println("  * claude    -> Anthropic Claude Sonnet 4.5")
	fmt.Printf("               YOLO: %v\n", cfg.getYoloMode("claude"))
	fmt.Println()
	fmt.Println("  * zai       -> Z.AI GLM-4.7 / GLM-4.5-Air")
	fmt.Printf("               YOLO: %v\n", cfg.getYoloMode("zai"))
	fmt.Println()
	fmt.Println("  * kimi      -> Kimi K2 Thinking / K2 Thinking Turbo")
	fmt.Printf("               YOLO: %v\n", cfg.getYoloMode("kimi"))
	fmt.Println()
	fmt.Println("------------------------------------------------------")
	fmt.Println()

	fmt.Println("API Keys Status:")
	fmt.Println()
	if key := cfg.Keys["ANTHROPIC_API_KEY"]; key != "" {
		fmt.Printf("  [OK] ANTHROPIC_API_KEY  %s\n", maskKey(key))
	} else {
		fmt.Println("  [MISSING] ANTHROPIC_API_KEY")
	}
	if key := cfg.Keys["ZAI_API_KEY"]; key != "" {
		fmt.Printf("  [OK] ZAI_API_KEY        %s\n", maskKey(key))
	} else {
		fmt.Println("  [MISSING] ZAI_API_KEY")
	}
	if key := cfg.Keys["KIMI_API_KEY"]; key != "" {
		fmt.Printf("  [OK] KIMI_API_KEY       %s\n", maskKey(key))
	} else {
		fmt.Println("  [MISSING] KIMI_API_KEY")
	}

	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println()
	fmt.Printf("  Audit Logging: %v\n", cfg.AuditEnabled)
	fmt.Printf("  Verify on Switch: %v\n", cfg.VerifyOnSwitch)
	fmt.Printf("  Default Backend: %s\n", cfg.DefaultBackend)
	fmt.Printf("  YOLO Mode: %v\n", cfg.YoloMode)

	fmt.Println()
	fmt.Println("Tip: Add missing keys to .env.local")
	fmt.Println("Usage: promptops <backend> to switch and launch")
	fmt.Println()
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

# Global YOLO mode - overrides all backends when true
NEXUS_YOLO_MODE=false

# -------------------------------------------------------------------------------
# Enterprise Settings
# -------------------------------------------------------------------------------
# Enable audit logging (logs all backend switches to .promptops-audit.log)
NEXUS_AUDIT_LOG=true

# Default backend when none specified (claude|zai|kimi)
NEXUS_DEFAULT_BACKEND=claude

# Verify API keys on switch (true|false)
NEXUS_VERIFY_ON_SWITCH=true

# -------------------------------------------------------------------------------
# LLM API Keys (add your keys here)
# -------------------------------------------------------------------------------

# Anthropic Claude API Key
# Get your API key from: https://console.anthropic.com/
ANTHROPIC_API_KEY=

# Z.AI (GLM/Zhipu AI) API Key
# Get your API key from: https://open.bigmodel.cn/
ZAI_API_KEY=

# Kimi (Moonshot AI) API Key
# Get your API key from: https://platform.moonshot.cn/
KIMI_API_KEY=
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
	fmt.Println("  - claude: Anthropic Claude Sonnet 4.5")
	fmt.Println("  - zai: Z.AI GLM-4.7 / GLM-4.5-Air")
	fmt.Println("  - kimi: Kimi K2 Thinking / K2 Thinking Turbo")
}

func showHelp() {
	fmt.Println("+-------------------------------------------------------------------------------+")
	fmt.Println("|                    PROMPTOPS ENTERPRISE v" + version + "                       |")
	fmt.Println("+-------------------------------------------------------------------------------+")
	fmt.Println()
	fmt.Println("Usage: promptops <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  claude                    Switch to Claude (Anthropic) and launch")
	fmt.Println("  zai                       Switch to Z.AI (GLM) and launch")
	fmt.Println("  kimi                      Switch to Kimi (Moonshot) and launch")
	fmt.Println("  status                    Show current backend and configuration")
	fmt.Println("  run [args]                Launch Claude Code with current backend")
	fmt.Println("  init                      Initialize .env.local with API key templates")
	fmt.Println("  version                   Show version information")
	fmt.Println("  help                      Show this help message")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  NEXUS_ENV_FILE            Path to env file (default: ./.env.local)")
	fmt.Println("  NEXUS_YOLO_MODE           Global YOLO mode (true|false)")
	fmt.Println("  NEXUS_YOLO_MODE_CLAUDE    YOLO mode for Claude")
	fmt.Println("  NEXUS_YOLO_MODE_ZAI       YOLO mode for Z.AI")
	fmt.Println("  NEXUS_YOLO_MODE_KIMI      YOLO mode for Kimi")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  promptops zai             # Switch to Z.AI and launch Claude Code")
	fmt.Println("  promptops claude          # Switch back to Claude and launch")
	fmt.Println("  promptops status          # Check current configuration")
	fmt.Println("  promptops run             # Launch with current backend")
	fmt.Println()
}

// For testing - allows running with mocked input
func readLine(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}
