// Package usage_test provides tests for the usage package.
package usage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nexus/internal/backend"
	"nexus/internal/config"
	"nexus/internal/usage"
)

// ============================================================================
// Tracker Tests
// ============================================================================

func TestNewTracker(t *testing.T) {
	cfg := &config.Config{}
	registry := backend.NewRegistry()
	getSession := func() string { return "test-session" }

	tracker := usage.NewTracker(cfg, registry, getSession)
	if tracker == nil {
		t.Fatal("NewTracker() returned nil")
	}
}

func TestTrackerLog(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		UsageFile: filepath.Join(tmpDir, "usage.jsonl"),
	}
	registry := backend.NewRegistry()
	getSession := func() string { return "test-session" }

	tracker := usage.NewTracker(cfg, registry, getSession)

	// Log usage for Claude backend
	err := tracker.Log("claude", 1000, 500)
	if err != nil {
		t.Errorf("Log() failed: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(cfg.UsageFile)
	if err != nil {
		t.Fatalf("Failed to read usage file: %v", err)
	}

	var record usage.Record
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("Failed to unmarshal usage record: %v", err)
	}

	if record.Backend != "claude" {
		t.Errorf("Expected Backend='claude', got %q", record.Backend)
	}

	if record.InputTokens != 1000 {
		t.Errorf("Expected InputTokens=1000, got %d", record.InputTokens)
	}

	if record.OutputTokens != 500 {
		t.Errorf("Expected OutputTokens=500, got %d", record.OutputTokens)
	}

	if record.SessionID != "test-session" {
		t.Errorf("Expected SessionID='test-session', got %q", record.SessionID)
	}

	// Check cost calculation (Claude: $3.00/$15.00 per 1M tokens)
	expectedCost := (1000.0 * 3.0 / 1000000.0) + (500.0 * 15.0 / 1000000.0)
	if diff := record.CostUSD - expectedCost; diff < -0.0001 || diff > 0.0001 {
		t.Errorf("Expected CostUSD=%.6f, got %.6f", expectedCost, record.CostUSD)
	}
}

func TestTrackerLogUnknownBackend(t *testing.T) {
	cfg := &config.Config{}
	registry := backend.NewRegistry()
	getSession := func() string { return "" }

	tracker := usage.NewTracker(cfg, registry, getSession)

	// Try to log for unknown backend
	err := tracker.Log("unknown-backend", 100, 50)
	if err == nil {
		t.Error("Expected error for unknown backend")
	}
}

func TestTrackerLogMultipleRecords(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		UsageFile: filepath.Join(tmpDir, "usage.jsonl"),
	}
	registry := backend.NewRegistry()
	getSession := func() string { return "session-1" }

	tracker := usage.NewTracker(cfg, registry, getSession)

	// Log multiple records
	tracker.Log("claude", 100, 50)
	tracker.Log("openai", 200, 100)
	tracker.Log("claude", 300, 150)

	records := tracker.LoadAll()
	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}
}

func TestTrackerLoadAll(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		UsageFile: filepath.Join(tmpDir, "usage.jsonl"),
	}
	registry := backend.NewRegistry()
	getSession := func() string { return "" }

	tracker := usage.NewTracker(cfg, registry, getSession)

	// Test with no file
	records := tracker.LoadAll()
	if len(records) != 0 {
		t.Errorf("Expected 0 records for missing file, got %d", len(records))
	}

	// Create test records
	testRecords := []usage.Record{
		{Timestamp: time.Now(), Backend: "claude", InputTokens: 100, OutputTokens: 50, CostUSD: 0.001},
		{Timestamp: time.Now(), Backend: "openai", InputTokens: 200, OutputTokens: 100, CostUSD: 0.002},
	}

	f, _ := os.Create(cfg.UsageFile)
	for _, r := range testRecords {
		data, _ := json.Marshal(r)
		f.WriteString(string(data) + "\n")
	}
	f.Close()

	records = tracker.LoadAll()
	if len(records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(records))
	}
}

func TestTrackerLoadAllInvalidLines(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		UsageFile: filepath.Join(tmpDir, "usage.jsonl"),
	}
	registry := backend.NewRegistry()
	getSession := func() string { return "" }

	tracker := usage.NewTracker(cfg, registry, getSession)

	// Create file with valid and invalid lines
	content := `{"timestamp":"2024-01-01T00:00:00Z","backend":"claude","input_tokens":100,"output_tokens":50,"cost_usd":0.001}
not valid json
{"timestamp":"2024-01-01T00:00:00Z","backend":"openai","input_tokens":200,"output_tokens":100,"cost_usd":0.002}
`
	os.WriteFile(cfg.UsageFile, []byte(content), 0600)

	records := tracker.LoadAll()
	if len(records) != 2 {
		t.Errorf("Expected 2 valid records, got %d", len(records))
	}
}

