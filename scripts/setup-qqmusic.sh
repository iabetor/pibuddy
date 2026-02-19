#!/bin/bash

# QQAFLMusicApi 部署脚本
# 用于安装和配置 QQ 音乐 API 服务

set -e

REPO_URL="https://github.com/QiuChenlyOpenSource/QQAFLMusicApi.git"
INSTALL_DIR="${1:-$HOME/QQAFLMusicApi}"
PORT="${2:-3300}"
NPM_REGISTRY="https://registry.npmmirror.com"

echo "=== QQAFLMusicApi 部署脚本 ==="
echo "安装目录: $INSTALL_DIR"
echo "服务端口: $PORT"
echo "NPM 源: $NPM_REGISTRY"
echo ""

# 检查 Node.js 是否安装
if ! command -v node &> /dev/null; then
    echo "错误: 未找到 Node.js，请先安装 Node.js (https://nodejs.org/)"
    exit 1
fi

echo "Node.js 版本: $(node -v)"
echo "NPM 版本: $(npm -v)"
echo ""

# 设置 npm 镜像源
echo "配置 npm 镜像源..."
npm config set registry "$NPM_REGISTRY"
echo "当前 npm 源: $(npm config get registry)"
echo ""

# 克隆仓库
if [ -d "$INSTALL_DIR" ]; then
    echo "目录已存在: $INSTALL_DIR"
    read -p "是否删除并重新克隆? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "删除现有目录..."
        rm -rf "$INSTALL_DIR"
        echo "克隆 QQAFLMusicApi 仓库..."
        git clone "$REPO_URL" "$INSTALL_DIR"
        cd "$INSTALL_DIR"
    else
        echo "使用现有目录"
        cd "$INSTALL_DIR"
    fi
else
    echo "克隆 QQAFLMusicApi 仓库..."
    git clone "$REPO_URL" "$INSTALL_DIR"
    cd "$INSTALL_DIR"
fi

# 安装依赖
echo ""
echo "安装依赖..."
npm install --registry="$NPM_REGISTRY"

# 配置端口
echo ""
echo "配置服务端口: $PORT"

# 创建启动脚本
cat > start.sh << EOF
#!/bin/bash
cd "$INSTALL_DIR"
export PORT=$PORT
node app.js
EOF
chmod +x start.sh

echo ""
echo "=== 部署完成 ==="
echo ""
echo "使用以下命令启动服务:"
echo "  cd $INSTALL_DIR && node app.js"
echo "  或"
echo "  $INSTALL_DIR/start.sh"
echo ""
echo "服务地址: http://localhost:$PORT"
echo ""
echo "PiBuddy 配置:"
echo "  在 configs/pibuddy.yaml 中设置:"
echo "  music:"
echo "    enabled: true"
echo "    provider: \"qq\""
echo "    qq:"
echo "      api_url: \"http://localhost:$PORT\""
echo ""

# 可选：创建 systemd 服务（仅 Linux）
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo ""
    read -p "是否创建 systemd 服务? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        SERVICE_FILE="/etc/systemd/system/qq-music-api.service"
        sudo tee "$SERVICE_FILE" > /dev/null << EOF
[Unit]
Description=QQAFLMusicApi
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$INSTALL_DIR
Environment="PORT=$PORT"
ExecStart=/usr/bin/node app.js
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

        sudo systemctl daemon-reload
        sudo systemctl enable qq-music-api
        echo ""
        echo "Systemd 服务已创建:"
        echo "  启动: sudo systemctl start qq-music-api"
        echo "  停止: sudo systemctl stop qq-music-api"
        echo "  状态: sudo systemctl status qq-music-api"
        echo "  日志: sudo journalctl -u qq-music-api -f"
    fi
fi

echo ""
echo "完成！"
