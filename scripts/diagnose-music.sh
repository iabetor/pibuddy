#!/bin/bash
# PiBuddy 音乐播放链路诊断脚本
# 用法：bash scripts/diagnose-music.sh

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

ok()   { echo -e "${GREEN}[OK]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
info() { echo -e "  → $1"; }

QQ_API="http://localhost:3300"

echo "========================================="
echo "  PiBuddy 音乐播放链路诊断"
echo "========================================="
echo ""

# ─── 1. 检查 QQ 音乐 API 服务 ───
echo "【1】检查 QQ 音乐 API 服务"

if systemctl is-active --quiet qqmusic-api 2>/dev/null; then
    ok "qqmusic-api 服务正在运行"
else
    fail "qqmusic-api 服务未运行"
    info "尝试启动: sudo systemctl start qqmusic-api"
    info "查看日志: journalctl -u qqmusic-api -n 20"
fi

# 检查端口
if ss -tlnp 2>/dev/null | grep -q ':3300'; then
    ok "端口 3300 正在监听"
else
    fail "端口 3300 未监听"
    info "QQ 音乐 API 可能未正确启动"
fi

echo ""

# ─── 2. 测试 QQ 音乐 API 搜索接口 ───
echo "【2】测试搜索接口"

SEARCH_RESP=$(curl -s --max-time 10 "${QQ_API}/search?key=%E5%91%A8%E6%9D%B0%E4%BC%A6%20%E9%9D%92%E8%8A%B1%E7%93%B7&pageSize=3" 2>&1) || true

if [ -z "$SEARCH_RESP" ]; then
    fail "搜索接口无响应（服务可能未运行）"
    echo ""
    echo "请先确保 QQ 音乐 API 服务正常运行后再重新执行此脚本。"
    exit 1
fi

RESULT_CODE=$(echo "$SEARCH_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('result',''))" 2>/dev/null || echo "parse_error")

if [ "$RESULT_CODE" = "100" ]; then
    ok "搜索接口正常 (result=100)"
    # 提取第一首歌信息
    FIRST_SONG=$(echo "$SEARCH_RESP" | python3 -c "
import sys, json
d = json.load(sys.stdin)
items = d.get('data',{}).get('list',[])
if items:
    s = items[0]
    artists = '/'.join([x['name'] for x in s.get('singer',[])])
    print(f\"  歌曲: {s['songname']} - {artists}\")
    print(f\"  SongID: {s['songid']}\")
    print(f\"  SongMID: {s['songmid']}\")
    print(f\"  StrMediaMid: {s.get('strMediaMid','')}\")
else:
    print('  无搜索结果')
" 2>/dev/null || echo "  (解析失败)")
    echo "$FIRST_SONG"
else
    fail "搜索接口返回异常: result=$RESULT_CODE"
    info "完整响应: $(echo "$SEARCH_RESP" | head -c 500)"
    
    if [ "$RESULT_CODE" = "300" ] || [ "$RESULT_CODE" = "301" ]; then
        warn "可能需要登录 QQ 音乐 Cookie"
    fi
fi

echo ""

# ─── 3. 测试获取歌曲 URL 接口 ───
echo "【3】测试获取歌曲 URL 接口"

# 从搜索结果提取 songmid
SONG_MID=$(echo "$SEARCH_RESP" | python3 -c "
import sys, json
d = json.load(sys.stdin)
items = d.get('data',{}).get('list',[])
if items:
    print(items[0].get('songmid',''))
" 2>/dev/null || echo "")

if [ -z "$SONG_MID" ]; then
    warn "无法从搜索结果提取 SongMID，跳过 URL 测试"
else
    URL_RESP=$(curl -s --max-time 10 "${QQ_API}/song/url?id=${SONG_MID}&mediaId=${SONG_MID}" 2>&1) || true
    
    URL_RESULT=$(echo "$URL_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('result',''))" 2>/dev/null || echo "parse_error")
    URL_DATA=$(echo "$URL_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',''))" 2>/dev/null || echo "")
    
    if [ "$URL_RESULT" = "100" ] && [ -n "$URL_DATA" ] && [ "$URL_DATA" != "" ] && [ "$URL_DATA" != "None" ]; then
        ok "获取播放 URL 成功"
        info "URL: $(echo "$URL_DATA" | head -c 120)..."
        
        # 测试 URL 是否可访问
        HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "$URL_DATA" 2>/dev/null || echo "000")
        if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "206" ]; then
            ok "播放 URL 可访问 (HTTP $HTTP_CODE)"
        else
            fail "播放 URL 不可访问 (HTTP $HTTP_CODE)"
            info "可能是 Cookie 过期或 IP 受限"
        fi
    else
        fail "获取播放 URL 失败: result=$URL_RESULT, data=$URL_DATA"
        info "完整响应: $(echo "$URL_RESP" | head -c 500)"
        warn "很可能是 QQ 音乐 Cookie 过期，需要重新登录"
    fi
fi

echo ""

# ─── 4. 检查 QQ 音乐 Cookie ───
echo "【4】检查 QQ 音乐 Cookie"

PIBUDDY_DATA_DIR="$HOME/.pibuddy"
COOKIE_FILE="$PIBUDDY_DATA_DIR/qq_cookie.json"

if [ -f "$COOKIE_FILE" ]; then
    ok "Cookie 文件存在: $COOKIE_FILE"
    
    COOKIE_AGE=$(python3 -c "
import json, datetime
with open('$COOKIE_FILE') as f:
    d = json.load(f)
updated = d.get('updated_at','')
if updated:
    try:
        t = datetime.datetime.fromisoformat(updated.replace('Z','+00:00'))
        age = datetime.datetime.now(datetime.timezone.utc) - t
        hours = age.total_seconds() / 3600
        print(f'{hours:.1f} 小时前更新')
        if hours > 72:
            print('EXPIRED')
    except:
        print('解析时间失败')
else:
    print('无 updated_at 字段')
" 2>/dev/null || echo "解析失败")
    
    info "Cookie: $COOKIE_AGE"
    
    if echo "$COOKIE_AGE" | grep -q "EXPIRED"; then
        fail "Cookie 已过期（超过 72 小时）"
        warn "请在树莓派上运行: pibuddy-music qq login --web"
    fi
else
    fail "Cookie 文件不存在: $COOKIE_FILE"
    warn "请在树莓派上运行: pibuddy-music qq login --web"
    warn "或: cd /data/workspace/pibuddy && go run cmd/music/main.go qq login --web"
fi

echo ""

# ─── 5. 检查 PiBuddy 配置 ───
echo "【5】检查 PiBuddy 音乐配置"

PIBUDDY_DIR="/data/workspace/pibuddy"
CONFIG="$PIBUDDY_DIR/configs/pibuddy.yaml"

if [ -f "$CONFIG" ]; then
    MUSIC_ENABLED=$(grep -A1 "music:" "$CONFIG" | grep "enabled" | awk '{print $2}' | head -1)
    MUSIC_PROVIDER=$(grep -A3 "music:" "$CONFIG" | grep "provider" | awk '{print $2}' | tr -d '"' | head -1)
    QQ_URL=$(grep -A2 "qq:" "$CONFIG" | grep "api_url" | awk '{print $2}' | tr -d '"' | head -1)
    
    info "music.enabled: $MUSIC_ENABLED"
    info "music.provider: $MUSIC_PROVIDER"
    info "music.qq.api_url: $QQ_URL"
    
    if [ "$MUSIC_ENABLED" = "true" ] && [ "$MUSIC_PROVIDER" = "qq" ] && [ "$QQ_URL" = "http://localhost:3300" ]; then
        ok "配置正确"
    else
        warn "请检查配置"
    fi
else
    fail "配置文件不存在: $CONFIG"
fi

echo ""

# ─── 总结 ───
echo "========================================="
echo "  诊断完成"
echo "========================================="
echo ""
echo "如果搜索正常但获取 URL 失败，说明 Cookie 过期。修复方法："
echo ""
echo "  方法 1（推荐）：在电脑上访问 QQ 音乐 API 的 Web 登录页面"
echo "    在浏览器打开: http://192.168.3.101:3300/login"
echo "    扫码登录后 Cookie 会自动保存到 QQ 音乐 API 服务"
echo ""
echo "  方法 2：使用 pibuddy-music 工具登录"
echo "    cd /data/workspace/pibuddy"
echo "    go run cmd/music/main.go qq login --web"
echo ""
echo "如果搜索也失败（result!=100），说明 QQ 音乐 API 服务有问题。检查："
echo "    sudo systemctl status qqmusic-api"
echo "    journalctl -u qqmusic-api -n 30"
