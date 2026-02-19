#!/bin/bash
# 导入 cookie 字符串到 JSON 文件
# 用法:
#   ./scripts/import-cookie.sh netease "MUSIC_U=xxx; __csrf=yyy; ..."
#   ./scripts/import-cookie.sh qq "qqmusic_key=xxx; Q_H_L=yyy; ..."
#
# 也可以从环境变量导入:
#   NETEASE_COOKIE="..." ./scripts/import-cookie.sh netease
#   QQ_COOKIE="..." ./scripts/import-cookie.sh qq

set -e

PROVIDER="${1:-}"
COOKIE_STR="${2:-}"
DATA_DIR="${PIBUDDY_DATA_DIR:-$HOME/.pibuddy}"

if [ -z "$PROVIDER" ]; then
    echo "用法: $0 <netease|qq> [cookie字符串]"
    echo ""
    echo "示例:"
    echo "  $0 netease \"MUSIC_U=xxx; __csrf=yyy\""
    echo "  $0 qq \"qqmusic_key=xxx; Q_H_L=yyy\""
    echo ""
    echo "也可从环境变量导入:"
    echo "  NETEASE_COOKIE=\"...\" $0 netease"
    echo "  QQ_COOKIE=\"...\" $0 qq"
    exit 1
fi

case "$PROVIDER" in
    netease)
        COOKIE_FILE="netease_cookie.json"
        [ -z "$COOKIE_STR" ] && COOKIE_STR="${NETEASE_COOKIE:-}"
        ;;
    qq)
        COOKIE_FILE="qq_cookie.json"
        [ -z "$COOKIE_STR" ] && COOKIE_STR="${QQ_COOKIE:-}"
        ;;
    *)
        echo "不支持的 provider: $PROVIDER (仅支持 netease, qq)"
        exit 1
        ;;
esac

if [ -z "$COOKIE_STR" ]; then
    echo "请提供 cookie 字符串（作为第二个参数或通过环境变量）"
    exit 1
fi

mkdir -p "$DATA_DIR"

# 使用 python3 解析 cookie 字符串并写入 JSON
python3 -c "
import json, sys, datetime

cookie_str = sys.argv[1]
cookies = []
for pair in cookie_str.split('; '):
    pair = pair.strip()
    if '=' in pair:
        name, value = pair.split('=', 1)
        cookies.append({'Name': name.strip(), 'Value': value.strip()})

data = {
    'cookies': cookies,
    'logged_in': True,
    'user': '$PROVIDER',
    'updated_at': datetime.datetime.now().isoformat()
}

path = '$DATA_DIR/$COOKIE_FILE'
with open(path, 'w') as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
print(f'✓ 写入 {len(cookies)} 个 cookie 到 {path}')
" "$COOKIE_STR"
