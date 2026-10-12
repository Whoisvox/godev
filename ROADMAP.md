# Go × SRE 练手项目路线图

> 目标：从 Go 语法基础水平，通过 4+1 个小而精的项目，达到能独立开发运维工具/服务、
> 有可展示亮点的中级 SRE 候选人水平。
>
> 协作模式：Claude 讲前置知识 + 拆任务 + code review + 验收；本人亲手写全部核心代码。

## 部署环境（已核实）

- **开发机**：/data/godev，Go 1.25.9，docker/git/kubectl/make 齐备
- **Harbor**：本机 docker 运行，v2.11，API 在 `http://localhost`（80 端口）
  - **镜像地址必须用 `192.168.5.174:80/godev/<name>:<tag>`**（不能用 localhost，
    集群其他节点解析不到）；daemon.json 已配 insecure-registries，链路已验证
- **K8s 集群**：kubeadm 4 节点（master + node1~3），v1.32.9，containerd 运行时
  - P4/P5 直接用此集群，无需 kind/minikube
- **集群已有可观测栈**：Prometheus Operator + Grafana + Loki + Jaeger
  - P2 阶段：先 docker-compose 本地实验，再把 exporter 部署进集群
    让现有 Prometheus 抓取（ServiceMonitor 或 pod annotation）
- 每个项目一个独立文件夹 + 独立 git 仓库，推送到 GitHub 记录学习过程

## 统一工程标准（每个项目都必须做到）

- 独立 git 仓库，语义化 commit message
- Makefile：`make build / test / lint / run / docker`
- golangci-lint 零告警
- `go test -race` 全部通过
- Dockerfile（多阶段构建）
- README：功能说明、架构图、关键设计决策、验收/压测数据

---

## P1 并发探测器 probe-cli（1~1.5 周）

**功能规格**
- 从配置文件/命令行读取一批探测目标（HTTP URL / TCP 地址）
- 并发探测：HTTP 状态码、延迟；TCP 连通性
- 两种模式：单次探测输出报告；daemon 模式按周期探测
- 失败重试（指数退避）、每目标独立超时
- Ctrl-C 优雅退出（等待进行中的探测完成）
- 进阶扩展：暴露 Prometheus 指标（为 P2 做铺垫）

**前置知识清单**
- goroutine 调度基础、channel（有/无缓冲、方向）、select、close 语义
- context：取消传播、超时控制（WithTimeout/WithCancel）、ctx.Done() 惯用法
- sync 包：WaitGroup、Mutex、Once、atomic
- fan-out/fan-in、worker pool 模式
- net/http Client 的 Timeout vs context 取消的区别
- os/signal 信号处理、优雅退出流程
- cobra 命令行框架基础

**常见坑（讲解阶段覆盖）**
- goroutine 泄露的几种成因与 pprof 检测方法
- channel 死锁场景
- 在 goroutine 里捕获循环变量（Go 1.22 前的坑）
- WaitGroup Add/Done 配对错误

**验收标准**
- [ ] `go vet`、golangci-lint、`go test -race` 全过
- [ ] 1000 个目标在合理并发度下完成探测，记录耗时数据
- [ ] Ctrl-C 后进程干净退出，无孤儿 goroutine（pprof goroutine 数验证）
- [ ] 单元测试覆盖核心逻辑（超时、重试、取消）
- [ ] 能口头解释：worker pool 的 channel 拓扑、context 取消如何逐层传播

---

## P2 自定义 Prometheus Exporter（3~5 天）

**功能规格**
- 写一个 exporter，暴露自定义指标（可选题材：本机进程/日志关键字监控，
  或直接消费 P1 的探测结果——推荐后者，形成链路）
- docker-compose 拉起 Prometheus + Grafana（+ Alertmanager）
- Prometheus 抓取 exporter，Grafana 建一块面板
- 写 1~2 条告警规则（如：探测失败率 > 阈值）

**前置知识清单**
- Prometheus 数据模型：四种 metric 类型（Counter/Gauge/Histogram/Summary）、
  label 与基数爆炸问题
- client_golang 用法：默认 registry、自定义 Collector、/metrics handler
- exposition 文本格式
- prometheus.yml 抓取配置、alerting rules、Grafana 数据源与 PromQL 基础

**验收标准**
- [ ] Prometheus targets 页面显示 UP
- [ ] Grafana 面板能看到实时数据，PromQL 查询正确
- [ ] 手动制造故障，告警规则成功触发
- [ ] 能解释 Histogram 的 bucket/le 含义，以及为什么 label 不能放高基数值

---

## P3 短链接服务（1.5~2 周）

