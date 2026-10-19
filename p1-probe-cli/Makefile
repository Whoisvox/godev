# probe-cli Makefile
# 用法: make help 查看所有目标

# ---------- 可配置变量（命令行可覆盖: make build VERSION=v1.2.3） ----------
BINARY    := probe-cli
CMD_DIR   := ./cmd/probe-cli
BIN_DIR   := bin
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_AT  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# 镜像仓库：集群节点通过 192.168.5.174:80 拉取（不要用 localhost）
REGISTRY  ?= 192.168.5.174:80/godev
IMAGE     := $(REGISTRY)/$(BINARY):$(VERSION)

# 版本信息注入到二进制（需在 main.go 定义同名变量，见 v1 任务）
LDFLAGS   := -s -w \
             -X main.version=$(VERSION) \
             -X main.commit=$(COMMIT) \
             -X main.buildTime=$(BUILD_AT)

GOFLAGS   := -trimpath

# ---------- 默认目标 ----------
.DEFAULT_GOAL := help

.PHONY: help build install run test race cover lint fmt vet tidy clean docker push k8s targets1000

help: ## 显示所有可用目标
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

# ---------- 构建 ----------
build: ## 编译 probe-cli + loadgen 到 bin/
	@mkdir -p $(BIN_DIR)
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD_DIR)
	go build $(GOFLAGS) -o $(BIN_DIR)/loadgen ./cmd/loadgen
	@echo "OK -> $(BIN_DIR)/$(BINARY) + $(BIN_DIR)/loadgen ($(VERSION) $(COMMIT))"

targets1000: ## 生成 1000 目标压测配置 targets1000.yaml（生成物，不入库）
	@{ echo "targets:"; \
	   seq 1 1000 | awk '{printf "  - name: target-%d\n    url: http://localhost:18081/target/%d\n", $$1, $$1}'; \
	 } > targets1000.yaml
	@echo "OK -> targets1000.yaml ($$(grep -c 'url:' targets1000.yaml) targets)"

install: ## 安装到 GOBIN
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" $(CMD_DIR)

run: build ## 编译并执行一次探测
	./$(BIN_DIR)/$(BINARY) run -c targets.yaml

# ---------- 质量 ----------
test: ## 单元测试
	go test ./...

race: ## 带竞态检测的测试（并发代码必须过）
	go test -race ./...

cover: ## 测试覆盖率
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint: ## golangci-lint 检查
	golangci-lint run ./...

fmt: ## 格式化
	gofmt -s -w .

vet: ## go vet 静态检查
	go vet ./...

tidy: ## 整理依赖
	go mod tidy

# ---------- 容器（v4 起可用，需先写 Dockerfile） ----------
docker: ## 构建镜像
	@test -f Dockerfile || { echo "ERROR: 缺少 Dockerfile（v4 任务）"; exit 1; }
	docker build -t $(IMAGE) .
	@echo "OK -> $(IMAGE)"

push: docker ## 推送到 Harbor
	docker push $(IMAGE)

k8s: push ## 推送后 apply 清单
	@test -d deploy || { echo "ERROR: 缺少 deploy/ 目录"; exit 1; }
	kubectl apply -f deploy/

# ---------- 清理 ----------
clean: ## 删除构建产物
	rm -rf $(BIN_DIR) coverage.out