// ============================================================================
// CalculateCosts Tests
// ============================================================================

func TestTrackerCalculateCosts(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		UsageFile: filepath.Join(tmpDir, "usage.jsonl"),
	}
	registry := backend.NewRegistry()
	getSession := func() string { return "" }

	tracker := usage.NewTracker(cfg, registry, getSession)

	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	// Create test records
	// Use timestamps that are clearly within expected ranges
	records := []usage.Record{
		{Timestamp: now, Backend: "claude", CostUSD: 1.00},                          // Today
		{Timestamp: now.Add(-time.Hour), Backend: "claude", CostUSD: 0.50},          // Today
		{Timestamp: today.AddDate(0, 0, -1).Add(time.Hour), Backend: "openai", CostUSD: 2.00}, // Yesterday
		{Timestamp: today.AddDate(0, 0, -5).Add(time.Hour), Backend: "claude", CostUSD: 5.00}, // Within week
		{Timestamp: today.AddDate(0, -1, 0), Backend: "openai", CostUSD: 10.00},     // Last month
	}

	f, _ := os.Create(cfg.UsageFile)
	for _, r := range records {
		data, _ := json.Marshal(r)
		f.WriteString(string(data) + "\n")
	}
	f.Close()

	costs := tracker.CalculateCosts()

	// Daily should include only today's records (1.00 + 0.50 = 1.50)
	if costs.Daily != 1.50 {
		t.Errorf("Expected Daily=1.50, got %.2f", costs.Daily)
	}

	// Backend breakdown should include all records
	if costs.ByBackend["claude"] != 6.50 {
		t.Errorf("Expected ByBackend['claude']=6.50, got %.2f", costs.ByBackend["claude"])
	}

	if costs.ByBackend["openai"] != 12.00 {
		t.Errorf("Expected ByBackend['openai']=12.00, got %.2f", costs.ByBackend["openai"])
	}

	// Verify weekly and monthly are non-negative
	if costs.Weekly < 0 {
		t.Errorf("Expected non-negative Weekly, got %.2f", costs.Weekly)
	}
	if costs.Monthly < 0 {
		t.Errorf("Expected non-negative Monthly, got %.2f", costs.Monthly)
	}
}

func TestTrackerCalculateCostsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		UsageFile: filepath.Join(tmpDir, "usage.jsonl"),
	}
	registry := backend.NewRegistry()
	getSession := func() string { return "" }

	tracker := usage.NewTracker(cfg, registry, getSession)

	costs := tracker.CalculateCosts()

	if costs.Daily != 0 {
		t.Errorf("Expected Daily=0, got %.2f", costs.Daily)
	}
	if costs.Weekly != 0 {
		t.Errorf("Expected Weekly=0, got %.2f", costs.Weekly)
	}
	if costs.Monthly != 0 {
		t.Errorf("Expected Monthly=0, got %.2f", costs.Monthly)
	}
	if len(costs.ByBackend) != 0 {
		t.Errorf("Expected empty ByBackend, got %d entries", len(costs.ByBackend))
	}
}

// ============================================================================
// AuditLogger Tests
// ============================================================================

func TestNewAuditLogger(t *testing.T) {
	cfg := &config.Config{}
	getSession := func() string { return "" }

	logger := usage.NewAuditLogger(cfg, getSession)
	if logger == nil {
		t.Fatal("NewAuditLogger() returned nil")
	}
}

func TestAuditLoggerLog(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		AuditLog:     filepath.Join(tmpDir, "audit.log"),
		AuditEnabled: true,
	}
	getSession := func() string { return "test-session" }

	logger := usage.NewAuditLogger(cfg, getSession)

	err := logger.Log("Test audit message")
	if err != nil {
		t.Errorf("Log() failed: %v", err)
	}

	data, err := os.ReadFile(cfg.AuditLog)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Test audit message") {
		t.Errorf("Expected log to contain 'Test audit message', got: %s", content)
	}

	if !strings.Contains(content, "[test-session]") {
		t.Errorf("Expected log to contain session name, got: %s", content)
	}
}

func TestAuditLoggerLogDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		AuditLog:     filepath.Join(tmpDir, "audit.log"),
		AuditEnabled: false,
	}
	getSession := func() string { return "" }

	logger := usage.NewAuditLogger(cfg, getSession)

	err := logger.Log("This should not be logged")
	if err != nil {
		t.Errorf("Log() failed: %v", err)
	}

	// File should not be created when disabled
	_, err = os.Stat(cfg.AuditLog)
	if !os.IsNotExist(err) {
		t.Error("Expected audit log file not to be created when disabled")
	}
}

