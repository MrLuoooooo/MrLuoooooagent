.PHONY: build test run clean ollama \
        build-docker up logs down restart health push

# ── Local development ────────────────────────────────────

build:
	mkdir -p bin
	go build -ldflags="-s -w" -o bin/goagent ./cmd/server

test:
	go test ./... -v -count=1

run: build
	./bin/goagent

clean:
	rm -rf bin/

# ── Ollama 一键本地运行 ────────────────────────────────

# 一键启动 ES + 应用（Ollama 自动检测）
ollama:
	@echo "=== 启动 ES ==="
	docker rm -f goagent-es 2>/dev/null; true
	docker run -d --name goagent-es -p 9200:9200 \
		-e "discovery.type=single-node" \
		-e "xpack.security.enabled=false" \
		-e "ES_JAVA_OPTS=-Xms512m -Xmx512m" \
		docker.elastic.co/elasticsearch/elasticsearch:8.14.0
	@echo "=== 等待 ES 就绪 ==="
	@until curl -s http://localhost:9200 >/dev/null 2>&1; do sleep 2; done
	@echo "=== ES 就绪，启动应用 ==="
	go run ./cmd/server

# ── Docker ───────────────────────────────────────────────

build-docker:
	docker compose build

up:
	docker compose up -d

logs:
	docker compose logs -f

down:
	docker compose down -v

restart: down build-docker up

health:
	curl -s http://localhost:8080/health | python3 -m json.tool 2>/dev/null || curl -s http://localhost:8080/health

IMAGE ?= yourusername/goagent-pro
# Example: make push IMAGE=registry.example.com/yourname/goagent-pro
push: build-docker
	docker tag goagentpro-app:latest $(IMAGE):latest
	docker push $(IMAGE):latest
