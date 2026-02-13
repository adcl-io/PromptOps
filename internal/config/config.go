// Package config handles configuration loading, validation, and management.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all application configuration.
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

// Loader handles configuration loading with dependency injection for testability.
type Loader struct {
	getenv    func(string) string
	getwd     func() (string, error)
	getexe    func() (string, error)
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
}

// NewLoader creates a new ConfigLoader with real OS functions.
func NewLoader() *Loader {
	return &Loader{
		getenv:    os.Getenv,
		getwd:     os.Getwd,
		getexe:    os.Executable,
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
	}
}

// Load loads configuration from environment and files.
func (l *Loader) Load() (*Config, error) {
	dir, err := l.getScriptDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine script directory: %w", err)
	}

	envFile := l.getenv("NEXUS_ENV_FILE")
	if envFile != "" {
		// Validate to prevent path traversal
		cleanPath := filepath.Clean(envFile)
		if strings.Contains(cleanPath, "..") {
			return nil, fmt.Errorf("NEXUS_ENV_FILE contains path traversal: %s", envFile)
		}
		envFile = cleanPath
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
	data, err := l.readFile(envFile)
	if err == nil {
		l.parseEnvFile(cfg, string(data))
	}

	return cfg, nil
}

func (l *Loader) getScriptDir() (string, error) {
	ex, err := l.getexe()
	if err != nil {
		// Fallback to working directory if executable path unavailable
		wd, wdErr := l.getwd()
		if wdErr != nil {
			return "", fmt.Errorf("executable error: %w; wd error: %w", err, wdErr)
		}
		return wd, nil
	}
	return filepath.Dir(ex), nil
}

func (l *Loader) parseEnvFile(cfg *Config, content string) {
	lines := strings.Split(content, "\n")
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
			}
		case "NEXUS_WEEKLY_BUDGET":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				cfg.WeeklyBudget = v
			}
		case "NEXUS_MONTHLY_BUDGET":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				cfg.MonthlyBudget = v
			}
		case "ANTHROPIC_API_KEY", "ZAI_API_KEY", "KIMI_API_KEY", "DEEPSEEK_API_KEY",
			"GEMINI_API_KEY", "MISTRAL_API_KEY", "GROQ_API_KEY", "TOGETHER_API_KEY",
			"OPENROUTER_API_KEY", "OPENAI_API_KEY", "OLLAMA_API_KEY":
			cfg.Keys[key] = value
		case "OLLAMA_HAIKU_MODEL":
			cfg.OllamaModels["haiku"] = value
		case "OLLAMA_SONNET_MODEL":
			cfg.OllamaModels["sonnet"] = value
		case "OLLAMA_OPUS_MODEL":
			cfg.OllamaModels["opus"] = value
		case "ZAI_HAIKU_MODEL":
			cfg.ZAIModels["haiku"] = value
		case "ZAI_SONNET_MODEL":
			cfg.ZAIModels["sonnet"] = value
		case "ZAI_OPUS_MODEL":
			cfg.ZAIModels["opus"] = value
		case "KIMI_HAIKU_MODEL":
			cfg.KimiModels["haiku"] = value
		case "KIMI_SONNET_MODEL":
			cfg.KimiModels["sonnet"] = value
		case "KIMI_OPUS_MODEL":
			cfg.KimiModels["opus"] = value
		}
	}
}

// GetYoloMode returns the YOLO mode setting for a given backend.
func (cfg *Config) GetYoloMode(backend string) bool {
	if cfg.YoloMode {
		return true
	}
	if cfg.YoloModes != nil {
		if v, ok := cfg.YoloModes[backend]; ok {
			return v
		}
	}
	return true
}

// WriteFileAtomic writes data to a file atomically using temp file + rename.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
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

	if err = os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err = os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
