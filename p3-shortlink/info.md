# v1
## 目的
做一个shortlink服务，把长的url转换为6位base62的短码，写入库，当访问短码时跳转原url。
- **接口定义在store包还是server包？**
    在store包，因为server依赖store.Store接口，不依赖具体实现。如果后面换其他的db/redis，server包代码零改动。
- **1062冲突重试放在store还是handler包?**
    重试放store层，因为重试逻辑和"怎么存"绑定(数据库约束)，handler不该知道MySQL的错误码。且store负责唯一性保证，这是其职责。
## 做完思考题
1. sql.Open 之后不 PingContext 直接提供服务，会发生什么？为什么 K8s 的 readinessProbe 会让这个问题更隐蔽？
    PingContext会尝试建立一条连接探测db是否存活。若直接提供服务不探测，若db已挂，请求会返回500。k8s的readinessProbe如果配置探测不当，mysql本来好的探测结果失败导致流量被摘除svc，导致mysql和挂了没区别。
2. MaxOpenConns=25 而 MySQL max_connections=151，如果这个服务扩到 6 个副本会发生什么？SRE 视角怎么算这个账？
    不是改成10了吗，如果扩大副本，每个pod都会尝试建立更多的conn，导致mysql连接池不断上升。算账就是 当前的mysql max_conn - 当前活跃 - 预留 / replicas。
3. 1062 冲突重试为什么要设上限？如果 62⁶ 快用满了，无限重试会怎样？
    上限是为了避免无限重试，服务卡死。
4. Get 里为什么返回自定义 ErrNotFound 而不是 sql.ErrNoRows？errors.Is 在这条链路上扮演什么角色？
    server层只看是否是哨兵错误来做决策，无需关心底下是Mysql还是别的什么。
5. handler 里用 context.Background() 代替 r.Context() 传给 store，功能上什么时候会出问题？
    超时控制/优雅退出/请求取消的场景出现问题