#!/usr/bin/env bash
# 交叉编译 shiling 为 linux/amd64 并打包（含 config/web/skills/systemd 脚本）
set -euo pipefail
cd "$(dirname "$0")"

APP=shiling
OUT=dist/${APP}-linux-amd64
STAMP=$(date +%Y%m%d-%H%M%S)

rm -rf "$OUT"
mkdir -p "$OUT"

echo "==> 交叉编译 ${APP} (linux/amd64, 静态)"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "${OUT}/${APP}" .

echo "==> 拷贝附属文件"
cp config.json "$OUT/"
cp -r web      "$OUT/"
cp -r skills   "$OUT/"
cp deploy/process/shiling.service "$OUT/"
cp deploy/process/install.sh      "$OUT/"

echo "==> 生成 API Key 环境文件模板"
if [ ! -f "$OUT/env.sh" ]; then
  cat > "$OUT/env.sh" <<'EOF'
# 在此填写 API Key（一行一个，KEY=value，不要加 export）
HY_API_KEY=你的key
# DEEPSEEK_API_KEY=
# OPENAI_API_KEY=
# ANTHROPIC_API_KEY=
# MOONSHOT_API_KEY=
# DASHSCOPE_API_KEY=
EOF
fi

echo "==> 打包"
TGZ=dist/${APP}-linux-amd64-${STAMP}.tar.gz
tar czf "$TGZ" -C "$OUT" .
ls -lh "$TGZ"
echo ""
echo "✅ 完成。上传到服务器后执行："
echo "   scp $TGZ root@<服务器IP>:/root/"
echo "   ssh root@<服务器IP> 'tar xzf ${TGZ##*/} && bash install.sh'"
