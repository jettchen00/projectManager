#!/usr/bin/env bash
# 停止 projectManager 服务（优雅退出，超时后强杀）。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_HOME="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${APP_HOME}"

APP_NAME="projectManagerSvr"
PID_FILE="${APP_HOME}/run/${APP_NAME}.pid"
# 优雅退出最大等待秒数：略大于 config.yaml 的 shutdown_timeout_seconds
GRACEFUL_TIMEOUT="${GRACEFUL_TIMEOUT:-15}"

# 1. 取 PID（优先 pid 文件，回退 pgrep）
PID=""
if [[ -f "${PID_FILE}" ]]; then
  PID="$(cat "${PID_FILE}" 2>/dev/null || true)"
fi
if [[ -z "${PID}" ]] || ! kill -0 "${PID}" 2>/dev/null; then
  PID="$(pgrep -f "bin/${APP_NAME}" | head -n1 || true)"
fi

if [[ -z "${PID}" ]]; then
  echo "[stop] ${APP_NAME} is not running"
  rm -f "${PID_FILE}"
  exit 0
fi

# 2. SIGTERM 触发优雅退出
echo "[stop] sending SIGTERM to ${APP_NAME}, pid=${PID}"
kill -TERM "${PID}" 2>/dev/null || true

# 3. 轮询等待退出
for ((i = 0; i < GRACEFUL_TIMEOUT; i++)); do
  if ! kill -0 "${PID}" 2>/dev/null; then
    echo "[stop] ${APP_NAME} stopped gracefully"
    rm -f "${PID_FILE}"
    exit 0
  fi
  sleep 1
done

# 4. 超时强杀
echo "[stop] graceful timeout (${GRACEFUL_TIMEOUT}s), sending SIGKILL"
kill -KILL "${PID}" 2>/dev/null || true
sleep 1
if kill -0 "${PID}" 2>/dev/null; then
  echo "[stop] ERROR: failed to kill pid=${PID}" >&2
  exit 1
fi
rm -f "${PID_FILE}"
echo "[stop] ${APP_NAME} killed"
