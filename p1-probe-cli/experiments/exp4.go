package main

import "fmt"

func main() {
	jobs := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		jobs <- i
	}
	//close(jobs)
	for j := range jobs {
		fmt.Println(j)
	}
	fmt.Println("all done")
}
