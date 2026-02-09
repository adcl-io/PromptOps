# PromptOps

AI Model Backend Switcher for Claude Code. Switch between 10+ LLM providers (OpenAI, DeepSeek, Gemini, Mistral, Anthropic, and more) without reconfiguring your workflow.

## Features

- **Multiple Backends**: 10 providers including OpenAI, DeepSeek, Gemini, Mistral, Claude, Z.AI, Kimi, Groq, Together AI, and OpenRouter
- **Seamless Switching**: One command to change backend and launch Claude Code
- **YOLO Mode**: Skip confirmations and auto-launch for rapid context switching
- **Secure by Default**: API keys stored with restricted permissions (0600), masked in all output
- **Cross-Platform**: Native binaries for macOS (Intel/Apple Silicon) and Linux
- **Audit Logging**: Track all backend switches with timestamps

## Requirements

- Go 1.21 or later (for building from source)
- Claude Code CLI (`claude` command must be in PATH)
- At least one API key from a supported LLM provider

## Installation

### Pre-built Binaries

Download the latest release for your platform:

```bash
# macOS Apple Silicon
curl -L https://github.com/adcl-io/PromptOps/releases/latest/download/promptops-darwin-arm64 -o promptops

# macOS Intel
curl -L https://github.com/adcl-io/PromptOps/releases/latest/download/promptops-darwin-amd64 -o promptops

# Linux AMD64
curl -L https://github.com/adcl-io/PromptOps/releases/latest/download/promptops-linux-amd64 -o promptops

chmod +x promptops
sudo mv promptops /usr/local/bin/
```

### Build from Source

Requires Go 1.21 or later:

```bash
git clone https://github.com/adcl-io/PromptOps.git
cd PromptOps
go build -o promptops .
sudo mv promptops /usr/local/bin/
```

## Quick Start

### 1. Initialize Configuration

```bash
promptops init
```

This creates `.env.local` in the current directory with templates for your API keys.

### 2. Add API Keys

Edit `.env.local` and add your keys:

```bash
# OpenAI - https://platform.openai.com/
OPENAI_API_KEY=sk-proj-...

# DeepSeek - https://platform.deepseek.com/
DEEPSEEK_API_KEY=sk-ds-...

# Google Gemini - https://ai.google.dev/
GEMINI_API_KEY=AIza...

# Mistral - https://console.mistral.ai/
MISTRAL_API_KEY=...

# Anthropic Claude - https://console.anthropic.com/
ANTHROPIC_API_KEY=sk-ant-api03-...

# Z.AI (GLM) - https://open.bigmodel.cn/
ZAI_API_KEY=5869b4b03f...

# Kimi (Moonshot) - https://platform.moonshot.cn/
KIMI_API_KEY=sk-kimi-...

# Groq - https://console.groq.com/
GROQ_API_KEY=gsk_...

# Together AI - https://api.together.xyz/
TOGETHER_API_KEY=...

# OpenRouter - https://openrouter.ai/
OPENROUTER_API_KEY=sk-or-...
```

The file is created with `0600` permissions (owner read/write only).

### 3. Switch Backend

```bash
# Tier 1 Backends (Recommended for code/security)
promptops openai      # OpenAI GPT-4o / GPT-4o-mini / o1
promptops deepseek    # DeepSeek V3 / R1
promptops gemini      # Google Gemini 2.5 Pro
promptops mistral     # Mistral Large / Codestral
promptops claude      # Anthropic Claude Sonnet 4.5
promptops zai         # Z.AI GLM-4.7 / GLM-4.5-Air
promptops kimi        # Kimi K2

# Tier 2 Backends (Alternative providers)
promptops groq        # Groq Llama 3.3 70B / 405B
promptops together    # Together AI (Llama/Qwen/DeepSeek)
promptops openrouter  # OpenRouter (200+ models)
```

Each command saves the backend to `state` and launches Claude Code with the appropriate environment.

### 4. Check Status

```bash
promptops status
```

Shows current backend, API key status (masked), and configuration.

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NEXUS_ENV_FILE` | Path to env file | `./.env.local` |
| `NEXUS_YOLO_MODE` | Global YOLO mode | `false` |
| `NEXUS_YOLO_MODE_CLAUDE` | YOLO for Claude | `false` |
| `NEXUS_YOLO_MODE_OPENAI` | YOLO for OpenAI | `false` |
| `NEXUS_YOLO_MODE_DEEPSEEK` | YOLO for DeepSeek | `false` |
| `NEXUS_YOLO_MODE_GEMINI` | YOLO for Gemini | `false` |
| `NEXUS_YOLO_MODE_MISTRAL` | YOLO for Mistral | `false` |
| `NEXUS_YOLO_MODE_ZAI` | YOLO for Z.AI | `false` |
| `NEXUS_YOLO_MODE_KIMI` | YOLO for Kimi | `false` |
| `NEXUS_YOLO_MODE_GROQ` | YOLO for Groq | `false` |
| `NEXUS_YOLO_MODE_TOGETHER` | YOLO for Together AI | `false` |
| `NEXUS_YOLO_MODE_OPENROUTER` | YOLO for OpenRouter | `false` |
| `NEXUS_DEFAULT_BACKEND` | Default backend | `claude` |
| `NEXUS_VERIFY_ON_SWITCH` | Verify on switch | `true` |
| `NEXUS_AUDIT_LOG` | Enable audit logging | `true` |

### YOLO Mode

Enable YOLO mode to skip animations and confirmations:

```bash
# Global YOLO (all backends)
NEXUS_YOLO_MODE=true