func TestAuditLoggerLogNoSession(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		AuditLog:     filepath.Join(tmpDir, "audit.log"),
		AuditEnabled: true,
	}
	getSession := func() string { return "" }

	logger := usage.NewAuditLogger(cfg, getSession)

	logger.Log("Message without session")

	data, _ := os.ReadFile(cfg.AuditLog)
	content := string(data)

	// Should not have session prefix when session is empty
	if strings.Contains(content, "[]") {
		t.Error("Expected no empty session prefix in log")
	}
}

// ============================================================================
// Record Struct Tests
// ============================================================================

func TestRecordFields(t *testing.T) {
	now := time.Now()
	record := usage.Record{
		Timestamp:    now,
		SessionID:    "session-1",
		Backend:      "claude",
		Model:        "claude-sonnet-4",
		InputTokens:  1000,
		OutputTokens: 500,
		CostUSD:      0.01,
	}

	if record.Backend != "claude" {
		t.Errorf("Expected Backend='claude', got %q", record.Backend)
	}
	if record.CostUSD != 0.01 {
		t.Errorf("Expected CostUSD=0.01, got %f", record.CostUSD)
	}
}

func TestInfoFields(t *testing.T) {
	info := usage.Info{
		Backend:      "claude",
		TotalTokens:  1000,
		InputTokens:  600,
		OutputTokens: 400,
		TotalCost:    0.01,
		RequestCount: 5,
		Period:       "current period",
		Error:        "",
	}

	if info.Backend != "claude" {
		t.Errorf("Expected Backend='claude', got %q", info.Backend)
	}
	if info.TotalTokens != 1000 {
		t.Errorf("Expected TotalTokens=1000, got %d", info.TotalTokens)
	}
}

func TestCostsFields(t *testing.T) {
	costs := usage.Costs{
		Daily:   1.50,
		Weekly:  10.00,
		Monthly: 50.00,
		ByBackend: map[string]float64{
			"claude": 30.00,
			"openai": 20.00,
		},
	}

	if costs.Daily != 1.50 {
		t.Errorf("Expected Daily=1.50, got %.2f", costs.Daily)
	}
	if costs.ByBackend["claude"] != 30.00 {
		t.Errorf("Expected ByBackend['claude']=30.00, got %.2f", costs.ByBackend["claude"])
	}
}

// ============================================================================
// Cost Calculation Tests for Different Backends
// ============================================================================

func TestTrackerLogCostCalculation(t *testing.T) {
	tests := []struct {
		name          string
		backend       string
		inputTokens   int64
		outputTokens  int64
	}{
		{
			name:         "Claude 1K tokens",
			backend:      "claude",
			inputTokens:  1000,
			outputTokens: 0,
		},
		{
			name:         "Ollama free",
			backend:      "ollama",
			inputTokens:  1000000,
			outputTokens: 1000000,
		},
		{
			name:         "DeepSeek pricing",
			backend:      "deepseek",
			inputTokens:  1000000,
			outputTokens: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Config{
				UsageFile: filepath.Join(tmpDir, "usage.jsonl"),
			}
			registry := backend.NewRegistry()
			getSession := func() string { return "" }

			tracker := usage.NewTracker(cfg, registry, getSession)
			tracker.Log(tt.backend, tt.inputTokens, tt.outputTokens)

			records := tracker.LoadAll()
			if len(records) != 1 {
				t.Fatalf("Expected 1 record, got %d", len(records))
			}

			// Check cost is calculated (specific values depend on backend pricing)
			if tt.backend == "ollama" && records[0].CostUSD != 0 {
				t.Errorf("Expected zero cost for Ollama, got %f", records[0].CostUSD)
			}
		})
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkTrackerLog(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := &config.Config{
		UsageFile: filepath.Join(tmpDir, "usage.jsonl"),
	}
	registry := backend.NewRegistry()
	getSession := func() string { return "benchmark-session" }

	tracker := usage.NewTracker(cfg, registry, getSession)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.Log("claude", 1000, 500)
	}
}

func BenchmarkTrackerLoadAll(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := &config.Config{
		UsageFile: filepath.Join(tmpDir, "usage.jsonl"),
	}
	registry := backend.NewRegistry()
	getSession := func() string { return "" }

	tracker := usage.NewTracker(cfg, registry, getSession)

	// Create test data
	for i := 0; i < 100; i++ {
		tracker.Log("claude", 1000, 500)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.LoadAll()
	}
}

func BenchmarkTrackerCalculateCosts(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := &config.Config{
		UsageFile: filepath.Join(tmpDir, "usage.jsonl"),
	}
	registry := backend.NewRegistry()
	getSession := func() string { return "" }

	tracker := usage.NewTracker(cfg, registry, getSession)

	// Create test data
	for i := 0; i < 100; i++ {
		tracker.Log("claude", 1000, 500)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.CalculateCosts()
	}
}
