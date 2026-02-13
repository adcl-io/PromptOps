// Package config_test provides tests for the config package.
package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nexus/internal/config"
)

// ============================================================================
// Config Struct Tests
// ============================================================================

func TestConfigGetYoloMode(t *testing.T) {
	tests := []struct {
		name           string
		globalYolo     bool
		backendYolo    map[string]bool
		backend        string
		expectedResult bool
	}{
		{
			name:           "global yolo true",
			globalYolo:     true,
			backendYolo:    map[string]bool{},
			backend:        "claude",
			expectedResult: true,
		},
		{
			name:           "backend specific true",
			globalYolo:     false,
			backendYolo:    map[string]bool{"claude": true},
			backend:        "claude",
			expectedResult: true,
		},
		{
			name:           "backend specific false",
			globalYolo:     false,
			backendYolo:    map[string]bool{"claude": false},
			backend:        "claude",
			expectedResult: false,
		},
		{
			name:           "no config defaults to true",
			globalYolo:     false,
			backendYolo:    map[string]bool{},
			backend:        "claude",
			expectedResult: true,
		},
		{
			name:           "different backend not in map",
			globalYolo:     false,
			backendYolo:    map[string]bool{"claude": false},
			backend:        "openai",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				YoloMode:  tt.globalYolo,
				YoloModes: tt.backendYolo,
			}
			result := cfg.GetYoloMode(tt.backend)
			if result != tt.expectedResult {
				t.Errorf("GetYoloMode(%q) = %v, want %v", tt.backend, result, tt.expectedResult)
			}
		})
	}
}

// ============================================================================
// Loader Tests
// ============================================================================

func TestNewLoader(t *testing.T) {
	loader := config.NewLoader()
	if loader == nil {
		t.Fatal("NewLoader() returned nil")
	}
}

func TestLoaderLoad(t *testing.T) {
	loader := config.NewLoader()

	// Load config - this will use the actual executable path
	cfg, err := loader.Load()
	if err != nil {
		// This is expected if the executable path can't be determined in test environment
		t.Skipf("Load() failed (expected in test environment): %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Verify defaults are set
	if cfg.DefaultBackend != "claude" {
		t.Errorf("Expected DefaultBackend='claude' by default, got %q", cfg.DefaultBackend)
	}

	if cfg.DailyBudget != 10.00 {
		t.Errorf("Expected DailyBudget=10.00 by default, got %f", cfg.DailyBudget)
	}
}

func TestLoaderLoadPathTraversal(t *testing.T) {
	// Set a malicious env file path
	oldEnv := os.Getenv("NEXUS_ENV_FILE")
	os.Setenv("NEXUS_ENV_FILE", "../../../etc/passwd")
	defer os.Setenv("NEXUS_ENV_FILE", oldEnv)

	loader := config.NewLoader()
	_, err := loader.Load()

	if err == nil {
		t.Error("Expected error for path traversal attempt")
	}

	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("Expected path traversal error, got: %v", err)
	}
}

// ============================================================================
// WriteFileAtomic Tests
// ============================================================================

func TestWriteFileAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content")
	if err := config.WriteFileAtomic(testFile, content, 0600); err != nil {
		t.Errorf("WriteFileAtomic failed: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("File content mismatch: got %q, want %q", data, content)
	}

	// Check permissions
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestWriteFileAtomicOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Write initial content
	os.WriteFile(testFile, []byte("old content"), 0644)

	// Overwrite with atomic write
	newContent := []byte("new content")
	if err := config.WriteFileAtomic(testFile, newContent, 0600); err != nil {
		t.Errorf("WriteFileAtomic failed: %v", err)
	}

	data, _ := os.ReadFile(testFile)
	if string(data) != "new content" {
		t.Errorf("File content not updated: got %q", data)
	}
}

// ============================================================================
// Config Default Values Tests
// ============================================================================

func TestConfigDefaults(t *testing.T) {
	cfg := &config.Config{
		Keys:         make(map[string]string),
		YoloModes:    make(map[string]bool),
		OllamaModels: make(map[string]string),
		ZAIModels:    make(map[string]string),
		KimiModels:   make(map[string]string),
	}

	// Test default values when not loaded from file
	if cfg.DefaultBackend != "" {
		t.Errorf("Expected empty DefaultBackend, got %q", cfg.DefaultBackend)
	}

	// Test GetYoloMode with nil YoloModes
	cfg.YoloModes = nil
	if !cfg.GetYoloMode("claude") {
		t.Error("Expected GetYoloMode to return true when YoloModes is nil")
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkWriteFileAtomic(b *testing.B) {
	tmpDir := b.TempDir()
	content := []byte("benchmark test content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, "test.txt")
		config.WriteFileAtomic(testFile, content, 0600)
	}
}

func BenchmarkConfigGetYoloMode(b *testing.B) {
	cfg := &config.Config{
		YoloMode:  false,
		YoloModes: map[string]bool{"claude": true, "openai": false},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.GetYoloMode("claude")
	}
}
