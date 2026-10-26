# P2 知识库（随进度持续补充，功能性知识直接查这里）

## Prometheus 功能性事实（不用自己撞，直接用）

1. **rate()/increase() 只接受 Counter**。对 Gauge 用 rate() 结果无意义。
   只要一个值语义是"累计次数"且将来要算速率（QPS、失败率、告警阈值），必须 Counter。
2. **Counter reset 语义**：exporter 重启后 Counter 归零，rate() 自动识别为"重启"而非负增长。
   Gauge 下降无法区分"真降了"还是"重启了"。
3. **go_\* / process_\* 指标**：client_golang 的 init() 自动向默认 registry 注册
   GoCollector（go_goroutines、go_gc_*、go_memstats_*）+ ProcessCollector（process_cpu_*、process_open_fds、process_start_time_seconds）。
   用 promhttp.Handler()（默认 registry）即白得。
4. **端口惯例**：9090 是 Prometheus server 自己的默认端口；exporter 惯例用 91xx
   （node_exporter=9100，blackbox_exporter=9115）。本项目用 **9115**。
5. **metric 命名规范**：`<namespace>_<name>_<unit>`，小写下划线；
   累计值以 `_total` 结尾；单位进名字（`probe_duration_seconds` 不是 `probe_duration`）。
6. **Help 必填**：会渲染进 /metrics 的 `# HELP` 行，是 Grafana/Prometheus UI 里的文档。
7. **label 基数铁律**：label 只放有限可枚举值。高基数（用户ID/请求ID/完整URL/时间戳）
   进 label = 时间序列爆炸 = Prometheus OOM。每个 label 组合 = 一条独立时间序列 = 持续占内存磁盘。
8. **MustRegister 重复注册同名 metric 会 panic**（启动期 fail fast，属编程错误）。
9. **单个 metric 对象并发安全**：Set/Inc/Add 内部原子实现，多 goroutine 更新同一 metric 无 race。
   但**聚合语义**要想清楚：多 worker 对同一 label 组合并发 `Set` 会互相覆盖（最后写的赢）；
   并发 `Inc` 则安全（原子累加）。→ 聚合值（如整体成功率）必须单 goroutine 汇总后 Set。
10. **scrape 与探测是两个独立周期**：exporter 后台 ticker 探测（如 30s），
    Prometheus 按 scrape_interval 来抓（如 30s）。被抓瞬间读到的是"最近一轮"结果。
    探测间隔应 ≤ scrape 间隔，否则 Prometheus 会抓到重复的旧数据。

## goroutine 基线构成（v0 实测 + SIGQUIT dump 验证）

静置成熟 exporter 基线 ≈ 5（**不是固定常数**，GC 内务 goroutine 懒创建）：

| 身份 | 状态 | 说明 |
|------|------|------|
| main | IO wait | 阻塞在 ListenAndServe → Accept |
| forcegchelper | force gc (idle) | runtime 内务 |
| GC sweep | GC sweep wait | **第一次 GC 后才出现** |
| GC scavenge worker | GC scavenge worker | 同上 |
| (+conn.serve ×活跃连接数) | IO wait/select | 每条抓取连接 1 个（服务端模型，无客户端 readLoop/writeLoop） |

- 抓取时刻 go_goroutines = 基线 + 当次 scrape 连接的 conn.serve（+1）
- curl keep-alive 连续请求复用连接 → 只 +1 不 +2（实测验证）
- **与 P1 的区别**：P1 probe-cli 是客户端（每连接 readLoop+writeLoop 2个）；
  exporter 纯服务端（每连接 conn.serve 1个）
- **泄露检测看趋势不看常数**：基线随 runtime 状态浮动（P1 泄露测试 +1 容忍度的深层原因）

## SIGQUIT dump 技巧（无 pprof 时看 goroutine）

```bash
kill -QUIT <pid>    # 进程退出，但 stderr 打印全部 goroutine 栈（比 pprof 更原始）
```
注意：只对能接受进程退出的场景用（调试/实验）。生产排查用 pprof。

## 与 P1 的知识串联

| P2 概念 | 对应 P1 知识 |
|---------|-------------|
| daemon 探测循环（ticker + ctx 退出） | P1 v3 的 signal.NotifyContext + 优雅退出 |
| probeOnce 的 worker pool | P1 v2 的 fan-out/fan-in 原样复用 |
| metric 并发更新语义 | P1 v2 data race：label 组合 = "预分配下标"，独占更新无竞争 |
| goroutine 数稳定性检查 | P1 v5 泄露检测（NumGoroutine 趋势 + pprof） |
| 聚合值单 goroutine 汇总 | P1 v2 的 collector 角色 |
