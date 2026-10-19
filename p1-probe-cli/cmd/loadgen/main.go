// loadgen 是 probe-cli 的本地压测靶 server：
// 随机延迟 10~60ms、10% 概率注入 503，并提供请求计数对账端点。
// 用法: ./bin/loadgen  （前台运行，Ctrl-C 停止）
package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"sync/atomic"
	"time"
)

// 单变量计数用 atomic，不需要 mutex（mutex 用于保护复合状态/多变量一致性）
var totalRequests atomic.Int64

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/target/", func(w http.ResponseWriter, r *http.Request) {
		totalRequests.Add(1)

		// 1. 随机响应延迟 10~60ms
		delay := time.Duration(rand.IntN(50)+10) * time.Millisecond
		time.Sleep(delay)

		// 2. 10% 概率注入 503（观察 probe-cli 的重试行为）
		if rand.Float64() < 0.1 {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		// 3. 正常响应
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// 对账端点：每轮压测后核对 total_requests ≈ 目标数 × (1 + 重试率)
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "total_requests=%d\n", totalRequests.Load())
	})

	// 每轮压测前归零计数，避免跨轮累计
	mux.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		totalRequests.Store(0)
		fmt.Fprintln(w, "ok")
	})

	// log 默认写 stderr；ListenAndServe 失败（如端口被占）必须让进程炸出来
	log.Println("loadgen listening on :18081")
	log.Fatal(http.ListenAndServe(":18081", mux))
}