# Per-backend YOLO
NEXUS_YOLO_MODE_DEEPSEEK=true
NEXUS_YOLO_MODE_OPENAI=true
```

## Commands

| Command | Description |
|---------|-------------|
| `promptops openai` | Switch to OpenAI and launch |
| `promptops deepseek` | Switch to DeepSeek and launch |
| `promptops gemini` | Switch to Gemini and launch |
| `promptops mistral` | Switch to Mistral and launch |
| `promptops claude` | Switch to Claude and launch |
| `promptops zai` | Switch to Z.AI and launch |
| `promptops kimi` | Switch to Kimi and launch |
| `promptops groq` | Switch to Groq and launch |
| `promptops together` | Switch to Together AI and launch |
| `promptops openrouter` | Switch to OpenRouter and launch |
| `promptops run` | Launch with current backend |
| `promptops status` | Show configuration |
| `promptops init` | Create `.env.local` template |
| `promptops version` | Show version |
| `promptops help` | Show help |

## Examples

### Daily Workflow

```bash
# Check current configuration
promptops status

# Switch to DeepSeek for a reasoning-heavy task
promptops deepseek

# Later, switch to GPT-4o for general coding
promptops openai

# Use Groq for ultra-fast responses
promptops groq

# Launch without switching (uses current backend)
promptops run
```

### Using YOLO Mode for Speed

```bash
# Enable YOLO for DeepSeek to skip confirmations
export NEXUS_YOLO_MODE_DEEPSEEK=true

# Now switching is instant
promptops deepseek
```

### Passing Arguments to Claude Code

```bash
# Run with specific permissions mode
promptops run --permission-mode acceptEdits

# Run in a specific directory
promptops run /path/to/project
```

## Backend Configuration

### Tier 1 Backends (Recommended for Code/Security)

#### OpenAI

Uses standard OpenAI API:

- Base URL: `https://api.openai.com/v1`
- Models: GPT-4o (Sonnet), GPT-4o-mini (Haiku), o1 (Opus)

#### DeepSeek

Uses DeepSeek API for reasoning and chat models:

- Base URL: `https://api.deepseek.com/v1`
- Models: DeepSeek-V3 (Sonnet), DeepSeek-R1 (Opus), DeepSeek-chat (Haiku)

#### Gemini (Google)

Uses Gemini API via OpenAI-compatible endpoint:

- Base URL: `https://generativelanguage.googleapis.com/v1beta/openai`
- Models: Gemini 2.5 Pro (Sonnet/Opus), Gemini 2.5 Flash (Haiku)

#### Mistral

Uses Mistral AI API:

- Base URL: `https://api.mistral.ai/v1`
- Models: Mistral Large (Sonnet/Opus), Codestral (Haiku)

#### Claude (Anthropic)

Uses standard Anthropic API. No additional configuration required beyond `ANTHROPIC_API_KEY`.

#### Z.AI (GLM)

Proxies Claude Code requests to Z.AI's GLM models:

- Base URL: `https://api.z.ai/api/anthropic`
- Models: GLM-4.7 (Sonnet/Opus), GLM-4.5-Air (Haiku)

#### Kimi (Moonshot)

Uses Kimi Code API for coding-optimized models:

- Base URL: `https://api.kimi.com/coding`
- Models: kimi-for-coding

### Tier 2 Backends (Alternative Providers)

#### Groq

Ultra-fast inference with Llama models:

- Base URL: `https://api.groq.com/openai/v1`
- Models: Llama 3.3 70B (Sonnet), Llama 3.1 405B (Opus)

#### Together AI

Aggregated model hosting:

- Base URL: `https://api.together.xyz/v1`
- Models: DeepSeek-V3 (Sonnet), Llama 3.1 405B (Opus), Llama 3.3 70B (Haiku)

#### OpenRouter

Unified API for 200+ models:

- Base URL: `https://openrouter.ai/api/v1`
- Models: Anthropic Claude 3.5 Sonnet (Sonnet), Claude 3 Opus (Opus), Gemini Flash 1.5 (Haiku)

## Project Structure

```
.
├── .env.local              # API keys and configuration (0600 permissions)
├── state                   # Current backend name (e.g., "kimi")
├── .promptops-audit.log    # Audit log (0600 permissions)
├── promptops               # Binary
├── main.go                 # Source code
├── Makefile                # Build automation
└── CLAUDE.md               # Project guidelines
```

## Security

- API keys stored only in `.env.local` with `0600` permissions
- Keys masked in all output (e.g., `sk-kimi-...F9OI`)
- Audit logs created with `0600` permissions
- State file contains only backend name, never keys
- Environment variables filtered before launching child process

### Development

### Building

```bash
# Local build
go build -o promptops .

# Install to /usr/local/bin
make install

# Cross-compile
make linux      # Linux AMD64/ARM64
make macos      # macOS AMD64
make macos-arm  # macOS ARM64 (Apple Silicon)
make release    # Build all platforms

# Development
make test       # Run tests
make fmt        # Format code
make clean      # Clean build artifacts
```

## License

MIT License - see LICENSE file for details.

## Contributing

See CLAUDE.md for project guidelines.