**功能规格**
- `POST /api/shorten`：长链接 → 短码（发号器或 hash 取短）
- `GET /{code}`：302 跳转，异步记录点击（channel + 落库）
- 存储：SQLite 起步，进阶换 MySQL（docker）；Redis 做热点缓存
- 中间件链：请求日志（slog）、恢复 panic、限流（令牌桶）、认证（简单 API key）
- `/healthz`、`/readyz`、`/metrics`（复用 P2 经验）
- 配置管理：环境变量 + 配置文件
- Dockerfile + docker-compose（app + mysql + redis）

**前置知识清单**
- net/http：Go 1.22+ 路由（`GET /{code}` 语法）、Handler/HandlerFunc、
  ServeMux、http.Server 各超时参数
- 中间件模式（装饰器思想，用标准库实现，不引框架）
- database/sql：连接池参数、预编译语句、事务、sql.NullString 等扫描坑
- 缓存三大问题：穿透（空值缓存/布隆过滤器）、击穿（singleflight）、
  雪崩（过期时间打散）
- 令牌桶限流原理（可自己实现一个，不用库）
- slog 结构化日志：Handler、Attr、With
- 表驱动测试、httptest、testify 可选
- pprof：CPU/内存 profile 的采集与火焰图解读

**验收标准**
- [ ] 全部 API 有表驱动测试，覆盖率 ≥ 70%
- [ ] wrk/hey 压测：记录优化前 QPS/P99，用 pprof 找瓶颈，优化后对比
  （目标：讲清楚至少一次"发现问题→定位→修复→数字变化"的完整故事）
- [ ] 混沌测试：Redis 挂掉时服务降级可用；MySQL 断连后自动恢复
- [ ] docker-compose 一键起全栈
- [ ] golangci-lint 零告警，README 含架构图与压测数据

---

## P4 K8s 巡检 CLI（client-go）（1 周）

**功能规格**
- 类 kubectl 的巡检小工具，子命令：
  - `check`：扫描异常 Pod（CrashLoopBackOff / Pending / 重启次数 > N / OOMKilled），
    输出巡检报告（表格 + 原因分析）
  - `events`：watch 指定 namespace 的事件流
  - `why`：给定 pod，聚合其 events + 容器退出原因 + 最近日志，给出诊断
- 支持 --kubeconfig / --namespace / --all-namespaces
- 本地用 docker 起 kind 或 minikube 做实验环境

**前置知识清单**
- K8s API 对象模型：Pod/Event/Node 的关键字段（status、conditions、
  containerStatuses）
- client-go 架构：RESTClient → ClientSet → Informer/Lister → WorkQueue
- Informer 机制重点：本地缓存、Reflector/DeltaFIFO、事件回调
  （这是理解一切 controller 的钥匙）
- kubectl 的 kubeconfig 加载规则
- client-go 的 API 兼容与版本选择

**验收标准**
- [ ] 在 kind 集群里人为制造 CrashLoopBackOff、OOMKilled、Pending（资源不足）
  三种异常，工具全部识别且原因描述准确
- [ ] events 子命令能实时 watch 到新建 pod 的事件
- [ ] 用 Informer 缓存而非直接 List 全量（能解释对 API Server 的压力差异）
- [ ] 单元测试用 client-go 的 fake clientset

---

## P5 简易 Operator（进阶选修，2 周）

**功能规格**（P4 完成后二选一）
- 方案 A：`CleanupJob` CRD——声明式清理已完成的 Job / 超龄 Pod
- 方案 B：`AppHealth` CRD——监控指定 Deployment 健康度，异常时更新 CR status
  并发 Event
- 用 kubebuilder 脚手架 + controller-runtime，部署到 kind 集群验证

**前置知识清单**
- CRD / controller 模式 / reconcile 循环（level-triggered vs edge-triggered）
- kubebuilder 项目结构、marker 注释生成 RBAC/CRD
- OwnerReference 与级联删除、finalizer
- controller-runtime 的 Manager/Reconciler/Builder

**验收标准**
- [ ] CR 创建后 reconcile 正确执行，status 实时更新
- [ ] 删除 CR 时 finalizer 逻辑正确
- [ ] envtest 或 fake client 的 reconcile 单测
- [ ] README 含 reconcile 流程图

---

## 里程碑总览

| 阶段 | 产出 | 检验方式 |
|------|------|---------|
| P1 完成 | 扎实的 Go 并发功底 | 能白板画 worker pool + context 取消链 |
| P2 完成 | 可观测性入门 | 完整监控告警链路跑通 |
| P3 完成 | 独立开发 Web 服务的能力 | 压测报告 + 一次完整的性能优化故事 |
| P4 完成 | K8s 编程能力 | 巡检工具在真实异常场景下工作 |
| P5 完成 | Operator 开发（强简历项） | CRD + controller 部署验证 |

每个项目结束写一篇简短复盘（踩坑 + 设计决策），沉淀为面试素材。
