package main

import (
	"fmt"
	"sync"
)

func main() {
	jobs := make(chan int, 10)
	results := make(chan int, 8)

	var wg sync.WaitGroup
	for w := 1; w <= 3; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range jobs {
				results <- j * 2
			}
		}(w)
	}

	for i := 1; i <= 9; i++ {
		jobs <- i
	}

	close(jobs)

	// go func() {
	// 	wg.Wait()
	// 	close(results)
	// }()
	wg.Wait()
	close(results)

	for r := range results {
		fmt.Println(r)
	}
}
