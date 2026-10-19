# probe-cli

批量 HTTP 探测工具：并发探测一组目标，输出 JSON 报告。Go 学习项目 P1（并发编程 + 工程化实践）。

```
[{"name":"httpbin-get","url":"https://httpbin.org/get","status_code":200,"latency":"1.31s"},
 {"name":"not-exist","url":"https://youcantfindme.com","latency":"17ms","err":"Get \"...\": dial tcp: lookup ... no such host"}]
```

## 功能

- **并发探测**：worker pool 模型，并发度可调（`-n`），避免为目标数开等量 goroutine
- **每请求超时**：context.WithTimeout 驱动，可配置（`-t`），超时立即中断请求
- **优雅退出**：捕获 SIGINT（Ctrl-C）/ SIGTERM，取消全部在途探测并输出已完成部分，不粗暴退出
- **失败重试**：指数退避（100ms → 200ms → 400ms...），仅对暂时性错误重试；退避等待可被取消信号打断
- **JSON 报告**：写 stdout（错误信息走 stderr，互不污染，可管道处理），latency 为人类可读格式

### 重试策略

| 情况 | 是否重试 | 原因 |
|------|---------|------|
| HTTP 5xx / 429 | ✅ | 服务端暂时性问题，可能快速恢复 |
| 网络超时（非 ctx） | ✅ | 可能是瞬时拥塞 |
| DNS 解析失败 | ❌ | 域名不存在，短期内重试无意义 |
| ctx 取消 / 超时 | ❌ | context 已死，重试必然失败 |
| HTTP 4xx（除 429） | ❌ | 请求本身的问题，重试无用 |

注：HTTP 状态码（含 5xx）在 Go 中 `client.Do` 返回 `err == nil`，只有网络层失败才有 error——`isRetryable` 据此分成两个互斥分支处理。

## 用法

```bash
make build
./bin/probe-cli run -c targets.yaml            # 默认：并发10、超时3s、重试2次
./bin/probe-cli run -c targets.yaml -n 20 -t 1s -N 3
```

| Flag | 说明 | 默认 |
|------|------|------|
| `-c, --config` | YAML 配置文件路径 | targets.yaml |
| `-n, --concurrency` | worker 数量（>= 1） | 10 |
| `-t, --timeout` | 单请求超时，**必须带单位**（`2s`、`500ms`，写 `2` 会报错） | 3s |
| `-N, --retries` | 最大重试次数（总尝试 = N+1） | 2 |

配置文件：

```yaml
targets:
  - name: httpbin-get
    url: https://httpbin.org/get
  - name: slow-2s
    url: https://httpbin.org/delay/2
```

校验 fail-fast：文件不存在/为空、targets 为空或超过 1024 条、flag 非法，均立即报错退出（exit 1）。

Docker（scratch 基础镜像，约 7MB，含 CA 证书以支持 HTTPS）：

```bash
make push    # 构建并推送到 192.168.5.174:80/godev/probe-cli
docker run --rm -v $PWD/targets.yaml:/targets.yaml:ro \
  192.168.5.174:80/godev/probe-cli:<tag> run -c /targets.yaml -n 5 -t 3s
# 注意：容器内 127.0.0.1 不是宿主机，探测宿主机服务需用宿主机 IP 或 --network host
```

## 架构

```
                    ┌─ worker-1 ─┐
 targets.yaml ──> jobs chan ──┼─ worker-2 ──┼──> results chan ──> collector ──> JSON stdout
   (Load+校验)     (buffered)  └─ worker-N ─┘      (buffered)      (主 goroutine)
                        │
              ctx 树：signal.NotifyContext(SIGINT/SIGTERM)
                        └─> 每请求 WithTimeout 子 ctx
                            取消时：worker 停领新任务，在途请求立即中断
```

- 分发：所有 target 先写入 buffered jobs channel，随后 close —— worker 的 `for range jobs` 取空后自然退出
- 收集：worker 将结果发往 results channel；单独 goroutine `wg.Wait()` 后 close(results)，collector `for range` 收齐
- 并发安全：worker 之间零共享内存，全部通过 channel 交接（fan-out/fan-in），`go test -race` 验证
- 结果**无序**：靠 Response 自带的 name/url 认领，不依赖完成顺序
- 连接复用：全局单个 `http.Client`，keep-alive 生效

## 实测数据

28 个目标（httpbin 系 + 各类异常目标），本机 2026-10 实测，仅供参考：

并发扩展性（timeout=3s）：

| -n | 总耗时 | 加速比 |
|----|--------|--------|
| 1 | 33.0s | 1× |
| 5 | 8.7s | 3.8× |
| 10 | 4.9s | 6.7× |
| 28 | 3.4s | 9.8× |

加速比低于并发度：受最慢任务制约；n=28 时耗时地板 ≈ 单个最慢请求（3s 超时），加 worker 不再有效。

超时对耗时地板的影响（n=10）：`-t 5s` → 7.1s，`-t 3s` → 5.3s，`-t 1s` → 3.3s。总时长下限由最慢请求的超时决定。

## 测试

```bash
make test     # 单元 + 集成测试
make race     # 竞态检测（并发代码必须零警告）
make cover    # 覆盖率
```

- 单元测试：`isRetryable` 表驱动（DNS/ctx canceled/deadline/4xx/5xx/429）
- 集成测试：`httptest.NewServer` 构造确定性场景——503×2 后收敛（断言 attempts=3）、503 打满放弃（attempts=N+1）、404 不重试（attempts=1）、超时不重试、`All` 端到端（len/name/status/attempts 全断言）
- 每个场景独立 server + 随机端口，无 order-dependent 问题；断言包含**请求计数**，防止"不重试也能通过"的假测试

## 开发

```bash
make help     # 全部目标
make lint     # golangci-lint
make docker   # 构建镜像（多阶段：golang:alpine 构建 → scratch 运行）
```

目录结构：

```
cmd/probe-cli/      # CLI 入口（cobra）
internal/probe/     # 探测核心：Probe / ProbeWithRetry / All（worker pool）
internal/config/    # YAML 配置加载与校验
```

## TODO

- [ ] TCP 端口探测（当前仅 HTTP）
- [ ] daemon 模式：周期性探测
- [ ] Prometheus 指标暴露（P2 项目衔接）
- [ ] BaseDelay 做成 flag（当前固定 100ms）
- [ ] 结果按配置顺序排序输出（当前为完成顺序）
