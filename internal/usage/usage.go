// Package usage handles cost tracking and budget management.
package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"nexus/internal/backend"
	"nexus/internal/config"
)

// Record represents a single API usage entry.
type Record struct {
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"session_id"`
	Backend      string    `json:"backend"`
	Model        string    `json:"model"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
}

// Info represents usage data from a provider.
type Info struct {
	Backend      string
	TotalTokens  int64
	InputTokens  int64
	OutputTokens int64
	TotalCost    float64
	RequestCount int64
	Period       string
	Error        string
}

// Tracker handles usage tracking operations.
type Tracker struct {
	cfg        *config.Config
	registry   *backend.Registry
	getSession func() string // returns current session ID
}

// NewTracker creates a new usage tracker.
func NewTracker(cfg *config.Config, registry *backend.Registry, getSession func() string) *Tracker {
	return &Tracker{
		cfg:        cfg,
		registry:   registry,
		getSession: getSession,
	}
}

// Log records usage for a backend.
func (t *Tracker) Log(backendName string, inputTokens, outputTokens int64) error {
	be, ok := t.registry.Get(backendName)
	if !ok {
		return fmt.Errorf("unknown backend: %s", backendName)
	}

	// Calculate cost
	inputCost := float64(inputTokens) * be.InputPrice / 1000000
	outputCost := float64(outputTokens) * be.OutputPrice / 1000000
	totalCost := inputCost + outputCost

	record := Record{
		Timestamp:    time.Now(),
		SessionID:    t.getSession(),
		Backend:      backendName,
		Model:        be.SonnetModel,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      totalCost,
	}

	// Append to usage file
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal usage record: %w", err)
	}

	f, err := os.OpenFile(t.cfg.UsageFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open usage file: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintln(f, string(data)); err != nil {
		return fmt.Errorf("write usage record: %w", err)
	}

	return nil
}

// LoadAll loads all usage records.
func (t *Tracker) LoadAll() []Record {
	data, err := os.ReadFile(t.cfg.UsageFile)
	if err != nil {
		return []Record{}
	}

	var records []Record
	lines := splitLines(string(data))
	for _, line := range lines {
		line = trimSpace(line)
		if line == "" {
			continue
		}
		var record Record
		if err := json.Unmarshal([]byte(line), &record); err == nil {
			records = append(records, record)
		}
	}
	return records
}

// Costs holds calculated costs for different time periods.
type Costs struct {
	Daily     float64
	Weekly    float64
	Monthly   float64
	ByBackend map[string]float64
}

// CalculateCosts calculates costs for different time periods.
func (t *Tracker) CalculateCosts() Costs {
	records := t.LoadAll()
	byBackend := make(map[string]float64)

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	monthStart := today.AddDate(0, 0, -today.Day()+1)

	var daily, weekly, monthly float64

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

	return Costs{
		Daily:     daily,
		Weekly:    weekly,
		Monthly:   monthly,
		ByBackend: byBackend,
	}
}

// AuditLogger handles audit logging.
type AuditLogger struct {
	cfg        *config.Config
	getSession func() string
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(cfg *config.Config, getSession func() string) *AuditLogger {
	return &AuditLogger{
		cfg:        cfg,
		getSession: getSession,
	}
}

// Log writes an audit log entry.
func (a *AuditLogger) Log(msg string) error {
	if !a.cfg.AuditEnabled {
		return nil
	}

	f, err := os.OpenFile(a.cfg.AuditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()

	// Include session name if available
	sessionName := a.getSession()
	if sessionName != "" {
		msg = fmt.Sprintf("[%s] %s", sessionName, msg)
	}

	if _, err := fmt.Fprintf(f, "[%s] %s\n", time.Now().Format(time.RFC3339), msg); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	return nil
}

// Helper functions
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

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
