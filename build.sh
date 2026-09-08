#!/usr/bin/env bash
# Z-Image 工作台一键构建（Linux / macOS）
# 用法: ./build.sh            构建当前平台
#       ./build.sh all        同时交叉编译 Windows / Linux / macOS (amd64)
set -e

cd "$(dirname "$0")"

echo "==> 构建前端"
cd web
if [ ! -d node_modules ]; then
  npm install
fi
npm run build
cd ..

echo "==> 编译后端"
go build -o zimage-webui .
echo "完成: ./zimage-webui"

if [ "$1" = "all" ]; then
  echo "==> 交叉编译多平台"
  GOOS=windows GOARCH=amd64 go build -o dist/zimage-webui-windows-amd64.exe .
  GOOS=linux   GOARCH=amd64 go build -o dist/zimage-webui-linux-amd64 .
  GOOS=darwin  GOARCH=arm64 go build -o dist/zimage-webui-darwin-arm64 .
  GOOS=darwin  GOARCH=amd64 go build -o dist/zimage-webui-darwin-amd64 .
  echo "完成: 产物在 dist/ 目录"
fi
