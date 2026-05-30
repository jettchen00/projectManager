#!/usr/bin/env bash
# 启动 projectManager 服务（后台守护方式）。
# - 工作目录固定为项目根，保证 etc/、web/、log/ 等相对路径生效
# - 通过 CONFIG_FILE 指定配置文件，环境变量可覆盖配置项（参考 etc/config.yaml）
# - PID 写入 run/projectManagerSvr.pid，stdout/stderr 重定向到 log/stdout.log
set -euo pipefail

# 1. 定位项目根目录（脚本所在目录的上一级）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_HOME="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${APP_HOME}"

APP_NAME="projectManagerSvr"
BIN="${APP_HOME}/bin/${APP_NAME}"
CONFIG_FILE="${CONFIG_FILE:-${APP_HOME}/etc/config.yaml}"
PID_DIR="${APP_HOME}/run"
PID_FILE="${PID_DIR}/${APP_NAME}.pid"
LOG_DIR="${APP_HOME}/log"
STDOUT_LOG="${LOG_DIR}/stdout.log"

mkdir -p "${PID_DIR}" "${LOG_DIR}"

# 2. 二进制不存在
if [[ ! -x "${BIN}" ]]; then
  echo "[start] binary not found..."
  exit 1
fi

# 3. 检查是否已在运行
if [[ -f "${PID_FILE}" ]]; then
  OLD_PID="$(cat "${PID_FILE}" 2>/dev/null || true)"
  if [[ -n "${OLD_PID}" ]] && kill -0 "${OLD_PID}" 2>/dev/null; then
    echo "[start] ${APP_NAME} already running, pid=${OLD_PID}"
    exit 0
  fi
  rm -f "${PID_FILE}"
fi

# 4. 后台启动
export CONFIG_FILE
echo "[start] launching ${APP_NAME} (config=${CONFIG_FILE})"
nohup "${BIN}" >>"${STDOUT_LOG}" 2>&1 &
NEW_PID=$!
echo "${NEW_PID}" >"${PID_FILE}"

# 5. 短暂等待后确认存活
sleep 1
if kill -0 "${NEW_PID}" 2>/dev/null; then
  echo "[start] ${APP_NAME} started, pid=${NEW_PID}, log=${STDOUT_LOG}"
else
  echo "[start] ERROR: ${APP_NAME} exited right after launch, see ${STDOUT_LOG}" >&2
  rm -f "${PID_FILE}"
  exit 1
fi
