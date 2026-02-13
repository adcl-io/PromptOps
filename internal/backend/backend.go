// Package backend defines backend types, registry, and provider configurations.
package backend

import (
	"fmt"
	"net/http"
	"time"

	"nexus/internal/config"
)

// DefaultTimeout is the default timeout for API calls (50 minutes).
const DefaultTimeout = 50 * time.Minute

// HealthCheckTimeout is the timeout for health check HTTP requests.
const HealthCheckTimeout = 5 * time.Second

// Backend represents an AI provider configuration.
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

// HealthResult represents the result of a backend health check.
type HealthResult struct {
	Backend string
	Status  string // ok, skip, error
	Latency time.Duration
	Message string
}

// Registry holds all available backends.
type Registry struct {
	backends map[string]Backend
	client   *http.Client
}

// NewRegistry creates a new backend registry with all supported providers.
func NewRegistry() *Registry {
	return &Registry{
		backends: map[string]Backend{
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
				Timeout:     DefaultTimeout,
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
				Timeout:     DefaultTimeout,
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
				Timeout:     DefaultTimeout,
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
				Timeout:     DefaultTimeout,
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
				Timeout:     DefaultTimeout,
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
				Timeout:     DefaultTimeout,
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
				Timeout:     DefaultTimeout,
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
				Timeout:     DefaultTimeout,
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
				Timeout:     DefaultTimeout,
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
				Timeout:     DefaultTimeout,
				HaikuModel:  "llama3.2",
				SonnetModel: "codellama",
				OpusModel:   "llama3.3",
				InputPrice:  0.00,
				OutputPrice: 0.00,
				CodingTier:  "B",
			},
		},
		client: &http.Client{
			Timeout: HealthCheckTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     30 * time.Second,
			},
		},
	}
}

// Get returns a backend by name.
func (r *Registry) Get(name string) (Backend, bool) {
	be, ok := r.backends[name]
	return be, ok
}

// GetAll returns all backends.
func (r *Registry) GetAll() map[string]Backend {
	return r.backends
}

// GetOrdered returns backends in a specific order.
func (r *Registry) GetOrdered() []string {
	return []string{
		"claude", "openai", "deepseek", "gemini", "mistral",
		"zai", "kimi", "groq", "together", "openrouter", "ollama",
	}
}

// CheckHealth performs a health check on a backend.
func (r *Registry) CheckHealth(cfg *config.Config, be Backend) HealthResult {
	apiKey := cfg.Keys[be.AuthVar]
	if apiKey == "" && be.Name != "ollama" {
		return HealthResult{Backend: be.Name, Status: "skip", Message: "No API key configured"}
	}

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

	resp, err := r.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return HealthResult{Backend: be.Name, Status: "error", Latency: latency, Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return HealthResult{Backend: be.Name, Status: "ok", Latency: latency, Message: "Connection verified"}
	}

	return HealthResult{
		Backend: be.Name,
		Status:  "error",
		Latency: latency,
		Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
	}
}

// StateManager handles reading and writing the current backend state.
type StateManager struct {
	cfg *config.Config
}

// NewStateManager creates a new state manager.
func NewStateManager(cfg *config.Config) *StateManager {
	return &StateManager{cfg: cfg}
}

// GetCurrent returns the current backend name from state file.
func (s *StateManager) GetCurrent() string {
	data, err := config.ReadFile(s.cfg.StateFile)
	if err != nil {
		return ""
	}
	return string(data)
}

// SetCurrent sets the current backend name in state file.
func (s *StateManager) SetCurrent(backend string) error {
	return config.WriteFileAtomic(s.cfg.StateFile, []byte(backend), 0600)
}

// ReadFile is a helper to read file contents.
func ReadFile(path string) ([]byte, error) {
	return config.ReadFile(path)
}
