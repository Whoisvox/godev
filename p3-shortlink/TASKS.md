# P3 shortlink 任务卡

> 规则不变：**核心代码你自己写**，我只做讲解 / review / 补充功能性知识进 KNOWLEDGE.md。
> 每版做完在 `info.md` 记录你的实测输出 + 思考题答案，然后喊 review。

---

## 目标产物

一个能拿出手的短链接服务，重点是**三个可讲的故事**：
1. 一次完整的「压测 → pprof 定位 → 优化 → 数字对比」（简历/面试硬通货）
2. 异步点击落库的并发设计（channel + worker pool + 优雅退出排空，P1/P2 知识的集大成）
3. 缓存故障降级（Redis 挂了服务仍可用，混沌测试验证）

## 版本规划

| 版本 | 目标 | 验收 |
|------|------|------|
| **v1** | 存储层 + 两个 API + 配置 + 中间件 | 表驱动测试过、`-race` 过、lint 零告警、能 curl 出 302 |
| **v2** | 可观测性：`/metrics` + `/healthz` + `/readyz` + slog 请求日志 | Prometheus 抓到、指标语义正确、压测有基线数字 |
| **v3** | 异步点击落库（worker pool + 优雅排空） | 压测中 kill -TERM，点击零丢失；goroutine 无泄漏 |
| **v4** | Redis 缓存 + singleflight + 降级 | 混沌测试：Redis 挂掉（`kubectl -n default scale deploy redis-deployment --replicas=0` 或断网模拟）服务仍可用、恢复后自动回补 |
| **v5** | 令牌桶限流 + 性能优化故事 + Dockerfile + K8s | wrk 前后对比、pprof 火焰图、部署进 godev ns |

---

# v1：跑通存储与两个 API

## 前置知识

**已写进 KNOWLEDGE.md，动手前先读这三节**：
- 「短码生成」— 为什么选随机码 + UNIQUE 索引 + 1062 重试，而不是 hash 截断或发号器；301 vs 302
- 「database/sql 功能性事实」— 连接池四参数、`sql.Open` 不建连接、`ErrNoRows` 不是错误
- 「Go 1.22+ 标准库路由」+「http.Server 超时四件套」

**这版的新原理知识**（讲解，不进 md）：

### 分层：为什么要 `internal/` 分包

```
cmd/shortlink/main.go        ← 只做装配：读配置 → 建依赖 → 组路由 → 起服务 → 优雅退出
internal/config/             ← 配置加载 + fail fast 校验（P2 的套路原样搬）
internal/store/              ← 只管 SQL。定义 interface，不知道 HTTP 的存在
internal/server/             ← 只管 HTTP handler。通过 store.Store 接口访问数据，不知道是 MySQL
internal/codegen/            ← 短码生成（纯函数 + crypto/rand，最好测）
```

**核心原理：依赖倒置。** `server` 依赖 `store.Store` **接口**，不依赖 `*store.MySQLStore`。收益是立竿见影的：
- handler 单测用 `fakeStore`（内存 map）就能测，**不需要起 MySQL**，`go test` 秒级跑完
- v4 加缓存时，写一个 `CachingStore` 包住 `MySQLStore`（装饰器模式），server 层**一行都不用改**
- 这是 Go 里"接口在**消费方**定义，不在实现方"的经典实践：`Store` 接口应该定义在 server 包（谁用谁定义），而不是 store 包

反面教材（你 P2 撞过的耦合问题）：如果 handler 直接 `db.QueryRow(...)`，那每个测试都要真数据库，测试会变慢、变脆、变顺序依赖——正是 P1 那个 `cmd/server` 全局计数器的坑的放大版。

### 依赖注入 vs 全局变量

`main` 里手动 new 出 `store` 传进 `server.New(store, cfg)`。**不要用包级全局变量 `var DB *sql.DB`**——测试无法替换、无法并行、初始化顺序隐式。Go 没有 Spring 那套 IoC 容器，就是手写构造函数，这叫 "accept interfaces, return structs"。

## 任务清单

### 1. 环境：**用集群里现成的中间件**（不在本机再起 docker，174 不浪费资源）

你集群里已有（我核实过连通性，174 → NodePort 全通）：

| 服务 | 地址 | 说明 |
|------|------|------|
| MySQL 8.0.44 | `192.168.5.241:31554` | `default/mysql-service`，**共享实例**，`max_connections=151` |
| Redis 8.0.3 | `192.168.5.241:32523` | `default/redis-nodeport`，**standalone**，无密码 |
| ~~Redis 6.2.3~~ | ~~`:31639`~~ | `redis` ns 那个是 **cluster 模式**，go-redis 要用 ClusterClient，v4 不用它 |
| Kafka / Mongo | 需要时再查 | P3 用不到 |

