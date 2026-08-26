#!/usr/bin/env bash
# shiling 一键安装到 /opt/shiling 并以 systemd 托管
# 用法: bash install.sh
set -euo pipefail

APP=shiling
INSTALL_DIR=/opt/shiling
SRC_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==> 安装目录: ${INSTALL_DIR}"
mkdir -p "$INSTALL_DIR"
cp -f "$SRC_DIR/$APP"        "$INSTALL_DIR/"
cp -rf "$SRC_DIR/web"        "$INSTALL_DIR/"
cp -rf "$SRC_DIR/skills"     "$INSTALL_DIR/"
cp -f "$SRC_DIR/config.json" "$INSTALL_DIR/"

# 环境文件：若已存在则保留，否则用模板
if [ ! -f "$INSTALL_DIR/env.sh" ]; then
  cp -f "$SRC_DIR/env.sh" "$INSTALL_DIR/env.sh"
  echo "⚠️  请先编辑 ${INSTALL_DIR}/env.sh 填入 API Key，然后执行: systemctl restart ${APP}"
fi

chmod +x "$INSTALL_DIR/$APP"
chmod 600 "$INSTALL_DIR/env.sh" 2>/dev/null || true

echo "==> 安装 systemd 服务"
cp -f "$SRC_DIR/shiling.service" /etc/systemd/system/shiling.service
systemctl daemon-reload
systemctl enable --now shiling || true

echo "==> 服务状态"
systemctl status shiling --no-pager || true
echo ""
echo "📌 下一步："
echo "  1. 编辑 API Key:  vim /opt/shiling/env.sh"
echo "  2. 重启服务:      systemctl restart shiling"
echo "  3. 健康检查:      curl http://127.0.0.1:8082/healthz"
echo "  4. 看日志:        journalctl -u shiling -f"
