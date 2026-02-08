# CLAUDE.md - Project Guidelines

## Project Overview

PromptOps is an open-source toolbox for working with large language models in a repeatable, vendor-neutral way. It provides shared prompts, adapters, and CLI shims that let tools like Claude-CLI, OpenAI, and local models behave consistently, without rewriting workflows every time a model or API changes.

## Architecture

The project is built as a single Go binary (`promptops`) that:

- Reads configuration from `.env.local` (API keys, YOLO mode settings)
- Maintains current backend state in `state` file (just the backend name)
- Launches Claude Code with the appropriate environment variables for the selected backend
- Supports three backends: Claude (Anthropic), Z.AI (GLM), and Kimi (Moonshot)

### Key Components

- `main.go` - CLI entry point and command dispatcher
- `Config` struct - Parses `.env.local` and holds settings
- `Backend` struct - Defines backend-specific configuration
- `launchClaudeWithBackend()` - Sets environment and execs Claude Code

## Hard Rules

1. **No co-authored commits** - Do not add "Co-Authored-By" lines to commit messages
2. **No promotions** - Do not add marketing language, hype, or promotional content
3. **No emoticons in code** - No emoji in source files, comments, or output strings (use plain ASCII)
4. **No debug logging of secrets** - API keys must never appear in logs, even masked

## Code Style

- Go standard formatting (`go fmt`)
- Clear, descriptive variable names
- Errors handled explicitly, no silent failures
- File permissions: sensitive files use `0600`, regular files use `0644`

## Security Requirements

- API keys only stored in `.env.local` with `0600` permissions
- Keys must be masked in all output (format: `sk-xx...xx`)
- Audit logs use `0600` permissions
- State file contains only backend name, never keys
- Environment variables filtered before launching child process

## Building

```bash
go build -o promptops .
```

Cross-compile:
```bash
make linux      # Linux AMD64/ARM64
make macos      # macOS AMD64
make macos-arm  # macOS ARM64 (Apple Silicon)
```

## Configuration

`.env.local` format:
```bash
NEXUS_YOLO_MODE=false
NEXUS_YOLO_MODE_CLAUDE=false
NEXUS_YOLO_MODE_ZAI=false
NEXUS_YOLO_MODE_KIMI=false
ANTHROPIC_API_KEY=sk-ant-...
ZAI_API_KEY=...
KIMI_API_KEY=sk-kimi-...
```

## Usage

```bash
./promptops status      # Show current backend and config
./promptops claude      # Switch to Claude and launch
./promptops zai         # Switch to Z.AI and launch
./promptops kimi        # Switch to Kimi and launch
./promptops run         # Launch with current backend
```
