#!/usr/bin/env sh
set -eu

if command -v systemctl >/dev/null 2>&1; then
  systemctl stop data-proxy-agent >/dev/null 2>&1 || true
  systemctl disable data-proxy-agent >/dev/null 2>&1 || true
fi