**初始化（你执行，一次性）**：
```bash
cd /data/godev/p3-shortlink
# 先把 deploy/mysql-init.sql 里的 CHANGE_ME 改成你自己定的密码
mysql -h192.168.5.241 -P31554 -uroot -p < deploy/mysql-init.sql
# 验证应用账号能连（用你设的密码）
mysql -h192.168.5.241 -P31554 -ushortlink -p shortlink -e "SHOW TABLES;"
```
脚本会建独立库 `shortlink` + 独立账号（只授 `shortlink.*`，不碰别人的库），表和 P2 时设计的 schema 一致。

**⚠️ 共享实例纪律（写进 config 默认值）**：
- `MaxOpenConns = 10`（不是 25！算术见 KNOWLEDGE.md「共享实例上的池容量算术」）
- 压测别把 MySQL 打死——wrk 并发 ≤ 100，压完看一眼 `SHOW PROCESSLIST` 确认连接都释放了
- DSN 密码**不要提交进 git**：用环境变量注入（`SHORTLINK_DSN`），仓库里只留 `.env.example` 模板

（原来的 docker-compose.yml 已删。本机留着的唯一理由场景是"集群不可用时的离线开发"，你现在不需要。）

### 2. 自己写代码
按上面分层建目录。**v1 范围**（不要多做，点击统计留给 v3）：

- `internal/config`：加载（环境变量 or yaml，你选一种，另一种说明为什么不用）+ 校验：DSN 非空、监听地址合法、`ReadHeaderTimeout > 0` 等。非法 → `slog.Error` + `os.Exit(1)`（fail fast，P2 同款）
- `internal/codegen`：`func New(n int) (string, error)`，base62 + `crypto/rand`
- `internal/store`：
  - `type Store interface`（**定义在 server 包还是 store 包？你想清楚后在 info.md 说明理由**）
  - `MySQLStore` 实现 `Create(ctx, url) (code, error)` / `Get(ctx, code) (url, error)`
  - 1062 冲突重试放在 store 还是 handler？说理由
  - `Get` 找不到 → 返回**哨兵错误** `var ErrNotFound = errors.New("link not found")`，不要返回 `sql.ErrNoRows`（泄漏存储实现细节给上层）
- `internal/server`：
  - `POST /api/shorten`：body `{"url": "..."}` → `201 {"code":"abc123","short_url":"http://host/abc123"}`
  - `GET /{code}`：查库 → 302 跳转；查不到 → 404
  - **URL 校验**：必须 `http.ParseRequestURI` + 只允许 `http/https` scheme + 拒绝空 host。否则你的服务变成**开放重定向器**（钓鱼跳板，真实安全问题）
  - body 大小限制 `http.MaxBytesReader(w, r.Body, 4<<10)`
  - 错误响应统一 JSON `{"error":"..."}`，**不要把 err.Error() 原文返回给客户端**（泄漏内部信息），记日志里
- `cmd/shortlink/main.go`：装配 + `http.Server` 四超时 + `signal.NotifyContext` + `srv.Shutdown`（P2 main.go 直接改）
- `Makefile`：从 P2 拷过来改 `BINARY`/`CMD_DIR`

### 3. 测试（v1 的验收重点）
- `codegen`：表驱动 — 长度正确、字符集合法、两次不相等、n=0 报错
- `server`：用 **fake store**（内存 map + 可注入错误）+ `httptest.NewServer`，表驱动覆盖：正常创建、非法 URL、超长 body、跳转成功、404、store 报错 → 500
- `store`：真 MySQL 的集成测试，加 `//go:build integration` build tag（`go test -tags integration ./...`），别让它拖慢日常 `go test`
- **`go test -race ./...` 必过**

### 4. 手动验收
```bash
make run &
curl -s -XPOST localhost:8080/api/shorten -d '{"url":"https://example.com/a/b?c=1"}'
curl -sI localhost:8080/<code>          # 期望 302 + Location
curl -sI localhost:8080/zzzzzz          # 期望 404
curl -s -XPOST localhost:8080/api/shorten -d '{"url":"javascript:alert(1)"}'   # 期望 400
curl -s -XPUT  localhost:8080/api/shorten                                       # 期望 405 + Allow 头
```

## v1 思考题（写进 info.md，答不上来就说）

1. `sql.Open` 之后不 `PingContext` 直接提供服务，会发生什么？为什么 K8s 的 readinessProbe 会让这个问题更隐蔽？
2. `MaxOpenConns=25` 而 MySQL `max_connections=151`，如果这个服务扩到 6 个副本会发生什么？SRE 视角怎么算这个账？
3. 1062 冲突重试为什么要设**上限**？如果 62⁶ 快用满了，无限重试会怎样？
4. `Get` 里为什么返回自定义 `ErrNotFound` 而不是 `sql.ErrNoRows`？`errors.Is` 在这条链路上扮演什么角色？
5. handler 里用 `context.Background()` 代替 `r.Context()` 传给 store，功能上什么时候会出问题？

## 我不做什么

- 不给你 v1 的代码实现，不给目录之外的文件内容
- 你卡住超过 30 分钟就来问，带上**具体报错**和**你已经试过什么**（P1/P2 的老规矩）
