#!/bin/bash
set -e

echo "=== 1. 安装 Go ==="
cd /tmp
curl -sLO https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
export PATH=$PATH:/usr/local/go/bin
go version

echo ""
echo "=== 2. 确认 Node.js ==="
node --version
npm --version

echo ""
echo "=== 3. 重装前端依赖（Linux 版）==="
cd /mnt/d/goagentpro/web
rm -rf node_modules
npm install

echo ""
echo "=== 4. 构建前端 ==="
npm run build

echo ""
echo "=== 5. 检查后端编译 ==="
cd /mnt/d/goagentpro
go build ./cmd/server

echo ""
echo "==================================="
echo "设置完成！启动方式："
echo "  后端: go run ./cmd/server"
echo "  前端: cd web && npm run dev"
echo "==================================="
