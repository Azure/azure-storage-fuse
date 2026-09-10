#!/usr/bin/env bash
#
# Uninstall OpenClaw and remove all its state + dependencies from $HOME.
#
set -uo pipefail

echo "==> Stopping any running OpenClaw processes"
openclaw gateway stop 2>/dev/null || true

# Stop the systemd user service if present
systemctl --user stop openclaw-gateway.service 2>/dev/null || true
systemctl --user disable openclaw-gateway.service 2>/dev/null || true

# Kill lingering gateway/node processes WITHOUT matching this script.
# Match the actual node process running openclaw, and exclude our own PID.
pkill -f 'node.*openclaw' 2>/dev/null || true
sleep 2

echo "==> Uninstalling the global npm package (removes all bundled dependencies)"
npm uninstall -g openclaw 2>/dev/null || echo "   (not found via npm, continuing)"

echo "==> Force-removing any leftover package dir + binary in \$HOME"
rm -rf "$HOME/.npm-global/lib/node_modules/openclaw"
rm -f  "$HOME/.npm-global/bin/openclaw"
rm -rf "$HOME/.node_modules/openclaw" \
       "$HOME/node_modules/openclaw"
rm -f  "$HOME/bin/openclaw"

echo "==> Removing systemd unit files (if any)"
rm -f "$HOME/.config/systemd/user/openclaw-gateway.service"
systemctl --user daemon-reload 2>/dev/null || true

echo "==> Deleting all OpenClaw state in \$HOME"
rm -rf "$HOME/.openclaw" \
       "$HOME/.config/openclaw" \
       "$HOME/.cache/openclaw" \
       "$HOME/.local/share/openclaw" \
       "$HOME/.local/state/openclaw"

echo "==> Cleaning npm cache"
npm cache clean --force 2>/dev/null || true

echo "==> Verifying"
if command -v openclaw >/dev/null 2>&1; then
  echo "      openclaw still on PATH: $(command -v openclaw)"
else
  echo "   ✅ openclaw CLI removed"
fi
if lsof -i :18789 >/dev/null 2>&1; then
  echo "      port 18789 still in use"
else
  echo "   ✅ port 18789 free"
fi
echo "   Remaining openclaw dirs under \$HOME (should be none):"
find "$HOME" -maxdepth 4 -name 'openclaw' -type d 2>/dev/null | sed 's/^/      /' || true

echo "Done."
