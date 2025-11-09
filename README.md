# crosh

A lightweight network acceleration tool for Chinese developers.

## What it does

**crosh** automatically configures package manager mirrors and proxy settings to speed up downloads in China.

- **Mirrors**: npm, pip, apt, cargo, go, docker
- **Proxy**: Xray-core based proxy with subscription support
- **Simple**: One command to enable/disable everything

## Installation

```bash
# Recommended for users in China (via Cloudflare CDN)
curl -fsSL https://crosh.boomyao.com/scripts/install.sh | bash

# Alternative (directly from GitHub)
curl -fsSL https://raw.githubusercontent.com/boomyao/crosh/main/scripts/install.sh | bash
```

Or build from source:

```bash
make build
sudo make install
```

## Usage

```bash
# Enable acceleration (mirrors only)
crosh

# Configure with proxy subscription
crosh https://your-subscription-url

# Disable all acceleration
crosh off

# Check current status
crosh status
```

That's it!

## How it works

- **Mirrors**: Updates config files for package managers to use Chinese mirrors
- **Proxy**: Downloads and runs Xray-core with your subscription URL
- All changes are reversible with `crosh off`

## Requirements

- Linux or macOS
- Root access (for system-level configs)

## License

MIT License - see [LICENSE](LICENSE)

