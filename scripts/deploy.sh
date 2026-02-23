#!/bin/bash
# 部署脚本 - 在当前目录部署 pibuddy
# 用法: ./scripts/deploy.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="${SCRIPT_DIR}"

echo "=== 部署目录: ${INSTALL_DIR} ==="

# 创建必要目录
mkdir -p ${INSTALL_DIR}/logs

# 设置权限
chmod +x ${INSTALL_DIR}/bin/pibuddy

echo "=== 安装 systemd 服务 ==="
sudo cp ${SCRIPT_DIR}/scripts/pibuddy.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable pibuddy

echo "=== 部署完成 ==="
echo "启动服务: sudo systemctl start pibuddy"
echo "查看状态: sudo systemctl status pibuddy"
echo "查看日志: journalctl -u pibuddy -f"
