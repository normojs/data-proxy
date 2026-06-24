#!/usr/bin/env sh
set -eu

if command -v useradd >/dev/null 2>&1 && ! id data-proxy-agent >/dev/null 2>&1; then
  useradd --system --home-dir /var/lib/data-proxy-agent --shell /usr/sbin/nologin data-proxy-agent || true
fi

mkdir -p /etc/data-proxy-agent /var/lib/data-proxy-agent /var/log/data-proxy-agent
chown -R data-proxy-agent:data-proxy-agent /var/lib/data-proxy-agent /var/log/data-proxy-agent 2>/dev/null || true
chmod 700 /etc/data-proxy-agent /var/lib/data-proxy-agent /var/log/data-proxy-agent

if [ ! -f /etc/data-proxy-agent/config.yaml ]; then
  cat > /etc/data-proxy-agent/config.yaml <<'EOF'
server:
  base_url: ""
agent:
  token_env: DATA_PROXY_API_KEY
  workspace: /var/lib/data-proxy-agent
policy:
  default_permission: read_only
logging:
  level: info
  local_audit_jsonl: /var/log/data-proxy-agent/audit.jsonl
runtime:
  reconnect: true
EOF
  chmod 600 /etc/data-proxy-agent/config.yaml
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi
