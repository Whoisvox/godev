# P3 知识库（功能性事实直接查这里，不用自己撞）

> 串联原则：P3 是**服务端**，P1/P2 你是**客户端**。同一套 HTTP/并发知识，视角反过来用。

---

## Go 1.22+ 标准库路由（不需要 gorilla/chi/gin）

| 事实 | 说明 |
|------|------|
| `mux.HandleFunc("GET /{code}", h)` | 方法 + 通配符，Go 1.22 起标准库自带 |
| `r.PathValue("code")` | 取通配符值 |
| `GET /{code}` vs `GET /api/shorten` | 更具体的 pattern 优先，不会冲突 |
| 方法不匹配 | ServeMux 自动返回 **405 + `Allow` 头**，不用自己写 |
| 路径不匹配 | 自动 **404**，不用自己写 |
| pattern 冲突注册 | **启动时 panic**（fail fast，是好事，别 recover 掉） |
| `{code}` 只匹配一段 | `/a/b` 不匹配 `/{code}`；要跨段用 `/{path...}` |
| 必须 `http.NewServeMux()` | 不要用包级 `http.DefaultServeMux`（全局可被任何包注册，测试互相污染） |

---

## http.Server 超时四件套（SRE 必背）

```go
srv := &http.Server{
    Addr:              ":8080",
    Handler:           mux,
    ReadHeaderTimeout: 5 * time.Second,   // ① 必设
    ReadTimeout:       10 * time.Second,  // ②
    WriteTimeout:      15 * time.Second,  // ③ 必须 > ②
    IdleTimeout:       60 * time.Second,  // ④ keep-alive
}
```

1. **`ReadHeaderTimeout`**：读请求头的超时。**不设 = Slowloris 攻击**——攻击者发 1 字节就停，连接永久占用，几千条就能打死服务。这是生产事故级漏洞，不是性能优化。
2. **`ReadTimeout`**：读**完整请求**（头+体）的超时。有大 body（文件上传）时要么调大，要么改用 `http.MaxBytesReader` 限 body 大小 + 只设 ReadHeaderTimeout。
3. **`WriteTimeout`**：⚠️ **它从"读完请求头"那一刻开始计时，不是从写响应开始**。所以必须 > ReadTimeout + handler 处理时间，否则慢查询会让响应写到一半被砍。
4. **`IdleTimeout`**：keep-alive 连接空闲保留时长。不设则回落到 ReadTimeout，或默认 3 分钟。
5. **`http.Server` 没有"handler 执行超时"**。要限制单个请求处理时间，只有两条路：
   - `http.TimeoutHandler(h, 3*time.Second, "timeout")`（注意：它**不会真的中断**你的 handler goroutine，只是提前给客户端写 503，handler 仍在跑 → 可能 goroutine 堆积）
   - 自己在 handler 里 `ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)`（**推荐**，能真正取消下游 DB 查询）
6. **`r.Context()` 什么时候被 cancel**：客户端断开连接时、Server.Shutdown 时，Go 自动 cancel。→ **handler 里所有下游调用都必须传 `r.Context()`**，客户端跑了就别白干活。这是 P3 与 P2 的 context 差异：P2 是你主动取消（信号），P3 是**客户端替你取消**。
7. **`http.Server` 每连接 1 个 `conn.serve` goroutine**（P2 KNOWLEDGE.md 那张表的服务端模型）。所以 `go_goroutines ≈ 基线 + 活跃连接数`，压测时看这个数字能直接反推连接是否复用（keep-alive 生效时连接数远小于请求数）。

---

## database/sql 功能性事实

### 连接池（最容易踩的一节）

| 事实 | 说明 |
|------|------|
| `sql.Open()` **不建连接** | 只校验参数格式、返回池句柄。DSN 写错密码它也不报错。→ 启动时必须 `db.PingContext(ctx)` 才能 fail fast |
| `*sql.DB` 是**池**，进程级单例 | 并发安全。**绝对不要每请求 Open/Close**（= 每请求新建一个池，经典性能错误） |
| `SetMaxOpenConns(n)` | 池上限。默认 **0 = 无限** → 高并发直接打爆 MySQL `max_connections`（默认 151），报 `Too many connections` |
| `SetMaxIdleConns(n)` | 空闲保留数。**默认只有 2**（和 `http.Transport.MaxIdleConnsPerHost=2` 一个脾气）。不调大 → 高并发下疯狂建连/TLS 握手 |
| `SetConnMaxLifetime(d)` | **必须设**。否则连接被中间层（LB / MySQL `wait_timeout`=8h）单方面断掉后，池里的**死连接**会让下个请求拿到 `invalid connection` / `broken pipe`。典型 5m |
| `SetConnMaxIdleTime(d)` | 空闲多久回收，典型 1m |
| 约束 | `MaxIdleConns` 超过 `MaxOpenConns` 会被自动截断 |
| 池满时 | 后续请求**阻塞等待**（受传入 ctx 控制），不是立刻报错 → 表现为延迟飙升而非 500 |

推荐起点：`MaxOpen=25, MaxIdle=25, ConnMaxLifetime=5m, ConnMaxIdleTime=1m`。压测阶段用 `db.Stats()` 观察（`InUse/Idle/WaitCount/WaitDuration` 四个字段直接告诉你池够不够）。

### ⚠️ 共享实例上的池容量算术（SRE 必算，本项目直接适用）

集群里的 MySQL 是**共享**的：`max_connections = 151`，全集群所有服务抢这一个池。

```
可用连接预算 = max_connections - 现有占用 - 运维预留(super + 你自己调试)
             = 151 - 2 - 10 ≈ 139
单副本上限   = 139 / 预期副本数
```

**结论：本项目 `MaxOpenConns` 设 10，不是 25。** 而且必须能配置化——将来部署到 K8s 扩到 3 副本时，每副本 10 × 3 = 30，仍在预算内；如果你按 25 配，3 副本就是 75，加上别人随时可能把你挤到 `Too many connections`。

这就是"**连接池容量是集群级共享资源，不是进程私有参数**"。同理适用于 HikariCP、pgbouncer、Redis PoolSize。面试常问"你怎么定连接池大小"，正确答案不是背公式，而是：**先看下游总容量和其他租户占用，再除以副本数，最后压测用 `db.Stats().WaitCount` 验证够不够**。

> 反过来：**池不是越大越快**。MySQL 单实例的有效并发远低于连接数（CPU/IO 才是瓶颈），池开 200 只会把排队从客户端搬到服务端，增加上下文切换和锁竞争。经验值：`MaxOpen ≈ CPU核数 × 2 ~ 4`（下游侧的核数）。

### 查询与错误

| 事实 | 说明 |
|------|------|
| `Rows` 必须 `defer rows.Close()` | 否则连接不归还池 → 连接泄漏。`QueryRow` 不用（内部自动处理） |
| `sql.ErrNoRows` **不是错误** | 是正常业务分支。`errors.Is(err, sql.ErrNoRows)` → 返回 404。**绝不能当 500 抛出去**（会把"短码不存在"变成告警风暴，参见 P2 ProbeErrorRateHigh） |
| `rows.Scan(&x)` 遇到 NULL | 目标必须是 `sql.NullString` / `NullInt64` / `NullTime`，直接扫进 `string` 报错。或建表就 `NOT NULL DEFAULT ''` 绕开 |
| 占位符 | MySQL `?`，Postgres `$1`。**永远不要 `fmt.Sprintf` 拼 SQL**（注入） |
| 预编译 | driver 对 `?` 内部会 prepare。循环里反复查同一语句可显式 `db.Prepare` 复用 |
| 必须带 `ctx` | 用 `QueryRowContext / ExecContext / QueryContext`，不用无 ctx 版本（否则客户端断开后 DB 查询照跑，连接被白占） |

### MySQL driver（go-sql-driver/mysql）

- DSN 格式：`user:pass@tcp(host:3306)/dbname?parseTime=true&charset=utf8mb4&timeout=3s`
  - **`parseTime=true` 必加**，否则 `DATETIME` 扫出来是 `[]byte` 不是 `time.Time`
  - `timeout=3s` 是**建连**超时；`readTimeout/writeTimeout` 是读写超时
- 判重键冲突：
  ```go
  var me *mysql.MySQLError
  if errors.As(err, &me) && me.Number == 1062 { /* duplicate key */ }
  ```
  1062 = `ER_DUP_ENTRY`。**这比"先 SELECT 再 INSERT"正确**——后者有竞态（两个请求同时查都不存在，然后都插）。
- driver 自身会注册 `go_sql_*` 指标？**不会**。要 SQL 层指标得自己埋（v2 任务）。

---

## 短码生成（本项目最核心的设计决策）

### 编码字符集
- **base62** = `0-9A-Za-z`（62 字符），URL 安全，无需转义。
- ❌ 不要用 base64：含 `+` `/` `=`，在 URL 里是保留字符。
- 6 位 base62 = 62⁶ ≈ **568 亿**，足够。

### 三种方案对比

| 方案 | 优点 | 致命缺点 |
|------|------|---------|
| **A. hash 截断**<br>`sha256(url)[:6]` | 同 URL → 同短码（天然去重） | 生日碰撞：568 亿空间在 **~34 万条**记录时碰撞率就到 50%。必须处理碰撞（加后缀 → 破坏"同 URL 同码"这个唯一优点） |
| **B. 发号器 + base62**<br>DB 自增 ID → 编码 | 零碰撞、无重试、O(1) | 短码**可枚举**（`abc` → `abd`），任何人能爬遍你的全部链接 = 信息泄漏 + 被刷 |
| **C. 随机码 + UNIQUE 索引 + 冲突重试** ✅ | 不可枚举、实现最简、无需发号器组件 | 有极小概率冲突，需重试（用错误码 1062 判定，见上文） |

**本项目选 C**。B 的"可枚举"问题在真实场景里是安全事故级别（短链服务存了内部系统链接就完蛋）；A 的碰撞处理逻辑复杂度不值得。
C 的实现骨架：
```go
for i := 0; i < maxRetry; i++ {
    code := genCode(6)
    _, err := db.ExecContext(ctx, `INSERT INTO links(code, url) VALUES(?, ?)`, code, url)
    if err == nil { return code, nil }
    var me *mysql.MySQLError
    if errors.As(err, &me) && me.Number == 1062 { continue } // 撞了就换一个
    return "", err  // 其他错误不要重试
}
return "", ErrCodeExhausted
```
⚠️ 关键：**判冲突靠 DB 的 UNIQUE 约束，不要"先 SELECT 再 INSERT"**（两个并发请求会同时查到不存在然后都插入，这就是 TOCTOU 竞态，和你 P1 遇到的"先检查再操作"是同一类 bug）。数据库约束是唯一可信的仲裁者。

- 随机源：`crypto/rand` 而非 `math/rand`（后者可预测，等于把可枚举问题换个形式带回来）。`crypto/rand.Read` 有 syscall 开销但可忽略；追求极致可预生成熵池。
- 冲突率参考：插入第 N 条时的碰撞概率 ≈ N / 62⁶。到 100 万条时单次碰撞率 0.0018% → 重试一次基本必成。

### 301 vs 302（**必须搞清，直接影响功能正确性**）

| | 301 Moved Permanently | 302 Found / 307 Temporary |
|---|---|---|
| 浏览器行为 | **缓存**，下次直连目标 URL，不再问你 | 每次都回到你的服务 |
| 点击统计 | **全部丢失** | 完整保留 |
| 服务端压力 | 极低 | 每次点击都打到你 |
| SEO 权重 | 转移给目标 URL | 保留在原 URL |

**短链服务用 302**（要统计点击）。这是功能需求决定的，不是性能偏好。`http.Redirect(w, r, url, http.StatusFound)`。
> Go 的 `http.Redirect` 对 301/302/303/307 会自动写一个 HTML body（老浏览器兼容），正常。

