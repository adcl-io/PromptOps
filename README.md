# PromptOps

AI Model Backend Switcher for Claude Code. Switch between Anthropic Claude, Z.AI (GLM), and Kimi (Moonshot) without reconfiguring your workflow.

## Features

- **Multiple Backends**: Claude (Anthropic), Z.AI (GLM-4.7), Kimi (Moonshot)
- **Seamless Switching**: One command to change backend and launch Claude Code
- **YOLO Mode**: Skip confirmations and auto-launch for rapid context switching
- **Secure by Default**: API keys stored with restricted permissions, masked in output
- **Cross-Platform**: Native binaries for macOS (Intel/Apple Silicon) and Linux

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
# Get your API key from: https://console.anthropic.com/
ANTHROPIC_API_KEY=sk-ant-api03-...

# Get your API key from: https://open.bigmodel.cn/
ZAI_API_KEY=5869b4b03f...

# Get your API key from: https://platform.moonshot.cn/
KIMI_API_KEY=sk-kimi-...
```

The file is created with `0600` permissions (owner read/write only).

### 3. Switch Backend

```bash
# Switch to Kimi
promptops kimi

# Switch to Z.AI
promptops zai

# Switch to Claude
promptops claude
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
| `NEXUS_YOLO_MODE_ZAI` | YOLO for Z.AI | `false` |
| `NEXUS_YOLO_MODE_KIMI` | YOLO for Kimi | `false` |
| `NEXUS_AUDIT_LOG` | Enable audit logging | `true` |

### YOLO Mode

Enable YOLO mode to skip animations and confirmations:

```bash
# Global YOLO (all backends)
NEXUS_YOLO_MODE=true

# Per-backend YOLO
NEXUS_YOLO_MODE_KIMI=true
```

## Commands

| Command | Description |
|---------|-------------|
| `promptops claude` | Switch to Claude and launch |
| `promptops zai` | Switch to Z.AI and launch |
| `promptops kimi` | Switch to Kimi and launch |
| `promptops run` | Launch with current backend |
| `promptops status` | Show configuration |
| `promptops init` | Create `.env.local` template |
| `promptops version` | Show version |
| `promptops help` | Show help |

## Backend Configuration

### Claude (Anthropic)

Uses standard Anthropic API. No additional configuration required beyond `ANTHROPIC_API_KEY`.

### Z.AI (GLM)

Proxies Claude Code requests to Z.AI's GLM models:

- Base URL: `https://api.z.ai/api/anthropic`
- Models: GLM-4.7 (Sonnet/Opus), GLM-4.5-Air (Haiku)

### Kimi (Moonshot)

Uses Kimi Code API for coding-optimized models:

- Base URL: `https://api.kimi.com/coding`
- Models: kimi-for-coding

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

## Development

### Building

```bash
# Local build
go build -o promptops .

# Cross-compile
make linux      # Linux AMD64/ARM64
make macos      # macOS AMD64
make macos-arm  # macOS ARM64
```

### Testing

```bash
make test
```

## License

MIT License - see LICENSE file for details.

## Contributing

See CLAUDE.md for project guidelines.
