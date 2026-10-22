# v0
## 自定义两个指标
我定义的指标都有了，.Set(0)，probe_success变为0。go_goroutines是6个（含curl），exporter是纯服务端，每连接+1，没有客户端readLoop/writeLoop"，没连接时是5个。
    root@vox-174:p2-exporter# pkill -f probe-exporter; sleep 2   
    root@vox-174:p2-exporter# 
    root@vox-174:p2-exporter# go run ./cmd/probe-exporter/ &
    [1] 3278796
    root@vox-174:p2-exporter# sleep 2
    root@vox-174:p2-exporter# curl -s localhost:9090/metrics | grep '^go_goroutines'
    go_goroutines 6
    root@vox-174:p2-exporter# sleep 3
    root@vox-174:p2-exporter# 
    root@vox-174:p2-exporter# curl -s localhost:9090/metrics | grep '^go_goroutines'
    go_goroutines 7
    root@vox-174:p2-exporter# sleep3
    Command 'sleep3' not found, did you mean:
    command 'sleep' from deb coreutils (9.4-3ubuntu6.2)
    Try: apt install <deb name>
    root@vox-174:p2-exporter# 
    root@vox-174:p2-exporter# sleep 3
    root@vox-174:p2-exporter# 
    root@vox-174:p2-exporter# curl -s localhost:9090/metrics | grep '^go_goroutines'
    go_goroutines 6
    root@vox-174:p2-exporter# kill %1
    root@vox-174:p2-exporter# echo $1
    [1]+  Terminated              go run ./cmd/probe-exporter/

    root@vox-174:p2-exporter# 
*三个回答*
    - probe_success是Gauge而不是Counter,因为计算速率得函数 rate()/increase()只能用于Counter。
    - 若probe_requests_total也用Gauge，没法算速率。
    - metrices里的哪些go_*是client_golang注册的，client_golang在init()时自动向默认regitry(prometheus.DefaultRegisterer)注册两个colector：NewGoCollector()和NewProcesssCollector()。用promhttp.Handler()默认读这个默认得registry。