---

## 缓存（v2 用）

| 问题 | 现象 | 解法 |
|------|------|------|
| 穿透 | 查一个**根本不存在**的 key，每次都打穿到 DB（攻击者可刷爆） | 空值也缓存（短 TTL，如 30s）；或布隆过滤器 |
| 击穿 | 某个**热点 key 刚好过期**，瞬间上千请求同时打 DB | `singleflight`（同 key 只放一个请求去查，其余等结果）；或热点 key 不过期 + 后台刷新 |
| 雪崩 | **大量 key 同时过期**（如批量写入时都设了 600s） | TTL 加随机抖动：`600s + rand(0~60s)` |
| 一致性 | 更新 DB 后缓存怎么办 | **先更新 DB，再删缓存**（不是更新缓存）。删失败靠 TTL 兜底 |

- `singleflight` 在 `golang.org/x/sync/singleflight`（不在标准库）。核心 API：`Do(key, fn)` → 同 key 并发调用只有一个真执行 fn，其余共享返回值。
- **降级**：Redis 连不上时不能返回 500，要**静默回落到 DB**（缓存是加速器不是数据源）。用 `errors.Is` 区分"缓存 miss"和"缓存故障"，后者打日志 + 计指标（`cache_errors_total`），但请求继续走。这是 v3 混沌测试的验收点。
- Redis 连接池：`redis.Options{PoolSize, MinIdleConns, DialTimeout, ReadTimeout}`，语义和 `sql.DB` 那套几乎一样（`go-redis` 底层也是池）。

---

## slog 结构化日志（v1 用）

- `slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))` —— **生产用 JSON**，Loki/ELK 才能按字段查。
- **所有日志写 stdout/stderr，不要写文件**（12-factor；K8s 里 kubelet 负责收集，你写文件 = 容器重启就丢 + readOnlyRootFilesystem 直接崩，P2 你已经开过这个 securityContext）。
- key-value 成对：`slog.Info("msg", "code", c, "dur", d)`。漏一个参数 → `go vet` 报 `slog` 检查（P1 你见过 lostcancel，同一个 vet 机制）。
- `logger.With("request_id", id)` 派生带固定字段的子 logger，中间件里生成 request_id 后往下传。
- **`slog.Any` 的坑**：传 error 要用 `slog.Any("err", err)` 或专门的 `slog.String("err", err.Error())`；直接 `slog.Error("failed", err)` 会把 err 当成 **msg 的一部分**而不是字段（`Error(msg string, args ...any)`）。
- 日志级别纪律：`Debug` 单请求详情、`Info` 启动/关闭/重要状态变更、`Warn` 可自愈异常（缓存降级）、`Error` 请求失败（5xx）。**别把 4xx 记成 Error**——那是客户端的错，会污染错误率。

---

## 令牌桶限流（v1 自己实现，不用库）

原理（**面试常问，要能白板画**）：
- 桶容量 `burst`，以固定速率 `rate` 个/秒往里放令牌，满了就丢
- 请求来了取 1 个令牌：拿到 → 放行；拿不到 → 429
- **效果**：长期平均速率 = `rate`，但允许短时突发到 `burst`（吸收抖动，不像漏桶那样死板）
- 实现要点：**不要真的开 goroutine 定时放令牌**（浪费且不准）。用"懒计算"：记录 `lastTime` 和 `tokens`，取令牌时按 `now - lastTime` 算出这段时间该补多少：
  ```
  tokens = min(burst, tokens + float64(now.Sub(last)) * ratePerNs)
  ```
  这就是 `golang.org/x/time/rate` 的做法。全程只需一把 `sync.Mutex`。
- 标准库没有，`golang.org/x/time/rate` 有（`rate.NewLimiter(rate.Limit(100), 200)` + `limiter.Allow()`）。**v1 先自己写一遍**，再用 x/time/rate 替换并对比——这样你才知道库在干什么。
- 限流维度：全局限流（保护进程）vs 按 IP/按 API key 限流（防单租户打爆）。按 IP 限流要 map + 锁 + **过期清理**（否则 map 无限增长 = 内存泄漏，本质和 goroutine 泄漏同类问题）。v1 先做全局的，按 key 的作为选修。
- 429 响应必须带 **`Retry-After: <秒>`** 头（HTTP 语义，客户端/SDK 会读）。

