#!/usr/bin/env bash
# 重启 projectManager 服务：先 stop 再 start。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

bash "${SCRIPT_DIR}/stop.sh"
bash "${SCRIPT_DIR}/start.sh"
