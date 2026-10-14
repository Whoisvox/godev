package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// 和 exp5 结构完全一样，唯一区别：每个任务耗时随机（模拟真实探测：有的 URL 快有的慢）
func main() {
	jobs := make(chan int, 10)
	results := make(chan string, 10)

	var wg sync.WaitGroup
	for w := 1; w <= 3; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range jobs {
				d := time.Duration(rand.Intn(50)) * time.Millisecond // 随机耗时
				time.Sleep(d)
				results <- fmt.Sprintf("job=%d worker=%d cost=%v", j, id, d)
			}
		}(w)
	}

	for i := 1; i <= 9; i++ {
		jobs <- i
	}
	close(jobs)

	go func() { wg.Wait(); close(results) }()

	for r := range results {
		fmt.Println(r)
	}
}