---

## 与 P1 / P2 的知识串联

| P3 概念 | 来源 |
|---------|------|
| 优雅退出（signal.NotifyContext + srv.Shutdown） | **P2 main.go 原样复用**，但要多加一步：排空异步点击 channel（P1 fan-in 排空经验） |
| 异步点击落库（jobs channel → N workers → 批量 INSERT） | **P1 v2 worker pool 原样搬**，fan-out/fan-in + WaitGroup |
| `context` 传播 | P2 是"我主动取消"（信号），P3 是 **`r.Context()` 客户端替我取消** |
| goroutine 数 ≈ 基线 + 活跃连接数 | P2 KNOWLEDGE.md 服务端模型表（`conn.serve`） |
| `/metrics` + Histogram + P99 | P2（你在 logsvc 已做过 Histogram/Grafana，直接搬经验） |
| data race 检测 | `go test -race`，P1 v2 那次 append 竞争的教训直接适用于点击聚合 |
| 错误分级 → 告警严重度 | P2 alerts.yml：4xx≠5xx≠"我瞎了"。短链服务同理：**DB 挂了要 page，某个短码 404 不该告警** |
| 配置 fail fast | P2 `UnmarshalStrict` + `Interval<=0` 校验 → P3 启动时 PingContext + 校验配置，**宁可起不来也别半死不活** |

---

## 工具链事实

- `wrk` 已装（`/usr/local/bin/wrk`），`hey` 没装。压测用 wrk：
  ```bash
  wrk -t4 -c100 -d30s --latency http://localhost:8080/<code>
  ```
  `-t` 线程（≤ CPU 核数）、`-c` 并发连接、`-d` 时长、`--latency` 输出 P50/P90/P99 分布。
  **wrk 默认不发 body**，POST 压测要配 `-s post.lua` 脚本。
- **压测纪律（P1 定过的规矩，继续有效）**：
  - 永远不要压公网目标
  - 压测前 `docker stats` / `kubectl top` 确认资源，压测中盯 `db.Stats()` 和 `go_goroutines`
  - **先建立基线再优化**：没有"优化前"数字的优化故事在面试里等于零分
- pprof：**只绑 127.0.0.1**（P1 定过），`import _ "net/http/pprof"` + 单独 goroutine 跑
  ```go
  go func() { log.Println(http.ListenAndServe("127.0.0.1:6060", nil)) }()
  ```
  ⚠️ 这里传 `nil` handler 会用 `DefaultServeMux`——pprof 的 init() 正是往 DefaultServeMux 注册的。**这是 DefaultServeMux 唯一合理的使用场景**，且因为只绑 loopback，不违反上面"用 NewServeMux"的建议。
  火焰图：`go tool pprof -http=:8085 'http://127.0.0.1:6060/debug/pprof/profile?seconds=30'`
- **中间件用集群现成的，不在 174 本机起容器**（本机还跑着 Harbor 全家桶）：
  - MySQL 8.0.44：`192.168.5.241:31554`（default/mysql-service，共享实例，max_connections=151）
  - Redis 8.0.3 standalone：`192.168.5.241:32523`（default/redis-nodeport）
  - ⚠️ `redis` ns 的 `:31639` 是 **cluster 模式**（6 节点），go-redis 得换 `NewClusterClient`，v4 不用它
  - 应用部署进 K8s 后走 ClusterIP/Service DNS（`mysql-service.default.svc.cluster.local:3306`），不再绕 NodePort
- **混沌测试的新手段**：没有本机容器可 `docker stop` 了，改用 `kubectl -n default scale deploy/redis-deployment --replicas=0` 模拟 Redis 消失（v4 验收点），完事记得恢复——P2 演练 B 的同款纪律。
