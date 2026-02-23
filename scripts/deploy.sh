#!/bin/bash
# 部署脚本 - 将 pibuddy 部署到 /opt/pibuddy
# 用法: ./scripts/deploy.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="/opt/pibuddy"

echo "=== 创建安装目录 ==="
sudo mkdir -p ${INSTALL_DIR}/{configs,models,logs}

echo "=== 复制文件 ==="
# 复制二进制文件
sudo cp ${SCRIPT_DIR}/bin/pibuddy ${INSTALL_DIR}/

# 复制配置文件
sudo cp -r ${SCRIPT_DIR}/configs/* ${INSTALL_DIR}/configs/

# 复制数据文件
sudo cp -r ${SCRIPT_DIR}/models/* ${INSTALL_DIR}/models/

# 设置权限
sudo chown -R pi:pi ${INSTALL_DIR}
sudo chmod +x ${INSTALL_DIR}/pibuddy

echo "=== 安装 systemd 服务 ==="
sudo cp ${SCRIPT_DIR}/scripts/pibuddy.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable pibuddy

echo "=== 部署完成 ==="
echo "启动服务: sudo systemctl start pibuddy"
echo "查看状态: sudo systemctl status pibuddy"
echo "查看日志: journalctl -u pibuddy -f"
