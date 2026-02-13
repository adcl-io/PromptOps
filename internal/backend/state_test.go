// Package backend_test provides tests for the backend state management.
package backend_test

import (
	"os"
	"path/filepath"
	"testing"

	"nexus/internal/backend"
	"nexus/internal/config"
)

// ============================================================================
// CurrentReader Tests
// ============================================================================

func TestNewCurrentReader(t *testing.T) {
	cfg := &config.Config{
		StateFile: "/tmp/test-state",
	}

	reader := backend.NewCurrentReader(cfg)
	if reader == nil {
		t.Fatal("NewCurrentReader() returned nil")
	}
}

func TestCurrentReaderGet(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state")

	// Test with no state file
	cfg := &config.Config{StateFile: stateFile}
	reader := backend.NewCurrentReader(cfg)

	result := reader.Get()
	if result != "" {
		t.Errorf("Expected empty string for missing state file, got %q", result)
	}

	// Test with state file
	if err := os.WriteFile(stateFile, []byte("claude\n"), 0600); err != nil {
		t.Fatalf("Failed to write state file: %v", err)
	}

	result = reader.Get()
	if result != "claude" {
		t.Errorf("Expected 'claude', got %q", result)
	}

	// Test with whitespace in state file
	if err := os.WriteFile(stateFile, []byte("  openai  \n"), 0600); err != nil {
		t.Fatalf("Failed to write state file: %v", err)
	}

	result = reader.Get()
	if result != "openai" {
		t.Errorf("Expected 'openai' (whitespace trimmed), got %q", result)
	}
}

func TestCurrentReaderGetEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state")

	// Create empty state file
	os.WriteFile(stateFile, []byte(""), 0600)

	cfg := &config.Config{StateFile: stateFile}
	reader := backend.NewCurrentReader(cfg)

	result := reader.Get()
	if result != "" {
		t.Errorf("Expected empty string for empty state file, got %q", result)
	}
}

// ============================================================================
// CurrentWriter Tests
// ============================================================================

func TestNewCurrentWriter(t *testing.T) {
	cfg := &config.Config{
		StateFile: "/tmp/test-state",
	}

	writer := backend.NewCurrentWriter(cfg)
	if writer == nil {
		t.Fatal("NewCurrentWriter() returned nil")
	}
}

func TestCurrentWriterSet(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state")

	cfg := &config.Config{StateFile: stateFile}
	writer := backend.NewCurrentWriter(cfg)

	if err := writer.Set("openai"); err != nil {
		t.Errorf("Set() failed: %v", err)
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	if string(data) != "openai" {
		t.Errorf("Expected state file to contain 'openai', got %q", string(data))
	}
}

func TestCurrentWriterSetOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state")

	cfg := &config.Config{StateFile: stateFile}
	writer := backend.NewCurrentWriter(cfg)

	// Set initial value
	writer.Set("claude")

	// Overwrite with new value
	if err := writer.Set("kimi"); err != nil {
		t.Errorf("Set() failed: %v", err)
	}

	data, _ := os.ReadFile(stateFile)
	if string(data) != "kimi" {
		t.Errorf("Expected state file to contain 'kimi', got %q", string(data))
	}
}

func TestCurrentWriterSetPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state")

	cfg := &config.Config{StateFile: stateFile}
	writer := backend.NewCurrentWriter(cfg)

	if err := writer.Set("claude"); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	info, err := os.Stat(stateFile)
	if err != nil {
		t.Fatalf("Failed to stat state file: %v", err)
	}

	// Check that file has 0600 permissions
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
	}
}

// ============================================================================
// State Manager Integration Tests
// ============================================================================

func TestStateManagerRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state")

	cfg := &config.Config{StateFile: stateFile}
	writer := backend.NewCurrentWriter(cfg)
	reader := backend.NewCurrentReader(cfg)

	// Write and read back
	backends := []string{"claude", "openai", "kimi", "zai", "deepseek"}
	for _, be := range backends {
		if err := writer.Set(be); err != nil {
			t.Errorf("Set(%q) failed: %v", be, err)
			continue
		}

		result := reader.Get()
		if result != be {
			t.Errorf("Get() returned %q, expected %q", result, be)
		}
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkCurrentReaderGet(b *testing.B) {
	tmpDir := b.TempDir()
	stateFile := filepath.Join(tmpDir, "state")
	os.WriteFile(stateFile, []byte("claude"), 0600)

	cfg := &config.Config{StateFile: stateFile}
	reader := backend.NewCurrentReader(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Get()
	}
}

func BenchmarkCurrentWriterSet(b *testing.B) {
	tmpDir := b.TempDir()
	stateFile := filepath.Join(tmpDir, "state")

	cfg := &config.Config{StateFile: stateFile}
	writer := backend.NewCurrentWriter(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Set("claude")
	}
}
