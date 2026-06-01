FROM node:22-alpine AS builder

WORKDIR /build

# 先装依赖
COPY web/package.json web/package-lock.json ./
RUN npm ci

# 构建前端
COPY web/ ./
RUN npm run build

# ── 运行阶段 ──
FROM nginx:latest

COPY --from=builder /build/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
