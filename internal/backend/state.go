// Package backend provides state management for the current backend.
package backend

import (
	"os"
	"strings"

	"nexus/internal/config"
)

// CurrentReader reads the current backend from state.
type CurrentReader struct {
	stateFile string
}

// NewCurrentReader creates a new current backend reader.
func NewCurrentReader(cfg *config.Config) *CurrentReader {
	return &CurrentReader{stateFile: cfg.StateFile}
}

// Get returns the current backend name from state file.
func (r *CurrentReader) Get() string {
	data, err := os.ReadFile(r.stateFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// CurrentWriter writes the current backend to state.
type CurrentWriter struct {
	stateFile string
}

// NewCurrentWriter creates a new current backend writer.
func NewCurrentWriter(cfg *config.Config) *CurrentWriter {
	return &CurrentWriter{stateFile: cfg.StateFile}
}

// Set sets the current backend name in state file.
func (w *CurrentWriter) Set(backend string) error {
	return config.WriteFileAtomic(w.stateFile, []byte(backend), 0600)
}
