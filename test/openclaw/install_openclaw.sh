#!/usr/bin/env bash
#
# install_openclaw.sh
# ---------------------------------------------------------------------------
# Installs and configures OpenClaw along with its prerequisites.
#
# What this script does:
#   1. Updates the system and installs base packages (git, curl, jq, etc.)
#   2. Installs Python 3 and pip
#   3. Installs the `uv` Python package manager
#   4. Installs a specific version of OpenClaw via npm
#   5. Authenticates OpenClaw with GitHub Copilot and selects a model
#   6. Installs and (re)starts the OpenClaw gateway systemd user service
#
# Usage:
#   chmod +x install_openclaw.sh
#   ./install_openclaw.sh
# ---------------------------------------------------------------------------

# Fail fast and loudly:
#   -e : exit on any command error
#   -u : treat unset variables as errors
#   -o pipefail : a pipeline fails if any component fails
set -euo pipefail

# Pin the OpenClaw version so installs are reproducible.
OPENCLAW_VERSION="2026.7.1-2"
OPENCLAW_MODEL="github-copilot/gpt-5.4-mini"

# Small helper for consistent, visible status messages.
log() {
    printf '\n\033[1;34m==>\033[0m %s\n' "$1"
}

# ---------------------------------------------------------------------------
# 1. System update and base packages
# ---------------------------------------------------------------------------
log "Updating package lists and upgrading installed packages..."
sudo apt-get update && sudo apt-get upgrade -y

log "Installing base build tools and utilities (git, curl, jq, build-essential)..."
sudo apt-get install -y git curl jq build-essential

# ---------------------------------------------------------------------------
# 2. Python 3
# ---------------------------------------------------------------------------
log "Installing Python 3 and pip..."
sudo apt-get install -y python3 python3-pip

log "Verifying Python installation..."
python3 --version

# ---------------------------------------------------------------------------
# 3. uv (fast Python package/dependency manager)
# ---------------------------------------------------------------------------
log "Installing the 'uv' package manager..."
curl -LsSf https://astral.sh/uv/install.sh | sh

# Load uv into the current shell's PATH so the next command can find it.
log "Loading uv environment into the current shell..."
source "$HOME/.local/bin/env"

log "Verifying uv installation..."
uv --version

# ---------------------------------------------------------------------------
# 4. OpenClaw
# ---------------------------------------------------------------------------
# Alternative install methods (kept for reference):
#   # Install the latest OpenClaw:
#   # curl -fsSL https://openclaw.ai/install.sh | bash

log "Installing OpenClaw version ${OPENCLAW_VERSION} via npm..."
npm install -g "openclaw@${OPENCLAW_VERSION}"

log "Verifying OpenClaw installation..."
openclaw --version

# ---------------------------------------------------------------------------
# 5. Authentication and model selection
# ---------------------------------------------------------------------------
# NOTE: This command is BLOCKING and interactive. It requires the user to
# authenticate OpenClaw using their GitHub Copilot subscription (e.g. by
# visiting a device-login URL and entering a code). The script will pause here
# until authentication is completed.
log "Authenticating OpenClaw with GitHub Copilot..."
openclaw models auth login-github-copilot

log "Setting the active model to ${OPENCLAW_MODEL}..."
openclaw models set "${OPENCLAW_MODEL}"

log "Checking model status..."
openclaw models status

# ---------------------------------------------------------------------------
# 6. Gateway service (systemd user unit)
# ---------------------------------------------------------------------------
log "Installing the OpenClaw gateway (forcing reinstall)..."
openclaw gateway install --force

log "Reloading systemd user units..."
systemctl --user daemon-reload

log "Resetting any previously failed gateway service state..."
systemctl --user reset-failed openclaw-gateway.service

log "Restarting the OpenClaw gateway service..."
systemctl --user restart openclaw-gateway.service

# Give the gateway a few seconds to start up before querying its status.
log "Waiting 5 seconds for the gateway to start..."
sleep 5

log "Checking gateway status..."
openclaw gateway status

log "OpenClaw installation and setup complete!"
